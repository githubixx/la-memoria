//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func insertBookmark(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, url, description string, createdAt time.Time, tags []string) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES ($1, $2, $3, $4, $4)`, id, url, description, createdAt)
	if err != nil {
		t.Fatalf("insert bookmark %q: %v", id, err)
	}
	for _, tagName := range tags {
		normalized := normalizeForFixture(tagName)
		var tagID string
		err := pool.QueryRow(ctx, `
			INSERT INTO tags (id, display_name, normalized_name) VALUES ($1, $2, $3)
			ON CONFLICT (normalized_name) DO UPDATE SET normalized_name = EXCLUDED.normalized_name
			RETURNING id`, uuid.NewString(), tagName, normalized).Scan(&tagID)
		if err != nil {
			t.Fatalf("upsert tag %q: %v", tagName, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES ($1, $2)`, id, tagID); err != nil {
			t.Fatalf("link tag %q to bookmark %q: %v", tagName, id, err)
		}
	}
}

func normalizeForFixture(name string) string {
	result := make([]rune, 0, len(name))
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		result = append(result, r)
	}
	return string(result)
}

func seedBookmarkQueryFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	base := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	insertBookmark(t, ctx, pool, uuid.NewString(), "https://example.test/dup", "first duplicate", base, []string{"Go"})
	insertBookmark(t, ctx, pool, uuid.NewString(), "https://example.test/dup", "second duplicate about database transactions", base.Add(time.Hour), []string{"go"})
	insertBookmark(t, ctx, pool, uuid.NewString(), "https://example.test/unrelated", "unrelated content", base.Add(2*time.Hour), []string{"rust"})
	for index := range 12 {
		insertBookmark(t, ctx, pool, uuid.NewString(), "https://example.test/page", "database transactions page filler", base.Add(time.Duration(3+index)*time.Hour), []string{"go"})
	}
}

func TestPostgreSQLBookmarkQueriesPreserveDuplicatesNormalizeTagsAndCapBeforePaginating(t *testing.T) {
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

	seedBookmarkQueryFixtures(t, ctx, pool)
	store := postgres.NewBookmarkQueryStore(pool)

	browsed, total, err := store.BrowsePage(ctx, "", 1, 10)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if total != 15 {
		t.Fatalf("browse total = %d, want 15 (duplicate URLs preserved as separate records)", total)
	}
	for index := 1; index < len(browsed); index++ {
		if !browsed[index-1].CreatedAt.After(browsed[index].CreatedAt) {
			t.Fatalf("browse order = %#v, want strict newest-first (created_at, id) ordering", browsed)
		}
	}

	_, tagTotal, err := store.BrowsePage(ctx, "go", 1, 20)
	if err != nil {
		t.Fatalf("browse by tag: %v", err)
	}
	if tagTotal != 14 {
		t.Fatalf("tag-filtered total = %d, want 14 (mixed-case tag spellings normalize together)", tagTotal)
	}

	items, totalWithinCap, isCapped, err := store.SearchPage(ctx, "go", []string{"database", "transactions"}, 1, 10, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !isCapped || totalWithinCap != 5 {
		t.Fatalf("search cap = (capped=%t, total=%d), want capped at 5", isCapped, totalWithinCap)
	}
	if len(items) != 5 {
		t.Fatalf("search page items = %d, want the full capped page of 5", len(items))
	}

	secondPage, _, _, err := store.SearchPage(ctx, "go", []string{"database", "transactions"}, 2, 10, 5)
	if err != nil {
		t.Fatalf("search page 2: %v", err)
	}
	if len(secondPage) != len(items) {
		t.Fatalf("requesting a page beyond the capped single-page set = %d items, want it clamped back to the same %d capped items", len(secondPage), len(items))
	}

	clamped, _, err := store.BrowsePage(ctx, "", 999, 10)
	if err != nil {
		t.Fatalf("browse clamped page: %v", err)
	}
	if len(clamped) == 0 {
		t.Fatal("browse page beyond the last page must clamp to the final available page, not return empty")
	}
}

type queryPool = interface {
	Exec(ctx context.Context, sql string, arguments ...any) (any, error)
}
