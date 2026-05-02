package parse

import (
	"html/template"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestRangeStaticsAlwaysPopulated ensures Range.Statics is populated even during updates
// when ShouldIncludeStatics() returns false.
func TestRangeStaticsAlwaysPopulated(t *testing.T) {
	templateStr := `{{range .Items}}<li id="{{.ID}}">{{.Text}}</li>{{end}}`

	tmpl, err := Parse(templateStr, template.FuncMap{})
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	type Item struct {
		ID   string
		Text string
	}
	data := struct{ Items []Item }{
		Items: []Item{{ID: "1", Text: "First"}},
	}

	// First render (with statics)
	ctx := build.NewContext()
	ctx.TemplateName = "test"

	tree1, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	t.Logf("First render - TreeNode.Statics: %v", tree1.Statics)
	t.Logf("First render - Range.Statics: %v", tree1.Range.Statics)

	if tree1.Range.Statics == nil {
		t.Error("First render: Range.Statics should not be nil")
	}

	// Update render (context says don't include statics)
	data.Items = append(data.Items, Item{ID: "2", Text: "Second"})

	ctx2 := build.NewUpdateContext(nil)
	ctx2.TemplateName = "test"

	tree2, err := BuildTree(tmpl, data, ctx2)
	if err != nil {
		t.Fatalf("Update render failed: %v", err)
	}

	t.Logf("Update render - TreeNode.Statics: %v", tree2.Statics)
	t.Logf("Update render - Range.Statics: %v", tree2.Range.Statics)

	// CRITICAL: Range.Statics MUST be populated even during updates
	// because diff operations need statics to:
	// 1. Determine key position via detectIDKey()
	// 2. Send proper statics in append/prepend/insert operations
	if tree2.Range.Statics == nil {
		t.Error("Update render: Range.Statics should NOT be nil - this breaks diff operations!")
	}
}

// TestRangeStaticsEmptyToItems ensures Range.Statics is populated during empty->items transition
func TestRangeStaticsEmptyToItems(t *testing.T) {
	templateStr := `{{range .Items}}<li id="{{.ID}}">{{.Text}}</li>{{end}}`

	tmpl, err := Parse(templateStr, template.FuncMap{})
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	type Item struct {
		ID   string
		Text string
	}

	// First render with empty items (simulates empty state)
	data := struct{ Items []Item }{Items: nil}
	ctx := build.NewContext()
	ctx.TemplateName = "test"

	tree1, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("Empty render failed: %v", err)
	}
	t.Logf("Empty render - Range.Items: %v, Range.Statics: %v, TreeNode.Statics: %v",
		tree1.Range.Items, tree1.Range.Statics, tree1.Statics)

	// Update render with items added
	data.Items = []Item{{ID: "1", Text: "First"}}
	ctx2 := build.NewUpdateContext(nil)
	ctx2.TemplateName = "test"

	tree2, err := BuildTree(tmpl, data, ctx2)
	if err != nil {
		t.Fatalf("Update with items failed: %v", err)
	}

	t.Logf("With items - Range.Items: %d items, Range.Statics: %v, TreeNode.Statics: %v",
		len(tree2.Range.Items), tree2.Range.Statics, tree2.Statics)

	// CRITICAL: Range.Statics must have the item template
	if tree2.Range.Statics == nil {
		t.Error("Range.Statics should have the item template for empty->items transition!")
	}
	if len(tree2.Range.Statics) == 0 {
		t.Error("Range.Statics should not be empty - needs item template for rendering!")
	}
}

// TestTodosScenario simulates the exact todos template scenario with conditional
func TestTodosScenario(t *testing.T) {
	templateStr := `{{range .Items}}<tr id="{{.ID}}">{{if .Completed}}completed{{else}}pending{{end}}</tr>{{end}}`

	tmpl, err := Parse(templateStr, template.FuncMap{})
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	type Item struct {
		ID        string
		Text      string
		Completed bool
	}

	// First render with empty items (simulates empty state)
	data := struct{ Items []Item }{Items: nil}
	ctx := build.NewContext()
	ctx.TemplateName = "test"

	tree1, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("Empty render failed: %v", err)
	}
	t.Logf("Empty render - Range.Items: %v, Range.Statics: %v", tree1.Range.Items, tree1.Range.Statics)

	// Update render with items added
	data.Items = []Item{{ID: "1", Text: "First"}}
	ctx2 := build.NewUpdateContext(nil)
	ctx2.TemplateName = "test"

	tree2, err := BuildTree(tmpl, data, ctx2)
	if err != nil {
		t.Fatalf("Update with items failed: %v", err)
	}

	t.Logf("With items - Range.Items: %d items, Range.Statics: %v", len(tree2.Range.Items), tree2.Range.Statics)

	// CRITICAL: Range.Statics must have the item template
	if tree2.Range.Statics == nil {
		t.Error("Range.Statics should have the item template for empty->items transition!")
	}
	if len(tree2.Range.Statics) == 0 {
		t.Error("Range.Statics should not be empty - needs item template for rendering!")
	}
}
