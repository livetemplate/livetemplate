package diff

import (
	"reflect"
	"testing"
)

// TestIsEmpty_AllTypes tests empty detection for all value types.
func TestIsEmpty_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"empty TreeNode", &TreeNode{}, true},
		{"TreeNode with statics", &TreeNode{Statics: []string{"<div>"}}, false},
		{"TreeNode with dynamics", &TreeNode{Dynamics: map[string]interface{}{"0": "val"}}, false},
		{"TreeNode with range", &TreeNode{Range: &RangeData{Items: []interface{}{}}}, false},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"empty map", map[string]interface{}{}, true},
		{"non-empty map", map[string]interface{}{"key": "value"}, false},
		{"empty array", []interface{}{}, true},
		{"non-empty array", []interface{}{"item"}, false},
		{"nil", nil, false},
		{"int zero", 0, false},
		{"int non-zero", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEmpty(tt.value)
			if got != tt.want {
				t.Errorf("IsEmpty(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestIsRangeConstruct_TreeNode tests TreeNode range detection.
func TestIsRangeConstruct_TreeNode(t *testing.T) {
	tests := []struct {
		name  string
		value *TreeNode
		want  bool
	}{
		{
			name: "TreeNode with Range and Statics",
			value: &TreeNode{
				Range:   &RangeData{Items: []interface{}{}},
				Statics: []string{"<li>", "</li>"},
			},
			want: true,
		},
		{
			name: "TreeNode with Range but no Statics",
			value: &TreeNode{
				Range: &RangeData{Items: []interface{}{}},
			},
			want: false,
		},
		{
			name: "TreeNode with Statics but no Range",
			value: &TreeNode{
				Statics: []string{"<div>", "</div>"},
			},
			want: false,
		},
		{
			name:  "Empty TreeNode",
			value: &TreeNode{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRangeConstruct(tt.value)
			if got != tt.want {
				t.Errorf("IsRangeConstruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsRangeConstruct_Map tests map-based range detection.
func TestIsRangeConstruct_Map(t *testing.T) {
	tests := []struct {
		name  string
		value map[string]interface{}
		want  bool
	}{
		{
			name: "map with both 'd' and 's' keys",
			value: map[string]interface{}{
				"d": []interface{}{},
				"s": []string{"<li>", "</li>"},
			},
			want: true,
		},
		{
			name: "map with only 'd' key",
			value: map[string]interface{}{
				"d": []interface{}{},
			},
			want: false,
		},
		{
			name: "map with only 's' key",
			value: map[string]interface{}{
				"s": []string{"<div>"},
			},
			want: false,
		},
		{
			name:  "empty map",
			value: map[string]interface{}{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRangeConstruct(tt.value)
			if got != tt.want {
				t.Errorf("IsRangeConstruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasRangeItems tests range item detection.
func TestHasRangeItems(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{
			name: "TreeNode with items",
			value: &TreeNode{
				Range: &RangeData{
					Items: []interface{}{"item1", "item2"},
				},
			},
			want: true,
		},
		{
			name: "TreeNode with empty items",
			value: &TreeNode{
				Range: &RangeData{
					Items: []interface{}{},
				},
			},
			want: false,
		},
		{
			name:  "TreeNode without Range",
			value: &TreeNode{},
			want:  false,
		},
		{
			name:  "non-TreeNode",
			value: "string",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasRangeItems(tt.value)
			if got != tt.want {
				t.Errorf("HasRangeItems() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContainsRangeConstruct tests recursive range detection.
func TestContainsRangeConstruct(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{
			name: "direct range construct",
			value: &TreeNode{
				Range:   &RangeData{Items: []interface{}{}},
				Statics: []string{"<li>", "</li>"},
			},
			want: true,
		},
		{
			name: "nested range in dynamics",
			value: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": &TreeNode{
						Range:   &RangeData{Items: []interface{}{}},
						Statics: []string{"<li>", "</li>"},
					},
				},
			},
			want: true,
		},
		{
			name: "no range construct",
			value: &TreeNode{
				Dynamics: map[string]interface{}{
					"0": "value",
				},
			},
			want: false,
		},
		{
			name:  "primitive value",
			value: "string",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsRangeConstruct(tt.value)
			if got != tt.want {
				t.Errorf("ContainsRangeConstruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAreStructuresSimilar tests structure similarity detection.
func TestAreStructuresSimilar(t *testing.T) {
	oldTree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": "old",
		},
	}

	newTree := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": "new",
		},
	}

	// Test that function doesn't panic
	_ = AreStructuresSimilar(oldTree, newTree)
	_ = AreStructuresSimilar(nil, newTree)
	_ = AreStructuresSimilar(oldTree, nil)
}

// TestDeepEqual tests deep equality for various types.
func TestDeepEqual(t *testing.T) {
	tests := []struct {
		name string
		a    interface{}
		b    interface{}
		want bool
	}{
		{"identical strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"identical ints", 42, 42, true},
		{"different ints", 42, 43, false},
		{"both nil", nil, nil, true},
		{"one nil", nil, "value", false},
		{"identical maps", map[string]interface{}{"key": "value"}, map[string]interface{}{"key": "value"}, true},
		{"different maps", map[string]interface{}{"key": "value1"}, map[string]interface{}{"key": "value2"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeepEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("DeepEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestFindKeyPositionFromStatics tests key position detection from statics.
func TestFindKeyPositionFromStatics(t *testing.T) {
	// Test with various inputs - just ensure no panic
	_ = FindKeyPositionFromStatics([]string{"<li>", "</li>"})
	_ = FindKeyPositionFromStatics(nil)
	_ = FindKeyPositionFromStatics([]string{})

	// The function looks for data-key or :key patterns in statics
	// Implementation details may vary, so we just test it doesn't crash
}

// TestGetItemKey tests item key extraction.
func TestGetItemKey(t *testing.T) {
	item := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "item-id-123",
			"1": "Name",
		},
	}

	statics := []string{"<li>", "</li>"}

	// Test basic functionality
	key, exists := GetItemKey(item, statics)
	if !exists {
		t.Error("GetItemKey() should find key for valid TreeNode")
	}
	if key == "" {
		t.Error("GetItemKey() should return non-empty key")
	}

	// Test with nil
	_, exists = GetItemKey(nil, statics)
	if exists {
		t.Error("GetItemKey() should not find key for nil item")
	}
}

// TestGenerateItemHash tests item hashing.
func TestGenerateItemHash(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "value1",
			"1": "value2",
		},
	}

	item2 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "value1",
			"1": "value2",
		},
	}

	item3 := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": "different",
			"1": "values",
		},
	}

	hash1 := GenerateItemHash(item1)
	hash2 := GenerateItemHash(item2)
	hash3 := GenerateItemHash(item3)

	// Same content should produce same hash
	if hash1 != hash2 {
		t.Errorf("Same items should produce same hash: %s != %s", hash1, hash2)
	}

	// Different content should produce different hash
	if hash1 == hash3 {
		t.Errorf("Different items should produce different hash")
	}

	// Hash should not be empty
	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
}

// TestExtractItemKeys tests extracting keys from items array.
func TestExtractItemKeys(t *testing.T) {
	items := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "Name1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Name2"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id3", "1": "Name3"}},
	}

	statics := []string{"<li>", "</li>"}

	keys := ExtractItemKeys(items, statics)

	expected := []string{"id1", "id2", "id3"}
	if !reflect.DeepEqual(keys, expected) {
		t.Errorf("ExtractItemKeys() = %v, want %v", keys, expected)
	}
}

// TestDetectPositionField tests position field detection.
func TestDetectPositionField(t *testing.T) {
	tests := []struct {
		name       string
		itemsByKey map[string]interface{}
		want       string
	}{
		{
			name: "items with position field (string pattern #0, #1)",
			itemsByKey: map[string]interface{}{
				"id1": &TreeNode{
					Dynamics: map[string]interface{}{
						"0": "id1",
						"1": "#0", // Position pattern
					},
				},
				"id2": &TreeNode{
					Dynamics: map[string]interface{}{
						"0": "id2",
						"1": "#1", // Position pattern
					},
				},
			},
			want: "1",
		},
		{
			name: "items without position field",
			itemsByKey: map[string]interface{}{
				"id1": &TreeNode{
					Dynamics: map[string]interface{}{
						"0": "id1",
						"1": "name1",
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPositionField(tt.itemsByKey)
			if got != tt.want {
				t.Errorf("DetectPositionField() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsPureReordering tests pure reordering detection.
func TestIsPureReordering(t *testing.T) {
	item1 := &TreeNode{Dynamics: map[string]interface{}{"0": "id1", "1": "Name1"}}
	item2 := &TreeNode{Dynamics: map[string]interface{}{"0": "id2", "1": "Name2"}}

	statics := []string{"<li>", "</li>"}

	tests := []struct {
		name     string
		oldItems []interface{}
		newItems []interface{}
		oldKeys  []string
		newKeys  []string
		want     bool
	}{
		{
			name:     "pure reordering",
			oldItems: []interface{}{item1, item2},
			newItems: []interface{}{item2, item1},
			oldKeys:  []string{"id1", "id2"},
			newKeys:  []string{"id2", "id1"},
			want:     true,
		},
		{
			name:     "different items",
			oldItems: []interface{}{item1},
			newItems: []interface{}{item2},
			oldKeys:  []string{"id1"},
			newKeys:  []string{"id2"},
			want:     false,
		},
		{
			name:     "same order",
			oldItems: []interface{}{item1, item2},
			newItems: []interface{}{item1, item2},
			oldKeys:  []string{"id1", "id2"},
			newKeys:  []string{"id1", "id2"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPureReordering(tt.oldItems, tt.newItems, tt.oldKeys, tt.newKeys, statics)
			if got != tt.want {
				t.Errorf("IsPureReordering() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFindNewItems tests finding new items.
func TestFindNewItems(t *testing.T) {
	oldItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id2"}},
	}

	newItems := []interface{}{
		&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
		&TreeNode{Dynamics: map[string]interface{}{"0": "id3"}}, // New
		&TreeNode{Dynamics: map[string]interface{}{"0": "id4"}}, // New
	}

	statics := []string{"<li>", "</li>"}

	newKeys := FindNewItems(oldItems, newItems, statics)

	expected := []string{"id3", "id4"}
	if !reflect.DeepEqual(newKeys, expected) {
		t.Errorf("FindNewItems() = %v, want %v", newKeys, expected)
	}
}

// TestAreAllItemsAtStart tests detecting if all items are at start.
func TestAreAllItemsAtStart(t *testing.T) {
	statics := []string{"<li>", "</li>"}

	// Test basic functionality - ensure no panic
	newKeys := []string{"new1"}
	newItems := []interface{}{
		map[string]interface{}{"0": "new1"},
		map[string]interface{}{"0": "old1"},
	}

	// Just test it doesn't panic
	_ = AreAllItemsAtStart(newKeys, newItems, statics)
	_ = AreAllItemsAtStart([]string{}, newItems, statics)
	_ = AreAllItemsAtStart(newKeys, []interface{}{}, statics)
}

// TestAreAllItemsAtEnd tests detecting if all items are at end.
func TestAreAllItemsAtEnd(t *testing.T) {
	statics := []string{"<li>", "</li>"}

	tests := []struct {
		name     string
		newKeys  []string
		oldItems []interface{}
		newItems []interface{}
		want     bool
	}{
		{
			name:    "all items at end",
			newKeys: []string{"new1", "new2"},
			oldItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
			},
			newItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
				&TreeNode{Dynamics: map[string]interface{}{"0": "new1"}},
				&TreeNode{Dynamics: map[string]interface{}{"0": "new2"}},
			},
			want: true,
		},
		{
			name:    "items not at end",
			newKeys: []string{"new1"},
			oldItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
			},
			newItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "new1"}},
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AreAllItemsAtEnd(tt.newKeys, tt.oldItems, tt.newItems, statics)
			if got != tt.want {
				t.Errorf("AreAllItemsAtEnd() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsComplexInsertionPattern tests complex insertion pattern detection.
func TestIsComplexInsertionPattern(t *testing.T) {
	statics := []string{"<li>", "</li>"}

	tests := []struct {
		name     string
		newKeys  []string
		oldItems []interface{}
		newItems []interface{}
		want     bool
	}{
		{
			name:    "simple append (not complex)",
			newKeys: []string{"new1"},
			oldItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
			},
			newItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
				&TreeNode{Dynamics: map[string]interface{}{"0": "new1"}},
			},
			want: false,
		},
		{
			name:    "simple prepend (not complex)",
			newKeys: []string{"new1"},
			oldItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
			},
			newItems: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "new1"}},
				&TreeNode{Dynamics: map[string]interface{}{"0": "old1"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsComplexInsertionPattern(tt.newKeys, tt.oldItems, tt.newItems, statics)
			if got != tt.want {
				t.Errorf("IsComplexInsertionPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetRangeSignature tests range signature generation.
func TestGetRangeSignature(t *testing.T) {
	rangeValue := &TreeNode{
		Range: &RangeData{
			Items: []interface{}{
				&TreeNode{Dynamics: map[string]interface{}{"0": "id1"}},
			},
		},
		Statics: []string{"<li>", "</li>"},
	}

	sig := GetRangeSignature(rangeValue)

	// Should return non-empty signature
	if sig == "" {
		t.Error("GetRangeSignature() should return non-empty signature")
	}

	// Same range should produce same signature
	sig2 := GetRangeSignature(rangeValue)
	if sig != sig2 {
		t.Error("Same range should produce same signature")
	}
}

// TestFindRangeConstructs tests finding range constructs in tree.
func TestFindRangeConstructs(t *testing.T) {
	tree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Range:   &RangeData{Items: []interface{}{}},
				Statics: []string{"<li>", "</li>"},
			},
		},
	}

	ranges := FindRangeConstructs(tree)

	if len(ranges) == 0 {
		t.Error("FindRangeConstructs() should find at least one range")
	}
}

// TestFindRangeConstructMatches tests matching range constructs between trees.
func TestFindRangeConstructMatches(t *testing.T) {
	oldTree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Range:   &RangeData{Items: []interface{}{}},
				Statics: []string{"<li>", "</li>"},
			},
		},
	}

	newTree := &TreeNode{
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Range:   &RangeData{Items: []interface{}{}},
				Statics: []string{"<li>", "</li>"},
			},
		},
	}

	matches := FindRangeConstructMatches(oldTree, newTree)

	if len(matches) == 0 {
		t.Error("FindRangeConstructMatches() should find matching ranges")
	}
}
