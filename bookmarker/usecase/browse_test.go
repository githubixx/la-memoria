package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func seededBookmark(id, description string, createdAt time.Time, tags ...string) model.Bookmark {
	return model.Bookmark{
		ID:          model.ID(id),
		URL:         "https://example.test/" + id,
		Description: description,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		Tags:        model.NormalizeTags(tags),
	}
}

func TestBrowseIsPublicNewestFirstAndSupportsExactTagFiltering(t *testing.T) {
	base := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	store := &testkit.BookmarkStore{Bookmarks: []model.Bookmark{
		seededBookmark("1", "oldest", base, "go"),
		seededBookmark("2", "middle", base.Add(time.Hour), "rust"),
		seededBookmark("3", "newest", base.Add(2*time.Hour), "Go"),
	}}
	browser := usecase.NewBrowser(usecase.BrowseDependencies{Bookmarks: store})

	page, err := browser.Browse(context.Background(), model.BrowseQuery{Page: 1})
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(page.Items) != 3 || page.Items[0].ID != "3" || page.Items[2].ID != "1" {
		t.Fatalf("browse items = %#v, want newest-first order", page.Items)
	}

	filtered, err := browser.Browse(context.Background(), model.BrowseQuery{Tag: "  GO  ", Page: 1})
	if err != nil {
		t.Fatalf("browse filtered: %v", err)
	}
	if len(filtered.Items) != 2 {
		t.Fatalf("tag-filtered items = %#v, want exactly the 2 bookmarks tagged go", filtered.Items)
	}
}

func TestBrowseClampsPagesAndReportsStablePagination(t *testing.T) {
	store := &testkit.BookmarkStore{}
	browser := usecase.NewBrowser(usecase.BrowseDependencies{Bookmarks: store})

	page, err := browser.Browse(context.Background(), model.BrowseQuery{Page: 5})
	if err != nil {
		t.Fatalf("browse empty collection: %v", err)
	}
	if len(page.Items) != 0 || page.Pagination.Page != 1 || len(page.Pagination.Links) != 0 {
		t.Fatalf("empty browse page = %#v, want page 1 with no pagination links", page)
	}
}
