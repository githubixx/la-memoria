package ports

import (
	"context"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

// BookmarkStore is the use-case-specific boundary for reading public
// bookmark data. BrowsePage and SearchPage each clamp the requested page
// internally and report the total matching count so callers can build
// consistent pagination.
type BookmarkStore interface {
	// BrowsePage returns one newest-first page, optionally filtered to an
	// exact normalized tag, plus the total number of matching bookmarks.
	BrowsePage(ctx context.Context, normalizedTag string, page, pageSize int) ([]model.Bookmark, int, error)

	// SearchPage returns one newest-first, capped page matching the exact
	// normalized tag and/or all-word description criteria, plus the total
	// number of matches found within maximumResults.
	SearchPage(ctx context.Context, normalizedTag string, words []string, page, pageSize, maximumResults int) (items []model.Bookmark, totalWithinCap int, isCapped bool, err error)
}

// NewBookmark describes a bookmark, its tags, and its optional promoted
// screenshot to be written transactionally. Duplicate URLs are always
// permitted; no uniqueness is enforced.
type NewBookmark struct {
	ID                   model.ID
	URL                  string
	Description          string
	Tags                 []model.Tag
	ScreenshotStorageKey string
	CapturedURL          string
	Now                  time.Time
}

// BookmarkWriter is the use-case-specific boundary for creating bookmarks
// with their tags and optional screenshot metadata in one transaction.
type BookmarkWriter interface {
	CreateBookmark(ctx context.Context, bookmark NewBookmark) error
}

// StoredBookmark includes the private storage key needed to safely replace
// or remove a bookmark screenshot.
type StoredBookmark struct {
	model.Bookmark
	ScreenshotStorageKey string
}

// UpdatedBookmark describes an atomic replacement of editable bookmark
// fields. CreatedAt is intentionally absent and must remain unchanged.
type UpdatedBookmark struct {
	ID                   model.ID
	URL                  string
	Description          string
	Tags                 []model.Tag
	ScreenshotStorageKey string
	CapturedURL          string
	ReplaceScreenshot    bool
	UpdatedAt            time.Time
}

// ScreenshotCleanup records an after-commit file operation that requires a
// bounded retry without changing the completed database transition.
type ScreenshotCleanup struct {
	ID             model.ID
	ScreenshotID   model.ID
	QuarantinedKey string
	Operation      string
	AttemptCount   int
	NextAttemptAt  time.Time
	State          string
	LastErrorCode  string
}

// ScreenshotCleanupStore claims due records and records their final state.
// Claiming is atomic so concurrent workers cannot process the same record.
type ScreenshotCleanupStore interface {
	ClaimDueScreenshotCleanup(ctx context.Context, now time.Time, limit int) ([]ScreenshotCleanup, error)
	CompleteScreenshotCleanup(ctx context.Context, cleanup ScreenshotCleanup) error
}

// BookmarkMaintainer is the transactional persistence boundary for bookmark
// maintenance and durable screenshot cleanup records.
type BookmarkMaintainer interface {
	FindBookmark(ctx context.Context, id model.ID) (StoredBookmark, error)
	UpdateBookmark(ctx context.Context, bookmark UpdatedBookmark) error
	DeleteBookmark(ctx context.Context, id model.ID) error
	CreateScreenshotCleanup(ctx context.Context, cleanup ScreenshotCleanup) error
}
