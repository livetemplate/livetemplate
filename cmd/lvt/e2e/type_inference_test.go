package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeInference tests field type inference
func TestTypeInference(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Build lvt
	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	// Create app
	newCmd := exec.Command(lvtBinary, "new", "testapp")
	newCmd.Dir = tmpDir
	if err := newCmd.Run(); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Generate resource with inferred types (no :type specified)
	genCmd := exec.Command(lvtBinary, "gen", "users", "name", "email", "age", "price", "published", "created_at")
	genCmd.Dir = appDir
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		t.Fatalf("Failed to generate resource with type inference: %v", err)
	}

	// Verify schema has correct inferred types
	schemaFile := filepath.Join(appDir, "internal", "database", "schema.sql")
	content, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("Failed to read schema: %v", err)
	}

	contentStr := string(content)

	// Check inferred types
	checks := map[string]string{
		"name":       "TEXT",     // string
		"email":      "TEXT",     // string
		"age":        "INTEGER",  // int
		"price":      "REAL",     // float
		"published":  "INTEGER",  // bool
		"created_at": "DATETIME", // time
	}

	for field, expectedType := range checks {
		if !strings.Contains(contentStr, field) || !strings.Contains(contentStr, expectedType) {
			t.Errorf("❌ Field '%s' not inferred as %s", field, expectedType)
		}
	}

	t.Log("✅ Type inference working correctly")
}
