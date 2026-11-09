package usecase_test

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	chromaui "github.com/githubixx/chroma-ui"
	"github.com/githubixx/la-memoria/adapters/chromatheme"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

func TestConfigurationThemeCandidateValidationAndPresence(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	options := catalog.Themes()
	if len(options) == 0 {
		t.Fatal("empty catalog")
	}
	last := ""
	for _, option := range options {
		if option.ID <= last || option.Name == "" || !option.Bundled || !catalog.Contains(option.ID) {
			t.Fatalf("invalid, unsorted, duplicate, or undiscoverable catalog option: %#v", option)
		}
		last = option.ID
	}
	if catalog.Contains("system") || catalog.Contains("not-a-theme") {
		t.Fatal("system and unknown values must not be catalog members")
	}
	valid := options[0].ID
	for _, tc := range []struct {
		name, stored, submitted string
		present, invalid        bool
	}{
		{"missing defaults to system", "", "", false, false},
		{"omitted retains valid", valid, "", false, false},
		{"omitted obsolete rejected", "obsolete", "", false, true},
		{"system", "", "system", true, false},
		{"catalog id", "", valid, true, false},
		{"empty explicit", valid, "", true, true},
		{"unknown", valid, "unknown-theme", true, true},
		{"case sensitive", valid, strings.ToUpper(valid), true, strings.ToUpper(valid) != valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, operation := range []string{"validate", "update"} {
				t.Run(operation, func(t *testing.T) {
					current := testConfiguration()
					current.Theme = tc.stored
					store := &configurationStore{configuration: current}
					service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store, Catalog: catalog})
					candidate := current.Redacted()
					candidate.Theme, candidate.ThemePresent = tc.submitted, tc.present
					candidate.Branding.PageTitle = "Changed"
					var change usecase.ConfigurationChange
					var err error
					if operation == "validate" {
						change, err = service.Validate(context.Background(), candidate)
					} else {
						change, err = service.Update(context.Background(), candidate)
					}
					if tc.invalid {
						if model.ErrorCode(err) != "validation_failed" || store.activations != 0 || !reflect.DeepEqual(store.configuration, current) {
							t.Fatalf("rejection = %v, activations = %d, configuration = %#v", err, store.activations, store.configuration)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					want := tc.submitted
					if !tc.present {
						want = tc.stored
						if want == "" {
							want = "system"
						}
					}
					if change.Configuration.Theme != want {
						t.Fatalf("theme = %q, want %q", change.Configuration.Theme, want)
					}
					if operation == "validate" && store.activations != 0 {
						t.Fatal("validate wrote configuration")
					}
				})
			}
		})
	}
}

func TestThemeValidationRejectsEntireInvalidCandidate(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	current := testConfiguration()
	store := &configurationStore{configuration: current}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store, Catalog: catalog})
	candidate := current.Redacted()
	candidate.Theme, candidate.ThemePresent = "system", true
	candidate.Search.PageSize = 0
	if _, err := service.Update(context.Background(), candidate); model.ErrorCode(err) != "validation_failed" {
		t.Fatalf("invalid complete candidate: %v", err)
	}
	if store.activations != 0 || !reflect.DeepEqual(store.configuration, current) {
		t.Fatal("invalid complete candidate was saved")
	}
}

func TestEditableConfigurationValidatesBoundsAndRedactsSecrets(t *testing.T) {
	configuration := model.Configuration{
		Administrator: model.Administrator{Username: "admin", PasswordHash: "secret-hash"},
		Branding:      model.BrandingConfig{PageTitle: "Bookmarker", FaviconPath: "favicon.png"},
		DefaultView:   "list",
		Database:      model.DatabaseConfig{Host: "db.example.test", Port: 5432, Name: "bookmarker", User: "bookmarker", PasswordEnv: "BOOKMARKER_DB_PASSWORD", Password: "resolved-secret", TLSMode: "require"},
		Screenshots:   model.ScreenshotConfig{Root: "/var/lib/bookmarker/screenshots"},
		Search:        model.SearchConfig{PageSize: 25, MaximumResults: 10000},
	}
	candidate, err := model.ValidateEditableConfiguration(configuration.Redacted())
	if err != nil {
		t.Fatalf("validate editable configuration: %v", err)
	}
	if candidate.Database.Password != "" {
		t.Fatal("editable configuration must not include the resolved database password")
	}
	if candidate.Branding.PageTitle != "Bookmarker" || candidate.Search.PageSize != 25 {
		t.Fatalf("candidate = %#v, want validated editable values", candidate)
	}

	invalid := configuration.Redacted()
	invalid.Search.PageSize = 0
	if _, err := model.ValidateEditableConfiguration(invalid); model.ErrorCode(err) != "validation_failed" {
		t.Fatalf("invalid page size error = %v, want validation_failed", err)
	}
}

func TestEditableConfigurationRejectsProtectedFieldsAndInvalidEnvironmentNames(t *testing.T) {
	candidate := testConfiguration().Redacted()
	candidate.Database.Password = "must-not-be-accepted"
	if _, err := model.ValidateEditableConfiguration(candidate); model.ErrorCode(err) != "validation_failed" {
		t.Fatalf("inline database password error = %v, want validation_failed", err)
	}

	candidate = testConfiguration().Redacted()
	candidate.Database.PasswordEnv = "not-valid"
	if _, err := model.ValidateEditableConfiguration(candidate); model.ErrorCode(err) != "validation_failed" {
		t.Fatalf("invalid environment variable name error = %v, want validation_failed", err)
	}
}

func TestConfigurationServiceValidatesBeforeActivatingAndPreservesDeploymentSecrets(t *testing.T) {
	store := &configurationStore{configuration: testConfiguration()}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store})
	candidate, err := service.Get(context.Background())
	if err != nil {
		t.Fatalf("get configuration: %v", err)
	}
	candidate.Branding.PageTitle = "Renamed"
	candidate.Database.Host = "other-db.example.test"
	change, err := service.Validate(context.Background(), candidate)
	if err != nil || !change.RestartRequired || store.activations != 0 {
		t.Fatalf("validate = %#v, %v; activations = %d", change, err, store.activations)
	}
	if _, err := service.Update(context.Background(), candidate); err != nil {
		t.Fatalf("update configuration: %v", err)
	}
	if store.activations != 1 || store.configuration.Administrator.PasswordHash != "secret-hash" || store.configuration.Database.Password != "resolved-secret" {
		t.Fatalf("activated configuration = %#v", store.configuration)
	}
}

func TestConfigurationServicePreservesLastActiveConfigurationWhenActivationFails(t *testing.T) {
	store := &configurationStore{configuration: testConfiguration(), activationError: &model.ApplicationError{Code: "dependency_unavailable", Message: "unavailable"}}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store})
	candidate := testConfiguration().Redacted()
	candidate.Branding.PageTitle = "Not Activated"
	if _, err := service.Update(context.Background(), candidate); model.ErrorCode(err) != "dependency_unavailable" {
		t.Fatalf("update error = %v, want dependency_unavailable", err)
	}
	if store.configuration.Branding.PageTitle != "Bookmarker" || store.activations != 0 {
		t.Fatalf("configuration changed after failed activation: %#v", store.configuration)
	}
}

func testConfiguration() model.Configuration {
	return model.Configuration{Administrator: model.Administrator{Username: "admin", PasswordHash: "secret-hash"}, Branding: model.BrandingConfig{PageTitle: "Bookmarker"}, DefaultView: "list", Database: model.DatabaseConfig{Host: "db.example.test", Port: 5432, Name: "bookmarker", User: "bookmarker", PasswordEnv: "BOOKMARKER_DB_PASSWORD", Password: "resolved-secret", TLSMode: "require"}, Screenshots: model.ScreenshotConfig{Root: "/var/lib/bookmarker/screenshots"}, Search: model.SearchConfig{PageSize: 10, MaximumResults: 10000}}
}

type configurationStore struct {
	configuration   model.Configuration
	activations     int
	activationError error
}

func (store *configurationStore) Load(context.Context) (model.Configuration, error) {
	return store.configuration, nil
}

func (store *configurationStore) Validate(_ context.Context, configuration model.Configuration) error {
	return nil
}

func (store *configurationStore) Activate(_ context.Context, configuration model.Configuration) error {
	if store.activationError != nil {
		return store.activationError
	}
	store.configuration = configuration
	store.activations++
	return nil
}

func TestBundledThemeCatalogHasSortedUniqueExactIDs(t *testing.T) {
	catalog, err := chromaui.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	themes := catalog.Themes()
	if len(themes) == 0 {
		t.Fatal("bundled catalog is empty")
	}
	ids := make([]string, 0, len(themes))
	seen := make(map[string]bool)
	for _, theme := range themes {
		if theme.ID == "" || seen[theme.ID] {
			t.Errorf("empty or duplicate theme ID %q", theme.ID)
		}
		seen[theme.ID] = true
		ids = append(ids, theme.ID)
		if _, ok := catalog.Theme(theme.ID); !ok {
			t.Errorf("catalog cannot find its theme %q", theme.ID)
		}
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("theme IDs are not sorted: %v", ids)
	}
	if _, ok := catalog.Theme("system"); ok {
		t.Error("System must not be a bundled theme")
	}
	for _, id := range ids {
		upper := strings.ToUpper(id)
		if upper != id && !seen[upper] {
			if _, ok := catalog.Theme(upper); ok {
				t.Errorf("catalog lookup is not case-sensitive for %q", id)
			}
			return
		}
	}
	t.Fatal("catalog has no ID with a case variant to test")
}

func TestConfigurationServiceGetProjectsDefaultAndUnavailableTheme(t *testing.T) {
	for _, test := range []struct {
		name, stored, want string
	}{
		{"missing", "", "system"},
		{"unavailable", "retired-theme", "retired-theme"},
	} {
		t.Run(test.name, func(t *testing.T) {
			configuration := testConfiguration()
			stored := reflect.ValueOf(&configuration).Elem().FieldByName("Theme")
			if !stored.IsValid() || stored.Kind() != reflect.String || !stored.CanSet() {
				t.Fatal("configuration must expose a settable string Theme")
			}
			stored.SetString(test.stored)
			store := &configurationStore{configuration: configuration}
			service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store})
			result, err := service.Get(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			theme := reflect.ValueOf(result).FieldByName("Theme")
			if !theme.IsValid() || theme.Kind() != reflect.String {
				t.Fatal("redacted configuration must expose a string Theme")
			}
			if got := theme.String(); got != test.want {
				t.Errorf("theme = %q, want %q", got, test.want)
			}
		})
	}
}
