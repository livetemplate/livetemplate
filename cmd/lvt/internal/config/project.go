package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ProjectConfigFileName is the name of the project config file
	ProjectConfigFileName = ".lvtrc"
)

// ProjectConfig represents the project-level configuration
type ProjectConfig struct {
	// Kit is the kit used for this project
	Kit string

	// DevMode indicates whether to use local client library
	DevMode bool
}

// DefaultProjectConfig returns a new ProjectConfig with default values
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Kit:     "multi",
		DevMode: false,
	}
}

// LoadProjectConfig loads the project configuration from .lvtrc in the specified directory
// If the file doesn't exist, returns a default config
func LoadProjectConfig(basePath string) (*ProjectConfig, error) {
	configPath := filepath.Join(basePath, ProjectConfigFileName)

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultProjectConfig(), nil
	}

	config := DefaultProjectConfig()

	// Read config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "kit":
			config.Kit = value
		case "dev_mode":
			config.DevMode = value == "true"
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	return config, nil
}

// SaveProjectConfig saves the project configuration to .lvtrc in the specified directory
func SaveProjectConfig(basePath string, config *ProjectConfig) error {
	configPath := filepath.Join(basePath, ProjectConfigFileName)

	content := fmt.Sprintf("kit=%s\ndev_mode=%v\n",
		config.Kit, config.DevMode)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	return nil
}

// GetKit returns the kit for the project
func (c *ProjectConfig) GetKit() string {
	if c.Kit == "" {
		return "multi"
	}
	return c.Kit
}

// Validate validates the project configuration
func (c *ProjectConfig) Validate() error {
	validKits := map[string]bool{"multi": true, "single": true, "simple": true}
	if !validKits[c.Kit] {
		return fmt.Errorf("invalid kit: %s (valid: multi, single, simple)", c.Kit)
	}

	return nil
}
