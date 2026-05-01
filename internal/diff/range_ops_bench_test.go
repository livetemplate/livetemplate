package diff

import (
	"encoding/json"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// Stream-mode benchmarks per proposal §13 Phase 5.
//
// Fixtures: build a TreeNode range tree the same way createTreeNodeRangeTree
// does, then call TransitionToStreamMode to populate the StreamState snapshot
// and drop Items. The benchmark loop calls GenerateRangeStreamOperations with
// the retained snapshot vs a fresh new-side tree.

// buildStreamModeOldTree returns (streamState, statics) for an N-item range
// that has already transitioned to stream mode. Panics if TransitionToStreamMode
// silently no-ops (e.g., fixture became heterogeneous) — without the check,
// benchmarks would measure the het-range fallback path instead of stream mode.
func buildStreamModeOldTree(b *testing.B, itemCount int) (*build.RangeStreamState, []string) {
	b.Helper()
	tree := createTreeNodeRangeTree(itemCount)
	TransitionToStreamMode(tree)
	if tree.Range.StreamState == nil {
		b.Fatalf("TransitionToStreamMode left StreamState nil at N=%d — fixture lost homogeneity?", itemCount)
	}
	return tree.Range.StreamState, tree.Range.Statics
}

func BenchmarkRangeDiff_Stream_Append_Small(b *testing.B) {
	benchmarkStreamAppend(b, 10, 1)
}

func BenchmarkRangeDiff_Stream_Append_Medium(b *testing.B) {
	benchmarkStreamAppend(b, 100, 1)
}

func BenchmarkRangeDiff_Stream_Append_Large(b *testing.B) {
	benchmarkStreamAppend(b, 10000, 1)
}

func BenchmarkRangeDiff_Stream_Update_Small(b *testing.B) {
	benchmarkStreamUpdate(b, 10)
}

func BenchmarkRangeDiff_Stream_Update_Medium(b *testing.B) {
	benchmarkStreamUpdate(b, 100)
}

func BenchmarkRangeDiff_Stream_Update_Large(b *testing.B) {
	benchmarkStreamUpdate(b, 10000)
}

func BenchmarkRangeDiff_Stream_Reorder_Small(b *testing.B) {
	benchmarkStreamReorder(b, 10)
}

func BenchmarkRangeDiff_Stream_Reorder_Medium(b *testing.B) {
	benchmarkStreamReorder(b, 100)
}

func BenchmarkRangeDiff_Stream_Reorder_Large(b *testing.B) {
	benchmarkStreamReorder(b, 10000)
}

// fixtureStatusPos is the dynamic position of the "status" field in
// createTreeNodeRangeTree's `[key, name, status]` shape. Centralised so the
// update benchmarks don't silently mutate the wrong position if the fixture
// shape changes.
const fixtureStatusPos = 2

func benchmarkStreamAppend(b *testing.B, oldCount, addCount int) {
	streamState, statics := buildStreamModeOldTree(b, oldCount)
	newTree := createTreeNodeRangeTree(oldCount + addCount)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeStreamOperations(streamState, newTree.Range.Items, statics, nil, false)
	}
}

func benchmarkStreamUpdate(b *testing.B, itemCount int) {
	streamState, statics := buildStreamModeOldTree(b, itemCount)
	newTree := createTreeNodeRangeTree(itemCount)
	mid := itemCount / 2
	newTree.Range.Items[mid].(*build.TreeNode).Dynamics[fixtureStatusPos] = "updated"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeStreamOperations(streamState, newTree.Range.Items, statics, nil, false)
	}
}

func benchmarkStreamReorder(b *testing.B, itemCount int) {
	streamState, statics := buildStreamModeOldTree(b, itemCount)
	newTree := createTreeNodeRangeTree(itemCount)
	newTree.Range.Items[0], newTree.Range.Items[itemCount-1] = newTree.Range.Items[itemCount-1], newTree.Range.Items[0]

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRangeStreamOperations(streamState, newTree.Range.Items, statics, nil, false)
	}
}

// Wire-size benchmarks: emit per-op JSON byte counts for the §7 table.
// "bytes/op" here is wire bytes per emitted op, not allocator bytes.

func BenchmarkRangeDiff_Stream_Append_WireSize_Small(b *testing.B) {
	benchmarkStreamAppendWireSize(b, 10)
}

func BenchmarkRangeDiff_Stream_Append_WireSize_Medium(b *testing.B) {
	benchmarkStreamAppendWireSize(b, 100)
}

func BenchmarkRangeDiff_Stream_Append_WireSize_Large(b *testing.B) {
	benchmarkStreamAppendWireSize(b, 10000)
}

func BenchmarkRangeDiff_Stream_Update_WireSize_Small(b *testing.B) {
	benchmarkStreamUpdateWireSize(b, 10)
}

func BenchmarkRangeDiff_Stream_Update_WireSize_Medium(b *testing.B) {
	benchmarkStreamUpdateWireSize(b, 100)
}

func BenchmarkRangeDiff_Stream_Update_WireSize_Large(b *testing.B) {
	benchmarkStreamUpdateWireSize(b, 10000)
}

func benchmarkStreamAppendWireSize(b *testing.B, oldCount int) {
	streamState, statics := buildStreamModeOldTree(b, oldCount)
	newTree := createTreeNodeRangeTree(oldCount + 1)

	var totalSize int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := GenerateRangeStreamOperations(streamState, newTree.Range.Items, statics, nil, true)
		if ops == nil {
			b.Fatalf("GenerateRangeStreamOperations returned nil — stream-mode dispatch failed")
		}
		data, err := json.Marshal(ops)
		if err != nil {
			b.Fatalf("json.Marshal failed: %v", err)
		}
		totalSize += int64(len(data))
	}
	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}

func benchmarkStreamUpdateWireSize(b *testing.B, itemCount int) {
	streamState, statics := buildStreamModeOldTree(b, itemCount)
	newTree := createTreeNodeRangeTree(itemCount)
	mid := itemCount / 2
	newTree.Range.Items[mid].(*build.TreeNode).Dynamics[fixtureStatusPos] = "updated"

	var totalSize int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := GenerateRangeStreamOperations(streamState, newTree.Range.Items, statics, nil, true)
		if ops == nil {
			b.Fatalf("GenerateRangeStreamOperations returned nil — stream-mode dispatch failed")
		}
		data, err := json.Marshal(ops)
		if err != nil {
			b.Fatalf("json.Marshal failed: %v", err)
		}
		totalSize += int64(len(data))
	}
	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}

// BenchmarkRangeDiff_LegacyPartialDelta_Update_WireSize synthesises the
// pre-Phase-4 partial-delta `["u", key, {pos: val}]` shape for the §7 wire-size
// comparison column. The legacy producer was deleted in Phase 4, so this op is
// constructed directly rather than emitted from the diff path. The hardcoded
// "item-50" key matches the mid-list update benchmarks above so the synthetic
// baseline is comparable byte-for-byte.
func BenchmarkRangeDiff_LegacyPartialDelta_Update_WireSize(b *testing.B) {
	op := []interface{}{
		"u",
		"item-50",
		map[string]interface{}{"2": "updated"},
	}

	var totalSize int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal([]interface{}{op})
		if err != nil {
			b.Fatalf("json.Marshal failed: %v", err)
		}
		totalSize += int64(len(data))
	}
	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}
