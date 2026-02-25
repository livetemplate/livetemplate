package diff

import (
	"fmt"
	"reflect"
	"testing"
)

// TestIsEmpty_AllTypes tests empty detection for all value types.
func TestIsEmpty_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"empty TreeNode", &TreeNode{}, true},
		{"TreeNode with statics", &TreeNode{Statics: []string{"<div>"}}, false},
		{"TreeNode with dynamics", &TreeNode{Dynamics: map[string]any{"0": "val"}}, false},
		{"TreeNode with range", &TreeNode{Range: &RangeData{Items: []any{}}}, false},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"empty map", map[string]any{}, true},
		{"non-empty map", map[string]any{"key": "value"}, false},
		{"empty array", []any{}, true},
		{"non-empty array", []any{"item"}, false},
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
				Range:   &RangeData{Items: []any{}},
				Statics: []string{"<li>", "</li>"},
			},
			want: true,
		},
		{
			name: "TreeNode with Range but no Statics",
			value: &TreeNode{
				Range: &RangeData{Items: []any{}},
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
		value map[string]any
		want  bool
	}{
		{
			name: "map with both 'd' and 's' keys",
			value: map[string]any{
				"d": []any{},
				"s": []string{"<li>", "</li>"},
			},
			want: true,
		},
		{
			name: "map with only 'd' key",
			value: map[string]any{
				"d": []any{},
			},
			want: false,
		},
		{
			name: "map with only 's' key",
			value: map[string]any{
				"s": []string{"<div>"},
			},
			want: false,
		},
		{
			name:  "empty map",
			value: map[string]any{},
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
		value any
		want  bool
	}{
		{
			name: "TreeNode with items",
			value: &TreeNode{
				Range: &RangeData{
					Items: []any{"item1", "item2"},
				},
			},
			want: true,
		},
		{
			name: "TreeNode with empty items",
			value: &TreeNode{
				Range: &RangeData{
					Items: []any{},
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
		value any
		want  bool
	}{
		{
			name: "direct range construct",
			value: &TreeNode{
				Range:   &RangeData{Items: []any{}},
				Statics: []string{"<li>", "</li>"},
			},
			want: true,
		},
		{
			name: "nested range in dynamics",
			value: &TreeNode{
				Dynamics: map[string]any{
					"0": &TreeNode{
						Range:   &RangeData{Items: []any{}},
						Statics: []string{"<li>", "</li>"},
					},
				},
			},
			want: true,
		},
		{
			name: "no range construct",
			value: &TreeNode{
				Dynamics: map[string]any{
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

// TestDeepEqual tests deep equality for various types.
func TestDeepEqual(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{"identical strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"identical ints", 42, 42, true},
		{"different ints", 42, 43, false},
		{"both nil", nil, nil, true},
		{"one nil", nil, "value", false},
		{"identical maps", map[string]any{"key": "value"}, map[string]any{"key": "value"}, true},
		{"different maps", map[string]any{"key": "value1"}, map[string]any{"key": "value2"}, false},
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
		Dynamics: map[string]any{
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
		Dynamics: map[string]any{
			"0": "value1",
			"1": "value2",
		},
	}

	item2 := &TreeNode{
		Dynamics: map[string]any{
			"0": "value1",
			"1": "value2",
		},
	}

	item3 := &TreeNode{
		Dynamics: map[string]any{
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
	items := []any{
		&TreeNode{Dynamics: map[string]any{"0": "id1", "1": "Name1"}},
		&TreeNode{Dynamics: map[string]any{"0": "id2", "1": "Name2"}},
		&TreeNode{Dynamics: map[string]any{"0": "id3", "1": "Name3"}},
	}

	// Include data-key attribute so explicit keys are detected at position 0
	statics := []string{`<li data-key="`, `">`, `</li>`}

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
		itemsByKey map[string]any
		want       string
	}{
		{
			name: "items with position field (string pattern #0, #1)",
			itemsByKey: map[string]any{
				"id1": &TreeNode{
					Dynamics: map[string]any{
						"0": "id1",
						"1": "#0", // Position pattern
					},
				},
				"id2": &TreeNode{
					Dynamics: map[string]any{
						"0": "id2",
						"1": "#1", // Position pattern
					},
				},
			},
			want: "1",
		},
		{
			name: "items without position field",
			itemsByKey: map[string]any{
				"id1": &TreeNode{
					Dynamics: map[string]any{
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
	item1 := &TreeNode{Dynamics: map[string]any{"0": "id1", "1": "Name1"}}
	item2 := &TreeNode{Dynamics: map[string]any{"0": "id2", "1": "Name2"}}

	// Include data-key attribute so explicit keys are detected at position 0
	statics := []string{`<li data-key="`, `">`, `</li>`}

	tests := []struct {
		name     string
		oldItems []any
		newItems []any
		oldKeys  []string
		newKeys  []string
		want     bool
	}{
		{
			name:     "pure reordering",
			oldItems: []any{item1, item2},
			newItems: []any{item2, item1},
			oldKeys:  []string{"id1", "id2"},
			newKeys:  []string{"id2", "id1"},
			want:     true,
		},
		{
			name:     "different items",
			oldItems: []any{item1},
			newItems: []any{item2},
			oldKeys:  []string{"id1"},
			newKeys:  []string{"id2"},
			want:     false,
		},
		{
			name:     "same order",
			oldItems: []any{item1, item2},
			newItems: []any{item1, item2},
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
	oldItems := []any{
		&TreeNode{Dynamics: map[string]any{"0": "id1"}},
		&TreeNode{Dynamics: map[string]any{"0": "id2"}},
	}

	newItems := []any{
		&TreeNode{Dynamics: map[string]any{"0": "id1"}},
		&TreeNode{Dynamics: map[string]any{"0": "id3"}}, // New
		&TreeNode{Dynamics: map[string]any{"0": "id4"}}, // New
	}

	// Include data-key attribute so explicit keys are detected at position 0
	statics := []string{`<li data-key="`, `">`, `</li>`}

	newKeys := FindNewItems(oldItems, newItems, statics)

	expected := []string{"id3", "id4"}
	if !reflect.DeepEqual(newKeys, expected) {
		t.Errorf("FindNewItems() = %v, want %v", newKeys, expected)
	}
}

// TestAreAllItemsAtStart tests detecting if all items are at start.
func TestAreAllItemsAtStart(t *testing.T) {
	// Include data-key attribute so explicit keys are detected at position 0
	statics := []string{`<li data-key="`, `">`, `</li>`}

	// Test basic functionality - ensure no panic
	newKeys := []string{"new1"}
	newItems := []any{
		map[string]any{"0": "new1"},
		map[string]any{"0": "old1"},
	}

	// Just test it doesn't panic
	_ = AreAllItemsAtStart(newKeys, newItems, statics)
	_ = AreAllItemsAtStart([]string{}, newItems, statics)
	_ = AreAllItemsAtStart(newKeys, []any{}, statics)
}

// TestAreAllItemsAtEnd tests detecting if all items are at end.
func TestAreAllItemsAtEnd(t *testing.T) {
	// Include data-key attribute so explicit keys are detected at position 0
	statics := []string{`<li data-key="`, `">`, `</li>`}

	tests := []struct {
		name     string
		newKeys  []string
		oldItems []any
		newItems []any
		want     bool
	}{
		{
			name:    "all items at end",
			newKeys: []string{"new1", "new2"},
			oldItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
			},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
				&TreeNode{Dynamics: map[string]any{"0": "new2"}},
			},
			want: true,
		},
		{
			name:    "items not at end",
			newKeys: []string{"new1"},
			oldItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
			},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
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
		oldItems []any
		newItems []any
		want     bool
	}{
		{
			name:    "simple append (not complex)",
			newKeys: []string{"new1"},
			oldItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
			},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
			},
			want: false,
		},
		{
			name:    "simple prepend (not complex)",
			newKeys: []string{"new1"},
			oldItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
			},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
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
			Items: []any{
				&TreeNode{Dynamics: map[string]any{"0": "id1"}},
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
		Dynamics: map[string]any{
			"0": &TreeNode{
				Range:   &RangeData{Items: []any{}},
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
		Dynamics: map[string]any{
			"0": &TreeNode{
				Range:   &RangeData{Items: []any{}},
				Statics: []string{"<li>", "</li>"},
			},
		},
	}

	newTree := &TreeNode{
		Dynamics: map[string]any{
			"0": &TreeNode{
				Range:   &RangeData{Items: []any{}},
				Statics: []string{"<li>", "</li>"},
			},
		},
	}

	matches := FindRangeConstructMatches(oldTree, newTree)

	if len(matches) == 0 {
		t.Error("FindRangeConstructMatches() should find matching ranges")
	}
}

// TestGenerateItemHash_Consistency tests hash stability and consistency.
func TestGenerateItemHash_Consistency(t *testing.T) {
	tests := []struct {
		name string
		item *TreeNode
	}{
		{
			name: "empty dynamics",
			item: &TreeNode{Dynamics: map[string]any{}},
		},
		{
			name: "nil dynamics",
			item: &TreeNode{},
		},
		{
			name: "with reserved key",
			item: &TreeNode{
				Dynamics: map[string]any{
					"_k": "reserved-key",
					"0":  "value1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Hash should be consistent across multiple calls
			hash1 := GenerateItemHash(tt.item)
			hash2 := GenerateItemHash(tt.item)

			if hash1 != hash2 {
				t.Errorf("Hash not consistent: %s != %s", hash1, hash2)
			}

			// Hash should not be empty
			if hash1 == "" {
				t.Error("Hash should not be empty")
			}

			// Hash should respect hashPrefixLength
			if len(hash1) > 12 {
				t.Errorf("Hash too long: %d characters (expected <= 12)", len(hash1))
			}
		})
	}
}

// TestGenerateItemHash_Collisions tests hash distribution and collision avoidance.
func TestGenerateItemHash_Collisions(t *testing.T) {
	// Test that different content produces different hashes
	t.Run("different content produces different hashes", func(t *testing.T) {
		item1 := &TreeNode{
			Dynamics: map[string]any{
				"0": "value1",
				"1": "value2",
			},
		}

		item2 := &TreeNode{
			Dynamics: map[string]any{
				"0": "different",
				"1": "values",
			},
		}

		hash1 := GenerateItemHash(item1)
		hash2 := GenerateItemHash(item2)

		if hash1 == hash2 {
			t.Error("Different items should produce different hashes")
		}
	})

	// Test hash distribution with many items
	t.Run("hash distribution across many items", func(t *testing.T) {
		const numItems = 1000
		hashes := make(map[string]bool, numItems)
		collisions := 0

		for i := range numItems {
			item := &TreeNode{
				Dynamics: map[string]any{
					"0": fmt.Sprintf("id-%d", i),
					"1": fmt.Sprintf("name-%d", i),
					"2": i,
				},
			}

			hash := GenerateItemHash(item)

			if hashes[hash] {
				collisions++
			}
			hashes[hash] = true
		}

		// We expect zero collisions for 1000 items with 48 bits of entropy
		if collisions > 0 {
			t.Errorf("Unexpected collisions: %d out of %d items", collisions, numItems)
		}

		// Verify we got unique hashes
		if len(hashes) != numItems {
			t.Errorf("Expected %d unique hashes, got %d", numItems, len(hashes))
		}
	})

	// Test with large TreeNode structures
	t.Run("large structures produce stable hashes", func(t *testing.T) {
		largeItem := &TreeNode{
			Dynamics: map[string]any{
				"0":  "id",
				"1":  "name",
				"2":  "description",
				"3":  123,
				"4":  456.789,
				"5":  true,
				"6":  []any{"a", "b", "c"},
				"7":  map[string]any{"key": "value"},
				"8":  "long text content that extends beyond normal field sizes",
				"9":  []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				"10": map[string]any{"nested": map[string]any{"deep": "value"}},
			},
		}

		// Hash should be stable even with large content
		hash1 := GenerateItemHash(largeItem)
		hash2 := GenerateItemHash(largeItem)

		if hash1 != hash2 {
			t.Error("Hash not consistent for large structure")
		}

		if hash1 == "" {
			t.Error("Hash should not be empty for large structure")
		}

		// Verify it's still within length limit
		if len(hash1) > 12 {
			t.Errorf("Hash too long for large structure: %d characters", len(hash1))
		}
	})

	// Test field order independence (should be deterministic due to sorting)
	t.Run("field order independence", func(t *testing.T) {
		item1 := &TreeNode{
			Dynamics: map[string]any{
				"0": "a",
				"1": "b",
				"2": "c",
			},
		}

		item2 := &TreeNode{
			Dynamics: map[string]any{
				"2": "c",
				"0": "a",
				"1": "b",
			},
		}

		hash1 := GenerateItemHash(item1)
		hash2 := GenerateItemHash(item2)

		// Should produce same hash since content is same (order irrelevant due to sorting)
		if hash1 != hash2 {
			t.Error("Same content in different field order should produce same hash")
		}
	})
}

// TestFindKeyPositionFromStatics_AllKeyTypes tests all key attribute types.
func TestFindKeyPositionFromStatics_AllKeyTypes(t *testing.T) {
	tests := []struct {
		name     string
		statics  any
		expected int
	}{
		{
			name:     "data-lvt-key attribute (highest priority)",
			statics:  []string{`<li data-lvt-key="`, `">`, `</li>`},
			expected: 0,
		},
		{
			name:     "data-key attribute",
			statics:  []string{`<li data-key="`, `">`, `</li>`},
			expected: 0,
		},
		{
			name:     "key attribute",
			statics:  []string{`<li key="`, `">`, `</li>`},
			expected: 0,
		},
		{
			name:     "id attribute",
			statics:  []string{`<li id="`, `">`, `</li>`},
			expected: 0,
		},
		{
			name:     "no key attribute",
			statics:  []string{`<li>`, `</li>`},
			expected: -1, // Returns -1 when no key attribute found, enabling fallback to hash
		},
		{
			name:     "key in middle position",
			statics:  []string{`<ul><li class="`, `" key="`, `">`, `</li></ul>`},
			expected: 1,
		},
		{
			name:     "[]interface{} format",
			statics:  []any{`<li data-key="`, `">`, `</li>`},
			expected: 0,
		},
		{
			name:     "[]interface{} with non-string",
			statics:  []any{`<li>`, 123, `</li>`},
			expected: -1, // No key attribute found, returns -1
		},
		{
			name:     "empty statics",
			statics:  []string{},
			expected: -1, // No key attribute found, returns -1
		},
		{
			name:     "nil statics",
			statics:  nil,
			expected: -1, // No key attribute found, returns -1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindKeyPositionFromStatics(tt.statics)
			if got != tt.expected {
				t.Errorf("FindKeyPositionFromStatics() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestAreAllItemsAtStart_TreeNode tests with TreeNode items (not just maps).
func TestAreAllItemsAtStart_TreeNode(t *testing.T) {
	// Include data-key attribute so explicit keys are detected at position 0
	statics := []string{`<li data-key="`, `">`, `</li>`}

	tests := []struct {
		name     string
		newKeys  []string
		newItems []any
		want     bool
	}{
		{
			name:    "TreeNode items at start",
			newKeys: []string{"new1", "new2"},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
				&TreeNode{Dynamics: map[string]any{"0": "new2"}},
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
			},
			want: true,
		},
		{
			name:    "TreeNode items not at start",
			newKeys: []string{"new1"},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
			},
			want: false,
		},
		{
			name:     "empty newKeys",
			newKeys:  []string{},
			newItems: []any{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AreAllItemsAtStart(tt.newKeys, tt.newItems, statics)
			if got != tt.want {
				t.Errorf("AreAllItemsAtStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsComplexInsertionPattern_EdgeCases tests edge cases.
func TestIsComplexInsertionPattern_EdgeCases(t *testing.T) {
	statics := []string{"<li>", "</li>"}

	tests := []struct {
		name     string
		newKeys  []string
		oldItems []any
		newItems []any
		want     bool
	}{
		{
			name:     "empty newKeys",
			newKeys:  []string{},
			oldItems: []any{},
			newItems: []any{},
			want:     false,
		},
		{
			name:    "single insertion point",
			newKeys: []string{"new1"},
			oldItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
			},
			newItems: []any{
				&TreeNode{Dynamics: map[string]any{"0": "new1"}},
				&TreeNode{Dynamics: map[string]any{"0": "old1"}},
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

// =============================================================================
// AUTO-KEY GENERATION TESTS
// These tests ensure the library correctly auto-generates keys when no explicit
// key attribute (data-key, id, etc.) is present in the template.
// =============================================================================

// TestGetItemKey_FallbackToHash verifies that GetItemKey falls back to content-based
// hash generation when no key attribute is found in statics.
func TestGetItemKey_FallbackToHash(t *testing.T) {
	tests := []struct {
		name           string
		statics        any
		items          []*TreeNode
		expectHashKeys bool
		description    string
	}{
		{
			name:    "no key attribute - should generate hash keys",
			statics: []string{"<li>", "</li>"},
			items: []*TreeNode{
				{Dynamics: map[string]any{"0": "content1"}},
				{Dynamics: map[string]any{"0": "content2"}},
			},
			expectHashKeys: true,
			description:    "Without data-key/id attributes, should use content hash",
		},
		{
			name:    "with data-key attribute - should use explicit key",
			statics: []string{`<li data-key="`, `">`, `</li>`},
			items: []*TreeNode{
				{Dynamics: map[string]any{"0": "id1", "1": "content1"}},
				{Dynamics: map[string]any{"0": "id2", "1": "content2"}},
			},
			expectHashKeys: false,
			description:    "With data-key attribute, should use position 0 value",
		},
		{
			name:    "with id attribute - should use explicit key",
			statics: []string{`<li id="`, `">`, `</li>`},
			items: []*TreeNode{
				{Dynamics: map[string]any{"0": "item-1", "1": "content"}},
			},
			expectHashKeys: false,
			description:    "With id attribute, should use position 0 value",
		},
		{
			name:    "empty statics - should generate hash keys",
			statics: []string{},
			items: []*TreeNode{
				{Dynamics: map[string]any{"0": "value"}},
			},
			expectHashKeys: true,
			description:    "Empty statics should fall back to hash",
		},
		{
			name:    "nil statics - should generate hash keys",
			statics: nil,
			items: []*TreeNode{
				{Dynamics: map[string]any{"0": "value"}},
			},
			expectHashKeys: true,
			description:    "Nil statics should fall back to hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, item := range tt.items {
				key, exists := GetItemKey(item, tt.statics)
				if !exists {
					t.Fatalf("Item %d: GetItemKey should return exists=true, got false", i)
				}
				if key == "" {
					t.Fatalf("Item %d: GetItemKey should return non-empty key", i)
				}

				if tt.expectHashKeys {
					// Hash keys are 12 hex characters (from hashPrefixLength)
					if len(key) != 12 {
						t.Errorf("Item %d: Expected 12-char hash key, got %d chars: %s (%s)",
							i, len(key), key, tt.description)
					}
					// Verify it's a valid hex string
					for _, c := range key {
						if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
							t.Errorf("Item %d: Hash key contains non-hex char: %s", i, key)
							break
						}
					}
				} else {
					// Should be the explicit key from position 0
					expectedKey := item.Dynamics["0"].(string)
					if key != expectedKey {
						t.Errorf("Item %d: Expected explicit key %q, got %q (%s)",
							i, expectedKey, key, tt.description)
					}
				}
			}
		})
	}
}

// TestGetItemKey_SamePosition0DifferentContent verifies that items with the same
// value at position 0 but different overall content get DIFFERENT hash keys.
// This was the core bug - before the fix, all items would get the same key.
func TestGetItemKey_SamePosition0DifferentContent(t *testing.T) {
	// Statics without key attribute - simulates templates like:
	// {{range .Items}}<div class="{{if .Active}}active{{end}}">{{.Name}}</div>{{end}}
	statics := []string{`<div class="`, `">`, `</div>`}

	// Items with same value at position 0 (CSS class) but different content
	item1 := &TreeNode{
		Dynamics: map[string]any{
			"0": "active", // Same CSS class
			"1": "Alice",  // Different name
		},
	}
	item2 := &TreeNode{
		Dynamics: map[string]any{
			"0": "active", // Same CSS class
			"1": "Bob",    // Different name
		},
	}
	item3 := &TreeNode{
		Dynamics: map[string]any{
			"0": "", // Different CSS class (empty)
			"1": "Charlie",
		},
	}

	key1, _ := GetItemKey(item1, statics)
	key2, _ := GetItemKey(item2, statics)
	key3, _ := GetItemKey(item3, statics)

	// All keys should be unique despite same value at position 0
	if key1 == key2 {
		t.Errorf("BUG: Items with same position 0 but different content got same key: %s", key1)
	}
	if key1 == key3 {
		t.Errorf("Keys should be unique: key1=%s, key3=%s", key1, key3)
	}
	if key2 == key3 {
		t.Errorf("Keys should be unique: key2=%s, key3=%s", key2, key3)
	}

	// All should be hash keys (12 hex chars)
	for i, key := range []string{key1, key2, key3} {
		if len(key) != 12 {
			t.Errorf("Key %d should be 12-char hash, got %d: %s", i+1, len(key), key)
		}
	}
}

// TestExtractItemKeys_WithoutKeyAttribute verifies that ExtractItemKeys works
// correctly when no key attribute is present, generating unique hash keys.
func TestExtractItemKeys_WithoutKeyAttribute(t *testing.T) {
	// No key attribute in statics
	statics := []string{"<li>", "</li>"}

	items := []any{
		&TreeNode{Dynamics: map[string]any{"0": "First item"}},
		&TreeNode{Dynamics: map[string]any{"0": "Second item"}},
		&TreeNode{Dynamics: map[string]any{"0": "Third item"}},
	}

	keys := ExtractItemKeys(items, statics)

	// Should get 3 unique keys
	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys, got %d", len(keys))
	}

	// All keys should be unique
	keySet := make(map[string]bool)
	for i, key := range keys {
		if key == "" {
			t.Errorf("Key %d should not be empty", i)
		}
		if keySet[key] {
			t.Errorf("Duplicate key found: %s", key)
		}
		keySet[key] = true
	}

	// All should be hash keys (12 hex chars)
	for i, key := range keys {
		if len(key) != 12 {
			t.Errorf("Key %d should be 12-char hash, got %d: %s", i, len(key), key)
		}
	}
}

// TestExtractItemKeys_MixedScenarios tests various real-world scenarios
func TestExtractItemKeys_MixedScenarios(t *testing.T) {
	tests := []struct {
		name            string
		statics         any
		items           []any
		expectUniqueLen int
		description     string
	}{
		{
			name:    "todo list without data-key",
			statics: []string{`<div class="todo `, `">`, " - ", `</div>`},
			items: []any{
				&TreeNode{Dynamics: map[string]any{"0": "", "1": "Buy milk", "2": "2024-01-01"}},
				&TreeNode{Dynamics: map[string]any{"0": "completed", "1": "Walk dog", "2": "2024-01-02"}},
				&TreeNode{Dynamics: map[string]any{"0": "", "1": "Call mom", "2": "2024-01-03"}},
			},
			expectUniqueLen: 3,
			description:     "All todos should have unique keys even with same completion status",
		},
		{
			name:    "items with identical content",
			statics: []string{"<span>", "</span>"},
			items: []any{
				&TreeNode{Dynamics: map[string]any{"0": "Same"}},
				&TreeNode{Dynamics: map[string]any{"0": "Same"}},
			},
			expectUniqueLen: 1, // Same content = same hash = 1 unique key
			description:     "Identical items get same hash (expected behavior)",
		},
		{
			name:    "table rows with ID in hidden field",
			statics: []string{`<tr><td><input type="hidden" value="`, `"></td><td>`, `</td></tr>`},
			items: []any{
				&TreeNode{Dynamics: map[string]any{"0": "row-1", "1": "Data A"}},
				&TreeNode{Dynamics: map[string]any{"0": "row-2", "1": "Data B"}},
				&TreeNode{Dynamics: map[string]any{"0": "row-3", "1": "Data A"}}, // Same data, different ID
			},
			expectUniqueLen: 3,
			description:     "Rows with hidden IDs should be unique even with same visible data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := ExtractItemKeys(tt.items, tt.statics)

			if len(keys) != len(tt.items) {
				t.Fatalf("Expected %d keys, got %d", len(tt.items), len(keys))
			}

			// Count unique keys
			keySet := make(map[string]bool)
			for _, key := range keys {
				keySet[key] = true
			}

			if len(keySet) != tt.expectUniqueLen {
				t.Errorf("Expected %d unique keys, got %d (%s). Keys: %v",
					tt.expectUniqueLen, len(keySet), tt.description, keys)
			}
		})
	}
}

// TestGenerateItemHash_Stability verifies that hash generation is deterministic
// and stable across multiple calls.
func TestGenerateItemHash_Stability(t *testing.T) {
	item := &TreeNode{
		Dynamics: map[string]any{
			"0": "value1",
			"1": "value2",
			"2": 123,
			"3": true,
		},
	}

	// Generate hash multiple times
	hashes := make([]string, 100)
	for i := range 100 {
		hashes[i] = GenerateItemHash(item)
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Hash is not stable: call 0=%s, call %d=%s", hashes[0], i, hashes[i])
		}
	}
}

// TestGenerateItemHash_FieldOrderIndependence verifies that field order doesn't
// affect the hash (keys are sorted before hashing).
func TestGenerateItemHash_FieldOrderIndependence(t *testing.T) {
	item1 := &TreeNode{
		Dynamics: map[string]any{
			"0": "a",
			"1": "b",
			"2": "c",
		},
	}

	// Create with different insertion order (Go maps don't guarantee order)
	item2 := &TreeNode{
		Dynamics: map[string]any{
			"2": "c",
			"0": "a",
			"1": "b",
		},
	}

	hash1 := GenerateItemHash(item1)
	hash2 := GenerateItemHash(item2)

	if hash1 != hash2 {
		t.Errorf("Hash should be independent of field order: %s != %s", hash1, hash2)
	}
}

// TestFindKeyPositionFromStatics_ReturnsNegativeOne verifies that -1 is returned
// when no key attribute is found, enabling the hash fallback.
func TestFindKeyPositionFromStatics_ReturnsNegativeOne(t *testing.T) {
	tests := []struct {
		name    string
		statics any
		want    int
	}{
		{
			name:    "no key attribute returns -1",
			statics: []string{"<div>", "</div>"},
			want:    -1,
		},
		{
			name:    "class attribute only returns -1",
			statics: []string{`<div class="`, `">`, `</div>`},
			want:    -1,
		},
		{
			name:    "style attribute only returns -1",
			statics: []string{`<span style="color: `, `">`, `</span>`},
			want:    -1,
		},
		{
			name:    "nil returns -1",
			statics: nil,
			want:    -1,
		},
		{
			name:    "empty slice returns -1",
			statics: []string{},
			want:    -1,
		},
		{
			name:    "data-key returns position",
			statics: []string{`<li data-key="`, `">`, `</li>`},
			want:    0,
		},
		{
			name:    "id returns position",
			statics: []string{`<div id="`, `">`, `</div>`},
			want:    0,
		},
		{
			name:    "key in second position",
			statics: []string{`<div class="`, `" data-key="`, `">`, `</div>`},
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindKeyPositionFromStatics(tt.statics)
			if got != tt.want {
				t.Errorf("FindKeyPositionFromStatics() = %d, want %d", got, tt.want)
			}
		})
	}
}
