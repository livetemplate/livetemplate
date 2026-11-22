package diff

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestOwnStructureChanged pins which differences make a nested node go whole.
// A change below the node (a child's own shape) doesn't: the recursion diffs that
// child in its own slot (prereview#273).
func TestOwnStructureChanged(t *testing.T) {
	child := func(st ...string) *TreeNode { return &TreeNode{Statics: st, Dynamics: []interface{}{"x"}} }
	node := func(st []string, dyn ...interface{}) *TreeNode { return &TreeNode{Statics: st, Dynamics: dyn} }
	ab := []string{"<a>", "</a>"}
	abc := []string{"<a>", "|", "</a>"}
	ranged := func(itemStatics ...string) *TreeNode {
		return &TreeNode{Statics: []string{"", ""}, Range: &build.RangeData{Statics: itemStatics}}
	}
	for _, tc := range []struct {
		name     string
		old, new *TreeNode
		want     bool
	}{
		{"identical", node(ab, "x"), node(ab, "y"), false},
		{"a child's own statics changed", node(ab, child("<p>", "</p>")), node(ab, child("<em>", "</em>")), false},
		{"a child slot went from text to a node", node(ab, ""), node(ab, child("<p>", "</p>")), false},
		{"different statics", node(ab, "x"), node([]string{"<b>", "</b>"}, "x"), true},
		{"different slot count", node(ab, "x"), node(abc, "x", "y"), true},
		{"a slot became nil", node(abc, "x", "y"), node(abc, "x", nil), true},
		{"a nil slot got filled", node(abc, "x", nil), node(abc, "x", "y"), true},
		{"range item statics changed", ranged("<li>", "</li>"), ranged("<tr>", "</tr>"), true},
		{"range unchanged", ranged("<li>", "</li>"), ranged("<li>", "</li>"), false},
	} {
		if got := ownStructureChanged(tc.old, tc.new); got != tc.want {
			t.Errorf("%s: ownStructureChanged = %v, want %v", tc.name, got, tc.want)
		}
	}
}
