package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestSearchRequiresAtLeastOneCriterion(t *testing.T) {
	store := &testkit.BookmarkStore{}
	searcher := usecase.NewSearcher(usecase.SearchDependencies{Bookmarks: store})

	_, err := searcher.Search(context.Background(), model.SearchQuery{Page: 1})
	if model.ErrorCode(err) != "search_criteria_required" {
		t.Fatalf("empty search error = %v, want search_criteria_required", err)
	}
}

func TestSearchCombinesTagAndAllWordCriteriaWithANDAndReportsCap(t *testing.T) {
	base := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	store := &testkit.BookmarkStore{Bookmarks: []model.Bookmark{
		seededBookmark("1", "distributed database transactions", base, "go"),
		seededBookmark("2", "database transactions in Go", base.Add(time.Hour), "go"),
		seededBookmark("3", "unrelated content", base.Add(2*time.Hour), "go"),
		seededBookmark("4", "database transactions elsewhere", base.Add(3*time.Hour), "rust"),
	}}
	searcher := usecase.NewSearcher(usecase.SearchDependencies{Bookmarks: store, MaximumResults: 1})

	result, err := searcher.Search(context.Background(), model.SearchQuery{Tag: "go", Words: []string{"TRANSACTIONS", "database"}, Page: 1})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !result.IsCapped || result.TotalWithinCap != 1 {
		t.Fatalf("search result = %#v, want capped at 1 of the 2 matching go+database+transactions bookmarks", result)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "2" {
		t.Fatalf("search items = %#v, want the single newest capped match", result.Items)
	}
}
