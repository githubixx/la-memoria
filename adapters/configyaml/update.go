package configyaml

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"gopkg.in/yaml.v3"
)

type StagedConfigurationWriter func(*os.File, []byte) (int, error)

type FilesystemHooks struct {
	SyncStaged    func(*os.File) error
	CloseStaged   func(*os.File) error
	OpenDirectory func(string) (*os.File, error)
	Rename        func(string, string) error
	SyncDirectory func(*os.File) error
	Warning       func(string)
}

func (store *Store) Activate(ctx context.Context, configuration model.Configuration) error {
	configuration.Database.Password = store.environment[configuration.Database.PasswordEnv]
	if err := store.Validate(ctx, configuration); err != nil {
		return err
	}
	if info, err := os.Stat(store.path); err != nil {
		return err
	} else if info.Mode().Perm()&0o077 != 0 {
		return &loadError{"unsafe_config_permissions", "configuration permissions must be owner-only"}
	}
	return store.writeConfiguration(configuration)
}

// writeConfiguration performs the staged atomic commit. Its caller validates
// the complete candidate and serializes writers before invoking it.
func (store *Store) writeConfiguration(configuration model.Configuration) error {
	encoded, err := yaml.Marshal(configuration)
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	directory := filepath.Dir(store.path)
	temporary, err := os.CreateTemp(directory, ".config-*")
	if err != nil {
		return fmt.Errorf("create staged configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect staged configuration: %w", err)
	}
	writer := store.stagedConfigurationWriter
	if writer == nil {
		writer = func(file *os.File, contents []byte) (int, error) { return file.Write(contents) }
	}
	n, err := writer(temporary, encoded)
	if err == nil && n != len(encoded) {
		err = io.ErrShortWrite
	}
	if err != nil {
		temporary.Close()
		return fmt.Errorf("write staged configuration: %w", err)
	}
	syncStaged := store.filesystemHooks.SyncStaged
	if syncStaged == nil {
		syncStaged = (*os.File).Sync
	}
	if err := syncStaged(temporary); err != nil {
		temporary.Close()
		return fmt.Errorf("sync staged configuration: %w", err)
	}
	closeStaged := store.filesystemHooks.CloseStaged
	if closeStaged == nil {
		closeStaged = (*os.File).Close
	}
	if err := closeStaged(temporary); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("close staged configuration: %w", err)
	}
	openDirectory := store.filesystemHooks.OpenDirectory
	if openDirectory == nil {
		openDirectory = os.Open
	}
	directoryHandle, err := openDirectory(directory)
	if err != nil {
		return fmt.Errorf("open configuration directory: %w", err)
	}
	defer directoryHandle.Close()
	rename := store.filesystemHooks.Rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(temporaryPath, store.path); err != nil {
		return fmt.Errorf("activate configuration: %w", err)
	}
	syncDirectory := store.filesystemHooks.SyncDirectory
	if syncDirectory == nil {
		syncDirectory = (*os.File).Sync
	}
	if err := syncDirectory(directoryHandle); err != nil {
		const warning = "configuration committed but directory durability sync failed"
		if store.filesystemHooks.Warning != nil {
			store.filesystemHooks.Warning(warning)
		} else {
			slog.Warn(warning)
		}
	}
	return nil
}
