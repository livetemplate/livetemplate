package parse

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template/parse"
)

// handleIfNode processes {{if}}...{{else}}...{{end}} constructs.
func handleIfNode(node *parse.IfNode, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Evaluate condition by executing just the if part
	pipeStr := formatPipe(node.Pipe)
	condTmpl := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", pipeStr)

	// Use cached template parsing to avoid repeated Parse() calls
	tmpl, err := getOrParseASTTemplate("cond:"+condTmpl, condTmpl, ctx)
	if err != nil {
		return nil, fmt.Errorf("condition parse error (pipe: %s): %w", pipeStr, err)
	}

	var condBuf bytes.Buffer
	if err := tmpl.Execute(&condBuf, data); err != nil {
		return nil, fmt.Errorf("condition execute error (pipe: %s): %w", pipeStr, err)
	}

	// Select branch based on condition result
	isTrue := condBuf.String() == "true"
	branch := selectBranch(node, isTrue)

	// Handle empty branch (false condition with no else clause)
	if branch == nil {
		return createEmptyConditionalWrapper(ctx), nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromAST(branch, data, keyGen, ctx)
	if err != nil {
		return nil, err
	}

	// Wrap branch tree to preserve conditional structure for diffing
	return createConditionalWrapper(branchTree, ctx), nil
}

// handleIfNodeWithVars handles if/else with variable context.
func handleIfNodeWithVars(node *parse.IfNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	pipeStr := formatPipe(node.Pipe)

	// Check if condition uses variables or root
	usesVars := false
	varCtx.vars.Range(func(varName string, _ interface{}) {
		if strings.Contains(pipeStr, "$"+varName) {
			usesVars = true
		}
	})
	usesRoot := detectsRootVariable(pipeStr, varCtx.vars)

	// If no variables or root, execute with dot context (simpler path)
	if !usesVars && !usesRoot {
		return handleIfWithDotContext(node, varCtx, keyGen, ctx)
	}

	// Condition uses variables or root - transform it
	transformedCond, execData, err := transformConditionWithVars(pipeStr, varCtx, usesRoot)
	if err != nil {
		return nil, err
	}

	// Execute condition with transformed template
	condTmplStr := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", transformedCond)

	// Use cached template parsing to avoid repeated Parse() calls
	tmpl, err := getOrParseASTTemplate("cond-vars:"+condTmplStr, condTmplStr, ctx)
	if err != nil {
		return nil, fmt.Errorf("condition parse error (pipe: %s, transformed: %s): %w", pipeStr, transformedCond, err)
	}

	var condBuf bytes.Buffer
	if err := tmpl.Execute(&condBuf, execData); err != nil {
		return nil, fmt.Errorf("condition execute error (pipe: %s, transformed: %s): %w", pipeStr, transformedCond, err)
	}

	// Select branch based on condition result
	isTrue := condBuf.String() == "true"
	branch := selectBranch(node, isTrue)

	// Handle empty branch (false condition with no else clause)
	if branch == nil {
		return createEmptyConditionalWrapper(ctx), nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromASTWithVars(branch, varCtx, keyGen, ctx)
	if err != nil {
		return nil, err
	}

	// Wrap branch tree to preserve conditional structure for diffing
	return createConditionalWrapper(branchTree, ctx), nil
}

// handleIfWithDotContext handles if/else when no variables are used, just dot context.
func handleIfWithDotContext(node *parse.IfNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	pipeStr := formatPipe(node.Pipe)
	condStr := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", pipeStr)

	// Use cached template parsing to avoid repeated Parse() calls
	tmpl, err := getOrParseASTTemplate("cond-novars:"+condStr, condStr, ctx)
	if err != nil {
		return nil, fmt.Errorf("condition parse error (pipe: %s): %w", pipeStr, err)
	}

	var condBuf bytes.Buffer
	if err := tmpl.Execute(&condBuf, varCtx.dot); err != nil {
		return nil, fmt.Errorf("condition execute error (pipe: %s): %w", pipeStr, err)
	}

	// Select branch based on condition result
	isTrue := condBuf.String() == "true"
	branch := selectBranch(node, isTrue)

	// Handle empty branch (false condition with no else clause)
	if branch == nil {
		return createEmptyConditionalWrapper(ctx), nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromASTWithVars(branch, varCtx, keyGen, ctx)
	if err != nil {
		return nil, err
	}

	// Wrap branch tree to preserve conditional structure for diffing
	return createConditionalWrapper(branchTree, ctx), nil
}

// selectBranch chooses which branch of an if/else to execute.
// Returns the true branch if isTrue, else branch if false, or nil if false with no else.
func selectBranch(node *parse.IfNode, isTrue bool) *parse.ListNode {
	if isTrue {
		return node.List
	}
	return node.ElseList // may be nil
}

// createConditionalWrapper wraps a branch tree to preserve conditional structure.
// The wrapper uses empty statics ["", ""] with the branch tree as dynamic value "0".
// This ensures consistent tree structure for efficient diffing on updates.
func createConditionalWrapper(branchTree *TreeNode, ctx *Context) *TreeNode {
	wrapper := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		wrapper.Statics = []string{"", ""}
	}
	wrapper.SetDynamic("0", branchTree)
	return wrapper
}

// createEmptyConditionalWrapper creates a wrapper for false conditions with no else clause.
// Returns a wrapper with empty statics and empty string as dynamic value.
func createEmptyConditionalWrapper(ctx *Context) *TreeNode {
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic("0", "")
	return tree
}

// transformConditionWithVars transforms template variables in a condition to field references.
// Converts $var to .Var and $. to .RootData. and builds execution data map.
func transformConditionWithVars(pipeStr string, varCtx *varContext, usesRoot bool) (string, map[string]interface{}, error) {
	transformedCond := pipeStr
	execData := make(map[string]interface{})

	// Handle named variables
	varCtx.vars.Range(func(varName string, varValue interface{}) {
		if strings.Contains(pipeStr, "$"+varName) {
			fieldName := capitalizeFieldName(varName)
			transformedCond = strings.Replace(transformedCond, "$"+varName, "."+fieldName, -1)
			execData[fieldName] = varValue
		}
	})

	// Handle root variable
	if usesRoot {
		transformedCond = strings.Replace(transformedCond, "$.", ".RootData.", -1)
		execData["RootData"] = varCtx.parent
	}

	// Merge current dot context into execData
	if err := mergeFieldsIntoMap(varCtx.dot, execData); err != nil {
		return "", nil, fmt.Errorf("failed to merge dot fields: %w", err)
	}

	return transformedCond, execData, nil
}

// capitalizeFieldName converts a variable name to a capitalized field name.
// Examples: "show" -> "Show", "isActive" -> "IsActive", "x" -> "X"
func capitalizeFieldName(varName string) string {
	if len(varName) == 0 {
		return varName
	}
	if len(varName) == 1 {
		return strings.ToUpper(varName)
	}
	return strings.ToUpper(varName[:1]) + varName[1:]
}

// mergeFieldsIntoMap copies all accessible fields from value into the target map.
// For structs, copies all exported fields. For maps, copies all entries.
// Skips keys that already exist in target to avoid silent overwrites.
func mergeFieldsIntoMap(value interface{}, target map[string]interface{}) error {
	if value == nil {
		return nil
	}

	v := reflect.ValueOf(value)

	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		// Copy all map entries
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			// Don't overwrite existing keys (variables take precedence)
			if _, exists := target[keyStr]; !exists {
				target[keyStr] = v.MapIndex(key).Interface()
			}
		}

	case reflect.Struct:
		// Copy all exported struct fields
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			// Only copy exported fields (PkgPath is empty for exported fields)
			if field.PkgPath == "" {
				fieldValue := v.Field(i)
				// Don't overwrite existing keys (variables take precedence)
				if _, exists := target[field.Name]; !exists {
					target[field.Name] = fieldValue.Interface()
				}
			}
		}

	default:
		// For primitive types, no fields to merge
		return nil
	}

	return nil
}
