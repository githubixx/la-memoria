package httpweb

import (
	"net/http"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

type maintainViewModel struct {
	pageContext
	Bookmark        ports.StoredBookmark
	ValidationError string
}

func (router *Router) handleBookmarkEdit(writer http.ResponseWriter, request *http.Request, id string) {
	if _, ok := router.principal(request); !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if router.dependencies.Maintainer == nil {
		http.Error(writer, "bookmarks unavailable", http.StatusServiceUnavailable)
		return
	}
	bookmark, err := router.dependencies.Maintainer.Find(request.Context(), model.ID(id))
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	view := maintainViewModel{pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}, Bookmark: bookmark}
	if err := router.renderer.Render(writer, request, "edit", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

func (router *Router) handleBookmarkUpdate(writer http.ResponseWriter, request *http.Request, id string) {
	principal, ok := router.principal(request)
	if !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if router.dependencies.Maintainer == nil {
		http.Error(writer, "bookmarks unavailable", http.StatusServiceUnavailable)
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
	_, err := router.dependencies.Maintainer.Update(request.Context(), usecase.UpdateBookmarkInput{SessionID: principal.SessionID, BookmarkID: model.ID(id), URL: request.PostFormValue("url"), Description: request.PostFormValue("description"), Tags: request.PostForm["tags"], CaptureID: captureID, SaveWithoutScreenshot: request.PostFormValue("save_without_screenshot") == "true"})
	if err != nil {
		bookmark, findErr := router.dependencies.Maintainer.Find(request.Context(), model.ID(id))
		if findErr != nil {
			http.Error(writer, "bookmark update failed", http.StatusUnprocessableEntity)
			return
		}
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_ = router.renderer.Render(writer, request, "edit", maintainViewModel{pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}, Bookmark: bookmark, ValidationError: err.Error()})
		return
	}
	if request.Header.Get("HX-Request") == "true" {
		writer.Header().Set("HX-Redirect", "/bookmarks")
		writer.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(writer, request, "/bookmarks", http.StatusSeeOther)
}

func (router *Router) handleBookmarkDeleteForm(writer http.ResponseWriter, request *http.Request, id string) {
	if _, ok := router.principal(request); !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if router.dependencies.Maintainer == nil {
		http.Error(writer, "bookmarks unavailable", http.StatusServiceUnavailable)
		return
	}
	bookmark, err := router.dependencies.Maintainer.Find(request.Context(), model.ID(id))
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	if err := router.renderer.Render(writer, request, "delete", maintainViewModel{pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}, Bookmark: bookmark}); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

func (router *Router) handleBookmarkDelete(writer http.ResponseWriter, request *http.Request, id string) {
	if _, ok := router.principal(request); !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if router.dependencies.Maintainer == nil {
		http.Error(writer, "bookmarks unavailable", http.StatusServiceUnavailable)
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
	_, err := router.dependencies.Maintainer.Delete(request.Context(), usecase.DeleteBookmarkInput{BookmarkID: model.ID(id), Confirm: request.PostFormValue("confirm") == "true"})
	if err != nil {
		http.Error(writer, "bookmark deletion failed", http.StatusUnprocessableEntity)
		return
	}
	if request.Header.Get("HX-Request") == "true" {
		writer.Header().Set("HX-Redirect", "/bookmarks")
		writer.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(writer, request, "/bookmarks", http.StatusSeeOther)
}
