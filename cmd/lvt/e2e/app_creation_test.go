package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAppCreation_DefaultsMultiTailwind tests creating an app with default settings
func TestAppCreation_DefaultsMultiTailwind(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app with defaults
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Verify .lvtrc has correct values
	kit := readLvtrc(t, appDir)
	if kit != "multi" {
		t.Errorf("Expected kit=multi, got kit=%s", kit)
	}

	// Verify expected files created
	expectedFiles := []string{
		"go.mod",
		"README.md",
		"cmd/testapp/main.go",
		"internal/database/db.go",
		"internal/database/schema.sql",
		"internal/database/queries.sql",
		"internal/database/sqlc.yaml",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", file)
		}
	}

	t.Log("✅ App creation with defaults test passed")
}

// TestAppCreation_CustomKitCSS tests creating an app with custom kit and CSS
func TestAppCreation_CustomKitCSS(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app with single kit
	opts := &AppOptions{
		Kit:     "single",
		DevMode: true,
	}
	appDir := createTestApp(t, tmpDir, "testapp", opts)

	// Verify .lvtrc has correct values
	kit := readLvtrc(t, appDir)
	if kit != "single" {
		t.Errorf("Expected kit=single, got kit=%s", kit)
	}

	t.Log("✅ App creation with custom kit test passed")
}

// TestAppCreation_SimpleKit tests creating an app with the simple kit
func TestAppCreation_SimpleKit(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app with simple kit
	opts := &AppOptions{
		Kit:     "simple",
		DevMode: true,
	}
	appDir := createTestApp(t, tmpDir, "testapp", opts)

	// Verify .lvtrc has correct values
	kit := readLvtrc(t, appDir)
	if kit != "simple" {
		t.Errorf("Expected kit=simple, got kit=%s", kit)
	}

	t.Log("✅ App creation with simple kit test passed")
}

// TestAppCreation_CustomModule tests creating an app with custom module name
func TestAppCreation_CustomModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app with custom module
	opts := &AppOptions{
		Module:  "github.com/testuser/customapp",
		DevMode: true,
	}
	appDir := createTestApp(t, tmpDir, "testapp", opts)

	// Verify go.mod has correct module name
	goModPath := filepath.Join(appDir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	goModContent := string(content)
	expectedModule := "module github.com/testuser/customapp"
	if !contains(goModContent, expectedModule) {
		t.Errorf("go.mod does not contain %q\nContent:\n%s", expectedModule, goModContent)
	}

	t.Log("✅ Custom module name test passed")
}

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
