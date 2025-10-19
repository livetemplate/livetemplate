package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestKits_List tests listing all available kits
func TestKits_List(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app first (kits commands need app context)
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// List kits
	t.Log("Listing all kits...")
	cmd := exec.Command(lvtBinary, "kits", "list")
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to list kits: %v\nOutput: %s", err, stdout.String())
	}

	output := stdout.String()
	t.Logf("Kits list output:\n%s", output)

	// Verify system kits appear in output
	systemKits := []string{"multi", "single", "simple"}
	for _, kit := range systemKits {
		if !strings.Contains(output, kit) {
			t.Errorf("System kit %s not found in list", kit)
		}
	}

	t.Log("✅ Kits list test passed")
}

// TestKits_ListFiltered tests listing kits with filter
func TestKits_ListFiltered(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app first
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// List local kits (should be empty or contain user kits)
	t.Log("Listing local kits...")
	cmd := exec.Command(lvtBinary, "kits", "list", "--filter", "local")
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to list filtered kits: %v\nOutput: %s", err, stdout.String())
	}

	output := stdout.String()
	t.Logf("Filtered kits output:\n%s", output)

	t.Log("✅ Filtered kits list test passed")
}

// TestKits_ListJSON tests listing kits in JSON format
func TestKits_ListJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app first
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// List kits as JSON
	t.Log("Listing kits as JSON...")
	cmd := exec.Command(lvtBinary, "kits", "list", "--format", "json")
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to list kits as JSON: %v\nOutput: %s", err, stdout.String())
	}

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it's valid JSON
	var kits []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &kits); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if len(kits) == 0 {
		t.Error("Expected at least one kit in JSON output")
	}

	t.Logf("✅ Found %d kits in JSON output", len(kits))
	t.Log("✅ JSON kits list test passed")
}

// TestKits_Info tests showing kit information
func TestKits_Info(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app first
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// Get info for multi kit
	t.Log("Getting multi kit info...")
	cmd := exec.Command(lvtBinary, "kits", "info", "multi")
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to get kit info: %v\nOutput: %s", err, stdout.String())
	}

	output := stdout.String()
	t.Logf("Kit info output:\n%s", output)

	// Verify expected information appears
	expectedInfo := []string{"multi", "kit"}
	for _, info := range expectedInfo {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(info)) {
			t.Errorf("Expected info %q not found in output", info)
		}
	}

	t.Log("✅ Kit info test passed")
}

// TestKits_Create tests creating a new kit
func TestKits_Create(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app first
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// Create a new kit
	kitName := "testkit"
	t.Logf("Creating new kit: %s...", kitName)
	cmd := exec.Command(lvtBinary, "kits", "create", kitName)
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create kit: %v\nOutput: %s", err, stdout.String())
	}

	t.Logf("Create kit output:\n%s", stdout.String())

	// Verify kit was created in .lvt/kits/ directory
	kitPath := filepath.Join(appDir, ".lvt/kits", kitName)
	if _, err := os.Stat(kitPath); os.IsNotExist(err) {
		t.Error("Kit directory not created in .lvt/kits/")
	}

	// Verify kit.yaml exists
	kitYaml := filepath.Join(kitPath, "kit.yaml")
	if _, err := os.Stat(kitYaml); os.IsNotExist(err) {
		t.Error("kit.yaml not created")
	}

	// Verify helpers.go exists
	helpersGo := filepath.Join(kitPath, "helpers.go")
	if _, err := os.Stat(helpersGo); os.IsNotExist(err) {
		t.Error("helpers.go not created")
	}

	t.Log("✅ Kit creation test passed")
}

// TestKits_Validate tests validating a kit
func TestKits_Validate(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app first
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// Create a kit first
	kitName := "validatekit"
	t.Log("Creating kit for validation...")
	createCmd := exec.Command(lvtBinary, "kits", "create", kitName)
	createCmd.Dir = appDir
	if err := createCmd.Run(); err != nil {
		t.Fatalf("Failed to create kit: %v", err)
	}

	// Validate the kit
	kitPath := filepath.Join(appDir, ".lvt/kits", kitName)
	t.Log("Validating kit...")
	cmd := exec.Command(lvtBinary, "kits", "validate", kitPath)
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Kit validation failed: %v\nOutput: %s", err, stdout.String())
	}

	output := stdout.String()
	t.Logf("Validation output:\n%s", output)

	// Check for success indicators
	if !strings.Contains(strings.ToLower(output), "valid") && !strings.Contains(output, "✓") {
		t.Error("Validation output doesn't indicate success")
	}

	t.Log("✅ Kit validation test passed")
}

// TestKits_CustomizeProject tests customizing kit at project level
func TestKits_CustomizeProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// Customize kit at project level
	t.Log("Customizing kit at project level...")
	cmd := exec.Command(lvtBinary, "kits", "customize", "multi", "--scope", "project")
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to customize kit: %v\nOutput: %s", err, stdout.String())
	}

	t.Logf("Customize output:\n%s", stdout.String())

	// Verify .lvt/kits/multi directory was created
	customKitPath := filepath.Join(appDir, ".lvt/kits/multi")
	if _, err := os.Stat(customKitPath); os.IsNotExist(err) {
		t.Error("Custom kit directory not created at project level")
	}

	// Verify kit.yaml exists in custom location
	kitYaml := filepath.Join(customKitPath, "kit.yaml")
	if _, err := os.Stat(kitYaml); os.IsNotExist(err) {
		t.Error("kit.yaml not copied to custom location")
	}

	t.Log("✅ Project-level kit customization test passed")
}

// TestKits_CustomizeGlobal tests customizing kit at global level
func TestKits_CustomizeGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Set XDG_CONFIG_HOME to temp directory for test isolation
	configDir := filepath.Join(tmpDir, ".config")
	os.Setenv("XDG_CONFIG_HOME", configDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	// Create app (needed for context)
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// Customize kit at global level
	t.Log("Customizing kit at global level...")
	cmd := exec.Command(lvtBinary, "kits", "customize", "simple", "--scope", "global")
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configDir)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to customize kit globally: %v\nOutput: %s", err, stdout.String())
	}

	t.Logf("Global customize output:\n%s", stdout.String())

	// Verify global kit directory was created
	globalKitPath := filepath.Join(configDir, "lvt/kits/simple")
	if _, err := os.Stat(globalKitPath); os.IsNotExist(err) {
		t.Error("Custom kit directory not created at global level")
	}

	t.Log("✅ Global kit customization test passed")
}

// TestKits_CustomizeComponentsOnly tests customizing only components
func TestKits_CustomizeComponentsOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := buildLvtBinary(t, tmpDir)

	// Create app
	appDir := createTestApp(t, lvtBinary, tmpDir, "testapp", nil)

	// Customize only components
	t.Log("Customizing components only...")
	cmd := exec.Command(lvtBinary, "kits", "customize", "multi", "--components-only")
	cmd.Dir = appDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to customize components: %v\nOutput: %s", err, stdout.String())
	}

	t.Logf("Components customize output:\n%s", stdout.String())

	// Verify components directory exists
	componentsPath := filepath.Join(appDir, ".lvt/kits/multi/components")
	if _, err := os.Stat(componentsPath); os.IsNotExist(err) {
		t.Error("Components directory not created")
	}

	// Verify some component files exist
	componentFiles, err := os.ReadDir(componentsPath)
	if err != nil {
		t.Fatalf("Failed to read components directory: %v", err)
	}

	if len(componentFiles) == 0 {
		t.Error("No component files were copied")
	} else {
		t.Logf("✅ Copied %d component files", len(componentFiles))
	}

	t.Log("✅ Components-only customization test passed")
}
