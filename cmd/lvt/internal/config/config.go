package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the config file
	ConfigFileName = "config.yaml"

	// DefaultConfigDir is the default directory for lvt configuration
	// This will be ~/.config/lvt/ on Unix systems
	DefaultConfigDir = ".config/lvt"
)

// Config represents the lvt configuration
type Config struct {
	// ComponentPaths are additional paths to search for components
	ComponentPaths []string `yaml:"component_paths,omitempty"`

	// KitPaths are additional paths to search for kits
	KitPaths []string `yaml:"kit_paths,omitempty"`

	// DefaultKit is the default kit to use when none is specified
	DefaultKit string `yaml:"default_kit,omitempty"`

	// Version tracks the config file version for future migrations
	Version string `yaml:"version,omitempty"`
}

// DefaultConfig returns a new Config with default values
func DefaultConfig() *Config {
	return &Config{
		ComponentPaths: []string{},
		KitPaths:       []string{},
		DefaultKit:     "tailwind",
		Version:        "1.0",
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
func GetConfigDir() (string, error) {
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
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

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
	if config.DefaultKit == "" {
		config.DefaultKit = "tailwind"
	}
	if config.Version == "" {
		config.Version = "1.0"
	}

	return &config, nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig(config *Config) error {
	// Ensure config directory exists
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configPath, err := GetConfigPath()
	if err != nil {
		return err
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

// AddComponentPath adds a component path to the config
func (c *Config) AddComponentPath(path string) error {
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
	for _, p := range c.ComponentPaths {
		if p == absPath {
			return fmt.Errorf("path already exists in config: %s", absPath)
		}
	}

	c.ComponentPaths = append(c.ComponentPaths, absPath)
	return nil
}

// RemoveComponentPath removes a component path from the config
func (c *Config) RemoveComponentPath(path string) error {
	// Convert to absolute path for comparison
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path // Use as-is if can't resolve
	}

	found := false
	newPaths := []string{}
	for _, p := range c.ComponentPaths {
		if p != absPath {
			newPaths = append(newPaths, p)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("path not found in config: %s", path)
	}

	c.ComponentPaths = newPaths
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

// SetDefaultKit sets the default kit
func (c *Config) SetDefaultKit(kit string) {
	c.DefaultKit = kit
}

// GetDefaultKit returns the default kit
func (c *Config) GetDefaultKit() string {
	if c.DefaultKit == "" {
		return "tailwind"
	}
	return c.DefaultKit
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate component paths exist
	for _, path := range c.ComponentPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("component path does not exist: %s", path)
		}
	}

	// Validate kit paths exist
	for _, path := range c.KitPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("kit path does not exist: %s", path)
		}
	}

	return nil
}
