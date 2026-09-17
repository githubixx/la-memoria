//go:build contract

package contract_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/adapters/agentbrowser"
	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/tests/testkit"
)

func TestAgentBrowserCaptureUsesExactArgumentArraysAndGuaranteesClose(t *testing.T) {
	process := &testkit.CaptureProcess{Result: ports.ProcessResult{Stdout: `{"success":true,"data":{"url":"https://example.test/final"}}`}}
	capturer := agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: process})

	result, err := capturer.Capture(context.Background(), "session-1", "https://example.test/start", "/tmp/staged.png")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if result.FinalURL != "https://example.test/final" || result.StagedKey != "/tmp/staged.png" {
		t.Fatalf("capture result = %#v, unexpected", result)
	}
	if len(process.Calls) != 4 {
		t.Fatalf("process calls = %#v, want open, wait, screenshot, close", process.Calls)
	}
	expectedCommands := [][]string{
		{"--session", "session-1", "--json", "open", "https://example.test/start"},
		{"--session", "session-1", "--json", "wait", "--load", "networkidle"},
		{"--session", "session-1", "--json", "screenshot", "/tmp/staged.png", "--full"},
		{"--session", "session-1", "--json", "close"},
	}
	for index, expected := range expectedCommands {
		if len(process.Calls[index]) != len(expected) {
			t.Fatalf("call %d = %#v, want %#v", index, process.Calls[index], expected)
		}
		for position, argument := range expected {
			if process.Calls[index][position] != argument {
				t.Fatalf("call %d = %#v, want %#v", index, process.Calls[index], expected)
			}
		}
	}
}

func TestAgentBrowserCaptureClosesSessionEvenAfterAnEarlierStepFails(t *testing.T) {
	process := &testkit.CaptureProcess{RunFunc: func(_ context.Context, arguments ...string) (ports.ProcessResult, error) {
		if len(arguments) > 3 && arguments[3] == "open" {
			return ports.ProcessResult{Stdout: `{"success":false}`}, nil
		}
		return ports.ProcessResult{Stdout: `{"success":true}`}, nil
	}}
	capturer := agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: process})

	_, err := capturer.Capture(context.Background(), "session-1", "https://example.test/start", "/tmp/staged.png")
	if model.ErrorCode(err) != "capture_failed" {
		t.Fatalf("error = %v, want capture_failed", err)
	}
	if len(process.Calls) != 2 {
		t.Fatalf("process calls = %#v, want exactly open then a guaranteed close", process.Calls)
	}
	if process.Calls[1][len(process.Calls[1])-1] != "close" {
		t.Fatalf("second call = %#v, want close", process.Calls[1])
	}
}

func TestAgentBrowserCaptureReportsMalformedOutputAndTimeout(t *testing.T) {
	malformed := &testkit.CaptureProcess{Result: ports.ProcessResult{Stdout: "not json"}}
	capturer := agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: malformed})
	_, err := capturer.Capture(context.Background(), "session-1", "https://example.test/start", "/tmp/staged.png")
	if model.ErrorCode(err) != "invalid_output" {
		t.Fatalf("malformed output error = %v, want invalid_output", err)
	}

	slow := &testkit.CaptureProcess{RunFunc: func(ctx context.Context, arguments ...string) (ports.ProcessResult, error) {
		if len(arguments) > 0 && arguments[len(arguments)-1] == "close" {
			return ports.ProcessResult{Stdout: `{"success":true}`}, nil
		}
		<-ctx.Done()
		return ports.ProcessResult{}, ctx.Err()
	}}
	timeoutCapturer := agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: slow, Timeout: 10 * time.Millisecond})
	_, err = timeoutCapturer.Capture(context.Background(), "session-1", "https://example.test/start", "/tmp/staged.png")
	if model.ErrorCode(err) != "timeout" {
		t.Fatalf("slow capture error = %v, want timeout", err)
	}
}

func TestAgentBrowserCaptureLimitsGlobalConcurrency(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	process := &testkit.CaptureProcess{RunFunc: func(ctx context.Context, arguments ...string) (ports.ProcessResult, error) {
		if len(arguments) > 3 && arguments[3] == "open" {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
		}
		return ports.ProcessResult{Stdout: `{"success":true}`}, nil
	}}
	capturer := agentbrowser.NewCapturer(agentbrowser.Dependencies{Runner: process, MaxConcurrent: 1})

	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		_, _ = capturer.Capture(context.Background(), "session-a", "https://example.test/a", "/tmp/a.png")
	}()
	<-started

	_, err := capturer.Capture(context.Background(), "session-b", "https://example.test/b", "/tmp/b.png")
	if model.ErrorCode(err) != "capacity_limited" {
		t.Fatalf("second concurrent capture error = %v, want capacity_limited", err)
	}
	close(release)
	group.Wait()
}
