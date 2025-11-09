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
	// When baseDir is empty, it tries runtime.Caller(3)
	// This will likely return nil since we're not at the right call depth
	files, err := DiscoverTemplateFiles("", nil)

	// Should not error, just return empty or nil
	if err != nil {
		t.Fatalf("Expected no error for empty baseDir, got: %v", err)
	}

	// Files can be nil or empty
	_ = files
}

// TestNormalizeStoreName verifies store name normalization
func TestNormalizeStoreName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MyStore", "mystore"},
		{"UPPERCASE", "uppercase"},
		{"lowercase", "lowercase"},
		{"MixedCase", "mixedcase"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeStoreName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeStoreName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeStoreName_CaseInsensitiveMatching verifies case-insensitive behavior
func TestNormalizeStoreName_CaseInsensitiveMatching(t *testing.T) {
	names := []string{"MyStore", "MYSTORE", "mystore", "MyStOrE"}

	normalized := make(map[string]bool)
	for _, name := range names {
		normalized[NormalizeStoreName(name)] = true
	}

	// All should normalize to the same value
	if len(normalized) != 1 {
		t.Errorf("Expected all names to normalize to same value, got %d different values", len(normalized))
	}

	// Verify the normalized value
	expected := "mystore"
	if _, exists := normalized[expected]; !exists {
		t.Errorf("Expected normalized value %q, got %v", expected, normalized)
	}
}
