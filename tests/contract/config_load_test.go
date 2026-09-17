//go:build contract

package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
