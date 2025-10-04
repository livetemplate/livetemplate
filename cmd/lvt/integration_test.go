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

	if err := generator.GenerateResource(tmpDir, "testmodule", "User", fields); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Generate view
	if err := generator.GenerateView(tmpDir, "testmodule", "Counter"); err != nil {
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

	// Generate app
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	if err := generator.GenerateApp("testapp"); err != nil {
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

	if err := generator.GenerateResource(appDir, "testapp", "Post", fields); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Check resource files
	expectedResourceFiles := []string{
		"internal/app/post/post.go",
		"internal/app/post/post.tmpl",
		"internal/app/post/post_ws_test.go",
		"internal/app/post/post_test.go",
	}

	for _, file := range expectedResourceFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected resource file not found: %s", path)
		}
	}

	// Generate view
	if err := generator.GenerateView(appDir, "testapp", "Dashboard"); err != nil {
		t.Fatalf("Failed to generate view: %v", err)
	}

	// Check view files
	expectedViewFiles := []string{
		"internal/app/dashboard/dashboard.go",
		"internal/app/dashboard/dashboard.tmpl",
		"internal/app/dashboard/dashboard_ws_test.go",
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
