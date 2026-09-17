package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RetentionStore struct{ pool *pgxpool.Pool }

func NewRetentionStore(pool *pgxpool.Pool) *RetentionStore { return &RetentionStore{pool: pool} }

func (store *RetentionStore) ExpireCaptureDrafts(ctx context.Context, before time.Time, limit int) error {
	_, err := store.pool.Exec(ctx, `
		WITH due AS (
			SELECT id FROM capture_drafts
			WHERE expires_at <= $1 AND state IN ('pending', 'capturing', 'ready', 'failed')
			ORDER BY expires_at FOR UPDATE SKIP LOCKED LIMIT $2
		)
		UPDATE capture_drafts draft SET state = 'expired' FROM due WHERE draft.id = due.id`, before, limit)
	if err != nil {
		return fmt.Errorf("expire capture drafts: %w", err)
	}
	return nil
}

func (store *RetentionStore) DeleteExpiredSessions(ctx context.Context, before time.Time, limit int) error {
	_, err := store.pool.Exec(ctx, `
		WITH due AS (
			SELECT id FROM web_sessions WHERE created_at <= $1 ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $2
		)
		DELETE FROM web_sessions session USING due WHERE session.id = due.id`, before, limit)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

func (store *RetentionStore) DeleteExpiredThrottlePairs(ctx context.Context, before time.Time, limit int) error {
	_, err := store.pool.Exec(ctx, `
		WITH due AS (
			SELECT username_key, source_address FROM login_throttle_pairs
			WHERE updated_at <= $1 ORDER BY updated_at FOR UPDATE SKIP LOCKED LIMIT $2
		)
		DELETE FROM login_throttle_pairs pair USING due
		WHERE pair.username_key = due.username_key AND pair.source_address = due.source_address`, before, limit)
	if err != nil {
		return fmt.Errorf("delete expired throttle pairs: %w", err)
	}
	return nil
}

func (store *RetentionStore) ClaimDueScreenshotCleanup(ctx context.Context, now time.Time, limit int) ([]ports.ScreenshotCleanup, error) {
	rows, err := store.pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM screenshot_cleanup WHERE state = 'pending' AND next_attempt_at <= $1
			ORDER BY next_attempt_at FOR UPDATE SKIP LOCKED LIMIT $2
		)
		UPDATE screenshot_cleanup cleanup SET state = 'running' FROM due WHERE cleanup.id = due.id
		RETURNING cleanup.id, cleanup.screenshot_id, cleanup.quarantined_key, cleanup.operation, cleanup.attempt_count, cleanup.next_attempt_at`, now, limit)
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

func (store *RetentionStore) CompleteScreenshotCleanup(ctx context.Context, cleanup ports.ScreenshotCleanup) error {
	_, err := store.pool.Exec(ctx, `UPDATE screenshot_cleanup SET state = $2, attempt_count = $3, next_attempt_at = $4, last_error_code = NULLIF($5, '') WHERE id = $1`, cleanup.ID.String(), cleanup.State, cleanup.AttemptCount, cleanup.NextAttemptAt, cleanup.LastErrorCode)
	if err != nil {
		return fmt.Errorf("complete screenshot cleanup: %w", err)
	}
	return nil
}
