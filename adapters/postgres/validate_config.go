package postgres

import (
	"context"
	"fmt"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

type ConfigurationValidator struct{}

func NewConfigurationValidator() ConfigurationValidator { return ConfigurationValidator{} }

func (ConfigurationValidator) Validate(ctx context.Context, configuration model.Configuration) error {
	pool, err := NewPool(ctx, DSN(configuration))
	if err != nil {
		return &model.ApplicationError{Code: "dependency_unavailable", Message: "database settings could not be validated", Retryable: true}
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("validate database connection: %w", err)
	}
	return nil
}
