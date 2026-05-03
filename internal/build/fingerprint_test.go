package build

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"hash/fnv"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
		Dynamics: []interface{}{"hello"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"world"},
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
		Dynamics: []interface{}{"value"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: []interface{}{"value"},
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
		Dynamics: []interface{}{"a", "b"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "", "</div>"},
		Dynamics: []interface{}{"x"}, // Only one dynamic
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
		Dynamics: []interface{}{
			&TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: []interface{}{"value1"},
			},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: []interface{}{
			&TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: []interface{}{"different_value"},
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
		Dynamics: []interface{}{
			&TreeNode{
				Statics:  []string{"<span>", "</span>"},
				Dynamics: []interface{}{"value"},
			},
		},
	}

	tree2 := &TreeNode{
		Statics: []string{"<div>", "</div>"},
		Dynamics: []interface{}{
			&TreeNode{
				Statics:  []string{"<p>", "</p>"}, // Different nested statics
				Dynamics: []interface{}{"value"},
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
		Dynamics: []interface{}{
			"value1",
			&TreeNode{
				Statics:  []string{"<p>", "</p>"},
				Dynamics: []interface{}{"nested"},
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
		Dynamics: []interface{}{nil},
	}
	// Create circular reference
	tree.Dynamics[0] = tree
	tree.dynamicCount = 1

	// Should not panic and should produce valid fingerprint
	sfp := CalculateStructureFingerprint(tree)
	if sfp == "" {
		t.Error("Circular reference should still produce non-empty fingerprint")
	}
}

// =============================================================================
// Lexicographic Sorting Tests (10+ Keys)
// =============================================================================

// TestCalculateStructureFingerprint_ManyDynamics tests that
// fingerprinting with 15 dynamics is deterministic (slice iteration is inherently ordered).
func TestCalculateStructureFingerprint_ManyDynamics(t *testing.T) {
	// Create tree with 15 dynamic positions (0-14)
	dynamics := make([]interface{}, 15)
	for i := 0; i < 15; i++ {
		dynamics[i] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: []interface{}{"nested"},
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

// TestCalculateStructureFingerprint_SliceOrder validates that identically-constructed
// slice-based dynamics produce consistent fingerprints.
func TestCalculateStructureFingerprint_SliceOrder(t *testing.T) {
	// Two trees with the same dynamics in the same slice positions
	tree1 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"val0", "val1", "val2", nil, nil, nil, nil, nil, nil, "val9", "val10", "val11"},
	}

	tree2 := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"val0", "val1", "val2", nil, nil, nil, nil, nil, nil, "val9", "val10", "val11"},
	}

	fp1 := CalculateStructureFingerprint(tree1)
	fp2 := CalculateStructureFingerprint(tree2)

	if fp1 != fp2 {
		t.Error("Identically constructed trees should produce same fingerprint")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func createBenchTreeSmall() *TreeNode {
	return &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"value"},
	}
}

func createBenchTreeMedium() *TreeNode {
	dynamics := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		dynamics[i] = &TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: []interface{}{"nested"},
		}
	}
	return &TreeNode{
		Statics:  []string{"<div class=\"container\">", "</div>"},
		Dynamics: dynamics,
	}
}

func createBenchTreeLarge() *TreeNode {
	dynamics := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		nested := make([]interface{}, 5)
		for j := 0; j < 5; j++ {
			nested[j] = "value"
		}
		dynamics[i] = &TreeNode{
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
			Dynamics: []interface{}{"leaf"},
		}
	}
	return &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{createBenchTreeDeepNested(depth - 1)},
	}
}

func createBenchRangeTree(itemCount int) *TreeNode {
	items := make([]interface{}, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = &TreeNode{
			Statics:  []string{"<li>", "</li>"},
			Dynamics: []interface{}{"item"},
			AutoKey:  strconv.Itoa(i),
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

// visitPathPool reuses the cycle-detection map across benchmark iterations
// so allocs/op for fingerprintWith reflect the hashing work, not per-call
// map setup. Production fingerprinting (CalculateStructureFingerprint) does
// its own allocation; this pool exists only for the benchmark helper.
var visitPathPool = sync.Pool{
	New: func() interface{} { return make(map[*TreeNode]struct{}, 16) },
}

// fingerprintWith computes a fingerprint using the given hash function.
// Used for comparing MD5 (previous algorithm) vs FNV-1a 128 (current algorithm)
// in benchmarks; the resulting digest is discarded since only timing matters.
func fingerprintWith(tree *TreeNode, h hash.Hash) {
	visitPath := visitPathPool.Get().(map[*TreeNode]struct{})
	hashStructureWithCircularDetection(tree, h, visitPath)
	_ = hex.EncodeToString(h.Sum(nil))
	for k := range visitPath {
		delete(visitPath, k)
	}
	visitPathPool.Put(visitPath)
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
				fingerprintWith(tc.tree, md5.New())
			}
		})
		b.Run(tc.name+"/FNV1a128-current", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fingerprintWith(tc.tree, fnv.New128a())
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
	dynamics := make([]interface{}, 10000)
	for i := 0; i < 10000; i++ {
		dynamics[i] = "value"
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

// TestFingerprintStress_ConcurrentInvalidateAndRead pins the contract that
// concurrent readers and invalidators on the same TreeNode never produce a
// race, panic, or empty/torn fingerprint. Existing stress tests cover
// concurrent reads (TestFingerprintStress_ConcurrentCompute) and sequential
// invalidate-then-read (TestFingerprintStress_CacheCoherency); this one
// covers the harder case — invalidate and read interleaved across goroutines.
//
// Because the tree's static structure never changes during the test, every
// reader must observe the SAME fingerprint regardless of when an invalidate
// race fired: the cache may flip between empty and the recomputed value, but
// the recomputed value is deterministic from the structure.
//
// Run under `go test -race -run TestFingerprintStress_ConcurrentInvalidateAndRead`.
func TestFingerprintStress_ConcurrentInvalidateAndRead(t *testing.T) {
	tree := createBenchTreeMedium()
	expected := tree.GetStructureFingerprint()
	if expected == "" {
		t.Fatal("expected non-empty baseline fingerprint")
	}

	const (
		readers   = 50
		writers   = 50
		runFor    = 1 * time.Second
		spinSleep = 0
	)

	var (
		wg          sync.WaitGroup
		stop        = make(chan struct{})
		mismatchMu  sync.Mutex
		mismatches  []string
		readSamples atomic.Uint64
	)

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				got := tree.GetStructureFingerprint()
				if got != expected {
					mismatchMu.Lock()
					mismatches = append(mismatches, got)
					mismatchMu.Unlock()
				}
				readSamples.Add(1)
				if spinSleep > 0 {
					time.Sleep(spinSleep)
				}
			}
		}()
	}

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				tree.InvalidateStructureFingerprint()
				if spinSleep > 0 {
					time.Sleep(spinSleep)
				}
			}
		}()
	}

	time.Sleep(runFor)
	close(stop)
	wg.Wait()

	if readSamples.Load() == 0 {
		t.Fatal("readers logged no samples — test did not exercise the path")
	}
	if len(mismatches) > 0 {
		t.Fatalf("readers observed %d mismatching fingerprints; first few: %v",
			len(mismatches), mismatches[:min(5, len(mismatches))])
	}
}

func TestFingerprintStress_CacheCoherency(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"value"},
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
			Dynamics: []interface{}{
				"value",
				&TreeNode{
					Statics:  []string{"<p>", "</p>"},
					Dynamics: []interface{}{"nested"},
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

func TestFingerprintStress_NoDuplicatesIn10kStructures(t *testing.T) {
	seen := make(map[string]int)
	const count = 10000

	for i := 0; i < count; i++ {
		tree := &TreeNode{
			Statics:  []string{"<div class=\"c" + strconv.Itoa(i) + "\">", "</div>"},
			Dynamics: []interface{}{"value"},
		}
		fp := CalculateStructureFingerprint(tree)
		if prev, exists := seen[fp]; exists {
			t.Fatalf("collision: tree %d and tree %d both have fingerprint %q", prev, i, fp)
		}
		seen[fp] = i
	}
}

// TestCalculateStaticsFingerprint_ScalarValuesIgnored is the key contract:
// outer Statics + nested-tree Statics determine the hash; scalar dynamic
// values (including nil-vs-non-nil presence) do NOT. This is what makes
// the stream-mode het-range check work for the proposal §5b nil↔""
// phantom-update case — items differ in scalar content, same template.
func TestCalculateStaticsFingerprint_ScalarValuesIgnored(t *testing.T) {
	statics := []string{"<li>", "</li>"}
	tree1 := &TreeNode{Statics: statics, Dynamics: []interface{}{"x"}}
	tree2 := &TreeNode{Statics: statics, Dynamics: []interface{}{"y"}}
	tree3 := &TreeNode{Statics: statics, Dynamics: []interface{}{nil}}
	tree4 := &TreeNode{Statics: statics, Dynamics: []interface{}{""}}

	fp1 := CalculateStaticsFingerprint(tree1)
	for _, other := range []*TreeNode{tree2, tree3, tree4} {
		if CalculateStaticsFingerprint(other) != fp1 {
			t.Errorf("Same Statics + scalar-only Dynamics must hash equal regardless of value; tree1=%q vs other=%q",
				fp1, CalculateStaticsFingerprint(other))
		}
	}
}

// TestCalculateStaticsFingerprint_NestedTreeVsScalarDiffers is the het-range
// trigger: a position holding a nested *TreeNode is structurally distinct
// from the same position holding a scalar (or nil). This is the conditional
// branch case (§5d) — once an item swaps a scalar for a subtree, the range
// is no longer homogeneous.
func TestCalculateStaticsFingerprint_NestedTreeVsScalarDiffers(t *testing.T) {
	statics := []string{"<li>", "</li>"}
	scalar := &TreeNode{Statics: statics, Dynamics: []interface{}{"x"}}
	nested := &TreeNode{
		Statics: statics,
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"<span>", "</span>"}, Dynamics: []interface{}{"x"}},
		},
	}
	if CalculateStaticsFingerprint(scalar) == CalculateStaticsFingerprint(nested) {
		t.Error("Position holding scalar vs nested *TreeNode must produce different fingerprints")
	}
}

// TestCalculateStaticsFingerprint_NestedTreeStaticsCaptured: two items with
// the same outer Statics but different INNER (nested-tree) Statics must
// fingerprint differently — the recursion captures nested template shape.
func TestCalculateStaticsFingerprint_NestedTreeStaticsCaptured(t *testing.T) {
	outer := []string{"<li>", "</li>"}
	tree1 := &TreeNode{
		Statics: outer,
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"<a>", "</a>"}, Dynamics: []interface{}{"x"}},
		},
	}
	tree2 := &TreeNode{
		Statics: outer,
		Dynamics: []interface{}{
			&TreeNode{Statics: []string{"<b>", "</b>"}, Dynamics: []interface{}{"x"}},
		},
	}
	if CalculateStaticsFingerprint(tree1) == CalculateStaticsFingerprint(tree2) {
		t.Error("Different nested-tree Statics must produce different outer fingerprints")
	}
}

// TestCalculateStaticsFingerprint_RangeStaticsCaptured covers Range.Statics
// (the per-item template), which mirrors the existing
// CalculateStructureFingerprint test for ranges.
func TestCalculateStaticsFingerprint_RangeStaticsCaptured(t *testing.T) {
	tree1 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range:   &RangeData{Statics: []string{"<li>", "</li>"}},
	}
	tree2 := &TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range:   &RangeData{Statics: []string{"<div>", "</div>"}},
	}
	if CalculateStaticsFingerprint(tree1) == CalculateStaticsFingerprint(tree2) {
		t.Error("Different Range.Statics must produce different fingerprints")
	}
}

// TestCalculateStaticsFingerprint_NilTree: nil → empty string (sentinel).
func TestCalculateStaticsFingerprint_NilTree(t *testing.T) {
	if CalculateStaticsFingerprint(nil) != "" {
		t.Errorf("nil tree should produce empty string, got %q", CalculateStaticsFingerprint(nil))
	}
}

// TestCalculateStaticsFingerprint_Deterministic confirms the function is a
// pure hash — same input always produces the same output.
func TestCalculateStaticsFingerprint_Deterministic(t *testing.T) {
	tree := &TreeNode{
		Statics:  []string{"<li>", "</li>"},
		Dynamics: []interface{}{"x"},
	}
	first := CalculateStaticsFingerprint(tree)
	second := CalculateStaticsFingerprint(tree)
	if first != second {
		t.Errorf("Same tree must hash equal across calls; got %q then %q", first, second)
	}
}
