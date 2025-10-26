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

	// IMPORTANT: First update from empty→items sends FULL structure, not operations
	// This is correct because empty range never rendered item templates to client
	// So client needs full structure with statics for first items

	// The tree should contain append operations for empty→content transition
	foundAppendOperation := false
	for key, value := range tree {
		t.Logf("Tree key: %s, value type: %T", key, value)

		// Check if value is operations array directly
		if opsList, ok := value.([]interface{}); ok {
			for _, op := range opsList {
				if opArray, ok := op.([]interface{}); ok && len(opArray) >= 3 {
					if opType, ok := opArray[0].(string); ok && opType == "a" {
						t.Log("✅ Found append operation for first item (correct)")
						foundAppendOperation = true
						t.Logf("  Append operation includes statics: %v", len(opArray) >= 3)
						if items, ok := opArray[1].([]interface{}); ok {
							t.Logf("  Items to append: %d", len(items))
						}
					}
				}
			}
		}
	}

	if !foundAppendOperation {
		t.Error("No append operation found - expected for empty→first transition")
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

	// Verify second update tree structure
	// This time we expect an APPEND operation since the new item is at the end
	// Format: ["a", items, statics]
	foundSecondAppend := false
	for _, value := range tree2 {
		// Check if value is a TreeNode structure (map with "d" and "s")
		if treeMap, ok := value.(map[string]interface{}); ok {
			// Look for operations in the "d" key
			if dValue, hasDKey := treeMap["d"]; hasDKey {
				if opsList, ok := dValue.([]interface{}); ok {
					for _, op := range opsList {
						if opArray, ok := op.([]interface{}); ok && len(opArray) >= 3 {
							if opType, ok := opArray[0].(string); ok && opType == "a" {
								t.Log("✅ Found append operation for second item (correct)")
								foundSecondAppend = true
								t.Logf("  Append operation includes statics: %v", len(opArray) >= 3)
								if items, ok := opArray[1].([]interface{}); ok {
									t.Logf("  Items to append: %d", len(items))
								}
							}
						}
					}
				}
			}
		} else if opsList, ok := value.([]interface{}); ok {
			// Fallback: direct operations array (old format)
			for _, op := range opsList {
				if opArray, ok := op.([]interface{}); ok && len(opArray) >= 3 {
					if opType, ok := opArray[0].(string); ok && opType == "a" {
						t.Log("✅ Found append operation for second item (correct)")
						foundSecondAppend = true
						t.Logf("  Append operation includes statics: %v", len(opArray) >= 3)
						if items, ok := opArray[1].([]interface{}); ok {
							t.Logf("  Items to append: %d", len(items))
						}
					}
				}
			}
		}
	}

	if !foundSecondAppend {
		t.Error("No append operation found - expected for adding item at end")
	}
}
