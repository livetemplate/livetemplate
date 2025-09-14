package diff

import (
	"encoding/json"
	"testing"
)

// TestEmptyFragmentFiltering verifies that empty/unnecessary fragments are not sent
func TestEmptyFragmentFiltering(t *testing.T) {
	tests := []struct {
		name          string
		template      string
		oldData       interface{}
		newData       interface{}
		expectChanges bool
		description   string
	}{
		{
			name:          "EmptyStringDynamic",
			template:      `<div>{{.Content}}</div>`,
			oldData:       map[string]interface{}{"Content": ""},
			newData:       map[string]interface{}{"Content": ""},
			expectChanges: false,
			description:   "Empty string dynamics should not trigger updates",
		},
		{
			name:          "WhitespaceOnlyDynamic",
			template:      `<div>{{.Content}}</div>`,
			oldData:       map[string]interface{}{"Content": "   \t\n  "},
			newData:       map[string]interface{}{"Content": "   \t\n  "},
			expectChanges: false,
			description:   "Whitespace-only dynamics should not trigger updates",
		},
		{
			name:          "ConditionalEvaluatesToEmpty",
			template:      `{{if .ShowContent}}{{.Content}}{{end}}`,
			oldData:       map[string]interface{}{"ShowContent": false, "Content": "Hello"},
			newData:       map[string]interface{}{"ShowContent": false, "Content": "World"},
			expectChanges: false,
			description:   "Conditional that evaluates to empty should not send updates",
		},
		{
			name:          "EmptyRangeNoChange",
			template:      `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			oldData:       map[string]interface{}{"Items": []string{}},
			newData:       map[string]interface{}{"Items": []string{}},
			expectChanges: false,
			description:   "Empty range with no changes should not send updates",
		},
		{
			name:          "NestedEmptyConditional",
			template:      `{{if .Show}}{{if .SubShow}}{{.Content}}{{end}}{{end}}`,
			oldData:       map[string]interface{}{"Show": true, "SubShow": false, "Content": "Test"},
			newData:       map[string]interface{}{"Show": true, "SubShow": false, "Content": "Different"},
			expectChanges: false,
			description:   "Nested conditional resulting in empty should not send updates",
		},
		{
			name:          "WithConstructEmpty",
			template:      `{{with .Profile}}{{.Name}}{{else}}No profile{{end}}`,
			oldData:       map[string]interface{}{"Profile": nil},
			newData:       map[string]interface{}{"Profile": nil},
			expectChanges: false,
			description:   "With construct with nil context should not send empty updates",
		},
		{
			name:          "MultipleEmptyFields",
			template:      `<div>{{.Field1}}{{.Field2}}{{.Field3}}</div>`,
			oldData:       map[string]interface{}{"Field1": "", "Field2": "", "Field3": ""},
			newData:       map[string]interface{}{"Field1": "", "Field2": "", "Field3": ""},
			expectChanges: false,
			description:   "Multiple empty fields should not trigger updates",
		},
		// Positive test cases - these SHOULD send updates
		{
			name:          "MeaningfulChange",
			template:      `<div>{{.Content}}</div>`,
			oldData:       map[string]interface{}{"Content": "Hello"},
			newData:       map[string]interface{}{"Content": "World"},
			expectChanges: true,
			description:   "Meaningful content changes should send updates",
		},
		{
			name:          "EmptyToContent",
			template:      `<div>{{.Content}}</div>`,
			oldData:       map[string]interface{}{"Content": ""},
			newData:       map[string]interface{}{"Content": "Hello"},
			expectChanges: true,
			description:   "Change from empty to content should send updates",
		},
		{
			name:          "ContentToEmpty",
			template:      `<div>{{.Content}}</div>`,
			oldData:       map[string]interface{}{"Content": "Hello"},
			newData:       map[string]interface{}{"Content": ""},
			expectChanges: true,
			description:   "Change from content to empty should send updates (clearing)",
		},
		{
			name:          "ConditionalToggle",
			template:      `{{if .Show}}Visible{{else}}Hidden{{end}}`,
			oldData:       map[string]interface{}{"Show": false},
			newData:       map[string]interface{}{"Show": true},
			expectChanges: true,
			description:   "Conditional toggle should send updates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()

			// First render to establish baseline
			_, err := tree.GenerateWithFragmentID(tt.template, tt.oldData, tt.oldData, "test-fragment")
			if err != nil {
				t.Fatalf("Failed to generate first render: %v", err)
			}

			// Generate update
			update, err := tree.GenerateWithFragmentID(tt.template, tt.oldData, tt.newData, "test-fragment")
			if err != nil {
				t.Fatalf("Failed to generate update: %v", err)
			}

			hasChanges := update.HasChanges()

			// Check if the result matches expectation
			if hasChanges != tt.expectChanges {
				t.Errorf("%s: Expected HasChanges=%v, got %v",
					tt.description, tt.expectChanges, hasChanges)

				// Log the actual update for debugging
				jsonData, _ := json.Marshal(update)
				t.Logf("Update JSON: %s", string(jsonData))
			}

			// Additional validation: if no changes expected, verify minimal JSON
			if !tt.expectChanges {
				jsonData, _ := json.Marshal(update)
				jsonStr := string(jsonData)

				// Should be empty object {} or have no meaningful dynamics
				if jsonStr != "{}" && update.HasChanges() {
					t.Errorf("%s: Expected empty or minimal JSON, got: %s",
						tt.description, jsonStr)
				}
			}
		})
	}
}

// TestEmptyFragmentBandwidthSavings verifies bandwidth savings from filtering empty fragments
func TestEmptyFragmentBandwidthSavings(t *testing.T) {
	tests := []struct {
		name              string
		template          string
		data              interface{}
		minSavingsPercent float64
	}{
		{
			name:              "ConditionalFalse",
			template:          `{{if .Show}}<div>{{.Content}}</div>{{end}}`,
			data:              map[string]interface{}{"Show": false, "Content": "Hidden"},
			minSavingsPercent: 90.0, // Expect >90% savings for empty conditional
		},
		{
			name:              "EmptyRange",
			template:          `{{range .Items}}<li>{{.}}</li>{{else}}No items{{end}}`,
			data:              map[string]interface{}{"Items": []string{}},
			minSavingsPercent: 80.0, // Expect >80% savings for empty range with else
		},
		{
			name:              "MultipleEmptyFields",
			template:          `<form>{{.Field1}}{{.Field2}}{{.Field3}}</form>`,
			data:              map[string]interface{}{"Field1": "", "Field2": "", "Field3": ""},
			minSavingsPercent: 70.0, // Expect >70% savings for multiple empty fields
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()

			// First render
			firstUpdate, err := tree.GenerateWithFragmentID(tt.template, tt.data, tt.data, "test")
			if err != nil {
				t.Fatalf("Failed to generate first render: %v", err)
			}

			// Second render (no changes)
			secondUpdate, err := tree.GenerateWithFragmentID(tt.template, tt.data, tt.data, "test")
			if err != nil {
				t.Fatalf("Failed to generate second render: %v", err)
			}

			// Calculate sizes
			firstJSON, _ := json.Marshal(firstUpdate)
			secondJSON, _ := json.Marshal(secondUpdate)

			firstSize := len(firstJSON)
			secondSize := len(secondJSON)

			// Calculate savings
			savings := float64(firstSize-secondSize) / float64(firstSize) * 100

			t.Logf("First render: %d bytes, Second render: %d bytes, Savings: %.1f%%",
				firstSize, secondSize, savings)

			// Verify minimum savings
			if savings < tt.minSavingsPercent {
				t.Errorf("Expected at least %.1f%% savings, got %.1f%%",
					tt.minSavingsPercent, savings)
				t.Logf("First JSON: %s", string(firstJSON))
				t.Logf("Second JSON: %s", string(secondJSON))
			}
		})
	}
}

// TestEmptyFragmentEdgeCases tests edge cases for empty fragment handling
func TestEmptyFragmentEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		template      string
		oldData       interface{}
		newData       interface{}
		expectChanges bool
		description   string
	}{
		{
			name:          "ZeroValueNumber",
			template:      `<div>Count: {{.Count}}</div>`,
			oldData:       map[string]interface{}{"Count": 0},
			newData:       map[string]interface{}{"Count": 0},
			expectChanges: false,
			description:   "Zero value number with no change should not send updates",
		},
		{
			name:          "ZeroToNonZero",
			template:      `<div>Count: {{.Count}}</div>`,
			oldData:       map[string]interface{}{"Count": 0},
			newData:       map[string]interface{}{"Count": 1},
			expectChanges: true,
			description:   "Zero to non-zero should send updates",
		},
		{
			name:          "FalseBooleanNoChange",
			template:      `<div>Active: {{.Active}}</div>`,
			oldData:       map[string]interface{}{"Active": false},
			newData:       map[string]interface{}{"Active": false},
			expectChanges: false,
			description:   "False boolean with no change should not send updates",
		},
		{
			name:          "EmptySliceToNil",
			template:      `{{range .Items}}<li>{{.}}</li>{{end}}`,
			oldData:       map[string]interface{}{"Items": []string{}},
			newData:       map[string]interface{}{"Items": nil},
			expectChanges: false,
			description:   "Empty slice to nil should not send updates (both are empty)",
		},
		{
			name:          "SpaceToEmpty",
			template:      `<div>{{.Text}}</div>`,
			oldData:       map[string]interface{}{"Text": " "},
			newData:       map[string]interface{}{"Text": ""},
			expectChanges: true,
			description:   "Single space to empty is a meaningful change",
		},
		{
			name:          "TabsAndSpacesNoChange",
			template:      `<div>{{.Text}}</div>`,
			oldData:       map[string]interface{}{"Text": "\t  \n\t"},
			newData:       map[string]interface{}{"Text": "\t  \n\t"},
			expectChanges: false,
			description:   "Whitespace-only with no change should not send updates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()

			// First render
			_, err := tree.GenerateWithFragmentID(tt.template, tt.oldData, tt.oldData, "edge-test")
			if err != nil {
				t.Fatalf("Failed to generate first render: %v", err)
			}

			// Generate update
			update, err := tree.GenerateWithFragmentID(tt.template, tt.oldData, tt.newData, "edge-test")
			if err != nil {
				t.Fatalf("Failed to generate update: %v", err)
			}

			hasChanges := update.HasChanges()

			if hasChanges != tt.expectChanges {
				t.Errorf("%s: Expected HasChanges=%v, got %v",
					tt.description, tt.expectChanges, hasChanges)

				jsonData, _ := json.Marshal(update)
				t.Logf("Update JSON: %s", string(jsonData))
			}
		})
	}
}
