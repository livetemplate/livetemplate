package parse

import (
	"fmt"
	"reflect"
	"strings"
	"text/template/parse"
)

// handleRangeNode processes {{range}}...{{end}} constructs.
func handleRangeNode(node *parse.RangeNode, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Extract collection to iterate over
	collection, err := extractRangeCollection(node, data, ctx)
	if err != nil {
		return nil, err
	}

	// Handle nil or empty collection
	collectionValue := reflect.ValueOf(collection)
	if isEmpty(collectionValue) {
		return handleEmptyRange(node, data, keyGen, ctx)
	}

	// Ensure it's a slice, array, or map
	kind := collectionValue.Kind()
	if kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
		return nil, fmt.Errorf("range over non-iterable type: %v", kind)
	}

	// Build trees for each item in the collection
	hasVarDecls := len(node.Pipe.Decl) > 0

	if kind == reflect.Map {
		return handleMapRange(node, collectionValue, data, hasVarDecls, keyGen, ctx)
	}

	return handleSliceRange(node, collectionValue, data, hasVarDecls, keyGen, ctx)
}

// extractRangeCollection extracts the collection expression from a range node.
func extractRangeCollection(node *parse.RangeNode, data interface{}, ctx *Context) (interface{}, error) {
	if len(node.Pipe.Decl) > 0 {
		// Has variable declarations - extract just the collection expression
		if len(node.Pipe.Cmds) > 0 {
			lastCmd := node.Pipe.Cmds[len(node.Pipe.Cmds)-1]
			if len(lastCmd.Args) > 0 {
				collectionExpr := lastCmd.Args[0].String()
				return evaluatePipe(collectionExpr, data, ctx)
			}
			return nil, fmt.Errorf("range with declarations has no collection expression")
		}
		return nil, fmt.Errorf("range with declarations has no commands")
	}

	// No variable declarations - simple {{range .Items}}
	pipeStr := formatPipe(node.Pipe)
	return evaluatePipe(pipeStr, data, ctx)
}

// isEmpty checks if a collection value is nil or empty.
func isEmpty(v reflect.Value) bool {
	return !v.IsValid() ||
		(v.Kind() == reflect.Slice && v.Len() == 0) ||
		(v.Kind() == reflect.Array && v.Len() == 0) ||
		(v.Kind() == reflect.Map && v.Len() == 0)
}

// handleEmptyRange handles empty collections or else branches.
func handleEmptyRange(node *parse.RangeNode, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Empty range - use else branch if available
	if node.ElseList != nil {
		return buildTreeFromAST(node.ElseList, data, keyGen, ctx)
	}

	// Return empty comprehension
	emptyRange := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		emptyRange.Statics = []string{""}
	}
	emptyRange.Range = NewRangeData([]interface{}{}, nil)
	return emptyRange, nil
}

// handleSliceRange processes slice/array range iterations.
func handleSliceRange(node *parse.RangeNode, collection reflect.Value, data interface{}, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	var itemTrees []interface{}
	var itemStatics []string

	for i := 0; i < collection.Len(); i++ {
		item := collection.Index(i).Interface()

		var itemTree *TreeNode
		var err error

		if hasVarDecls {
			itemTree, err = executeRangeBodyWithVars(node, i, item, data, keyGen, ctx)
		} else {
			varCtx := &varContext{
				parent: data,
				vars:   newOrderedVars(),
				dot:    item,
			}
			itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
		}

		if err != nil {
			return nil, fmt.Errorf("range item %d error: %w", i, err)
		}

		// Extract statics from first item
		if i == 0 {
			itemStatics = itemTree.Statics
		}

		// Store the item tree's dynamics only
		itemNode := &TreeNode{
			Dynamics: make(map[string]interface{}),
		}
		for k, v := range itemTree.Dynamics {
			itemNode.Dynamics[k] = v
		}

		itemTrees = append(itemTrees, itemNode)
	}

	return buildRangeTree(itemTrees, itemStatics, ctx)
}

// handleMapRange processes map range iterations.
func handleMapRange(node *parse.RangeNode, collection reflect.Value, data interface{}, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	var itemTrees []interface{}
	var itemStatics []string

	iter := 0
	for _, key := range collection.MapKeys() {
		item := collection.MapIndex(key).Interface()

		var itemTree *TreeNode
		var err error

		if hasVarDecls {
			itemTree, err = executeRangeBodyWithVarsMap(node, key.Interface(), item, data, keyGen, ctx)
		} else {
			varCtx := &varContext{
				parent: data,
				vars:   newOrderedVars(),
				dot:    item,
			}
			itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
		}

		if err != nil {
			return nil, fmt.Errorf("range item error: %w", err)
		}

		// Extract statics from first item
		if iter == 0 {
			itemStatics = itemTree.Statics
		}

		// Store the item tree's dynamics only
		itemNode := &TreeNode{
			Dynamics: make(map[string]interface{}),
		}
		for k, v := range itemTree.Dynamics {
			itemNode.Dynamics[k] = v
		}

		itemTrees = append(itemTrees, itemNode)
		iter++
	}

	return buildRangeTree(itemTrees, itemStatics, ctx)
}

// buildRangeTree constructs the final range tree with metadata.
func buildRangeTree(itemTrees []interface{}, itemStatics []string, ctx *Context) (*TreeNode, error) {
	// Detect ID key position in statics
	idKey := detectIDKey(itemStatics)

	// Return range comprehension format with ID metadata
	rangeTree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		rangeTree.Statics = itemStatics
	}
	rangeTree.Range = NewRangeData(itemTrees, nil)
	rangeTree.Metadata = NewTreeMetadata(idKey)
	return rangeTree, nil
}

// executeRangeBodyWithVars executes a range body with variable declarations.
func executeRangeBodyWithVars(node *parse.RangeNode, index int, item interface{}, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Create a variable context
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

// executeRangeBodyWithVarsMap executes a range body with variable declarations for maps.
func executeRangeBodyWithVarsMap(node *parse.RangeNode, key interface{}, item interface{}, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Create a variable context
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

// detectIDKey detects the ID key position in statics.
// TODO: Implement ID key detection logic.
func detectIDKey(statics []string) string {
	// Look for patterns like data-id=" or id=" in statics
	for i, static := range statics {
		if strings.Contains(static, "data-id=\"") || strings.Contains(static, "id=\"") {
			return fmt.Sprintf("%d", i)
		}
	}
	return ""
}
