package usecase_test

import (
	"context"
	"testing"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

func TestEditableConfigurationValidatesBoundsAndRedactsSecrets(t *testing.T) {
	configuration := model.Configuration{
		Administrator: model.Administrator{Username: "admin", PasswordHash: "secret-hash"},
		Branding:      model.BrandingConfig{PageTitle: "Bookmarker", FaviconPath: "favicon.png"},
		DefaultView:   "list",
		Database: model.DatabaseConfig{
			Host: "db.example.test", Port: 5432, Name: "bookmarker", User: "bookmarker",
			PasswordEnv: "BOOKMARKER_DB_PASSWORD", Password: "resolved-secret", TLSMode: "require",
		},
		Screenshots: model.ScreenshotConfig{Root: "/var/lib/bookmarker/screenshots"},
		Search:      model.SearchConfig{PageSize: 25, MaximumResults: 10000},
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
