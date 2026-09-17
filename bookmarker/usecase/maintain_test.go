package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestUpdateConsumesOnlyOwnedBookmarkBoundReplacementAndPreservesCreationTime(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	bookmarkID := model.ID("bookmark-1")
	createdAt := clock.Now().Add(-24 * time.Hour)
	replacementID := model.ID("capture-1")
	store := &maintenanceStore{bookmark: ports.StoredBookmark{
		Bookmark:             model.Bookmark{ID: bookmarkID, URL: "https://old.example.test", Description: "old", CreatedAt: createdAt, UpdatedAt: createdAt},
		ScreenshotStorageKey: "promoted/old.png",
	}}
	files := newMaintenanceFiles()
	drafts := testkit.NewCaptureDraftStore()
	_ = drafts.CreateDraft(context.Background(), model.CaptureDraft{
		ID: replacementID, OwnerSessionID: "session-1", BookmarkID: &bookmarkID,
		TargetURL: "https://new.example.test", State: model.CaptureStateReady,
		StagedKey: files.StagingPath("session-1"), FinalURL: "https://new.example.test",
		ExpiresAt: clock.Now().Add(time.Hour),
	})
	maintainer := usecase.NewMaintainer(usecase.MaintainDependencies{Bookmarks: store, Drafts: drafts, Files: files, Clock: clock})

	updated, err := maintainer.Update(context.Background(), usecase.UpdateBookmarkInput{
		SessionID: "session-1", BookmarkID: bookmarkID, URL: "https://new.example.test", Description: "new", Tags: []string{"Go", "go"}, CaptureID: &replacementID,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CreatedAt != createdAt {
		t.Fatalf("created at = %s, want original %s", updated.CreatedAt, createdAt)
	}
	if store.updated.ScreenshotStorageKey == "" || store.updated.ScreenshotStorageKey == "promoted/old.png" {
		t.Fatalf("replacement storage key = %q, want newly promoted key", store.updated.ScreenshotStorageKey)
	}
	if !files.removed["promoted/old.png"] {
		t.Fatal("old screenshot must be removed only after the replacement is durable")
	}
	draft, err := drafts.FindDraft(context.Background(), replacementID)
	if err != nil || draft.State != model.CaptureStateConsumed {
		t.Fatalf("replacement draft = %#v (err=%v), want consumed", draft, err)
	}
}

func TestUpdateRejectsAReplacementDraftOwnedByAnotherSessionOrBookmark(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	bookmarkID := model.ID("bookmark-1")
	otherBookmarkID := model.ID("bookmark-2")
	replacementID := model.ID("capture-1")
	store := &maintenanceStore{bookmark: ports.StoredBookmark{Bookmark: model.Bookmark{ID: bookmarkID, URL: "https://old.example.test", Description: "old", CreatedAt: clock.Now()}}}
	files := newMaintenanceFiles()
	drafts := testkit.NewCaptureDraftStore()
	_ = drafts.CreateDraft(context.Background(), model.CaptureDraft{
		ID: replacementID, OwnerSessionID: "other-session", BookmarkID: &otherBookmarkID,
		TargetURL: "https://new.example.test", State: model.CaptureStateReady,
		StagedKey: files.StagingPath("other-session"), ExpiresAt: clock.Now().Add(time.Hour),
	})
	maintainer := usecase.NewMaintainer(usecase.MaintainDependencies{Bookmarks: store, Drafts: drafts, Files: files, Clock: clock})

	_, err := maintainer.Update(context.Background(), usecase.UpdateBookmarkInput{
		SessionID: "session-1", BookmarkID: bookmarkID, URL: "https://new.example.test", Description: "new", CaptureID: &replacementID,
	})
	if model.ErrorCode(err) != "forbidden" {
		t.Fatalf("update error = %v, want forbidden", err)
	}
	if store.updateCalls != 0 || len(files.promoted) != 0 {
		t.Fatalf("unauthorized draft mutated state: updates=%d promoted=%#v", store.updateCalls, files.promoted)
	}
}

func TestUpdateCompensatesNewFileWhenDatabaseSwapFailsAndRetainsOldFile(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	bookmarkID := model.ID("bookmark-1")
	replacementID := model.ID("capture-1")
	store := &maintenanceStore{
		bookmark:  ports.StoredBookmark{Bookmark: model.Bookmark{ID: bookmarkID, URL: "https://old.example.test", Description: "old", CreatedAt: clock.Now()}, ScreenshotStorageKey: "promoted/old.png"},
		updateErr: errors.New("database unavailable"),
	}
	files := newMaintenanceFiles()
	drafts := testkit.NewCaptureDraftStore()
	_ = drafts.CreateDraft(context.Background(), model.CaptureDraft{
		ID: replacementID, OwnerSessionID: "session-1", BookmarkID: &bookmarkID,
		TargetURL: "https://new.example.test", State: model.CaptureStateReady,
		StagedKey: files.StagingPath("session-1"), ExpiresAt: clock.Now().Add(time.Hour),
	})
	maintainer := usecase.NewMaintainer(usecase.MaintainDependencies{Bookmarks: store, Drafts: drafts, Files: files, Clock: clock})

	_, err := maintainer.Update(context.Background(), usecase.UpdateBookmarkInput{
		SessionID: "session-1", BookmarkID: bookmarkID, URL: "https://new.example.test", Description: "new", CaptureID: &replacementID,
	})
	if err == nil {
		t.Fatal("update with failed database swap must fail")
	}
	if len(files.removed) != 1 {
		t.Fatalf("removed files = %#v, want the newly promoted orphan removed", files.removed)
	}
	if files.removed["promoted/old.png"] {
		t.Fatal("old screenshot must remain available when the database swap fails")
	}
}

func TestDeleteRequiresConfirmationAndQueuesCleanupAfterCommittedDeletion(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	bookmarkID := model.ID("bookmark-1")
	store := &maintenanceStore{bookmark: ports.StoredBookmark{
		Bookmark:             model.Bookmark{ID: bookmarkID, URL: "https://example.test", Description: "example", CreatedAt: clock.Now()},
		ScreenshotStorageKey: "promoted/current.png",
	}}
	files := newMaintenanceFiles()
	files.removeErr = errors.New("device temporarily unavailable")
	maintainer := usecase.NewMaintainer(usecase.MaintainDependencies{Bookmarks: store, Files: files, Clock: clock})

	_, err := maintainer.Delete(context.Background(), usecase.DeleteBookmarkInput{BookmarkID: bookmarkID})
	if model.ErrorCode(err) != "confirmation_required" {
		t.Fatalf("unconfirmed deletion error = %v, want confirmation_required", err)
	}
	if store.deleteCalls != 0 || len(files.quarantined) != 0 {
		t.Fatal("unconfirmed deletion must not mutate bookmark or screenshot state")
	}

	result, err := maintainer.Delete(context.Background(), usecase.DeleteBookmarkInput{BookmarkID: bookmarkID, Confirm: true})
	if err != nil {
		t.Fatalf("confirmed deletion: %v", err)
	}
	if !result.CleanupPending || store.deleteCalls != 1 {
		t.Fatalf("delete result = %#v, delete calls = %d; want committed deletion with pending cleanup", result, store.deleteCalls)
	}
	if len(store.cleanupRecords) != 1 {
		t.Fatalf("cleanup records = %#v, want one durable retry record", store.cleanupRecords)
	}
}

type maintenanceStore struct {
	bookmark       ports.StoredBookmark
	updated        ports.UpdatedBookmark
	updateErr      error
	updateCalls    int
	deleteCalls    int
	cleanupRecords []ports.ScreenshotCleanup
}

func (store *maintenanceStore) FindBookmark(_ context.Context, id model.ID) (ports.StoredBookmark, error) {
	if store.bookmark.ID != id {
		return ports.StoredBookmark{}, model.ErrCaptureNotFound
	}
	return store.bookmark, nil
}

func (store *maintenanceStore) UpdateBookmark(_ context.Context, bookmark ports.UpdatedBookmark) error {
	store.updateCalls++
	if store.updateErr != nil {
		return store.updateErr
	}
	store.updated = bookmark
	return nil
}

func (store *maintenanceStore) DeleteBookmark(_ context.Context, id model.ID) error {
	if store.bookmark.ID != id {
		return model.ErrCaptureNotFound
	}
	store.deleteCalls++
	return nil
}

func (store *maintenanceStore) CreateScreenshotCleanup(_ context.Context, cleanup ports.ScreenshotCleanup) error {
	store.cleanupRecords = append(store.cleanupRecords, cleanup)
	return nil
}

type maintenanceFiles struct {
	*testkit.ScreenshotFileStore
	promoted    map[string]bool
	removed     map[string]bool
	quarantined map[string]bool
	removeErr   error
	next        int
}

func newMaintenanceFiles() *maintenanceFiles {
	return &maintenanceFiles{ScreenshotFileStore: testkit.NewScreenshotFileStore(), promoted: map[string]bool{}, removed: map[string]bool{}, quarantined: map[string]bool{}}
}

func (files *maintenanceFiles) Promote(_ context.Context, stagedPath string) (string, error) {
	files.next++
	key := "promoted/replacement-" + time.Now().UTC().Format("150405") + ".png"
	files.promoted[key] = true
	return key, nil
}

func (files *maintenanceFiles) Remove(_ context.Context, storageKey string) error {
	files.removed[storageKey] = true
	if files.removeErr != nil {
		return files.removeErr
	}
	return nil
}

func (files *maintenanceFiles) Quarantine(_ context.Context, storageKey string) (string, error) {
	quarantinedKey := "quarantine/" + storageKey
	files.quarantined[quarantinedKey] = true
	return quarantinedKey, nil
}

func TestCleanupProcessorRetriesThenAbandonsAndContinuesOtherRecords(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	store := &cleanupStore{due: []ports.ScreenshotCleanup{
		{ID: "retry", QuarantinedKey: "quarantine/retry.png", AttemptCount: 1},
		{ID: "remove", QuarantinedKey: "quarantine/remove.png"},
	}}
	files := newMaintenanceFiles()
	files.removeErr = errors.New("temporarily unavailable")
	processor := usecase.NewCleanupProcessor(usecase.CleanupDependencies{Store: store, Files: files, Clock: clock, MaximumAttempts: 2, Limit: 10})

	if err := processor.Process(context.Background()); err != nil {
		t.Fatalf("process cleanup: %v", err)
	}
	if len(store.completed) != 2 {
		t.Fatalf("completed cleanup records = %#v, want two", store.completed)
	}
	if store.completed[0].State != "abandoned" || store.completed[1].State != "pending" {
		t.Fatalf("cleanup states = %#v, want abandonment then a retryable record", store.completed)
	}
}

type cleanupStore struct {
	due       []ports.ScreenshotCleanup
	completed []ports.ScreenshotCleanup
}

func (store *cleanupStore) ClaimDueScreenshotCleanup(_ context.Context, _ time.Time, _ int) ([]ports.ScreenshotCleanup, error) {
	return store.due, nil
}

func (store *cleanupStore) CompleteScreenshotCleanup(_ context.Context, cleanup ports.ScreenshotCleanup) error {
	store.completed = append(store.completed, cleanup)
	return nil
}
