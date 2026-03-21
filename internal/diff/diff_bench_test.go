package diff

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// Benchmark helpers

func createSimpleTree() *build.TreeNode {
	return &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"value"},
	}
}

func createTreeWithNFields(n int) *build.TreeNode {
	dynamics := make([]interface{}, n)
	for i := range dynamics {
		dynamics[i] = "value"
	}
	return &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: dynamics,
	}
}

func createRangeTree(itemCount int) *build.TreeNode {
	items := make([]interface{}, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = map[string]interface{}{
			"key": i,
			"tree": &build.TreeNode{
				Statics:  []string{"<li>", "</li>"},
				Dynamics: []interface{}{"item"},
			},
		}
	}
	return &build.TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &build.RangeData{
			Items:   items,
			Statics: []string{"<li>", "</li>"},
		},
		Metadata: &build.TreeMetadata{
			IDKey: "key",
		},
	}
}

// Comparison operations

func BenchmarkCompareTreesNoChanges(b *testing.B) {
	tree := createTreeWithNFields(10)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CompareTreesAndGetChangesWithPath(tree, tree, false, "", nil)
	}
}

func BenchmarkCompareTreesSmallChange(b *testing.B) {
	tree1 := createSimpleTree()
	tree2 := createSimpleTree()
	tree2.Dynamics[0] = "changed"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CompareTreesAndGetChangesWithPath(tree1, tree2, false, "", nil)
	}
}

func BenchmarkCompareTreesLargeChange(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"10", 10},
		{"100", 100},
		{"1000", 1000},
	}

	for _, tt := range sizes {
		b.Run(tt.name, func(b *testing.B) {
			tree1 := createTreeWithNFields(tt.size)
			tree2 := createTreeWithNFields(tt.size)
			// Change every field
			for i := 0; i < tt.size; i++ {
				tree2.SetDynamic(i, "changed")
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = CompareTreesAndGetChangesWithPath(tree1, tree2, false, "", nil)
			}
		})
	}
}

// Range differential operations

func BenchmarkRangeDiffUpdate(b *testing.B) {
	oldTree := createRangeTree(100)
	newTree := createRangeTree(100)
	// Change one item
	newItem := newTree.Range.Items[50].(map[string]interface{})
	newItemTree := newItem["tree"].(*build.TreeNode)
	newItemTree.Dynamics[0] = "updated"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeDifferentialOperations(oldTree, newTree, false)
	}
}

func BenchmarkRangeDiffInsert(b *testing.B) {
	oldTree := createRangeTree(100)
	newTree := createRangeTree(101)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeDifferentialOperations(oldTree, newTree, false)
	}
}

func BenchmarkRangeDiffRemove(b *testing.B) {
	oldTree := createRangeTree(100)
	newTree := createRangeTree(99)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeDifferentialOperations(oldTree, newTree, false)
	}
}

// =============================================================================
// TreeNode-based Range Benchmarks (realistic production items)
// =============================================================================

// createTreeNodeRangeTree creates a range tree where items are *TreeNode
// with explicit data-key attributes, matching production behavior.
func createTreeNodeRangeTree(itemCount int) *build.TreeNode {
	statics := []string{`<li data-key="`, `">`, ` - `, `</li>`}
	items := make([]interface{}, itemCount)
	for i := 0; i < itemCount; i++ {
		key := "item-" + strconv.Itoa(i)
		items[i] = &build.TreeNode{
			Dynamics: []interface{}{key, "Item " + strconv.Itoa(i), "active"},
		}
	}
	return &build.TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &build.RangeData{
			Items:   items,
			Statics: statics,
		},
	}
}

func BenchmarkRangeDiff_TreeNode_Update(b *testing.B) {
	oldTree := createTreeNodeRangeTree(100)
	newTree := createTreeNodeRangeTree(100)
	newTree.Range.Items[50].(*build.TreeNode).Dynamics[2] = "updated"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeDifferentialOperations(oldTree, newTree, false)
	}
}

func BenchmarkRangeDiff_TreeNode_Reorder(b *testing.B) {
	oldTree := createTreeNodeRangeTree(100)
	newTree := createTreeNodeRangeTree(100)
	// Swap first and last items
	newTree.Range.Items[0], newTree.Range.Items[99] = newTree.Range.Items[99], newTree.Range.Items[0]

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeDifferentialOperations(oldTree, newTree, false)
	}
}

func BenchmarkRangeDiff_TreeNode_LargeList(b *testing.B) {
	oldTree := createTreeNodeRangeTree(1000)
	newTree := createTreeNodeRangeTree(1000)
	newTree.Range.Items[500].(*build.TreeNode).Dynamics[2] = "updated"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeDifferentialOperations(oldTree, newTree, false)
	}
}

// Client preparation

func BenchmarkPrepareTreeForClient(b *testing.B) {
	tree := createTreeWithNFields(100)

	b.Run("with-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = PrepareTreeForClient(tree, false)
		}
	})

	b.Run("without-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = PrepareTreeForClient(tree, true)
		}
	})
}

// =============================================================================
// ClientNeedsStatics Benchmarks (Fingerprint-based approach)
// =============================================================================

func createNestedTree(depth int) *build.TreeNode {
	if depth == 0 {
		return &build.TreeNode{
			Statics:  []string{"<span>", "</span>"},
			Dynamics: []interface{}{"leaf"},
		}
	}
	return &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{createNestedTree(depth - 1)},
	}
}

// BenchmarkClientNeedsStatics_SameStructure benchmarks comparison when structures are identical.
func BenchmarkClientNeedsStatics_SameStructure(b *testing.B) {
	tree1 := createTreeWithNFields(50)
	tree2 := createTreeWithNFields(50)
	// Same structure, different dynamics
	for k := range tree2.Dynamics {
		tree2.Dynamics[k] = "different"
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ClientNeedsStatics(tree1, tree2)
	}
}

// BenchmarkClientNeedsStatics_DifferentStructure benchmarks comparison when structures differ.
func BenchmarkClientNeedsStatics_DifferentStructure(b *testing.B) {
	tree1 := &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: []interface{}{"value"},
	}
	tree2 := &build.TreeNode{
		Statics:  []string{"<span>", "</span>"}, // Different statics
		Dynamics: []interface{}{"value"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ClientNeedsStatics(tree1, tree2)
	}
}

// BenchmarkClientNeedsStatics_DeepNested benchmarks deeply nested tree comparison.
func BenchmarkClientNeedsStatics_DeepNested(b *testing.B) {
	tree1 := createNestedTree(15)
	tree2 := createNestedTree(15)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ClientNeedsStatics(tree1, tree2)
	}
}

// BenchmarkClientNeedsStatics_Range benchmarks range tree comparison.
func BenchmarkClientNeedsStatics_Range(b *testing.B) {
	tree1 := createRangeTree(100)
	tree2 := createRangeTree(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ClientNeedsStatics(tree1, tree2)
	}
}

// BenchmarkClientNeedsStatics_NilOld benchmarks first render case.
func BenchmarkClientNeedsStatics_NilOld(b *testing.B) {
	tree := createTreeWithNFields(50)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ClientNeedsStatics(nil, tree)
	}
}

// =============================================================================
// Wire Size Comparison Benchmarks
// =============================================================================

// BenchmarkWireSize_WithStatics measures wire size with statics included.
func BenchmarkWireSize_WithStatics(b *testing.B) {
	tree := createTreeWithNFields(20)

	var totalSize int64
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prepared := PrepareTreeForClient(tree, false) // Include statics
		data, _ := json.Marshal(prepared)
		totalSize += int64(len(data))
	}
	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}

// BenchmarkWireSize_WithoutStatics measures wire size without statics.
func BenchmarkWireSize_WithoutStatics(b *testing.B) {
	tree := createTreeWithNFields(20)

	var totalSize int64
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prepared := PrepareTreeForClient(tree, true) // Strip statics
		data, _ := json.Marshal(prepared)
		totalSize += int64(len(data))
	}
	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}

// BenchmarkWireSizeComparison compares wire sizes for different scenarios.
func BenchmarkWireSizeComparison(b *testing.B) {
	sizes := []struct {
		name   string
		fields int
	}{
		{"small_5", 5},
		{"medium_20", 20},
		{"large_100", 100},
	}

	for _, s := range sizes {
		tree := createTreeWithNFields(s.fields)

		b.Run(s.name+"_with_statics", func(b *testing.B) {
			var totalSize int64
			for i := 0; i < b.N; i++ {
				prepared := PrepareTreeForClient(tree, false)
				data, _ := json.Marshal(prepared)
				totalSize += int64(len(data))
			}
			b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
		})

		b.Run(s.name+"_without_statics", func(b *testing.B) {
			var totalSize int64
			for i := 0; i < b.N; i++ {
				prepared := PrepareTreeForClient(tree, true)
				data, _ := json.Marshal(prepared)
				totalSize += int64(len(data))
			}
			b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
		})
	}
}
