package parse

import (
	"bytes"
	"fmt"
	"strings"
	"text/template/parse"
)

// createSingleDynamicTree creates a tree node with a single dynamic value at position 0.
// This is a common pattern for field/action nodes that produce a single output.
func createSingleDynamicTree(value string, ctx *Context) *TreeNode {
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic("0", value)
	return tree
}

// handleActionNode processes {{.Field}} or {{.Method}} expressions.
func handleActionNode(node *parse.ActionNode, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Execute the action to get its value
	nodeStr := node.String()
	// Use cached template parsing to avoid repeated Parse() calls
	tmpl, err := getOrParseASTTemplate("action:"+nodeStr, nodeStr, ctx)
	if err != nil {
		return nil, fmt.Errorf("action parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("action execute error: %w", err)
	}

	return createSingleDynamicTree(buf.String(), ctx), nil
}

// handleActionNodeWithVars handles {{.Field}} or {{$var}} with variable context.
func handleActionNodeWithVars(node *parse.ActionNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	nodeStr := node.String()

	// Check if any command contains a variable reference
	hasVars := false
	for _, cmd := range node.Pipe.Cmds {
		for _, arg := range cmd.Args {
			if _, ok := arg.(*parse.VariableNode); ok {
				hasVars = true
				break
			}
		}
		if hasVars {
			break
		}
	}

	if !hasVars {
		// No variables - execute normally with dot context
		// Use cached template parsing to avoid repeated Parse() calls
		tmpl, err := getOrParseASTTemplate("action-novars:"+nodeStr, nodeStr, ctx)
		if err != nil {
			return nil, fmt.Errorf("action parse error: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, varCtx.dot); err != nil {
			return nil, fmt.Errorf("action execute error: %w", err)
		}

		return createSingleDynamicTree(buf.String(), ctx), nil
	}

	// Has variables - evaluate with variable context
	result, err := evaluateActionWithVars(nodeStr, varCtx, ctx)
	if err != nil {
		return nil, fmt.Errorf("action with vars error: %w", err)
	}
	return createSingleDynamicTree(result, ctx), nil
}

// evaluateActionWithVars evaluates an action string that contains variable references.
func evaluateActionWithVars(actionStr string, varCtx *varContext, ctx *Context) (string, error) {
	transformedAction, execData, err := buildExecData(actionStr, varCtx)
	if err != nil {
		return "", fmt.Errorf("action var transform error: %w", err)
	}

	// Defensive guard: should not happen since the caller detected a VariableNode,
	// but if buildExecData couldn't match any variables and dot is nil, execData
	// will be empty — return empty string rather than executing with no data.
	if len(execData) == 0 {
		return "", nil
	}

	tmpl, err := getOrParseASTTemplate("varaction:"+transformedAction, transformedAction, ctx)
	if err != nil {
		return "", fmt.Errorf("action parse error (transformed: %s): %w", transformedAction, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, execData); err != nil {
		return "", fmt.Errorf("action execute error (transformed: %s): %w", transformedAction, err)
	}

	return buf.String(), nil
}

// detectsRootVariable checks if an action string uses the $ root variable.
func detectsRootVariable(actionStr string, vars orderedVars) bool {
	// Check for $. pattern (e.g., $.Field)
	if strings.Contains(actionStr, "$.") {
		return true
	}

	// Check for $ followed by non-letter character
	for i := 0; i < len(actionStr); i++ {
		if actionStr[i] == '$' {
			if i+1 >= len(actionStr) {
				return true
			}
			nextChar := actionStr[i+1]
			if nextChar == '.' {
				return true
			}
			if !isLetter(nextChar) {
				return true
			}
		}
	}

	return false
}

// isLetter checks if a byte is a letter (a-z, A-Z).
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
