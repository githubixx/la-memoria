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

func TestConfigurationActivationRejectsShortWriteWithoutChangingFile(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store := configyaml.NewStoreWithStagedWriter(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"}, func(file *os.File, contents []byte) (int, error) {
		return file.Write(contents[:len(contents)/2])
	})
	configuration, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	configuration.Branding.PageTitle = "Must Not Activate"
	if err := store.Activate(context.Background(), configuration); err == nil {
		t.Error("short write must fail before commit")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("short write changed the active configuration")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("configuration permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestConfigurationActivationCommitBoundaryFailures(t *testing.T) {
	injected := errors.New("injected filesystem failure")
	tests := []struct {
		name      string
		hooks     configyaml.FilesystemHooks
		committed bool
	}{
		{"staged sync", configyaml.FilesystemHooks{SyncStaged: func(*os.File) error { return injected }}, false},
		{"staged close", configyaml.FilesystemHooks{CloseStaged: func(f *os.File) error { _ = f.Close(); return injected }}, false},
		{"directory open", configyaml.FilesystemHooks{OpenDirectory: func(string) (*os.File, error) { return nil, injected }}, false},
		{"rename", configyaml.FilesystemHooks{Rename: func(string, string) error { return injected }}, false},
		{"directory sync", configyaml.FilesystemHooks{SyncDirectory: func(*os.File) error { return injected }}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfiguration(t, validConfiguration, 0o600)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var warnings []string
			tc.hooks.Warning = func(message string) { warnings = append(warnings, message) }
			store := configyaml.NewStoreWithFilesystemHooks(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"}, tc.hooks)
			current, err := store.Load(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			current.Branding.PageTitle = "Committed title"
			err = store.Activate(context.Background(), current)
			if tc.committed && err != nil {
				t.Fatalf("post-commit sync must report success: %v", err)
			}
			if !tc.committed && !errors.Is(err, injected) {
				t.Fatalf("pre-commit error = %v, want injected failure", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.committed {
				if string(after) == string(before) || len(warnings) != 1 || strings.Contains(warnings[0], "database-secret") {
					t.Fatalf("committed bytes/warning incorrect: changed=%v warnings=%v", string(after) != string(before), warnings)
				}
				if loaded, err := store.Load(context.Background()); err != nil || loaded.Branding.PageTitle != current.Branding.PageTitle {
					t.Fatalf("committed file unreadable or unchanged: %v", err)
				}
			} else if string(after) != string(before) || len(warnings) != 0 {
				t.Fatal("pre-commit failure changed file or emitted durability warning")
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("active file permissions: %v, %v", info, err)
			}
			staged, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*"))
			if err != nil || len(staged) != 0 {
				t.Fatalf("staging files not cleaned up: %v, %v", staged, err)
			}
		})
	}
}
func TestConfigurationActivationPreservesFileWhenStagedSyncFails(t *testing.T) {
	path := writeConfiguration(t, validConfiguration, 0o600)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store := configyaml.NewStoreWithStagedWriter(path, configyaml.Environment{"BOOKMARKER_DB_PASSWORD": "database-secret"}, func(file *os.File, contents []byte) (int, error) {
		n, err := file.Write(contents)
		if err != nil {
			return n, err
		}
		// A closed descriptor makes the subsequent real file Sync fail.
		if err := file.Close(); err != nil {
			return n, err
		}
		return n, nil
	})
	configuration, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	configuration.Branding.PageTitle = "Must Not Activate"
	if err := store.Activate(context.Background(), configuration); err == nil {
		t.Error("staged sync failure must fail before commit")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("staged sync failure changed the active configuration")
	}
	staged, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*"))
	if err != nil || len(staged) != 0 {
		t.Fatalf("staged files after sync failure = %q, %v; want none", staged, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("configuration permissions = %04o, want 0600", info.Mode().Perm())
	}
}
