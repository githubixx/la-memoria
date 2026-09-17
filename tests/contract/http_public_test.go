//go:build contract

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/httpweb"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func newPublicRouter(t *testing.T, bookmarks []model.Bookmark) *httpweb.Router {
	t.Helper()
	store := &testkit.BookmarkStore{Bookmarks: bookmarks}
	return httpweb.NewRouter(httpweb.Dependencies{
		TemplatesDir: "../../web/templates",
		Browser:      usecase.NewBrowser(usecase.BrowseDependencies{Bookmarks: store}),
		Searcher:     usecase.NewSearcher(usecase.SearchDependencies{Bookmarks: store, MaximumResults: 1}),
	})
}

func TestRootRedirectsToTheConfiguredDefaultDestination(t *testing.T) {
	handler := newPublicRouter(t, nil).Handler()
	request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther || response.Result().Header.Get("Location") != "/bookmarks" {
		t.Fatalf("root response = %d %q, want 303 to /bookmarks", response.Code, response.Result().Header.Get("Location"))
	}
}

func TestBookmarksListRendersFieldOrderTagLinksAndExternalLinkSafety(t *testing.T) {
	bookmark := model.Bookmark{
		ID:          "1",
		URL:         "https://example.test/reference",
		Description: "Database transaction reference",
		CreatedAt:   time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		Tags:        model.NormalizeTags([]string{"Go"}),
	}
	handler := newPublicRouter(t, []model.Bookmark{bookmark}).Handler()
	request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/bookmarks", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("bookmarks response = %d, want 200", response.Code)
	}
	body := response.Body.String()
	urlIndex := strings.Index(body, "https://example.test/reference")
	descriptionIndex := strings.Index(body, "Database transaction reference")
	dateIndex := strings.Index(body, "2026-08-31")
	tagIndex := strings.Index(body, ">Go<")
	if urlIndex < 0 || descriptionIndex < urlIndex || dateIndex < descriptionIndex || tagIndex < dateIndex {
		t.Fatalf("field order in body = %q, want URL, description, date, then tags in order", body)
	}
	if !strings.Contains(body, `target="_blank"`) || !strings.Contains(body, `rel="noopener noreferrer"`) {
		t.Fatalf("external bookmark link missing safe target/rel attributes: %q", body)
	}
	if !strings.Contains(body, `tag=go`) {
		t.Fatalf("tag link missing normalized tag query: %q", body)
	}
}

func TestBookmarksEmptyCollectionShowsEmptyStateWithoutPagination(t *testing.T) {
	handler := newPublicRouter(t, nil).Handler()
	request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/bookmarks", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	body := response.Body.String()
	if !strings.Contains(body, "No bookmarks found") {
		t.Fatalf("empty collection body = %q, want an empty-state message", body)
	}
	if strings.Contains(body, `class="pagination"`) {
		t.Fatalf("empty collection body = %q, must not render pagination controls", body)
	}
}

func TestBookmarksFullPageAndFragmentParityAdvertiseVaryHXRequest(t *testing.T) {
	handler := newPublicRouter(t, nil).Handler()

	fullPageRequest := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/bookmarks", nil)
	fullPageResponse := httptest.NewRecorder()
	handler.ServeHTTP(fullPageResponse, fullPageRequest)
	if fullPageResponse.Result().Header.Get("Vary") != "HX-Request" {
		t.Fatalf("full-page response Vary header = %q, want HX-Request", fullPageResponse.Result().Header.Get("Vary"))
	}
	if !strings.Contains(fullPageResponse.Body.String(), "<html") {
		t.Fatalf("full-page response body = %q, want a complete HTML document", fullPageResponse.Body.String())
	}

	fragmentRequest := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/bookmarks", nil)
	fragmentRequest.Header.Set("HX-Request", "true")
	fragmentResponse := httptest.NewRecorder()
	handler.ServeHTTP(fragmentResponse, fragmentRequest)
	if fragmentResponse.Result().Header.Get("Vary") != "HX-Request" {
		t.Fatalf("fragment response Vary header = %q, want HX-Request", fragmentResponse.Result().Header.Get("Vary"))
	}
	if strings.Contains(fragmentResponse.Body.String(), "<html") {
		t.Fatalf("fragment response body = %q, must not include the full document shell", fragmentResponse.Body.String())
	}
}

func TestSearchRequiresExplicitCriteriaAndReportsCapNotice(t *testing.T) {
	base := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	bookmarks := []model.Bookmark{
		{ID: "1", URL: "https://example.test/1", Description: "database transactions", CreatedAt: base, Tags: model.NormalizeTags([]string{"go"})},
		{ID: "2", URL: "https://example.test/2", Description: "database transactions again", CreatedAt: base.Add(time.Hour), Tags: model.NormalizeTags([]string{"go"})},
	}
	handler := newPublicRouter(t, bookmarks).Handler()

	unloadedForm := httptest.NewRecorder()
	handler.ServeHTTP(unloadedForm, httptest.NewRequest(http.MethodGet, "https://bookmarker.test/search", nil))
	if unloadedForm.Code != http.StatusOK {
		t.Fatalf("unloaded search form = %d, want 200 without executing a search", unloadedForm.Code)
	}

	emptySubmission := httptest.NewRecorder()
	handler.ServeHTTP(emptySubmission, httptest.NewRequest(http.MethodGet, "https://bookmarker.test/search?q=", nil))
	if emptySubmission.Code != http.StatusUnprocessableEntity {
		t.Fatalf("explicit empty search = %d, want 422", emptySubmission.Code)
	}

	capped := httptest.NewRecorder()
	handler.ServeHTTP(capped, httptest.NewRequest(http.MethodGet, "https://bookmarker.test/search?tag=go&q=database+transactions", nil))
	if capped.Code != http.StatusOK {
		t.Fatalf("capped search = %d, want 200", capped.Code)
	}
	if !strings.Contains(capped.Body.String(), "cap-notice") {
		t.Fatalf("capped search body = %q, want a visible cap notice", capped.Body.String())
	}
}
