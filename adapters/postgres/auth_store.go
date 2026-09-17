package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

var errAuthenticationRequired = &model.ApplicationError{Code: "authentication_required", Message: "authentication required"}

const throttleFailureLimit = 5

// AuthStore implements ports.SessionStore and ports.LoginThrottleStore against
// real PostgreSQL tables so revocation and pair-specific throttling remain
// correct across processes and restarts.
type AuthStore struct{ pool *pgxpool.Pool }

func NewAuthStore(ctx context.Context, connectionString string) (*AuthStore, error) {
	pool, err := NewPool(ctx, connectionString)
	if err != nil {
		return nil, err
	}
	return &AuthStore{pool: pool}, nil
}

func (store *AuthStore) Close() { store.pool.Close() }

func (store *AuthStore) CreateSession(ctx context.Context, tokenDigest, csrfDigest []byte, state model.SessionState, now time.Time) (model.WebSession, error) {
	id := model.NewID()
	_, err := store.pool.Exec(ctx,
		`INSERT INTO web_sessions (id, token_digest, csrf_token_digest, state, created_at) VALUES ($1, $2, $3, $4, $5)`,
		id.String(), tokenDigest, csrfDigest, string(state), now)
	if err != nil {
		return model.WebSession{}, err
	}
	return model.WebSession{ID: id, TokenDigest: tokenDigest, CSRFDigest: csrfDigest, State: state, CreatedAt: now}, nil
}

func (store *AuthStore) FindActiveSession(ctx context.Context, tokenDigest []byte) (model.WebSession, error) {
	var (
		id, state string
		csrf      []byte
		createdAt time.Time
	)
	err := store.pool.QueryRow(ctx,
		`SELECT id, csrf_token_digest, state, created_at FROM web_sessions WHERE token_digest = $1`,
		tokenDigest).Scan(&id, &csrf, &state, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.WebSession{}, errAuthenticationRequired
	}
	if err != nil {
		return model.WebSession{}, err
	}
	if model.SessionState(state) == model.SessionRevoked {
		return model.WebSession{}, errAuthenticationRequired
	}
	return model.WebSession{ID: model.ID(id), TokenDigest: tokenDigest, CSRFDigest: csrf, State: model.SessionState(state), CreatedAt: createdAt}, nil
}

func (store *AuthStore) RevokeSession(ctx context.Context, id model.ID, now time.Time) error {
	tag, err := store.pool.Exec(ctx,
		`UPDATE web_sessions SET state = $1, revoked_at = $2 WHERE id = $3`,
		string(model.SessionRevoked), now, id.String())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errAuthenticationRequired
	}
	return nil
}

func (store *AuthStore) IsBlocked(ctx context.Context, username, source string, now time.Time) (bool, time.Duration, error) {
	var blockedUntil *time.Time
	err := store.pool.QueryRow(ctx,
		`SELECT blocked_until FROM login_throttle_pairs WHERE username_key = $1 AND source_address = $2`,
		usernameKey(username), source).Scan(&blockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if blockedUntil == nil || !blockedUntil.After(now) {
		return false, 0, nil
	}
	return true, blockedUntil.Sub(now), nil
}

func (store *AuthStore) RecordFailure(ctx context.Context, username, source string, now time.Time) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO login_throttle_pairs (username_key, source_address, window_started_at, failure_count, blocked_until, updated_at)
		VALUES ($1, $2, $3, 1, NULL, $3)
		ON CONFLICT (username_key, source_address) DO UPDATE SET
			failure_count = CASE
				WHEN login_throttle_pairs.window_started_at <= $3 - INTERVAL '10 minutes' THEN 1
				ELSE login_throttle_pairs.failure_count + 1
			END,
			window_started_at = CASE
				WHEN login_throttle_pairs.window_started_at <= $3 - INTERVAL '10 minutes' THEN $3
				ELSE login_throttle_pairs.window_started_at
			END,
			blocked_until = CASE
				WHEN (CASE WHEN login_throttle_pairs.window_started_at <= $3 - INTERVAL '10 minutes' THEN 1 ELSE login_throttle_pairs.failure_count + 1 END) >= $4
				THEN $3 + INTERVAL '15 minutes'
				ELSE login_throttle_pairs.blocked_until
			END,
			updated_at = $3
	`, usernameKey(username), source, now, throttleFailureLimit)
	return err
}

func (store *AuthStore) ClearFailures(ctx context.Context, username, source string) error {
	_, err := store.pool.Exec(ctx,
		`DELETE FROM login_throttle_pairs WHERE username_key = $1 AND source_address = $2`,
		usernameKey(username), source)
	return err
}

func usernameKey(username string) []byte {
	sum := sha256.Sum256([]byte(username))
	return sum[:]
}
