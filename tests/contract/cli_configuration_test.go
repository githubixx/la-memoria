//go:build contract

package contract_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/chromatheme"
	"github.com/githubixx/la-memoria/adapters/configyaml"
	"github.com/githubixx/la-memoria/adapters/jsonl"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestCLIGetReturnsSystemAndRequiresAuthentication(t *testing.T) {
	store := &configurationTestStore{configuration: testConfiguration()}
	out, _ := serveStream(t, newConfigurationServer(store), []string{
		`{"version":"v1","id":"unauthorized","operation":"configuration.get","payload":{}}`,
		`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"get","operation":"configuration.get","payload":{}}`,
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"authentication_required"`) || !strings.Contains(lines[2], `"theme":"system"`) {
		t.Fatal("get did not require authentication or return System")
	}
	for _, secret := range []string{"correct horse", "resolved-secret", `"password_hash"`} {
		if strings.Contains(out, secret) {
			t.Fatal("configuration stream exposed a secret")
		}
	}
}
func TestCLIThemePresenceValidationAndAtomicity(t *testing.T) {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	theme := catalog.Themes()[0].ID
	for _, tc := range []struct {
		name, stored     string
		value            any
		present, invalid bool
	}{
		{"omitted old client", theme, nil, false, false},
		{"missing defaults", "", nil, false, false},
		{"obsolete omission", "obsolete-theme", nil, false, true},
		{"null", theme, nil, true, true},
		{"empty", theme, "", true, true},
		{"wrong type", theme, 42, true, true},
		{"unknown", theme, "unknown-theme", true, true},
		{"case mismatch", theme, strings.ToUpper(theme), true, true},
		{"system", theme, "system", true, false},
		{"valid id", "", theme, true, false},
	} {
		for _, operation := range []string{"configuration.validate", "configuration.update"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				current := testConfiguration()
				current.Theme = tc.stored
				store := &configurationTestStore{configuration: current}
				initial, err := json.Marshal(current.Redacted())
				if err != nil {
					t.Fatal(err)
				}
				var payload map[string]any
				if err := json.Unmarshal(initial, &payload); err != nil {
					t.Fatal(err)
				}
				delete(payload, "theme")
				if tc.present {
					payload["theme"] = tc.value
				}
				payload["branding"].(map[string]any)["page_title"] = "Changed"
				encoded, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				out, _ := serveStream(t, newConfigurationServer(store), []string{
					`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
					`{"version":"v1","id":"change","operation":"` + operation + `","payload":` + string(encoded) + `}`,
				})
				lines := strings.Split(strings.TrimSpace(out), "\n")
				var result struct {
					OK    bool `json:"ok"`
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
					Result struct {
						Configuration model.EditableConfiguration `json:"configuration"`
					} `json:"result"`
				}
				if len(lines) != 2 || json.Unmarshal([]byte(lines[1]), &result) != nil {
					t.Fatalf("invalid response stream %q", out)
				}
				if tc.invalid {
					if result.OK || result.Error.Code != "validation_failed" || !reflect.DeepEqual(store.configuration, current) {
						t.Fatalf("rejection = %#v, state changed = %v", result, !reflect.DeepEqual(store.configuration, current))
					}
					return
				}
				want := tc.stored
				if tc.present {
					want = tc.value.(string)
				}
				if want == "" {
					want = "system"
				}
				if !result.OK || result.Result.Configuration.Theme != want {
					t.Fatalf("response = %#v, want theme %q", result, want)
				}
				if operation == "configuration.validate" && !reflect.DeepEqual(store.configuration, current) {
					t.Fatal("validate changed configuration")
				}
				if operation == "configuration.update" && store.configuration.Theme != want {
					t.Fatalf("saved theme = %q", store.configuration.Theme)
				}
			})
		}
	}
}

func TestCLIConfigurationUsesRedactedStrictAtomicOperations(t *testing.T) {
	configuration := &configurationTestStore{configuration: testConfiguration()}
	server := newConfigurationServer(configuration)
	candidate := `{"branding":{"page_title":"Updated Bookmarker","favicon_path":""},"default_view":"list","database":{"host":"replacement-db.test","port":5432,"name":"bookmarker","user":"bookmarker","password_env":"BOOKMARKER_DB_PASSWORD","tls_mode":"require"},"screenshots":{"root":"/var/lib/bookmarker/screenshots"},"search":{"page_size":10,"maximum_results":10000}}`

	stdout, stderr := serveStream(t, server, []string{
		`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"get","operation":"configuration.get","payload":{}}`,
		`{"version":"v1","id":"validate","operation":"configuration.validate","payload":` + candidate + `}`,
		`{"version":"v1","id":"credentials","operation":"configuration.validate","payload":{"administrator":{"username":"leak"}}}`,
	})

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 4 {
		t.Fatalf("response count = %d, want 4: %q", len(lines), stdout)
	}
	for _, secret := range []string{"admin", "hash", "resolved-secret", "correct horse"} {
		if strings.Contains(stdout, secret) || strings.Contains(stderr, secret) {
			t.Fatalf("CLI output disclosed %q: stdout=%q stderr=%q", secret, stdout, stderr)
		}
	}
	if !strings.Contains(lines[1], `"page_title":"Bookmarker"`) || !strings.Contains(lines[1], `"password_env":"BOOKMARKER_DB_PASSWORD"`) {
		t.Fatalf("configuration.get response = %q, want redacted editable values", lines[1])
	}
	if !strings.Contains(lines[2], `"restart_required":true`) || configuration.configuration.Database.Host != "db.test" {
		t.Fatalf("configuration.validate response = %q; active host = %q, want restart notice without activation", lines[2], configuration.configuration.Database.Host)
	}
	if !strings.Contains(lines[3], `"unknown_field"`) {
		t.Fatalf("credential field response = %q, want unknown_field", lines[3])
	}

	updateOutput, updateDiagnostics := serveStream(t, newConfigurationServer(configuration), []string{
		`{"version":"v1","id":"login-update","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"update","operation":"configuration.update","payload":` + candidate + `}`,
	})
	if strings.Contains(updateOutput, "correct horse") || strings.Contains(updateDiagnostics, "correct horse") {
		t.Fatalf("update stream disclosed password: stdout=%q stderr=%q", updateOutput, updateDiagnostics)
	}
	if !strings.Contains(updateOutput, `"restart_required":true`) || configuration.configuration.Database.Host != "replacement-db.test" {
		t.Fatalf("configuration.update response = %q; active host = %q, want atomic update", updateOutput, configuration.configuration.Database.Host)
	}
}

func newConfigurationServer(configuration *configurationTestStore) *jsonl.Server {
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		panic(err)
	}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: configuration, Activator: configuration, Catalog: catalog})
	return newConfigurationServerWithService(service)
}

func newConfigurationServerWithService(service *usecase.ConfigurationService) *jsonl.Server {
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	authenticator := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock:         clock,
		Passwords:     &testkit.PasswordVerifier{ExpectedHash: "hash", ExpectedPassword: "correct horse"},
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
	})
	return jsonl.NewServer(jsonl.Dependencies{Authenticator: authenticator, Configuration: service})
}

func TestCLIStorageFailureKeepsCommittedBytes(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := configyaml.NewStoreWithStagedWriter(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"}, func(*os.File, []byte) (int, error) { return 0, nil })
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store, Catalog: catalog})
	current, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	candidate := current.Redacted()
	candidate.Branding.PageTitle = "Uncommitted title"
	payload, _ := json.Marshal(candidate)
	out, diagnostics := serveStream(t, newConfigurationServerWithService(service), []string{
		`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"update","operation":"configuration.update","payload":` + string(payload) + `}`,
	})
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) || !strings.Contains(out, `"internal_error"`) || strings.Contains(out, "database-secret") || strings.Contains(diagnostics, "database-secret") {
		t.Fatal("storage failure did not preserve bytes and use a redacted error envelope")
	}
}

func TestCLIRecoversObsoleteThemeWithoutPartialUpdates(t *testing.T) {
	path := writeConfiguration(t, validConfiguration+"\ntheme: retired-theme\n", 0o600)
	store := configyaml.NewStore(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
	catalog, err := chromatheme.NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: store, Validator: store, Activator: store, Catalog: catalog})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(current.Redacted())
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	payload["branding"].(map[string]any)["page_title"] = "Should not persist"
	delete(payload, "theme")
	for _, operation := range []string{"configuration.validate", "configuration.update"} {
		for _, invalid := range []bool{false, true} {
			if invalid {
				payload["theme"] = "system"
				payload["search"].(map[string]any)["page_size"] = float64(0)
			}
			data, _ := json.Marshal(payload)
			out, diagnostics := serveStream(t, newConfigurationServerWithService(service), []string{
				`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
				`{"version":"v1","id":"change","operation":"` + operation + `","payload":` + string(data) + `}`,
			})
			if !strings.Contains(out, `"validation_failed"`) || strings.Contains(out, "database-secret") || strings.Contains(diagnostics, "database-secret") {
				t.Fatal("invalid recovery was not safely rejected")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatal("invalid recovery changed saved bytes")
			}
			delete(payload, "theme")
			payload["search"].(map[string]any)["page_size"] = float64(10)
		}
	}
	payload["theme"] = "system"
	data, _ := json.Marshal(payload)
	out, _ := serveStream(t, newConfigurationServerWithService(service), []string{
		`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"change","operation":"configuration.update","payload":` + string(data) + `}`,
	})
	if !strings.Contains(out, `"theme":"system"`) {
		t.Fatal("explicit valid recovery failed")
	}
	loaded, err := store.Load(context.Background())
	if err != nil || loaded.Theme != "system" || loaded.Branding.PageTitle != "Should not persist" {
		t.Fatalf("recovered configuration incorrect: %v", err)
	}
}
