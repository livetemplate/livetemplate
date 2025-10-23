package livetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"text/template/parse"
)

// orderedVars is a deterministic map-like structure that preserves insertion order
// This ensures that variable iteration is deterministic, which is critical for
// reproducible tree generation across multiple parses
type orderedVars []struct {
	key   string
	value interface{}
}

// newOrderedVars creates an empty orderedVars
func newOrderedVars() orderedVars {
	return make(orderedVars, 0, 2) // Most ranges have 1-2 variables
}

// Set adds or updates a key-value pair
func (ov *orderedVars) Set(key string, value interface{}) {
	// Check if key exists - update it
	for i := range *ov {
		if (*ov)[i].key == key {
			(*ov)[i].value = value
			return
		}
	}
	// Key doesn't exist - append it
	*ov = append(*ov, struct {
		key   string
		value interface{}
	}{key, value})
}

// Get retrieves a value by key
func (ov orderedVars) Get(key string) (interface{}, bool) {
	for _, pair := range ov {
		if pair.key == key {
			return pair.value, true
		}
	}
	return nil, false
}

// Len returns the number of key-value pairs
func (ov orderedVars) Len() int {
	return len(ov)
}

// Range iterates over all key-value pairs in insertion order
func (ov orderedVars) Range(fn func(key string, value interface{})) {
	for _, pair := range ov {
		fn(pair.key, pair.value)
	}
}

// getOrderedDynamicKeys returns numeric keys from a treeNode in sorted order
// This ensures deterministic iteration over tree dynamics
func getOrderedDynamicKeys(tree treeNode) []string {
	var keys []string
	for k := range tree {
		if k != "s" && k != "f" && k != "d" {
			keys = append(keys, k)
		}
	}

	// Simple bubble sort for numeric string keys like "0", "1", "2"
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			// Parse as integers for comparison (errors intentionally ignored - defaults to 0 for non-numeric keys)
			var iVal, jVal int
			_, _ = fmt.Sscanf(keys[i], "%d", &iVal)
			_, _ = fmt.Sscanf(keys[j], "%d", &jVal)
			if iVal > jVal {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	return keys
}

// parseTemplateToTreeAST is the AST-based parser that replaces regex approach
// It walks the parse tree from Go's template/parse package directly
func parseTemplateToTreeAST(templateStr string, data interface{}, keyGen *keyGenerator) (tree treeNode, err error) {
	// Recover from panics in template execution (can happen with fuzz-generated templates)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("template execution panic: %v", r)
		}
	}()

	// Normalize template spacing
	templateStr = normalizeTemplateSpacing(templateStr)

	// Parse template to get AST
	tmpl, err := template.New("temp").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	// Check if template uses composition and flatten if needed
	if hasTemplateComposition(tmpl) {
		flattenedStr, err := flattenTemplate(tmpl)
		if err != nil {
			return nil, fmt.Errorf("template flatten error: %w", err)
		}
		// Re-parse flattened template
		tmpl, err = template.New("temp-flattened").Parse(flattenedStr)
		if err != nil {
			return nil, fmt.Errorf("flattened template parse error: %w", err)
		}
	}

	// Verify we have a parse tree
	if tmpl.Tree == nil || tmpl.Tree.Root == nil {
		return nil, fmt.Errorf("template has no parse tree")
	}

	// Build tree by walking AST
	tree, err = buildTreeFromAST(tmpl.Tree.Root, data, keyGen)
	if err != nil {
		return nil, fmt.Errorf("AST walk error: %w", err)
	}

	return tree, nil
}

// buildTreeFromAST recursively walks the AST and constructs the tree structure
// This is the core function that replaces regex-based expression extraction
func buildTreeFromAST(node parse.Node, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	if node == nil {
		return treeNode{"s": []string{""}}, nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		return buildTreeFromList(n, data, keyGen)

	case *parse.TextNode:
		// Pure static text
		return treeNode{"s": []string{string(n.Text)}}, nil

	case *parse.ActionNode:
		return handleActionNode(n, data, keyGen)

	case *parse.IfNode:
		return handleIfNode(n, data, keyGen)

	case *parse.RangeNode:
		return handleRangeNode(n, data, keyGen)

	case *parse.WithNode:
		return handleWithNode(n, data, keyGen)

	case *parse.TemplateNode:
		// Should have been flattened already
		return nil, fmt.Errorf("template invocation found - should be flattened: %s", n.Name)

	default:
		return nil, fmt.Errorf("unhandled node type: %T", n)
	}
}

// buildTreeFromList processes a list of nodes and merges their trees
func buildTreeFromList(node *parse.ListNode, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	if node == nil || len(node.Nodes) == 0 {
		return treeNode{"s": []string{""}}, nil
	}

	// Walk AST and merge trees from all nodes
	// Ranges will return comprehension format with "d" key
	var statics []string
	tree := make(treeNode)
	dynamicIndex := 0

	// Start with empty static
	statics = append(statics, "")

	for _, child := range node.Nodes {
		childTree, err := buildTreeFromAST(child, data, keyGen)
		if err != nil {
			return nil, err
		}

		// Check if child is a range comprehension (has "d" key)
		if _, hasD := childTree["d"]; hasD {
			// This is a range - if it's the only node, return it as-is
			// Otherwise, embed it as a nested comprehension
			if len(node.Nodes) == 1 {
				return childTree, nil
			}

			// Range is part of a larger template - embed the entire range tree
			// as a nested structure. Do NOT merge its statics - they belong inside
			// the range comprehension, not in the outer template.
			tree[fmt.Sprintf("%d", dynamicIndex)] = childTree
			dynamicIndex++
			statics = append(statics, "")
			continue
		}

		// Merge child tree into current tree
		childStatics, ok := childTree["s"].([]string)
		if !ok || len(childStatics) == 0 {
			continue
		}

		// First static of child appends to last static of parent
		if len(statics) > 0 && len(childStatics) > 0 {
			statics[len(statics)-1] += childStatics[0]
		}

		// Add remaining statics from child
		if len(childStatics) > 1 {
			statics = append(statics, childStatics[1:]...)
		}

		// Copy dynamic values from child, renumbering them (deterministic order)
		for _, k := range getOrderedDynamicKeys(childTree) {
			tree[fmt.Sprintf("%d", dynamicIndex)] = childTree[k]
			dynamicIndex++
		}
	}

	// Ensure we have enough statics for dynamics
	for len(statics) <= dynamicIndex {
		statics = append(statics, "")
	}

	tree["s"] = statics
	return tree, nil
}

// handleActionNode processes {{.Field}} or {{.Method}} expressions
func handleActionNode(node *parse.ActionNode, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	// Execute the action to get its value
	nodeStr := node.String()
	tmpl, err := template.New("action").Parse(nodeStr)
	if err != nil {
		return nil, fmt.Errorf("action parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("action execute error: %w", err)
	}

	// Create tree with one dynamic value
	return treeNode{
		"s": []string{"", ""},
		"0": buf.String(),
	}, nil
}

// handleIfNode processes {{if}}...{{else}}...{{end}} constructs
func handleIfNode(node *parse.IfNode, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	// Evaluate condition by executing just the if part
	condTmpl := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", formatPipe(node.Pipe))
	tmpl, err := template.New("cond").Parse(condTmpl)
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
		// This allows the conditional to be tracked in diffs
		return treeNode{
			"s": []string{"", ""},
			"0": "",
		}, nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromAST(branch, data, keyGen)
	if err != nil {
		return nil, err
	}

	// Wrap the branch tree to preserve conditional structure
	// The wrapper allows the diff logic to track when the conditional switches branches
	return treeNode{
		"s": []string{"", ""},
		"0": branchTree,
	}, nil
}

// handleRangeNode processes {{range}}...{{end}} constructs
func handleRangeNode(node *parse.RangeNode, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	// For range with variable declarations like {{range $i, $v := .Items}}
	// We need to extract just the collection expression (.Items)
	// The pipe.Decl contains the variable declarations ($i, $v)
	// The pipe.Cmds contains the actual expression to evaluate

	var collection interface{}
	var err error

	if len(node.Pipe.Decl) > 0 {
		// Has variable declarations - extract just the collection expression
		// The collection is in the last command's arguments
		if len(node.Pipe.Cmds) > 0 {
			lastCmd := node.Pipe.Cmds[len(node.Pipe.Cmds)-1]
			if len(lastCmd.Args) > 0 {
				// Get the field/expression being ranged over
				collectionExpr := lastCmd.Args[0].String()
				collection, err = evaluatePipe(collectionExpr, data)
				if err != nil {
					return nil, fmt.Errorf("range evaluation error: %w", err)
				}
			} else {
				return nil, fmt.Errorf("range with declarations has no collection expression")
			}
		} else {
			return nil, fmt.Errorf("range with declarations has no commands")
		}
	} else {
		// No variable declarations - simple {{range .Items}}
		pipeStr := formatPipe(node.Pipe)
		collection, err = evaluatePipe(pipeStr, data)
		if err != nil {
			return nil, fmt.Errorf("range evaluation error: %w", err)
		}
	}

	// Handle nil or empty collection
	collectionValue := reflect.ValueOf(collection)

	if !collectionValue.IsValid() ||
		(collectionValue.Kind() == reflect.Slice && collectionValue.Len() == 0) ||
		(collectionValue.Kind() == reflect.Array && collectionValue.Len() == 0) ||
		(collectionValue.Kind() == reflect.Map && collectionValue.Len() == 0) {
		// Empty range - use else branch if available
		if node.ElseList != nil {
			return buildTreeFromAST(node.ElseList, data, keyGen)
		}
		// Return empty comprehension with at least one empty static
		return treeNode{
			"s": []string{""},
			"d": []interface{}{},
		}, nil
	}

	// Ensure it's a slice, array, or map
	kind := collectionValue.Kind()
	if kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
		return nil, fmt.Errorf("range over non-iterable type: %v", kind)
	}

	// Build trees for each item in the collection
	var itemTrees []interface{}
	var itemStatics []string

	// Check if there are variable declarations
	hasVarDecls := len(node.Pipe.Decl) > 0

	// Iterate based on collection type
	if kind == reflect.Map {
		// For maps, iterate over keys
		iter := 0
		for _, key := range collectionValue.MapKeys() {
			item := collectionValue.MapIndex(key).Interface()

			var itemTree treeNode
			var err error

			if hasVarDecls {
				// For ranges with variable declarations, pass key as index
				itemTree, err = executeRangeBodyWithVarsMap(node, key.Interface(), item, data, keyGen)
				if err != nil {
					return nil, fmt.Errorf("range item error: %w", err)
				}
			} else {
				// Simple range without variables - execute with item as context
				// BUT we still need to preserve the root context for $ variable access
				varCtx := &varContext{
					parent: data,             // Root context for $ access
					vars:   newOrderedVars(), // No variables
					dot:    item,             // Current item for . access
				}
				itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen)
				if err != nil {
					return nil, fmt.Errorf("range item error: %w", err)
				}
			}

			// Extract statics from first item (they're the same for all)
			if iter == 0 {
				if statics, ok := itemTree["s"].([]string); ok {
					itemStatics = statics
				}
			}

			// Store the item tree's dynamics only
			itemDynamics := make(map[string]interface{})
			for k, v := range itemTree {
				if k != "s" && k != "f" {
					itemDynamics[k] = v
				}
			}

			itemTrees = append(itemTrees, itemDynamics)
			iter++
		}
	} else {
		// For slices/arrays, use index-based iteration
		for i := 0; i < collectionValue.Len(); i++ {
			item := collectionValue.Index(i).Interface()

			var itemTree treeNode
			var err error

			if hasVarDecls {
				// For ranges with variable declarations, we need to execute within template context
				// Build a mini-template that sets up the variables and executes the range body
				// We'll use template execution to handle variables properly
				itemTree, err = executeRangeBodyWithVars(node, i, item, data, keyGen)
				if err != nil {
					return nil, fmt.Errorf("range item %d error: %w", i, err)
				}
			} else {
				// Simple range without variables - execute with item as context
				// BUT we still need to preserve the root context for $ variable access
				varCtx := &varContext{
					parent: data,             // Root context for $ access
					vars:   newOrderedVars(), // No variables
					dot:    item,             // Current item for . access
				}
				itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen)
				if err != nil {
					return nil, fmt.Errorf("range item %d error: %w", i, err)
				}
			}

			// Extract statics from first item (they're the same for all)
			if i == 0 {
				if statics, ok := itemTree["s"].([]string); ok {
					itemStatics = statics
				}
			}

			// Store the item tree's dynamics only
			itemDynamics := make(map[string]interface{})
			for k, v := range itemTree {
				if k != "s" && k != "f" {
					itemDynamics[k] = v
				}
			}

			itemTrees = append(itemTrees, itemDynamics)
		}
	}

	// Return range comprehension format
	return treeNode{
		"s": itemStatics,
		"d": itemTrees,
	}, nil
}

// executeRangeBodyWithVars executes a range body with variable declarations
// This properly handles {{range $i, $v := .Collection}} by executing the body
// within a template context that has the variables defined
func executeRangeBodyWithVars(node *parse.RangeNode, index int, item interface{}, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	// Create a variable context that maps variable names to their values
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    item,
	}

	// Populate variables from declarations
	if len(node.Pipe.Decl) == 1 {
		// {{range $v := ...}} - single variable (value)
		varName := node.Pipe.Decl[0].Ident[0]
		varCtx.vars.Set(varName, item)
	} else if len(node.Pipe.Decl) >= 2 {
		// {{range $i, $v := ...}} - index and value
		indexVar := node.Pipe.Decl[0].Ident[0]
		valueVar := node.Pipe.Decl[1].Ident[0]
		varCtx.vars.Set(indexVar, index)
		varCtx.vars.Set(valueVar, item)
	}

	// Walk the range body AST with the variable context
	return buildTreeFromASTWithVars(node.List, varCtx, keyGen)
}

// executeRangeBodyWithVarsMap executes a range body with variable declarations for maps
// This handles {{range $k, $v := .Map}} by executing the body with key and value
func executeRangeBodyWithVarsMap(node *parse.RangeNode, key interface{}, item interface{}, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	// Create a variable context that maps variable names to their values
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    item,
	}

	// Populate variables from declarations
	if len(node.Pipe.Decl) == 1 {
		// {{range $v := ...}} - single variable (value)
		varName := node.Pipe.Decl[0].Ident[0]
		varCtx.vars.Set(varName, item)
	} else if len(node.Pipe.Decl) >= 2 {
		// {{range $k, $v := ...}} - key and value
		keyVar := node.Pipe.Decl[0].Ident[0]
		valueVar := node.Pipe.Decl[1].Ident[0]
		varCtx.vars.Set(keyVar, key)
		varCtx.vars.Set(valueVar, item)
	}

	// Walk the range body AST with the variable context
	return buildTreeFromASTWithVars(node.List, varCtx, keyGen)
}

// varContext holds variable bindings for template execution
type varContext struct {
	parent interface{} // Original data
	vars   orderedVars // Variable bindings ($index, $todo, etc.) - deterministic order
	dot    interface{} // Current dot context
}

// buildTreeFromASTWithVars is like buildTreeFromAST but handles variable references
func buildTreeFromASTWithVars(node parse.Node, varCtx *varContext, keyGen *keyGenerator) (treeNode, error) {
	if node == nil {
		return treeNode{"s": []string{""}}, nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		return buildTreeFromListWithVars(n, varCtx, keyGen)

	case *parse.TextNode:
		return treeNode{"s": []string{string(n.Text)}}, nil

	case *parse.ActionNode:
		return handleActionNodeWithVars(n, varCtx, keyGen)

	case *parse.IfNode:
		return handleIfNodeWithVars(n, varCtx, keyGen)

	case *parse.RangeNode:
		// Nested range - handle recursively
		return handleRangeNode(n, varCtx.dot, keyGen)

	case *parse.WithNode:
		return handleWithNode(n, varCtx.dot, keyGen)

	default:
		return nil, fmt.Errorf("unhandled node type in varCtx: %T", n)
	}
}

// buildTreeFromListWithVars processes a list of nodes with variable context
func buildTreeFromListWithVars(node *parse.ListNode, varCtx *varContext, keyGen *keyGenerator) (treeNode, error) {
	if node == nil || len(node.Nodes) == 0 {
		return treeNode{"s": []string{""}}, nil
	}

	var statics []string
	tree := make(treeNode)
	dynamicIndex := 0
	statics = append(statics, "")

	for _, child := range node.Nodes {
		childTree, err := buildTreeFromASTWithVars(child, varCtx, keyGen)
		if err != nil {
			return nil, err
		}

		// Merge child tree
		childStatics, ok := childTree["s"].([]string)
		if !ok || len(childStatics) == 0 {
			continue
		}

		if len(statics) > 0 && len(childStatics) > 0 {
			statics[len(statics)-1] += childStatics[0]
		}

		if len(childStatics) > 1 {
			statics = append(statics, childStatics[1:]...)
		}

		// Copy dynamic values from child, renumbering them (deterministic order)
		for _, k := range getOrderedDynamicKeys(childTree) {
			tree[fmt.Sprintf("%d", dynamicIndex)] = childTree[k]
			dynamicIndex++
		}
	}

	for len(statics) <= dynamicIndex {
		statics = append(statics, "")
	}

	tree["s"] = statics
	return tree, nil
}

// handleActionNodeWithVars handles {{.Field}} or {{$var}} with variable context
func handleActionNodeWithVars(node *parse.ActionNode, varCtx *varContext, keyGen *keyGenerator) (treeNode, error) {
	// For actions with variable references, we need to execute them in a context
	// where the variables are defined. We can't just create a mini-template because
	// Go templates don't allow defining variables inline.
	//
	// Solution: Build a wrapper template that defines the variables using range/with,
	// then executes the action.

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
		tmpl, err := template.New("action").Parse(nodeStr)
		if err != nil {
			return nil, fmt.Errorf("action parse error: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, varCtx.dot); err != nil {
			return nil, fmt.Errorf("action execute error: %w", err)
		}

		return treeNode{
			"s": []string{"", ""},
			"0": buf.String(),
		}, nil
	}

	// Has variables - we need to build a template that defines them
	// For {{$index | printf "#%d"}}, we build:
	// {{range $index, $todo := .Items}}{{$index | printf "#%d"}}{{end}}
	// But we only execute it for one item

	// Better approach: Build a mini data structure that wraps the variables
	// and execute the action after transforming variable references to field references
	result := evaluateActionWithVars(nodeStr, varCtx)

	return treeNode{
		"s": []string{"", ""},
		"0": result,
	}, nil
}

// detectsRootVariable checks if an action string uses the $ root variable
// It distinguishes between $. (root access) and $varName (named variable)
func detectsRootVariable(actionStr string, vars orderedVars) bool {
	// Check for $. pattern (e.g., $.Field)
	if strings.Contains(actionStr, "$.") {
		return true
	}

	// Check for $ followed by non-letter character (e.g., $ | printf, $ }}, etc.)
	// This indicates standalone $ usage
	for i := 0; i < len(actionStr); i++ {
		if actionStr[i] == '$' {
			// Check what follows the $
			if i+1 >= len(actionStr) {
				// $ at end of string
				return true
			}
			nextChar := actionStr[i+1]
			// If next char is not a letter, it's standalone $ or $.
			// If next char is '.', it's $.Field
			if nextChar == '.' {
				return true
			}
			if !isLetter(nextChar) {
				// Could be $ | or $ }} or other delimiter
				return true
			}
			// If next char is a letter, it could be $varName (known variable)
			// or $Field (which should be treated as $.Field in standard Go templates)
			// For now, be conservative and only detect explicit $. patterns
		}
	}

	return false
}

// isLetter checks if a byte is a letter (a-z, A-Z)
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// mergeFieldsIntoMap copies all accessible fields from value into the target map
// This is used to merge the current dot context with RootData and variables
// for template execution. It handles structs, maps, and pointers generically.
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
			// Don't overwrite existing keys (like RootData or variables)
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
		// This shouldn't normally happen in our use case
		return nil
	}

	return nil
}

// evaluateActionWithVars evaluates an action string that contains variable references
// It does this by building a wrapper template that defines the variables using a range
func evaluateActionWithVars(actionStr string, varCtx *varContext) string {
	// Build a wrapper template that defines the variables
	// For {{$index | printf "#%d"}}, if $index=0, we build:
	// {{range $i := slice 0}}{{$i | printf "#%d"}}{{end}}
	//
	// Actually, simpler: Build a template with a range that assigns the variables,
	// then executes the action body.

	// Identify which variables are used in the action
	usedVars := newOrderedVars()
	varCtx.vars.Range(func(varName string, varValue interface{}) {
		if strings.Contains(actionStr, "$"+varName) {
			usedVars.Set(varName, varValue)
		}
	})

	// Check if the action uses the root $ variable (e.g., $.Field or $ by itself)
	// We need to detect $. or $ followed by a space/delimiter, but NOT $varName
	usesRootVar := detectsRootVariable(actionStr, varCtx.vars)

	// If we have no variables and don't use root, shouldn't happen but handle gracefully
	if usedVars.Len() == 0 && !usesRootVar {
		// No variables used - shouldn't happen but handle gracefully
		return ""
	}

	// Build the wrapper template
	// We need to create data that allows us to range and assign the right values
	// For $index=0, $todo=item, we can do:
	// {{range $index, $todo := .Data}}{{$index | printf "#%d"}}{{end}}
	// where .Data is a slice [item]

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
		// Replace $. with .RootData.
		// This transforms $.Field to .RootData.Field
		transformedAction = strings.Replace(transformedAction, "$.", ".RootData.", -1)

		// Also handle standalone $ (rare but valid in Go templates)
		// Replace $ followed by space or delimiter with .RootData
		// This is tricky - we need to preserve $varName but replace standalone $
		// For now, just handle the $.Field case which is the common one

		// Add root context to exec data
		execData["RootData"] = varCtx.parent
	}

	// Execute the wrapper template
	tmpl, err := template.New("varAction").Parse(transformedAction)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, execData); err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	return buf.String()
}

// handleIfNodeWithVars handles if/else with variable context
func handleIfNodeWithVars(node *parse.IfNode, varCtx *varContext, keyGen *keyGenerator) (treeNode, error) {
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
		tmpl, err := template.New("cond").Parse(condStr)
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
			return treeNode{"s": []string{"", ""}, "0": ""}, nil
		}

		branchTree, err := buildTreeFromASTWithVars(branch, varCtx, keyGen)
		if err != nil {
			return nil, err
		}

		// Wrap the branch tree to preserve conditional structure
		return treeNode{"s": []string{"", ""}, "0": branchTree}, nil
	}

	// Condition uses variables or root - transform it
	transformedCond := pipeStr

	// Build exec data - we need to provide access to:
	// 1. Current dot context fields (for .Field access)
	// 2. Root context (for $.Field -> .RootData.Field access)
	// 3. Named variables (for $var -> .Var access)
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

	// Merge current dot context into execData so .Field access works
	// This is the key fix: copy all accessible fields from varCtx.dot into execData
	if err := mergeFieldsIntoMap(varCtx.dot, execData); err != nil {
		// If we can't merge, fall back to using dot directly (and hope RootData isn't needed)
		// This shouldn't happen in practice
		return nil, fmt.Errorf("failed to merge dot fields: %w", err)
	}

	// Execute condition with transformed template
	condTmplStr := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", transformedCond)
	tmpl, err := template.New("cond").Parse(condTmplStr)
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
		// Condition false and no else - treat as dynamic segment with empty value
		return treeNode{
			"s": []string{"", ""},
			"0": "",
		}, nil
	}

	// Walk the selected branch
	branchTree, err := buildTreeFromASTWithVars(branch, varCtx, keyGen)
	if err != nil {
		return nil, err
	}

	// Wrap the branch tree to preserve conditional structure
	return treeNode{
		"s": []string{"", ""},
		"0": branchTree,
	}, nil
}

// handleWithNode processes {{with}}...{{end}} constructs
func handleWithNode(node *parse.WithNode, data interface{}, keyGen *keyGenerator) (treeNode, error) {
	// Evaluate the with pipe to get the new context
	pipeStr := formatPipe(node.Pipe)

	newContext, err := evaluatePipe(pipeStr, data)
	if err != nil {
		return nil, fmt.Errorf("with evaluation error: %w", err)
	}

	// Check if context is nil/zero
	contextValue := reflect.ValueOf(newContext)
	if !contextValue.IsValid() || isZeroValue(contextValue) {
		// Use else branch if available
		if node.ElseList != nil {
			return buildTreeFromAST(node.ElseList, data, keyGen)
		}
		// Return empty tree
		return treeNode{"s": []string{""}}, nil
	}

	// Execute body with new context
	return buildTreeFromAST(node.List, newContext, keyGen)
}

// evaluatePipe evaluates a pipe expression against data
func evaluatePipe(pipeStr string, data interface{}) (interface{}, error) {
	// Create a template with the pipe expression
	tmplStr := fmt.Sprintf("{{%s}}", pipeStr)
	tmpl, err := template.New("pipe").Parse(tmplStr)
	if err != nil {
		return nil, err
	}

	// Execute to get the value
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	// For simple field access, we need the actual value, not string
	// Try to get it via reflection
	if pipeStr == "." {
		return data, nil
	}

	// For field access like .Items, .User, etc.
	if len(pipeStr) > 1 && pipeStr[0] == '.' {
		fieldName := pipeStr[1:]
		val, err := getFieldValue(data, fieldName)
		if err == nil {
			return val, nil
		}
	}

	// Fall back to string representation
	return buf.String(), nil
}

// isZeroValue checks if a reflect.Value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	default:
		// For structs and other types, compare with zero value
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}

// renderTreeToHTML renders a tree structure back to HTML by merging statics and dynamics
func renderTreeToHTML(tree treeNode) (string, error) {
	// Check if this is a range comprehension (has "d" key with items)
	if itemsRaw, hasD := tree["d"]; hasD {
		return renderRangeComprehensionToHTML(tree, itemsRaw)
	}

	statics, ok := tree["s"].([]string)
	if !ok || len(statics) == 0 {
		return "", fmt.Errorf("invalid tree: no statics")
	}

	var result strings.Builder

	// Interleave statics and dynamics
	dynamicIndex := 0
	for i, static := range statics {
		result.WriteString(static)

		// After each static (except the last), add the corresponding dynamic
		if i < len(statics)-1 {
			dynKey := fmt.Sprintf("%d", dynamicIndex)
			if dynValue, exists := tree[dynKey]; exists {
				// Handle nested trees (like ranges)
				if nestedTree, ok := dynValue.(treeNode); ok {
					nestedHTML, err := renderTreeToHTML(nestedTree)
					if err != nil {
						return "", err
					}
					result.WriteString(nestedHTML)
				} else if nestedMap, ok := dynValue.(map[string]interface{}); ok {
					// Also handle as treeNode
					nestedHTML, err := renderTreeToHTML(treeNode(nestedMap))
					if err != nil {
						return "", err
					}
					result.WriteString(nestedHTML)
				} else {
					// Simple value - convert to string
					result.WriteString(fmt.Sprintf("%v", dynValue))
				}
			}
			dynamicIndex++
		}
	}

	return result.String(), nil
}

// renderRangeComprehensionToHTML renders a range comprehension (with "d" and "s" keys) to HTML
func renderRangeComprehensionToHTML(tree treeNode, itemsRaw interface{}) (string, error) {
	// Get statics for the range items
	statics, ok := tree["s"].([]string)
	if !ok {
		return "", fmt.Errorf("range comprehension missing statics")
	}

	// Convert items to []interface{}
	var items []interface{}
	switch v := itemsRaw.(type) {
	case []interface{}:
		items = v
	case []map[string]interface{}:
		items = make([]interface{}, len(v))
		for i, item := range v {
			items[i] = item
		}
	default:
		return "", fmt.Errorf("unexpected items type: %T", itemsRaw)
	}

	var result strings.Builder

	// Render each item using the statics as template
	for _, itemRaw := range items {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Interleave statics and item dynamics
		for i, static := range statics {
			result.WriteString(static)

			// After each static (except the last), add the corresponding dynamic
			if i < len(statics)-1 {
				dynKey := fmt.Sprintf("%d", i)
				if dynValue, exists := itemMap[dynKey]; exists {
					// Recursively render nested trees
					if nestedTree, ok := dynValue.(treeNode); ok {
						nestedHTML, err := renderTreeToHTML(nestedTree)
						if err != nil {
							return "", err
						}
						result.WriteString(nestedHTML)
					} else if nestedMap, ok := dynValue.(map[string]interface{}); ok {
						nestedHTML, err := renderTreeToHTML(treeNode(nestedMap))
						if err != nil {
							return "", err
						}
						result.WriteString(nestedHTML)
					} else {
						// Simple value
						result.WriteString(fmt.Sprintf("%v", dynValue))
					}
				}
			}
		}
	}

	return result.String(), nil
}
