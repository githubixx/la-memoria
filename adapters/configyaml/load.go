package configyaml

import (
	"context"
	"errors"
	"io"
	"os"
	"regexp"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
	"gopkg.in/yaml.v3"
)

type Environment map[string]string
type loadError struct{ code, message string }

type Store struct {
	path                      string
	environment               Environment
	stagedConfigurationWriter StagedConfigurationWriter
}

func NewStore(path string, environment Environment) *Store {
	return NewStoreWithStagedWriter(path, environment, nil)
}

func NewStoreWithStagedWriter(path string, environment Environment, writer StagedConfigurationWriter) *Store {
	return &Store{path: path, environment: environment, stagedConfigurationWriter: writer}
}

func (store *Store) Load(context.Context) (model.Configuration, error) {
	return Load(store.path, store.environment)
}

func (store *Store) Validate(_ context.Context, configuration model.Configuration) error {
	if _, err := model.ValidateEditableConfiguration(configuration.Redacted()); err != nil {
		return err
	}
	if configuration.Administrator.Username == "" || !validPHC(configuration.Administrator.PasswordHash) {
		return &loadError{"invalid_password_hash", "administrator password hash is invalid"}
	}
	if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(configuration.Database.PasswordEnv) {
		return &loadError{"invalid_environment_reference", "database password environment reference is invalid"}
	}
	return nil
}

func (err *loadError) Error() string { return err.message }
func ErrorCode(err error) string {
	var target *loadError
	if errors.As(err, &target) {
		return target.code
	}
	return "internal_error"
}

func Load(path string, environment Environment) (model.Configuration, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return model.Configuration{}, &loadError{"config_not_found", "configuration file was not found"}
	}
	if err != nil {
		return model.Configuration{}, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return model.Configuration{}, &loadError{"unsafe_config_permissions", "configuration permissions must be owner-only"}
	}
	file, err := os.Open(path)
	if err != nil {
		return model.Configuration{}, err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var configuration model.Configuration
	if err := decoder.Decode(&configuration); err != nil {
		message := err.Error()
		code := "invalid_configuration"
		if regexp.MustCompile(`(?i)(mapping key .* already defined|duplicate key)`).MatchString(message) {
			code = "duplicate_key"
		}
		if regexp.MustCompile(`(?i)field .* not found`).MatchString(message) {
			code = "unknown_field"
		}
		if regexp.MustCompile(`(?i)field password not found`).MatchString(message) {
			code = "secret_not_allowed"
		}
		return model.Configuration{}, &loadError{code, "invalid configuration"}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return model.Configuration{}, &loadError{"invalid_configuration", "configuration must contain one document"}
	}
	if configuration.Administrator.Username == "" || !validPHC(configuration.Administrator.PasswordHash) {
		return model.Configuration{}, &loadError{"invalid_password_hash", "administrator password hash is invalid"}
	}
	if configuration.Database.PasswordEnv == "" {
		return model.Configuration{}, &loadError{"secret_not_allowed", "database password must be an environment reference"}
	}
	if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(configuration.Database.PasswordEnv) {
		return model.Configuration{}, &loadError{"invalid_environment_reference", "database password environment reference is invalid"}
	}
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
	if _, err := model.ValidateEditableConfiguration(configuration.Redacted()); err != nil {
		return model.Configuration{}, err
	}
	return configuration, nil
}
func validPHC(value string) bool {
	_, err := usecase.VerifyPassword(nil, value, "probe")
	return err == nil
}
