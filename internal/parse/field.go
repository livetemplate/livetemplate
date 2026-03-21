package parse

import (
	"text/template/parse"
)

// createSingleDynamicTree creates a tree node with a single dynamic value at position 0.
func createSingleDynamicTree(value string, ctx *Context) *TreeNode {
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic(0, value)
	return tree
}

// handleAction processes {{.Field}}, {{.Method}}, {{$var}}, and function calls.
func handleAction(node *parse.ActionNode, eval *evaluator, data interface{}, varCtx *varContext, ctx *Context) (*TreeNode, error) {
	d := dot(data, varCtx)
	val, err := eval.evalPipe(node.Pipe, d, varCtx)
	if err != nil {
		return nil, &ParseError{
			Phase: "eval", NodeType: "action",
			Expr: node.String(),
			Err:  err,
		}
	}
	return createSingleDynamicTree(valueToString(val), ctx), nil
}
