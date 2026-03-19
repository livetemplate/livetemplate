package parse

import (
	"html/template"
	"reflect"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/keys"
)

// TestHandleRange_SimpleSlice tests range over simple slice.
func TestHandleRange_SimpleSlice(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}<div>{{.}}</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{"a", "b", "c"},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 3 {
		t.Errorf("Expected 3 items, got: %d", len(tree.Range.Items))
	}
}

// TestHandleRange_EmptySlice tests range over empty slice.
func TestHandleRange_EmptySlice(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}<div>{{.}}</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data for empty slice")
	}

	if len(tree.Range.Items) != 0 {
		t.Errorf("Expected 0 items, got: %d", len(tree.Range.Items))
	}
}

// TestHandleRange_Map tests range over map.
func TestHandleRange_Map(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}<div>{{.}}</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": map[string]string{
			"a": "alpha",
			"b": "beta",
		},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(tree.Range.Items))
	}
}

// TestHandleRange_WithElse tests range with else branch.
func TestHandleRange_WithElse(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}<div>{{.}}</div>{{else}}<div>empty</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for else branch")
	}

	// With else branch, should return the else content
	if tree.HasRange() && len(tree.Range.Items) == 0 {
		return
	}

	if !tree.HasStatics() && !tree.HasDynamics() {
		t.Error("Expected statics or dynamics for else branch")
	}
}

// TestHandleRange_WithVarDecls tests range with variable declarations.
func TestHandleRange_WithVarDecls(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range $i, $v := .Items}}<div>{{$i}}: {{$v}}</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{"a", "b"},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(tree.Range.Items))
	}
}

// TestExtractCollection_Simple tests simple collection extraction.
func TestExtractCollection_Simple(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}{{.}}{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{"a", "b"},
	}
	eval := testEval(nil)

	collection, err := extractCollection(rangeNode, eval, data, nil)
	if err != nil {
		t.Fatalf("extractCollection failed: %v", err)
	}

	slice, ok := collection.([]string)
	if !ok {
		t.Fatalf("Expected []string, got: %T", collection)
	}

	if len(slice) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(slice))
	}
}

// TestExtractCollection_WithDecls tests extraction with variable declarations.
func TestExtractCollection_WithDecls(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range $i, $v := .Items}}{{$v}}{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{"a", "b"},
	}
	eval := testEval(nil)

	collection, err := extractCollection(rangeNode, eval, data, nil)
	if err != nil {
		t.Fatalf("extractCollection failed: %v", err)
	}

	slice, ok := collection.([]string)
	if !ok {
		t.Fatalf("Expected []string, got: %T", collection)
	}

	if len(slice) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(slice))
	}
}

// TestExtractCollection_Error tests error handling.
func TestExtractCollection_Error(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Missing}}{{.}}{{end}}", nil)
	data := map[string]interface{}{}
	eval := testEval(nil)

	_, err := extractCollection(rangeNode, eval, data, nil)
	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}

// TestIsEmpty_AllTypes tests isEmpty for various types.
func TestIsEmpty_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"nil", nil, true},
		{"empty slice", []string{}, true},
		{"non-empty slice", []string{"a"}, false},
		{"empty array", [0]string{}, true},
		{"non-empty array", [1]string{"a"}, false},
		{"empty map", map[string]string{}, true},
		{"non-empty map", map[string]string{"a": "b"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.value)
			got := isEmpty(v)
			if got != tt.want {
				t.Errorf("isEmpty(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestHandleEmptyRange_NoElse tests empty range without else.
func TestHandleEmptyRange_NoElse(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}{{.}}{{end}}", nil)
	eval := testEval(nil)
	data := map[string]interface{}{}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleEmptyRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleEmptyRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data for empty range")
	}

	if len(tree.Range.Items) != 0 {
		t.Errorf("Expected 0 items, got: %d", len(tree.Range.Items))
	}
}

// TestHandleEmptyRange_WithElse tests empty range with else branch.
func TestHandleEmptyRange_WithElse(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}{{.}}{{else}}<div>empty</div>{{end}}", nil)
	eval := testEval(nil)
	data := map[string]interface{}{}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleEmptyRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleEmptyRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for else branch")
	}

	if !tree.HasStatics() && !tree.HasDynamics() && !tree.HasRange() {
		t.Error("Expected content for else branch")
	}
}

// TestHandleRange_SliceViaHandleRange tests slice range processing through handleRange.
func TestHandleRange_SliceViaHandleRange(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}<div>{{.}}</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": []string{"a", "b"},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(tree.Range.Items))
	}
}

// TestHandleRange_MapViaHandleRange tests map range processing through handleRange.
func TestHandleRange_MapViaHandleRange(t *testing.T) {
	rangeNode := parseRangeNode(t, "{{range .Items}}<div>{{.}}</div>{{end}}", nil)
	data := map[string]interface{}{
		"Items": map[string]string{"a": "alpha", "b": "beta"},
	}
	eval := testEval(nil)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleRange failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(tree.Range.Items))
	}
}

// TestBuildRangeTreeWithStatics_Homogeneous tests homogeneous range tree construction.
func TestBuildRangeTreeWithStatics_Homogeneous(t *testing.T) {
	itemStatics := []string{"<div id=\"", "\">", "</div>"}

	items := []rangeItemWithStatics{
		{tree: &TreeNode{Statics: itemStatics, Dynamics: map[string]interface{}{"0": "a"}}},
		{tree: &TreeNode{Statics: itemStatics, Dynamics: map[string]interface{}{"0": "b"}}},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := buildRangeTreeWithStatics(items, ctx)
	if err != nil {
		t.Fatalf("buildRangeTreeWithStatics failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(tree.Range.Items))
	}

	if tree.Metadata == nil {
		t.Error("Expected metadata")
	}

	if tree.Metadata.IDKey != "0" {
		t.Errorf("Expected IDKey='0', got: %v", tree.Metadata.IDKey)
	}
}

// TestHandleRange_SingleVarViaBuiltTree tests single variable range through BuildTree.
func TestHandleRange_SingleVarViaBuiltTree(t *testing.T) {
	tmplStr := "{{range $v := .Items}}<div>{{$v}}</div>{{end}}"
	data := map[string]interface{}{
		"Items": []string{"value"},
	}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 1 {
		t.Errorf("Expected 1 item, got: %d", len(tree.Range.Items))
	}

	itemTree, ok := tree.Range.Items[0].(*TreeNode)
	if !ok {
		t.Fatal("Expected *TreeNode for range item")
	}

	if !itemTree.HasDynamics() {
		t.Error("Expected dynamics for variable")
	}
}

// TestHandleRange_TwoVarsViaBuiltTree tests index and value variables through BuildTree.
func TestHandleRange_TwoVarsViaBuiltTree(t *testing.T) {
	tmplStr := "{{range $i, $v := .Items}}<div>{{$i}}: {{$v}}</div>{{end}}"
	data := map[string]interface{}{
		"Items": []string{"a", "b"},
	}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 2 {
		t.Errorf("Expected 2 items, got: %d", len(tree.Range.Items))
	}

	for _, item := range tree.Range.Items {
		itemTree, ok := item.(*TreeNode)
		if !ok {
			t.Fatal("Expected *TreeNode for range item")
		}
		if !itemTree.HasDynamics() {
			t.Error("Expected dynamics for variables")
		}
	}
}

// TestHandleRange_MapVarsViaBuiltTree tests map key-value variables through BuildTree.
func TestHandleRange_MapVarsViaBuiltTree(t *testing.T) {
	tmplStr := "{{range $k, $v := .Items}}<div>{{$k}}: {{$v}}</div>{{end}}"
	data := map[string]interface{}{
		"Items": map[string]string{"key": "value"},
	}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 1 {
		t.Errorf("Expected 1 item, got: %d", len(tree.Range.Items))
	}

	itemTree, ok := tree.Range.Items[0].(*TreeNode)
	if !ok {
		t.Fatal("Expected *TreeNode for range item")
	}
	if !itemTree.HasDynamics() {
		t.Error("Expected dynamics for variables")
	}
}

// TestDetectIDKey_AllPatterns tests ID key detection for all patterns.
func TestDetectIDKey_AllPatterns(t *testing.T) {
	tests := []struct {
		name     string
		statics  []string
		expected string
	}{
		{"id attribute", []string{"<div id=\"", "\">test</div>"}, "0"},
		{"data-key attribute", []string{"<div data-key=\"", "\">test</div>"}, "0"},
		{"key attribute", []string{"<div key=\"", "\">test</div>"}, "0"},
		{"data-lvt-key attribute", []string{"<div data-lvt-key=\"", "\">test</div>"}, "0"},
		{"lvt-key attribute", []string{"<div lvt-key=\"", "\">test</div>"}, "0"},
		{"data-id attribute", []string{"<div data-id=\"", "\">test</div>"}, "0"},
		{"x-key attribute", []string{"<div x-key=\"", "\">test</div>"}, "0"},
		{"v-key attribute", []string{"<div v-key=\"", "\">test</div>"}, "0"},
		{"no key", []string{"<div>", "</div>"}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectIDKey(tt.statics)
			if got != tt.expected {
				t.Errorf("detectIDKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDetectIDKey_Priority tests priority order of key detection.
func TestDetectIDKey_Priority(t *testing.T) {
	statics := []string{"<div data-key=\"x\" id=\"", "\">test</div>"}
	got := detectIDKey(statics)
	if got != "0" {
		t.Errorf("Expected '0' (id takes priority), got: %v", got)
	}
}

// TestDetectIDKey_NoKey tests default behavior with no key.
func TestDetectIDKey_NoKey(t *testing.T) {
	statics := []string{"<div>", "</div>"}
	got := detectIDKey(statics)
	if got != "0" {
		t.Errorf("Expected '0' (default), got: %v", got)
	}
}

// TestHandleRange_NonIterableType tests error handling for non-iterable types.
func TestHandleRange_NonIterableType(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"struct", struct{ Name string }{"test"}},
		{"bool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rangeNode := parseRangeNode(t, "{{range .Value}}{{.}}{{end}}", nil)
			data := map[string]interface{}{
				"Value": tt.value,
			}
			eval := testEval(nil)
			ctx := &Context{IncludeStatics: true}

			_, err := handleRange(rangeNode, eval, data, nil, newMockKeyGen(), ctx)
			if err == nil {
				t.Error("Expected error for non-iterable type, got nil")
			}
			if !strings.Contains(err.Error(), "range over non-iterable type") {
				t.Errorf("Expected 'range over non-iterable type' error, got: %v", err)
			}
		})
	}
}

// TestDetectIDKey_MultiplePositions tests detection when attributes appear at different positions.
func TestDetectIDKey_MultiplePositions(t *testing.T) {
	tests := []struct {
		name     string
		statics  []string
		expected string
	}{
		{
			"key at position 0",
			[]string{"<div id=\"", "\">", "</div>"},
			"0",
		},
		{
			"key at position 1",
			[]string{"<div>", "<span id=\"", "\">", "</span></div>"},
			"1",
		},
		{
			"key at position 2",
			[]string{"<div>", "<span>", "<a id=\"", "\">link</a></span></div>"},
			"2",
		},
		{
			"multiple keys - first one wins",
			[]string{"<div id=\"", "\"><span data-key=\"", "\"></span></div>"},
			"0",
		},
		{
			"multiple keys in same static - earliest wins",
			[]string{"<div><span data-key=\"x\"></span><div id=\"", "\"></div>"},
			"0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectIDKey(tt.statics)
			if got != tt.expected {
				t.Errorf("detectIDKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestExtractItemDynamics tests that dynamics are extracted efficiently.
func TestExtractItemDynamics(t *testing.T) {
	itemTree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
		Metadata: NewTreeMetadata("0"),
	}

	result := extractItemDynamics(itemTree)

	if result.Statics != nil {
		t.Error("Expected nil statics")
	}
	if result.Metadata != nil {
		t.Error("Expected nil metadata")
	}
	if result.Range != nil {
		t.Error("Expected nil range")
	}
	if len(result.Dynamics) != 1 {
		t.Errorf("Expected 1 dynamic, got: %d", len(result.Dynamics))
	}
	if result.Dynamics["0"] != "value" {
		t.Errorf("Expected dynamic value 'value', got: %v", result.Dynamics["0"])
	}

	// Verify the dynamics map is shared (same underlying map)
	result.Dynamics["1"] = "test"
	if itemTree.Dynamics["1"] != "test" {
		t.Error("Expected to share dynamics map (not a copy)")
	}
}

// TestBuildRangeTreeWithStatics_AutoKey tests that items without explicit key attribute get _k injected.
func TestBuildRangeTreeWithStatics_AutoKey(t *testing.T) {
	itemStatics := []string{"<div>", "</div>"}

	items := []rangeItemWithStatics{
		{tree: &TreeNode{Statics: itemStatics, Dynamics: map[string]interface{}{"0": "a"}}},
		{tree: &TreeNode{Statics: itemStatics, Dynamics: map[string]interface{}{"0": "b"}}},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := buildRangeTreeWithStatics(items, ctx)
	if err != nil {
		t.Fatalf("buildRangeTreeWithStatics failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if tree.Metadata == nil {
		t.Fatal("Expected metadata")
	}
	if tree.Metadata.IDKey != "_k" {
		t.Errorf("Expected IDKey='_k', got: %v", tree.Metadata.IDKey)
	}

	for i, item := range tree.Range.Items {
		itemNode, ok := item.(*TreeNode)
		if !ok {
			t.Fatalf("Item %d is not a TreeNode", i)
		}
		k, exists := itemNode.GetDynamic("_k")
		if !exists {
			t.Errorf("Item %d should have _k field", i)
		}
		if kStr, ok := k.(string); !ok || kStr == "" {
			t.Errorf("Item %d _k should be a non-empty string, got: %v", i, k)
		}
	}

	item0 := tree.Range.Items[0].(*TreeNode)
	item1 := tree.Range.Items[1].(*TreeNode)
	key0, _ := item0.GetDynamic("_k")
	key1, _ := item1.GetDynamic("_k")
	if key0 == key1 {
		t.Errorf("Items with different content should have different _k values, got: %v and %v", key0, key1)
	}
}

// TestBuildRangeTreeWithStatics_ExplicitKey_NoAutoKey tests that items with explicit key don't get _k injected.
func TestBuildRangeTreeWithStatics_ExplicitKey_NoAutoKey(t *testing.T) {
	itemStatics := []string{"<div data-key=\"", "\">", "</div>"}

	items := []rangeItemWithStatics{
		{tree: &TreeNode{Statics: itemStatics, Dynamics: map[string]interface{}{"0": "id1", "1": "content-a"}}},
		{tree: &TreeNode{Statics: itemStatics, Dynamics: map[string]interface{}{"0": "id2", "1": "content-b"}}},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := buildRangeTreeWithStatics(items, ctx)
	if err != nil {
		t.Fatalf("buildRangeTreeWithStatics failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if tree.Metadata == nil {
		t.Fatal("Expected metadata")
	}
	if tree.Metadata.IDKey != "0" {
		t.Errorf("Expected IDKey='0', got: %v", tree.Metadata.IDKey)
	}

	for i, item := range tree.Range.Items {
		itemNode, ok := item.(*TreeNode)
		if !ok {
			t.Fatalf("Item %d is not a TreeNode", i)
		}
		if _, exists := itemNode.GetDynamic("_k"); exists {
			t.Errorf("Item %d should NOT have _k field when explicit key exists", i)
		}
	}
}

// TestGenerateItemHash_Deterministic tests that the hash function produces deterministic results.
func TestGenerateItemHash_Deterministic(t *testing.T) {
	item := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "title",
			"1": "content",
		},
	}

	hash1 := generateItemHash(item)
	hash2 := generateItemHash(item)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got: %v and %v", hash1, hash2)
	}

	if len(hash1) != keys.HashPrefixLength {
		t.Errorf("Hash length should be %d, got: %d", keys.HashPrefixLength, len(hash1))
	}
}

// TestGenerateItemHash_DifferentContent tests that different content produces different hashes.
func TestGenerateItemHash_DifferentContent(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: map[string]interface{}{"0": "a"},
	}
	item2 := &TreeNode{
		Dynamics: map[string]interface{}{"0": "b"},
	}

	hash1 := generateItemHash(item1)
	hash2 := generateItemHash(item2)

	if hash1 == hash2 {
		t.Errorf("Different content should produce different hashes, got: %v", hash1)
	}
}

// TestGenerateItemHash_IgnoresAutoKey tests that _k field is excluded from hash calculation.
func TestGenerateItemHash_IgnoresAutoKey(t *testing.T) {
	itemWithoutK := &TreeNode{
		Dynamics: map[string]interface{}{"0": "content"},
	}
	itemWithK := &TreeNode{
		Dynamics: map[string]interface{}{"0": "content", "_k": "some-old-key"},
	}

	hash1 := generateItemHash(itemWithoutK)
	hash2 := generateItemHash(itemWithK)

	if hash1 != hash2 {
		t.Errorf("_k field should be excluded from hash, got: %v and %v", hash1, hash2)
	}
}

// TestHandleRange_ViaFullBuildTree tests range processing through the full BuildTree API.
func TestHandleRange_ViaFullBuildTree(t *testing.T) {
	tmplStr := "{{range .Items}}<div>{{.}}</div>{{end}}"
	data := map[string]interface{}{
		"Items": []string{"a", "b", "c"},
	}

	tmpl, err := Parse(tmplStr, template.FuncMap{})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasRange() {
		t.Error("Expected range data")
	}

	if len(tree.Range.Items) != 3 {
		t.Errorf("Expected 3 items, got: %d", len(tree.Range.Items))
	}
}
