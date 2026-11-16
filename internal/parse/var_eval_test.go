package parse

import (
	"html/template"
	"testing"
	"text/template/parse"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestVarEval_SimpleVarAccess tests that simple variable references evaluate correctly
// through BuildTree.
func TestVarEval_SimpleVarAccess(t *testing.T) {
	tests := []struct {
		name   string
		tmpl   string
		data   interface{}
		wantD0 string
	}{
		{
			name:   "ascii lowercase var",
			tmpl:   `{{$name := .Name}}<span>{{$name}}</span>`,
			data:   map[string]interface{}{"Name": "John"},
			wantD0: "John",
		},
		{
			name:   "single char var",
			tmpl:   `{{$a := .Val}}<span>{{$a}}</span>`,
			data:   map[string]interface{}{"Val": "1"},
			wantD0: "1",
		},
		{
			name:   "already upper var",
			tmpl:   `{{$Name := .Name}}<span>{{$Name}}</span>`,
			data:   map[string]interface{}{"Name": "test"},
			wantD0: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.tmpl, nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			ctx := build.NewContext()
			tree, err := BuildTree(tmpl, tt.data, ctx)
			if err != nil {
				t.Fatalf("BuildTree failed: %v", err)
			}

			d0, ok := tree.GetDynamic(0)
			if !ok {
				t.Fatal("missing dynamic 0")
			}
			if d0 != tt.wantD0 {
				t.Errorf("dynamic 0 = %v, want %v", d0, tt.wantD0)
			}
		})
	}
}

// TestVarEval_PartialMatchPrevention tests that variables with overlapping names
// (like $c and $col) don't interfere with each other.
func TestVarEval_PartialMatchPrevention(t *testing.T) {
	tests := []struct {
		name   string
		tmpl   string
		data   interface{}
		wantD0 string
	}{
		{
			name:   "$c and $col - use $col",
			tmpl:   `{{$c := "short"}}{{$col := "long"}}<span>{{$col}}</span>`,
			data:   map[string]interface{}{},
			wantD0: "long",
		},
		{
			name:   "$item and $itemCount",
			tmpl:   `{{$item := "x"}}{{$itemCount := "5"}}<span>{{$itemCount}}</span>`,
			data:   map[string]interface{}{},
			wantD0: "5",
		},
		{
			name:   "both $c and $col used",
			tmpl:   `{{$c := "short"}}{{$col := "long"}}<span>{{$c}}</span><span>{{$col}}</span>`,
			data:   map[string]interface{}{},
			wantD0: "short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Parse(tt.tmpl, nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			ctx := build.NewContext()
			tree, err := BuildTree(tmpl, tt.data, ctx)
			if err != nil {
				t.Fatalf("BuildTree failed: %v", err)
			}

			d0, ok := tree.GetDynamic(0)
			if !ok {
				t.Fatal("missing dynamic 0")
			}
			if d0 != tt.wantD0 {
				t.Errorf("dynamic 0 = %v, want %v", d0, tt.wantD0)
			}
		})
	}
}

// TestVarEval_RootVariable tests root variable ($.) handling through BuildTree.
func TestVarEval_RootVariable(t *testing.T) {
	tmplStr := `{{with .Item}}{{$.Title}}{{end}}`
	data := map[string]interface{}{
		"Title": "rootValue",
		"Item":  map[string]interface{}{"Name": "child"},
	}

	if got, want := lvtRender(t, tmplStr, data, nil), "rootValue"; got != want {
		t.Errorf("rendered %q, want %q", got, want)
	}
}

// TestVarEval_MixedRootAndVars tests expressions with both root ($.) and named variables.
func TestVarEval_MixedRootAndVars(t *testing.T) {
	tmplStr := `{{$x := .Count}}{{with .Item}}{{$x}} {{$.Title}}{{end}}`
	data := map[string]interface{}{
		"Title": "Root Title",
		"Count": "42",
		"Item":  map[string]interface{}{"Name": "child"},
	}

	if got, want := lvtRender(t, tmplStr, data, nil), "42 Root Title"; got != want {
		t.Errorf("rendered %q, want %q", got, want)
	}
}

// TestVarEval_DotContextMerging tests that expressions mixing variables and
// dot fields work correctly through BuildTree.
func TestVarEval_DotContextMerging(t *testing.T) {
	t.Run("map dot context in with block", func(t *testing.T) {
		tmplStr := `{{$c := .Controller}}{{with .Item}}{{$c}} {{.Type}}{{end}}`
		data := map[string]interface{}{
			"Controller": "ctrl",
			"Item":       map[string]interface{}{"Type": "button"},
		}

		if got, want := lvtRender(t, tmplStr, data, nil), "ctrl button"; got != want {
			t.Errorf("rendered %q, want %q", got, want)
		}
	})

	t.Run("struct dot context in with block", func(t *testing.T) {
		type Inner struct {
			Type string
		}
		type Data struct {
			Controller string
			Item       *Inner
		}

		tmplStr := `{{$c := .Controller}}{{with .Item}}{{$c}} {{.Type}}{{end}}`
		data := Data{Controller: "ctrl", Item: &Inner{Type: "input"}}

		if got, want := lvtRender(t, tmplStr, data, nil), "ctrl input"; got != want {
			t.Errorf("rendered %q, want %q", got, want)
		}
	})
}

// TestVarEval_NoVarsPlainDotField tests expression with no variables (plain dot field access).
func TestVarEval_NoVarsPlainDotField(t *testing.T) {
	tmplStr := `<span>{{.Field}}</span>`
	data := map[string]interface{}{"Field": "value"}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	d0, ok := tree.GetDynamic(0)
	if !ok {
		t.Fatal("missing dynamic 0")
	}
	if d0 != "value" {
		t.Errorf("dynamic 0 = %v, want value", d0)
	}
}

// TestVarEval_ErrorPropagation tests that missing struct fields produce empty output
// (matching Go template behavior where missing fields render as <no value> or empty).
func TestVarEval_ErrorPropagation(t *testing.T) {
	type Data struct{}
	tmplStr := `<span>{{.NonExistentField}}</span>`
	data := Data{}

	tmpl, err := Parse(tmplStr, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := build.NewContext()
	tree, err := BuildTree(tmpl, data, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d0, _ := tree.GetDynamic(0)
	if d0 != "" {
		t.Errorf("expected empty string for missing field, got %q", d0)
	}
}

// TestVarEval_DotContextAccess tests that expressions mixing variables and dot fields
// work correctly via handleAction with an evaluator.
func TestVarEval_DotContextAccess(t *testing.T) {
	tmplStr := `{{$cls := .Class}}{{with .Item}}<div class="{{$cls}}">{{.Type}}</div>{{end}}`
	data := map[string]interface{}{
		"Class": "primary",
		"Item":  map[string]interface{}{"Type": "button"},
	}

	if got, want := lvtRender(t, tmplStr, data, nil), `<div class="primary">button</div>`; got != want {
		t.Errorf("rendered %q, want %q", got, want)
	}
}

// TestVarEval_HandleAction_Direct tests handleAction with an evaluator and varContext directly.
func TestVarEval_HandleAction_Direct(t *testing.T) {
	// Extract an ActionNode that references $cls from within a range template
	// (Go's parser requires $cls to be declared in scope).
	actionNode := func() *parse.ActionNode {
		t.Helper()
		tmpl := template.New("test")
		parsed, err := tmpl.Parse("{{range $cls := .Items}}{{$cls}}{{end}}")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		rangeNode := parsed.Tree.Root.Nodes[0].(*parse.RangeNode)
		return rangeNode.List.Nodes[0].(*parse.ActionNode)
	}()

	eval := testEval()
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{},
	}
	varCtx.vars.Set("cls", "primary")
	ctx := &Context{IncludeStatics: true}

	tree, err := handleAction(actionNode, eval, map[string]interface{}{}, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	d0, ok := tree.GetDynamic(0)
	if !ok {
		t.Fatal("missing dynamic 0")
	}
	if d0 != "primary" {
		t.Errorf("dynamic 0 = %v, want primary", d0)
	}
}

// TestVarEval_HandleAction_DotField tests handleAction accessing a dot field.
func TestVarEval_HandleAction_DotField(t *testing.T) {
	actionNode := parseActionNode(t, "{{.Type}}", nil)
	eval := testEval()
	varCtx := &varContext{
		parent: map[string]interface{}{},
		vars:   newOrderedVars(),
		dot:    map[string]interface{}{"Type": "button"},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := handleAction(actionNode, eval, map[string]interface{}{}, varCtx, ctx)
	if err != nil {
		t.Fatalf("handleAction failed: %v", err)
	}

	d0, ok := tree.GetDynamic(0)
	if !ok {
		t.Fatal("missing dynamic 0")
	}
	if d0 != "button" {
		t.Errorf("dynamic 0 = %v, want button", d0)
	}
}

// TestVarEval_HandleAction_MixedVarAndDot tests handleAction with both var and dot field.
func TestVarEval_HandleAction_MixedVarAndDot(t *testing.T) {
	// We test through BuildTree since handleAction processes single expressions.
	// printf is a builtin so no custom FuncMap needed.
	tmplStr := `{{$cls := .Class}}{{with .Item}}{{printf "%s %s" $cls .Type}}{{end}}`
	data := map[string]interface{}{
		"Class": "primary",
		"Item":  map[string]interface{}{"Type": "button"},
	}

	if got, want := lvtRender(t, tmplStr, data, nil), "primary button"; got != want {
		t.Errorf("rendered %q, want %q", got, want)
	}
}
