package livetemplate

import (
	"fmt"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/compat"
	"github.com/livetemplate/livetemplate/internal/render"
)

// chainDir builds a linear chain of `depth` nested directories, each holding the
// next as its single child (the leaf is a file). Used to exercise the depth cap
// with a finite-but-deep tree.
func chainDir(depth int) fileNode {
	n := fileNode{Name: fmt.Sprint(depth), Path: fmt.Sprintf("/%d", depth)}
	if depth > 0 {
		n.IsDir = true
		n.Children = []fileNode{chainDir(depth - 1)}
	}
	return n
}

// fileNode is a self-referential tree, the canonical recursive-template case
// (a file browser / comment thread / org chart). Rendering it requires the
// {{template "treeNode" .}} call inside the {{range .Children}} to be evaluated
// recursively at build time.
type fileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []fileNode
}

const recursiveTreeSrc = `{{define "treeNode"}}<li data-key="{{.Path}}"><span>{{.Name}}</span>` +
	`{{if .IsDir}}<ul>{{range .Children}}{{template "treeNode" .}}{{end}}</ul>{{end}}` +
	`</li>{{end}}` +
	`<ul>{{template "treeNode" .}}</ul>`

// buildTreeHTML parses src, builds the tree through the reactive tree path
// (parse.BuildTree → walkAST → invokeTemplate — NOT html/template Execute, which
// would render recursion natively and thus not exercise our code), and
// reconstructs the HTML the client would render from that tree.
func buildTreeHTML(t *testing.T, src string, data interface{}) (string, error) {
	t.Helper()
	tmpl, err := Must(New("test")).Parse(src)
	if err != nil {
		return "", err
	}
	tree, err := tmpl.buildTree(data, nil)
	if err != nil {
		return "", err
	}
	return render.TreeToHTML(tree.ToMap())
}

// TestRecursiveTemplate_RendersNestedTree is the core proof for C8: a recursive
// {{template}} now renders through the reactive tree path instead of crashing at
// parse time. The reconstructed HTML must reproduce the full nested structure.
func TestRecursiveTemplate_RendersNestedTree(t *testing.T) {
	data := fileNode{
		Name: "root", Path: "/", IsDir: true,
		Children: []fileNode{
			{Name: "a.go", Path: "/a.go"},
			{
				Name: "sub", Path: "/sub", IsDir: true,
				Children: []fileNode{
					{Name: "b.go", Path: "/sub/b.go"},
				},
			},
		},
	}

	html, err := buildTreeHTML(t, recursiveTreeSrc, data)
	if err != nil {
		t.Fatalf("recursive template must render, got error: %v", err)
	}

	want := `<ul><li data-key="/"><span>root</span><ul>` +
		`<li data-key="/a.go"><span>a.go</span></li>` +
		`<li data-key="/sub"><span>sub</span><ul>` +
		`<li data-key="/sub/b.go"><span>b.go</span></li>` +
		`</ul></li>` +
		`</ul></li></ul>`
	if html != want {
		t.Errorf("recursive render mismatch\nwant: %s\ngot:  %s", want, html)
	}
}

// TestRecursiveTemplate_SingleLevel is the base case: a "directory" with no
// children must terminate cleanly (the {{range}} over an empty slice invokes
// nothing), proving recursion stops when the data does.
func TestRecursiveTemplate_SingleLevel(t *testing.T) {
	data := fileNode{Name: "empty", Path: "/empty", IsDir: true}

	html, err := buildTreeHTML(t, recursiveTreeSrc, data)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	want := `<ul><li data-key="/empty"><span>empty</span><ul></ul></li></ul>`
	if html != want {
		t.Errorf("base-case mismatch\nwant: %s\ngot:  %s", want, html)
	}
}

// TestRecursiveTemplate_MutualRecursion proves the registry holds more than one
// cycle member: two templates that call each other, terminated by the data.
func TestRecursiveTemplate_MutualRecursion(t *testing.T) {
	// a renders a <div> then, if it has a child, delegates to b; b renders a
	// <span> then delegates back to a. Termination is by nil child.
	src := `{{define "a"}}<div>{{.Name}}{{with .Next}}{{template "b" .}}{{end}}</div>{{end}}` +
		`{{define "b"}}<span>{{.Name}}{{with .Next}}{{template "a" .}}{{end}}</span>{{end}}` +
		`{{template "a" .}}`

	type chain struct {
		Name string
		Next interface{}
	}
	data := chain{Name: "1", Next: chain{Name: "2", Next: chain{Name: "3"}}}

	html, err := buildTreeHTML(t, src, data)
	if err != nil {
		t.Fatalf("mutual recursion must render, got error: %v", err)
	}
	want := `<div>1<span>2<div>3</div></span></div>`
	if html != want {
		t.Errorf("mutual-recursion mismatch\nwant: %s\ngot:  %s", want, html)
	}
}

// TestRecursiveTemplate_DepthGuard_TreePath proves the reactive depth guard is
// the sole thing standing between unbounded recursion and a Go stack overflow on
// the tree-build path that does NOT go through html/template Execute (which has
// its own depth limit). compat.ParseTemplateToTree walks straight into
// parse.BuildTree → invokeTemplate, so our guard must fire on infinite data.
func TestRecursiveTemplate_DepthGuard_TreePath(t *testing.T) {
	type loopNode struct {
		Name string
		Self interface{}
	}
	n := &loopNode{Name: "x"}
	n.Self = n // infinite: Self is always non-nil

	// The {{define}} block is present in the source, so parse.Parse registers
	// "loop" as a recursive template even without the flatten step.
	src := `{{define "loop"}}<div>{{.Name}}{{with .Self}}{{template "loop" .}}{{end}}</div>{{end}}` +
		`{{template "loop" .}}`

	_, err := compat.ParseTemplateToTree("test", src, n)
	if err == nil {
		t.Fatal("expected the reactive depth guard to stop unbounded recursion, got nil")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Errorf("error should explain the depth limit, got: %v", err)
	}
}

// TestRecursiveTemplate_DepthGuard_NoCrash proves the full public path
// (buildTree → Execute → tree) never crashes on unbounded recursion — it returns
// a clean depth error instead. (On this path html/template Execute's own depth
// limit trips first; either way the outcome is a bounded error, not overflow.)
func TestRecursiveTemplate_DepthGuard_NoCrash(t *testing.T) {
	type loopNode struct {
		Name string
		Self interface{}
	}
	n := &loopNode{Name: "x"}
	n.Self = n

	src := `{{define "loop"}}<div>{{.Name}}{{with .Self}}{{template "loop" .}}{{end}}</div>{{end}}` +
		`{{template "loop" .}}`

	_, err := buildTreeHTML(t, src, n)
	if err == nil {
		t.Fatal("expected a depth-limit error for unbounded recursion, got nil")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Errorf("error should explain the depth limit, got: %v", err)
	}
}

// TestRecursiveTemplate_ConfigurableDepth proves WithMaxTemplateDepth is wired
// end-to-end from Option → Config → build.Context → invokeTemplate. A first
// render within the limit establishes the reactive tree; a subsequent update
// that grows past the limit is rejected with a depth error (the update path
// propagates the guard, unlike the first-render path which degrades to the HTML
// fallback). The default (128) renders the same deep tree without complaint.
func TestRecursiveTemplate_ConfigurableDepth(t *testing.T) {
	// Default limit: a depth-6 tree renders fine (well under 128).
	if _, err := buildTreeHTML(t, recursiveTreeSrc, chainDir(6)); err != nil {
		t.Fatalf("depth-6 tree must render under the default limit, got: %v", err)
	}

	// Lowered limit of 3: first render a shallow tree (within the limit) to
	// establish the reactive baseline, then update to a deep tree.
	tmpl, err := Must(New("test", WithMaxTemplateDepth(3))).Parse(recursiveTreeSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := tmpl.buildTree(chainDir(2), nil); err != nil {
		t.Fatalf("shallow first render (within limit) must succeed, got: %v", err)
	}
	_, err = tmpl.buildTree(chainDir(6), nil)
	if err == nil {
		t.Fatal("expected a depth error updating past WithMaxTemplateDepth(3), got nil")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Errorf("error should explain the depth limit, got: %v", err)
	}
}

// TestParse_NonRecursiveComposition_StillWorks is the companion positive gate:
// ordinary (acyclic) {{define}}/{{template}} composition must keep parsing and
// rendering exactly as before the recursion feature landed.
func TestParse_NonRecursiveComposition_StillWorks(t *testing.T) {
	src := `{{define "header"}}<h1>{{.Title}}</h1>{{end}}` +
		`{{template "header" .}}<main>{{.Body}}</main>`
	data := struct {
		Title string
		Body  string
	}{Title: "Hi", Body: "world"}

	html, err := buildTreeHTML(t, src, data)
	if err != nil {
		t.Fatalf("non-recursive composition must render, got: %v", err)
	}
	want := `<h1>Hi</h1><main>world</main>`
	if html != want {
		t.Errorf("non-recursive composition mismatch\nwant: %s\ngot:  %s", want, html)
	}
}
