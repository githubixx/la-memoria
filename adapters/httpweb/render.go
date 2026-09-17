package httpweb

import (
	"fmt"
	"html/template"
	"net/http"
)

// Renderer holds one parsed template set per page, each combining the
// shared layout and fragments with that page's own content block so the
// same set can render either the full document or the "content" fragment.
type Renderer struct {
	pages     map[string]*template.Template
	fragments map[string]*template.Template
}

var pageNames = []string{"list", "search", "login", "add", "edit", "delete", "configuration"}
var fragmentNames = []string{"capture", "delete_result"}

func NewRenderer(templatesDir string) (*Renderer, error) {
	base, err := template.ParseFiles(
		templatesDir+"/layouts/base.html",
		templatesDir+"/fragments/menu.html",
		templatesDir+"/fragments/bookmark_results.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse shared templates: %w", err)
	}
	pages := map[string]*template.Template{}
	for _, name := range pageNames {
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone template set for %q: %w", name, err)
		}
		clone, err = clone.ParseFiles(fmt.Sprintf("%s/pages/%s.html", templatesDir, name))
		if err != nil {
			return nil, fmt.Errorf("parse page template %q: %w", name, err)
		}
		pages[name] = clone
	}
	fragments := map[string]*template.Template{}
	for _, name := range fragmentNames {
		fragment, err := template.ParseFiles(fmt.Sprintf("%s/fragments/%s.html", templatesDir, name))
		if err != nil {
			return nil, fmt.Errorf("parse fragment template %q: %w", name, err)
		}
		fragments[name] = fragment
	}
	return &Renderer{pages: pages, fragments: fragments}, nil
}

// Render writes the full page when the request is not an htmx request, or
// just the "content" fragment when it is, always advertising Vary:
// HX-Request so caches keep the two forms distinct.
func (renderer *Renderer) Render(writer http.ResponseWriter, request *http.Request, page string, data any) error {
	tmpl, ok := renderer.pages[page]
	if !ok {
		return fmt.Errorf("unknown page template %q", page)
	}
	writer.Header().Set("Vary", "HX-Request")
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if request.Header.Get("HX-Request") == "true" {
		return tmpl.ExecuteTemplate(writer, "content", data)
	}
	return tmpl.ExecuteTemplate(writer, "base.html", data)
}

// RenderFragment writes a standalone fragment template (not wrapped in the
// base layout), used for auxiliary regions such as capture status that are
// always swapped in place rather than navigated to directly.
func (renderer *Renderer) RenderFragment(writer http.ResponseWriter, name string, data any) error {
	tmpl, ok := renderer.fragments[name]
	if !ok {
		return fmt.Errorf("unknown fragment template %q", name)
	}
	return tmpl.Execute(writer, data)
}
