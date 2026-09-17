package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

var ErrAuthenticationFailed = &model.ApplicationError{Code: "authentication_failed", Message: "invalid username or password"}
var ErrAuthenticationRequired = &model.ApplicationError{Code: "authentication_required", Message: "authentication required"}

type AuthenticationDependencies struct {
	Clock            ports.Clock
	IDs              ports.IDGenerator
	Passwords        ports.PasswordVerifier
	Sessions         ports.SessionStore
	LoginThrottles   ports.LoginThrottleStore
	Administrator    model.Administrator
	SessionLifetime  time.Duration
	ThrottleWindow   time.Duration
	ThrottleFailures int
	BlockDuration    time.Duration
}
type SignInInput struct{ SessionToken, Username, Password, Source string }
type Session struct {
	ID               model.ID
	Token, CSRFToken string
	State            model.SessionState
}
type Authenticator struct{ dependencies AuthenticationDependencies }

func NewAuthenticator(dependencies AuthenticationDependencies) *Authenticator {
	if dependencies.Clock == nil {
		dependencies.Clock = systemClock{}
	}
	if dependencies.Sessions == nil {
		dependencies.Sessions = NewMemorySessionStore()
	}
	if dependencies.LoginThrottles == nil {
		dependencies.LoginThrottles = NewMemoryLoginThrottleStore()
	}
	if dependencies.Passwords == nil {
		dependencies.Passwords = Argon2Verifier{}
	}
	if dependencies.ThrottleWindow == 0 {
		dependencies.ThrottleWindow = 10 * time.Minute
	}
	if dependencies.ThrottleFailures == 0 {
		dependencies.ThrottleFailures = 5
	}
	if dependencies.BlockDuration == 0 {
		dependencies.BlockDuration = 15 * time.Minute
	}
	return &Authenticator{dependencies: dependencies}
}
func (service *Authenticator) NewAnonymousSession(ctx context.Context) (Session, error) {
	return service.createSession(ctx, model.SessionAnonymous)
}
func (service *Authenticator) SignIn(ctx context.Context, input SignInInput) (Session, error) {
	now := service.dependencies.Clock.Now().UTC()
	blocked, _, err := service.dependencies.LoginThrottles.IsBlocked(ctx, input.Username, input.Source, now)
	if err != nil || blocked {
		return Session{}, ErrAuthenticationFailed
	}
	valid, err := service.dependencies.Passwords.Verify(ctx, service.dependencies.Administrator.PasswordHash, input.Password)
	if err != nil || input.Username != service.dependencies.Administrator.Username || !valid {
		_ = service.dependencies.LoginThrottles.RecordFailure(ctx, input.Username, input.Source, now)
		return Session{}, ErrAuthenticationFailed
	}
	if input.SessionToken != "" {
		if old, findErr := service.dependencies.Sessions.FindActiveSession(ctx, digest(input.SessionToken)); findErr == nil {
			_ = service.dependencies.Sessions.RevokeSession(ctx, old.ID, now)
		}
	}
	if err := service.dependencies.LoginThrottles.ClearFailures(ctx, input.Username, input.Source); err != nil {
		return Session{}, err
	}
	return service.createSession(ctx, model.SessionAuthenticated)
}
func (service *Authenticator) SignOut(ctx context.Context, token string) error {
	session, err := service.dependencies.Sessions.FindActiveSession(ctx, digest(token))
	if err != nil {
		return ErrAuthenticationRequired
	}
	return service.dependencies.Sessions.RevokeSession(ctx, session.ID, service.dependencies.Clock.Now().UTC())
}
func (service *Authenticator) Authorize(ctx context.Context, token string) (model.Principal, error) {
	session, err := service.dependencies.Sessions.FindActiveSession(ctx, digest(token))
	if err != nil || session.State != model.SessionAuthenticated {
		return model.Principal{}, ErrAuthenticationRequired
	}
	return model.Principal{SessionID: session.ID, Username: service.dependencies.Administrator.Username}, nil
}

// VerifyCSRF reports whether csrfToken matches the synchronizer value
// deterministically derived from the active session identified by token,
// using a constant-time comparison. An absent or invalid session never
// matches.
func (service *Authenticator) VerifyCSRF(ctx context.Context, token, csrfToken string) bool {
	if token == "" || csrfToken == "" {
		return false
	}
	if _, err := service.dependencies.Sessions.FindActiveSession(ctx, digest(token)); err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(CSRFTokenFor(token)), []byte(csrfToken)) == 1
}

// CSRFTokenFor deterministically derives the synchronizer token for a
// session token, so any page render can embed the current CSRF value
// without separate server-side storage.
func CSRFTokenFor(sessionToken string) string {
	sum := sha256.Sum256([]byte(sessionToken + ":csrf"))
	return hex.EncodeToString(sum[:])
}
func (service *Authenticator) createSession(ctx context.Context, state model.SessionState) (Session, error) {
	token := randomToken()
	csrf := CSRFTokenFor(token)
	record, err := service.dependencies.Sessions.CreateSession(ctx, digest(token), digest(csrf), state, service.dependencies.Clock.Now().UTC())
	if err != nil {
		return Session{}, err
	}
	return Session{ID: record.ID, Token: token, CSRFToken: csrf, State: state}, nil
}
func IsAuthenticationFailure(err error) bool {
	return errors.Is(err, ErrAuthenticationFailed) || model.ErrorCode(err) == "authentication_failed"
}
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func SessionCookieMaxAge() int   { return 0 }
func digest(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }
func randomToken() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	const alphabet = "0123456789abcdef"
	output := make([]byte, len(bytes)*2)
	for i, value := range bytes {
		output[i*2] = alphabet[value>>4]
		output[i*2+1] = alphabet[value&15]
	}
	return string(output)
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Argon2Verifier struct{}

func (Argon2Verifier) Verify(ctx context.Context, hash, password string) (bool, error) {
	return VerifyPassword(ctx, hash, password)
}

type memorySessionStore struct {
	mutex    sync.Mutex
	sessions map[string]model.WebSession
}

func NewMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: map[string]model.WebSession{}}
}
func (store *memorySessionStore) CreateSession(_ context.Context, token, csrf []byte, state model.SessionState, now time.Time) (model.WebSession, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	id := model.NewID()
	session := model.WebSession{ID: id, TokenDigest: append([]byte(nil), token...), CSRFDigest: append([]byte(nil), csrf...), State: state, CreatedAt: now}
	store.sessions[string(token)] = session
	return session, nil
}
func (store *memorySessionStore) FindActiveSession(_ context.Context, token []byte) (model.WebSession, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	session, ok := store.sessions[string(token)]
	if !ok || session.State == model.SessionRevoked {
		return model.WebSession{}, ErrAuthenticationRequired
	}
	return session, nil
}
func (store *memorySessionStore) RevokeSession(_ context.Context, id model.ID, now time.Time) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	for key, session := range store.sessions {
		if session.ID == id {
			session.State = model.SessionRevoked
			session.RevokedAt = &now
			store.sessions[key] = session
			return nil
		}
	}
	return ErrAuthenticationRequired
}

type memoryThrottleStore struct {
	mutex  sync.Mutex
	values map[string]model.LoginThrottle
}

func NewMemoryLoginThrottleStore() *memoryThrottleStore {
	return &memoryThrottleStore{values: map[string]model.LoginThrottle{}}
}
func (store *memoryThrottleStore) key(username, source string) string {
	return username + "\x00" + source
}
func (store *memoryThrottleStore) IsBlocked(_ context.Context, username, source string, now time.Time) (bool, time.Duration, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	value, ok := store.values[store.key(username, source)]
	if !ok || value.BlockedUntil == nil || !value.BlockedUntil.After(now) {
		return false, 0, nil
	}
	return true, value.BlockedUntil.Sub(now), nil
}
func (store *memoryThrottleStore) RecordFailure(_ context.Context, username, source string, now time.Time) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	key := store.key(username, source)
	value := store.values[key]
	if value.WindowStartedAt.IsZero() || now.Sub(value.WindowStartedAt) >= 10*time.Minute {
		value = model.LoginThrottle{UsernameKey: username, SourceAddress: source, WindowStartedAt: now}
	}
	value.FailureCount++
	value.UpdatedAt = now
	if value.FailureCount >= 5 {
		until := now.Add(15 * time.Minute)
		value.BlockedUntil = &until
	}
	store.values[key] = value
	return nil
}
func (store *memoryThrottleStore) ClearFailures(_ context.Context, username, source string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	delete(store.values, store.key(username, source))
	return nil
}
