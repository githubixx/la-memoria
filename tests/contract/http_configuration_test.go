//go:build contract

package contract_test

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/chromatheme"
	"github.com/githubixx/la-memoria/adapters/httpweb"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestHTTPThemeSelectionAndInvalidSubmissions(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	theme := catalog.Themes()[0].ID
	current := testConfiguration()
	current.Theme = theme
	store := &configurationTestStore{configuration: current}
	handler := newConfigurationRouter(store).Handler()
	cookie, csrf := signInConfiguration(t, handler)
	get := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/configuration", nil)
	get.AddCookie(cookie)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, get)
	if page.Code != 200 || page.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("GET = %d, headers = %v", page.Code, page.Header())
	}
	t.Run("catalog options", func(t *testing.T) {
		body := page.Body.String()
		if !strings.Contains(body, `for="theme"`) {
			t.Fatal("missing labeled theme select")
		}
		selectRE := regexp.MustCompile(`(?s)<select[^>]*name="theme"[^>]*>(.*?)</select>`)
		selectMatch := selectRE.FindStringSubmatch(body)
		if len(selectMatch) != 2 {
			t.Fatal("missing theme select")
		}
		optionRE := regexp.MustCompile(`<option value="([^"]*)"([^>]*)>([^<]*)</option>`)
		options := optionRE.FindAllStringSubmatch(selectMatch[1], -1)
		if len(options) != len(catalog.Themes())+1 {
			t.Fatalf("options = %d, want %d", len(options), len(catalog.Themes())+1)
		}
		ids, labels := map[string]bool{}, map[string]bool{}
		for _, option := range options {
			id, label := html.UnescapeString(option[1]), html.UnescapeString(strings.TrimSpace(option[3]))
			if ids[id] || labels[label] || label == "" {
				t.Fatalf("duplicate option or label: %q %q", id, label)
			}
			ids[id], labels[label] = true, true
			if id == theme && !strings.Contains(option[2], "selected") {
				t.Fatal("stored option not selected")
			}
		}
		if !ids["system"] {
			t.Fatal("System missing")
		}
		for _, option := range catalog.Themes() {
			if !ids[option.ID] {
				t.Errorf("missing theme %q", option.ID)
			}
		}
	})
	for _, hx := range []bool{false, true} {
		for _, invalid := range []string{"missing", "unknown", "empty"} {
			t.Run(fmt.Sprintf("%t/%s", hx, invalid), func(t *testing.T) {
				store.configuration = current
				form := validConfigurationForm()
				form.Set("csrf_token", csrf)
				form.Set("page_title", "Must not save")
				form.Set("database_host", "changed.test")
				switch invalid {
				case "missing":
					form.Del("theme")
				case "unknown":
					form.Set("theme", "../invalid")
				case "empty":
					form.Set("theme", "")
				}
				req := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Origin", "https://bookmarker.test")
				if hx {
					req.Header.Set("HX-Request", "true")
				}
				req.AddCookie(cookie)
				res := httptest.NewRecorder()
				handler.ServeHTTP(res, req)
				if res.Code != http.StatusUnprocessableEntity || res.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(store.configuration, current) {
					t.Fatalf("invalid update = %d, state changed=%v", res.Code, !reflect.DeepEqual(store.configuration, current))
				}
			})
		}
	}
	for _, hx := range []bool{false, true} {
		store.configuration = current
		form := validConfigurationForm()
		form.Set("theme", theme)
		form.Set("csrf_token", csrf)
		form.Set("database_host", "changed.test")
		req := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://bookmarker.test")
		if hx {
			req.Header.Set("HX-Request", "true")
		}
		req.AddCookie(cookie)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		want := "/configuration?restart_required=true"
		if hx && (res.Code != 200 || res.Header().Get("HX-Redirect") != want) {
			t.Fatalf("htmx = %d, redirect=%s", res.Code, res.Header().Get("HX-Redirect"))
		}
		if !hx && (res.Code != 303 || res.Header().Get("Location") != want) {
			t.Fatalf("normal = %d, redirect=%s", res.Code, res.Header().Get("Location"))
		}
		store.configuration = current
	}
	form := validConfigurationForm()
	form.Set("theme", theme)
	form.Set("csrf_token", "invalid")
	req := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://bookmarker.test")
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden || !reflect.DeepEqual(store.configuration, current) {
		t.Fatal("invalid CSRF was not rejected atomically")
	}
}

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

type configurationTestStore struct {
	configuration   model.Configuration
	activationError error
}

func (store *configurationTestStore) Load(context.Context) (model.Configuration, error) {
	return store.configuration, nil
}

func (store *configurationTestStore) Activate(_ context.Context, configuration model.Configuration) error {
	if store.activationError != nil {
		return store.activationError
	}
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
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		panic(err)
	}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: configuration, Activator: configuration, Catalog: catalog})
	return httpweb.NewRouter(httpweb.Dependencies{TemplatesDir: "../../web/templates", DefaultView: "configuration", Authenticator: authenticator, Configuration: service, ThemeCatalog: catalog, Appearance: usecase.NewAppearanceService(configuration, catalog, nil)})
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
		"theme":                  {"system"},
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

func TestHTTPThemeRecoveryAndFailurePresentation(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	theme := catalog.Themes()[0].ID
	t.Run("obsolete selection", func(t *testing.T) {
		current := testConfiguration()
		current.Theme = `retired-<theme>`
		store := &configurationTestStore{configuration: current}
		handler := newConfigurationRouter(store).Handler()
		cookie, csrf := signInConfiguration(t, handler)
		get := httptest.NewRequest(http.MethodGet, "https://bookmarker.test/configuration", nil)
		get.AddCookie(cookie)
		page := httptest.NewRecorder()
		handler.ServeHTTP(page, get)
		body := page.Body.String()
		if !strings.Contains(body, "retired-&lt;theme&gt;") || !strings.Contains(body, "unavailable") || !strings.Contains(body, "selected") || strings.Contains(body, "/ui/themes/retired-") {
			t.Fatal("obsolete selection was not safely presented as unavailable")
		}
		form := validConfigurationForm()
		form.Set("theme", current.Theme)
		form.Set("page_title", "Candidate title")
		form.Set("csrf_token", csrf)
		req := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://bookmarker.test")
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != 422 || response.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(store.configuration, current) {
			t.Fatal("obsolete selection changed configuration or lacked 422/no-store")
		}
		body = response.Body.String()
		if !strings.Contains(body, `value="Candidate title"`) || !strings.Contains(body, `aria-invalid="true"`) || !strings.Contains(body, `aria-describedby=`) || strings.Contains(body, "/ui/themes/retired-") {
			t.Fatal("obsolete error did not preserve input, associate error, and retain System shell")
		}
		form.Set("theme", theme)
		req = httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://bookmarker.test")
		req.AddCookie(cookie)
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != 303 || store.configuration.Theme != theme {
			t.Fatal("valid replacement did not recover")
		}
	})
	for _, hx := range []bool{false, true} {
		for _, failure := range []string{"invalid fields", "writer"} {
			t.Run(fmt.Sprintf("%t/%s", hx, failure), func(t *testing.T) {
				current := testConfiguration()
				current.Theme = theme
				store := &configurationTestStore{configuration: current}
				if failure == "writer" {
					store.activationError = errors.New("private writer secret")
				}
				handler := newConfigurationRouter(store).Handler()
				cookie, csrf := signInConfiguration(t, handler)
				form := validConfigurationForm()
				form.Set("theme", "system")
				form.Set("page_title", "Candidate title")
				form.Set("csrf_token", csrf)
				if failure == "invalid fields" {
					form.Set("search_page_size", "0")
				}
				req := httptest.NewRequest(http.MethodPost, "https://bookmarker.test/configuration", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Origin", "https://bookmarker.test")
				if hx {
					req.Header.Set("HX-Request", "true")
				}
				req.AddCookie(cookie)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				body := response.Body.String()
				if response.Code != 422 || response.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(store.configuration, current) {
					t.Fatal("failure changed the saved configuration")
				}
				if !strings.Contains(body, `value="Candidate title"`) || strings.Contains(body, "private writer secret") || strings.Contains(body, "resolved-secret") {
					t.Fatal("failure did not preserve candidate or leaked secrets")
				}
				if !hx && !strings.Contains(body, "/ui/themes/"+theme+"/theme.css") {
					t.Fatal("full error did not retain persisted appearance")
				}
				if hx && strings.Contains(body, "<html") {
					t.Fatal("htmx error must be a fragment")
				}
			})
		}
	}
}
