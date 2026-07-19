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

	if node.Dynamics != nil {
		t.Error("Dynamics slice should be nil (lazy-initialized)")
	}

	if len(node.Statics) != 0 {
		t.Error("Statics should be empty")
	}

	// Verify SetDynamic lazy-inits the slice
	node.SetDynamic(0, "value")
	if node.Dynamics == nil {
		t.Error("Dynamics slice should be initialized after SetDynamic")
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

	if node.Dynamics != nil {
		t.Error("Dynamics slice should be nil (lazy-initialized)")
	}

	// Verify SetDynamic lazy-inits the slice
	node.SetDynamic(0, "value")
	if node.Dynamics == nil {
		t.Error("Dynamics slice should be initialized after SetDynamic")
	}
}

// TestTreeNode_SetDynamic tests setting dynamic values.
func TestTreeNode_SetDynamic(t *testing.T) {
	node := NewTreeNode()
	node.SetDynamic(0, "value1")
	node.SetDynamic(1, "value2")

	if node.Dynamics[0] != "value1" {
		t.Errorf("Dynamics[0] = %v, want 'value1'", node.Dynamics[0])
	}

	if node.Dynamics[1] != "value2" {
		t.Errorf("Dynamics[1] = %v, want 'value2'", node.Dynamics[1])
	}
}

// TestTreeNode_SetDynamic_TypeGuard tests that incompatible types are converted to strings.
func TestTreeNode_SetDynamic_TypeGuard(t *testing.T) {
	// Define a custom struct (like EditingPosts) that should be converted to string
	type EditingPosts struct {
		Title   string
		Content string
	}

	node := NewTreeNode()

	// Test: raw struct should be converted to string
	post := EditingPosts{Title: "Hello", Content: "World"}
	node.SetDynamic(0, post)
	if _, ok := node.Dynamics[0].(string); !ok {
		t.Errorf("Raw struct should be converted to string, got type %T", node.Dynamics[0])
	}

	// Test: pointer to struct should also be converted to string
	node.SetDynamic(1, &post)
	if _, ok := node.Dynamics[1].(string); !ok {
		t.Errorf("Pointer to struct should be converted to string, got type %T", node.Dynamics[1])
	}

	// Test: TreeNode pointer should be preserved (allowed type)
	childTree := NewTreeNode()
	childTree.SetDynamic(0, "value")
	node.SetDynamic(2, childTree)
	if _, ok := node.Dynamics[2].(*TreeNode); !ok {
		t.Errorf("TreeNode pointer should be preserved, got type %T", node.Dynamics[2])
	}

	// Test: primitive types should be preserved
	node.SetDynamic(3, "string value")
	node.SetDynamic(4, 42)
	node.SetDynamic(5, true)
	node.SetDynamic(6, 3.14)

	if node.Dynamics[3] != "string value" {
		t.Errorf("String should be preserved, got %v", node.Dynamics[3])
	}
	if node.Dynamics[4] != 42 {
		t.Errorf("Int should be preserved, got %v", node.Dynamics[4])
	}
	if node.Dynamics[5] != true {
		t.Errorf("Bool should be preserved, got %v", node.Dynamics[5])
	}
	if node.Dynamics[6] != 3.14 {
		t.Errorf("Float should be preserved, got %v", node.Dynamics[6])
	}
}

// TestIsTreeCompatible tests the isTreeCompatible function directly.
func TestIsTreeCompatible(t *testing.T) {
	type CustomStruct struct {
		Field string
	}

	tests := []struct {
		name       string
		value      interface{}
		compatible bool
	}{
		{"nil", nil, true},
		{"string", "hello", true},
		{"int", 42, true},
		{"int64", int64(42), true},
		{"float64", 3.14, true},
		{"bool", true, true},
		{"*TreeNode", NewTreeNode(), true},
		{"*RangeData", &RangeData{}, true},
		{"map[string]interface{}", map[string]interface{}{"key": "value"}, true},
		{"[]interface{}", []interface{}{"a", "b"}, true},
		// Typed slices should be compatible (valid JSON arrays)
		{"[]string", []string{"a", "b", "c"}, true},
		{"[]int", []int{1, 2, 3}, true},
		{"[]CustomStruct", []CustomStruct{{Field: "a"}, {Field: "b"}}, true},
		// Arrays should be compatible (valid JSON arrays)
		{"[3]int", [3]int{1, 2, 3}, true},
		{"[2]string", [2]string{"a", "b"}, true},
		// Typed maps should be compatible (valid JSON objects)
		{"map[string]string", map[string]string{"key": "value"}, true},
		{"map[string]int", map[string]int{"count": 42}, true},
		// Raw structs should NOT be compatible
		{"custom struct", CustomStruct{Field: "test"}, false},
		{"*custom struct", &CustomStruct{Field: "test"}, false},
		// Channels and functions should NOT be compatible
		{"chan int", make(chan int), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTreeCompatible(tt.value)
			if result != tt.compatible {
				t.Errorf("isTreeCompatible(%s) = %v, want %v", tt.name, result, tt.compatible)
			}
		})
	}
}

// TestTreeNode_GetDynamic tests getting dynamic values.
func TestTreeNode_GetDynamic(t *testing.T) {
	node := NewTreeNode()
	node.SetDynamic(0, "test-value")

	val, ok := node.GetDynamic(0)
	if !ok {
		t.Error("GetDynamic(0) should return ok=true")
	}
	if val != "test-value" {
		t.Errorf("GetDynamic(0) = %v, want 'test-value'", val)
	}

	_, ok = node.GetDynamic(999)
	if ok {
		t.Error("GetDynamic(999) should return ok=false for non-existent index")
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

	node.SetDynamic(0, "value")
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
		Dynamics:    []interface{}{"Hello"},
		Fingerprint: "abc123",
	}
	node.dynamicCount = 1

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

	if node.Dynamics[0] != "Hello" {
		t.Errorf("Dynamics[0] = %v, want 'Hello'", node.Dynamics[0])
	}

	if node.Fingerprint != "abc123" {
		t.Errorf("Fingerprint = %v, want 'abc123'", node.Fingerprint)
	}
}

// TestTreeNode_ToMap tests conversion to map.
func TestTreeNode_ToMap(t *testing.T) {
	node := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"value"},
	}
	node.dynamicCount = 1

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

	if node.Dynamics[0] != "value" {
		t.Errorf("Dynamics[0] = %v, want 'value'", node.Dynamics[0])
	}
}

// TestTreeNode_Clone tests deep cloning.
func TestTreeNode_Clone(t *testing.T) {
	original := &TreeNode{
		Statics:     []string{"<div>", "</div>"},
		Dynamics:    []interface{}{"value"},
		Fingerprint: "fp123",
	}
	original.dynamicCount = 1

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
		Dynamics: []interface{}{"nested-value"},
	}
	nested.dynamicCount = 1

	original := &TreeNode{
		Dynamics: []interface{}{nested},
	}
	original.dynamicCount = 1

	clone := original.Clone()

	clonedNested, ok := clone.Dynamics[0].(*TreeNode)
	if !ok {
		t.Fatal("Cloned nested should be *TreeNode")
	}

	// Modify nested clone
	clonedNested.SetDynamic(0, "modified")

	// Original nested should be unchanged
	if nested.Dynamics[0] == "modified" {
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

// Items==nil must stay nil through clone — stream-mode detection downstream
// relies on the nil-ness.
func TestTreeNode_Clone_PreservesNilItems(t *testing.T) {
	original := &TreeNode{
		Range: &RangeData{
			Items:   nil,
			Statics: []string{"<li>", "</li>"},
		},
	}

	clone := original.Clone()

	if clone.Range == nil {
		t.Fatal("Clone Range should not be nil")
	}
	if clone.Range.Items != nil {
		t.Errorf("Clone Range.Items should be nil, got %v", clone.Range.Items)
	}
	if !reflect.DeepEqual(clone.Range.Statics, original.Range.Statics) {
		t.Errorf("Clone Range.Statics = %v, want %v", clone.Range.Statics, original.Range.Statics)
	}
}

func TestRangeStreamState_DeepClone(t *testing.T) {
	original := &TreeNode{
		Range: &RangeData{
			StreamState: &RangeStreamState{
				Keys:        []string{"k1", "k2"},
				Hashes:      []uint64{0xdeadbeef, 0xcafebabe},
				Fingerprint: "fp123",
			},
		},
	}

	clone := original.Clone()

	if clone.Range == nil || clone.Range.StreamState == nil {
		t.Fatal("Clone Range.StreamState should not be nil")
	}
	if !reflect.DeepEqual(clone.Range.StreamState.Keys, original.Range.StreamState.Keys) {
		t.Errorf("Clone Keys = %v, want %v", clone.Range.StreamState.Keys, original.Range.StreamState.Keys)
	}
	if !reflect.DeepEqual(clone.Range.StreamState.Hashes, original.Range.StreamState.Hashes) {
		t.Errorf("Clone Hashes = %v, want %v", clone.Range.StreamState.Hashes, original.Range.StreamState.Hashes)
	}
	if clone.Range.StreamState.Fingerprint != original.Range.StreamState.Fingerprint {
		t.Errorf("Clone Fingerprint = %v, want %v", clone.Range.StreamState.Fingerprint, original.Range.StreamState.Fingerprint)
	}

	// Mutate clone, confirm original unchanged
	clone.Range.StreamState.Keys[0] = "mutated"
	if original.Range.StreamState.Keys[0] == "mutated" {
		t.Error("Mutating clone Keys should not affect original")
	}
	clone.Range.StreamState.Hashes[0] = 0xff
	if original.Range.StreamState.Hashes[0] == 0xff {
		t.Error("Mutating clone Hashes should not affect original")
	}
}

// Items==nil && StreamState!=nil emits {} not {"d": null}.
func TestTreeNode_MarshalJSON_OmitsD_StreamMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{
			Items: nil,
			StreamState: &RangeStreamState{
				Keys:        []string{"k1"},
				Hashes:      []uint64{0x1},
				Fingerprint: "fp",
			},
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, hasD := result["d"]; hasD {
		t.Errorf("Stream-mode tree should NOT have 'd' key, got %v", result)
	}
}

func TestTreeNode_MarshalJSON_EmitsD_LegacyMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{
			Items: []interface{}{"item1"},
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, hasD := result["d"]; !hasD {
		t.Errorf("Legacy-mode tree should have 'd' key, got %v", result)
	}
}

// TestTreeNode_ToMap_OmitsD_StreamMode mirrors MarshalJSON_OmitsD_StreamMode for ToMap.
func TestTreeNode_ToMap_OmitsD_StreamMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{
			Items: nil,
			StreamState: &RangeStreamState{
				Keys: []string{"k1"},
			},
		},
	}

	m := node.ToMap()

	if _, hasD := m["d"]; hasD {
		t.Errorf("Stream-mode tree's ToMap() should NOT have 'd' key, got %v", m)
	}
}

// TestTreeNode_ToMap_EmitsD_LegacyMode mirrors MarshalJSON_EmitsD_LegacyMode for ToMap.
func TestTreeNode_ToMap_EmitsD_LegacyMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{
			Items: []interface{}{"item1"},
		},
	}

	m := node.ToMap()

	if _, hasD := m["d"]; !hasD {
		t.Errorf("Legacy-mode tree's ToMap() should have 'd' key, got %v", m)
	}
}

// TestTreeNode_MarshalJSON_PreservesStatics_StreamMode guards against tying
// "s" emission to "d" emission: stream-mode trees omit "d" but must still
// emit "s" so the client can render items from cached statics.
func TestTreeNode_MarshalJSON_PreservesStatics_StreamMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{
			Items:   nil,
			Statics: []string{"<li>", "</li>"},
			StreamState: &RangeStreamState{
				Keys: []string{"k1"},
			},
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, hasD := result["d"]; hasD {
		t.Errorf("Stream-mode tree should NOT have 'd' key, got %v", result)
	}
	if _, hasS := result["s"]; !hasS {
		t.Errorf("Stream-mode tree SHOULD have 's' key for cached client statics, got %v", result)
	}
}

// Boundary case: Items==nil && StreamState==nil (legacy-empty) — the omit
// guard must NOT fire here; pre-Phase-1 wire output emitted "d": null and
// that behaviour is preserved.
func TestTreeNode_MarshalJSON_EmitsNullD_LegacyEmptyMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{Items: nil},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	v, hasD := result["d"]
	if !hasD {
		t.Errorf("Legacy-empty tree should still have 'd' key (preserves prior behaviour), got %v", result)
	}
	if v != nil {
		t.Errorf("Legacy-empty 'd' should be null, got %v (%T)", v, v)
	}
}

// IsStreamMode covers the 2x2 grid of (Items present/nil) x (StreamState
// present/nil); only Items==nil && StreamState!=nil counts as stream mode.
func TestRangeData_IsStreamMode(t *testing.T) {
	tests := []struct {
		name string
		rd   *RangeData
		want bool
	}{
		{"nil receiver", nil, false},
		{"legacy non-empty", &RangeData{Items: []interface{}{"x"}}, false},
		{"legacy empty", &RangeData{Items: nil}, false},
		{"stream mode", &RangeData{Items: nil, StreamState: &RangeStreamState{Keys: []string{"k"}}}, true},
		{"transitional both set (illegal but defensive)", &RangeData{Items: []interface{}{"x"}, StreamState: &RangeStreamState{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rd.IsStreamMode(); got != tt.want {
				t.Errorf("IsStreamMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ToMap parallel of EmitsNullD_LegacyEmptyMode. ToMap pre-Phase-1 emits
// "d": [] (not "d": null) because of the convertedItems := make(...) deep-
// convert path — that pre-existing shape is preserved by the omit guard.
func TestTreeNode_ToMap_EmitsEmptyD_LegacyEmptyMode(t *testing.T) {
	node := &TreeNode{
		Range: &RangeData{Items: nil},
	}

	m := node.ToMap()

	v, hasD := m["d"]
	if !hasD {
		t.Errorf("Legacy-empty ToMap should still have 'd' key, got %v", m)
	}
	items, ok := v.([]interface{})
	if !ok {
		t.Fatalf("Legacy-empty ToMap 'd' should be []interface{}, got %T (%v)", v, v)
	}
	if len(items) != 0 {
		t.Errorf("Legacy-empty ToMap 'd' should be empty slice, got len=%d", len(items))
	}
}

// TestTreeNode_Clone_CarriesWrapper guards the field against Clone's
// named-field construction.
//
// Clone builds the copy from an explicit field list, so anything added to
// TreeNode is dropped unless someone remembers to list it. A cloned wrapper
// silently reverting to WrapperNone is the exact failure the kind exists to
// prevent: it becomes indistinguishable from an ordinary node again, and
// through-wrapper range keying falls back to content hashing.
//
// No caller in internal/parse clones a tagged node before wrappedItemKey reads
// it, so this is latent rather than live — but Clone is exported and documented
// as a deep copy, so "deep copy" should mean it.
func TestTreeNode_Clone_CarriesWrapper(t *testing.T) {
	for _, kind := range []WrapperKind{WrapperNone, WrapperConditional, WrapperInvocation} {
		original := NewTreeNode()
		original.Statics = []string{"", ""}
		original.Wrapper = kind
		original.SetDynamic(0, "x")

		if got := original.Clone().Wrapper; got != kind {
			t.Errorf("Clone dropped the wrapper kind: got %v, want %v", got, kind)
		}
	}
}
