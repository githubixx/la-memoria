package ports

import (
	"context"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID() string }
type PasswordVerifier interface {
	Verify(context.Context, string, string) (bool, error)
}
type SessionStore interface {
	CreateSession(ctx context.Context, tokenDigest, csrfDigest []byte, state model.SessionState, now time.Time) (model.WebSession, error)
	FindActiveSession(context.Context, []byte) (model.WebSession, error)
	RevokeSession(context.Context, model.ID, time.Time) error
}
type LoginThrottleStore interface {
	IsBlocked(context.Context, string, string, time.Time) (bool, time.Duration, error)
	RecordFailure(context.Context, string, string, time.Time) error
	ClearFailures(context.Context, string, string) error
}
