package livetemplate

import (
	"html/template"
	"strings"
	"testing"
)

func TestFlattenTemplate_Simple(t *testing.T) {
	// Test basic {{define}} and {{template}}
	templateStr := `
{{define "header"}}
<h1>{{.Title}}</h1>
{{end}}

{{template "header" .}}
`

	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := flattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Should contain the h1 with title
	if !strings.Contains(flattened, "<h1>{{.Title}}</h1>") {
		t.Errorf("Flattened template missing expected content. Got: %s", flattened)
	}

	// Should NOT contain {{define}} or {{template}}
	if strings.Contains(flattened, "{{define") {
		t.Errorf("Flattened template still contains {{define}}")
	}
	if strings.Contains(flattened, "{{template") {
		t.Errorf("Flattened template still contains {{template}}")
	}
}

func TestFlattenTemplate_WithLayout(t *testing.T) {
	// Test layout pattern with block
	templateStr := `
{{define "layout"}}
<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>
{{template "content" .}}
</body>
</html>
{{end}}

{{define "content"}}
<div>{{.Body}}</div>
{{end}}

{{template "layout" .}}
`

	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := flattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Should contain both title and body fields
	if !strings.Contains(flattened, "{{.Title}}") {
		t.Errorf("Flattened template missing {{.Title}}")
	}
	if !strings.Contains(flattened, "{{.Body}}") {
		t.Errorf("Flattened template missing {{.Body}}")
	}

	// Should contain HTML structure
	if !strings.Contains(flattened, "<!DOCTYPE html>") {
		t.Errorf("Flattened template missing DOCTYPE")
	}
}

func TestFlattenTemplate_NestedTemplates(t *testing.T) {
	// Test nested template invocations
	templateStr := `
{{define "nested_outer"}}
<div>{{template "nested_inner" .}}</div>
{{end}}

{{define "nested_inner"}}
<span>{{.Value}}</span>
{{end}}

{{template "nested_outer" .}}
`

	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := flattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Should have nested structure flattened
	if !strings.Contains(flattened, "<div>") {
		t.Errorf("Flattened template missing <div>")
	}
	if !strings.Contains(flattened, "<span>{{.Value}}</span>") {
		t.Errorf("Flattened template missing span content")
	}
}

func TestFlattenTemplate_WithConditionals(t *testing.T) {
	// Test that conditionals are preserved during flattening
	templateStr := `
{{define "item"}}
{{if .Active}}
<span class="active">{{.Name}}</span>
{{else}}
<span class="inactive">{{.Name}}</span>
{{end}}
{{end}}

{{template "item" .}}
`

	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := flattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Should preserve if/else structure
	if !strings.Contains(flattened, "{{if .Active}}") {
		t.Errorf("Flattened template missing {{if}}")
	}
	if !strings.Contains(flattened, "{{else}}") {
		t.Errorf("Flattened template missing {{else}}")
	}
	if !strings.Contains(flattened, "{{end}}") {
		t.Errorf("Flattened template missing {{end}}")
	}
}

func TestFlattenTemplate_WithRange(t *testing.T) {
	// Test that range loops are preserved
	templateStr := `
{{define "list"}}
<ul>
{{range .Items}}
<li>{{.Name}}</li>
{{end}}
</ul>
{{end}}

{{template "list" .}}
`

	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := flattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Should preserve range structure
	if !strings.Contains(flattened, "{{range .Items}}") {
		t.Errorf("Flattened template missing {{range}}")
	}
	if !strings.Contains(flattened, "<li>{{.Name}}</li>") {
		t.Errorf("Flattened template missing list item")
	}
}

func TestHasTemplateComposition(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected bool
	}{
		{
			name:     "simple template",
			template: `<div>{{.Title}}</div>`,
			expected: false,
		},
		{
			name: "with define",
			template: `{{define "foo"}}<div>{{.Title}}</div>{{end}}
{{template "foo" .}}`,
			expected: true,
		},
		{
			name:     "with template invocation",
			template: `<div>{{template "header" .}}</div>`,
			expected: true,
		},
		{
			name:     "with if",
			template: `{{if .Show}}<div>{{.Title}}</div>{{end}}`,
			expected: false,
		},
		{
			name:     "with range",
			template: `{{range .Items}}<li>{{.Name}}</li>{{end}}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New(t.Name()).Parse(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			result := hasTemplateComposition(tmpl)
			if result != tt.expected {
				t.Errorf("hasTemplateComposition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFlattenTemplate_IntegrationWithTreeGeneration(t *testing.T) {
	// Test that flattened templates work with tree generation
	templateStr := `
{{define "layout"}}
<!DOCTYPE html>
<html>
<body>
<h1>{{.Title}}</h1>
{{template "content" .}}
</body>
</html>
{{end}}

{{define "content"}}
<div>
{{range .Items}}
<p>{{.Name}}</p>
{{end}}
</div>
{{end}}

{{template "layout" .}}
`

	// Parse and flatten
	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := flattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Test with tree generation
	data := map[string]interface{}{
		"Title": "Test Page",
		"Items": []map[string]string{
			{"Name": "Item 1"},
			{"Name": "Item 2"},
		},
	}

	tree, err := parseTemplateToTree(flattened, data, NewKeyGenerator())
	if err != nil {
		t.Fatalf("Failed to generate tree from flattened template: %v", err)
	}

	// Verify tree was generated
	if tree == nil {
		t.Fatal("Tree is nil")
	}

	// Tree should have statics
	if _, ok := tree["s"]; !ok {
		t.Error("Tree missing statics ('s' key)")
	}
}

func TestFlattenTemplate_ErrorCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name: "undefined template reference",
			template: `{{define "error_test_defined"}}<div>{{.Title}}</div>{{end}}
{{template "error_test_undefined" .}}`,
			wantErr: true,
		},
		{
			name:     "template with no main execution",
			template: `{{define "error_test_noexec"}}<div>{{.Title}}</div>{{end}}`,
			wantErr:  false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New(t.Name()).Parse(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			_, err = flattenTemplate(tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("flattenTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
