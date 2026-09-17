//go:build integration

package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgreSQLRetentionExpiresOnlyDueRecordsInBoundedConcurrentBatches(t *testing.T) {
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

	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	seedRetentionRecords(t, ctx, pool, now)
	store := postgres.NewRetentionStore(pool)

	if err := store.ExpireCaptureDrafts(ctx, now, 1); err != nil {
		t.Fatalf("expire first draft batch: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM capture_drafts WHERE state = 'expired'`, 1)
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM capture_drafts WHERE state = 'ready'`, 2)
	if err := store.ExpireCaptureDrafts(ctx, now, 10); err != nil {
		t.Fatalf("expire remaining drafts: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM capture_drafts WHERE state = 'expired'`, 2)
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM capture_drafts WHERE id = 'draft-live' AND state = 'ready'`, 1)

	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := store.DeleteExpiredThrottlePairs(ctx, now.Add(-time.Hour), 1); err != nil {
				t.Errorf("delete throttle batch: %v", err)
			}
		}()
	}
	workers.Wait()
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM login_throttle_pairs WHERE updated_at <= $1`, 0, now.Add(-time.Hour))
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM login_throttle_pairs WHERE source_address = 'live'`, 1)

	if err := store.DeleteExpiredSessions(ctx, now.Add(-time.Hour), 1); err != nil {
		t.Fatalf("delete expired session: %v", err)
	}
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM web_sessions WHERE id = 'session-expired'`, 0)
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM web_sessions WHERE id = 'session-live'`, 1)
}

func TestPostgreSQLRetentionProcessesScreenshotCleanupRetriesWithoutDuplicateClaims(t *testing.T) {
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

	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	for _, cleanup := range []struct {
		id            string
		nextAttemptAt time.Time
	}{
		{id: "cleanup-retry", nextAttemptAt: now},
		{id: "cleanup-concurrent-1", nextAttemptAt: now.Add(time.Hour)},
		{id: "cleanup-concurrent-2", nextAttemptAt: now.Add(time.Hour)},
		{id: "cleanup-live", nextAttemptAt: now.Add(2 * time.Hour)},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO screenshot_cleanup (id, quarantined_key, operation, state, attempt_count, next_attempt_at) VALUES ($1, $2, 'remove_deleted', 'pending', 0, $3)`, cleanup.id, "quarantine/"+cleanup.id+".png", cleanup.nextAttemptAt); err != nil {
			t.Fatalf("seed cleanup %q: %v", cleanup.id, err)
		}
	}
	store := postgres.NewRetentionStore(pool)
	clock := testkit.NewClock(now)
	failingFiles := &retentionFiles{removeErr: context.DeadlineExceeded}
	processor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: store, CleanupStore: store, Files: failingFiles, Clock: clock, Limit: 1, MaximumCleanupAttempts: 3})
	if err := processor.Process(ctx); err != nil {
		t.Fatalf("process failed cleanup: %v", err)
	}
	assertCleanupState(t, ctx, pool, "cleanup-retry", "pending", 1, now.Add(time.Minute), "cleanup_failed")

	clock.Advance(time.Minute)
	processor = usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: store, CleanupStore: store, Files: testkit.NewScreenshotFileStore(), Clock: clock, Limit: 1})
	if err := processor.Process(ctx); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	assertCleanupState(t, ctx, pool, "cleanup-retry", "succeeded", 1, now.Add(time.Minute), "")

	clock.Advance(59 * time.Minute)
	files := testkit.NewScreenshotFileStore()
	processor = usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: store, CleanupStore: store, Files: files, Clock: clock, Limit: 1})
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := processor.Process(ctx); err != nil {
				t.Errorf("process concurrent cleanup: %v", err)
			}
		}()
	}
	workers.Wait()
	assertCount(t, ctx, pool, `SELECT COUNT(*) FROM screenshot_cleanup WHERE state = 'succeeded'`, 3)
	assertCleanupState(t, ctx, pool, "cleanup-live", "pending", 0, now.Add(2*time.Hour), "")
	if len(files.Removed) != 2 {
		t.Fatalf("removed cleanup files = %#v, want two distinct concurrent claims", files.Removed)
	}
}

func seedRetentionRecords(t *testing.T, ctx context.Context, pool *pgxpool.Pool, now time.Time) {
	t.Helper()
	for _, session := range []struct {
		id      string
		created time.Time
	}{{"session-expired", now.Add(-2 * time.Hour)}, {"session-live", now}} {
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (id, token_digest, csrf_token_digest, state, created_at) VALUES ($1, $2, $3, 'authenticated', $4)`, session.id, []byte(session.id), []byte("csrf-"+session.id), session.created); err != nil {
			t.Fatalf("seed session %q: %v", session.id, err)
		}
	}
	for _, draft := range []struct {
		id, owner string
		expires   time.Time
	}{{"draft-expired-1", "session-expired", now.Add(-time.Minute)}, {"draft-expired-2", "session-expired", now}, {"draft-live", "session-live", now.Add(time.Hour)}} {
		if _, err := pool.Exec(ctx, `INSERT INTO capture_drafts (id, owner_session_id, target_url, state, created_at, expires_at) VALUES ($1, $2, 'https://example.test', 'ready', $3, $4)`, draft.id, draft.owner, now, draft.expires); err != nil {
			t.Fatalf("seed draft %q: %v", draft.id, err)
		}
	}
	for _, pair := range []struct {
		address string
		updated time.Time
	}{{"expired-1", now.Add(-2 * time.Hour)}, {"expired-2", now.Add(-3 * time.Hour)}, {"live", now}} {
		if _, err := pool.Exec(ctx, `INSERT INTO login_throttle_pairs (username_key, source_address, window_started_at, failure_count, updated_at) VALUES ($1, $2, $3, 1, $3)`, []byte(pair.address), pair.address, pair.updated); err != nil {
			t.Fatalf("seed throttle pair %q: %v", pair.address, err)
		}
	}
}

func assertCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, want int, arguments ...any) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, query, arguments...).Scan(&got); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d for %q", got, want, query)
	}
}

func assertCleanupState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, state string, attempts int, nextAttemptAt time.Time, errorCode string) {
	t.Helper()
	var gotState, gotErrorCode string
	var gotAttempts int
	var gotNextAttemptAt time.Time
	if err := pool.QueryRow(ctx, `SELECT state, attempt_count, next_attempt_at, COALESCE(last_error_code, '') FROM screenshot_cleanup WHERE id = $1`, id).Scan(&gotState, &gotAttempts, &gotNextAttemptAt, &gotErrorCode); err != nil {
		t.Fatalf("load cleanup %q: %v", id, err)
	}
	if gotState != state || gotAttempts != attempts || !gotNextAttemptAt.Equal(nextAttemptAt) || gotErrorCode != errorCode {
		t.Fatalf("cleanup %q = state=%q attempts=%d next=%s error=%q; want state=%q attempts=%d next=%s error=%q", id, gotState, gotAttempts, gotNextAttemptAt, gotErrorCode, state, attempts, nextAttemptAt, errorCode)
	}
}

type retentionFiles struct {
	*testkit.ScreenshotFileStore
	removeErr error
}

func (files *retentionFiles) Remove(ctx context.Context, storageKey string) error {
	if files.ScreenshotFileStore == nil {
		files.ScreenshotFileStore = testkit.NewScreenshotFileStore()
	}
	if files.removeErr != nil {
		return files.removeErr
	}
	return files.ScreenshotFileStore.Remove(ctx, storageKey)
}
