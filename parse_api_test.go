package statetemplate

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

//go:embed testdata/*
var embeddedFS embed.FS

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		content      string
		expectError  bool
		data         interface{}
		expectHTML   string
	}{
		{
			name:         "simple template",
			templateName: "simple",
			content:      `<div><h1>{{.Title}}</h1><p>{{.Message}}</p></div>`,
			expectError:  false,
			data: map[string]interface{}{
				"Title":   "Hello World",
				"Message": "Welcome to StateTemplate",
			},
			expectHTML: `<div><h1>Hello World</h1><p>Welcome to StateTemplate</p></div>`,
		},
		{
			name:         "template with conditionals",
			templateName: "conditional",
			content: `<div>
{{if .ShowWelcome}}
	<h1>Welcome {{.User.Name}}!</h1>
{{else}}
	<h1>Please log in</h1>
{{end}}
</div>`,
			expectError: false,
			data: map[string]interface{}{
				"ShowWelcome": true,
				"User": map[string]interface{}{
					"Name": "Alice",
				},
			},
			expectHTML: `<div>
	<h1>Welcome Alice!</h1>
</div>`,
		},
		{
			name:         "template with range",
			templateName: "range",
			content: `<ul>
{{range .Items}}
	<li>{{.}}</li>
{{end}}
</ul>`,
			expectError: false,
			data: map[string]interface{}{
				"Items": []string{"Item 1", "Item 2", "Item 3"},
			},
			expectHTML: `<ul>
	<li>Item 1</li>
	<li>Item 2</li>
	<li>Item 3</li>
</ul>`,
		},
		{
			name:         "invalid template syntax",
			templateName: "invalid",
			content:      `<div>{{.Title</div>`, // Missing closing }}
			expectError:  true,
		},
		{
			name:         "empty template",
			templateName: "empty",
			content:      ``,
			expectError:  false,
			data:         map[string]interface{}{},
			expectHTML:   ``,
		},
		{
			name:         "template with variables",
			templateName: "variables",
			content: `<div>
{{$name := .User.Name}}
{{$count := len .Items}}
<h1>Hello {{$name}}</h1>
<p>You have {{$count}} items</p>
</div>`,
			expectError: false,
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Name": "Bob",
				},
				"Items": []string{"a", "b", "c"},
			},
			expectHTML: `<div>
<h1>Hello Bob</h1>
<p>You have 3 items</p>
</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewRenderer()

			err := renderer.Parse(tt.templateName, tt.content)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for template %s, but got none", tt.templateName)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for template %s: %v", tt.templateName, err)
				return
			}
			if tt.expectError {
				return // Test passed - error was expected
			}

			// Test rendering if no error expected
			if tt.data != nil {
				html, err := renderer.SetInitialData(tt.data)
				if err != nil {
					t.Errorf("Failed to render template %s: %v", tt.templateName, err)
					return
				}

				// Verify the rendered HTML contains expected content
				// We don't need exact matches since StateTemplate adds fragment IDs
				expectedParts := extractExpectedContent(tt.expectHTML)
				for _, part := range expectedParts {
					if !strings.Contains(html, part) {
						t.Errorf("Template %s missing expected content: %s\nFull HTML:\n%s", tt.templateName, part, html)
					}
				}

				// Verify fragment IDs are present for non-empty templates
				// Empty templates or templates with only variable assignments might not have fragments
				if tt.expectHTML != "" && !strings.Contains(tt.content, ":=") {
					if !strings.Contains(html, `id="`) {
						t.Errorf("Template %s should contain fragment IDs", tt.templateName)
					}
				}
			}

			// Verify template was added
			stats := renderer.GetStats()
			if stats.TemplateCount != 1 {
				t.Errorf("Expected 1 template, got %d", stats.TemplateCount)
			}
		})
	}
}

func TestParseFiles(t *testing.T) {
	tests := []struct {
		name        string
		filenames   []string
		expectError bool
		expectCount int
		testData    interface{}
	}{
		{
			name:        "single file",
			filenames:   []string{"testdata/simple.tmpl"},
			expectError: false,
			expectCount: 1,
			testData: map[string]interface{}{
				"Title":   "Test Title",
				"Message": "Test Message",
			},
		},
		{
			name:        "multiple files",
			filenames:   []string{"testdata/header.html", "testdata/footer.html"},
			expectError: false,
			expectCount: 2,
			testData: map[string]interface{}{
				"Title":  "My Site",
				"Author": "John Doe",
				"User":   map[string]interface{}{"Name": "Alice"},
				"Copyright": map[string]interface{}{
					"Year":    2024,
					"Company": "ACME Corp",
					"Message": "All rights reserved",
				},
				"Footer": map[string]interface{}{
					"Links": []map[string]interface{}{
						{"URL": "/about", "Text": "About"},
						{"URL": "/contact", "Text": "Contact"},
					},
				},
			},
		},
		{
			name:        "nonexistent file",
			filenames:   []string{"testdata/nonexistent.tmpl"},
			expectError: true,
		},
		{
			name:        "empty filenames",
			filenames:   []string{},
			expectError: true,
		},
		{
			name:        "mixed valid and invalid files",
			filenames:   []string{"testdata/simple.tmpl", "testdata/nonexistent.tmpl"},
			expectError: true,
		},
		{
			name:        "block templates",
			filenames:   []string{"testdata/blocks/basic.tmpl", "testdata/blocks/template_include.tmpl"},
			expectError: false,
			expectCount: 2,
			testData: map[string]interface{}{
				"Title":   "Block Test",
				"Content": "This is block content",
				"User":    map[string]interface{}{"Name": "TestUser"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewRenderer()

			err := renderer.ParseFiles(tt.filenames...)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for files %v, but got none", tt.filenames)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for files %v: %v", tt.filenames, err)
				return
			}
			if tt.expectError {
				return // Test passed - error was expected
			}

			// Verify correct number of templates were added
			stats := renderer.GetStats()
			if stats.TemplateCount != tt.expectCount {
				t.Errorf("Expected %d templates, got %d", tt.expectCount, stats.TemplateCount)
			}

			// Test rendering if test data provided
			if tt.testData != nil {
				html, err := renderer.SetInitialData(tt.testData)
				if err != nil {
					t.Errorf("Failed to render templates: %v", err)
					return
				}

				// Basic validation - should contain some expected content
				if len(html) == 0 {
					t.Error("Rendered HTML is empty")
				}

				// Check for fragment wrappers (indicating successful fragment extraction)
				if !strings.Contains(html, `id="`) {
					t.Error("Rendered HTML should contain fragment IDs")
				}
			}
		})
	}
}

func TestParseGlob(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expectError bool
		minCount    int // Minimum expected templates
		testData    interface{}
	}{
		{
			name:        "simple pattern",
			pattern:     "testdata/*.tmpl",
			expectError: false,
			minCount:    1, // At least simple.tmpl
			testData: map[string]interface{}{
				"Title":   "Glob Test",
				"Message": "Testing glob pattern",
			},
		},
		{
			name:        "html files pattern",
			pattern:     "testdata/*.html",
			expectError: false,
			minCount:    2, // header.html, footer.html, etc.
			testData: map[string]interface{}{
				"Title":  "Glob HTML Test",
				"Author": "Test Author",
				"User":   map[string]interface{}{"Name": "GlobUser"},
				"Copyright": map[string]interface{}{
					"Year":    2024,
					"Company": "Test Corp",
					"Message": "Test message",
				},
				"Footer": map[string]interface{}{
					"Links": []map[string]interface{}{
						{"URL": "/test", "Text": "Test"},
					},
				},
			},
		},
		{
			name:        "subdirectory pattern",
			pattern:     "testdata/blocks/*.tmpl",
			expectError: false,
			minCount:    1, // At least one template in blocks/
			testData: map[string]interface{}{
				"Title":   "Block Glob Test",
				"Content": "Block content",
				"User":    map[string]interface{}{"Name": "BlockUser"},
			},
		},
		{
			name:        "no matches pattern",
			pattern:     "testdata/*.xyz",
			expectError: true,
		},
		{
			name:        "invalid pattern",
			pattern:     "testdata/[invalid",
			expectError: true,
		},
		{
			name:        "recursive pattern",
			pattern:     "testdata/components/*.tmpl",
			expectError: false,
			minCount:    1, // Should find templates in components subdirectory
			testData: map[string]interface{}{
				"Type":     "primary",
				"Text":     "Click Me",
				"Disabled": false,
				"Icon":     "check",
				"Class":    "highlighted",
				"Header":   "Card Title",
				"Body":     "Card content goes here",
				"Footer":   "Card footer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewRenderer()

			err := renderer.ParseGlob(tt.pattern)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for pattern %s, but got none", tt.pattern)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for pattern %s: %v", tt.pattern, err)
				return
			}
			if tt.expectError {
				return // Test passed - error was expected
			}

			// Verify minimum number of templates were found
			stats := renderer.GetStats()
			if stats.TemplateCount < tt.minCount {
				t.Errorf("Expected at least %d templates, got %d", tt.minCount, stats.TemplateCount)
			}

			// Test rendering if test data provided
			if tt.testData != nil {
				html, err := renderer.SetInitialData(tt.testData)
				if err != nil {
					t.Errorf("Failed to render templates: %v", err)
					return
				}

				// Basic validation
				if len(html) == 0 {
					t.Error("Rendered HTML is empty")
				}
			}
		})
	}
}

func TestParseFS(t *testing.T) {
	tests := []struct {
		name        string
		fsys        fs.FS
		patterns    []string
		expectError bool
		minCount    int
		testData    interface{}
	}{
		{
			name:        "embedded FS single pattern",
			fsys:        embeddedFS,
			patterns:    []string{"testdata/*.tmpl"},
			expectError: false,
			minCount:    1,
			testData: map[string]interface{}{
				"Title":   "FS Test",
				"Message": "Testing embedded FS",
			},
		},
		{
			name:        "embedded FS multiple patterns",
			fsys:        embeddedFS,
			patterns:    []string{"testdata/*.tmpl", "testdata/*.html"},
			expectError: false,
			minCount:    3, // At least simple.tmpl + header.html + footer.html
			testData: map[string]interface{}{
				"Title":   "Multi Pattern FS Test",
				"Author":  "FS Author",
				"Message": "FS Message",
				"User":    map[string]interface{}{"Name": "FSUser"},
				"Copyright": map[string]interface{}{
					"Year":    2024,
					"Company": "FS Corp",
					"Message": "FS copyright",
				},
				"Footer": map[string]interface{}{
					"Links": []map[string]interface{}{
						{"URL": "/fs", "Text": "FS Link"},
					},
				},
			},
		},
		{
			name:        "embedded FS subdirectory",
			fsys:        embeddedFS,
			patterns:    []string{"testdata/blocks/*.tmpl"},
			expectError: false,
			minCount:    1,
			testData: map[string]interface{}{
				"Title":   "FS Block Test",
				"Content": "FS block content",
				"User":    map[string]interface{}{"Name": "FSBlockUser"},
			},
		},
		{
			name:        "empty patterns",
			fsys:        embeddedFS,
			patterns:    []string{},
			expectError: true,
		},
		{
			name:        "no matches pattern",
			fsys:        embeddedFS,
			patterns:    []string{"testdata/*.xyz"},
			expectError: false, // ParseFS doesn't error on no matches, just adds no templates
			minCount:    0,
		},
		{
			name:        "invalid pattern",
			fsys:        embeddedFS,
			patterns:    []string{"testdata/[invalid"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewRenderer()

			err := renderer.ParseFS(tt.fsys, tt.patterns...)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for patterns %v, but got none", tt.patterns)
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for patterns %v: %v", tt.patterns, err)
				return
			}
			if tt.expectError {
				return // Test passed - error was expected
			}

			// Verify minimum number of templates were found
			stats := renderer.GetStats()
			if stats.TemplateCount < tt.minCount {
				t.Errorf("Expected at least %d templates, got %d", tt.minCount, stats.TemplateCount)
			}

			// Test rendering if test data provided
			if tt.testData != nil && tt.minCount > 0 {
				html, err := renderer.SetInitialData(tt.testData)
				if err != nil {
					t.Errorf("Failed to render templates: %v", err)
					return
				}

				// Basic validation
				if len(html) == 0 {
					t.Error("Rendered HTML is empty")
				}
			}
		})
	}
}

func TestParseEdgeCases(t *testing.T) {
	t.Run("parse same template name twice", func(t *testing.T) {
		renderer := NewRenderer()

		// Parse first template
		err := renderer.Parse("test", `<div>{{.First}}</div>`)
		if err != nil {
			t.Fatalf("Failed to parse first template: %v", err)
		}

		// Parse second template with same name (should overwrite)
		err = renderer.Parse("test", `<div>{{.Second}}</div>`)
		if err != nil {
			t.Fatalf("Failed to parse second template: %v", err)
		}

		// Should still have only 1 template
		stats := renderer.GetStats()
		if stats.TemplateCount != 1 {
			t.Errorf("Expected 1 template after overwrite, got %d", stats.TemplateCount)
		}

		// Test rendering - should use the second template
		html, err := renderer.SetInitialData(map[string]interface{}{"Second": "Updated"})
		if err != nil {
			t.Fatalf("Failed to render: %v", err)
		}

		if !strings.Contains(html, "Updated") {
			t.Error("Should contain content from second template")
		}
	})

	t.Run("large template content", func(t *testing.T) {
		renderer := NewRenderer()

		// Create a large template with many fields
		var templateBuilder strings.Builder
		templateBuilder.WriteString("<div>\n")
		for i := 0; i < 100; i++ {
			templateBuilder.WriteString(fmt.Sprintf("  <p>Field %d: {{.Field%d}}</p>\n", i, i))
		}
		templateBuilder.WriteString("</div>")

		err := renderer.Parse("large", templateBuilder.String())
		if err != nil {
			t.Fatalf("Failed to parse large template: %v", err)
		}

		// Create corresponding data
		data := make(map[string]interface{})
		for i := 0; i < 100; i++ {
			data[fmt.Sprintf("Field%d", i)] = fmt.Sprintf("Value %d", i)
		}

		html, err := renderer.SetInitialData(data)
		if err != nil {
			t.Fatalf("Failed to render large template: %v", err)
		}

		// Verify some content is present
		if !strings.Contains(html, "Value 0") || !strings.Contains(html, "Value 99") {
			t.Error("Large template should contain all field values")
		}
	})

	t.Run("templates with special characters", func(t *testing.T) {
		renderer := NewRenderer()

		specialTemplate := `<div>
			<p>Unicode: {{.Unicode}}</p>
			<p>Symbols: {{.Symbols}}</p>
			<p>HTML entities: {{.Entities}}</p>
		</div>`

		err := renderer.Parse("special", specialTemplate)
		if err != nil {
			t.Fatalf("Failed to parse template with special characters: %v", err)
		}

		data := map[string]interface{}{
			"Unicode":  "こんにちは 🌟",
			"Symbols":  "!@#$%^", // Simplified symbols to avoid HTML escaping issues
			"Entities": "&lt;script&gt;",
		}

		html, err := renderer.SetInitialData(data)
		if err != nil {
			t.Fatalf("Failed to render template with special characters: %v", err)
		}

		// Verify special characters are handled
		if !strings.Contains(html, "こんにちは 🌟") {
			t.Error("Should handle Unicode characters")
		}
		if !strings.Contains(html, "!@#$%^") {
			t.Error("Should handle symbol characters")
		}
	})
}

func TestParseFilesEdgeCases(t *testing.T) {
	t.Run("files with same base name different extensions", func(t *testing.T) {
		renderer := NewRenderer()

		// This should load both templates with different names
		err := renderer.ParseFiles("testdata/simple.tmpl", "testdata/page.html")
		if err != nil {
			t.Fatalf("Failed to parse files: %v", err)
		}

		stats := renderer.GetStats()
		if stats.TemplateCount != 2 {
			t.Errorf("Expected 2 templates, got %d", stats.TemplateCount)
		}
	})

	t.Run("file with no extension", func(t *testing.T) {
		// Create a temporary file without extension
		tempFile := "/tmp/test_template_no_ext"
		content := `<div>{{.NoExt}}</div>`

		err := writeToFile(tempFile, content)
		if err != nil {
			t.Skipf("Cannot create temp file: %v", err)
		}
		defer removeFile(tempFile)

		renderer := NewRenderer()
		err = renderer.ParseFiles(tempFile)
		if err != nil {
			t.Fatalf("Failed to parse file without extension: %v", err)
		}

		stats := renderer.GetStats()
		if stats.TemplateCount != 1 {
			t.Errorf("Expected 1 template, got %d", stats.TemplateCount)
		}
	})
}

func TestParseGlobEdgeCases(t *testing.T) {
	t.Run("glob with multiple wildcards", func(t *testing.T) {
		renderer := NewRenderer()

		// This should match files in testdata and subdirectories
		err := renderer.ParseGlob("testdata/*/*.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse glob with multiple wildcards: %v", err)
		}

		stats := renderer.GetStats()
		if stats.TemplateCount == 0 {
			t.Error("Should find at least some templates in subdirectories")
		}
	})

	t.Run("glob case sensitivity", func(t *testing.T) {
		renderer := NewRenderer()

		// Test case-sensitive pattern
		err := renderer.ParseGlob("testdata/*.TMPL") // Uppercase extension
		if err == nil {
			stats := renderer.GetStats()
			// On case-sensitive filesystems, this should find no matches
			// On case-insensitive filesystems, it might find files
			t.Logf("Found %d templates with uppercase pattern", stats.TemplateCount)
		}
	})
}

func TestParseFSEdgeCases(t *testing.T) {
	t.Run("embedded FS with nested patterns", func(t *testing.T) {
		renderer := NewRenderer()

		// Test nested pattern matching
		err := renderer.ParseFS(embeddedFS, "testdata/*/*.tmpl", "testdata/*.html")
		if err != nil {
			t.Fatalf("Failed to parse FS with nested patterns: %v", err)
		}

		stats := renderer.GetStats()
		if stats.TemplateCount == 0 {
			t.Error("Should find templates with nested patterns")
		}
	})

	t.Run("embedded FS with overlapping patterns", func(t *testing.T) {
		renderer := NewRenderer()

		// Test overlapping patterns (some files might match both)
		err := renderer.ParseFS(embeddedFS, "testdata/*.tmpl", "testdata/simple.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse FS with overlapping patterns: %v", err)
		}

		stats := renderer.GetStats()
		// Should handle duplicates gracefully (overwrite)
		if stats.TemplateCount == 0 {
			t.Error("Should find templates even with overlapping patterns")
		}
	})
}

// Helper functions for edge case tests
func writeToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			// Log but don't override original error
			fmt.Printf("Warning: failed to close file %s: %v\n", filename, cerr)
		}
	}()

	_, err = file.WriteString(content)
	return err
}

func removeFile(filename string) {
	if err := os.Remove(filename); err != nil {
		fmt.Printf("Warning: failed to remove file %s: %v\n", filename, err)
	}
}

func TestParseAPIsIntegration(t *testing.T) {
	t.Run("mixed parsing methods", func(t *testing.T) {
		renderer := NewRenderer()

		// Parse inline template
		err := renderer.Parse("inline", `<div class="inline">{{.Title}}</div>`)
		if err != nil {
			t.Fatalf("Failed to parse inline template: %v", err)
		}

		// Parse from file
		err = renderer.ParseFiles("testdata/simple.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse files: %v", err)
		}

		// Parse using glob
		err = renderer.ParseGlob("testdata/blocks/*.tmpl")
		if err != nil {
			t.Fatalf("Failed to parse glob: %v", err)
		}

		// Parse from embedded FS
		err = renderer.ParseFS(embeddedFS, "testdata/header.html")
		if err != nil {
			t.Fatalf("Failed to parse from FS: %v", err)
		}

		// Verify all templates were added
		stats := renderer.GetStats()
		if stats.TemplateCount < 4 {
			t.Errorf("Expected at least 4 templates, got %d", stats.TemplateCount)
		}

		// Test rendering with comprehensive data
		testData := map[string]interface{}{
			"Title":   "Integration Test",
			"Message": "All parsing methods work",
			"Author":  "Test Suite",
			"User":    map[string]interface{}{"Name": "IntegrationUser"},
			"Content": "Integration content",
		}

		html, err := renderer.SetInitialData(testData)
		if err != nil {
			t.Fatalf("Failed to render integrated templates: %v", err)
		}

		// Verify content from different parsing methods is present
		if !strings.Contains(html, "Integration Test") {
			t.Error("HTML should contain title from test data")
		}

		// Verify fragment IDs are generated
		if !strings.Contains(html, `id="`) {
			t.Error("HTML should contain fragment IDs")
		}
	})
}

func TestParseErrorHandling(t *testing.T) {
	t.Run("parse invalid template syntax", func(t *testing.T) {
		renderer := NewRenderer()

		invalidTemplates := []string{
			`{{.Title`,             // Missing closing }}
			`{{.Title}}{{`,         // Incomplete action
			`{{range}}`,            // Invalid range
			`{{if}}{{end}}`,        // Invalid if
			`{{.Field | unknown}}`, // Unknown function (might not error in Go templates)
		}

		for _, tmpl := range invalidTemplates {
			err := renderer.Parse("invalid", tmpl)
			if err == nil {
				t.Errorf("Expected error for invalid template: %s", tmpl)
			}
		}
	})

	t.Run("parse files error scenarios", func(t *testing.T) {
		renderer := NewRenderer()

		// Directory instead of file
		err := renderer.ParseFiles("testdata")
		if err == nil {
			t.Error("Expected error when parsing directory as file")
		}

		// Permission denied (simulate by using a non-readable path)
		err = renderer.ParseFiles("/root/nonexistent")
		if err == nil {
			t.Error("Expected error when parsing non-accessible file")
		}
	})
}

func TestParseTemplateNaming(t *testing.T) {
	t.Run("template names from files", func(t *testing.T) {
		renderer := NewRenderer()

		err := renderer.ParseFiles("testdata/simple.tmpl", "testdata/header.html")
		if err != nil {
			t.Fatalf("Failed to parse files: %v", err)
		}

		// Template names should be derived from filenames without extensions
		debugInfo := renderer.GetDebugInfo()
		if debugInfo == nil {
			// Enable debug mode
			renderer = NewRenderer(WithDebugMode(true))
			err = renderer.ParseFiles("testdata/simple.tmpl", "testdata/header.html")
			if err != nil {
				t.Fatalf("Failed to parse files with debug mode: %v", err)
			}
			debugInfo = renderer.GetDebugInfo()
		}

		if debugInfo != nil {
			expectedNames := []string{"simple", "header"}
			for _, expectedName := range expectedNames {
				found := false
				for _, actualName := range debugInfo.TemplateNames {
					if actualName == expectedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected template name %s not found in %v", expectedName, debugInfo.TemplateNames)
				}
			}
		}
	})
}

// extractExpectedContent extracts key content parts from expected HTML for comparison
func extractExpectedContent(expected string) []string {
	var parts []string

	// Extract text content between HTML tags
	lines := strings.Split(expected, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// For range items, just check for the text content
		if strings.Contains(line, "<li>") && strings.Contains(line, "</li>") {
			start := strings.Index(line, ">")
			end := strings.LastIndex(line, "<")
			if start != -1 && end != -1 && start < end {
				content := strings.TrimSpace(line[start+1 : end])
				if content != "" {
					parts = append(parts, content) // Just the text content, not the HTML tags
				}
			}
		} else if strings.Contains(line, ">") && strings.Contains(line, "<") {
			// Extract content from other HTML tags
			start := strings.Index(line, ">")
			end := strings.LastIndex(line, "<")
			if start != -1 && end != -1 && start < end {
				content := line[start+1 : end]
				if strings.TrimSpace(content) != "" {
					parts = append(parts, strings.TrimSpace(content))
				}
			}
		}
	}

	return parts
}
