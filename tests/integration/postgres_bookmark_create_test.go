//go:build integration

package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestPostgreSQLBookmarkCreateWritesTransactionallyAndPreservesDuplicateURLs(t *testing.T) {
	ctx := context.Background()
	database := testkit.StartPostgreSQL(ctx, t)
	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.up.sql")); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	pool, err := postgres.NewPool(ctx, database.ConnectionString)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	store := postgres.NewBookmarkCreateStore(pool)
	now := time.Now().UTC()

	first := ports.NewBookmark{ID: model.NewID(), URL: "https://example.test/dup", Description: "first", Tags: model.NormalizeTags([]string{"Go"}), ScreenshotStorageKey: "promoted/one.png", CapturedURL: "https://example.test/dup", Now: now}
	if err := store.CreateBookmark(ctx, first); err != nil {
		t.Fatalf("create first bookmark: %v", err)
	}
	second := ports.NewBookmark{ID: model.NewID(), URL: "https://example.test/dup", Description: "second", Tags: model.NormalizeTags([]string{"go"}), Now: now}
	if err := store.CreateBookmark(ctx, second); err != nil {
		t.Fatalf("create second bookmark with duplicate URL: %v", err)
	}

	queryStore := postgres.NewBookmarkQueryStore(pool)
	items, total, err := queryStore.BrowsePage(ctx, "go", 1, 10)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if total != 2 {
		t.Fatalf("total with normalized go tag = %d, want 2 (concurrent-safe normalized tag upsert)", total)
	}
	found := false
	for _, item := range items {
		if item.ID == first.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("first bookmark must be findable by its normalized tag")
	}
}

func TestPostgreSQLConcurrentTagCreationConvergesOnOneNormalizedRow(t *testing.T) {
	ctx := context.Background()
	database := testkit.StartPostgreSQL(ctx, t)
	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.up.sql")); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	pool, err := postgres.NewPool(ctx, database.ConnectionString)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	store := postgres.NewBookmarkCreateStore(pool)
	now := time.Now().UTC()

	var group sync.WaitGroup
	for index := range 10 {
		group.Go(func() {
			bookmark := ports.NewBookmark{ID: model.NewID(), URL: "https://example.test/concurrent", Description: "concurrent", Tags: model.NormalizeTags([]string{"Concurrent"}), Now: now.Add(time.Duration(index) * time.Second)}
			if err := store.CreateBookmark(ctx, bookmark); err != nil {
				t.Errorf("concurrent create: %v", err)
			}
		})
	}
	group.Wait()

	var tagCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM tags WHERE normalized_name = 'concurrent'`).Scan(&tagCount); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tagCount != 1 {
		t.Fatalf("normalized tag rows = %d, want exactly 1 despite concurrent creation", tagCount)
	}
}

func TestPostgreSQLCaptureDraftPersistsStateAcrossRequests(t *testing.T) {
	ctx := context.Background()
	database := testkit.StartPostgreSQL(ctx, t)
	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.up.sql")); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	pool, err := postgres.NewPool(ctx, database.ConnectionString)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	authStore, err := postgres.NewAuthStore(ctx, database.ConnectionString)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	t.Cleanup(authStore.Close)
	session, err := authStore.CreateSession(ctx, []byte("token"), []byte("csrf"), model.SessionAuthenticated, time.Now().UTC())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	drafts := postgres.NewCaptureDraftStore(pool)
	now := time.Now().UTC()
	draft := model.CaptureDraft{
		ID: model.NewID(), OwnerSessionID: session.ID, TargetURL: "https://example.test/page",
		State: model.CaptureStateCapturing, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	if err := drafts.CreateDraft(ctx, draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	draft.State = model.CaptureStateReady
	draft.StagedKey = "staging/abc.png"
	draft.FinalURL = "https://example.test/page"
	if err := drafts.UpdateDraft(ctx, draft); err != nil {
		t.Fatalf("update draft: %v", err)
	}

	found, err := drafts.FindDraft(ctx, draft.ID)
	if err != nil {
		t.Fatalf("find draft: %v", err)
	}
	if found.State != model.CaptureStateReady || found.StagedKey != "staging/abc.png" || found.OwnerSessionID != session.ID {
		t.Fatalf("persisted draft = %#v, want updated ready state with staged key and owner", found)
	}
}
