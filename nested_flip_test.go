package livetemplate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNestedConditionalFlip: an {{if}} that flips inside another conditional sends
// only its own slot. The nested node used to be judged by its recursive
// fingerprint, so the flip made every ancestor look changed and the whole outer
// branch (here, the range beside the flip) went out again with its statics, on
// every flip. In prereview, whose page body sits in one {{if}}/{{else}}, a toast
// appearing resent the page (prereview#273).
//
// Each update is applied by the TypeScript oracle (the real client) and must give
// the HTML a fresh render of the new state gives.
func TestNestedConditionalFlip(t *testing.T) {
	oracle := getTSOracle(t)
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
	parse := func(t *testing.T, src string) *Template {
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
	render1 := func(t *testing.T, tmpl *Template, data any) (string, map[string]any) {
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

	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			tmpl := parse(t, sh.src)
			_, client := render1(t, tmpl, state(""))
			// on, off, on again: every flip, not just the first.
			for _, flash := range []string{"on", "", "on"} {
				wire, update := render1(t, tmpl, state(flash))
				if len(wire) > 100 || strings.Contains(wire, "<b>") {
					t.Errorf("flash=%q sent %d bytes, want only the flipped slot (no range markup): %s", flash, len(wire), wire)
				}
				resp, err := oracle.ApplyDiffRaw(client, update)
				if err != nil {
					t.Fatalf("flash=%q: oracle: %v", flash, err)
				}
				if m, ok := resp.Tree.(map[string]any); ok {
					client = m
				}
				if shown := strings.Contains(resp.HTML, "<p>on</p>"); shown != (flash != "") || !strings.Contains(resp.HTML, "<b>f</b>") ||
					strings.Contains(sh.src, "<em>") && strings.Contains(resp.HTML, "<em>none</em>") != (flash == "") {
					t.Errorf("flash=%q: the client shows %s", flash, normalizeHTMLForTest(resp.HTML))
				}
				// The expected page: a fresh first render of the new state, drawn by the
				// same client (an empty diff), so both sides use one renderer.
				_, fresh := render1(t, parse(t, sh.src), state(flash))
				wantResp, err := oracle.ApplyDiffRaw(fresh, map[string]any{})
				if err != nil {
					t.Fatalf("flash=%q: oracle on the fresh render: %v", flash, err)
				}
				want := wantResp.HTML
				if !htmlEquivalent(resp.HTML, want) {
					t.Errorf("flash=%q: the client shows\n  %s\nwant\n  %s\nafter update %s", flash,
						normalizeHTMLForTest(resp.HTML), normalizeHTMLForTest(want), wire)
				}
			}
		})
	}
}

// TestNestedFlipWithRangeChange: a flip and a change to the range beside it in
// the same update. The parent's own structure is unchanged, so the range is
// diffed in its own slot (an append, not a resend) and the flip in its own.
func TestNestedFlipWithRangeChange(t *testing.T) {
	oracle := getTSOracle(t)
	src := `{{if not .Ext}}<div>{{range .KB}}<b data-key="{{.}}">{{.}}</b>{{end}}</div>{{if .Flash}}<p>{{.Flash}}</p>{{end}}{{end}}`
	run := func(tmpl *Template, kb []string, flash string) (string, map[string]any) {
		t.Helper()
		var b strings.Builder
		if err := tmpl.ExecuteUpdates(&b, map[string]any{"KB": kb, "Flash": flash, "Ext": false}); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(b.String()), &m); err != nil {
			t.Fatal(err)
		}
		return b.String(), m
	}
	parse := func() *Template {
		tmpl, err := New("t")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpl.Parse(src); err != nil {
			t.Fatal(err)
		}
		return tmpl
	}
	tmpl := parse()
	_, client := run(tmpl, []string{"a", "b", "c", "d", "e", "f"}, "")
	steps := []struct {
		kb    []string
		flash string
	}{
		{[]string{"a", "b", "c", "d", "e", "f", "g"}, "on"}, // add an item and flip on
		{[]string{"b", "c", "d", "e", "f", "g"}, ""},        // remove one and flip off
	}
	for i, st := range steps {
		wire, update := run(tmpl, st.kb, st.flash)
		if strings.Count(wire, "<b data-key") > 0 || len(wire) > 200 {
			t.Errorf("step %d sent %d bytes, want the range diffed (no item markup resent): %s", i, len(wire), wire)
		}
		resp, err := oracle.ApplyDiffRaw(client, update)
		if err != nil {
			t.Fatalf("step %d: oracle: %v", i, err)
		}
		if m, ok := resp.Tree.(map[string]any); ok {
			client = m
		}
		_, fresh := run(parse(), st.kb, st.flash)
		want, err := oracle.ApplyDiffRaw(fresh, map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if !htmlEquivalent(resp.HTML, want.HTML) {
			t.Errorf("step %d: the client shows\n  %s\nwant\n  %s\nafter %s", i,
				normalizeHTMLForTest(resp.HTML), normalizeHTMLForTest(want.HTML), wire)
		}
	}
}
