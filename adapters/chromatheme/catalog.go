// Package chromatheme adapts the embedded Chroma catalog without overrides.
package chromatheme

import (
	"log/slog"
	"net/http"

	chromaui "github.com/githubixx/chroma-ui"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

type Catalog struct{ bundled *chromaui.Catalog }

func NewCatalog() (*Catalog, error) {
	bundled, err := chromaui.NewCatalog(nil)
	if err != nil {
		return nil, err
	}
	return &Catalog{bundled: bundled}, nil
}

func (catalog *Catalog) Themes() []ports.ThemeOption {
	themes := catalog.bundled.Themes()
	options := make([]ports.ThemeOption, 0, len(themes))
	for _, theme := range themes {
		options = append(options, ports.ThemeOption{ID: theme.ID, Name: theme.Name, Bundled: theme.Bundled})
	}
	return options
}

func (catalog *Catalog) Contains(id string) bool {
	_, ok := catalog.bundled.Theme(id)
	return ok
}

func (catalog *Catalog) Handler(logger *slog.Logger) http.Handler {
	return catalog.bundled.Handler(logger)
}
