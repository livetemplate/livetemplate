package parse

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestVarDeclaration_Basic tests that {{$c := .}} is properly handled
// and $c references work throughout the template.
func TestVarDeclaration_Basic(t *testing.T) {
	type Data struct {
		Name  string
		Title string
	}

	tmplStr := `{{$c := .}}<div>{{$c.Name}}</div><span>{{$c.Title}}</span>`
	data := Data{Name: "hello", Title: "world"}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	d0, ok := tree.GetDynamic("0")
	if !ok {
		t.Fatal("missing dynamic 0")
	}
	if d0 != "hello" {
		t.Errorf("dynamic 0 = %v, want hello", d0)
	}

	d1, ok := tree.GetDynamic("1")
	if !ok {
		t.Fatal("missing dynamic 1")
	}
	if d1 != "world" {
		t.Errorf("dynamic 1 = %v, want world", d1)
	}

	if len(tree.Statics) != 3 {
		t.Fatalf("statics length = %d, want 3", len(tree.Statics))
	}
	if tree.Statics[0] != "<div>" {
		t.Errorf("statics[0] = %q, want <div>", tree.Statics[0])
	}
	if tree.Statics[1] != "</div><span>" {
		t.Errorf("statics[1] = %q, want </div><span>", tree.Statics[1])
	}
	if tree.Statics[2] != "</span>" {
		t.Errorf("statics[2] = %q, want </span>", tree.Statics[2])
	}
}

// TestVarDeclaration_WithRange tests $c access inside range blocks.
func TestVarDeclaration_WithRange(t *testing.T) {
	type Data struct {
		Class string
		Items []string
	}

	tmplStr := `{{$c := .}}<ul class="{{$c.Class}}">{{range $c.Items}}<li class="{{$c.Class}}">{{.}}</li>{{end}}</ul>`
	data := Data{Class: "my-class", Items: []string{"a", "b"}}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	d0, ok := tree.GetDynamic("0")
	if !ok {
		t.Fatal("missing dynamic 0 (outer class)")
	}
	if d0 != "my-class" {
		t.Errorf("dynamic 0 = %v, want my-class", d0)
	}

	d1, ok := tree.GetDynamic("1")
	if !ok {
		t.Fatal("missing dynamic 1 (range)")
	}
	rangeTree, ok := d1.(*TreeNode)
	if !ok {
		t.Fatalf("dynamic 1 is not a TreeNode: %T", d1)
	}
	if rangeTree.Range == nil {
		t.Fatal("range tree has no Range data")
	}

	items := rangeTree.Range.Items
	if len(items) != 2 {
		t.Fatalf("range items = %d, want 2", len(items))
	}

	for i, item := range items {
		itemTree, ok := item.(*TreeNode)
		if !ok {
			t.Fatalf("range item %d is not a TreeNode: %T", i, item)
		}
		classVal, ok := itemTree.GetDynamic("0")
		if !ok {
			t.Fatalf("range item %d missing dynamic 0", i)
		}
		if classVal != "my-class" {
			t.Errorf("range item %d class = %v, want my-class", i, classVal)
		}
	}
}

// TestVarDeclaration_WithIf tests $c access inside if blocks.
func TestVarDeclaration_WithIf(t *testing.T) {
	type Data struct {
		Show  bool
		Label string
	}

	tmplStr := `{{$c := .}}{{if $c.Show}}<span>{{$c.Label}}</span>{{end}}`
	data := Data{Show: true, Label: "visible"}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	d0, ok := tree.GetDynamic("0")
	if !ok {
		t.Fatal("missing dynamic 0")
	}
	ifTree, ok := d0.(*TreeNode)
	if !ok {
		t.Fatalf("dynamic 0 is not a TreeNode: %T", d0)
	}
	labelVal, ok := ifTree.GetDynamic("0")
	if !ok {
		t.Fatal("missing if-body dynamic 0")
	}
	if labelVal != "visible" {
		t.Errorf("label = %v, want visible", labelVal)
	}
}

// TestVarDeclaration_WithWith tests $c access inside with blocks.
func TestVarDeclaration_WithWith(t *testing.T) {
	type Inner struct {
		Value string
	}
	type Data struct {
		Style string
		Inner *Inner
	}

	tmplStr := `{{$c := .}}{{with $c.Inner}}<div class="{{$c.Style}}">{{.Value}}</div>{{end}}`
	data := Data{Style: "bold", Inner: &Inner{Value: "content"}}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	// With blocks don't wrap -- their content merges into the parent tree.
	// $c.Style and .Value should appear as direct dynamics.
	d0, ok := tree.GetDynamic("0")
	if !ok {
		t.Fatal("missing dynamic 0 (style)")
	}
	if d0 != "bold" {
		t.Errorf("dynamic 0 (style) = %v, want bold", d0)
	}

	d1, ok := tree.GetDynamic("1")
	if !ok {
		t.Fatal("missing dynamic 1 (value)")
	}
	if d1 != "content" {
		t.Errorf("dynamic 1 (value) = %v, want content", d1)
	}
}

// TestVarDeclaration_NestedRange tests $c access inside nested range blocks.
func TestVarDeclaration_NestedRange(t *testing.T) {
	type Data struct {
		Style string
		Weeks [][]string
	}

	tmplStr := `{{$c := .}}<div>{{range $c.Weeks}}<div>{{range .}}<span class="{{$c.Style}}">{{.}}</span>{{end}}</div>{{end}}</div>`
	data := Data{
		Style: "day-class",
		Weeks: [][]string{{"Mon", "Tue"}, {"Wed"}},
	}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	if tree == nil {
		t.Fatal("tree is nil")
	}

	d0, ok := tree.GetDynamic("0")
	if !ok {
		t.Fatal("missing dynamic 0 (outer range)")
	}
	outerRange, ok := d0.(*TreeNode)
	if !ok {
		t.Fatalf("dynamic 0 is not a TreeNode: %T", d0)
	}
	if outerRange.Range == nil {
		t.Fatal("outer range has no Range data")
	}
	if len(outerRange.Range.Items) != 2 {
		t.Fatalf("outer range items = %d, want 2", len(outerRange.Range.Items))
	}
}

// TestVarDeclaration_MixedWithDot tests using both $c and . in templates.
func TestVarDeclaration_MixedWithDot(t *testing.T) {
	type Data struct {
		ID    string
		Label string
	}

	tmplStr := `{{$c := .}}<div id="{{$c.ID}}">{{.Label}}</div>`
	data := Data{ID: "my-id", Label: "my-label"}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	d0, _ := tree.GetDynamic("0")
	if d0 != "my-id" {
		t.Errorf("dynamic 0 (ID) = %v, want my-id", d0)
	}

	d1, _ := tree.GetDynamic("1")
	if d1 != "my-label" {
		t.Errorf("dynamic 1 (Label) = %v, want my-label", d1)
	}
}

// TestVarDeclaration_WithFuncMap tests $c access with a custom func map.
func TestVarDeclaration_WithFuncMap(t *testing.T) {
	tmplStr := `{{$c := .}}<div>{{$c.Name}}</div>`

	type NamedData struct {
		Name string
	}
	data := NamedData{Name: "test"}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	d0, _ := tree.GetDynamic("0")
	if d0 != "test" {
		t.Errorf("dynamic 0 = %v, want test", d0)
	}
}

// TestVarDeclaration_WithRangeVarDecl tests $c with range that also has variable declarations.
func TestVarDeclaration_WithRangeVarDecl(t *testing.T) {
	type Data struct {
		Style string
		Items []string
	}

	tmplStr := `{{$c := .}}{{range $idx, $item := $c.Items}}<span class="{{$c.Style}}">{{$idx}}:{{$item}}</span>{{end}}`
	data := Data{Style: "item-class", Items: []string{"a", "b"}}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, nil, ctx)
	if err != nil {
		t.Fatalf("BuildTree error: %v", err)
	}

	if tree == nil {
		t.Fatal("tree is nil")
	}

	d0, ok := tree.GetDynamic("0")
	if !ok {
		t.Fatal("missing dynamic 0")
	}
	rangeTree, ok := d0.(*TreeNode)
	if !ok {
		t.Fatalf("dynamic 0 is not TreeNode: %T", d0)
	}
	if rangeTree.Range == nil {
		t.Fatal("no Range data")
	}

	items := rangeTree.Range.Items
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}

	for i, item := range items {
		itemTree, ok := item.(*TreeNode)
		if !ok {
			t.Fatalf("item %d not TreeNode", i)
		}
		styleVal, ok := itemTree.GetDynamic("0")
		if !ok {
			t.Fatalf("item %d missing dynamic 0", i)
		}
		if styleVal != "item-class" {
			t.Errorf("item %d style = %v, want item-class", i, styleVal)
		}
	}
}
