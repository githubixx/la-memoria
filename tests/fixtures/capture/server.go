// Package capture provides a deterministic HTTP fixture server used by
// capture-related tests so screenshot behavior does not depend on the
// public internet.
package capture

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// NewServer starts an httptest server exposing predictable fixture pages:
//   - "/capture-target" always succeeds quickly with static content.
//   - "/capture-target/redirect" issues one redirect to "/capture-target".
//   - "/capture-target/slow" waits for the request context before responding,
//     letting tests exercise capture timeouts deterministically.
//   - "/capture-target/unreachable" always returns 503.
func NewServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/capture-target", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(writer, "<!doctype html><title>Capture fixture</title><h1>Capture fixture</h1>")
	})
	mux.HandleFunc("/capture-target/redirect", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/capture-target", http.StatusFound)
	})
	mux.HandleFunc("/capture-target/slow", func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	})
	mux.HandleFunc("/capture-target/unreachable", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	})
	return httptest.NewServer(mux)
}
