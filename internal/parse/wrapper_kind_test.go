package parse

import (
	"strings"
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
// would drop {{if}}-wrapped keyed ranges to content hashing — a keying
// regression rather than a fix. Verified against main, where an {{if}}-wrapped
// keyed range keys by data-key.
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

// TestWalkList_WrapperSurvivesSurroundingText is the regression guard for the
// bug an earlier revision of this change shipped.
//
// walkList propagated the kind when exactly one child *contributed statics*.
// A TextNode yields one static even when it is only the whitespace around an
// indented construct, so that count rose with formatting: the minified body
// below has one child, the indented one has three (text, if, text), and the
// container-wrapped one has the <li> text as well. The tag stopped propagating
// for every shape but the minified one, dropping realistic keyed ranges to
// content hashing — the exact regression this change exists to avoid, arriving
// through formatting rather than through wrapper kind.
//
// Counting wrappers instead of contributors is what makes keying independent of
// how the template happens to be laid out. All three cases below key by
// data-key on main; they must keep doing so.
func TestWalkList_WrapperSurvivesSurroundingText(t *testing.T) {
	type item struct{ Name, Path string }
	data := struct{ Items []item }{Items: []item{{Name: "a.go", Path: "/a.go"}}}

	cases := []struct{ name, src string }{
		{"minified", `{{range .Items}}{{if .Name}}<li data-key="{{.Path}}">{{.Name}}</li>{{end}}{{end}}`},
		{"indented", "{{range .Items}}\n  {{if .Name}}<li data-key=\"{{.Path}}\">{{.Name}}</li>{{end}}\n{{end}}"},
		{"container wrapped", `{{range .Items}}<li>{{if .Name}}<span data-key="{{.Path}}">{{.Name}}</span>{{end}}</li>{{end}}`},
		{"text sibling", `{{range .Items}}x{{if .Name}}<li data-key="{{.Path}}">{{.Name}}</li>{{end}}{{end}}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl, err := Parse(c.src, nil)
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
			if itemTree.AutoKey != "/a.go" {
				t.Errorf("keyed by %q, want the child's data-key %q — surrounding text "+
					"changed whether the wrapper tag propagated", itemTree.AutoKey, "/a.go")
			}
		})
	}
}

// TestWrappedItemKey_DescendsNestedWrappers covers issue #505: nested constructs
// stack wrappers, so {{if}}{{if}}<li data-key=…> puts the keyed element two
// levels below the range item. Looking only one level down meant those items
// fell back to content hashing despite carrying a perfectly good explicit key —
// losing stable identity, so a change to one item made the client rebuild the
// row instead of patching it.
//
// Descending is safe only because wrappers are tagged (#497): each step tests a
// node the parser marked, never a shape that happens to resemble one.
func TestWrappedItemKey_DescendsNestedWrappers(t *testing.T) {
	type item struct{ Name, Path string }
	data := struct{ Items []item }{Items: []item{{Name: "a.go", Path: "/a.go"}}}

	keyed := func(depth int) string {
		var b strings.Builder
		b.WriteString("{{range .Items}}")
		for i := 0; i < depth; i++ {
			b.WriteString("{{if .Name}}")
		}
		b.WriteString(`<li data-key="{{.Path}}">{{.Name}}</li>`)
		for i := 0; i < depth; i++ {
			b.WriteString("{{end}}")
		}
		b.WriteString("{{end}}")
		return b.String()
	}

	tests := []struct {
		name    string
		src     string
		wantKey string // "" means content-hash fallback
	}{
		{"one wrapper", keyed(1), "/a.go"},
		{"two wrappers", keyed(2), "/a.go"},
		{"three wrappers", keyed(3), "/a.go"},
		{"at the descent limit", keyed(maxWrapperDescent), "/a.go"},
		{
			// Bounded deliberately: allWrappedItemKeys is all-or-nothing, so an
			// item that never yields a key makes the whole range pay the full
			// descent on every render. Falling back here is correct, not a gap.
			name: "beyond the descent limit falls back", src: keyed(maxWrapperDescent + 1), wantKey: "",
		},
		{
			// Nothing to find however deep we look.
			name: "nested but unkeyed falls back",
			src:  `{{range .Items}}{{if .Name}}{{if .Path}}<li>{{.Name}}</li>{{end}}{{end}}{{end}}`, wantKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.src, nil)
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

			if tt.wantKey != "" {
				if itemTree.AutoKey != tt.wantKey {
					t.Errorf("keyed by %q, want the child's data-key %q", itemTree.AutoKey, tt.wantKey)
				}
				return
			}
			if itemTree.AutoKey == "/a.go" {
				t.Errorf("expected a content hash, got the data-key %q", itemTree.AutoKey)
			}
		})
	}
}

// TestSoleNestedChild pins the "exactly one" rule the descent relies on. A node
// holding two nested trees is real content, not a wrapper around a single
// element, so there is no one child for the item's identity to come from.
func TestSoleNestedChild(t *testing.T) {
	one := NewTreeNode()
	one.SetDynamic(0, NewTreeNode())
	if soleNestedChild(one) == nil {
		t.Error("a node with exactly one nested child must yield it")
	}

	two := NewTreeNode()
	two.SetDynamic(0, NewTreeNode())
	two.SetDynamic(1, NewTreeNode())
	if soleNestedChild(two) != nil {
		t.Error("a node with two nested children must yield none")
	}

	none := NewTreeNode()
	none.SetDynamic(0, "text")
	if soleNestedChild(none) != nil {
		t.Error("a node whose dynamics are all strings must yield none")
	}
}
