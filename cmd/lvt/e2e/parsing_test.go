package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParse_ValidTemplate tests parsing a valid template file
func TestParse_ValidTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create a valid template file
	templateContent := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
</head>
<body>
    <h1>{{.Heading}}</h1>
    {{range .Items}}
    <div class="item">
        <h2>{{.Name}}</h2>
        <p>{{.Description}}</p>
    </div>
    {{end}}

    {{if .ShowFooter}}
    <footer>Copyright 2025</footer>
    {{end}}
</body>
</html>`

	templateFile := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	// Parse the template
	t.Log("Parsing valid template...")
	if err := runLvtCommand(t, tmpDir, "parse", templateFile); err != nil {
		t.Fatalf("Failed to parse valid template: %v", err)
	}

	t.Log("✅ Valid template parsing test passed")
}

// TestParse_InvalidTemplate tests parsing an invalid template file
func TestParse_InvalidTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create an invalid template file (unclosed tag)
	templateContent := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
</head>
<body>
    <h1>{{.Heading</h1>
    {{range .Items}}
    <div class="item">
        <h2>{{.Name}}</h2>
    </div>
    {{end}}
</body>
</html>`

	templateFile := filepath.Join(tmpDir, "invalid.html")
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid template file: %v", err)
	}

	// Try to parse the invalid template (should fail)
	t.Log("Parsing invalid template (expecting failure)...")
	err := runLvtCommand(t, tmpDir, "parse", templateFile)

	// We expect this to fail
	if err == nil {
		t.Error("Expected parse to fail for invalid template, but it succeeded")
	} else {
		t.Logf("✅ Correctly detected invalid template: %v", err)
	}

	t.Log("✅ Invalid template parsing test passed")
}
