package parse

import (
	"bytes"
	"fmt"
	"strings"
	"text/template/parse"
)

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

	// Create tree with one dynamic value
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic("0", buf.String())
	return tree, nil
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

		result := buf.String()

		tree := NewTreeNodeWithStatics([]string{"", ""})
		tree.SetDynamic("0", result)
		return tree, nil
	}

	// Has variables - evaluate with variable context
	result := evaluateActionWithVars(nodeStr, varCtx, ctx)

	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic("0", result)
	return tree, nil
}

// evaluateActionWithVars evaluates an action string that contains variable references.
func evaluateActionWithVars(actionStr string, varCtx *varContext, ctx *Context) string {
	// Identify which variables are used in the action
	usedVars := newOrderedVars()
	varCtx.vars.Range(func(varName string, varValue interface{}) {
		if strings.Contains(actionStr, "$"+varName) {
			usedVars.Set(varName, varValue)
		}
	})

	// Check if the action uses the root $ variable
	usesRootVar := detectsRootVariable(actionStr, varCtx.vars)

	// If we have no variables and don't use root, shouldn't happen but handle gracefully
	if usedVars.Len() == 0 && !usesRootVar {
		return ""
	}

	// Transform the action string to replace variable references with field accesses
	transformedAction := actionStr

	// Build exec data map
	execData := make(map[string]interface{})

	// Handle named variables ($index, $todo, etc.)
	usedVars.Range(func(varName string, varValue interface{}) {
		// Capitalize first letter for field access
		fieldName := strings.ToUpper(varName[:1]) + varName[1:]
		transformedAction = strings.Replace(transformedAction, "$"+varName, "."+fieldName, -1)
		execData[fieldName] = varValue
	})

	// Handle root variable ($. or standalone $)
	if usesRootVar {
		transformedAction = strings.Replace(transformedAction, "$.", ".RootData.", -1)
		execData["RootData"] = varCtx.parent
	}

	// Execute the wrapper template
	// Use cached template parsing to avoid repeated Parse() calls
	tmpl, err := getOrParseASTTemplate("varaction:"+transformedAction, transformedAction, ctx)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, execData); err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	return buf.String()
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
