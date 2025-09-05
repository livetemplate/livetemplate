package page

import (
	"html/template"
	"testing"
)

func TestDetectTemplateRegions(t *testing.T) {
	tests := []struct {
		name            string
		templateSource  string
		expectedRegions int
		expectedIDs     []string
		expectedSources []string
	}{
		{
			name: "Single dynamic element with ID",
			templateSource: `<!DOCTYPE html>
<html>
<body>
	<h1>Counter App</h1>
	<div id="counter">Hello {{.Counter}}</div>
	<button>Click</button>
</body>
</html>`,
			expectedRegions: 1,
			expectedIDs:     []string{"counter"},
			expectedSources: []string{"Hello {{.Counter}}"},
		},
		{
			name: "Multiple dynamic elements",
			templateSource: `<html>
<body>
	<div id="name">Welcome {{.User.Name}}</div>
	<div id="status">Status: {{.Status}}</div>
	<span>Static content</span>
</body>
</html>`,
			expectedRegions: 2,
			expectedIDs:     []string{"name", "status"},
			expectedSources: []string{"Welcome {{.User.Name}}", "Status: {{.Status}}"},
		},
		{
			name: "Element without ID gets generated ID",
			templateSource: `<html>
<body>
	<span>Count: {{.Counter}}</span>
</body>
</html>`,
			expectedRegions: 1,
			expectedIDs:     []string{"region_0"},
			expectedSources: []string{"Count: {{.Counter}}"},
		},
		{
			name: "No dynamic elements",
			templateSource: `<html>
<body>
	<h1>Static Page</h1>
	<p>No dynamic content here</p>
</body>
</html>`,
			expectedRegions: 0,
			expectedIDs:     []string{},
			expectedSources: []string{},
		},
		{
			name: "Complex nested template expressions",
			templateSource: `<html>
<body>
	<div id="user-info">{{.User.Name}} ({{if .User.Active}}Active{{else}}Inactive{{end}})</div>
	<div id="items">Items: {{range .Items}}{{.Name}} {{end}}</div>
</body>
</html>`,
			expectedRegions: 2,
			expectedIDs:     []string{"user-info", "items"},
			expectedSources: []string{
				"{{.User.Name}} ({{if .User.Active}}Active{{else}}Inactive{{end}})",
				"Items: {{range .Items}}{{.Name}} {{end}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a template from the source
			tmpl, err := template.New("test").Parse(tt.templateSource)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Create a page
			page, err := NewPage("test-app", tmpl, map[string]interface{}{}, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			// Detect regions
			regions, err := page.detectTemplateRegions()
			if err != nil {
				t.Fatalf("Failed to detect regions: %v", err)
			}

			// Verify number of regions
			if len(regions) != tt.expectedRegions {
				t.Errorf("Expected %d regions, got %d", tt.expectedRegions, len(regions))
			}

			// Verify region details
			for i, region := range regions {
				if i >= len(tt.expectedIDs) {
					t.Errorf("Unexpected region %d: %+v", i, region)
					continue
				}

				if region.ID != tt.expectedIDs[i] {
					t.Errorf("Region %d: expected ID %q, got %q", i, tt.expectedIDs[i], region.ID)
				}

				if region.TemplateSource != tt.expectedSources[i] {
					t.Errorf("Region %d: expected template source %q, got %q", i, tt.expectedSources[i], region.TemplateSource)
				}

				// Verify field paths are extracted
				if len(region.FieldPaths) == 0 && containsTemplateExpression(region.TemplateSource) {
					t.Errorf("Region %d: expected field paths to be extracted from %q", i, region.TemplateSource)
				}
			}
		})
	}
}

func TestExtractIDFromTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "Double quotes",
			tag:      `div id="counter" class="widget"`,
			expected: "counter",
		},
		{
			name:     "Single quotes",
			tag:      `div id='status' class='info'`,
			expected: "status",
		},
		{
			name:     "No ID attribute",
			tag:      `div class="widget"`,
			expected: "",
		},
		{
			name:     "ID at end",
			tag:      `span class="text" id="label"`,
			expected: "label",
		},
		{
			name:     "Empty ID",
			tag:      `div id=""`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIDFromTag(tt.tag)
			if result != tt.expected {
				t.Errorf("extractIDFromTag(%q) = %q, expected %q", tt.tag, result, tt.expected)
			}
		})
	}
}

func TestExtractFieldPaths(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Single field",
			content:  "Hello {{.Name}}",
			expected: []string{".Name"},
		},
		{
			name:     "Multiple fields",
			content:  "{{.First}} and {{.Second}}",
			expected: []string{".First", ".Second"},
		},
		{
			name:     "Nested fields",
			content:  "Welcome {{.User.Name}} from {{.User.Location}}",
			expected: []string{".User", ".User"},
		},
		{
			name:     "Complex expressions",
			content:  "{{if .Active}}{{.Name}}{{else}}Guest{{end}}",
			expected: []string{".Active", ".Name"},
		},
		{
			name:     "Range expressions",
			content:  "{{range .Items}}{{.Name}} {{end}}",
			expected: []string{".Items", ".Name"},
		},
		{
			name:     "No template expressions",
			content:  "Static content only",
			expected: []string{},
		},
		{
			name:     "Duplicate fields",
			content:  "{{.Name}} - {{.Name}} again",
			expected: []string{".Name"}, // Should deduplicate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFieldPaths(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("extractFieldPaths(%q) = %v, expected %v", tt.content, result, tt.expected)
				return
			}

			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("extractFieldPaths(%q) = %v, expected %v", tt.content, result, tt.expected)
					break
				}
			}
		})
	}
}

func TestGenerateRegionFragment(t *testing.T) {
	tests := []struct {
		name        string
		region      TemplateRegion
		oldData     interface{}
		newData     interface{}
		shouldError bool
		expectedID  string
	}{
		{
			name: "Simple region fragment",
			region: TemplateRegion{
				ID:             "counter",
				TemplateSource: "Hello {{.Counter}}",
				FieldPaths:     []string{".Counter"},
			},
			oldData:     map[string]interface{}{"Counter": 1},
			newData:     map[string]interface{}{"Counter": 2},
			shouldError: false,
			expectedID:  "counter",
		},
		{
			name: "Complex region fragment",
			region: TemplateRegion{
				ID:             "user-status",
				TemplateSource: "{{if .Active}}{{.Name}} is online{{else}}Offline{{end}}",
				FieldPaths:     []string{".Active", ".Name"},
			},
			oldData:     map[string]interface{}{"Active": false, "Name": "Alice"},
			newData:     map[string]interface{}{"Active": true, "Name": "Alice"},
			shouldError: false,
			expectedID:  "user-status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple page for testing
			tmpl := template.Must(template.New("test").Parse(tt.region.TemplateSource))
			page, err := NewPage("test-app", tmpl, tt.oldData, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			// Generate region fragment
			fragment, err := page.generateRegionFragment(tt.region, tt.newData)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if fragment.ID != tt.expectedID {
				t.Errorf("Expected fragment ID %q, got %q", tt.expectedID, fragment.ID)
			}

			// Strategy and Action fields removed - verify basic fragment structure
			if fragment.Data == nil {
				t.Error("Expected fragment with data")
			}

			if fragment.Data == nil {
				t.Errorf("Expected fragment data to be non-nil")
			}
		})
	}
}

// Helper function to check if content contains template expressions
func containsTemplateExpression(content string) bool {
	return len(content) > 4 && (content[0:2] == "{{" || (len(content) > 6 && content[0:6] == "{{if ") ||
		(len(content) > 8 && content[0:8] == "{{range "))
}
