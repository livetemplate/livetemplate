package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.DefaultKit != "tailwind" {
		t.Errorf("Expected default kit 'tailwind', got '%s'", config.DefaultKit)
	}

	if config.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", config.Version)
	}

	if config.ComponentPaths == nil {
		t.Error("Expected component paths to be initialized")
	}

	if config.KitPaths == nil {
		t.Error("Expected kit paths to be initialized")
	}
}

func TestAddComponentPath(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add valid path
	err := config.AddComponentPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to add component path: %v", err)
	}

	if len(config.ComponentPaths) != 1 {
		t.Errorf("Expected 1 component path, got %d", len(config.ComponentPaths))
	}

	// Check it's absolute
	if !filepath.IsAbs(config.ComponentPaths[0]) {
		t.Error("Expected absolute path")
	}
}

func TestAddComponentPath_NonExistent(t *testing.T) {
	config := DefaultConfig()

	// Try to add non-existent path
	err := config.AddComponentPath("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestAddComponentPath_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add path first time
	err := config.AddComponentPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to add component path: %v", err)
	}

	// Try to add same path again
	err = config.AddComponentPath(tmpDir)
	if err == nil {
		t.Error("Expected error for duplicate path")
	}

	if len(config.ComponentPaths) != 1 {
		t.Errorf("Expected 1 component path after duplicate, got %d", len(config.ComponentPaths))
	}
}

func TestRemoveComponentPath(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add path
	err := config.AddComponentPath(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove path
	err = config.RemoveComponentPath(tmpDir)
	if err != nil {
		t.Errorf("Failed to remove component path: %v", err)
	}

	if len(config.ComponentPaths) != 0 {
		t.Errorf("Expected 0 component paths after removal, got %d", len(config.ComponentPaths))
	}
}

func TestRemoveComponentPath_NotFound(t *testing.T) {
	config := DefaultConfig()

	// Try to remove path that doesn't exist
	err := config.RemoveComponentPath("/some/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestAddKitPath(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add valid path
	err := config.AddKitPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to add kit path: %v", err)
	}

	if len(config.KitPaths) != 1 {
		t.Errorf("Expected 1 kit path, got %d", len(config.KitPaths))
	}

	// Check it's absolute
	if !filepath.IsAbs(config.KitPaths[0]) {
		t.Error("Expected absolute path")
	}
}

func TestAddKitPath_NonExistent(t *testing.T) {
	config := DefaultConfig()

	// Try to add non-existent path
	err := config.AddKitPath("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestAddKitPath_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add path first time
	err := config.AddKitPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to add kit path: %v", err)
	}

	// Try to add same path again
	err = config.AddKitPath(tmpDir)
	if err == nil {
		t.Error("Expected error for duplicate path")
	}

	if len(config.KitPaths) != 1 {
		t.Errorf("Expected 1 kit path after duplicate, got %d", len(config.KitPaths))
	}
}

func TestRemoveKitPath(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add path
	err := config.AddKitPath(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove path
	err = config.RemoveKitPath(tmpDir)
	if err != nil {
		t.Errorf("Failed to remove kit path: %v", err)
	}

	if len(config.KitPaths) != 0 {
		t.Errorf("Expected 0 kit paths after removal, got %d", len(config.KitPaths))
	}
}

func TestRemoveKitPath_NotFound(t *testing.T) {
	config := DefaultConfig()

	// Try to remove path that doesn't exist
	err := config.RemoveKitPath("/some/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestSetDefaultKit(t *testing.T) {
	config := DefaultConfig()

	config.SetDefaultKit("bulma")

	if config.DefaultKit != "bulma" {
		t.Errorf("Expected default kit 'bulma', got '%s'", config.DefaultKit)
	}
}

func TestGetDefaultKit(t *testing.T) {
	tests := []struct {
		name        string
		defaultKit  string
		expectedKit string
	}{
		{"returns set kit", "bulma", "bulma"},
		{"returns default when empty", "", "tailwind"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultKit: tt.defaultKit,
			}

			result := config.GetDefaultKit()
			if result != tt.expectedKit {
				t.Errorf("Expected '%s', got '%s'", tt.expectedKit, result)
			}
		})
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add valid paths
	err := config.AddComponentPath(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	err = config.AddKitPath(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Validate
	err = config.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidComponentPath(t *testing.T) {
	config := &Config{
		ComponentPaths: []string{"/non/existent/path"},
		KitPaths:       []string{},
		DefaultKit:     "tailwind",
		Version:        "1.0",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for non-existent component path")
	}
}

func TestValidate_InvalidKitPath(t *testing.T) {
	config := &Config{
		ComponentPaths: []string{},
		KitPaths:       []string{"/non/existent/path"},
		DefaultKit:     "tailwind",
		Version:        "1.0",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for non-existent kit path")
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	// This test requires modifying GetConfigPath to support a test mode
	// or using environment variables. For now, we test that it returns
	// default config when file doesn't exist (which it does in the real implementation)
	// Skip this test as it depends on user's home directory
	t.Skip("Skipping test that depends on user home directory")
}

func TestSaveConfig(t *testing.T) {
	// This test requires a temporary config directory
	// Skip for now as it depends on user's home directory
	t.Skip("Skipping test that depends on user home directory")
}

func TestConfigPaths(t *testing.T) {
	tmpDir := t.TempDir()
	compDir := filepath.Join(tmpDir, "components")
	kitDir := filepath.Join(tmpDir, "kits")

	// Create directories
	if err := os.MkdirAll(compDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := DefaultConfig()

	// Add multiple paths
	err := config.AddComponentPath(compDir)
	if err != nil {
		t.Fatal(err)
	}

	err = config.AddKitPath(kitDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify both paths exist
	if len(config.ComponentPaths) != 1 {
		t.Errorf("Expected 1 component path, got %d", len(config.ComponentPaths))
	}

	if len(config.KitPaths) != 1 {
		t.Errorf("Expected 1 kit path, got %d", len(config.KitPaths))
	}

	// Validate
	err = config.Validate()
	if err != nil {
		t.Errorf("Expected valid config: %v", err)
	}
}

func TestAddMultiplePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple directories
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	dir3 := filepath.Join(tmpDir, "dir3")

	for _, dir := range []string{dir1, dir2, dir3} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	config := DefaultConfig()

	// Add component paths
	for _, dir := range []string{dir1, dir2, dir3} {
		err := config.AddComponentPath(dir)
		if err != nil {
			t.Fatalf("Failed to add component path %s: %v", dir, err)
		}
	}

	if len(config.ComponentPaths) != 3 {
		t.Errorf("Expected 3 component paths, got %d", len(config.ComponentPaths))
	}

	// Remove middle path
	err := config.RemoveComponentPath(dir2)
	if err != nil {
		t.Fatalf("Failed to remove component path: %v", err)
	}

	if len(config.ComponentPaths) != 2 {
		t.Errorf("Expected 2 component paths after removal, got %d", len(config.ComponentPaths))
	}

	// Verify correct paths remain
	hasDir1 := false
	hasDir3 := false
	for _, p := range config.ComponentPaths {
		if p == dir1 {
			hasDir1 = true
		}
		if p == dir3 {
			hasDir3 = true
		}
	}

	if !hasDir1 {
		t.Error("Expected dir1 to remain")
	}
	if !hasDir3 {
		t.Error("Expected dir3 to remain")
	}
}

func TestConfigVersion(t *testing.T) {
	config := DefaultConfig()

	if config.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", config.Version)
	}
}

func TestConfigEmptyState(t *testing.T) {
	config := &Config{}

	// GetDefaultKit should return default even when empty
	kit := config.GetDefaultKit()
	if kit != "tailwind" {
		t.Errorf("Expected default kit 'tailwind', got '%s'", kit)
	}

	// Validate should pass with no paths
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected empty config to validate: %v", err)
	}
}

func TestRemoveComponentPath_OrderPreserved(t *testing.T) {
	tmpDir := t.TempDir()

	// Create three directories
	dirs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dirs[i] = filepath.Join(tmpDir, filepath.Join("dir"+string(rune('0'+i+1))))
		if err := os.MkdirAll(dirs[i], 0755); err != nil {
			t.Fatal(err)
		}
	}

	config := DefaultConfig()

	// Add in order
	for _, dir := range dirs {
		if err := config.AddComponentPath(dir); err != nil {
			t.Fatal(err)
		}
	}

	// Remove first path
	if err := config.RemoveComponentPath(dirs[0]); err != nil {
		t.Fatal(err)
	}

	// Verify order preserved
	if len(config.ComponentPaths) != 2 {
		t.Fatalf("Expected 2 paths, got %d", len(config.ComponentPaths))
	}

	if config.ComponentPaths[0] != dirs[1] {
		t.Errorf("Expected first path to be '%s', got '%s'", dirs[1], config.ComponentPaths[0])
	}

	if config.ComponentPaths[1] != dirs[2] {
		t.Errorf("Expected second path to be '%s', got '%s'", dirs[2], config.ComponentPaths[1])
	}
}

func TestRemoveKitPath_OrderPreserved(t *testing.T) {
	tmpDir := t.TempDir()

	// Create three directories
	dirs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dirs[i] = filepath.Join(tmpDir, "dir"+string(rune('0'+i+1)))
		if err := os.MkdirAll(dirs[i], 0755); err != nil {
			t.Fatal(err)
		}
	}

	config := DefaultConfig()

	// Add in order
	for _, dir := range dirs {
		if err := config.AddKitPath(dir); err != nil {
			t.Fatal(err)
		}
	}

	// Remove middle path
	if err := config.RemoveKitPath(dirs[1]); err != nil {
		t.Fatal(err)
	}

	// Verify order preserved
	if len(config.KitPaths) != 2 {
		t.Fatalf("Expected 2 paths, got %d", len(config.KitPaths))
	}

	if config.KitPaths[0] != dirs[0] {
		t.Errorf("Expected first path to be '%s', got '%s'", dirs[0], config.KitPaths[0])
	}

	if config.KitPaths[1] != dirs[2] {
		t.Errorf("Expected second path to be '%s', got '%s'", dirs[2], config.KitPaths[1])
	}
}
