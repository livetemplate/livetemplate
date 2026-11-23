package livetemplate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestStaticsTable: rows wrapped in a conditional send their statics once (spec
// §2.5), and the client (the TypeScript oracle) renders the same page.
func TestStaticsTable(t *testing.T) {
	src := `<div>{{range .L}}{{if .Fold}}<p class="fold">…</p>{{else}}` +
		`<div class="line-row" data-key="{{.K}}"><span class="gutter">{{.K}}</span><span class="content">{{.Text}}</span></div>{{end}}{{end}}</div>`
	type row struct {
		K, Text string
		Fold    bool
	}
	state := func(text string) map[string]any {
		rows := make([]row, 100)
		for i := range rows {
			rows[i] = row{K: fmt.Sprintf("L%03d", i), Text: text, Fold: i%25 == 24}
		}
		return map[string]any{"L": rows}
	}
	tmpl := flipTemplate(t, src)
	raw, first := flipRender(t, tmpl, state("x"))
	wire := onTheWire(t, first)
	b, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), "line-row"); n != 1 {
		t.Errorf("line-row appears %d times on the wire, want once", n)
	}
	// What is left is the rows' own values; the repeated markup is gone.
	if len(b)*3 > len(raw) {
		t.Errorf("interned first render is %d bytes (was %d), want under a third", len(b), len(raw))
	}

	oracle := getTSOracle(t)
	plain, err := oracle.ApplyDiffRaw(map[string]any{}, first)
	if err != nil {
		t.Fatal(err)
	}
	interned, err := oracle.ApplyDiffRaw(map[string]any{}, wire)
	if err != nil {
		t.Fatal(err)
	}
	if plain.HTML != interned.HTML || !strings.Contains(interned.HTML, `class="line-row"`) {
		t.Fatalf("interned render differs:\nplain:    %.300s\ninterned: %.300s", plain.HTML, interned.HTML)
	}
	// A later update through the table applies like any other; flipApply checks
	// the page against a fresh render.
	_, update := flipRender(t, tmpl, state("y"))
	flipApply(t, src, plain.Tree.(map[string]any), update, state("y"), "")
}
