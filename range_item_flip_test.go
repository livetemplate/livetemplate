package livetemplate

import (
	"fmt"
	"strings"
	"testing"
)

type flipRow struct {
	K, T string
	Mark bool
}

// flipRows is 20 keyed rows; mark lists the rows whose {{if .Mark}} is true.
func flipRows(text string, mark ...int) []flipRow {
	rows := make([]flipRow, 20)
	for i := range rows {
		rows[i] = flipRow{K: fmt.Sprintf("row%02d", i), T: "text"}
	}
	for _, i := range mark {
		rows[i].Mark = true
	}
	rows[5].T = text
	return rows
}

// TestRangeItemFlip: an {{if}} that flips inside one kept range item sends a ["u"]
// for that item, not the whole range. The item's fingerprint covers its nested
// structure, so the flip used to fail both range paths (stream mode's homogeneity
// check, then the differential path's kept-item check) and every item went out
// again with its statics. In prereview each code line is a range item with an
// is-cursor conditional, so one ArrowDown resent the file (~490 KB on 300 lines).
//
// Each update is applied by the TypeScript oracle and must match a fresh render.
func TestRangeItemFlip(t *testing.T) {
	shapes := []struct{ name, src string }{
		{"attribute if", `<div>{{range .L}}<p data-key="{{.K}}"{{if .Mark}} x="1"{{end}}>{{.T}}</p>{{end}}</div>`},
		{"body if", `<div>{{range .L}}<p data-key="{{.K}}">{{.T}}{{if .Mark}}<b>m</b>{{end}}</p>{{end}}</div>`},
		{"inside an if", `{{if not .Off}}<div>{{range .L}}<p data-key="{{.K}}"{{if .Mark}} x="1"{{end}}>{{.T}}</p>{{end}}</div>{{end}}`},
	}
	// Each step: which rows are marked (the cursor), and row 5's text.
	steps := []struct {
		what  string
		mark  []int
		text  string
		items int // how many items the ["u"]s may touch
	}{
		{"row 0 flips on", []int{0}, "text", 1},
		{"the mark moves to row 1 (two flips)", []int{1}, "text", 2},
		{"row 1 flips off", nil, "text", 1},
		{"row 0 flips on again", []int{0}, "text", 1},
		{"then a plain text change", []int{0}, "edited", 1},
	}
	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			state := func(mark []int, text string) map[string]any {
				return map[string]any{"L": flipRows(text, mark...), "Off": false}
			}
			tmpl := flipTemplate(t, sh.src)
			_, client := flipRender(t, tmpl, state(nil, "text"))
			for _, st := range steps {
				wire, update := flipRender(t, tmpl, state(st.mark, st.text))
				// Under 150 bytes per changed item; the bug resent ~750.
				if len(wire) > 150*st.items || strings.Count(wire, `"u"`) != st.items {
					t.Errorf("%s: sent %d bytes, want %d [\"u\"] op(s) under %d bytes: %s",
						st.what, len(wire), st.items, 150*st.items, wire)
				}
				var html string
				client, html = flipApply(t, sh.src, client, update, state(st.mark, st.text), wire)
				if !strings.Contains(html, ">edited<") && st.text == "edited" {
					t.Errorf("%s: the text change is missing from the page: %s", st.what, normalizeHTMLForTest(html))
				}
			}
		})
	}
}

// TestRangeItemFlipAllThenOne: every row flips together, so the range is
// homogeneous again and returns to stream mode with an exemplar in the NEW shape.
// Then one row flips back: its ["u"] must carry the old branch's statics, because
// the client now holds the new shape. The exemplar always matches what the client
// has, so this stays correct across the stream-mode round trip.
func TestRangeItemFlipAllThenOne(t *testing.T) {
	src := `<div>{{range .L}}<p data-key="{{.K}}"{{if .Mark}} x="1"{{end}}>{{.T}}</p>{{end}}</div>`
	all := make([]int, 20)
	for i := range all {
		all[i] = i
	}
	state := func(mark []int) map[string]any { return map[string]any{"L": flipRows("text", mark...)} }
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state(nil))
	for _, st := range []struct {
		what string
		mark []int
	}{
		{"every row flips on", all},
		{"a render with no change", all},
		{"row 3 flips back off", append(append([]int{}, all[:3]...), all[4:]...)},
		{"row 3 flips on again", all},
	} {
		wire, update := flipRender(t, tmpl, state(st.mark))
		var html string
		client, html = flipApply(t, src, client, update, state(st.mark), wire)
		if want := strings.Count(html, `x="1"`); want != len(st.mark) {
			t.Errorf("%s: %d rows show the attribute, want %d: %s", st.what, want, len(st.mark), wire)
		}
	}
}

// TestRangeItemFlipWithNestedInsert: in the same update, an item's {{if}} flips
// AND its nested range gains a sub-item. Ranges holding nested ranges stay on the
// differential path; the item's ["u"] carries both the flipped slot's statics and
// the nested insert's.
func TestRangeItemFlipWithNestedInsert(t *testing.T) {
	src := `<ul>{{range .L}}<li data-key="{{.K}}"{{if .Mark}} x="1"{{end}}><ol>{{range .Subs}}<b data-key="{{.}}">{{.}}</b>{{end}}</ol></li>{{end}}</ul>`
	type row struct {
		K    string
		Mark bool
		Subs []string
	}
	state := func(mark bool, subs ...string) map[string]any {
		return map[string]any{"L": []row{{K: "a", Mark: mark, Subs: subs}, {K: "b", Subs: []string{"y"}}}}
	}
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state(false, "x"))
	wire, update := flipRender(t, tmpl, state(true, "x", "z"))
	_, html := flipApply(t, src, client, update, state(true, "x", "z"), wire)
	if !strings.Contains(html, `<b data-key="z">z</b>`) || !strings.Contains(html, `x="1"`) || strings.Contains(wire, `data-key=\"b\"`) {
		t.Errorf("want the flip and the new sub-item on row a only; page %s\nupdate %s", normalizeHTMLForTest(html), wire)
	}
}
