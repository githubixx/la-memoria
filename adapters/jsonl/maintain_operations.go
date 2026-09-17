package jsonl

import (
	"context"
	"encoding/json"
	"io"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

func (server *Server) handleBookmarkUpdate(ctx context.Context, value request, output io.Writer, sessionToken string) {
	principal, ok := server.requirePrincipal(ctx, sessionToken)
	if !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.Maintainer == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload struct {
		BookmarkID            string   `json:"bookmark_id"`
		URL                   string   `json:"url"`
		Description           string   `json:"description"`
		Tags                  []string `json:"tags"`
		CaptureID             string   `json:"capture_id"`
		SaveWithoutScreenshot bool     `json:"save_without_screenshot"`
	}
	if err := json.Unmarshal(value.Payload, &payload); err != nil || payload.BookmarkID == "" {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	var captureID *model.ID
	if payload.CaptureID != "" {
		id := model.ID(payload.CaptureID)
		captureID = &id
	}
	bookmark, err := server.dependencies.Maintainer.Update(ctx, usecase.UpdateBookmarkInput{SessionID: principal.SessionID, BookmarkID: model.ID(payload.BookmarkID), URL: payload.URL, Description: payload.Description, Tags: payload.Tags, CaptureID: captureID, SaveWithoutScreenshot: payload.SaveWithoutScreenshot})
	if err != nil {
		write(output, responseError{value.ID, model.ErrorCode(err)})
		return
	}
	write(output, responseOK{ID: value.ID, Result: map[string]any{"id": bookmark.ID.String(), "created_at": bookmark.CreatedAt}})
}

func (server *Server) handleBookmarkDelete(ctx context.Context, value request, output io.Writer, sessionToken string) {
	if _, ok := server.requirePrincipal(ctx, sessionToken); !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.Maintainer == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload struct {
		BookmarkID string `json:"bookmark_id"`
		Confirm    bool   `json:"confirm"`
	}
	if err := json.Unmarshal(value.Payload, &payload); err != nil || payload.BookmarkID == "" {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	result, err := server.dependencies.Maintainer.Delete(ctx, usecase.DeleteBookmarkInput{BookmarkID: model.ID(payload.BookmarkID), Confirm: payload.Confirm})
	if err != nil {
		write(output, responseError{value.ID, model.ErrorCode(err)})
		return
	}
	write(output, responseOK{ID: value.ID, Result: map[string]any{"id": result.ID.String(), "cleanup_pending": result.CleanupPending}})
}
