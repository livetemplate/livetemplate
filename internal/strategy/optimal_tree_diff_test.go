package strategy

import (
	"encoding/json"
	"testing"
)

func TestOptimalTreeDiff_CounterExample(t *testing.T) {
	templateSource := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>
<button data-action="increment">+</button>
<button data-action="decrement">-</button>`

	differ := NewOptimalTreeDiff()

	// Initial state
	initialData := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b",
	}

	// First render - should include statics AND dynamics
	update1, err := differ.GenerateOptimalUpdate(templateSource, nil, initialData, "counter")
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	t.Logf("First render:\n%s", update1.String())

	// Verify first render has both statics and dynamics
	if update1.Type != "full" {
		t.Errorf("Expected type 'full', got '%s'", update1.Type)
	}
	if len(update1.Statics) == 0 {
		t.Error("Expected static segments in first render")
	}
	if len(update1.Dynamics) == 0 {
		t.Error("Expected dynamic values in first render")
	}

	t.Logf("Static segments extracted: %d", len(update1.Statics))
	t.Logf("Dynamic values: %v", update1.Dynamics)

	// Update state
	updatedData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#45b7d1",
	}

	// Second render - should only send dynamics (no statics!)
	update2, err := differ.GenerateOptimalUpdate(templateSource, initialData, updatedData, "counter")
	if err != nil {
		t.Fatalf("Update render failed: %v", err)
	}

	t.Logf("Update render:\n%s", update2.String())

	// Verify update only has dynamics, NO statics
	if update2.Type != "dynamic" {
		t.Errorf("Expected type 'dynamic', got '%s'", update2.Type)
	}
	if len(update2.Statics) > 0 {
		t.Error("Update should NOT include static segments (cached on client)")
	}
	if len(update2.Dynamics) == 0 {
		t.Error("Expected dynamic values in update")
	}

	// No-change test
	update3, err := differ.GenerateOptimalUpdate(templateSource, updatedData, updatedData, "counter")
	if err != nil {
		t.Fatalf("No-change render failed: %v", err)
	}

	if update3.Type != "none" {
		t.Errorf("Expected type 'none' for no-change, got '%s'", update3.Type)
	}

	// Calculate bandwidth savings
	firstSize := update1.GetUpdateSize()
	updateSize := update2.GetUpdateSize()

	t.Logf("\n=== BANDWIDTH ANALYSIS ===")
	t.Logf("First render: %d bytes (includes statics + dynamics)", firstSize)
	t.Logf("Updates: %d bytes (dynamics only!)", updateSize)
	t.Logf("Savings: %.1f%% bandwidth saved on updates",
		float64(firstSize-updateSize)/float64(firstSize)*100)

	// The key insight: updates should be MUCH smaller
	if updateSize > firstSize/2 {
		t.Errorf("Updates should be <50%% of first render size. Got %d vs %d",
			updateSize, firstSize)
	}
}

func TestOptimalTreeDiff_StaticDynamicSeparation(t *testing.T) {
	// Template with clear static/dynamic boundaries
	templateSource := `<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>
  <h1>Welcome {{.User}}</h1>
  <p>Count: {{.Count}}</p>
  <div class="status">{{.Status}}</div>
</body>
</html>`

	differ := NewOptimalTreeDiff()

	data1 := map[string]interface{}{
		"Title":  "MyApp",
		"User":   "Alice",
		"Count":  0,
		"Status": "Active",
	}

	// First render
	update1, err := differ.GenerateOptimalUpdate(templateSource, nil, data1, "test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("First render - Statics extracted:")
	for i, s := range update1.Statics {
		t.Logf("  S[%d]: %q", i, s)
	}

	t.Logf("First render - Dynamics:")
	for k, v := range update1.Dynamics {
		t.Logf("  D[%s]: %v", k, v)
	}

	// Verify static segments are the HTML structure
	expectedStaticCount := 5 // Before and after each {{}} expression
	if len(update1.Statics) < expectedStaticCount {
		t.Errorf("Expected at least %d static segments, got %d",
			expectedStaticCount, len(update1.Statics))
	}

	// Verify dynamics match our data
	if len(update1.Dynamics) != 4 {
		t.Errorf("Expected 4 dynamic values, got %d", len(update1.Dynamics))
	}

	// Update only dynamic values
	data2 := map[string]interface{}{
		"Title":  "MyApp",    // Same
		"User":   "Bob",      // Changed
		"Count":  5,          // Changed
		"Status": "Inactive", // Changed
	}

	update2, err := differ.GenerateOptimalUpdate(templateSource, data1, data2, "test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("\nUpdate - Dynamics only:")
	for k, v := range update2.Dynamics {
		t.Logf("  D[%s]: %v", k, v)
	}

	// Key assertion: NO statics in update
	if len(update2.Statics) > 0 {
		t.Error("❌ Update should NOT send static segments!")
	}

	// Verify only dynamics sent
	if len(update2.Dynamics) == 0 {
		t.Error("❌ Update should send dynamic values!")
	}
}

func TestOptimalTreeDiff_JSONEfficiency(t *testing.T) {
	templateSource := `<div>Count: {{.Count}}</div>`
	differ := NewOptimalTreeDiff()

	data1 := map[string]interface{}{"Count": 0}
	data2 := map[string]interface{}{"Count": 1}

	// First render
	update1, err := differ.GenerateOptimalUpdate(templateSource, nil, data1, "json-test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Update
	update2, err := differ.GenerateOptimalUpdate(templateSource, data1, data2, "json-test")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Serialize updates to JSON
	json1, _ := json.Marshal(update1)
	json2, _ := json.Marshal(update2)

	t.Logf("First render JSON: %d bytes", len(json1))
	t.Logf("First render: %s", string(json1))

	t.Logf("\nUpdate JSON: %d bytes", len(json2))
	t.Logf("Update: %s", string(json2))

	// The update JSON should be TINY - just dynamics
	if len(json2) > 100 {
		t.Errorf("Update JSON too large: %d bytes. Should be minimal (just dynamics)",
			len(json2))
	}

	// Verify structure
	var decoded OptimalTreeUpdate
	if err := json.Unmarshal(json2, &decoded); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if decoded.Type != "dynamic" {
		t.Errorf("Expected type 'dynamic', got '%s'", decoded.Type)
	}

	if len(decoded.Statics) > 0 {
		t.Error("Update JSON should NOT contain statics")
	}
}
