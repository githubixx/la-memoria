package jsonl

import (
	"context"
	"encoding/json"
	"io"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

func (server *Server) requirePrincipal(ctx context.Context, sessionToken string) (model.Principal, bool) {
	if server.dependencies.Authenticator == nil || sessionToken == "" {
		return model.Principal{}, false
	}
	principal, err := server.dependencies.Authenticator.Authorize(ctx, sessionToken)
	if err != nil {
		return model.Principal{}, false
	}
	return principal, true
}

func (server *Server) handleSignIn(ctx context.Context, value request, output io.Writer, sessionToken *string) {
	if server.dependencies.Authenticator == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	session, err := server.dependencies.Authenticator.SignIn(ctx, usecase.SignInInput{
		SessionToken: *sessionToken,
		Username:     payload.Username,
		Password:     payload.Password,
		Source:       "cli-local",
	})
	if err != nil {
		write(output, responseError{value.ID, "authentication_failed"})
		return
	}
	*sessionToken = session.Token
	write(output, responseOK{ID: value.ID, Result: map[string]any{"authenticated": true}})
}

func (server *Server) handleSignOut(ctx context.Context, value request, output io.Writer, sessionToken *string) {
	if server.dependencies.Authenticator != nil && *sessionToken != "" {
		_ = server.dependencies.Authenticator.SignOut(ctx, *sessionToken)
	}
	*sessionToken = ""
	write(output, responseOK{ID: value.ID, Result: map[string]any{"authenticated": false}})
}

func (server *Server) handleCaptureStart(ctx context.Context, value request, output io.Writer, sessionToken string) {
	principal, ok := server.requirePrincipal(ctx, sessionToken)
	if !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.CaptureService == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload struct {
		URL        string `json:"url"`
		BookmarkID string `json:"bookmark_id"`
	}
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	var bookmarkID *model.ID
	if payload.BookmarkID != "" {
		value := model.ID(payload.BookmarkID)
		bookmarkID = &value
	}
	draft, err := server.dependencies.CaptureService.Start(ctx, usecase.StartCaptureInput{SessionID: principal.SessionID, TargetURL: payload.URL, BookmarkID: bookmarkID})
	if err != nil {
		write(output, responseError{value.ID, "invalid_url"})
		return
	}
	write(output, responseOK{ID: value.ID, Result: captureStatusResult(draft)})
}

func (server *Server) handleCaptureStatus(ctx context.Context, value request, output io.Writer, sessionToken string) {
	server.withCaptureID(ctx, value, output, sessionToken, server.dependencies.CaptureService.Status)
}

func (server *Server) handleCaptureRetry(ctx context.Context, value request, output io.Writer, sessionToken string) {
	server.withCaptureID(ctx, value, output, sessionToken, server.dependencies.CaptureService.Retry)
}

func (server *Server) handleCaptureDiscard(ctx context.Context, value request, output io.Writer, sessionToken string) {
	server.withCaptureID(ctx, value, output, sessionToken, server.dependencies.CaptureService.Discard)
}

func (server *Server) withCaptureID(ctx context.Context, value request, output io.Writer, sessionToken string, action func(context.Context, model.ID, model.ID) (model.CaptureDraft, error)) {
	principal, ok := server.requirePrincipal(ctx, sessionToken)
	if !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.CaptureService == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload struct {
		CaptureID string `json:"capture_id"`
	}
	if err := json.Unmarshal(value.Payload, &payload); err != nil || payload.CaptureID == "" {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	draft, err := action(ctx, principal.SessionID, model.ID(payload.CaptureID))
	if err != nil {
		write(output, responseError{value.ID, model.ErrorCode(err)})
		return
	}
	write(output, responseOK{ID: value.ID, Result: captureStatusResult(draft)})
}

func (server *Server) handleBookmarkCreate(ctx context.Context, value request, output io.Writer, sessionToken string) {
	principal, ok := server.requirePrincipal(ctx, sessionToken)
	if !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.Creator == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	var payload struct {
		URL                   string   `json:"url"`
		Description           string   `json:"description"`
		Tags                  []string `json:"tags"`
		CaptureID             string   `json:"capture_id"`
		SaveWithoutScreenshot bool     `json:"save_without_screenshot"`
	}
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	var captureID *model.ID
	if payload.CaptureID != "" {
		id := model.ID(payload.CaptureID)
		captureID = &id
	}
	id, err := server.dependencies.Creator.Create(ctx, usecase.CreateBookmarkInput{
		SessionID:             principal.SessionID,
		URL:                   payload.URL,
		Description:           payload.Description,
		Tags:                  payload.Tags,
		CaptureID:             captureID,
		SaveWithoutScreenshot: payload.SaveWithoutScreenshot,
	})
	if err != nil {
		write(output, responseError{value.ID, model.ErrorCode(err)})
		return
	}
	write(output, responseOK{ID: value.ID, Result: map[string]any{"id": id.String()}})
}

func captureStatusResult(draft model.CaptureDraft) map[string]any {
	result := map[string]any{
		"capture_id":           draft.ID.String(),
		"state":                string(draft.State),
		"can_retry":            draft.CanRetry(),
		"can_continue_without": draft.CanDiscard(),
		"failure_code":         nil,
	}
	if draft.FailureCode != "" {
		result["failure_code"] = draft.FailureCode
	}
	return result
}
