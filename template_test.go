package livetemplate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Test data structures
type Counter struct {
	Value int    `json:"value"`
	Color string `json:"color"`
}

type Todo struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type TodoList struct {
	Todos          []Todo `json:"todos"`
	Count          int    `json:"count"`
	CompletedCount int    `json:"completedCount"`
}

// Test cases for the new public API
func TestTemplate_New(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name:     "valid template name",
			template: "test-template",
			wantErr:  false,
		},
		{
			name:     "empty template name",
			template: "",
			wantErr:  false, // Should allow empty names like html/template
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New(tt.template)
			if (tmpl == nil) != tt.wantErr {
				t.Errorf("New() returned nil = %v, wantErr %v", tmpl == nil, tt.wantErr)
			}
		})
	}
}

func TestTemplate_Parse(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		wantErr      bool
	}{
		{
			name:         "simple field template",
			templateText: "<p>Hello {{.Name}}!</p>",
			wantErr:      false,
		},
		{
			name:         "counter template",
			templateText: `<div class="counter"><span>{{.Value}}</span><span style="color: {{.Color}}">{{.Color}}</span></div>`,
			wantErr:      false,
		},
		{
			name: "full HTML document",
			templateText: `<!DOCTYPE html>
<html>
<head>
    <title>Counter</title>
</head>
<body>
    <div class="container">
        <h1>Counter: {{.Value}}</h1>
        <p style="color: {{.Color}}">Current color: {{.Color}}</p>
    </div>
</body>
</html>`,
			wantErr: false,
		},
		{
			name: "template with range",
			templateText: `<ul>
{{range .Items}}
<li>{{.Text}} - {{if .Completed}}✓{{else}}✗{{end}}</li>
{{end}}
</ul>`,
			wantErr: false,
		},
		{
			name:         "invalid template syntax",
			templateText: "<p>Hello {{.Name}!</p>{{",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New("test")
			result, err := tmpl.Parse(tt.templateText)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Errorf("Parse() returned nil template without error")
			}

			if !tt.wantErr && result != tmpl {
				t.Errorf("Parse() should return the same template instance")
			}
		})
	}
}

func TestTemplate_ParseFiles(t *testing.T) {
	// Create temporary template files for testing
	tests := []struct {
		name      string
		filenames []string
		wantErr   bool
	}{
		{
			name:      "parse single file",
			filenames: []string{"testdata/simple.html"},
			wantErr:   false,
		},
		{
			name:      "parse multiple files",
			filenames: []string{"testdata/layout.html", "testdata/content.html"},
			wantErr:   false,
		},
		{
			name:      "parse nonexistent file",
			filenames: []string{"testdata/nonexistent.html"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New("test")
			result, err := tmpl.ParseFiles(tt.filenames...)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Errorf("ParseFiles() returned nil template without error")
			}

			if !tt.wantErr && result != tmpl {
				t.Errorf("ParseFiles() should return the same template instance")
			}
		})
	}
}

func TestTemplate_Execute(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		data         interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name:         "simple field rendering",
			templateText: "<p>Hello {{.Name}}!</p>",
			data:         map[string]interface{}{"Name": "World"},
			wantContains: []string{"<p>Hello World!</p>", "data-lvt-id=\""},
			wantErr:      false,
		},
		{
			name:         "counter rendering",
			templateText: `<div class="counter"><span>{{.Value}}</span><span style="color: {{.Color}}">{{.Color}}</span></div>`,
			data:         Counter{Value: 42, Color: "blue"},
			wantContains: []string{"<span>42</span>", "blue", "data-lvt-id=\""},
			wantErr:      false,
		},
		{
			name: "full HTML document with wrapper injection",
			templateText: `<!DOCTYPE html>
<html>
<head>
    <title>Counter</title>
</head>
<body>
    <div class="container">
        <h1>Counter: {{.Value}}</h1>
        <p style="color: {{.Color}}">Current color: {{.Color}}</p>
    </div>
</body>
</html>`,
			data:         Counter{Value: 10, Color: "red"},
			wantContains: []string{"<!DOCTYPE html>", "<title>Counter</title>", "Counter: 10", "red", "data-lvt-id=\""},
			wantErr:      false,
		},
		{
			name: "template with range",
			templateText: `<ul>
{{range .Todos}}
<li>{{.Text}} - {{if .Completed}}✓{{else}}✗{{end}}</li>
{{end}}
</ul>`,
			data: TodoList{
				Todos: []Todo{
					{Text: "Buy milk", Completed: false},
					{Text: "Walk dog", Completed: true},
				},
			},
			wantContains: []string{"<li>Buy milk - ✗</li>", "<li>Walk dog - ✓</li>", "data-lvt-id=\""},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New("test")
			_, err := tmpl.Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()
				for _, want := range tt.wantContains {
					if !strings.Contains(output, want) {
						t.Errorf("Execute() output should contain %q, got: %s", want, output)
					}
				}

				// Verify wrapper injection for full HTML documents
				if strings.Contains(tt.templateText, "<!DOCTYPE html>") || strings.Contains(tt.templateText, "<html") {
					if !strings.Contains(output, "data-lvt-id=\"") {
						t.Errorf("Execute() should inject wrapper div with data-lvt-id for full HTML documents")
					}
				}
			}
		})
	}
}

func TestTemplate_ExecuteUpdates(t *testing.T) {
	tests := []struct {
		name             string
		templateText     string
		initialData      interface{}
		updatedData      interface{}
		wantInitialKeys  []string
		wantUpdateKeys   []string
		wantStaticCached bool
	}{
		{
			name:             "simple field update",
			templateText:     "<p>Hello {{.Name}}!</p>",
			initialData:      map[string]interface{}{"Name": "World"},
			updatedData:      map[string]interface{}{"Name": "Alice"},
			wantInitialKeys:  []string{"s", "0"},
			wantUpdateKeys:   []string{"0"}, // Only dynamic content should be in update
			wantStaticCached: true,
		},
		{
			name:             "counter update",
			templateText:     `<div class="counter"><span>{{.Value}}</span><span style="color: {{.Color}}">{{.Color}}</span></div>`,
			initialData:      Counter{Value: 0, Color: "blue"},
			updatedData:      Counter{Value: 1, Color: "red"},
			wantInitialKeys:  []string{"s", "0", "1", "2"},
			wantUpdateKeys:   []string{"0", "1", "2"}, // All dynamic values
			wantStaticCached: true,
		},
		{
			name: "todo list update - add item (range optimization enabled)",
			templateText: `<ul>
{{range .Todos}}
<li>{{.Text}} - {{if .Completed}}✓{{else}}✗{{end}}</li>
{{end}}
</ul>`,
			initialData: TodoList{
				Todos: []Todo{
					{Text: "Buy milk", Completed: false},
				},
			},
			updatedData: TodoList{
				Todos: []Todo{
					{Text: "Buy milk", Completed: false},
					{Text: "Walk dog", Completed: true},
				},
			},
			wantInitialKeys:  []string{"s", "0"}, // Static segments and range content
			wantUpdateKeys:   []string{"0"},      // Range content updates
			wantStaticCached: true,
		},
		{
			name:             "no changes - empty update",
			templateText:     "<p>Hello {{.Name}}!</p>",
			initialData:      map[string]interface{}{"Name": "World"},
			updatedData:      map[string]interface{}{"Name": "World"}, // Same data
			wantInitialKeys:  []string{"s", "0"},
			wantUpdateKeys:   []string{}, // Empty update when no changes
			wantStaticCached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New("test")
			_, err := tmpl.Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			// First call to ExecuteUpdates should include static structure
			var initialBuf bytes.Buffer
			err = tmpl.ExecuteUpdates(&initialBuf, tt.initialData)
			if err != nil {
				t.Errorf("ExecuteUpdates() initial call failed: %v", err)
				return
			}

			var initialTree map[string]interface{}
			err = json.Unmarshal(initialBuf.Bytes(), &initialTree)
			if err != nil {
				t.Errorf("ExecuteUpdates() initial output is not valid JSON: %v", err)
				return
			}

			// Verify initial tree structure contains expected keys
			for _, key := range tt.wantInitialKeys {
				if _, exists := initialTree[key]; !exists {
					t.Errorf("ExecuteUpdates() initial tree missing key %q, got keys: %v", key, getKeys(initialTree))
				}
			}

			// Second call should be cache-aware
			var updateBuf bytes.Buffer
			err = tmpl.ExecuteUpdates(&updateBuf, tt.updatedData)
			if err != nil {
				t.Errorf("ExecuteUpdates() update call failed: %v", err)
				return
			}

			updateBytes := updateBuf.Bytes()

			// Handle empty updates (no changes)
			if len(tt.wantUpdateKeys) == 0 {
				if len(updateBytes) > 2 { // Allow for empty JSON object "{}"
					var updateTree map[string]interface{}
					err = json.Unmarshal(updateBytes, &updateTree)
					if err == nil && len(updateTree) > 0 {
						t.Errorf("ExecuteUpdates() should return empty update when data unchanged, got: %s", updateBytes)
					}
				}
				return
			}

			var updateTree map[string]interface{}
			err = json.Unmarshal(updateBytes, &updateTree)
			if err != nil {
				t.Errorf("ExecuteUpdates() update output is not valid JSON: %v", err)
				return
			}

			// Verify update tree contains expected keys
			for _, key := range tt.wantUpdateKeys {
				if _, exists := updateTree[key]; !exists {
					t.Errorf("ExecuteUpdates() update tree missing key %q, got keys: %v", key, getKeys(updateTree))
				}
			}

			// Verify static content caching - updates should not contain "s" key
			if tt.wantStaticCached {
				if _, hasStatics := updateTree["s"]; hasStatics {
					t.Errorf("ExecuteUpdates() update should not contain static structure ('s' key) when cached")
				}
			}
		})
	}
}

func TestTemplate_CompileTimeTreeGeneration(t *testing.T) {
	tests := []struct {
		name                string
		templateText        string
		wantRuntimeStatics  bool // True if some parts need runtime hydration
		wantCompiledStatics bool // True if some parts can be determined at compile time
	}{
		{
			name:                "simple static text",
			templateText:        "<p>Hello World!</p>",
			wantRuntimeStatics:  false,
			wantCompiledStatics: true,
		},
		{
			name:                "mixed static and dynamic",
			templateText:        "<p>Hello {{.Name}}!</p>",
			wantRuntimeStatics:  false,
			wantCompiledStatics: true,
		},
		{
			name:                "conditional with unknown structure",
			templateText:        "{{if .ShowDetails}}<div>{{.Details}}</div>{{else}}<span>{{.Summary}}</span>{{end}}",
			wantRuntimeStatics:  true, // Structure depends on data
			wantCompiledStatics: false,
		},
		{
			name:                "range with unknown length",
			templateText:        "{{range .Items}}<li>{{.Text}}</li>{{end}}",
			wantRuntimeStatics:  true, // Number of items unknown at compile time
			wantCompiledStatics: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New("test")
			_, err := tmpl.Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			// ExecuteUpdates should work even without prior Execute call
			// This tests the compile-time tree generation
			var buf bytes.Buffer
			err = tmpl.ExecuteUpdates(&buf, map[string]interface{}{
				"Name":        "Test",
				"ShowDetails": true,
				"Details":     "Some details",
				"Summary":     "Some summary",
				"Items":       []map[string]interface{}{{"Text": "Item 1"}},
			})

			if err != nil {
				t.Errorf("ExecuteUpdates() failed on first call: %v", err)
				return
			}

			var tree map[string]interface{}
			err = json.Unmarshal(buf.Bytes(), &tree)
			if err != nil {
				t.Errorf("ExecuteUpdates() output is not valid JSON: %v", err)
				return
			}

			// Test compile-time static detection
			if tt.wantCompiledStatics {
				if _, hasStatics := tree["s"]; !hasStatics {
					t.Errorf("Template should have compile-time static parts, got keys: %v", getKeys(tree))
				}
			}

			// Note: Runtime statics testing requires more complex implementation
			// This is a placeholder for the behavior specification
		})
	}
}

func TestTemplate_RuntimeHydrationAndDiffing(t *testing.T) {
	tests := []struct {
		name          string
		templateText  string
		data1         interface{}
		data2         interface{}
		wantDifferent bool
	}{
		{
			name:          "field value change",
			templateText:  "<p>Hello {{.Name}}!</p>",
			data1:         map[string]interface{}{"Name": "World"},
			data2:         map[string]interface{}{"Name": "Alice"},
			wantDifferent: true,
		},
		{
			name:          "no change",
			templateText:  "<p>Hello {{.Name}}!</p>",
			data1:         map[string]interface{}{"Name": "World"},
			data2:         map[string]interface{}{"Name": "World"},
			wantDifferent: false,
		},
		{
			name:          "structural change in conditional",
			templateText:  "{{if .Show}}<div>{{.Content}}</div>{{else}}<span>Hidden</span>{{end}}",
			data1:         map[string]interface{}{"Show": true, "Content": "Visible"},
			data2:         map[string]interface{}{"Show": false, "Content": "Visible"},
			wantDifferent: true,
		},
		{
			name:          "list length change",
			templateText:  "{{range .Items}}<li>{{.}}</li>{{end}}",
			data1:         map[string]interface{}{"Items": []string{"A", "B"}},
			data2:         map[string]interface{}{"Items": []string{"A", "B", "C"}},
			wantDifferent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := New("test")
			_, err := tmpl.Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			// First ExecuteUpdates call
			var buf1 bytes.Buffer
			err = tmpl.ExecuteUpdates(&buf1, tt.data1)
			if err != nil {
				t.Errorf("ExecuteUpdates() first call failed: %v", err)
				return
			}

			// Second ExecuteUpdates call
			var buf2 bytes.Buffer
			err = tmpl.ExecuteUpdates(&buf2, tt.data2)
			if err != nil {
				t.Errorf("ExecuteUpdates() second call failed: %v", err)
				return
			}

			// Compare outputs
			output1 := buf1.String()
			output2 := buf2.String()

			if tt.wantDifferent {
				if output1 == output2 {
					t.Errorf("ExecuteUpdates() should produce different output for different data")
				}
			} else {
				// For no change, second call should return minimal/empty update
				var tree2 map[string]interface{}
				if len(output2) > 2 { // More than empty JSON object
					err = json.Unmarshal([]byte(output2), &tree2)
					if err == nil && len(tree2) > 0 {
						t.Errorf("ExecuteUpdates() should return minimal update when data unchanged, got: %s", output2)
					}
				}
			}
		})
	}
}

// Helper function to extract keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Test compatibility with html/template interface
func TestTemplate_HtmlTemplateCompatibility(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		data         interface{}
	}{
		{
			name:         "basic rendering compatibility",
			templateText: "<p>Hello {{.Name}}!</p>",
			data:         map[string]interface{}{"Name": "World"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that our Template behaves similarly to html/template for basic operations
			tmpl := New("test")
			_, err := tmpl.Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Parse() failed: %v", err)
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, tt.data)
			if err != nil {
				t.Errorf("Execute() failed: %v", err)
			}

			// The output should contain the rendered content
			// (wrapper injection makes it different from html/template, but core content should be there)
			output := buf.String()
			if !strings.Contains(output, "Hello World!") {
				t.Errorf("Execute() output should contain rendered content, got: %s", output)
			}
		})
	}
}

// Benchmark tests for performance characteristics
func BenchmarkTemplate_Execute(b *testing.B) {
	tmpl := New("benchmark")
	tmpl.Parse("<p>Hello {{.Name}}!</p>")
	data := map[string]interface{}{"Name": "World"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		tmpl.Execute(&buf, data)
	}
}

func BenchmarkTemplate_ExecuteUpdates(b *testing.B) {
	tmpl := New("benchmark")
	tmpl.Parse("<p>Hello {{.Name}}!</p>")

	// Prime the template
	var initBuf bytes.Buffer
	tmpl.ExecuteUpdates(&initBuf, map[string]interface{}{"Name": "World"})

	data := map[string]interface{}{"Name": "Alice"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		tmpl.ExecuteUpdates(&buf, data)
	}
}
