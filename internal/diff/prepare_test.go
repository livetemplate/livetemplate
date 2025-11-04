package diff

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestPrepareTreeForClient_WithoutStatics tests that when client doesn't have statics,
// everything is returned as-is without modification.
func TestPrepareTreeForClient_WithoutStatics(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name: "TreeNode with statics and dynamics",
			input: &TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{"0": "Hello"},
			},
		},
		{
			name: "map with statics",
			input: map[string]interface{}{
				"s": []string{"<span>", "</span>"},
				"0": "World",
			},
		},
		{
			name:  "primitive value",
			input: "simple string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareTreeForClient(tt.input, false)

			// When clientHasStatics=false, should return input unchanged
			if !reflect.DeepEqual(result, tt.input) {
				t.Errorf("PrepareTreeForClient() with clientHasStatics=false should return input unchanged\ngot:  %#v\nwant: %#v", result, tt.input)
			}
		})
	}
}

// TestPrepareTreeForClient_WithStatics_TreeNode tests that TreeNode statics are stripped
// when client has them cached.
func TestPrepareTreeForClient_WithStatics_TreeNode(t *testing.T) {
	tests := []struct {
		name     string
		input    *TreeNode
		expected *TreeNode
	}{
		{
			name: "simple TreeNode with statics",
			input: &TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{"0": "Hello"},
			},
			expected: &TreeNode{
				Dynamics: map[string]interface{}{"0": "Hello"},
			},
		},
		{
			name: "TreeNode with fingerprint (should be stripped)",
			input: &TreeNode{
				Statics:     []string{"<p>", "</p>"},
				Dynamics:    map[string]interface{}{"0": "Content"},
				Fingerprint: "abc123",
			},
			expected: &TreeNode{
				Dynamics: map[string]interface{}{"0": "Content"},
			},
		},
		{
			name: "TreeNode with nested TreeNode",
			input: &TreeNode{
				Statics: []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{
					"0": &TreeNode{
						Statics:  []string{"<span>", "</span>"},
						Dynamics: map[string]interface{}{"0": "Nested"},
					},
				},
			},
			expected: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": &TreeNode{
						Dynamics: map[string]interface{}{"0": "Nested"},
					},
				},
			},
		},
		{
			name: "TreeNode with empty dynamic (should be excluded)",
			input: &TreeNode{
				Statics: []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{
					"0": "Value",
					"1": "",
				},
			},
			expected: &TreeNode{
				Dynamics: map[string]interface{}{"0": "Value"},
			},
		},
		{
			name: "TreeNode with Range (statics stripped from RangeData)",
			input: &TreeNode{
				Statics:  []string{"<ul>", "</ul>"},
				Dynamics: map[string]interface{}{},
				Range: &RangeData{
					Items:   []interface{}{"item1", "item2"},
					Statics: []string{"<li>", "</li>"},
				},
			},
			expected: &TreeNode{
				Dynamics: map[string]interface{}{},
				Range: &RangeData{
					Items: []interface{}{"item1", "item2"},
					// Statics should be empty (not included)
				},
			},
		},
		{
			name: "TreeNode with Metadata (should be preserved)",
			input: &TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{"0": "Value"},
				Metadata: &TreeMetadata{IDKey: "id"},
			},
			expected: &TreeNode{
				Dynamics: map[string]interface{}{"0": "Value"},
				Metadata: &TreeMetadata{IDKey: "id"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareTreeForClient(tt.input, true)

			resultNode, ok := result.(*TreeNode)
			if !ok {
				t.Fatalf("PrepareTreeForClient() returned %T, want *TreeNode", result)
			}

			// Check Statics are removed
			if len(resultNode.Statics) > 0 {
				t.Errorf("Statics should be empty, got: %v", resultNode.Statics)
			}

			// Check Fingerprint is removed
			if resultNode.Fingerprint != "" {
				t.Errorf("Fingerprint should be empty, got: %s", resultNode.Fingerprint)
			}

			// Check Dynamics match expected
			if !reflect.DeepEqual(resultNode.Dynamics, tt.expected.Dynamics) {
				t.Errorf("Dynamics mismatch\ngot:  %#v\nwant: %#v", resultNode.Dynamics, tt.expected.Dynamics)
			}

			// Check Range handling
			if tt.expected.Range != nil {
				if resultNode.Range == nil {
					t.Error("Range should be present")
				} else {
					if !reflect.DeepEqual(resultNode.Range.Items, tt.expected.Range.Items) {
						t.Errorf("Range.Items mismatch\ngot:  %v\nwant: %v", resultNode.Range.Items, tt.expected.Range.Items)
					}
					if len(resultNode.Range.Statics) > 0 {
						t.Errorf("Range.Statics should be empty when clientHasStatics=true, got: %v", resultNode.Range.Statics)
					}
				}
			} else if resultNode.Range != nil {
				t.Error("Range should not be present")
			}

			// Check Metadata is preserved
			if !reflect.DeepEqual(resultNode.Metadata, tt.expected.Metadata) {
				t.Errorf("Metadata mismatch\ngot:  %#v\nwant: %#v", resultNode.Metadata, tt.expected.Metadata)
			}
		})
	}
}

// TestPrepareTreeForClient_WithStatics_Map tests that map-based trees have statics stripped.
func TestPrepareTreeForClient_WithStatics_Map(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "map with statics key 's'",
			input: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": "Hello",
			},
			expected: map[string]interface{}{
				"0": "Hello",
			},
		},
		{
			name: "map with fingerprint key 'f'",
			input: map[string]interface{}{
				"s": []string{"<p>", "</p>"},
				"f": "abc123",
				"0": "Content",
			},
			expected: map[string]interface{}{
				"0": "Content",
			},
		},
		{
			name: "nested map with statics",
			input: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": map[string]interface{}{
					"s": []string{"<span>", "</span>"},
					"0": "Nested",
				},
			},
			expected: map[string]interface{}{
				"0": map[string]interface{}{
					"0": "Nested",
				},
			},
		},
		{
			name: "map with empty value (should be excluded)",
			input: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": "Value",
				"1": "",
			},
			expected: map[string]interface{}{
				"0": "Value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareTreeForClient(tt.input, true)

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatalf("PrepareTreeForClient() returned %T, want map[string]interface{}", result)
			}

			// Check that 's' and 'f' keys are removed
			if _, hasS := resultMap["s"]; hasS {
				t.Error("'s' key should be removed")
			}
			if _, hasF := resultMap["f"]; hasF {
				t.Error("'f' key should be removed")
			}

			// Compare result with expected
			if !reflect.DeepEqual(resultMap, tt.expected) {
				t.Errorf("Result mismatch\ngot:  %#v\nwant: %#v", resultMap, tt.expected)
			}
		})
	}
}

// TestPrepareTreeForClient_WithStatics_Array tests array handling.
func TestPrepareTreeForClient_WithStatics_Array(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []interface{}
	}{
		{
			name: "array of primitives",
			input: []interface{}{
				"item1",
				"item2",
				"item3",
			},
			expected: []interface{}{
				"item1",
				"item2",
				"item3",
			},
		},
		{
			name: "array with maps containing statics",
			input: []interface{}{
				map[string]interface{}{
					"s": []string{"<li>", "</li>"},
					"0": "First",
				},
				map[string]interface{}{
					"s": []string{"<li>", "</li>"},
					"0": "Second",
				},
			},
			expected: []interface{}{
				map[string]interface{}{
					"0": "First",
				},
				map[string]interface{}{
					"0": "Second",
				},
			},
		},
		{
			name: "array with empty string (should be excluded)",
			input: []interface{}{
				"value1",
				"",
				"value2",
			},
			expected: []interface{}{
				"value1",
				"value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareTreeForClient(tt.input, true)

			resultArray, ok := result.([]interface{})
			if !ok {
				t.Fatalf("PrepareTreeForClient() returned %T, want []interface{}", result)
			}

			if !reflect.DeepEqual(resultArray, tt.expected) {
				t.Errorf("Result mismatch\ngot:  %#v\nwant: %#v", resultArray, tt.expected)
			}
		})
	}
}

// TestPrepareTreeForClient_Primitives tests primitive value handling.
func TestPrepareTreeForClient_Primitives(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// With statics
			result := PrepareTreeForClient(tt.input, true)
			if !reflect.DeepEqual(result, tt.input) {
				t.Errorf("Primitive values should pass through unchanged\ngot:  %#v\nwant: %#v", result, tt.input)
			}

			// Without statics
			result = PrepareTreeForClient(tt.input, false)
			if !reflect.DeepEqual(result, tt.input) {
				t.Errorf("Primitive values should pass through unchanged\ngot:  %#v\nwant: %#v", result, tt.input)
			}
		})
	}
}

// TestPrepareTreeForClient_ComplexNesting tests deeply nested structures.
func TestPrepareTreeForClient_ComplexNesting(t *testing.T) {
	input := &TreeNode{
		Statics:     []string{"<div>", "</div>"},
		Fingerprint: "root-fp",
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:     []string{"<section>", "</section>"},
				Fingerprint: "section-fp",
				Dynamics: map[string]interface{}{
					"0": map[string]interface{}{
						"s": []string{"<p>", "</p>"},
						"f": "para-fp",
						"0": "Deep content",
					},
				},
			},
			"1": []interface{}{
				map[string]interface{}{
					"s": []string{"<li>", "</li>"},
					"0": "Item 1",
				},
				map[string]interface{}{
					"s": []string{"<li>", "</li>"},
					"0": "Item 2",
				},
			},
		},
	}

	result := PrepareTreeForClient(input, true)
	resultNode, ok := result.(*TreeNode)
	if !ok {
		t.Fatalf("Expected *TreeNode, got %T", result)
	}

	// Root should have no statics or fingerprint
	if len(resultNode.Statics) > 0 {
		t.Error("Root statics should be stripped")
	}
	if resultNode.Fingerprint != "" {
		t.Error("Root fingerprint should be stripped")
	}

	// Check nested TreeNode at position "0"
	nested0, ok := resultNode.Dynamics["0"].(*TreeNode)
	if !ok {
		t.Fatalf("Dynamics['0'] should be *TreeNode, got %T", resultNode.Dynamics["0"])
	}
	if len(nested0.Statics) > 0 {
		t.Error("Nested TreeNode statics should be stripped")
	}
	if nested0.Fingerprint != "" {
		t.Error("Nested TreeNode fingerprint should be stripped")
	}

	// Check deeply nested map at position "0" -> "0"
	nestedMap, ok := nested0.Dynamics["0"].(map[string]interface{})
	if !ok {
		t.Fatalf("Dynamics['0']['0'] should be map, got %T", nested0.Dynamics["0"])
	}
	if _, hasS := nestedMap["s"]; hasS {
		t.Error("Deeply nested map should not have 's' key")
	}
	if _, hasF := nestedMap["f"]; hasF {
		t.Error("Deeply nested map should not have 'f' key")
	}
	if nestedMap["0"] != "Deep content" {
		t.Errorf("Deeply nested content = %v, want %v", nestedMap["0"], "Deep content")
	}

	// Check array at position "1"
	array, ok := resultNode.Dynamics["1"].([]interface{})
	if !ok {
		t.Fatalf("Dynamics['1'] should be []interface{}, got %T", resultNode.Dynamics["1"])
	}
	if len(array) != 2 {
		t.Errorf("Array length = %d, want 2", len(array))
	}
	for i, item := range array {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("Array item %d should be map, got %T", i, item)
			continue
		}
		if _, hasS := itemMap["s"]; hasS {
			t.Errorf("Array item %d should not have 's' key", i)
		}
	}
}

// TestPrepareTreeForClient_WireFormat tests that the result can be marshaled to JSON
// and matches expected wire format.
func TestPrepareTreeForClient_WireFormat(t *testing.T) {
	input := &TreeNode{
		Statics:     []string{"<div>", "</div>"},
		Fingerprint: "fp123",
		Dynamics: map[string]interface{}{
			"0": "Hello",
			"1": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "World"},
			},
		},
		Metadata: &TreeMetadata{IDKey: "id"},
	}

	// Prepare for wire (client has statics)
	result := PrepareTreeForClient(input, true)

	// Marshal to JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Unmarshal back to check structure
	var wireFormat map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &wireFormat); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Wire format should NOT have "s" or "f" keys
	if _, hasS := wireFormat["s"]; hasS {
		t.Error("Wire format should not contain 's' key")
	}
	if _, hasF := wireFormat["f"]; hasF {
		t.Error("Wire format should not contain 'f' key")
	}

	// Should have "0", "1", and "m" (metadata) keys
	if _, has0 := wireFormat["0"]; !has0 {
		t.Error("Wire format should contain '0' key")
	}
	if _, has1 := wireFormat["1"]; !has1 {
		t.Error("Wire format should contain '1' key")
	}
	if _, hasM := wireFormat["m"]; !hasM {
		t.Error("Wire format should contain 'm' (metadata) key")
	}

	// Check that nested TreeNode also has statics stripped
	nested1, ok := wireFormat["1"].(map[string]interface{})
	if !ok {
		t.Fatalf("wireFormat['1'] should be map, got %T", wireFormat["1"])
	}
	if _, hasS := nested1["s"]; hasS {
		t.Error("Nested TreeNode in wire format should not contain 's' key")
	}
	if nested1["0"] != "World" {
		t.Errorf("Nested content = %v, want 'World'", nested1["0"])
	}
}
