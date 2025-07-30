package main

import (
	"fmt"
	"html/template"

	"github.com/livefir/statetemplate"
)

func main() {
	analyzer := statetemplate.NewAdvancedTemplateAnalyzer()

	// Test with simple nested template
	tmplText := `
{{.Page.Title}}
{{define "header"}}
	<h1>{{.Site.Name}}</h1>
	<p>{{.User.Name}}</p>
{{end}}
{{template "header" .}}
`

	tmpl, err := template.New("test").Parse(tmplText)
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
		return
	}

	fmt.Printf("Main template tree: %v\n", tmpl.Tree.Root.String())

	// Check for associated templates
	fmt.Printf("Template names: %v\n", tmpl.Templates())
	for _, t := range tmpl.Templates() {
		if t.Tree != nil && t.Tree.Root != nil {
			fmt.Printf("Template %s: %v\n", t.Name(), t.Tree.Root.String())
		}
	}

	deps := analyzer.AnalyzeTemplate(tmpl)
	fmt.Printf("Found dependencies: %v\n", deps)
}
