package parse

import (
	"fmt"
	"reflect"
	"text/template/parse"
)

// handleWithNode processes {{with}}...{{end}} constructs.
func handleWithNode(node *parse.WithNode, data any, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Evaluate the with pipe to get the new context
	pipeStr := formatPipe(node.Pipe)

	newContext, err := evaluatePipeWithCache(ctx.TemplateName, pipeStr, data, ctx)
	if err != nil {
		return nil, fmt.Errorf("with evaluation error: %w", err)
	}

	// Check if context is nil/zero
	contextValue := reflect.ValueOf(newContext)
	if !contextValue.IsValid() || isZeroValue(contextValue) {
		// Use else branch if available
		if node.ElseList != nil {
			return buildTreeFromAST(node.ElseList, data, keyGen, ctx)
		}
		// Return empty tree
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{""}), nil
		}
		return NewTreeNode(), nil
	}

	// Execute body with new context
	return buildTreeFromAST(node.List, newContext, keyGen, ctx)
}
