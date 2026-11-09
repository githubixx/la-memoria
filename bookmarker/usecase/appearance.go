package usecase

import (
	"context"
	"log/slog"

	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// Appearance contains only the effective, catalog-approved shell selection.
type Appearance struct {
	Theme         string
	StylesheetURL string
}

type AppearanceService struct {
	store   ports.ConfigurationStore
	catalog ports.ThemeCatalog
	logger  *slog.Logger
}

func NewAppearanceService(store ports.ConfigurationStore, catalog ports.ThemeCatalog, logger *slog.Logger) *AppearanceService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AppearanceService{store: store, catalog: catalog, logger: logger}
}

func (service *AppearanceService) Get(ctx context.Context) Appearance {
	fallback := Appearance{Theme: "system"}
	if service == nil || service.store == nil {
		return fallback
	}
	configuration, err := service.store.Load(ctx)
	if err != nil {
		service.logger.Warn("configuration appearance unavailable; using system")
		return fallback
	}
	if configuration.Theme == "" || configuration.Theme == "system" || service.catalog == nil || !service.catalog.Contains(configuration.Theme) {
		return fallback
	}
	return Appearance{Theme: configuration.Theme, StylesheetURL: "/ui/themes/" + configuration.Theme + "/theme.css"}
}
