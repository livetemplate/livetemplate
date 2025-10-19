package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
	"github.com/livefir/livetemplate/cmd/lvt/internal/parser"
)

// TestGeneratedCodeSyntax validates that generated code has valid Go syntax
func TestGeneratedCodeSyntax(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database directory structure
	dbDir := filepath.Join(tmpDir, "internal", "database")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create database directory: %v", err)
	}

	// Generate CRUD resource
	fields := []parser.Field{
		{Name: "name", Type: "string", GoType: "string", SQLType: "TEXT"},
		{Name: "email", Type: "string", GoType: "string", SQLType: "TEXT"},
	}

	if err := generator.GenerateResource(tmpDir, "testmodule", "User", fields, "multi", "tailwind", "infinite", 20, "modal"); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Generate view
	if err := generator.GenerateView(tmpDir, "testmodule", "Counter", "multi", "tailwind"); err != nil {
		t.Fatalf("Failed to generate view: %v", err)
	}

	// Check generated Go files for syntax errors
	goFiles := []string{
		filepath.Join(tmpDir, "internal", "app", "user", "user.go"),
		filepath.Join(tmpDir, "internal", "app", "counter", "counter.go"),
	}

	for _, file := range goFiles {
		// Use go/parser to check syntax
		cmd := exec.Command("go", "tool", "compile", "-o", "/dev/null", file)
		// We expect this to fail due to unresolved imports, but syntax should be valid
		output, _ := cmd.CombinedOutput()

		// Check for syntax errors (not import errors)
		if strings.Contains(string(output), "syntax error") {
			t.Errorf("Syntax error in %s:\n%s", file, output)
		}
	}

	t.Log("✅ Generated code has valid Go syntax")
}

// TestGeneratedFilesExist validates that all expected files are generated
func TestGeneratedFilesExist(t *testing.T) {
	tmpDir := t.TempDir()

	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	// Restore working directory after test
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	})

	// Generate app
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	if err := generator.GenerateApp("testapp", "testapp", "multi", "tailwind", false); err != nil { // false = production mode
		t.Fatalf("Failed to generate app: %v", err)
	}

	appDir := "testapp"

	// Check app files
	expectedAppFiles := []string{
		"go.mod",
		"README.md",
		"cmd/testapp/main.go",
		"internal/database/db.go",
		"internal/database/schema.sql",
		"internal/database/queries.sql",
		"internal/database/sqlc.yaml",
	}

	for _, file := range expectedAppFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", path)
		}
	}

	// Generate resource
	fields := []parser.Field{
		{Name: "title", Type: "string", GoType: "string", SQLType: "TEXT"},
	}

	if err := generator.GenerateResource(appDir, "testapp", "Post", fields, "multi", "tailwind", "infinite", 20, "modal"); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Check resource files
	expectedResourceFiles := []string{
		"internal/app/post/post.go",
		"internal/app/post/post.tmpl",
		"internal/app/post/post_test.go",
	}

	for _, file := range expectedResourceFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected resource file not found: %s", path)
		}
	}

	// Generate view
	if err := generator.GenerateView(appDir, "testapp", "Dashboard", "multi", "tailwind"); err != nil {
		t.Fatalf("Failed to generate view: %v", err)
	}

	// Check view files
	expectedViewFiles := []string{
		"internal/app/dashboard/dashboard.go",
		"internal/app/dashboard/dashboard.tmpl",
		"internal/app/dashboard/dashboard_test.go",
	}

	for _, file := range expectedViewFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected view file not found: %s", path)
		}
	}

	t.Log("✅ All expected files generated")
}

// TestForeignKeyGeneration validates that foreign keys are properly generated
func TestForeignKeyGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database directory structure
	dbDir := filepath.Join(tmpDir, "internal", "database", "migrations")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create database directory: %v", err)
	}

	// Generate parent resource (posts)
	parentFields := []parser.Field{
		{Name: "title", Type: "string", GoType: "string", SQLType: "TEXT"},
		{Name: "content", Type: "string", GoType: "string", SQLType: "TEXT"},
	}

	if err := generator.GenerateResource(tmpDir, "testmodule", "Post", parentFields, "multi", "tailwind", "infinite", 20, "modal"); err != nil {
		t.Fatalf("Failed to generate parent resource: %v", err)
	}

	// Generate child resource with foreign key
	childFields := []parser.Field{
		{
			Name:            "post_id",
			Type:            "references:posts",
			GoType:          "string",
			SQLType:         "TEXT",
			IsReference:     true,
			ReferencedTable: "posts",
			OnDelete:        "CASCADE",
		},
		{Name: "author", Type: "string", GoType: "string", SQLType: "TEXT"},
		{Name: "text", Type: "string", GoType: "string", SQLType: "TEXT"},
	}

	if err := generator.GenerateResource(tmpDir, "testmodule", "Comment", childFields, "multi", "tailwind", "infinite", 20, "modal"); err != nil {
		t.Fatalf("Failed to generate child resource: %v", err)
	}

	// Read the generated migration file for comments
	migrationsDir := filepath.Join(tmpDir, "internal", "database", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("Failed to read migrations directory: %v", err)
	}

	var commentsMigration string
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "comments") {
			data, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
			if err != nil {
				t.Fatalf("Failed to read migration file: %v", err)
			}
			commentsMigration = string(data)
			break
		}
	}

	if commentsMigration == "" {
		t.Fatal("Comments migration file not found")
	}

	// Validate that foreign key is defined inline (not using ALTER TABLE)
	if strings.Contains(commentsMigration, "ALTER TABLE") && strings.Contains(commentsMigration, "ADD CONSTRAINT") {
		t.Errorf("Migration uses ALTER TABLE ADD CONSTRAINT which is not supported in SQLite")
	}

	// Validate that inline FOREIGN KEY definition exists
	if !strings.Contains(commentsMigration, "FOREIGN KEY") {
		t.Errorf("Migration missing FOREIGN KEY definition")
	}

	// Validate that FOREIGN KEY references the correct table
	if !strings.Contains(commentsMigration, "REFERENCES posts(id)") {
		t.Errorf("FOREIGN KEY does not reference posts(id)")
	}

	// Validate ON DELETE CASCADE
	if !strings.Contains(commentsMigration, "ON DELETE CASCADE") {
		t.Errorf("FOREIGN KEY missing ON DELETE CASCADE")
	}

	// Validate that index is created for foreign key column
	if !strings.Contains(commentsMigration, "idx_comments_post_id") {
		t.Errorf("Index not created for foreign key column post_id")
	}

	// Check schema.sql as well
	schemaPath := filepath.Join(tmpDir, "internal", "database", "schema.sql")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}
	schema := string(schemaData)

	// Validate schema also has inline FOREIGN KEY
	if !strings.Contains(schema, "FOREIGN KEY (post_id) REFERENCES posts(id)") {
		t.Errorf("schema.sql missing inline FOREIGN KEY definition")
	}

	t.Log("✅ Foreign key generation test passed")
	t.Logf("Migration content:\n%s", commentsMigration)
}
