package digitalocean

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack"
)

func TestGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDigitalOcean,
		Database: stack.DatabasePostgres,
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
		"app-spec.yaml",
		"Dockerfile",
		"README.md",
		".env.example",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", file)
		}
	}
}

func TestGenerator_Generate_SQLite(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDigitalOcean,
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
		"app-spec.yaml",
		"Dockerfile",
		"README.md",
		".env.example",
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
		Provider: stack.ProviderDigitalOcean,
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

func TestGenerator_Generate_WithRedis(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDigitalOcean,
		Database: stack.DatabasePostgres,
		Backup:   stack.BackupNone,
		Redis:    stack.RedisUpstash,
		Storage:  stack.StorageNone,
		CI:       stack.CINone,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check that files are generated
	expectedFiles := []string{
		"app-spec.yaml",
		"README.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", file)
		}
	}
}
