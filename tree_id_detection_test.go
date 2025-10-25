package livetemplate

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestIDKeyDetection tests that the server correctly detects ID positions in range statics
func TestIDKeyDetection(t *testing.T) {
	tests := []struct {
		name          string
		template      string
		expectedIDKey string
		description   string
	}{
		{
			name:          "ID attribute",
			template:      `{{range .Items}}<li id="{{.ID}}">{{.Name}}</li>{{end}}`,
			expectedIDKey: "0",
			description:   "Should detect id attribute at position 0",
		},
		{
			name:          "data-key attribute",
			template:      `{{range .Items}}<div data-key="{{.Key}}">{{.Value}}</div>{{end}}`,
			expectedIDKey: "0",
			description:   "Should detect data-key attribute at position 0",
		},
		{
			name:          "key attribute",
			template:      `{{range .Items}}<span key="{{.ItemID}}">{{.Text}}</span>{{end}}`,
			expectedIDKey: "0",
			description:   "Should detect key attribute at position 0",
		},
		{
			name:          "data-lvt-key attribute",
			template:      `{{range .Items}}<p data-lvt-key="{{.UID}}">{{.Content}}</p>{{end}}`,
			expectedIDKey: "0",
			description:   "Should detect data-lvt-key attribute at position 0",
		},
		{
			name:          "ID at second position",
			template:      `{{range .Items}}<div class="{{.Class}}" id="{{.ID}}">{{.Name}}</div>{{end}}`,
			expectedIDKey: "1",
			description:   "Should detect id attribute at position 1",
		},
		{
			name:          "No ID attribute",
			template:      `{{range .Items}}<div>{{.Name}}</div>{{end}}`,
			expectedIDKey: "0",
			description:   "Should default to position 0 when no ID attribute found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template
			tmpl, err := New("test").Parse(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Test data
			data := map[string]interface{}{
				"Items": []map[string]interface{}{
					{"ID": "1", "Key": "k1", "ItemID": "i1", "UID": "u1", "Class": "active", "Name": "Item 1", "Value": "V1", "Text": "T1", "Content": "C1"},
					{"ID": "2", "Key": "k2", "ItemID": "i2", "UID": "u2", "Class": "inactive", "Name": "Item 2", "Value": "V2", "Text": "T2", "Content": "C2"},
				},
			}

			// Execute to get initial tree as JSON
			var buf bytes.Buffer
			err = tmpl.ExecuteUpdates(&buf, data)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			// Parse JSON output
			var tree map[string]interface{}
			err = json.Unmarshal(buf.Bytes(), &tree)
			if err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			// Debug: print the tree structure
			treeJSON, _ := json.MarshalIndent(tree, "", "  ")
			t.Logf("Tree structure:\n%s", treeJSON)

			// The range node IS the root tree itself (not nested)
			// Check for _idKey field
			idKey, exists := tree["_idKey"]
			if !exists {
				t.Errorf("Expected _idKey field in range node, but it was missing")
				return
			}

			// Verify the ID key matches expected
			if idKey != tt.expectedIDKey {
				t.Errorf("%s\nExpected _idKey: %q, got: %q", tt.description, tt.expectedIDKey, idKey)
			}
		})
	}
}

// TestDetectIDKeyFunction tests the detectIDKey function directly
func TestDetectIDKeyFunction(t *testing.T) {
	tests := []struct {
		name     string
		statics  []string
		expected string
	}{
		{
			name:     "ID at position 0",
			statics:  []string{`<li id="`, `">`, `</li>`},
			expected: "0",
		},
		{
			name:     "data-key at position 0",
			statics:  []string{`<div data-key="`, `">`, `</div>`},
			expected: "0",
		},
		{
			name:     "ID at position 1",
			statics:  []string{`<div class="`, `" id="`, `">`, `</div>`},
			expected: "1",
		},
		{
			name:     "key attribute",
			statics:  []string{`<span key="`, `">`, `</span>`},
			expected: "0",
		},
		{
			name:     "data-lvt-key attribute",
			statics:  []string{`<p data-lvt-key="`, `">`, `</p>`},
			expected: "0",
		},
		{
			name:     "lvt-key attribute",
			statics:  []string{`<div lvt-key="`, `">`, `</div>`},
			expected: "0",
		},
		{
			name:     "x-key attribute (Alpine.js)",
			statics:  []string{`<div x-key="`, `">`, `</div>`},
			expected: "0",
		},
		{
			name:     "v-key attribute (Vue.js)",
			statics:  []string{`<div v-key="`, `">`, `</div>`},
			expected: "0",
		},
		{
			name:     "No key attribute",
			statics:  []string{`<div>`, `</div>`},
			expected: "0",
		},
		{
			name:     "Empty statics",
			statics:  []string{},
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectIDKey(tt.statics)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
