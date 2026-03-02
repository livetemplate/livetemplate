package parse

import (
	"fmt"
	"reflect"
	"strings"
	"text/template/parse"
	"unicode"
	"unicode/utf8"
)

// handleWithNode processes {{with}}...{{end}} constructs.
func handleWithNode(node *parse.WithNode, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
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

// handleWithNodeWithVars processes {{with}}...{{end}} constructs while propagating
// inherited variables from the parent scope. This enables variables like $c defined
// outside the with block to be accessible inside it.
func handleWithNodeWithVars(node *parse.WithNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	pipeStr := formatPipe(node.Pipe)

	// Evaluate pipe, handling variable references like $c.Inner
	newContext, err := evaluatePipeWithVarCtx(pipeStr, varCtx, ctx)
	if err != nil {
		return nil, fmt.Errorf("with evaluation error: %w", err)
	}

	contextValue := reflect.ValueOf(newContext)
	if !contextValue.IsValid() || isZeroValue(contextValue) {
		if node.ElseList != nil {
			return buildTreeFromASTWithVars(node.ElseList, varCtx, keyGen, ctx)
		}
		return createEmptyTree(ctx), nil
	}

	// Create new varCtx with changed dot but copied variables to prevent
	// inner scope mutations from leaking to outer scope.
	copiedVars := newOrderedVars()
	varCtx.vars.Range(func(key string, value interface{}) {
		copiedVars.Set(key, value)
	})
	newVarCtx := &varContext{
		parent: varCtx.parent,
		vars:   copiedVars,
		dot:    newContext,
	}
	return buildTreeFromASTWithVars(node.List, newVarCtx, keyGen, ctx)
}

// evaluatePipeWithVarCtx evaluates a pipe expression, transforming variable references
// ($c.Field) into field accesses (.C.Field) so they work in standalone template evaluation.
func evaluatePipeWithVarCtx(pipeStr string, varCtx *varContext, ctx *Context) (interface{}, error) {
	// Check if pipe references any variables (sorted by descending length to prevent partial matches)
	sortedNames := sortedVarNames(&varCtx.vars)
	usesVar := false
	for _, varName := range sortedNames {
		if strings.Contains(pipeStr, "$"+varName) {
			usesVar = true
			break
		}
	}

	if !usesVar {
		return evaluatePipeWithCache(ctx.TemplateName, pipeStr, varCtx.dot, ctx)
	}

	// Transform variable references: $c.Field → .C.Field
	// Process longer names first to prevent partial matches (e.g., $col before $c)
	transformedExpr := pipeStr
	execData := make(map[string]interface{})
	for _, varName := range sortedNames {
		if strings.Contains(pipeStr, "$"+varName) {
			r, size := utf8.DecodeRuneInString(varName)
			fieldName := string(unicode.ToUpper(r)) + varName[size:]
			transformedExpr = strings.ReplaceAll(transformedExpr, "$"+varName, "."+fieldName)
			varValue, _ := varCtx.vars.Get(varName)
			execData[fieldName] = varValue
		}
	}
	return evaluatePipeWithCache(ctx.TemplateName, transformedExpr, execData, ctx)
}
