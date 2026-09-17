// Package observability provides the shared JSON structured-logging
// construction used by both delivery adapters.
package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/url"
	"strings"
)

// SensitiveFields lists log attribute keys that must never be emitted in
// plaintext, across both the HTTP and CLI adapters.
var SensitiveFields = []string{
	"password",
	"plaintext_password",
	"password_hash",
	"hash",
	"token",
	"session_token",
	"csrf_token",
	"cookie",
	"database_password",
	"secret",
	"environment_value",
	"body",
	"url",
	"target_url",
}

// NewLogger builds a JSON slog.Logger that redacts SensitiveFields and
// carries no attribute state beyond what each call site supplies.
func NewLogger(output io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{ReplaceAttr: redactAttribute})
	return slog.New(handler)
}

func redactAttribute(_ []string, attribute slog.Attr) slog.Attr {
	for _, name := range SensitiveFields {
		if strings.EqualFold(attribute.Key, name) {
			return slog.String(attribute.Key, "[redacted]")
		}
	}
	return attribute
}

// HostFingerprint returns a capture destination's scheme plus a
// non-reversible fingerprint of its host, safe to log instead of the full
// target URL.
func HostFingerprint(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "invalid"
	}
	sum := sha256.Sum256([]byte(parsed.Host))
	return parsed.Scheme + "://" + hex.EncodeToString(sum[:8])
}
