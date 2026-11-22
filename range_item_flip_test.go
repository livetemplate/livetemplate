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
