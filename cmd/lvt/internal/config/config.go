package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// globalConfigPath stores the custom config path if set via --config flag
var globalConfigPath string

// SetConfigPath sets a custom config path for the current session
func SetConfigPath(path string) {
	globalConfigPath = path
}

const (
	// ConfigFileName is the name of the config file
	ConfigFileName = "config.yaml"

	// DefaultConfigDir is the default directory for lvt configuration
	// This will be ~/.config/lvt/ on Unix systems
	DefaultConfigDir = ".config/lvt"
)

// Config represents the lvt configuration
type Config struct {
	// KitPaths are additional paths to search for kits
	// Standard paths (~/.config/lvt/kits/ and .lvt/kits/) are searched automatically
	KitPaths []string `yaml:"kit_paths,omitempty"`

	// Version tracks the config file version for future migrations
	Version string `yaml:"version,omitempty"`
}

// DefaultConfig returns a new Config with default values
func DefaultConfig() *Config {
	return &Config{
		KitPaths: []string{},
		Version:  "1.0",
	}
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, DefaultConfigDir)
	return filepath.Join(configDir, ConfigFileName), nil
}

// GetConfigDir returns the directory containing the config file
// Uses custom config path if set via SetConfigPath
func GetConfigDir() (string, error) {
	// If custom config path is set, return its directory
	if globalConfigPath != "" {
		return filepath.Dir(globalConfigPath), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, DefaultConfigDir), nil
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return nil
}

// LoadConfig loads the configuration from the config file
// If the file doesn't exist, returns a default config
// Uses custom config path if set via SetConfigPath, otherwise uses default
func LoadConfig() (*Config, error) {
	var configPath string
	var err error

	// Use custom config path if set, otherwise use default
	if globalConfigPath != "" {
		configPath = globalConfigPath
	} else {
		configPath, err = GetConfigPath()
		if err != nil {
			return nil, err
		}
	}

	return LoadConfigFromPath(configPath)
}

// SaveConfig saves the configuration to the config file
// Uses custom config path if set via SetConfigPath
func SaveConfig(config *Config) error {
	// Ensure config directory exists
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	var configPath string
	var err error

	// Use custom config path if set, otherwise use default
	if globalConfigPath != "" {
		configPath = globalConfigPath
	} else {
		configPath, err = GetConfigPath()
		if err != nil {
			return err
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddKitPath adds a kit path to the config
func (c *Config) AddKitPath(path string) error {
	// Validate path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if already exists
	for _, p := range c.KitPaths {
		if p == absPath {
			return fmt.Errorf("path already exists in config: %s", absPath)
		}
	}

	c.KitPaths = append(c.KitPaths, absPath)
	return nil
}

// RemoveKitPath removes a kit path from the config
func (c *Config) RemoveKitPath(path string) error {
	// Convert to absolute path for comparison
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path // Use as-is if can't resolve
	}

	found := false
	newPaths := []string{}
	for _, p := range c.KitPaths {
		if p != absPath {
			newPaths = append(newPaths, p)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("path not found in config: %s", path)
	}

	c.KitPaths = newPaths
	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate kit paths exist
	for _, path := range c.KitPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("kit path does not exist: %s", path)
		}
	}

	return nil
}

// LoadConfigFromPath loads configuration from a specific path
func LoadConfigFromPath(configPath string) (*Config, error) {
	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults for missing fields
	if config.Version == "" {
		config.Version = "1.0"
	}

	return &config, nil
}
