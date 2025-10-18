package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
	"github.com/livefir/livetemplate/cmd/lvt/internal/parser"
)

// TestResourceHandlerGolden validates handler generation against golden file
func TestResourceHandlerGolden(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database directory structure (required by GenerateResource)
	dbDir := filepath.Join(tmpDir, "internal", "database")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create database directory: %v", err)
	}

	fields := []parser.Field{
		{Name: "name", Type: "string", GoType: "string", SQLType: "TEXT"},
		{Name: "age", Type: "int", GoType: "int64", SQLType: "INTEGER"},
	}

	if err := generator.GenerateResource(tmpDir, "testmodule", "User", fields, "multi", "tailwind", "infinite", 20, "modal"); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Read generated handler
	handlerPath := filepath.Join(tmpDir, "internal", "app", "user", "user.go")
	generated, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read generated handler: %v", err)
	}

	goldenPath := "testdata/golden/resource_handler.go.golden"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		// Update golden file
		if err := os.MkdirAll("testdata/golden", 0755); err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, generated, 0644); err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Log("✅ Updated golden file")
		return
	}

	// Compare with golden file
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Golden file doesn't exist. Run with UPDATE_GOLDEN=1 to create it.")
		}
		t.Fatalf("Failed to read golden file: %v", err)
	}

	if string(golden) != string(generated) {
		// Show diff
		t.Errorf("Generated code differs from golden file.\n"+
			"Run with UPDATE_GOLDEN=1 to update.\n"+
			"Golden: %d bytes, Generated: %d bytes",
			len(golden), len(generated))

		// Show first difference
		goldenLines := strings.Split(string(golden), "\n")
		genLines := strings.Split(string(generated), "\n")

		for i := 0; i < len(goldenLines) && i < len(genLines); i++ {
			if goldenLines[i] != genLines[i] {
				t.Logf("First difference at line %d:", i+1)
				t.Logf("  Golden:    %s", goldenLines[i])
				t.Logf("  Generated: %s", genLines[i])
				break
			}
		}
	}
}

// TestViewHandlerGolden validates view generation against golden file
func TestViewHandlerGolden(t *testing.T) {
	tmpDir := t.TempDir()

	if err := generator.GenerateView(tmpDir, "testmodule", "Counter", "multi", "tailwind"); err != nil {
		t.Fatalf("Failed to generate view: %v", err)
	}

	// Read generated handler
	handlerPath := filepath.Join(tmpDir, "internal", "app", "counter", "counter.go")
	generated, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read generated handler: %v", err)
	}

	goldenPath := "testdata/golden/view_handler.go.golden"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		// Update golden file
		if err := os.MkdirAll("testdata/golden", 0755); err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, generated, 0644); err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Log("✅ Updated golden file")
		return
	}

	// Compare with golden file
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Golden file doesn't exist. Run with UPDATE_GOLDEN=1 to create it.")
		}
		t.Fatalf("Failed to read golden file: %v", err)
	}

	if string(golden) != string(generated) {
		t.Errorf("Generated code differs from golden file.\n"+
			"Run with UPDATE_GOLDEN=1 to update.\n"+
			"Golden: %d bytes, Generated: %d bytes",
			len(golden), len(generated))
	}
}

// TestResourceTemplateGolden validates template generation
func TestResourceTemplateGolden(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database directory structure (required by GenerateResource)
	dbDir := filepath.Join(tmpDir, "internal", "database")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create database directory: %v", err)
	}

	fields := []parser.Field{
		{Name: "title", Type: "string", GoType: "string", SQLType: "TEXT"},
		{Name: "published", Type: "bool", GoType: "bool", SQLType: "BOOLEAN"},
	}

	if err := generator.GenerateResource(tmpDir, "testmodule", "Post", fields, "multi", "tailwind", "prev-next", 10, "modal"); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Read generated template
	tmplPath := filepath.Join(tmpDir, "internal", "app", "post", "post.tmpl")
	generated, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("Failed to read generated template: %v", err)
	}

	goldenPath := "testdata/golden/resource_template.tmpl.golden"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata/golden", 0755); err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, generated, 0644); err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Log("✅ Updated golden file")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Golden file doesn't exist. Run with UPDATE_GOLDEN=1 to create it.")
		}
		t.Fatalf("Failed to read golden file: %v", err)
	}

	if string(golden) != string(generated) {
		t.Errorf("Generated template differs from golden file.\n" +
			"Run with UPDATE_GOLDEN=1 to update.")
	}
}
