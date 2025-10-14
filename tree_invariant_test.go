package livetemplate

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestTreeInvariantGuarantee(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{
			name:     "simple template",
			template: `<p>Hello {{.Name}}!</p>`,
			data:     struct{ Name string }{Name: "World"},
		},
		{
			name:     "multiple fields",
			template: `<div>Name: {{.Name}}, Age: {{.Age}}</div>`,
			data: struct {
				Name string
				Age  int
			}{Name: "Alice", Age: 30},
		},
		{
			name:     "with conditionals",
			template: `{{if .Show}}<p>Visible: {{.Text}}</p>{{else}}<p>Hidden</p>{{end}}`,
			data: struct {
				Show bool
				Text string
			}{Show: true, Text: "Hello"},
		},
		{
			name:     "complex conditionals",
			template: `<div class="{{if .Active}}active{{else}}inactive{{end}}">Status: {{if .Active}}On{{else}}Off{{end}}</div>`,
			data:     struct{ Active bool }{Active: true},
		},
		{
			name:     "nested conditionals",
			template: `{{if .User}}{{if .User.Active}}<p>{{.User.Name}} is active</p>{{else}}<p>{{.User.Name}} is inactive</p>{{end}}{{else}}<p>No user</p>{{end}}`,
			data: struct {
				User *struct {
					Name   string
					Active bool
				}
			}{User: &struct {
				Name   string
				Active bool
			}{Name: "Alice", Active: true}},
		},
		{
			name:     "with range",
			template: `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			data:     struct{ Items []string }{Items: []string{"A", "B", "C"}},
		},
		{
			name:     "complex range",
			template: `{{range .Users}}<div>{{.Name}}: {{if .Active}}✓{{else}}✗{{end}}</div>{{end}}`,
			data: struct {
				Users []struct {
					Name   string
					Active bool
				}
			}{
				Users: []struct {
					Name   string
					Active bool
				}{
					{"Alice", true},
					{"Bob", false},
				},
			},
		},
		{
			name:     "no dynamic values",
			template: `<p>Static content only</p>`,
			data:     struct{}{},
		},
		{
			name:     "empty template",
			template: ``,
			data:     struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := parseTemplateToTree(tt.template, tt.data, NewKeyGenerator())
			if err != nil {
				t.Errorf("parseTemplateToTree() error = %v", err)
				return
			}

			// Check invariant for initial tree generation
			err = checkTreeInvariant(tree, "parseTemplateToTree")
			if err != nil {
				t.Error(err)

				// Print tree for debugging
				jsonBytes, _ := json.MarshalIndent(tree, "", "  ")
				t.Logf("Tree structure:\n%s", string(jsonBytes))
			}
		})
	}
}

func TestTreeInvariantInTemplate(t *testing.T) {
	// Test with actual Template type to ensure invariant in real usage
	templateContent := `<div>
		<h1>{{.Title}}</h1>
		<p>Count: {{.Count}}</p>
		{{if .Active}}
			<div class="active">Status: {{.Status}}</div>
		{{else}}
			<div class="inactive">Inactive</div>
		{{end}}
		{{range .Items}}
			<span>{{.}}</span>
		{{end}}
	</div>`

	data := struct {
		Title  string
		Count  int
		Active bool
		Status string
		Items  []string
	}{
		Title:  "Test",
		Count:  42,
		Active: true,
		Status: "Running",
		Items:  []string{"A", "B", "C"},
	}

	// Test the parseTemplateToTree function directly (this is what Template uses internally)
	tree, err := parseTemplateToTree(templateContent, data, NewKeyGenerator())
	if err != nil {
		t.Fatalf("parseTemplateToTree error: %v", err)
	}

	err = checkTreeInvariant(tree, "Template parseTemplateToTree")
	if err != nil {
		t.Error(err)
		jsonBytes, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Tree structure:\n%s", string(jsonBytes))
	}
}

func TestE2EInvariantGuarantee(t *testing.T) {
	// Read the E2E template content from input.tmpl
	templateBytes, err := os.ReadFile("testdata/e2e/todos/input.tmpl")
	if err != nil {
		t.Fatalf("Failed to read template file: %v", err)
	}
	templateContent := string(templateBytes)

	// Test data similar to E2E test
	data := struct {
		Title          string
		Counter        int
		TodoCount      int
		CompletedCount int
		RemainingCount int
		CompletionRate float64
		Todos          []struct {
			ID        string
			Text      string
			Completed bool
			Priority  string
		}
		LastUpdated string
		SessionID   string
	}{
		Title:          "Task Manager",
		Counter:        3,
		TodoCount:      3,
		CompletedCount: 1,
		RemainingCount: 2,
		CompletionRate: 33.33,
		Todos: []struct {
			ID        string
			Text      string
			Completed bool
			Priority  string
		}{
			{"todo-1", "Learn Go templates", false, "high"},
			{"todo-2", "Build live updates", false, "medium"},
			{"todo-3", "Write documentation", true, "low"},
		},
		LastUpdated: "2023-01-01 10:15:00",
		SessionID:   "session-12345",
	}

	// Test initial tree generation using the same function as the Template
	tree, err := parseTemplateToTree(templateContent, data, NewKeyGenerator())
	if err != nil {
		t.Fatalf("parseTemplateToTree error: %v", err)
	}

	err = checkTreeInvariant(tree, "E2E parseTemplateToTree")
	if err != nil {
		t.Error(err)
		jsonBytes, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("E2E Tree structure:\n%s", string(jsonBytes))

		// Also show what expressions were found for debugging
		t.Logf("This test demonstrates that the current implementation violates the invariant")
		t.Logf("The issue is with complex expressions that evaluate to nil")
	}
}

// checkTreeInvariant verifies the statics/dynamics invariant
func checkTreeInvariant(tree TreeNode, context string) error {
	// Check if this is a dynamics-only update (no statics)
	statics, hasStatics := tree["s"]
	if !hasStatics {
		// Dynamics-only updates don't need to maintain the invariant
		return nil
	}

	// Count statics
	var staticsCount int
	if staticsArray, ok := statics.([]string); ok {
		staticsCount = len(staticsArray)
	} else {
		return fmt.Errorf("%s: statics is not a string array, got %T", context, statics)
	}

	// Check if this is a range comprehension (has "d" key with items)
	if itemsRaw, hasD := tree["d"]; hasD {
		// For range comprehensions, validate the item structure
		// The invariant is: len(statics) = len(item_dynamics) + 1

		// Get items array
		var items []interface{}
		switch v := itemsRaw.(type) {
		case []interface{}:
			items = v
		case []map[string]interface{}:
			items = make([]interface{}, len(v))
			for i, item := range v {
				items[i] = item
			}
		default:
			return fmt.Errorf("%s: range comprehension 'd' key has unexpected type: %T", context, itemsRaw)
		}

		if len(items) == 0 {
			// Empty range - no items to validate
			return nil
		}

		// Get first item to check dynamics count
		firstItem, ok := items[0].(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: range item is not a map, got %T", context, items[0])
		}

		// Count dynamics in the item (all keys are dynamics)
		itemDynamicsCount := len(firstItem)

		// Verify the invariant for range items
		if staticsCount != itemDynamicsCount+1 {
			return fmt.Errorf("%s: INVARIANT VIOLATED for range comprehension - len(statics)=%d, len(item_dynamics)=%d, expected len(statics)=len(item_dynamics)+1",
				context, staticsCount, itemDynamicsCount)
		}

		return nil
	}

	// Regular tree (not a range comprehension)
	// Count dynamics (exclude 's' and 'f')
	dynamicsCount := 0
	for k := range tree {
		if k != "s" && k != "f" { // Skip statics and fingerprint
			dynamicsCount++
		}
	}

	// Verify the invariant
	if staticsCount != dynamicsCount+1 {
		return fmt.Errorf("%s: INVARIANT VIOLATED - len(statics)=%d, len(dynamics)=%d, expected len(statics)=len(dynamics)+1",
			context, staticsCount, dynamicsCount)
	}

	return nil
}
