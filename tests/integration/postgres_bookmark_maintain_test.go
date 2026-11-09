//go:build integration

package integration_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/postgres"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestPostgreSQLBookmarkMaintainFindHydratesTags(t *testing.T) {
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
	create := postgres.NewBookmarkCreateStore(pool)
	store := postgres.NewBookmarkMaintainStore(pool)
	var taggedID model.ID
	for _, tc := range []struct {
		name string
		tags []model.Tag
		want []model.Tag
	}{
		{name: "tagged", tags: model.NormalizeTags([]string{"Zulu", "Alpha"}), want: []model.Tag{{DisplayName: "Alpha", NormalizedName: "alpha"}, {DisplayName: "Zulu", NormalizedName: "zulu"}}},
		{name: "untagged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := model.NewID()
			if tc.name == "tagged" {
				taggedID = id
			}
			if err := create.CreateBookmark(ctx, ports.NewBookmark{ID: id, URL: "https://example.test/" + tc.name, Description: tc.name, Tags: tc.tags, Now: time.Now().UTC()}); err != nil {
				t.Fatalf("create bookmark: %v", err)
			}
			got, err := store.FindBookmark(ctx, id)
			if err != nil {
				t.Fatalf("find bookmark: %v", err)
			}
			if len(got.Tags) != len(tc.want) || (len(tc.want) > 0 && !reflect.DeepEqual(got.Tags, tc.want)) {
				t.Fatalf("tags = %#v, want %#v", got.Tags, tc.want)
			}
		})
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE bookmark_tags RENAME TO inaccessible_bookmark_tags`); err != nil {
		t.Fatalf("make tag lookup unavailable: %v", err)
	}
	if _, err := store.FindBookmark(ctx, taggedID); err == nil {
		t.Fatal("find bookmark succeeded despite unavailable tag table")
	}
}

func TestPostgreSQLBookmarkMaintainUpdatesTagsAndPreservesCreationTime(t *testing.T) {
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

	createdAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	bookmarkID := model.NewID()
	if err := postgres.NewBookmarkCreateStore(pool).CreateBookmark(ctx, ports.NewBookmark{
		ID: bookmarkID, URL: "https://old.example.test", Description: "old", Tags: model.NormalizeTags([]string{"old"}), Now: createdAt,
	}); err != nil {
		t.Fatalf("create bookmark: %v", err)
	}

	updatedAt := createdAt.Add(time.Hour)
	if err := postgres.NewBookmarkMaintainStore(pool).UpdateBookmark(ctx, ports.UpdatedBookmark{
		ID: bookmarkID, URL: "https://new.example.test", Description: "new", Tags: model.NormalizeTags([]string{"new"}), UpdatedAt: updatedAt,
	}); err != nil {
		t.Fatalf("update bookmark: %v", err)
	}

	var url, description string
	var persistedCreatedAt, persistedUpdatedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT url, description, created_at, updated_at FROM bookmarks WHERE id = $1`, bookmarkID.String()).Scan(&url, &description, &persistedCreatedAt, &persistedUpdatedAt); err != nil {
		t.Fatalf("read updated bookmark: %v", err)
	}
	if url != "https://new.example.test" || description != "new" || !persistedCreatedAt.Equal(createdAt) || !persistedUpdatedAt.Equal(updatedAt) {
		t.Fatalf("persisted bookmark = url=%q description=%q created=%s updated=%s", url, description, persistedCreatedAt, persistedUpdatedAt)
	}
	var tagCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM bookmark_tags bt JOIN tags t ON t.id = bt.tag_id WHERE bt.bookmark_id = $1 AND t.normalized_name = 'new'`, bookmarkID.String()).Scan(&tagCount); err != nil {
		t.Fatalf("count replacement tags: %v", err)
	}
	if tagCount != 1 {
		t.Fatalf("replacement tag rows = %d, want 1", tagCount)
	}
}
