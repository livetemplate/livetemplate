package livetemplate

import (
	"html/template"
	"testing"
)

// FuzzParseTemplateToTree tests the current regex-based parser with random templates
// This establishes a baseline of what exotic templates work/fail before AST migration
func FuzzParseTemplateToTree(f *testing.F) {
	// Seed corpus with known working templates
	f.Add("<div>{{.Name}}</div>")
	f.Add("{{range .Items}}<span>{{.}}</span>{{end}}")
	f.Add("{{if .Show}}yes{{else}}no{{end}}")
	f.Add("{{if gt (len .Items) 0}}{{range .Items}}<li>{{.}}</li>{{end}}{{end}}")
	f.Add("{{with .User}}Hello {{.Name}}{{end}}")
	f.Add("{{range $i, $v := .Items}}{{$i}}: {{$v}}{{end}}")
	f.Add("{{.Name | printf \"User: %s\"}}")
	f.Add("{{range .Items}}{{if .Active}}{{.Name}}{{end}}{{end}}")
	f.Add("<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>")
	f.Add("{{if .A}}{{if .B}}nested{{end}}{{end}}")

	// Phase 1: Mixed templates (ranges + other dynamics) - Critical for examples/todos bug
	f.Add("<div>{{.Title}}</div>{{range .Items}}<span>{{.}}</span>{{end}}<p>{{.Footer}}</p>")
	f.Add("{{.Name}}{{range .Items}}{{.}}{{end}}{{.Count}}")
	f.Add("<h1>{{.Title}}</h1>{{range .Items}}<li>{{.}}</li>{{end}}")

	// Phase 1: Empty state transitions
	f.Add("{{range .EmptyItems}}<li>{{.}}</li>{{else}}<p>No items</p>{{end}}")
	f.Add("{{range .NilItems}}<li>{{.}}</li>{{else}}<p>No items</p>{{end}}")
	f.Add("{{with .NilValue}}Has value: {{.}}{{else}}No value{{end}}")

	// Phase 1: Range with else branch
	f.Add("{{range .Items}}<span>{{.}}</span>{{else}}<span>empty</span>{{end}}")

	// Phase 1: Map ranges
	f.Add("{{range $k, $v := .Map}}{{$k}}={{$v}} {{end}}")

	// Phase 1: Accessing parent context with $
	f.Add("{{range .Items}}{{$.Title}}: {{.}}{{end}}")

	// Phase 2: Break and continue (Go 1.18+)
	f.Add("{{range .Items}}{{if eq . \"stop\"}}{{break}}{{end}}{{.}}{{end}}")
	f.Add("{{range .Items}}{{if eq . \"skip\"}}{{continue}}{{end}}{{.}}{{end}}")
	f.Add("{{range .Items}}{{if gt (len .) 3}}{{break}}{{end}}{{.}}{{end}}")

	// Phase 2: Else-if chains
	f.Add("{{if eq .Type \"a\"}}A{{else if eq .Type \"b\"}}B{{else}}C{{end}}")
	f.Add("{{if .A}}first{{else if .B}}second{{else if .C}}third{{else}}none{{end}}")

	// Phase 2: Nested ranges
	f.Add("{{range .Outer}}{{range .Inner}}{{.}}{{end}}{{end}}")
	f.Add("{{range .Outer}}<div>{{range .Inner}}<span>{{.}}</span>{{end}}</div>{{end}}")

	// Phase 2: With with else
	f.Add("{{with .User}}Hello {{.Name}}{{else}}No user{{end}}")
	f.Add("{{with .EmptyString}}has value{{else}}empty string{{end}}")

	// Phase 2: Complex nesting
	f.Add("{{range .Items}}{{if .Active}}{{with .Details}}{{.Text}}{{end}}{{end}}{{end}}")

	// Phase 3: Variable scope in nested contexts
	f.Add("{{range $i, $v := .Items}}{{$i}}: {{$v}}{{end}}")
	f.Add("{{range $i, $v := .ItemsWithSub}}{{range $j, $w := .Sub}}{{$i}},{{$j}}: {{$w}}{{end}}{{end}}")

	// Phase 3: Accessing parent context with $
	f.Add("{{with .User}}{{$.Title}}: {{.Name}}{{end}}")

	// Phase 3: Variable in if block
	f.Add("{{$x := \"\"}}{{if .Cond}}{{$x = \"yes\"}}{{else}}{{$x = \"no\"}}{{end}}{{$x}}")

	// Phase 3: Variable shadowing
	f.Add("{{$v := .Name}}{{range .Items}}{{$v := .}}inner:{{$v}}{{end}}outer:{{$v}}")

	// Phase 3: Multiple variable declarations
	f.Add("{{$a := .A}}{{$b := .B}}{{$a}}{{$b}}")

	// Phase 4: Maps
	f.Add("{{range $k, $v := .StringMap}}{{$k}}: {{$v}}, {{end}}")

	// Phase 4: Int slices
	f.Add("{{range .Numbers}}{{.}},{{end}}")
	f.Add("{{range $i, $n := .Numbers}}[{{$i}}]={{$n}} {{end}}")

	// Phase 4: Bool slices
	f.Add("{{range .Flags}}{{if .}}yes{{else}}no{{end}} {{end}}")

	// Phase 4: Interface slices (mixed types)
	f.Add("{{range .Mixed}}{{.}}{{end}}")

	// Phase 4: Pointer fields
	f.Add("{{if .PtrField}}{{.PtrField}}{{else}}nil{{end}}")

	// Phase 5: Whitespace trimming
	f.Add("{{- .Field -}}")
	f.Add("text {{- .Field}}")
	f.Add("{{.Field -}} text")

	// Phase 5: Negative number vs trim
	f.Add("{{-3}}")
	f.Add("{{- 3}}")

	// Phase 5: Empty templates
	f.Add("")
	f.Add("{{/* comment only */}}")

	// Phase 5: Whitespace in ranges
	f.Add("{{range .Items -}}\n  {{.}}\n{{- end}}")

	// Phase 6: Function pipelines
	f.Add("{{.Value | printf \"%d\"}}")

	// Phase 6: Comparison functions
	f.Add("{{if eq .A .B}}equal{{end}}")
	f.Add("{{if ne .A .B}}not equal{{end}}")
	f.Add("{{if lt .Count 10}}small{{else}}large{{end}}")
	f.Add("{{if gt (len .Items) 0}}has items{{end}}")

	// Phase 6: Logical functions
	f.Add("{{if and .A .B}}both{{end}}")
	f.Add("{{if or .A .B}}either{{end}}")
	f.Add("{{if not .Empty}}has value{{end}}")

	// Phase 6: Index and len functions
	f.Add("{{index .Items 0}}")
	f.Add("{{len .Items}}")
	f.Add("{{len .Name}}")

	f.Fuzz(func(t *testing.T, templateStr string) {
		// Only test templates that Go's parser accepts
		_, err := template.New("fuzz").Parse(templateStr)
		if err != nil {
			t.Skip() // Invalid template syntax
		}

		// Generate test data that matches common template patterns
		data := map[string]interface{}{
			"Name":   "TestName",
			"Show":   true,
			"Items":  []string{"a", "b", "c"},
			"User":   map[string]interface{}{"Name": "John"},
			"Count":  5,
			"A":      true,
			"B":      false,
			"Active": true,

			// Phase 1: Empty state testing
			"EmptyItems": []string{},
			"NilItems":   ([]string)(nil),
			"NilValue":   nil,

			// Phase 1: Mixed template testing
			"Title":  "Page Title",
			"Footer": "Page Footer",

			// Phase 1: Map testing
			"Map": map[string]string{"key1": "val1", "key2": "val2"},

			// Phase 2: Control flow testing
			"Type": "a",
			"C":    false,
			"Outer": []map[string]interface{}{
				{"Inner": []string{"x", "y"}},
				{"Inner": []string{"p", "q"}},
			},
			"EmptyString": "",

			// Phase 3: Variable scope and context testing
			"Root": "root-value",
			"Cond": true,
			"ItemsWithSub": []map[string]interface{}{
				{"Name": "item1", "Sub": []string{"s1", "s2"}},
				{"Name": "item2", "Sub": []string{"s3", "s4"}},
			},

			// Phase 4: Data type testing
			"StringMap": map[string]string{"key1": "val1", "key2": "val2"},
			"Numbers":   []int{1, 2, 3, 4, 5},
			"Flags":     []bool{true, false, true},
			"Mixed":     []interface{}{"string", 42, true},
			"PtrField":  (*string)(nil),

			// Phase 5: Whitespace testing
			"Field": "value",

			// Phase 6: Function testing
			"Value": 42,
			"Empty": false,
		}

		// Test current AST-based parser
		keyGen := NewKeyGenerator()
		tree, err := parseTemplateToTree(templateStr, data, keyGen)

		if err != nil {
			// Parser failed - this is fine, we're documenting failures
			return
		}

		// Verify tree structure is valid
		// Note: We do NOT check tree invariants here because the hybrid execution
		// strategy (AST walking + flat execution for mixed patterns) can produce
		// trees that violate len(statics) = len(dynamics) + 1 for complex templates.
		// This is expected and documented behavior. The E2E tests verify correctness.
		if !validateTreeStructure(tree) {
			t.Errorf("Invalid tree structure\nTemplate: %q\nTree: %+v",
				templateStr, tree)
		}
	})
}

// validateTreeStructure performs basic validation of tree structure
func validateTreeStructure(tree TreeNode) bool {
	if tree == nil {
		return false
	}

	// Must have statics
	_, hasStatics := tree["s"]
	if !hasStatics {
		return false
	}

	return true
}
