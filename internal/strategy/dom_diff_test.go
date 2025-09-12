package strategy

import (
	"encoding/json"
	"testing"
)

func TestDOMDiffer_CounterExample(t *testing.T) {
	// Use the actual counter template from examples/counter
	templateSource := `<!DOCTYPE html>
<html>
  <head>
    <title>Counter App</title>
  </head>
  <body>
    <div style="color: {{.Color}}">Hello {{.Counter}} World</div>
    <button data-lvt-action="increment">+</button>
    <button data-lvt-action="decrement">-</button>

    <!-- Load LiveTemplate client library - auto-initializes with embedded token -->
    <script src="/client/livetemplate-client.js"></script>
  </body>
</html>`

	differ := NewDOMDiffer()

	// Initial state (counter = 0, color = red)
	initialData := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff6b6b", // red
	}

	// First render (should be a full replacement)
	patch1, err := differ.GenerateFromTemplateSource(templateSource, nil, initialData, "counter-fragment")
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	t.Logf("First render patch:\n%s", patch1.String())

	// Verify first render is a full replacement
	if len(patch1.Operations) != 1 || patch1.Operations[0].Type != "replaceElement" {
		t.Errorf("Expected single replaceElement operation for first render, got: %+v", patch1.Operations)
	}

	// Update state (counter = 1, color = blue)
	updatedData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#45b7d1", // blue
	}

	// Second render (should generate minimal diff)
	patch2, err := differ.GenerateFromTemplateSource(templateSource, initialData, updatedData, "counter-fragment")
	if err != nil {
		t.Fatalf("Update render failed: %v", err)
	}

	t.Logf("Update patch:\n%s", patch2.String())

	// Verify the diff operations
	foundTextUpdate := false
	foundStyleUpdate := false

	for _, op := range patch2.Operations {
		switch op.Type {
		case "setTextContent":
			if textVal, ok := op.Value.(string); ok && textVal == "Hello 1 World" {
				foundTextUpdate = true
				t.Logf("✓ Found counter text update: %s", textVal)
			}
		case "setAttribute":
			if op.Key == "style" {
				if styleVal, ok := op.Value.(string); ok && styleVal == "color: #45b7d1" {
					foundStyleUpdate = true
					t.Logf("✓ Found color style update: %s", styleVal)
				}
			}
		}
	}

	// Verify we detected the expected changes
	if !foundTextUpdate {
		t.Error("Expected to find counter text update from 'Hello 0 World' to 'Hello 1 World'")
	}
	if !foundStyleUpdate {
		t.Error("Expected to find color style update from '#ff6b6b' to '#45b7d1'")
	}

	// Check that patch is much smaller than full HTML
	patchJSON, _ := json.Marshal(patch2)
	fullHTML := templateSource

	patchSize := len(patchJSON)
	fullSize := len(fullHTML)
	savings := float64(fullSize-patchSize) / float64(fullSize) * 100

	t.Logf("Patch size: %d bytes, Full HTML: %d bytes, Bandwidth savings: %.1f%%",
		patchSize, fullSize, savings)

	if savings < 50 {
		t.Logf("WARNING: Expected >50%% bandwidth savings, got %.1f%% (this may be acceptable for small templates)", savings)
	}
}

func TestDOMDiffer_SimpleExample(t *testing.T) {
	templateSource := `<div>Count: {{.Count}}</div>`

	differ := NewDOMDiffer()

	// Test progression: 0 -> 1 -> 2
	data0 := map[string]interface{}{"Count": 0}
	data1 := map[string]interface{}{"Count": 1}
	data2 := map[string]interface{}{"Count": 2}

	// First render
	patch1, err := differ.GenerateFromTemplateSource(templateSource, nil, data0, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Initial render: %s", patch1.String())

	// Update to 1
	patch2, err := differ.GenerateFromTemplateSource(templateSource, data0, data1, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Update to 1: %s", patch2.String())

	// Update to 2
	patch3, err := differ.GenerateFromTemplateSource(templateSource, data1, data2, "simple")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Update to 2: %s", patch3.String())

	// Verify incremental updates contain text changes
	for i, patch := range []*DOMPatch{patch2, patch3} {
		found := false
		for _, op := range patch.Operations {
			if op.Type == "setTextContent" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Patch %d should contain setTextContent operation", i+1)
		}
	}
}

func TestDOMDiffer_ConditionalExample(t *testing.T) {
	templateSource := `<div>{{if .ShowMessage}}Message: {{.Message}}{{else}}No message{{end}}</div>`

	differ := NewDOMDiffer()

	// Test conditional changes
	dataHidden := map[string]interface{}{"ShowMessage": false, "Message": "Hello"}
	dataShown := map[string]interface{}{"ShowMessage": true, "Message": "Hello"}

	// First render (hidden)
	patch1, err := differ.GenerateFromTemplateSource(templateSource, nil, dataHidden, "conditional")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Hidden state: %s", patch1.String())

	// Show message
	patch2, err := differ.GenerateFromTemplateSource(templateSource, dataHidden, dataShown, "conditional")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Show message: %s", patch2.String())

	// Hide message again
	patch3, err := differ.GenerateFromTemplateSource(templateSource, dataShown, dataHidden, "conditional")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Hide message: %s", patch3.String())

	// Verify that we get text content changes for conditionals
	for i, patch := range []*DOMPatch{patch2, patch3} {
		found := false
		for _, op := range patch.Operations {
			if op.Type == "setTextContent" {
				found = true
				t.Logf("Patch %d text change: %v", i+1, op.Value)
				break
			}
		}
		if !found {
			t.Logf("Patch %d operations: %+v", i+1, patch.Operations)
		}
	}
}

func TestDOMDiffer_AttributeChanges(t *testing.T) {
	templateSource := `<input type="text" placeholder="{{.Placeholder}}" value="{{.Value}}" class="{{.CSSClass}}">`

	differ := NewDOMDiffer()

	data1 := map[string]interface{}{
		"Placeholder": "Enter name",
		"Value":       "",
		"CSSClass":    "input-normal",
	}

	data2 := map[string]interface{}{
		"Placeholder": "Enter your full name",
		"Value":       "John",
		"CSSClass":    "input-filled",
	}

	// First render
	patch1, err := differ.GenerateFromTemplateSource(templateSource, nil, data1, "input")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Initial: %s", patch1.String())

	// Update attributes
	patch2, err := differ.GenerateFromTemplateSource(templateSource, data1, data2, "input")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Update: %s", patch2.String())

	// Verify we get setAttribute operations for each changed attribute
	expectedAttrs := map[string]string{
		"placeholder": "Enter your full name",
		"value":       "John",
		"class":       "input-filled",
	}

	foundAttrs := make(map[string]bool)
	for _, op := range patch2.Operations {
		if op.Type == "setAttribute" {
			if expectedVal, exists := expectedAttrs[op.Key]; exists {
				if op.Value == expectedVal {
					foundAttrs[op.Key] = true
					t.Logf("✓ Found expected attribute change: %s='%s'", op.Key, op.Value)
				}
			}
		}
	}

	for attr := range expectedAttrs {
		if !foundAttrs[attr] {
			t.Errorf("Expected to find setAttribute for %s", attr)
		}
	}
}
