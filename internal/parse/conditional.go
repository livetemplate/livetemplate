package parse

import (
	"text/template/parse"
)

// handleIf processes {{if}}...{{else}}...{{end}} constructs.
func handleIf(node *parse.IfNode, eval *evaluator, data interface{}, varCtx *varContext, ctx *Context) (*TreeNode, error) {
	d := dot(data, varCtx)
	val, err := eval.evalPipe(node.Pipe, d, varCtx)
	if err != nil {
		return nil, &ParseError{
			Phase: "eval", NodeType: "if",
			Expr: formatPipe(node.Pipe),
			Err:  err,
		}
	}

	branch := selectBranch(node, isTrue(val))
	if branch == nil {
		return createEmptyConditionalWrapper(ctx), nil
	}

	branchTree, err := walkAST(branch, eval, data, varCtx, ctx)
	if err != nil {
		return nil, err
	}
	return createWrapper(branchTree, ctx, WrapperConditional), nil
}

// selectBranch chooses which branch of an if/else to execute.
func selectBranch(node *parse.IfNode, condResult bool) *parse.ListNode {
	if condResult {
		return node.List
	}
	return node.ElseList
}

// createWrapper gives a child tree its own dynamic slot, recording which
// construct asked for it.
//
// The kind is what makes the result identifiable later. A wrapper is
// `["", ""]` statics plus one nested child, which is exactly what a plain field
// node holding a tree looks like (see defaultFieldStatics in field.go), so no
// predicate over the finished node can recover what produced it. Every wrapper
// in the tree is born here, so tagging at this one point is what lets
// wrappedItemKey ask instead of guess (issue #497).
func createWrapper(child *TreeNode, ctx *Context, kind WrapperKind) *TreeNode {
	wrapper := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		wrapper.Statics = defaultFieldStatics
	}
	wrapper.Wrapper = kind
	wrapper.SetDynamic(0, child)
	return wrapper
}

// createEmptyConditionalWrapper creates a wrapper for false conditions with no else clause.
func createEmptyConditionalWrapper(ctx *Context) *TreeNode {
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = defaultFieldStatics
	}
	tree.SetDynamic(0, "")
	return tree
}
