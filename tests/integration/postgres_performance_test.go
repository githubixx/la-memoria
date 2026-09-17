//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/tests/testkit"
	"github.com/jackc/pgx/v5/pgxpool"
)

const performanceBookmarkCount = 100000

func TestPostgreSQLBrowseAndSearch100KBookmarksWithinTwoSeconds(t *testing.T) {
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
	seedPerformanceBookmarks(t, ctx, pool)
	store := postgres.NewBookmarkQueryStore(pool)

	started := time.Now()
	items, total, err := store.BrowsePage(ctx, "", 1, 10)
	if err != nil || total != performanceBookmarkCount || len(items) != 10 {
		t.Fatalf("browse = %d items, total %d, err %v", len(items), total, err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("browse took %s, want at most 2s", elapsed)
	}

	started = time.Now()
	items, total, capped, err := store.SearchPage(ctx, "", []string{"performance", "bookmark"}, 1, 10, 10000)
	if err != nil || !capped || total != 10000 || len(items) != 10 {
		t.Fatalf("search = %d items, total %d, capped %t, err %v", len(items), total, capped, err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("search took %s, want at most 2s", elapsed)
	}
}

func TestPostgreSQLPerformanceQueryPlansUseIndexes(t *testing.T) {
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
	seedPerformanceBookmarks(t, ctx, pool)

	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire plan connection: %v", err)
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, `SET enable_seqscan TO off`); err != nil {
		t.Fatalf("disable sequential scans for plan check: %v", err)
	}
	assertPlanUsesIndex(t, ctx, connection, `EXPLAIN (COSTS OFF) SELECT id FROM bookmarks ORDER BY created_at DESC, id DESC LIMIT 10`)
	assertPlanUsesIndex(t, ctx, connection, `EXPLAIN (COSTS OFF) SELECT id FROM bookmarks WHERE to_tsvector('simple', description) @@ plainto_tsquery('simple', 'rare') LIMIT 10`)
}

func seedPerformanceBookmarks(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO bookmarks (id, url, description, created_at, updated_at)
		SELECT lpad(to_hex(number), 32, '0'), 'https://example.test/performance/' || number,
			CASE WHEN number = 1 THEN 'rare performance bookmark' WHEN number <= 20000 THEN 'performance bookmark ' || number ELSE 'ordinary bookmark ' || number END,
			now() - number * interval '1 second', now() - number * interval '1 second'
		FROM generate_series(1, $1) AS number`, performanceBookmarkCount)
	if err != nil {
		t.Fatalf("seed %d bookmarks: %v", performanceBookmarkCount, err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE bookmarks`); err != nil {
		t.Fatalf("analyze bookmarks: %v", err)
	}
}

func assertPlanUsesIndex(t *testing.T, ctx context.Context, pool *pgxpool.Conn, query string) {
	t.Helper()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("explain query: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	plan := strings.Join(lines, "\n")
	if !strings.Contains(plan, "Index Scan") && !strings.Contains(plan, "Index Only Scan") && !strings.Contains(plan, "Bitmap Index Scan") {
		t.Fatalf("query plan did not use an index:\n%s", plan)
	}
}
