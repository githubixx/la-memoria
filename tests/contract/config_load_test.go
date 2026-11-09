//go:build contract

package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	chromaui "github.com/githubixx/chroma-ui"
	"github.com/githubixx/la-memoria/adapters/configyaml"
)

func TestLoadRejectsUnsafeOrAmbiguousDeploymentConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantCode string
	}{
		{"unknown field", validConfiguration + "\nunknown: value\n", "unknown_field"},
		{"duplicate key", strings.Replace(validConfiguration, "page_title: Bookmarker", "page_title: Bookmarker\n  page_title: Duplicate", 1), "duplicate_key"},
		{"invalid Argon2id verifier", strings.Replace(validConfiguration, validPHC, "$argon2i$not-valid", 1), "invalid_password_hash"},
		{"inline database secret", strings.Replace(validConfiguration, "password_env: BOOKMARKER_DB_PASSWORD", "password: database-secret", 1), "secret_not_allowed"},
		{"malformed environment reference", strings.Replace(validConfiguration, "password_env: BOOKMARKER_DB_PASSWORD", "password_env: 123", 1), "invalid_environment_reference"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfiguration(t, test.contents, 0o600)
			_, err := configyaml.Load(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
			if configyaml.ErrorCode(err) != test.wantCode {
				t.Fatalf("error code = %q, want %q (error: %v)", configyaml.ErrorCode(err), test.wantCode, err)
			}
		})
	}
}

func TestLoadResolvesEnvironmentReferencesAppliesDefaultsAndRejectsUnsafeFiles(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	configuration, err := configyaml.Load(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
	if err != nil {
		t.Fatalf("load valid configuration: %v", err)
	}
	if configuration.Search.PageSize != 10 || configuration.Search.MaximumResults != 10_000 {
		t.Fatalf("search defaults = %#v, want page size 10 and maximum results 10000", configuration.Search)
	}
	if configuration.Database.Password != "database-secret" {
		t.Fatal("database password environment reference was not resolved")
	}

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	if configyaml.ErrorCode(mustLoadError(t, missing, 0)) != "config_not_found" {
		t.Fatal("missing configuration must report config_not_found")
	}
	for _, mode := range []os.FileMode{0o640, 0o604} {
		unsafePath := writeConfiguration(t, validConfiguration, mode)
		if configyaml.ErrorCode(mustLoadError(t, unsafePath, mode)) != "unsafe_config_permissions" {
			t.Fatalf("mode %04o must be rejected", mode)
		}
	}
}

func mustLoadError(t *testing.T, path string, _ os.FileMode) error {
	t.Helper()
	_, err := configyaml.Load(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
	return err
}

func writeConfiguration(t *testing.T, contents string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod configuration: %v", err)
	}
	return path
}

const validPHC = "$argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA"

const validConfiguration = `administrator:
  username: admin
  password_hash: $argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA
branding:
  page_title: Bookmarker
database:
  host: 127.0.0.1
  port: 5432
  name: bookmarker
  user: bookmarker
  password_env: BOOKMARKER_DB_PASSWORD
  tls_mode: disable
screenshots:
  root: /tmp/bookmarker-screenshots
`

func TestLoadThemeDefaultsAndPreservesStoredValues(t *testing.T) {
	catalog, err := chromaui.NewCatalog(nil)
	if err != nil {
		t.Fatalf("construct bundled Chroma catalog: %v", err)
	}
	themes := catalog.Themes()
	if len(themes) == 0 {
		t.Fatal("expected at least one bundled Chroma theme")
	}
	tests := []struct {
		name, suffix, want string
	}{
		{"missing uses system", "", "system"},
		{"explicit system", "\ntheme: system\n", "system"},
		{"bundled theme", "\ntheme: " + themes[0].ID + "\n", themes[0].ID},
		{"stored unavailable ID", "\ntheme: retired-theme\n", "retired-theme"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfiguration(t, validConfiguration+test.suffix, 0o600)
			configuration, err := configyaml.Load(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
			if err != nil {
				t.Fatalf("load configuration: %v", err)
			}
			theme := reflect.ValueOf(configuration.Redacted()).FieldByName("Theme")
			if !theme.IsValid() || theme.Kind() != reflect.String {
				t.Fatal("redacted configuration must expose a string Theme")
			}
			if got := theme.String(); got != test.want {
				t.Fatalf("redacted theme = %q, want %q", got, test.want)
			}
			if configuration.Database.Password != "database-secret" {
				t.Fatal("resolved database password changed")
			}
			redacted, err := json.Marshal(configuration.Redacted())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(redacted), "database-secret") || strings.Contains(string(redacted), validPHC) {
				t.Fatal("redacted configuration exposed a deployment secret")
			}
		})
	}
}

func TestLoadThemeDoesNotRelaxStrictFieldsOrPermissions(t *testing.T) {
	catalog, err := chromaui.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	themes := catalog.Themes()
	if len(themes) == 0 {
		t.Fatal("expected bundled themes")
	}
	withTheme := validConfiguration + "\ntheme: " + themes[0].ID + "\n"
	t.Run("unknown field", func(t *testing.T) {
		validPath := writeConfiguration(t, withTheme, 0o600)
		if _, err := configyaml.Load(validPath, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"}); err != nil {
			t.Fatalf("theme alone must be accepted before checking unknown fields: %v", err)
		}
		path := writeConfiguration(t, withTheme+"unexpected: true\n", 0o600)
		_, err := configyaml.Load(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
		if got := configyaml.ErrorCode(err); got != "unknown_field" {
			t.Fatalf("error code = %q, want unknown_field", got)
		}
	})
	t.Run("unsafe permissions", func(t *testing.T) {
		path := writeConfiguration(t, withTheme, 0o640)
		_, err := configyaml.Load(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
		if got := configyaml.ErrorCode(err); got != "unsafe_config_permissions" {
			t.Fatalf("error code = %q, want unsafe_config_permissions", got)
		}
	})
}
