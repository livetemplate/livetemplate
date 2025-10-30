package e2e

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestSeed_GenerateData tests generating test data for a resource
func TestSeed_GenerateData(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate a resource
	t.Log("Generating products resource...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "products", "name", "price:float"); err != nil {
		t.Fatalf("Failed to generate products: %v", err)
	}

	// Run migrations
	t.Log("Running migrations...")
	if err := runLvtCommand(t, appDir, "migration", "up"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed data
	seedCount := 25
	t.Logf("Seeding %d products...", seedCount)
	if err := runLvtCommand(t, appDir, "seed", "products", "--count", "25"); err != nil {
		t.Fatalf("Failed to seed data: %v", err)
	}

	// Verify data was created in database
	dbPath := filepath.Join(appDir, "app.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query products count: %v", err)
	}

	if count != seedCount {
		t.Errorf("Expected %d products, got %d", seedCount, count)
	} else {
		t.Logf("✅ Successfully seeded %d products", count)
	}

	t.Log("✅ Generate data test passed")
}

// TestSeed_Cleanup tests cleaning up seeded data
func TestSeed_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate a resource
	t.Log("Generating users resource...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "users", "name", "email"); err != nil {
		t.Fatalf("Failed to generate users: %v", err)
	}

	// Run migrations
	t.Log("Running migrations...")
	if err := runLvtCommand(t, appDir, "migration", "up"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed data
	t.Log("Seeding 20 users...")
	if err := runLvtCommand(t, appDir, "seed", "users", "--count", "20"); err != nil {
		t.Fatalf("Failed to seed data: %v", err)
	}

	// Verify data exists
	dbPath := filepath.Join(appDir, "app.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var countBefore int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&countBefore)
	if err != nil {
		t.Fatalf("Failed to query users count: %v", err)
	}
	t.Logf("Users before cleanup: %d", countBefore)

	if countBefore != 20 {
		t.Errorf("Expected 20 users before cleanup, got %d", countBefore)
	}

	// Cleanup
	t.Log("Cleaning up seeded data...")
	if err := runLvtCommand(t, appDir, "seed", "users", "--cleanup"); err != nil {
		t.Fatalf("Failed to cleanup data: %v", err)
	}

	// Verify data was removed
	var countAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&countAfter)
	if err != nil {
		t.Fatalf("Failed to query users count after cleanup: %v", err)
	}
	t.Logf("Users after cleanup: %d", countAfter)

	if countAfter != 0 {
		t.Errorf("Expected 0 users after cleanup, got %d", countAfter)
	}

	t.Log("✅ Cleanup test passed")
}

// TestSeed_CleanupAndReseed tests cleaning up and reseeding in one command
func TestSeed_CleanupAndReseed(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate a resource
	t.Log("Generating tasks resource...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "tasks", "title", "completed:bool"); err != nil {
		t.Fatalf("Failed to generate tasks: %v", err)
	}

	// Run migrations
	t.Log("Running migrations...")
	if err := runLvtCommand(t, appDir, "migration", "up"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initial seed
	t.Log("Initial seed: 15 tasks...")
	if err := runLvtCommand(t, appDir, "seed", "tasks", "--count", "15"); err != nil {
		t.Fatalf("Failed to seed initial data: %v", err)
	}

	// Cleanup and reseed with different count
	newCount := 30
	t.Logf("Cleanup and reseed with %d tasks...", newCount)
	if err := runLvtCommand(t, appDir, "seed", "tasks", "--count", "30", "--cleanup"); err != nil {
		t.Fatalf("Failed to cleanup and reseed: %v", err)
	}

	// Verify new count
	dbPath := filepath.Join(appDir, "app.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tasks count: %v", err)
	}

	if count != newCount {
		t.Errorf("Expected %d tasks after cleanup and reseed, got %d", newCount, count)
	} else {
		t.Logf("✅ Successfully reseeded to %d tasks", count)
	}

	t.Log("✅ Cleanup and reseed test passed")
}
