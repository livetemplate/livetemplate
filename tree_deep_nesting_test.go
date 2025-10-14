package livetemplate

import (
	"testing"
)

// TestDeepNesting specifically tests regex parser with deeply nested constructs
// This addresses the concern: "regexes just stop working on deeply nested conditionals"
func TestDeepNesting(t *testing.T) {
	data := map[string]interface{}{
		"A": true,
		"B": true,
		"C": true,
		"D": true,
		"E": true,
		"F": true,
		"G": true,
		"H": true,
		"I": true,
		"J": true,
		"User": map[string]interface{}{
			"Name": "John",
			"A":    true,
			"B":    true,
		},
		"Items": []map[string]interface{}{
			{"Name": "Item1", "Active": true, "A": true, "B": true},
		},
	}

	tests := []struct {
		name     string
		template string
		nesting  int
		expected string
	}{
		// Pure if nesting
		{"Level 2", "{{if .A}}{{if .B}}nested{{end}}{{end}}", 2, "nested"},
		{"Level 3", "{{if .A}}{{if .B}}{{if .C}}triple{{end}}{{end}}{{end}}", 3, "triple"},
		{"Level 4", "{{if .A}}{{if .B}}{{if .C}}{{if .D}}quad{{end}}{{end}}{{end}}{{end}}", 4, "quad"},
		{"Level 5", "{{if .A}}{{if .B}}{{if .C}}{{if .D}}{{if .E}}five{{end}}{{end}}{{end}}{{end}}{{end}}", 5, "five"},
		{"Level 10", "{{if .A}}{{if .B}}{{if .C}}{{if .D}}{{if .E}}{{if .F}}{{if .G}}{{if .H}}{{if .I}}{{if .J}}ten{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}", 10, "ten"},

		// With construct nesting
		{"With simple", "{{with .User}}Hello {{.Name}}{{end}}", 1, "Hello John"},
		{"With + if", "{{with .User}}{{if .A}}{{.Name}}{{end}}{{end}}", 2, "John"},
		{"With + if + if", "{{with .User}}{{if .A}}{{if .B}}{{.Name}}{{end}}{{end}}{{end}}", 3, "John"},
		{"With + with", "{{with .User}}{{with .Name}}User: {{.}}{{end}}{{end}}", 2, "User: John"},
		{"If + with + if", "{{if .A}}{{with .User}}{{if .B}}{{.Name}}{{end}}{{end}}{{end}}", 3, "John"},

		// Range construct nesting
		{"Range simple", "{{range .Items}}<span>{{.Name}}</span>{{end}}", 1, "<span>Item1</span>"},
		{"Range + if", "{{range .Items}}{{if .Active}}{{.Name}}{{end}}{{end}}", 2, "Item1"},
		{"If + range + if", "{{if .A}}{{range .Items}}{{if .Active}}{{.Name}}{{end}}{{end}}{{end}}", 3, "Item1"},
		{"Range + if + if", "{{range .Items}}{{if .A}}{{if .Active}}{{.Name}}{{end}}{{end}}{{end}}", 3, "Item1"},
		{"Range + if + if + if", "{{range .Items}}{{if .A}}{{if .B}}{{if .Active}}{{.Name}}{{end}}{{end}}{{end}}{{end}}", 4, "Item1"},

		// Complex mixed patterns
		{"Mixed 3", "{{if .A}}{{range .Items}}{{if .Active}}{{.Name}}{{end}}{{end}}{{end}}", 3, "Item1"},
		{"Complex branches", "{{if .A}}{{if .B}}b{{else}}not-b{{end}}{{else}}{{if .C}}c{{else}}not-c{{end}}{{end}}", 3, "b"},
		{"With + range", "{{with .Items}}{{range .}}{{.Name}}{{end}}{{end}}", 2, "Item1"},
		{"If + with + range", "{{if .A}}{{with .Items}}{{range .}}{{.Name}}{{end}}{{end}}{{end}}", 3, "Item1"},

		// Template composition (requires flattening first)
		// Note: These will be tested separately since they need special handling
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyGen := NewKeyGenerator()
			tree, err := parseTemplateToTree(tt.template, data, keyGen)

			if err != nil {
				t.Fatalf("❌ Failed at nesting level %d: %v\nTemplate: %s", tt.nesting, err, tt.template)
			}

			// Verify tree invariant
			if err := checkTreeInvariant(tree, tt.name); err != nil {
				t.Fatalf("❌ Invariant violation at level %d: %v\nTree: %+v", tt.nesting, err, tree)
			}

			// Verify tree produces expected output
			output := reconstructHTML(tree)
			if output != tt.expected {
				t.Errorf("❌ Output mismatch at level %d\nExpected: %q\nGot: %q\nTree: %+v",
					tt.nesting, tt.expected, output, tree)
				return
			}

			t.Logf("✅ Level %d passed - Output: %q", tt.nesting, output)
		})
	}
}

// TestTemplateComposition tests {{define}}/{{template}}/{{block}} constructs
// These require template flattening before tree generation
func TestTemplateComposition(t *testing.T) {
	data := map[string]interface{}{
		"A":     true,
		"B":     true,
		"Title": "Page Title",
		"User": map[string]interface{}{
			"Name": "John",
		},
		"Items": []map[string]interface{}{
			{"Name": "Item1", "Active": true, "A": true, "B": true},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			"Simple define+template",
			`{{define "greeting"}}Hello{{end}}{{template "greeting" .}}`,
			"Hello",
		},
		{
			"Define with data",
			`{{define "user"}}{{.Name}}{{end}}{{template "user" .User}}`,
			"John",
		},
		{
			"Define with if",
			`{{define "item"}}{{if .Active}}{{.Name}}{{end}}{{end}}{{range .Items}}{{template "item" .}}{{end}}`,
			"Item1",
		},
		{
			"Nested defines",
			`{{define "inner"}}Inner{{end}}{{define "outer"}}Outer:{{template "inner" .}}{{end}}{{template "outer" .}}`,
			"Outer:Inner",
		},
		{
			"Block with default",
			`{{block "content" .}}Default{{end}}`,
			"Default",
		},
		{
			"Define + if nesting",
			`{{define "check"}}{{if .A}}{{if .B}}OK{{end}}{{end}}{{end}}{{template "check" .}}`,
			"OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyGen := NewKeyGenerator()
			tree, err := parseTemplateToTree(tt.template, data, keyGen)

			if err != nil {
				t.Fatalf("❌ Failed: %v\nTemplate: %s", err, tt.template)
			}

			// Verify tree invariant
			if err := checkTreeInvariant(tree, tt.name); err != nil {
				t.Fatalf("❌ Invariant violation: %v\nTree: %+v", err, tree)
			}

			// Verify output
			output := reconstructHTML(tree)
			if output != tt.expected {
				t.Errorf("❌ Output mismatch\nExpected: %q\nGot: %q\nTree: %+v",
					tt.expected, output, tree)
				return
			}

			t.Logf("✅ Passed - Output: %q", output)
		})
	}
}

// Note: reconstructHTML is defined in tree_test_helpers.go
