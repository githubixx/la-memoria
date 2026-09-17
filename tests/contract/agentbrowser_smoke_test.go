//go:build contract

package contract_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/agentbrowser"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/tests/fixtures/capture"
)

func TestAgentBrowserSmokeCapturesFixtureAndClosesSession(t *testing.T) {
	if os.Getenv("BOOKMARKER_AGENT_BROWSER_SMOKE") != "1" {
		t.Skip("set BOOKMARKER_AGENT_BROWSER_SMOKE=1 to run the real agent-browser smoke test")
	}
	if _, err := exec.LookPath("agent-browser"); err != nil {
		t.Skip("agent-browser is not available")
	}
	server := capture.NewServer()
	t.Cleanup(server.Close)
	directory := t.TempDir()
	runner := &recordingProcessRunner{}
	capturer := agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: runner, Timeout: 15 * time.Second})

	stagedPath := filepath.Join(directory, "capture.png")
	result, err := capturer.Capture(context.Background(), "bookmarker-contract-smoke", server.URL+"/capture-target", stagedPath)
	if err != nil {
		t.Fatalf("capture fixture: %v", err)
	}
	if result.FinalURL != server.URL+"/capture-target" {
		t.Fatalf("final URL = %q", result.FinalURL)
	}
	info, err := os.Stat(stagedPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("captured PNG = %v, size %d", err, info.Size())
	}
	if !runner.closed("bookmarker-contract-smoke") {
		t.Fatal("capture did not issue an agent-browser close command")
	}
}

type recordingProcessRunner struct {
	mutex sync.Mutex
	calls [][]string
}

func (runner *recordingProcessRunner) Run(ctx context.Context, arguments ...string) (ports.ProcessResult, error) {
	runner.mutex.Lock()
	runner.calls = append(runner.calls, append([]string(nil), arguments...))
	runner.mutex.Unlock()
	command := exec.CommandContext(ctx, "agent-browser", arguments...)
	stdout, err := command.Output()
	if exitError, ok := err.(*exec.ExitError); ok {
		return ports.ProcessResult{Stdout: string(stdout), Stderr: string(exitError.Stderr)}, err
	}
	return ports.ProcessResult{Stdout: string(stdout)}, err
}

func (runner *recordingProcessRunner) closed(sessionID string) bool {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	for _, arguments := range runner.calls {
		if len(arguments) == 4 && arguments[1] == sessionID && arguments[3] == "close" {
			return true
		}
	}
	return false
}
