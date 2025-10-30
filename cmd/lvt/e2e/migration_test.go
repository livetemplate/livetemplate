package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration_Workflow tests the complete migration workflow
func TestMigration_Workflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate a resource to create migrations
	t.Log("Generating users resource...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "users", "name", "email"); err != nil {
		t.Fatalf("Failed to generate users: %v", err)
	}

	// Check migration status (should show pending migration)
	t.Log("Checking migration status...")
	if err := runLvtCommand(t, appDir, "migration", "status"); err != nil {
		t.Fatalf("Failed to check migration status: %v", err)
	}

	// Run migrations
	t.Log("Running migrations up...")
	if err := runLvtCommand(t, appDir, "migration", "up"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify database file was created
	dbPath := filepath.Join(appDir, "app.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file not created after migration")
	}

	// Check migration status again (should show no pending)
	t.Log("Checking migration status after up...")
	if err := runLvtCommand(t, appDir, "migration", "status"); err != nil {
		t.Fatalf("Failed to check migration status: %v", err)
	}

	// Generate another resource
	t.Log("Generating posts resource...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "posts", "title", "content:text"); err != nil {
		t.Fatalf("Failed to generate posts: %v", err)
	}

	// Run new migrations
	t.Log("Running new migrations...")
	if err := runLvtCommand(t, appDir, "migration", "up"); err != nil {
		t.Fatalf("Failed to run new migrations: %v", err)
	}

	t.Log("✅ Migration workflow test passed")
}

// TestMigration_Rollback tests rolling back migrations
func TestMigration_Rollback(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resources to create multiple migrations
	t.Log("Generating resources...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "users", "name"); err != nil {
		t.Fatalf("Failed to generate users: %v", err)
	}
	if err := runLvtCommand(t, appDir, "gen", "resource", "posts", "title"); err != nil {
		t.Fatalf("Failed to generate posts: %v", err)
	}

	// Run all migrations
	t.Log("Running migrations...")
	if err := runLvtCommand(t, appDir, "migration", "up"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify database exists
	dbPath := filepath.Join(appDir, "app.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file not created")
	}

	// Rollback one migration
	t.Log("Rolling back one migration...")
	if err := runLvtCommand(t, appDir, "migration", "down"); err != nil {
		t.Fatalf("Failed to rollback migration: %v", err)
	}

	// Check status
	t.Log("Checking status after rollback...")
	if err := runLvtCommand(t, appDir, "migration", "status"); err != nil {
		t.Fatalf("Failed to check status: %v", err)
	}

	t.Log("✅ Migration rollback test passed")
}

// TestMigration_CreateCustom tests creating custom migration files
func TestMigration_CreateCustom(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Create a custom migration
	migrationName := "add_indexes"
	t.Logf("Creating custom migration: %s...", migrationName)
	if err := runLvtCommand(t, appDir, "migration", "create", migrationName); err != nil {
		t.Fatalf("Failed to create custom migration: %v", err)
	}

	// Verify migration file was created
	migrationsDir := filepath.Join(appDir, "internal/database/migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("Failed to read migrations directory: %v", err)
	}

	found := false
	var migrationFile string
	for _, entry := range entries {
		if strings.Contains(entry.Name(), migrationName) && strings.HasSuffix(entry.Name(), ".sql") {
			found = true
			migrationFile = entry.Name()
			break
		}
	}

	if !found {
		t.Errorf("Custom migration file not found with name: %s", migrationName)
	} else {
		t.Logf("✅ Found migration file: %s", migrationFile)

		// Verify file is not empty and has expected structure
		migrationPath := filepath.Join(migrationsDir, migrationFile)
		content, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("Failed to read migration file: %v", err)
		}

		contentStr := string(content)
		// Check for goose directives
		if !strings.Contains(contentStr, "-- +goose Up") {
			t.Error("Migration file missing +goose Up directive")
		}
		if !strings.Contains(contentStr, "-- +goose Down") {
			t.Error("Migration file missing +goose Down directive")
		}
	}

	t.Log("✅ Custom migration creation test passed")
}
