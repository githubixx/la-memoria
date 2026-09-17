package usecase

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

// BrowseDependencies configures the public browse use case.
type BrowseDependencies struct {
	Bookmarks ports.BookmarkStore
	PageSize  int
}

// Browser implements the fixed-page-size public browse use case, available
// without authentication.
type Browser struct{ dependencies BrowseDependencies }

func NewBrowser(dependencies BrowseDependencies) *Browser {
	if dependencies.PageSize == 0 {
		dependencies.PageSize = 10
	}
	return &Browser{dependencies: dependencies}
}

// Browse returns one newest-first page, optionally filtered to an exact
// normalized tag.
func (browser *Browser) Browse(ctx context.Context, query model.BrowseQuery) (model.BookmarkPage, error) {
	normalizedTag := model.NormalizeTagName(query.Tag)
	pageSize := browser.dependencies.PageSize
	items, total, err := browser.dependencies.Bookmarks.BrowsePage(ctx, normalizedTag, query.Page, pageSize)
	if err != nil {
		return model.BookmarkPage{}, err
	}
	return model.BookmarkPage{Items: items, Pagination: model.NewPagination(query.Page, total, pageSize), Total: total}, nil
}
