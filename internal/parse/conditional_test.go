package parse

import (
	"html/template"
	"testing"
	"text/template/parse"
)

// TestHandleIfNode_TrueBranch tests if node when condition is true.
func TestHandleIfNode_TrueBranch(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]any{"Show": true}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	// Should have wrapper with branch content
	if !tree.HasDynamics() {
		t.Error("Expected dynamics for if branch")
	}
}

// TestHandleIfNode_FalseBranch tests if node when condition is false with no else.
func TestHandleIfNode_FalseBranch(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]any{"Show": false}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed: %v", err)
	}

	// Should have wrapper with empty dynamic value
	if tree.Dynamics["0"] != "" {
		t.Errorf("Expected empty string for false condition, got: %v", tree.Dynamics["0"])
	}
}

// TestHandleIfNode_WithElse tests if/else branches.
func TestHandleIfNode_WithElse(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{else}}hidden{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)

	// Test true branch
	data := map[string]any{"Show": true}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for true branch")
	}

	// Test false branch (else)
	data = map[string]any{"Show": false}
	tree, err = handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed on else: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for else branch")
	}
}

// TestHandleIfNode_NoElse tests if without else clause when condition is false.
func TestHandleIfNode_NoElse(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}content{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]any{"Show": false}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed: %v", err)
	}

	// Should return wrapper with empty dynamic
	if tree.Dynamics["0"] != "" {
		t.Errorf("Expected empty dynamic for false with no else, got: %v", tree.Dynamics["0"])
	}
}

// TestHandleIfNode_NestedIf tests nested if statements.
func TestHandleIfNode_NestedIf(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Outer}}{{if .Inner}}nested{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]any{"Outer": true, "Inner": true}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree for nested if")
	}

	// Should have nested structure
	if !tree.HasDynamics() {
		t.Error("Expected dynamics for nested if")
	}
}

// TestHandleIfNode_ComplexCondition tests complex conditions.
func TestHandleIfNode_ComplexCondition(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if and .A .B}}both{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	data := map[string]any{"A": true, "B": true}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for complex condition")
	}
}

// TestHandleIfNodeWithVars_NoVars tests if node with vars but condition doesn't use them.
func TestHandleIfNodeWithVars_NoVars(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if .Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	varCtx := &varContext{
		parent: map[string]any{},
		vars:   newOrderedVars(),
		dot:    map[string]any{"Show": true},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNodeWithVars(ifNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNodeWithVars failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for true condition")
	}
}

// TestHandleIfNodeWithVars_WithVars tests if node using variables.
func TestHandleIfNodeWithVars_WithVars(t *testing.T) {
	// Parse a template with range that defines a variable, containing an if
	tmpl, err := template.New("test").Parse("{{range $show := .Items}}{{if $show}}visible{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Extract the range node, then the if node inside it
	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	ifNode := rangeNode.List.Nodes[0].(*parse.IfNode)

	varCtx := &varContext{
		parent: map[string]any{},
		vars:   newOrderedVars(),
		dot:    map[string]any{},
	}
	varCtx.vars.Set("show", true)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNodeWithVars(ifNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNodeWithVars failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for variable condition")
	}
}

// TestHandleIfNodeWithVars_RootVar tests if node using root variable.
func TestHandleIfNodeWithVars_RootVar(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{if $.Show}}visible{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	varCtx := &varContext{
		parent: map[string]any{"Show": true},
		vars:   newOrderedVars(),
		dot:    map[string]any{},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNodeWithVars(ifNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNodeWithVars failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for root variable condition")
	}
}

// TestMergeFieldsIntoMap_Struct tests merging struct fields into map.
func TestMergeFieldsIntoMap_Struct(t *testing.T) {
	type TestStruct struct {
		Name  string
		Value int
	}

	data := TestStruct{Name: "test", Value: 42}
	target := make(map[string]any)

	err := mergeFieldsIntoMap(data, target)
	if err != nil {
		t.Fatalf("mergeFieldsIntoMap failed: %v", err)
	}

	if target["Name"] != "test" {
		t.Errorf("Expected Name='test', got: %v", target["Name"])
	}

	if target["Value"] != 42 {
		t.Errorf("Expected Value=42, got: %v", target["Value"])
	}
}

// TestMergeFieldsIntoMap_Map tests merging map into map.
func TestMergeFieldsIntoMap_Map(t *testing.T) {
	data := map[string]any{
		"name":  "test",
		"value": 42,
	}
	target := make(map[string]any)

	err := mergeFieldsIntoMap(data, target)
	if err != nil {
		t.Fatalf("mergeFieldsIntoMap failed: %v", err)
	}

	if target["name"] != "test" {
		t.Errorf("Expected name='test', got: %v", target["name"])
	}

	if target["value"] != 42 {
		t.Errorf("Expected value=42, got: %v", target["value"])
	}
}

// TestMergeFieldsIntoMap_Primitive tests merging primitive value (should be no-op).
func TestMergeFieldsIntoMap_Primitive(t *testing.T) {
	data := "string value"
	target := make(map[string]any)

	err := mergeFieldsIntoMap(data, target)
	if err != nil {
		t.Fatalf("mergeFieldsIntoMap failed: %v", err)
	}

	// Should not add anything for primitive
	if len(target) != 0 {
		t.Errorf("Expected empty map for primitive, got: %v", target)
	}
}

// TestMergeFieldsIntoMap_Nil tests merging nil value.
func TestMergeFieldsIntoMap_Nil(t *testing.T) {
	target := make(map[string]any)

	err := mergeFieldsIntoMap(nil, target)
	if err != nil {
		t.Fatalf("mergeFieldsIntoMap failed: %v", err)
	}

	// Should not add anything for nil
	if len(target) != 0 {
		t.Errorf("Expected empty map for nil, got: %v", target)
	}
}

// TestMergeFieldsIntoMap_ExistingKeys tests that existing keys are not overwritten.
func TestMergeFieldsIntoMap_ExistingKeys(t *testing.T) {
	data := map[string]any{
		"name": "from-data",
		"age":  30,
	}
	target := map[string]any{
		"name": "existing",
	}

	err := mergeFieldsIntoMap(data, target)
	if err != nil {
		t.Fatalf("mergeFieldsIntoMap failed: %v", err)
	}

	// Should preserve existing "name", add "age"
	if target["name"] != "existing" {
		t.Errorf("Expected name='existing', got: %v", target["name"])
	}
	if target["age"] != 30 {
		t.Errorf("Expected age=30, got: %v", target["age"])
	}
}

// TestMergeFieldsIntoMap_UnexportedFields tests that unexported struct fields are skipped.
func TestMergeFieldsIntoMap_UnexportedFields(t *testing.T) {
	type TestStruct struct {
		Public  string
		private string
	}

	data := TestStruct{Public: "visible", private: "hidden"}
	target := make(map[string]any)

	err := mergeFieldsIntoMap(data, target)
	if err != nil {
		t.Fatalf("mergeFieldsIntoMap failed: %v", err)
	}

	// Should only have Public field
	if target["Public"] != "visible" {
		t.Errorf("Expected Public='visible', got: %v", target["Public"])
	}
	if _, exists := target["private"]; exists {
		t.Error("Expected unexported field 'private' to be skipped")
	}
}

// TestCapitalizeFieldName tests field name capitalization.
func TestCapitalizeFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"show", "Show"},
		{"isActive", "IsActive"},
		{"x", "X"},
		{"userName", "UserName"},
		{"", ""},
		{"a", "A"},
		{"ABC", "ABC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalizeFieldName(tt.input)
			if result != tt.expected {
				t.Errorf("capitalizeFieldName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestHandleIfNode_ElseIf tests else-if chains.
func TestHandleIfNode_ElseIf(t *testing.T) {
	// Template engines typically treat {{else if}} as nested ifs in else branch
	tmpl, err := template.New("test").Parse("{{if .A}}first{{else if .B}}second{{else}}third{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ifNode := tmpl.Tree.Root.Nodes[0].(*parse.IfNode)
	ctx := &Context{IncludeStatics: true}

	// Test first condition true
	data := map[string]any{"A": true, "B": false}
	tree, err := handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed on first branch: %v", err)
	}
	if tree == nil {
		t.Fatal("Expected non-nil tree for first branch")
	}

	// Test second condition true
	data = map[string]any{"A": false, "B": true}
	tree, err = handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed on second branch: %v", err)
	}
	if tree == nil {
		t.Fatal("Expected non-nil tree for second branch")
	}

	// Test else branch
	data = map[string]any{"A": false, "B": false}
	tree, err = handleIfNode(ifNode, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNode failed on else branch: %v", err)
	}
	if tree == nil {
		t.Fatal("Expected non-nil tree for else branch")
	}
}

// TestHandleIfNodeWithVars_ComplexNesting tests complex nested variable scenarios.
func TestHandleIfNodeWithVars_ComplexNesting(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $item := .Items}}{{if $item.Active}}{{$item.Name}}{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	ifNode := rangeNode.List.Nodes[0].(*parse.IfNode)

	varCtx := &varContext{
		parent: map[string]any{},
		vars:   newOrderedVars(),
		dot:    map[string]any{"Active": true, "Name": "test"},
	}
	varCtx.vars.Set("item", map[string]any{"Active": true, "Name": "test"})
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNodeWithVars(ifNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNodeWithVars failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for complex nested condition")
	}
}

// TestHandleIfNodeWithVars_SingleCharVariable tests single-character variable names.
func TestHandleIfNodeWithVars_SingleCharVariable(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{range $x := .Items}}{{if $x}}visible{{end}}{{end}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	rangeNode := tmpl.Tree.Root.Nodes[0].(*parse.RangeNode)
	ifNode := rangeNode.List.Nodes[0].(*parse.IfNode)

	varCtx := &varContext{
		parent: map[string]any{},
		vars:   newOrderedVars(),
		dot:    map[string]any{},
	}
	varCtx.vars.Set("x", true)
	ctx := &Context{IncludeStatics: true}

	tree, err := handleIfNodeWithVars(ifNode, varCtx, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("handleIfNodeWithVars failed for single-char var: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if !tree.HasDynamics() {
		t.Error("Expected dynamics for single-char variable")
	}
}
