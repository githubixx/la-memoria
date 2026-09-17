package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestRetentionProcessorExpiresRecordsUsingTheConfiguredClockAndBatchLimit(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	store := &retentionStore{}
	processor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: store, Clock: clock, Limit: 7, SessionAge: 24 * time.Hour, ThrottleAge: time.Hour})

	if err := processor.Process(context.Background()); err != nil {
		t.Fatalf("process retention: %v", err)
	}
	if store.limit != 7 || !store.draftsBefore.Equal(clock.Now()) || !store.sessionsBefore.Equal(clock.Now().Add(-24*time.Hour)) || !store.throttlesBefore.Equal(clock.Now().Add(-time.Hour)) {
		t.Fatalf("retention calls = %#v", store)
	}
}

func TestRetentionProcessorStopsAfterTheFirstStorageFailure(t *testing.T) {
	want := errors.New("draft retention failed")
	store := &retentionStore{draftError: want}
	processor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: store, Clock: testkit.NewClock(time.Now())})

	if err := processor.Process(context.Background()); !errors.Is(err, want) {
		t.Fatalf("process error = %v, want %v", err, want)
	}
	if store.sessionCalls != 0 || store.throttleCalls != 0 {
		t.Fatalf("retention continued after draft failure: sessions=%d throttles=%d", store.sessionCalls, store.throttleCalls)
	}
}

func TestRetentionProcessorHonorsCanceledContextBeforeStorage(t *testing.T) {
	store := &retentionStore{}
	processor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{Store: store, Clock: testkit.NewClock(time.Now())})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := processor.Process(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("process error = %v, want context canceled", err)
	}
	if store.draftCalls != 0 || store.sessionCalls != 0 || store.throttleCalls != 0 {
		t.Fatalf("retention touched storage after cancellation: %#v", store)
	}
}

func TestRetentionProcessorProcessesDueScreenshotCleanupRecords(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	store := &retentionStore{}
	cleanup := &retentionCleanupStore{due: []ports.ScreenshotCleanup{{ID: "cleanup-1", QuarantinedKey: "quarantine/old.png"}}}
	files := testkit.NewScreenshotFileStore()
	processor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{
		Store:        store,
		CleanupStore: cleanup,
		Files:        files,
		Clock:        clock,
		Limit:        7,
	})

	if err := processor.Process(context.Background()); err != nil {
		t.Fatalf("process retention: %v", err)
	}
	if cleanup.claimCalls != 1 || len(cleanup.completed) != 1 {
		t.Fatalf("cleanup calls = claimed %d, completed %#v; want one successful cleanup", cleanup.claimCalls, cleanup.completed)
	}
	if cleanup.completed[0].State != "succeeded" || cleanup.completed[0].AttemptCount != 0 {
		t.Fatalf("completed cleanup = %#v, want succeeded without a retry", cleanup.completed[0])
	}
}

func TestRetentionProcessorReturnsScreenshotCleanupFailure(t *testing.T) {
	want := errors.New("complete cleanup record")
	cleanup := &retentionCleanupStore{
		due:         []ports.ScreenshotCleanup{{ID: "cleanup-1", QuarantinedKey: "quarantine/old.png"}},
		completeErr: want,
	}
	processor := usecase.NewRetentionProcessor(usecase.RetentionDependencies{
		Store:        &retentionStore{},
		CleanupStore: cleanup,
		Files:        testkit.NewScreenshotFileStore(),
		Clock:        testkit.NewClock(time.Now()),
	})

	if err := processor.Process(context.Background()); !errors.Is(err, want) {
		t.Fatalf("process error = %v, want %v", err, want)
	}
}

type retentionStore struct {
	limit           int
	draftsBefore    time.Time
	sessionsBefore  time.Time
	throttlesBefore time.Time
	draftCalls      int
	sessionCalls    int
	throttleCalls   int
	draftError      error
}

func (store *retentionStore) ExpireCaptureDrafts(_ context.Context, before time.Time, limit int) error {
	store.draftCalls++
	store.draftsBefore, store.limit = before, limit
	return store.draftError
}

func (store *retentionStore) DeleteExpiredSessions(_ context.Context, before time.Time, _ int) error {
	store.sessionCalls++
	store.sessionsBefore = before
	return nil
}

func (store *retentionStore) DeleteExpiredThrottlePairs(_ context.Context, before time.Time, _ int) error {
	store.throttleCalls++
	store.throttlesBefore = before
	return nil
}

type retentionCleanupStore struct {
	due         []ports.ScreenshotCleanup
	claimCalls  int
	completed   []ports.ScreenshotCleanup
	completeErr error
}

func (store *retentionCleanupStore) ClaimDueScreenshotCleanup(_ context.Context, _ time.Time, _ int) ([]ports.ScreenshotCleanup, error) {
	store.claimCalls++
	return store.due, nil
}

func (store *retentionCleanupStore) CompleteScreenshotCleanup(_ context.Context, cleanup ports.ScreenshotCleanup) error {
	store.completed = append(store.completed, cleanup)
	return store.completeErr
}
