//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestMigration0001CreatesBookmarkSchemaAndRollsBack(t *testing.T) {
	ctx := context.Background()
	database := testkit.StartPostgreSQL(ctx, t)

	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.up.sql")); err != nil {
		t.Fatalf("apply migration 0001: %v", err)
	}

	connection := database.Connect(ctx, t)
	assertSchemaVersion(t, ctx, connection, "0001")
	assertConstraint(t, ctx, connection, "tags_normalized_name_key")
	assertConstraint(t, ctx, connection, "bookmark_tags_pkey")
	assertConstraint(t, ctx, connection, "screenshots_bookmark_id_key")
	assertIndex(t, ctx, connection, "bookmarks_created_at_id_desc_idx")
	assertIndex(t, ctx, connection, "bookmarks_description_search_idx")

	if err := database.ApplyMigration(ctx, testkit.MigrationPath("0001_bookmarks.down.sql")); err != nil {
		t.Fatalf("roll back migration 0001: %v", err)
	}
	assertTableAbsent(t, ctx, connection, "bookmarks")
	assertTableAbsent(t, ctx, connection, "schema_migrations")
}

func assertSchemaVersion(t *testing.T, ctx context.Context, connection Queryer, want string) {
	t.Helper()
	var version string
	if err := connection.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != want {
		t.Fatalf("schema version = %q, want %q", version, want)
	}
}

func assertConstraint(t *testing.T, ctx context.Context, connection Queryer, name string) {
	t.Helper()
	var found bool
	if err := connection.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)", name).Scan(&found); err != nil {
		t.Fatalf("query constraint %q: %v", name, err)
	}
	if !found {
		t.Fatalf("missing constraint %q", name)
	}
}

func assertIndex(t *testing.T, ctx context.Context, connection Queryer, name string) {
	t.Helper()
	var found bool
	if err := connection.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1)", name).Scan(&found); err != nil {
		t.Fatalf("query index %q: %v", name, err)
	}
	if !found {
		t.Fatalf("missing index %q", name)
	}
}

func assertTableAbsent(t *testing.T, ctx context.Context, connection Queryer, name string) {
	t.Helper()
	var found bool
	if err := connection.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", fmt.Sprintf("public.%s", name)).Scan(&found); err != nil {
		t.Fatalf("check table %q after rollback: %v", name, err)
	}
	if found {
		t.Fatalf("table %q remains after rollback", name)
	}
}

type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
