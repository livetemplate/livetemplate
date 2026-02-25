package diff

import (
	"html/template"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/parse"
)

// mockKeyGen implements parse.KeyGenerator for testing
type mockKeyGen struct {
	counter int
}

func newMockKeyGen() *mockKeyGen {
	return &mockKeyGen{}
}

func (m *mockKeyGen) Next() string {
	m.counter++
	return string(rune('a' + m.counter - 1))
}

// TestE2E_EmptyToItems_Statics tests the full flow from template parsing
// through tree building to diff generation, verifying statics are preserved
func TestE2E_EmptyToItems_Statics(t *testing.T) {
	// Template similar to todos
	templateStr := `{{range .Items}}<li id="{{.ID}}">{{.Text}}</li>{{end}}`

	tmpl, err := parse.Parse(templateStr, template.FuncMap{})
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	type Item struct {
		ID   string
		Text string
	}

	// First render with empty items
	dataEmpty := struct{ Items []Item }{Items: nil}
	ctx := build.NewContext()
	ctx.TemplateName = "test"
	keyGen := newMockKeyGen()

	tree1, err := parse.BuildTree(tmpl, dataEmpty, keyGen, ctx)
	if err != nil {
		t.Fatalf("Empty render failed: %v", err)
	}

	t.Logf("Empty tree - TreeNode.Statics: %v, Range.Statics: %v", tree1.Statics, tree1.Range.Statics)

	// Update render with items added
	dataWithItems := struct{ Items []Item }{
		Items: []Item{{ID: "1", Text: "First"}},
	}
	ctx2 := build.NewUpdateContext(nil)
	ctx2.TemplateName = "test"
	keyGen2 := newMockKeyGen()

	tree2, err := parse.BuildTree(tmpl, dataWithItems, keyGen2, ctx2)
	if err != nil {
		t.Fatalf("Update render failed: %v", err)
	}

	t.Logf("With items tree - TreeNode.Statics: %v, Range.Statics: %v", tree2.Statics, tree2.Range.Statics)

	// Verify Range.Statics is populated in new tree
	if tree2.Range.Statics == nil {
		t.Fatal("tree2.Range.Statics is nil - fix in parse layer not working!")
	}

	// Now generate diff operations
	// stripStatics=false because client hasn't seen this structure
	operations := GenerateRangeDifferentialOperations(tree1, tree2, false)

	t.Logf("Operations: %v", operations)

	if len(operations) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(operations))
	}

	op, ok := operations[0].([]any)
	if !ok {
		t.Fatalf("Expected operation to be []interface{}, got %T", operations[0])
	}

	// Format: ['a', items, statics, metadata]
	if op[0] != "a" {
		t.Errorf("Expected 'a' operation, got %v", op[0])
	}

	if len(op) < 3 {
		t.Fatalf("Operation too short, expected at least 3 elements: %v", op)
	}

	statics := op[2]
	t.Logf("Statics in append operation: %v (type: %T)", statics, statics)

	if statics == nil {
		t.Error("Statics in append operation should NOT be nil!")
	}

	staticsSlice, ok := statics.([]string)
	if !ok {
		t.Fatalf("Expected statics to be []string, got %T", statics)
	}

	if len(staticsSlice) == 0 {
		t.Error("Statics slice should not be empty")
	}

	t.Logf("✅ End-to-end test passed - statics properly propagated: %v", staticsSlice)
}
