package httpweb

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

// pageContext carries the fields every page template needs from the shared
// layout and menu fragment.
type pageContext struct {
	PageTitle     string
	Authenticated bool
	CSRFToken     string
}

// resultsViewModel is the data shape shared by the list and search page
// templates through the "bookmark_results" fragment.
type resultsViewModel struct {
	Items          []model.Bookmark
	Pagination     model.Pagination
	TotalWithinCap int
	IsCapped       bool
	Authenticated  bool
}

type listViewModel struct {
	pageContext
	Tag     string
	Results resultsViewModel
}

type searchViewModel struct {
	pageContext
	Query struct {
		Tag   string
		Words string
	}
	HasSearched     bool
	ValidationError string
	Results         resultsViewModel
}

func (router *Router) handleRoot(writer http.ResponseWriter, request *http.Request) {
	defaultView := router.dependencies.DefaultView
	if router.dependencies.Configuration != nil {
		if configuration, err := router.dependencies.Configuration.Get(request.Context()); err == nil {
			defaultView = configuration.DefaultView
		}
	}
	if defaultView == "" {
		defaultView = "list"
	}
	destination := "/bookmarks"
	if defaultView == "search" {
		destination = "/search"
	}
	if defaultView == "add" {
		destination = "/bookmarks/new"
	}
	if defaultView == "configuration" {
		destination = "/configuration"
	}
	if (defaultView == "add" || defaultView == "configuration") && !isAuthenticated(router, request) {
		destination = "/login?return_to=" + url.QueryEscape(destination)
	}
	http.Redirect(writer, request, destination, http.StatusSeeOther)
}

func isAuthenticated(router *Router, request *http.Request) bool {
	_, authenticated := router.principal(request)
	return authenticated
}

func (router *Router) handleBookmarks(writer http.ResponseWriter, request *http.Request) {
	if router.renderer == nil || router.dependencies.Browser == nil {
		http.Error(writer, "bookmarks unavailable", http.StatusInternalServerError)
		return
	}
	query := request.URL.Query()
	tag := query.Get("tag")
	page, _ := strconv.Atoi(query.Get("page"))
	if page == 0 {
		page = 1
	}

	result, err := router.dependencies.Browser.Browse(request.Context(), model.BrowseQuery{Tag: tag, Page: page})
	if err != nil {
		http.Error(writer, "bookmarks unavailable", http.StatusInternalServerError)
		return
	}

	_, authenticated := router.principal(request)
	view := listViewModel{
		pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: authenticated},
		Tag:         tag,
		Results: resultsViewModel{
			Items:         result.Items,
			Pagination:    result.Pagination,
			Authenticated: authenticated,
		},
	}
	if err := router.renderer.Render(writer, request, "list", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}

func (router *Router) handleSearch(writer http.ResponseWriter, request *http.Request) {
	if router.renderer == nil || router.dependencies.Searcher == nil {
		http.Error(writer, "search unavailable", http.StatusInternalServerError)
		return
	}
	query := request.URL.Query()
	_, hasTag := query["tag"]
	_, hasWords := query["q"]
	tag := query.Get("tag")
	words := query.Get("q")
	page, _ := strconv.Atoi(query.Get("page"))
	if page == 0 {
		page = 1
	}

	_, authenticated := router.principal(request)
	view := searchViewModel{pageContext: pageContext{PageTitle: "Bookmarker", Authenticated: authenticated}}
	view.Query.Tag = tag
	view.Query.Words = words

	if !hasTag && !hasWords {
		// Unsubmitted form load: show the search form without executing a
		// search or reporting a validation error.
		if err := router.renderer.Render(writer, request, "search", view); err != nil {
			http.Error(writer, "render failed", http.StatusInternalServerError)
		}
		return
	}

	view.HasSearched = true
	result, err := router.dependencies.Searcher.Search(request.Context(), model.SearchQuery{Tag: tag, Words: model.SplitSearchWords(words), Page: page})
	if err != nil {
		view.ValidationError = err.Error()
		writer.WriteHeader(http.StatusUnprocessableEntity)
		if renderErr := router.renderer.Render(writer, request, "search", view); renderErr != nil {
			http.Error(writer, "render failed", http.StatusInternalServerError)
		}
		return
	}

	view.Results = resultsViewModel{
		Items:          result.Items,
		Pagination:     result.Pagination,
		TotalWithinCap: result.TotalWithinCap,
		IsCapped:       result.IsCapped,
		Authenticated:  authenticated,
	}
	if err := router.renderer.Render(writer, request, "search", view); err != nil {
		http.Error(writer, "render failed", http.StatusInternalServerError)
	}
}
