package parse

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

// treeToHTML reconstructs HTML from a TreeNode by interleaving statics and dynamics.
// This is a test helper that mirrors the render package's logic.
// NOTE: This helper does not handle RangeData (tree.Range). Any parity test
// involving {{range}} will produce incomplete HTML from the LVT side.
// Range tests should use BuildTree output directly or a dedicated range helper.
func treeToHTML(tree *TreeNode) string {
	if tree == nil {
		return ""
	}

	var result strings.Builder
	for i, static := range tree.Statics {
		result.WriteString(static)
		if i < len(tree.Statics)-1 {
			key := fmt.Sprintf("%d", i)
			if dyn, ok := tree.Dynamics[key]; ok {
				switch v := dyn.(type) {
				case string:
					result.WriteString(v)
				case *TreeNode:
					result.WriteString(treeToHTML(v))
				default:
					fmt.Fprintf(&result, "%v", v)
				}
			}
		}
	}
	return result.String()
}

// stdlibRender executes a template using the standard html/template engine.
func stdlibRender(t *testing.T, tmplStr string, data interface{}, funcMap template.FuncMap) string {
	t.Helper()
	tmpl := template.New("test")
	if funcMap != nil {
		tmpl = tmpl.Funcs(funcMap)
	}
	tmpl, err := tmpl.Parse(tmplStr)
	if err != nil {
		t.Fatalf("stdlib parse failed: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("stdlib execute failed: %v", err)
	}
	return buf.String()
}

// lvtRender builds a tree using livetemplate's parse engine and reconstructs HTML.
func lvtRender(t *testing.T, tmplStr string, data interface{}, funcMap template.FuncMap) string {
	t.Helper()
	tmpl, err := Parse(tmplStr, funcMap)
	if err != nil {
		t.Fatalf("livetemplate parse failed: %v", err)
	}
	ctx := &Context{IncludeStatics: true, FuncMap: funcMap}
	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("livetemplate BuildTree failed: %v", err)
	}
	return treeToHTML(tree)
}

// TestStdlibParity_SimpleFields tests that simple field access produces identical output.
func TestStdlibParity_SimpleFields(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "string field",
			tmpl: "<span>{{.Name}}</span>",
			data: map[string]interface{}{"Name": "John"},
		},
		{
			name: "integer field",
			tmpl: "<span>{{.Count}}</span>",
			data: map[string]interface{}{"Count": 42},
		},
		{
			name: "empty string field",
			tmpl: "<span>{{.Name}}</span>",
			data: map[string]interface{}{"Name": ""},
		},
		{
			name: "multiple fields",
			tmpl: "<div>{{.First}} {{.Last}}</div>",
			data: map[string]interface{}{"First": "Jane", "Last": "Doe"},
		},
		{
			name: "nested html",
			tmpl: "<div class=\"test\">{{.Value}}</div>",
			data: map[string]interface{}{"Value": "hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdlib := stdlibRender(t, tt.tmpl, tt.data, nil)
			lvt := lvtRender(t, tt.tmpl, tt.data, nil)

			if stdlib != lvt {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvt)
			}
		})
	}
}

// TestStdlibParity_Conditionals tests if/else branch selection matches.
func TestStdlibParity_Conditionals(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "true condition",
			tmpl: "<div>{{if .Show}}visible{{end}}</div>",
			data: map[string]interface{}{"Show": true},
		},
		{
			name: "false condition",
			tmpl: "<div>{{if .Show}}visible{{end}}</div>",
			data: map[string]interface{}{"Show": false},
		},
		{
			name: "if-else true",
			tmpl: "<div>{{if .Show}}yes{{else}}no{{end}}</div>",
			data: map[string]interface{}{"Show": true},
		},
		{
			name: "if-else false",
			tmpl: "<div>{{if .Show}}yes{{else}}no{{end}}</div>",
			data: map[string]interface{}{"Show": false},
		},
		{
			name: "truthy string",
			tmpl: "{{if .Name}}has name{{else}}no name{{end}}",
			data: map[string]interface{}{"Name": "John"},
		},
		{
			name: "falsy empty string",
			tmpl: "{{if .Name}}has name{{else}}no name{{end}}",
			data: map[string]interface{}{"Name": ""},
		},
		{
			name: "truthy slice",
			tmpl: "{{if .Items}}has items{{else}}empty{{end}}",
			data: map[string]interface{}{"Items": []string{"a"}},
		},
		{
			name: "falsy nil slice",
			tmpl: "{{if .Items}}has items{{else}}empty{{end}}",
			data: map[string]interface{}{"Items": []string(nil)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdlib := stdlibRender(t, tt.tmpl, tt.data, nil)
			lvt := lvtRender(t, tt.tmpl, tt.data, nil)

			if stdlib != lvt {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvt)
			}
		})
	}
}

// TestStdlibParity_Pipelines tests pipeline expressions.
func TestStdlibParity_Pipelines(t *testing.T) {
	funcs := template.FuncMap{
		"upper": strings.ToUpper,
		"add":   func(a, b int) int { return a + b },
	}

	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "simple pipe",
			tmpl: "{{.Name | upper}}",
			data: map[string]interface{}{"Name": "john"},
		},
		{
			name: "printf pipe",
			tmpl: `{{printf "%s %s" .First .Last}}`,
			data: map[string]interface{}{"First": "Jane", "Last": "Doe"},
		},
		{
			name: "len function",
			tmpl: "{{len .Items}}",
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdlib := stdlibRender(t, tt.tmpl, tt.data, funcs)
			lvt := lvtRender(t, tt.tmpl, tt.data, funcs)

			if stdlib != lvt {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvt)
			}
		})
	}
}

// TestStdlibParity_WithBlocks tests with block behavior.
func TestStdlibParity_WithBlocks(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "with non-nil",
			tmpl: "<div>{{with .Item}}{{.Name}}{{end}}</div>",
			data: map[string]interface{}{
				"Item": map[string]interface{}{"Name": "found"},
			},
		},
		{
			name: "with root access via $",
			tmpl: "{{with .Item}}{{.Name}} from {{$.Title}}{{end}}",
			data: map[string]interface{}{
				"Title": "root",
				"Item":  map[string]interface{}{"Name": "child"},
			},
		},
		{
			name: "with root access standalone $",
			tmpl: `{{with .Item}}{{$.Title}}{{end}}`,
			data: map[string]interface{}{
				"Title": "hello",
				"Item":  map[string]interface{}{"Name": "child"},
			},
		},
		{
			name: "with nil value",
			tmpl: "<div>{{with .Item}}{{.Name}}{{end}}</div>",
			data: map[string]interface{}{"Item": nil},
		},
		{
			name: "with nil and else",
			tmpl: "{{with .Item}}{{.Name}}{{else}}nothing{{end}}",
			data: map[string]interface{}{"Item": nil},
		},
		{
			name: "with empty string",
			tmpl: "{{with .Val}}got {{.}}{{else}}empty{{end}}",
			data: map[string]interface{}{"Val": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdlib := stdlibRender(t, tt.tmpl, tt.data, nil)
			lvt := lvtRender(t, tt.tmpl, tt.data, nil)

			if stdlib != lvt {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvt)
			}
		})
	}
}

// TestStdlibParity_ConditionWithFields tests conditions that access dot fields,
// verifying the same branch is selected.
func TestStdlibParity_ConditionWithFields(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "eq comparison true",
			tmpl: `{{if eq .Status "active"}}active{{else}}inactive{{end}}`,
			data: map[string]interface{}{"Status": "active"},
		},
		{
			name: "eq comparison false",
			tmpl: `{{if eq .Status "active"}}active{{else}}inactive{{end}}`,
			data: map[string]interface{}{"Status": "disabled"},
		},
		{
			name: "not condition",
			tmpl: `{{if not .Hidden}}visible{{else}}hidden{{end}}`,
			data: map[string]interface{}{"Hidden": false},
		},
		{
			name: "and condition",
			tmpl: `{{if and .A .B}}both{{else}}not both{{end}}`,
			data: map[string]interface{}{"A": true, "B": true},
		},
		{
			name: "or condition",
			tmpl: `{{if or .A .B}}some{{else}}none{{end}}`,
			data: map[string]interface{}{"A": false, "B": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdlib := stdlibRender(t, tt.tmpl, tt.data, nil)
			lvt := lvtRender(t, tt.tmpl, tt.data, nil)

			if stdlib != lvt {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvt)
			}
		})
	}
}

// TestStdlibParity_RangeVarWithDotField tests that range bodies accessing both
// declared variables ($index) and dot fields (.Name) produce the same output as
// the standard library. This is the primary regression scenario from the PR:
// $index was not being resolved in pipe expressions.
// Uses BuildTree + direct dynamic inspection because treeToHTML does not handle RangeData.
func TestStdlibParity_RangeVarWithDotField(t *testing.T) {
	type Item struct {
		Name string
	}

	tests := []struct {
		name    string
		tmpl    string
		data    interface{}
		wantStd string
	}{
		{
			name: "index with printf and dot field",
			tmpl: `{{range $i, $v := .Items}}#{{$i}}:{{$v.Name}} {{end}}`,
			data: map[string]interface{}{
				"Items": []Item{{Name: "a"}, {Name: "b"}},
			},
			wantStd: "#0:a #1:b ",
		},
		{
			name: "index piped to printf",
			tmpl: `{{range $i, $v := .Items}}{{$i | printf "#%d"}}={{.Name}} {{end}}`,
			data: map[string]interface{}{
				"Items": []Item{{Name: "x"}, {Name: "y"}, {Name: "z"}},
			},
			wantStd: "#0=x #1=y #2=z ",
		},
		{
			name: "var and dot field in same expression",
			tmpl: `{{range $i, $v := .Items}}{{printf "%d-%s" $i .Name}} {{end}}`,
			data: map[string]interface{}{
				"Items": []Item{{Name: "foo"}, {Name: "bar"}},
			},
			wantStd: "0-foo 1-bar ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify stdlib produces expected output
			stdlib := stdlibRender(t, tt.tmpl, tt.data, nil)
			if stdlib != tt.wantStd {
				t.Fatalf("stdlib output %q does not match expected %q", stdlib, tt.wantStd)
			}

			// Build LVT tree and extract range item dynamics to reconstruct output
			tmpl, err := Parse(tt.tmpl, nil)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			ctx := &Context{IncludeStatics: true}
			tree, err := BuildTree(tmpl, tt.data, newMockKeyGen(), ctx)
			if err != nil {
				t.Fatalf("BuildTree failed: %v", err)
			}

			// The range produces a tree with Range data containing items.
			// Each item is a *TreeNode with dynamics only (statics are shared).
			// Reconstruct output using the shared statics + per-item dynamics.
			if tree.Range == nil {
				t.Fatal("expected tree to have Range data")
			}

			var lvtOutput strings.Builder
			for _, item := range tree.Range.Items {
				itemTree, ok := item.(*TreeNode)
				if !ok {
					t.Fatalf("range item is %T, want *TreeNode", item)
				}
				// Reconstruct with shared range statics
				itemWithStatics := &TreeNode{
					Statics:  tree.Range.Statics,
					Dynamics: itemTree.Dynamics,
				}
				lvtOutput.WriteString(treeToHTML(itemWithStatics))
			}

			if lvtOutput.String() != stdlib {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvtOutput.String())
			}
		})
	}
}

// TestStdlibParity_HTMLEscaping tests that HTML-sensitive characters are handled.
func TestStdlibParity_HTMLEscaping(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "ampersand",
			tmpl: "{{.Val}}",
			data: map[string]interface{}{"Val": "a&b"},
		},
		{
			name: "angle brackets",
			tmpl: "{{.Val}}",
			data: map[string]interface{}{"Val": "<script>"},
		},
		{
			name: "quotes",
			tmpl: "{{.Val}}",
			data: map[string]interface{}{"Val": `he said "hi"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdlib := stdlibRender(t, tt.tmpl, tt.data, nil)
			lvt := lvtRender(t, tt.tmpl, tt.data, nil)

			if stdlib != lvt {
				t.Errorf("output mismatch:\n  stdlib: %q\n  lvt:    %q", stdlib, lvt)
			}
		})
	}
}
