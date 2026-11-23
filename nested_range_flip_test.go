package livetemplate

import (
	"fmt"
	"strings"
	"testing"
)

type flipCard struct {
	ID    string
	Reply bool
}

type flipLine struct {
	K     string
	Cards []flipCard
}

// flipLines is 30 keyed lines; line 5 holds two cards, the first of which has its
// {{if .Reply}} set when reply is true.
func flipLines(reply bool) []flipLine {
	lines := make([]flipLine, 30)
	for i := range lines {
		lines[i] = flipLine{K: fmt.Sprintf("L%02d", i)}
	}
	lines[5].Cards = []flipCard{{ID: "c1", Reply: reply}, {ID: "c2"}}
	return lines
}

// TestNestedRangeItemFlip: an {{if}} that flips inside an item of a NESTED range
// (a comment card inside a code line) sends a ["u"] for that card inside a ["u"]
// for its line, not the whole outer range. The inner ["u"] carries the flipped
// slot's statics, which the outer kept-item diff used to reject as unexplained,
// falling back to a full-range replace (105–108 KB per comment action in
// prereview). Each update is applied by the TypeScript oracle.
func TestNestedRangeItemFlip(t *testing.T) {
	src := `<div>{{range .L}}<p data-key="{{.K}}">{{.K}}{{range .Cards}}<i data-key="{{.ID}}">{{.ID}}{{if .Reply}}<b>reply</b>{{end}}</i>{{end}}</p>{{end}}</div>`
	state := func(reply bool) map[string]any { return map[string]any{"L": flipLines(reply)} }
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state(false))
	for _, reply := range []bool{true, false, true} {
		wire, update := flipRender(t, tmpl, state(reply))
		// The bug resent all 30 lines (~1.5 KB here); one card's flip is far smaller.
		if len(wire) > 200 || strings.Contains(wire, `"L00"`) {
			t.Errorf("reply=%v sent %d bytes, want a nested [\"u\"] for card c1 only: %s", reply, len(wire), wire)
		}
		var html string
		client, html = flipApply(t, src, client, update, state(reply), wire)
		if strings.Contains(html, "<b>reply</b>") != reply {
			t.Errorf("reply=%v: the page is wrong: %s", reply, normalizeHTMLForTest(html))
		}
	}
}

// TestNestedRangeItemGainsThread: an inner item's {{if}} turns true and brings a
// new range with it (a comment card's first reply shows its thread), whose items
// hold nested nodes of their own. Everything inside that new node is new to the
// client, the new range's items and their nested statics included, so the inner
// ["u"] may carry all of it; the outer range is not resent.
func TestNestedRangeItemGainsThread(t *testing.T) {
	src := `<div>{{range .L}}<p data-key="{{.K}}">{{.K}}{{range .Cards}}<i data-key="{{.ID}}">{{.ID}}` +
		`{{if .Replies}}<div class="thread">{{range .Replies}}<s data-key="{{.}}">{{if .}}<b>{{.}}</b>{{end}}</s>{{end}}</div>{{end}}</i>{{end}}</p>{{end}}</div>`
	type card struct {
		ID      string
		Replies []string
	}
	type line struct {
		K     string
		Cards []card
	}
	state := func(replies ...string) map[string]any {
		lines := make([]line, 30)
		for i := range lines {
			lines[i] = line{K: fmt.Sprintf("L%02d", i)}
		}
		lines[5].Cards = []card{{ID: "c1", Replies: replies}}
		return map[string]any{"L": lines}
	}
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state())
	for _, replies := range [][]string{{"first"}, {"first", "second"}} {
		wire, update := flipRender(t, tmpl, state(replies...))
		if strings.Contains(wire, `"L00"`) || len(wire) > 400 {
			t.Errorf("%d replies: sent %d bytes, want a nested [\"u\"] for card c1 only: %s", len(replies), len(wire), wire)
		}
		var html string
		client, html = flipApply(t, src, client, update, state(replies...), wire)
		if !strings.Contains(html, "<b>"+replies[len(replies)-1]+"</b>") {
			t.Errorf("%d replies: the newest reply is missing: %s", len(replies), normalizeHTMLForTest(html))
		}
	}
}

// TestNestedRangeThreeLevels: a flip in a range three levels down (line > card >
// reply) is a ["u"] chain down to the reply, not a resend of any outer range.
func TestNestedRangeThreeLevels(t *testing.T) {
	src := `<div>{{range .L}}<p data-key="{{.K}}">{{.K}}{{range .Cards}}<i data-key="{{.ID}}">{{.ID}}` +
		`{{range .Replies}}<s data-key="{{.ID}}">{{.ID}}{{if .Liked}}<b>liked</b>{{end}}</s>{{end}}</i>{{end}}</p>{{end}}</div>`
	type reply struct {
		ID    string
		Liked bool
	}
	type card struct {
		ID      string
		Replies []reply
	}
	type line struct {
		K     string
		Cards []card
	}
	state := func(liked bool) map[string]any {
		lines := make([]line, 30)
		for i := range lines {
			lines[i] = line{K: fmt.Sprintf("L%02d", i)}
		}
		lines[5].Cards = []card{{ID: "c1", Replies: []reply{{ID: "r1", Liked: liked}, {ID: "r2"}}}}
		return map[string]any{"L": lines}
	}
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state(false))
	for _, liked := range []bool{true, false} {
		wire, update := flipRender(t, tmpl, state(liked))
		if strings.Contains(wire, `"L00"`) || strings.Contains(wire, `"r2"`) || len(wire) > 250 {
			t.Errorf("liked=%v: sent %d bytes, want a [\"u\"] chain down to reply r1: %s", liked, len(wire), wire)
		}
		var html string
		client, html = flipApply(t, src, client, update, state(liked), wire)
		if strings.Contains(html, "<b>liked</b>") != liked {
			t.Errorf("liked=%v: the page is wrong: %s", liked, normalizeHTMLForTest(html))
		}
	}
}

// TestContiguousInsert: a run of new items inserted at one place is one insertion,
// whatever its length. A run of four or more used to count as four insertion
// points, over the cap of three, and resent the whole range (115 KB on prereview's
// first comment save, which unfolds lines around the comment). Scattered inserts
// at four or more places still fall back. Each update goes through the oracle.
func TestContiguousInsert(t *testing.T) {
	src := `<ul>{{range .L}}<li data-key="{{.}}">{{.}}</li>{{end}}</ul>`
	base := make([]string, 30)
	for i := range base {
		base[i] = fmt.Sprintf("r%02d", i)
	}
	withRun := func(n int) []string {
		var out []string
		out = append(out, base[:10]...)
		for i := range n {
			out = append(out, fmt.Sprintf("new%d", i))
		}
		return append(out, base[10:]...)
	}
	for _, n := range []int{1, 3, 4, 8} {
		t.Run(fmt.Sprintf("run of %d", n), func(t *testing.T) {
			tmpl := flipTemplate(t, src)
			_, client := flipRender(t, tmpl, map[string]any{"L": base})
			next := map[string]any{"L": withRun(n)}
			wire, update := flipRender(t, tmpl, next)
			if strings.Contains(wire, `"r00"`) || !strings.Contains(wire, `"i"`) {
				t.Errorf("want [\"i\"] ops for the run only, got %d bytes: %s", len(wire), wire)
			}
			flipApply(t, src, client, update, next, wire)
		})
	}
	// Runs placed by index into base: each run is one insertion point, so up to
	// three runs stay incremental and four fall back.
	runsAt := func(at ...int) []string {
		var out []string
		for i := 0; i <= len(base); i++ {
			for _, a := range at {
				if a == i {
					out = append(out, fmt.Sprintf("n%da", i), fmt.Sprintf("n%db", i), fmt.Sprintf("n%dc", i), fmt.Sprintf("n%dd", i))
				}
			}
			if i < len(base) {
				out = append(out, base[i])
			}
		}
		return out
	}
	for _, tc := range []struct {
		name     string
		at       []int
		fallback bool
	}{
		{"run at the start", []int{0}, false},
		{"run at the end", []int{30}, false},
		{"two runs", []int{5, 20}, false},
		{"four runs", []int{3, 10, 17, 24}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := flipTemplate(t, src)
			_, client := flipRender(t, tmpl, map[string]any{"L": base})
			next := map[string]any{"L": runsAt(tc.at...)}
			wire, update := flipRender(t, tmpl, next)
			if got := strings.Contains(wire, `"r00"`); got != tc.fallback {
				t.Errorf("resent the range = %v, want %v: %s", got, tc.fallback, wire)
			}
			flipApply(t, src, client, update, next, wire)
		})
	}
	t.Run("scattered at five places", func(t *testing.T) {
		tmpl := flipTemplate(t, src)
		_, client := flipRender(t, tmpl, map[string]any{"L": base})
		var scattered []string
		for i, r := range base {
			scattered = append(scattered, r)
			if i%6 == 0 {
				scattered = append(scattered, fmt.Sprintf("s%d", i))
			}
		}
		next := map[string]any{"L": scattered}
		wire, update := flipRender(t, tmpl, next)
		if !strings.Contains(wire, `"r00"`) {
			t.Errorf("scattered inserts at five places should still resend the range: %s", wire)
		}
		flipApply(t, src, client, update, next, wire)
	})
}
