package configyaml

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"gopkg.in/yaml.v3"
)

type StagedConfigurationWriter func(*os.File, []byte) (int, error)

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
	if _, err := writer(temporary, encoded); err != nil {
		temporary.Close()
		return fmt.Errorf("write staged configuration: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync staged configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close staged configuration: %w", err)
	}
	if err := os.Rename(temporaryPath, store.path); err != nil {
		return fmt.Errorf("activate configuration: %w", err)
	}
	if err := os.Chmod(store.path, 0o600); err != nil {
		return fmt.Errorf("protect activated configuration: %w", err)
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open configuration directory: %w", err)
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync configuration directory: %w", err)
	}
	return nil
}
