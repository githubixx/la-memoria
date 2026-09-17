//go:build contract

package contract_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githubixx/la-memoria/adapters/configyaml"
	"github.com/githubixx/la-memoria/bookmarker/model"
)

func TestConfigurationStoreAtomicallyActivatesOnlyRedactedEditableFields(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	store := configyaml.NewStore(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
	current, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load initial configuration: %v", err)
	}
	candidate := current.Redacted()
	candidate.Branding.PageTitle = "Renamed Bookmarker"
	updated, err := model.ApplyEditableConfiguration(current, candidate)
	if err != nil {
		t.Fatalf("apply editable candidate: %v", err)
	}
	if err := store.Activate(context.Background(), updated); err != nil {
		t.Fatalf("activate configuration: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("activated file mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read activated configuration: %v", err)
	}
	if strings.Contains(string(contents), "database-secret") || !strings.Contains(string(contents), "password_env: BOOKMARKER_DB_PASSWORD") {
		t.Fatalf("activated configuration exposed or discarded environment reference: %s", contents)
	}
	reloaded, err := store.Load(context.Background())
	if err != nil || reloaded.Branding.PageTitle != "Renamed Bookmarker" || reloaded.Administrator.PasswordHash != current.Administrator.PasswordHash {
		t.Fatalf("reloaded configuration = %#v, %v", reloaded, err)
	}
}

func TestConfigurationStoreRejectsUnsafeExistingConfiguration(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o644)
	store := configyaml.NewStore(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
	current := model.Configuration{Administrator: model.Administrator{Username: "admin", PasswordHash: validPHC}, Branding: model.BrandingConfig{PageTitle: "Bookmarker"}, DefaultView: "list", Database: model.DatabaseConfig{Host: "127.0.0.1", Port: 5432, Name: "bookmarker", User: "bookmarker", PasswordEnv: "BOOKMARKER_DB_PASSWORD", TLSMode: "disable"}, Screenshots: model.ScreenshotConfig{Root: "/tmp/bookmarker-screenshots"}, Search: model.SearchConfig{PageSize: 10, MaximumResults: 10000}}
	if err := store.Activate(context.Background(), current); configyaml.ErrorCode(err) != "unsafe_config_permissions" {
		t.Fatalf("unsafe activation error = %v", err)
	}
}

func TestConfigurationActivationWritesAnOwnerOnlyAtomicReplacement(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	store := configyaml.NewStore(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"})
	configuration, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	configuration.Branding.PageTitle = "Updated Bookmarker"
	if err := store.Activate(context.Background(), configuration); err != nil {
		t.Fatalf("activate configuration: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat activated configuration: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("activated configuration mode = %04o, want 0600", info.Mode().Perm())
	}
	loaded, err := store.Load(context.Background())
	if err != nil || loaded.Branding.PageTitle != "Updated Bookmarker" || loaded.Database.Password != "database-secret" {
		t.Fatalf("activated configuration = %#v, %v", loaded, err)
	}
}

func TestConfigurationActivationPreservesTheLastValidFileWhenStagedWriteFails(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initial configuration: %v", err)
	}
	writeFailure := errors.New("injected staged write failure")
	store := configyaml.NewStoreWithStagedWriter(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"}, func(_ *os.File, _ []byte) (int, error) {
		return 0, writeFailure
	})
	configuration, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	configuration.Branding.PageTitle = "Must Not Activate"

	if err := store.Activate(context.Background(), configuration); !errors.Is(err, writeFailure) {
		t.Fatalf("activate error = %v, want injected write failure", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("active configuration changed after write failure: %q, %v", after, err)
	}
	staged, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*"))
	if err != nil || len(staged) != 0 {
		t.Fatalf("staged files = %q, %v; want cleanup after write failure", staged, err)
	}
}
