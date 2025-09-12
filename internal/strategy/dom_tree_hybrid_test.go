package strategy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDOMTreeHybrid_CounterExample(t *testing.T) {
	// Use the actual counter template from examples/counter
	templateSource := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>
<button data-lvt-action="increment">+</button>
<button data-lvt-action="decrement">-</button>`

	hybrid := NewDOMTreeHybrid()

	// Initial state (counter = 0, color = red)
	initialData := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b", // red
	}

	// First render (should return full HTML)
	update1, err := hybrid.GenerateTreeUpdate(templateSource, nil, initialData, "counter-fragment")
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	t.Logf("First render:\n%s", update1.String())

	// Verify first render contains full HTML
	if update1.HTML == "" {
		t.Error("Expected full HTML for first render")
	}
	if update1.IsEmpty() {
		t.Error("First render should not be empty")
	}

	// Update state (counter = 1, color = blue)
	updatedData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#45b7d1", // blue
	}

	// Second render (should detect changes and return update)
	update2, err := hybrid.GenerateTreeUpdate(templateSource, initialData, updatedData, "counter-fragment")
	if err != nil {
		t.Fatalf("Update render failed: %v", err)
	}

	t.Logf("Update render:\n%s", update2.String())

	// Verify update contains new HTML
	if update2.HTML == "" {
		t.Error("Expected HTML update for changed data")
	}

	// Verify HTML contains updated values
	if !containsString(update2.HTML, "Hello 1 World") {
		t.Error("Updated HTML should contain 'Hello 1 World'")
	}
	if !containsString(update2.HTML, "#45b7d1") {
		t.Error("Updated HTML should contain new color '#45b7d1'")
	}

	// Test no-change scenario
	update3, err := hybrid.GenerateTreeUpdate(templateSource, updatedData, updatedData, "counter-fragment")
	if err != nil {
		t.Fatalf("No-change render failed: %v", err)
	}

	t.Logf("No-change render:\n%s", update3.String())

	// Verify no-change update is empty
	if !update3.IsEmpty() {
		t.Error("Expected empty update when data hasn't changed")
	}
}

func TestDOMTreeHybrid_SimpleExample(t *testing.T) {
	templateSource := `<div>Count: {{.Count}}</div>`

	hybrid := NewDOMTreeHybrid()

	// Test progression: 0 -> 1 -> 2 -> 2 (no change)
	data0 := map[string]interface{}{"Count": 0}
	data1 := map[string]interface{}{"Count": 1}
	data2 := map[string]interface{}{"Count": 2}

	// First render
	update1, err := hybrid.GenerateTreeUpdate(templateSource, nil, data0, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Initial render: %s", update1.String())

	if update1.IsEmpty() || update1.HTML == "" {
		t.Error("First render should contain HTML")
	}

	// Update to 1
	update2, err := hybrid.GenerateTreeUpdate(templateSource, data0, data1, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Update to 1: %s", update2.String())

	if update2.IsEmpty() {
		t.Error("Update should not be empty when data changes")
	}
	if !containsString(update2.HTML, "Count: 1") {
		t.Error("Update should contain new count value")
	}

	// Update to 2
	update3, err := hybrid.GenerateTreeUpdate(templateSource, data1, data2, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Update to 2: %s", update3.String())

	if update3.IsEmpty() {
		t.Error("Update should not be empty when data changes")
	}

	// No change (2 -> 2)
	update4, err := hybrid.GenerateTreeUpdate(templateSource, data2, data2, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("No change: %s", update4.String())

	if !update4.IsEmpty() {
		t.Error("Update should be empty when data doesn't change")
	}
}

func TestDOMTreeHybrid_ConditionalExample(t *testing.T) {
	templateSource := `<div>{{if .ShowMessage}}Message: {{.Message}}{{else}}No message{{end}}</div>`

	hybrid := NewDOMTreeHybrid()

	// Test conditional changes
	dataHidden := map[string]interface{}{"ShowMessage": false, "Message": "Hello"}
	dataShown := map[string]interface{}{"ShowMessage": true, "Message": "Hello"}

	// First render (hidden)
	update1, err := hybrid.GenerateTreeUpdate(templateSource, nil, dataHidden, "conditional")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Hidden state: %s", update1.String())

	if !containsString(update1.HTML, "No message") {
		t.Error("Hidden state should show 'No message'")
	}

	// Show message
	update2, err := hybrid.GenerateTreeUpdate(templateSource, dataHidden, dataShown, "conditional")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Show message: %s", update2.String())

	if !containsString(update2.HTML, "Message: Hello") {
		t.Error("Shown state should show 'Message: Hello'")
	}

	// Hide message again
	update3, err := hybrid.GenerateTreeUpdate(templateSource, dataShown, dataHidden, "conditional")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Hide message: %s", update3.String())

	if !containsString(update3.HTML, "No message") {
		t.Error("Hidden state should show 'No message'")
	}
}

func TestDOMTreeHybrid_ComplexHTML(t *testing.T) {
	// Test with more complex HTML structure
	templateSource := `<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>
  <div class="container">
    <h1>{{.Heading}}</h1>
    <p>Count: {{.Count}}</p>
    {{range .Items}}
    <div class="item">{{.Name}}</div>
    {{end}}
  </div>
  <script src="/app.js"></script>
</body>
</html>`

	hybrid := NewDOMTreeHybrid()

	data1 := map[string]interface{}{
		"Title":   "Test App",
		"Heading": "Welcome",
		"Count":   0,
		"Items": []map[string]interface{}{
			{"Name": "Item 1"},
			{"Name": "Item 2"},
		},
	}

	data2 := map[string]interface{}{
		"Title":   "Test App", // Same
		"Heading": "Hello",    // Changed
		"Count":   5,          // Changed
		"Items": []map[string]interface{}{
			{"Name": "Item 1"}, // Same
			{"Name": "Item 2"}, // Same
			{"Name": "Item 3"}, // Added
		},
	}

	// First render
	update1, err := hybrid.GenerateTreeUpdate(templateSource, nil, data1, "complex")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Update
	update2, err := hybrid.GenerateTreeUpdate(templateSource, data1, data2, "complex")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Complex update: %s", update2.String())

	// Verify changes are reflected
	if !containsString(update2.HTML, "Hello") {
		t.Error("Should contain updated heading 'Hello'")
	}
	if !containsString(update2.HTML, "Count: 5") {
		t.Error("Should contain updated count '5'")
	}
	if !containsString(update2.HTML, "Item 3") {
		t.Error("Should contain new item 'Item 3'")
	}

	// Calculate approximate bandwidth usage
	update1Size := update1.GetUpdateSize()
	update2Size := update2.GetUpdateSize()

	t.Logf("First render size: %d bytes", update1Size)
	t.Logf("Update size: %d bytes", update2Size)

	if update2Size > 0 {
		savings := float64(update1Size-update2Size) / float64(update1Size) * 100
		t.Logf("Potential bandwidth comparison: %.1f%% difference in size", savings)
	}
}

func TestDOMTreeHybrid_JSONSerialization(t *testing.T) {
	templateSource := `<div>Hello {{.Name}}</div>`
	hybrid := NewDOMTreeHybrid()

	data := map[string]interface{}{"Name": "World"}

	update, err := hybrid.GenerateTreeUpdate(templateSource, nil, data, "json-test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Test JSON serialization (important for WebSocket transmission)
	jsonData, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("Failed to serialize to JSON: %v", err)
	}

	// Test deserialization
	var decoded TreeUpdate
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to deserialize from JSON: %v", err)
	}

	// Verify data integrity
	if decoded.FragmentID != update.FragmentID {
		t.Error("FragmentID mismatch after JSON round-trip")
	}
	if decoded.HTML != update.HTML {
		t.Error("HTML mismatch after JSON round-trip")
	}

	t.Logf("JSON serialization test passed. Size: %d bytes", len(jsonData))
}

// Helper function to check if string contains substring
func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) > 0 &&
		(haystack == needle || strings.Contains(haystack, needle))
}
