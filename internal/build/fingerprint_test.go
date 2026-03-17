package build

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"hash/fnv"
	"strconv"
	"sync"
	"testing"
)

// =============================================================================
// CalculateStructureFingerprint Tests
// =============================================================================

// TestCalculateStructureFingerprint_SameStaticsDifferentDynamics tests that
// trees with the same statics but different dynamic values produce the SAME
// structure fingerprint.
func TestCalculateStructureFingerprint_SameStaticsDifferentDynamics(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "hello"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "world"},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 != sfp2 {
		t.Errorf("Same statics with different dynamics should produce same structure fingerprint.\nTree1: %s\nTree2: %s", sfp1, sfp2)
	}
}

// TestCalculateStructureFingerprint_DifferentStatics tests that trees with
// different statics produce different structure fingerprints.
func TestCalculateStructureFingerprint_DifferentStatics(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Different statics should produce different structure fingerprints")
	}
}

// TestCalculateStructureFingerprint_DifferentDynamicPositions tests that
// trees with different dynamic positions produce different structure fingerprints.
func TestCalculateStructureFingerprint_DifferentDynamicPositions(t *testing.T) {
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "", "</div>"},
		Dynamics: map[string]interface{}{"0": "a", "1": "b"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "", "</div>"},
		Dynamics: map[string]interface{}{"0": "x"}, // Only one dynamic
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Different dynamic positions should produce different structure fingerprints")
	}
}

// TestCalculateStructureFingerprint_NestedTreesSameStructure tests that nested
// trees with the same structure produce the same structure fingerprint.
func TestCalculateStructureFingerprint_NestedTreesSameStructure(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "value1"},
			},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "different_value"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 != sfp2 {
		t.Errorf("Nested trees with same structure should produce same fingerprint.\nTree1: %s\nTree2: %s", sfp1, sfp2)
	}
}

// TestCalculateStructureFingerprint_NestedTreesDifferentStructure tests that nested
// trees with different structures produce different structure fingerprints.
func TestCalculateStructureFingerprint_NestedTreesDifferentStructure(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: map[string]interface{}{"0": "value"},
			},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": &TreeNode{
				Statics:  []string{"<p>", "</p>"}, // Different nested statics
				Dynamics: map[string]interface{}{"0": "value"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Nested trees with different structures should produce different fingerprints")
	}
}

// TestCalculateStructureFingerprint_WithRange tests that range statics are
// included in the structure fingerprint.
func TestCalculateStructureFingerprint_WithRange(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li>", "</li>"},
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li class=\"new\">", "</li>"}, // Different range statics
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 == sfp2 {
		t.Error("Different range statics should produce different structure fingerprints")
	}
}

// TestCalculateStructureFingerprint_RangeSameStaticsDifferentItems tests that
// range with same statics but different items produces same structure fingerprint.
func TestCalculateStructureFingerprint_RangeSameStaticsDifferentItems(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li>", "</li>"},
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Statics: []string{"<li>", "</li>"},
			Items: []interface{}{
				map[string]interface{}{"0": "different1"},
				map[string]interface{}{"0": "different2"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree1)
	sfp2 := CalculateStructureFingerprint(tree2)

	if sfp1 != sfp2 {
		t.Errorf("Range with same statics but different items should produce same structure fingerprint.\nTree1: %s\nTree2: %s", sfp1, sfp2)
	}
}

// TestCalculateStructureFingerprint_Deterministic tests that the function is deterministic.
func TestCalculateStructureFingerprint_Deterministic(t *testing.T) {
	tree := &TreeNode{
		Statics: []string{"<div>", "<span>", "</span>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": "value1",
			"1": &TreeNode{
				Statics:  []string{"<p>", "</p>"},
				Dynamics: map[string]interface{}{"0": "nested"},
			},
		},
	}

	sfp1 := CalculateStructureFingerprint(tree)
	sfp2 := CalculateStructureFingerprint(tree)
	sfp3 := CalculateStructureFingerprint(tree)

	if sfp1 != sfp2 || sfp2 != sfp3 {
		t.Errorf("Structure fingerprint should be deterministic.\nFP1: %s\nFP2: %s\nFP3: %s", sfp1, sfp2, sfp3)
	}
}

// TestCalculateStructureFingerprint_NilTree tests that nil tree returns empty string.
func TestCalculateStructureFingerprint_NilTree(t *testing.T) {
	sfp := CalculateStructureFingerprint(nil)
	if sfp != "" {
		t.Errorf("Nil tree should return empty string, got: %s", sfp)
	}
}

// TestCalculateStructureFingerprint_EmptyTree tests that empty tree produces valid fingerprint.
func TestCalculateStructureFingerprint_EmptyTree(t *testing.T) {
	tree := &TreeNode{}
	sfp := CalculateStructureFingerprint(tree)
	if sfp == "" {
		t.Error("Empty tree should produce non-empty fingerprint")
	}
}

// TestCalculateStructureFingerprint_CircularReference tests circular reference handling.
func TestCalculateStructureFingerprint_CircularReference(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}
	// Create circular reference
	tree.Dynamics["0"] = tree

	// Should not panic and should produce valid fingerprint
	sfp := CalculateStructureFingerprint(tree)
	if sfp == "" {
		t.Error("Circular reference should still produce non-empty fingerprint")
	}
}

// =============================================================================
// Lexicographic Sorting Tests (10+ Keys)
// =============================================================================

// TestCalculateStructureFingerprint_LexicographicSorting10Keys tests that
// lexicographic sorting handles 10+ keys correctly.
// Issue: Keys "0"-"9" sort correctly, but "10", "11" may sort before "2" lexicographically.
// This tests that the fingerprint is deterministic regardless of map iteration order.
func TestCalculateStructureFingerprint_LexicographicSorting10Keys(t *testing.T) {
	// Create tree with 15 dynamic keys (0-14)
	dynamics := make(map[string]interface{})
	for i := 0; i < 15; i++ {
		key := string(rune('0' + i))
		if i >= 10 {
			key = "1" + string(rune('0'+i-10))
		}
		dynamics[key] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: map[string]interface{}{"0": "nested"},
		}
	}

	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: dynamics,
	}

	// Calculate fingerprint multiple times to verify determinism
	fingerprints := make([]string, 10)
	for i := 0; i < 10; i++ {
		fingerprints[i] = CalculateStructureFingerprint(tree)
	}

	// All fingerprints should be identical
	for i := 1; i < 10; i++ {
		if fingerprints[i] != fingerprints[0] {
			t.Errorf("Fingerprint %d differs from fingerprint 0: %s vs %s", i, fingerprints[i], fingerprints[0])
		}
	}
}

// TestCalculateStructureFingerprint_LexicographicOrder validates correct ordering.
// Keys: "0", "1", "10", "11", "2", "3"... should sort consistently.
func TestCalculateStructureFingerprint_LexicographicOrder(t *testing.T) {
	// Tree with keys that lexicographically sort differently than numerically
	tree1 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0":  "val0",
			"1":  "val1",
			"10": "val10",
			"11": "val11",
			"2":  "val2",
			"9":  "val9",
		},
	}

	// Same tree, constructed in different order (should produce same fingerprint)
	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"9":  "val9",
			"2":  "val2",
			"11": "val11",
			"10": "val10",
			"1":  "val1",
			"0":  "val0",
		},
	}

	fp1 := CalculateStructureFingerprint(tree1)
	fp2 := CalculateStructureFingerprint(tree2)

	if fp1 != fp2 {
		t.Error("Same tree with different construction order should produce same fingerprint")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func createBenchTreeSmall() *TreeNode {
	return &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}
}

func createBenchTreeMedium() *TreeNode {
	dynamics := make(map[string]interface{})
	for i := 0; i < 20; i++ {
		dynamics[string(rune('a'+i))] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: map[string]interface{}{"0": "nested"},
		}
	}
	return &TreeNode{
		Statics:  []string{"<div class=\"container\">", "</div>"},
		Dynamics: dynamics,
	}
}

func createBenchTreeLarge() *TreeNode {
	dynamics := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		nested := make(map[string]interface{})
		for j := 0; j < 5; j++ {
			nested[string(rune('0'+j))] = "value"
		}
		dynamics[string(rune('a'+i%26))+string(rune('0'+i/26))] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: nested,
		}
	}
	return &TreeNode{
		Statics:  []string{"<div class=\"large-container\">", "</div>"},
		Dynamics: dynamics,
	}
}

func createBenchTreeDeepNested(depth int) *TreeNode {
	if depth == 0 {
		return &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: map[string]interface{}{"0": "leaf"},
		}
	}
	return &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{
			"0": createBenchTreeDeepNested(depth - 1),
		},
	}
}

func createBenchRangeTree(itemCount int) *TreeNode {
	items := make([]interface{}, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = &TreeNode{
			Statics:  []string{"<li>", "</li>"},
			Dynamics: map[string]interface{}{"0": "item", "_k": i},
		}
	}
	return &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &RangeData{
			Items:   items,
			Statics: []string{"<li>", "</li>"},
		},
	}
}

func BenchmarkCalculateStructureFingerprint_Small(b *testing.B) {
	tree := createBenchTreeSmall()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

func BenchmarkCalculateStructureFingerprint_Medium(b *testing.B) {
	tree := createBenchTreeMedium()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

func BenchmarkCalculateStructureFingerprint_Large(b *testing.B) {
	tree := createBenchTreeLarge()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

func BenchmarkCalculateStructureFingerprint_DeepNested(b *testing.B) {
	tree := createBenchTreeDeepNested(20)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

func BenchmarkCalculateStructureFingerprint_Range100(b *testing.B) {
	tree := createBenchRangeTree(100)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

func BenchmarkCalculateStructureFingerprint_Range1000(b *testing.B) {
	tree := createBenchRangeTree(1000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CalculateStructureFingerprint(tree)
	}
}

// =============================================================================
// Algorithm Comparison Benchmarks (#90)
// =============================================================================

// fingerprintWith computes a fingerprint using the given hash function.
// Used for comparing MD5 (previous algorithm) vs FNV-1a 128 (current algorithm).
func fingerprintWith(tree *TreeNode, h hash.Hash) string {
	visitPath := make(map[*TreeNode]struct{})
	hashStructureWithCircularDetection(tree, h, visitPath)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func BenchmarkFingerprintAlgorithms(b *testing.B) {
	trees := []struct {
		name string
		tree *TreeNode
	}{
		{"small", createBenchTreeSmall()},
		{"medium", createBenchTreeMedium()},
		{"large", createBenchTreeLarge()},
		{"deep-20", createBenchTreeDeepNested(20)},
	}

	for _, tc := range trees {
		b.Run(tc.name+"/MD5-previous", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = fingerprintWith(tc.tree, md5.New())
			}
		})
		b.Run(tc.name+"/FNV1a128-current", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = fingerprintWith(tc.tree, fnv.New128a())
			}
		})
	}
}

// =============================================================================
// Stress Tests (#93)
// =============================================================================

func TestFingerprintStress_DeepNesting(t *testing.T) {
	tree := createBenchTreeDeepNested(100)
	fp := CalculateStructureFingerprint(tree)
	if fp == "" {
		t.Error("expected non-empty fingerprint for 100-level deep tree")
	}
}

func TestFingerprintStress_WideTree(t *testing.T) {
	dynamics := make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		dynamics[strconv.Itoa(i)] = "value"
	}
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: dynamics,
	}
	fp := CalculateStructureFingerprint(tree)
	if fp == "" {
		t.Error("expected non-empty fingerprint for 10k-wide tree")
	}
}

func TestFingerprintStress_ConcurrentCompute(t *testing.T) {
	// Do NOT pre-populate cache — let goroutines race to compute.
	// All writers produce the same value, so this tests that concurrent
	// compute-and-cache is safe (benign race on identical values).
	tree := createBenchTreeMedium()

	var wg sync.WaitGroup
	results := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = tree.GetStructureFingerprint()
		}(i)
	}
	wg.Wait()

	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("goroutine %d got %q, expected %q", i, results[i], results[0])
		}
	}
}

func TestFingerprintStress_CacheCoherency(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}

	fp1 := tree.GetStructureFingerprint()
	if fp1 == "" {
		t.Fatal("expected non-empty fingerprint")
	}

	tree.Statics = []string{"<span>", "</span>"}
	tree.InvalidateStructureFingerprint()

	fp2 := tree.GetStructureFingerprint()
	if fp2 == "" {
		t.Fatal("expected non-empty fingerprint after invalidation")
	}
	if fp1 == fp2 {
		t.Error("fingerprint should change after structural modification")
	}
}

func TestFingerprintStress_IndependentTreeEquivalence(t *testing.T) {
	buildTree := func() *TreeNode {
		return &TreeNode{
			Statics: []string{"<div class=\"test\">", "<span>", "</span>", "</div>"},
			Dynamics: map[string]interface{}{
				"0": "value",
				"1": &TreeNode{
					Statics:  []string{"<p>", "</p>"},
					Dynamics: map[string]interface{}{"0": "nested"},
				},
			},
		}
	}

	fp1 := CalculateStructureFingerprint(buildTree())
	fp2 := CalculateStructureFingerprint(buildTree())

	if fp1 != fp2 {
		t.Errorf("independently built identical trees should have same fingerprint: %q != %q", fp1, fp2)
	}
}

func TestFingerprintStress_CollisionResistance(t *testing.T) {
	seen := make(map[string]int)
	const count = 10000

	for i := 0; i < count; i++ {
		tree := &TreeNode{
			Statics: []string{"<div class=\"c" + strconv.Itoa(i) + "\">", "</div>"},
			Dynamics: map[string]interface{}{
				"0": "value",
			},
		}
		fp := CalculateStructureFingerprint(tree)
		if prev, exists := seen[fp]; exists {
			t.Fatalf("collision: tree %d and tree %d both have fingerprint %q", prev, i, fp)
		}
		seen[fp] = i
	}
}
