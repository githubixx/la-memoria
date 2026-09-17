package usecase

import (
	"context"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/ports"
)

type RetentionDependencies struct {
	Store                  ports.RetentionStore
	CleanupStore           ports.ScreenshotCleanupStore
	Files                  ports.ScreenshotFileStore
	Clock                  ports.Clock
	Limit                  int
	SessionAge             time.Duration
	ThrottleAge            time.Duration
	MaximumCleanupAttempts int
}

type RetentionProcessor struct{ dependencies RetentionDependencies }

func NewRetentionProcessor(dependencies RetentionDependencies) *RetentionProcessor {
	if dependencies.Clock == nil {
		dependencies.Clock = systemClock{}
	}
	if dependencies.Limit == 0 {
		dependencies.Limit = 100
	}
	if dependencies.SessionAge == 0 {
		dependencies.SessionAge = 24 * time.Hour
	}
	if dependencies.ThrottleAge == 0 {
		dependencies.ThrottleAge = 24 * time.Hour
	}
	return &RetentionProcessor{dependencies: dependencies}
}

func (processor *RetentionProcessor) Process(ctx context.Context) error {
	if processor.dependencies.Store == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := processor.dependencies.Clock.Now().UTC()
	if err := processor.dependencies.Store.ExpireCaptureDrafts(ctx, now, processor.dependencies.Limit); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := processor.dependencies.Store.DeleteExpiredSessions(ctx, now.Add(-processor.dependencies.SessionAge), processor.dependencies.Limit); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := processor.dependencies.Store.DeleteExpiredThrottlePairs(ctx, now.Add(-processor.dependencies.ThrottleAge), processor.dependencies.Limit); err != nil {
		return err
	}
	if processor.dependencies.CleanupStore == nil || processor.dependencies.Files == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return NewCleanupProcessor(CleanupDependencies{
		Store:           processor.dependencies.CleanupStore,
		Files:           processor.dependencies.Files,
		Clock:           processor.dependencies.Clock,
		Limit:           processor.dependencies.Limit,
		MaximumAttempts: processor.dependencies.MaximumCleanupAttempts,
	}).Process(ctx)
}
