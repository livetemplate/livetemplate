package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", config.Version)
	}

	if config.KitPaths == nil {
		t.Error("Expected kit paths to be initialized")
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

func TestValidate_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()

	// Add valid path
	err := config.AddKitPath(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Validate
	err = config.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidKitPath(t *testing.T) {
	config := &Config{
		KitPaths: []string{"/non/existent/path"},
		Version:  "1.0",
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for non-existent kit path")
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "config.yaml")

	// Create a dedicated manager for this test
	mgr := NewManager()
	mgr.SetCustomPath(testConfigPath)

	// Load config - should return default config since file doesn't exist
	config, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config == nil {
		t.Fatal("Expected default config, got nil")
	}

	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}

	if len(config.KitPaths) != 0 {
		t.Errorf("Expected empty kit paths, got %d paths", len(config.KitPaths))
	}
}

func TestSaveConfig(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "config.yaml")

	// Create a dedicated manager for this test
	mgr := NewManager()
	mgr.SetCustomPath(testConfigPath)

	// Create config with test data
	config := &Config{
		Version:  "1.0",
		KitPaths: []string{"/test/path1", "/test/path2"},
	}

	// Save config
	err := mgr.SaveConfig(config)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testConfigPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load config back and verify
	loadedConfig, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loadedConfig.Version != config.Version {
		t.Errorf("Version mismatch: expected %s, got %s", config.Version, loadedConfig.Version)
	}

	if len(loadedConfig.KitPaths) != len(config.KitPaths) {
		t.Errorf("KitPaths length mismatch: expected %d, got %d", len(config.KitPaths), len(loadedConfig.KitPaths))
	}

	for i, path := range config.KitPaths {
		if i >= len(loadedConfig.KitPaths) || loadedConfig.KitPaths[i] != path {
			t.Errorf("KitPath[%d] mismatch: expected %s, got %s", i, path, loadedConfig.KitPaths[i])
		}
	}
}

func TestConfigPaths(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "kits")

	// Create directory
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := DefaultConfig()

	// Add path
	err := config.AddKitPath(kitDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify path exists
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

	// Add kit paths
	for _, dir := range []string{dir1, dir2, dir3} {
		err := config.AddKitPath(dir)
		if err != nil {
			t.Fatalf("Failed to add kit path %s: %v", dir, err)
		}
	}

	if len(config.KitPaths) != 3 {
		t.Errorf("Expected 3 kit paths, got %d", len(config.KitPaths))
	}

	// Remove middle path
	err := config.RemoveKitPath(dir2)
	if err != nil {
		t.Fatalf("Failed to remove kit path: %v", err)
	}

	if len(config.KitPaths) != 2 {
		t.Errorf("Expected 2 kit paths after removal, got %d", len(config.KitPaths))
	}

	// Verify correct paths remain
	hasDir1 := false
	hasDir3 := false
	for _, p := range config.KitPaths {
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

	// Validate should pass with no paths
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected empty config to validate: %v", err)
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

func TestLoadConfigFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test loading non-existent config (should return default)
	cfg, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load non-existent config: %v", err)
	}

	if cfg.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", cfg.Version)
	}

	// Create a config file
	configContent := `kit_paths:
  - /tmp/test
version: "1.0"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test loading existing config
	cfg, err = LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.KitPaths) != 1 {
		t.Errorf("Expected 1 kit path, got %d", len(cfg.KitPaths))
	}
}

func TestSetConfigPath(t *testing.T) {
	// Test setting custom config path
	customPath := "/tmp/custom-config.yaml"
	SetConfigPath(customPath)

	// The global variable should be set
	// We can't directly test this without exposing it, but we can test
	// that LoadConfig uses it indirectly
	defer SetConfigPath("") // Reset after test
}

func TestManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create multiple managers with different config paths
	managers := make([]*Manager, 10)
	for i := 0; i < 10; i++ {
		mgr := NewManager()
		configPath := filepath.Join(tmpDir, "config-"+string(rune('0'+i))+".yaml")
		mgr.SetCustomPath(configPath)
		managers[i] = mgr
	}

	// Run concurrent operations on different managers
	done := make(chan bool)
	for i, mgr := range managers {
		go func(idx int, m *Manager) {
			defer func() { done <- true }()

			// Create test config
			cfg := &Config{
				Version:  "1.0",
				KitPaths: []string{tmpDir},
			}

			// Save config
			if err := m.SaveConfig(cfg); err != nil {
				t.Errorf("Manager %d: SaveConfig failed: %v", idx, err)
				return
			}

			// Load config back
			loadedCfg, err := m.LoadConfig()
			if err != nil {
				t.Errorf("Manager %d: LoadConfig failed: %v", idx, err)
				return
			}

			// Verify
			if loadedCfg.Version != cfg.Version {
				t.Errorf("Manager %d: Version mismatch", idx)
			}
		}(i, mgr)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(managers); i++ {
		<-done
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Create invalid YAML file
	invalidYAML := `
kit_paths:
  - /test/path
version: 1.0
  invalid: indentation
`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to load invalid config
	cfg, err := LoadConfigFromPath(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
	if cfg != nil {
		t.Error("Expected nil config for invalid YAML")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	// Test package-level wrapper functions
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	// Set custom path
	SetConfigPath(configPath)
	defer SetConfigPath("") // Reset

	// Test GetConfigPath
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath failed: %v", err)
	}
	if path != configPath {
		t.Errorf("Expected path %s, got %s", configPath, path)
	}

	// Test GetConfigDir
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir failed: %v", err)
	}
	if dir != tmpDir {
		t.Errorf("Expected dir %s, got %s", tmpDir, dir)
	}

	// Test EnsureConfigDir
	if err := EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir failed: %v", err)
	}

	// Test LoadConfig and SaveConfig wrappers
	testConfig := &Config{
		Version:  "1.0",
		KitPaths: []string{"/test/path"},
	}

	if err := SaveConfig(testConfig); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loadedConfig.Version != testConfig.Version {
		t.Errorf("Version mismatch: expected %s, got %s", testConfig.Version, loadedConfig.Version)
	}
}

func TestManager_GetConfigPath_WithoutCustomPath(t *testing.T) {
	mgr := NewManager()

	path, err := mgr.GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath failed: %v", err)
	}

	// Should contain default config file name
	if !strings.Contains(path, ConfigFileName) {
		t.Errorf("Expected path to contain %s, got %s", ConfigFileName, path)
	}

	// Should contain default config dir
	if !strings.Contains(path, DefaultConfigDir) {
		t.Errorf("Expected path to contain %s, got %s", DefaultConfigDir, path)
	}
}

func TestLoadConfigFromPath_WithMissingVersionField(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "no-version.yaml")

	// Create YAML without version field
	yamlContent := `kit_paths:
  - /test/path
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFromPath failed: %v", err)
	}

	// Should default to "1.0"
	if cfg.Version != "1.0" {
		t.Errorf("Expected default version '1.0', got '%s'", cfg.Version)
	}
}

// Project config tests

func TestDefaultProjectConfig(t *testing.T) {
	cfg := DefaultProjectConfig()

	if cfg == nil {
		t.Fatal("Expected non-nil project config")
	}

	if cfg.Kit != "multi" {
		t.Errorf("Expected kit 'multi', got '%s'", cfg.Kit)
	}

	if cfg.DevMode != false {
		t.Error("Expected DevMode false")
	}
}

func TestLoadProjectConfig_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Load config from directory without .lvtrc
	cfg, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected default config, got nil")
	}

	// Should return default values
	if cfg.Kit != "multi" {
		t.Errorf("Expected default kit 'multi', got '%s'", cfg.Kit)
	}
}

func TestLoadProjectConfig_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ProjectConfigFileName)

	// Create .lvtrc file with correct key names
	content := `# Project configuration
kit=simple
dev_mode=true
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if cfg.Kit != "simple" {
		t.Errorf("Expected kit 'simple', got '%s'", cfg.Kit)
	}

	if !cfg.DevMode {
		t.Error("Expected DevMode true")
	}
}

func TestSaveProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &ProjectConfig{
		Kit:     "multi",
		DevMode: true,
	}

	// Save config
	if err := SaveProjectConfig(tmpDir, cfg); err != nil {
		t.Fatalf("SaveProjectConfig failed: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, ProjectConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back
	loadedCfg, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if loadedCfg.Kit != cfg.Kit {
		t.Errorf("Kit mismatch: expected %s, got %s", cfg.Kit, loadedCfg.Kit)
	}

	if loadedCfg.DevMode != cfg.DevMode {
		t.Errorf("DevMode mismatch: expected %v, got %v", cfg.DevMode, loadedCfg.DevMode)
	}
}

func TestProjectConfig_GetKit(t *testing.T) {
	cfg := &ProjectConfig{
		Kit: "pico",
	}

	if cfg.GetKit() != "pico" {
		t.Errorf("Expected 'pico', got '%s'", cfg.GetKit())
	}
}

// Removed TestProjectConfig_GetCSSFramework as CSS framework is now part of kit manifest

func TestProjectConfig_Validate(t *testing.T) {
	// Valid config with multi kit
	cfg := &ProjectConfig{
		Kit:     "multi",
		DevMode: false,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}

	// Valid config with simple kit
	cfg2 := &ProjectConfig{
		Kit: "simple",
	}

	if err := cfg2.Validate(); err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}

	// Invalid: invalid kit name
	cfg3 := &ProjectConfig{
		Kit: "invalid-kit",
	}

	if err := cfg3.Validate(); err == nil {
		t.Error("Expected error for invalid kit")
	}
}
