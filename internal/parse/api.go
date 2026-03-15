package parse

import (
	"fmt"
	"html/template"
	"text/template/parse"
)

// Template represents a parsed template with its AST.
type Template struct {
	name string
	ast  *parse.Tree
}

// Parse parses a template string into an executable template structure.
func Parse(templateStr string, funcMap template.FuncMap) (*Template, error) {
	tmpl := template.New("temp")
	if len(funcMap) > 0 {
		tmpl = tmpl.Funcs(funcMap)
	}
	parsed, err := tmpl.Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}
	if parsed.Tree == nil || parsed.Tree.Root == nil {
		return nil, fmt.Errorf("template has no parse tree")
	}
	return &Template{
		name: parsed.Name(),
		ast:  parsed.Tree,
	}, nil
}

// BuildTree constructs a tree structure from the parsed AST and data.
func BuildTree(tmpl *Template, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	if ctx == nil {
		ctx = &Context{}
	}
	eval := newEvaluator(ctx.FuncMap)
	return walkAST(tmpl.ast.Root, eval, data, nil, keyGen, ctx)
}

// InvalidateExecutionCache is a no-op retained for API compatibility.
// The new evaluator does not use caches.
func InvalidateExecutionCache() {}
