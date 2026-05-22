package livetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// Benchmarks for issue #413: measure the WIRE PAYLOAD size of a full content
// swap of a {{range}} at varying row counts. We measure the JSON length, not
// allocator bytes, because the issue is about transmitted bytes.
//
// Two shapes:
//   1. "Simple": homogeneous range with primitive dynamics — the case from
//      the issue. After the existing fingerprint-based strip, this should
//      scale near-linearly in dynamic content size with zero per-row static
//      overhead.
//   2. "DynamicBranch": homogeneous range whose item template contains an
//      {{if}}…{{else}}{{end}} branch with a dynamic inside the if-arm. Before
//      the fix in #413 the stream-mode insert path re-emitted those branch
//      statics; after the fix they are stripped under the §5b homogeneity
//      guarantee. Static-only branches still retain statics (the special
//      case in PrepareTreeForClient preserves branch identity).
//
// Reported metric: bytes/op = wire JSON bytes per update.

const benchSimpleTemplate = `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span><span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

const benchDynamicBranchTemplate = `<div class="diff">{{range .Lines}}<div class="line-row" data-key="{{.ID}}"><span class="kind">{{.Kind}}</span><span class="ln">{{.LineNo}}</span>{{if .HasMarker}}<span class="marker">{{.MarkerText}}</span>{{end}}<span class="content">{{.HighlightedContent}}</span></div>{{end}}</div>`

type benchLine struct {
	ID                 string
	Kind               string
	LineNo             int
	HighlightedContent template.HTML
	HasMarker          bool
	MarkerText         string
}

type benchState struct {
	Lines []benchLine
}

func benchMakeLines(n int, prefix string) []benchLine {
	out := make([]benchLine, n)
	for i := 0; i < n; i++ {
		out[i] = benchLine{
			ID:                 fmt.Sprintf("%s-%d", prefix, i),
			Kind:               []string{"add", "rem", "ctx"}[i%3],
			LineNo:             i + 1,
			HighlightedContent: template.HTML(fmt.Sprintf("<span>%s line %d %s</span>", prefix, i, strings.Repeat("x", 16))),
			// HasMarker is uniform across items so the range stays
			// homogeneous and TransitionToStreamMode succeeds — the
			// improvement in #413 is in the stream-mode insert path.
			// Heterogeneous ranges (some items with markers, some without)
			// fall back to the legacy diff path which conservatively keeps
			// nested statics.
			HasMarker:  true,
			MarkerText: fmt.Sprintf("M%d", i),
		}
	}
	return out
}

func benchmarkFullContentSwapPayload(b *testing.B, tmplStr string, n int) {
	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		b.Fatalf("Parse failed: %v", err)
	}

	d1 := benchState{Lines: benchMakeLines(n, "A")}
	d2 := benchState{Lines: benchMakeLines(n, "B")}

	// Prime the template once with the initial render so subsequent
	// iterations exercise the update (stream-mode) path.
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, d1); err != nil {
		b.Fatalf("initial render failed: %v", err)
	}

	var totalBytes int64
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		// Alternate between two payloads so each iteration is a real swap
		// (not a no-op repeat).
		d := d1
		if i%2 == 0 {
			d = d2
		}
		if err := tmpl.ExecuteUpdates(&buf, d); err != nil {
			b.Fatalf("ExecuteUpdates failed: %v", err)
		}
		totalBytes += int64(buf.Len())
	}
	b.StopTimer()
	b.ReportMetric(float64(totalBytes)/float64(b.N), "bytes/op")
}

// Simple (no nested conditionals) — the "premise" case from issue #413.
// Expected: zero per-row statics; bytes/op proportional to per-row dynamic
// content size.
func BenchmarkRangeFullSwap_Simple_N10(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchSimpleTemplate, 10)
}
func BenchmarkRangeFullSwap_Simple_N100(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchSimpleTemplate, 100)
}
func BenchmarkRangeFullSwap_Simple_N1000(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchSimpleTemplate, 1000)
}

// DynamicBranch (nested {{if}} with dynamic inside the if-arm) — exercises
// the stream-mode insert path that previously re-emitted nested branch
// statics. After the fix in #413, the if-arm statics are stripped under the
// §5b homogeneity guarantee.
func BenchmarkRangeFullSwap_DynamicBranch_N10(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchDynamicBranchTemplate, 10)
}
func BenchmarkRangeFullSwap_DynamicBranch_N100(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchDynamicBranchTemplate, 100)
}
func BenchmarkRangeFullSwap_DynamicBranch_N1000(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchDynamicBranchTemplate, 1000)
}
