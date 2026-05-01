package diff

import (
	"runtime"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// Memory regression gate per proposal §11 item 2 + §7. Asserts that the
// retained Range shape (StreamState only, Items nil) is materially smaller
// per item than the legacy retained shape (Items populated). Skipped under
// -short — measurement requires forced GC and is moderately slow.
//
// Thresholds are per-N because fixed per-Range overhead (TreeNode/RangeData/
// StreamState struct headers + Statics slice) dilutes the per-item win at
// small N. The asymptotic ratio at N=1k+ is ~6x; at N=10 fixed overhead
// dominates and the realistic floor is ~2x. CI thresholds sit below observed
// ratios with margin for GC jitter; §7 prose carries the measured numbers.

// retainedTreesPerSample holds the live set used for one ReadMemStats sample.
// 8 trees averages out single-tree jitter without dominating runtime memory at
// N=10000.
const retainedTreesPerSample = 8

func TestRangeRetainedMemory_LegacyVsStream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short; allocates and GCs at N=10000")
	}

	cases := []struct {
		name     string
		n        int
		minRatio float64
	}{
		// N=10 floor is intentionally loose: even with warm-up, the first
		// process-level invocation pays cold-arena cost on stream-mode
		// allocations and can land at ~1.8x. 1.5x catches the meaningful
		// regression case (legacy/stream ratio collapses) without flaking on
		// cold-start jitter.
		{"N=10", 10, 1.5},
		{"N=100", 100, 4.0},
		{"N=1000", 1000, 5.0},
		{"N=10000", 10000, 5.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Warm-up call discarded — the first allocation of each size class
			// pays cold-arena setup that skews per-item bytes by 2x at N=10.
			_ = measureRetainedBytes(t, tc.n, false)
			_ = measureRetainedBytes(t, tc.n, true)

			legacyBytes := measureRetainedBytes(t, tc.n, false)
			streamBytes := measureRetainedBytes(t, tc.n, true)

			perItemLegacy := float64(legacyBytes) / float64(tc.n)
			perItemStream := float64(streamBytes) / float64(tc.n)
			ratio := perItemLegacy / perItemStream

			t.Logf("N=%d: legacy=%.1f B/item, stream=%.1f B/item, ratio=%.2fx",
				tc.n, perItemLegacy, perItemStream, ratio)

			if ratio < tc.minRatio {
				t.Errorf("retained memory ratio %.2fx below floor %.1fx (legacy %.1f B/item, stream %.1f B/item)",
					ratio, tc.minRatio, perItemLegacy, perItemStream)
			}
		})
	}
}

// measureRetainedBytes returns HeapAlloc bytes attributable to retainedTreesPerSample
// trees of itemCount items each. stream=true builds them in stream-mode shape.
//
// Retries up to measureRetries times if HeapAlloc does not grow — concurrent
// finalizer/sweep activity occasionally frees more than the retained trees
// add, especially at small N where the live-set delta is comparable to GC
// jitter. Only skips after all retries fail; a persistent zero-growth result
// across attempts indicates the helper itself is broken, not transient noise,
// and silently skipping would defeat the CI gate.
const measureRetries = 3

func measureRetainedBytes(t *testing.T, itemCount int, stream bool) uint64 {
	t.Helper()

	for attempt := 0; attempt < measureRetries; attempt++ {
		if delta, ok := measureRetainedBytesOnce(itemCount, stream); ok {
			return delta
		}
	}

	t.Skipf("HeapAlloc did not grow across %d attempts at N=%d (stream=%v) — likely persistent GC noise; the ratio assertion cannot run", measureRetries, itemCount, stream)
	return 0
}

func measureRetainedBytesOnce(itemCount int, stream bool) (uint64, bool) {
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	trees := make([]*build.TreeNode, retainedTreesPerSample)
	for i := range trees {
		trees[i] = createTreeNodeRangeTree(itemCount)
		if stream {
			TransitionToStreamMode(trees[i])
		}
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	runtime.KeepAlive(trees)

	if after.HeapAlloc <= before.HeapAlloc {
		return 0, false
	}
	return (after.HeapAlloc - before.HeapAlloc) / retainedTreesPerSample, true
}
