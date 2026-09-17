package usecase

import (
	"context"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

var ErrSearchCriteriaRequired = &model.ValidationError{Code: "search_criteria_required", Field: "q", Message: "enter a tag or description words to search"}

// SearchDependencies configures the public search use case.
type SearchDependencies struct {
	Bookmarks      ports.BookmarkStore
	PageSize       int
	MaximumResults int
}

// Searcher implements exact-tag and/or all-word description search, capped
// and paginated according to configured limits, available without
// authentication.
type Searcher struct{ dependencies SearchDependencies }

func NewSearcher(dependencies SearchDependencies) *Searcher {
	if dependencies.PageSize == 0 {
		dependencies.PageSize = 10
	}
	if dependencies.MaximumResults == 0 {
		dependencies.MaximumResults = 10_000
	}
	return &Searcher{dependencies: dependencies}
}

// Search requires at least one non-empty tag or description-word criterion;
// when both are supplied they combine with AND.
func (searcher *Searcher) Search(ctx context.Context, query model.SearchQuery) (model.SearchResult, error) {
	normalizedTag := model.NormalizeTagName(query.Tag)
	if normalizedTag == "" && len(query.Words) == 0 {
		return model.SearchResult{}, ErrSearchCriteriaRequired
	}
	pageSize := searcher.dependencies.PageSize
	maximumResults := searcher.dependencies.MaximumResults
	items, totalWithinCap, isCapped, err := searcher.dependencies.Bookmarks.SearchPage(ctx, normalizedTag, query.Words, query.Page, pageSize, maximumResults)
	if err != nil {
		return model.SearchResult{}, err
	}
	return model.SearchResult{
		Items:          items,
		Pagination:     model.NewPagination(query.Page, totalWithinCap, pageSize),
		TotalWithinCap: totalWithinCap,
		IsCapped:       isCapped,
	}, nil
}
