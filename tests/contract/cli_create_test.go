//go:build contract

package contract_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/jsonl"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func newCreateServer() (*jsonl.Server, *testkit.BookmarkWriter) {
	clock := testkit.NewClock(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	authenticator := usecase.NewAuthenticator(usecase.AuthenticationDependencies{
		Clock:         clock,
		Passwords:     &testkit.PasswordVerifier{ExpectedHash: "hash", ExpectedPassword: "correct horse"},
		Administrator: model.Administrator{Username: "admin", PasswordHash: "hash"},
	})
	drafts := testkit.NewCaptureDraftStore()
	files := testkit.NewScreenshotFileStore()
	writer := &testkit.BookmarkWriter{}
	server := jsonl.NewServer(jsonl.Dependencies{
		Authenticator: authenticator,
		CaptureService: usecase.NewCaptureService(usecase.CaptureDependencies{
			Capturer: &testkit.ScreenshotCapturer{}, Files: files, Drafts: drafts, Clock: clock,
			IDs: testkit.NewIDGenerator("capture"),
		}),
		Creator: usecase.NewCreator(usecase.CreateDependencies{Bookmarks: writer, Drafts: drafts, Files: files, Clock: clock}),
	})
	return server, writer
}

func serveStream(t *testing.T, server *jsonl.Server, inputLines []string) (string, string) {
	t.Helper()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	input := strings.Join(inputLines, "\n") + "\n"
	if err := server.Serve(strings.NewReader(input), stdout, stderr); err != nil {
		t.Fatalf("serve JSON Lines: %v", err)
	}
	return stdout.String(), stderr.String()
}

func serveLines(t *testing.T, server *jsonl.Server, input string) string {
	t.Helper()
	stdout, _ := serveStream(t, server, []string{input})
	return strings.TrimSpace(stdout)
}

func TestCLICreateProtectedOperationsRequireStreamAuthentication(t *testing.T) {
	server, _ := newCreateServer()
	response := serveLines(t, server, `{"version":"v1","id":"capture-before","operation":"capture.start","payload":{"url":"https://example.test/page"}}`)
	if !strings.Contains(response, "authentication_required") {
		t.Fatalf("capture before sign-in = %q, want authentication_required", response)
	}
}

func TestCLICreateSignInCaptureAndBookmarkCreateDiscloseNoTokenAndPreserveOrder(t *testing.T) {
	server, writer := newCreateServer()

	stdout, stderr := serveStream(t, server, []string{
		`{"version":"v1","id":"login-1","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"capture-1","operation":"capture.start","payload":{"url":"https://example.test/page"}}`,
		`{"version":"v1","id":"create-1","operation":"bookmark.create","payload":{"url":"https://example.test/page","description":"A page","tags":["Go","go"],"capture_id":"capture-1"}}`,
		`{"version":"v1","id":"logout-1","operation":"auth.sign_out","payload":{}}`,
	})
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 4 {
		t.Fatalf("stdout lines = %d, want 4 (one response per request): %q", len(lines), stdout)
	}

	var loginResponse struct {
		OK     bool
		Result map[string]any
	}
	if err := json.Unmarshal([]byte(lines[0]), &loginResponse); err != nil || !loginResponse.OK {
		t.Fatalf("sign-in response = %q (err=%v), want success", lines[0], err)
	}
	if _, hasToken := loginResponse.Result["token"]; hasToken {
		t.Fatal("sign-in response must never disclose a reusable stream session token")
	}

	var captureResponse struct {
		OK     bool
		Result struct {
			CaptureID string `json:"capture_id"`
			State     string `json:"state"`
		}
	}
	if err := json.Unmarshal([]byte(lines[1]), &captureResponse); err != nil {
		t.Fatalf("decode capture response: %v; %q", err, lines[1])
	}
	if !captureResponse.OK || captureResponse.Result.State != "ready" || captureResponse.Result.CaptureID != "capture-1" {
		t.Fatalf("capture response = %#v, want ready with predictable id", captureResponse)
	}

	var createResponse struct {
		OK     bool
		Result struct{ ID string }
	}
	if err := json.Unmarshal([]byte(lines[2]), &createResponse); err != nil || !createResponse.OK || createResponse.Result.ID == "" {
		t.Fatalf("bookmark.create response = %q (err=%v), want a created bookmark id", lines[2], err)
	}
	if len(writer.Created) != 1 || len(writer.Created[0].Tags) != 1 || writer.Created[0].ScreenshotStorageKey == "" {
		t.Fatalf("created bookmarks = %#v, want one bookmark with normalized tags and a promoted screenshot", writer.Created)
	}

	var logoutResponse struct {
		OK     bool
		Result struct{ Authenticated bool }
	}
	if err := json.Unmarshal([]byte(lines[3]), &logoutResponse); err != nil || !logoutResponse.OK || logoutResponse.Result.Authenticated {
		t.Fatalf("sign-out response = %q (err=%v), want authenticated:false", lines[3], err)
	}
	if strings.Contains(stderr, "correct horse") {
		t.Fatalf("stderr disclosed password: %q", stderr)
	}
}

func TestCLICreateValidatesFieldsAndRejectsInvalidURL(t *testing.T) {
	server, _ := newCreateServer()
	stdout, _ := serveStream(t, server, []string{
		`{"version":"v1","id":"login-1","operation":"auth.sign_in","payload":{"username":"admin","password":"correct horse"}}`,
		`{"version":"v1","id":"create-invalid","operation":"bookmark.create","payload":{"url":"not a url","description":"x","save_without_screenshot":true}}`,
	})
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], "invalid_url") {
		t.Fatalf("invalid URL create response = %q, want invalid_url", stdout)
	}
}
