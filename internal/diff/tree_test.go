package diff

import (
	"encoding/json"
	"testing"
)

func TestUnifiedTreeDiff_CounterExample(t *testing.T) {
	templateSource := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>`

	differ := NewTree()

	// Initial render
	data1 := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b",
	}

	update1, err := differ.Generate(templateSource, nil, data1)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	t.Logf("First render:\n%s", update1.String())

	// Verify first render has both S and D
	if !update1.HasStatics() {
		t.Error("First render should have statics")
	}
	if !update1.HasDynamics() {
		t.Error("First render should have dynamics")
	}

	// Test reconstruction
	reconstructed := update1.Reconstruct(nil)
	t.Logf("Reconstructed HTML: %s", reconstructed)

	if reconstructed != `<div style="color: #ff6b6b" lvt-id="default">Hello 0 World</div>` {
		t.Errorf("Reconstruction failed. Got: %s", reconstructed)
	}

	// Update - only dynamics change
	data2 := map[string]interface{}{
		"Counter": 1,
		"Color":   "#45b7d1",
	}

	update2, err := differ.Generate(templateSource, data1, data2)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	t.Logf("\nUpdate (dynamics only):\n%s", update2.String())

	// Update should only have dynamics, no statics
	if update2.HasStatics() {
		t.Error("Update should NOT have statics (cached on client)")
	}
	if !update2.HasDynamics() {
		t.Error("Update should have dynamics")
	}

	// Client reconstruction using cached statics
	reconstructed2 := update2.Reconstruct(update1.S)
	t.Logf("Reconstructed with cached statics: %s", reconstructed2)

	if reconstructed2 != `<div style="color: #45b7d1" lvt-id="default">Hello 1 World</div>` {
		t.Errorf("Update reconstruction failed. Got: %s", reconstructed2)
	}

	// No change scenario
	update3, err := differ.Generate(templateSource, data2, data2)
	if err != nil {
		t.Fatalf("No-change update failed: %v", err)
	}

	t.Logf("\nNo-change update:\n%s", update3.String())

	if !update3.IsEmpty() {
		t.Error("No-change update should be empty")
	}

	// Bandwidth analysis
	size1 := update1.GetSize()
	size2 := update2.GetSize()
	savings := float64(size1-size2) / float64(size1) * 100

	t.Logf("\n=== BANDWIDTH ANALYSIS ===")
	t.Logf("First render: %d bytes", size1)
	t.Logf("Updates: %d bytes", size2)
	t.Logf("Savings: %.1f%%", savings)

	if savings < 60 {
		t.Errorf("Expected >60%% savings, got %.1f%%", savings)
	}
}

func TestUnifiedTreeDiff_PureStaticTemplate(t *testing.T) {
	// Template with no dynamic parts
	templateSource := `<!DOCTYPE html>
<html>
<head><title>Static Page</title></head>
<body>
  <h1>Welcome</h1>
  <p>This is a static page with no dynamic content.</p>
</body>
</html>`

	differ := NewTree()

	update, err := differ.Generate(templateSource, nil, nil)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Pure static template:\n%s", update.String())

	// Should have statics but no dynamics
	if !update.HasStatics() {
		t.Error("Should have static content")
	}
	if update.HasDynamics() {
		t.Error("Should NOT have dynamics")
	}

	// Statics should be the entire template
	if len(update.S) != 1 || update.S[0] != templateSource {
		t.Error("Static should be the entire template")
	}
}

func TestUnifiedTreeDiff_JSONSerialization(t *testing.T) {
	templateSource := `<span>Count: {{.Count}}</span>`
	differ := NewTree()

	// First render
	data1 := map[string]interface{}{"Count": 0}
	update1, _ := differ.Generate(templateSource, nil, data1)

	json1, _ := json.Marshal(update1)
	t.Logf("First render JSON (%d bytes):\n%s", len(json1), string(json1))

	// Update
	data2 := map[string]interface{}{"Count": 42}
	update2, _ := differ.Generate(templateSource, data1, data2)

	json2, _ := json.Marshal(update2)
	t.Logf("\nUpdate JSON (%d bytes):\n%s", len(json2), string(json2))

	// Update should be tiny
	if len(json2) > 30 {
		t.Errorf("Update JSON too large: %d bytes", len(json2))
	}

	// Verify structure
	var decoded Update
	_ = json.Unmarshal(json2, &decoded)

	if len(decoded.S) > 0 {
		t.Error("Update should not have statics in JSON")
	}
	if len(decoded.Dynamics) != 1 {
		t.Error("Update should have exactly 1 dynamic value")
	}
}

func TestUnifiedTreeDiff_ComplexTemplate(t *testing.T) {
	// Mix of static and dynamic, including conditionals
	templateSource := `<div class="container">
  <h1>{{.Title}}</h1>
  {{if .ShowMessage}}
    <div class="message">{{.Message}}</div>
  {{end}}
  <footer>© 2024</footer>
</div>`

	differ := NewTree()

	data1 := map[string]interface{}{
		"Title":       "Home",
		"ShowMessage": true,
		"Message":     "Welcome!",
	}

	update1, err := differ.Generate(templateSource, nil, data1)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("Complex template first render:\n%s", update1.String())

	// Should have both statics and dynamics
	if !update1.HasStatics() || !update1.HasDynamics() {
		t.Error("Should have both statics and dynamics")
	}

	// Change only dynamic values
	data2 := map[string]interface{}{
		"Title":       "Dashboard",
		"ShowMessage": false,
		"Message":     "",
	}

	update2, err := differ.Generate(templateSource, data1, data2)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("\nComplex template update:\n%s", update2.String())

	// Update should only have dynamics
	if update2.HasStatics() {
		t.Error("Update should not have statics")
	}
	if !update2.HasDynamics() {
		t.Error("Update should have dynamics")
	}
}

func TestUnifiedTreeDiff_Reconstruction(t *testing.T) {
	templateSource := `<p>Hello {{.Name}}, you have {{.Count}} messages</p>`

	differ := NewTree()

	data := map[string]interface{}{
		"Name":  "Alice",
		"Count": 5,
	}

	update, _ := differ.Generate(templateSource, nil, data)

	// Test reconstruction
	html := update.Reconstruct(nil)
	expected := `<p>Hello Alice, you have 5 messages</p>`

	if html != expected {
		t.Errorf("Reconstruction failed.\nExpected: %s\nGot: %s", expected, html)
	}

	// Test update reconstruction with cached statics
	data2 := map[string]interface{}{
		"Name":  "Bob",
		"Count": 10,
	}

	update2, _ := differ.Generate(templateSource, data, data2)
	html2 := update2.Reconstruct(update.S)
	expected2 := `<p>Hello Bob, you have 10 messages</p>`

	if html2 != expected2 {
		t.Errorf("Update reconstruction failed.\nExpected: %s\nGot: %s", expected2, html2)
	}
}
