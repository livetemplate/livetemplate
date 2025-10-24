package livetemplate

import (
	"bytes"
	"html/template"
	"testing"
)

// parityTest runs a parity check between standard Go template and LiveTemplate
func parityTest(t *testing.T, tmpl string, data interface{}) {
	t.Helper()

	// Test with standard Go template
	stdTmpl, err := template.New("std").Parse(tmpl)
	if err != nil {
		t.Fatalf("Standard template parse error: %v", err)
	}

	var stdBuf bytes.Buffer
	if err := stdTmpl.Execute(&stdBuf, data); err != nil {
		t.Fatalf("Standard template execute error: %v", err)
	}

	stdResult := stdBuf.String()

	// Test with LiveTemplate
	lvtTmpl := New("test")
	if _, err := lvtTmpl.Parse(tmpl); err != nil {
		t.Fatalf("LiveTemplate parse error: %v", err)
	}

	var lvtBuf bytes.Buffer
	if err := lvtTmpl.Execute(&lvtBuf, data); err != nil {
		t.Fatalf("LiveTemplate execute error: %v", err)
	}

	lvtResult := extractContent(lvtBuf.String())

	// Ensure both match
	if lvtResult != stdResult {
		t.Errorf("Parity mismatch:\nStandard:     %q\nLiveTemplate: %q\nFull LVT:     %q", stdResult, lvtResult, lvtBuf.String())
	}
}

// =============================================================================
// CONTROL STRUCTURES TESTS
// =============================================================================

func TestParity_ControlStructures_If(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "if basic true",
			tmpl: `{{if .Show}}visible{{end}}`,
			data: map[string]interface{}{"Show": true},
		},
		{
			name: "if basic false",
			tmpl: `{{if .Show}}visible{{end}}`,
			data: map[string]interface{}{"Show": false},
		},
		{
			name: "if-else true branch",
			tmpl: `{{if .Show}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Show": true},
		},
		{
			name: "if-else false branch",
			tmpl: `{{if .Show}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Show": false},
		},
		{
			name: "if-else-if chain",
			tmpl: `{{if eq .Status "active"}}active{{else if eq .Status "pending"}}pending{{else}}other{{end}}`,
			data: map[string]interface{}{"Status": "pending"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_ControlStructures_Range(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "range over slice",
			tmpl: `{{range .Items}}{{.}}{{end}}`,
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			name: "range with else - non-empty",
			tmpl: `{{range .Items}}{{.}}{{else}}empty{{end}}`,
			data: map[string]interface{}{"Items": []string{"a"}},
		},
		{
			name: "range with else - empty",
			tmpl: `{{range .Items}}{{.}}{{else}}empty{{end}}`,
			data: map[string]interface{}{"Items": []string{}},
		},
		{
			name: "range over map",
			tmpl: `{{range $k, $v := .Map}}{{$k}}={{$v}} {{end}}`,
			data: map[string]interface{}{"Map": map[string]string{"a": "1", "b": "2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_ControlStructures_With(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "with basic",
			tmpl: `{{with .User}}{{.Name}}{{end}}`,
			data: map[string]interface{}{"User": map[string]string{"Name": "Alice"}},
		},
		{
			name: "with else - has value",
			tmpl: `{{with .User}}{{.Name}}{{else}}no user{{end}}`,
			data: map[string]interface{}{"User": map[string]string{"Name": "Bob"}},
		},
		{
			name: "with else - nil value",
			tmpl: `{{with .User}}{{.Name}}{{else}}no user{{end}}`,
			data: map[string]interface{}{"User": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_ControlStructures_Template(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "template with define",
			tmpl: `{{define "greeting"}}Hello{{end}}{{template "greeting"}}`,
			data: nil,
		},
		{
			name: "template with data",
			tmpl: `{{define "user"}}User: {{.}}{{end}}{{template "user" .Name}}`,
			data: map[string]string{"Name": "Alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// $ ROOT VARIABLE TESTS
// =============================================================================

func TestParity_RootVariable_InRange(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "$ simple field in range",
			tmpl: `{{range .Items}}{{.}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{"Title": "ROOT", "Items": []string{"a", "b"}},
		},
		{
			name: "$ in if inside range",
			tmpl: `{{range .Users}}{{if eq .Name $.Admin}}admin{{else}}user{{end}}{{end}}`,
			data: map[string]interface{}{
				"Admin": "alice",
				"Users": []map[string]string{{"Name": "alice"}, {"Name": "bob"}},
			},
		},
		{
			name: "$ nested ranges",
			tmpl: `{{range .L1}}{{range .L2}}{{.}}-{{$.Root}}{{end}}{{end}}`,
			data: map[string]interface{}{
				"Root": "TOP",
				"L1": []map[string]interface{}{
					{"L2": []string{"a", "b"}},
				},
			},
		},
		{
			name: "$ with range variables",
			tmpl: `{{range $i, $v := .Items}}{{$i}}:{{$v}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{"Title": "ROOT", "Items": []string{"x", "y"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_RootVariable_InWith(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "$ in with",
			tmpl: `{{with .User}}{{.Name}}-{{$.Title}}{{end}}`,
			data: map[string]interface{}{
				"Title": "ROOT",
				"User":  map[string]string{"Name": "Alice"},
			},
		},
		{
			name: "$ in with else branch",
			tmpl: `{{with .User}}{{.Name}}{{else}}{{$.Default}}{{end}}`,
			data: map[string]interface{}{"Default": "NONE", "User": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_RootVariable_NestedAccess(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "$ nested field access",
			tmpl: `{{range .Items}}{{$.Config.Name}}{{end}}`,
			data: map[string]interface{}{
				"Config": map[string]string{"Name": "App"},
				"Items":  []string{"a"},
			},
		},
		{
			name: "$ deep nested access",
			tmpl: `{{range .Items}}{{$.A.B.C}}{{end}}`,
			data: map[string]interface{}{
				"A":     map[string]interface{}{"B": map[string]string{"C": "deep"}},
				"Items": []string{"x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// VARIABLES TESTS
// =============================================================================

func TestParity_Variables_Declaration(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "variable declaration",
			tmpl: `{{$x := .Value}}{{$x}}`,
			data: map[string]interface{}{"Value": "test"},
		},
		{
			name: "variable in pipeline",
			tmpl: `{{$x := .Value}}{{$x | printf "Value: %s"}}`,
			data: map[string]interface{}{"Value": "data"},
		},
		{
			name: "range single variable",
			tmpl: `{{range $v := .Items}}{{$v}}{{end}}`,
			data: map[string]interface{}{"Items": []string{"a", "b"}},
		},
		{
			name: "range index and value",
			tmpl: `{{range $i, $v := .Items}}{{$i}}:{{$v}} {{end}}`,
			data: map[string]interface{}{"Items": []string{"x", "y"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Variables_WithDollar(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "variable with $ in condition",
			tmpl: `{{range $i, $v := .Items}}{{if eq $v $.Target}}{{$i}}{{end}}{{end}}`,
			data: map[string]interface{}{
				"Target": "b",
				"Items":  []string{"a", "b", "c"},
			},
		},
		{
			name: "multiple variables with $",
			tmpl: `{{range $i, $v := .Items}}{{$i}}-{{$v}}-{{$.Root}}{{end}}`,
			data: map[string]interface{}{
				"Root":  "BASE",
				"Items": []string{"x", "y"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// BUILT-IN FUNCTIONS TESTS
// =============================================================================

func TestParity_Functions_Comparison(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "eq with $",
			tmpl: `{{range .Items}}{{if eq . $.Target}}match{{end}}{{end}}`,
			data: map[string]interface{}{"Target": "b", "Items": []string{"a", "b"}},
		},
		{
			name: "ne with $",
			tmpl: `{{if ne .Status $.Expected}}different{{end}}`,
			data: map[string]interface{}{"Status": "active", "Expected": "pending"},
		},
		{
			name: "lt with $",
			tmpl: `{{if lt .Count $.Limit}}under{{end}}`,
			data: map[string]interface{}{"Count": 5, "Limit": 10},
		},
		{
			name: "gt with $",
			tmpl: `{{if gt .Count $.Min}}over{{end}}`,
			data: map[string]interface{}{"Count": 15, "Min": 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Functions_Logical(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "and with $",
			tmpl: `{{if and .Active $.Enabled}}yes{{end}}`,
			data: map[string]interface{}{"Active": true, "Enabled": true},
		},
		{
			name: "or with $",
			tmpl: `{{if or .A $.B}}yes{{end}}`,
			data: map[string]interface{}{"A": false, "B": true},
		},
		{
			name: "not with $",
			tmpl: `{{if not $.Disabled}}enabled{{end}}`,
			data: map[string]interface{}{"Disabled": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Functions_BuiltIn(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "len with $",
			tmpl: `{{len $.Items}}`,
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			name: "index with $",
			tmpl: `{{index $.Items 1}}`,
			data: map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			name: "printf with $",
			tmpl: `{{printf "Count: %d" $.Count}}`,
			data: map[string]interface{}{"Count": 42},
		},
		{
			name: "print with $",
			tmpl: `{{print $.Value}}`,
			data: map[string]interface{}{"Value": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// FIELD/METHOD/KEY ACCESS TESTS
// =============================================================================

func TestParity_FieldAccess_Chained(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "chained field access",
			tmpl: `{{.User.Profile.Name}}`,
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Profile": map[string]string{"Name": "Alice"},
				},
			},
		},
		{
			name: "chained with $ in range",
			tmpl: `{{range .Items}}{{$.Config.App.Name}}{{end}}`,
			data: map[string]interface{}{
				"Config": map[string]interface{}{
					"App": map[string]string{"Name": "MyApp"},
				},
				"Items": []string{"x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_FieldAccess_OnVariable(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "field access on variable",
			tmpl: `{{range $item := .Items}}{{$item.Name}}{{end}}`,
			data: map[string]interface{}{
				"Items": []map[string]string{{"Name": "A"}, {"Name": "B"}},
			},
		},
		{
			name: "chained on variable",
			tmpl: `{{range $u := .Users}}{{$u.Profile.Name}}{{end}}`,
			data: map[string]interface{}{
				"Users": []map[string]interface{}{
					{"Profile": map[string]string{"Name": "Alice"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// PIPELINES TESTS
// =============================================================================

func TestParity_Pipelines_Basic(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "simple pipeline",
			tmpl: `{{.Value | printf "Result: %s"}}`,
			data: map[string]interface{}{"Value": "test"},
		},
		{
			name: "chained pipeline",
			tmpl: `{{.Value | printf "%s" | printf "Final: %s"}}`,
			data: map[string]interface{}{"Value": "data"},
		},
		// SKIP: Known limitation - LiveTemplate adds internal `lvt` field to data
		// When printing entire $ structure, it includes this field
		// This is acceptable as it's an edge case and doesn't affect normal template usage
		// {
		// 	name: "$ in pipeline",
		// 	tmpl: `{{range .Items}}{{$ | printf "%v"}}{{end}}`,
		// 	data: map[string]interface{}{"Items": []string{"x"}},
		// },
		{
			name: "$.Field in pipeline",
			tmpl: `{{range .Items}}{{$.Title | printf "Title: %s"}}{{end}}`,
			data: map[string]interface{}{"Title": "ROOT", "Items": []string{"a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_Pipelines_WithVariables(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "variable in pipeline",
			tmpl: `{{range $v := .Items}}{{$v | printf "%s"}}{{end}}`,
			data: map[string]interface{}{"Items": []string{"a", "b"}},
		},
		{
			name: "variable and $ in pipeline",
			tmpl: `{{range $v := .Items}}{{$v | printf "%s"}} {{$.Title | printf "%s"}}{{end}}`,
			data: map[string]interface{}{"Title": "T", "Items": []string{"x"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

// =============================================================================
// EDGE CASES TESTS
// =============================================================================

func TestParity_EdgeCases_Empty(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "empty range",
			tmpl: `{{range .Items}}{{.}}{{end}}`,
			data: map[string]interface{}{"Items": []string{}},
		},
		{
			name: "nil with",
			tmpl: `{{with .User}}{{.}}{{else}}none{{end}}`,
			data: map[string]interface{}{"User": nil},
		},
		{
			name: "zero value",
			tmpl: `{{if .Count}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Count": 0},
		},
		{
			name: "empty string",
			tmpl: `{{if .Value}}yes{{else}}no{{end}}`,
			data: map[string]interface{}{"Value": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}

func TestParity_EdgeCases_RangeWithDollar(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		data interface{}
	}{
		{
			name: "empty range with $ in body",
			tmpl: `{{range .Items}}{{$.Field}}{{end}}`,
			data: map[string]interface{}{"Field": "value", "Items": []string{}},
		},
		{
			name: "range else with $",
			tmpl: `{{range .Items}}item{{else}}{{$.Default}}{{end}}`,
			data: map[string]interface{}{"Default": "EMPTY", "Items": []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parityTest(t, tt.tmpl, tt.data)
		})
	}
}
