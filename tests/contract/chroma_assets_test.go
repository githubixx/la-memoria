//go:build contract

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/githubixx/la-memoria/adapters/chromatheme"
	"github.com/githubixx/la-memoria/adapters/httpweb"
)

func TestChromaAssetsAreAllowlistedAndRevalidate(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}

	handler := httpweb.NewRouter(httpweb.Dependencies{ThemeAssets: catalog.Handler(nil)}).Handler()
	for _, path := range []string{"/ui/css/base.css", "/ui/themes/" + catalog.Themes()[0].ID + "/theme.css"} {
		t.Run(path, func(t *testing.T) {
			get := httptest.NewRecorder()
			handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, path, nil))
			if get.Code != 200 || !strings.HasPrefix(get.Header().Get("Content-Type"), "text/css") || get.Header().Get("ETag") == "" || get.Header().Get("Cache-Control") != "no-cache" {
				t.Fatalf("GET = %d, headers = %v", get.Code, get.Header())
			}
			head := httptest.NewRecorder()
			handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, path, nil))
			if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("ETag") != get.Header().Get("ETag") {
				t.Fatalf("HEAD = %d, headers = %v", head.Code, head.Header())
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("If-None-Match", get.Header().Get("ETag"))
			notModified := httptest.NewRecorder()
			handler.ServeHTTP(notModified, req)
			if notModified.Code != http.StatusNotModified {
				t.Fatalf("revalidation = %d", notModified.Code)
			}
		})
	}
	for _, path := range []string{"/ui/", "/ui/themes/", "/ui/themes/unknown/theme.css", "/ui/themes/" + catalog.Themes()[0].ID + "/theme.yaml", "/ui/css/../theme.yaml"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/css/base.css", nil)
	req.Header.Set("Origin", "http://example.com")
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST = %d, Allow=%q", response.Code, response.Header().Get("Allow"))
	}
}

func TestChroma030ThemesAreAvailable(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	handler := httpweb.NewRouter(httpweb.Dependencies{ThemeAssets: catalog.Handler(nil)}).Handler()
	for _, id := range []string{"leather-style-black-1", "leather-style-red-1"} {
		t.Run(id, func(t *testing.T) {
			if !catalog.Contains(id) {
				t.Fatalf("theme %q is missing from the catalog", id)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/themes/"+id+"/theme.css", nil))
			if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/css") {
				t.Fatalf("theme stylesheet = %d, headers = %v", response.Code, response.Header())
			}
		})
	}
}
