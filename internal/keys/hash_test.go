package keys

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

func TestGenerateItemHash_Deterministic(t *testing.T) {
	dynamics := map[string]interface{}{
		"0": "title",
		"1": "content",
	}

	hash1 := GenerateItemHash(dynamics)
	hash2 := GenerateItemHash(dynamics)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got: %v and %v", hash1, hash2)
	}

	if len(hash1) != HashPrefixLength {
		t.Errorf("Hash length should be %d, got: %d", HashPrefixLength, len(hash1))
	}
}

func TestGenerateItemHash_DifferentContent(t *testing.T) {
	d1 := map[string]interface{}{"0": "a"}
	d2 := map[string]interface{}{"0": "b"}

	hash1 := GenerateItemHash(d1)
	hash2 := GenerateItemHash(d2)

	if hash1 == hash2 {
		t.Errorf("Different content should produce different hashes, got: %v", hash1)
	}
}

func TestGenerateItemHash_IgnoresAutoKey(t *testing.T) {
	without := map[string]interface{}{"0": "content"}
	with := map[string]interface{}{"0": "content", "_k": "some-old-key"}

	hash1 := GenerateItemHash(without)
	hash2 := GenerateItemHash(with)

	if hash1 != hash2 {
		t.Errorf("_k field should be excluded from hash, got: %v and %v", hash1, hash2)
	}
}

func TestGenerateItemHash_NilDynamics(t *testing.T) {
	hash := GenerateItemHash(nil)
	if hash == "" {
		t.Error("Nil dynamics should still produce a hash (hash of empty content)")
	}
}

func TestGenerateItemHash_EmptyDynamics(t *testing.T) {
	hash := GenerateItemHash(map[string]interface{}{})
	if hash == "" {
		t.Error("Empty dynamics should return a non-empty hash")
	}
}

func TestGenerateItemHash_UnhashableFallback(t *testing.T) {
	// Values that json.Marshal cannot serialize use a deterministic type-based fallback
	// instead of %v (which includes non-deterministic pointer addresses).
	ch := make(chan int)
	d1 := map[string]interface{}{"0": ch}
	d2 := map[string]interface{}{"0": ch}

	hash1 := GenerateItemHash(d1)
	hash2 := GenerateItemHash(d2)

	if hash1 != hash2 {
		t.Errorf("Unhashable fallback should be deterministic, got: %v and %v", hash1, hash2)
	}

	if hash1 == "" {
		t.Error("Unhashable values should still produce a non-empty hash")
	}
}

func TestGenerateItemHash_FieldOrderIndependence(t *testing.T) {
	d1 := map[string]interface{}{"0": "a", "1": "b", "2": "c"}
	d2 := map[string]interface{}{"2": "c", "0": "a", "1": "b"}

	hash1 := GenerateItemHash(d1)
	hash2 := GenerateItemHash(d2)

	if hash1 != hash2 {
		t.Errorf("Field order should not affect hash, got: %v and %v", hash1, hash2)
	}
}

func TestGenerateItemHashFromSlice_Deterministic(t *testing.T) {
	dynamics := []interface{}{"title", "content"}

	hash1 := GenerateItemHashFromSlice(dynamics)
	hash2 := GenerateItemHashFromSlice(dynamics)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got: %v and %v", hash1, hash2)
	}
	if len(hash1) != HashPrefixLength {
		t.Errorf("Hash length should be %d, got: %d", HashPrefixLength, len(hash1))
	}
}

func TestGenerateItemHashFromSlice_SkipsNilEntries(t *testing.T) {
	// Nil entries should be skipped: [a, nil, b] should hash identically
	// to [a, nil, b] — they represent the same logical content.
	// Also verify that the hash is non-empty (nils don't break hashing).
	withNils := []interface{}{"a", nil, "b"}
	hash := GenerateItemHashFromSlice(withNils)
	if hash == "" {
		t.Error("Hash should not be empty for non-nil content")
	}

	// Verify nil-only and empty slices produce the same hash (no content)
	nilOnly := []interface{}{nil, nil}
	empty := []interface{}{}
	if GenerateItemHashFromSlice(nilOnly) != GenerateItemHashFromSlice(empty) {
		t.Error("Nil-only and empty slices should produce the same hash")
	}

	// Verify position matters: [a, nil, b] != [b, nil, a]
	reversed := []interface{}{"b", nil, "a"}
	revHash := GenerateItemHashFromSlice(reversed)
	if hash == revHash {
		t.Errorf("Different orderings should produce different hashes")
	}
}

func TestGenerateItemHashFromSlice_MatchesMapVersion(t *testing.T) {
	// For single-digit keys (0-9), map sort order matches numeric order,
	// so slice and map versions should produce identical hashes.
	sliceHash := GenerateItemHashFromSlice([]interface{}{"a", "b", "c"})
	mapHash := GenerateItemHash(map[string]interface{}{"0": "a", "1": "b", "2": "c"})

	if sliceHash != mapHash {
		t.Errorf("Slice and map versions should match for single-digit keys, got slice=%v map=%v", sliceHash, mapHash)
	}
}

func TestGenerateItemHashFromSlice_Empty(t *testing.T) {
	hash := GenerateItemHashFromSlice([]interface{}{})
	if hash == "" {
		t.Error("Empty slice should return a non-empty hash")
	}
}

func TestGenerateItemHashFromSlice_Nil(t *testing.T) {
	hash := GenerateItemHashFromSlice(nil)
	if hash == "" {
		t.Error("Nil slice should return a non-empty hash")
	}
}

func TestItemHashUint64_Deterministic(t *testing.T) {
	dynamics := []interface{}{"title", "content"}
	h1 := ItemHashUint64(dynamics)
	h2 := ItemHashUint64(dynamics)
	if h1 != h2 {
		t.Errorf("Hash should be deterministic, got: %d and %d", h1, h2)
	}
}

func TestItemHashUint64_NilVsEmptyString(t *testing.T) {
	// Per godoc: nil entries are SKIPPED; "" entries are INCLUDED. So
	// [nil, "x"] and ["", "x"] must produce DIFFERENT hashes — proves the
	// nil-vs-"" divergence promise (a transition from "field set to empty"
	// to "field omitted entirely" is detected as a content change).
	withNil := []interface{}{nil, "x"}
	withEmpty := []interface{}{"", "x"}
	if ItemHashUint64(withNil) == ItemHashUint64(withEmpty) {
		t.Error("[nil, \"x\"] and [\"\", \"x\"] should produce DIFFERENT hashes")
	}
}

func TestItemHashUint64_PositionMatters(t *testing.T) {
	h1 := ItemHashUint64([]interface{}{"a", "b"})
	h2 := ItemHashUint64([]interface{}{"b", "a"})
	if h1 == h2 {
		t.Error("Different positional orderings should produce different hashes")
	}
}

func TestItemHashUint64_NilEntriesSkipped(t *testing.T) {
	// nil is skipped but position is preserved by formatHashPart's index key.
	// [nil, "x"] formats as `1:"x"`; [nil, nil, "x"] formats as `2:"x"`.
	// Different position keys → different hashes.
	h1 := ItemHashUint64([]interface{}{nil, "x"})
	h2 := ItemHashUint64([]interface{}{nil, nil, "x"})
	if h1 == h2 {
		t.Error("Position of non-nil entry should affect hash even when prior entries are all nil")
	}
}

func TestItemHashUint64_CollisionStress_NoDupesIn10k(t *testing.T) {
	// Mirrors TestFingerprintStress_NoDuplicatesIn10kStructures pattern:
	// 10k synthetic dynamics with one varying field, all hashes must be unique.
	const count = 10000
	seen := make(map[uint64]int, count)
	for i := 0; i < count; i++ {
		h := ItemHashUint64([]interface{}{"item-" + strconv.Itoa(i), "x"})
		if prev, exists := seen[h]; exists {
			t.Fatalf("collision: input %d and input %d both hash to %d", prev, i, h)
		}
		seen[h] = i
	}
}

// TestItemHashUint64_NestedTreeNode locks in the load-bearing assumption that
// formatHashPart's json.Marshal correctly captures nested *TreeNode content.
// The stream-mode range diff (proposal §5b) hashes whole items via
// ItemHashUint64; if a TreeNode value were stringified as a pointer or struct
// header, content-equivalent items would hash differently and the algorithm
// would emit spurious updates on every render. Phase 1 backfill — was in the
// original Phase 1 plan but dropped during execution; advisor flagged it as
// load-bearing for Phase 2 correctness.
func TestItemHashUint64_NestedTreeNode(t *testing.T) {
	makeNested := func(value string) *build.TreeNode {
		return &build.TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: []interface{}{value},
		}
	}

	dyn1 := []interface{}{"id1", makeNested("inner")}
	dyn2 := []interface{}{"id1", makeNested("inner")}
	if ItemHashUint64(dyn1) != ItemHashUint64(dyn2) {
		t.Error("Two structurally-identical items with same nested content must hash equal")
	}

	dyn3 := []interface{}{"id1", makeNested("changed")}
	if ItemHashUint64(dyn1) == ItemHashUint64(dyn3) {
		t.Error("Items differing only in nested-TreeNode content must hash differently")
	}
}

// TestFormatHashPart_ByteEquivalentToJSON locks the wire-stable contract that
// formatHashPart's output is `{key}:{json.Marshal(val)}` for every supported
// type. The Phase 7 type-direct hash optimization (replacing reflective
// json.Marshal with strconv-based fast paths for common types) MUST preserve
// this byte-for-byte. Any divergence flips RangeStreamState.Hashes for that
// type, breaking stream-mode transition fingerprints across upgrades.
func TestFormatHashPart_ByteEquivalentToJSON(t *testing.T) {
	cases := []struct {
		name string
		val  interface{}
	}{
		{"empty_string", ""},
		{"ascii_string", "hello"},
		{"largetable_id", "row-00001"},
		{"largetable_name", "User 00001"},
		{"largetable_email", "user00001@example.com"},
		{"int_zero", 0},
		{"int_positive", 42},
		{"int_negative", -7},
		{"int_max", int(^uint(0) >> 1)},
		{"int64_positive", int64(1234567890123)},
		{"int32_positive", int32(99)},
		{"bool_true", true},
		{"bool_false", false},
		{"string_with_double_quote", `say "hi"`},
		{"string_with_backslash", `back\slash`},
		{"string_with_newline", "line1\nline2"},
		{"string_with_html_chars", `<a href="x">&amp;</a>`},
		{"string_with_unicode", "日本語"},
		{"string_with_tab", "col1\tcol2"},
		{"slice_fallback", []int{1, 2, 3}},
		{"map_fallback", map[string]int{"a": 1, "b": 2}},
		{"float64_simple", 3.14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatHashPart("k", tc.val)
			jsonBytes, err := json.Marshal(tc.val)
			if err != nil {
				t.Fatalf("baseline json.Marshal(%T) failed: %v", tc.val, err)
			}
			want := "k:" + string(jsonBytes)
			if got != want {
				t.Errorf("formatHashPart drift for %T(%v):\n  got  %q\n  want %q", tc.val, tc.val, got, want)
			}
		})
	}
}
