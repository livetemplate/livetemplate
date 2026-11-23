package livetemplate

import (
	"encoding/json"
	"strings"
	"testing"
)

// flipTemplate parses src as a fresh template.
func flipTemplate(t *testing.T, src string) *Template {
	t.Helper()
	tmpl, err := New("t")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpl.Parse(src); err != nil {
		t.Fatal(err)
	}
	return tmpl
}

// flipRender runs ExecuteUpdates and returns the wire JSON and its decoded tree.
func flipRender(t *testing.T, tmpl *Template, data any) (string, map[string]any) {
	t.Helper()
	var b strings.Builder
	if err := tmpl.ExecuteUpdates(&b, data); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(b.String()), &m); err != nil {
		t.Fatalf("decode %s: %v", b.String(), err)
	}
	return b.String(), m
}

// flipApply applies update to the client tree with the TypeScript oracle (the
// real client) and checks the page matches a fresh first render of data, drawn by
// the same client so both sides use one renderer. It returns the new client tree
// and the page HTML.
func flipApply(t *testing.T, src string, client, update map[string]any, data any, wire string) (map[string]any, string) {
	t.Helper()
	oracle := getTSOracle(t)
	resp, err := oracle.ApplyDiffRaw(client, onTheWire(t, update))
	if err != nil {
		t.Fatalf("oracle: %v", err)
	}
	m, ok := resp.Tree.(map[string]any)
	if !ok {
		t.Fatalf("oracle returned a %T tree, want an object", resp.Tree)
	}
	client = m
	_, fresh := flipRender(t, flipTemplate(t, src), data)
	want, err := oracle.ApplyDiffRaw(fresh, map[string]any{})
	if err != nil {
		t.Fatalf("oracle on the fresh render: %v", err)
	}
	if !htmlEquivalent(resp.HTML, want.HTML) {
		t.Errorf("the client shows\n  %s\nwant\n  %s\nafter update %s",
			normalizeHTMLForTest(resp.HTML), normalizeHTMLForTest(want.HTML), wire)
	}
	return client, resp.HTML
}

// TestNestedConditionalFlip: an {{if}} that flips inside another conditional sends
// only its own slot. The nested node used to be judged by its recursive
// fingerprint, so the flip made every ancestor look changed and the whole outer
// branch (here, the range beside the flip) went out again with its statics, on
// every flip. In prereview, whose page body sits in one {{if}}/{{else}}, a toast
// appearing resent the page (prereview#273).
func TestNestedConditionalFlip(t *testing.T) {
	const items = `<div>{{range .KB}}<b>{{.}}</b>{{end}}</div>`
	shapes := []struct{ name, src string }{
		{"top level", items + `{{if .Flash}}<p>{{.Flash}}</p>{{end}}`},
		{"inside if", `{{if not .Ext}}` + items + `{{if .Flash}}<p>{{.Flash}}</p>{{end}}{{end}}`},
		{"inside else", `{{if .Ext}}<i>x</i>{{else}}` + items + `{{if .Flash}}<p>{{.Flash}}</p>{{end}}{{end}}`},
		{"in a template called from else", `{{if .Ext}}<i>x</i>{{else}}` + items + `{{template "toast" $}}{{end}}` +
			`{{define "toast"}}{{if .Flash}}<p>{{.Flash}}</p>{{end}}{{end}}`},
		{"two levels deep", `{{if not .Ext}}<main>{{if not .Off}}` + items + `{{if .Flash}}<p>{{.Flash}}</p>{{end}}{{end}}</main>{{end}}`},
		// The child swaps branches, so its OWN statics change: it goes whole, the
		// range beside it doesn't.
		{"inner if/else swap", `{{if not .Ext}}` + items + `{{if .Flash}}<p>{{.Flash}}</p>{{else}}<em>none</em>{{end}}{{end}}`},
	}
	state := func(flash string) map[string]any {
		return map[string]any{"KB": []string{"a", "b", "c", "d", "e", "f"}, "Flash": flash, "Ext": false, "Off": false}
	}
	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			tmpl := flipTemplate(t, sh.src)
			_, client := flipRender(t, tmpl, state(""))
			// on, off, on again: every flip, not just the first.
			for _, flash := range []string{"on", "", "on"} {
				wire, update := flipRender(t, tmpl, state(flash))
				// A fixed bound on purpose: the flipped slot is a few dozen bytes,
				// and the bug resent ~360. A bound derived from the first render
				// would let a partial regression through.
				if len(wire) > 100 || strings.Contains(wire, "<b>") {
					t.Errorf("flash=%q sent %d bytes, want only the flipped slot (no range markup): %s", flash, len(wire), wire)
				}
				var html string
				client, html = flipApply(t, sh.src, client, update, state(flash), wire)
				if strings.Contains(html, "<p>on</p>") != (flash != "") {
					t.Errorf("flash=%q: the toast is wrong on the page: %s", flash, normalizeHTMLForTest(html))
				}
				if !strings.Contains(html, "<b>f</b>") {
					t.Errorf("flash=%q: the range is missing from the page: %s", flash, normalizeHTMLForTest(html))
				}
				if strings.Contains(sh.src, "<em>") && strings.Contains(html, "<em>none</em>") != (flash == "") {
					t.Errorf("flash=%q: the else branch is wrong on the page: %s", flash, normalizeHTMLForTest(html))
				}
			}
		})
	}
}

// TestNestedFlipWithRangeChange: a flip and a change to the range beside it in
// the same update. The parent's own structure is unchanged, so the range is
// diffed in its own slot (an append or removal, not a resend) and the flip in its
// own.
func TestNestedFlipWithRangeChange(t *testing.T) {
	src := `{{if not .Ext}}<div>{{range .KB}}<b data-key="{{.}}">{{.}}</b>{{end}}</div>{{if .Flash}}<p>{{.Flash}}</p>{{end}}{{end}}`
	state := func(kb []string, flash string) map[string]any {
		return map[string]any{"KB": kb, "Flash": flash, "Ext": false}
	}
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state([]string{"a", "b", "c", "d", "e", "f"}, ""))
	for i, st := range []map[string]any{
		state([]string{"a", "b", "c", "d", "e", "f", "g"}, "on"), // add an item and flip on
		state([]string{"b", "c", "d", "e", "f", "g"}, ""),        // remove one and flip off
	} {
		wire, update := flipRender(t, tmpl, st)
		if strings.Contains(wire, "<b data-key") || len(wire) > 200 {
			t.Errorf("step %d sent %d bytes, want the range diffed (no item markup resent): %s", i, len(wire), wire)
		}
		client, _ = flipApply(t, src, client, update, st, wire)
	}
}

// TestNestedRangeItemShapeChange: #489 one level down. The page from
// TestRangeItemStaticsChange_UpdateIsNotDropped, wrapped in an {{if}}: an item's
// note template renders nothing, then something, so the item's slot shape changes
// under an ancestor whose own structure doesn't. The change must still reach the
// page; the outer node not going whole must not drop it.
func TestNestedRangeItemShapeChange(t *testing.T) {
	src := `{{if not .Ext}}<ul>{{range .Tags}}<li class="tag">{{.}}</li>{{end}}</ul>` +
		`<ul>{{range .Items}}<li data-key="{{.ID}}"><span>{{.Body}}</span>{{template "note" .Note}}</li>{{end}}</ul>{{end}}` +
		`{{define "note"}}{{if .}}<em class="note">{{.}}</em>{{end}}{{end}}`
	type item struct{ ID, Body, Note string }
	state := func(note string) map[string]any {
		return map[string]any{"Ext": false, "Tags": []string{"go"}, "Items": []item{{ID: "a", Body: "hello", Note: note}}}
	}
	tmpl := flipTemplate(t, src)
	_, client := flipRender(t, tmpl, state(""))
	wire, update := flipRender(t, tmpl, state("now it has a note"))
	_, html := flipApply(t, src, client, update, state("now it has a note"), wire)
	if !strings.Contains(html, "now it has a note") {
		t.Errorf("the note never reaches the page: %s\nupdate: %s", normalizeHTMLForTest(html), wire)
	}
}
