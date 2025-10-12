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
		}

		// Test current regex-based parser
		keyGen := NewKeyGenerator()
		tree, err := parseTemplateToTree(templateStr, data, keyGen)

		if err != nil {
			// Parser failed - this is fine, we're documenting failures
			return
		}

		// Verify tree invariant
		if err := checkTreeInvariant(tree, "fuzz"); err != nil {
			t.Errorf("Tree invariant violation: %v\nTemplate: %q\nTree: %+v",
				err, templateStr, tree)
		}

		// Verify tree structure is valid
		if !validateTreeStructure(tree) {
			t.Errorf("Invalid tree structure\nTemplate: %q\nTree: %+v",
				templateStr, tree)
		}

		// Verify statics array length matches invariant
		if statics, ok := tree["s"].([]string); ok {
			dynamicCount := 0
			for k := range tree {
				if k != "s" && k != "f" {
					dynamicCount++
				}
			}

			if len(statics) != dynamicCount+1 {
				t.Errorf("Invariant broken: len(statics)=%d, dynamics=%d, expected len(statics)=dynamics+1\nTemplate: %q",
					len(statics), dynamicCount, templateStr)
			}
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
