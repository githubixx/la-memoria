package model

import (
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Tag is a normalized label shared by bookmarks. Comparison and uniqueness
// always use NormalizedName; DisplayName retains the first accepted spelling.
type Tag struct {
	DisplayName    string
	NormalizedName string
}

// NormalizeTagName trims surrounding whitespace and applies Unicode-aware
// case folding so tag comparison ignores case and whitespace.
func NormalizeTagName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// NewTag validates and normalizes one tag input, rejecting blank values.
func NewTag(raw string) (Tag, error) {
	trimmed := strings.TrimSpace(raw)
	normalized := NormalizeTagName(trimmed)
	if normalized == "" {
		return Tag{}, &ValidationError{Code: "tag_required", Field: "tag", Message: "tag must not be empty"}
	}
	return Tag{DisplayName: trimmed, NormalizedName: normalized}, nil
}

// NormalizeTags ignores empty entries, collapses normalized duplicates
// keeping the first display spelling, and returns tags in deterministic
// normalized-name order.
func NormalizeTags(raw []string) []Tag {
	seen := map[string]Tag{}
	order := make([]string, 0, len(raw))
	for _, value := range raw {
		tag, err := NewTag(value)
		if err != nil {
			continue
		}
		if _, exists := seen[tag.NormalizedName]; !exists {
			seen[tag.NormalizedName] = tag
			order = append(order, tag.NormalizedName)
		}
	}
	sort.Strings(order)
	tags := make([]Tag, 0, len(order))
	for _, normalizedName := range order {
		tags = append(tags, seen[normalizedName])
	}
	return tags
}

// ScreenshotSummary is the public view of a bookmark's screenshot state.
type ScreenshotSummary struct {
	ID        ID
	Available bool
}

// Bookmark is a saved web reference returned by browse and search use cases.
type Bookmark struct {
	ID          ID
	URL         string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Tags        []Tag
	Screenshot  *ScreenshotSummary
}

var ErrBookmarkNotFound = &ApplicationError{Code: "not_found", Message: "bookmark was not found"}

// ValidateBookmarkURL requires an absolute HTTP/HTTPS URL without embedded
// credentials.
func ValidateBookmarkURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", &ValidationError{Code: "invalid_url", Field: "url", Message: "url must be an absolute HTTP or HTTPS URL"}
	}
	if parsed.User != nil {
		return "", &ValidationError{Code: "invalid_url", Field: "url", Message: "url must not contain embedded credentials"}
	}
	return trimmed, nil
}

// ValidateDescription requires non-empty trimmed plain text.
func ValidateDescription(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", &ValidationError{Code: "description_required", Field: "description", Message: "description must not be empty"}
	}
	return trimmed, nil
}

// BrowseQuery selects one page of the public bookmark list, optionally
// filtered to an exact normalized tag.
type BrowseQuery struct {
	Tag  string
	Page int
}

// SearchQuery selects bookmarks by exact normalized tag and/or all-word
// description matching.
type SearchQuery struct {
	Tag   string
	Words []string
	Page  int
}

// SplitSearchWords parses free-text description input into the words that
// must all match, ignoring case and treating punctuation as a separator.
func SplitSearchWords(input string) []string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	words := make([]string, 0, len(fields))
	for _, field := range fields {
		words = append(words, strings.ToLower(field))
	}
	return words
}

// BookmarkPage is one browse result page.
type BookmarkPage struct {
	Items      []Bookmark
	Pagination Pagination
	Total      int
}

// SearchResult is one capped, paginated search result page.
type SearchResult struct {
	Items          []Bookmark
	Pagination     Pagination
	TotalWithinCap int
	IsCapped       bool
}
