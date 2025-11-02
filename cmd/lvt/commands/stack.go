package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/digitalocean"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/docker"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/fly"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/k8s"
)

func Stack(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("subcommand required\n\nUsage: lvt stack <subcommand>\n\nSubcommands:\n  validate  Validate stack configuration\n  info      Show stack information")
	}

	subcommand := args[0]

	switch subcommand {
	case "validate":
		return StackValidate(args[1:])
	case "info":
		return StackInfo(args[1:])
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nAvailable subcommands:\n  validate  Validate stack configuration\n  info      Show stack information", subcommand)
	}
}

func StackValidate(args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Read tracking file
	trackingPath := filepath.Join(wd, ".lvtstack")
	tracking, err := stack.ReadTrackingFile(trackingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no stack found. Run 'lvt gen stack <provider>' to create one")
		}
		return fmt.Errorf("failed to read tracking file: %w", err)
	}

	// Check for modifications
	fmt.Println("Checking for modified files...")
	modified, err := tracking.CheckModifications(wd)
	if err != nil {
		return fmt.Errorf("failed to check modifications: %w", err)
	}

	if len(modified) > 0 {
		fmt.Println()
		fmt.Println("Warning: The following files have been modified:")
		for _, f := range modified {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
		fmt.Println("Manual changes may be overwritten if you regenerate the stack.")
	} else {
		fmt.Println("All tracked files are unchanged.")
		fmt.Println()
	}

	// Create generator
	generator, err := createGenerator(stack.Provider(tracking.Provider))
	if err != nil {
		return err
	}

	// Run provider-specific validation
	stackDir := filepath.Join(wd, "deploy")
	ctx := context.Background()

	fmt.Println("Running provider-specific validation...")
	if err := generator.Validate(ctx, stackDir); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		return err
	}

	fmt.Println("Stack configuration is valid!")
	return nil
}

func StackInfo(args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Read tracking file
	trackingPath := filepath.Join(wd, ".lvtstack")
	tracking, err := stack.ReadTrackingFile(trackingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no stack found. Run 'lvt gen stack <provider>' to create one")
		}
		return fmt.Errorf("failed to read tracking file: %w", err)
	}

	// Print basic information
	fmt.Println("Stack Information")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()
	fmt.Printf("Provider:         %s\n", tracking.Provider)
	fmt.Printf("Generated:        %s\n", tracking.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Generator:        %s\n", tracking.GeneratorVersion)
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Database:       %s\n", tracking.Configuration.Database)
	if tracking.Configuration.Backup != "" && tracking.Configuration.Backup != "none" {
		fmt.Printf("  Backup:         %s\n", tracking.Configuration.Backup)
	}
	if tracking.Configuration.Redis != "" && tracking.Configuration.Redis != "none" {
		fmt.Printf("  Redis:          %s\n", tracking.Configuration.Redis)
	}
	if tracking.Configuration.Storage != "" && tracking.Configuration.Storage != "none" {
		fmt.Printf("  Storage:        %s\n", tracking.Configuration.Storage)
	}
	if tracking.Configuration.CI != "" && tracking.Configuration.CI != "none" {
		fmt.Printf("  CI/CD:          %s\n", tracking.Configuration.CI)
	}
	if tracking.Configuration.MultiRegion {
		fmt.Printf("  Multi-Region:   enabled\n")
	}
	if tracking.Configuration.Namespace != "" {
		fmt.Printf("  Namespace:      %s\n", tracking.Configuration.Namespace)
	}
	if tracking.Configuration.Ingress != "" && tracking.Configuration.Ingress != "none" {
		fmt.Printf("  Ingress:        %s\n", tracking.Configuration.Ingress)
	}
	if tracking.Configuration.Registry != "" {
		fmt.Printf("  Registry:       %s\n", tracking.Configuration.Registry)
	}

	// Check for modifications
	modified, err := tracking.CheckModifications(wd)
	if err != nil {
		return fmt.Errorf("failed to check modifications: %w", err)
	}

	fmt.Println()
	fmt.Printf("Tracked Files:    %d\n", len(tracking.Files))
	if len(modified) > 0 {
		fmt.Printf("Modified Files:   %d\n", len(modified))
		fmt.Println()
		fmt.Println("Modified:")
		for _, f := range modified {
			fmt.Printf("  %s\n", f)
		}
	} else {
		fmt.Printf("Modified Files:   0\n")
	}

	// Get provider-specific info
	generator, err := createGenerator(stack.Provider(tracking.Provider))
	if err != nil {
		return err
	}

	stackDir := filepath.Join(wd, "deploy")
	ctx := context.Background()

	info, err := generator.GetInfo(ctx, stackDir)
	if err != nil {
		// Don't fail if provider-specific info is not available
		fmt.Printf("\nNote: Provider-specific information not available: %v\n", err)
		return nil
	}

	if info != nil {
		if len(info.RequiredSecrets) > 0 {
			fmt.Println()
			fmt.Println("Required Secrets:")
			for _, secret := range info.RequiredSecrets {
				fmt.Printf("  - %s\n", secret)
			}
		}

		if info.DeploymentCommand != "" {
			fmt.Println()
			fmt.Println("Deployment Command:")
			fmt.Printf("  %s\n", info.DeploymentCommand)
		}

		if info.EstimatedCost != "" {
			fmt.Println()
			fmt.Printf("Estimated Cost:   %s\n", info.EstimatedCost)
		}
	}

	return nil
}

func createGenerator(provider stack.Provider) (stack.Generator, error) {
	switch provider {
	case stack.ProviderDocker:
		return docker.New(), nil
	case stack.ProviderFly:
		return fly.New(), nil
	case stack.ProviderDigitalOcean:
		return digitalocean.New(), nil
	case stack.ProviderK8s:
		return k8s.New(), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
