package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func newCaptureService(clock *testkit.Clock, capturer *testkit.ScreenshotCapturer, files *testkit.ScreenshotFileStore) (*usecase.CaptureService, *testkit.CaptureDraftStore) {
	drafts := testkit.NewCaptureDraftStore()
	service := usecase.NewCaptureService(usecase.CaptureDependencies{
		Capturer: capturer,
		Files:    files,
		Drafts:   drafts,
		Clock:    clock,
	})
	return service, drafts
}

func TestCaptureStartRunsToReadyForPublicAndPrivateTargets(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	for _, url := range []string{"https://example.test/page", "http://127.0.0.1:9000/private"} {
		capturer := &testkit.ScreenshotCapturer{}
		files := testkit.NewScreenshotFileStore()
		service, _ := newCaptureService(clock, capturer, files)

		draft, err := service.Start(context.Background(), usecase.StartCaptureInput{SessionID: "session-1", TargetURL: url})
		if err != nil {
			t.Fatalf("start capture for %q: %v", url, err)
		}
		if draft.State != model.CaptureStateReady {
			t.Fatalf("draft state for %q = %q, want ready", url, draft.State)
		}
	}
}

func TestCaptureStartMarksFailedAndAllowsRetry(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	capturer := &testkit.ScreenshotCapturer{Err: &model.ApplicationError{Code: "capture_failed", Message: "failed"}}
	files := testkit.NewScreenshotFileStore()
	service, _ := newCaptureService(clock, capturer, files)

	draft, err := service.Start(context.Background(), usecase.StartCaptureInput{SessionID: "session-1", TargetURL: "https://example.test/page"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if draft.State != model.CaptureStateFailed || draft.FailureCode != "capture_failed" {
		t.Fatalf("failed draft = %#v, want state failed with capture_failed code", draft)
	}

	capturer.Err = nil
	retried, err := service.Retry(context.Background(), "session-1", draft.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.State != model.CaptureStateReady {
		t.Fatalf("retried draft = %#v, want ready", retried)
	}
	if capturer.Calls != 2 {
		t.Fatalf("capture calls = %d, want exactly 2 (initial + retry)", capturer.Calls)
	}
}

func TestCaptureDiscardAllowsExplicitContinuationWithoutScreenshot(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	capturer := &testkit.ScreenshotCapturer{}
	files := testkit.NewScreenshotFileStore()
	service, _ := newCaptureService(clock, capturer, files)

	draft, err := service.Start(context.Background(), usecase.StartCaptureInput{SessionID: "session-1", TargetURL: "https://example.test/page"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	discarded, err := service.Discard(context.Background(), "session-1", draft.ID)
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if discarded.State != model.CaptureStateDiscarded {
		t.Fatalf("discarded draft = %#v, want discarded", discarded)
	}
	if !files.Discarded[draft.StagedKey] {
		t.Fatal("discard must remove the staged file")
	}

	if _, err := service.Discard(context.Background(), "session-1", draft.ID); model.ErrorCode(err) != "conflict" {
		t.Fatalf("discarding an already-terminal draft = %v, want conflict", err)
	}
}

func TestCaptureStatusRejectsAccessFromAnotherSession(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	capturer := &testkit.ScreenshotCapturer{}
	files := testkit.NewScreenshotFileStore()
	service, _ := newCaptureService(clock, capturer, files)

	draft, err := service.Start(context.Background(), usecase.StartCaptureInput{SessionID: "session-1", TargetURL: "https://example.test/page"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := service.Status(context.Background(), "session-2", draft.ID); model.ErrorCode(err) != "forbidden" {
		t.Fatalf("cross-session status = %v, want forbidden", err)
	}
}

func TestCapturePreviewReadsReadyDraftScreenshot(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	capturer := &testkit.ScreenshotCapturer{}
	files := testkit.NewScreenshotFileStore()
	service, _ := newCaptureService(clock, capturer, files)

	draft, err := service.Start(context.Background(), usecase.StartCaptureInput{SessionID: "session-1", TargetURL: "https://example.test/page"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	image, err := service.Preview(context.Background(), draft)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(image) == 0 {
		t.Fatal("preview image must not be empty")
	}

	draft.State = model.CaptureStateFailed
	if _, err := service.Preview(context.Background(), draft); model.ErrorCode(err) != "conflict" {
		t.Fatalf("previewing failed draft = %v, want conflict", err)
	}
}
