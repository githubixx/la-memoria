package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestCreateRequiresAuthenticationCallerContextAndValidatesFields(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	writer := &testkit.BookmarkWriter{}
	creator := usecase.NewCreator(usecase.CreateDependencies{
		Bookmarks: writer,
		Drafts:    testkit.NewCaptureDraftStore(),
		Files:     testkit.NewScreenshotFileStore(),
		Clock:     clock,
	})

	if _, err := creator.Create(context.Background(), usecase.CreateBookmarkInput{URL: "not a url", Description: "x", SaveWithoutScreenshot: true}); model.ErrorCode(err) != "invalid_url" {
		t.Fatalf("invalid URL error = %v, want invalid_url", err)
	}
	if _, err := creator.Create(context.Background(), usecase.CreateBookmarkInput{URL: "https://example.test", Description: "  ", SaveWithoutScreenshot: true}); model.ErrorCode(err) != "description_required" {
		t.Fatalf("empty description error = %v, want description_required", err)
	}
	if _, err := creator.Create(context.Background(), usecase.CreateBookmarkInput{URL: "https://example.test", Description: "valid"}); model.ErrorCode(err) != "screenshot_required" {
		t.Fatalf("missing capture without explicit omission = %v, want screenshot_required", err)
	}
}

func TestCreateConsumesAReadyDraftAndPromotesItsStagedScreenshot(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	writer := &testkit.BookmarkWriter{}
	files := testkit.NewScreenshotFileStore()
	drafts := testkit.NewCaptureDraftStore()
	creator := usecase.NewCreator(usecase.CreateDependencies{Bookmarks: writer, Drafts: drafts, Files: files, Clock: clock})

	stagedPath := files.StagingPath("session-1")
	draftID := model.NewID()
	_ = drafts.CreateDraft(context.Background(), model.CaptureDraft{
		ID: draftID, OwnerSessionID: "session-1", TargetURL: "https://example.test/page",
		State: model.CaptureStateReady, StagedKey: stagedPath, FinalURL: "https://example.test/page",
		ExpiresAt: clock.Now().Add(time.Hour),
	})

	id, err := creator.Create(context.Background(), usecase.CreateBookmarkInput{
		SessionID: "session-1", URL: "https://example.test/page", Description: "valid", Tags: []string{"Go", "go"}, CaptureID: &draftID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" {
		t.Fatal("create must return a new bookmark id")
	}
	if len(writer.Created) != 1 || writer.Created[0].ScreenshotStorageKey == "" {
		t.Fatalf("created bookmarks = %#v, want one bookmark with a promoted screenshot", writer.Created)
	}
	if len(writer.Created[0].Tags) != 1 {
		t.Fatalf("created tags = %#v, want normalized duplicates collapsed", writer.Created[0].Tags)
	}
	consumedDraft, err := drafts.FindDraft(context.Background(), draftID)
	if err != nil || consumedDraft.State != model.CaptureStateConsumed {
		t.Fatalf("draft after create = %#v (err=%v), want consumed", consumedDraft, err)
	}
	if !files.Promoted[writer.Created[0].ScreenshotStorageKey] {
		t.Fatal("promoted screenshot must be recorded in the file store")
	}
}

func TestCreateCompensatesAFailedWriteByRemovingThePromotedScreenshot(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	writer := &testkit.BookmarkWriter{Err: &model.ApplicationError{Code: "internal_error", Message: "write failed"}}
	files := testkit.NewScreenshotFileStore()
	drafts := testkit.NewCaptureDraftStore()
	creator := usecase.NewCreator(usecase.CreateDependencies{Bookmarks: writer, Drafts: drafts, Files: files, Clock: clock})

	stagedPath := files.StagingPath("session-1")
	draftID := model.NewID()
	_ = drafts.CreateDraft(context.Background(), model.CaptureDraft{
		ID: draftID, OwnerSessionID: "session-1", TargetURL: "https://example.test/page",
		State: model.CaptureStateReady, StagedKey: stagedPath, ExpiresAt: clock.Now().Add(time.Hour),
	})

	_, err := creator.Create(context.Background(), usecase.CreateBookmarkInput{
		SessionID: "session-1", URL: "https://example.test/page", Description: "valid", CaptureID: &draftID,
	})
	if model.ErrorCode(err) != "internal_error" {
		t.Fatalf("create error = %v, want internal_error propagated", err)
	}
	if len(files.Removed) != 1 {
		t.Fatalf("removed files = %#v, want the orphaned promoted screenshot removed", files.Removed)
	}
}

func TestCreateAllowsExplicitOmissionAfterFailedCapture(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	writer := &testkit.BookmarkWriter{}
	files := testkit.NewScreenshotFileStore()
	drafts := testkit.NewCaptureDraftStore()
	creator := usecase.NewCreator(usecase.CreateDependencies{Bookmarks: writer, Drafts: drafts, Files: files, Clock: clock})

	draftID := model.NewID()
	_ = drafts.CreateDraft(context.Background(), model.CaptureDraft{
		ID: draftID, OwnerSessionID: "session-1", TargetURL: "https://example.test/page",
		State: model.CaptureStateFailed, ExpiresAt: clock.Now().Add(time.Hour),
	})

	id, err := creator.Create(context.Background(), usecase.CreateBookmarkInput{
		SessionID: "session-1", URL: "https://example.test/page", Description: "valid", CaptureID: &draftID, SaveWithoutScreenshot: true,
	})
	if err != nil {
		t.Fatalf("create without screenshot: %v", err)
	}
	if len(writer.Created) != 1 || writer.Created[0].ScreenshotStorageKey != "" {
		t.Fatalf("created bookmarks = %#v, want one bookmark with no screenshot", writer.Created)
	}
	_ = id
}
