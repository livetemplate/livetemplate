package kits

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// KitLoader handles loading kits from various sources
type KitLoader struct {
	searchPaths []string            // Paths to search for kits
	cache       map[string]*KitInfo // Cached loaded kits
	embedFS     *embed.FS           // Embedded filesystem for system kits
	configPaths []string            // Paths from user config
	projectPath string              // Project-specific path (.lvt/kits)
}

// NewLoader creates a new kit loader with default paths
func NewLoader(embedFS *embed.FS) *KitLoader {
	loader := &KitLoader{
		cache:   make(map[string]*KitInfo),
		embedFS: embedFS,
	}

	// Build search paths in priority order
	loader.buildSearchPaths()

	return loader
}

// buildSearchPaths constructs the search paths in priority order:
// 1. Project path (.lvt/kits/)
// 2. Config paths (from ~/.config/lvt/config.yaml)
// 3. Embedded system kits (fallback)
func (l *KitLoader) buildSearchPaths() {
	var paths []string

	// 1. Project path
	if projectPath := findProjectKitDir(); projectPath != "" {
		l.projectPath = projectPath
		paths = append(paths, projectPath)
	}

	// 2. Config paths (to be loaded from config)
	// TODO: Load from ~/.config/lvt/config.yaml when config package is ready

	l.searchPaths = paths
}

// Load loads a kit by name from the first matching source
func (l *KitLoader) Load(name string) (*KitInfo, error) {
	// Check cache first
	if cached, exists := l.cache[name]; exists {
		return cached, nil
	}

	// Try to load from search paths (local)
	for _, basePath := range l.searchPaths {
		kitPath := filepath.Join(basePath, name)
		if kit, err := l.loadFromPath(kitPath, SourceLocal); err == nil {
			l.cache[name] = kit
			return kit, nil
		}
	}

	// Try to load from embedded system kits
	if l.embedFS != nil {
		if kit, err := l.loadFromEmbedded(name); err == nil {
			l.cache[name] = kit
			return kit, nil
		}
	}

	return nil, ErrKitNotFound{Name: name}
}

// loadFromPath loads a kit from a filesystem path
func (l *KitLoader) loadFromPath(path string, source KitSource) (*KitInfo, error) {
	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("kit directory not found: %s", path)
	}

	// Check if manifest exists
	if !ManifestExists(path) {
		return nil, fmt.Errorf("kit.yaml not found in: %s", path)
	}

	// Load manifest
	manifest, err := LoadManifest(path)
	if err != nil {
		return nil, err
	}

	// Load helpers based on framework
	helpers, err := loadHelpers(manifest.Framework, path)
	if err != nil {
		return nil, ErrHelperLoad{
			Kit: manifest.Name,
			Err: err,
		}
	}

	kit := &KitInfo{
		Manifest: *manifest,
		Source:   source,
		Path:     path,
		Helpers:  helpers,
	}

	return kit, nil
}

// loadFromEmbedded loads a kit from the embedded filesystem
func (l *KitLoader) loadFromEmbedded(name string) (*KitInfo, error) {
	if l.embedFS == nil {
		return nil, fmt.Errorf("embedded filesystem not available")
	}

	// Embedded kits are in system/ directory
	kitPath := filepath.Join("system", name)

	// Read manifest from embedded FS
	manifestPath := filepath.Join(kitPath, ManifestFileName)
	data, err := l.embedFS.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("kit not found in embedded FS: %s", name)
	}

	// Parse manifest
	var manifest KitManifest
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

	// Load helpers based on framework
	helpers, err := loadHelpers(manifest.Framework, "")
	if err != nil {
		return nil, ErrHelperLoad{
			Kit: manifest.Name,
			Err: err,
		}
	}

	kit := &KitInfo{
		Manifest: manifest,
		Source:   SourceSystem,
		Path:     kitPath,
		Helpers:  helpers,
	}

	return kit, nil
}

// List returns all available kits, optionally filtered
func (l *KitLoader) List(opts *KitSearchOptions) ([]*KitInfo, error) {
	var kits []*KitInfo
	seen := make(map[string]bool)

	// Collect from search paths (local)
	for _, basePath := range l.searchPaths {
		localKits, err := l.listFromPath(basePath, SourceLocal)
		if err == nil {
			for _, kit := range localKits {
				if !seen[kit.Manifest.Name] {
					if matchesOptions(kit, opts) {
						kits = append(kits, kit)
						seen[kit.Manifest.Name] = true
					}
				}
			}
		}
	}

	// Collect from embedded system kits
	if l.embedFS != nil {
		systemKits, err := l.listFromEmbedded()
		if err == nil {
			for _, kit := range systemKits {
				if !seen[kit.Manifest.Name] {
					if matchesOptions(kit, opts) {
						kits = append(kits, kit)
						seen[kit.Manifest.Name] = true
					}
				}
			}
		}
	}

	return kits, nil
}

// listFromPath lists all kits in a directory
func (l *KitLoader) listFromPath(basePath string, source KitSource) ([]*KitInfo, error) {
	var kits []*KitInfo

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		kitPath := filepath.Join(basePath, entry.Name())
		if ManifestExists(kitPath) {
			if kit, err := l.loadFromPath(kitPath, source); err == nil {
				kits = append(kits, kit)
			}
		}
	}

	return kits, nil
}

// listFromEmbedded lists all kits from embedded filesystem
func (l *KitLoader) listFromEmbedded() ([]*KitInfo, error) {
	var kits []*KitInfo

	// List directories in system/
	entries, err := l.embedFS.ReadDir("system")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if kit, err := l.loadFromEmbedded(entry.Name()); err == nil {
				kits = append(kits, kit)
			}
		}
	}

	return kits, nil
}

// ClearCache clears the kit cache
func (l *KitLoader) ClearCache() {
	l.cache = make(map[string]*KitInfo)
}

// GetSearchPaths returns the current search paths
func (l *KitLoader) GetSearchPaths() []string {
	return append([]string{}, l.searchPaths...)
}

// AddSearchPath adds a custom search path
func (l *KitLoader) AddSearchPath(path string) {
	l.searchPaths = append(l.searchPaths, path)
	l.ClearCache() // Clear cache when paths change
}

// Helper functions

// findProjectKitDir walks up to find .lvt/kits/ directory
func findProjectKitDir() string {
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		checkPath := filepath.Join(currentDir, ".lvt", "kits")
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

// loadHelpers loads the appropriate CSS helpers implementation based on framework
func loadHelpers(framework string, kitPath string) (CSSHelpers, error) {
	// TODO: Implement actual helper loading
	// For now, return a factory-based helper
	switch framework {
	case "tailwind":
		return NewTailwindHelpers(), nil
	case "bulma":
		return NewBulmaHelpers(), nil
	case "pico":
		return NewPicoHelpers(), nil
	case "none":
		return NewNoneHelpers(), nil
	default:
		return nil, fmt.Errorf("unsupported framework: %s", framework)
	}
}

// matchesOptions checks if a kit matches search options
func matchesOptions(kit *KitInfo, opts *KitSearchOptions) bool {
	if opts == nil {
		return true
	}

	// Filter by source
	if opts.Source != "" && kit.Source != opts.Source {
		return false
	}

	// Filter by query
	if opts.Query != "" && !kit.Manifest.MatchesQuery(opts.Query) {
		return false
	}

	return true
}

// unmarshalYAML is a helper function to unmarshal YAML data
func unmarshalYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
