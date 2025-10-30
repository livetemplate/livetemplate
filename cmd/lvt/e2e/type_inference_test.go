package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeInference tests field type inference
func TestTypeInference(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Create app
	if err := runLvtCommand(t, tmpDir, "new", "testapp"); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Generate resource with inferred types (no :type specified)
	if err := runLvtCommand(t, appDir, "gen", "resource", "users", "name", "email", "age", "price", "published", "created_at"); err != nil {
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
