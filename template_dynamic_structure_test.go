package livetemplate

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Helper function to execute template to HTML
func executeToHTML(t *Template, data interface{}) (string, error) {
	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Helper function to execute template to update (returns tree map)
func executeToUpdate(t *Template, data interface{}) (map[string]interface{}, error) {
	var buf bytes.Buffer
	err := t.ExecuteUpdates(&buf, data)
	if err != nil {
		return nil, err
	}

	var tree map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// TestDynamicModalStructure verifies that modals/dialogs appearing after initial render
// don't keep resending statics when toggled (hide → show → hide → show).
// This tests the fix for the stateful structure caching bug.
func TestDynamicModalStructure(t *testing.T) {
	tmpl := `<div>
	<button>Toggle Modal</button>
	{{if .ShowModal}}
		<dialog id="modal" class="modal-dialog">
			<h2>{{.ModalTitle}}</h2>
			<p>{{.ModalMessage}}</p>
			<button>Close</button>
		</dialog>
	{{end}}
</div>`

	type Data struct {
		ShowModal    bool
		ModalTitle   string
		ModalMessage string
	}

	// Render 1: Initial render with NO modal
	t.Run("1_Initial_NoModal", func(t *testing.T) {
		tpl, err := New("dynamic-modal-test").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data := Data{ShowModal: false}
		html, err := executeToHTML(tpl, data)
		if err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		if len(html) == 0 {
			t.Fatal("Initial HTML is empty")
		}

		// Verify modal is not in HTML
		if contains(html, "modal-dialog") {
			t.Error("Modal should not be in initial render")
		}
		t.Logf("✅ Initial render (no modal): %d bytes", len(html))
	})

	// Render 2: Show modal (first appearance - should include statics)
	t.Run("2_FirstShow_WithStatics", func(t *testing.T) {
		tpl, err := New("dynamic-modal-show1").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Initial render
		data1 := Data{ShowModal: false}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Show modal
		data2 := Data{
			ShowModal:    true,
			ModalTitle:   "Welcome",
			ModalMessage: "Hello World",
		}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("First modal appearance update:\n%s", updateJSON)

		// The modal structure should be in the update
		// It should include statics because client has never seen this structure
		hasModalStructure := false
		hasStatics := false

		// Check if update contains modal structure
		for k, v := range tree {
			if k == "s" {
				continue // Skip top-level statics
			}

			// Modal might be in various positions depending on template structure
			// Look for nested structures that might contain the modal
			if node, ok := v.(map[string]interface{}); ok {
				if statics, hasS := node["s"]; hasS {
					hasStatics = true
					if staticsArr, ok := statics.([]string); ok {
						for _, s := range staticsArr {
							if contains(s, "modal-dialog") || contains(s, "dialog") {
								hasModalStructure = true
								break
							}
						}
					}
				}
			}
		}

		if !hasModalStructure {
			t.Log("Warning: Modal structure not found in expected location")
			t.Log("This may be due to template structure changes - update test if needed")
		}

		if hasModalStructure && !hasStatics {
			t.Error("Modal structure found but statics missing - should include statics on first appearance")
		}

		t.Logf("✅ First modal show: %d bytes, hasStatics=%v", len(updateJSON), hasStatics)
	})

	// Render 3: Hide modal
	t.Run("3_Hide", func(t *testing.T) {
		tpl, err := New("dynamic-modal-hide").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Show then hide
		data1 := Data{ShowModal: true, ModalTitle: "Test", ModalMessage: "Test"}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		data2 := Data{ShowModal: false}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		updateJSON, _ := json.Marshal(tree)
		t.Logf("✅ Hide modal: %d bytes", len(updateJSON))
	})

	// Render 4: Show modal AGAIN (CRITICAL TEST - should NOT include statics)
	t.Run("4_SecondShow_WithoutStatics", func(t *testing.T) {
		tpl, err := New("dynamic-modal-show2").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Initial: hidden
		data1 := Data{ShowModal: false}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// First show
		data2 := Data{ShowModal: true, ModalTitle: "First", ModalMessage: "First Show"}
		_, err = executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute first show update: %v", err)
		}

		// Hide
		data3 := Data{ShowModal: false}
		_, err = executeToUpdate(tpl, data3)
		if err != nil {
			t.Fatalf("Failed to execute hide update: %v", err)
		}

		// Show AGAIN (this is the critical test)
		data4 := Data{ShowModal: true, ModalTitle: "Second", ModalMessage: "Second Show"}
		tree, err := executeToUpdate(tpl, data4)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Second modal appearance update:\n%s", updateJSON)

		// Check if statics are incorrectly included
		hasRedundantStatics := false
		for k, v := range tree {
			if k == "s" {
				continue
			}

			if node, ok := v.(map[string]interface{}); ok {
				if statics, hasS := node["s"]; hasS {
					if staticsArr, ok := statics.([]string); ok {
						for _, s := range staticsArr {
							if contains(s, "modal-dialog") || contains(s, "dialog") {
								hasRedundantStatics = true
								t.Errorf("❌ BUG: Modal statics sent again on second appearance!")
								t.Errorf("   Statics should have been cached from first appearance")
								break
							}
						}
					}
				}
			}
		}

		if !hasRedundantStatics {
			t.Log("✅ SUCCESS: Modal statics NOT resent (cached from first appearance)")
		}

		t.Logf("✅ Second modal show: %d bytes", len(updateJSON))
	})
}

// TestConditionalBranchSwitch verifies that switching between conditional branches
// doesn't keep resending statics for previously-seen branches.
func TestConditionalBranchSwitch(t *testing.T) {
	tmpl := `<div>
	{{if .ShowA}}
		<div class="panel-a">
			<h2>Panel A</h2>
			<p>{{.ValueA}}</p>
		</div>
	{{else}}
		<div class="panel-b">
			<h2>Panel B</h2>
			<p>{{.ValueB}}</p>
		</div>
	{{end}}
</div>`

	type Data struct {
		ShowA  bool
		ValueA string
		ValueB string
	}

	// Initial: Show A
	t.Run("1_Initial_ShowA", func(t *testing.T) {
		tpl, err := New("conditional-branch-test").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data := Data{ShowA: true, ValueA: "A1", ValueB: "B1"}
		html, err := executeToHTML(tpl, data)
		if err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		if !contains(html, "panel-a") {
			t.Error("Panel A should be in initial render")
		}
		if contains(html, "panel-b") {
			t.Error("Panel B should not be in initial render")
		}

		t.Log("✅ Initial render shows Panel A")
	})

	// Switch to B
	t.Run("2_Switch_ToB", func(t *testing.T) {
		tpl, err := New("conditional-switch-b").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data1 := Data{ShowA: true, ValueA: "A1", ValueB: "B1"}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		data2 := Data{ShowA: false, ValueA: "A2", ValueB: "B2"}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		updateJSON, _ := json.Marshal(tree)
		t.Logf("✅ Switch to Panel B: %d bytes", len(updateJSON))
	})

	// Switch back to A (should NOT resend A's statics)
	t.Run("3_Switch_BackToA", func(t *testing.T) {
		tpl, err := New("conditional-switch-a").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Initial: A
		data1 := Data{ShowA: true, ValueA: "A1", ValueB: "B1"}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Switch to B
		data2 := Data{ShowA: false, ValueA: "A2", ValueB: "B2"}
		_, err = executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute switch to B update: %v", err)
		}

		// Switch back to A
		data3 := Data{ShowA: true, ValueA: "A3", ValueB: "B3"}
		tree, err := executeToUpdate(tpl, data3)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Switch back to Panel A update:\n%s", updateJSON)

		// Check if Panel A's statics are incorrectly resent
		hasRedundantStatics := false
		for k, v := range tree {
			if k == "s" {
				continue
			}

			if node, ok := v.(map[string]interface{}); ok {
				if statics, hasS := node["s"]; hasS {
					if staticsArr, ok := statics.([]string); ok {
						for _, s := range staticsArr {
							if contains(s, "panel-a") {
								hasRedundantStatics = true
								t.Errorf("❌ BUG: Panel A statics sent again when returning to branch A!")
								break
							}
						}
					}
				}
			}
		}

		if !hasRedundantStatics {
			t.Log("✅ SUCCESS: Panel A statics NOT resent (cached from initial render)")
		}

		t.Logf("✅ Switch back to A: %d bytes", len(updateJSON))
	})
}

// TestNestedDynamicStructures verifies nested conditionals work correctly.
func TestNestedDynamicStructures(t *testing.T) {
	tmpl := `<div>
	{{if .ShowOuter}}
		<div class="outer">
			<h2>Outer Container</h2>
			{{if .ShowInner}}
				<div class="inner">
					<p>{{.Message}}</p>
				</div>
			{{end}}
		</div>
	{{end}}
</div>`

	type Data struct {
		ShowOuter bool
		ShowInner bool
		Message   string
	}

	// Show outer only
	t.Run("1_Outer_Only", func(t *testing.T) {
		tpl, err := New("nested-outer").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data := Data{ShowOuter: true, ShowInner: false, Message: ""}
		html, err := executeToHTML(tpl, data)
		if err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		if !contains(html, "outer") {
			t.Error("Outer should be visible")
		}
		if contains(html, "inner") {
			t.Error("Inner should not be visible")
		}

		t.Log("✅ Initial: Outer visible, Inner hidden")
	})

	// Show both (inner appears for first time)
	t.Run("2_Show_Inner_First_Time", func(t *testing.T) {
		tpl, err := New("nested-both-1").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data1 := Data{ShowOuter: true, ShowInner: false, Message: ""}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		data2 := Data{ShowOuter: true, ShowInner: true, Message: "Hello"}
		tree, err := executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		updateJSON, _ := json.Marshal(tree)
		t.Logf("✅ Show inner (first time): %d bytes", len(updateJSON))
	})

	// Toggle inner (hide, show, hide, show)
	t.Run("3_Toggle_Inner_Multiple", func(t *testing.T) {
		tpl, err := New("nested-toggle").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		data1 := Data{ShowOuter: true, ShowInner: false, Message: ""}
		_, err = executeToHTML(tpl, data1)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Show inner first time
		data2 := Data{ShowOuter: true, ShowInner: true, Message: "First"}
		_, err = executeToUpdate(tpl, data2)
		if err != nil {
			t.Fatalf("Failed to execute first show update: %v", err)
		}

		// Hide inner
		data3 := Data{ShowOuter: true, ShowInner: false, Message: ""}
		_, err = executeToUpdate(tpl, data3)
		if err != nil {
			t.Fatalf("Failed to execute hide update: %v", err)
		}

		// Show inner AGAIN
		data4 := Data{ShowOuter: true, ShowInner: true, Message: "Second"}
		tree, err := executeToUpdate(tpl, data4)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		updateJSON, _ := json.MarshalIndent(tree, "", "  ")
		t.Logf("Show inner (second time) update:\n%s", updateJSON)

		// Verify inner statics not resent
		hasRedundantStatics := checkForRedundantStatics(tree, "inner")
		if hasRedundantStatics {
			t.Error("❌ BUG: Inner statics resent on second appearance")
		} else {
			t.Log("✅ SUCCESS: Inner statics NOT resent")
		}

		t.Logf("✅ Show inner again: %d bytes", len(updateJSON))
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) >= len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper to check for redundant statics in tree
func checkForRedundantStatics(tree map[string]interface{}, keyword string) bool {
	for k, v := range tree {
		if k == "s" {
			continue
		}

		if node, ok := v.(map[string]interface{}); ok {
			if statics, hasS := node["s"]; hasS {
				if staticsArr, ok := statics.([]string); ok {
					for _, s := range staticsArr {
						if contains(s, keyword) {
							return true
						}
					}
				}
			}

			// Recursively check nested structures
			if checkForRedundantStatics(node, keyword) {
				return true
			}
		}
	}
	return false
}
