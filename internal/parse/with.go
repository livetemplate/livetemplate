package parse

import (
	"reflect"
	"text/template/parse"
)

// handleWith processes {{with}}...{{else}}...{{end}} constructs. Like handleIf, every
// outcome goes through walkBranch, so it is one wrapped slot whatever the value: a
// body returned unwrapped (and no slot when empty) shifted range items' values.
func handleWith(node *parse.WithNode, eval *evaluator, data interface{}, varCtx *varContext, ctx *Context) (*TreeNode, error) {
	d := dot(data, varCtx)
	newContext, err := eval.evalPipe(node.Pipe, d, varCtx)
	if err != nil {
		return nil, &ParseError{
			Phase: "eval", NodeType: "with",
			Expr: formatPipe(node.Pipe),
			Err:  err,
		}
	}

	contextValue := reflect.ValueOf(newContext)
	if !contextValue.IsValid() || isZeroValue(contextValue) {
		return walkBranch(node.ElseList, eval, data, varCtx, ctx)
	}
	return walkBranch(node.List, eval, data, createWithBodyVarCtx(data, varCtx, newContext), ctx)
}
