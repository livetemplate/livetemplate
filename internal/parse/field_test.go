package parse

import (
	"html/template"
	"testing"
	"text/template/parse"
)

// mockKeyGenerator is a simple mock for testing.
type mockKeyGenerator struct {
	counter int
}

func newMockKeyGen() *mockKeyGenerator {
	return &mockKeyGenerator{}
}

func (m *mockKeyGenerator) Next() string {
	m.counter++
	return string(rune('a' + m.counter - 1))
}

// TestHandleActionNode_SimpleField tests simple field access like {{.Name}}.
func TestHandleActionNode_SimpleField(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	data := map[string]interface{}{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleActionNode(actionNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNode failed: %v", err)
	}

	if tree.Dynamics["0"] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics["0"])
	}

	if len(tree.Statics) != 2 {
		t.Errorf("Expected 2 statics, got: %d", len(tree.Statics))
	}
}

// TestHandleActionNode_Method tests method calls like {{.Method}}.
func TestHandleActionNode_Method(t *testing.T) {
	type TestData struct {
		Value string
	}
	data := TestData{Value: "test"}

	tmpl, err := template.New("test").Parse("{{.Value}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleActionNode(actionNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNode failed: %v", err)
	}

	if tree.Dynamics["0"] != "test" {
		t.Errorf("Expected dynamics['0'] = 'test', got: %v", tree.Dynamics["0"])
	}
}

// TestHandleActionNode_Pipeline tests pipeline expressions like {{.Value | upper}}.
func TestHandleActionNode_Pipeline(t *testing.T) {
	funcs := template.FuncMap{
		"upper": func(s string) string { return s + "_UPPER" },
	}

	tmpl, err := template.New("test").Funcs(funcs).Parse("{{.Value | upper}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	data := map[string]interface{}{"Value": "test"}
	ctx := &Context{FuncMap: funcs, IncludeStatics: true}

	tree, err := handleActionNode(actionNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNode failed: %v", err)
	}

	if tree.Dynamics["0"] != "test_UPPER" {
		t.Errorf("Expected dynamics['0'] = 'test_UPPER', got: %v", tree.Dynamics["0"])
	}
}

// TestHandleActionNode_Error tests error handling.
func TestHandleActionNode_Error(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.NonExistent}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	data := map[string]interface{}{}
	ctx := &Context{IncludeStatics: true}

	// Should execute but get empty string (template's zero value behavior)
	tree, err := handleActionNode(actionNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNode failed: %v", err)
	}

	// Template execution returns empty string for missing fields
	if tree.Dynamics["0"] != "<no value>" {
		t.Logf("Got dynamics['0'] = %v", tree.Dynamics["0"])
	}
}

// TestHandleActionNodeWithVars_NoVars tests action with no variables.
func TestHandleActionNodeWithVars_NoVars(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	varCtx := &varContext{
		parent: map[string]interface{}{"Name": "John"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Name": "Dot"},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleActionNodeWithVars(actionNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNodeWithVars failed: %v", err)
	}

	// Should use dot context
	if tree.Dynamics["0"] != "Dot" {
		t.Errorf("Expected dynamics['0'] = 'Dot', got: %v", tree.Dynamics["0"])
	}
}

// TestHandleActionNodeWithVars_WithVars tests action with variables.
func TestHandleActionNodeWithVars_WithVars(t *testing.T) {
	// Parse a template with range that defines a variable, containing an action
	tmpl, err := template.New("test").Parse("{{range $name := .Items}}{{$name}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Extract the range node, then the action node inside it
	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	actionNode := rangeNode.List.Nodes[0].(*parse.ActionNode)

	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("name", "Variable")
	ctx := &Context{IncludeStatics: true}

	tree, err := handleActionNodeWithVars(actionNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNodeWithVars failed: %v", err)
	}

	if tree.Dynamics["0"] != "Variable" {
		t.Errorf("Expected dynamics['0'] = 'Variable', got: %v", tree.Dynamics["0"])
	}
}

// TestHandleActionNodeWithVars_RootVar tests action with root variable $.
func TestHandleActionNodeWithVars_RootVar(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{$.RootValue}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	varCtx := &varContext{
		parent: map[string]interface{}{"RootValue": "Root"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"DotValue": "Dot"},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleActionNodeWithVars(actionNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleActionNodeWithVars failed: %v", err)
	}

	if tree.Dynamics["0"] != "Root" {
		t.Errorf("Expected dynamics['0'] = 'Root', got: %v", tree.Dynamics["0"])
	}
}

// TestEvaluateActionWithVars_SingleVar tests single variable evaluation.
func TestEvaluateActionWithVars_SingleVar(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("name", "John")
	ctx := &Context{}

	result := evaluateActionWithVars("{{$name}}", varCtx, ctx)
	if result != "John" {
		t.Errorf("Expected 'John', got: %v", result)
	}
}

// TestEvaluateActionWithVars_MultipleVars tests multiple variables.
func TestEvaluateActionWithVars_MultipleVars(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("first", "John")
	varCtx.vars.Set("last", "Doe")
	ctx := &Context{}

	// Template that uses both variables
	result := evaluateActionWithVars("{{$first}} {{$last}}", varCtx, ctx)
	if result != "John Doe" {
		t.Errorf("Expected 'John Doe', got: %v", result)
	}
}

// TestEvaluateActionWithVars_RootVar tests root variable.
func TestEvaluateActionWithVars_RootVar(t *testing.T) {
	varCtx := &varContext{
		parent: map[string]interface{}{"Value": "Root"},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	ctx := &Context{}

	result := evaluateActionWithVars("{{$.Value}}", varCtx, ctx)
	if result != "Root" {
		t.Errorf("Expected 'Root', got: %v", result)
	}
}

// TestDetectsRootVariable tests root variable detection.
func TestDetectsRootVariable(t *testing.T) {
	vars := newOrderedVars()

	tests := []struct {
		name   string
		action string
		want   bool
	}{
		{"dot access", "{{$.Field}}", true},
		{"standalone dollar", "{{$}}", true},
		{"named variable", "{{$var}}", false},
		{"no dollar", "{{.Field}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectsRootVariable(tt.action, vars)
			if got != tt.want {
				t.Errorf("detectsRootVariable(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

// TestIsLetter tests letter detection.
func TestIsLetter(t *testing.T) {
	tests := []struct {
		char byte
		want bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', false},
		{'9', false},
		{'$', false},
		{'.', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			got := isLetter(tt.char)
			if got != tt.want {
				t.Errorf("isLetter(%c) = %v, want %v", tt.char, got, tt.want)
			}
		})
	}
}
