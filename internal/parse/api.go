package parse

import (
	"fmt"
	"html/template"
	"reflect"
	"text/template/parse"
)

// Template represents a parsed template with its AST.
type Template struct {
	name     string
	ast      *parse.Tree
	builtins map[string]reflect.Value

	// registry holds the ASTs of recursively-invoked templates, keyed by name.
	// FlattenTemplate leaves recursive {{template}} calls un-inlined and appends
	// their (flattened) bodies as {{define}} blocks; those bodies re-parse here
	// as associated templates, which Parse collects into this map. Empty when the
	// template uses no recursion. Threaded to the evaluator by BuildTree.
	registry map[string]*parse.Tree
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

	// Collect any associated {{define}} templates into the recursion registry.
	// These are present only when FlattenTemplate detected recursion and emitted
	// the cycle members as defines; a normal (fully-inlined) template has none.
	var registry map[string]*parse.Tree
	for _, assoc := range tmpl.Templates() {
		if assoc.Name() == parsed.Name() || assoc.Tree == nil || assoc.Tree.Root == nil {
			continue
		}
		if registry == nil {
			registry = make(map[string]*parse.Tree)
		}
		registry[assoc.Name()] = assoc.Tree
	}

	return &Template{
		name:     parsed.Name(),
		ast:      parsed.Tree,
		builtins: precomputeBuiltins(funcMap),
		registry: registry,
	}, nil
}

// BuildTree constructs a tree structure from the parsed AST and data.
// tmpl.builtins is always set by Parse() via precomputeBuiltins.
func BuildTree(tmpl *Template, data interface{}, ctx *Context) (*TreeNode, error) {
	if ctx == nil {
		ctx = &Context{}
	}
	eval := &evaluator{builtins: tmpl.builtins, templates: tmpl.registry}
	return walkAST(tmpl.ast.Root, eval, data, nil, ctx)
}
