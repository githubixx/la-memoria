package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// BookmarkCreateStore implements ports.BookmarkWriter: one transaction per
// bookmark, tags upserted by normalized name, and an optional promoted
// screenshot. Duplicate URLs are always accepted.
type BookmarkCreateStore struct{ pool *pgxpool.Pool }

func NewBookmarkCreateStore(pool *pgxpool.Pool) *BookmarkCreateStore {
	return &BookmarkCreateStore{pool: pool}
}

func (store *BookmarkCreateStore) CreateBookmark(ctx context.Context, bookmark ports.NewBookmark) error {
	return WithTransaction(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES ($1, $2, $3, $4, $4)`,
			bookmark.ID.String(), bookmark.URL, bookmark.Description, bookmark.Now); err != nil {
			return fmt.Errorf("insert bookmark: %w", err)
		}
		for _, tag := range bookmark.Tags {
			var tagID string
			err := tx.QueryRow(ctx, `
				INSERT INTO tags (id, display_name, normalized_name) VALUES ($1, $2, $3)
				ON CONFLICT (normalized_name) DO UPDATE SET normalized_name = EXCLUDED.normalized_name
				RETURNING id`, model.NewID().String(), tag.DisplayName, tag.NormalizedName).Scan(&tagID)
			if err != nil {
				return fmt.Errorf("upsert tag %q: %w", tag.NormalizedName, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES ($1, $2)`, bookmark.ID.String(), tagID); err != nil {
				return fmt.Errorf("link tag %q: %w", tag.NormalizedName, err)
			}
		}
		if bookmark.ScreenshotStorageKey != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO screenshots (id, bookmark_id, storage_key, captured_url, captured_at, media_type, byte_size)
				VALUES ($1, $2, $3, $4, $5, 'image/png', 0)`,
				model.NewID().String(), bookmark.ID.String(), bookmark.ScreenshotStorageKey, bookmark.CapturedURL, bookmark.Now); err != nil {
				return fmt.Errorf("insert screenshot: %w", err)
			}
		}
		return nil
	})
}
