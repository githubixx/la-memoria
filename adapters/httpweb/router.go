package httpweb

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

// Dependencies configures the HTTP delivery adapter's composition.
type Dependencies struct {
	TrustedProxyCIDRs []string
	TemplatesDir      string
	Browser           *usecase.Browser
	Searcher          *usecase.Searcher
	DefaultView       string
	Authenticator     *usecase.Authenticator
	CaptureService    *usecase.CaptureService
	Creator           *usecase.Creator
	Maintainer        *usecase.Maintainer
	Configuration     *usecase.ConfigurationService
}

type Router struct {
	dependencies Dependencies
	trusted      []*net.IPNet
	renderer     *Renderer
	renderErr    error
}

func NewRouter(dependencies Dependencies) *Router {
	router := &Router{dependencies: dependencies}
	for _, value := range dependencies.TrustedProxyCIDRs {
		if _, network, err := net.ParseCIDR(value); err == nil {
			router.trusted = append(router.trusted, network)
		}
	}
	if dependencies.TemplatesDir != "" {
		router.renderer, router.renderErr = NewRenderer(dependencies.TemplatesDir)
	}
	return router
}

func (router *Router) Handler() http.Handler {
	return router.middleware(http.HandlerFunc(router.route))
}

func (router *Router) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = randomID()
		}
		writer.Header().Set("X-Request-ID", requestID)
		if request.Method == http.MethodPost && !sameOrigin(request) {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (router *Router) route(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	switch {
	case path == "/":
		router.handleRoot(writer, request)
	case path == "/bookmarks" && request.Method == http.MethodGet:
		router.handleBookmarks(writer, request)
	case path == "/bookmarks" && request.Method == http.MethodPost:
		router.handleBookmarkCreate(writer, request)
	case strings.HasPrefix(path, "/bookmarks/") && strings.HasSuffix(path, "/edit") && request.Method == http.MethodGet:
		router.handleBookmarkEdit(writer, request, bookmarkIDFromPath(path))
	case strings.HasPrefix(path, "/bookmarks/") && strings.HasSuffix(path, "/delete") && request.Method == http.MethodGet:
		router.handleBookmarkDeleteForm(writer, request, bookmarkIDFromPath(path))
	case strings.HasPrefix(path, "/bookmarks/") && strings.HasSuffix(path, "/delete") && request.Method == http.MethodPost:
		router.handleBookmarkDelete(writer, request, bookmarkIDFromPath(path))
	case strings.HasPrefix(path, "/bookmarks/") && request.Method == http.MethodPost:
		router.handleBookmarkUpdate(writer, request, bookmarkIDFromPath(path))
	case path == "/bookmarks/new" && request.Method == http.MethodGet:
		router.handleAddForm(writer, request)
	case path == "/search":
		router.handleSearch(writer, request)
	case path == "/login" && request.Method == http.MethodGet:
		router.handleLoginGet(writer, request)
	case path == "/login" && request.Method == http.MethodPost:
		router.handleLoginPost(writer, request)
	case path == "/logout" && request.Method == http.MethodPost:
		router.handleLogout(writer, request)
	case path == "/captures" && request.Method == http.MethodPost:
		router.handleCaptureStart(writer, request)
	case strings.HasPrefix(path, "/captures/") && strings.HasSuffix(path, "/retry") && request.Method == http.MethodPost:
		router.handleCaptureRetry(writer, request, captureIDFromPath(path))
	case strings.HasPrefix(path, "/captures/") && strings.HasSuffix(path, "/discard") && request.Method == http.MethodPost:
		router.handleCaptureDiscard(writer, request, captureIDFromPath(path))
		case strings.HasPrefix(path, "/captures/") && strings.HasSuffix(path, "/preview") && request.Method == http.MethodGet:
			router.handleCapturePreview(writer, request, captureIDFromPath(path))
	case strings.HasPrefix(path, "/captures/") && request.Method == http.MethodGet:
		router.handleCaptureStatus(writer, request, captureIDFromPath(path))
	case path == "/configuration" && request.Method == http.MethodGet:
		router.handleConfigurationGet(writer, request)
	case path == "/configuration" && request.Method == http.MethodPost:
		router.handleConfigurationPost(writer, request)
	case path == "/favicon" && request.Method == http.MethodGet:
		router.handleFavicon(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func bookmarkIDFromPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/bookmarks/")
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		return trimmed[:slash]
	}
	return trimmed
}

func ClientAddress(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		referer := request.Referer()
		if referer == "" {
			return false
		}
		parsed, err := url.Parse(referer)
		return err == nil && parsed.Host == request.Host
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, request.Host)
}

func randomID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}
