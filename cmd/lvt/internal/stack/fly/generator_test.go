package fly

import (
	"bytes"
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

	// Check Dockerfile contains SHA256 checksum verification
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}

	expectedChecksum := "eb75a3de5cab03875cdae9f5f539e6aedadd66607003d9b1e7a9077948818ba0"
	if !bytes.Contains(dockerfileContent, []byte(expectedChecksum)) {
		t.Errorf("Dockerfile does not contain expected SHA256 checksum for Litestream v0.3.13")
	}

	if !bytes.Contains(dockerfileContent, []byte("sha256sum -c -")) {
		t.Errorf("Dockerfile does not contain SHA256 verification command")
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
