package ports

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

// CaptureDraftStore persists capture draft state across the request that
// starts a capture and later requests that inspect, retry, discard, or
// consume it.
type CaptureDraftStore interface {
	CreateDraft(ctx context.Context, draft model.CaptureDraft) error
	FindDraft(ctx context.Context, id model.ID) (model.CaptureDraft, error)
	UpdateDraft(ctx context.Context, draft model.CaptureDraft) error
}
