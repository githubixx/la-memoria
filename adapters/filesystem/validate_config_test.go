package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

func TestConfigurationValidatorAcceptsContainedPNGFavicon(t *testing.T) {
	assetRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(assetRoot, "images"), 0o700); err != nil {
		t.Fatalf("create asset directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetRoot, "images", "favicon.png"), pngSignature, 0o600); err != nil {
		t.Fatalf("write favicon: %v", err)
	}

	configuration := model.Configuration{
		Branding:    model.BrandingConfig{FaviconPath: "images/favicon.png"},
		Screenshots: model.ScreenshotConfig{Root: t.TempDir()},
	}
	if err := NewConfigurationValidator(assetRoot).Validate(context.Background(), configuration); err != nil {
		t.Fatalf("contained PNG favicon was rejected: %v", err)
	}
}

func TestConfigurationValidatorRejectsFaviconSymlinkOutsideAssetRoot(t *testing.T) {
	assetRoot := t.TempDir()
	outside := filepath.Join(t.TempDir(), "favicon.png")
	if err := os.WriteFile(outside, pngSignature, 0o600); err != nil {
		t.Fatalf("write external favicon: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(assetRoot, "favicon.png")); err != nil {
		t.Fatalf("create favicon symlink: %v", err)
	}

	configuration := model.Configuration{
		Branding:    model.BrandingConfig{FaviconPath: "favicon.png"},
		Screenshots: model.ScreenshotConfig{Root: t.TempDir()},
	}
	if err := NewConfigurationValidator(assetRoot).Validate(context.Background(), configuration); err == nil {
		t.Fatal("favicon symlink outside the asset root was accepted")
	}
}
