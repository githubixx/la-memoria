//go:build contract

package contract_test

import (
	"bytes"
	"strings"
	"testing"

	"log/slog"

	"github.com/githubixx/la-memoria/adapters/observability"
)

func TestStructuredLoggerRedactsCredentialsTokensAndRequestBodies(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(&output)
	logger.Info("request completed",
		slog.String("password", "correct horse"),
		slog.String("password_hash", "$argon2id$secret"),
		slog.String("cookie", "session-cookie"),
		slog.String("cookies", "other-cookie"),
		slog.String("csrf_token", "csrf-value"),
		slog.String("database_password", "database-secret"),
		slog.String("environment_value", "environment-secret"),
		slog.String("form_body", "username=admin&password=correct+horse"),
		slog.String("target_url", "https://private.example.test/path"),
	)

	for _, secret := range []string{
		"correct horse", "$argon2id$secret", "session-cookie", "other-cookie", "csrf-value",
		"database-secret", "environment-secret", "username=admin", "private.example.test",
	} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("log output exposed %q: %s", secret, output.String())
		}
	}
	if strings.Count(output.String(), "[redacted]") != 9 {
		t.Fatalf("redactions = %d, want 9: %s", strings.Count(output.String(), "[redacted]"), output.String())
	}
}
