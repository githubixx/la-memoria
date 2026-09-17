package usecase

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// MaintainDependencies configures authenticated bookmark update and deletion.
type MaintainDependencies struct {
	Bookmarks ports.BookmarkMaintainer
	Drafts    ports.CaptureDraftStore
	Files     ports.ScreenshotFileStore
	Clock     ports.Clock
}

// Maintainer coordinates metadata writes with screenshot compensation.
type Maintainer struct{ dependencies MaintainDependencies }

func NewMaintainer(dependencies MaintainDependencies) *Maintainer {
	if dependencies.Clock == nil {
		dependencies.Clock = systemClock{}
	}
	return &Maintainer{dependencies: dependencies}
}

// Find loads a bookmark for an authenticated maintenance page.
func (maintainer *Maintainer) Find(ctx context.Context, id model.ID) (ports.StoredBookmark, error) {
	return maintainer.dependencies.Bookmarks.FindBookmark(ctx, id)
}

type UpdateBookmarkInput struct {
	SessionID             model.ID
	BookmarkID            model.ID
	URL                   string
	Description           string
	Tags                  []string
	CaptureID             *model.ID
	SaveWithoutScreenshot bool
}

// Update changes the mutable bookmark fields while retaining its creation
// time. A changed URL requires either a bound ready replacement or explicit
// screenshot omission.
func (maintainer *Maintainer) Update(ctx context.Context, input UpdateBookmarkInput) (model.Bookmark, error) {
	current, err := maintainer.dependencies.Bookmarks.FindBookmark(ctx, input.BookmarkID)
	if err != nil {
		return model.Bookmark{}, err
	}
	url, err := model.ValidateBookmarkURL(input.URL)
	if err != nil {
		return model.Bookmark{}, err
	}
	description, err := model.ValidateDescription(input.Description)
	if err != nil {
		return model.Bookmark{}, err
	}

	storageKey, capturedURL := current.ScreenshotStorageKey, ""
	replaceScreenshot := false
	var draft model.CaptureDraft
	if input.CaptureID != nil {
		draft, err = maintainer.dependencies.Drafts.FindDraft(ctx, *input.CaptureID)
		if err != nil {
			return model.Bookmark{}, err
		}
		if err := draft.EnsureUsableBy(input.SessionID, url, maintainer.dependencies.Clock.Now().UTC()); err != nil {
			return model.Bookmark{}, err
		}
		if draft.BookmarkID == nil || *draft.BookmarkID != input.BookmarkID || !draft.CanConsume() {
			return model.Bookmark{}, model.ErrCaptureConflict
		}
		storageKey, err = maintainer.dependencies.Files.Promote(ctx, draft.StagedKey)
		if err != nil {
			return model.Bookmark{}, err
		}
		capturedURL, replaceScreenshot = draft.FinalURL, true
	} else if url != current.URL {
		if !input.SaveWithoutScreenshot {
			return model.Bookmark{}, ErrScreenshotRequired
		}
		storageKey, replaceScreenshot = "", true
	}

	now := maintainer.dependencies.Clock.Now().UTC()
	write := ports.UpdatedBookmark{ID: input.BookmarkID, URL: url, Description: description, Tags: model.NormalizeTags(input.Tags), ScreenshotStorageKey: storageKey, CapturedURL: capturedURL, ReplaceScreenshot: replaceScreenshot, UpdatedAt: now}
	if err := maintainer.dependencies.Bookmarks.UpdateBookmark(ctx, write); err != nil {
		if replaceScreenshot && storageKey != "" {
			_ = maintainer.dependencies.Files.Remove(ctx, storageKey)
		}
		return model.Bookmark{}, err
	}
	if input.CaptureID != nil {
		draft.State = model.CaptureStateConsumed
		_ = maintainer.dependencies.Drafts.UpdateDraft(ctx, draft)
	}
	if replaceScreenshot && current.ScreenshotStorageKey != "" {
		if err := maintainer.dependencies.Files.Remove(ctx, current.ScreenshotStorageKey); err != nil {
			_ = maintainer.dependencies.Bookmarks.CreateScreenshotCleanup(ctx, ports.ScreenshotCleanup{ID: model.NewID(), QuarantinedKey: current.ScreenshotStorageKey, Operation: "remove_replaced", NextAttemptAt: now})
		}
	}
	return model.Bookmark{ID: current.ID, URL: url, Description: description, CreatedAt: current.CreatedAt, UpdatedAt: now, Tags: write.Tags}, nil
}
