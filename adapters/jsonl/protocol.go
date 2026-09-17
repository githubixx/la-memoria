package jsonl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/githubixx/la-memoria/bookmarker/usecase"
)

const MaximumLineBytes = 64 * 1024

// Dependencies configures the JSON Lines delivery adapter's composition.
type Dependencies struct {
	Browser        *usecase.Browser
	Searcher       *usecase.Searcher
	Authenticator  *usecase.Authenticator
	CaptureService *usecase.CaptureService
	Creator        *usecase.Creator
	Maintainer     *usecase.Maintainer
	Configuration  *usecase.ConfigurationService
}

type Server struct{ dependencies Dependencies }

func NewServer(dependencies Dependencies) *Server { return &Server{dependencies: dependencies} }

type request struct {
	Version   string          `json:"version"`
	ID        string          `json:"id"`
	Operation string          `json:"operation"`
	Payload   json.RawMessage `json:"payload"`
}

func (server *Server) Serve(input io.Reader, output, diagnostics io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), MaximumLineBytes+1)
	var sessionToken string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > MaximumLineBytes {
			write(output, responseError{"", "line_too_large"})
			continue
		}
		server.handle(line, output, &sessionToken)
	}
	if err := scanner.Err(); err != nil {
		if err == bufio.ErrTooLong {
			write(output, responseError{"", "line_too_large"})
			return nil
		}
		return fmt.Errorf("read JSON Lines: %w", err)
	}
	return nil
}
func (server *Server) handle(line string, output io.Writer, sessionToken *string) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	var value request
	if err := decoder.Decode(&value); err != nil {
		code := "invalid_json"
		if strings.Contains(err.Error(), "unknown field") {
			code = "unknown_field"
		}
		write(output, responseError{"", code})
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		write(output, responseError{value.ID, "invalid_json"})
		return
	}
	if value.Version != "v1" {
		write(output, responseError{value.ID, "unsupported_version"})
		return
	}
	if value.ID == "" || value.Operation == "" || value.Payload == nil {
		write(output, responseError{value.ID, "validation_failed"})
		return
	}
	ctx := context.Background()
	switch value.Operation {
	case "bookmark.list":
		server.handleBookmarkList(ctx, server.dependencies.Browser, value, output)
	case "bookmark.search":
		server.handleBookmarkSearch(ctx, server.dependencies.Searcher, value, output)
	case "auth.sign_in":
		server.handleSignIn(ctx, value, output, sessionToken)
	case "auth.sign_out":
		server.handleSignOut(ctx, value, output, sessionToken)
	case "capture.start":
		server.handleCaptureStart(ctx, value, output, *sessionToken)
	case "capture.status":
		server.handleCaptureStatus(ctx, value, output, *sessionToken)
	case "capture.retry":
		server.handleCaptureRetry(ctx, value, output, *sessionToken)
	case "capture.discard":
		server.handleCaptureDiscard(ctx, value, output, *sessionToken)
	case "bookmark.create":
		server.handleBookmarkCreate(ctx, value, output, *sessionToken)
	case "bookmark.update":
		server.handleBookmarkUpdate(ctx, value, output, *sessionToken)
	case "bookmark.delete":
		server.handleBookmarkDelete(ctx, value, output, *sessionToken)
	case "configuration.get":
		server.handleConfigurationGet(ctx, value, output, *sessionToken)
	case "configuration.validate":
		server.handleConfigurationValidate(ctx, value, output, *sessionToken, false)
	case "configuration.update":
		server.handleConfigurationValidate(ctx, value, output, *sessionToken, true)
	default:
		write(output, responseError{value.ID, "unknown_operation"})
	}
}

type responseOK struct {
	Version string `json:"version"`
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Result  any    `json:"result"`
}

func (value responseOK) MarshalJSON() ([]byte, error) {
	type alias responseOK
	return json.Marshal(alias{Version: "v1", ID: value.ID, OK: true, Result: value.Result})
}

type responseError struct{ ID, Code string }

func (value responseError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Version string `json:"version"`
		ID      string `json:"id"`
		OK      bool   `json:"ok"`
		Error   struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}{Version: "v1", ID: value.ID, OK: false, Error: struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
	}{Code: value.Code, Message: "request could not be processed"}})
}
func write(writer io.Writer, value any) {
	encoded, _ := json.Marshal(value)
	_, _ = fmt.Fprintln(writer, string(encoded))
}
