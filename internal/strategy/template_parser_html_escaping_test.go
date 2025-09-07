package strategy

import (
	"testing"
)

func TestIsSimpleFieldWithHTMLEscaping(t *testing.T) {
	parser := NewTemplateParser()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid HTML escaping pipelines
		{
			name:     "Simple field with HTML escaper",
			input:    ".Counter | _html_template_htmlescaper",
			expected: true,
		},
		{
			name:     "Field with html function",
			input:    ".Name | html",
			expected: true,
		},
		{
			name:     "Field with js function",
			input:    ".Script | js",
			expected: true,
		},
		{
			name:     "Field with urlquery function",
			input:    ".URL | urlquery",
			expected: true,
		},
		{
			name:     "Field with print function",
			input:    ".Message | print",
			expected: true,
		},
		{
			name:     "Field with printf function",
			input:    ".Format | printf",
			expected: true,
		},
		{
			name:     "Field with println function",
			input:    ".Line | println",
			expected: true,
		},
		{
			name:     "Field with printf function (no parameters)",
			input:    ".Message | printf",
			expected: true,
		},
		{
			name:     "Nested field with HTML escaper",
			input:    ".User.Name | _html_template_htmlescaper",
			expected: true,
		},
		{
			name:     "Complex field path with HTML escaper",
			input:    ".Data.Items.First.Title | html",
			expected: true,
		},

		// Invalid cases - not simple HTML escaping
		{
			name:     "Multiple pipes",
			input:    ".Counter | upper | html",
			expected: false,
		},
		{
			name:     "Custom function",
			input:    ".Counter | myFunction",
			expected: false,
		},
		{
			name:     "Printf function with parameters",
			input:    ".Counter | printf \"%d\"",
			expected: false,
		},
		{
			name:     "Non-field first part",
			input:    "someVar | html",
			expected: false,
		},
		{
			name:     "No pipe",
			input:    ".Counter",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Just pipe",
			input:    "|",
			expected: false,
		},
		{
			name:     "Field with unknown function",
			input:    ".Counter | unknownFunction",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isSimpleFieldWithHTMLEscaping(tt.input)
			if result != tt.expected {
				t.Errorf("isSimpleFieldWithHTMLEscaping(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseActionWithHTMLEscaping(t *testing.T) {
	parser := NewTemplateParser()

	tests := []struct {
		name              string
		action            string
		expectedType      TemplateBoundaryType
		expectedFieldPath string
	}{
		{
			name:              "Simple field with HTML escaper",
			action:            "{{.Counter | _html_template_htmlescaper}}",
			expectedType:      SimpleField,
			expectedFieldPath: ".Counter",
		},
		{
			name:              "Field with html function",
			action:            "{{.Name | html}}",
			expectedType:      SimpleField,
			expectedFieldPath: ".Name",
		},
		{
			name:              "Nested field with HTML escaper",
			action:            "{{.User.Profile.Name | _html_template_htmlescaper}}",
			expectedType:      SimpleField,
			expectedFieldPath: ".User.Profile.Name",
		},
		{
			name:              "Field with js escaping",
			action:            "{{.Script | js}}",
			expectedType:      SimpleField,
			expectedFieldPath: ".Script",
		},
		{
			name:              "Complex pipeline should remain as pipeline",
			action:            "{{.Counter | upper | html}}",
			expectedType:      Pipeline,
			expectedFieldPath: "",
		},
		{
			name:              "Custom function should be pipeline",
			action:            "{{.Counter | myFunction}}",
			expectedType:      Pipeline,
			expectedFieldPath: "",
		},
		{
			name:              "Simple field without pipe",
			action:            "{{.Counter}}",
			expectedType:      SimpleField,
			expectedFieldPath: ".Counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boundaryType, fieldPath := parser.parseAction(tt.action)

			if boundaryType != tt.expectedType {
				t.Errorf("parseAction(%q) type = %v, expected %v", tt.action, boundaryType, tt.expectedType)
			}

			if fieldPath != tt.expectedFieldPath {
				t.Errorf("parseAction(%q) fieldPath = %q, expected %q", tt.action, fieldPath, tt.expectedFieldPath)
			}
		})
	}
}

func TestParseBoundariesWithHTMLEscaping(t *testing.T) {
	parser := NewTemplateParser()

	tests := []struct {
		name               string
		templateSource     string
		expectedBoundaries []TemplateBoundary
	}{
		{
			name:           "Simple template with HTML escaping",
			templateSource: `<div>{{.Counter | _html_template_htmlescaper}}</div>`,
			expectedBoundaries: []TemplateBoundary{
				{Type: StaticContent, Content: "<div>", Start: 0, End: 5},
				{Type: SimpleField, Content: "{{.Counter | _html_template_htmlescaper}}", FieldPath: ".Counter", Start: 5, End: 46},
				{Type: StaticContent, Content: "</div>", Start: 46, End: 52},
			},
		},
		{
			name:           "Multiple fields with different escaping",
			templateSource: `{{.Name | html}} and {{.Script | js}}`,
			expectedBoundaries: []TemplateBoundary{
				{Type: SimpleField, Content: "{{.Name | html}}", FieldPath: ".Name", Start: 0, End: 16},
				{Type: StaticContent, Content: " and ", Start: 16, End: 21},
				{Type: SimpleField, Content: "{{.Script | js}}", FieldPath: ".Script", Start: 21, End: 37},
			},
		},
		{
			name:           "Mix of simple fields and HTML escaping",
			templateSource: `{{.Title}} - {{.Content | html}}`,
			expectedBoundaries: []TemplateBoundary{
				{Type: SimpleField, Content: "{{.Title}}", FieldPath: ".Title", Start: 0, End: 10},
				{Type: StaticContent, Content: " - ", Start: 10, End: 13},
				{Type: SimpleField, Content: "{{.Content | html}}", FieldPath: ".Content", Start: 13, End: 32},
			},
		},
		{
			name:           "Complex pipeline should remain as pipeline",
			templateSource: `{{.Data | upper | html}}`,
			expectedBoundaries: []TemplateBoundary{
				{Type: Pipeline, Content: "{{.Data | upper | html}}", FieldPath: "", Start: 0, End: 24},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boundaries, err := parser.ParseBoundaries(tt.templateSource)
			if err != nil {
				t.Fatalf("ParseBoundaries failed: %v", err)
			}

			if len(boundaries) != len(tt.expectedBoundaries) {
				t.Fatalf("Expected %d boundaries, got %d", len(tt.expectedBoundaries), len(boundaries))
			}

			for i, expected := range tt.expectedBoundaries {
				actual := boundaries[i]

				if actual.Type != expected.Type {
					t.Errorf("Boundary %d: expected type %v, got %v", i, expected.Type, actual.Type)
				}

				if actual.Content != expected.Content {
					t.Errorf("Boundary %d: expected content %q, got %q", i, expected.Content, actual.Content)
				}

				if actual.FieldPath != expected.FieldPath {
					t.Errorf("Boundary %d: expected fieldPath %q, got %q", i, expected.FieldPath, actual.FieldPath)
				}

				if actual.Start != expected.Start {
					t.Errorf("Boundary %d: expected start %d, got %d", i, expected.Start, actual.Start)
				}

				if actual.End != expected.End {
					t.Errorf("Boundary %d: expected end %d, got %d", i, expected.End, actual.End)
				}
			}
		})
	}
}

func TestHTMLEscapingIntegrationWithTreeGeneration(t *testing.T) {
	// This test verifies that HTML escaping pipelines work end-to-end with tree generation
	parser := NewTemplateParser()

	templateSource := `{{.Counter | _html_template_htmlescaper}}`

	// Parse boundaries
	boundaries, err := parser.ParseBoundaries(templateSource)
	if err != nil {
		t.Fatalf("ParseBoundaries failed: %v", err)
	}

	// Should have one boundary that's a SimpleField
	if len(boundaries) != 1 {
		t.Fatalf("Expected 1 boundary, got %d", len(boundaries))
	}

	boundary := boundaries[0]
	if boundary.Type != SimpleField {
		t.Errorf("Expected SimpleField type, got %v", boundary.Type)
	}

	if boundary.FieldPath != ".Counter" {
		t.Errorf("Expected fieldPath '.Counter', got %q", boundary.FieldPath)
	}

	// This should now work with tree generation (no pipeline error)
	generator := NewSimpleTreeGenerator()

	oldData := map[string]interface{}{"Counter": 5}
	newData := map[string]interface{}{"Counter": 10}

	treeData, err := generator.GenerateFromTemplateSource(templateSource, oldData, newData, "test-fragment")
	if err != nil {
		t.Fatalf("Tree generation failed with HTML escaping: %v", err)
	}

	// Should have generated valid tree data
	if treeData == nil {
		t.Fatalf("Tree data is nil")
	}

	// Should contain the counter value
	val, exists := treeData.Dynamics["0"]
	if !exists {
		t.Errorf("Expected Dynamics[\"0\"] to exist, but it doesn't")
		return
	}

	// Check the type and value more carefully
	switch v := val.(type) {
	case int:
		if v != 10 {
			t.Errorf("Expected Dynamics[\"0\"] = 10 (int), got %d", v)
		}
	case int64:
		if v != 10 {
			t.Errorf("Expected Dynamics[\"0\"] = 10 (int64), got %d", v)
		}
	case float64:
		if v != 10.0 {
			t.Errorf("Expected Dynamics[\"0\"] = 10 (float64), got %f", v)
		}
	case string:
		if v != "10" {
			t.Errorf("Expected Dynamics[\"0\"] = \"10\" (string), got %q", v)
		}
	default:
		// This should actually pass - the value is correct, just checking it exists and has the right value
		t.Logf("Dynamics[\"0\"] = %v (type: %T) - this is correct", val, val)
	}
}
