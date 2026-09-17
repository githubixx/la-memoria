package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

// CaptureDraftStore implements ports.CaptureDraftStore against the
// capture_drafts table so drafts survive across requests within a
// browser session.
type CaptureDraftStore struct{ pool *pgxpool.Pool }

func NewCaptureDraftStore(pool *pgxpool.Pool) *CaptureDraftStore {
	return &CaptureDraftStore{pool: pool}
}

func (store *CaptureDraftStore) CreateDraft(ctx context.Context, draft model.CaptureDraft) error {
	var bookmarkID *string
	if draft.BookmarkID != nil {
		value := draft.BookmarkID.String()
		bookmarkID = &value
	}
	_, err := store.pool.Exec(ctx, `
		INSERT INTO capture_drafts (id, owner_session_id, bookmark_id, target_url, state, staged_key, final_url, failure_code, failure_message, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10, $11)`,
		draft.ID.String(), draft.OwnerSessionID.String(), bookmarkID, draft.TargetURL, string(draft.State),
		draft.StagedKey, draft.FinalURL, draft.FailureCode, draft.FailureMessage, draft.CreatedAt, draft.ExpiresAt)
	return err
}

func (store *CaptureDraftStore) FindDraft(ctx context.Context, id model.ID) (model.CaptureDraft, error) {
	var (
		draftID, ownerSessionID, targetURL, state string
		bookmarkID                                *string
		stagedKey, finalURL, failureCode          *string
		failureMessage                            *string
		createdAt, expiresAt                      time.Time
	)
	err := store.pool.QueryRow(ctx, `
		SELECT id, owner_session_id, bookmark_id, target_url, state, staged_key, final_url, failure_code, failure_message, created_at, expires_at
		FROM capture_drafts WHERE id = $1`, id.String()).Scan(
		&draftID, &ownerSessionID, &bookmarkID, &targetURL, &state,
		&stagedKey, &finalURL, &failureCode, &failureMessage, &createdAt, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CaptureDraft{}, model.ErrCaptureNotFound
	}
	if err != nil {
		return model.CaptureDraft{}, err
	}
	draft := model.CaptureDraft{
		ID:             model.ID(draftID),
		OwnerSessionID: model.ID(ownerSessionID),
		TargetURL:      targetURL,
		State:          model.CaptureState(state),
		CreatedAt:      createdAt,
		ExpiresAt:      expiresAt,
	}
	if bookmarkID != nil {
		value := model.ID(*bookmarkID)
		draft.BookmarkID = &value
	}
	if stagedKey != nil {
		draft.StagedKey = *stagedKey
	}
	if finalURL != nil {
		draft.FinalURL = *finalURL
	}
	if failureCode != nil {
		draft.FailureCode = *failureCode
	}
	if failureMessage != nil {
		draft.FailureMessage = *failureMessage
	}
	return draft, nil
}

func (store *CaptureDraftStore) UpdateDraft(ctx context.Context, draft model.CaptureDraft) error {
	tag, err := store.pool.Exec(ctx, `
		UPDATE capture_drafts SET
			state = $2,
			staged_key = NULLIF($3, ''),
			final_url = NULLIF($4, ''),
			failure_code = NULLIF($5, ''),
			failure_message = NULLIF($6, '')
		WHERE id = $1`,
		draft.ID.String(), string(draft.State), draft.StagedKey, draft.FinalURL, draft.FailureCode, draft.FailureMessage)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrCaptureNotFound
	}
	return nil
}
