package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestAuthenticateRotatesSessionAndRevokesPriorSession(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	service := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock:            clock,
		IDs:              testkit.NewIDGenerator("session"),
		Passwords:        &testkit.PasswordVerifier{ExpectedHash: "$argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA", ExpectedPassword: "correct horse"},
		Sessions:         usecase.NewMemorySessionStore(),
		LoginThrottles:   usecase.NewMemoryLoginThrottleStore(),
		Administrator:    model.Administrator{Username: "admin", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA"},
		SessionLifetime:  0,
		ThrottleWindow:   10 * time.Minute,
		ThrottleFailures: 5,
		BlockDuration:    15 * time.Minute,
	})

	anonymous, err := service.NewAnonymousSession(context.Background())
	if err != nil {
		t.Fatalf("create anonymous session: %v", err)
	}

	authenticated, err := service.SignIn(context.Background(), usecase.SignInInput{
		SessionToken: anonymous.Token,
		Username:     "admin",
		Password:     "correct horse",
		Source:       "192.0.2.10",
	})
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if authenticated.Token == anonymous.Token || authenticated.CSRFToken == anonymous.CSRFToken {
		t.Fatal("successful sign-in must rotate both session and CSRF tokens")
	}
	if _, err := service.Authorize(context.Background(), anonymous.Token); err == nil {
		t.Fatal("pre-authentication session must be revoked after rotation")
	}

	if err := service.SignOut(context.Background(), authenticated.Token); err != nil {
		t.Fatalf("sign out: %v", err)
	}
	if _, err := service.Authorize(context.Background(), authenticated.Token); err == nil {
		t.Fatal("signed-out session must not authorize")
	}
}

func TestAuthenticateExpiresWhenBrowserSessionEndsAndReturnsGenericFailures(t *testing.T) {
	service := usecase.NewAuthenticator(usecase.AuthenticationDependencies{})

	_, err := service.SignIn(context.Background(), usecase.SignInInput{Username: "unknown", Password: "wrong", Source: "192.0.2.10"})
	if !usecase.IsAuthenticationFailure(err) {
		t.Fatalf("invalid credentials error = %v, want generic authentication failure", err)
	}
	if usecase.ErrorMessage(err) != "invalid username or password" {
		t.Fatalf("failure message = %q, must not distinguish username, password, or throttle state", usecase.ErrorMessage(err))
	}
	if usecase.SessionCookieMaxAge() != 0 {
		t.Fatalf("browser-session cookie Max-Age = %d, want 0", usecase.SessionCookieMaxAge())
	}
}
