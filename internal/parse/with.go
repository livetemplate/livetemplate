package parse

import (
	"reflect"
	"text/template/parse"
)

// handleWith processes {{with}}...{{else}}...{{end}} constructs.
func handleWith(node *parse.WithNode, eval *evaluator, data interface{}, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
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
		if node.ElseList != nil {
			return walkAST(node.ElseList, eval, data, varCtx, keyGen, ctx)
		}
		return createEmptyTree(ctx), nil
	}

	bodyVarCtx := createWithBodyVarCtx(data, varCtx, newContext)
	return walkAST(node.List, eval, data, bodyVarCtx, keyGen, ctx)
}
