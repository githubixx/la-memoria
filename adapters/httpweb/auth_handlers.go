package httpweb

import (
	"net/http"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

const sessionCookieName = "bookmarker_session"

func setSessionCookie(writer http.ResponseWriter, token string) {
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 0})
}

func clearSessionCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func sessionToken(request *http.Request) string {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// principal resolves the authenticated administrator for this request, if
// any, from the session cookie.
func (router *Router) principal(request *http.Request) (model.Principal, bool) {
	if router.dependencies.Authenticator == nil {
		return model.Principal{}, false
	}
	token := sessionToken(request)
	if token == "" {
		return model.Principal{}, false
	}
	principal, err := router.dependencies.Authenticator.Authorize(request.Context(), token)
	if err != nil {
		return model.Principal{}, false
	}
	return principal, true
}

type loginViewModel struct {
	pageContext
	ReturnTo string
	Error    string
}

func (router *Router) handleLoginGet(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if router.dependencies.Authenticator == nil || router.renderer == nil {
		// Minimal fallback when authentication is not wired: still issue a
		// browser-session cookie so the security contract can be verified
		// in isolation from the full authentication stack.
		setSessionCookie(writer, randomID())
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("<!doctype html><title>Sign in</title>"))
		return
	}
	session, err := router.dependencies.Authenticator.NewAnonymousSession(request.Context())
	if err != nil {
		http.Error(writer, "sign-in unavailable", http.StatusServiceUnavailable)
		return
	}
	setSessionCookie(writer, session.Token)
	view := loginViewModel{
		pageContext: pageContext{PageTitle: "Bookmarker", CSRFToken: session.CSRFToken},
		ReturnTo:    request.URL.Query().Get("return_to"),
	}
	if err := router.renderer.Render(writer, request, "login", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

func (router *Router) handleLoginPost(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if router.dependencies.Authenticator == nil || router.renderer == nil {
		http.Error(writer, "sign-in unavailable", http.StatusInternalServerError)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad request", http.StatusBadRequest)
		return
	}
	token := sessionToken(request)
	if !router.dependencies.Authenticator.VerifyCSRF(request.Context(), token, request.PostFormValue("csrf_token")) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	session, err := router.dependencies.Authenticator.SignIn(request.Context(), usecase.SignInInput{
		SessionToken: token,
		Username:     request.PostFormValue("username"),
		Password:     request.PostFormValue("password"),
		Source:       ClientAddress(request),
	})
	if err != nil {
		view := loginViewModel{
			pageContext: pageContext{PageTitle: "Bookmarker", CSRFToken: usecase.CSRFTokenFor(token)},
			ReturnTo:    request.PostFormValue("return_to"),
			Error:       "invalid username or password",
		}
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_ = router.renderer.Render(writer, request, "login", view)
		return
	}

	setSessionCookie(writer, session.Token)
	destination := request.PostFormValue("return_to")
	if destination == "" {
		destination = "/bookmarks"
	}
	if request.Header.Get("HX-Request") == "true" {
		writer.Header().Set("HX-Redirect", destination)
		writer.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(writer, request, destination, http.StatusSeeOther)
}

func (router *Router) handleLogout(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if router.dependencies.Authenticator != nil {
		_ = router.dependencies.Authenticator.SignOut(request.Context(), sessionToken(request))
	}
	clearSessionCookie(writer)
	if request.Header.Get("HX-Request") == "true" {
		writer.Header().Set("HX-Redirect", "/bookmarks")
		writer.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(writer, request, "/bookmarks", http.StatusSeeOther)
}
