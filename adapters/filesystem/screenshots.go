// Package filesystem implements ports.ScreenshotFileStore: contained
// staging under a validated root, PNG validation, atomic promotion,
// discard, and removal.
package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// ScreenshotStore implements ports.ScreenshotFileStore under one
// configured, validated root directory.
type ScreenshotStore struct {
	root string
}

func NewScreenshotStore(root string) (*ScreenshotStore, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve screenshot root: %w", err)
	}
	if err := os.MkdirAll(absoluteRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create screenshot root: %w", err)
	}
	// The staging directory must exist upfront: agent-browser writes the
	// screenshot file directly to a path under it and cannot create
	// missing parent directories itself.
	if err := os.MkdirAll(filepath.Join(absoluteRoot, "staging"), 0o700); err != nil {
		return nil, fmt.Errorf("create screenshot staging directory: %w", err)
	}
	return &ScreenshotStore{root: absoluteRoot}, nil
}

// StagingPath returns a unique absolute path for a new staged capture
// under a "staging" subdirectory of the configured root.
func (store *ScreenshotStore) StagingPath(sessionID string) string {
	return filepath.Join(store.root, "staging", sanitizeComponent(sessionID)+"-"+uuid.NewString()+".png")
}

func (store *ScreenshotStore) Validate(_ context.Context, stagedPath string) error {
	resolved, err := store.resolve(stagedPath)
	if err != nil {
		return err
	}
	header := make([]byte, len(pngSignature))
	file, err := os.Open(resolved)
	if err != nil {
		return &notFoundError{path: stagedPath}
	}
	defer file.Close()
	if _, err := file.Read(header); err != nil || !bytes.Equal(header, pngSignature) {
		return &invalidImageError{path: stagedPath}
	}
	return nil
}

// Promote atomically renames a validated staged file into the permanent
// "promoted" directory, returning its durable storage key (a root-relative
// path).
func (store *ScreenshotStore) Promote(_ context.Context, stagedPath string) (string, error) {
	resolved, err := store.resolve(stagedPath)
	if err != nil {
		return "", err
	}
	storageKey := filepath.Join("promoted", uuid.NewString()+".png")
	destination := filepath.Join(store.root, storageKey)
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", fmt.Errorf("create promoted directory: %w", err)
	}
	if err := os.Rename(resolved, destination); err != nil {
		return "", fmt.Errorf("promote staged file: %w", err)
	}
	return storageKey, nil
}

func (store *ScreenshotStore) Discard(_ context.Context, stagedPath string) error {
	resolved, err := store.resolve(stagedPath)
	if err != nil {
		return err
	}
	if err := os.Remove(resolved); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("discard staged file: %w", err)
	}
	return nil
}

func (store *ScreenshotStore) Remove(_ context.Context, storageKey string) error {
	resolved, err := store.resolve(storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(resolved); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove promoted file: %w", err)
	}
	return nil
}

// Quarantine atomically moves a promoted screenshot out of its public key so
// a failed database deletion can restore it without a visible orphan.
func (store *ScreenshotStore) Quarantine(_ context.Context, storageKey string) (string, error) {
	source, err := store.resolve(storageKey)
	if err != nil {
		return "", err
	}
	quarantinedKey := filepath.Join("quarantine", uuid.NewString()+filepath.Ext(storageKey))
	destination := filepath.Join(store.root, quarantinedKey)
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", fmt.Errorf("create quarantine directory: %w", err)
	}
	if err := os.Rename(source, destination); err != nil {
		return "", fmt.Errorf("quarantine promoted file: %w", err)
	}
	return quarantinedKey, nil
}

// Restore atomically moves a quarantined screenshot back to its original
// public storage key after the authoritative database change failed.
func (store *ScreenshotStore) Restore(_ context.Context, quarantinedKey, storageKey string) error {
	source, err := store.resolve(quarantinedKey)
	if err != nil {
		return err
	}
	destination, err := store.resolve(storageKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create restore directory: %w", err)
	}
	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("restore quarantined file: %w", err)
	}
	return nil
}

// Read returns the bytes of a promoted file for serving, or a not-found
// error if it is missing or unreadable.
func (store *ScreenshotStore) Read(_ context.Context, storageKey string) ([]byte, error) {
	resolved, err := store.resolve(storageKey)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, &notFoundError{path: storageKey}
	}
	return data, nil
}

// resolve joins a relative key to the root and rejects any path that
// would escape it (containment against traversal and absolute paths).
func (store *ScreenshotStore) resolve(key string) (string, error) {
	joined := key
	if !filepath.IsAbs(key) {
		joined = filepath.Join(store.root, key)
	}
	cleaned := filepath.Clean(joined)
	if cleaned != store.root && !strings.HasPrefix(cleaned, store.root+string(filepath.Separator)) {
		return "", &containmentError{path: key}
	}
	return cleaned, nil
}

func sanitizeComponent(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	return replacer.Replace(value)
}

type containmentError struct{ path string }

func (err *containmentError) Error() string {
	return fmt.Sprintf("path %q escapes the screenshot root", err.path)
}

type notFoundError struct{ path string }

func (err *notFoundError) Error() string { return fmt.Sprintf("file %q was not found", err.path) }

type invalidImageError struct{ path string }

func (err *invalidImageError) Error() string {
	return fmt.Sprintf("file %q is not a supported image", err.path)
}
