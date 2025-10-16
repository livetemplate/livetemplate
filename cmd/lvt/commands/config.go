package commands

import (
	"fmt"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/config"
)

// Config handles configuration management commands
func Config(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required: get, set, list, add-component-path, remove-component-path, add-kit-path, remove-kit-path")
	}

	command := args[0]

	switch command {
	case "get":
		return configGet(args[1:])
	case "set":
		return configSet(args[1:])
	case "list":
		return configList(args[1:])
	case "add-component-path":
		return configAddComponentPath(args[1:])
	case "remove-component-path":
		return configRemoveComponentPath(args[1:])
	case "add-kit-path":
		return configAddKitPath(args[1:])
	case "remove-kit-path":
		return configRemoveKitPath(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// configGet retrieves a configuration value
func configGet(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("key required: lvt config get <key>")
	}

	key := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch key {
	case "default_kit":
		fmt.Println(cfg.GetDefaultKit())
	case "component_paths":
		if len(cfg.ComponentPaths) == 0 {
			fmt.Println("(none)")
		} else {
			for _, path := range cfg.ComponentPaths {
				fmt.Println(path)
			}
		}
	case "kit_paths":
		if len(cfg.KitPaths) == 0 {
			fmt.Println("(none)")
		} else {
			for _, path := range cfg.KitPaths {
				fmt.Println(path)
			}
		}
	default:
		return fmt.Errorf("unknown key: %s (expected: default_kit, component_paths, kit_paths)", key)
	}

	return nil
}

// configSet sets a configuration value
func configSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("key and value required: lvt config set <key> <value>")
	}

	key := args[0]
	value := strings.Join(args[1:], " ")

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch key {
	case "default_kit":
		cfg.SetDefaultKit(value)
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("✅ Set default_kit to: %s\n", value)
	default:
		return fmt.Errorf("unknown key: %s (expected: default_kit)", key)
	}

	return nil
}

// configList lists all configuration values
func configList(args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Configuration:")
	fmt.Println()

	fmt.Printf("Default Kit:        %s\n", cfg.GetDefaultKit())
	fmt.Println()

	fmt.Println("Component Paths:")
	if len(cfg.ComponentPaths) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, path := range cfg.ComponentPaths {
			fmt.Printf("  - %s\n", path)
		}
	}
	fmt.Println()

	fmt.Println("Kit Paths:")
	if len(cfg.KitPaths) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, path := range cfg.KitPaths {
			fmt.Printf("  - %s\n", path)
		}
	}
	fmt.Println()

	configPath, _ := config.GetConfigPath()
	fmt.Printf("Config file: %s\n", configPath)

	return nil
}

// configAddComponentPath adds a component path to the configuration
func configAddComponentPath(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path required: lvt config add-component-path <path>")
	}

	path := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.AddComponentPath(path); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Added component path: %s\n", path)
	return nil
}

// configRemoveComponentPath removes a component path from the configuration
func configRemoveComponentPath(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path required: lvt config remove-component-path <path>")
	}

	path := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.RemoveComponentPath(path); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Removed component path: %s\n", path)
	return nil
}

// configAddKitPath adds a kit path to the configuration
func configAddKitPath(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path required: lvt config add-kit-path <path>")
	}

	path := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.AddKitPath(path); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Added kit path: %s\n", path)
	return nil
}

// configRemoveKitPath removes a kit path from the configuration
func configRemoveKitPath(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path required: lvt config remove-kit-path <path>")
	}

	path := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.RemoveKitPath(path); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Removed kit path: %s\n", path)
	return nil
}
