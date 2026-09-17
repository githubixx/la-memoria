package filesystem

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

type ConfigurationValidator struct{ assetRoot string }

func NewConfigurationValidator(assetRoot string) ConfigurationValidator {
	return ConfigurationValidator{assetRoot: assetRoot}
}

func (validator ConfigurationValidator) Validate(_ context.Context, configuration model.Configuration) error {
	root, err := filepath.Abs(configuration.Screenshots.Root)
	if err != nil {
		return unavailable("screenshot storage could not be validated")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return unavailable("screenshot storage could not be validated")
	}
	probe, err := os.CreateTemp(root, ".validation-*")
	if err != nil {
		return unavailable("screenshot storage could not be validated")
	}
	probePath := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return unavailable("screenshot storage could not be validated")
	}
	if err := os.Remove(probePath); err != nil {
		return unavailable("screenshot storage could not be validated")
	}
	if configuration.Branding.FaviconPath == "" {
		return nil
	}
	path := filepath.Join(validator.assetRoot, configuration.Branding.FaviconPath)
	absoluteRoot, rootErr := filepath.Abs(validator.assetRoot)
	resolvedRoot, rootErr := filepath.EvalSymlinks(absoluteRoot)
	resolvedPath, pathErr := filepath.EvalSymlinks(path)
	if rootErr != nil || pathErr != nil || resolvedPath != resolvedRoot && !bytes.HasPrefix([]byte(resolvedPath), []byte(resolvedRoot+string(filepath.Separator))) {
		return &model.ValidationError{Code: "validation_failed", Field: "favicon_path", Message: "must be a contained image asset"}
	}
	contents, err := os.ReadFile(resolvedPath)
	if err != nil || len(contents) < len(pngSignature) || !bytes.Equal(contents[:len(pngSignature)], pngSignature) {
		return &model.ValidationError{Code: "validation_failed", Field: "favicon_path", Message: "must reference a PNG image"}
	}
	return nil
}

func unavailable(message string) error {
	return &model.ApplicationError{Code: "dependency_unavailable", Message: message, Retryable: true}
}
