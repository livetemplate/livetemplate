package strategy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPureTreeDiff_CounterExample(t *testing.T) {
	// Counter template from examples/counter
	templateSource := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>
<button data-action="increment">+</button>
<button data-action="decrement">-</button>`

	differ := NewPureTreeDiff()

	// Initial state (counter = 0, color = red)
	initialData := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b",
	}

	// First render (should return full HTML)
	update1, err := differ.GenerateMinimalUpdate(templateSource, nil, initialData, "counter")
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	t.Logf("First render:\n%s", update1.String())

	if update1.Type != "full" {
		t.Errorf("Expected type 'full' for first render, got '%s'", update1.Type)
	}
	if update1.FullHTML == "" {
		t.Error("Expected full HTML for first render")
	}

	// Update state (counter = 1, color = blue)
	updatedData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#45b7d1",
	}

	// Second render (should return minimal changes)
	update2, err := differ.GenerateMinimalUpdate(templateSource, initialData, updatedData, "counter")
	if err != nil {
		t.Fatalf("Update render failed: %v", err)
	}

	t.Logf("Update render:\n%s", update2.String())

	if update2.Type != "partial" {
		t.Errorf("Expected type 'partial' for update, got '%s'", update2.Type)
	}
	if len(update2.Changes) == 0 {
		t.Error("Expected changes for update")
	}

	// Verify we detect the specific changes
	foundColorChange := false
	foundTextChange := false

	for _, change := range update2.Changes {
		if change.Type == "attr" && change.Key == "style" {
			if strings.Contains(change.Value, "#45b7d1") {
				foundColorChange = true
				t.Logf("✓ Found color change: %s", change.Value)
			}
		}
		if change.Type == "text" && change.Value == "Hello 1 World" {
			foundTextChange = true
			t.Logf("✓ Found text change: %s", change.Value)
		}
	}

	if !foundColorChange {
		t.Error("Expected to find color style change")
	}
	if !foundTextChange {
		t.Error("Expected to find counter text change")
	}

	// Test no-change scenario
	update3, err := differ.GenerateMinimalUpdate(templateSource, updatedData, updatedData, "counter")
	if err != nil {
		t.Fatalf("No-change render failed: %v", err)
	}

	t.Logf("No-change render:\n%s", update3.String())

	if update3.Type != "none" {
		t.Errorf("Expected type 'none' for no-change, got '%s'", update3.Type)
	}
	if len(update3.Changes) > 0 {
		t.Error("Expected no changes when data unchanged")
	}

	// Calculate bandwidth savings
	fullSize := len(update1.FullHTML)
	updateSize := update2.GetUpdateSize()
	savings := float64(fullSize-updateSize) / float64(fullSize) * 100

	t.Logf("Bandwidth analysis:")
	t.Logf("  Full HTML: %d bytes", fullSize)
	t.Logf("  Update: %d bytes", updateSize)
	t.Logf("  Savings: %.1f%%", savings)

	if savings < 70 {
		t.Logf("WARNING: Expected >70%% bandwidth savings, got %.1f%%", savings)
	}
}

func TestPureTreeDiff_AttributeChanges(t *testing.T) {
	templateSource := `<input type="text" placeholder="{{.Placeholder}}" value="{{.Value}}" class="{{.Class}}">`

	differ := NewPureTreeDiff()

	data1 := map[string]interface{}{
		"Placeholder": "Enter name",
		"Value":       "",
		"Class":       "input-normal",
	}

	data2 := map[string]interface{}{
		"Placeholder": "Enter full name",
		"Value":       "John",
		"Class":       "input-filled",
	}

	// First render
	_, err := differ.GenerateMinimalUpdate(templateSource, nil, data1, "input")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Update
	update2, err := differ.GenerateMinimalUpdate(templateSource, data1, data2, "input")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Attribute changes:\n%s", update2.String())

	// Verify all attribute changes detected
	expectedAttrs := map[string]string{
		"placeholder": "Enter full name",
		"value":       "John",
		"class":       "input-filled",
	}

	foundAttrs := make(map[string]bool)
	for _, change := range update2.Changes {
		if change.Type == "attr" {
			if expectedVal, ok := expectedAttrs[change.Key]; ok {
				if change.Value == expectedVal {
					foundAttrs[change.Key] = true
					t.Logf("✓ Found attr change: %s='%s'", change.Key, change.Value)
				}
			}
		}
	}

	for attr := range expectedAttrs {
		if !foundAttrs[attr] {
			t.Errorf("Missing expected attribute change: %s", attr)
		}
	}
}

func TestPureTreeDiff_ListChanges(t *testing.T) {
	templateSource := `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`

	differ := NewPureTreeDiff()

	data1 := map[string]interface{}{
		"Items": []string{"Item 1", "Item 2"},
	}

	data2 := map[string]interface{}{
		"Items": []string{"Item 1", "Item 2", "Item 3"},
	}

	data3 := map[string]interface{}{
		"Items": []string{"Item 1", "Updated Item 2", "Item 3"},
	}

	// First render
	_, err := differ.GenerateMinimalUpdate(templateSource, nil, data1, "list")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Add item
	update2, err := differ.GenerateMinimalUpdate(templateSource, data1, data2, "list")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Add item:\n%s", update2.String())

	// Check for add operation
	hasAdd := false
	for _, change := range update2.Changes {
		if change.Type == "add" && strings.Contains(change.HTML, "Item 3") {
			hasAdd = true
			t.Logf("✓ Found add operation for Item 3")
		}
	}
	if !hasAdd {
		t.Error("Expected add operation for new item")
	}

	// Update item
	update3, err := differ.GenerateMinimalUpdate(templateSource, data2, data3, "list")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Update item:\n%s", update3.String())

	// Check for text change
	hasTextChange := false
	for _, change := range update3.Changes {
		if change.Type == "text" && change.Value == "Updated Item 2" {
			hasTextChange = true
			t.Logf("✓ Found text change for Item 2")
		}
	}
	if !hasTextChange {
		t.Error("Expected text change for updated item")
	}
}

func TestPureTreeDiff_JSONSerialization(t *testing.T) {
	templateSource := `<div>Count: {{.Count}}</div>`
	differ := NewPureTreeDiff()

	data1 := map[string]interface{}{"Count": 0}
	data2 := map[string]interface{}{"Count": 1}

	// First render
	_, err := differ.GenerateMinimalUpdate(templateSource, nil, data1, "json-test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Update
	update2, err := differ.GenerateMinimalUpdate(templateSource, data1, data2, "json-test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(update2)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	t.Logf("JSON size: %d bytes", len(jsonData))
	t.Logf("JSON: %s", string(jsonData))

	// Verify it's minimal
	if len(jsonData) > 200 {
		t.Logf("WARNING: JSON seems large for a simple text change: %d bytes", len(jsonData))
	}

	// Test deserialization
	var decoded MinimalTreeUpdate
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if decoded.Type != update2.Type {
		t.Error("Type mismatch after JSON round-trip")
	}
	if len(decoded.Changes) != len(update2.Changes) {
		t.Error("Changes count mismatch after JSON round-trip")
	}
}

func TestPureTreeDiff_ComplexNesting(t *testing.T) {
	templateSource := `<div class="{{.Class}}">
{{if .ShowHeader}}<h1>{{.Title}}</h1>{{end}}
<div class="content">
  {{range .Items}}
  <div class="item">
    <span>{{.Name}}</span>
    {{if .Active}}<span class="badge">Active</span>{{end}}
  </div>
  {{end}}
</div>
</div>`

	differ := NewPureTreeDiff()

	data1 := map[string]interface{}{
		"Class":      "container",
		"ShowHeader": true,
		"Title":      "Original",
		"Items": []map[string]interface{}{
			{"Name": "Item 1", "Active": true},
			{"Name": "Item 2", "Active": false},
		},
	}

	data2 := map[string]interface{}{
		"Class":      "container-updated",
		"ShowHeader": true,
		"Title":      "Updated",
		"Items": []map[string]interface{}{
			{"Name": "Item 1", "Active": false},         // Active changed
			{"Name": "Item 2 Updated", "Active": false}, // Name changed
			{"Name": "Item 3", "Active": true},          // New item
		},
	}

	// First render
	update1, err := differ.GenerateMinimalUpdate(templateSource, nil, data1, "complex")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Complex update
	update2, err := differ.GenerateMinimalUpdate(templateSource, data1, data2, "complex")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Complex update:\n%s", update2.String())
	t.Logf("Number of changes: %d", len(update2.Changes))

	// Verify we're not sending full HTML
	if update2.Type != "partial" {
		t.Error("Expected partial update for complex changes")
	}

	// Calculate efficiency
	fullSize := len(update1.FullHTML)
	updateSize := update2.GetUpdateSize()

	t.Logf("Efficiency analysis:")
	t.Logf("  Full HTML: %d bytes", fullSize)
	t.Logf("  Update: %d bytes", updateSize)
	t.Logf("  Ratio: %.1f%%", float64(updateSize)/float64(fullSize)*100)
}
