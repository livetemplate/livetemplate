package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateKit_ValidKit(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid kit.yaml
	kitYAML := `name: test-kit
version: 1.0.0
description: A test CSS kit
framework: test
author: Test Author
license: MIT
cdn: https://example.com/test.css
tags:
  - test
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create valid helpers.go
	helpers := `package testkit

import "github.com/livefir/livetemplate/cmd/lvt/internal/kits"

type Helpers struct{}

func NewHelpers() kits.CSSHelpers {
	return &Helpers{}
}

func (h *Helpers) ContainerClass() string { return "container" }
func (h *Helpers) BoxClass() string { return "box" }
func (h *Helpers) TitleClass(level int) string { return "title" }
func (h *Helpers) ButtonClass(variant string) string { return "btn" }
func (h *Helpers) InputClass() string { return "input" }
func (h *Helpers) TableClass() string { return "table" }
func (h *Helpers) CSSCDN() string { return "https://example.com/test.css" }
`
	if err := os.WriteFile(filepath.Join(kitDir, "helpers.go"), []byte(helpers), 0644); err != nil {
		t.Fatal(err)
	}

	// Create README
	readme := "# Test Kit\n\nThis is a test CSS kit."
	if err := os.WriteFile(filepath.Join(kitDir, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	if !result.Valid {
		t.Errorf("Expected valid kit, got invalid: %s", result.Format())
	}
}

func TestValidateKit_MissingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	if result.Valid {
		t.Error("Expected invalid kit with missing manifest")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for missing manifest")
	}
}

func TestValidateKit_InvalidGoSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid manifest
	kitYAML := `name: test-kit
version: 1.0.0
description: A test CSS kit
framework: test
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create helpers.go with syntax error
	helpers := `package testkit

func (h *Helpers) ContainerClass() string { return "container"
`
	if err := os.WriteFile(filepath.Join(kitDir, "helpers.go"), []byte(helpers), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	if result.Valid {
		t.Error("Expected invalid kit with Go syntax error")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for Go syntax")
	}
}

func TestValidateKit_MissingHelpersStruct(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid manifest
	kitYAML := `name: test-kit
version: 1.0.0
description: A test CSS kit
framework: test
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create helpers.go without Helpers struct
	helpers := `package testkit

import "github.com/livefir/livetemplate/cmd/lvt/internal/kits"

func NewHelpers() kits.CSSHelpers {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(kitDir, "helpers.go"), []byte(helpers), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	if result.Valid {
		t.Error("Expected invalid kit without Helpers struct")
	}

	if result.ErrorCount() == 0 {
		t.Error("Expected at least one error for missing Helpers struct")
	}
}

func TestValidateKit_MissingRequiredMethods(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid manifest
	kitYAML := `name: test-kit
version: 1.0.0
description: A test CSS kit
framework: test
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create helpers.go with only partial methods
	helpers := `package testkit

import "github.com/livefir/livetemplate/cmd/lvt/internal/kits"

type Helpers struct{}

func NewHelpers() kits.CSSHelpers {
	return &Helpers{}
}

func (h *Helpers) ContainerClass() string { return "container" }
`
	if err := os.WriteFile(filepath.Join(kitDir, "helpers.go"), []byte(helpers), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	if !result.Valid {
		t.Error("Expected valid kit (warnings only for missing methods)")
	}

	// Should have warnings for missing methods
	if result.WarningCount() == 0 {
		t.Error("Expected warnings for missing methods")
	}
}

func TestValidateKit_MissingREADME(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid manifest
	kitYAML := `name: test-kit
version: 1.0.0
description: A test CSS kit
framework: test
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	// Should have warning for missing README
	if result.WarningCount() == 0 {
		t.Error("Expected warning for missing README")
	}
}

func TestValidateKit_CDNOnly(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create manifest for CDN-only kit (no helpers.go)
	kitYAML := `name: test-kit
version: 1.0.0
description: A CDN-only CSS kit
framework: test
cdn: https://example.com/test.css
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateKit(kitDir)

	// Should be valid with just a warning about missing helpers
	if !result.Valid {
		t.Errorf("Expected valid CDN-only kit: %s", result.Format())
	}

	if result.WarningCount() == 0 {
		t.Error("Expected warning for missing helpers.go")
	}
}
