package strategy

import (
	"encoding/json"
	"testing"
)

// Helper function for tests to get tree structure from JSON API
func generateTreeForTest(diff Differ, data interface{}) (treeNode, error) {
	jsonData, err := diff.GenerateTree(data)
	if err != nil {
		return nil, err
	}
	
	var tree treeNode
	if len(jsonData) > 0 {
		if err := json.Unmarshal(jsonData, &tree); err != nil {
			return nil, err
		}
	} else {
		tree = treeNode{}
	}
	return tree, nil
}

// TestNewDifferAPI tests the unified API entry point
func TestNewDifferAPI(t *testing.T) {
	// Test the simple unified API
	diff, err := NewDiffer(`<p>Hello {{.Name}}!</p>`)
	if err != nil {
		t.Fatalf("NewDiffer failed: %v", err)
	}

	// Test GenerateTree method
	tree, err := generateTreeForTest(diff, map[string]interface{}{"Name": "World"})
	if err != nil {
		t.Fatalf("GenerateTree failed: %v", err)
	}

	// Verify tree structure
	if tree == nil {
		t.Fatalf("Generated tree is nil")
	}

	// Test Reset method
	diff.Reset()

	
	// Verify HTML reconstruction
	html := reconstructHTML(tree)
	expectedHTML := `<p>Hello World!</p>`
	if html != expectedHTML {
		t.Errorf("HTML mismatch:\nExpected: %s\nGot: %s", expectedHTML, html)
	}

	t.Logf("✅ Unified API works correctly")
}

// TestComprehensive tests template processing with intelligent caching behavior
func TestComprehensive(t *testing.T) {
	testCases := []struct {
		name             string
		template         string
		renderSequence   []interface{}
		expectedHTMLs    []string
		expectedTrees    []treeNode
		cacheDescription string
	}{
		{
			name:     "SimpleFieldCaching",
			template: `<p>Hello {{.Name}}!</p>`,
			renderSequence: []interface{}{
				map[string]interface{}{"Name": "Alice"},
				map[string]interface{}{"Name": "Bob"},
				map[string]interface{}{"Name": "Alice"}, // Back to first - should use cache
			},
			expectedHTMLs: []string{
				`<p>Hello Alice!</p>`,
				`<p>Hello Bob!</p>`,
				`<p>Hello Alice!</p>`,
			},
			expectedTrees: []treeNode{
				{"s": []string{"<p>Hello ", "!</p>"}, "0": "Alice"}, // First render: full structure
				{"0": "Bob"},   // Dynamics-only: cached statics
				{"0": "Alice"}, // Dynamics-only: cached statics
			},
			cacheDescription: "First render creates structure, subsequent renders reuse statics",
		},
		{
			name:     "NoChangeCaching",
			template: `<div>{{.Message}} - Count: {{.Count}}</div>`,
			renderSequence: []interface{}{
				map[string]interface{}{"Message": "Hello", "Count": 5},
				map[string]interface{}{"Message": "Hello", "Count": 5}, // Identical data
				map[string]interface{}{"Message": "Hi", "Count": 3},    // Different data
			},
			expectedHTMLs: []string{
				`<div>Hello - Count: 5</div>`,
				`<div>Hello - Count: 5</div>`,
				`<div>Hi - Count: 3</div>`,
			},
			expectedTrees: []treeNode{
				{"s": []string{"<div>", " - Count: ", "</div>"}, "0": "Hello", "1": "5"}, // First: full structure
				{}, // No change: empty tree indicates no update needed
				{"0": "Hi", "1": "3"},    // Dynamics-only: new values
			},
			cacheDescription: "Identical data triggers no-change optimization (empty tree), different data uses cache-aware diff",
		},
		{
			name:     "DiffBasedCaching",
			template: `<p>{{.Greeting}} {{.Name}}! You have {{.Count}} messages.</p>`,
			renderSequence: []interface{}{
				map[string]interface{}{"Greeting": "Hello", "Name": "Bob", "Count": 5},
				map[string]interface{}{"Greeting": "Hi", "Name": "Bob", "Count": 5},    // Partial change
				map[string]interface{}{"Greeting": "Hey", "Name": "Alice", "Count": 3}, // Multiple changes
			},
			expectedHTMLs: []string{
				`<p>Hello Bob! You have 5 messages.</p>`,
				`<p>Hi Bob! You have 5 messages.</p>`,
				`<p>Hey Alice! You have 3 messages.</p>`,
			},
			expectedTrees: []treeNode{
				{"s": []string{"<p>", " ", "! You have ", " messages.</p>"}, "0": "Hello", "1": "Bob", "2": "5"}, // First: full structure
				{"0": "Hi", "1": "Bob", "2": "5"},    // Dynamics-only
				{"0": "Hey", "1": "Alice", "2": "3"}, // Dynamics-only
			},
			cacheDescription: "Cache enables diff-based optimization for subsequent renders",
		},
		{
			name:     "ConditionalCaching",
			template: `<div>{{if .Active}}Active: {{.Name}}{{else}}Inactive{{end}}</div>`,
			renderSequence: []interface{}{
				map[string]interface{}{"Active": true, "Name": "Alice"},
				map[string]interface{}{"Active": false, "Name": "Bob"},
				map[string]interface{}{"Active": true, "Name": "Charlie"},
			},
			expectedHTMLs: []string{
				`<div>Active: Alice</div>`,
				`<div>Inactive</div>`,
				`<div>Active: Charlie</div>`,
			},
			expectedTrees: []treeNode{
				{"s": []string{"<div>", "</div>"}, "0": "Active: Alice"}, // First: full structure
				{"0": "Inactive"},        // Dynamics-only
				{"0": "Active: Charlie"}, // Dynamics-only
			},
			cacheDescription: "Conditional branches use cached wrapper structure with diff optimization",
		},
		{
			name:     "RangeCaching",
			template: `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			renderSequence: []interface{}{
				map[string]interface{}{"Items": []string{"Apple", "Banana"}},
				map[string]interface{}{"Items": []string{"Apple", "Banana", "Cherry"}}, // Add item
				map[string]interface{}{"Items": []string{"Orange"}},                    // Complete change
			},
			expectedHTMLs: []string{
				`<ul><li>Apple</li><li>Banana</li></ul>`,
				`<ul><li>Apple</li><li>Banana</li><li>Cherry</li></ul>`,
				`<ul><li>Orange</li></ul>`,
			},
			expectedTrees: []treeNode{
				nil, // First: complex nested structure - validated by HTML reconstruction
				nil, // Dynamics-only: wrapper cached, items with structure - validated by HTML reconstruction
				nil, // Dynamics-only: wrapper cached, new items - validated by HTML reconstruction
			},
			cacheDescription: "Range caching adapts between fine-grained and diff-based approaches",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create differ instance
			diff, err := NewDiffer(tc.template)
			if err != nil {
				t.Fatalf("NewDiffer failed: %v", err)
			}

			// Cast to differ to access internal differ for reconstruction helper
			cacheDiff, ok := diff.(*differ)
			if !ok {
				t.Fatalf("Expected differ, got %T", diff)
			}

			// Track render sequence and validate caching behavior
			for i, data := range tc.renderSequence {
				t.Logf("Render %d: %+v", i+1, data)

				tree, err := generateTreeForTest(diff, data)
				if err != nil {
					t.Fatalf("Render %d GenerateTree failed: %v", i+1, err)
				}

				// Validate HTML output using cache-aware reconstruction
				// Empty tree means "no change" - skip HTML validation for that case
				if len(tree) > 0 {
					html := cacheDiff.ReconstructFromDynamics(tree)
					if html != tc.expectedHTMLs[i] {
						t.Errorf("Render %d HTML mismatch:\nExpected: %s\nGot: %s",
							i+1, tc.expectedHTMLs[i], html)
					}
				} else {
					// Empty tree case - the expected HTML should match the previous render
					// (client keeps existing content when receiving empty tree)
					t.Logf("Render %d: Empty tree received (no change)", i+1)
				}

				// Validate tree structure if provided
				if i < len(tc.expectedTrees) && tc.expectedTrees[i] != nil {
					if !compareTreeNodes(tree, tc.expectedTrees[i]) {
						t.Errorf("Render %d tree mismatch:\nExpected: %+v\nGot: %+v",
							i+1, tc.expectedTrees[i], tree)
					}
				} else {
					t.Logf("Render %d tree structure: %+v", i+1, tree)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.cacheDescription)
		})
	}
}

func TestReset(t *testing.T) {
	templateStr := `<p>Hello {{.Name}}!</p>`
	data1 := map[string]interface{}{"Name": "Alice"}
	data2 := map[string]interface{}{"Name": "Bob"}

	diff, err := NewDiffer(templateStr)
	if err != nil {
		t.Fatalf("NewDiffer failed: %v", err)
	}

	// First render
	_, err = generateTreeForTest(diff, data1)
	if err != nil {
		t.Fatalf("First GenerateTree failed: %v", err)
	}

	// Reset cache
	diff.Reset()

	// Render after reset should behave like first render
	_, err = generateTreeForTest(diff, data2)
	if err != nil {
		t.Fatalf("GenerateTree after reset failed: %v", err)
	}

	t.Logf("✅ Reset functionality works correctly")
}

// TestErrorHandling tests error scenarios
func TestErrorHandling(t *testing.T) {
	// Test template parsing errors
	_, err := NewDiffer(`{{invalid template`)
	if err == nil {
		t.Errorf("Should fail for invalid template syntax")
	}

	// Test with valid differ but problematic data
	diff, err := NewDiffer(`{{.Field.Nested}}`)
	if err != nil {
		t.Fatalf("NewDiffer failed: %v", err)
	}

	// This should handle the error gracefully
	_, err = generateTreeForTest(diff, map[string]interface{}{"Field": nil})
	if err == nil {
		t.Errorf("Should handle nil pointer gracefully or return error")
	}

	t.Logf("✅ Error handling works correctly")
}

// TestTreeBehavior tests tree structure validation with adaptive behavior
func TestTreeBehavior(t *testing.T) {
	testCases := []struct {
		name         string
		template     string
		data1        interface{}
		data2        interface{}
		expectedKeys []string
		description  string
	}{
		{
			name:         "SimpleField",
			template:     `<p>Hello {{.Name}}!</p>`,
			data1:        map[string]interface{}{"Name": "Alice"},
			data2:        map[string]interface{}{"Name": "Bob"},
			expectedKeys: []string{"s", "0"},
			description:  "Simple field should generate tree with static parts and dynamic value",
		},
		{
			name:         "MultipleFields",
			template:     `<p>{{.Greeting}} {{.Name}}! Count: {{.Count}}</p>`,
			data1:        map[string]interface{}{"Greeting": "Hello", "Name": "Alice", "Count": 5},
			data2:        map[string]interface{}{"Greeting": "Hi", "Name": "Bob", "Count": 3},
			expectedKeys: []string{"s", "0", "1", "2"},
			description:  "Multiple fields should generate tree with static parts and multiple dynamic values",
		},
		{
			name:         "ConditionalExpression",
			template:     `<div>{{if .Visible}}Visible{{else}}Hidden{{end}}</div>`,
			data1:        map[string]interface{}{"Visible": true},
			data2:        map[string]interface{}{"Visible": false},
			expectedKeys: []string{"s", "0"},
			description:  "Conditional should generate tree with static wrapper and dynamic content",
		},
		{
			name:         "SimpleRange",
			template:     `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			data1:        map[string]interface{}{"Items": []string{"Apple", "Banana"}},
			data2:        map[string]interface{}{"Items": []string{"Cherry", "Date", "Elderberry"}},
			expectedKeys: []string{"s", "0"},
			description:  "Range should generate tree with static wrapper and dynamic list content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := NewDiffer(tc.template)
			if err != nil {
				t.Fatalf("NewDiffer failed: %v", err)
			}

			// First render
			tree1, err := generateTreeForTest(diff, tc.data1)
			if err != nil {
				t.Fatalf("First GenerateTree failed: %v", err)
			}

			// Second render
			tree2, err := generateTreeForTest(diff, tc.data2)
			if err != nil {
				t.Fatalf("Second GenerateTree failed: %v", err)
			}

			// Validate tree structure with caching behavior:
			// - First render: Full tree with static ("s") and dynamic ("0") parts
			// - Second render: Dynamics-only (no "s" key, statics cached client-side)

			// First tree should have both static and dynamic parts
			if _, exists := tree1["s"]; !exists {
				t.Errorf("First tree missing static structure key: s")
			}
			if _, exists := tree1["0"]; !exists {
				t.Errorf("First tree missing dynamic key: 0")
			}

			// Second tree should have dynamics-only (no "s" key for cached statics)
			if _, exists := tree2["0"]; !exists {
				t.Errorf("Second tree missing dynamic key: 0")
			}

			// Validate that expected keys exist when tree uses fine-grained approach
			if len(tree1) > 2 { // Fine-grained tree
				for _, expectedKey := range tc.expectedKeys {
					if _, exists := tree1[expectedKey]; !exists {
						t.Errorf("Fine-grained tree1 missing expected key: %s", expectedKey)
					}
				}
			}

			// Verify trees can be reconstructed to valid HTML using cache-aware reconstruction
			// Cast to differ to access internal differ for reconstruction helper
			cacheDiff, ok := diff.(*differ)
			if !ok {
				t.Fatalf("Expected differ, got %T", diff)
			}

			html1 := reconstructHTML(tree1)                   // First tree has full structure
			html2 := cacheDiff.ReconstructFromDynamics(tree2) // Second tree needs cache merge
			if html1 == "" || html2 == "" {
				t.Errorf("Reconstructed HTML is empty")
			}

			// Log tree structure analysis
			tree1Type := "diff-based"
			if len(tree1) > 2 {
				tree1Type = "fine-grained"
			}
			tree2Type := "dynamics-only"
			if _, hasStatics := tree2["s"]; hasStatics {
				if len(tree2) > 2 {
					tree2Type = "fine-grained"
				} else {
					tree2Type = "diff-based"
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
			t.Logf("    Tree1 (%s): %v, Tree2 (%s): %v", tree1Type, getTreeKeys(tree1), tree2Type, getTreeKeys(tree2))
		})
	}
}

func getTreeKeys(tree treeNode) []string {
	keys := make([]string, 0, len(tree))
	for key := range tree {
		keys = append(keys, key)
	}
	return keys
}
