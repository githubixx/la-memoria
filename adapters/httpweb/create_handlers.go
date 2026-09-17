package httpweb

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

type addViewModel struct {
	pageContext
	ValidationError string
}

type captureViewModel struct {
	CaptureID          string
	State              string
	PreviewURL         string
	Message            string
	CanRetry           bool
	CanContinueWithout bool
}

func (router *Router) respondUnauthorized(writer http.ResponseWriter, request *http.Request) {
	destination := "/login?return_to=" + url.QueryEscape(request.URL.Path)
	if request.Header.Get("HX-Request") == "true" {
		writer.Header().Set("HX-Redirect", destination)
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(writer, request, destination, http.StatusSeeOther)
}

func (router *Router) handleAddForm(writer http.ResponseWriter, request *http.Request) {
	if _, ok := router.principal(request); !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	view := addViewModel{pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}}
	if err := router.renderer.Render(writer, request, "add", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

func (router *Router) handleBookmarkCreate(writer http.ResponseWriter, request *http.Request) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad request", http.StatusBadRequest)
		return
	}
	if !router.dependencies.Authenticator.VerifyCSRF(request.Context(), sessionToken(request), request.PostFormValue("csrf_token")) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	var captureID *model.ID
	if raw := request.PostFormValue("capture_id"); raw != "" {
		value := model.ID(raw)
		captureID = &value
	}

	_, err := router.dependencies.Creator.Create(request.Context(), usecase.CreateBookmarkInput{
		SessionID:             principal.SessionID,
		URL:                   request.PostFormValue("url"),
		Description:           request.PostFormValue("description"),
		Tags:                  request.PostForm["tags"],
		CaptureID:             captureID,
		SaveWithoutScreenshot: request.PostFormValue("save_without_screenshot") == "true",
	})
	if err != nil {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		view := addViewModel{pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}, ValidationError: err.Error()}
		_ = router.renderer.Render(writer, request, "add", view)
		return
	}

	if request.Header.Get("HX-Request") == "true" {
		writer.Header().Set("HX-Redirect", "/bookmarks")
		writer.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(writer, request, "/bookmarks", http.StatusSeeOther)
}

func (router *Router) handleCaptureStart(writer http.ResponseWriter, request *http.Request) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad request", http.StatusBadRequest)
		return
	}
	if !router.dependencies.Authenticator.VerifyCSRF(request.Context(), sessionToken(request), request.PostFormValue("csrf_token")) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	var bookmarkID *model.ID
	if raw := request.PostFormValue("bookmark_id"); raw != "" {
		value := model.ID(raw)
		bookmarkID = &value
	}
	draft, err := router.dependencies.CaptureService.Start(request.Context(), usecase.StartCaptureInput{
		SessionID:  principal.SessionID,
		TargetURL:  request.PostFormValue("url"),
		BookmarkID: bookmarkID,
	})
	if err != nil {
		http.Error(writer, "invalid capture request", http.StatusUnprocessableEntity)
		return
	}
	writer.WriteHeader(http.StatusCreated)
	router.renderCaptureFragment(writer, draft)
}

func (router *Router) handleCaptureStatus(writer http.ResponseWriter, request *http.Request, id string) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	draft, err := router.dependencies.CaptureService.Status(request.Context(), principal.SessionID, model.ID(id))
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	router.renderCaptureFragment(writer, draft)
}

func (router *Router) handleCapturePreview(writer http.ResponseWriter, request *http.Request, id string) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	draft, err := router.dependencies.CaptureService.Status(request.Context(), principal.SessionID, model.ID(id))
	if err != nil || draft.State != model.CaptureStateReady {
		http.NotFound(writer, request)
		return
	}
	image, err := router.dependencies.CaptureService.Preview(request.Context(), draft)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "image/png")
	_, _ = writer.Write(image)
}

func (router *Router) handleCaptureRetry(writer http.ResponseWriter, request *http.Request, id string) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if !router.dependencies.Authenticator.VerifyCSRF(request.Context(), sessionToken(request), request.PostFormValue("csrf_token")) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	draft, err := router.dependencies.CaptureService.Retry(request.Context(), principal.SessionID, model.ID(id))
	if err != nil {
		http.Error(writer, "capture cannot be retried", http.StatusConflict)
		return
	}
	router.renderCaptureFragment(writer, draft)
}

func (router *Router) handleCaptureDiscard(writer http.ResponseWriter, request *http.Request, id string) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if !router.dependencies.Authenticator.VerifyCSRF(request.Context(), sessionToken(request), request.PostFormValue("csrf_token")) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	draft, err := router.dependencies.CaptureService.Discard(request.Context(), principal.SessionID, model.ID(id))
	if err != nil {
		http.Error(writer, "capture cannot be discarded", http.StatusConflict)
		return
	}
	router.renderCaptureFragment(writer, draft)
}

func (router *Router) renderCaptureFragment(writer http.ResponseWriter, draft model.CaptureDraft) {
	view := captureViewModel{
		CaptureID:          draft.ID.String(),
		State:              string(draft.State),
		CanRetry:           draft.CanRetry(),
		CanContinueWithout: draft.CanDiscard(),
	}
	if draft.State == model.CaptureStateReady {
		view.PreviewURL = "/captures/" + draft.ID.String() + "/preview"
	}
	if draft.State == model.CaptureStateFailed {
		view.Message = "Screenshot capture failed. You can retry or continue without a screenshot."
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := router.renderer.RenderFragment(writer, "capture", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

// captureIDFromPath extracts the opaque capture id from a "/captures/{id}"
// or "/captures/{id}/{action}" path.
func captureIDFromPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/captures/")
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		return trimmed[:slash]
	}
	return trimmed
}
