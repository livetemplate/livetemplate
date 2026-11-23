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
