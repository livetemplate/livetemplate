package parse

import (
	"html/template"
	"testing"
	"text/template/parse"
)

func TestHandleAction_SimpleField(t *testing.T) {
	node := parseActionNode(t, "{{.Name}}", nil)
	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics[0])
	}

	if len(tree.Statics) != 2 {
		t.Errorf("Expected 2 statics, got: %d", len(tree.Statics))
	}
}

func TestHandleAction_StructField(t *testing.T) {
	type TestData struct {
		Value string
	}
	data := TestData{Value: "test"}

	node := parseActionNode(t, "{{.Value}}", nil)
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "test" {
		t.Errorf("Expected dynamics['0'] = 'test', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_Pipeline(t *testing.T) {
	funcs := template.FuncMap{
		"upper": func(s string) string { return s + "_UPPER" },
	}

	node := parseActionNode(t, "{{.Value | upper}}", funcs)
	data := map[string]interface{}{"Value": "test"}
	ctx := &Context{FuncMap: funcs, IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "test_UPPER" {
		t.Errorf("Expected dynamics['0'] = 'test_UPPER', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_MissingField(t *testing.T) {
	node := parseActionNode(t, "{{.NonExistent}}", nil)
	data := map[string]interface{}{}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	// Map returns nil for missing keys; valueToString converts nil to ""
	if tree.Dynamics[0] != "" {
		t.Errorf("Expected empty dynamics['0'], got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_EmptyValue(t *testing.T) {
	node := parseActionNode(t, "{{.Name}}", nil)
	data := map[string]interface{}{"Name": ""}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, data, nil, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "" {
		t.Errorf("Expected empty dynamics['0'], got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_WithVarCtx_DotAccess(t *testing.T) {
	node := parseActionNode(t, "{{.Name}}", nil)
	varCtx := &varContext{
		parent: map[string]interface{}{"Name": "John"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Name": "Dot"},
	}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, varCtx.parent, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "Dot" {
		t.Errorf("Expected dynamics['0'] = 'Dot', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_WithVarCtx_VariableAccess(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $name := .Items}}{{$name}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	actionNode := rangeNode.List.Nodes[0].(*parse.ActionNode)

	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("name", "Variable")
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(actionNode, eval, varCtx.parent, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "Variable" {
		t.Errorf("Expected dynamics['0'] = 'Variable', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_WithVarCtx_RootVariable(t *testing.T) {
	node := parseActionNode(t, "{{$.RootValue}}", nil)
	varCtx := &varContext{
		parent: map[string]interface{}{"RootValue": "Root"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"DotValue": "Dot"},
	}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, varCtx.parent, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "Root" {
		t.Errorf("Expected dynamics['0'] = 'Root', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_SingleVariable(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $name := .Items}}{{$name}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	actionNode := rangeNode.List.Nodes[0].(*parse.ActionNode)

	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("name", "John")
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(actionNode, eval, varCtx.parent, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_RootVariableFromVarCtx(t *testing.T) {
	node := parseActionNode(t, "{{$.Value}}", nil)
	varCtx := &varContext{
		parent: map[string]interface{}{"Value": "Root"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(node, eval, varCtx.parent, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "Root" {
		t.Errorf("Expected dynamics['0'] = 'Root', got: %v", tree.Dynamics[0])
	}
}

func TestHandleAction_VariableWithUnderscore(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $var_name := .Items}}{{$var_name}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	actionNode := rangeNode.List.Nodes[0].(*parse.ActionNode)

	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("var_name", "UnderscoreValue")
	ctx := &Context{IncludeStatics: true}
	eval := newEvaluator(ctx.FuncMap)

	tree, err := handleAction(actionNode, eval, varCtx.parent, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	if tree.Dynamics[0] != "UnderscoreValue" {
		t.Errorf("Expected dynamics['0'] = 'UnderscoreValue', got: %v", tree.Dynamics[0])
	}
}

func TestCreateSingleDynamicTree_WithStatics(t *testing.T) {
	ctx := &Context{IncludeStatics: true}
	tree := createSingleDynamicTree("test value", ctx)

	if tree.Dynamics[0] != "test value" {
		t.Errorf("Expected dynamics['0'] = 'test value', got: %v", tree.Dynamics[0])
	}

	if len(tree.Statics) != 2 {
		t.Errorf("Expected 2 statics, got: %d", len(tree.Statics))
	}

	if tree.Statics[0] != "" || tree.Statics[1] != "" {
		t.Errorf("Expected empty statics, got: %v", tree.Statics)
	}
}

func TestCreateSingleDynamicTree_WithoutStatics(t *testing.T) {
	ctx := &Context{IncludeStatics: false}
	tree := createSingleDynamicTree("test value", ctx)

	if tree.Dynamics[0] != "test value" {
		t.Errorf("Expected dynamics['0'] = 'test value', got: %v", tree.Dynamics[0])
	}

	if tree.Statics != nil {
		t.Errorf("Expected nil statics, got: %v", tree.Statics)
	}
}
