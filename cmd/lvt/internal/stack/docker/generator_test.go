package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack"
)

func TestGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
		Database: stack.DatabaseSQLite,
		Backup:   stack.BackupNone,
		Redis:    stack.RedisNone,
		Storage:  stack.StorageNone,
		CI:       stack.CINone,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check expected files exist
	expectedFiles := []string{
		"docker-compose.yml",
		"Dockerfile",
		".dockerignore",
		".env.example",
		"README.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", file)
		}
	}
}

func TestGenerator_Generate_WithLitestream(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
		Database: stack.DatabaseSQLite,
		Backup:   stack.BackupLitestream,
		Storage:  stack.StorageS3,
		Redis:    stack.RedisNone,
		CI:       stack.CINone,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check litestream.yml exists
	litestreamPath := filepath.Join(tmpDir, "litestream.yml")
	if _, err := os.Stat(litestreamPath); os.IsNotExist(err) {
		t.Errorf("Expected litestream.yml does not exist")
	}
}

func TestDockerComposeYAML_NoDuplicateVolumes(t *testing.T) {
	tests := []struct {
		name     string
		database stack.DatabaseType
		backup   stack.BackupType
	}{
		{
			name:     "sqlite only",
			database: stack.DatabaseSQLite,
			backup:   stack.BackupNone,
		},
		{
			name:     "postgres only",
			database: stack.DatabasePostgres,
			backup:   stack.BackupNone,
		},
		{
			name:     "sqlite with litestream",
			database: stack.DatabaseSQLite,
			backup:   stack.BackupLitestream,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			config := stack.StackConfig{
				Provider: stack.ProviderDocker,
				Database: tt.database,
				Backup:   tt.backup,
				Redis:    stack.RedisNone,
				Storage:  stack.StorageNone,
				CI:       stack.CINone,
			}

			gen := New()
			ctx := context.Background()

			err := gen.Generate(ctx, config, tmpDir)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			composeFile := filepath.Join(tmpDir, "docker-compose.yml")
			content, err := os.ReadFile(composeFile)
			if err != nil {
				t.Fatalf("Failed to read docker-compose.yml: %v", err)
			}

			lines := strings.Split(string(content), "\n")
			topLevelVolumesCount := 0
			for _, line := range lines {
				if line == "volumes:" {
					topLevelVolumesCount++
				}
			}

			if topLevelVolumesCount > 1 {
				t.Errorf("Found %d top-level 'volumes:' declarations, expected at most 1. Content:\n%s", topLevelVolumesCount, string(content))
			}
		})
	}
}
