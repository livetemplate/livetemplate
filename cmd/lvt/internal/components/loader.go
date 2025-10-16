package components

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// ComponentLoader handles loading components from various sources
type ComponentLoader struct {
	searchPaths []string              // Paths to search for components
	cache       map[string]*Component // Cached loaded components
	embedFS     *embed.FS             // Embedded filesystem for system components
	configPaths []string              // Paths from user config
	projectPath string                // Project-specific path (.lvt/components)
}

// NewLoader creates a new component loader with default paths
func NewLoader(embedFS *embed.FS) *ComponentLoader {
	loader := &ComponentLoader{
		cache:   make(map[string]*Component),
		embedFS: embedFS,
	}

	// Build search paths in priority order
	loader.buildSearchPaths()

	return loader
}

// buildSearchPaths constructs the search paths in priority order:
// 1. Project path (.lvt/components/)
// 2. Config paths (from ~/.config/lvt/config.yaml)
// 3. Embedded system components (fallback)
func (l *ComponentLoader) buildSearchPaths() {
	var paths []string

	// 1. Project path
	if projectPath := findProjectComponentDir(); projectPath != "" {
		l.projectPath = projectPath
		paths = append(paths, projectPath)
	}

	// 2. Config paths (to be loaded from config)
	// TODO: Load from ~/.config/lvt/config.yaml when config package is ready
	// For now, we'll just use these search paths

	l.searchPaths = paths
}

// Load loads a component by name from the first matching source
func (l *ComponentLoader) Load(name string) (*Component, error) {
	// Check cache first
	if cached, exists := l.cache[name]; exists {
		return cached, nil
	}

	// Try to load from search paths (local)
	for _, basePath := range l.searchPaths {
		componentPath := filepath.Join(basePath, name)
		if component, err := l.loadFromPath(componentPath, SourceLocal); err == nil {
			l.cache[name] = component
			return component, nil
		}
	}

	// Try to load from embedded system components
	if l.embedFS != nil {
		if component, err := l.loadFromEmbedded(name); err == nil {
			l.cache[name] = component
			return component, nil
		}
	}

	return nil, ErrComponentNotFound{Name: name}
}

// loadFromPath loads a component from a filesystem path
func (l *ComponentLoader) loadFromPath(path string, source ComponentSource) (*Component, error) {
	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("component directory not found: %s", path)
	}

	// Check if manifest exists
	if !ManifestExists(path) {
		return nil, fmt.Errorf("component.yaml not found in: %s", path)
	}

	// Load manifest
	manifest, err := LoadManifest(path)
	if err != nil {
		return nil, err
	}

	// Load templates
	templates, err := loadTemplates(path, manifest.Templates)
	if err != nil {
		return nil, err
	}

	component := &Component{
		Manifest:  *manifest,
		Source:    source,
		Path:      path,
		Templates: templates,
	}

	return component, nil
}

// loadFromEmbedded loads a component from the embedded filesystem
func (l *ComponentLoader) loadFromEmbedded(name string) (*Component, error) {
	if l.embedFS == nil {
		return nil, fmt.Errorf("embedded filesystem not available")
	}

	// Embedded components are in system/ directory
	componentPath := filepath.Join("system", name)

	// Read manifest from embedded FS
	manifestPath := filepath.Join(componentPath, ManifestFileName)
	data, err := l.embedFS.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("component not found in embedded FS: %s", name)
	}

	// Parse manifest
	var manifest ComponentManifest
	if err := unmarshalYAML(data, &manifest); err != nil {
		return nil, ErrManifestParse{
			Path: manifestPath,
			Err:  err,
		}
	}

	// Validate
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	// Load templates from embedded FS
	templates, err := loadTemplatesFromEmbed(l.embedFS, componentPath, manifest.Templates)
	if err != nil {
		return nil, err
	}

	component := &Component{
		Manifest:  manifest,
		Source:    SourceSystem,
		Path:      componentPath,
		Templates: templates,
	}

	return component, nil
}

// List returns all available components, optionally filtered
func (l *ComponentLoader) List(opts *ComponentSearchOptions) ([]*Component, error) {
	var components []*Component
	seen := make(map[string]bool)

	// Collect from search paths (local)
	for _, basePath := range l.searchPaths {
		localComps, err := l.listFromPath(basePath, SourceLocal)
		if err == nil {
			for _, comp := range localComps {
				if !seen[comp.Manifest.Name] {
					if matchesOptions(comp, opts) {
						components = append(components, comp)
						seen[comp.Manifest.Name] = true
					}
				}
			}
		}
	}

	// Collect from embedded system components
	if l.embedFS != nil {
		systemComps, err := l.listFromEmbedded()
		if err == nil {
			for _, comp := range systemComps {
				if !seen[comp.Manifest.Name] {
					if matchesOptions(comp, opts) {
						components = append(components, comp)
						seen[comp.Manifest.Name] = true
					}
				}
			}
		}
	}

	return components, nil
}

// listFromPath lists all components in a directory
func (l *ComponentLoader) listFromPath(basePath string, source ComponentSource) ([]*Component, error) {
	var components []*Component

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		componentPath := filepath.Join(basePath, entry.Name())
		if ManifestExists(componentPath) {
			if comp, err := l.loadFromPath(componentPath, source); err == nil {
				components = append(components, comp)
			}
		}
	}

	return components, nil
}

// listFromEmbedded lists all components from embedded filesystem
func (l *ComponentLoader) listFromEmbedded() ([]*Component, error) {
	var components []*Component

	// List directories in system/
	entries, err := l.embedFS.ReadDir("system")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if comp, err := l.loadFromEmbedded(entry.Name()); err == nil {
				components = append(components, comp)
			}
		}
	}

	return components, nil
}

// ClearCache clears the component cache
func (l *ComponentLoader) ClearCache() {
	l.cache = make(map[string]*Component)
}

// GetSearchPaths returns the current search paths
func (l *ComponentLoader) GetSearchPaths() []string {
	return append([]string{}, l.searchPaths...)
}

// AddSearchPath adds a custom search path
func (l *ComponentLoader) AddSearchPath(path string) {
	l.searchPaths = append(l.searchPaths, path)
	l.ClearCache() // Clear cache when paths change
}

// Helper functions

// findProjectComponentDir walks up to find .lvt/components/ directory
func findProjectComponentDir() string {
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		checkPath := filepath.Join(currentDir, ".lvt", "components")
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

// loadTemplates loads template files from a component directory
func loadTemplates(componentPath string, templateFiles []string) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)

	for _, filename := range templateFiles {
		templatePath := filepath.Join(componentPath, filename)
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, ErrTemplateParse{
				Path: templatePath,
				Err:  fmt.Errorf("failed to read template file: %w", err),
			}
		}

		tmpl, err := template.New(filename).Parse(string(data))
		if err != nil {
			return nil, ErrTemplateParse{
				Path: templatePath,
				Err:  fmt.Errorf("failed to parse template: %w", err),
			}
		}

		templates[filename] = tmpl
	}

	return templates, nil
}

// loadTemplatesFromEmbed loads templates from embedded filesystem
func loadTemplatesFromEmbed(embedFS *embed.FS, componentPath string, templateFiles []string) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)

	for _, filename := range templateFiles {
		templatePath := filepath.Join(componentPath, filename)
		data, err := embedFS.ReadFile(templatePath)
		if err != nil {
			return nil, ErrTemplateParse{
				Path: templatePath,
				Err:  fmt.Errorf("failed to read embedded template: %w", err),
			}
		}

		tmpl, err := template.New(filename).Parse(string(data))
		if err != nil {
			return nil, ErrTemplateParse{
				Path: templatePath,
				Err:  fmt.Errorf("failed to parse template: %w", err),
			}
		}

		templates[filename] = tmpl
	}

	return templates, nil
}

// matchesOptions checks if a component matches search options
func matchesOptions(comp *Component, opts *ComponentSearchOptions) bool {
	if opts == nil {
		return true
	}

	// Filter by source
	if opts.Source != "" && comp.Source != opts.Source {
		return false
	}

	// Filter by category
	if opts.Category != "" && comp.Manifest.Category != opts.Category {
		return false
	}

	// Filter by query
	if opts.Query != "" && !comp.Manifest.MatchesQuery(opts.Query) {
		return false
	}

	return true
}
