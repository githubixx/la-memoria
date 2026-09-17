package ports

import (
	"context"
	"github.com/githubixx/la-memoria/bookmarker/model"
)

type ConfigurationStore interface {
	Load(context.Context) (model.Configuration, error)
}
type ConfigurationValidator interface {
	Validate(context.Context, model.Configuration) error
}
type ConfigurationActivator interface {
	Activate(context.Context, model.Configuration) error
}
