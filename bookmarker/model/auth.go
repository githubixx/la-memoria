package model

import "time"

type Administrator struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
}

type SessionState string

const (
	SessionAnonymous     SessionState = "anonymous"
	SessionAuthenticated SessionState = "authenticated"
	SessionRevoked       SessionState = "revoked"
)

type WebSession struct {
	ID              ID
	TokenDigest     []byte
	CSRFDigest      []byte
	State           SessionState
	CreatedAt       time.Time
	AuthenticatedAt *time.Time
	RevokedAt       *time.Time
}

type Principal struct {
	SessionID ID
	Username  string
}

type LoginThrottle struct {
	UsernameKey     string
	SourceAddress   string
	WindowStartedAt time.Time
	FailureCount    int
	BlockedUntil    *time.Time
	UpdatedAt       time.Time
}
