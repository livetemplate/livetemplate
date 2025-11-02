package fly

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

func TestGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderFly,
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
		"fly.toml",
		"Dockerfile",
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
		Provider: stack.ProviderFly,
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

func TestGenerator_Generate_Postgres(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderFly,
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

	// fly.toml should exist
	flyTomlPath := filepath.Join(tmpDir, "fly.toml")
	if _, err := os.Stat(flyTomlPath); os.IsNotExist(err) {
		t.Errorf("Expected fly.toml does not exist")
	}
}

func TestGenerator_Generate_MultiRegion(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:    stack.ProviderFly,
		Database:    stack.DatabaseSQLite,
		Backup:      stack.BackupNone,
		Redis:       stack.RedisNone,
		Storage:     stack.StorageNone,
		CI:          stack.CINone,
		MultiRegion: true,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check fly.toml exists
	flyTomlPath := filepath.Join(tmpDir, "fly.toml")
	if _, err := os.Stat(flyTomlPath); os.IsNotExist(err) {
		t.Errorf("Expected fly.toml does not exist")
	}
}
