package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateComponent_ValidComponent(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid component.yaml
	componentYAML := `name: test-component
version: 1.0.0
description: A test component
category: layout
author: Test Author
license: MIT
tags:
  - test
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(componentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create valid template
	template := `[[ define "test" ]]
<div>Test Component</div>
[[ end ]]
`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	// Create README
	readme := "# Test Component\n\nThis is a test component."
	if err := os.WriteFile(filepath.Join(componentDir, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}

	// Validate
	result := ValidateComponent(componentDir)

	if !result.Valid {
		t.Errorf("Expected valid component, got invalid: %s", result.Format())
	}

	// Should only have info messages, no errors or warnings
	if result.ErrorCount() > 0 {
		t.Errorf("Expected no errors, got %d", result.ErrorCount())
	}
}

func TestValidateComponent_MissingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	result := ValidateComponent(componentDir)

	if result.Valid {
		t.Error("Expected invalid component with missing manifest")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for missing manifest")
	}
}

func TestValidateComponent_InvalidTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid manifest
	componentYAML := `name: test-component
version: 1.0.0
description: A test component
category: layout
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(componentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create template with syntax error
	template := `[[ define "test" ]]
<div>Test Component
[[ end
`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateComponent(componentDir)

	if result.Valid {
		t.Error("Expected invalid component with template syntax error")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for template syntax")
	}
}

func TestValidateComponent_MissingTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create manifest referencing non-existent template
	componentYAML := `name: test-component
version: 1.0.0
description: A test component
category: layout
templates:
  - missing.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(componentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateComponent(componentDir)

	if result.Valid {
		t.Error("Expected invalid component with missing template file")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for missing template file")
	}
}

func TestValidateComponent_MissingREADME(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid manifest
	componentYAML := `name: test-component
version: 1.0.0
description: A test component
category: layout
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(componentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create valid template
	template := `[[ define "test" ]]
<div>Test Component</div>
[[ end ]]
`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateComponent(componentDir)

	// Should pass validation but with a warning
	if !result.Valid {
		t.Error("Expected valid component")
	}

	if result.WarningCount() == 0 {
		t.Error("Expected warning for missing README")
	}
}

func TestValidateComponent_InvalidCategory(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create manifest with invalid category
	componentYAML := `name: test-component
version: 1.0.0
description: A test component
category: invalid-category
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(componentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateComponent(componentDir)

	if result.Valid {
		t.Error("Expected invalid component with invalid category")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for invalid category")
	}
}
