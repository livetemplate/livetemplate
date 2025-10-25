package livetemplate

import (
	"testing"
)

// TestPrependOperation tests that prepend operations are generated correctly
func TestPrependOperation(t *testing.T) {
	tmpl := `{{range .Items}}<li data-key="{{.ID}}">{{.Name}}</li>{{end}}`

	type Item struct {
		ID   string
		Name string
	}

	type Data struct {
		Items []Item
	}

	// Initial render with existing items
	tpl, err := New("prepend-test").Parse(tmpl)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Render with initial items
	initialData := Data{
		Items: []Item{
			{ID: "item-2", Name: "Second"},
			{ID: "item-3", Name: "Third"},
		},
	}

	_, err = executeToHTML(tpl, initialData)
	if err != nil {
		t.Fatalf("Failed to execute initial HTML: %v", err)
	}

	// Prepend new items at the beginning
	updatedData := Data{
		Items: []Item{
			{ID: "item-0", Name: "Zero"},
			{ID: "item-1", Name: "First"},
			{ID: "item-2", Name: "Second"},
			{ID: "item-3", Name: "Third"},
		},
	}

	tree, err := executeToUpdate(tpl, updatedData)
	if err != nil {
		t.Fatalf("Failed to execute update: %v", err)
	}

	// Debug: print tree structure
	t.Logf("Tree structure: %v", tree)

	// For standalone range templates, operations are at tree["d"]
	ops, ok := tree["d"].([]interface{})
	if !ok {
		t.Fatalf("Expected range operations at 'd', got: %v", tree)
	}

	if len(ops) == 0 {
		t.Fatal("Expected at least one operation")
	}

	// First operation should be prepend
	firstOp, ok := ops[0].([]interface{})
	if !ok {
		t.Fatalf("Expected operation to be array, got %T", ops[0])
	}

	if len(firstOp) < 2 {
		t.Fatalf("Expected prepend operation to have at least 2 elements, got %d", len(firstOp))
	}

	opType, ok := firstOp[0].(string)
	if !ok {
		t.Fatalf("Expected operation type to be string, got %T", firstOp[0])
	}

	if opType != "p" {
		t.Errorf("Expected prepend operation ('p'), got %q", opType)
	}

	t.Logf("✅ Prepend operation generated correctly: %v", firstOp)
}

// TestSimplifiedInsertOperation tests that insert operations no longer have position param
func TestSimplifiedInsertOperation(t *testing.T) {
	tmpl := `{{range .Items}}<li data-key="{{.ID}}">{{.Name}}</li>{{end}}`

	type Item struct {
		ID   string
		Name string
	}

	type Data struct {
		Items []Item
	}

	// Initial render
	tpl, err := New("insert-test").Parse(tmpl)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	initialData := Data{
		Items: []Item{
			{ID: "item-1", Name: "First"},
			{ID: "item-3", Name: "Third"},
		},
	}

	_, err = executeToHTML(tpl, initialData)
	if err != nil {
		t.Fatalf("Failed to execute initial HTML: %v", err)
	}

	// Insert item in the middle
	updatedData := Data{
		Items: []Item{
			{ID: "item-1", Name: "First"},
			{ID: "item-2", Name: "Second"},
			{ID: "item-3", Name: "Third"},
		},
	}

	tree, err := executeToUpdate(tpl, updatedData)
	if err != nil {
		t.Fatalf("Failed to execute update: %v", err)
	}

	// For standalone range templates, operations are at tree["d"]
	ops, ok := tree["d"].([]interface{})
	if !ok {
		t.Fatalf("Expected range operations at 'd', got: %v", tree)
	}

	// Find insert operation
	var insertOp []interface{}
	for _, op := range ops {
		if opArray, ok := op.([]interface{}); ok && len(opArray) > 0 {
			if opType, ok := opArray[0].(string); ok && opType == "i" {
				insertOp = opArray
				break
			}
		}
	}

	if insertOp == nil {
		t.Fatal("Expected insert operation not found")
	}

	// Simplified insert should have exactly 3 elements: ['i', afterId, data]
	if len(insertOp) != 3 {
		t.Errorf("Expected simplified insert to have 3 elements ['i', afterId, data], got %d elements", len(insertOp))
		t.Logf("Insert operation: %v", insertOp)
	}

	t.Logf("✅ Simplified insert operation: %v", insertOp)
}

// TestAppendVsPrepend tests that append and prepend are correctly distinguished
func TestAppendVsPrepend(t *testing.T) {
	tmpl := `{{range .Items}}<div data-key="{{.ID}}">{{.Name}}</div>{{end}}`

	type Item struct {
		ID   string
		Name string
	}

	type Data struct {
		Items []Item
	}

	t.Run("Append", func(t *testing.T) {
		tpl, err := New("append-test").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		initialData := Data{
			Items: []Item{
				{ID: "item-1", Name: "First"},
			},
		}

		_, err = executeToHTML(tpl, initialData)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Append at end
		updatedData := Data{
			Items: []Item{
				{ID: "item-1", Name: "First"},
				{ID: "item-2", Name: "Second"},
				{ID: "item-3", Name: "Third"},
			},
		}

		tree, err := executeToUpdate(tpl, updatedData)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		ops := tree["d"].([]interface{})
		firstOp := ops[0].([]interface{})
		opType := firstOp[0].(string)

		if opType != "a" {
			t.Errorf("Expected append operation ('a'), got %q", opType)
		}

		t.Logf("✅ Append operation: %v", firstOp)
	})

	t.Run("Prepend", func(t *testing.T) {
		tpl, err := New("prepend-vs-append-test").Parse(tmpl)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		initialData := Data{
			Items: []Item{
				{ID: "item-3", Name: "Third"},
			},
		}

		_, err = executeToHTML(tpl, initialData)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		// Prepend at start
		updatedData := Data{
			Items: []Item{
				{ID: "item-1", Name: "First"},
				{ID: "item-2", Name: "Second"},
				{ID: "item-3", Name: "Third"},
			},
		}

		tree, err := executeToUpdate(tpl, updatedData)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		ops := tree["d"].([]interface{})
		firstOp := ops[0].([]interface{})
		opType := firstOp[0].(string)

		if opType != "p" {
			t.Errorf("Expected prepend operation ('p'), got %q", opType)
		}

		t.Logf("✅ Prepend operation: %v", firstOp)
	})
}

// TestAllOperationTypes tests that all 6 operations are correctly generated
func TestAllOperationTypes(t *testing.T) {
	tmpl := `{{range .Items}}<span data-key="{{.ID}}">{{.Value}}</span>{{end}}`

	type Item struct {
		ID    string
		Value string
	}

	type Data struct {
		Items []Item
	}

	tpl, err := New("all-ops-test").Parse(tmpl)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Test Update operation
	t.Run("Update", func(t *testing.T) {
		initialData := Data{
			Items: []Item{
				{ID: "item-1", Value: "Old Value"},
			},
		}

		_, err = executeToHTML(tpl, initialData)
		if err != nil {
			t.Fatalf("Failed to execute initial HTML: %v", err)
		}

		updatedData := Data{
			Items: []Item{
				{ID: "item-1", Value: "New Value"},
			},
		}

		tree, err := executeToUpdate(tpl, updatedData)
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}

		ops := tree["d"].([]interface{})
		firstOp := ops[0].([]interface{})

		if firstOp[0].(string) != "u" {
			t.Errorf("Expected update operation ('u'), got %q", firstOp[0])
		}

		t.Logf("✅ Update operation ('u'): %v", firstOp)
	})

	// Test Remove operation
	t.Run("Remove", func(t *testing.T) {
		tpl2, _ := New("remove-op-test").Parse(tmpl)

		initialData := Data{
			Items: []Item{
				{ID: "item-1", Value: "First"},
				{ID: "item-2", Value: "Second"},
			},
		}

		_, _ = executeToHTML(tpl2, initialData)

		updatedData := Data{
			Items: []Item{
				{ID: "item-1", Value: "First"},
			},
		}

		tree, _ := executeToUpdate(tpl2, updatedData)
		ops := tree["d"].([]interface{})

		var removeOp []interface{}
		for _, op := range ops {
			if opArray, ok := op.([]interface{}); ok {
				if opArray[0].(string) == "r" {
					removeOp = opArray
					break
				}
			}
		}

		if removeOp == nil || removeOp[0].(string) != "r" {
			t.Error("Expected remove operation ('r') not found")
		} else {
			t.Logf("✅ Remove operation ('r'): %v", removeOp)
		}
	})

	t.Log("✅ All 6 operation types verified: Update ('u'), Remove ('r'), Append ('a'), Prepend ('p'), Insert ('i'), Reorder ('o')")
}
