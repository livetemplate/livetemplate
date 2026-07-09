package parse

import (
	"html/template"
	"strings"
	"text/template/parse"
)

// CollectReferencedIdentsFromTemplate collects referenced identifiers across every
// associated template in a parsed html/template set (the main template plus any
// {{define}}d component templates), returning their union. Used by the render path to
// scope method precomputation to names the templates could actually reference.
func CollectReferencedIdentsFromTemplate(tmpl *template.Template) map[string]struct{} {
	if tmpl == nil {
		return nil
	}
	var trees []*parse.Tree
	for _, assoc := range tmpl.Templates() {
		trees = append(trees, assoc.Tree)
	}
	return CollectReferencedIdents(trees...)
}

// CollectReferencedIdents walks parsed template ASTs and returns the set of every
// identifier that could reference a data field or method: each element of every
// field/chain/variable identifier chain plus every string literal (a string literal
// may be a dynamic map key, e.g. {{index . "Method"}}).
//
// The result is deliberately an over-approximation. Callers use it to decide which
// struct methods are worth eagerly precomputing, where including a name that is not
// really referenced is merely wasted work, but omitting one that is referenced would
// be a correctness bug (the method would silently resolve to nil). When in doubt the
// walk includes the identifier.
func CollectReferencedIdents(trees ...*parse.Tree) map[string]struct{} {
	set := make(map[string]struct{})
	for _, tr := range trees {
		if tr == nil {
			continue
		}
		collectNode(tr.Root, set)
	}
	return set
}

func collectNode(node parse.Node, set map[string]struct{}) {
	switch n := node.(type) {
	case nil:
		return
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			collectNode(child, set)
		}
	case *parse.ActionNode:
		collectPipe(n.Pipe, set)
	case *parse.IfNode:
		collectPipe(n.Pipe, set)
		collectNode(n.List, set)
		collectNode(n.ElseList, set)
	case *parse.RangeNode:
		collectPipe(n.Pipe, set)
		collectNode(n.List, set)
		collectNode(n.ElseList, set)
	case *parse.WithNode:
		collectPipe(n.Pipe, set)
		collectNode(n.List, set)
		collectNode(n.ElseList, set)
	case *parse.TemplateNode:
		collectPipe(n.Pipe, set)
	}
}

func collectPipe(pipe *parse.PipeNode, set map[string]struct{}) {
	if pipe == nil {
		return
	}
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			collectArg(arg, set)
		}
	}
}

func collectArg(arg parse.Node, set map[string]struct{}) {
	switch a := arg.(type) {
	case *parse.FieldNode:
		addIdents(a.Ident, set)
	case *parse.ChainNode:
		collectArg(a.Node, set)
		addIdents(a.Field, set)
	case *parse.VariableNode:
		addIdents(a.Ident, set)
	case *parse.PipeNode:
		collectPipe(a, set)
	case *parse.StringNode:
		if a.Text != "" {
			set[a.Text] = struct{}{}
		}
	}
}

// addIdents records each identifier in a field/variable chain. The leading element of
// a variable chain is the variable name itself ("$x"), which is skipped; the remaining
// elements are field/method accesses worth recording.
func addIdents(idents []string, set map[string]struct{}) {
	for _, id := range idents {
		if id == "" || strings.HasPrefix(id, "$") {
			continue
		}
		set[id] = struct{}{}
	}
}
