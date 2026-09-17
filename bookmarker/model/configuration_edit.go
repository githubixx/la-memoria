package model

import (
	"path/filepath"
	"strings"
)

type EditableConfiguration struct {
	Branding    BrandingConfig   `json:"branding"`
	DefaultView string           `json:"default_view"`
	Database    DatabaseConfig   `json:"database"`
	Screenshots ScreenshotConfig `json:"screenshots"`
	Search      SearchConfig     `json:"search"`
}

func (configuration Configuration) Redacted() EditableConfiguration {
	database := configuration.Database
	database.Password = ""
	return EditableConfiguration{Branding: configuration.Branding, DefaultView: configuration.DefaultView, Database: database, Screenshots: configuration.Screenshots, Search: configuration.Search}
}

func ValidateEditableConfiguration(candidate EditableConfiguration) (EditableConfiguration, error) {
	candidate.Branding.PageTitle = strings.TrimSpace(candidate.Branding.PageTitle)
	candidate.Branding.FaviconPath = strings.TrimSpace(candidate.Branding.FaviconPath)
	candidate.DefaultView = strings.TrimSpace(candidate.DefaultView)
	candidate.Database.Host = strings.TrimSpace(candidate.Database.Host)
	candidate.Database.Name = strings.TrimSpace(candidate.Database.Name)
	candidate.Database.User = strings.TrimSpace(candidate.Database.User)
	candidate.Database.PasswordEnv = strings.TrimSpace(candidate.Database.PasswordEnv)
	candidate.Database.TLSMode = strings.TrimSpace(candidate.Database.TLSMode)
	candidate.Screenshots.Root = strings.TrimSpace(candidate.Screenshots.Root)
	if candidate.Database.Password != "" {
		return EditableConfiguration{}, invalidConfiguration("database.password", "must be an environment reference")
	}
	candidate.Database.Password = ""
	if candidate.Branding.PageTitle == "" || len(candidate.Branding.PageTitle) > 120 {
		return EditableConfiguration{}, invalidConfiguration("page_title", "must be between 1 and 120 characters")
	}
	if candidate.Branding.FaviconPath != "" && (filepath.IsAbs(candidate.Branding.FaviconPath) || strings.Contains(filepath.Clean(candidate.Branding.FaviconPath), "..")) {
		return EditableConfiguration{}, invalidConfiguration("favicon_path", "must be a contained asset path")
	}
	switch candidate.DefaultView {
	case "list", "add", "search", "configuration":
	default:
		return EditableConfiguration{}, invalidConfiguration("default_view", "must be list, add, search, or configuration")
	}
	if candidate.Screenshots.Root == "" {
		return EditableConfiguration{}, invalidConfiguration("screenshot_root", "must not be empty")
	}
	if candidate.Database.Host == "" || candidate.Database.Name == "" || candidate.Database.User == "" || candidate.Database.Port < 1 || candidate.Database.Port > 65535 {
		return EditableConfiguration{}, invalidConfiguration("database", "contains invalid connection settings")
	}
	if !environmentNamePattern.MatchString(candidate.Database.PasswordEnv) {
		return EditableConfiguration{}, invalidConfiguration("database.password_env", "must name an environment variable")
	}
	switch candidate.Database.TLSMode {
	case "disable", "require", "verify-ca", "verify-full":
	default:
		return EditableConfiguration{}, invalidConfiguration("database.tls_mode", "is not supported")
	}
	if candidate.Search.PageSize < 1 || candidate.Search.PageSize > 100 || candidate.Search.MaximumResults < candidate.Search.PageSize || candidate.Search.MaximumResults > 10000 {
		return EditableConfiguration{}, invalidConfiguration("search", "contains invalid result limits")
	}
	return candidate, nil
}

func ApplyEditableConfiguration(current Configuration, candidate EditableConfiguration) (Configuration, error) {
	validated, err := ValidateEditableConfiguration(candidate)
	if err != nil {
		return Configuration{}, err
	}
	updated := current
	updated.Branding, updated.DefaultView, updated.Database = validated.Branding, validated.DefaultView, validated.Database
	if updated.Database.PasswordEnv == current.Database.PasswordEnv {
		updated.Database.Password = current.Database.Password
	}
	updated.Screenshots, updated.Search = validated.Screenshots, validated.Search
	return updated, nil
}

func invalidConfiguration(field, message string) error {
	return &ValidationError{Code: "validation_failed", Field: field, Message: message}
}
