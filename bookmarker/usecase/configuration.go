package usecase

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

type ConfigurationDependencies struct {
	Store     ports.ConfigurationStore
	Validator ports.ConfigurationValidator
	Activator ports.ConfigurationActivator
	Catalog   ports.ThemeCatalog
}

type ConfigurationChange struct {
	Configuration   model.EditableConfiguration
	RestartRequired bool
}

type ConfigurationService struct{ dependencies ConfigurationDependencies }

type ConfigurationValidators []ports.ConfigurationValidator

func (validators ConfigurationValidators) Validate(ctx context.Context, configuration model.Configuration) error {
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		if err := validator.Validate(ctx, configuration); err != nil {
			return err
		}
	}
	return nil
}

func NewConfigurationService(dependencies ConfigurationDependencies) *ConfigurationService {
	return &ConfigurationService{dependencies: dependencies}
}

func (service *ConfigurationService) Get(ctx context.Context) (model.EditableConfiguration, error) {
	configuration, err := service.dependencies.Store.Load(ctx)
	if err != nil {
		return model.EditableConfiguration{}, err
	}
	return configuration.Redacted(), nil
}

func (service *ConfigurationService) Validate(ctx context.Context, candidate model.EditableConfiguration) (ConfigurationChange, error) {
	current, err := service.dependencies.Store.Load(ctx)
	if err != nil {
		return ConfigurationChange{}, err
	}
	change, _, err := service.validateCandidate(ctx, current, candidate)
	return change, err
}

func (service *ConfigurationService) validateCandidate(ctx context.Context, current model.Configuration, candidate model.EditableConfiguration) (ConfigurationChange, model.Configuration, error) {
	if !candidate.ThemePresent {
		candidate.Theme = current.Theme
		if candidate.Theme == "" {
			candidate.Theme = "system"
		}
	}
	if candidate.Theme != "system" && (service.dependencies.Catalog == nil || !service.dependencies.Catalog.Contains(candidate.Theme)) {
		return ConfigurationChange{}, model.Configuration{}, &model.ValidationError{Code: "validation_failed", Field: "theme", Message: "select an available theme"}
	}
	updated, err := model.ApplyEditableConfiguration(current, candidate)
	if err != nil {
		return ConfigurationChange{}, model.Configuration{}, err
	}
	if service.dependencies.Validator != nil {
		if err := service.dependencies.Validator.Validate(ctx, updated); err != nil {
			return ConfigurationChange{}, model.Configuration{}, err
		}
	}
	return ConfigurationChange{Configuration: updated.Redacted(), RestartRequired: databaseChanged(current.Database, updated.Database)}, updated, nil
}

func (service *ConfigurationService) Update(ctx context.Context, candidate model.EditableConfiguration) (ConfigurationChange, error) {
	var change ConfigurationChange
	transaction := func() error {
		current, err := service.dependencies.Store.Load(ctx)
		if err != nil {
			return err
		}
		var updated model.Configuration
		change, updated, err = service.validateCandidate(ctx, current, candidate)
		if err != nil {
			return err
		}
		if service.dependencies.Activator == nil {
			return &model.ApplicationError{Code: "dependency_unavailable", Message: "configuration activation is unavailable"}
		}
		return service.dependencies.Activator.Activate(ctx, updated)
	}
	var err error
	if locker, ok := service.dependencies.Store.(ports.ConfigurationTransaction); ok {
		err = locker.WithLock(ctx, transaction)
	} else {
		err = transaction()
	}
	if err != nil {
		return ConfigurationChange{}, err
	}
	return change, nil
}

func databaseChanged(before, after model.DatabaseConfig) bool {
	return before.Host != after.Host || before.Port != after.Port || before.Name != after.Name || before.User != after.User || before.PasswordEnv != after.PasswordEnv || before.TLSMode != after.TLSMode
}
