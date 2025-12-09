package diff

import (
	"testing"
)

// mockStructureRegistry is a simple mock implementation for testing.
type mockStructureRegistry struct {
	seen map[string]bool
}

func newMockRegistry() *mockStructureRegistry {
	return &mockStructureRegistry{
		seen: make(map[string]bool),
	}
}

func (m *mockStructureRegistry) HasSeen(path string, value interface{}) bool {
	return m.seen[path]
}

func (m *mockStructureRegistry) MarkSeen(path string, value interface{}) {
	m.seen[path] = true
}

// TestCompareTreesAndGetChangesWithPath_NoDiff tests when trees are identical.
func TestCompareTreesAndGetChangesWithPath_NoDiff(t *testing.T) {
	oldTree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "same"},
	}
	newTree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "same"},
	}

	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", nil, nil)

	if changes.HasDynamics() {
		t.Errorf("Expected no changes for identical trees, got: %+v", changes.Dynamics)
	}
}

// TestCompareTreesAndGetChangesWithPath_SimpleDiff tests simple field changes.
func TestCompareTreesAndGetChangesWithPath_SimpleDiff(t *testing.T) {
	oldTree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "old"},
	}
	newTree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "new"},
	}

	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", nil, nil)

	if !changes.HasDynamics() {
		t.Fatal("Expected changes for different values")
	}

	if changes.Dynamics["0"] != "new" {
		t.Errorf("Expected changes['0'] = 'new', got: %v", changes.Dynamics["0"])
	}
}

// TestCompareTreesAndGetChangesWithPath_NewField tests when a new field appears.
func TestCompareTreesAndGetChangesWithPath_NewField(t *testing.T) {
	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "value"},
	}
	newTree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "value",
			"1": "new field",
		},
	}

	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", nil, nil)

	if changes.Dynamics["1"] != "new field" {
		t.Errorf("Expected new field '1' = 'new field', got: %v", changes.Dynamics["1"])
	}
}

// TestCompareTreesAndGetChangesWithPath_NilTrees tests nil tree handling.
func TestCompareTreesAndGetChangesWithPath_NilTrees(t *testing.T) {
	tests := []struct {
		name      string
		oldTree   *TreeNode
		newTree   *TreeNode
		wantNil   bool
		wantEmpty bool
	}{
		{
			name:      "both nil",
			oldTree:   nil,
			newTree:   nil,
			wantEmpty: true,
		},
		{
			name:      "old nil, new has data",
			oldTree:   nil,
			newTree:   &TreeNode{Dynamics: map[string]interface{}{"0": "value"}},
			wantEmpty: false,
		},
		{
			name:      "old has data, new nil",
			oldTree:   &TreeNode{Dynamics: map[string]interface{}{"0": "value"}},
			newTree:   nil,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := CompareTreesAndGetChangesWithPath(tt.oldTree, tt.newTree, false, "", nil, nil)

			if changes == nil {
				if !tt.wantNil {
					t.Error("Expected non-nil changes")
				}
				return
			}

			isEmpty := !changes.HasDynamics()
			if isEmpty != tt.wantEmpty {
				t.Errorf("Expected isEmpty=%v, got %v", tt.wantEmpty, isEmpty)
			}
		})
	}
}

// TestCompareTreesAndGetChangesWithPath_NestedTreeNode tests nested TreeNode comparison.
func TestCompareTreesAndGetChangesWithPath_NestedTreeNode(t *testing.T) {
	oldNested := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "old"},
	}
	newNested := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "new"},
	}

	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": oldNested},
	}
	newTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": newNested},
	}

	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", nil, nil)

	if !changes.HasDynamics() {
		t.Fatal("Expected changes for nested TreeNode")
	}

	nestedChanges, ok := changes.Dynamics["0"].(*TreeNode)
	if !ok {
		t.Fatalf("Expected nested changes to be *TreeNode, got: %T", changes.Dynamics["0"])
	}

	if nestedChanges.Dynamics["0"] != "new" {
		t.Errorf("Expected nested change '0' = 'new', got: %v", nestedChanges.Dynamics["0"])
	}
}

// TestCompareTreesAndGetChangesWithPath_TopLevelRange tests top-level range constructs.
func TestCompareTreesAndGetChangesWithPath_TopLevelRange(t *testing.T) {
	item1 := map[string]interface{}{"id": "1", "name": "Item 1"}
	item2 := map[string]interface{}{"id": "2", "name": "Item 2"}

	oldTree := &TreeNode{
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items:   []interface{}{item1},
			Statics: []string{"<li>", "</li>"},
		},
	}
	newTree := &TreeNode{
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items:   []interface{}{item1, item2},
			Statics: []string{"<li>", "</li>"},
		},
	}

	rangeMatches := map[string]string{"": "matched"}
	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", rangeMatches, nil)

	// Should return changes (either operations or fallback to full tree)
	// The function may return operations or fallback depending on implementation details
	if changes == nil {
		t.Fatal("Expected non-nil changes for top-level range")
	}

	// Check if we got operations or full tree fallback
	if changes.HasDynamics() {
		// Got operations
		if ops, ok := changes.Dynamics["d"]; ok && ops != nil {
			// Successfully got differential operations
			return
		}
	}

	// Otherwise should have returned full tree (statics + range)
	if !changes.HasStatics() && !changes.HasRange() {
		t.Error("Expected either operations or full tree structure")
	}
}

// TestHandleTopLevelRange_BothRanges tests when both trees are ranges.
func TestHandleTopLevelRange_BothRanges(t *testing.T) {
	oldTree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range: &RangeData{
			Items: []interface{}{map[string]interface{}{"id": "1"}},
		},
	}
	newTree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range: &RangeData{
			Items: []interface{}{map[string]interface{}{"id": "1"}},
		},
	}

	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	rangeMatches := map[string]string{"": "matched"}

	handled := handleTopLevelRange(oldTree, newTree, "", rangeMatches, nil, changes)

	if !handled {
		t.Error("Expected handleTopLevelRange to return true for matched ranges")
	}
}

// TestHandleTopLevelRange_NewRange tests when range appears for first time.
func TestHandleTopLevelRange_NewRange(t *testing.T) {
	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "else content"},
	}
	newTree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range: &RangeData{
			Items: []interface{}{map[string]interface{}{"id": "1"}},
		},
	}

	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	handled := handleTopLevelRange(oldTree, newTree, "", nil, nil, changes)

	if !handled {
		t.Error("Expected handleTopLevelRange to return true for new range")
	}

	// Should return full new tree
	if !changes.HasRange() {
		t.Error("Expected changes to have Range data")
	}
}

// TestHandleMatchedRanges_WithOps tests matched ranges with operations.
func TestHandleMatchedRanges_WithOps(t *testing.T) {
	item1 := map[string]interface{}{"id": "1", "name": "Item 1"}
	item2 := map[string]interface{}{"id": "2", "name": "Item 2"}

	oldTree := &TreeNode{
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items:   []interface{}{item1},
			Statics: []string{"<li>", "</li>"},
		},
	}
	newTree := &TreeNode{
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items:   []interface{}{item1, item2},
			Statics: []string{"<li>", "</li>"},
		},
	}

	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	handled := handleMatchedRanges(oldTree, newTree, "", nil, changes)

	if !handled {
		t.Error("Expected handleMatchedRanges to return true")
	}

	// Function should handle the range - either with operations or fallback
	// Check if we got operations
	if _, hasOps := changes.Dynamics["d"]; hasOps {
		// Got operations - verify statics included (since no registry, first time)
		if !changes.HasStatics() {
			t.Error("Expected statics to be included when operations present (no registry)")
		}
		return
	}

	// If no operations, should have full tree as fallback
	if !changes.HasStatics() && !changes.HasRange() {
		t.Error("Expected either operations or full tree fallback")
	}
}

// TestHandleMatchedRanges_EmptyRanges tests when both ranges are empty.
func TestHandleMatchedRanges_EmptyRanges(t *testing.T) {
	oldTree := &TreeNode{
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items:   []interface{}{},
			Statics: []string{"<li>", "</li>"},
		},
	}
	newTree := &TreeNode{
		Statics: []string{"<li>", "</li>"},
		Range: &RangeData{
			Items:   []interface{}{},
			Statics: []string{"<li>", "</li>"},
		},
	}

	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	handled := handleMatchedRanges(oldTree, newTree, "", nil, changes)

	if !handled {
		t.Error("Expected handleMatchedRanges to return true for empty ranges")
	}

	// Both empty means no change
	if changes.HasDynamics() && len(changes.Dynamics) > 0 {
		t.Error("Expected no operations for both empty ranges")
	}
}

// TestCompareDynamicSegments_NewField tests new field detection.
func TestCompareDynamicSegments_NewField(t *testing.T) {
	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "value"},
	}
	newTree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "value",
			"1": "new",
		},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	compareDynamicSegments(oldTree, newTree, false, "", nil, nil, changes)

	if changes.Dynamics["1"] != "new" {
		t.Errorf("Expected new field '1' = 'new', got: %v", changes.Dynamics["1"])
	}
}

// TestCompareDynamicSegments_ChangedField tests changed field detection.
func TestCompareDynamicSegments_ChangedField(t *testing.T) {
	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "old"},
	}
	newTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "new"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	compareDynamicSegments(oldTree, newTree, false, "", nil, nil, changes)

	if changes.Dynamics["0"] != "new" {
		t.Errorf("Expected changed field '0' = 'new', got: %v", changes.Dynamics["0"])
	}
}

// TestCompareDynamicSegments_UnchangedField tests that unchanged fields are not included.
func TestCompareDynamicSegments_UnchangedField(t *testing.T) {
	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "same"},
	}
	newTree := &TreeNode{
		Dynamics: map[string]interface{}{"0": "same"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	compareDynamicSegments(oldTree, newTree, false, "", nil, nil, changes)

	if changes.HasDynamics() {
		t.Errorf("Expected no changes for unchanged field, got: %+v", changes.Dynamics)
	}
}

// TestBuildFieldPath tests path construction.
func TestBuildFieldPath(t *testing.T) {
	tests := []struct {
		name        string
		currentPath string
		key         string
		want        string
	}{
		{"empty path", "", "0", "0"},
		{"with path", "parent", "0", "parent.0"},
		{"nested path", "parent.child", "0", "parent.child.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFieldPath(tt.currentPath, tt.key)
			if got != tt.want {
				t.Errorf("buildFieldPath(%q, %q) = %q, want %q", tt.currentPath, tt.key, got, tt.want)
			}
		})
	}
}

// TestHandleNewField_Primitive tests handling new primitive fields.
func TestHandleNewField_Primitive(t *testing.T) {
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleNewField("0", "value", "0", false, nil, changes)

	if changes.Dynamics["0"] != "value" {
		t.Errorf("Expected '0' = 'value', got: %v", changes.Dynamics["0"])
	}
}

// TestHandleNewField_TreeNode tests handling new TreeNode fields.
func TestHandleNewField_TreeNode(t *testing.T) {
	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	registry := newMockRegistry()

	handleNewField("0", newNode, "0", false, registry, changes)

	// Should set the full TreeNode (client doesn't have structure yet)
	if changes.Dynamics["0"] == nil {
		t.Error("Expected field '0' to be set")
	}

	// Registry should track that structure was sent
	if !registry.HasSeen("0", newNode) {
		t.Error("Expected registry to mark structure as seen")
	}
}

// TestHandleNewField_InsideNewStructure tests handling fields inside new structures.
func TestHandleNewField_InsideNewStructure(t *testing.T) {
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleNewField("0", "value", "0", true, nil, changes)

	if changes.Dynamics["0"] != "value" {
		t.Errorf("Expected '0' = 'value' inside new structure, got: %v", changes.Dynamics["0"])
	}
}

// TestHandleNewTreeNodeField tests TreeNode field handling.
func TestHandleNewTreeNodeField(t *testing.T) {
	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	registry := newMockRegistry()

	// Test when client doesn't have structure
	handled := handleNewTreeNodeField("0", newNode, false, "0", registry, true, changes)

	if !handled {
		t.Error("Expected handleNewTreeNodeField to return true for TreeNode")
	}

	if changes.Dynamics["0"] == nil {
		t.Error("Expected field to be set")
	}

	// Test when client already has structure
	registry.MarkSeen("1", newNode)
	changes = &TreeNode{Dynamics: make(map[string]interface{})}
	handleNewTreeNodeField("1", newNode, true, "1", registry, true, changes)

	// Should strip statics when client has structure
	result := changes.Dynamics["1"]
	if result == nil {
		t.Error("Expected field '1' to be set even when structure known")
	}
}

// TestHandleNewMapField tests map field handling.
func TestHandleNewMapField(t *testing.T) {
	newMap := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "content",
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	registry := newMockRegistry()

	handled := handleNewMapField("0", newMap, false, "0", registry, true, changes)

	if !handled {
		t.Error("Expected handleNewMapField to return true for map")
	}

	if changes.Dynamics["0"] == nil {
		t.Error("Expected field to be set")
	}
}

// TestExtractTreeNodePair tests TreeNode pair extraction.
func TestExtractTreeNodePair(t *testing.T) {
	oldNode := &TreeNode{Dynamics: map[string]interface{}{"0": "old"}}
	newNode := &TreeNode{Dynamics: map[string]interface{}{"0": "new"}}

	// Both TreeNodes
	oldPtr, newPtr, both := extractTreeNodePair(oldNode, newNode)
	if !both {
		t.Error("Expected both=true for two TreeNodes")
	}
	if oldPtr != oldNode || newPtr != newNode {
		t.Error("Expected pointers to match input TreeNodes")
	}

	// One TreeNode
	oldPtr, newPtr, both = extractTreeNodePair("string", newNode)
	if both {
		t.Error("Expected both=false when old is not TreeNode")
	}
	if oldPtr != nil {
		t.Error("Expected oldPtr=nil when old is not TreeNode")
	}
	if newPtr != newNode {
		t.Error("Expected newPtr to match input TreeNode")
	}

	// No TreeNodes
	oldPtr, newPtr, both = extractTreeNodePair("string", 123)
	if both {
		t.Error("Expected both=false for primitives")
	}
	if oldPtr != nil || newPtr != nil {
		t.Error("Expected nil pointers for primitives")
	}
}

// TestHandleNestedTreeNodes_StructureChanged tests structure change detection.
func TestHandleNestedTreeNodes_StructureChanged(t *testing.T) {
	// Different structures
	oldNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}
	newNode := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleNestedTreeNodes("0", oldNode, newNode, "0", false, nil, nil, changes)

	// Should send full new structure when structure changed
	if changes.Dynamics["0"] == nil {
		t.Error("Expected field '0' to be set for structure change")
	}
}

// TestHandleNestedTreeNodes_Similar tests similar structure handling.
func TestHandleNestedTreeNodes_Similar(t *testing.T) {
	// Similar structures, different content
	oldNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "old"},
	}
	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "new"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleNestedTreeNodes("0", oldNode, newNode, "0", false, nil, nil, changes)

	// Should produce nested changes
	nestedChanges, ok := changes.Dynamics["0"].(*TreeNode)
	if !ok {
		t.Fatalf("Expected nested changes to be *TreeNode, got: %T", changes.Dynamics["0"])
	}

	if nestedChanges.Dynamics["0"] != "new" {
		t.Errorf("Expected nested change, got: %v", nestedChanges.Dynamics["0"])
	}
}

// TestHandleStaticOnlyChanges tests static-only change detection.
func TestHandleStaticOnlyChanges(t *testing.T) {
	// Same dynamics, different statics
	oldNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{},
	}
	newNode := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleStaticOnlyChanges("0", oldNode, newNode, changes)

	// Should set empty string to signal static change
	if val, ok := changes.Dynamics["0"]; !ok || val != "" {
		t.Errorf("Expected field '0' = empty string for static-only change, got: %v", val)
	}
}

// TestHandleNewTreeNodeFromPrimitive tests TreeNode appearing where primitive was.
func TestHandleNewTreeNodeFromPrimitive(t *testing.T) {
	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	registry := newMockRegistry()

	handleNewTreeNodeFromPrimitive("0", newNode, "0", registry, true, changes)

	// Should set full TreeNode
	if changes.Dynamics["0"] == nil {
		t.Error("Expected field '0' to be set")
	}

	// Should mark as seen in registry
	if !registry.HasSeen("0", newNode) {
		t.Error("Expected registry to mark as seen")
	}
}

// TestIsNilRegistry tests nil registry detection.
func TestIsNilRegistry(t *testing.T) {
	// Nil interface
	var nilRegistry StructureRegistry
	if !isNilRegistry(nilRegistry) {
		t.Error("Expected isNilRegistry(nil) = true")
	}

	// Non-nil registry
	registry := newMockRegistry()
	if isNilRegistry(registry) {
		t.Error("Expected isNilRegistry(non-nil) = false")
	}

	// Nil pointer wrapped in interface
	var nilPtr *mockStructureRegistry
	var interfaceWithNilPtr StructureRegistry = nilPtr
	if !isNilRegistry(interfaceWithNilPtr) {
		t.Error("Expected isNilRegistry(interface with nil pointer) = true")
	}
}

// TestHandleChangedField_TypeChange tests when field type changes.
func TestHandleChangedField_TypeChange(t *testing.T) {
	oldTree := &TreeNode{Dynamics: map[string]interface{}{"0": "old"}}
	newTree := &TreeNode{Dynamics: map[string]interface{}{"0": "new"}}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleChangedField("0", "string", 123, "0", false, nil, nil, oldTree, newTree, changes)

	// Should set new value when type changes
	if changes.Dynamics["0"] != 123 {
		t.Errorf("Expected field '0' = 123 for type change, got: %v", changes.Dynamics["0"])
	}
}

// TestHandleChangedField_RangeMatch tests matched range handling.
func TestHandleChangedField_RangeMatch(t *testing.T) {
	item1 := map[string]interface{}{"id": "1", "name": "Item 1"}
	item2 := map[string]interface{}{"id": "2", "name": "Item 2"}

	oldValue := &TreeNode{
		Range: &RangeData{
			Items:   []interface{}{item1},
			Statics: []string{"<li>", "</li>"},
		},
	}
	newValue := &TreeNode{
		Range: &RangeData{
			Items:   []interface{}{item1, item2},
			Statics: []string{"<li>", "</li>"},
		},
	}

	oldTree := &TreeNode{Dynamics: map[string]interface{}{"0": oldValue}}
	newTree := &TreeNode{Dynamics: map[string]interface{}{"0": newValue}}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	rangeMatches := map[string]string{"0": "matched"}

	handleChangedField("0", oldValue, newValue, "0", false, rangeMatches, nil, oldTree, newTree, changes)

	// Should generate differential operations
	if changes.Dynamics["0"] == nil {
		t.Error("Expected field '0' to have range operations")
	}
}

// TestHandleChangedField_TreeNodes tests TreeNode comparison.
func TestHandleChangedField_TreeNodes(t *testing.T) {
	oldNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "old"},
	}
	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "new"},
	}

	oldTree := &TreeNode{Dynamics: map[string]interface{}{"0": oldNode}}
	newTree := &TreeNode{Dynamics: map[string]interface{}{"0": newNode}}
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	handleChangedField("0", oldNode, newNode, "0", false, nil, nil, oldTree, newTree, changes)

	// Should produce nested changes
	nestedChanges, ok := changes.Dynamics["0"].(*TreeNode)
	if !ok {
		t.Fatalf("Expected nested changes, got: %T", changes.Dynamics["0"])
	}

	if nestedChanges.Dynamics["0"] != "new" {
		t.Errorf("Expected nested change, got: %v", nestedChanges.Dynamics["0"])
	}
}

// TestIsStrippedValueEmpty tests the empty value detection helper.
func TestIsStrippedValueEmpty(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		wantEmpty bool
	}{
		{"empty map", map[string]interface{}{}, true},
		{"empty string", "", true},
		{"non-empty map", map[string]interface{}{"key": "value"}, false},
		{"non-empty string", "content", false},
		{"nil", nil, false},
		{"number", 42, false},
		{"map with nil value", map[string]interface{}{"key": nil}, false},
		{
			"empty TreeNode (no statics, no dynamics)",
			&TreeNode{Statics: nil, Dynamics: map[string]interface{}{}},
			true,
		},
		{
			"TreeNode with statics only",
			&TreeNode{Statics: []string{"<div>", "</div>"}, Dynamics: map[string]interface{}{}},
			false,
		},
		{
			"TreeNode with dynamics only",
			&TreeNode{Statics: nil, Dynamics: map[string]interface{}{"0": "value"}},
			false,
		},
		{
			"TreeNode with both statics and dynamics",
			&TreeNode{Statics: []string{"<div>", "</div>"}, Dynamics: map[string]interface{}{"0": "value"}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStrippedValueEmpty(tt.value)
			if got != tt.wantEmpty {
				t.Errorf("isStrippedValueEmpty(%v) = %v, want %v", tt.value, got, tt.wantEmpty)
			}
		})
	}
}

// TestHandleStructureValue tests the core structure value handling logic.
func TestHandleStructureValue(t *testing.T) {
	tests := []struct {
		name                string
		newValue            interface{}
		clientHasStructure bool
		wantShouldTrack     bool
		checkValue          func(t *testing.T, value interface{})
	}{
		{
			name: "client has structure - returns stripped",
			newValue: &TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{"0": "content"},
			},
			clientHasStructure: true,
			wantShouldTrack:     false,
			checkValue: func(t *testing.T, value interface{}) {
				// Stripped value should not include statics
				if valueMap, ok := value.(map[string]interface{}); ok {
					if _, hasStatics := valueMap["s"]; hasStatics {
						t.Error("Expected statics to be stripped")
					}
				}
			},
		},
		{
			name: "client doesn't have structure - returns full",
			newValue: &TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{"0": "content"},
			},
			clientHasStructure: false,
			wantShouldTrack:     true,
			checkValue: func(t *testing.T, value interface{}) {
				// Should return original TreeNode
				if _, ok := value.(*TreeNode); !ok {
					t.Errorf("Expected *TreeNode, got %T", value)
				}
			},
		},
		{
			name: "static-only structure (no dynamics) - returns empty string",
			newValue: &TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]interface{}{},
			},
			clientHasStructure: true, // Client has structure, so we strip
			wantShouldTrack:     false,
			checkValue: func(t *testing.T, value interface{}) {
				// With the fix to isStrippedValueEmpty, empty TreeNodes are now recognized
				// So we should get an empty string
				if value != "" {
					t.Errorf("Expected empty string for static-only structure, got %T: %v", value, value)
				}
			},
		},
		{
			name:                "nil value returns empty string",
			newValue:            nil,
			clientHasStructure:  false,
			wantShouldTrack:     false,
			checkValue: func(t *testing.T, value interface{}) {
				if value != "" {
					t.Errorf("Expected empty string for nil value, got %v", value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, shouldTrack := handleStructureValue(tt.newValue, tt.clientHasStructure)
			if shouldTrack != tt.wantShouldTrack {
				t.Errorf("shouldTrack = %v, want %v", shouldTrack, tt.wantShouldTrack)
			}
			if tt.checkValue != nil {
				tt.checkValue(t, value)
			}
		})
	}
}

// TestHandleNewField_WithRegistry tests registry interaction.
func TestHandleNewField_WithRegistry(t *testing.T) {
	registry := newMockRegistry()
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}

	// First time: registry doesn't have structure
	handleNewField("0", newNode, "0", false, registry, changes)

	if !registry.HasSeen("0", newNode) {
		t.Error("Expected registry to track new structure")
	}

	// Second time: registry already has structure
	changes = &TreeNode{Dynamics: make(map[string]interface{})}
	handleNewField("1", newNode, "1", false, registry, changes)

	// Should use stripped value since not first time
	if changes.Dynamics["1"] == nil {
		t.Error("Expected field to be set")
	}
}

// TestHandleNewField_NilRegistryInInterface tests the nil-in-interface edge case.
func TestHandleNewField_NilRegistryInInterface(t *testing.T) {
	// Create nil pointer wrapped in interface
	var nilPtr *mockStructureRegistry
	var registry StructureRegistry = nilPtr

	changes := &TreeNode{Dynamics: make(map[string]interface{})}
	newNode := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "content"},
	}

	// Should handle gracefully without panic
	handleNewField("0", newNode, "0", false, registry, changes)

	if changes.Dynamics["0"] == nil {
		t.Error("Expected field to be set even with nil registry")
	}
}

// TestCompareTreesAndGetChangesWithPath_DeepNesting tests deeply nested structures.
func TestCompareTreesAndGetChangesWithPath_DeepNesting(t *testing.T) {
	// Create a deeply nested structure (5 levels)
	var createNested func(depth int, value string) *TreeNode
	createNested = func(depth int, value string) *TreeNode {
		if depth == 0 {
			return &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": value},
			}
		}
		return &TreeNode{
			Statics:  []string{"<div>", "</div>"},
			Dynamics: map[string]interface{}{"0": createNested(depth-1, value)},
		}
	}

	oldTree := createNested(5, "old")
	newTree := createNested(5, "new")

	changes := CompareTreesAndGetChangesWithPath(oldTree, newTree, false, "", nil, nil)

	if !changes.HasDynamics() {
		t.Fatal("Expected changes for deeply nested structure")
	}

	// Verify change propagated to deepest level
	current := changes
	for i := 0; i < 5; i++ {
		if !current.HasDynamics() {
			t.Fatalf("Expected dynamics at level %d", i)
		}
		if i < 4 {
			next, ok := current.Dynamics["0"].(*TreeNode)
			if !ok {
				t.Fatalf("Expected TreeNode at level %d, got %T", i, current.Dynamics["0"])
			}
			current = next
		}
	}
}

// TestHandleEmptyRangeDiff tests empty range handling.
func TestHandleEmptyRangeDiff(t *testing.T) {
	changes := &TreeNode{Dynamics: make(map[string]interface{})}

	// Both empty ranges - this needs Statics to qualify as range constructs
	oldRange := &TreeNode{
		Range:   &RangeData{Items: []interface{}{}, Statics: []string{"<li>", "</li>"}},
		Statics: []string{"<li>", "</li>"},
	}
	newRange := &TreeNode{
		Range:   &RangeData{Items: []interface{}{}, Statics: []string{"<li>", "</li>"}},
		Statics: []string{"<li>", "</li>"},
	}

	handleEmptyRangeDiff("0", oldRange, newRange, changes)

	// Should not set anything (both empty)
	if changes.HasDynamics() {
		t.Error("Expected no changes for both empty ranges")
	}

	// Transition from items to empty
	changes = &TreeNode{Dynamics: make(map[string]interface{})}
	oldRange = &TreeNode{
		Range: &RangeData{
			Items:   []interface{}{map[string]interface{}{"id": "1"}},
			Statics: []string{"<li>", "</li>"},
		},
		Statics: []string{"<li>", "</li>"},
	}
	newRange = &TreeNode{
		Range:   &RangeData{Items: []interface{}{}, Statics: []string{"<li>", "</li>"}},
		Statics: []string{"<li>", "</li>"},
	}

	handleEmptyRangeDiff("0", oldRange, newRange, changes)

	// Should send the empty range structure
	if changes.Dynamics["0"] == nil {
		t.Error("Expected empty range structure to be sent")
	}
}
