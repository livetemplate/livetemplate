package livetemplate

import (
	"bytes"
	"html/template"
	"testing"
)

// TestTemplateParity_DollarInRange tests that $ refers to root context in range loops
// This is a critical parity check with Go's standard template package
func TestTemplateParity_DollarInRange(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     interface{}
		expected string
	}{
		{
			name: "$.Field in range",
			tmpl: `{{range .Items}}{{.Name}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{
				"Title": "ROOT",
				"Items": []map[string]string{
					{"Name": "A"},
					{"Name": "B"},
				},
			},
			expected: "A-ROOTB-ROOT",
		},
		{
			name: "$.Field in if inside range",
			tmpl: `{{range .Messages}}{{if eq .Username $.CurrentUser}}mine{{else}}other{{end}}{{end}}`,
			data: map[string]interface{}{
				"CurrentUser": "alice",
				"Messages": []map[string]string{
					{"Username": "alice"},
					{"Username": "bob"},
					{"Username": "alice"},
				},
			},
			expected: "mineothermine",
		},
		{
			name: "nested range with $",
			tmpl: `{{range .Outer}}{{range .Inner}}{{.}}-{{$.Root}}{{end}}{{end}}`,
			data: map[string]interface{}{
				"Root": "TOP",
				"Outer": []map[string]interface{}{
					{
						"Inner": []string{"a", "b"},
					},
				},
			},
			expected: "a-TOPb-TOP",
		},
		{
			name: "$ with variable in range",
			tmpl: `{{range $i, $v := .Items}}{{$i}}: {{$v.Name}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{
				"Title": "ROOT",
				"Items": []map[string]string{
					{"Name": "A"},
					{"Name": "B"},
				},
			},
			expected: "0: A-ROOT1: B-ROOT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with standard Go template
			stdTmpl, err := template.New("std").Parse(tt.tmpl)
			if err != nil {
				t.Fatalf("Standard template parse error: %v", err)
			}

			var stdBuf bytes.Buffer
			if err := stdTmpl.Execute(&stdBuf, tt.data); err != nil {
				t.Fatalf("Standard template execute error: %v", err)
			}

			stdResult := stdBuf.String()
			if stdResult != tt.expected {
				t.Errorf("Standard template result mismatch:\nGot:  %q\nWant: %q", stdResult, tt.expected)
			}

			// Test with LiveTemplate
			lvtTmpl := New("test")
			if _, err := lvtTmpl.Parse(tt.tmpl); err != nil {
				t.Fatalf("LiveTemplate parse error: %v", err)
			}

			var lvtBuf bytes.Buffer
			if err := lvtTmpl.Execute(&lvtBuf, tt.data); err != nil {
				t.Fatalf("LiveTemplate execute error: %v", err)
			}

			lvtResult := lvtBuf.String()

			// LiveTemplate adds wrapper div, so extract content between div tags
			// This is a simple extraction - for more complex cases we'd need proper parsing
			lvtResultStripped := extractContent(lvtResult)

			if lvtResultStripped != tt.expected {
				t.Errorf("LiveTemplate result mismatch:\nGot:  %q\nWant: %q\nFull: %q", lvtResultStripped, tt.expected, lvtResult)
			}

			// Ensure both match
			if lvtResultStripped != stdResult {
				t.Errorf("Parity mismatch between standard and LiveTemplate:\nStandard:     %q\nLiveTemplate: %q", stdResult, lvtResultStripped)
			}
		})
	}
}

// TestTemplateParity_DotInRange tests that . refers to current item in range loops
func TestTemplateParity_DotInRange(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     interface{}
		expected string
	}{
		{
			name: "simple . in range",
			tmpl: `{{range .Items}}{{.}}{{end}}`,
			data: map[string]interface{}{
				"Items": []string{"a", "b", "c"},
			},
			expected: "abc",
		},
		{
			name: ". field access in range",
			tmpl: `{{range .Items}}{{.Name}}{{end}}`,
			data: map[string]interface{}{
				"Items": []map[string]string{
					{"Name": "Alice"},
					{"Name": "Bob"},
				},
			},
			expected: "AliceBob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with standard Go template
			stdTmpl, err := template.New("std").Parse(tt.tmpl)
			if err != nil {
				t.Fatalf("Standard template parse error: %v", err)
			}

			var stdBuf bytes.Buffer
			if err := stdTmpl.Execute(&stdBuf, tt.data); err != nil {
				t.Fatalf("Standard template execute error: %v", err)
			}

			stdResult := stdBuf.String()

			// Test with LiveTemplate
			lvtTmpl := New("test")
			if _, err := lvtTmpl.Parse(tt.tmpl); err != nil {
				t.Fatalf("LiveTemplate parse error: %v", err)
			}

			var lvtBuf bytes.Buffer
			if err := lvtTmpl.Execute(&lvtBuf, tt.data); err != nil {
				t.Fatalf("LiveTemplate execute error: %v", err)
			}

			lvtResult := extractContent(lvtBuf.String())

			// Ensure both match
			if lvtResult != stdResult {
				t.Errorf("Parity mismatch:\nStandard:     %q\nLiveTemplate: %q", stdResult, lvtResult)
			}
		})
	}
}

// extractContent extracts content between the wrapper div tags that LiveTemplate adds
func extractContent(html string) string {
	// Simple extraction: find content between first > and last <
	// This works for simple cases but may need refinement for complex HTML
	start := -1
	end := -1

	// Find first >
	for i := 0; i < len(html); i++ {
		if html[i] == '>' {
			start = i + 1
			break
		}
	}

	// Find last <
	for i := len(html) - 1; i >= 0; i-- {
		if html[i] == '<' {
			end = i
			break
		}
	}

	if start >= 0 && end >= start {
		return html[start:end]
	}

	return html
}

// TestTemplateParity_VariablesInRange tests variable declarations in range
func TestTemplateParity_VariablesInRange(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     interface{}
		expected string
	}{
		{
			name: "single variable in range",
			tmpl: `{{range $v := .Items}}{{$v}}{{end}}`,
			data: map[string]interface{}{
				"Items": []string{"x", "y", "z"},
			},
			expected: "xyz",
		},
		{
			name: "index and value variables in range",
			tmpl: `{{range $i, $v := .Items}}{{$i}}:{{$v}} {{end}}`,
			data: map[string]interface{}{
				"Items": []string{"a", "b"},
			},
			expected: "0:a 1:b ",
		},
		{
			name: "variables with $ in if condition",
			tmpl: `{{range $i, $v := .Items}}{{if eq $v $.Target}}{{$i}}{{end}}{{end}}`,
			data: map[string]interface{}{
				"Target": "b",
				"Items":  []string{"a", "b", "c"},
			},
			expected: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with standard Go template
			stdTmpl, err := template.New("std").Parse(tt.tmpl)
			if err != nil {
				t.Fatalf("Standard template parse error: %v", err)
			}

			var stdBuf bytes.Buffer
			if err := stdTmpl.Execute(&stdBuf, tt.data); err != nil {
				t.Fatalf("Standard template execute error: %v", err)
			}

			stdResult := stdBuf.String()

			// Test with LiveTemplate
			lvtTmpl := New("test")
			if _, err := lvtTmpl.Parse(tt.tmpl); err != nil {
				t.Fatalf("LiveTemplate parse error: %v", err)
			}

			var lvtBuf bytes.Buffer
			if err := lvtTmpl.Execute(&lvtBuf, tt.data); err != nil {
				t.Fatalf("LiveTemplate execute error: %v", err)
			}

			lvtResult := extractContent(lvtBuf.String())

			// Ensure both match
			if lvtResult != stdResult {
				t.Errorf("Parity mismatch:\nStandard:     %q\nLiveTemplate: %q", stdResult, lvtResult)
			}
		})
	}
}
