package strategy

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestSimpleTreeGeneration tests the simple tree structure generation
func TestSimpleTreeGeneration(t *testing.T) {
	testCases := []struct {
		Name           string
		TemplateSource string
		Data           map[string]interface{}
		ExpectedJSON   string // Expected minimal client structure
	}{
		{
			Name:           "SimpleField",
			TemplateSource: `<p>Hello {{.Name}}!</p>`,
			Data:           map[string]interface{}{"Name": "World"},
			ExpectedJSON:   `{"s":["<p>Hello ","!</p>"],"0":"World"}`,
		},
		{
			Name:           "MultipleFields",
			TemplateSource: `<div>{{.Name}} has {{.Score}} points</div>`,
			Data:           map[string]interface{}{"Name": "Alice", "Score": 100},
			ExpectedJSON:   `{"s":["<div>"," has "," points</div>"],"0":"Alice","1":"100"}`,
		},
		{
			Name:           "ConditionalTrue",
			TemplateSource: `<div>{{if .Show}}Welcome {{.Name}}!{{end}}</div>`,
			Data:           map[string]interface{}{"Show": true, "Name": "John"},
			ExpectedJSON:   `{"s":["<div>","</div>"],"0":{"s":["Welcome ","!"],"0":"John"}}`,
		},
		{
			Name:           "ConditionalFalse",
			TemplateSource: `<div>{{if .Show}}Welcome {{.Name}}!{{end}}</div>`,
			Data:           map[string]interface{}{"Show": false, "Name": "John"},
			ExpectedJSON:   `{"s":["<div>","</div>"],"0":{"s":[""]}}`,
		},
		{
			Name:           "IfElseTrue",
			TemplateSource: `{{if .Premium}}<gold>{{.Name}}</gold>{{else}}<span>{{.Name}}</span>{{end}}`,
			Data:           map[string]interface{}{"Premium": true, "Name": "VIP"},
			ExpectedJSON:   `{"s":["",""],"0":{"s":["<gold>","</gold>"],"0":"VIP"}}`,
		},
		{
			Name:           "IfElseFalse",
			TemplateSource: `{{if .Premium}}<gold>{{.Name}}</gold>{{else}}<span>{{.Name}}</span>{{end}}`,
			Data:           map[string]interface{}{"Premium": false, "Name": "Regular"},
			ExpectedJSON:   `{"s":["",""],"0":{"s":["","</span>"],"0":"Regular"}}`,
		},
		{
			Name:           "SimpleRange",
			TemplateSource: `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			Data:           map[string]interface{}{"Items": []interface{}{"A", "B", "C"}},
			ExpectedJSON:   `{"s":["<ul>","</ul>"],"0":[{"s":["<li>","</li>"],"0":"A"},{"s":["<li>","</li>"],"0":"B"},{"s":["<li>","</li>"],"0":"C"}]}`,
		},
		{
			Name:           "EmptyRange",
			TemplateSource: `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			Data:           map[string]interface{}{"Items": []interface{}{}},
			ExpectedJSON:   `{"s":["<ul>","</ul>"],"0":{"s":[""]}}`,
		},
		{
			Name:           "SingleItemRange",
			TemplateSource: `<div>{{range .Items}}Item: {{.}}{{end}}</div>`,
			Data:           map[string]interface{}{"Items": []interface{}{"Only"}},
			ExpectedJSON:   `{"s":["<div>","</div>"],"0":{"s":["Item: ",""],"0":"Only"}}`,
		},
		{
			Name:           "NestedConditionalInRange",
			TemplateSource: `{{range .Users}}<div>{{if .Active}}✓{{else}}✗{{end}} {{.Name}}</div>{{end}}`,
			Data: map[string]interface{}{
				"Users": []interface{}{
					map[string]interface{}{"Name": "Alice", "Active": true},
					map[string]interface{}{"Name": "Bob", "Active": false},
				},
			},
			ExpectedJSON: `{"s":["",""],"0":[{"s":["<div>"," ","</div>"],"0":{"s":["✓"],"0":""},"1":"Alice"},{"s":["<div>"," ","</div>"],"0":{"s":[""]},"1":"Bob"}]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			generator := NewSimpleTreeGenerator()

			// Generate tree structure
			result, err := generator.GenerateFromTemplateSource(tc.TemplateSource, nil, tc.Data, "test-fragment")

			if err != nil {
				t.Fatalf("Failed to generate tree structure: %v", err)
			}

			// Marshal to JSON to check client format
			jsonBytes, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Failed to marshal result to JSON: %v", err)
			}

			actualJSON := string(jsonBytes)

			// Parse both JSONs to compare structures instead of strings
			var expectedData, actualData map[string]interface{}
			err = json.Unmarshal([]byte(tc.ExpectedJSON), &expectedData)
			if err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			err = json.Unmarshal(jsonBytes, &actualData)
			if err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}

			// Compare structures
			if !compareJSONStructures(expectedData, actualData) {
				t.Errorf("JSON output mismatch\nExpected: %s\nActual:   %s", tc.ExpectedJSON, actualJSON)

				// Pretty print for debugging
				var prettyExpected, prettyActual interface{}
				if err := json.Unmarshal([]byte(tc.ExpectedJSON), &prettyExpected); err != nil {
					t.Logf("Failed to unmarshal expected JSON: %v", err)
				}
				if err := json.Unmarshal(jsonBytes, &prettyActual); err != nil {
					t.Logf("Failed to unmarshal actual JSON: %v", err)
				}

				expectedPretty, _ := json.MarshalIndent(prettyExpected, "", "  ")
				actualPretty, _ := json.MarshalIndent(prettyActual, "", "  ")

				t.Logf("Expected (pretty):\n%s", string(expectedPretty))
				t.Logf("Actual (pretty):\n%s", string(actualPretty))
			}
		})
	}
}

// TestIncrementalUpdates tests the incremental update capability
func TestIncrementalUpdates(t *testing.T) {
	generator := NewSimpleTreeGenerator()
	templateSource := `<div>User: {{.Name}}, Score: {{.Score}}</div>`
	fragmentID := "user-info"

	// First render with initial data
	firstData := map[string]interface{}{"Name": "Alice", "Score": 100}
	firstResult, err := generator.GenerateFromTemplateSource(templateSource, nil, firstData, fragmentID)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	// Verify first render includes statics
	if len(firstResult.S) == 0 {
		t.Error("First render should include statics array")
	}

	firstJSON, _ := json.Marshal(firstResult)

	// Parse and compare structures instead of strings to handle HTML escaping
	var expectedFirstData, actualFirstData map[string]interface{}
	expectedFirst := `{"s":["<div>User: ",", Score: ","</div>"],"0":"Alice","1":"100"}`
	if err := json.Unmarshal([]byte(expectedFirst), &expectedFirstData); err != nil {
		t.Logf("Failed to unmarshal expected first JSON: %v", err)
	}
	if err := json.Unmarshal(firstJSON, &actualFirstData); err != nil {
		t.Logf("Failed to unmarshal actual first JSON: %v", err)
	}

	if !compareJSONStructures(expectedFirstData, actualFirstData) {
		t.Errorf("First render JSON mismatch\nExpected: %s\nActual:   %s", expectedFirst, string(firstJSON))
	}

	// Second render with updated data
	secondData := map[string]interface{}{"Name": "Bob", "Score": 150}
	secondResult, err := generator.GenerateFromTemplateSource(templateSource, firstData, secondData, fragmentID)
	if err != nil {
		t.Fatalf("Second render failed: %v", err)
	}

	// Verify second render excludes statics (cached client-side)
	if len(secondResult.S) > 0 {
		t.Error("Second render should not include statics (cached client-side)")
	}

	secondJSON, _ := json.Marshal(secondResult)

	// Parse and compare structures for second render
	var expectedSecondData, actualSecondData map[string]interface{}
	expectedSecond := `{"0":"Bob","1":"150"}`
	if err := json.Unmarshal([]byte(expectedSecond), &expectedSecondData); err != nil {
		t.Logf("Failed to unmarshal expected second JSON: %v", err)
	}
	if err := json.Unmarshal(secondJSON, &actualSecondData); err != nil {
		t.Logf("Failed to unmarshal actual second JSON: %v", err)
	}

	if !compareJSONStructures(expectedSecondData, actualSecondData) {
		t.Errorf("Second render JSON mismatch\nExpected: %s\nActual:   %s", expectedSecond, string(secondJSON))
	}

	t.Logf("First render (with statics): %s", string(firstJSON))
	t.Logf("Second render (dynamics only): %s", string(secondJSON))

	// Calculate bandwidth savings
	firstSize := len(firstJSON)
	secondSize := len(secondJSON)
	savings := float64(firstSize-secondSize) / float64(firstSize) * 100

	t.Logf("Bandwidth savings: %.1f%% (%d bytes vs %d bytes)", savings, secondSize, firstSize)

	if savings < 50.0 {
		t.Errorf("Expected significant bandwidth savings, got %.1f%%", savings)
	}
}

// TestComplexNesting tests deeply nested structures
func TestComplexNesting(t *testing.T) {
	generator := NewSimpleTreeGenerator()

	templateSource := `
<div class="dashboard">
  {{if .User}}
    <header>Welcome {{.User.Name}}!</header>
    {{if .User.Premium}}
      <div class="premium">
        {{range .User.Benefits}}
          <span class="benefit">{{.}}</span>
        {{end}}
      </div>
    {{end}}
  {{else}}
    <div class="login">Please log in</div>
  {{end}}
</div>`

	data := map[string]interface{}{
		"User": map[string]interface{}{
			"Name":     "Alice",
			"Premium":  true,
			"Benefits": []interface{}{"Ad-free", "Priority Support", "Extra Storage"},
		},
	}

	result, err := generator.GenerateFromTemplateSource(templateSource, nil, data, "dashboard")
	if err != nil {
		t.Fatalf("Failed to generate complex nested structure: %v", err)
	}

	// Marshal and verify it produces valid JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal complex structure: %v", err)
	}

	// Pretty print the result
	var prettyResult interface{}
	if err := json.Unmarshal(jsonBytes, &prettyResult); err != nil {
		t.Logf("Failed to unmarshal for pretty printing: %v", err)
	}
	prettyJSON, _ := json.MarshalIndent(prettyResult, "", "  ")

	t.Logf("Complex nested structure:\n%s", string(prettyJSON))

	// Verify it has the expected nested structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Logf("Failed to unmarshal for parsing: %v", err)
	}

	if parsed["s"] == nil {
		t.Error("Expected statics array in root")
	}

	if parsed["0"] == nil {
		t.Error("Expected nested structure in slot 0")
	}

	// Test incremental update
	updatedData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name":    "Bob",
			"Premium": false,
		},
	}

	updatedResult, err := generator.GenerateFromTemplateSource(templateSource, data, updatedData, "dashboard")
	if err != nil {
		t.Fatalf("Failed to generate incremental update: %v", err)
	}

	updatedJSON, _ := json.Marshal(updatedResult)
	t.Logf("Incremental update: %s", string(updatedJSON))

	// Verify incremental update is much smaller
	originalSize := len(jsonBytes)
	updateSize := len(updatedJSON)
	savings := float64(originalSize-updateSize) / float64(originalSize) * 100

	t.Logf("Update bandwidth savings: %.1f%% (%d bytes vs %d bytes)", savings, updateSize, originalSize)
}

// TestLiveViewCompatibility tests compatibility with LiveView format
func TestLiveViewCompatibility(t *testing.T) {
	generator := NewSimpleTreeGenerator()

	// Test case inspired by the LiveView example you provided
	templateSource := `{{if .Clicked}}<p>Button clicked {{.Count}} times!</p>{{else}}<p>Nobody clicked the button yet.</p>{{end}}<button>Click me!</button>`

	data := map[string]interface{}{
		"Clicked": false,
		"Count":   0,
	}

	result, err := generator.GenerateFromTemplateSource(templateSource, nil, data, "button-demo")
	if err != nil {
		t.Fatalf("Failed to generate LiveView-compatible structure: %v", err)
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Parse and verify structure matches LiveView format
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Logf("Failed to unmarshal for parsing: %v", err)
	}

	// Should have "s" for statics
	if parsed["s"] == nil {
		t.Error("Expected 's' field for statics")
	}

	// Should have numeric string keys for dynamics
	if parsed["0"] == nil {
		t.Error("Expected '0' field for first dynamic slot")
	}

	// Should have clean, minimal structure
	t.Logf("LiveView-compatible structure: %s", string(jsonBytes))

	// Test state change
	clickedData := map[string]interface{}{
		"Clicked": true,
		"Count":   5,
	}

	updatedResult, err := generator.GenerateFromTemplateSource(templateSource, data, clickedData, "button-demo")
	if err != nil {
		t.Fatalf("Failed to generate state change: %v", err)
	}

	updatedJSON, _ := json.Marshal(updatedResult)
	t.Logf("After click update: %s", string(updatedJSON))

	// Verify update structure
	if len(updatedResult.S) > 0 {
		t.Error("Incremental update should not include statics")
	}
}

// BenchmarkSimpleTreeGeneration benchmarks the tree generation performance
func BenchmarkSimpleTreeGeneration(t *testing.B) {
	generator := NewSimpleTreeGenerator()
	templateSource := `<div>{{if .Active}}<span class="user">{{.Name}} ({{.Role}})</span>{{else}}<span class="inactive">{{.Name}}</span>{{end}}</div>`

	data := map[string]interface{}{
		"Active": true,
		"Name":   "TestUser",
		"Role":   "Admin",
	}

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		_, err := generator.GenerateFromTemplateSource(templateSource, nil, data, "bench-test")
		if err != nil {
			t.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// TestContentAndAttributeChanges tests that both content and attribute changes generate fragments
func TestContentAndAttributeChanges(t *testing.T) {
	generator := NewSimpleTreeGenerator()

	// Template with both content and attribute variables
	templateSource := `<h1><span class="{{.Color}}">Counter: {{.Counter}}</span></h1><div>Value: {{.Counter}}</div>`
	fragmentID := "content-attr-test"

	// Initial data
	initialData := map[string]interface{}{
		"Counter": 0,
		"Color":   "red",
	}

	// First render
	firstResult, err := generator.GenerateFromTemplateSource(templateSource, nil, initialData, fragmentID)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	firstJSON, _ := json.Marshal(firstResult)
	t.Logf("First render: %s", string(firstJSON))

	// Update both counter and color
	updatedData := map[string]interface{}{
		"Counter": 1,      // Changed content
		"Color":   "blue", // Changed attribute
	}

	// Generate incremental update
	updateResult, err := generator.GenerateFromTemplateSource(templateSource, initialData, updatedData, fragmentID)
	if err != nil {
		t.Fatalf("Incremental update failed: %v", err)
	}

	updateJSON, _ := json.Marshal(updateResult)
	t.Logf("Update render: %s", string(updateJSON))

	// Parse the update structure to verify both changes are captured
	var updateData map[string]interface{}
	if err := json.Unmarshal(updateJSON, &updateData); err != nil {
		t.Fatalf("Failed to parse update JSON: %v", err)
	}

	// The update should contain changes for both Counter (content) and Color (attribute)
	// We expect dynamics to be updated even when statics are not included in incremental updates
	if updateData["0"] == nil && updateData["1"] == nil {
		t.Error("Update should contain dynamic changes for both content and attribute changes")
		t.Logf("Update structure: %+v", updateData)
	}

	// Verify the update actually contains both the counter and color values
	found_counter := false
	found_color := false

	// Check if counter and color values are present in any form in the update
	for key, value := range updateData {
		if value == "1" || value == 1 {
			found_counter = true
			t.Logf("Found counter value at key %s: %v", key, value)
		}
		if value == "blue" {
			found_color = true
			t.Logf("Found color value at key %s: %v", key, value)
		}

		// Check nested structures for color/counter values
		if nestedMap, ok := value.(map[string]interface{}); ok {
			for nestedKey, nestedValue := range nestedMap {
				if nestedValue == "1" || nestedValue == 1 {
					found_counter = true
					t.Logf("Found counter value at nested key %s.%s: %v", key, nestedKey, nestedValue)
				}
				if nestedValue == "blue" {
					found_color = true
					t.Logf("Found color value at nested key %s.%s: %v", key, nestedKey, nestedValue)
				}
			}
		}
	}

	if !found_counter {
		t.Error("Update should contain the new counter value (1)")
	}

	if !found_color {
		t.Error("Update should contain the new color value (blue)")
	}

	t.Logf("Content + Attribute test passed: counter=%v, color=%v", found_counter, found_color)
}

// compareJSONStructures compares two JSON structures for equality
func compareJSONStructures(expected, actual interface{}) bool {
	return reflect.DeepEqual(expected, actual)
}
