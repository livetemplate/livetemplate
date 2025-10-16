package components

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const (
	ManifestFileName = "component.yaml"
)

var (
	// semverRegex validates semantic versioning (e.g., 1.0.0, 1.2.3-beta)
	semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[a-zA-Z0-9\-\.]+)?(\+[a-zA-Z0-9\-\.]+)?$`)
)

// LoadManifest loads and parses a component manifest from a directory
func LoadManifest(dir string) (*ComponentManifest, error) {
	manifestPath := filepath.Join(dir, ManifestFileName)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, ErrManifestParse{
			Path: manifestPath,
			Err:  fmt.Errorf("failed to read file: %w", err),
		}
	}

	var manifest ComponentManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, ErrManifestParse{
			Path: manifestPath,
			Err:  fmt.Errorf("failed to parse YAML: %w", err),
		}
	}

	// Validate the manifest
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	// Additional validation
	if err := validateVersion(manifest.Version); err != nil {
		return nil, ErrInvalidManifest{
			Field:  "version",
			Reason: err.Error(),
		}
	}

	// Validate component name matches directory name
	dirName := filepath.Base(dir)
	if manifest.Name != dirName {
		return nil, ErrInvalidManifest{
			Field:  "name",
			Reason: fmt.Sprintf("component name '%s' must match directory name '%s'", manifest.Name, dirName),
		}
	}

	return &manifest, nil
}

// SaveManifest saves a component manifest to a directory
func SaveManifest(dir string, manifest *ComponentManifest) error {
	// Validate before saving
	if err := manifest.Validate(); err != nil {
		return err
	}

	manifestPath := filepath.Join(dir, ManifestFileName)

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

// validateVersion checks if version follows semantic versioning
func validateVersion(version string) error {
	if !semverRegex.MatchString(version) {
		return fmt.Errorf("version must follow semantic versioning (e.g., 1.0.0)")
	}
	return nil
}

// ManifestExists checks if a component manifest exists in a directory
func ManifestExists(dir string) bool {
	manifestPath := filepath.Join(dir, ManifestFileName)
	_, err := os.Stat(manifestPath)
	return err == nil
}

// ValidateManifestFile validates a manifest file without loading the full component
func ValidateManifestFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return ErrManifestParse{
			Path: path,
			Err:  fmt.Errorf("failed to read file: %w", err),
		}
	}

	var manifest ComponentManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return ErrManifestParse{
			Path: path,
			Err:  fmt.Errorf("failed to parse YAML: %w", err),
		}
	}

	return manifest.Validate()
}
