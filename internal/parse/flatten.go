package parse

import (
	"bytes"
	"fmt"
	"html/template"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/template/parse"
)

// FlattenTemplate resolves all {{define}}/{{template}}/{{block}} constructs into a single template.
// This allows tree generation to work with templates that use Go's template composition features.
//
// The function:
//  1. Identifies the main executable template (entry point)
//  2. Walks the AST and inlines all {{template}} invocations
//  3. Returns the flattened document, plus any recursion {{define}} blocks separately
//
// The two returns are deliberately not concatenated here. Templates on an
// invocation cycle cannot be inlined, so their bodies are re-emitted as
// {{define}} blocks for the build-time registry to pick up. Callers need those
// blocks in two different places — appended to the document for html/template,
// and appended to the extracted <body> content for the reactive parse — and
// handing them back separately lets each caller assemble what it needs. The
// alternative, concatenating them onto the document and having the body
// extractor scan them back out, made a pure HTML slicer encode this function's
// private output layout: appending anything after them would silently sweep it
// into the template and degrade recursion with no error (issue #496).
//
// defines is "" when nothing recursive was found, which is every
// non-recursive template.
func FlattenTemplate(tmpl *template.Template) (document string, defines string, err error) {
	// The main template is the one that was explicitly named when calling New()
	// This is the entry point for execution
	mainTemplate := tmpl

	// Check if the main template itself is executable
	// If it only contains {{define}} nodes with no actual execution, we need to find the entry point
	if mainTemplate.Tree != nil && mainTemplate.Tree.Root != nil {
		if !hasExecutableContent(mainTemplate.Tree.Root) {
			// Main template is only definitions, find the template invoked at top level
			// Look for {{template "name" .}} invocations to identify the entry point
			entryPointName := findTopLevelTemplateInvocation(mainTemplate.Tree.Root)

			if entryPointName != "" {
				// Found a top-level {{template}} invocation - use that as entry point
				for _, t := range tmpl.Templates() {
					if t.Name() == entryPointName {
						mainTemplate = t
						break
					}
				}
			} else {
				// No top-level invocation found, fall back to first template with executable content
				// This handles edge cases where template structure is unusual
				for _, t := range tmpl.Templates() {
					if t.Tree != nil && t.Tree.Root != nil && t.Name() != mainTemplate.Name() {
						if hasExecutableContent(t.Tree.Root) {
							mainTemplate = t
							break
						}
					}
				}
			}
		}
	}

	if mainTemplate.Tree == nil || mainTemplate.Tree.Root == nil {
		return "", "", fmt.Errorf("template has no parse tree")
	}

	// Build map of all template definitions
	templates := make(map[string]*template.Template)
	for _, t := range tmpl.Templates() {
		templates[t.Name()] = t
	}

	// Identify templates that participate in a cycle. Their {{template}} calls are
	// left un-inlined (emitted verbatim) and their bodies are re-emitted once as
	// {{define}} blocks, so the recursion is resolved at build time rather than by
	// infinite inlining. Non-recursive composition is unaffected — recursive is
	// empty and every call inlines as before.
	recursive := detectRecursiveTemplates(templates)

	// Walk the tree and flatten. The active-path stack is seeded with the entry
	// point's name so a self-referential entry point is caught on its first
	// re-invocation and mutual-recursion errors print the cycle as authored.
	// checkFlattenCycle stays as a backstop: if detection ever under-identifies a
	// cycle, the un-emitted call re-enters here and is caught with a clean error
	// instead of overflowing the stack.
	var buf bytes.Buffer
	if err := walkAndFlatten(mainTemplate.Tree.Root, templates, &buf, []string{mainTemplate.Name()}, recursive); err != nil {
		return "", "", err
	}

	// Emit each recursive template's flattened body as a {{define}} block, into a
	// buffer of its own. On re-parse these become associated templates that
	// parse.Parse collects into the recursion registry; at build time
	// invokeTemplate walks them per call. Bodies are flattened with the same
	// recursive set, so their own recursive calls stay verbatim while any
	// non-recursive {{template}} calls inline.
	// Sorted for deterministic output (stable fingerprints and caching).
	// detectRecursiveTemplates only records names whose templates have a non-nil
	// Tree/Root, so templates[name] is safe to dereference here.
	var defBuf bytes.Buffer
	for _, name := range slices.Sorted(maps.Keys(recursive)) {
		body := templates[name].Tree.Root
		defBuf.WriteString("{{define ")
		defBuf.WriteString(strconv.Quote(name))
		defBuf.WriteString("}}")
		if err := walkAndFlatten(body, templates, &defBuf, []string{name}, recursive); err != nil {
			return "", "", err
		}
		defBuf.WriteString("{{end}}")
	}

	// document + defines reproduces exactly what this function used to return as
	// a single string; callers that need both simply concatenate.
	return buf.String(), defBuf.String(), nil
}

// detectRecursiveTemplates returns the set of template names that participate in
// a cycle in the invocation graph — a name reachable from itself by following
// {{template}} calls. Direct self-recursion, mutual recursion, and longer cycles
// are all captured; a template merely on a path *into* a cycle is not.
func detectRecursiveTemplates(templates map[string]*template.Template) map[string]bool {
	graph := make(map[string][]string, len(templates))
	for name, t := range templates {
		if t.Tree == nil || t.Tree.Root == nil {
			continue
		}
		graph[name] = collectInvokedTemplateNames(t.Tree.Root)
	}

	recursive := make(map[string]bool)
	for name := range graph {
		if reachableFromSelf(name, graph) {
			recursive[name] = true
		}
	}
	return recursive
}

// collectInvokedTemplateNames returns the names invoked by {{template}} nodes
// anywhere within node's subtree (duplicates allowed; callers only test membership).
func collectInvokedTemplateNames(node parse.Node) []string {
	var names []string
	forEachTemplateNode(node, func(tn *parse.TemplateNode) bool {
		names = append(names, tn.Name)
		return true
	})
	return names
}

// reachableFromSelf reports whether start can reach itself by following the
// invocation graph — i.e. start lies on a cycle.
func reachableFromSelf(start string, graph map[string][]string) bool {
	visited := make(map[string]bool)
	stack := append([]string(nil), graph[start]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == start {
			return true
		}
		if visited[n] {
			continue
		}
		visited[n] = true
		stack = append(stack, graph[n]...)
	}
	return false
}

// HasTemplateComposition checks if template uses {{define}}/{{template}}/{{block}}.
// Returns true if the template uses composition features that require flattening.
func HasTemplateComposition(tmpl *template.Template) bool {
	// Check if template has associated templates (from {{define}})
	if len(tmpl.Templates()) > 1 {
		return true
	}

	// Check if template tree contains {{template}} nodes
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		return hasTemplateNode(tmpl.Tree.Root)
	}

	return false
}

// hasExecutableContent checks if a template node tree has executable content.
// Returns false if it only contains {{define}} declarations.
func hasExecutableContent(node *parse.ListNode) bool {
	if node == nil || len(node.Nodes) == 0 {
		return false
	}

	// Look for any node that represents actual execution, not just {{define}} declarations
	for _, n := range node.Nodes {
		switch typed := n.(type) {
		case *parse.TextNode:
			// Non-whitespace text is executable content
			if len(strings.TrimSpace(string(typed.Text))) > 0 {
				return true
			}
		case *parse.ActionNode:
			// Check if this is a {{define}} or {{block}} - these are declarations, not execution
			if len(typed.Pipe.Cmds) > 0 && len(typed.Pipe.Cmds[0].Args) > 0 {
				if ident, ok := typed.Pipe.Cmds[0].Args[0].(*parse.IdentifierNode); ok {
					if ident.Ident == "define" || ident.Ident == "block" {
						// This is a declaration, keep looking
						continue
					}
				}
			}
			// Any other action is executable
			return true
		case *parse.TemplateNode:
			// {{template}} invocation is executable content
			return true
		case *parse.IfNode, *parse.RangeNode, *parse.WithNode:
			// Control structures are executable content
			return true
		}
	}

	// Only found declarations or whitespace
	return false
}

// findTopLevelTemplateInvocation finds the first {{template}} invocation at the top level
// (not inside {{define}} blocks) and returns the template name being invoked.
func findTopLevelTemplateInvocation(node *parse.ListNode) string {
	if node == nil || len(node.Nodes) == 0 {
		return ""
	}

	for _, n := range node.Nodes {
		switch child := n.(type) {
		case *parse.TemplateNode:
			// Found a top-level {{template}} invocation
			return child.Name
		case *parse.ActionNode:
			// Skip {{define}} and {{block}} declarations - we only want invocations
			// These are not top-level invocations, they are declarations
			continue
		}
	}

	return ""
}

// forEachTemplateNode walks node's subtree and calls fn for every {{template}}
// (or {{block}}) invocation found, in source order. fn returns false to stop the
// walk early; forEachTemplateNode returns false in that case, true if it ran to
// completion. It descends List/If/Range/With bodies (both branches); a nil node
// is a no-op, so callers need no nil guards.
func forEachTemplateNode(node parse.Node, fn func(*parse.TemplateNode) bool) bool {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return true
		}
		for _, child := range n.Nodes {
			if !forEachTemplateNode(child, fn) {
				return false
			}
		}
	case *parse.IfNode:
		return forEachTemplateNode(n.List, fn) && forEachTemplateNode(n.ElseList, fn)
	case *parse.RangeNode:
		return forEachTemplateNode(n.List, fn) && forEachTemplateNode(n.ElseList, fn)
	case *parse.WithNode:
		return forEachTemplateNode(n.List, fn) && forEachTemplateNode(n.ElseList, fn)
	case *parse.TemplateNode:
		return fn(n)
	}
	return true
}

// hasTemplateNode recursively checks for {{template}} or {{block}} nodes.
func hasTemplateNode(node parse.Node) bool {
	found := false
	forEachTemplateNode(node, func(*parse.TemplateNode) bool {
		found = true
		return false // stop at the first hit
	})
	return found
}

// walkAndFlatten recursively walks the AST and builds flattened template string.
//
// stack holds the names of the templates whose bodies are currently being
// inlined on the path from the entry point to this node (an active-path set,
// not a global visited-set). A {{template "X"}} whose name is already on the
// stack is a self-referential cycle: inlining it would expand forever and
// stack-overflow at Parse, so it returns a ParseError instead. The same name
// invoked twice on non-nested paths (a diamond) is not a cycle and still
// inlines, because each invocation pops off the stack when its body returns.
func walkAndFlatten(node parse.Node, templates map[string]*template.Template, buf *bytes.Buffer, stack []string, recursive map[string]bool) error {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		// Process all child nodes
		for _, child := range n.Nodes {
			if err := walkAndFlatten(child, templates, buf, stack, recursive); err != nil {
				return err
			}
		}

	case *parse.TextNode:
		// Plain text - copy as-is
		buf.Write(n.Text)

	case *parse.ActionNode:
		// {{.Field}}, {{.Method}}, etc. - copy as-is
		buf.WriteString("{{")
		buf.WriteString(n.String()[2 : len(n.String())-2]) // Remove outer {{ }}
		buf.WriteString("}}")

	case *parse.IfNode:
		// {{if}}...{{else}}...{{end}}
		buf.WriteString("{{if ")
		buf.WriteString(formatPipeForFlatten(n.Pipe))
		buf.WriteString("}}")

		if err := walkAndFlatten(n.List, templates, buf, stack, recursive); err != nil {
			return err
		}

		if n.ElseList != nil {
			buf.WriteString("{{else}}")
			if err := walkAndFlatten(n.ElseList, templates, buf, stack, recursive); err != nil {
				return err
			}
		}

		buf.WriteString("{{end}}")

	case *parse.RangeNode:
		// {{range}}...{{else}}...{{end}}
		buf.WriteString("{{range ")
		buf.WriteString(formatPipeForFlatten(n.Pipe))
		buf.WriteString("}}")

		if err := walkAndFlatten(n.List, templates, buf, stack, recursive); err != nil {
			return err
		}

		if n.ElseList != nil {
			buf.WriteString("{{else}}")
			if err := walkAndFlatten(n.ElseList, templates, buf, stack, recursive); err != nil {
				return err
			}
		}

		buf.WriteString("{{end}}")

	case *parse.WithNode:
		// {{with}}...{{else}}...{{end}}
		buf.WriteString("{{with ")
		buf.WriteString(formatPipeForFlatten(n.Pipe))
		buf.WriteString("}}")

		if err := walkAndFlatten(n.List, templates, buf, stack, recursive); err != nil {
			return err
		}

		if n.ElseList != nil {
			buf.WriteString("{{else}}")
			if err := walkAndFlatten(n.ElseList, templates, buf, stack, recursive); err != nil {
				return err
			}
		}

		buf.WriteString("{{end}}")

	case *parse.TemplateNode:
		// A recursive template is left un-inlined: emit the invocation verbatim so
		// it survives re-parse as a {{template}} node and is evaluated at build
		// time (see invokeTemplate). Its body is emitted once as a {{define}} block
		// by FlattenTemplate. This branch is what breaks the infinite inlining that
		// checkFlattenCycle below can only detect after the fact.
		if recursive[n.Name] {
			buf.WriteString("{{template ")
			buf.WriteString(strconv.Quote(n.Name))
			if n.Pipe != nil {
				buf.WriteByte(' ')
				buf.WriteString(formatPipeForFlatten(n.Pipe))
			}
			buf.WriteString("}}")
			return nil
		}

		// {{template "name" .}} - inline the template
		refTemplate, exists := templates[n.Name]
		if !exists {
			return fmt.Errorf("template %q not defined", n.Name)
		}

		if refTemplate.Tree == nil || refTemplate.Tree.Root == nil {
			return fmt.Errorf("template %q has no parse tree", n.Name)
		}

		// Backstop only: recursive names were already marked above and returned
		// verbatim, so a name on the active path here means the recursive set
		// disagrees with this walk. Erroring beats overflowing the stack.
		if err := checkFlattenCycle(n, stack); err != nil {
			return err
		}
		// Push this name for the body recursion below. Reusing stack's backing
		// array across sibling {{template}} invocations is safe only because the
		// walk is strictly sequential (each child fully returns before the next
		// starts) — a stale tail is never read concurrently. This invariant must
		// hold if the walk is ever parallelized.
		stack = append(stack, n.Name)

		// Handle data context changes
		// If template invocation passes a different context (e.g., {{template "name" .Field}}),
		// we need to wrap the inlined template in {{with}} to change the context
		needsContextWrapper := false
		if n.Pipe != nil {
			pipeStr := formatPipeForFlatten(n.Pipe)
			// Only wrap if the pipe is not "." (which means same context)
			if pipeStr != "." {
				needsContextWrapper = true
			}
		}

		if needsContextWrapper {
			// Wrap template body in {{with}} to change context
			buf.WriteString("{{with ")
			buf.WriteString(formatPipeForFlatten(n.Pipe))
			buf.WriteString("}}")

			if err := walkAndFlatten(refTemplate.Tree.Root, templates, buf, stack, recursive); err != nil {
				return err
			}

			buf.WriteString("{{end}}")
		} else {
			// No context change needed - inline as-is
			if err := walkAndFlatten(refTemplate.Tree.Root, templates, buf, stack, recursive); err != nil {
				return err
			}
		}

	default:
		// For any node type we don't explicitly handle, try to preserve as-is
		// This includes BranchNode and other internal nodes
		buf.WriteString(n.String())
	}

	return nil
}

// checkFlattenCycle is a backstop against inlining a cycle forever. It is not
// reachable through FlattenTemplate: detectRecursiveTemplates runs first over
// the same invocation edges this walk follows, so any name on a cycle is marked
// recursive and emitted verbatim rather than pushed onto the stack. It stays as
// a guard for direct walkAndFlatten callers (and any future traversal that
// drifts from detectRecursiveTemplates), where an under-populated recursive set
// would otherwise expand forever and overflow the stack.
//
// Recursion itself is supported — see invokeTemplate for the runtime path.
func checkFlattenCycle(n *parse.TemplateNode, stack []string) error {
	i := slices.Index(stack, n.Name)
	if i < 0 {
		return nil
	}
	cycle := strings.Join(stack[i:], " -> ") + " -> " + n.Name
	return &ParseError{
		Phase:    "parse",
		NodeType: "template",
		Expr:     n.Name,
		Pos:      int(n.Position()),
		Msg: fmt.Sprintf("internal: reached the flatten-cycle backstop for %s; "+
			"recursive templates should have been detected before inlining "+
			"(this indicates detectRecursiveTemplates and walkAndFlatten "+
			"disagree about the invocation graph)", cycle),
	}
}

// formatPipe converts a pipe to its string representation.
func formatPipeForFlatten(pipe *parse.PipeNode) string {
	if pipe == nil {
		return ""
	}

	var buf bytes.Buffer

	// Handle declarations like $var := expr
	if len(pipe.Decl) > 0 {
		for i, decl := range pipe.Decl {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(decl.String())
		}
		buf.WriteString(" := ")
	}

	// Handle commands
	for i, cmd := range pipe.Cmds {
		if i > 0 {
			buf.WriteString(" | ")
		}
		buf.WriteString(formatCommandForFlatten(cmd))
	}

	return buf.String()
}

// formatCommand converts a command to its string representation.
func formatCommandForFlatten(cmd *parse.CommandNode) string {
	if cmd == nil {
		return ""
	}

	var buf bytes.Buffer
	for i, arg := range cmd.Args {
		if i > 0 {
			buf.WriteString(" ")
		}

		switch a := arg.(type) {
		case *parse.FieldNode:
			buf.WriteString(a.String())
		case *parse.IdentifierNode:
			buf.WriteString(a.Ident)
		case *parse.StringNode:
			fmt.Fprintf(&buf, "%q", a.Text)
		case *parse.NumberNode:
			buf.WriteString(a.String())
		case *parse.BoolNode:
			fmt.Fprintf(&buf, "%v", a.True)
		case *parse.DotNode:
			buf.WriteString(".")
		case *parse.NilNode:
			buf.WriteString("nil")
		case *parse.PipeNode:
			// Nested function call - needs parentheses
			// e.g., (len .Items) in {{if gt (len .Items) 0}}
			buf.WriteString("(")
			buf.WriteString(formatPipeForFlatten(a))
			buf.WriteString(")")
		default:
			buf.WriteString(arg.String())
		}
	}

	return buf.String()
}
