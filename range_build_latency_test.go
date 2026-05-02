package livetemplate

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"
)

// rowFiveField mirrors the LargeTable demo's 5-field row (ID + 4 dynamic
// fields), so the latency regression assertion exercises the same per-item
// shape we measured at 306 ms / N=10k pre-Phase-7.
type rowFiveField struct {
	ID     string
	Name   string
	Email  string
	Status string
	Score  int
}

type latencyState struct {
	Items []rowFiveField
}

const latencyTemplate = `<table><tbody>{{range .Items}}<tr data-key="{{.ID}}"><td>{{.Name}}</td><td>{{.Email}}</td><td>{{.Status}}</td><td>{{.Score}}</td></tr>{{end}}</tbody></table>`

// TestRangeBuildLatency_PostPhase7 is the wall-clock CPU regression gate for
// the Phase 7 (A) type-direct hash + (B) parallel range build optimizations.
// Pre-Phase-7 baseline at N=10k was ~306 ms/render; ceilings here sit
// comfortably below that with CI-jitter margin and represent the level of
// regression that would silently undo the work if a future change reverted
// the parallel dispatch or re-introduced JSON-marshal in the hash path.
//
// Skipped under -short (each case forces multiple full Execute/ExecuteUpdates
// cycles and the N=10k case allocates ~90 MB per iteration before GC).
func TestRangeBuildLatency_PostPhase7(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short; allocates ~90MB at N=10000")
	}

	cases := []struct {
		name      string
		n         int
		ceilingNs int64
	}{
		// Ceilings sit comfortably above measured medians on a recent
		// linux/arm64 8-core host (Post-Phase-7+8: N=1000 ≈ 7 ms;
		// N=10000 ≈ 60 ms). Pre-Phase-7 baseline at N=10k was 306 ms —
		// these gates would trip on regressions that re-serialised
		// iterateSlice, reverted the type-switch hash to JSON-marshal,
		// or re-introduced the renderHTMLWithData dead work on the
		// steady-state path.
		{"N=1000", 1000, int64(25 * time.Millisecond)},
		{"N=10000", 10000, int64(150 * time.Millisecond)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := Must(New("phase7-latency"))
			if _, err := tmpl.Parse(latencyTemplate); err != nil {
				t.Fatalf("parse: %v", err)
			}

			state := latencyState{Items: makeLatencyRows(tc.n)}

			if err := tmpl.Execute(io.Discard, state); err != nil {
				t.Fatalf("initial Execute: %v", err)
			}
			if err := tmpl.ExecuteUpdates(io.Discard, state); err != nil {
				t.Fatalf("transition ExecuteUpdates: %v", err)
			}
			for i := 0; i < 3; i++ {
				mutateOneRow(state.Items, i)
				if err := tmpl.ExecuteUpdates(io.Discard, state); err != nil {
					t.Fatalf("warm-up ExecuteUpdates iter %d: %v", i, err)
				}
			}

			const samples = 5
			durations := make([]int64, samples)
			var buf bytes.Buffer
			for i := 0; i < samples; i++ {
				mutateOneRow(state.Items, i)
				buf.Reset()
				start := time.Now()
				if err := tmpl.ExecuteUpdates(&buf, state); err != nil {
					t.Fatalf("sample %d ExecuteUpdates: %v", i, err)
				}
				durations[i] = time.Since(start).Nanoseconds()
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			median := durations[samples/2]

			ceiling := tc.ceilingNs
			t.Logf("median=%v ceiling=%v samples_ns=%v",
				time.Duration(median), time.Duration(ceiling), durations)
			if median > ceiling {
				t.Errorf("Phase 7 latency regression at %s: median %v exceeds ceiling %v\nall samples: %v",
					tc.name, time.Duration(median), time.Duration(ceiling), durations)
			}
		})
	}
}

func makeLatencyRows(n int) []rowFiveField {
	rows := make([]rowFiveField, n)
	statuses := []string{"active", "pending", "blocked", "archived"}
	for i := range rows {
		id := i + 1
		rows[i] = rowFiveField{
			ID:     fmt.Sprintf("row-%05d", id),
			Name:   fmt.Sprintf("User %05d", id),
			Email:  fmt.Sprintf("user%05d@example.com", id),
			Status: statuses[id%4],
			Score:  (id * 37) % 1000,
		}
	}
	return rows
}

func mutateOneRow(rows []rowFiveField, seed int) {
	if len(rows) == 0 {
		return
	}
	idx := seed % len(rows)
	rows[idx].Score = (rows[idx].Score + 1) % 1000
}
