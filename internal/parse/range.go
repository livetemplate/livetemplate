package parse

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"text/template/parse"

	"github.com/livetemplate/livetemplate/internal/build"
)

// parallelIterateThreshold is the minimum collection length at which
// iterateSlice splits work across goroutines. Below this, the goroutine
// setup + sync overhead exceeds the per-item walk cost. The threshold was
// chosen empirically: at N=256 with 8 cores the parallel path matches the
// sequential one; above it parallel pulls ahead.
const parallelIterateThreshold = 256

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
		emptyRange.Statics = defaultEmptyStatics
	}
	emptyRange.Range = NewRangeData([]interface{}{}, nil)
	return emptyRange, nil
}

// iterateSlice handles slice/array range iteration. Above
// parallelIterateThreshold items it dispatches to iterateSliceParallel,
// which splits per-item walkAST calls across runtime.NumCPU() workers.
// The walker, evaluator, parsed AST, and Context are all read-only after
// construction; each item gets its own varContext so no shared mutable
// state crosses goroutines.
func iterateSlice(node *parse.RangeNode, collection reflect.Value, eval *evaluator, data interface{}, parentVarCtx *varContext, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	n := collection.Len()
	if n >= parallelIterateThreshold {
		return iterateSliceParallel(node, collection, eval, data, parentVarCtx, hasVarDecls, keyGen, ctx, n)
	}
	itemsWithStatics := make([]rangeItemWithStatics, 0, n)
	itemCtx := contextWithStatics(ctx)

	for i := 0; i < n; i++ {
		item := collection.Index(i).Interface()
		itemVarCtx := buildRangeItemVarCtx(node, i, item, data, parentVarCtx, hasVarDecls)

		itemTree, err := walkAST(node.List, eval, data, itemVarCtx, keyGen, itemCtx)
		if err != nil {
			return nil, fmt.Errorf("range item %d error: %w", i, err)
		}

		itemsWithStatics = append(itemsWithStatics, rangeItemWithStatics{tree: itemTree})
	}
	return buildRangeTreeWithStatics(itemsWithStatics, ctx)
}

// iterateSliceParallel is the parallel implementation of iterateSlice.
// Each worker handles a contiguous chunk of the input collection and writes
// directly to its assigned slice indices, so result order matches input
// order without merge work. Reflect Index calls are pre-extracted into a
// flat []interface{} on the main goroutine to avoid concurrent reflect
// access on the same reflect.Value.
//
// The keyGen parameter is shared across workers but never invoked from
// inside walkAST (verified by grep — `.Next()` returns zero hits in
// internal/parse/). If a future change adds a keyGen.Next() call on a
// path reachable from a range item body, this function must serialize
// that call or fall back to the sequential path; the current behaviour
// is safe only because nothing calls it.
func iterateSliceParallel(node *parse.RangeNode, collection reflect.Value, eval *evaluator, data interface{}, parentVarCtx *varContext, hasVarDecls bool, keyGen KeyGenerator, ctx *Context, n int) (*TreeNode, error) {
	itemValues := make([]interface{}, n)
	for i := 0; i < n; i++ {
		itemValues[i] = collection.Index(i).Interface()
	}

	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}
	chunkSize := (n + workers - 1) / workers

	items := make([]rangeItemWithStatics, n)
	errs := make([]error, workers)
	itemCtx := contextWithStatics(ctx)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(workerIdx, start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				itemVarCtx := buildRangeItemVarCtx(node, i, itemValues[i], data, parentVarCtx, hasVarDecls)
				itemTree, err := walkAST(node.List, eval, data, itemVarCtx, keyGen, itemCtx)
				if err != nil {
					errs[workerIdx] = fmt.Errorf("range item %d error: %w", i, err)
					return
				}
				items[i] = rangeItemWithStatics{tree: itemTree}
			}
		}(w, start, end)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return buildRangeTreeWithStatics(items, ctx)
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

		itemsWithStatics = append(itemsWithStatics, rangeItemWithStatics{tree: itemTree})
	}
	return buildRangeTreeWithStatics(itemsWithStatics, ctx)
}

// buildRangeTreeWithStatics constructs the final range tree.
func buildRangeTreeWithStatics(items []rangeItemWithStatics, ctx *Context) (*TreeNode, error) {
	if len(items) == 0 {
		rangeTree := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			rangeTree.Statics = defaultEmptyStatics
		}
		rangeTree.Range = NewRangeData([]interface{}{}, nil)
		return rangeTree, nil
	}

	firstStatics := items[0].tree.Statics

	idKey := build.PositionKey(detectIDKey(firstStatics))

	if !hasExplicitKeyAttribute(firstStatics) {
		for _, item := range items {
			hash := generateItemHash(item.tree)
			item.tree.AutoKey = hash
		}
		idKey = "_k"
	}

	itemTrees := make([]interface{}, len(items))
	for i, item := range items {
		itemTrees[i] = extractItemDynamics(item.tree)
	}

	rangeTree := NewTreeNode()
	if ctx.ShouldIncludeStatics() {
		rangeTree.Statics = firstStatics
	}
	rangeTree.Range = NewRangeData(itemTrees, firstStatics)
	rangeTree.Metadata = NewTreeMetadata(idKey)
	return rangeTree, nil
}

// extractItemDynamics extracts only the dynamics from an item tree (no statics).
func extractItemDynamics(itemTree *TreeNode) *TreeNode {
	result := NewTreeNode()
	result.AutoKey = itemTree.AutoKey
	if len(itemTree.Dynamics) > 0 {
		result.Dynamics = make([]interface{}, len(itemTree.Dynamics))
		copy(result.Dynamics, itemTree.Dynamics)
	}
	return result
}
