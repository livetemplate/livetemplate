package parse

import (
	"text/template/parse"
)

// handleIf processes {{if}}...{{else}}...{{end}} constructs.
func handleIf(node *parse.IfNode, eval *evaluator, data interface{}, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
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

	branchTree, err := walkAST(branch, eval, data, varCtx, keyGen, ctx)
	if err != nil {
		return nil, err
	}
	return createConditionalWrapper(branchTree, ctx), nil
}

// selectBranch chooses which branch of an if/else to execute.
func selectBranch(node *parse.IfNode, condResult bool) *parse.ListNode {
	if condResult {
		return node.List
	}
	return node.ElseList
}

// createConditionalWrapper wraps a branch tree to preserve conditional structure for diffing.
func createConditionalWrapper(branchTree *TreeNode, ctx *Context) *TreeNode {
	wrapper := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		wrapper.Statics = []string{"", ""}
	}
	wrapper.SetDynamic(0, branchTree)
	return wrapper
}

// createEmptyConditionalWrapper creates a wrapper for false conditions with no else clause.
func createEmptyConditionalWrapper(ctx *Context) *TreeNode {
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic(0, "")
	return tree
}
