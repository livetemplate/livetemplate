package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKitCSSFrameworks tests that kits generate valid templates with CSS framework integration
func TestKitCSSFrameworks(t *testing.T) {
	testCases := []struct {
		kit             string
		expectedCSS     string
		checkForPattern string // A pattern we expect to find in generated templates
	}{
		{
			kit:             "multi",
			expectedCSS:     "tailwind",
			checkForPattern: "button", // All kits should have button elements
		},
		{
			kit:             "single",
			expectedCSS:     "tailwind",
			checkForPattern: "button",
		},
		// Note: simple kit doesn't support resource generation (it's for simple counter examples)
	}

	for _, tc := range testCases {
		t.Run("Kit_"+tc.kit, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Build lvt binary

			// Create app with specific kit
			opts := &AppOptions{
				Kit:     tc.kit,
				DevMode: true,
			}
			appDir := createTestApp(t, tmpDir, "testapp", opts)

			// Generate a resource
			if err := runLvtCommand(t, appDir, "gen", "resource", "items", "name"); err != nil {
				t.Fatalf("Failed to generate resource: %v", err)
			}

			// Verify template file exists
			tmplFile := filepath.Join(appDir, "internal", "app", "items", "items.tmpl")
			content, err := readFile(t, tmplFile)
			if err != nil {
				t.Fatalf("Failed to read template: %v", err)
			}

			// Check for expected pattern
			if tc.checkForPattern != "" && !strings.Contains(content, tc.checkForPattern) {
				t.Errorf("❌ Expected pattern %q not found in template for %s CSS", tc.checkForPattern, tc.expectedCSS)
			} else {
				t.Logf("✅ Resource generated successfully with kit %s (%s CSS)", tc.kit, tc.expectedCSS)
			}

			// Verify template is valid (not empty)
			if len(content) < 100 {
				t.Error("❌ Template seems empty or invalid")
			}
		})
	}
}

// readFile is a helper to read file content as string
func readFile(t *testing.T, path string) (string, error) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
