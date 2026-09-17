package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// BookmarkMaintainStore implements the transactional persistence operations
// used by authenticated bookmark maintenance.
type BookmarkMaintainStore struct{ pool *pgxpool.Pool }

func NewBookmarkMaintainStore(pool *pgxpool.Pool) *BookmarkMaintainStore {
	return &BookmarkMaintainStore{pool: pool}
}

func (store *BookmarkMaintainStore) FindBookmark(ctx context.Context, id model.ID) (ports.StoredBookmark, error) {
	var bookmark ports.StoredBookmark
	var screenshotKey *string
	err := store.pool.QueryRow(ctx, `
		SELECT b.id, b.url, b.description, b.created_at, b.updated_at, s.storage_key
		FROM bookmarks b LEFT JOIN screenshots s ON s.bookmark_id = b.id WHERE b.id = $1`, id.String()).Scan(
		&bookmark.ID, &bookmark.URL, &bookmark.Description, &bookmark.CreatedAt, &bookmark.UpdatedAt, &screenshotKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.StoredBookmark{}, model.ErrBookmarkNotFound
	}
	if err != nil {
		return ports.StoredBookmark{}, err
	}
	if screenshotKey != nil {
		bookmark.ScreenshotStorageKey = *screenshotKey
	}
	return bookmark, nil
}

func (store *BookmarkMaintainStore) UpdateBookmark(ctx context.Context, bookmark ports.UpdatedBookmark) error {
	return WithTransaction(ctx, store.pool, func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE bookmarks SET url = $2, description = $3, updated_at = $4 WHERE id = $1`, bookmark.ID.String(), bookmark.URL, bookmark.Description, bookmark.UpdatedAt)
		if err != nil {
			return fmt.Errorf("update bookmark: %w", err)
		}
		if command.RowsAffected() == 0 {
			return model.ErrBookmarkNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM bookmark_tags WHERE bookmark_id = $1`, bookmark.ID.String()); err != nil {
			return fmt.Errorf("clear bookmark tags: %w", err)
		}
		for _, tag := range bookmark.Tags {
			var tagID string
			if err := tx.QueryRow(ctx, `INSERT INTO tags (id, display_name, normalized_name) VALUES ($1, $2, $3) ON CONFLICT (normalized_name) DO UPDATE SET normalized_name = EXCLUDED.normalized_name RETURNING id`, model.NewID().String(), tag.DisplayName, tag.NormalizedName).Scan(&tagID); err != nil {
				return fmt.Errorf("upsert tag %q: %w", tag.NormalizedName, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES ($1, $2)`, bookmark.ID.String(), tagID); err != nil {
				return fmt.Errorf("link tag %q: %w", tag.NormalizedName, err)
			}
		}
		if !bookmark.ReplaceScreenshot {
			return nil
		}
		if _, err := tx.Exec(ctx, `DELETE FROM screenshots WHERE bookmark_id = $1`, bookmark.ID.String()); err != nil {
			return fmt.Errorf("remove replaced screenshot: %w", err)
		}
		if bookmark.ScreenshotStorageKey == "" {
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO screenshots (id, bookmark_id, storage_key, captured_url, captured_at, media_type, byte_size) VALUES ($1, $2, $3, $4, $5, 'image/png', 0)`, model.NewID().String(), bookmark.ID.String(), bookmark.ScreenshotStorageKey, bookmark.CapturedURL, bookmark.UpdatedAt); err != nil {
			return fmt.Errorf("insert replacement screenshot: %w", err)
		}
		return nil
	})
}

func (store *BookmarkMaintainStore) DeleteBookmark(ctx context.Context, id model.ID) error {
	command, err := store.pool.Exec(ctx, `DELETE FROM bookmarks WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	if command.RowsAffected() == 0 {
		return model.ErrBookmarkNotFound
	}
	return nil
}

func (store *BookmarkMaintainStore) CreateScreenshotCleanup(ctx context.Context, cleanup ports.ScreenshotCleanup) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO screenshot_cleanup (id, screenshot_id, quarantined_key, operation, state, attempt_count, next_attempt_at) VALUES ($1, NULLIF($2, ''), $3, $4, 'pending', $5, $6)`, cleanup.ID.String(), cleanup.ScreenshotID.String(), cleanup.QuarantinedKey, cleanup.Operation, cleanup.AttemptCount, cleanup.NextAttemptAt)
	if err != nil {
		return fmt.Errorf("create screenshot cleanup: %w", err)
	}
	return nil
}

func (store *BookmarkMaintainStore) ClaimDueScreenshotCleanup(ctx context.Context, now time.Time, limit int) ([]ports.ScreenshotCleanup, error) {
	rows, err := store.pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM screenshot_cleanup WHERE state = 'pending' AND next_attempt_at <= $1
			ORDER BY next_attempt_at FOR UPDATE SKIP LOCKED LIMIT $2
		)
		UPDATE screenshot_cleanup c SET state = 'running' FROM due WHERE c.id = due.id
		RETURNING c.id, c.screenshot_id, c.quarantined_key, c.operation, c.attempt_count, c.next_attempt_at`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim screenshot cleanup: %w", err)
	}
	defer rows.Close()
	var records []ports.ScreenshotCleanup
	for rows.Next() {
		var record ports.ScreenshotCleanup
		var screenshotID *string
		if err := rows.Scan(&record.ID, &screenshotID, &record.QuarantinedKey, &record.Operation, &record.AttemptCount, &record.NextAttemptAt); err != nil {
			return nil, err
		}
		if screenshotID != nil {
			record.ScreenshotID = model.ID(*screenshotID)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (store *BookmarkMaintainStore) CompleteScreenshotCleanup(ctx context.Context, cleanup ports.ScreenshotCleanup) error {
	_, err := store.pool.Exec(ctx, `UPDATE screenshot_cleanup SET state = $2, attempt_count = $3, next_attempt_at = $4, last_error_code = NULLIF($5, '') WHERE id = $1`, cleanup.ID.String(), cleanup.State, cleanup.AttemptCount, cleanup.NextAttemptAt, cleanup.LastErrorCode)
	if err != nil {
		return fmt.Errorf("complete screenshot cleanup: %w", err)
	}
	return nil
}
