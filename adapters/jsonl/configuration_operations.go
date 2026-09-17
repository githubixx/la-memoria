package jsonl

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

func (server *Server) handleConfigurationGet(ctx context.Context, value request, output io.Writer, sessionToken string) {
	if _, ok := server.requirePrincipal(ctx, sessionToken); !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.Configuration == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	if strings.TrimSpace(string(value.Payload)) != "{}" {
		write(output, responseError{value.ID, "unknown_field"})
		return
	}
	configuration, err := server.dependencies.Configuration.Get(ctx)
	if err != nil {
		write(output, responseError{value.ID, model.ErrorCode(err)})
		return
	}
	write(output, responseOK{ID: value.ID, Result: configuration})
}

func (server *Server) handleConfigurationValidate(ctx context.Context, value request, output io.Writer, sessionToken string, activate bool) {
	if _, ok := server.requirePrincipal(ctx, sessionToken); !ok {
		write(output, responseError{value.ID, "authentication_required"})
		return
	}
	if server.dependencies.Configuration == nil {
		write(output, responseError{value.ID, "dependency_unavailable"})
		return
	}
	decoder := json.NewDecoder(strings.NewReader(string(value.Payload)))
	decoder.DisallowUnknownFields()
	var candidate model.EditableConfiguration
	if err := decoder.Decode(&candidate); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		write(output, responseError{value.ID, "unknown_field"})
		return
	}
	var change struct {
		Configuration   model.EditableConfiguration `json:"configuration"`
		RestartRequired bool                        `json:"restart_required"`
	}
	if activate {
		result, err := server.dependencies.Configuration.Update(ctx, candidate)
		if err != nil {
			write(output, responseError{value.ID, model.ErrorCode(err)})
			return
		}
		change.Configuration, change.RestartRequired = result.Configuration, result.RestartRequired
	} else {
		result, err := server.dependencies.Configuration.Validate(ctx, candidate)
		if err != nil {
			write(output, responseError{value.ID, model.ErrorCode(err)})
			return
		}
		change.Configuration, change.RestartRequired = result.Configuration, result.RestartRequired
	}
	write(output, responseOK{ID: value.ID, Result: change})
}
