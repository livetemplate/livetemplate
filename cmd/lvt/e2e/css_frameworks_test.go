package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCSSFrameworks tests different CSS frameworks
func TestCSSFrameworks(t *testing.T) {
	frameworks := []string{"bulma", "pico", "none"}

	for _, framework := range frameworks {
		t.Run("CSS_"+framework, func(t *testing.T) {
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
			newCmd.Stdout = os.Stdout
			newCmd.Stderr = os.Stderr
			if err := newCmd.Run(); err != nil {
				t.Fatalf("Failed to create app: %v", err)
			}

			// Generate resource with specific CSS framework
			genCmd := exec.Command(lvtBinary, "gen", "items", "name", "--css", framework)
			genCmd.Dir = appDir
			genCmd.Stdout = os.Stdout
			genCmd.Stderr = os.Stderr
			if err := genCmd.Run(); err != nil {
				t.Fatalf("Failed to generate resource with --css %s: %v", framework, err)
			}

			// Verify template file exists
			tmplFile := filepath.Join(appDir, "internal", "app", "items", "items.tmpl")
			if _, err := os.Stat(tmplFile); err != nil {
				t.Fatalf("Template file not created: %v", err)
			}

			// Check for CSS framework-specific content
			content, err := os.ReadFile(tmplFile)
			if err != nil {
				t.Fatalf("Failed to read template: %v", err)
			}

			contentStr := string(content)
			switch framework {
			case "bulma":
				if !strings.Contains(contentStr, "button") {
					t.Error("❌ Bulma CSS classes not found in template")
				}
			case "pico":
				if !strings.Contains(contentStr, "button") {
					t.Error("❌ Pico CSS classes not found in template")
				}
			case "none":
				// Template should still be valid
				if len(contentStr) < 100 {
					t.Error("❌ Template seems empty or invalid")
				}
			}

			t.Logf("✅ Resource generated successfully with --css %s", framework)
		})
	}
}
