package parse

import (
	"html/template"
	"testing"
	"text/template/parse"
)

// parseActionNode parses a template string and extracts the first ActionNode.
func parseActionNode(t *testing.T, tmplStr string, funcs template.FuncMap) *parse.ActionNode {
	t.Helper()
	tmpl := template.New("test")
	if funcs != nil {
		tmpl = tmpl.Funcs(funcs)
	}
	parsed, err := tmpl.Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template %q: %v", tmplStr, err)
	}
	for _, node := range parsed.Tree.Root.Nodes {
		if action, ok := node.(*parse.ActionNode); ok {
			return action
		}
	}
	t.Fatalf("no ActionNode found in template %q", tmplStr)
	return nil
}

// parseRangeNode parses a template string and extracts the first RangeNode.
func parseRangeNode(t *testing.T, tmplStr string, funcs template.FuncMap) *parse.RangeNode {
	t.Helper()
	tmpl := template.New("test")
	if funcs != nil {
		tmpl = tmpl.Funcs(funcs)
	}
	parsed, err := tmpl.Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template %q: %v", tmplStr, err)
	}
	for _, node := range parsed.Tree.Root.Nodes {
		if rangeNode, ok := node.(*parse.RangeNode); ok {
			return rangeNode
		}
	}
	t.Fatalf("no RangeNode found in template %q", tmplStr)
	return nil
}

// testEval creates a test evaluator with the given FuncMap.
func testEval(funcs template.FuncMap) *evaluator {
	return newEvaluator(funcs)
}
