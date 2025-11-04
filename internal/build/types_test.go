package build

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestNewTreeNode tests TreeNode constructor.
func TestNewTreeNode(t *testing.T) {
	node := NewTreeNode()

	if node == nil {
		t.Fatal("NewTreeNode() should not return nil")
	}

	if node.Dynamics == nil {
		t.Error("Dynamics map should be initialized")
	}

	if len(node.Statics) != 0 {
		t.Error("Statics should be empty")
	}
}

// TestNewTreeNodeWithStatics tests TreeNode constructor with statics.
func TestNewTreeNodeWithStatics(t *testing.T) {
	statics := []string{"<div>", "</div>"}
	node := NewTreeNodeWithStatics(statics)

	if node == nil {
		t.Fatal("NewTreeNodeWithStatics() should not return nil")
	}

	if !reflect.DeepEqual(node.Statics, statics) {
		t.Errorf("Statics = %v, want %v", node.Statics, statics)
	}

	if node.Dynamics == nil {
		t.Error("Dynamics map should be initialized")
	}
}

// TestTreeNode_SetDynamic tests setting dynamic values.
func TestTreeNode_SetDynamic(t *testing.T) {
	node := NewTreeNode()
	node.SetDynamic("0", "value1")
	node.SetDynamic("1", "value2")

	if node.Dynamics["0"] != "value1" {
		t.Errorf("Dynamics['0'] = %v, want 'value1'", node.Dynamics["0"])
	}

	if node.Dynamics["1"] != "value2" {
		t.Errorf("Dynamics['1'] = %v, want 'value2'", node.Dynamics["1"])
	}
}

// TestTreeNode_GetDynamic tests getting dynamic values.
func TestTreeNode_GetDynamic(t *testing.T) {
	node := NewTreeNode()
	node.SetDynamic("0", "test-value")

	val, ok := node.GetDynamic("0")
	if !ok {
		t.Error("GetDynamic('0') should return ok=true")
	}
	if val != "test-value" {
		t.Errorf("GetDynamic('0') = %v, want 'test-value'", val)
	}

	_, ok = node.GetDynamic("999")
	if ok {
		t.Error("GetDynamic('999') should return ok=false for non-existent key")
	}
}

// TestTreeNode_HasStatics tests static detection.
func TestTreeNode_HasStatics(t *testing.T) {
	node := NewTreeNode()
	if node.HasStatics() {
		t.Error("Empty node should not have statics")
	}

	node.Statics = []string{"<div>", "</div>"}
	if !node.HasStatics() {
		t.Error("Node with statics should return true")
	}
}

// TestTreeNode_HasDynamics tests dynamic detection.
func TestTreeNode_HasDynamics(t *testing.T) {
	node := NewTreeNode()
	if node.HasDynamics() {
		t.Error("Empty node should not have dynamics")
	}

	node.SetDynamic("0", "value")
	if !node.HasDynamics() {
		t.Error("Node with dynamics should return true")
	}
}

// TestTreeNode_HasRange tests range detection.
func TestTreeNode_HasRange(t *testing.T) {
	node := NewTreeNode()
	if node.HasRange() {
		t.Error("Empty node should not have range")
	}

	node.Range = &RangeData{Items: []interface{}{}}
	if !node.HasRange() {
		t.Error("Node with range should return true")
	}
}

// TestTreeNode_MarshalJSON tests JSON marshaling.
func TestTreeNode_MarshalJSON(t *testing.T) {
	node := &TreeNode{
		Statics:     []string{"<div>", "</div>"},
		Dynamics:    map[string]interface{}{"0": "Hello"},
		Fingerprint: "abc123",
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Check expected keys
	if _, hasS := result["s"]; !hasS {
		t.Error("Marshaled JSON should have 's' key")
	}
	if _, has0 := result["0"]; !has0 {
		t.Error("Marshaled JSON should have '0' key")
	}
	if _, hasF := result["f"]; !hasF {
		t.Error("Marshaled JSON should have 'f' key")
	}
}

// TestTreeNode_UnmarshalJSON tests JSON unmarshaling.
func TestTreeNode_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"s": ["<div>", "</div>"],
		"0": "Hello",
		"f": "abc123"
	}`

	var node TreeNode
	if err := json.Unmarshal([]byte(jsonData), &node); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	expectedStatics := []string{"<div>", "</div>"}
	if !reflect.DeepEqual(node.Statics, expectedStatics) {
		t.Errorf("Statics = %v, want %v", node.Statics, expectedStatics)
	}

	if node.Dynamics["0"] != "Hello" {
		t.Errorf("Dynamics['0'] = %v, want 'Hello'", node.Dynamics["0"])
	}

	if node.Fingerprint != "abc123" {
		t.Errorf("Fingerprint = %v, want 'abc123'", node.Fingerprint)
	}
}

// TestTreeNode_ToMap tests conversion to map.
func TestTreeNode_ToMap(t *testing.T) {
	node := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	m := node.ToMap()

	if m["s"] == nil {
		t.Error("Map should have 's' key")
	}
	if m["0"] != "value" {
		t.Errorf("Map['0'] = %v, want 'value'", m["0"])
	}
}

// TestTreeNode_FromMap tests creation from map.
func TestTreeNode_FromMap(t *testing.T) {
	m := map[string]interface{}{
		"s": []interface{}{"<div>", "</div>"},
		"0": "value",
	}

	node, err := FromMap(m)
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	if len(node.Statics) != 2 {
		t.Errorf("Statics length = %d, want 2", len(node.Statics))
	}

	if node.Dynamics["0"] != "value" {
		t.Errorf("Dynamics['0'] = %v, want 'value'", node.Dynamics["0"])
	}
}

// TestTreeNode_Clone tests deep cloning.
func TestTreeNode_Clone(t *testing.T) {
	original := &TreeNode{
		Statics:     []string{"<div>", "</div>"},
		Dynamics:    map[string]interface{}{"0": "value"},
		Fingerprint: "fp123",
	}

	clone := original.Clone()

	// Verify deep copy
	if !reflect.DeepEqual(clone.Statics, original.Statics) {
		t.Error("Clone statics should match original")
	}

	// Modify clone and ensure original unchanged
	clone.Statics[0] = "<span>"
	if original.Statics[0] == "<span>" {
		t.Error("Modifying clone should not affect original")
	}
}

// TestTreeNode_NestedClone tests cloning with nested TreeNodes.
func TestTreeNode_NestedClone(t *testing.T) {
	nested := &TreeNode{
		Dynamics: map[string]interface{}{"0": "nested-value"},
	}

	original := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": nested,
		},
	}

	clone := original.Clone()

	clonedNested, ok := clone.Dynamics["0"].(*TreeNode)
	if !ok {
		t.Fatal("Cloned nested should be *TreeNode")
	}

	// Modify nested clone
	clonedNested.SetDynamic("0", "modified")

	// Original nested should be unchanged
	if nested.Dynamics["0"] == "modified" {
		t.Error("Modifying cloned nested should not affect original")
	}
}

// TestRangeData_Creation tests RangeData constructor.
func TestRangeData_Creation(t *testing.T) {
	items := []interface{}{"item1", "item2"}
	statics := []string{"<li>", "</li>"}

	rd := NewRangeData(items, statics)

	if rd == nil {
		t.Fatal("NewRangeData should not return nil")
	}

	if len(rd.Items) != 2 {
		t.Errorf("Items length = %d, want 2", len(rd.Items))
	}

	if !reflect.DeepEqual(rd.Statics, statics) {
		t.Errorf("Statics = %v, want %v", rd.Statics, statics)
	}
}

// TestTreeMetadata_Creation tests TreeMetadata constructor.
func TestTreeMetadata_Creation(t *testing.T) {
	meta := NewTreeMetadata("id")

	if meta == nil {
		t.Fatal("NewTreeMetadata should not return nil")
	}

	if meta.IDKey != "id" {
		t.Errorf("IDKey = %v, want 'id'", meta.IDKey)
	}
}

// TestContext_NewContext tests default context creation.
func TestContext_NewContext(t *testing.T) {
	ctx := NewContext()

	if ctx == nil {
		t.Fatal("NewContext should not return nil")
	}

	if !ctx.IsFirstRender {
		t.Error("NewContext should set IsFirstRender=true")
	}

	if !ctx.IncludeStatics {
		t.Error("NewContext should set IncludeStatics=true")
	}

	if ctx.ClientStructures == nil {
		t.Error("ClientStructures should be initialized")
	}
}

// TestContext_NewUpdateContext tests update context creation.
func TestContext_NewUpdateContext(t *testing.T) {
	clientStructures := map[string]bool{"0": true}
	ctx := NewUpdateContext(clientStructures)

	if ctx == nil {
		t.Fatal("NewUpdateContext should not return nil")
	}

	if ctx.IsFirstRender {
		t.Error("NewUpdateContext should set IsFirstRender=false")
	}

	if ctx.IncludeStatics {
		t.Error("NewUpdateContext should set IncludeStatics=false")
	}

	if !reflect.DeepEqual(ctx.ClientStructures, clientStructures) {
		t.Error("ClientStructures should match provided map")
	}
}

// TestContext_ShouldIncludeStatics tests static inclusion logic.
func TestContext_ShouldIncludeStatics(t *testing.T) {
	tests := []struct {
		name string
		ctx  *Context
		want bool
	}{
		{
			name: "first render",
			ctx:  NewContext(),
			want: true,
		},
		{
			name: "update with no path",
			ctx:  NewUpdateContext(nil),
			want: false,
		},
		{
			name: "update with new path",
			ctx:  NewUpdateContext(map[string]bool{}),
			want: false,
		},
		{
			name: "nil context (backward compatibility)",
			ctx:  nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.ShouldIncludeStatics()
			if got != tt.want {
				t.Errorf("ShouldIncludeStatics() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContext_WithPath tests path tracking.
func TestContext_WithPath(t *testing.T) {
	ctx := NewContext()
	ctx2 := ctx.WithPath("0")

	if ctx2.CurrentPath != "0" {
		t.Errorf("WithPath('0') set CurrentPath = %v, want '0'", ctx2.CurrentPath)
	}

	// Nested path
	ctx3 := ctx2.WithPath("1")
	if ctx3.CurrentPath != "0.1" {
		t.Errorf("Nested path = %v, want '0.1'", ctx3.CurrentPath)
	}

	// Original unchanged
	if ctx.CurrentPath != "" {
		t.Error("Original context should be unchanged")
	}
}
