package discovery

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

// ignoredTemplateDirs lists directories we skip during auto-discovery to avoid vendor assets.
var ignoredTemplateDirs = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	".git":         {},
	"internal":     {}, // Skip internal directories (e.g., code generator templates with mixed delimiters)
}

// DiscoverTemplateFiles searches for template files in the specified directory and subdirectories.
// If baseDir is empty, attempts to determine the caller's directory using runtime.Caller (for backward compatibility).
// Returns nil if baseDir is empty and caller directory cannot be determined.
func DiscoverTemplateFiles(baseDir string, customIgnoreDirs []string) ([]string, error) {
	// If no base directory provided, try to determine caller's directory for backward compatibility
	if baseDir == "" {
		// Try to get the caller's directory (3 frames up: discoverTemplateFiles -> New -> user code)
		// This is brittle and maintained only for backward compatibility
		_, filename, _, ok := runtime.Caller(3)
		if !ok {
			return nil, nil // Can't determine caller, skip auto-discovery
		}
		baseDir = filepath.Dir(filename)
	}
	var files []string

	// Build combined ignore map (default + custom)
	ignoreMap := make(map[string]struct{}, len(ignoredTemplateDirs)+len(customIgnoreDirs))
	for k, v := range ignoredTemplateDirs {
		ignoreMap[k] = v
	}
	for _, dir := range customIgnoreDirs {
		ignoreMap[dir] = struct{}{}
	}

	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if _, skip := ignoreMap[d.Name()]; skip {
				return fs.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".tmpl" || ext == ".html" || ext == ".gotmpl" {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// NormalizeStoreName converts a store name to lowercase for case-insensitive matching
func NormalizeStoreName(name string) string {
	return strings.ToLower(name)
}
