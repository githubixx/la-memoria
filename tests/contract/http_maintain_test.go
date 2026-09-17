//go:build contract

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
