package usecase

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/githubixx/la-memoria/bookmarker/model"
	"github.com/githubixx/la-memoria/bookmarker/ports"
)

type ScreenshotService struct {
	lookup ports.ScreenshotLookup
	files  ports.ScreenshotFileStore
}

func NewScreenshotService(lookup ports.ScreenshotLookup, files ports.ScreenshotFileStore) *ScreenshotService {
	return &ScreenshotService{lookup: lookup, files: files}
}

func (service *ScreenshotService) Read(ctx context.Context, id model.ID) ([]byte, error) {
	key, err := service.lookup.ScreenshotStorageKey(ctx, id)
	if err != nil {
		return nil, err
	}
	image, err := service.files.Read(ctx, key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, model.ErrScreenshotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read screenshot: %w", err)
	}
	return image, nil
}
