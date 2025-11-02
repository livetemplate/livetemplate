package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/digitalocean"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/docker"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/fly"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/k8s"
)

func GenStack(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("provider required\n\nUsage: lvt gen stack <provider> [flags]\n\nProviders:\n  docker  - Docker Compose deployment\n  fly     - Fly.io deployment\n  do      - DigitalOcean App Platform\n  k8s     - Kubernetes deployment\n\nFlags:\n  --db <sqlite|postgres|none>               Database type (default: sqlite)\n  --backup <litestream|none>                Backup strategy (default: none)\n  --redis <upstash|fly|none>                Redis provider (default: none)\n  --storage <s3|do-spaces|b2|none>          Storage provider (default: none)\n  --ci <github|gitlab|none>                 CI/CD provider (default: none)\n  --multi-region                            Enable multi-region (fly, k8s only)\n  --namespace <name>                        Kubernetes namespace (k8s only)\n  --ingress <nginx|traefik|none>            Ingress controller (k8s only, default: nginx)\n  --registry <ghcr|docker|gcr|ecr>          Container registry (k8s only, default: ghcr)\n  --force                                   Overwrite existing files")
	}

	provider := args[0]

	// Parse flags
	config := stack.StackConfig{
		Provider: stack.Provider(provider),
		Database: stack.DatabaseSQLite, // default
		Backup:   stack.BackupNone,     // default
		Redis:    stack.RedisNone,      // default
		Storage:  stack.StorageNone,    // default
		CI:       stack.CINone,         // default
	}

	force := false

	// Parse flags
	for i := 1; i < len(args); i++ {
		if args[i] == "--db" && i+1 < len(args) {
			config.Database = stack.DatabaseType(args[i+1])
			i++
		} else if args[i] == "--backup" && i+1 < len(args) {
			config.Backup = stack.BackupType(args[i+1])
			i++
		} else if args[i] == "--redis" && i+1 < len(args) {
			config.Redis = stack.RedisType(args[i+1])
			i++
		} else if args[i] == "--storage" && i+1 < len(args) {
			config.Storage = stack.StorageType(args[i+1])
			i++
		} else if args[i] == "--ci" && i+1 < len(args) {
			config.CI = stack.CIType(args[i+1])
			i++
		} else if args[i] == "--namespace" && i+1 < len(args) {
			config.Namespace = args[i+1]
			i++
		} else if args[i] == "--ingress" && i+1 < len(args) {
			config.Ingress = stack.IngressType(args[i+1])
			i++
		} else if args[i] == "--registry" && i+1 < len(args) {
			config.Registry = stack.RegistryType(args[i+1])
			i++
		} else if args[i] == "--multi-region" {
			config.MultiRegion = true
		} else if args[i] == "--force" {
			force = true
		} else {
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	// Set k8s-specific defaults if not provided
	if config.Provider == stack.ProviderK8s {
		if config.Ingress == "" {
			config.Ingress = stack.IngressNginx
		}
		if config.Registry == "" {
			config.Registry = stack.RegistryGHCR
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check for existing stack
	trackingPath := filepath.Join(wd, ".lvtstack")
	if _, err := os.Stat(trackingPath); err == nil && !force {
		return fmt.Errorf("stack already exists (use --force to overwrite)\n\nRun 'lvt stack info' to see current stack configuration")
	}

	// Create output directory
	outputDir := filepath.Join(wd, "deploy")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create deploy directory: %w", err)
	}

	// Create generator based on provider
	var generator stack.Generator
	switch config.Provider {
	case stack.ProviderDocker:
		generator = docker.New()
	case stack.ProviderFly:
		generator = fly.New()
	case stack.ProviderDigitalOcean:
		generator = digitalocean.New()
	case stack.ProviderK8s:
		generator = k8s.New()
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	// Generate stack
	ctx := context.Background()
	fmt.Printf("Generating %s deployment stack...\n", provider)
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Database: %s\n", config.Database)
	if config.Backup != stack.BackupNone {
		fmt.Printf("  Backup: %s\n", config.Backup)
	}
	if config.Redis != stack.RedisNone {
		fmt.Printf("  Redis: %s\n", config.Redis)
	}
	if config.Storage != stack.StorageNone {
		fmt.Printf("  Storage: %s\n", config.Storage)
	}
	if config.CI != stack.CINone {
		fmt.Printf("  CI/CD: %s\n", config.CI)
	}
	if config.MultiRegion {
		fmt.Printf("  Multi-Region: enabled\n")
	}
	if config.Provider == stack.ProviderK8s {
		if config.Namespace != "" {
			fmt.Printf("  Namespace: %s\n", config.Namespace)
		}
		if config.Ingress != stack.IngressNone {
			fmt.Printf("  Ingress: %s\n", config.Ingress)
		}
		fmt.Printf("  Registry: %s\n", config.Registry)
	}
	fmt.Println()

	if err := generator.Generate(ctx, config, outputDir); err != nil {
		return fmt.Errorf("failed to generate stack: %w", err)
	}

	// Create tracking file
	tracking := stack.NewTrackingFile(config, version)

	// Add generated files to tracking
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(wd, path)
		if err != nil {
			return err
		}

		// Calculate checksum
		checksum, err := calculateFileChecksum(path)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum for %s: %w", relPath, err)
		}

		tracking.AddFile(relPath, checksum)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to track generated files: %w", err)
	}

	// Write tracking file
	if err := tracking.Write(trackingPath); err != nil {
		return fmt.Errorf("failed to write tracking file: %w", err)
	}

	// Print success message
	fmt.Println("Stack generated successfully!")
	fmt.Println()
	fmt.Println("Generated files:")
	for _, f := range tracking.Files {
		fmt.Printf("  %s\n", f.Path)
	}
	fmt.Println()
	fmt.Println("Tracking file created: .lvtstack")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review generated files in %s/\n", outputDir)
	fmt.Println("  2. Configure environment variables")

	switch config.Provider {
	case stack.ProviderDocker:
		fmt.Println("  3. Run: docker-compose up")
	case stack.ProviderFly:
		fmt.Println("  3. Run: fly launch (or fly deploy for existing apps)")
	case stack.ProviderDigitalOcean:
		fmt.Println("  3. Run: doctl apps create --spec deploy/app.yaml")
	case stack.ProviderK8s:
		fmt.Println("  3. Run: kubectl apply -f deploy/")
	}

	fmt.Println()

	return nil
}

// calculateFileChecksum calculates SHA256 checksum for tracking
func calculateFileChecksum(path string) (string, error) {
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

// version is set by the main package
var version = "dev"
