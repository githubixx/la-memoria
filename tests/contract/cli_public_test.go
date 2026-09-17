//go:build contract

package contract_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/jsonl"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func newPublicServer(bookmarks []model.Bookmark, maximumResults int) *jsonl.Server {
	store := &testkit.BookmarkStore{Bookmarks: bookmarks}
	return jsonl.NewServer(jsonl.Dependencies{
		Browser:  usecase.NewBrowser(usecase.BrowseDependencies{Bookmarks: store}),
		Searcher: usecase.NewSearcher(usecase.SearchDependencies{Bookmarks: store, MaximumResults: maximumResults}),
	})
}

func serveOne(t *testing.T, server *jsonl.Server, input string) string {
	t.Helper()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := server.Serve(strings.NewReader(input+"\n"), stdout, stderr); err != nil {
		t.Fatalf("serve JSON Lines: %v", err)
	}
	return strings.TrimSpace(stdout.String())
}

func TestCLIBookmarkListReturnsPaginatedPublicPayloadWithDuplicateURLs(t *testing.T) {
	base := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	bookmarks := []model.Bookmark{
		{ID: "1", URL: "https://example.test/dup", Description: "first", CreatedAt: base, Tags: model.NormalizeTags([]string{"go"})},
		{ID: "2", URL: "https://example.test/dup", Description: "second", CreatedAt: base.Add(time.Hour)},
	}
	server := newPublicServer(bookmarks, 10_000)
	stdout := serveOne(t, server, `{"version":"v1","id":"list-1","operation":"bookmark.list","payload":{"page":1}}`)

	var response struct {
		OK     bool `json:"ok"`
		Result struct {
			Page           int  `json:"page"`
			PageSize       int  `json:"page_size"`
			LastPage       int  `json:"last_page"`
			TotalWithinCap int  `json:"total_within_cap"`
			IsCapped       bool `json:"is_capped"`
			Items          []struct {
				URL  string `json:"url"`
				Tags []struct {
					Name string `json:"name"`
				} `json:"tags"`
			} `json:"items"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode response: %v; stdout=%q", err, stdout)
	}
	if !response.OK || response.Result.TotalWithinCap != 2 || len(response.Result.Items) != 2 {
		t.Fatalf("bookmark.list response = %#v, want both duplicate-URL bookmarks", response)
	}
	if response.Result.Items[0].URL != "https://example.test/dup" || response.Result.Items[1].URL != "https://example.test/dup" {
		t.Fatalf("bookmark.list items = %#v, want duplicate URLs preserved", response.Result.Items)
	}
}

func TestCLIBookmarkSearchValidatesAndReportsCap(t *testing.T) {
	base := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	bookmarks := []model.Bookmark{
		{ID: "1", URL: "https://example.test/1", Description: "database transactions", CreatedAt: base, Tags: model.NormalizeTags([]string{"go"})},
		{ID: "2", URL: "https://example.test/2", Description: "database transactions two", CreatedAt: base.Add(time.Hour), Tags: model.NormalizeTags([]string{"go"})},
	}
	server := newPublicServer(bookmarks, 1)

	invalidStdout := serveOne(t, server, `{"version":"v1","id":"search-invalid","operation":"bookmark.search","payload":{}}`)
	if !strings.Contains(invalidStdout, "search_criteria_required") {
		t.Fatalf("empty search response = %q, want search_criteria_required", invalidStdout)
	}

	cappedStdout := serveOne(t, server, `{"version":"v1","id":"search-1","operation":"bookmark.search","payload":{"tag":"go","words":["database","transactions"],"page":1}}`)
	var response struct {
		Result struct {
			TotalWithinCap int  `json:"total_within_cap"`
			IsCapped       bool `json:"is_capped"`
			Items          []struct {
				URL string `json:"url"`
			} `json:"items"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(cappedStdout), &response); err != nil {
		t.Fatalf("decode search response: %v; body=%q", err, cappedStdout)
	}
	if !response.Result.IsCapped || response.Result.TotalWithinCap != 1 || len(response.Result.Items) != 1 {
		t.Fatalf("bookmark.search response = %#v, want capped at 1", response.Result)
	}
}
