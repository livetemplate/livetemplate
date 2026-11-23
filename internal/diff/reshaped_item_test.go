package diff

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestPrepareAgainst pins how a reshaped stream-mode item's nested node is
// encoded: statics stripped where the exemplar's slot has the same shape (the
// client has them), kept anywhere the shape is new.
func TestPrepareAgainst(t *testing.T) {
	p := &TreeNode{Statics: []string{"<p>", "</p>"}, Dynamics: []interface{}{"x"}}
	same := &TreeNode{Statics: []string{"<p>", "</p>"}, Dynamics: []interface{}{"old"}}
	other := &TreeNode{Statics: []string{"<em>", "</em>"}, Dynamics: []interface{}{"x"}}
	ranged := &TreeNode{Statics: []string{"<p>", "</p>"}, Range: &build.RangeData{}}
	hasStatics := func(v interface{}) bool {
		switch n := v.(type) {
		case *TreeNode:
			return len(n.Statics) > 0
		case map[string]interface{}:
			_, ok := n["s"]
			return ok
		}
		return false
	}
	for _, tc := range []struct {
		name string
		ref  interface{}
		want bool // statics kept
	}{
		{"no slot in the exemplar", nil, true},
		{"the exemplar slot is text", "", true},
		{"the exemplar slot has other statics", other, true},
		{"the exemplar slot holds a range", ranged, true},
		{"the exemplar slot has the same statics", same, false},
	} {
		if got := hasStatics(prepareAgainst(p, tc.ref)); got != tc.want {
			t.Errorf("%s: statics kept = %v, want %v", tc.name, got, tc.want)
		}
	}
	// Same shape at the top, a flip one level down: only the inner node keeps statics.
	outer := &TreeNode{Statics: []string{"<div>", "</div>"}, Dynamics: []interface{}{p}}
	refOuter := &TreeNode{Statics: []string{"<div>", "</div>"}, Dynamics: []interface{}{""}}
	got, _ := prepareAgainst(outer, refOuter).(map[string]interface{})
	if got == nil || hasStatics(got) || !hasStatics(got["0"]) {
		t.Errorf("want the outer node stripped and the inner one with statics, got %#v", got)
	}
}

func TestItemUsesRangeTemplate(t *testing.T) {
	rs := []string{"<li>", "</li>"}
	for _, tc := range []struct {
		name string
		item *TreeNode
		want bool
	}{
		{"no statics of its own", &TreeNode{}, true},
		{"the range's statics", &TreeNode{Statics: []string{"<li>", "</li>"}}, true},
		{"statics of its own", &TreeNode{Statics: []string{"<tr>", "</tr>"}}, false},
	} {
		if got := itemUsesRangeTemplate(tc.item, rs); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A stream state with no exemplar can't scope a reshaped item's statics, so the
// range falls back to a full replace rather than sending a stripped ["u"].
func TestStreamOps_ReshapedWithoutExemplar_FallsBack(t *testing.T) {
	item := func(nested *TreeNode) *TreeNode {
		return &TreeNode{Dynamics: []interface{}{"k1", nested}}
	}
	statics := []string{`<li data-key="`, `">`, `</li>`}
	old := item(&TreeNode{Statics: []string{""}})
	state := &build.RangeStreamState{
		Keys:        []string{"k1"},
		Hashes:      []uint64{1},
		Fingerprint: build.CalculateStaticsFingerprint(old),
	}
	reshaped := item(&TreeNode{Statics: []string{"<b>", "</b>"}, Dynamics: []interface{}{"x"}})
	if ops := GenerateRangeStreamOperations(state, []interface{}{reshaped}, statics, nil, true); ops != nil {
		t.Errorf("want nil (full replace) without an exemplar, got %v", ops)
	}
	state.Exemplar = exemplarShape(old)
	if ops := GenerateRangeStreamOperations(state, []interface{}{reshaped}, statics, nil, true); len(ops) != 1 {
		t.Errorf("with an exemplar, want one [\"u\"], got %v", ops)
	}
}

// exemplarShape keeps structure and drops values.
func TestExemplarShape(t *testing.T) {
	in := &TreeNode{Statics: []string{"<p>", "|", "</p>"}, Dynamics: []interface{}{
		"secret value", &TreeNode{Statics: []string{"<b>", "</b>"}, Dynamics: []interface{}{"inner value"}}, nil,
	}}
	out := exemplarShape(in)
	inner, ok := out.Dynamics[1].(*TreeNode)
	if out.Dynamics[0] != "" || !ok || inner.Dynamics[0] != "" || out.Dynamics[2] != nil || len(inner.Statics) != 2 {
		t.Errorf("want statics and slot kinds kept, values dropped: %#v", out)
	}
}
