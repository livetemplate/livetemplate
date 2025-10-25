package livetemplate

import (
	"encoding/json"
	"testing"
)

func TestNewTreeNode(t *testing.T) {
	tn := NewTreeNode()

	if tn == nil {
		t.Fatal("NewTreeNode returned nil")
	}
	if tn.Dynamics == nil {
		t.Error("Dynamics map should be initialized")
	}
	if len(tn.Statics) != 0 {
		t.Error("Statics should be empty")
	}
}

func TestNewTreeNodeWithStatics(t *testing.T) {
	statics := []string{"<div>", "</div>"}
	tn := NewTreeNodeWithStatics(statics)

	if tn == nil {
		t.Fatal("NewTreeNodeWithStatics returned nil")
	}
	if len(tn.Statics) != 2 {
		t.Errorf("Expected 2 statics, got %d", len(tn.Statics))
	}
	if tn.Statics[0] != "<div>" || tn.Statics[1] != "</div>" {
		t.Error("Statics not properly initialized")
	}
}

func TestTreeNode_SetDynamic(t *testing.T) {
	tn := NewTreeNode()

	tn.SetDynamic("0", "Hello")
	tn.SetDynamic("1", 42)
	tn.SetDynamic("2", true)

	if val, ok := tn.GetDynamic("0"); !ok || val != "Hello" {
		t.Error("Failed to get string dynamic")
	}
	if val, ok := tn.GetDynamic("1"); !ok || val != 42 {
		t.Error("Failed to get int dynamic")
	}
	if val, ok := tn.GetDynamic("2"); !ok || val != true {
		t.Error("Failed to get bool dynamic")
	}
}

func TestTreeNode_GetDynamic(t *testing.T) {
	tn := NewTreeNode()
	tn.SetDynamic("0", "test")

	val, ok := tn.GetDynamic("0")
	if !ok {
		t.Error("GetDynamic should return true for existing key")
	}
	if val != "test" {
		t.Errorf("Expected 'test', got %v", val)
	}

	_, ok = tn.GetDynamic("999")
	if ok {
		t.Error("GetDynamic should return false for non-existing key")
	}
}

func TestTreeNode_HasStatics(t *testing.T) {
	tn1 := NewTreeNode()
	if tn1.HasStatics() {
		t.Error("Empty TreeNode should not have statics")
	}

	tn2 := NewTreeNodeWithStatics([]string{"<div>"})
	if !tn2.HasStatics() {
		t.Error("TreeNode with statics should return true")
	}
}

func TestTreeNode_HasDynamics(t *testing.T) {
	tn := NewTreeNode()
	if tn.HasDynamics() {
		t.Error("Empty TreeNode should not have dynamics")
	}

	tn.SetDynamic("0", "test")
	if !tn.HasDynamics() {
		t.Error("TreeNode with dynamics should return true")
	}
}

func TestTreeNode_HasRange(t *testing.T) {
	tn := NewTreeNode()
	if tn.HasRange() {
		t.Error("TreeNode without range should return false")
	}

	tn.Range = NewRangeData([]interface{}{}, []string{})
	if !tn.HasRange() {
		t.Error("TreeNode with range should return true")
	}
}

func TestTreeNode_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		node     *TreeNode
		expected string
	}{
		{
			name:     "empty node",
			node:     NewTreeNode(),
			expected: `{}`,
		},
		{
			name:     "node with statics only",
			node:     NewTreeNodeWithStatics([]string{"<div>", "</div>"}),
			expected: `{"s":["<div>","</div>"]}`,
		},
		{
			name: "node with dynamics only",
			node: func() *TreeNode {
				tn := NewTreeNode()
				tn.SetDynamic("0", "Hello")
				tn.SetDynamic("1", "World")
				return tn
			}(),
			expected: `{"0":"Hello","1":"World"}`,
		},
		{
			name: "node with statics and dynamics",
			node: func() *TreeNode {
				tn := NewTreeNodeWithStatics([]string{"<h1>", "</h1>"})
				tn.SetDynamic("0", "Title")
				return tn
			}(),
			expected: `{"0":"Title","s":["<h1>","</h1>"]}`,
		},
		{
			name: "node with fingerprint",
			node: func() *TreeNode {
				tn := NewTreeNode()
				tn.Fingerprint = "abc123"
				return tn
			}(),
			expected: `{"f":"abc123"}`,
		},
		{
			name: "node with range data",
			node: func() *TreeNode {
				tn := NewTreeNode()
				tn.Range = NewRangeData(
					[]interface{}{
						[]interface{}{"u", "item-1"},
					},
					[]string{"<li>", "</li>"},
				)
				return tn
			}(),
			expected: `{"d":[["u","item-1"]]}`,
		},
		{
			name: "node with metadata",
			node: func() *TreeNode {
				tn := NewTreeNode()
				tn.Metadata = NewTreeMetadata("id")
				return tn
			}(),
			expected: `{"m":{"idKey":"id"}}`,
		},
		{
			name: "complete node",
			node: func() *TreeNode {
				tn := NewTreeNodeWithStatics([]string{"<div>", "</div>"})
				tn.SetDynamic("0", "Content")
				tn.Fingerprint = "xyz789"
				tn.Range = NewRangeData([]interface{}{}, []string{})
				tn.Metadata = NewTreeMetadata("key")
				return tn
			}(),
			expected: `{"0":"Content","d":[],"f":"xyz789","m":{"idKey":"key"},"s":["<div>","</div>"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.node)
			if err != nil {
				t.Fatalf("MarshalJSON failed: %v", err)
			}

			// Compare as JSON objects to avoid key ordering issues
			var expected, actual map[string]interface{}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}

			expectedJSON, _ := json.Marshal(expected)
			actualJSON, _ := json.Marshal(actual)
			if string(expectedJSON) != string(actualJSON) {
				t.Errorf("JSON mismatch:\nExpected: %s\nGot: %s", expectedJSON, actualJSON)
			}
		})
	}
}

func TestTreeNode_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(*testing.T, *TreeNode)
	}{
		{
			name:  "empty object",
			input: `{}`,
			check: func(t *testing.T, tn *TreeNode) {
				if tn.HasStatics() || tn.HasDynamics() || tn.Fingerprint != "" {
					t.Error("Empty JSON should create empty TreeNode")
				}
			},
		},
		{
			name:  "statics only",
			input: `{"s":["<div>","</div>"]}`,
			check: func(t *testing.T, tn *TreeNode) {
				if len(tn.Statics) != 2 {
					t.Errorf("Expected 2 statics, got %d", len(tn.Statics))
				}
				if tn.Statics[0] != "<div>" || tn.Statics[1] != "</div>" {
					t.Error("Statics not properly unmarshaled")
				}
			},
		},
		{
			name:  "dynamics only",
			input: `{"0":"Hello","1":"World"}`,
			check: func(t *testing.T, tn *TreeNode) {
				if val, ok := tn.GetDynamic("0"); !ok || val != "Hello" {
					t.Error("Dynamic 0 not properly unmarshaled")
				}
				if val, ok := tn.GetDynamic("1"); !ok || val != "World" {
					t.Error("Dynamic 1 not properly unmarshaled")
				}
			},
		},
		{
			name:  "fingerprint",
			input: `{"f":"abc123"}`,
			check: func(t *testing.T, tn *TreeNode) {
				if tn.Fingerprint != "abc123" {
					t.Errorf("Expected fingerprint 'abc123', got '%s'", tn.Fingerprint)
				}
			},
		},
		{
			name:  "range data",
			input: `{"d":[["u","item-1"]]}`,
			check: func(t *testing.T, tn *TreeNode) {
				if !tn.HasRange() {
					t.Error("Range data not unmarshaled")
				}
				if len(tn.Range.Items) != 1 {
					t.Errorf("Expected 1 range item, got %d", len(tn.Range.Items))
				}
			},
		},
		{
			name:  "metadata",
			input: `{"m":{"idKey":"id"}}`,
			check: func(t *testing.T, tn *TreeNode) {
				if tn.Metadata == nil {
					t.Error("Metadata not unmarshaled")
				}
				if tn.Metadata.IDKey != "id" {
					t.Errorf("Expected IDKey 'id', got '%s'", tn.Metadata.IDKey)
				}
			},
		},
		{
			name:  "nested dynamics",
			input: `{"0":"text","1":{"s":["<span>","</span>"],"0":"nested"}}`,
			check: func(t *testing.T, tn *TreeNode) {
				val, ok := tn.GetDynamic("1")
				if !ok {
					t.Error("Nested dynamic not found")
				}
				nested, ok := val.(map[string]interface{})
				if !ok {
					t.Error("Nested value should be a map")
				}
				if nested["0"] != "nested" {
					t.Error("Nested dynamic content incorrect")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tn TreeNode
			if err := json.Unmarshal([]byte(tt.input), &tn); err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}
			tt.check(t, &tn)
		})
	}
}

func TestTreeNode_ToMap(t *testing.T) {
	tn := NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	tn.SetDynamic("0", "Content")
	tn.Fingerprint = "xyz789"
	tn.Range = NewRangeData([]interface{}{}, []string{"<li>", "</li>"})
	tn.Metadata = NewTreeMetadata("key")

	m := tn.ToMap()

	if statics, ok := m["s"].([]string); !ok || len(statics) != 2 {
		t.Error("Statics not properly converted to map")
	}
	if m["0"] != "Content" {
		t.Error("Dynamic not properly converted to map")
	}
	if m["f"] != "xyz789" {
		t.Error("Fingerprint not properly converted to map")
	}
	if _, ok := m["d"]; !ok {
		t.Error("Range data not in map")
	}
	if meta, ok := m["m"].(map[string]interface{}); !ok || meta["idKey"] != "key" {
		t.Error("Metadata not properly converted to map")
	}
}

func TestTreeNode_FromMap(t *testing.T) {
	m := map[string]interface{}{
		"s": []interface{}{"<div>", "</div>"},
		"0": "Content",
		"f": "xyz789",
		"d": []interface{}{},
		"m": map[string]interface{}{
			"idKey": "key",
		},
	}

	tn, err := FromMap(m)
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	if len(tn.Statics) != 2 {
		t.Error("Statics not properly converted from map")
	}
	if val, ok := tn.GetDynamic("0"); !ok || val != "Content" {
		t.Error("Dynamic not properly converted from map")
	}
	if tn.Fingerprint != "xyz789" {
		t.Error("Fingerprint not properly converted from map")
	}
	if !tn.HasRange() {
		t.Error("Range not properly converted from map")
	}
	if tn.Metadata == nil || tn.Metadata.IDKey != "key" {
		t.Error("Metadata not properly converted from map")
	}
}

func TestTreeNode_Clone(t *testing.T) {
	original := NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	original.SetDynamic("0", "Content")
	original.Fingerprint = "abc123"
	original.Range = NewRangeData([]interface{}{}, []string{"<li>", "</li>"})
	original.Metadata = NewTreeMetadata("id")

	clone := original.Clone()

	// Verify clone has same values
	if len(clone.Statics) != 2 || clone.Statics[0] != "<div>" {
		t.Error("Clone statics don't match")
	}
	if val, ok := clone.GetDynamic("0"); !ok || val != "Content" {
		t.Error("Clone dynamics don't match")
	}
	if clone.Fingerprint != "abc123" {
		t.Error("Clone fingerprint doesn't match")
	}
	if !clone.HasRange() {
		t.Error("Clone range not copied")
	}
	if clone.Metadata == nil || clone.Metadata.IDKey != "id" {
		t.Error("Clone metadata doesn't match")
	}

	// Verify clone is independent (modify original)
	original.SetDynamic("0", "Modified")
	if val, _ := clone.GetDynamic("0"); val == "Modified" {
		t.Error("Clone should be independent of original")
	}
}

func TestTreeNode_NestedClone(t *testing.T) {
	nested := NewTreeNodeWithStatics([]string{"<span>", "</span>"})
	nested.SetDynamic("0", "Nested")

	parent := NewTreeNode()
	parent.SetDynamic("0", nested)

	clone := parent.Clone()

	// Get nested from clone
	clonedNested, ok := clone.GetDynamic("0")
	if !ok {
		t.Fatal("Nested node not found in clone")
	}
	clonedNestedNode, ok := clonedNested.(*TreeNode)
	if !ok {
		t.Fatal("Cloned nested should be TreeNode")
	}

	// Modify original nested
	nested.SetDynamic("0", "Modified")

	// Verify clone's nested is independent
	if val, _ := clonedNestedNode.GetDynamic("0"); val == "Modified" {
		t.Error("Cloned nested node should be independent")
	}
}

func TestRangeData_Creation(t *testing.T) {
	items := []interface{}{
		[]interface{}{"u", "item-1"},
		[]interface{}{"r", "item-2"},
	}
	statics := []string{"<li>", "</li>"}

	rd := NewRangeData(items, statics)

	if rd == nil {
		t.Fatal("NewRangeData returned nil")
	}
	if len(rd.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(rd.Items))
	}
	if len(rd.Statics) != 2 {
		t.Errorf("Expected 2 statics, got %d", len(rd.Statics))
	}
}

func TestTreeMetadata_Creation(t *testing.T) {
	meta := NewTreeMetadata("id")

	if meta == nil {
		t.Fatal("NewTreeMetadata returned nil")
	}
	if meta.IDKey != "id" {
		t.Errorf("Expected IDKey 'id', got '%s'", meta.IDKey)
	}
}

func TestTreeNode_RoundTrip(t *testing.T) {
	// Create a complex tree
	original := NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	original.SetDynamic("0", "Content")
	original.Fingerprint = "abc123"

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal back
	var restored TreeNode
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify round trip
	if len(restored.Statics) != 2 {
		t.Error("Round trip lost statics")
	}
	if val, ok := restored.GetDynamic("0"); !ok || val != "Content" {
		t.Error("Round trip lost dynamics")
	}
	if restored.Fingerprint != "abc123" {
		t.Error("Round trip lost fingerprint")
	}
}

func TestTreeNode_BackwardCompatibility(t *testing.T) {
	// This is the old format (map[string]interface{})
	oldFormat := map[string]interface{}{
		"s": []interface{}{"<div>", "</div>"},
		"0": "Hello",
		"1": map[string]interface{}{
			"s": []interface{}{"<span>", "</span>"},
			"0": "World",
		},
		"f": "abc123",
	}

	// Convert old format to JSON
	oldJSON, err := json.Marshal(oldFormat)
	if err != nil {
		t.Fatalf("Failed to marshal old format: %v", err)
	}

	// Unmarshal into new TreeNode type
	var tn TreeNode
	if err := json.Unmarshal(oldJSON, &tn); err != nil {
		t.Fatalf("Failed to unmarshal old format into TreeNode: %v", err)
	}

	// Verify it was parsed correctly
	if len(tn.Statics) != 2 {
		t.Error("Old format statics not parsed correctly")
	}
	if val, ok := tn.GetDynamic("0"); !ok || val != "Hello" {
		t.Error("Old format dynamic not parsed correctly")
	}
	if tn.Fingerprint != "abc123" {
		t.Error("Old format fingerprint not parsed correctly")
	}

	// Marshal back to JSON and verify format matches
	newJSON, err := json.Marshal(&tn)
	if err != nil {
		t.Fatalf("Failed to marshal TreeNode: %v", err)
	}

	var oldParsed, newParsed map[string]interface{}
	if err := json.Unmarshal(oldJSON, &oldParsed); err != nil {
		t.Fatalf("Failed to unmarshal old JSON: %v", err)
	}
	if err := json.Unmarshal(newJSON, &newParsed); err != nil {
		t.Fatalf("Failed to unmarshal new JSON: %v", err)
	}

	// Both should have the same keys
	if len(oldParsed) != len(newParsed) {
		t.Errorf("Key count mismatch: old=%d, new=%d", len(oldParsed), len(newParsed))
	}
}
