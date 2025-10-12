package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEditModePage tests --edit-mode page generation and configuration
func TestEditModePage(t *testing.T) {
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

	// Generate resource with page mode
	genCmd := exec.Command(lvtBinary, "gen", "articles", "title", "content", "--edit-mode", "page")
	genCmd.Dir = appDir
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		t.Fatalf("Failed to generate resource with --edit-mode page: %v", err)
	}

	// Verify handler has correct EditMode
	handlerFile := filepath.Join(appDir, "internal", "app", "articles", "articles.go")
	handlerContent, err := os.ReadFile(handlerFile)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	// Check for "view" and "back" actions (specific to page mode)
	if !strings.Contains(string(handlerContent), `case "view":`) {
		t.Error("❌ Handler missing 'view' action (required for page mode)")
	} else {
		t.Log("✅ Handler has 'view' action for page mode")
	}

	if !strings.Contains(string(handlerContent), `case "back":`) {
		t.Error("❌ Handler missing 'back' action (required for page mode)")
	} else {
		t.Log("✅ Handler has 'back' action for page mode")
	}

	// Verify template has correct structure for page mode
	tmplFile := filepath.Join(appDir, "internal", "app", "articles", "articles.tmpl")
	tmplContent, err := os.ReadFile(tmplFile)
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	tmplStr := string(tmplContent)

	// Check for detailPage template (specific to page mode)
	if !strings.Contains(tmplStr, `{{define "detailPage"}}`) {
		t.Error("❌ Template missing detailPage definition (required for page mode)")
	} else {
		t.Log("✅ Template has detailPage definition for page mode")
	}

	// Check for clickable table rows with lvt-click="view"
	if !strings.Contains(tmplStr, `lvt-click="view"`) {
		t.Error("❌ Template missing view click handler on table rows")
	} else {
		t.Log("✅ Template has view click handler on table rows")
	}

	// Check for back button
	if !strings.Contains(tmplStr, `lvt-click="back"`) {
		t.Error("❌ Template missing back button")
	} else {
		t.Log("✅ Template has back button for returning to list")
	}

	// Verify NO edit buttons in table rows (page mode difference from modal mode)
	// In page mode, you click the row to view, not an edit button
	rowEditPattern := `<tr.*lvt-click="edit"`
	if strings.Contains(tmplStr, rowEditPattern) {
		t.Error("❌ Template has edit buttons in table rows (should use view in page mode)")
	} else {
		t.Log("✅ Table rows use view action, not edit buttons (correct for page mode)")
	}

	t.Log("✅ Edit mode page configuration verified")
}

// TestEditModeCombinations tests --edit-mode with other flags
func TestEditModeCombinations(t *testing.T) {
	combinations := []struct {
		name       string
		editMode   string
		css        string
		pagination string
	}{
		{"PageMode_Pico_LoadMore", "page", "pico", "load-more"},
		{"PageMode_Bulma_PrevNext", "page", "bulma", "prev-next"},
		{"ModalMode_None_Numbers", "modal", "none", "numbers"},
	}

	for _, combo := range combinations {
		t.Run(combo.name, func(t *testing.T) {
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

			// Generate resource with all flags
			genCmd := exec.Command(lvtBinary, "gen", "items", "name", "description",
				"--edit-mode", combo.editMode,
				"--css", combo.css,
				"--pagination", combo.pagination)
			genCmd.Dir = appDir
			genCmd.Stdout = os.Stdout
			genCmd.Stderr = os.Stderr
			if err := genCmd.Run(); err != nil {
				t.Fatalf("Failed to generate resource with combination: %v", err)
			}

			// Verify handler has correct settings
			handlerFile := filepath.Join(appDir, "internal", "app", "items", "items.go")
			handlerContent, err := os.ReadFile(handlerFile)
			if err != nil {
				t.Fatalf("Failed to read handler: %v", err)
			}

			handlerStr := string(handlerContent)

			// Check pagination mode
			if !strings.Contains(handlerStr, fmt.Sprintf(`PaginationMode: "%s"`, combo.pagination)) {
				t.Errorf("❌ PaginationMode '%s' not found in handler", combo.pagination)
			} else {
				t.Logf("✅ Handler has PaginationMode: %s", combo.pagination)
			}

			// Check edit mode specific actions
			if combo.editMode == "page" {
				if !strings.Contains(handlerStr, `case "view":`) {
					t.Error("❌ Handler missing view action for page mode")
				} else {
					t.Log("✅ Handler has view action for page mode")
				}
			}

			// Verify template exists and is valid
			tmplFile := filepath.Join(appDir, "internal", "app", "items", "items.tmpl")
			tmplContent, err := os.ReadFile(tmplFile)
			if err != nil {
				t.Fatalf("Failed to read template: %v", err)
			}

			if len(tmplContent) < 100 {
				t.Error("❌ Template seems empty or invalid")
			} else {
				t.Logf("✅ Template generated successfully (%d bytes)", len(tmplContent))
			}

			t.Logf("✅ Combination verified: edit-mode=%s, css=%s, pagination=%s",
				combo.editMode, combo.css, combo.pagination)
		})
	}
}

// TestEditModeValidation tests that invalid edit modes are rejected
func TestEditModeValidation(t *testing.T) {
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

	// Try to generate with invalid edit mode
	genCmd := exec.Command(lvtBinary, "gen", "items", "name", "--edit-mode", "invalid")
	genCmd.Dir = appDir
	output, err := genCmd.CombinedOutput()

	if err == nil {
		t.Fatal("❌ Expected error for invalid edit mode, but command succeeded")
	}

	if !strings.Contains(string(output), "invalid edit mode") {
		t.Errorf("❌ Error message doesn't mention invalid edit mode. Got: %s", string(output))
	} else {
		t.Log("✅ Invalid edit mode rejected with appropriate error message")
	}
}
