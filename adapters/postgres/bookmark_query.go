package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// BookmarkQueryStore implements ports.BookmarkStore against real PostgreSQL:
// newest-first ordering, exact normalized-tag filtering, language-neutral
// all-word description search, and cap-before-pagination.
type BookmarkQueryStore struct {
	pool  *pgxpool.Pool
	files ports.ScreenshotAvailability
}

// Without a file availability checker, screenshot references are returned as
// unavailable rather than assuming that their files exist.
func NewBookmarkQueryStore(pool *pgxpool.Pool) *BookmarkQueryStore {
	return &BookmarkQueryStore{pool: pool}
}

func NewBookmarkQueryStoreWithScreenshots(pool *pgxpool.Pool, files ports.ScreenshotAvailability) *BookmarkQueryStore {
	return &BookmarkQueryStore{pool: pool, files: files}
}

func (store *BookmarkQueryStore) BrowsePage(ctx context.Context, normalizedTag string, page, pageSize int) ([]model.Bookmark, int, error) {
	var total int
	if err := store.pool.QueryRow(ctx, browseCountQuery, normalizedTag).Scan(&total); err != nil {
		return nil, 0, err
	}
	clampedPage := model.ClampPage(page, total, pageSize)
	offset := (clampedPage - 1) * pageSize

	rows, err := store.pool.Query(ctx, browsePageQuery, normalizedTag, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	bookmarks, err := store.scanBookmarks(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	if err := store.hydrateTags(ctx, bookmarks); err != nil {
		return nil, 0, err
	}
	return bookmarks, total, nil
}

func (store *BookmarkQueryStore) SearchPage(ctx context.Context, normalizedTag string, words []string, page, pageSize, maximumResults int) ([]model.Bookmark, int, bool, error) {
	hasWords := len(words) > 0
	tsQuery := strings.Join(words, " ")

	var actualMatches int
	if err := store.pool.QueryRow(ctx, searchCountQuery, normalizedTag, hasWords, tsQuery).Scan(&actualMatches); err != nil {
		return nil, 0, false, err
	}
	totalWithinCap := actualMatches
	isCapped := false
	if maximumResults > 0 && actualMatches > maximumResults {
		totalWithinCap = maximumResults
		isCapped = true
	}

	clampedPage := model.ClampPage(page, totalWithinCap, pageSize)
	offset := (clampedPage - 1) * pageSize

	rows, err := store.pool.Query(ctx, searchPageQuery, normalizedTag, hasWords, tsQuery, maximumResults, pageSize, offset)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	bookmarks, err := store.scanBookmarks(ctx, rows)
	if err != nil {
		return nil, 0, false, err
	}
	if err := store.hydrateTags(ctx, bookmarks); err != nil {
		return nil, 0, false, err
	}
	return bookmarks, totalWithinCap, isCapped, nil
}

const tagFilterCondition = `(
	$1 = '' OR EXISTS (
		SELECT 1 FROM bookmark_tags bt JOIN tags t ON t.id = bt.tag_id
		WHERE bt.bookmark_id = b.id AND t.normalized_name = $1
	)
)`

const searchFilterCondition = `(
	$2 = false OR to_tsvector('simple', b.description) @@ plainto_tsquery('simple', $3)
)`

const browseCountQuery = `SELECT COUNT(*) FROM bookmarks b WHERE ` + tagFilterCondition

const browsePageQuery = `
	SELECT b.id, b.url, b.description, b.created_at, b.updated_at, s.id, s.storage_key
	FROM bookmarks b
	LEFT JOIN screenshots s ON s.bookmark_id = b.id
	WHERE ` + tagFilterCondition + `
	ORDER BY b.created_at DESC, b.id DESC
	LIMIT $2 OFFSET $3`

const searchCountQuery = `
	SELECT COUNT(*) FROM bookmarks b
	WHERE ` + tagFilterCondition + ` AND ` + searchFilterCondition

const searchPageQuery = `
	SELECT capped.id, capped.url, capped.description, capped.created_at, capped.updated_at, s.id, s.storage_key FROM (
		SELECT b.id, b.url, b.description, b.created_at, b.updated_at
		FROM bookmarks b
		WHERE ` + tagFilterCondition + ` AND ` + searchFilterCondition + `
		ORDER BY b.created_at DESC, b.id DESC
		LIMIT $4
	) capped
	LEFT JOIN screenshots s ON s.bookmark_id = capped.id
	ORDER BY capped.created_at DESC, capped.id DESC
	LIMIT $5 OFFSET $6`

func (store *BookmarkQueryStore) scanBookmarks(ctx context.Context, rows pgx.Rows) ([]model.Bookmark, error) {
	bookmarks := make([]model.Bookmark, 0)
	for rows.Next() {
		var (
			id                   string
			url, description     string
			createdAt, updatedAt time.Time
			screenshotID         *string
			screenshotKey        *string
		)
		if err := rows.Scan(&id, &url, &description, &createdAt, &updatedAt, &screenshotID, &screenshotKey); err != nil {
			return nil, err
		}
		bookmark := model.Bookmark{
			ID:          model.ID(id),
			URL:         url,
			Description: description,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		}
		if screenshotID != nil {
			available := false
			if store.files != nil && screenshotKey != nil {
				var err error
				available, err = store.files.Available(ctx, *screenshotKey)
				if err != nil {
					return nil, err
				}
			}
			bookmark.Screenshot = &model.ScreenshotSummary{ID: model.ID(*screenshotID), Available: available}
		}
		bookmarks = append(bookmarks, bookmark)
	}

	return bookmarks, rows.Err()
}

func (store *BookmarkQueryStore) ScreenshotStorageKey(ctx context.Context, id model.ID) (string, error) {
	var key string
	err := store.pool.QueryRow(ctx, `SELECT storage_key FROM screenshots WHERE id = $1`, id.String()).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", model.ErrScreenshotNotFound
	}
	return key, err
}

func (store *BookmarkQueryStore) hydrateTags(ctx context.Context, bookmarks []model.Bookmark) error {
	if len(bookmarks) == 0 {
		return nil
	}
	ids := make([]string, len(bookmarks))
	index := map[string]int{}
	for position, bookmark := range bookmarks {
		ids[position] = bookmark.ID.String()
		index[bookmark.ID.String()] = position
	}
	rows, err := store.pool.Query(ctx, `
		SELECT bt.bookmark_id, t.display_name, t.normalized_name
		FROM bookmark_tags bt JOIN tags t ON t.id = bt.tag_id
		WHERE bt.bookmark_id = ANY($1)
		ORDER BY t.normalized_name`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var bookmarkID, displayName, normalizedName string
		if err := rows.Scan(&bookmarkID, &displayName, &normalizedName); err != nil {
			return err
		}
		position := index[bookmarkID]
		bookmarks[position].Tags = append(bookmarks[position].Tags, model.Tag{DisplayName: displayName, NormalizedName: normalizedName})
	}
	return rows.Err()
}
