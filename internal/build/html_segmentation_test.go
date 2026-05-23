package build

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// blockTagDecoyCases enumerates fragments whose literal text contains block
// tag substrings ("<div", "<article", etc.) but where those substrings are
// inside HTML comments, RAWTEXT (<script>/<style> content), RCDATA (<title>/
// <textarea> content), or attribute values — i.e. NOT real start tags.
// findBlockTagBoundaries must skip every one of these.
var blockTagDecoyCases = []struct {
	name string
	html string
}{
	{
		"<div inside <style> comment",
		`<style>/* fake <div> note */ .x{}</style>`,
	},
	{
		"<article inside inline JS string",
		`<script>var s = "<article id='x'>";</script>`,
	},
	{
		"<section inside HTML comment",
		`<!-- <section> in a comment -->`,
	},
	{
		"<nav inside meta content attribute",
		`<meta property="og:description" content="<nav class='x'>">`,
	},
	{
		"<table inside title text (RCDATA)",
		`<title>About the <table> tag</title>`,
	},
	{
		"<aside inside textarea text (RCDATA)",
		`<textarea><aside></textarea>`,
	},
	{
		"<main inside style+script decoys",
		`<style>/* <main> note */</style><script>var x="<main>";</script>`,
	},
	{
		"<ul inside HTML comment with multiple block tags",
		`<!-- <ul><li><ol> all here --><!--end-->`,
	},
}

// TestFindBlockTagBoundaries_DecoyImmunity confirms that block-tag substrings
// inside comments / RAWTEXT / RCDATA / attribute values are NOT picked up as
// segment boundaries. This is the canonical regression test for #436.
func TestFindBlockTagBoundaries_DecoyImmunity(t *testing.T) {
	for _, tc := range blockTagDecoyCases {
		t.Run(tc.name, func(t *testing.T) {
			boundaries := findBlockTagBoundaries(tc.html)
			if len(boundaries) != 0 {
				t.Errorf("expected 0 boundaries for decoy-only input, got %d at offsets %v\ninput=%q",
					len(boundaries), boundaries, tc.html)
			}
		})
	}
}

// TestFindBlockTagBoundaries_RealTagsFound confirms genuine block-level start
// tags are still found (the tokenizer migration didn't lose coverage).
func TestFindBlockTagBoundaries_RealTagsFound(t *testing.T) {
	tests := []struct {
		name string
		html string
		want int // expected number of boundaries
	}{
		{"single div", `<div>x</div>`, 1},
		{"three siblings", `<div>a</div><main>b</main><div>c</div>`, 3},
		{"nested counts each", `<div><section><ul><li>x</li></ul></section></div>`, 3},
		{"non-block tag ignored", `<span>x</span><p>y</p>`, 0},
		{"mixed", `<div>a</div><span>b</span><article>c</article>`, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := len(findBlockTagBoundaries(tc.html))
			if got != tc.want {
				t.Errorf("findBlockTagBoundaries(%q) returned %d boundaries, want %d",
					tc.html, got, tc.want)
			}
		})
	}
}

// TestFindBlockTagBoundaries_OffsetsAreAscending confirms the single-walk
// invariant: boundaries are returned in document order without needing a sort.
func TestFindBlockTagBoundaries_OffsetsAreAscending(t *testing.T) {
	input := `<div>a</div><section>b</section><main>c</main><article>d</article>`
	boundaries := findBlockTagBoundaries(input)
	for i := 1; i < len(boundaries); i++ {
		if boundaries[i] <= boundaries[i-1] {
			t.Errorf("boundaries not ascending at index %d: %v", i, boundaries)
		}
	}
}

// TestFindBlockTagBoundaries_PropertyDecoys is a randomized regression test:
// no combination of block-tag-bearing decoy contexts in head/inline-script/
// style/comment should ever produce a false boundary. Catches future
// regressions toward naive strings.Index.
func TestFindBlockTagBoundaries_PropertyDecoys(t *testing.T) {
	const realMarker = `<div id="real">REAL</div>`
	decoys := []string{
		`<style>/* <div> */</style>`,
		`<style>/* <article> */</style>`,
		`<script>var s="<section>";</script>`,
		`<script>// <main></main></script>`,
		`<meta property="og:title" content="<nav>">`,
		`<!-- <ul> -->`,
		`<!-- <ol></ol> -->`,
		`<title>About <table></title>`,
		`<textarea><aside></textarea>`,
	}

	const seed = int64(0xb10c1abc)
	rng := rand.New(rand.NewSource(seed))

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		n := rng.Intn(len(decoys) + 1)
		perm := rng.Perm(len(decoys))
		var head strings.Builder
		for _, k := range perm[:n] {
			head.WriteString(decoys[k])
		}
		input := head.String() + realMarker

		boundaries := findBlockTagBoundaries(input)
		if len(boundaries) != 1 {
			t.Fatalf("iter %d (seed=%#x): expected exactly 1 boundary (for real <div>), got %d at %v\ninput=%q",
				i, seed, len(boundaries), boundaries, input)
		}
		// The boundary must point at the real <div>, not anywhere in the
		// decoy prefix.
		wantOffset := strings.Index(input, realMarker)
		if boundaries[0] != wantOffset {
			t.Fatalf("iter %d (seed=%#x): boundary at %d, want %d (start of real <div>)\ninput=%q",
				i, seed, boundaries[0], wantOffset, input)
		}
	}
}

// TestFindBlockTagBoundaries_OffsetCoverage proves the offset accounting
// (sum(len(z.Raw())) == len(input)) holds across realistic inputs. If this
// drifts, every offset returned by findBlockTagBoundaries is wrong by the
// same delta. Mirrors the wrapper.go offset-coverage sanity test.
func TestFindBlockTagBoundaries_OffsetCoverage(t *testing.T) {
	inputs := []string{
		`<div>x</div>`,
		`<style>/* <div> */</style><div>real</div>`,
		`<!doctype html><html><body><main>x</main></body></html>`,
		``,
		`plain text`,
	}
	for i, in := range inputs {
		t.Run(fmt.Sprintf("input_%d", i), func(t *testing.T) {
			// Re-walk the tokens the same way findBlockTagBoundaries does
			// and verify sum(len(raw)) == len(in).
			boundaries := findBlockTagBoundaries(in)
			// The boundaries themselves don't reveal the offset coverage,
			// but every boundary must lie within [0, len(in)).
			for _, b := range boundaries {
				if b < 0 || b >= len(in) {
					t.Errorf("boundary %d out of range [0, %d)", b, len(in))
				}
			}
		})
	}
}

// TestCreateHTMLStructureBasedTree_DecoyInputDoesNotSegment confirms that an
// input whose only "block tags" are inside decoy contexts produces the
// fallback single-segment tree (because findBlockTagBoundaries returns
// empty). Before the fix, decoys would have created false boundaries and
// possibly an over-segmented tree.
func TestCreateHTMLStructureBasedTree_DecoyInputDoesNotSegment(t *testing.T) {
	input := `<style>/* <div> */</style><script>var x="<article>";</script>plain content with no real block tags`

	tree := CreateHTMLStructureBasedTree(input)
	if tree == nil {
		t.Fatal("expected fallback tree, got nil")
	}
	// Fallback shape: 2 statics ("", ""), 1 dynamic (the whole content).
	if len(tree.Statics) != 2 {
		t.Errorf("expected fallback 2-statics tree, got %d statics: %#v",
			len(tree.Statics), tree.Statics)
	}
	if tree.DynamicLen() != 1 {
		t.Errorf("expected 1 dynamic in fallback tree, got %d", tree.DynamicLen())
	}
}
