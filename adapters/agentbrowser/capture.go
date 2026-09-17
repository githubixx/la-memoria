package agentbrowser

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// Dependencies configures the agent-browser screenshot capture adapter.
type Dependencies struct {
	Runner        ports.ProcessRunner
	Timeout       time.Duration
	MaxConcurrent int
}

// Capturer implements ports.ScreenshotCapturer around the external
// agent-browser executable: unique sessions, bounded timeout, bounded
// global concurrency, structured JSON output, and a guaranteed close.
type Capturer struct {
	dependencies Dependencies
	semaphore    chan struct{}
}

func NewCapturer(dependencies Dependencies) *Capturer {
	if dependencies.Timeout == 0 {
		dependencies.Timeout = 15 * time.Second
	}
	if dependencies.MaxConcurrent == 0 {
		dependencies.MaxConcurrent = 4
	}
	return &Capturer{dependencies: dependencies, semaphore: make(chan struct{}, dependencies.MaxConcurrent)}
}

var ErrCapacityLimited = &model.ApplicationError{Code: "capacity_limited", Message: "capture capacity is currently full", Retryable: true}

func (capturer *Capturer) Capture(ctx context.Context, sessionID, targetURL, stagedPath string) (ports.CaptureResult, error) {
	select {
	case capturer.semaphore <- struct{}{}:
		defer func() { <-capturer.semaphore }()
	default:
		return ports.CaptureResult{}, ErrCapacityLimited
	}

	boundedCtx, cancel := context.WithTimeout(ctx, capturer.dependencies.Timeout)
	defer cancel()

	defer func() {
		_, _ = capturer.dependencies.Runner.Run(context.Background(), "--session", sessionID, "--json", "close")
	}()

	openOutput, err := capturer.run(boundedCtx, "--session", sessionID, "--json", "open", targetURL)
	if err != nil {
		return ports.CaptureResult{}, err
	}
	if _, err := capturer.run(boundedCtx, "--session", sessionID, "--json", "wait", "--load", "networkidle"); err != nil {
		return ports.CaptureResult{}, err
	}
	if _, err := capturer.run(boundedCtx, "--session", sessionID, "--json", "screenshot", stagedPath, "--full"); err != nil {
		return ports.CaptureResult{}, err
	}

	finalURL := targetURL
	if data, ok := openOutput["data"].(map[string]any); ok {
		if url, ok := data["url"].(string); ok && url != "" {
			finalURL = url
		}
	}
	return ports.CaptureResult{StagedKey: stagedPath, FinalURL: finalURL}, nil
}

// run executes one agent-browser subcommand and returns its decoded
// output. agent-browser reports outcome as top-level "success" (bool) and
// "data" (object); it does not use an "ok" field.
func (capturer *Capturer) run(ctx context.Context, arguments ...string) (map[string]any, error) {
	output, err := capturer.dependencies.Runner.Run(ctx, arguments...)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, &model.ApplicationError{Code: "timeout", Message: "capture timed out", Retryable: true}
		}
		return nil, &model.ApplicationError{Code: "unreachable", Message: "capture destination could not be reached"}
	}
	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(output.Stdout), &parsed); jsonErr != nil {
		return nil, &model.ApplicationError{Code: "invalid_output", Message: "capture process returned malformed output"}
	}
	if success, _ := parsed["success"].(bool); !success {
		return nil, &model.ApplicationError{Code: "capture_failed", Message: "capture reported failure"}
	}
	return parsed, nil
}
