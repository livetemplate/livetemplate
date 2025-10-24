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

	// Step 1: Render with empty list (initial WebSocket tree)
	// This matches the WebSocket flow where initial state is sent via ExecuteUpdates
	state1 := &State{
		PaginatedTodos: []Todo{},
	}

	var buf1 bytes.Buffer
	err = tmpl.ExecuteUpdates(&buf1, state1)
	if err != nil {
		t.Fatalf("Failed initial ExecuteUpdates: %v", err)
	}

	var initialTree map[string]interface{}
	if err := json.Unmarshal(buf1.Bytes(), &initialTree); err != nil {
		t.Fatalf("Failed to parse initial tree JSON: %v", err)
	}

	prettyInitial, _ := json.MarshalIndent(initialTree, "", "  ")
	t.Logf("Initial tree (empty state):\n%s", string(prettyInitial))

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

	// The tree should contain range updates with operations
	// Look for append operation with statics included (first item case)
	// Statics should be at the operation level: ["a", items, statics]
	foundAppendWithStatics := false
	for key, value := range tree {
		t.Logf("Tree key: %s, value type: %T", key, value)

		// Check for range operations (array of operations)
		if opsList, ok := value.([]interface{}); ok {
			for _, op := range opsList {
				if opArray, ok := op.([]interface{}); ok && len(opArray) >= 2 {
					if opType, ok := opArray[0].(string); ok && opType == "a" {
						t.Log("Found append operation")
						// Check if operation includes statics (3rd element)
						if len(opArray) >= 3 {
							if statics, ok := opArray[2].([]interface{}); ok {
								foundAppendWithStatics = true
								t.Logf("✅ Append operation includes statics: %v", statics)
								t.Logf("First item data: %+v", opArray[1])
							} else {
								t.Error("❌ Append operation statics has wrong type")
							}
						} else {
							t.Error("❌ Append operation missing statics (only has 2 elements)")
						}
					}
				}
			}
		}
	}

	if !foundAppendWithStatics {
		t.Error("No append operation with statics found - expected for empty→first transition")
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

	// Verify second item has statics STRIPPED (optimization working)
	// Should be ["a", items] with NO third element
	foundAppendWithoutStatics := false
	for _, value := range tree2 {
		if opsList, ok := value.([]interface{}); ok {
			for _, op := range opsList {
				if opArray, ok := op.([]interface{}); ok && len(opArray) >= 2 {
					if opType, ok := opArray[0].(string); ok && opType == "a" {
						t.Log("Found append operation for second item")
						// This time statics should be stripped (optimization)
						// Operation should only have 2 elements: ["a", items]
						if len(opArray) == 2 {
							foundAppendWithoutStatics = true
							t.Log("✅ Second item statics stripped (optimization working)")
							t.Logf("Second item data: %+v", opArray[1])
						} else if len(opArray) == 3 {
							t.Error("❌ Second item includes statics - optimization not working!")
						}
					}
				}
			}
		}
	}

	if !foundAppendWithoutStatics {
		t.Log("Note: No append operation found for second item (might be update instead)")
	}
}
