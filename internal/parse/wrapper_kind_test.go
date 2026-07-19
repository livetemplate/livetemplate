package parse

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// keyedChild is the <li data-key="…"> a range item hides one level down.
func keyedChild() *TreeNode {
	child := NewTreeNode()
	child.Statics = []string{`<li data-key="`, `">`, `</li>`}
	child.SetDynamic(0, "/a.go")
	child.SetDynamic(1, "a.go")
	return child
}

// TestWrappedItemKey_RequiresTheMarker is the guard the #497 fix exists for.
//
// The two nodes below are byte-identical in every respect the old structural
// predicate could see — `["", ""]` statics, exactly one nested *TreeNode child,
// child carries data-key — and differ only in whether the parser recorded that
// it created the node as a wrapper. The old test could not tell them apart and
// keyed both by the child's data-key; only the tagged one qualifies now.
//
// No template reachable today produces the untagged shape: createWrapper is the
// only thing that nests a TreeNode under empty statics, and field nodes hold
// strings (createSingleDynamicTree). So this is constructed by hand deliberately
// — the fix is a guard against a future construct acquiring through-wrapper
// keying by coincidence, not a fix for observed breakage.
func TestWrappedItemKey_RequiresTheMarker(t *testing.T) {
	tagged := NewTreeNode()
	tagged.Statics = defaultFieldStatics
	tagged.Wrapper = build.WrapperInvocation
	tagged.SetDynamic(0, keyedChild())

	untagged := NewTreeNode()
	untagged.Statics = defaultFieldStatics
	untagged.SetDynamic(0, keyedChild())

	if key, ok := wrappedItemKey(tagged); !ok || key != "/a.go" {
		t.Errorf("tagged wrapper must key by the child's data-key, got (%q, %v)", key, ok)
	}

	if key, ok := wrappedItemKey(untagged); ok {
		t.Errorf("untagged node must not inherit through-wrapper keying, got %q", key)
	}
}

// TestWrappedItemKey_AcceptsEveryWrapperKind pins the deliberate choice not to
// narrow this to WrapperInvocation. The child's real data-key is the right
// identity for a conditional-wrapped item too, and requiring WrapperInvocation
// would drop {{if}}/{{with}}-wrapped keyed ranges to content hashing — a keying
// regression rather than a fix. Verified against main: if_wrapped_keyed_range
// keys by data-key there.
func TestWrappedItemKey_AcceptsEveryWrapperKind(t *testing.T) {
	for _, kind := range []build.WrapperKind{build.WrapperConditional, build.WrapperInvocation} {
		item := NewTreeNode()
		item.Statics = defaultFieldStatics
		item.Wrapper = kind
		item.SetDynamic(0, keyedChild())

		if key, ok := wrappedItemKey(item); !ok || key != "/a.go" {
			t.Errorf("wrapper kind %v must key by data-key, got (%q, %v)", kind, key, ok)
		}
	}
}

// TestWalkList_CarriesWrapperAcrossTheMerge guards the step the issue did not
// anticipate. walkList does not keep a wrapper as a node: it merges the
// wrapper's statics and dynamics into a fresh tree, so what reaches
// wrappedItemKey has the wrapper's shape without being the wrapper. If the kind
// did not survive that merge the marker would never be observed and EVERY
// through-wrapper keying would silently regress to content hashing — a far wider
// break than the coincidence the marker is meant to prevent.
func TestWalkList_CarriesWrapperAcrossTheMerge(t *testing.T) {
	src := `{{range .Items}}{{if .Name}}<li data-key="{{.Path}}">{{.Name}}</li>{{end}}{{end}}`
	type item struct{ Name, Path string }
	data := struct{ Items []item }{Items: []item{{Name: "a.go", Path: "/a.go"}}}

	tmpl, err := Parse(src, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tree, err := BuildTree(tmpl, data, build.NewContext())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tree.Range == nil || len(tree.Range.Items) != 1 {
		t.Fatalf("expected a one-item range, got %#v", tree.Range)
	}
	itemTree, ok := tree.Range.Items[0].(*TreeNode)
	if !ok {
		t.Fatalf("range item is %T, want *TreeNode", tree.Range.Items[0])
	}

	// Assert the resulting key, not the marker. Items are rebuilt by
	// extractItemDynamics before they reach RangeData, so the stored node is a
	// derived one that never carried the kind — the marker is consumed earlier,
	// by wrappedItemKey. The key is the observable proof it arrived: drop the
	// propagation in walkList and this becomes a content hash.
	if itemTree.AutoKey != "/a.go" {
		t.Errorf("expected the item keyed by its child's data-key, got %q — "+
			"the wrapper kind did not survive walkList's merge, so through-wrapper "+
			"keying fell back to content hashing", itemTree.AutoKey)
	}
}
