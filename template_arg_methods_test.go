package livetemplate

import (
	"bytes"
	"strings"
	"testing"
)

// These tests lock the supported boundary for argument-accepting template
// methods (boilerplate-reduction.md item C7). The rule is a consequence of how
// LiveTemplate renders: State is flattened into a map so the {{.lvt}} namespace
// can be injected, and that map is consumed by TWO renderers — html/template for
// the initial HTTP response (renderHTML) and the internal tree evaluator for
// WebSocket updates (buildTree). html/template cannot call an argument-accepting
// method once its receiver has been flattened into a map key, so a top-level
// State method that takes arguments is unsupported. The working idiom is to hang
// such methods off a struct FIELD (a "view helper"), exactly like the framework's
// own {{.lvt.AriaInvalid "field"}} — the field is stored in the map as a struct
// value, and both renderers call its methods natively.

// argViews is a view-helper sub-struct: its methods take arguments and are
// reached through a State field ({{.Views.Display $key}}).
type argViews struct {
	prefix string
}

func (v argViews) Display(key string) string { return v.prefix + ":" + key }

// argHelperState hangs the arg-accepting methods off the Views field and also
// declares a top-level arg-accepting method (Lookup) to pin the unsupported case.
type argHelperState struct {
	Title string
	Views argViews
}

func (s argHelperState) Lookup(key string) string { return "top:" + key }

// TestArgMethod_ViaStructField_InitialRender proves the view-helper pattern works
// through the html/template initial render path (Execute).
func TestArgMethod_ViaStructField_InitialRender(t *testing.T) {
	tmpl := Must(New("argfield"))
	if _, err := tmpl.Parse(`<span>{{.Title}}|{{.Views.Display "k1"}}</span>`); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	got := renderNormalized(t, tmpl, argHelperState{Title: "T", Views: argViews{prefix: "disp"}})
	if !strings.Contains(got, "disp:k1") {
		t.Errorf("struct-field arg-method not rendered in initial HTML: %q", got)
	}
}

// TestArgMethod_ViaStructField_UpdateTree proves the same pattern works through
// the WebSocket update path. ExecuteUpdates surfaces tree-build errors (Execute
// swallows them), so this is the honest check for the evaluator phase.
func TestArgMethod_ViaStructField_UpdateTree(t *testing.T) {
	tmpl := Must(New("argfieldupd"))
	if _, err := tmpl.Parse(`<span>{{.Title}}|{{.Views.Display "k1"}}</span>`); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	var first bytes.Buffer
	if err := tmpl.ExecuteUpdates(&first, argHelperState{Title: "A", Views: argViews{prefix: "disp"}}); err != nil {
		t.Fatalf("ExecuteUpdates() first render failed: %v", err)
	}
	if got := first.String(); !strings.Contains(got, "disp:k1") {
		t.Errorf("struct-field arg-method not present in update tree: %q", got)
	}
	// A second render with an unchanged Views output must diff it away, proving
	// the evaluator treats the method result as a normal dynamic value.
	var second bytes.Buffer
	if err := tmpl.ExecuteUpdates(&second, argHelperState{Title: "B", Views: argViews{prefix: "disp"}}); err != nil {
		t.Fatalf("ExecuteUpdates() second render failed: %v", err)
	}
	if got := second.String(); strings.Contains(got, "disp:k1") {
		t.Errorf("unchanged arg-method output should be diffed away, got: %q", got)
	}
}

// TestArgMethod_TopLevel_Unsupported pins the boundary: a top-level State method
// that takes arguments cannot be rendered, because html/template cannot call it
// once State is flattened to a map. If this ever starts passing, the boundary
// documented in boilerplate-reduction.md (C7) has moved and the docs must follow.
func TestArgMethod_TopLevel_Unsupported(t *testing.T) {
	tmpl := Must(New("argtop"))
	if _, err := tmpl.Parse(`<span>{{.Lookup "k1"}}</span>`); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, argHelperState{})
	if err == nil {
		t.Fatalf("expected top-level arg-method to be unsupported, but Execute succeeded: %q", buf.String())
	}
	// The boundary is "it errors" (asserted above). We also check the error names
	// the offending method so a failure points at the right cause, without pinning
	// html/template's exact wording (which isn't covered by Go's compat promise).
	if !strings.Contains(err.Error(), "Lookup") {
		t.Errorf("expected error to reference the top-level arg-method Lookup, got: %v", err)
	}
}
