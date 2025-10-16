package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/livefir/livetemplate/cmd/lvt/internal/components"
)

// ValidateComponent validates a component directory
func ValidateComponent(path string) *ValidationResult {
	result := NewValidationResult()

	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		result.AddError(fmt.Sprintf("Component directory not found: %s", path), path, 0)
		return result
	}

	if !info.IsDir() {
		result.AddError("Path is not a directory", path, 0)
		return result
	}

	// Validate structure
	structureResult := validateComponentStructure(path)
	result.Merge(structureResult)

	// Validate manifest
	manifestResult := validateComponentManifest(path)
	result.Merge(manifestResult)

	// If manifest is valid, validate templates
	if !manifestResult.HasErrors() {
		templateResult := validateComponentTemplates(path)
		result.Merge(templateResult)
	}

	// Validate README
	readmeResult := validateComponentReadme(path)
	result.Merge(readmeResult)

	return result
}

// validateComponentStructure checks if required files exist
func validateComponentStructure(path string) *ValidationResult {
	result := NewValidationResult()

	// Check for component.yaml
	manifestPath := filepath.Join(path, components.ManifestFileName)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		result.AddError("Missing component.yaml", path, 0)
	}

	// Check for at least one .tmpl file
	entries, err := os.ReadDir(path)
	if err != nil {
		result.AddError(fmt.Sprintf("Failed to read directory: %v", err), path, 0)
		return result
	}

	hasTmpl := false
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tmpl" {
			hasTmpl = true
			break
		}
	}

	if !hasTmpl {
		result.AddError("No template files (.tmpl) found", path, 0)
	}

	return result
}

// validateComponentManifest validates the component.yaml file
func validateComponentManifest(path string) *ValidationResult {
	result := NewValidationResult()

	manifestPath := filepath.Join(path, components.ManifestFileName)

	// Load manifest
	manifest, err := components.LoadManifest(path)
	if err != nil {
		result.AddError(fmt.Sprintf("Failed to load manifest: %v", err), manifestPath, 0)
		return result
	}

	// Validate manifest (uses built-in validation)
	if err := manifest.Validate(); err != nil {
		result.AddError(fmt.Sprintf("Manifest validation failed: %v", err), manifestPath, 0)
		return result
	}

	// Additional checks
	if len(manifest.Templates) == 0 {
		result.AddError("No templates specified in manifest", manifestPath, 0)
	}

	// Check that specified templates exist
	for _, tmplFile := range manifest.Templates {
		tmplPath := filepath.Join(path, tmplFile)
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			result.AddError(fmt.Sprintf("Template file not found: %s", tmplFile), manifestPath, 0)
		}
	}

	// Check for recommended fields
	if manifest.Author == "" {
		result.AddWarning("Author field is empty", manifestPath, 0)
	}

	if manifest.License == "" {
		result.AddWarning("License field is empty", manifestPath, 0)
	}

	if len(manifest.Tags) == 0 {
		result.AddInfo("No tags specified - consider adding tags for discoverability", manifestPath, 0)
	}

	return result
}

// validateComponentTemplates validates all template files
func validateComponentTemplates(path string) *ValidationResult {
	result := NewValidationResult()

	// Load manifest to get template list
	manifest, err := components.LoadManifest(path)
	if err != nil {
		// Already reported in manifest validation
		return result
	}

	// Validate each template
	for _, tmplFile := range manifest.Templates {
		tmplPath := filepath.Join(path, tmplFile)
		tmplResult := validateTemplateFile(tmplPath)
		result.Merge(tmplResult)
	}

	return result
}

// validateTemplateFile validates a single template file
func validateTemplateFile(path string) *ValidationResult {
	result := NewValidationResult()

	// Read template file
	data, err := os.ReadFile(path)
	if err != nil {
		result.AddError(fmt.Sprintf("Failed to read template: %v", err), path, 0)
		return result
	}

	// Parse template with [[ ]] delimiters
	tmpl, err := template.New(filepath.Base(path)).Delims("[[", "]]").Parse(string(data))
	if err != nil {
		result.AddError(fmt.Sprintf("Template syntax error: %v", err), path, 0)
		return result
	}

	// Check if template defines at least one block
	if tmpl.Tree == nil {
		result.AddWarning("Template appears to be empty", path, 0)
	}

	return result
}

// validateComponentReadme checks for README.md
func validateComponentReadme(path string) *ValidationResult {
	result := NewValidationResult()

	readmePath := filepath.Join(path, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		result.AddWarning("Missing README.md - documentation is recommended", path, 0)
		return result
	}

	// Check if README has content
	data, err := os.ReadFile(readmePath)
	if err != nil {
		result.AddWarning(fmt.Sprintf("Failed to read README.md: %v", err), readmePath, 0)
		return result
	}

	if len(data) < 50 {
		result.AddWarning("README.md appears to be very short", readmePath, 0)
	}

	return result
}
