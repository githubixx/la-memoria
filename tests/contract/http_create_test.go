//go:build contract

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/httpweb"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func newCreateRouter(t *testing.T) (*httpweb.Router, *testkit.BookmarkWriter) {
	t.Helper()
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	authenticator := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock:         clock,
		Passwords:     &testkit.PasswordVerifier{ExpectedHash: "hash", ExpectedPassword: "correct horse"},
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
	})
	capturer := &testkit.ScreenshotCapturer{}
	files := testkit.NewScreenshotFileStore()
	drafts := testkit.NewCaptureDraftStore()
	writer := &testkit.BookmarkWriter{}

	router := httpweb.NewRouter(httpweb.Dependencies{
		TemplatesDir:   "../../web/templates",
		Authenticator:  authenticator,
		CaptureService: usecase.NewCaptureService(usecase.CaptureDependencies{Capturer: capturer, Files: files, Drafts: drafts, Clock: clock}),
		Creator:        usecase.NewCreator(usecase.CreateDependencies{Bookmarks: writer, Drafts: drafts, Files: files, Clock: clock}),
	})
	return router, writer
}

// signIn performs the login handshake and returns the authenticated
// session cookie and rotated CSRF token for use in subsequent requests.
func signIn(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	loginPage := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/login", nil)
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginPage)
	anonymousCookie := loginResponse.Result().Cookies()[0]
	body := loginResponse.Body.String()
	csrfIndex := strings.Index(body, `name="csrf_token" value="`)
	if csrfIndex < 0 {
		t.Fatalf("login page missing csrf token: %q", body)
	}
	csrfStart := csrfIndex + len(`name="csrf_token" value="`)
	csrfToken := body[csrfStart : csrfStart+strings.Index(body[csrfStart:], `"`)]

	form := url.Values{"username": {"admin"}, "password": {"correct horse"}, "csrf_token": {csrfToken}}
	loginRequest := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/login", strings.NewReader(form.Encode()))
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRequest.Header.Set("Origin", "https://bookmarker.test")
	loginRequest.AddCookie(anonymousCookie)
	loginPostResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginPostResponse, loginRequest)
	if loginPostResponse.Code != http.StatusSeeOther {
		t.Fatalf("login response = %d, want 303: %s", loginPostResponse.Code, loginPostResponse.Body.String())
	}
	authenticatedCookie := loginPostResponse.Result().Cookies()[0]

	addPage := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/bookmarks/new", nil)
	addPage.AddCookie(authenticatedCookie)
	addResponse := httptest.NewRecorder()
	handler.ServeHTTP(addResponse, addPage)
	addBody := addResponse.Body.String()
	addCSRFIndex := strings.Index(addBody, `name="csrf_token" value="`)
	if addCSRFIndex < 0 {
		t.Fatalf("add page missing csrf token: %q", addBody)
	}
	addCSRFStart := addCSRFIndex + len(`name="csrf_token" value="`)
	addCSRFToken := addBody[addCSRFStart : addCSRFStart+strings.Index(addBody[addCSRFStart:], `"`)]
	return authenticatedCookie, addCSRFToken
}

func TestHTTPCreateRequiresAuthenticationForProtectedRoutes(t *testing.T) {
	router, _ := newCreateRouter(t)
	handler := router.Handler()

	for _, target := range []string{"/bookmarks/new"} {
		request := httptest.NewRequest(http.MethodGet, "https://bookmarker.test"+target, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther || !strings.Contains(response.Result().Header.Get("Location"), "/login") {
			t.Fatalf("unauthenticated GET %s = %d %q, want redirect to login", target, response.Code, response.Result().Header.Get("Location"))
		}
	}
}

func TestHTTPCreateSignInThenCaptureLifecycleAndBookmarkCreation(t *testing.T) {
	router, writer := newCreateRouter(t)
	handler := router.Handler()
	cookie, csrfToken := signIn(t, handler)

	form := url.Values{"url": {"https://example.test/target"}, "csrf_token": {csrfToken}}
	captureRequest := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/captures", strings.NewReader(form.Encode()))
	captureRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	captureRequest.Header.Set("Origin", "https://bookmarker.test")
	captureRequest.AddCookie(cookie)
	captureResponse := httptest.NewRecorder()
	handler.ServeHTTP(captureResponse, captureRequest)
	if captureResponse.Code != http.StatusCreated {
		t.Fatalf("capture start = %d, want 201: %s", captureResponse.Code, captureResponse.Body.String())
	}
	body := captureResponse.Body.String()
	idIndex := strings.Index(body, `name="capture_id" value="`)
	if idIndex < 0 {
		t.Fatalf("capture fragment missing capture_id: %q", body)
	}
	idStart := idIndex + len(`name="capture_id" value="`)
	captureID := body[idStart : idStart+strings.Index(body[idStart:], `"`)]
	if !strings.Contains(body, `data-state="ready"`) {
		t.Fatalf("capture fragment = %q, want ready state", body)
	}
	previewRequest := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/captures/"+captureID+"/preview", nil)
	previewRequest.AddCookie(cookie)
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || previewResponse.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(previewResponse.Body.String(), "\x89PNG") {
		t.Fatalf("capture preview = %d %q %q, want PNG image", previewResponse.Code, previewResponse.Header().Get("Content-Type"), previewResponse.Body.String())
	}

	createForm := url.Values{
		"url": {"https://example.test/target"}, "description": {"A target page"},
		"tags": {"Go", "go"}, "capture_id": {captureID}, "csrf_token": {csrfToken},
	}
	createRequest := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/bookmarks", strings.NewReader(createForm.Encode()))
	createRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRequest.Header.Set("Origin", "https://bookmarker.test")
	createRequest.AddCookie(cookie)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusSeeOther {
		t.Fatalf("bookmark create = %d, want 303: %s", createResponse.Code, createResponse.Body.String())
	}
	if len(writer.Created) != 1 || writer.Created[0].ScreenshotStorageKey == "" {
		t.Fatalf("created bookmarks = %#v, want one bookmark with a promoted screenshot", writer.Created)
	}
	if len(writer.Created[0].Tags) != 1 {
		t.Fatalf("created tags = %#v, want normalized duplicates collapsed", writer.Created[0].Tags)
	}
}

func TestHTTPCreateRejectsMissingOrInvalidCSRFToken(t *testing.T) {
	router, _ := newCreateRouter(t)
	handler := router.Handler()
	cookie, _ := signIn(t, handler)

	form := url.Values{"url": {"https://example.test/target"}, "csrf_token": {"wrong-token"}}
	request := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/captures", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://bookmarker.test")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("invalid CSRF token response = %d, want 403", response.Code)
	}
}

func TestHTTPCreatePreservesValidationOnInvalidSubmission(t *testing.T) {
	router, _ := newCreateRouter(t)
	handler := router.Handler()
	cookie, csrfToken := signIn(t, handler)

	form := url.Values{"url": {"not a url"}, "description": {"x"}, "save_without_screenshot": {"true"}, "csrf_token": {csrfToken}}
	request := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/bookmarks", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://bookmarker.test")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid bookmark submission = %d, want 422: %s", response.Code, response.Body.String())
	}
}
