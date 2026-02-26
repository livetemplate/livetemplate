package invariants

import (
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

func TestTypeScriptOracle_SimpleField(t *testing.T) {
	clientDir, err := FindClientDir()
	if err != nil {
		t.Skip("TypeScript client not found:", err)
	}

	oracle, err := NewTypeScriptOracle(clientDir)
	if err != nil {
		t.Fatalf("Failed to create oracle: %v", err)
	}
	defer func() {
		if err := oracle.Close(); err != nil {
			t.Errorf("Failed to close oracle: %v", err)
		}
	}()

	oldTree := &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]any{"0": "hello"},
	}

	diffTree := &build.TreeNode{
		Dynamics: map[string]any{"0": "world"},
	}

	response, err := oracle.ApplyDiff(oldTree, diffTree)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}

	expectedHTML := "<div>world</div>"
	if normalizeHTML(response.HTML) != normalizeHTML(expectedHTML) {
		t.Errorf("HTML mismatch:\n  got:      %q\n  expected: %q", response.HTML, expectedHTML)
	}
}

func TestTypeScriptOracle_RangeAppend(t *testing.T) {
	clientDir, err := FindClientDir()
	if err != nil {
		t.Skip("TypeScript client not found:", err)
	}

	oracle, err := NewTypeScriptOracle(clientDir)
	if err != nil {
		t.Fatalf("Failed to create oracle: %v", err)
	}
	defer func() {
		if err := oracle.Close(); err != nil {
			t.Errorf("Failed to close oracle: %v", err)
		}
	}()

	// Old tree with one item in range
	oldTreeMap := map[string]any{
		"s": []any{"<ul>", "</ul>"},
		"0": map[string]any{
			"s": []any{"<li>", "</li>"},
			"d": []any{
				map[string]any{"0": "item-1", "_k": "k1"},
			},
		},
	}

	// Diff: append a new item
	diffTreeMap := map[string]any{
		"0": []any{
			[]any{"a", []any{map[string]any{"0": "item-2", "_k": "k2"}}},
		},
	}

	response, err := oracle.ApplyDiffRaw(oldTreeMap, diffTreeMap)
	if err != nil {
		t.Fatalf("ApplyDiffRaw error: %v", err)
	}

	t.Logf("HTML result: %s", response.HTML)

	// Should contain both items
	if !strings.Contains(response.HTML, "item-1") {
		t.Errorf("HTML should contain item-1: %s", response.HTML)
	}
	if !strings.Contains(response.HTML, "item-2") {
		t.Errorf("HTML should contain item-2: %s", response.HTML)
	}
}

func TestTypeScriptOracle_MultipleCalls(t *testing.T) {
	clientDir, err := FindClientDir()
	if err != nil {
		t.Skip("TypeScript client not found:", err)
	}

	oracle, err := NewTypeScriptOracle(clientDir)
	if err != nil {
		t.Fatalf("Failed to create oracle: %v", err)
	}
	defer func() {
		if err := oracle.Close(); err != nil {
			t.Errorf("Failed to close oracle: %v", err)
		}
	}()

	// Make multiple calls to verify the persistent process works
	for i := 0; i < 10; i++ {
		oldTree := &build.TreeNode{
			Statics:  []string{"<div>", "</div>"},
			Dynamics: map[string]any{"0": "old"},
		}

		diffTree := &build.TreeNode{
			Dynamics: map[string]any{"0": "new"},
		}

		response, err := oracle.ApplyDiff(oldTree, diffTree)
		if err != nil {
			t.Fatalf("ApplyDiff error on iteration %d: %v", i, err)
		}

		expectedHTML := "<div>new</div>"
		if normalizeHTML(response.HTML) != normalizeHTML(expectedHTML) {
			t.Errorf("Iteration %d: HTML mismatch:\n  got:      %q\n  expected: %q", i, response.HTML, expectedHTML)
		}
	}
}

func TestTypeScriptOracle_ConditionalBranchChange(t *testing.T) {
	clientDir, err := FindClientDir()
	if err != nil {
		t.Skip("TypeScript client not found:", err)
	}

	oracle, err := NewTypeScriptOracle(clientDir)
	if err != nil {
		t.Fatalf("Failed to create oracle: %v", err)
	}
	defer func() {
		if err := oracle.Close(); err != nil {
			t.Errorf("Failed to close oracle: %v", err)
		}
	}()

	// Old: conditional shows "ON"
	oldTree := &build.TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]any{
			"0": &build.TreeNode{
				Statics: []string{"<span class=\"on\">", "</span>"},
			},
		},
	}

	// Diff: conditional changes to show "OFF"
	diffTree := &build.TreeNode{
		Dynamics: map[string]any{
			"0": &build.TreeNode{
				Statics: []string{"<span class=\"off\">", "</span>"},
			},
		},
	}

	response, err := oracle.ApplyDiff(oldTree, diffTree)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}

	t.Logf("HTML result: %s", response.HTML)

	// Should have the OFF class
	if !strings.Contains(response.HTML, "off") {
		t.Errorf("HTML should contain 'off' class: %s", response.HTML)
	}
}
