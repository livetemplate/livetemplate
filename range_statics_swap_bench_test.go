package livetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// Measures wire-payload bytes per full content swap. DynamicBranch bytes/op are intentionally higher than Simple — nested branch statics MUST stay in the insert payload (no client-side branch-statics cache).

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
			// Uniform HasMarker keeps the range homogeneous so TransitionToStreamMode succeeds.
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

	// Prime with initial render so subsequent iterations exercise the update path.
	if err := tmpl.ExecuteUpdates(&bytes.Buffer{}, d1); err != nil {
		b.Fatalf("initial render failed: %v", err)
	}

	var totalBytes int64
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		// Alternate payloads so each iteration is a real swap.
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

// Simple range: zero per-row statics; bytes/op scales with dynamic content size.
func BenchmarkRangeFullSwap_Simple_N10(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchSimpleTemplate, 10)
}
func BenchmarkRangeFullSwap_Simple_N100(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchSimpleTemplate, 100)
}
func BenchmarkRangeFullSwap_Simple_N1000(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchSimpleTemplate, 1000)
}

// DynamicBranch: nested {{if}} with dynamic. Branch statics MUST stay in the wire — bytes/op reflects that cost.
func BenchmarkRangeFullSwap_DynamicBranch_N10(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchDynamicBranchTemplate, 10)
}
func BenchmarkRangeFullSwap_DynamicBranch_N100(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchDynamicBranchTemplate, 100)
}
func BenchmarkRangeFullSwap_DynamicBranch_N1000(b *testing.B) {
	benchmarkFullContentSwapPayload(b, benchDynamicBranchTemplate, 1000)
}
