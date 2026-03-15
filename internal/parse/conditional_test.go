package parse

import (
	"html/template"
	"testing"
	"text/template/parse"
)

// TestHandleIf_TrueBranch tests if node when condition is true.
func TestHandleIf_TrueBranch(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{"Show": true}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for if branch")
	}
}

// TestHandleIf_FalseBranch tests if node when condition is false with no else.
func TestHandleIf_FalseBranch(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{"Show": false}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	// Should have wrapper with empty dynamic value
	if tree.Dynamics["0"] != "" {
		t.Errorf("Expected empty string for false condition, got: %v", tree.Dynamics["0"])
	}
}

// TestHandleIf_WithElse tests if/else branches.
func TestHandleIf_WithElse(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{else}}hidden{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	// Test true branch
	data := map[string]interface{}{"Show": true}
	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for true branch")
	}

	// Test false branch (else)
	data = map[string]interface{}{"Show": false}
	tree, err = handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed on else: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for else branch")
	}
}

// TestHandleIf_NoElse tests if without else clause when condition is false.
func TestHandleIf_NoElse(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}content{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{"Show": false}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	// Should return wrapper with empty dynamic
	if tree.Dynamics["0"] != "" {
		t.Errorf("Expected empty dynamic for false with no else, got: %v", tree.Dynamics["0"])
	}
}

// TestHandleIf_NestedIf tests nested if statements.
func TestHandleIf_NestedIf(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Outer}}{{if .Inner}}nested{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{"Outer": true, "Inner": true}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for nested if")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for nested if")
	}
}

// TestHandleIf_ComplexCondition tests complex conditions.
func TestHandleIf_ComplexCondition(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if and .A .B}}both{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{"A": true, "B": true}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for complex condition")
	}
}

// TestHandleIf_WithVarCtx_NoVars tests if node with varCtx but condition doesn't use variables.
func TestHandleIf_WithVarCtx_NoVars(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{}
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Show": true},
	}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for true condition")
	}
}

// TestHandleIf_WithVarCtx_WithVars tests if node using variables.
func TestHandleIf_WithVarCtx_WithVars(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $show := .Items}}{{if $show}}visible{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	ifNode := rangeNode.List.Nodes[0].(*parse.IfNode)

	data := map[string]interface{}{}
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("show", true)
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for variable condition")
	}
}

// TestHandleIf_WithVarCtx_RootVar tests if node using root variable.
func TestHandleIf_WithVarCtx_RootVar(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if $.Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]interface{}{"Show": true}
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for root variable condition")
	}
}

// TestHandleIf_ElseIf tests else-if chains.
func TestHandleIf_ElseIf(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .A}}first{{else if .B}}second{{else}}third{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	// Test first condition true
	data := map[string]interface{}{"A": true, "B": false}
	tree, err := handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed on first branch: %v", err)
	}
	if tree == nil {
		t.Fatal("Expected non-nil tree for first branch")
	}

	// Test second condition true
	data = map[string]interface{}{"A": false, "B": true}
	tree, err = handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed on second branch: %v", err)
	}
	if tree == nil {
		t.Fatal("Expected non-nil tree for second branch")
	}

	// Test else branch
	data = map[string]interface{}{"A": false, "B": false}
	tree, err = handleIf(ifNode, eval, data, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed on else branch: %v", err)
	}
	if tree == nil {
		t.Fatal("Expected non-nil tree for else branch")
	}
}

// TestHandleIf_WithVarCtx_ComplexNesting tests complex nested variable scenarios.
func TestHandleIf_WithVarCtx_ComplexNesting(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $item := .Items}}{{if $item.Active}}{{$item.Name}}{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	ifNode := rangeNode.List.Nodes[0].(*parse.IfNode)

	data := map[string]interface{}{}
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Active": true, "Name": "test"},
	}
	varCtx.vars.Set("item", map[string]interface{}{"Active": true, "Name": "test"})
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for complex nested condition")
	}
}

// TestHandleIf_WithVarCtx_SingleCharVariable tests single-character variable names.
func TestHandleIf_WithVarCtx_SingleCharVariable(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $x := .Items}}{{if $x}}visible{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	ifNode := rangeNode.List.Nodes[0].(*parse.IfNode)

	data := map[string]interface{}{}
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("x", true)
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleIf(ifNode, eval, data, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIf failed for single-char var: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for single-char variable")
	}
}
