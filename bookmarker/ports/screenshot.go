package ports

import "context"

// ProcessResult is the captured stdout/stderr of one external process
// invocation.
type ProcessResult struct {
	Stdout string
	Stderr string
}

// ProcessRunner executes one external command to completion or until ctx
// is done, returning its captured output.
type ProcessRunner interface {
	Run(ctx context.Context, arguments ...string) (ProcessResult, error)
}

// CaptureResult is the outcome of one successful screenshot capture
// attempt.
type CaptureResult struct {
	StagedKey string
	FinalURL  string
}

// ScreenshotCapturer runs one bounded screenshot capture attempt for a
// unique session and stages its output at stagedPath.
type ScreenshotCapturer interface {
	Capture(ctx context.Context, sessionID, targetURL, stagedPath string) (CaptureResult, error)
}

// ScreenshotFileStore manages screenshot files staged before a bookmark
// write commits and promoted or removed after it does.
type ScreenshotFileStore interface {
	// StagingPath returns a unique absolute path for a new staged capture
	// under the configured root.
	StagingPath(sessionID string) string
	// Validate confirms a staged file exists and is a supported image
	// format.
	Validate(ctx context.Context, stagedPath string) error
	// Read returns the bytes of a staged or promoted screenshot for serving.
	Read(ctx context.Context, storageKey string) ([]byte, error)
	// Promote atomically moves a validated staged file into permanent
	// storage, returning its durable storage key.
	Promote(ctx context.Context, stagedPath string) (storageKey string, err error)
	// Discard removes a staged file that will not be used.
	Discard(ctx context.Context, stagedPath string) error
	// Remove deletes a promoted file, used for replacement/deletion
	// cleanup.
	Remove(ctx context.Context, storageKey string) error
	// Quarantine atomically hides a promoted file before a delete transaction.
	Quarantine(ctx context.Context, storageKey string) (quarantinedKey string, err error)
	// Restore moves a quarantined file back after a failed database deletion.
	Restore(ctx context.Context, quarantinedKey, storageKey string) error
}
