package livetemplate

import (
	"sync"

	"github.com/livetemplate/livetemplate/internal/upload"
)

var uploadFactoriesOnce sync.Once

// initUploadFactories initializes the upload factory functions.
// Called lazily when first handler is created to avoid import cycles.
func initUploadFactories() {
	uploadFactoriesOnce.Do(func() {
		newUploadRegistry = func() uploadRegistry {
			return upload.NewRegistry()
		}
		newUploadTempFileManager = func(baseDir string) (uploadTempFileManager, error) {
			return upload.NewTempFileManager(baseDir)
		}
	})
}
