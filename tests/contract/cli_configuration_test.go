//go:build contract

package contract_test

import (
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/jsonl"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

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
	clock := testkit.NewClock(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	authenticator := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock:         clock,
		Passwords:     &testkit.PasswordVerifier{ExpectedHash: "hash", ExpectedPassword: "correct horse"},
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
	})
	service := usecase.NewConfigurationService(usecase.ConfigurationDependencies{Store: configuration, Activator: configuration})
	return jsonl.NewServer(jsonl.Dependencies{Authenticator: authenticator, Configuration: service})
}
