//go:build contract

package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/httpweb"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

type maintainContractStore struct {
	bookmark ports.StoredBookmark
	updated  ports.UpdatedBookmark
}

func (store *maintainContractStore) FindBookmark(_ context.Context, id model.ID) (ports.StoredBookmark, error) {
	if id != store.bookmark.ID {
		return ports.StoredBookmark{}, model.ErrBookmarkNotFound
	}
	return store.bookmark, nil
}

func (store *maintainContractStore) UpdateBookmark(_ context.Context, bookmark ports.UpdatedBookmark) error {
	store.updated = bookmark
	return nil
}

func (store *maintainContractStore) DeleteBookmark(context.Context, model.ID) error { return nil }
func (store *maintainContractStore) CreateScreenshotCleanup(context.Context, ports.ScreenshotCleanup) error {
	return nil
}

func TestHTTPMaintainRendersAndReplacesTags(t *testing.T) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	store := &maintainContractStore{bookmark: ports.StoredBookmark{Bookmark: model.Bookmark{
		ID: "bookmark-1", URL: "https://example.test", Description: "original",
		Tags: model.NormalizeTags([]string{"Go", `A"<tag>`}),
	}}}
	authenticator := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock: clock, Passwords: &testkit.PasswordVerifier{ExpectedHash: "hash", ExpectedPassword: "correct horse"},
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
	})
	router := httpweb.NewRouter(httpweb.Dependencies{
		TemplatesDir: "../../web/templates", Authenticator: authenticator,
		Maintainer: usecase.NewMaintainer(usecase.MaintainDependencies{Bookmarks: store, Clock: clock}),
	})
	handler := router.Handler()
	cookie, csrf := signIn(t, handler)
	request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/bookmarks/bookmark-1/edit", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("edit response = %d: %s", response.Code, response.Body.String())
	}
	for _, field := range []string{`name="tags" value="Go"`, `name="tags" value="A&#34;&lt;tag&gt;"`, `data-tag-remove`} {
		if !strings.Contains(response.Body.String(), field) {
			t.Errorf("edit form missing %q", field)
		}
	}

	for _, tc := range []struct {
		name string
		tags []string
	}{
		{name: "multiple", tags: []string{"Go", "Rust"}},
		{name: "clear all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{"url": {"https://example.test"}, "description": {"updated"}, "csrf_token": {csrf}, "tags": tc.tags}
			request := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/bookmarks/bookmark-1", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Origin", "https://bookmarker.test")
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusSeeOther {
				t.Fatalf("update response = %d: %s", response.Code, response.Body.String())
			}
			if want := model.NormalizeTags(tc.tags); !reflect.DeepEqual(store.updated.Tags, want) {
				t.Fatalf("updated tags = %#v, want %#v", store.updated.Tags, want)
			}
		})
	}
}

func TestHTTPMaintainDirectEditAndDeleteAreProtected(t *testing.T) {
	router, _ := newCreateRouter(t)
	for _, target := range []string{"/bookmarks/bookmark-1/edit", "/bookmarks/bookmark-1/delete"} {
		request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test"+target, nil)
		response := httptest.NewRecorder()
		router.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther || !strings.Contains(response.Result().Header.Get("Location"), "/login") {
			t.Fatalf("unauthenticated GET %s = %d location=%q, want login redirect", target, response.Code, response.Result().Header.Get("Location"))
		}
	}
}
