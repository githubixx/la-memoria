package usecase

import (
	"context"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// CaptureDependencies configures the screenshot capture-draft use cases.
type CaptureDependencies struct {
	Capturer ports.ScreenshotCapturer
	Files    ports.ScreenshotFileStore
	Drafts   ports.CaptureDraftStore
	Clock    ports.Clock
	IDs      ports.IDGenerator
	Lifetime time.Duration
}

// CaptureService starts, inspects, retries, and discards session-owned
// screenshot capture drafts.
type CaptureService struct{ dependencies CaptureDependencies }

func NewCaptureService(dependencies CaptureDependencies) *CaptureService {
	if dependencies.Clock == nil {
		dependencies.Clock = systemClock{}
	}
	if dependencies.IDs == nil {
		dependencies.IDs = idGeneratorFunc(func() string { return model.NewID().String() })
	}
	if dependencies.Lifetime == 0 {
		dependencies.Lifetime = 15 * time.Minute
	}
	return &CaptureService{dependencies: dependencies}
}

type StartCaptureInput struct {
	SessionID  model.ID
	TargetURL  string
	BookmarkID *model.ID
}

type idGeneratorFunc func() string

func (fn idGeneratorFunc) NewID() string { return fn() }

// Start validates the target URL, creates a pending draft, and runs the
// capture attempt to completion before returning the resulting status.
func (service *CaptureService) Start(ctx context.Context, input StartCaptureInput) (model.CaptureDraft, error) {
	targetURL, err := model.ValidateBookmarkURL(input.TargetURL)
	if err != nil {
		return model.CaptureDraft{}, err
	}

	now := service.dependencies.Clock.Now().UTC()
	draft := model.CaptureDraft{
		ID:             model.ID(service.dependencies.IDs.NewID()),
		OwnerSessionID: input.SessionID,
		BookmarkID:     input.BookmarkID,
		TargetURL:      targetURL,
		State:          model.CaptureStateCapturing,
		CreatedAt:      now,
		ExpiresAt:      now.Add(service.dependencies.Lifetime),
	}
	if err := service.dependencies.Drafts.CreateDraft(ctx, draft); err != nil {
		return model.CaptureDraft{}, err
	}
	service.run(ctx, draft)
	return service.dependencies.Drafts.FindDraft(ctx, draft.ID)
}

func (service *CaptureService) run(ctx context.Context, draft model.CaptureDraft) {
	stagedPath := service.dependencies.Files.StagingPath(draft.OwnerSessionID.String())
	result, err := service.dependencies.Capturer.Capture(ctx, draft.ID.String(), draft.TargetURL, stagedPath)
	if err != nil {
		draft.State = model.CaptureStateFailed
		draft.FailureCode = model.ErrorCode(err)
		draft.FailureMessage = "the destination could not be captured"
		_ = service.dependencies.Drafts.UpdateDraft(ctx, draft)
		return
	}
	if err := service.dependencies.Files.Validate(ctx, result.StagedKey); err != nil {
		draft.State = model.CaptureStateFailed
		draft.FailureCode = "invalid_output"
		draft.FailureMessage = "the captured file was not a valid image"
		_ = service.dependencies.Drafts.UpdateDraft(ctx, draft)
		return
	}
	draft.State = model.CaptureStateReady
	draft.StagedKey = result.StagedKey
	draft.FinalURL = result.FinalURL
	_ = service.dependencies.Drafts.UpdateDraft(ctx, draft)
}

// Status returns the current draft, enforcing that only its owning
// session may observe it.
func (service *CaptureService) Status(ctx context.Context, sessionID, id model.ID) (model.CaptureDraft, error) {
	draft, err := service.dependencies.Drafts.FindDraft(ctx, id)
	if err != nil {
		return model.CaptureDraft{}, err
	}
	if draft.OwnerSessionID != sessionID {
		return model.CaptureDraft{}, model.ErrCaptureForbidden
	}
	return draft, nil
}

// Preview returns the staged screenshot bytes for a ready capture draft.
func (service *CaptureService) Preview(ctx context.Context, draft model.CaptureDraft) ([]byte, error) {
	if draft.State != model.CaptureStateReady || draft.StagedKey == "" {
		return nil, model.ErrCaptureNotFound
	}
	return service.dependencies.Files.Read(ctx, draft.StagedKey)
}

// Retry re-runs a failed, unexpired draft owned by the caller.
func (service *CaptureService) Retry(ctx context.Context, sessionID, id model.ID) (model.CaptureDraft, error) {
	draft, err := service.Status(ctx, sessionID, id)
	if err != nil {
		return model.CaptureDraft{}, err
	}
	now := service.dependencies.Clock.Now().UTC()
	if !draft.CanRetry() || draft.IsExpired(now) {
		return model.CaptureDraft{}, model.ErrCaptureConflict
	}
	draft.State = model.CaptureStateCapturing
	draft.FailureCode = ""
	draft.FailureMessage = ""
	if err := service.dependencies.Drafts.UpdateDraft(ctx, draft); err != nil {
		return model.CaptureDraft{}, err
	}
	service.run(ctx, draft)
	return service.dependencies.Drafts.FindDraft(ctx, draft.ID)
}

// Discard marks a ready or failed draft as discarded, removing any staged
// file, so the administrator may explicitly continue without a screenshot.
func (service *CaptureService) Discard(ctx context.Context, sessionID, id model.ID) (model.CaptureDraft, error) {
	draft, err := service.Status(ctx, sessionID, id)
	if err != nil {
		return model.CaptureDraft{}, err
	}
	if !draft.CanDiscard() {
		return model.CaptureDraft{}, model.ErrCaptureConflict
	}
	if draft.StagedKey != "" {
		_ = service.dependencies.Files.Discard(ctx, draft.StagedKey)
	}
	draft.State = model.CaptureStateDiscarded
	if err := service.dependencies.Drafts.UpdateDraft(ctx, draft); err != nil {
		return model.CaptureDraft{}, err
	}
	return draft, nil
}
