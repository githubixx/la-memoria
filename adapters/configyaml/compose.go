package configyaml

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"gopkg.in/yaml.v3"
)

var composeUsername = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// InitializeCompose imports once or refreshes deployment-owned credentials in
// an existing configuration. The same lock and atomic commit as updates are
// used; explicitly requested imports never fall back to a template.
func InitializeCompose(ctx context.Context, target, template, importPath, username, verifier string, environment Environment) error {
	if !composeUsername.MatchString(username) || !validPHC(verifier) {
		return errors.New("invalid deployment administrator credentials")
	}
	store := NewStore(target, environment)
	return store.WithLock(ctx, func() error {
		_, statErr := os.Stat(target)
		exists := statErr == nil
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return errors.New("cannot inspect configuration volume")
		}
		if exists && importPath != "" {
			return errors.New("configuration volume already initialized; remove the import setting")
		}
		var configuration model.Configuration
		var err error
		switch {
		case exists:
			configuration, err = store.Load(ctx)
		case importPath != "":
			info, statErr := os.Stat(importPath)
			if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o400 == 0 || info.Mode().Perm()&0o077 != 0 {
				return errors.New("requested configuration import is unavailable or not owner-only")
			}
			configuration, err = Load(importPath, environment)
		default:
			configuration, err = loadComposeTemplate(template)
		}
		if err != nil {
			return errors.New("configuration source is invalid or unavailable")
		}
		configuration.Administrator.Username = username
		configuration.Administrator.PasswordHash = verifier
		configuration.Database.Password = environment[configuration.Database.PasswordEnv]
		if configuration.DefaultView == "" {
			configuration.DefaultView = "list"
		}
		if configuration.Search.PageSize == 0 {
			configuration.Search.PageSize = 10
		}
		if configuration.Search.MaximumResults == 0 {
			configuration.Search.MaximumResults = 10000
		}
		if err := store.Validate(ctx, configuration); err != nil {
			return errors.New("configuration source failed validation")
		}
		if err := store.writeConfiguration(configuration); err != nil {
			return fmt.Errorf("commit configuration: %w", err)
		}
		return nil
	})
}

func loadComposeTemplate(path string) (model.Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return model.Configuration{}, err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var configuration model.Configuration
	if err := decoder.Decode(&configuration); err != nil {
		return model.Configuration{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return model.Configuration{}, errors.New("template must contain one document")
	}
	return configuration, nil
}
