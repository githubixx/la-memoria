package usecase

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

var ErrConfirmationRequired = &model.ValidationError{Code: "confirmation_required", Field: "confirm", Message: "deletion requires explicit confirmation"}

type DeleteBookmarkInput struct {
	BookmarkID model.ID
	Confirm    bool
}

type DeleteBookmarkResult struct {
	ID             model.ID
	CleanupPending bool
}

// Delete hides the screenshot before the database transition, restores it on
// rollback, and leaves an observable retry record after an unlink failure.
func (maintainer *Maintainer) Delete(ctx context.Context, input DeleteBookmarkInput) (DeleteBookmarkResult, error) {
	if !input.Confirm {
		return DeleteBookmarkResult{}, ErrConfirmationRequired
	}
	current, err := maintainer.dependencies.Bookmarks.FindBookmark(ctx, input.BookmarkID)
	if err != nil {
		return DeleteBookmarkResult{}, err
	}
	quarantinedKey := ""
	if current.ScreenshotStorageKey != "" {
		quarantinedKey, err = maintainer.dependencies.Files.Quarantine(ctx, current.ScreenshotStorageKey)
		if err != nil {
			return DeleteBookmarkResult{}, err
		}
	}
	if err := maintainer.dependencies.Bookmarks.DeleteBookmark(ctx, input.BookmarkID); err != nil {
		if quarantinedKey != "" {
			_ = maintainer.dependencies.Files.Restore(ctx, quarantinedKey, current.ScreenshotStorageKey)
		}
		return DeleteBookmarkResult{}, err
	}
	result := DeleteBookmarkResult{ID: input.BookmarkID}
	if quarantinedKey != "" {
		if err := maintainer.dependencies.Files.Remove(ctx, quarantinedKey); err != nil {
			result.CleanupPending = true
			_ = maintainer.dependencies.Bookmarks.CreateScreenshotCleanup(ctx, ports.ScreenshotCleanup{ID: model.NewID(), QuarantinedKey: quarantinedKey, Operation: "remove_deleted", NextAttemptAt: maintainer.dependencies.Clock.Now().UTC()})
		}
	}
	return result, nil
}
