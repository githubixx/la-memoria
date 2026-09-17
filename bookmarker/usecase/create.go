package usecase

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

var ErrScreenshotRequired = &model.ValidationError{Code: "screenshot_required", Field: "capture_id", Message: "capture a screenshot or explicitly continue without one"}

// CreateDependencies configures the authenticated bookmark creation use
// case.
type CreateDependencies struct {
	Bookmarks ports.BookmarkWriter
	Drafts    ports.CaptureDraftStore
	Files     ports.ScreenshotFileStore
	Clock     ports.Clock
}

// Creator implements authenticated bookmark creation with draft
// validation, tag normalization, explicit-omission handling, staged
// promotion, and failed-promotion compensation.
type Creator struct{ dependencies CreateDependencies }

func NewCreator(dependencies CreateDependencies) *Creator {
	if dependencies.Clock == nil {
		dependencies.Clock = systemClock{}
	}
	return &Creator{dependencies: dependencies}
}

type CreateBookmarkInput struct {
	SessionID             model.ID
	URL                   string
	Description           string
	Tags                  []string
	CaptureID             *model.ID
	SaveWithoutScreenshot bool
}

func (creator *Creator) Create(ctx context.Context, input CreateBookmarkInput) (model.ID, error) {
	url, err := model.ValidateBookmarkURL(input.URL)
	if err != nil {
		return "", err
	}
	description, err := model.ValidateDescription(input.Description)
	if err != nil {
		return "", err
	}
	tags := model.NormalizeTags(input.Tags)

	storageKey, capturedURL, err := creator.resolveScreenshot(ctx, input, url)
	if err != nil {
		return "", err
	}

	id := model.NewID()
	writeErr := creator.dependencies.Bookmarks.CreateBookmark(ctx, ports.NewBookmark{
		ID:                   id,
		URL:                  url,
		Description:          description,
		Tags:                 tags,
		ScreenshotStorageKey: storageKey,
		CapturedURL:          capturedURL,
		Now:                  creator.dependencies.Clock.Now().UTC(),
	})
	if writeErr != nil {
		if storageKey != "" {
			// Compensate a failed reference: the promoted file has no
			// bookmark, so it must not remain user-visible.
			_ = creator.dependencies.Files.Remove(ctx, storageKey)
		}
		return "", writeErr
	}
	return id, nil
}

// resolveScreenshot consumes a ready owned draft matching the given URL,
// promoting its staged file, or requires explicit confirmation to
// continue without one.
func (creator *Creator) resolveScreenshot(ctx context.Context, input CreateBookmarkInput, url string) (storageKey, capturedURL string, err error) {
	if input.CaptureID == nil {
		if !input.SaveWithoutScreenshot {
			return "", "", ErrScreenshotRequired
		}
		return "", "", nil
	}

	draft, err := creator.dependencies.Drafts.FindDraft(ctx, *input.CaptureID)
	if err != nil {
		return "", "", err
	}
	now := creator.dependencies.Clock.Now().UTC()
	if err := draft.EnsureUsableBy(input.SessionID, url, now); err != nil {
		return "", "", err
	}
	if !draft.CanConsume() {
		if !input.SaveWithoutScreenshot {
			return "", "", ErrScreenshotRequired
		}
		return "", "", nil
	}

	storageKey, err = creator.dependencies.Files.Promote(ctx, draft.StagedKey)
	if err != nil {
		return "", "", err
	}
	draft.State = model.CaptureStateConsumed
	_ = creator.dependencies.Drafts.UpdateDraft(ctx, draft)
	return storageKey, draft.FinalURL, nil
}
