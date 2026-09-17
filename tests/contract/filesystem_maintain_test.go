//go:build contract

package contract_test

import (
	"context"
	"os"
	"testing"
)

func TestScreenshotStoreQuarantinesAndRestoresPromotedFiles(t *testing.T) {
	store := newScreenshotStore(t)
	ctx := context.Background()
	stagedPath := store.StagingPath("session-1")
	writePNG(t, stagedPath)
	storageKey, err := store.Promote(ctx, stagedPath)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	quarantinedKey, err := store.Quarantine(ctx, storageKey)
	if err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	if _, err := store.Read(ctx, storageKey); err == nil {
		t.Fatal("quarantine must hide the promoted file before database deletion")
	}
	if err := store.Restore(ctx, quarantinedKey, storageKey); err != nil {
		t.Fatalf("restore after database failure: %v", err)
	}
	if _, err := store.Read(ctx, storageKey); err != nil {
		t.Fatalf("restored screenshot unavailable: %v", err)
	}
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatal("promotion must not leave the staged file behind")
	}
}
