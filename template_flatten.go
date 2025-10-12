package livetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"text/template/parse"
)

// flattenTemplate resolves all {{define}}/{{template}}/{{block}} constructs into a single template
// This allows tree generation to work with templates that use Go's template composition features
func flattenTemplate(tmpl *template.Template) (string, error) {
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
		return "", fmt.Errorf("template has no parse tree")
	}

	// Build map of all template definitions
	templates := make(map[string]*template.Template)
	for _, t := range tmpl.Templates() {
		templates[t.Name()] = t
	}

	// Walk the tree and flatten
	var buf bytes.Buffer
	if err := walkAndFlatten(mainTemplate.Tree.Root, templates, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// hasExecutableContent checks if a template node tree has executable content
// Returns false if it only contains {{define}} declarations
func hasExecutableContent(node *parse.ListNode) bool {
	if node == nil || len(node.Nodes) == 0 {
		return false
	}

	// Look for any node that represents actual execution, not just {{define}} declarations
	for _, n := range node.Nodes {
		switch n.(type) {
		case *parse.TextNode:
			// Non-whitespace text is executable content
			if txt := n.(*parse.TextNode); len(strings.TrimSpace(string(txt.Text))) > 0 {
				return true
			}
		case *parse.ActionNode:
			// Check if this is a {{define}} or {{block}} - these are declarations, not execution
			action := n.(*parse.ActionNode)
			if len(action.Pipe.Cmds) > 0 && len(action.Pipe.Cmds[0].Args) > 0 {
				if ident, ok := action.Pipe.Cmds[0].Args[0].(*parse.IdentifierNode); ok {
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

// walkAndFlatten recursively walks the AST and builds flattened template string
func walkAndFlatten(node parse.Node, templates map[string]*template.Template, buf *bytes.Buffer) error {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		// Process all child nodes
		for _, child := range n.Nodes {
			if err := walkAndFlatten(child, templates, buf); err != nil {
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
		buf.WriteString(formatPipe(n.Pipe))
		buf.WriteString("}}")

		if err := walkAndFlatten(n.List, templates, buf); err != nil {
			return err
		}

		if n.ElseList != nil {
			buf.WriteString("{{else}}")
			if err := walkAndFlatten(n.ElseList, templates, buf); err != nil {
				return err
			}
		}

		buf.WriteString("{{end}}")

	case *parse.RangeNode:
		// {{range}}...{{else}}...{{end}}
		buf.WriteString("{{range ")
		buf.WriteString(formatPipe(n.Pipe))
		buf.WriteString("}}")

		if err := walkAndFlatten(n.List, templates, buf); err != nil {
			return err
		}

		if n.ElseList != nil {
			buf.WriteString("{{else}}")
			if err := walkAndFlatten(n.ElseList, templates, buf); err != nil {
				return err
			}
		}

		buf.WriteString("{{end}}")

	case *parse.WithNode:
		// {{with}}...{{else}}...{{end}}
		buf.WriteString("{{with ")
		buf.WriteString(formatPipe(n.Pipe))
		buf.WriteString("}}")

		if err := walkAndFlatten(n.List, templates, buf); err != nil {
			return err
		}

		if n.ElseList != nil {
			buf.WriteString("{{else}}")
			if err := walkAndFlatten(n.ElseList, templates, buf); err != nil {
				return err
			}
		}

		buf.WriteString("{{end}}")

	case *parse.TemplateNode:
		// {{template "name" .}} - inline the template
		refTemplate, exists := templates[n.Name]
		if !exists {
			return fmt.Errorf("template %q not defined", n.Name)
		}

		if refTemplate.Tree == nil || refTemplate.Tree.Root == nil {
			return fmt.Errorf("template %q has no parse tree", n.Name)
		}

		// Handle data context
		// If template passes data (e.g., {{template "name" .Field}}), we need to adjust context
		// For now, we inline the template as-is and the context will be handled at execution
		if err := walkAndFlatten(refTemplate.Tree.Root, templates, buf); err != nil {
			return err
		}

	default:
		// For any node type we don't explicitly handle, try to preserve as-is
		// This includes BranchNode and other internal nodes
		buf.WriteString(n.String())
	}

	return nil
}

// formatPipe converts a pipe to its string representation
func formatPipe(pipe *parse.PipeNode) string {
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
		buf.WriteString(formatCommand(cmd))
	}

	return buf.String()
}

// formatCommand converts a command to its string representation
func formatCommand(cmd *parse.CommandNode) string {
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
			buf.WriteString(fmt.Sprintf("%q", a.Text))
		case *parse.NumberNode:
			buf.WriteString(a.String())
		case *parse.BoolNode:
			buf.WriteString(fmt.Sprintf("%v", a.True))
		case *parse.DotNode:
			buf.WriteString(".")
		case *parse.NilNode:
			buf.WriteString("nil")
		case *parse.PipeNode:
			// Nested function call - needs parentheses
			// e.g., (len .Items) in {{if gt (len .Items) 0}}
			buf.WriteString("(")
			buf.WriteString(formatPipe(a))
			buf.WriteString(")")
		default:
			buf.WriteString(arg.String())
		}
	}

	return buf.String()
}

// hasTemplateComposition checks if template uses {{define}}/{{template}}/{{block}}
func hasTemplateComposition(tmpl *template.Template) bool {
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

// findTopLevelTemplateInvocation finds the first {{template}} invocation at the top level
// (not inside {{define}} blocks) and returns the template name being invoked
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

// hasTemplateNode recursively checks for {{template}} or {{block}} nodes
func hasTemplateNode(node parse.Node) bool {
	if node == nil {
		return false
	}

	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return false
		}
		for _, child := range n.Nodes {
			if hasTemplateNode(child) {
				return true
			}
		}
	case *parse.IfNode:
		if n == nil {
			return false
		}
		if hasTemplateNode(n.List) {
			return true
		}
		if n.ElseList != nil && hasTemplateNode(n.ElseList) {
			return true
		}
	case *parse.RangeNode:
		if n == nil {
			return false
		}
		if hasTemplateNode(n.List) {
			return true
		}
		if n.ElseList != nil && hasTemplateNode(n.ElseList) {
			return true
		}
	case *parse.WithNode:
		if n == nil {
			return false
		}
		if hasTemplateNode(n.List) {
			return true
		}
		if n.ElseList != nil && hasTemplateNode(n.ElseList) {
			return true
		}
	case *parse.TemplateNode:
		return true
	}

	return false
}
