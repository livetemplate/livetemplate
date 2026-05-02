package parse

import (
	"fmt"
	"html/template"
	"reflect"
	"testing"
	"text/template/parse"
)

// TestParse_SimpleTemplate tests parsing a simple template.
func TestParse_SimpleTemplate(t *testing.T) {
	tmpl, err := Parse("<div>{{.Name}}</div>", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template")
	}

	if tmpl.ast == nil {
		t.Error("Expected non-nil AST")
	}
}

// TestParse_WithFuncMap tests parsing with function map.
func TestParse_WithFuncMap(t *testing.T) {
	funcMap := template.FuncMap{
		"upper": func(s string) string { return s },
	}

	tmpl, err := Parse("{{.Name | upper}}", funcMap)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template")
	}
}

// TestParse_InvalidSyntax tests parsing with syntax errors.
func TestParse_InvalidSyntax(t *testing.T) {
	_, err := Parse("{{.Name", nil)
	if err == nil {
		t.Error("Expected error for invalid syntax")
	}
}

// TestParse_EmptyTemplate tests parsing an empty template.
func TestParse_EmptyTemplate(t *testing.T) {
	tmpl, err := Parse("", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template for empty string")
	}
}

// TestBuildTree_SimpleField tests building tree from simple field.
func TestBuildTree_SimpleField(t *testing.T) {
	tmpl, err := Parse("{{.Name}}", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics[0])
	}
}

// TestBuildTree_NestedFields tests building tree with nested data.
func TestBuildTree_NestedFields(t *testing.T) {
	tmpl, err := Parse("{{.User.Name}}", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{
		"User": map[string]interface{}{
			"Name": "John",
		},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics[0])
	}
}

// TestWalkAST_TextNode tests text node handling.
func TestWalkAST_TextNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("Hello World")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)
	tree, err := walkAST(tmpl.Tree.Root, eval, nil, nil, ctx)
	if err != nil {
		t.Fatalf("walkAST failed: %v", err)
	}

	if len(tree.Statics) == 0 {
		t.Error("Expected statics for text node")
	}
}

// TestWalkAST_ActionNode tests action node handling.
func TestWalkAST_ActionNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := walkAST(tmpl.Tree.Root, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("walkAST failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics[0])
	}
}

// TestWalkAST_CommentNode tests comment handling.
func TestWalkAST_CommentNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{/* comment */}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)
	tree, err := walkAST(tmpl.Tree.Root, eval, nil, nil, ctx)
	if err != nil {
		t.Fatalf("walkAST failed: %v", err)
	}

	// Comments should produce empty tree
	if tree.HasDynamics() {
		t.Error("Expected no dynamics for comment node")
	}
}

// TestWalkList_SingleNode tests list with single node.
func TestWalkList_SingleNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := walkList(tmpl.Tree.Root, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("walkList failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics[0])
	}
}

// TestWalkList_MultipleNodes tests list with multiple nodes.
func TestWalkList_MultipleNodes(t *testing.T) {
	tmpl, err := template.New("test").Parse("<div>{{.Name}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := walkList(tmpl.Tree.Root, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("walkList failed: %v", err)
	}

	// Should have merged statics from multiple nodes
	if len(tree.Statics) < 2 {
		t.Errorf("Expected at least 2 statics, got: %d", len(tree.Statics))
	}
}

// TestWalkList_EmptyList tests empty list handling.
func TestWalkList_EmptyList(t *testing.T) {
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)
	tree, err := walkList(nil, eval, nil, nil, ctx)
	if err != nil {
		t.Fatalf("walkList failed: %v", err)
	}

	// Empty list should return tree with single empty static
	if len(tree.Statics) != 1 {
		t.Errorf("Expected 1 static for empty list, got: %d", len(tree.Statics))
	}
}

// TestEvalPipe_Simple tests simple dot access via BuildTree.
func TestEvalPipe_Simple(t *testing.T) {
	tmpl, err := Parse("{{.Name}}", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected 'John', got: %v", tree.Dynamics[0])
	}
}

// TestEvalPipe_Complex tests complex pipeline via BuildTree.
func TestEvalPipe_Complex(t *testing.T) {
	funcMap := template.FuncMap{
		"len": func(s interface{}) int {
			return reflect.ValueOf(s).Len()
		},
	}
	tmpl, err := Parse("{{len .Items}}", funcMap)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{
		"Items": []string{"a", "b", "c"},
	}
	ctx := &Context{IncludeStatics: true, FuncMap: funcMap}

	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	// The dynamic should contain the result of len(.Items) = 3
	val := tree.Dynamics[0]
	if val == nil {
		t.Fatal("Expected non-nil dynamic at position 0")
	}
	if fmt.Sprint(val) != "3" {
		t.Errorf("Expected len result '3', got: %v", val)
	}
}

// TestEvalPipe_WithFuncs tests pipeline with functions via BuildTree.
func TestEvalPipe_WithFuncs(t *testing.T) {
	funcMap := template.FuncMap{
		"upper": func(s string) string { return s + "_UPPER" },
	}

	tmpl, err := Parse("{{.Name | upper}}", funcMap)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true, FuncMap: funcMap}

	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree.Dynamics[0] != "John_UPPER" {
		t.Errorf("Expected 'John_UPPER', got: %v", tree.Dynamics[0])
	}
}

// TestFormatPipe tests pipe formatting.
func TestFormatPipe(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	pipeStr := formatPipe(actionNode.Pipe)

	if pipeStr != ".Name" {
		t.Errorf("Expected '.Name', got: %q", pipeStr)
	}
}

// TestIsZeroValue_AllTypes tests zero value detection for all types.
func TestIsZeroValue_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"nil", nil, true},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"zero string", "", true},
		{"non-zero string", "hello", false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"nil slice", []string(nil), true},
		{"empty slice", []string{}, true},
		{"non-empty slice", []string{"a"}, false},
		{"nil map", map[string]string(nil), true},
		{"empty map", map[string]string{}, true},
		{"non-empty map", map[string]string{"a": "b"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.value)
			got := isZeroValue(v)
			if got != tt.want {
				t.Errorf("isZeroValue(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestWalkList_ErrorContext tests error reporting with context.
func TestWalkList_ErrorContext(t *testing.T) {
	// This tests that errors from child nodes include context.
	// We test with a template invocation which should fail with an error message.
	tmpl, err := template.New("test").Parse("{{template \"missing\" .}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]interface{}{}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	_, err = walkList(tmpl.Tree.Root, eval, data, nil, ctx)
	if err == nil {
		t.Error("Expected error for template invocation")
	}

	// Error should contain child node information
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Expected non-empty error message")
	}
}
