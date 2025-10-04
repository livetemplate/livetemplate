package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

// TemplateLoader provides cascading template lookup:
// 1. Project templates (.lvt/templates/)
// 2. User templates (~/.config/lvt/templates/)
// 3. Embedded defaults (templatesFS)
type TemplateLoader struct {
	projectTemplateDir string
	userTemplateDir    string
}

// NewTemplateLoader creates a new template loader with auto-detected paths
func NewTemplateLoader() *TemplateLoader {
	return &TemplateLoader{
		projectTemplateDir: findProjectTemplateDir(),
		userTemplateDir:    findUserTemplateDir(),
	}
}

// Load attempts to load a template file using cascading lookup
// name should be relative path like "resource/handler.go.tmpl"
func (l *TemplateLoader) Load(name string) ([]byte, error) {
	// Try project templates first
	if l.projectTemplateDir != "" {
		path := filepath.Join(l.projectTemplateDir, name)
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}

	// Try user templates second
	if l.userTemplateDir != "" {
		path := filepath.Join(l.userTemplateDir, name)
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}

	// Fallback to embedded templates
	templatePath := filepath.Join("templates", name)
	data, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return data, nil
}

// HasCustomTemplate checks if a custom template exists for the given name
func (l *TemplateLoader) HasCustomTemplate(name string) bool {
	if l.projectTemplateDir != "" {
		path := filepath.Join(l.projectTemplateDir, name)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	if l.userTemplateDir != "" {
		path := filepath.Join(l.userTemplateDir, name)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// GetProjectTemplateDir returns the project template directory path
func (l *TemplateLoader) GetProjectTemplateDir() string {
	return l.projectTemplateDir
}

// GetUserTemplateDir returns the user template directory path
func (l *TemplateLoader) GetUserTemplateDir() string {
	return l.userTemplateDir
}

// findProjectTemplateDir walks up the directory tree to find .lvt/templates/
func findProjectTemplateDir() string {
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		checkPath := filepath.Join(currentDir, ".lvt", "templates")
		if info, err := os.Stat(checkPath); err == nil && info.IsDir() {
			return checkPath
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached root
			break
		}
		currentDir = parent
	}

	return ""
}

// findUserTemplateDir returns ~/.config/lvt/templates/ if it exists
func findUserTemplateDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	templateDir := filepath.Join(homeDir, ".config", "lvt", "templates")
	if info, err := os.Stat(templateDir); err == nil && info.IsDir() {
		return templateDir
	}

	return ""
}

// CopyEmbeddedTemplate copies an embedded template to the specified destination
func CopyEmbeddedTemplate(templateName, destPath string) error {
	// Read from embedded FS
	templatePath := filepath.Join("templates", templateName)
	data, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read embedded template %s: %w", templateName, err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write template to %s: %w", destPath, err)
	}

	return nil
}
