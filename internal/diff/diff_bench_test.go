package diff

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// Benchmark helpers

func createSimpleTree() *build.TreeNode {
	return &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]interface{}{"0": "value"},
	}
}

func createTreeWithNFields(n int) *build.TreeNode {
	tree := &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: make(map[string]interface{}),
	}
	for i := 0; i < n; i++ {
		key := string(rune('0' + i))
		tree.Dynamics[key] = "value"
	}
	return tree
}

func createRangeTree(itemCount int) *build.TreeNode {
	items := make([]interface{}, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = map[string]interface{}{
			"key": i,
			"tree": &build.TreeNode{
				Statics:  []string{"<li>", "</li>"},
				Dynamics: map[string]interface{}{"0": "item"},
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
		_ = CompareTreesAndGetChangesWithPath(tree, tree, false, "", nil, nil)
	}
}

func BenchmarkCompareTreesSmallChange(b *testing.B) {
	tree1 := createSimpleTree()
	tree2 := createSimpleTree()
	tree2.Dynamics["0"] = "changed"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CompareTreesAndGetChangesWithPath(tree1, tree2, false, "", nil, nil)
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
				key := string(rune('0' + i))
				tree2.Dynamics[key] = "changed"
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = CompareTreesAndGetChangesWithPath(tree1, tree2, false, "", nil, nil)
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
	newItemTree.Dynamics["0"] = "updated"

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
