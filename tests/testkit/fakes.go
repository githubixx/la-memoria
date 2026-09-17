package testkit

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func NewClock(now time.Time) *Clock {
	return &Clock{now: now.UTC()}
}

func (clock *Clock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *Clock) Advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
}

type IDGenerator struct {
	mu     sync.Mutex
	prefix string
	next   int
}

func NewIDGenerator(prefix string) *IDGenerator {
	return &IDGenerator{prefix: prefix}
}

func (generator *IDGenerator) NewID() string {
	generator.mu.Lock()
	defer generator.mu.Unlock()
	generator.next++
	return generator.prefix + "-" + itoa(generator.next)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

type PasswordVerifier struct {
	ExpectedHash     string
	ExpectedPassword string
	VerifyCalls      int
}

func (verifier *PasswordVerifier) Verify(_ context.Context, hash, password string) (bool, error) {
	verifier.VerifyCalls++
	return hash == verifier.ExpectedHash && password == verifier.ExpectedPassword, nil
}

type CaptureProcess struct {
	mu      sync.Mutex
	Calls   [][]string
	RunFunc func(ctx context.Context, arguments ...string) (ports.ProcessResult, error)
	Result  ports.ProcessResult
	Err     error
}

func (process *CaptureProcess) Run(ctx context.Context, arguments ...string) (ports.ProcessResult, error) {
	process.mu.Lock()
	process.Calls = append(process.Calls, append([]string(nil), arguments...))
	process.mu.Unlock()
	if process.RunFunc != nil {
		return process.RunFunc(ctx, arguments...)
	}
	return process.Result, process.Err
}

func (process *CaptureProcess) CallCount() int {
	process.mu.Lock()
	defer process.mu.Unlock()
	return len(process.Calls)
}

var ErrUnavailable = errors.New("test dependency unavailable")

type ValuePort[T any] struct {
	Value T
	Err   error
}

// BookmarkStore is an in-memory ports.BookmarkStore double: it applies the
// same newest-first ordering, exact-tag filtering, all-word matching,
// cap-before-pagination, and page-clamping semantics as the real
// PostgreSQL adapter, without a database.
type BookmarkStore struct {
	Bookmarks []model.Bookmark
}

func (store *BookmarkStore) BrowsePage(_ context.Context, normalizedTag string, page, pageSize int) ([]model.Bookmark, int, error) {
	matches := store.filter(normalizedTag, nil)
	return paginate(matches, page, pageSize), len(matches), nil
}

func (store *BookmarkStore) SearchPage(_ context.Context, normalizedTag string, words []string, page, pageSize, maximumResults int) ([]model.Bookmark, int, bool, error) {
	matches := store.filter(normalizedTag, words)
	totalWithinCap := len(matches)
	isCapped := false
	if maximumResults > 0 && totalWithinCap > maximumResults {
		matches = matches[:maximumResults]
		totalWithinCap = maximumResults
		isCapped = true
	}
	return paginate(matches, page, pageSize), totalWithinCap, isCapped, nil
}

func (store *BookmarkStore) filter(normalizedTag string, words []string) []model.Bookmark {
	sorted := append([]model.Bookmark(nil), store.Bookmarks...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.After(sorted[j].CreatedAt) })

	matches := make([]model.Bookmark, 0, len(sorted))
	for _, bookmark := range sorted {
		if normalizedTag != "" && !hasTag(bookmark, normalizedTag) {
			continue
		}
		if !matchesAllWords(bookmark, words) {
			continue
		}
		matches = append(matches, bookmark)
	}
	return matches
}

func hasTag(bookmark model.Bookmark, normalizedTag string) bool {
	for _, tag := range bookmark.Tags {
		if tag.NormalizedName == normalizedTag {
			return true
		}
	}
	return false
}

func matchesAllWords(bookmark model.Bookmark, words []string) bool {
	description := strings.ToLower(bookmark.Description)
	for _, word := range words {
		if !strings.Contains(description, strings.ToLower(word)) {
			return false
		}
	}
	return true
}

func paginate(items []model.Bookmark, page, pageSize int) []model.Bookmark {
	page = model.ClampPage(page, len(items), pageSize)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []model.Bookmark{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return append([]model.Bookmark(nil), items[start:end]...)
}

func (port *ValuePort[T]) Get(context.Context) (T, error) {
	return port.Value, port.Err
}

type CaptureDraftStore struct {
	mu     sync.Mutex
	drafts map[string]model.CaptureDraft
}

func NewCaptureDraftStore() *CaptureDraftStore {
	return &CaptureDraftStore{drafts: map[string]model.CaptureDraft{}}
}

func (store *CaptureDraftStore) CreateDraft(_ context.Context, draft model.CaptureDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.drafts[draft.ID.String()] = draft
	return nil
}

func (store *CaptureDraftStore) FindDraft(_ context.Context, id model.ID) (model.CaptureDraft, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	draft, ok := store.drafts[id.String()]
	if !ok {
		return model.CaptureDraft{}, model.ErrCaptureNotFound
	}
	return draft, nil
}

func (store *CaptureDraftStore) UpdateDraft(_ context.Context, draft model.CaptureDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.drafts[draft.ID.String()] = draft
	return nil
}

type ScreenshotCapturer struct {
	mu     sync.Mutex
	Calls  int
	Result ports.CaptureResult
	Err    error
}

func (capturer *ScreenshotCapturer) Capture(_ context.Context, _, _, stagedPath string) (ports.CaptureResult, error) {
	capturer.mu.Lock()
	defer capturer.mu.Unlock()
	capturer.Calls++
	if capturer.Err != nil {
		return ports.CaptureResult{}, capturer.Err
	}
	result := capturer.Result
	if result.StagedKey == "" {
		result.StagedKey = stagedPath
	}
	return result, nil
}

type ScreenshotFileStore struct {
	mu          sync.Mutex
	Staged      map[string]bool
	Promoted    map[string]bool
	Removed     map[string]bool
	Discarded   map[string]bool
	Quarantined map[string]bool
	Restored    map[string]bool
	ValidateErr error
	PromoteErr  error
	next        int
}

func NewScreenshotFileStore() *ScreenshotFileStore {
	return &ScreenshotFileStore{Staged: map[string]bool{}, Promoted: map[string]bool{}, Removed: map[string]bool{}, Discarded: map[string]bool{}, Quarantined: map[string]bool{}, Restored: map[string]bool{}}
}

func (store *ScreenshotFileStore) StagingPath(sessionID string) string {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.next++
	path := "/staging/" + sessionID + "-" + itoa(store.next) + ".png"
	store.Staged[path] = true
	return path
}

func (store *ScreenshotFileStore) Validate(_ context.Context, _ string) error {
	return store.ValidateErr
}

func (store *ScreenshotFileStore) Read(_ context.Context, storageKey string) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.Staged[storageKey] && !store.Promoted[storageKey] {
		return nil, errors.New("screenshot not found")
	}
	return []byte{0x89, 'P', 'N', 'G'}, nil
}

func (store *ScreenshotFileStore) Promote(_ context.Context, stagedPath string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.PromoteErr != nil {
		return "", store.PromoteErr
	}
	store.next++
	key := "/promoted/" + itoa(store.next) + ".png"
	store.Promoted[key] = true
	delete(store.Staged, stagedPath)
	return key, nil
}

func (store *ScreenshotFileStore) Discard(_ context.Context, stagedPath string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.Discarded[stagedPath] = true
	delete(store.Staged, stagedPath)
	return nil
}

func (store *ScreenshotFileStore) Remove(_ context.Context, storageKey string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.Removed[storageKey] = true
	delete(store.Promoted, storageKey)
	return nil
}

func (store *ScreenshotFileStore) Quarantine(_ context.Context, storageKey string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	quarantinedKey := "/quarantine/" + storageKey
	store.Quarantined[quarantinedKey] = true
	delete(store.Promoted, storageKey)
	return quarantinedKey, nil
}

func (store *ScreenshotFileStore) Restore(_ context.Context, quarantinedKey, storageKey string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.Restored[quarantinedKey] = true
	delete(store.Quarantined, quarantinedKey)
	store.Promoted[storageKey] = true
	return nil
}

type BookmarkWriter struct {
	mu      sync.Mutex
	Created []ports.NewBookmark
	Err     error
}

func (writer *BookmarkWriter) CreateBookmark(_ context.Context, bookmark ports.NewBookmark) error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.Err != nil {
		return writer.Err
	}
	writer.Created = append(writer.Created, bookmark)
	return nil
}
