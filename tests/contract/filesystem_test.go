//go:build contract

package contract_test

import (
	"context"
	"os"
	"testing"

	"github.com/githubixx/la-memoria/adapters/filesystem"
)

func newScreenshotStore(t *testing.T) *filesystem.ScreenshotStore {
	t.Helper()
	store, err := filesystem.NewScreenshotStore(t.TempDir())
	if err != nil {
		t.Fatalf("new screenshot store: %v", err)
	}
	return store
}

func writePNG(t *testing.T, path string) {
	t.Helper()
	// Deliberately does not create the parent directory: StagingPath must
	// only ever return paths under directories NewScreenshotStore already
	// created, since agent-browser cannot create missing directories.
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, []byte("fake-image-data")...)
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatalf("write staged PNG: %v", err)
	}
}

func TestScreenshotStoreStagesValidatesAndPromotesAtomically(t *testing.T) {
	store := newScreenshotStore(t)
	ctx := context.Background()

	stagedPath := store.StagingPath("session-1")
	writePNG(t, stagedPath)

	if err := store.Validate(ctx, stagedPath); err != nil {
		t.Fatalf("validate staged PNG: %v", err)
	}

	storageKey, err := store.Promote(ctx, stagedPath)
	if err != nil {
		t.Fatalf("promote staged file: %v", err)
	}
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatal("promotion must move the staged file, not copy it")
	}
	data, err := store.Read(ctx, storageKey)
	if err != nil || len(data) == 0 {
		t.Fatalf("read promoted file: data=%d err=%v", len(data), err)
	}
}

func TestScreenshotStoreRejectsNonImageStagedFiles(t *testing.T) {
	store := newScreenshotStore(t)
	ctx := context.Background()

	stagedPath := store.StagingPath("session-1")
	if err := os.WriteFile(stagedPath, []byte("not a png"), 0o600); err != nil {
		t.Fatalf("write non-image staged file: %v", err)
	}

	if err := store.Validate(ctx, stagedPath); err == nil {
		t.Fatal("validate must reject a non-PNG staged file")
	}
}

func TestScreenshotStoreDiscardsAndRemovesFiles(t *testing.T) {
	store := newScreenshotStore(t)
	ctx := context.Background()

	stagedPath := store.StagingPath("session-1")
	writePNG(t, stagedPath)
	if err := store.Discard(ctx, stagedPath); err != nil {
		t.Fatalf("discard staged file: %v", err)
	}
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatal("discard must remove the staged file")
	}
	// Discarding an already-removed file must not error (idempotent cleanup).
	if err := store.Discard(ctx, stagedPath); err != nil {
		t.Fatalf("discard missing file: %v", err)
	}

	secondStagedPath := store.StagingPath("session-2")
	writePNG(t, secondStagedPath)
	storageKey, err := store.Promote(ctx, secondStagedPath)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if err := store.Remove(ctx, storageKey); err != nil {
		t.Fatalf("remove promoted file: %v", err)
	}
	if _, err := store.Read(ctx, storageKey); err == nil {
		t.Fatal("read after remove must fail")
	}
}

func TestScreenshotStoreRejectsPathsThatEscapeTheConfiguredRoot(t *testing.T) {
	store := newScreenshotStore(t)
	ctx := context.Background()

	for _, escapingPath := range []string{"../outside.png", "/etc/passwd", "staging/../../outside.png"} {
		if err := store.Validate(ctx, escapingPath); err == nil {
			t.Fatalf("validate(%q) must reject paths escaping the root", escapingPath)
		}
	}
}

func TestScreenshotStoreReadReportsMissingFiles(t *testing.T) {
	store := newScreenshotStore(t)
	if _, err := store.Read(context.Background(), "promoted/does-not-exist.png"); err == nil {
		t.Fatal("read must fail for a missing promoted file")
	}
}
