//go:build contract

package contract_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/githubixx/la-memoria/adapters/jsonl"
)

func TestJSONLProtocolRejectsInvalidEnvelopesAndOversizedLines(t *testing.T) {
	tests := []struct {
		input string
		code  string
	}{
		{`{"version":"v2","id":"one","operation":"bookmark.list","payload":{}}`, "unsupported_version"},
		{`{"version":"v1","id":"one","operation":"bookmark.list","payload":{},"extra":true}`, "unknown_field"},
		{`{"version":"v1","id":"one","operation":"missing","payload":{}}`, "unknown_operation"},
		{`{"version":"v1","id":"one","operation":"bookmark.list","payload":{}} {}`, "invalid_json"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			stdout, _ := runProtocol(t, test.input+"\n")
			var response struct{ Error struct{ Code string } }
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v; stdout=%q", err, stdout.String())
			}
			if response.Error.Code != test.code {
				t.Fatalf("error code = %q, want %q", response.Error.Code, test.code)
			}
		})
	}

	stdout, _ := runProtocol(t, strings.Repeat("x", jsonl.MaximumLineBytes+1)+"\n")
	if !bytes.Contains(stdout.Bytes(), []byte(`"line_too_large"`)) {
		t.Fatalf("oversized line response = %q, want line_too_large", stdout.String())
	}
}

func TestJSONLProtocolPreservesOrderStreamAuthenticationAndStdoutPurity(t *testing.T) {
	input := strings.Join([]string{
		`{"version":"v1","id":"first","operation":"bookmark.list","payload":{"page":1}}`,
		`{"version":"v1","id":"login","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"logout","operation":"auth.sign_out","payload":{}}`,
	}, "\n") + "\n"
	stdout, stderr := runProtocol(t, input)
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("stdout lines = %d, want exactly one response per request: %q", len(lines), stdout.String())
	}
	for index, id := range []string{"first", "login", "logout"} {
		var response struct {
			ID     string
			Result map[string]any
		}
		if err := json.Unmarshal([]byte(lines[index]), &response); err != nil {
			t.Fatalf("decode response %d: %v", index, err)
		}
		if response.ID != id {
			t.Fatalf("response %d id = %q, want %q", index, response.ID, id)
		}
		if _, hasToken := response.Result["token"]; hasToken {
			t.Fatal("stdout must never disclose a reusable stream session token")
		}
	}
	if strings.Contains(stderr.String(), "correct horse") {
		t.Fatalf("stderr disclosed password: %q", stderr.String())
	}
}

func runProtocol(t *testing.T, input string) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := jsonl.NewServer(jsonl.Dependencies{}).Serve(strings.NewReader(input), stdout, stderr); err != nil {
		t.Fatalf("serve JSON Lines: %v", err)
	}
	return stdout, stderr
}
