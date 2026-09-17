//go:build integration

package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestPostgreSQLLoginThrottleIsPairSpecificAndAtomic(t *testing.T) {
	ctx := context.Background()
	database := testkit.StartPostgreSQL(ctx, t)
	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.up.sql")); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	store, err := postgres.NewAuthStore(ctx, database.ConnectionString)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	var group sync.WaitGroup
	for range 5 {
		group.Go(func() {
			if err := store.RecordFailure(ctx, "admin", "192.0.2.10", now); err != nil {
				t.Errorf("record failed login: %v", err)
			}
		})
	}
	group.Wait()

	blocked, retryAfter, err := store.IsBlocked(ctx, "admin", "192.0.2.10", now)
	if err != nil || !blocked || retryAfter != 15*time.Minute {
		t.Fatalf("same pair block = (%t, %s, %v), want (true, 15m, nil)", blocked, retryAfter, err)
	}
	otherAddressBlocked, _, err := store.IsBlocked(ctx, "admin", "192.0.2.11", now)
	if err != nil || otherAddressBlocked {
		t.Fatalf("other source must remain unblocked: blocked=%t err=%v", otherAddressBlocked, err)
	}
}

func TestPostgreSQLSessionRevocationIsImmediatelyAuthoritative(t *testing.T) {
	ctx := context.Background()
	database := testkit.StartPostgreSQL(ctx, t)
	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.up.sql")); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	store, err := postgres.NewAuthStore(ctx, database.ConnectionString)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	session, err := store.CreateSession(ctx, []byte("token-digest"), []byte("csrf-digest"), model.SessionAuthenticated, time.Now().UTC())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := store.RevokeSession(ctx, session.ID, time.Now().UTC()); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := store.FindActiveSession(ctx, []byte("token-digest")); err == nil {
		t.Fatal("revoked session must not be returned as active")
	}
}
