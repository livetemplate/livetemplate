package parse

import (
	"bytes"
	"fmt"
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
	transformedCond, execData, err := transformConditionWithVars(pipeStr, varCtx)
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
// Delegates to buildExecData for unified variable transformation and dot context merging.
func transformConditionWithVars(pipeStr string, varCtx *varContext) (string, map[string]interface{}, error) {
	return buildExecData(pipeStr, varCtx)
}
