package httpweb

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

type configurationViewModel struct {
	pageContext
	Configuration   model.EditableConfiguration
	ValidationError string
	RestartRequired bool
}

func (router *Router) handleConfigurationGet(writer http.ResponseWriter, request *http.Request) {
	if _, ok := router.principal(request); !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if router.dependencies.Configuration == nil || router.renderer == nil {
		http.Error(writer, "configuration unavailable", http.StatusServiceUnavailable)
		return
	}
	configuration, err := router.dependencies.Configuration.Get(request.Context())
	if err != nil {
		http.Error(writer, "configuration unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	view := configurationViewModel{pageContext: pageContext{PageTitle: configuration.Branding.PageTitle, Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}, Configuration: configuration, RestartRequired: request.URL.Query().Get("restart_required") == "true"}
	if err := router.renderer.Render(writer, request, "configuration", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

func (router *Router) handleFavicon(writer http.ResponseWriter, request *http.Request) {
	const contentType = "image/png"
	var configuration model.EditableConfiguration
	var err error
	if router.dependencies.Configuration != nil {
		configuration, err = router.dependencies.Configuration.Get(request.Context())
	}
	if err == nil && configuration.Branding.FaviconPath != "" {
		path := filepath.Join("web/static", configuration.Branding.FaviconPath)
		if contents, readErr := os.ReadFile(path); readErr == nil && len(contents) >= 8 {
			writer.Header().Set("Content-Type", contentType)
			_, _ = writer.Write(contents)
			return
		}
	}
	writer.Header().Set("Content-Type", contentType)
	_, _ = writer.Write(defaultFaviconPNG)
}

var defaultFaviconPNG = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89, 0, 0, 0, 0x0d, 'I', 'D', 'A', 'T', 8, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0xf0, 0x1f, 0, 5, 0x83, 2, 0x7f, 0x96, 0x8b, 0x0b, 0x8a, 0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}

func (router *Router) handleConfigurationPost(writer http.ResponseWriter, request *http.Request) {
	if _, ok := router.principal(request); !ok {
		router.respondUnauthorized(writer, request)
		return
	}
	if router.dependencies.Configuration == nil || router.renderer == nil {
		http.Error(writer, "configuration unavailable", http.StatusServiceUnavailable)
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
	candidate := editableConfigurationFromForm(request)
	change, err := router.dependencies.Configuration.Update(request.Context(), candidate)
	if err != nil {
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_ = router.renderer.Render(writer, request, "configuration", configurationViewModel{pageContext: pageContext{PageTitle: candidate.Branding.PageTitle, Authenticated: true, CSRFToken: usecase.CSRFTokenFor(sessionToken(request))}, Configuration: candidate, ValidationError: configurationValidationMessage(err)})
		return
	}
	if request.Header.Get("HX-Request") == "true" {
		if change.RestartRequired {
			writer.Header().Set("HX-Redirect", "/configuration?restart_required=true")
		} else {
			writer.Header().Set("HX-Redirect", "/configuration")
		}
		writer.WriteHeader(http.StatusOK)
		return
	}
	target := "/configuration"
	if change.RestartRequired {
		target += "?restart_required=true"
	}
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func configurationValidationMessage(err error) string {
	var validation *model.ValidationError
	if errors.As(err, &validation) {
		return validation.Field + ": " + validation.Message
	}
	return "configuration could not be applied"
}

func editableConfigurationFromForm(request *http.Request) model.EditableConfiguration {
	port, _ := strconv.Atoi(request.PostFormValue("database_port"))
	pageSize, _ := strconv.Atoi(request.PostFormValue("search_page_size"))
	maximumResults, _ := strconv.Atoi(request.PostFormValue("maximum_search_results"))
	return model.EditableConfiguration{
		Branding:    model.BrandingConfig{PageTitle: request.PostFormValue("page_title"), FaviconPath: request.PostFormValue("favicon_path")},
		DefaultView: request.PostFormValue("default_view"),
		Database:    model.DatabaseConfig{Host: request.PostFormValue("database_host"), Port: port, Name: request.PostFormValue("database_name"), User: request.PostFormValue("database_user"), PasswordEnv: request.PostFormValue("database_password_env"), TLSMode: request.PostFormValue("database_tls_mode")},
		Screenshots: model.ScreenshotConfig{Root: request.PostFormValue("screenshot_root")},
		Search:      model.SearchConfig{PageSize: pageSize, MaximumResults: maximumResults},
	}
}
