package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscoverTemplateFiles verifies template file discovery
func TestDiscoverTemplateFiles(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create test template files
	testFiles := []string{
		"template1.tmpl",
		"template2.html",
		"template3.gotmpl",
		"notatemplate.txt", // Should be ignored
	}

	for _, name := range testFiles {
		f, err := os.Create(filepath.Join(tmpDir, name))
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.Close()
	}

	// Test discovery
	files, err := DiscoverTemplateFiles(tmpDir, nil)
	if err != nil {
		t.Fatalf("DiscoverTemplateFiles failed: %v", err)
	}

	// Should find 3 template files (not the .txt file)
	if len(files) != 3 {
		t.Errorf("Expected 3 template files, got %d", len(files))
	}

	// Verify all are valid paths
	for _, f := range files {
		if !filepath.IsAbs(f) {
			t.Errorf("Expected absolute path, got: %s", f)
		}
	}
}

// TestDiscoverTemplateFiles_WithIgnoreDirs verifies directory ignoring
func TestDiscoverTemplateFiles_WithIgnoreDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories
	nodeModules := filepath.Join(tmpDir, "node_modules")
	vendor := filepath.Join(tmpDir, "vendor")
	customIgnore := filepath.Join(tmpDir, "ignored")

	for _, dir := range []string{nodeModules, vendor, customIgnore} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}

	// Create template files in ignored directories
	for _, dir := range []string{nodeModules, vendor, customIgnore} {
		f, err := os.Create(filepath.Join(dir, "template.tmpl"))
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.Close()
	}

	// Create a template in the root
	rootFile := filepath.Join(tmpDir, "root.tmpl")
	f, err := os.Create(rootFile)
	if err != nil {
		t.Fatalf("Failed to create root file: %v", err)
	}
	f.Close()

	// Test discovery with custom ignore
	files, err := DiscoverTemplateFiles(tmpDir, []string{"ignored"})
	if err != nil {
		t.Fatalf("DiscoverTemplateFiles failed: %v", err)
	}

	// Should only find the root file (node_modules and vendor are always ignored)
	if len(files) != 1 {
		t.Errorf("Expected 1 template file, got %d: %v", len(files), files)
	}

	if len(files) > 0 && filepath.Base(files[0]) != "root.tmpl" {
		t.Errorf("Expected root.tmpl, got %s", files[0])
	}
}

// TestDiscoverTemplateFiles_EmptyDirectory verifies handling of empty directories
func TestDiscoverTemplateFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := DiscoverTemplateFiles(tmpDir, nil)
	if err != nil {
		t.Fatalf("DiscoverTemplateFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files in empty directory, got %d", len(files))
	}
}

// TestDiscoverTemplateFiles_NonexistentDirectory verifies error handling
func TestDiscoverTemplateFiles_NonexistentDirectory(t *testing.T) {
	_, err := DiscoverTemplateFiles("/nonexistent/path/that/does/not/exist", nil)
	if err == nil {
		t.Error("Expected error for nonexistent directory, got nil")
	}
}

// TestDiscoverTemplateFiles_EmptyBaseDir verifies runtime.Caller fallback
func TestDiscoverTemplateFiles_EmptyBaseDir(t *testing.T) {
	// When baseDir is empty, it tries runtime.Caller with multiple depths
	// This will likely return nil since we're in a test context
	files, err := DiscoverTemplateFiles("", nil)

	// Should not error, just return empty or nil
	if err != nil {
		t.Fatalf("Expected no error for empty baseDir, got: %v", err)
	}

	// Files can be nil or empty
	_ = files
}

// TestDiscoverTemplateFiles_MultipleCallDepths verifies runtime.Caller tries multiple depths
func TestDiscoverTemplateFiles_MultipleCallDepths(t *testing.T) {
	// Create a temporary directory with test template
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.tmpl")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	// Test with explicit baseDir - should always work
	files, err := DiscoverTemplateFiles(tmpDir, nil)
	if err != nil {
		t.Fatalf("DiscoverTemplateFiles with explicit baseDir failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 template file, got %d", len(files))
	}

	// Test with empty baseDir through wrapper functions to simulate different depths
	callFromDepth2 := func() ([]string, error) {
		return DiscoverTemplateFiles("", nil)
	}

	callFromDepth3 := func() ([]string, error) {
		return callFromDepth2()
	}

	callFromDepth4 := func() ([]string, error) {
		return callFromDepth3()
	}

	// Try calling from different depths - should handle all gracefully
	for i, fn := range []func() ([]string, error){callFromDepth2, callFromDepth3, callFromDepth4} {
		files, err := fn()
		if err != nil {
			t.Errorf("Call depth %d: unexpected error: %v", i+2, err)
		}
		// We don't verify files content here because runtime.Caller may or may not work
		// depending on call depth - we just verify it doesn't crash
		_ = files
	}
}

// TestDiscoverTemplateFiles_SmartFallback verifies smart fallback behavior
// When runtime.Caller fails, should try ./templates if it exists, otherwise use .
func TestDiscoverTemplateFiles_SmartFallback(t *testing.T) {
	// Save current directory to restore later
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	t.Run("fallback to templates directory when it exists", func(t *testing.T) {
		// Create temporary directory structure
		tmpDir := t.TempDir()
		templatesDir := filepath.Join(tmpDir, "templates")
		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			t.Fatalf("Failed to create templates directory: %v", err)
		}

		// Create template file in ./templates
		templateFile := filepath.Join(templatesDir, "test.tmpl")
		f, err := os.Create(templateFile)
		if err != nil {
			t.Fatalf("Failed to create template file: %v", err)
		}
		f.Close()

		// Change to tmpDir so ./templates is accessible
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}

		// Call with empty baseDir - should fall back to ./templates
		files, err := DiscoverTemplateFiles("", nil)
		if err != nil {
			t.Fatalf("DiscoverTemplateFiles failed: %v", err)
		}

		// Should find the template file
		if len(files) != 1 {
			t.Errorf("Expected 1 template file with ./templates fallback, got %d", len(files))
		}
	})

	t.Run("fallback to current directory when templates does not exist", func(t *testing.T) {
		// Create temporary directory WITHOUT templates subdirectory
		tmpDir := t.TempDir()

		// Create template file directly in tmpDir
		templateFile := filepath.Join(tmpDir, "colocated.tmpl")
		f, err := os.Create(templateFile)
		if err != nil {
			t.Fatalf("Failed to create template file: %v", err)
		}
		f.Close()

		// Change to tmpDir
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}

		// Call with empty baseDir - should fall back to . (current directory)
		files, err := DiscoverTemplateFiles("", nil)
		if err != nil {
			t.Fatalf("DiscoverTemplateFiles failed: %v", err)
		}

		// Should find the colocated template file
		if len(files) != 1 {
			t.Errorf("Expected 1 template file with . fallback, got %d", len(files))
		}
	})

	t.Run("nested discovery from current directory", func(t *testing.T) {
		// Create directory structure with nested templates
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "views", "auth")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		// Create templates at different levels
		files := map[string]string{
			filepath.Join(tmpDir, "root.tmpl"):            "root",
			filepath.Join(tmpDir, "views", "index.tmpl"):  "views",
			filepath.Join(subDir, "login.tmpl"):           "nested",
		}

		for path := range files {
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("Failed to create template file %s: %v", path, err)
			}
			f.Close()
		}

		// Change to tmpDir
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}

		// Call with empty baseDir - should discover all nested templates
		discoveredFiles, err := DiscoverTemplateFiles("", nil)
		if err != nil {
			t.Fatalf("DiscoverTemplateFiles failed: %v", err)
		}

		// Should find all 3 template files (recursive discovery)
		if len(discoveredFiles) != 3 {
			t.Errorf("Expected 3 template files with nested discovery, got %d", len(discoveredFiles))
		}
	})
}
