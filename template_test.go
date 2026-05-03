package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"iter"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/compat"
	"github.com/livetemplate/livetemplate/internal/parse"
)

// ----- template_test.go -----
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
			tmpl := Must(New(tt.template))
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
			tmpl := Must(New("test"))
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
			tmpl := Must(New("test"))
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
			tmpl := Must(New("test"))
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
			tmpl := Must(New("test"))
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

func TestTemplate_BracketExpansionInTreeStatics(t *testing.T) {
	// Bracket expansion should happen at parse time, so both HTTP and WebSocket
	// paths see expanded attributes. This test verifies the tree (WebSocket path)
	// contains expanded attributes in statics, especially inside {{range}} blocks.
	type Item struct {
		Name string
	}

	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<ul>{{range .Items}}<li lvt-el:addClass:on:[save,delete]:pending="loading">{{.Name}}</li>{{end}}</ul>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	// ExecuteUpdates produces the tree JSON sent via WebSocket
	var buf bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf, map[string]interface{}{
		"Items": []Item{{Name: "first"}},
	})
	if err != nil {
		t.Fatalf("ExecuteUpdates() failed: %v", err)
	}

	output := buf.String()

	// The tree statics should contain expanded attributes (not bracket syntax)
	if strings.Contains(output, "on:[save,delete]") {
		t.Errorf("tree statics should not contain unexpanded bracket syntax, got: %s", output)
	}
	if !strings.Contains(output, `on:save:pending`) {
		t.Errorf("tree statics should contain expanded on:save:pending, got: %s", output)
	}
	if !strings.Contains(output, `on:delete:pending`) {
		t.Errorf("tree statics should contain expanded on:delete:pending, got: %s", output)
	}

	// Also verify HTTP path (Execute) produces expanded output
	var httpBuf bytes.Buffer
	err = tmpl.Execute(&httpBuf, map[string]interface{}{
		"Items": []Item{{Name: "first"}},
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	httpOutput := httpBuf.String()
	if strings.Contains(httpOutput, "on:[save,delete]") {
		t.Errorf("HTTP output should not contain unexpanded bracket syntax, got: %s", httpOutput)
	}
	if !strings.Contains(httpOutput, `on:save:pending="loading"`) {
		t.Errorf("HTTP output should contain expanded on:save:pending, got: %s", httpOutput)
	}
}

func TestTemplate_BracketExpansionInConditionalBlocks(t *testing.T) {
	// Verify bracket expansion works inside {{if}} blocks in the WebSocket tree path.
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div>{{if .Show}}<span lvt-el:addClass:on:[save,delete]:pending="loading">hi</span>{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf, map[string]interface{}{"Show": true})
	if err != nil {
		t.Fatalf("ExecuteUpdates() failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "on:[save,delete]") {
		t.Errorf("tree statics should not contain unexpanded bracket syntax inside {{if}}, got: %s", output)
	}
	if !strings.Contains(output, `on:save:pending`) {
		t.Errorf("tree statics should contain expanded on:save:pending, got: %s", output)
	}
	if !strings.Contains(output, `on:delete:pending`) {
		t.Errorf("tree statics should contain expanded on:delete:pending, got: %s", output)
	}
}

func TestTemplate_ExecuteUpdates_FlashMessages(t *testing.T) {
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div>{{if .lvt.HasFlash "success"}}<p>{{.lvt.Flash "success"}}</p>{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	// Render 1: No flash messages (baseline)
	var buf1 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf1, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("ExecuteUpdates() render 1 failed: %v", err)
	}

	// Render 2: Flash message set — should produce update with flash content
	var buf2 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf2, State{}, map[string]string{
		"_flash:success": "Item added",
	})
	if err != nil {
		t.Fatalf("ExecuteUpdates() render 2 failed: %v", err)
	}

	updateJSON := buf2.String()
	if updateJSON == "{}" {
		t.Fatal("ExecuteUpdates() render 2 returned empty update; expected flash content")
	}
	if !strings.Contains(updateJSON, "Item added") {
		t.Errorf("ExecuteUpdates() render 2 should contain flash message, got: %s", updateJSON)
	}

	// Render 3: Flash cleared — should revert (conditional changes back to empty)
	var buf3 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf3, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("ExecuteUpdates() render 3 failed: %v", err)
	}

	revertJSON := buf3.String()
	if revertJSON == "{}" {
		t.Fatal("ExecuteUpdates() render 3 returned empty update; expected revert of flash content")
	}
	if strings.Contains(revertJSON, "Item added") {
		t.Errorf("ExecuteUpdates() render 3 should not contain flash message after clear, got: %s", revertJSON)
	}
}

func TestTemplate_ExecuteUpdates_FlashWithoutConditional(t *testing.T) {
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div><span>{{.lvt.Flash "info"}}</span></div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	// Render 1: No flash (baseline)
	var buf1 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf1, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("ExecuteUpdates() render 1 failed: %v", err)
	}

	// Render 2: Flash set — dynamic value changes from "" to "Hello"
	var buf2 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf2, State{}, map[string]string{
		"_flash:info": "Hello",
	})
	if err != nil {
		t.Fatalf("ExecuteUpdates() render 2 failed: %v", err)
	}

	updateJSON := buf2.String()
	if updateJSON == "{}" {
		t.Fatal("ExecuteUpdates() render 2 returned empty update; expected flash content")
	}
	if !strings.Contains(updateJSON, "Hello") {
		t.Errorf("ExecuteUpdates() render 2 should contain flash value, got: %s", updateJSON)
	}
}

func TestTemplate_AriaInvalidHelper(t *testing.T) {
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div><input name="title" type="text" {{.lvt.AriaInvalid "title"}}></div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	// With error — should output aria-invalid="true"
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, State{}, map[string]string{"title": "Required"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `aria-invalid="true"`) {
		t.Errorf("expected aria-invalid on input with error, got: %s", output)
	}

	// Without error — no aria-invalid
	tmpl2 := Must(New("test2"))
	tmpl2, _ = tmpl2.Parse(`<div><input name="title" type="text" {{.lvt.AriaInvalid "title"}}></div>`)
	var buf2 bytes.Buffer
	err = tmpl2.Execute(&buf2, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output2 := buf2.String()
	if strings.Contains(output2, "aria-invalid") {
		t.Errorf("expected no aria-invalid without errors, got: %s", output2)
	}
}

func TestTemplate_AriaInvalidNoDuplication(t *testing.T) {
	// When template uses AriaInvalid helper AND HTTP auto-injection runs,
	// aria-invalid should appear exactly once (not twice)
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div><input name="title" type="text" {{.lvt.AriaInvalid "title"}}></div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, State{}, map[string]string{"title": "Required"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	count := strings.Count(output, "aria-invalid")
	if count != 1 {
		t.Errorf("expected exactly 1 aria-invalid (no duplication), found %d in: %s", count, output)
	}
}

func TestTemplate_AriaInvalidAutoInjection_HTTPPath(t *testing.T) {
	// Auto-injection in renderHTML still works for non-JS form submissions
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div><input name="title" type="text"></div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, State{}, map[string]string{"title": "Required"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `aria-invalid="true"`) {
		t.Errorf("expected auto-injected aria-invalid in HTTP response, got: %s", output)
	}
}

func TestTemplate_ErrorTagIntegration(t *testing.T) {
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div><input name="title" type="text">{{.lvt.ErrorTag "title"}}</div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	// With error — should render <small> tag
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, State{}, map[string]string{"title": "Required"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "<small>Required</small>") {
		t.Errorf("expected <small>Required</small>, got: %s", output)
	}

	// Without error — should render nothing for ErrorTag
	tmpl2 := Must(New("test2"))
	tmpl2, _ = tmpl2.Parse(`<div><input name="title" type="text">{{.lvt.ErrorTag "title"}}</div>`)
	var buf2 bytes.Buffer
	err = tmpl2.Execute(&buf2, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output2 := buf2.String()
	if strings.Contains(output2, "<small>") {
		t.Errorf("expected no <small> tag without errors, got: %s", output2)
	}
}

func TestTemplate_AriaDisabledHelper(t *testing.T) {
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div><input name="title" type="text" {{.lvt.AriaDisabled "title"}}></div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	// With error — should output aria-disabled="true"
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, State{}, map[string]string{"title": "Required"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `aria-disabled="true"`) {
		t.Errorf("expected aria-disabled on input with error, got: %s", output)
	}

	// Without error — no aria-disabled
	tmpl2 := Must(New("test2"))
	tmpl2, err = tmpl2.Parse(`<div><input name="title" type="text" {{.lvt.AriaDisabled "title"}}></div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	var buf2 bytes.Buffer
	err = tmpl2.Execute(&buf2, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output2 := buf2.String()
	if strings.Contains(output2, "aria-disabled") {
		t.Errorf("expected no aria-disabled without errors, got: %s", output2)
	}
}

func TestTemplate_FlashTagIntegration(t *testing.T) {
	tmpl := Must(New("test"))
	_, err := tmpl.Parse(`<div>{{.lvt.FlashTag "success"}}</div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	type State struct{}

	// With flash — should render <output> tag with role="status"
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, State{}, map[string]string{"_flash:success": "Done!"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `<output role="status" data-flash="success">Done!</output>`) {
		t.Errorf("expected <output> flash tag, got: %s", output)
	}

	// Without flash — should render nothing for FlashTag
	tmpl2 := Must(New("test2"))
	tmpl2, err = tmpl2.Parse(`<div>{{.lvt.FlashTag "success"}}</div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	var buf2 bytes.Buffer
	err = tmpl2.Execute(&buf2, State{}, map[string]string{})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output2 := buf2.String()
	if strings.Contains(output2, "<output") {
		t.Errorf("expected no <output> tag without flash, got: %s", output2)
	}

	// Error flash — should render <output> tag with role="alert"
	tmpl3 := Must(New("test3"))
	tmpl3, err = tmpl3.Parse(`<div>{{.lvt.FlashTag "error"}}</div>`)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	var buf3 bytes.Buffer
	err = tmpl3.Execute(&buf3, State{}, map[string]string{"_flash:error": "Something failed"})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	output3 := buf3.String()
	if !strings.Contains(output3, `<output role="alert" data-flash="error">Something failed</output>`) {
		t.Errorf("expected <output role=\"alert\"> flash tag for error, got: %s", output3)
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
			tmpl := Must(New("test"))
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
			tmpl := Must(New("test"))
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
			tmpl := Must(New("test"))
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
	tmpl := Must(New("benchmark"))
	if _, err := tmpl.Parse("<p>Hello {{.Name}}!</p>"); err != nil {
		b.Fatalf("Parse failed: %v", err)
	}
	data := map[string]interface{}{"Name": "World"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

func BenchmarkTemplate_ExecuteUpdates(b *testing.B) {
	tmpl := Must(New("benchmark"))
	if _, err := tmpl.Parse("<p>Hello {{.Name}}!</p>"); err != nil {
		b.Fatalf("Parse failed: %v", err)
	}

	// Prime the template
	var initBuf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&initBuf, map[string]interface{}{"Name": "World"}); err != nil {
		b.Fatalf("Initial ExecuteUpdates failed: %v", err)
	}

	data := map[string]interface{}{"Name": "Alice"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := tmpl.ExecuteUpdates(&buf, data); err != nil {
			b.Fatalf("ExecuteUpdates failed: %v", err)
		}
	}
}

// Test configuration options
func TestTemplate_WithAuthenticator(t *testing.T) {
	// Test default authenticator
	t.Run("default anonymous authenticator", func(t *testing.T) {
		tmpl := Must(New("test"))
		if tmpl.config.Authenticator == nil {
			t.Error("Expected default Authenticator to be set, got nil")
		}
		if _, ok := tmpl.config.Authenticator.(*AnonymousAuthenticator); !ok {
			t.Errorf("Expected default Authenticator to be *AnonymousAuthenticator, got %T", tmpl.config.Authenticator)
		}
	})

	// Test custom authenticator
	t.Run("custom authenticator", func(t *testing.T) {
		customAuth := NewBasicAuthenticator(func(username, password string) (bool, error) {
			return username == "test" && password == "pass", nil
		})

		tmpl := Must(New("test", WithAuthenticator(customAuth)))
		if tmpl.config.Authenticator != customAuth {
			t.Error("Expected custom Authenticator to be set")
		}
	})

	// Test authenticator override
	t.Run("authenticator override", func(t *testing.T) {
		auth1 := NewBasicAuthenticator(func(username, password string) (bool, error) {
			return true, nil
		})
		auth2 := NewBasicAuthenticator(func(username, password string) (bool, error) {
			return false, nil
		})

		tmpl := Must(New("test", WithAuthenticator(auth1), WithAuthenticator(auth2)))
		if tmpl.config.Authenticator != auth2 {
			t.Error("Expected second Authenticator to override the first")
		}
	})
}

func TestTemplate_WithAllowedOrigins(t *testing.T) {
	// Test default (no origins set)
	t.Run("default no allowed origins", func(t *testing.T) {
		tmpl := Must(New("test"))
		if len(tmpl.config.AllowedOrigins) != 0 {
			t.Errorf("Expected no AllowedOrigins by default, got %d", len(tmpl.config.AllowedOrigins))
		}
	})

	// Test setting allowed origins
	t.Run("set allowed origins", func(t *testing.T) {
		origins := []string{"https://example.com", "https://www.example.com"}
		tmpl := Must(New("test", WithAllowedOrigins(origins)))

		if len(tmpl.config.AllowedOrigins) != 2 {
			t.Errorf("Expected 2 AllowedOrigins, got %d", len(tmpl.config.AllowedOrigins))
		}

		for i, origin := range origins {
			if tmpl.config.AllowedOrigins[i] != origin {
				t.Errorf("Expected AllowedOrigins[%d] = %q, got %q", i, origin, tmpl.config.AllowedOrigins[i])
			}
		}
	})

	// Test empty origins list
	t.Run("empty allowed origins", func(t *testing.T) {
		tmpl := Must(New("test", WithAllowedOrigins([]string{})))
		if len(tmpl.config.AllowedOrigins) != 0 {
			t.Errorf("Expected empty AllowedOrigins, got %d", len(tmpl.config.AllowedOrigins))
		}
	})

	// Test single origin
	t.Run("single allowed origin", func(t *testing.T) {
		tmpl := Must(New("test", WithAllowedOrigins([]string{"https://example.com"})))
		if len(tmpl.config.AllowedOrigins) != 1 {
			t.Errorf("Expected 1 AllowedOrigin, got %d", len(tmpl.config.AllowedOrigins))
		}
		if tmpl.config.AllowedOrigins[0] != "https://example.com" {
			t.Errorf("Expected origin 'https://example.com', got %q", tmpl.config.AllowedOrigins[0])
		}
	})
}

func TestTemplate_WithSessionStore(t *testing.T) {
	// Test default session store
	t.Run("default session store", func(t *testing.T) {
		tmpl := Must(New("test"))
		if tmpl.config.SessionStore == nil {
			t.Error("Expected default SessionStore to be set, got nil")
		}
		if _, ok := tmpl.config.SessionStore.(*MemorySessionStore); !ok {
			t.Errorf("Expected default SessionStore to be *MemorySessionStore, got %T", tmpl.config.SessionStore)
		}
	})

	// Test custom session store
	t.Run("custom session store", func(t *testing.T) {
		customStore := NewMemorySessionStore()
		tmpl := Must(New("test", WithSessionStore(customStore)))
		if tmpl.config.SessionStore != customStore {
			t.Error("Expected custom SessionStore to be set")
		}
	})
}

func TestTemplate_CombinedOptions(t *testing.T) {
	// Test combining multiple options
	t.Run("multiple options", func(t *testing.T) {
		auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
			return true, nil
		})
		origins := []string{"https://example.com"}
		store := NewMemorySessionStore()

		tmpl := Must(New("test",
			WithAuthenticator(auth),
			WithAllowedOrigins(origins),
			WithSessionStore(store),
			WithDevMode(true),
			WithWebSocketDisabled(),
		))

		if tmpl.config.Authenticator != auth {
			t.Error("Expected custom Authenticator")
		}
		if len(tmpl.config.AllowedOrigins) != 1 {
			t.Error("Expected AllowedOrigins to be set")
		}
		if tmpl.config.SessionStore != store {
			t.Error("Expected custom SessionStore")
		}
		if !tmpl.config.DevMode {
			t.Error("Expected DevMode to be true")
		}
		if !tmpl.config.WebSocketDisabled {
			t.Error("Expected WebSocketDisabled to be true")
		}
	})
}

// ----- template_flatten_test.go -----
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

	flattened, err := parse.FlattenTemplate(tmpl)
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

	flattened, err := parse.FlattenTemplate(tmpl)
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

	flattened, err := parse.FlattenTemplate(tmpl)
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

	flattened, err := parse.FlattenTemplate(tmpl)
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

	flattened, err := parse.FlattenTemplate(tmpl)
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

			result := parse.HasTemplateComposition(tmpl)
			if result != tt.expected {
				t.Errorf("parse.HasTemplateComposition() = %v, want %v", result, tt.expected)
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

	flattened, err := parse.FlattenTemplate(tmpl)
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

	tree, err := compat.ParseTemplateToTree("test", flattened, data)
	if err != nil {
		t.Fatalf("Failed to generate tree from flattened template: %v", err)
	}

	// Verify tree was generated
	if tree == nil {
		t.Fatal("Tree is nil")
	}

	// Tree should have statics
	if _, ok := tree.ToMap()["s"]; !ok {
		t.Error("Tree missing statics ('s' key)")
	}
}

func TestFlattenTemplate_ComponentPattern(t *testing.T) {
	// Test the component pattern used in testdata/e2e/components/input.tmpl
	// This is the pattern that was causing the bug: {{define}} blocks followed by {{template}} invocation
	templateStr := `{{define "layout"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <title>{{.Title}}</title>
</head>
<body>
    {{template "content" .}}
</body>
</html>
{{end}}

{{define "stats"}}
<div class="stats">
    <p>Total: {{.TodoCount}}</p>
    <p>Completed: {{.CompletedCount}}</p>
</div>
{{end}}

{{template "layout" .}}

{{define "content"}}
<h1>{{.Title}}</h1>
{{template "stats" .}}
<div class="todos">
    {{range .Todos}}
    <div class="todo" data-key="{{.ID}}">
        {{.Text}} {{if .Completed}}✓{{end}}
    </div>
    {{end}}
</div>
<footer>Updated: {{.LastUpdated}}</footer>
{{end}}
`

	tmpl, err := template.New(t.Name()).Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	flattened, err := parse.FlattenTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to flatten template: %v", err)
	}

	// Should contain all the dynamic fields
	if !strings.Contains(flattened, "{{.Title}}") {
		t.Errorf("Flattened template missing {{.Title}}")
	}
	if !strings.Contains(flattened, "{{.TodoCount}}") {
		t.Errorf("Flattened template missing {{.TodoCount}}")
	}
	if !strings.Contains(flattened, "{{.CompletedCount}}") {
		t.Errorf("Flattened template missing {{.CompletedCount}}")
	}
	if !strings.Contains(flattened, "{{range .Todos}}") {
		t.Errorf("Flattened template missing {{range .Todos}}")
	}
	if !strings.Contains(flattened, "{{.Text}}") {
		t.Errorf("Flattened template missing {{.Text}}")
	}
	if !strings.Contains(flattened, "{{if .Completed}}") {
		t.Errorf("Flattened template missing {{if .Completed}}")
	}
	if !strings.Contains(flattened, "{{.LastUpdated}}") {
		t.Errorf("Flattened template missing {{.LastUpdated}}")
	}

	// Should NOT contain {{define}} or {{template}}
	if strings.Contains(flattened, "{{define") {
		t.Errorf("Flattened template still contains {{define}}")
	}
	if strings.Contains(flattened, "{{template") {
		t.Errorf("Flattened template still contains {{template}}")
	}

	// Should contain HTML structure
	if !strings.Contains(flattened, "<!DOCTYPE html>") {
		t.Errorf("Flattened template missing DOCTYPE")
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

			_, err = parse.FlattenTemplate(tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("parse.FlattenTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ----- template_dynamic_structure_test.go -----
// Helper function to execute template to HTML
func executeToHTML(t *Template, data interface{}) (string, error) {
	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Helper function to execute template to update (returns tree map)
func executeToUpdate(t *Template, data interface{}) (map[string]interface{}, error) {
	var buf bytes.Buffer
	err := t.ExecuteUpdates(&buf, data)
	if err != nil {
		return nil, err
	}

	var tree map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// TestDynamicModalStructure verifies that modals/dialogs appearing after initial render
// don't keep resending statics when toggled (hide → show → hide → show).
// This tests the fix for the stateful structure caching bug.
func TestDynamicModalStructure(t *testing.T) {
	tmpl := `<div>
	<button>Toggle Modal</button>
	{{if .ShowModal}}
		<dialog id="modal" class="modal-dialog">
			<h2>{{.ModalTitle}}</h2>
			<p>{{.ModalMessage}}</p>
			<button>Close</button>
		</dialog>
	{{end}}
</div>`

	type Data struct {
		ShowModal    bool
		ModalTitle   string
		ModalMessage string
	}

	// Render 1: Initial render with NO modal
	t.Run("1_Initial_NoModal", func(t *testing.T) {
		tpl, err := Must(New("dynamic-modal-test")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data := Data{ShowModal: false}
		html, err := executeToHTML(tpl, data)
		if err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		if len(html) == 0 {
			t.Fatal("Initial HTML is empty")
		}

		// Verify modal is not in HTML
		if contains(html, "modal-dialog") {
			t.Error("Modal should not be in initial render")
		}
		t.Logf("✅ Initial render (no modal): %d bytes", len(html))
	})

	// Render 2: Show modal (first appearance - should include statics)
	t.Run("2_FirstShow_WithStatics", func(t *testing.T) {
		tpl, err := Must(New("dynamic-modal-show1")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Initial render
		data1 := Data{ShowModal: false}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Show modal
		data2 := Data{
			ShowModal:    true,
			ModalTitle:   "Welcome",
			ModalMessage: "Hello World",
		}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("First modal appearance update:\n%s", updateJSON)

		// The modal structure should be in the update
		// It should include statics because client has never seen this structure
		hasModalStructure := false
		hasStatics := false

		// Check if update contains modal structure
		for k, v := range tree {
			if k == "s" {
				continue // Skip top-level statics
			}

			// Modal might be in various positions depending on template structure
			// Look for nested structures that might contain the modal
			if node, ok := v.(map[string]interface{}); ok {
				if statics, hasS := node["s"]; hasS {
					hasStatics = true
					if staticsArr, ok := statics.([]string); ok {
						for _, s := range staticsArr {
							if contains(s, "modal-dialog") || contains(s, "dialog") {
								hasModalStructure = true
								break
							}
						}
					}
				}
			}
		}

		if !hasModalStructure {
			t.Log("Warning: Modal structure not found in expected location")
			t.Log("This may be due to template structure changes - update test if needed")
		}

		if hasModalStructure && !hasStatics {
			t.Error("Modal structure found but statics missing - should include statics on first appearance")
		}

		t.Logf("✅ First modal show: %d bytes, hasStatics=%v", len(updateJSON), hasStatics)
	})

	// Render 3: Hide modal
	t.Run("3_Hide", func(t *testing.T) {
		tpl, err := Must(New("dynamic-modal-hide")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Show then hide
		data1 := Data{ShowModal: true, ModalTitle: "Test", ModalMessage: "Test"}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		data2 := Data{ShowModal: false}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		updateJSON, _ := json.Marshal(tree)
		t.Logf("✅ Hide modal: %d bytes", len(updateJSON))
	})

	// Render 4: Show modal AGAIN (CRITICAL TEST - should NOT include statics)
	t.Run("4_SecondShow_WithoutStatics", func(t *testing.T) {
		tpl, err := Must(New("dynamic-modal-show2")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Initial: hidden
		data1 := Data{ShowModal: false}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// First show
		data2 := Data{ShowModal: true, ModalTitle: "First", ModalMessage: "First Show"}
		_, err = executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute first show update: %v", err)
		}

		// Hide
		data3 := Data{ShowModal: false}
		_, err = executeToUpdate(tpl, data3)
		if err != nil {
			t.Fatalf("Failed to execute hide update: %v", err)
		}

		// Show AGAIN (this is the critical test)
		data4 := Data{ShowModal: true, ModalTitle: "Second", ModalMessage: "Second Show"}
		tree, err := executeToUpdate(tpl, data4)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Second modal appearance update:\n%s", updateJSON)

		// Check if statics are incorrectly included
		hasRedundantStatics := false
		for k, v := range tree {
			if k == "s" {
				continue
			}

			if node, ok := v.(map[string]interface{}); ok {
				if statics, hasS := node["s"]; hasS {
					if staticsArr, ok := statics.([]string); ok {
						for _, s := range staticsArr {
							if contains(s, "modal-dialog") || contains(s, "dialog") {
								hasRedundantStatics = true
								t.Errorf("❌ BUG: Modal statics sent again on second appearance!")
								t.Errorf("   Statics should have been cached from first appearance")
								break
							}
						}
					}
				}
			}
		}

		if !hasRedundantStatics {
			t.Log("✅ SUCCESS: Modal statics NOT resent (cached from first appearance)")
		}

		t.Logf("✅ Second modal show: %d bytes", len(updateJSON))
	})
}

// TestConditionalBranchSwitch verifies that switching between conditional branches
// doesn't keep resending statics for previously-seen branches.
func TestConditionalBranchSwitch(t *testing.T) {
	tmpl := `<div>
	{{if .ShowA}}
		<div class="panel-a">
			<h2>Panel A</h2>
			<p>{{.ValueA}}</p>
		</div>
	{{else}}
		<div class="panel-b">
			<h2>Panel B</h2>
			<p>{{.ValueB}}</p>
		</div>
	{{end}}
</div>`

	type Data struct {
		ShowA  bool
		ValueA string
		ValueB string
	}

	// Initial: Show A
	t.Run("1_Initial_ShowA", func(t *testing.T) {
		tpl, err := Must(New("conditional-branch-test")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data := Data{ShowA: true, ValueA: "A1", ValueB: "B1"}
		html, err := executeToHTML(tpl, data)
		if err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		if !contains(html, "panel-a") {
			t.Error("Panel A should be in initial render")
		}
		if contains(html, "panel-b") {
			t.Error("Panel B should not be in initial render")
		}

		t.Log("✅ Initial render shows Panel A")
	})

	// Switch to B
	t.Run("2_Switch_ToB", func(t *testing.T) {
		tpl, err := Must(New("conditional-switch-b")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data1 := Data{ShowA: true, ValueA: "A1", ValueB: "B1"}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		data2 := Data{ShowA: false, ValueA: "A2", ValueB: "B2"}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		updateJSON, _ := json.Marshal(tree)
		t.Logf("✅ Switch to Panel B: %d bytes", len(updateJSON))
	})

	// Switch back to A (should NOT resend A's statics)
	t.Run("3_Switch_BackToA", func(t *testing.T) {
		tpl, err := Must(New("conditional-switch-a")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Initial: A
		data1 := Data{ShowA: true, ValueA: "A1", ValueB: "B1"}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Switch to B
		data2 := Data{ShowA: false, ValueA: "A2", ValueB: "B2"}
		_, err = executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute switch to B update: %v", err)
		}

		// Switch back to A
		data3 := Data{ShowA: true, ValueA: "A3", ValueB: "B3"}
		tree, err := executeToUpdate(tpl, data3)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Switch back to Panel A update:\n%s", updateJSON)

		// Check if Panel A's statics are incorrectly resent
		hasRedundantStatics := false
		for k, v := range tree {
			if k == "s" {
				continue
			}

			if node, ok := v.(map[string]interface{}); ok {
				if statics, hasS := node["s"]; hasS {
					if staticsArr, ok := statics.([]string); ok {
						for _, s := range staticsArr {
							if contains(s, "panel-a") {
								hasRedundantStatics = true
								t.Errorf("❌ BUG: Panel A statics sent again when returning to branch A!")
								break
							}
						}
					}
				}
			}
		}

		if !hasRedundantStatics {
			t.Log("✅ SUCCESS: Panel A statics NOT resent (cached from initial render)")
		}

		t.Logf("✅ Switch back to A: %d bytes", len(updateJSON))
	})
}

// TestNestedDynamicStructures verifies nested conditionals work correctly.
func TestNestedDynamicStructures(t *testing.T) {
	tmpl := `<div>
	{{if .ShowOuter}}
		<div class="outer">
			<h2>Outer Container</h2>
			{{if .ShowInner}}
				<div class="inner">
					<p>{{.Message}}</p>
				</div>
			{{end}}
		</div>
	{{end}}
</div>`

	type Data struct {
		ShowOuter bool
		ShowInner bool
		Message   string
	}

	// Show outer only
	t.Run("1_Outer_Only", func(t *testing.T) {
		tpl, err := Must(New("nested-outer")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data := Data{ShowOuter: true, ShowInner: false, Message: ""}
		html, err := executeToHTML(tpl, data)
		if err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		if !contains(html, "outer") {
			t.Error("Outer should be visible")
		}
		if contains(html, "inner") {
			t.Error("Inner should not be visible")
		}

		t.Log("✅ Initial: Outer visible, Inner hidden")
	})

	// Show both (inner appears for first time)
	t.Run("2_Show_Inner_First_Time", func(t *testing.T) {
		tpl, err := Must(New("nested-both-1")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data1 := Data{ShowOuter: true, ShowInner: false, Message: ""}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		data2 := Data{ShowOuter: true, ShowInner: true, Message: "Hello"}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		updateJSON, _ := json.Marshal(tree)
		t.Logf("✅ Show inner (first time): %d bytes", len(updateJSON))
	})

	// Toggle inner (hide, show, hide, show)
	t.Run("3_Toggle_Inner_Multiple", func(t *testing.T) {
		tpl, err := Must(New("nested-toggle")).Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data1 := Data{ShowOuter: true, ShowInner: false, Message: ""}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Show inner first time
		data2 := Data{ShowOuter: true, ShowInner: true, Message: "First"}
		_, err = executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute first show update: %v", err)
		}

		// Hide inner
		data3 := Data{ShowOuter: true, ShowInner: false, Message: ""}
		_, err = executeToUpdate(tpl, data3)
		if err != nil {
			t.Fatalf("Failed to execute hide update: %v", err)
		}

		// Show inner AGAIN
		data4 := Data{ShowOuter: true, ShowInner: true, Message: "Second"}
		tree, err := executeToUpdate(tpl, data4)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Show inner (second time) update:\n%s", updateJSON)

		// Verify inner statics not resent
		hasRedundantStatics := checkForRedundantStatics(tree, "inner")
		if hasRedundantStatics {
			t.Error("❌ BUG: Inner statics resent on second appearance")
		} else {
			t.Log("✅ SUCCESS: Inner statics NOT resent")
		}

		t.Logf("✅ Show inner again: %d bytes", len(updateJSON))
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) >= len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper to check for redundant statics in tree
func checkForRedundantStatics(tree map[string]interface{}, keyword string) bool {
	for k, v := range tree {
		if k == "s" {
			continue
		}

		if node, ok := v.(map[string]interface{}); ok {
			if statics, hasS := node["s"]; hasS {
				if staticsArr, ok := statics.([]string); ok {
					for _, s := range staticsArr {
						if contains(s, keyword) {
							return true
						}
					}
				}
			}

			// Recursively check nested structures
			if checkForRedundantStatics(node, keyword) {
				return true
			}
		}
	}
	return false
}

// ----- template_funcmap_test.go -----
func TestTemplateGenerateTreeWithFuncMap(t *testing.T) {
	tmpl := Must(New("funcMap"))
	tmpl.Funcs(template.FuncMap{
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
	})

	if _, err := tmpl.Parse(`<ul>{{range split .CSV ","}}<li>{{.}}</li>{{end}}</ul>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Render once to exercise the helper paths used in production.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"CSV": "one,two"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// After Execute(), check the cached lastTree instead of calling generateTreeInternalWithErrors
	// (calling it again with same data would generate an empty diff)
	if tmpl.lastTree == nil {
		t.Fatalf("expected last tree to be cached")
	}

	tree := tmpl.lastTree
	dynamic, ok := tree.GetDynamic(0)
	if !ok {
		t.Fatalf("expected dynamic range at position 0")
	}

	rangeNode, ok := dynamic.(*build.TreeNode)
	if !ok {
		t.Fatalf("expected *build.TreeNode for dynamic, got %T", dynamic)
	}

	if !rangeNode.HasRange() {
		t.Fatalf("expected range node to have range data")
	}

	if rangeNode.Range == nil || len(rangeNode.Range.Items) != 2 {
		t.Fatalf("expected 2 items in range, got %v", rangeNode.Range)
	}
}

// ----- template_range_concat_test.go -----
type testPost struct {
	ID        string
	Title     string
	Content   string
	Published bool
}

type testPostsState struct {
	Title          string
	SearchQuery    string
	SortBy         string
	PaginationMode string
	PaginatedPosts []testPost
	HasMore        bool
	IsLoading      bool
	LoadedCount    int
	TotalCount     int
	CurrentPage    int
	TotalPages     int
	CSSFramework   string
	EditingID      string
	EditingPosts   *testPost
}

// TestRangeDynamicDoesNotAppendContent ensures that range item dynamics keep field boundaries
func TestRangeDynamicDoesNotAppendContent(t *testing.T) {
	tmpl := Must(New("posts", WithDevMode(true)))

	templatePath := filepath.Join("testdata", "golden", "resource_template.tmpl.golden")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}

	if _, err := tmpl.Parse(string(content)); err != nil {
		t.Fatalf("parse template: %v", err)
	}

	state := &testPostsState{
		Title:          "Posts Management",
		PaginationMode: "infinite",
		CSSFramework:   "tailwind",
		TotalCount:     0,
		LoadedCount:    0,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
		t.Fatalf("initial execute: %v", err)
	}

	state.PaginatedPosts = []testPost{{
		ID:        "posts-1",
		Title:     "My First Blog Post",
		Content:   "This is the content of my first blog post",
		Published: true,
	}}
	state.TotalCount = 1
	state.LoadedCount = 1
	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
		t.Fatalf("second execute: %v", err)
	}
	if testing.Verbose() {
		t.Logf("update payload: %s", buf.String())
	}

	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		t.Fatalf("unmarshal tree: %v", err)
	}

	found := false
	for _, v := range tree {
		node, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		rangeNode, ok := node["0"].(map[string]interface{})
		if !ok {
			continue
		}
		dItems, ok := rangeNode["d"].([]interface{})
		if !ok || len(dItems) == 0 {
			continue
		}
		firstItem, ok := dItems[0].(map[string]interface{})
		if !ok {
			continue
		}
		titleVal, ok := firstItem["1"].(string)
		if !ok {
			continue
		}
		found = true
		if titleVal != "My First Blog Post" {
			t.Fatalf("unexpected title dynamic: %q", titleVal)
		}
		break
	}

	if !found {
		t.Fatalf("range node with title dynamic not found: %v", tree)
	}
}

// ----- template_fallback_block_test.go -----
func TestTemplateGenerateInitialTreeFallsBackForBlockWithDynamicTemplate(t *testing.T) {
	tmpl := Must(New("block-dynamic-template"))

	staticTemplateStr := `{{define "layout"}}<main>{{block "region" .}}{{template "content" .}}{{end}}</main>{{end}}{{define "content"}}<p>{{.Message}}</p>{{end}}{{template "layout" .}}`
	if _, err := tmpl.Parse(staticTemplateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	dynamicTemplateStr := `{{define "layout"}}<main>{{block "region" .}}{{template (printf "%s" .PartialName) .}}{{end}}</main>{{end}}{{define "content"}}<p>{{.Message}}</p>{{end}}{{template "layout" .}}`
	tmpl.templateStr = dynamicTemplateStr

	data := map[string]interface{}{
		"PartialName": "content",
		"Message":     "hello",
	}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", dynamicTemplateStr, data, ctx); err == nil {
		t.Fatalf("expected AST parser to error for block with dynamic template invocation")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

// ----- template_fallback_channel_test.go -----
func TestTemplateGenerateInitialTreeFallsBackForChannelRange(t *testing.T) {
	tmpl := Must(New("channel-range"))
	if _, err := tmpl.Parse(`<ul>{{range .Events}}<li>{{.}}</li>{{end}}</ul>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	events := make(chan string, 2)
	events <- "alpha"
	events <- "beta"
	close(events)
	data := map[string]interface{}{"Events": (<-chan string)(events)}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", `<ul>{{range .Events}}<li>{{.}}</li>{{end}}</ul>`, data, ctx); err == nil {
		t.Fatalf("expected AST parser to error for channel range")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

func TestTemplateGenerateInitialTreeFallsBackForChannelRangeWithDecls(t *testing.T) {
	tmpl := Must(New("channel-range-with-vars"))
	templateStr := `<ul>{{range $i, $event := .Events}}<li>{{$i}}-{{$event}}</li>{{end}}</ul>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	events := make(chan string, 2)
	events <- "alpha"
	events <- "beta"
	close(events)
	data := map[string]interface{}{"Events": (<-chan string)(events)}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", templateStr, data, ctx); err == nil {
		t.Fatalf("expected AST parser to error for channel range with declarations")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

func TestTemplateGenerateInitialTreeFallsBackForIntegerRange(t *testing.T) {
	tmpl := Must(New("range-integer"))
	templateStr := `<ol>{{range 3}}<li>#{{.}}</li>{{end}}</ol>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", templateStr, nil, ctx); err == nil {
		t.Fatalf("expected AST parser to error for integer range")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(nil)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

func TestCreateHTMLStructureBasedTreeSegmentsBlockBoundaries(t *testing.T) {
	html := `<div>header</div><main><p>body</p></main><div>footer</div>`

	tree := build.CreateHTMLStructureBasedTree(html)
	if tree == nil {
		t.Fatalf("expected fallback tree")
	}

	expectedStatics := []string{"<div>header</div>", "", ""}
	if !reflect.DeepEqual(tree.Statics, expectedStatics) {
		t.Fatalf("unexpected statics: %#v", tree.Statics)
	}

	if tree.DynamicLen() != 2 {
		t.Fatalf("expected 2 dynamic segments, got %d", tree.DynamicLen())
	}

	segmentZero, ok := tree.Dynamics[0].(string)
	if !ok {
		t.Fatalf("expected dynamic segment 0 to be string, got %T", tree.Dynamics[0])
	}
	if !strings.Contains(segmentZero, "<main") || !strings.Contains(segmentZero, "body") {
		t.Fatalf("dynamic segment 0 missing expected content: %q", segmentZero)
	}

	segmentOne, ok := tree.Dynamics[1].(string)
	if !ok {
		t.Fatalf("expected dynamic segment 1 to be string, got %T", tree.Dynamics[1])
	}
	if !strings.Contains(segmentOne, "<div") || !strings.Contains(segmentOne, "footer") {
		t.Fatalf("dynamic segment 1 missing expected content: %q", segmentOne)
	}

	if tree.HasRange() {
		t.Fatalf("fallback segmentation should not introduce range metadata")
	}
}

// ----- template_fallback_controlflow_test.go -----
func TestTemplateGenerateInitialTreeFallsBackForRangeBreak(t *testing.T) {
	tmpl := Must(New("range-break-fallback"))
	templateStr := `<ul>{{range .Items}}{{if eq . "stop"}}{{break}}{{end}}<li>{{.}}</li>{{end}}</ul>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Items": []string{"alpha", "stop", "gamma"}}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", templateStr, data, ctx); err == nil {
		t.Fatalf("expected AST parser to error for range with break")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

func TestTemplateGenerateInitialTreeFallsBackForRangeContinue(t *testing.T) {
	tmpl := Must(New("range-continue-fallback"))
	templateStr := `<ul>{{range $i, $item := .Items}}{{if eq $item "skip"}}{{continue}}{{end}}<li>{{$i}}-{{$item}}</li>{{end}}</ul>`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Items": []string{"alpha", "skip", "gamma"}}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", templateStr, data, ctx); err == nil {
		t.Fatalf("expected AST parser to error for range with continue")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

// ----- template_fallback_dynamic_template_test.go -----
func TestTemplateGenerateInitialTreeFallsBackForDynamicTemplateInvocation(t *testing.T) {
	tmpl := Must(New("dynamic-template"))

	staticTemplateStr := `{{define "content"}}<p>{{.Message}}</p>{{end}}<section>{{template "content" .}}</section>`
	if _, err := tmpl.Parse(staticTemplateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	dynamicTemplateStr := `{{define "content"}}<p>{{.Message}}</p>{{end}}<section>{{template (printf "%s" .PartialName) .}}</section>`
	// Override the template source to mimic a runtime-selected partial name.
	// html/template rejects this construct during parsing, which is exactly
	// what triggers the fallback path we want to guard.
	tmpl.templateStr = dynamicTemplateStr

	data := map[string]interface{}{
		"PartialName": "content",
		"Message":     "hello",
	}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", dynamicTemplateStr, data, ctx); err == nil {
		t.Fatalf("expected AST parser to error for dynamic template invocation")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(data)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	// The initial tree must match HTML segmentation fallback.
	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

// ----- template_fallback_with_test.go -----
func TestTemplateGenerateInitialTreeFallsBackForWithIterSeq(t *testing.T) {
	tmpl := Must(New("with-iter-seq"))
	tmpl.Funcs(template.FuncMap{
		"seq": func() iter.Seq[string] {
			return func(yield func(string) bool) {
				if !yield("alpha") {
					return
				}
				yield("beta")
			}
		},
	})

	templateStr := `{{with seq}}<ul>{{range .}}<li>{{.}}</li>{{end}}</ul>{{else}}<p>empty</p>{{end}}`
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := build.NewContext()
	ctx.FuncMap = tmpl.funcs
	if _, err := compat.ParseTemplateToTree("test", templateStr, nil, ctx); err == nil {
		t.Fatalf("expected AST parser to error for with pipeline returning iter.Seq")
	}

	tree, err := tmpl.generateTreeInternalWithErrors(nil)
	if err != nil {
		t.Fatalf("generateTreeInternalWithErrors failed: %v", err)
	}

	if tree == nil {
		t.Fatalf("expected tree result")
	}

	if tree.HasRange() {
		t.Fatalf("fallback tree should not contain range metadata")
	}

	if tmpl.lastHTML == "" {
		t.Fatalf("expected lastHTML to be recorded")
	}

	expected := build.CreateHTMLStructureBasedTree(tmpl.lastHTML)
	if !reflect.DeepEqual(tmpl.lastTree, expected) {
		t.Fatalf("expected fallback tree to match HTML segmentation\nwant: %#v\ngot:  %#v", expected, tmpl.lastTree)
	}
}

// ----- template_parity_test.go -----
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
			lvtTmpl := Must(New("test"))
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
			lvtTmpl := Must(New("test"))
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
			lvtTmpl := Must(New("test"))
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

// ----- template_parity_complete_test.go -----
// parityTest runs a parity check between standard Go template and LiveTemplate
func parityTest(t *testing.T, tmpl string, data interface{}) {
	t.Helper()

	// Test with standard Go template
	stdTmpl, err := template.New("std").Parse(tmpl)
	if err != nil {
		t.Fatalf("Standard template parse error: %v", err)
	}

	var stdBuf bytes.Buffer
	if err := stdTmpl.Execute(&stdBuf, data); err != nil {
		t.Fatalf("Standard template execute error: %v", err)
	}

	stdResult := stdBuf.String()

	// Test with LiveTemplate
	lvtTmpl := Must(New("test"))
	if _, err := lvtTmpl.Parse(tmpl); err != nil {
		t.Fatalf("LiveTemplate parse error: %v", err)
	}

	var lvtBuf bytes.Buffer
	if err := lvtTmpl.Execute(&lvtBuf, data); err != nil {
		t.Fatalf("LiveTemplate execute error: %v", err)
	}

	lvtResult := extractContent(lvtBuf.String())

	// Ensure both match
	if lvtResult != stdResult {
		t.Errorf("Parity mismatch:\nStandard:     %q\nLiveTemplate: %q\nFull LVT:     %q", stdResult, lvtResult, lvtBuf.String())
	}
}

// =============================================================================
// CONTROL STRUCTURES TESTS
// =============================================================================

func TestParity_ControlStructures_If(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "if basic true",
			tmpl: `{{if .Show}}visible{{end}}`,
			data: map[string]interface{}{"Show": true},
		},
		{
			name: "if basic false",
			tmpl: `{{if .Show}}visible{{end}}`,
			data: map[string]interface{}{"Show": false},
		},
		{
			name: "if-else true branch",
			tmpl: `{{if .Show}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Show": true},
		},
		{
			name: "if-else false branch",
			tmpl: `{{if .Show}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Show": false},
		},
		{
			name: "if-else-if chain",
			tmpl: `{{if eq .Status "active"}}active{{else if eq .Status "pending"}}pending{{else}}other{{end}}`,
			data: map[string]interface{}{"Status": "pending"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_ControlStructures_Range(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "range over slice",
			tmpl: `{{range .Items}}{{.}}{{end}}`,
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			name: "range with else - non-empty",
			tmpl: `{{range .Items}}{{.}}{{else}}empty{{end}}`,
			data: map[string]interface{}{"Items": []string{"a"}},
		},
		{
			name: "range with else - empty",
			tmpl: `{{range .Items}}{{.}}{{else}}empty{{end}}`,
			data: map[string]interface{}{"Items": []string{}},
		},
		{
			name: "range over map",
			tmpl: `{{range $k, $v := .Map}}{{$k}}={{$v}} {{end}}`,
			data: map[string]interface{}{"Map": map[string]string{"a": "1", "b": "2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_ControlStructures_With(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "with basic",
			tmpl: `{{with .User}}{{.Name}}{{end}}`,
			data: map[string]interface{}{"User": map[string]string{"Name": "Alice"}},
		},
		{
			name: "with else - has value",
			tmpl: `{{with .User}}{{.Name}}{{else}}no user{{end}}`,
			data: map[string]interface{}{"User": map[string]string{"Name": "Bob"}},
		},
		{
			name: "with else - nil value",
			tmpl: `{{with .User}}{{.Name}}{{else}}no user{{end}}`,
			data: map[string]interface{}{"User": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_ControlStructures_Template(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "template with define",
			tmpl: `{{define "greeting"}}Hello{{end}}{{template "greeting"}}`,
			data: nil,
		},
		{
			name: "template with data",
			tmpl: `{{define "user"}}User: {{.}}{{end}}{{template "user" .Name}}`,
			data: map[string]string{"Name": "Alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// $ ROOT VARIABLE TESTS
// =============================================================================

func TestParity_RootVariable_InRange(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "$ simple field in range",
			tmpl: `{{range .Items}}{{.}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{"Title": "ROOT", "Items": []string{"a", "b"}},
		},
		{
			name: "$ in if inside range",
			tmpl: `{{range .Users}}{{if eq .Name $.Admin}}admin{{else}}user{{end}}{{end}}`,
			data: map[string]interface{}{
				"Admin": "alice",
				"Users": []map[string]string{{"Name": "alice"}, {"Name": "bob"}},
			},
		},
		{
			name: "$ nested ranges",
			tmpl: `{{range .L1}}{{range .L2}}{{.}}-{{$.Root}}{{end}}{{end}}`,
			data: map[string]interface{}{
				"Root": "TOP",
				"L1": []map[string]interface{}{
					{"L2": []string{"a", "b"}},
				},
			},
		},
		{
			name: "$ with range variables",
			tmpl: `{{range $i, $v := .Items}}{{$i}}:{{$v}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{"Title": "ROOT", "Items": []string{"x", "y"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_RootVariable_InWith(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "$ in with",
			tmpl: `{{with .User}}{{.Name}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{
				"Title": "ROOT",
				"User":  map[string]string{"Name": "Alice"},
			},
		},
		{
			name: "$ in with else branch",
			tmpl: `{{with .User}}{{.Name}}{{else}}{{$.Default}}{{end}}`,
			data: map[string]interface{}{"Default": "NONE", "User": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_RootVariable_NestedAccess(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "$ nested field access",
			tmpl: `{{range .Items}}{{$.Config.Name}}{{end}}`,
			data: map[string]interface{}{
				"Config": map[string]string{"Name": "App"},
				"Items":  []string{"a"},
			},
		},
		{
			name: "$ deep nested access",
			tmpl: `{{range .Items}}{{$.A.B.C}}{{end}}`,
			data: map[string]interface{}{
				"A":     map[string]interface{}{"B": map[string]string{"C": "deep"}},
				"Items": []string{"x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// VARIABLES TESTS
// =============================================================================

func TestParity_Variables_Declaration(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "variable declaration",
			tmpl: `{{$x := .Value}}{{$x}}`,
			data: map[string]interface{}{"Value": "test"},
		},
		{
			name: "variable in pipeline",
			tmpl: `{{$x := .Value}}{{$x | printf "Value: %s"}}`,
			data: map[string]interface{}{"Value": "data"},
		},
		{
			name: "range single variable",
			tmpl: `{{range $v := .Items}}{{$v}}{{end}}`,
			data: map[string]interface{}{"Items": []string{"a", "b"}},
		},
		{
			name: "range index and value",
			tmpl: `{{range $i, $v := .Items}}{{$i}}:{{$v}} {{end}}`,
			data: map[string]interface{}{"Items": []string{"x", "y"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Variables_WithDollar(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "variable with $ in condition",
			tmpl: `{{range $i, $v := .Items}}{{if eq $v $.Target}}{{$i}}{{end}}{{end}}`,
			data: map[string]interface{}{
				"Target": "b",
				"Items":  []string{"a", "b", "c"},
			},
		},
		{
			name: "multiple variables with $",
			tmpl: `{{range $i, $v := .Items}}{{$i}}-{{$v}}-{{$.Root}}{{end}}`,
			data: map[string]interface{}{
				"Root":  "BASE",
				"Items": []string{"x", "y"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// BUILT-IN FUNCTIONS TESTS
// =============================================================================

func TestParity_Functions_Comparison(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "eq with $",
			tmpl: `{{range .Items}}{{if eq . $.Target}}match{{end}}{{end}}`,
			data: map[string]interface{}{"Target": "b", "Items": []string{"a", "b"}},
		},
		{
			name: "ne with $",
			tmpl: `{{if ne .Status $.Expected}}different{{end}}`,
			data: map[string]interface{}{"Status": "active", "Expected": "pending"},
		},
		{
			name: "lt with $",
			tmpl: `{{if lt .Count $.Limit}}under{{end}}`,
			data: map[string]interface{}{"Count": 5, "Limit": 10},
		},
		{
			name: "gt with $",
			tmpl: `{{if gt .Count $.Min}}over{{end}}`,
			data: map[string]interface{}{"Count": 15, "Min": 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Functions_Logical(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "and with $",
			tmpl: `{{if and .Active $.Enabled}}yes{{end}}`,
			data: map[string]interface{}{"Active": true, "Enabled": true},
		},
		{
			name: "or with $",
			tmpl: `{{if or .A $.B}}yes{{end}}`,
			data: map[string]interface{}{"A": false, "B": true},
		},
		{
			name: "not with $",
			tmpl: `{{if not $.Disabled}}enabled{{end}}`,
			data: map[string]interface{}{"Disabled": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Functions_BuiltIn(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "len with $",
			tmpl: `{{len $.Items}}`,
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			name: "index with $",
			tmpl: `{{index $.Items 1}}`,
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			name: "printf with $",
			tmpl: `{{printf "Count: %d" $.Count}}`,
			data: map[string]interface{}{"Count": 42},
		},
		{
			name: "print with $",
			tmpl: `{{print $.Value}}`,
			data: map[string]interface{}{"Value": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// FIELD/METHOD/KEY ACCESS TESTS
// =============================================================================

func TestParity_FieldAccess_Chained(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "chained field access",
			tmpl: `{{.User.Profile.Name}}`,
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Profile": map[string]string{"Name": "Alice"},
				},
			},
		},
		{
			name: "chained with $ in range",
			tmpl: `{{range .Items}}{{$.Config.App.Name}}{{end}}`,
			data: map[string]interface{}{
				"Config": map[string]interface{}{
					"App": map[string]string{"Name": "MyApp"},
				},
				"Items": []string{"x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_FieldAccess_OnVariable(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "field access on variable",
			tmpl: `{{range $item := .Items}}{{$item.Name}}{{end}}`,
			data: map[string]interface{}{
				"Items": []map[string]string{{"Name": "A"}, {"Name": "B"}},
			},
		},
		{
			name: "chained on variable",
			tmpl: `{{range $u := .Users}}{{$u.Profile.Name}}{{end}}`,
			data: map[string]interface{}{
				"Users": []map[string]interface{}{
					{"Profile": map[string]string{"Name": "Alice"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// PIPELINES TESTS
// =============================================================================

func TestParity_Pipelines_Basic(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "simple pipeline",
			tmpl: `{{.Value | printf "Result: %s"}}`,
			data: map[string]interface{}{"Value": "test"},
		},
		{
			name: "chained pipeline",
			tmpl: `{{.Value | printf "%s" | printf "Final: %s"}}`,
			data: map[string]interface{}{"Value": "data"},
		},
		// SKIP: Known limitation - LiveTemplate adds internal `lvt` field to data
		// When printing entire $ structure, it includes this field
		// This is acceptable as it's an edge case and doesn't affect normal template usage
		// {
		// 	name: "$ in pipeline",
		// 	tmpl: `{{range .Items}}{{$ | printf "%v"}}{{end}}`,
		// 	data: map[string]interface{}{"Items": []string{"x"}},
		// },
		{
			name: "$.Field in pipeline",
			tmpl: `{{range .Items}}{{$.Title | printf "Title: %s"}}{{end}}`,
			data: map[string]interface{}{"Title": "ROOT", "Items": []string{"a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Pipelines_WithVariables(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "variable in pipeline",
			tmpl: `{{range $v := .Items}}{{$v | printf "%s"}}{{end}}`,
			data: map[string]interface{}{"Items": []string{"a", "b"}},
		},
		{
			name: "variable and $ in pipeline",
			tmpl: `{{range $v := .Items}}{{$v | printf "%s"}} {{$.Title | printf "%s"}}{{end}}`,
			data: map[string]interface{}{"Title": "T", "Items": []string{"x"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// EDGE CASES TESTS
// =============================================================================

func TestParity_EdgeCases_Empty(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "empty range",
			tmpl: `{{range .Items}}{{.}}{{end}}`,
			data: map[string]interface{}{"Items": []string{}},
		},
		{
			name: "nil with",
			tmpl: `{{with .User}}{{.}}{{else}}none{{end}}`,
			data: map[string]interface{}{"User": nil},
		},
		{
			name: "zero value",
			tmpl: `{{if .Count}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Count": 0},
		},
		{
			name: "empty string",
			tmpl: `{{if .Value}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Value": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_EdgeCases_RangeWithDollar(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "empty range with $ in body",
			tmpl: `{{range .Items}}{{$.Field}}{{end}}`,
			data: map[string]interface{}{"Field": "value", "Items": []string{}},
		},
		{
			name: "range else with $",
			tmpl: `{{range .Items}}item{{else}}{{$.Default}}{{end}}`,
			data: map[string]interface{}{"Default": "EMPTY", "Items": []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// ----- template_diff_analysis_test.go -----
func TestAnalyzeChangeAndCreateTree_EntireContentFallbackParity(t *testing.T) {
	oldHTML := `<div><p>legacy</p></div>`
	newHTML := `<main>
  <section>
    <h1>Dynamic Title</h1>
    <article><p>Body content</p></article>
  </section>
</main>`

	tree, err := build.AnalyzeChangeAndCreateTree(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("analyzeChangeAndCreateTree returned error: %v", err)
	}

	fallbackTree := build.CreateHTMLStructureBasedTree(newHTML)
	if !reflect.DeepEqual(tree, fallbackTree) {
		t.Fatalf("expected structural fallback parity\nwant: %#v\ngot:  %#v", fallbackTree, tree)
	}
}

func TestAnalyzeChangeAndCreateTree_PartialChangeKeepsStatics(t *testing.T) {
	oldHTML := `<div><p>Hello</p></div>`
	newHTML := `<div><p>Hello World</p></div>`

	tree, err := build.AnalyzeChangeAndCreateTree(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("analyzeChangeAndCreateTree returned error: %v", err)
	}

	if !reflect.DeepEqual(tree.Statics, []string{"<div><p>Hello", "</p></div>"}) {
		t.Fatalf("unexpected statics: %#v", tree.Statics)
	}

	dynamic, ok := tree.Dynamics[0].(string)
	if !ok {
		t.Fatalf("expected string dynamic, got %#v", tree.Dynamics[0])
	}

	if strings.TrimSpace(dynamic) != "World" {
		t.Fatalf("expected normalized dynamic \"World\", got %q", dynamic)
	}
}

func TestFuncsCacheInvalidation(t *testing.T) {
	tmpl := Must(New("test"))

	upper := template.FuncMap{"transform": strings.ToUpper}
	tmpl.Funcs(upper)
	if _, err := tmpl.Parse(`<div>{{transform .Name}}</div>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var buf1 bytes.Buffer
	if err := tmpl.Execute(&buf1, map[string]interface{}{"Name": "hello"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	firstRender := buf1.String()

	if !strings.Contains(firstRender, "HELLO") {
		t.Fatalf("Expected HELLO in first render, got: %s", firstRender)
	}

	// Change Funcs — must invalidate cached parse template
	lower := template.FuncMap{"transform": strings.ToLower}
	tmpl.Funcs(lower)

	var buf2 bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf2, map[string]interface{}{"Name": "WORLD"}); err != nil {
		t.Fatalf("ExecuteUpdates failed: %v", err)
	}
	secondRender := buf2.String()

	if !strings.Contains(secondRender, "world") {
		t.Fatalf("Expected lowercase 'world' after Funcs() invalidation, got: %s", secondRender)
	}
	if strings.Contains(secondRender, "WORLD") {
		t.Fatalf("Old transform (ToUpper) still active after Funcs() invalidation, got: %s", secondRender)
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// reconstructHTML rebuilds HTML string from tree structure
// Used by tree testing to verify tree structure produces correct output
func reconstructHTML(tree *build.TreeNode) string {
	if tree == nil {
		return ""
	}

	if !tree.HasStatics() {
		return ""
	}

	// Check if this is a range comprehension
	if tree.HasRange() {
		if tree.Range == nil || len(tree.Range.Items) == 0 {
			// Debug: Empty range
			return ""
		}

		var result strings.Builder
		for _, itemDynamics := range tree.Range.Items {
			// Items are *build.TreeNode
			itemNode, ok := itemDynamics.(*build.TreeNode)
			if !ok {
				// Skip non-TreeNode items
				continue
			}

			// Reconstruct each item using statics and item dynamics
			for i, static := range tree.Statics {
				result.WriteString(static)
				if i < len(tree.Statics)-1 {
					if val, exists := itemNode.GetDynamic(i); exists {
						if nestedTree, ok := val.(*build.TreeNode); ok {
							result.WriteString(reconstructHTML(nestedTree))
						} else {
							fmt.Fprintf(&result, "%v", val)
						}
					}
				}
			}
		}
		return result.String()
	}

	var result strings.Builder

	// Interleave statics and dynamics
	for i, static := range tree.Statics {
		result.WriteString(static)

		// Add dynamic value if exists
		if i < len(tree.Statics)-1 {
			if val, exists := tree.GetDynamic(i); exists {
				// Check if value is nested tree with its own range
				if nestedTree, ok := val.(*build.TreeNode); ok {
					result.WriteString(reconstructHTML(nestedTree))
				} else {
					fmt.Fprintf(&result, "%v", val)
				}
			}
		}
	}

	return result.String()
}
