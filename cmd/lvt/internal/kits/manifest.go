package kits

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const ManifestFileName = "kit.yaml"

// LoadManifest loads a kit manifest from a directory
func LoadManifest(dir string) (*KitManifest, error) {
	manifestPath := filepath.Join(dir, ManifestFileName)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kit manifest: %w", err)
	}

	var manifest KitManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, ErrManifestParse{
			Path: manifestPath,
			Err:  err,
		}
	}

	// Validate version format
	if err := validateVersion(manifest.Version); err != nil {
		return nil, ErrInvalidManifest{
			Field:  "version",
			Reason: err.Error(),
		}
	}

	// Validate manifest
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	// Validate kit name matches directory name
	dirName := filepath.Base(dir)
	if manifest.Name != dirName {
		return nil, ErrInvalidManifest{
			Field:  "name",
			Reason: fmt.Sprintf("kit name '%s' must match directory name '%s'", manifest.Name, dirName),
		}
	}

	return &manifest, nil
}

// SaveManifest saves a kit manifest to a directory
func SaveManifest(dir string, manifest *KitManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}

	manifestPath := filepath.Join(dir, ManifestFileName)

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal kit manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write kit manifest: %w", err)
	}

	return nil
}

// ManifestExists checks if a kit manifest exists in a directory
func ManifestExists(dir string) bool {
	manifestPath := filepath.Join(dir, ManifestFileName)
	_, err := os.Stat(manifestPath)
	return err == nil
}

// validateVersion checks if a version string is valid semantic version
func validateVersion(version string) error {
	// Semantic versioning regex: MAJOR.MINOR.PATCH
	semverRegex := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

	if !semverRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s (expected semantic version like 1.0.0)", version)
	}

	return nil
}
