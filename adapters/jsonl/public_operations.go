package jsonl

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

type bookmarkListPayload struct {
	Page int    `json:"page"`
	Tag  string `json:"tag"`
}

type bookmarkSearchPayload struct {
	Tag   string   `json:"tag"`
	Words []string `json:"words"`
	Page  int      `json:"page"`
}

type tagSummary struct {
	Name string `json:"name"`
}

type screenshotSummary struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
}

type bookmarkSummary struct {
	ID          string             `json:"id"`
	URL         string             `json:"url"`
	Description string             `json:"description"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
	Tags        []tagSummary       `json:"tags"`
	Screenshot  *screenshotSummary `json:"screenshot,omitempty"`
}

type paginatedResult struct {
	Page           int               `json:"page"`
	PageSize       int               `json:"page_size"`
	LastPage       int               `json:"last_page"`
	TotalWithinCap int               `json:"total_within_cap"`
	IsCapped       bool              `json:"is_capped"`
	Items          []bookmarkSummary `json:"items"`
}

func toBookmarkSummary(bookmark model.Bookmark) bookmarkSummary {
	tags := make([]tagSummary, 0, len(bookmark.Tags))
	for _, tag := range bookmark.Tags {
		tags = append(tags, tagSummary{Name: tag.DisplayName})
	}
	summary := bookmarkSummary{
		ID:          bookmark.ID.String(),
		URL:         bookmark.URL,
		Description: bookmark.Description,
		CreatedAt:   bookmark.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   bookmark.UpdatedAt.Format(time.RFC3339),
		Tags:        tags,
	}
	if bookmark.Screenshot != nil {
		summary.Screenshot = &screenshotSummary{ID: bookmark.Screenshot.ID.String(), Available: bookmark.Screenshot.Available}
	}
	return summary
}

func toBookmarkSummaries(bookmarks []model.Bookmark) []bookmarkSummary {
	summaries := make([]bookmarkSummary, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		summaries = append(summaries, toBookmarkSummary(bookmark))
	}
	return summaries
}

func (server *Server) handleBookmarkList(ctx context.Context, browser *usecase.Browser, value request, output io.Writer) {
	if browser == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload bookmarkListPayload
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	page, err := browser.Browse(ctx, model.BrowseQuery{Tag: payload.Tag, Page: payload.Page})
	if err != nil {
		write(output, responseError{value.ID, "internal_error"})
		return
	}
	write(output, responseOK{ID: value.ID, Result: paginatedResult{
		Page:           page.Pagination.Page,
		PageSize:       page.Pagination.PageSize,
		LastPage:       page.Pagination.LastPage,
		TotalWithinCap: page.Total,
		IsCapped:       false,
		Items:          toBookmarkSummaries(page.Items),
	}})
}

func (server *Server) handleBookmarkSearch(ctx context.Context, searcher *usecase.Searcher, value request, output io.Writer) {
	if searcher == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload bookmarkSearchPayload
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	result, err := searcher.Search(ctx, model.SearchQuery{Tag: payload.Tag, Words: payload.Words, Page: payload.Page})
	if err != nil {
		write(output, responseError{value.ID, model.ErrorCode(err)})
		return
	}
	write(output, responseOK{ID: value.ID, Result: paginatedResult{
		Page:           result.Pagination.Page,
		PageSize:       result.Pagination.PageSize,
		LastPage:       result.Pagination.LastPage,
		TotalWithinCap: result.TotalWithinCap,
		IsCapped:       result.IsCapped,
		Items:          toBookmarkSummaries(result.Items),
	}})
}
