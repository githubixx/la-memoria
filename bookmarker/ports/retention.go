package ports

import (
	"context"
	"time"
)

type RetentionStore interface {
	ExpireCaptureDrafts(context.Context, time.Time, int) error
	DeleteExpiredSessions(context.Context, time.Time, int) error
	DeleteExpiredThrottlePairs(context.Context, time.Time, int) error
}
