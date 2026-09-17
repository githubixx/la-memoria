//go:build contract

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/githubixx/la-memoria/adapters/httpweb"
)

func TestSecurityMiddlewareSetsRequestIDAndNoStoreForSessionResponses(t *testing.T) {
	handler := httpweb.NewRouter(httpweb.Dependencies{}).Handler()
	request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/login", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Result().Header.Get("X-Request-ID") == "" {
		t.Fatal("login response must carry a generated request ID")
	}
	if response.Result().Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Result().Header.Get("Cache-Control"))
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie security attributes = %#v", cookie)
	}
	if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("session cookie must not persist, MaxAge=%d Expires=%s", cookie.MaxAge, cookie.Expires)
	}
}

func TestSecurityMiddlewareRejectsUntrustedForwardingCrossOriginCSRFAndDirectProtectedRequests(t *testing.T) {
	handler := httpweb.NewRouter(httpweb.Dependencies{TrustedProxyCIDRs: []string{"192.0.2.0/24"}}).Handler()

	protected := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/configuration", nil)
	protected.RemoteAddr = "198.51.100.7:443"
	protected.Header.Set("X-Forwarded-For", "203.0.113.8")
	protectedResponse := httptest.NewRecorder()
	handler.ServeHTTP(protectedResponse, protected)
	if protectedResponse.Code != http.StatusSeeOther || protectedResponse.Result().Header.Get("Location") != "/login?return_to=%2Fconfiguration" {
		t.Fatalf("direct protected request = %d %q, want local login redirect", protectedResponse.Code, protectedResponse.Result().Header.Get("Location"))
	}

	unsafe := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/logout", nil)
	unsafe.RemoteAddr = "198.51.100.7:443"
	unsafe.Header.Set("Origin", "https://attacker.test")
	unsafe.Header.Set("X-Forwarded-For", "203.0.113.8")
	unsafeResponse := httptest.NewRecorder()
	handler.ServeHTTP(unsafeResponse, unsafe)
	if unsafeResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request without CSRF token = %d, want 403", unsafeResponse.Code)
	}
	if httpweb.ClientAddress(unsafe) != "198.51.100.7" {
		t.Fatalf("untrusted forwarded source must be ignored, got %q", httpweb.ClientAddress(unsafe))
	}
}
