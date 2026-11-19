package diff

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestRangeStream_RepeatedKeysFallBack: a stream-mode range whose keys repeat
// returns nil (full replace), never ["u", key, …] ops that the client would
// apply to the first item holding the key (#547).
func TestRangeStream_RepeatedKeysFallBack(t *testing.T) {
	statics := []string{`<li data-key="`, `">`, `</li>`}
	items := func(second string) []interface{} {
		return []interface{}{
			&build.TreeNode{Dynamics: []interface{}{"x", "first"}},
			&build.TreeNode{Dynamics: []interface{}{"x", second}},
		}
	}
	tree := &build.TreeNode{Statics: []string{"<ul>", "</ul>"}, Range: &build.RangeData{Items: items("second"), Statics: statics}}
	TransitionToStreamMode(tree)
	if tree.Range.StreamState == nil {
		t.Fatal("no stream state")
	}
	if ops := GenerateRangeStreamOperations(tree.Range.StreamState, items("changed"), statics, nil, false); ops != nil {
		t.Fatalf("repeated keys produced ops %v, want nil (full replace)", ops)
	}
}
