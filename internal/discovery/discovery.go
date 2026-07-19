package discovery

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// ignoredTemplateDirs lists directories we skip during auto-discovery to avoid vendor assets.
var ignoredTemplateDirs = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	".git":         {},
	// Uploaded files, never templates, and a tree that is written and removed
	// while the app runs — so skipping it also keeps discovery out of the way of
	// the churn walkErrAction tolerates. A complement rather than the fix: the
	// upload directory is configurable, so only walkErrAction covers the general
	// case. Note this shadows a real template directory literally named
	// .uploads, which is why the general fix carries the weight.
	".uploads": {},
}

// walkErrAction decides what a WalkDir error means for template discovery.
//
// A path disappearing mid-walk is not a discovery failure: anything may be
// writing and removing directories under the tree while it is searched, and
// aborting the walk over one of them fails template construction outright.
//
// The root is the exception — WalkDir reports a missing root through this same
// callback, and that means the caller asked to search a directory that does not
// exist, which is worth surfacing rather than reporting as "no templates found".
// Errors that are not ErrNotExist, a permissions problem say, always surface
// too, so tolerance widens to the concurrent-removal case and nothing else.
func walkErrAction(root, path string, d fs.DirEntry, err error) error {
	if !errors.Is(err, fs.ErrNotExist) || path == root {
		return err
	}
	// SkipDir on a non-directory would skip the rest of the containing
	// directory, silently dropping sibling templates, so it is only correct for
	// a directory. The non-directory path is defensive: WalkDir currently pairs
	// a non-nil error only with a directory entry or with a nil entry at the
	// root, so nothing reaches it today.
	if d != nil && d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

// DiscoverTemplateFiles searches for template files in the specified directory and subdirectories.
// If baseDir is empty, attempts to determine the caller's directory using runtime.Caller (for backward compatibility).
// If no templates are found and baseDir was auto-detected, tries smart fallback (./templates or .).
func DiscoverTemplateFiles(baseDir string, customIgnoreDirs []string) ([]string, error) {
	var autoDetected bool
	originalBaseDir := baseDir

	// If no base directory provided, try to determine caller's directory for backward compatibility
	if baseDir == "" {
		autoDetected = true
		// Try multiple call stack depths to find the caller's directory
		// This handles cases where intermediate function calls add frames
		// Try depths 2, 3, and 4 (most common scenarios)
		for depth := 2; depth <= 4; depth++ {
			_, filename, _, ok := runtime.Caller(depth)
			if !ok {
				continue
			}
			dir := filepath.Dir(filename)
			// Verify the directory exists and is accessible
			if _, err := filepath.Abs(dir); err == nil {
				baseDir = dir
				break
			}
		}

		// If we still couldn't determine the base directory, use smart fallback immediately
		if baseDir == "" {
			// Try ./templates first (common convention)
			if _, err := os.Stat("./templates"); err == nil {
				baseDir = "./templates"
			} else {
				// Fall back to current directory (for colocated templates)
				baseDir = "."
			}
		}
	}

	// Helper function to search for templates in a given directory
	searchDir := func(dir string) ([]string, error) {
		var files []string

		// Build combined ignore map (default + custom)
		ignoreMap := make(map[string]struct{}, len(ignoredTemplateDirs)+len(customIgnoreDirs))
		for k, v := range ignoredTemplateDirs {
			ignoreMap[k] = v
		}
		for _, d := range customIgnoreDirs {
			ignoreMap[d] = struct{}{}
		}

		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return walkErrAction(dir, path, d, err)
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

		return files, err
	}

	// Try searching in the determined directory
	files, err := searchDir(baseDir)
	if err != nil {
		return nil, err
	}

	// If auto-detected (not explicitly provided), no templates found, and we haven't tried smart fallback yet
	// then try the smart fallback paths
	if autoDetected && len(files) == 0 && originalBaseDir == "" {
		// Runtime.Caller might have returned a path that doesn't have templates (e.g., Go build cache)
		// Try smart fallback: ./templates first, then current directory
		var fallbackDir string
		if _, err := os.Stat("./templates"); err == nil {
			fallbackDir = "./templates"
		} else {
			fallbackDir = "."
		}

		// Only try fallback if it's different from what runtime.Caller found
		if fallbackDir != baseDir {
			fallbackFiles, fallbackErr := searchDir(fallbackDir)
			if fallbackErr == nil && len(fallbackFiles) > 0 {
				return fallbackFiles, nil
			}
		}
	}

	return files, nil
}
