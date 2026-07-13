package livetemplate

import (
	"bytes"
	"strings"
	"testing"
)

// A range item whose body contains a nested template that renders NOTHING for an
// empty argument has a data-dependent slot shape: the item's statics differ
// between the render where the nested template is empty and the one where it
// isn't. That makes the range fail signature matching (FindRangeConstructMatches
// pairs ranges by their item statics), so the range-diff ops — the only channel a
// matched range has for item changes — never run.
//
// The regression this guards: when that happened, the diff fell through to the
// nested-node path, which found nothing to send (a range's content lives in
// Range.Items, not Dynamics) and emitted an EMPTY update. The change was silently
// dropped and the client kept showing the stale item forever, with no error
// anywhere. An unmatched range whose structure changed must send the full subtree.
func TestRangeItemStaticsChange_UpdateIsNotDropped(t *testing.T) {
	// The second range matters: with exactly one range on each side,
	// FindRangeConstructMatches falls back to matching by path, which papers over
	// the signature mismatch. A real page has several, so the fallback doesn't
	// apply and the unmatched range takes the broken path.
	const src = `<ul>{{range .Tags}}<li class="tag">{{.}}</li>{{end}}</ul>
<ul>{{range .Items}}<li data-key="{{.ID}}"><span>{{.Body}}</span>{{template "note" .Note}}</li>{{end}}</ul>
{{define "note"}}{{if .}}<em class="note">{{.}}</em>{{end}}{{end}}`

	type item struct {
		ID   string
		Body string
		Note string
	}
	type state struct {
		Tags  []string
		Items []item
	}

	tmpl := Must(New("notes"))
	if _, err := tmpl.Parse(src); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// First render: the note is empty, so {{template "note"}} contributes no slot.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, state{Tags: []string{"go"}, Items: []item{{ID: "a", Body: "hello"}}}); err != nil {
		t.Fatalf("initial Execute: %v", err)
	}

	// Second render: same item key, but the note now renders — the item's slot
	// shape changes, so the range no longer matches by signature.
	const note = "now it has a note"
	buf.Reset()
	if err := tmpl.ExecuteUpdates(&buf, state{Tags: []string{"go"}, Items: []item{{ID: "a", Body: "hello", Note: note}}}); err != nil {
		t.Fatalf("ExecuteUpdates: %v", err)
	}
	if !strings.Contains(buf.String(), note) {
		t.Errorf("the changed item never reaches the client: it is absent from the update payload, "+
			"so the rendered list stays stale until a full page load.\npayload: %s", buf.String())
	}
}
