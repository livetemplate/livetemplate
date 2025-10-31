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
	condTmpl := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", formatPipe(node.Pipe))
	tmpl, err := newTemplateWithFuncs("cond", ctx).Parse(condTmpl)
	if err != nil {
		return nil, fmt.Errorf("condition parse error: %w", err)
	}

	var condBuf bytes.Buffer
	if err := tmpl.Execute(&condBuf, data); err != nil {
		return nil, fmt.Errorf("condition execute error: %w", err)
	}

	// Choose branch based on condition
	var branch *parse.ListNode
	if condBuf.String() == "true" {
		branch = node.List
	} else if node.ElseList != nil {
		branch = node.ElseList
	} else {
		// Condition false and no else - treat as dynamic segment with empty value
		tree := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			tree.Statics = []string{"", ""}
		}
		tree.SetDynamic("0", "")
		return tree, nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromAST(branch, data, keyGen, ctx)
	if err != nil {
		return nil, err
	}

	// Wrap the branch tree to preserve conditional structure
	wrapper := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		wrapper.Statics = []string{"", ""}
	}
	wrapper.SetDynamic("0", branchTree)
	return wrapper, nil
}

// handleIfNodeWithVars handles if/else with variable context.
func handleIfNodeWithVars(node *parse.IfNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Evaluate condition - needs to handle both variables and root context
	pipeStr := formatPipe(node.Pipe)
	condStr := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", pipeStr)

	// Check if condition uses variables or root
	usesVars := false
	varCtx.vars.Range(func(varName string, _ interface{}) {
		if strings.Contains(pipeStr, "$"+varName) {
			usesVars = true
		}
	})
	usesRoot := detectsRootVariable(pipeStr, varCtx.vars)

	// If no variables or root, execute with dot context
	if !usesVars && !usesRoot {
		tmpl, err := newTemplateWithFuncs("cond", ctx).Parse(condStr)
		if err != nil {
			return nil, fmt.Errorf("condition parse error: %w", err)
		}

		var condBuf bytes.Buffer
		if err := tmpl.Execute(&condBuf, varCtx.dot); err != nil {
			return nil, fmt.Errorf("condition execute error: %w", err)
		}

		var branch *parse.ListNode
		if condBuf.String() == "true" {
			branch = node.List
		} else if node.ElseList != nil {
			branch = node.ElseList
		} else {
			tree := NewTreeNode()
			if ctx.ShouldIncludeStatics() {
				tree.Statics = []string{"", ""}
			}
			tree.SetDynamic("0", "")
			return tree, nil
		}

		branchTree, err := buildTreeFromASTWithVars(branch, varCtx, keyGen, ctx)
		if err != nil {
			return nil, err
		}

		wrapper := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			wrapper.Statics = []string{"", ""}
		}
		wrapper.SetDynamic("0", branchTree)
		return wrapper, nil
	}

	// Condition uses variables or root - transform it
	transformedCond := pipeStr

	// Build exec data
	execData := make(map[string]interface{})

	// Handle named variables
	varCtx.vars.Range(func(varName string, varValue interface{}) {
		if strings.Contains(pipeStr, "$"+varName) {
			fieldName := strings.ToUpper(varName[:1]) + varName[1:]
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
		return nil, fmt.Errorf("failed to merge dot fields: %w", err)
	}

	// Execute condition with transformed template
	condTmplStr := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", transformedCond)
	tmpl, err := newTemplateWithFuncs("cond", ctx).Parse(condTmplStr)
	if err != nil {
		return nil, fmt.Errorf("condition parse error: %w", err)
	}

	var condBuf bytes.Buffer
	if err := tmpl.Execute(&condBuf, execData); err != nil {
		return nil, fmt.Errorf("condition execute error: %w", err)
	}

	var branch *parse.ListNode
	if condBuf.String() == "true" {
		branch = node.List
	} else if node.ElseList != nil {
		branch = node.ElseList
	} else {
		tree := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			tree.Statics = []string{"", ""}
		}
		tree.SetDynamic("0", "")
		return tree, nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromASTWithVars(branch, varCtx, keyGen, ctx)
	if err != nil {
		return nil, err
	}

	// Wrap the branch tree to preserve conditional structure
	wrapper := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		wrapper.Statics = []string{"", ""}
	}
	wrapper.SetDynamic("0", branchTree)
	return wrapper, nil
}

// mergeFieldsIntoMap copies all accessible fields from value into the target map.
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
			// Don't overwrite existing keys
			if _, exists := target[keyStr]; !exists {
				target[keyStr] = v.MapIndex(key).Interface()
			}
		}

	case reflect.Struct:
		// Copy all exported struct fields
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			// Only copy exported fields
			if field.PkgPath == "" {
				fieldValue := v.Field(i)
				// Don't overwrite existing keys
				if _, exists := target[field.Name]; !exists {
					target[field.Name] = fieldValue.Interface()
				}
			}
		}

	default:
		// For primitive types, just set the value directly
		return nil
	}

	return nil
}
