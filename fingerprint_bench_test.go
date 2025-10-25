package livetemplate

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"testing"
)

// Old implementation using full JSON marshaling for comparison
func calculateFingerprintOld(tree treeNode) string {
	hasher := md5.New()

	// Add statics to hash (template structure)
	if statics, exists := tree["s"]; exists {
		if staticsArray, ok := statics.([]string); ok {
			staticsJSON, _ := json.Marshal(staticsArray)
			hasher.Write(staticsJSON)
		}
	}

	// Add dynamics to hash in sorted order for consistency
	var keys []string
	for k := range tree {
		if k != "s" && k != "f" {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		num1, err1 := strconv.Atoi(keys[i])
		num2, err2 := strconv.Atoi(keys[j])
		if err1 == nil && err2 == nil {
			return num1 < num2
		}
		return keys[i] < keys[j]
	})

	// Add dynamic values to hash (OLD: Full JSON marshal)
	for _, k := range keys {
		value := tree[k]
		valueJSON, _ := json.Marshal(value) // This is the bottleneck!
		hasher.Write([]byte(k))
		hasher.Write(valueJSON)
	}

	fullHash := hex.EncodeToString(hasher.Sum(nil))
	if len(fullHash) >= 16 {
		return fullHash[:16]
	}
	return fullHash
}

// Helper to create a tree with nested structure
func createNestedTree(depth, breadth int) treeNode {
	if depth == 0 {
		return treeNode{
			"s": []string{"<span>", "</span>"},
			"0": "leaf value",
		}
	}

	tree := treeNode{
		"s": []string{"<div>", "</div>"},
	}

	for i := 0; i < breadth; i++ {
		tree[strconv.Itoa(i)] = createNestedTree(depth-1, breadth)
	}

	return tree
}

// Helper to create a flat tree with many siblings
func createFlatTree(nodes int) treeNode {
	tree := treeNode{
		"s": []string{"<div>", "</div>"},
	}

	for i := 0; i < nodes; i++ {
		tree[strconv.Itoa(i)] = map[string]interface{}{
			"s": []string{"<p>", "</p>"},
			"0": "content " + strconv.Itoa(i),
		}
	}

	return tree
}

// Helper to create range-like structure
func createRangeTree(items int) treeNode {
	var rangeItems []interface{}
	for i := 0; i < items; i++ {
		rangeItems = append(rangeItems, map[string]interface{}{
			"1": "item-" + strconv.Itoa(i),
			"3": "Item description " + strconv.Itoa(i),
			"5": map[string]interface{}{
				"0": "Priority " + strconv.Itoa(i%3),
			},
		})
	}

	return treeNode{
		"s": []string{"<div>", "</div>"},
		"0": map[string]interface{}{
			"s": []string{"<ul>", "</ul>"},
			"d": rangeItems,
		},
	}
}

// Benchmarks: Small tree (10 nodes)
func BenchmarkFingerprint_Small_Old(b *testing.B) {
	tree := createFlatTree(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_Small_New(b *testing.B) {
	tree := createFlatTree(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}

// Benchmarks: Medium tree (100 nodes)
func BenchmarkFingerprint_Medium_Old(b *testing.B) {
	tree := createFlatTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_Medium_New(b *testing.B) {
	tree := createFlatTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}

// Benchmarks: Large tree (1000 nodes)
func BenchmarkFingerprint_Large_Old(b *testing.B) {
	tree := createFlatTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_Large_New(b *testing.B) {
	tree := createFlatTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}

// Benchmarks: Deep nested tree (depth=4, breadth=3)
func BenchmarkFingerprint_DeepNested_Old(b *testing.B) {
	tree := createNestedTree(4, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_DeepNested_New(b *testing.B) {
	tree := createNestedTree(4, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}

// Benchmarks: Range structure (100 items)
func BenchmarkFingerprint_Range100_Old(b *testing.B) {
	tree := createRangeTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_Range100_New(b *testing.B) {
	tree := createRangeTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}

// Benchmarks: Range structure (1000 items)
func BenchmarkFingerprint_Range1000_Old(b *testing.B) {
	tree := createRangeTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_Range1000_New(b *testing.B) {
	tree := createRangeTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}

// Test to verify determinism - same input produces same fingerprint
func TestFingerprint_Determinism(t *testing.T) {
	tests := []struct {
		name string
		tree treeNode
	}{
		{"small flat", createFlatTree(10)},
		{"medium flat", createFlatTree(100)},
		{"deep nested", createNestedTree(3, 3)},
		{"range 50", createRangeTree(50)},
		{
			"complex mixed",
			treeNode{
				"s": []string{"<div>", "</div>"},
				"0": "simple string",
				"1": 42,
				"2": map[string]interface{}{
					"s": []string{"<span>", "</span>"},
					"0": "nested",
				},
				"3": []interface{}{"array", "values", 123},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate fingerprint multiple times
			fp1 := calculateFingerprint(tt.tree)
			fp2 := calculateFingerprint(tt.tree)
			fp3 := calculateFingerprint(tt.tree)

			// All should be identical (deterministic)
			if fp1 != fp2 || fp2 != fp3 {
				t.Errorf("Fingerprints not deterministic!\nFP1: %s\nFP2: %s\nFP3: %s", fp1, fp2, fp3)
			}

			// Verify fingerprint is non-empty
			if fp1 == "" {
				t.Error("Fingerprint should not be empty")
			}
		})
	}
}

// Memory allocation benchmarks
func BenchmarkFingerprint_Allocations_Old(b *testing.B) {
	tree := createFlatTree(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

func BenchmarkFingerprint_Allocations_New(b *testing.B) {
	tree := createFlatTree(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprint(tree)
	}
}
