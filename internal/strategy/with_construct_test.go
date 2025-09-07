package strategy

import (
	"testing"
)

func TestSimpleTreeGenerator_WithConstruct(t *testing.T) {
	generator := NewSimpleTreeGenerator()

	tests := []struct {
		name           string
		templateSource string
		data           interface{}
		shouldSucceed  bool
	}{
		{
			name:           "Basic with construct",
			templateSource: `<div>{{with .User}}<span>{{.Name}}</span>{{end}}</div>`,
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Name": "Alice",
				},
			},
			shouldSucceed: true,
		},
		{
			name:           "With construct with else",
			templateSource: `<div>{{with .User}}<span>{{.Name}}</span>{{else}}<span>No user</span>{{end}}</div>`,
			data:           map[string]interface{}{},
			shouldSucceed:  true,
		},
		{
			name:           "Nested fields in with",
			templateSource: `{{with .Profile}}Name: {{.FirstName}} {{.LastName}}{{end}}`,
			data: map[string]interface{}{
				"Profile": map[string]interface{}{
					"FirstName": "John",
					"LastName":  "Doe",
				},
			},
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache to ensure we test full structure generation
			generator.ClearCache()

			result, err := generator.GenerateFromTemplateSource(tt.templateSource, nil, tt.data, "test-fragment")

			if tt.shouldSucceed {
				if err != nil {
					t.Fatalf("GenerateFromTemplateSource failed: %v", err)
				}

				if result == nil {
					t.Fatal("Result is nil")
				}

				// Basic validation - should have static segments and dynamics
				if len(result.S) == 0 {
					t.Error("Expected static segments")
				}

				// Should have at least one dynamic entry for the with content
				if len(result.Dynamics) == 0 {
					t.Error("Expected dynamic content")
				}

				t.Logf("Generated tree structure: s=%v, dynamics=%v", result.S, result.Dynamics)
			} else {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			}
		})
	}
}

func TestSimpleTreeGenerator_WithConstructNil(t *testing.T) {
	generator := NewSimpleTreeGenerator()

	templateSource := `<div>{{with .User}}User: {{.Name}}{{else}}No user{{end}}</div>`
	data := map[string]interface{}{
		"User": nil,
	}

	result, err := generator.GenerateFromTemplateSource(templateSource, nil, data, "test-fragment")

	if err != nil {
		t.Fatalf("GenerateFromTemplateSource failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Should have static segments
	if len(result.S) == 0 {
		t.Error("Expected static segments")
	}

	// Should have dynamic content (the else case)
	if len(result.Dynamics) == 0 {
		t.Error("Expected dynamic content for else case")
	}
}
