package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

func TestGenStack_FlagParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
		validate    func(*testing.T, stack.StackConfig)
	}{
		{
			name:        "no provider",
			args:        []string{},
			wantErr:     true,
			errContains: "provider required",
		},
		{
			name: "docker with defaults",
			args: []string{"docker"},
			validate: func(t *testing.T, cfg stack.StackConfig) {
				if cfg.Provider != stack.ProviderDocker {
					t.Errorf("Provider = %v, want %v", cfg.Provider, stack.ProviderDocker)
				}
				if cfg.Database != stack.DatabaseSQLite {
					t.Errorf("Database = %v, want %v", cfg.Database, stack.DatabaseSQLite)
				}
			},
		},
		{
			name: "fly with postgres",
			args: []string{"fly", "--db", "postgres"},
			validate: func(t *testing.T, cfg stack.StackConfig) {
				if cfg.Provider != stack.ProviderFly {
					t.Errorf("Provider = %v, want %v", cfg.Provider, stack.ProviderFly)
				}
				if cfg.Database != stack.DatabasePostgres {
					t.Errorf("Database = %v, want %v", cfg.Database, stack.DatabasePostgres)
				}
			},
		},
		{
			name: "k8s with all options",
			args: []string{"k8s", "--db", "postgres", "--redis", "upstash", "--storage", "s3", "--ci", "github", "--namespace", "production", "--ingress", "nginx", "--registry", "ghcr", "--multi-region"},
			validate: func(t *testing.T, cfg stack.StackConfig) {
				if cfg.Provider != stack.ProviderK8s {
					t.Errorf("Provider = %v, want %v", cfg.Provider, stack.ProviderK8s)
				}
				if cfg.Database != stack.DatabasePostgres {
					t.Errorf("Database = %v, want %v", cfg.Database, stack.DatabasePostgres)
				}
				if cfg.Redis != stack.RedisUpstash {
					t.Errorf("Redis = %v, want %v", cfg.Redis, stack.RedisUpstash)
				}
				if cfg.Storage != stack.StorageS3 {
					t.Errorf("Storage = %v, want %v", cfg.Storage, stack.StorageS3)
				}
				if cfg.CI != stack.CIGitHub {
					t.Errorf("CI = %v, want %v", cfg.CI, stack.CIGitHub)
				}
				if cfg.Namespace != "production" {
					t.Errorf("Namespace = %v, want production", cfg.Namespace)
				}
				if cfg.Ingress != stack.IngressNginx {
					t.Errorf("Ingress = %v, want %v", cfg.Ingress, stack.IngressNginx)
				}
				if cfg.Registry != stack.RegistryGHCR {
					t.Errorf("Registry = %v, want %v", cfg.Registry, stack.RegistryGHCR)
				}
				if !cfg.MultiRegion {
					t.Error("MultiRegion = false, want true")
				}
			},
		},
		{
			name: "docker with backup and storage",
			args: []string{"docker", "--db", "sqlite", "--backup", "litestream", "--storage", "s3"},
			validate: func(t *testing.T, cfg stack.StackConfig) {
				if cfg.Backup != stack.BackupLitestream {
					t.Errorf("Backup = %v, want %v", cfg.Backup, stack.BackupLitestream)
				}
				if cfg.Storage != stack.StorageS3 {
					t.Errorf("Storage = %v, want %v", cfg.Storage, stack.StorageS3)
				}
			},
		},
		{
			name:        "backup without storage",
			args:        []string{"docker", "--backup", "litestream"},
			wantErr:     true,
			errContains: "storage flag is required",
		},
		{
			name:        "namespace on non-k8s",
			args:        []string{"docker", "--namespace", "production"},
			wantErr:     true,
			errContains: "namespace only applies to k8s",
		},
		{
			name:        "ingress on non-k8s",
			args:        []string{"fly", "--ingress", "nginx"},
			wantErr:     true,
			errContains: "ingress only applies to k8s",
		},
		{
			name:        "unknown flag",
			args:        []string{"docker", "--unknown"},
			wantErr:     true,
			errContains: "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for testing
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Chdir(oldWd); err != nil {
					t.Error(err)
				}
			}()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			err = GenStack(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("GenStack() error = nil, want error")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GenStack() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil && !tt.wantErr {
				t.Fatalf("GenStack() unexpected error = %v", err)
			}

			// For successful cases, validate the configuration was parsed correctly
			// by reading the tracking file
			if tt.validate != nil && !tt.wantErr {
				trackingPath := filepath.Join(tmpDir, ".lvtstack")
				tracking, err := stack.ReadTrackingFile(trackingPath)
				if err != nil {
					t.Fatalf("Failed to read tracking file: %v", err)
				}

				// Reconstruct config from tracking
				cfg := stack.StackConfig{
					Provider:    stack.Provider(tracking.Provider),
					Database:    stack.DatabaseType(tracking.Configuration.Database),
					Backup:      stack.BackupType(tracking.Configuration.Backup),
					Redis:       stack.RedisType(tracking.Configuration.Redis),
					Storage:     stack.StorageType(tracking.Configuration.Storage),
					CI:          stack.CIType(tracking.Configuration.CI),
					Namespace:   tracking.Configuration.Namespace,
					MultiRegion: tracking.Configuration.MultiRegion,
					Ingress:     stack.IngressType(tracking.Configuration.Ingress),
					Registry:    stack.RegistryType(tracking.Configuration.Registry),
				}

				tt.validate(t, cfg)
			}
		})
	}
}

func TestGenStack_ForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// First generation
	err = GenStack([]string{"docker"})
	if err != nil {
		t.Fatalf("First GenStack() error = %v", err)
	}

	// Second generation without force should fail
	err = GenStack([]string{"docker"})
	if err == nil {
		t.Fatal("Second GenStack() without --force should fail")
	}
	if !contains(err.Error(), "already exists") {
		t.Errorf("Error should mention 'already exists', got: %v", err)
	}

	// Third generation with force should succeed
	err = GenStack([]string{"docker", "--force"})
	if err != nil {
		t.Fatalf("GenStack() with --force error = %v", err)
	}
}

func TestGenStack_ProviderRouting(t *testing.T) {
	providers := []string{"docker", "fly", "do", "k8s"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Chdir(oldWd); err != nil {
					t.Error(err)
				}
			}()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			err = GenStack([]string{provider})
			if err != nil {
				t.Fatalf("GenStack(%s) error = %v", provider, err)
			}

			// Verify deploy directory was created
			deployDir := filepath.Join(tmpDir, "deploy")
			if _, err := os.Stat(deployDir); os.IsNotExist(err) {
				t.Errorf("Deploy directory not created for provider %s", provider)
			}

			// Verify tracking file was created
			trackingPath := filepath.Join(tmpDir, ".lvtstack")
			if _, err := os.Stat(trackingPath); os.IsNotExist(err) {
				t.Errorf("Tracking file not created for provider %s", provider)
			}

			// Verify provider in tracking file
			tracking, err := stack.ReadTrackingFile(trackingPath)
			if err != nil {
				t.Fatalf("Failed to read tracking file: %v", err)
			}
			if tracking.Provider != provider {
				t.Errorf("Tracking file provider = %s, want %s", tracking.Provider, provider)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
