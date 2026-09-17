//go:build contract

package contract_test

import (
	"context"
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

func TestConfigurationRoutesProtectAdministrationAndServeAFaviconFallback(t *testing.T) {
	handler := httpweb.NewRouter(httpweb.Dependencies{}).Handler()

	protected := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/configuration", nil)
	protectedResponse := httptest.NewRecorder()
	handler.ServeHTTP(protectedResponse, protected)
	if protectedResponse.Code != http.StatusSeeOther || protectedResponse.Header().Get("Location") != "/login?return_to=%2Fconfiguration" {
		t.Fatalf("configuration response = %d %q", protectedResponse.Code, protectedResponse.Header().Get("Location"))
	}

	favicon := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/favicon", nil)
	faviconResponse := httptest.NewRecorder()
	handler.ServeHTTP(faviconResponse, favicon)
	if faviconResponse.Code != http.StatusOK || faviconResponse.Header().Get("Content-Type") != "image/png" || faviconResponse.Body.Len() < 8 {
		t.Fatalf("favicon response = %d %q (%d bytes)", faviconResponse.Code, faviconResponse.Header().Get("Content-Type"), faviconResponse.Body.Len())
	}
}

func TestHTTPConfigurationRendersRedactedValuesAndPreservesFieldValidation(t *testing.T) {
	configuration := &configurationTestStore{configuration: testConfiguration()}
	handler := newConfigurationRouter(configuration).Handler()
	cookie, csrfToken := signInConfiguration(t, handler)

	page := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/configuration", nil)
	page.AddCookie(cookie)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, page)
	pageBody := pageResponse.Body.String()
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("configuration GET = %d, want 200: %s", pageResponse.Code, pageBody)
	}
	for _, secret := range []string{"admin", "hash", "resolved-secret"} {
		if strings.Contains(pageBody, secret) {
			t.Fatalf("configuration GET exposed %q: %s", secret, pageBody)
		}
	}

	form := validConfigurationForm()
	form.Set("page_title", " ")
	form.Set("csrf_token", csrfToken)
	post := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Origin", "https://bookmarker.test")
	post.AddCookie(cookie)
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, post)
	postBody := postResponse.Body.String()
	if postResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid configuration POST = %d, want 422: %s", postResponse.Code, postBody)
	}
	if !strings.Contains(postBody, "page_title:") || !strings.Contains(postBody, `value=" "`) {
		t.Fatalf("validation response did not identify page_title and preserve the submitted value: %s", postBody)
	}
	if configuration.configuration.Branding.PageTitle != "Bookmarker" {
		t.Fatalf("invalid configuration changed active title to %q", configuration.configuration.Branding.PageTitle)
	}
}

func TestHTTPConfigurationUpdateUsesHTMXRestartRedirectAndProtectedDefault(t *testing.T) {
	configuration := &configurationTestStore{configuration: testConfiguration()}
	handler := newConfigurationRouter(configuration).Handler()
	cookie, csrfToken := signInConfiguration(t, handler)

	form := validConfigurationForm()
	form.Set("database_host", "replacement-db.test")
	form.Set("csrf_token", csrfToken)
	update := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
	update.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	update.Header.Set("Origin", "https://bookmarker.test")
	update.Header.Set("HX-Request", "true")
	update.AddCookie(cookie)
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, update)
	if updateResponse.Code != http.StatusOK || updateResponse.Header().Get("HX-Redirect") != "/configuration?restart_required=true" {
		t.Fatalf("htmx configuration update = %d redirect=%q, want 200 restart redirect", updateResponse.Code, updateResponse.Header().Get("HX-Redirect"))
	}
	if configuration.configuration.Database.Host != "replacement-db.test" {
		t.Fatalf("updated database host = %q", configuration.configuration.Database.Host)
	}

	defaultRoute := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/", nil)
	defaultResponse := httptest.NewRecorder()
	handler.ServeHTTP(defaultResponse, defaultRoute)
	if defaultResponse.Code != http.StatusSeeOther || defaultResponse.Header().Get("Location") != "/bookmarks" {
		t.Fatalf("active default route = %d %q, want /bookmarks", defaultResponse.Code, defaultResponse.Header().Get("Location"))
	}
}

func TestHTTPConfigurationUpdateChangesTheActiveDefaultView(t *testing.T) {
	configuration := &configurationTestStore{configuration: testConfiguration()}
	handler := newConfigurationRouter(configuration).Handler()
	cookie, csrfToken := signInConfiguration(t, handler)

	form := validConfigurationForm()
	form.Set("default_view", "search")
	form.Set("csrf_token", csrfToken)
	update := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
	update.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	update.Header.Set("Origin", "https://bookmarker.test")
	update.AddCookie(cookie)
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, update)
	if updateResponse.Code != http.StatusSeeOther || updateResponse.Header().Get("Location") != "/configuration" {
		t.Fatalf("default-view update = %d %q", updateResponse.Code, updateResponse.Header().Get("Location"))
	}

	root := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/", nil)
	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, root)
	if rootResponse.Code != http.StatusSeeOther || rootResponse.Header().Get("Location") != "/search" {
		t.Fatalf("active default route = %d %q, want /search", rootResponse.Code, rootResponse.Header().Get("Location"))
	}
}

type configurationTestStore struct{ configuration model.Configuration }

func (store *configurationTestStore) Load(context.Context) (model.Configuration, error) {
	return store.configuration, nil
}

func (store *configurationTestStore) Activate(_ context.Context, configuration model.Configuration) error {
	store.configuration = configuration
	return nil
}

func newConfigurationRouter(configuration *configurationTestStore) *httpweb.Router {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	authenticator := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock:         clock,
		Passwords:     &testkit.PasswordVerifier{ExpectedHash: "hash", ExpectedPassword: "correct horse"},
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
	})
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: configuration, Activator: configuration})
	return httpweb.NewRouter(httpweb.Dependencies{TemplatesDir: "../../web/templates", DefaultView: "configuration", Authenticator: authenticator, Configuration: service})
}

func signInConfiguration(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	loginPage := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/login", nil)
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginPage)
	anonymousCookie := loginResponse.Result().Cookies()[0]
	loginToken := csrfTokenFromHTML(t, loginResponse.Body.String())

	login := url.Values{"username": {"admin"}, "password": {"correct horse"}, "csrf_token": {loginToken}}
	loginRequest := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/login", strings.NewReader(login.Encode()))
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRequest.Header.Set("Origin", "https://bookmarker.test")
	loginRequest.AddCookie(anonymousCookie)
	loginResponse = httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusSeeOther {
		t.Fatalf("login = %d, want 303: %s", loginResponse.Code, loginResponse.Body.String())
	}
	cookie := loginResponse.Result().Cookies()[0]

	configurationPage := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/configuration", nil)
	configurationPage.AddCookie(cookie)
	configurationResponse := httptest.NewRecorder()
	handler.ServeHTTP(configurationResponse, configurationPage)
	return cookie, csrfTokenFromHTML(t, configurationResponse.Body.String())
}

func csrfTokenFromHTML(t *testing.T, body string) string {
	t.Helper()
	const prefix = `name="csrf_token" value="`
	start := strings.Index(body, prefix)
	if start < 0 {
		t.Fatalf("response missing csrf token: %s", body)
	}
	value := body[start+len(prefix):]
	return value[:strings.Index(value, `"`)]
}

func testConfiguration() model.Configuration {
	return model.Configuration{
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
		Branding:      model.BrandingConfig{PageTitle: "Bookmarker"},
		DefaultView:   "list",
		Database:      model.DatabaseConfig{Host: "db.test", Port: 5432, Name: "bookmarker", User: "bookmarker", PasswordEnv: "BOOKMARKER_DB_PASSWORD", Password: "resolved-secret", TLSMode: "require"},
		Screenshots:   model.ScreenshotConfig{Root: "/var/lib/bookmarker/screenshots"},
		Search:        model.SearchConfig{PageSize: 10, MaximumResults: 10000},
	}
}

func validConfigurationForm() url.Values {
	return url.Values{
		"page_title":             {"Bookmarker"},
		"favicon_path":           {""},
		"default_view":           {"list"},
		"screenshot_root":        {"/var/lib/bookmarker/screenshots"},
		"database_host":          {"db.test"},
		"database_port":          {"5432"},
		"database_name":          {"bookmarker"},
		"database_user":          {"bookmarker"},
		"database_password_env":  {"BOOKMARKER_DB_PASSWORD"},
		"database_tls_mode":      {"require"},
		"search_page_size":       {"10"},
		"maximum_search_results": {"10000"},
	}
}
