package parse

import (
	"fmt"
	"strings"
	"text/template/parse"
)

// walkAST is the single unified AST walker.
// When varCtx is nil, expressions evaluate against data directly.
// When varCtx is non-nil, variable references resolve via varCtx.
func walkAST(node parse.Node, eval *evaluator, data interface{}, varCtx *varContext, ctx *Context) (*TreeNode, error) {
	if node == nil {
		return createEmptyTree(ctx), nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		return walkList(n, eval, data, varCtx, ctx)

	case *parse.TextNode:
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{string(n.Text)}), nil
		}
		return NewTreeNode(), nil

	case *parse.ActionNode:
		return handleAction(n, eval, data, varCtx, ctx)

	case *parse.IfNode:
		return handleIf(n, eval, data, varCtx, ctx)

	case *parse.RangeNode:
		return handleRange(n, eval, data, varCtx, ctx)

	case *parse.WithNode:
		return handleWith(n, eval, data, varCtx, ctx)

	case *parse.CommentNode:
		return NewTreeNode(), nil

	case *parse.TemplateNode:
		return nil, &ParseError{
			Phase: "build", NodeType: "template",
			Expr: n.Name,
			Msg:  "template invocation found - should be flattened",
		}

	default:
		return nil, &ParseError{
			Phase: "build", NodeType: fmt.Sprintf("%T", n),
			Msg: "unhandled node type",
		}
	}
}

// walkList processes a list of nodes and merges their trees.
func walkList(node *parse.ListNode, eval *evaluator, data interface{}, varCtx *varContext, ctx *Context) (*TreeNode, error) {
	if node == nil || len(node.Nodes) == 0 {
		return createEmptyTree(ctx), nil
	}

	// If any child has variable declarations, ensure varCtx exists
	if listHasVarDeclarations(node) && varCtx == nil {
		varCtx = &varContext{parent: data, vars: newOrderedVars(), dot: data}
	}

	var statics []string
	tree := NewTreeNode()
	dynamicIndex := 0
	statics = append(statics, "")

	for i, child := range node.Nodes {
		// Handle variable declarations
		if varCtx != nil {
			if actionNode, ok := child.(*parse.ActionNode); ok &&
				actionNode.Pipe != nil && len(actionNode.Pipe.Decl) > 0 {
				if err := registerVarDeclaration(eval, actionNode, varCtx); err != nil {
					return nil, &ParseError{
						Phase: "build", NodeType: "list",
						Msg: fmt.Sprintf("child node %d: var declaration", i),
						Err: err,
					}
				}
				continue
			}
		}

		childTree, err := walkAST(child, eval, data, varCtx, ctx)
		if err != nil {
			return nil, &ParseError{
				Phase: "build", NodeType: "list",
				Msg: fmt.Sprintf("child node %d (%T)", i, child),
				Err: err,
			}
		}

		// Range comprehension: embed as nested structure
		if childTree.HasRange() {
			if len(node.Nodes) == 1 {
				return childTree, nil
			}
			tree.SetDynamic(dynamicIndex, childTree)
			dynamicIndex++
			statics = append(statics, "")
			continue
		}

		// Merge child tree into current tree
		childStatics := childTree.Statics
		if len(childStatics) == 0 {
			continue
		}

		// First static of child appends to last static of parent
		if len(statics) > 0 && len(childStatics) > 0 {
			statics[len(statics)-1] += childStatics[0]
		}
		if len(childStatics) > 1 {
			statics = append(statics, childStatics[1:]...)
		}

		// Copy dynamic values, renumbering them (skip nil gaps)
		for _, v := range childTree.Dynamics {
			if v == nil {
				continue
			}
			tree.SetDynamic(dynamicIndex, v)
			dynamicIndex++
		}
	}

	for len(statics) <= dynamicIndex {
		statics = append(statics, "")
	}

	if ctx.ShouldIncludeStatics() {
		tree.Statics = statics
	}
	return tree, nil
}

// dot returns the current dot context — varCtx.dot if varCtx is set, else data.
func dot(data interface{}, varCtx *varContext) interface{} {
	if varCtx != nil {
		return varCtx.dot
	}
	return data
}

// createWithBodyVarCtx creates a varContext for the body of a with block.
// Copies inherited variables and sets the new dot context.
func createWithBodyVarCtx(data interface{}, parentVarCtx *varContext, newDot interface{}) *varContext {
	copiedVars := newOrderedVars()
	parent := data
	if parentVarCtx != nil {
		parent = parentVarCtx.parent
		parentVarCtx.vars.Range(func(key string, value interface{}) {
			copiedVars.Set(key, value)
		})
	}
	return &varContext{
		parent: parent,
		vars:   copiedVars,
		dot:    newDot,
	}
}

// buildRangeItemVarCtx creates the variable context for a single range iteration.
func buildRangeItemVarCtx(node *parse.RangeNode, indexOrKey interface{}, item interface{}, data interface{}, parentVarCtx *varContext, hasVarDecls bool) *varContext {
	parentData := data
	if parentVarCtx != nil {
		parentData = parentVarCtx.parent
	}

	vc := &varContext{
		parent: parentData,
		vars:   newOrderedVars(),
		dot:    item,
	}

	// Copy inherited variables from parent scope
	if parentVarCtx != nil {
		parentVarCtx.vars.Range(func(key string, value interface{}) {
			vc.vars.Set(key, value)
		})
	}

	// Populate range-declared variables (override inherited if same name)
	if hasVarDecls && len(node.Pipe.Decl) >= 1 {
		if len(node.Pipe.Decl) == 1 {
			varName := strings.TrimPrefix(node.Pipe.Decl[0].Ident[0], "$")
			vc.vars.Set(varName, item)
		} else {
			indexKeyVar := strings.TrimPrefix(node.Pipe.Decl[0].Ident[0], "$")
			valueVar := strings.TrimPrefix(node.Pipe.Decl[1].Ident[0], "$")
			vc.vars.Set(indexKeyVar, indexOrKey)
			vc.vars.Set(valueVar, item)
		}
	}

	return vc
}
