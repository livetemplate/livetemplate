package parse

import (
	"fmt"
	"reflect"
	"text/template/parse"
)

// handleRange processes {{range}}...{{end}} constructs.
func handleRange(node *parse.RangeNode, eval *evaluator, data interface{}, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	collection, err := extractCollection(node, eval, data, varCtx)
	if err != nil {
		return nil, err
	}

	collectionValue := reflect.ValueOf(collection)
	if isEmpty(collectionValue) {
		return handleEmptyRange(node, eval, data, varCtx, keyGen, ctx)
	}

	kind := collectionValue.Kind()
	if kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
		return nil, &ParseError{
			Phase: "eval", NodeType: "range",
			Msg: fmt.Sprintf("range over non-iterable type: %v", kind),
		}
	}

	hasVarDecls := len(node.Pipe.Decl) > 0

	if kind == reflect.Map {
		return iterateMap(node, collectionValue, eval, data, varCtx, hasVarDecls, keyGen, ctx)
	}
	return iterateSlice(node, collectionValue, eval, data, varCtx, hasVarDecls, keyGen, ctx)
}

// extractCollection extracts the collection expression from a range node.
func extractCollection(node *parse.RangeNode, eval *evaluator, data interface{}, varCtx *varContext) (interface{}, error) {
	d := dot(data, varCtx)
	// Evaluate the pipe to get the collection
	val, err := eval.evalPipe(node.Pipe, d, varCtx)
	if err != nil {
		return nil, &ParseError{
			Phase: "eval", NodeType: "range",
			Expr: formatPipe(node.Pipe),
			Err:  err,
		}
	}
	return val, nil
}

// isEmpty checks if a collection value is nil or empty.
func isEmpty(v reflect.Value) bool {
	return !v.IsValid() ||
		(v.Kind() == reflect.Slice && v.Len() == 0) ||
		(v.Kind() == reflect.Array && v.Len() == 0) ||
		(v.Kind() == reflect.Map && v.Len() == 0)
}

// handleEmptyRange handles empty collections or else branches.
func handleEmptyRange(node *parse.RangeNode, eval *evaluator, data interface{}, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	if node.ElseList != nil {
		return walkAST(node.ElseList, eval, data, varCtx, keyGen, ctx)
	}
	emptyRange := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		emptyRange.Statics = []string{""}
	}
	emptyRange.Range = NewRangeData([]interface{}{}, nil)
	return emptyRange, nil
}

// iterateSlice handles slice/array range iteration.
func iterateSlice(node *parse.RangeNode, collection reflect.Value, eval *evaluator, data interface{}, parentVarCtx *varContext, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	itemsWithStatics := make([]rangeItemWithStatics, 0, collection.Len())
	itemCtx := contextWithStatics(ctx)

	for i := 0; i < collection.Len(); i++ {
		item := collection.Index(i).Interface()
		itemVarCtx := buildRangeItemVarCtx(node, i, item, data, parentVarCtx, hasVarDecls)

		itemTree, err := walkAST(node.List, eval, data, itemVarCtx, keyGen, itemCtx)
		if err != nil {
			return nil, fmt.Errorf("range item %d error: %w", i, err)
		}

		itemsWithStatics = append(itemsWithStatics, rangeItemWithStatics{
			tree:    itemTree,
			statics: itemTree.Statics,
			hash:    hashStatics(itemTree.Statics),
		})
	}
	return buildRangeTreeWithStatics(itemsWithStatics, ctx)
}

// iterateMap handles map range iteration.
func iterateMap(node *parse.RangeNode, collection reflect.Value, eval *evaluator, data interface{}, parentVarCtx *varContext, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	keys := collection.MapKeys()
	itemsWithStatics := make([]rangeItemWithStatics, 0, len(keys))
	itemCtx := contextWithStatics(ctx)

	for i, key := range keys {
		item := collection.MapIndex(key).Interface()
		itemVarCtx := buildRangeItemVarCtx(node, key.Interface(), item, data, parentVarCtx, hasVarDecls)

		itemTree, err := walkAST(node.List, eval, data, itemVarCtx, keyGen, itemCtx)
		if err != nil {
			return nil, fmt.Errorf("range item %d error: %w", i, err)
		}

		itemsWithStatics = append(itemsWithStatics, rangeItemWithStatics{
			tree:    itemTree,
			statics: itemTree.Statics,
			hash:    hashStatics(itemTree.Statics),
		})
	}
	return buildRangeTreeWithStatics(itemsWithStatics, ctx)
}

// buildRangeTreeWithStatics constructs the final range tree.
func buildRangeTreeWithStatics(items []rangeItemWithStatics, ctx *Context) (*TreeNode, error) {
	if len(items) == 0 {
		rangeTree := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			rangeTree.Statics = []string{""}
		}
		rangeTree.Range = NewRangeData([]interface{}{}, nil)
		return rangeTree, nil
	}

	// Check if all items have the same statics (homogeneous)
	firstHash := items[0].hash
	isHomogeneous := true
	for i := 1; i < len(items); i++ {
		if items[i].hash != firstHash {
			isHomogeneous = false
			break
		}
	}
	_ = isHomogeneous // both paths treated the same now

	idKey := detectIDKey(items[0].statics)

	if !hasExplicitKeyAttribute(items[0].statics) {
		for _, item := range items {
			hash := generateItemHash(item.tree)
			item.tree.SetDynamic("_k", hash)
		}
		idKey = "_k"
	}

	itemTrees := make([]interface{}, len(items))
	for i, item := range items {
		itemTrees[i] = extractItemDynamics(item.tree)
	}

	rangeTree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		rangeTree.Statics = items[0].statics
	}
	rangeTree.Range = NewRangeData(itemTrees, items[0].statics)
	rangeTree.Metadata = NewTreeMetadata(idKey)
	return rangeTree, nil
}

// extractItemDynamics extracts only the dynamics from an item tree.
func extractItemDynamics(itemTree *TreeNode) *TreeNode {
	return &TreeNode{
		Dynamics: itemTree.Dynamics,
	}
}
