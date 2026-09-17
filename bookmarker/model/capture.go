package model

import "time"

// CaptureState is the lifecycle state of a session-owned screenshot
// capture draft.
type CaptureState string

const (
	CaptureStatePending   CaptureState = "pending"
	CaptureStateCapturing CaptureState = "capturing"
	CaptureStateReady     CaptureState = "ready"
	CaptureStateFailed    CaptureState = "failed"
	CaptureStateConsumed  CaptureState = "consumed"
	CaptureStateDiscarded CaptureState = "discarded"
	CaptureStateExpired   CaptureState = "expired"
)

// CaptureDraft is a staged screenshot attempt that exists before a bookmark
// create or replacement is committed.
type CaptureDraft struct {
	ID             ID
	OwnerSessionID ID
	BookmarkID     *ID
	TargetURL      string
	State          CaptureState
	StagedKey      string
	FinalURL       string
	FailureCode    string
	FailureMessage string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

var ErrCaptureForbidden = &ApplicationError{Code: "forbidden", Message: "capture draft is not owned by this session"}
var ErrCaptureConflict = &ApplicationError{Code: "conflict", Message: "capture draft is stale, consumed, expired, or in an invalid state"}
var ErrCaptureNotFound = &ApplicationError{Code: "not_found", Message: "capture draft was not found"}

// EnsureUsableBy validates that a draft may be acted on by the given
// session for the given target URL at the given time: it must be owned by
// that session, match the URL exactly, and not be expired.
func (draft CaptureDraft) EnsureUsableBy(sessionID ID, targetURL string, now time.Time) error {
	if draft.OwnerSessionID != sessionID {
		return ErrCaptureForbidden
	}
	if draft.TargetURL != targetURL {
		return ErrCaptureConflict
	}
	if draft.IsExpired(now) {
		return ErrCaptureConflict
	}
	return nil
}

func (draft CaptureDraft) IsExpired(now time.Time) bool {
	return !draft.ExpiresAt.IsZero() && now.After(draft.ExpiresAt)
}

func (draft CaptureDraft) CanRetry() bool { return draft.State == CaptureStateFailed }

func (draft CaptureDraft) CanDiscard() bool {
	return draft.State == CaptureStateReady || draft.State == CaptureStateFailed
}

func (draft CaptureDraft) CanConsume() bool { return draft.State == CaptureStateReady }

func (draft CaptureDraft) IsTerminal() bool {
	switch draft.State {
	case CaptureStateConsumed, CaptureStateDiscarded, CaptureStateExpired:
		return true
	default:
		return false
	}
}
