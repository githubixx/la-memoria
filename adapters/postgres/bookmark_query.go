package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

// BookmarkQueryStore implements ports.BookmarkStore against real PostgreSQL:
// newest-first ordering, exact normalized-tag filtering, language-neutral
// all-word description search, and cap-before-pagination.
type BookmarkQueryStore struct{ pool *pgxpool.Pool }

func NewBookmarkQueryStore(pool *pgxpool.Pool) *BookmarkQueryStore {
	return &BookmarkQueryStore{pool: pool}
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
	bookmarks, err := scanBookmarks(rows)
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
	bookmarks, err := scanBookmarks(rows)
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
	SELECT b.id, b.url, b.description, b.created_at, b.updated_at
	FROM bookmarks b
	WHERE ` + tagFilterCondition + `
	ORDER BY b.created_at DESC, b.id DESC
	LIMIT $2 OFFSET $3`

const searchCountQuery = `
	SELECT COUNT(*) FROM bookmarks b
	WHERE ` + tagFilterCondition + ` AND ` + searchFilterCondition

const searchPageQuery = `
	SELECT id, url, description, created_at, updated_at FROM (
		SELECT b.id, b.url, b.description, b.created_at, b.updated_at
		FROM bookmarks b
		WHERE ` + tagFilterCondition + ` AND ` + searchFilterCondition + `
		ORDER BY b.created_at DESC, b.id DESC
		LIMIT $4
	) capped
	ORDER BY created_at DESC, id DESC
	LIMIT $5 OFFSET $6`

func scanBookmarks(rows pgx.Rows) ([]model.Bookmark, error) {
	bookmarks := make([]model.Bookmark, 0)
	for rows.Next() {
		var (
			id                   string
			url, description     string
			createdAt, updatedAt time.Time
		)
		if err := rows.Scan(&id, &url, &description, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, model.Bookmark{
			ID:          model.ID(id),
			URL:         url,
			Description: description,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}
	return bookmarks, rows.Err()
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
