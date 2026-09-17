//go:build contract

package contract_test

import (
	"strings"
	"testing"
)

func TestCLIMaintainRequiresAuthenticationAndLiteralDeleteConfirmation(t *testing.T) {
	server, _ := newCreateServer()
	response := serveLines(t, server, `{"version":"v1","id":"update-before-login","operation":"bookmark.update","payload":{"bookmark_id":"bookmark-1","url":"https://example.test","description":"updated","tags":[],"save_without_screenshot":true}}`)
	if !strings.Contains(response, "authentication_required") {
		t.Fatalf("update before sign-in = %q, want authentication_required", response)
	}
	response = serveLines(t, server, `{"version":"v1","id":"delete-without-confirmation","operation":"bookmark.delete","payload":{"bookmark_id":"bookmark-1"}}`)
	if !strings.Contains(response, "authentication_required") {
		t.Fatalf("delete before sign-in = %q, want authentication_required", response)
	}
}
