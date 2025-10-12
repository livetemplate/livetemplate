package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestViewGeneration tests view-only generation
func TestViewGeneration(t *testing.T) {
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

	// Generate view
	genCmd := exec.Command(lvtBinary, "gen", "view", "dashboard")
	genCmd.Dir = appDir
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		t.Fatalf("Failed to generate view: %v", err)
	}

	// Verify files exist
	handlerFile := filepath.Join(appDir, "internal", "app", "dashboard", "dashboard.go")
	tmplFile := filepath.Join(appDir, "internal", "app", "dashboard", "dashboard.tmpl")
	testFile := filepath.Join(appDir, "internal", "app", "dashboard", "dashboard_test.go")

	for _, file := range []string{handlerFile, tmplFile, testFile} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("❌ Expected file not created: %s", file)
		}
	}

	// Verify handler doesn't have CRUD operations
	content, err := os.ReadFile(handlerFile)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "PaginationMode") {
		t.Error("❌ View handler should not have pagination")
	}
	if strings.Contains(contentStr, "handleAdd") {
		t.Error("❌ View handler should not have CRUD operations")
	}

	t.Log("✅ View-only handler generated successfully")
}
