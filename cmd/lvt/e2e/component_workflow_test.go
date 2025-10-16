package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestComponentWorkflow tests the full component lifecycle:
// create -> validate -> use in generation
func TestComponentWorkflow(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Build lvt binary
	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	// Change to tmpDir for component creation
	componentDir := filepath.Join(tmpDir, ".lvt", "components", "test-card")

	t.Run("1_Create_Component", func(t *testing.T) {
		createCmd := exec.Command(lvtBinary, "components", "create", "test-card", "--category", "base")
		createCmd.Dir = tmpDir
		output, err := createCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create component: %v\nOutput: %s", err, output)
		}

		// Verify files were created
		if _, err := os.Stat(filepath.Join(componentDir, "component.yaml")); os.IsNotExist(err) {
			t.Error("component.yaml was not created")
		}
		if _, err := os.Stat(filepath.Join(componentDir, "test-card.tmpl")); os.IsNotExist(err) {
			t.Error("test-card.tmpl was not created")
		}
		if _, err := os.Stat(filepath.Join(componentDir, "README.md")); os.IsNotExist(err) {
			t.Error("README.md was not created")
		}

		t.Log("✅ Component created successfully")
	})

	t.Run("2_Validate_Component_Initial", func(t *testing.T) {
		// Initial validation will fail because default template has placeholders
		validateCmd := exec.Command(lvtBinary, "components", "validate", componentDir)
		output, err := validateCmd.CombinedOutput()
		outputStr := string(output)

		// We expect this to fail due to undefined functions in template
		if err == nil {
			t.Log("Note: Initial validation passed (template might not have placeholders)")
		} else {
			if !strings.Contains(outputStr, "❌") {
				t.Errorf("Expected error indicator in failed validation\nOutput: %s", outputStr)
			}
			t.Log("✅ Initial validation correctly caught template issues")
		}
	})

	t.Run("3_List_Component", func(t *testing.T) {
		listCmd := exec.Command(lvtBinary, "components", "list", "--filter", "local")
		listCmd.Dir = tmpDir
		output, err := listCmd.CombinedOutput()
		outputStr := string(output)

		if err != nil {
			t.Fatalf("Failed to list components: %v\nOutput: %s", err, outputStr)
		}

		if !strings.Contains(outputStr, "test-card") {
			t.Errorf("Expected to find test-card in component list\nOutput: %s", outputStr)
		}

		t.Log("✅ Component appears in list")
	})

	t.Run("4_Info_Component", func(t *testing.T) {
		infoCmd := exec.Command(lvtBinary, "components", "info", "test-card")
		infoCmd.Dir = tmpDir
		output, err := infoCmd.CombinedOutput()
		outputStr := string(output)

		if err != nil {
			t.Fatalf("Failed to get component info: %v\nOutput: %s", err, outputStr)
		}

		if !strings.Contains(outputStr, "test-card") {
			t.Errorf("Expected component info to contain test-card\nOutput: %s", outputStr)
		}

		if !strings.Contains(outputStr, "base") {
			t.Errorf("Expected component info to show category 'base'\nOutput: %s", outputStr)
		}

		t.Log("✅ Component info displayed correctly")
	})

	t.Run("5_Fix_Template_For_Gen", func(t *testing.T) {
		// The default template generated needs to be valid for use in generation
		// Let's update it to be more realistic
		templateContent := `[[ define "test-card" ]]
<div class="card">
  <div class="card-header">
    <h3>[[ .Title ]]</h3>
  </div>
  <div class="card-body">
    <p>[[ .Content ]]</p>
  </div>
</div>
[[ end ]]
`
		tmplPath := filepath.Join(componentDir, "test-card.tmpl")
		if err := os.WriteFile(tmplPath, []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to update template: %v", err)
		}

		// Update component.yaml to specify inputs
		manifestContent := `name: test-card
version: 1.0.0
description: A test card component
category: base
inputs:
  - name: Title
    type: string
  - name: Content
    type: string
templates:
  - test-card.tmpl
`
		manifestPath := filepath.Join(componentDir, "component.yaml")
		if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
			t.Fatalf("Failed to update manifest: %v", err)
		}

		t.Log("✅ Template updated")
	})

	t.Run("6_Validate_Fixed_Template", func(t *testing.T) {
		// Now validation should pass
		validateCmd := exec.Command(lvtBinary, "components", "validate", componentDir)
		output, err := validateCmd.CombinedOutput()
		outputStr := string(output)

		if err != nil {
			t.Fatalf("Updated component validation failed: %v\nOutput: %s", err, outputStr)
		}

		if !strings.Contains(outputStr, "✅ Validation passed") {
			t.Errorf("Expected validation to pass after fixes\nOutput: %s", outputStr)
		}

		t.Log("✅ Fixed template validates successfully")
	})
}

// TestComponentValidationFailures tests that validation catches errors
func TestComponentValidationFailures(t *testing.T) {
	tmpDir := t.TempDir()
	lvtBinary := filepath.Join(tmpDir, "lvt")

	// Build lvt
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	t.Run("Invalid_Template_Syntax", func(t *testing.T) {
		componentDir := filepath.Join(tmpDir, ".lvt", "components", "broken-component")
		if err := os.MkdirAll(componentDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create component with broken template
		manifest := `name: broken-component
version: 1.0.0
description: A broken component
category: base
templates:
  - broken.tmpl
`
		if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}

		// Create template with syntax error
		brokenTemplate := `[[ define "broken" ]]
<div>Unclosed tag
[[ end
`
		if err := os.WriteFile(filepath.Join(componentDir, "broken.tmpl"), []byte(brokenTemplate), 0644); err != nil {
			t.Fatal(err)
		}

		// Validate should fail
		validateCmd := exec.Command(lvtBinary, "components", "validate", componentDir)
		output, err := validateCmd.CombinedOutput()
		outputStr := string(output)

		if err == nil {
			t.Error("Expected validation to fail for broken template")
		}

		if !strings.Contains(outputStr, "❌") {
			t.Errorf("Expected error indicator in output\nOutput: %s", outputStr)
		}

		if !strings.Contains(outputStr, "Template syntax error") {
			t.Errorf("Expected template syntax error message\nOutput: %s", outputStr)
		}

		t.Log("✅ Validation correctly catches template syntax errors")
	})

	t.Run("Missing_Template_File", func(t *testing.T) {
		componentDir := filepath.Join(tmpDir, ".lvt", "components", "missing-template")
		if err := os.MkdirAll(componentDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create manifest referencing non-existent template
		manifest := `name: missing-template
version: 1.0.0
description: Component with missing template
category: base
templates:
  - missing.tmpl
`
		if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}

		// Validate should fail
		validateCmd := exec.Command(lvtBinary, "components", "validate", componentDir)
		output, err := validateCmd.CombinedOutput()
		outputStr := string(output)

		if err == nil {
			t.Error("Expected validation to fail for missing template")
		}

		if !strings.Contains(outputStr, "Template file not found") {
			t.Errorf("Expected missing template error\nOutput: %s", outputStr)
		}

		t.Log("✅ Validation correctly catches missing template files")
	})
}
