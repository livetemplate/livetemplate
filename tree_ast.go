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

// parseTemplateToTreeAST is the AST-based parser that replaces regex approach
// It walks the parse tree from Go's template/parse package directly
func parseTemplateToTreeAST(templateStr string, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (tree *TreeNode, err error) {
	// Recover from panics in template execution (can happen with fuzz-generated templates)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("template execution panic: %v", r)
		}
	}()

	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	// Normalize template spacing
	templateStr = normalizeTemplateSpacing(templateStr)

	// Parse template to get AST
	tmpl, err := newTemplateWithFuncs("temp", ctx).Parse(templateStr)
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
		tmpl, err = newTemplateWithFuncs("temp-flattened", ctx).Parse(flattenedStr)
		if err != nil {
			return nil, fmt.Errorf("flattened template parse error: %w", err)
		}
	}

	// Verify we have a parse tree
	if tmpl.Tree == nil || tmpl.Tree.Root == nil {
		return nil, fmt.Errorf("template has no parse tree")
	}

	// Build tree by walking AST with generation context
	tree, err = buildTreeFromAST(tmpl.Tree.Root, data, keyGen, ctx)
	if err != nil {
		return nil, fmt.Errorf("AST walk error: %w", err)
	}

	return tree, nil
}

// newTemplateWithFuncs creates a template preloaded with the context's function map.
func newTemplateWithFuncs(name string, ctx *TreeGenerationContext) *template.Template {
	tmpl := template.New(name)
	if ctx != nil && ctx.FuncMap != nil && len(ctx.FuncMap) > 0 {
		tmpl = tmpl.Funcs(ctx.FuncMap)
	}
	return tmpl
}

// buildTreeFromAST recursively walks the AST and constructs the tree structure
// This is the core function that replaces regex-based expression extraction
func buildTreeFromAST(node parse.Node, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	if node == nil {
		// Context-aware static inclusion
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{""}), nil
		}
		return NewTreeNode(), nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		return buildTreeFromList(n, data, keyGen, ctx)

	case *parse.TextNode:
		// Pure static text
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{string(n.Text)}), nil
		}
		return NewTreeNode(), nil

	case *parse.ActionNode:
		return handleActionNode(n, data, keyGen, ctx)

	case *parse.IfNode:
		return handleIfNode(n, data, keyGen, ctx)

	case *parse.CommentNode:
		// Comments render nothing; treat as empty segment
		return NewTreeNode(), nil

	case *parse.RangeNode:
		return handleRangeNode(n, data, keyGen, ctx)

	case *parse.WithNode:
		return handleWithNode(n, data, keyGen, ctx)

	case *parse.TemplateNode:
		// Should have been flattened already
		return nil, fmt.Errorf("template invocation found - should be flattened: %s", n.Name)

	default:
		return nil, fmt.Errorf("unhandled node type: %T", n)
	}
}

// buildTreeFromList processes a list of nodes and merges their trees
func buildTreeFromList(node *parse.ListNode, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	if node == nil || len(node.Nodes) == 0 {
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{""}), nil
		}
		return NewTreeNode(), nil
	}

	// Walk AST and merge trees from all nodes
	// Ranges will return comprehension format with Range field set
	var statics []string
	tree := NewTreeNode()
	dynamicIndex := 0

	// Start with empty static
	statics = append(statics, "")

	for _, child := range node.Nodes {
		childTree, err := buildTreeFromAST(child, data, keyGen, ctx)
		if err != nil {
			return nil, err
		}

		// Check if child is a range comprehension (has Range field)
		if childTree.HasRange() {
			// This is a range - if it's the only node, return it as-is
			// Otherwise, embed it as a nested comprehension
			if len(node.Nodes) == 1 {
				return childTree, nil
			}

			// Range is part of a larger template - embed the entire range tree
			// as a nested structure. Do NOT merge its statics - they belong inside
			// the range comprehension, not in the outer template.
			tree.SetDynamic(fmt.Sprintf("%d", dynamicIndex), childTree)
			dynamicIndex++
			statics = append(statics, "")
			continue
		}

		// Merge child tree into current tree
		childStatics := childTree.Statics
		if len(childStatics) == 0 {
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
		// Get ordered keys from child dynamics
		var childKeys []string
		for k := range childTree.Dynamics {
			childKeys = append(childKeys, k)
		}
		// Sort them numerically
		for i := 0; i < len(childKeys); i++ {
			for j := i + 1; j < len(childKeys); j++ {
				var iVal, jVal int
				_, _ = fmt.Sscanf(childKeys[i], "%d", &iVal)
				_, _ = fmt.Sscanf(childKeys[j], "%d", &jVal)
				if iVal > jVal {
					childKeys[i], childKeys[j] = childKeys[j], childKeys[i]
				}
			}
		}

		for _, k := range childKeys {
			tree.SetDynamic(fmt.Sprintf("%d", dynamicIndex), childTree.Dynamics[k])
			dynamicIndex++
		}
	}

	// Ensure we have enough statics for dynamics
	for len(statics) <= dynamicIndex {
		statics = append(statics, "")
	}

	// Only include statics if context allows
	if ctx.ShouldIncludeStatics() {
		tree.Statics = statics
	}
	return tree, nil
}

// handleActionNode processes {{.Field}} or {{.Method}} expressions
func handleActionNode(node *parse.ActionNode, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	// Execute the action to get its value
	nodeStr := node.String()
	tmpl, err := newTemplateWithFuncs("action", ctx).Parse(nodeStr)
	if err != nil {
		return nil, fmt.Errorf("action parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("action execute error: %w", err)
	}

	// Create tree with one dynamic value, conditionally include statics
	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic("0", buf.String())
	return tree, nil
}

// handleIfNode processes {{if}}...{{else}}...{{end}} constructs
func handleIfNode(node *parse.IfNode, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

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
		// This allows the conditional to be tracked in diffs
		tree := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			tree.Statics = []string{"", ""}
		}
		tree.SetDynamic("0", "")
		return tree, nil
	}

	// Walk the selected branch with context
	branchTree, err := buildTreeFromAST(branch, data, keyGen, ctx)
	if err != nil {
		return nil, err
	}

	// Wrap the branch tree to preserve conditional structure
	// The wrapper allows the diff logic to track when the conditional switches branches
	wrapper := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		wrapper.Statics = []string{"", ""}
	}
	wrapper.SetDynamic("0", branchTree)
	return wrapper, nil
}

// handleRangeNode processes {{range}}...{{end}} constructs
func handleRangeNode(node *parse.RangeNode, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

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
				collection, err = evaluatePipe(collectionExpr, data, ctx)
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
		collection, err = evaluatePipe(pipeStr, data, ctx)
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
			return buildTreeFromAST(node.ElseList, data, keyGen, ctx)
		}
		// Return empty comprehension with at least one empty static
		emptyRange := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			emptyRange.Statics = []string{""}
		}
		emptyRange.Range = NewRangeData([]interface{}{}, nil)
		return emptyRange, nil
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

			var itemTree *TreeNode
			var err error

			if hasVarDecls {
				// For ranges with variable declarations, pass key as index
				itemTree, err = executeRangeBodyWithVarsMap(node, key.Interface(), item, data, keyGen, ctx)
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
				itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
				if err != nil {
					return nil, fmt.Errorf("range item error: %w", err)
				}
			}

			// Extract statics from first item (they're the same for all)
			if iter == 0 {
				itemStatics = itemTree.Statics
			}

			// Store the item tree's dynamics only (not statics or fingerprint)
			// Create a new TreeNode with just the dynamics
			itemNode := &TreeNode{
				Dynamics: make(map[string]interface{}),
			}
			for k, v := range itemTree.Dynamics {
				itemNode.Dynamics[k] = v
			}

			itemTrees = append(itemTrees, itemNode)
			iter++
		}
	} else {
		// For slices/arrays, use index-based iteration
		for i := 0; i < collectionValue.Len(); i++ {
			item := collectionValue.Index(i).Interface()

			var itemTree *TreeNode
			var err error

			if hasVarDecls {
				// For ranges with variable declarations, we need to execute within template context
				// Build a mini-template that sets up the variables and executes the range body
				// We'll use template execution to handle variables properly
				itemTree, err = executeRangeBodyWithVars(node, i, item, data, keyGen, ctx)
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
				itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
				if err != nil {
					return nil, fmt.Errorf("range item %d error: %w", i, err)
				}
			}

			// Extract statics from first item (they're the same for all)
			if i == 0 {
				itemStatics = itemTree.Statics
			}

			// Store the item tree's dynamics only (not statics or fingerprint)
			// Create a new TreeNode with just the dynamics
			itemNode := &TreeNode{
				Dynamics: make(map[string]interface{}),
			}
			for k, v := range itemTree.Dynamics {
				itemNode.Dynamics[k] = v
			}

			itemTrees = append(itemTrees, itemNode)
		}
	}

	// Detect ID key position in statics
	idKey := detectIDKey(itemStatics)

	// Return range comprehension format with ID metadata
	rangeTree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		rangeTree.Statics = itemStatics
	}
	rangeTree.Range = NewRangeData(itemTrees, nil) // statics are in the main Statics field
	rangeTree.Metadata = NewTreeMetadata(idKey)
	return rangeTree, nil
}

// executeRangeBodyWithVars executes a range body with variable declarations
// This properly handles {{range $i, $v := .Collection}} by executing the body
// within a template context that has the variables defined
func executeRangeBodyWithVars(node *parse.RangeNode, index int, item interface{}, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}
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
	return buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
}

// executeRangeBodyWithVarsMap executes a range body with variable declarations for maps
// This handles {{range $k, $v := .Map}} by executing the body with key and value
func executeRangeBodyWithVarsMap(node *parse.RangeNode, key interface{}, item interface{}, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}
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
	return buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
}

// varContext holds variable bindings for template execution
type varContext struct {
	parent interface{} // Original data
	vars   orderedVars // Variable bindings ($index, $todo, etc.) - deterministic order
	dot    interface{} // Current dot context
}

// buildTreeFromASTWithVars is like buildTreeFromAST but handles variable references
func buildTreeFromASTWithVars(node parse.Node, varCtx *varContext, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	if node == nil {
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{""}), nil
		}
		return NewTreeNode(), nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		return buildTreeFromListWithVars(n, varCtx, keyGen, ctx)

	case *parse.TextNode:
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{string(n.Text)}), nil
		}
		return NewTreeNode(), nil

	case *parse.ActionNode:
		return handleActionNodeWithVars(n, varCtx, keyGen, ctx)

	case *parse.IfNode:
		return handleIfNodeWithVars(n, varCtx, keyGen, ctx)

	case *parse.RangeNode:
		// Nested range - handle recursively
		return handleRangeNode(n, varCtx.dot, keyGen, ctx)

	case *parse.WithNode:
		return handleWithNode(n, varCtx.dot, keyGen, ctx)

	default:
		return nil, fmt.Errorf("unhandled node type in varCtx: %T", n)
	}
}

// buildTreeFromListWithVars processes a list of nodes with variable context
func buildTreeFromListWithVars(node *parse.ListNode, varCtx *varContext, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	if node == nil || len(node.Nodes) == 0 {
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{""}), nil
		}
		return NewTreeNode(), nil
	}

	var statics []string
	tree := NewTreeNode()
	dynamicIndex := 0
	statics = append(statics, "")

	for _, child := range node.Nodes {
		childTree, err := buildTreeFromASTWithVars(child, varCtx, keyGen, ctx)
		if err != nil {
			return nil, err
		}

		// Merge child tree
		childStatics := childTree.Statics
		if len(childStatics) == 0 {
			continue
		}

		if len(statics) > 0 && len(childStatics) > 0 {
			statics[len(statics)-1] += childStatics[0]
		}

		if len(childStatics) > 1 {
			statics = append(statics, childStatics[1:]...)
		}

		// Copy dynamic values from child, renumbering them (deterministic order)
		// Get ordered keys from child dynamics
		var childKeys []string
		for k := range childTree.Dynamics {
			childKeys = append(childKeys, k)
		}
		// Sort them numerically
		for i := 0; i < len(childKeys); i++ {
			for j := i + 1; j < len(childKeys); j++ {
				var iVal, jVal int
				_, _ = fmt.Sscanf(childKeys[i], "%d", &iVal)
				_, _ = fmt.Sscanf(childKeys[j], "%d", &jVal)
				if iVal > jVal {
					childKeys[i], childKeys[j] = childKeys[j], childKeys[i]
				}
			}
		}

		for _, k := range childKeys {
			tree.SetDynamic(fmt.Sprintf("%d", dynamicIndex), childTree.Dynamics[k])
			dynamicIndex++
		}
	}

	for len(statics) <= dynamicIndex {
		statics = append(statics, "")
	}

	if ctx.ShouldIncludeStatics() {
		tree.Statics = statics
	}
	return tree, nil
}

// handleActionNodeWithVars handles {{.Field}} or {{$var}} with variable context
func handleActionNodeWithVars(node *parse.ActionNode, varCtx *varContext, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}
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
		tmpl, err := newTemplateWithFuncs("action", ctx).Parse(nodeStr)
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

	// Has variables - we need to build a template that defines them
	// For {{$index | printf "#%d"}}, we build:
	// {{range $index, $todo := .Items}}{{$index | printf "#%d"}}{{end}}
	// But we only execute it for one item

	// Better approach: Build a mini data structure that wraps the variables
	// and execute the action after transforming variable references to field references
	result := evaluateActionWithVars(nodeStr, varCtx, ctx)

	tree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		tree.Statics = []string{"", ""}
	}
	tree.SetDynamic("0", result)
	return tree, nil
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
func evaluateActionWithVars(actionStr string, varCtx *varContext, ctx *TreeGenerationContext) string {
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
	tmpl, err := newTemplateWithFuncs("varAction", ctx).Parse(transformedAction)
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
func handleIfNodeWithVars(node *parse.IfNode, varCtx *varContext, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

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

		// Wrap the branch tree to preserve conditional structure
		wrapper := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			wrapper.Statics = []string{"", ""}
		}
		wrapper.SetDynamic("0", branchTree)
		return wrapper, nil
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
		// Condition false and no else - treat as dynamic segment with empty value
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

// handleWithNode processes {{with}}...{{end}} constructs
func handleWithNode(node *parse.WithNode, data interface{}, keyGen *keyGenerator, ctx *TreeGenerationContext) (*TreeNode, error) {
	// Default context if not provided (backward compatibility)
	if ctx == nil {
		ctx = NewTreeGenerationContext()
	}

	// Evaluate the with pipe to get the new context
	pipeStr := formatPipe(node.Pipe)

	newContext, err := evaluatePipe(pipeStr, data, ctx)
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

// evaluatePipe evaluates a pipe expression against data
func evaluatePipe(pipeStr string, data interface{}, ctx *TreeGenerationContext) (interface{}, error) {
	if pipeStr == "." {
		return data, nil
	}

	const captureName = "__lvt_capture_result__"
	var (
		captured   interface{}
		didCapture bool
	)

	// Build function map with capture helper and user-provided functions.
	funcs := make(template.FuncMap)
	if ctx != nil && ctx.FuncMap != nil {
		for name, fn := range ctx.FuncMap {
			funcs[name] = fn
		}
	}
	funcs[captureName] = func(v interface{}) string {
		captured = v
		didCapture = true
		return ""
	}

	// Execute the pipeline through a capture helper to retain the concrete value.
	tmpl, err := template.New("pipe").Funcs(funcs).Parse(fmt.Sprintf("{{%s (%s)}}", captureName, pipeStr))
	if err != nil {
		return nil, err
	}

	if err := tmpl.Execute(&bytes.Buffer{}, data); err != nil {
		return nil, err
	}

	if didCapture {
		return captured, nil
	}

	// Fallback to string representation if the capture did not run.
	fallbackTmpl, err := template.New("pipe-fallback").Funcs(funcs).Parse(fmt.Sprintf("{{%s}}", pipeStr))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := fallbackTmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
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
func renderTreeToHTML(tree map[string]interface{}) (string, error) {
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
				if nestedTree, ok := dynValue.(map[string]interface{}); ok {
					nestedHTML, err := renderTreeToHTML(nestedTree)
					if err != nil {
						return "", err
					}
					result.WriteString(nestedHTML)
				} else if nestedMap, ok := dynValue.(map[string]interface{}); ok {
					// Also handle as map[string]interface{}
					nestedHTML, err := renderTreeToHTML(map[string]interface{}(nestedMap))
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
func renderRangeComprehensionToHTML(tree map[string]interface{}, itemsRaw interface{}) (string, error) {
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
					if nestedTree, ok := dynValue.(map[string]interface{}); ok {
						nestedHTML, err := renderTreeToHTML(nestedTree)
						if err != nil {
							return "", err
						}
						result.WriteString(nestedHTML)
					} else if nestedMap, ok := dynValue.(map[string]interface{}); ok {
						nestedHTML, err := renderTreeToHTML(map[string]interface{}(nestedMap))
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
