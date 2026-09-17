package usecase

import (
	"context"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/ports"
)

type CleanupDependencies struct {
	Store           ports.ScreenshotCleanupStore
	Files           ports.ScreenshotFileStore
	Clock           ports.Clock
	Limit           int
	MaximumAttempts int
}

// CleanupProcessor retries post-commit screenshot removal without changing
// the completed bookmark transaction.
type CleanupProcessor struct{ dependencies CleanupDependencies }

func NewCleanupProcessor(dependencies CleanupDependencies) *CleanupProcessor {
	if dependencies.Clock == nil {
		dependencies.Clock = systemClock{}
	}
	if dependencies.Limit == 0 {
		dependencies.Limit = 25
	}
	if dependencies.MaximumAttempts == 0 {
		dependencies.MaximumAttempts = 3
	}
	return &CleanupProcessor{dependencies: dependencies}
}

func (processor *CleanupProcessor) Process(ctx context.Context) error {
	now := processor.dependencies.Clock.Now().UTC()
	records, err := processor.dependencies.Store.ClaimDueScreenshotCleanup(ctx, now, processor.dependencies.Limit)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := processor.dependencies.Files.Remove(ctx, record.QuarantinedKey); err == nil {
			record.State, record.LastErrorCode = "succeeded", ""
		} else {
			record.AttemptCount++
			record.LastErrorCode = "cleanup_failed"
			if record.AttemptCount >= processor.dependencies.MaximumAttempts {
				record.State = "abandoned"
			} else {
				record.State = "pending"
				record.NextAttemptAt = now.Add(time.Duration(record.AttemptCount) * time.Minute)
			}
		}
		if err := processor.dependencies.Store.CompleteScreenshotCleanup(ctx, record); err != nil {
			return err
		}
	}
	return nil
}
