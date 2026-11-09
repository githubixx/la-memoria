//go:build contract

package contract_test

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/chromatheme"
	"github.com/githubixx/la-memoria/adapters/httpweb"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestBookmarkActionLinksUseThemeButtonsOnlyWhenAuthenticated(t *testing.T) {
	tmpl, err := template.ParseFiles("../../web/templates/fragments/bookmark_results.html")
	if err != nil {
		t.Fatal(err)
	}
	data := struct {
		Items         []model.Bookmark
		Authenticated bool
		IsCapped      bool
		Pagination    struct{ Links []string }
	}{
		Items: []model.Bookmark{{ID: "bookmark-1", URL: "https://example.test", CreatedAt: time.Now()}},
	}
	for _, authenticated := range []bool{false, true} {
		data.Authenticated = authenticated
		var output bytes.Buffer
		if err := tmpl.ExecuteTemplate(&output, "bookmark_results", data); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{
			`<a class="btn btn-secondary" href="/bookmarks/bookmark-1/edit">Edit</a>`,
			`<a class="btn btn-danger" href="/bookmarks/bookmark-1/delete">Delete</a>`,
		} {
			if strings.Contains(output.String(), action) != authenticated {
				t.Errorf("authenticated=%t: action %q presence mismatch", authenticated, action)
			}
		}
		if !authenticated && strings.Contains(output.String(), `class="bookmark-actions"`) {
			t.Error("anonymous users must not see bookmark actions")
		}
	}
}

type appearanceTestStore struct {
	configuration model.Configuration
	err           error
}

type screenshotLookupFunc func(context.Context, model.ID) (string, error)

func (lookup screenshotLookupFunc) ScreenshotStorageKey(ctx context.Context, id model.ID) (string, error) {
	return lookup(ctx, id)
}

func TestPublicScreenshotEndpoint(t *testing.T) {
	files := newScreenshotStore(t)
	staged := files.StagingPath("session")
	writePNG(t, staged)
	key, err := files.Promote(context.Background(), staged)
	if err != nil {
		t.Fatal(err)
	}
	lookup := screenshotLookupFunc(func(_ context.Context, id model.ID) (string, error) {
		switch id {
		case "shot-1":
			return key, nil
		case "failure":
			return "", errors.New("private database failure")
		default:
			return "", model.ErrScreenshotNotFound
		}
	})
	handler := httpweb.NewRouter(httpweb.Dependencies{Screenshots: usecase.NewScreenshotService(lookup, files)}).Handler()
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/screenshots/shot-1", http.StatusOK},
		{"/screenshots/missing", http.StatusNotFound},
		{"/screenshots/failure", http.StatusInternalServerError},
		{"/screenshots/..%2Fpromoted%2Fimage.png", http.StatusNotFound},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://bookmarker.test"+tc.path, nil))
		if response.Code != tc.want {
			t.Fatalf("%s = %d, want %d", tc.path, response.Code, tc.want)
		}
		if tc.want == http.StatusOK && (response.Header().Get("Content-Type") != "image/png" || response.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.HasPrefix(response.Body.Bytes(), []byte("\x89PNG"))) {
			t.Fatalf("invalid screenshot response headers/body: %v", response.Header())
		}
		if strings.Contains(response.Body.String(), key) || strings.Contains(response.Body.String(), "private database failure") {
			t.Fatal("screenshot response exposed private storage or error details")
		}
	}
	if err := files.Remove(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://bookmarker.test/screenshots/shot-1", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing image = %d, want 404", response.Code)
	}
}

func (store appearanceTestStore) Load(context.Context) (model.Configuration, error) {
	return store.configuration, store.err
}

func TestPublicShellUsesOnlyApprovedAppearance(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	valid := catalog.Themes()[0].ID
	for _, tc := range []struct {
		name, stored, expected string
		err                    error
	}{
		{"missing", "", "", nil},
		{"unavailable", "../../not-a-theme", "", nil},
		{"valid", valid, "/ui/themes/" + valid + "/theme.css", nil},
		{"read failure", "", "", errors.New("private-secret-do-not-log")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			store := appearanceTestStore{configuration: model.Configuration{Theme: tc.stored}, err: tc.err}
			appearance := usecase.NewAppearanceService(store, catalog, slog.New(slog.NewJSONHandler(&logs, nil)))
			bookmarks := &testkit.BookmarkStore{}
			router := httpweb.NewRouter(httpweb.Dependencies{
				TemplatesDir: "../../web/templates", Appearance: appearance,
				Browser: usecase.NewBrowser(usecase.BrowseDependencies{Bookmarks: bookmarks}),
			})
			response := httptest.NewRecorder()
			router.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/bookmarks", nil))
			body := response.Body.String()
			if response.Code != 200 || !strings.Contains(body, "/ui/css/base.css") {
				t.Fatalf("response = %d, body = %s", response.Code, body)
			}
			if tc.expected == "" && strings.Contains(body, "/ui/themes/") {
				t.Fatal("System shell must not render a theme stylesheet")
			}
			if tc.expected != "" && !strings.Contains(body, tc.expected) {
				t.Fatalf("missing approved theme URL %s", tc.expected)
			}
			if strings.Contains(body, tc.stored) && tc.stored == "../../not-a-theme" {
				t.Fatal("untrusted persisted value reflected into shell")
			}
			if tc.err != nil && (logs.Len() == 0 || strings.Contains(logs.String(), "private-secret-do-not-log")) {
				t.Fatalf("read failure logging not redacted: %s", logs.String())
			}
		})
	}
}

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

func TestPublicBookmarkResultsIncludeAvailableScreenshot(t *testing.T) {
	bookmarks := []model.Bookmark{
		{ID: "one", URL: "https://example.test/one", Description: "screenshot fixture", CreatedAt: time.Now().Add(time.Hour), Screenshot: &model.ScreenshotSummary{ID: "shot-1", Available: true}},
		{ID: "two", URL: "https://example.test/two", Description: "screenshot fixture", CreatedAt: time.Now(), Screenshot: &model.ScreenshotSummary{ID: "shot-2", Available: false}},
	}
	handler := newPublicRouter(t, bookmarks).Handler()
	for _, path := range []string{"/bookmarks", "/search?q=screenshot"} {
		for _, fragment := range []bool{false, true} {
			request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test"+path, nil)
			if fragment {
				request.Header.Set("HX-Request", "true")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `src="/screenshots/shot-1"`) || strings.Contains(response.Body.String(), `src="/screenshots/shot-2"`) {
				t.Fatalf("%s fragment=%t: status=%d body=%s", path, fragment, response.Code, response.Body.String())
			}
		}
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
