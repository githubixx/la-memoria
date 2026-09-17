package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

func TestConfigurationValidatorReturnsSafeErrorForUnreachableDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	configuration := model.Configuration{Database: model.DatabaseConfig{
		Host: "127.0.0.1", Port: 1, Name: "bookmarker", User: "bookmarker", Password: "resolved-secret", TLSMode: "require",
	}}

	err := NewConfigurationValidator().Validate(ctx, configuration)
	if model.ErrorCode(err) != "dependency_unavailable" {
		t.Fatalf("validation error code = %q, want dependency_unavailable: %v", model.ErrorCode(err), err)
	}
	if strings.Contains(err.Error(), configuration.Database.Password) {
		t.Fatalf("validation error exposed resolved password: %v", err)
	}
}
