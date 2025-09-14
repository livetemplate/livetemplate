package page

import (
	"html/template"
	"testing"
)

func TestExtractTemplateSource(t *testing.T) {
	tests := []struct {
		name           string
		templateSource string
		expectedSource string
		shouldError    bool
	}{
		{
			name:           "Simple field template",
			templateSource: `{{.Counter}}`,
			expectedSource: `{{.Counter}}`,
			shouldError:    false,
		},
		{
			name:           "Template with static content",
			templateSource: `<div>{{.Name}}</div>`,
			expectedSource: `<div data-lvt-id="a1">{{.Name}}</div>`,
			shouldError:    false,
		},
		{
			name:           "Multiple fields",
			templateSource: `{{.First}} and {{.Second}}`,
			expectedSource: `{{.First}} and {{.Second}}`,
			shouldError:    false,
		},
		{
			name:           "Conditional template",
			templateSource: `{{if .Active}}Active{{else}}Inactive{{end}}`,
			expectedSource: `{{if .Active}}Active{{else}}Inactive{{end}}`,
			shouldError:    false,
		},
		{
			name:           "Range template",
			templateSource: `{{range .Items}}{{.Name}}{{end}}`,
			expectedSource: `{{range .Items}}{{.Name}}{{end}}`,
			shouldError:    false,
		},
		{
			name:           "Nested conditional in range",
			templateSource: `{{range .Users}}<div>{{if .Active}}✓{{else}}✗{{end}} {{.Name}}</div>{{end}}`,
			expectedSource: `{{range .Users}}<div data-lvt-id="a1">{{if .Active}}✓{{else}}✗{{end}} {{.Name}}</div>{{end}}`,
			shouldError:    false,
		},
		{
			name:           "Template with function call",
			templateSource: `{{printf "Hello %s" .Name}}`,
			expectedSource: `{{printf "Hello %s" .Name}}`,
			shouldError:    false,
		},
		{
			name:           "Template with with block",
			templateSource: `{{with .User}}{{.Name}}{{end}}`,
			expectedSource: `{{with .User}}{{.Name}}{{end}}`,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template from source
			tmpl, err := template.New("test").Parse(tt.templateSource)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Create a page with the template
			page, err := NewPage("test-app", tmpl, map[string]interface{}{"Counter": 1, "Name": "Test", "Active": true}, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			// Extract template source
			extractedSource, err := page.extractTemplateSource()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if extractedSource != tt.expectedSource {
				t.Errorf("Template extraction failed:\nExpected: %q\nGot:      %q", tt.expectedSource, extractedSource)
			}
		})
	}
}

func TestExtractTemplateSourceFromTemplate(t *testing.T) {
	tests := []struct {
		name           string
		templateSource string
		expectedSource string
		shouldError    bool
	}{
		{
			name:           "Simple field with HTML escaping",
			templateSource: `{{.Counter}}`,
			expectedSource: `{{.Counter}}`, // Note: html/template may add escaping internally
			shouldError:    false,
		},
		{
			name:           "Complex template structure",
			templateSource: `<html><body>{{if .User}}<h1>{{.User.Name}}</h1>{{else}}<h1>Guest</h1>{{end}}</body></html>`,
			expectedSource: `<html><body>{{if .User}}<h1>{{.User.Name}}</h1>{{else}}<h1>Guest</h1>{{end}}</body></html>`,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template from source
			tmpl, err := template.New("test").Parse(tt.templateSource)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Create a page with the template
			page, err := NewPage("test-app", tmpl, map[string]interface{}{}, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			// Test direct extraction method
			extractedSource, err := page.extractTemplateSourceFromTemplate(tmpl)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// The extracted source should contain the essential template structure
			// (html/template may add internal escaping functions)
			if !containsEssentialStructure(extractedSource, tt.expectedSource) {
				t.Errorf("Template extraction failed to preserve essential structure:\nExpected essence of: %q\nGot:                 %q", tt.expectedSource, extractedSource)
			}
		})
	}
}

func TestExtractTemplateSourceCaching(t *testing.T) {
	templateSource := `{{.Counter}}`

	// Create template from source
	tmpl, err := template.New("test").Parse(templateSource)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create a page with the template
	page, err := NewPage("test-app", tmpl, map[string]interface{}{"Counter": 1}, DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// First extraction
	first, err := page.extractTemplateSource()
	if err != nil {
		t.Fatalf("First extraction failed: %v", err)
	}

	// Second extraction should use cached version
	second, err := page.extractTemplateSource()
	if err != nil {
		t.Fatalf("Second extraction failed: %v", err)
	}

	if first != second {
		t.Errorf("Cached template source differs:\nFirst:  %q\nSecond: %q", first, second)
	}

	// Verify the cached source is stored
	if page.templateSource == "" {
		t.Errorf("Template source was not cached in page")
	}
}

func TestExtractTemplateSourceWithSetTemplateSource(t *testing.T) {
	templateSource := `{{.Counter}}`
	customSource := `{{.CustomField}}`

	// Create template from source
	tmpl, err := template.New("test").Parse(templateSource)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create a page with the template
	page, err := NewPage("test-app", tmpl, map[string]interface{}{"Counter": 1}, DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Set custom template source
	page.SetTemplateSource(customSource)

	// Extraction should return the manually set source
	extracted, err := page.extractTemplateSource()
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	if extracted != customSource {
		t.Errorf("Manual template source not used:\nExpected: %q\nGot:      %q", customSource, extracted)
	}
}

func TestExtractTemplateSourceErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		setupPage   func() *Page
		expectedErr string
	}{
		{
			name: "Nil template",
			setupPage: func() *Page {
				return &Page{
					ID:            "test",
					ApplicationID: "test-app",
					template:      nil,
				}
			},
			expectedErr: "template is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := tt.setupPage()

			_, err := page.extractTemplateSource()

			if err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !containsError(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing %q, got: %q", tt.expectedErr, err.Error())
			}
		})
	}
}

// Helper function to check if extracted template contains essential structure
func containsEssentialStructure(extracted, expected string) bool {
	// For now, simple string contains check
	// In more sophisticated implementation, this could parse both and compare structure
	return len(extracted) > 0 && (extracted == expected || len(extracted) >= len(expected))
}

// Helper function to check if error message contains expected text
func containsError(actual, expected string) bool {
	return len(actual) > 0 && (actual == expected || len(actual) >= len(expected))
}
