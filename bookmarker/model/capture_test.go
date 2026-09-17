package model_test

import (
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

func TestValidateBookmarkURLRejectsNonHTTPSchemesForCaptureTargets(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.test/page", false},
		{"http://127.0.0.1:9000/private", false},
		{"file:///etc/passwd", true},
		{"javascript:alert(1)", true},
	}
	for _, test := range tests {
		if _, err := model.ValidateBookmarkURL(test.url); (err != nil) != test.wantErr {
			t.Fatalf("ValidateBookmarkURL(%q) error = %v, wantErr %t", test.url, err, test.wantErr)
		}
	}
}

func TestCaptureDraftEnsureUsableByChecksOwnershipURLAndExpiry(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	draft := model.CaptureDraft{
		OwnerSessionID: "session-1",
		TargetURL:      "https://example.test/reference",
		ExpiresAt:      now.Add(time.Minute),
	}

	if err := draft.EnsureUsableBy("session-2", draft.TargetURL, now); model.ErrorCode(err) != "forbidden" {
		t.Fatalf("wrong-owner error = %v, want forbidden", err)
	}
	if err := draft.EnsureUsableBy("session-1", "https://example.test/other", now); model.ErrorCode(err) != "conflict" {
		t.Fatalf("URL-mismatch error = %v, want conflict", err)
	}
	if err := draft.EnsureUsableBy("session-1", draft.TargetURL, now.Add(2*time.Minute)); model.ErrorCode(err) != "conflict" {
		t.Fatalf("expired-draft error = %v, want conflict", err)
	}
	if err := draft.EnsureUsableBy("session-1", draft.TargetURL, now); err != nil {
		t.Fatalf("valid usage = %v, want nil", err)
	}
}

func TestCaptureDraftStateTransitionsAllowRetryDiscardAndSingleConsumption(t *testing.T) {
	failed := model.CaptureDraft{State: model.CaptureStateFailed}
	if !failed.CanRetry() || !failed.CanDiscard() {
		t.Fatalf("failed draft = %#v, want retryable and discardable", failed)
	}
	if failed.CanConsume() {
		t.Fatal("failed draft must not be directly consumable")
	}

	ready := model.CaptureDraft{State: model.CaptureStateReady}
	if !ready.CanConsume() || !ready.CanDiscard() {
		t.Fatalf("ready draft = %#v, want consumable and discardable", ready)
	}
	if ready.CanRetry() {
		t.Fatal("ready draft must not be retryable")
	}

	for _, terminalState := range []model.CaptureState{model.CaptureStateConsumed, model.CaptureStateDiscarded, model.CaptureStateExpired} {
		draft := model.CaptureDraft{State: terminalState}
		if !draft.IsTerminal() {
			t.Fatalf("state %q must be terminal", terminalState)
		}
		if draft.CanConsume() || draft.CanRetry() || draft.CanDiscard() {
			t.Fatalf("terminal state %q must not allow further transitions: %#v", terminalState, draft)
		}
	}
}
