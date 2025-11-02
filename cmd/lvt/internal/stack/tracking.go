package stack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// TrackingFile represents the .lvtstack file
type TrackingFile struct {
	Version          int            `yaml:"version"`
	Provider         string         `yaml:"provider"`
	GeneratedAt      time.Time      `yaml:"generated_at"`
	GeneratorVersion string         `yaml:"generator_version"`
	Configuration    TrackingConfig `yaml:"configuration"`
	Files            []TrackedFile  `yaml:"files"`
}

// TrackingConfig holds the configuration used to generate the stack
type TrackingConfig struct {
	Database    string `yaml:"database"`
	Backup      string `yaml:"backup"`
	Redis       string `yaml:"redis"`
	Storage     string `yaml:"storage"`
	CI          string `yaml:"ci"`
	Namespace   string `yaml:"namespace,omitempty"`
	MultiRegion bool   `yaml:"multi_region,omitempty"`
	Ingress     string `yaml:"ingress,omitempty"`
	Registry    string `yaml:"registry,omitempty"`
}

// TrackedFile represents a single tracked file
type TrackedFile struct {
	Path     string `yaml:"path"`
	Checksum string `yaml:"checksum"`
	Modified bool   `yaml:"modified"`
}

// Write writes the tracking file to disk
func (t *TrackingFile) Write(path string) error {
	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal tracking file: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracking file: %w", err)
	}

	return nil
}

// ReadTrackingFile reads a tracking file from disk
func ReadTrackingFile(path string) (*TrackingFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracking file: %w", err)
	}

	var tracking TrackingFile
	if err := yaml.Unmarshal(data, &tracking); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tracking file: %w", err)
	}

	return &tracking, nil
}

// CheckModifications checks if any tracked files have been modified
func (t *TrackingFile) CheckModifications(baseDir string) ([]string, error) {
	var modified []string

	for i := range t.Files {
		file := &t.Files[i]
		fullPath := filepath.Join(baseDir, file.Path)

		currentChecksum, err := calculateChecksum(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File might have been deleted
			}
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", file.Path, err)
		}

		if currentChecksum != file.Checksum {
			file.Modified = true
			modified = append(modified, file.Path)
		}
	}

	return modified, nil
}

// calculateChecksum calculates SHA256 checksum of a file
func calculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// NewTrackingFile creates a new tracking file from config
func NewTrackingFile(config StackConfig, generatorVersion string) *TrackingFile {
	return &TrackingFile{
		Version:          1,
		Provider:         string(config.Provider),
		GeneratedAt:      time.Now(),
		GeneratorVersion: generatorVersion,
		Configuration: TrackingConfig{
			Database:    string(config.Database),
			Backup:      string(config.Backup),
			Redis:       string(config.Redis),
			Storage:     string(config.Storage),
			CI:          string(config.CI),
			Namespace:   config.Namespace,
			MultiRegion: config.MultiRegion,
			Ingress:     string(config.Ingress),
			Registry:    string(config.Registry),
		},
		Files: []TrackedFile{},
	}
}

// AddFile adds a file to tracking
func (t *TrackingFile) AddFile(path, checksum string) {
	t.Files = append(t.Files, TrackedFile{
		Path:     path,
		Checksum: checksum,
		Modified: false,
	})
}
