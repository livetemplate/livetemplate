package livetemplate

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

// discoverTemplateFiles searches for template files in the calling directory and subdirectories
func discoverTemplateFiles() ([]string, error) {
	// Get the caller's directory (2 frames up: discoverTemplateFiles -> New -> user code)
	_, filename, _, ok := runtime.Caller(2)
	if !ok {
		return nil, nil // Can't determine caller, skip auto-discovery
	}

	baseDir := filepath.Dir(filename)
	var files []string

	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".tmpl" || ext == ".html" || ext == ".gotmpl" {
				files = append(files, path)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// normalizeStoreName converts a store name to lowercase for case-insensitive matching
func normalizeStoreName(name string) string {
	return strings.ToLower(name)
}
