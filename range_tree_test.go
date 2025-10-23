package livetemplate

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestRangeTreeGeneration tests the tree generation for range constructs
// This test mimics the todos scenario: empty list -> add item -> verify tree
func TestRangeTreeGeneration(t *testing.T) {
	type Todo struct {
		ID   string
		Text string
	}

	type State struct {
		PaginatedTodos []Todo
	}

	// Template mimicking todos.tmpl
	templateStr := `
<table>
<tbody>
{{ range .PaginatedTodos }}
<tr data-key="{{.ID}}">
  <td>{{.Text}}</td>
</tr>
{{ end }}
</tbody>
</table>
`

	tmpl := New("test")
	_, err := tmpl.Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Step 1: Render with empty list (initial state)
	state1 := &State{
		PaginatedTodos: []Todo{},
	}

	var buf1 bytes.Buffer
	err = tmpl.Execute(&buf1, state1)
	if err != nil {
		t.Fatalf("Failed initial execute: %v", err)
	}

	html1 := buf1.String()
	t.Logf("Initial render HTML:\n%s", html1)

	// Check that table is empty
	if !contains(html1, "<tbody>") {
		t.Error("Initial render missing tbody")
	}

	// Step 2: Update with one item
	state2 := &State{
		PaginatedTodos: []Todo{
			{ID: "todo-1", Text: "First Todo Item"},
		},
	}

	var buf2 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf2, state2)
	if err != nil {
		t.Fatalf("Failed update execute: %v", err)
	}

	// Parse the update tree
	var tree map[string]interface{}
	if err := json.Unmarshal(buf2.Bytes(), &tree); err != nil {
		t.Fatalf("Failed to parse update JSON: %v", err)
	}

	// Pretty print the tree
	prettyJSON, _ := json.MarshalIndent(tree, "", "  ")
	t.Logf("Update tree after adding one item:\n%s", string(prettyJSON))

	// Verify tree structure
	if tree == nil {
		t.Fatal("Update tree is nil")
	}

	// The tree should contain range updates
	// Look for numeric keys or range operation structures
	foundRangeData := false
	for key, value := range tree {
		t.Logf("Tree key: %s, value type: %T", key, value)

		// Check for range comprehension structure (has "s" and "d" keys)
		if valueMap, ok := value.(map[string]interface{}); ok {
			if _, hasS := valueMap["s"]; hasS {
				if d, hasD := valueMap["d"]; hasD {
					foundRangeData = true
					t.Logf("Found range data structure: s=%v, d=%v", valueMap["s"], d)

					// Check if "d" is an array
					if dArray, ok := d.([]interface{}); ok {
						t.Logf("Range data array length: %d", len(dArray))
						if len(dArray) > 0 {
							t.Logf("First item in range: %v", dArray[0])
						} else {
							t.Error("Range data array is empty - this is the bug!")
						}
					} else {
						t.Errorf("Range data 'd' is not an array: %T", d)
					}
				}
			}
		}
	}

	if !foundRangeData {
		t.Error("No range data structure found in update tree")
	}

	// Step 3: Add a second item
	state3 := &State{
		PaginatedTodos: []Todo{
			{ID: "todo-1", Text: "First Todo Item"},
			{ID: "todo-2", Text: "Second Todo Item"},
		},
	}

	var buf3 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf3, state3)
	if err != nil {
		t.Fatalf("Failed second update execute: %v", err)
	}

	var tree2 map[string]interface{}
	if err := json.Unmarshal(buf3.Bytes(), &tree2); err != nil {
		t.Fatalf("Failed to parse second update JSON: %v", err)
	}

	prettyJSON2, _ := json.MarshalIndent(tree2, "", "  ")
	t.Logf("Update tree after adding second item:\n%s", string(prettyJSON2))
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) >= len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
