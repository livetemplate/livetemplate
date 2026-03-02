package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
	"text/template/parse"
)

// staticsHashPrefixLen is the number of hex characters to use for statics hash keys.
// 16 hex chars (64 bits) provides low collision risk even with many unique statics variants.
const staticsHashPrefixLen = 16

// contextWithStatics returns a context that always includes statics for internal use.
//
// Range items ALWAYS need statics collected internally for:
// - detectIDKey() to find key attribute position in statics
// - Range.Statics for diff operations (insert/append/prepend)
// - handleEmptyToItemsTransition to send statics to client
//
// Note: This affects internal tree building only. The wire format optimization
// (stripping statics for updates) happens later in prepareTreeForClient().
// Setting IsFirstRender=true here ensures ShouldIncludeStatics() returns true
// regardless of CurrentPath checks - it's not semantically a "first render" but
// rather a way to force statics inclusion for internal processing.
func contextWithStatics(ctx *Context) *Context {
	if ctx == nil {
		return &Context{
			IsFirstRender:  true,
			IncludeStatics: true,
		}
	}

	// If already including statics, return as-is
	if ctx.ShouldIncludeStatics() {
		return ctx
	}

	// Create a copy that forces statics inclusion.
	// We set IsFirstRender=true because ShouldIncludeStatics() checks it first,
	// bypassing CurrentPath/ClientStructures checks that might return false.
	newCtx := *ctx
	newCtx.IncludeStatics = true
	newCtx.IsFirstRender = true
	return &newCtx
}

// hashStatics creates a short hash key for a statics array.
// Used for deduplicating statics in heterogeneous ranges.
func hashStatics(statics []string) string {
	h := sha256.New()
	for _, s := range statics {
		h.Write([]byte(s))
		h.Write([]byte{0}) // separator
	}
	return hex.EncodeToString(h.Sum(nil))[:staticsHashPrefixLen]
}

// rangeItemWithStatics holds an item tree and its statics together.
type rangeItemWithStatics struct {
	tree    *TreeNode
	statics []string
	hash    string
}

// handleRangeNode processes {{range}}...{{end}} constructs.
// It supports slices, arrays, and maps with optional variable declarations.
// Returns a TreeNode with Range field containing all items and metadata with ID key position.
// For empty collections, returns the else branch or an empty range tree.
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
// Handles both simple ranges ({{range .Items}}) and ranges with variable declarations
// ({{range $i, $v := .Items}}). Returns the collection to iterate over or an error.
func extractRangeCollection(node *parse.RangeNode, data interface{}, ctx *Context) (interface{}, error) {
	if len(node.Pipe.Decl) > 0 {
		// Has variable declarations - extract just the collection expression
		if len(node.Pipe.Cmds) > 0 {
			lastCmd := node.Pipe.Cmds[len(node.Pipe.Cmds)-1]
			if len(lastCmd.Args) > 0 {
				collectionExpr := lastCmd.Args[0].String()
				return evaluatePipeWithCache(ctx.TemplateName, collectionExpr, data, ctx)
			}
			return nil, fmt.Errorf("range with declarations has no collection expression")
		}
		return nil, fmt.Errorf("range with declarations has no commands")
	}

	// No variable declarations - simple {{range .Items}}
	pipeStr := formatPipe(node.Pipe)
	return evaluatePipeWithCache(ctx.TemplateName, pipeStr, data, ctx)
}

// isEmpty checks if a collection value is nil or empty.
// Returns true for nil, zero-length slices, arrays, or maps.
func isEmpty(v reflect.Value) bool {
	return !v.IsValid() ||
		(v.Kind() == reflect.Slice && v.Len() == 0) ||
		(v.Kind() == reflect.Array && v.Len() == 0) ||
		(v.Kind() == reflect.Map && v.Len() == 0)
}

// handleEmptyRange handles empty collections or else branches.
// If an else branch exists, builds and returns its tree. Otherwise returns an empty range tree.
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
// Builds a tree for each item, collecting statics from all items for heterogeneous support.
// The hasVarDecls flag determines whether to use variable context or direct dot context.
func handleSliceRange(node *parse.RangeNode, collection reflect.Value, data interface{}, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Collect all items with their statics
	itemsWithStatics := make([]rangeItemWithStatics, 0, collection.Len())

	// IMPORTANT: Range items ALWAYS need statics for internal use:
	// 1. detectIDKey() needs statics to find key attribute position
	// 2. Range.Statics is used for diff operations (insert/append/prepend)
	// 3. handleEmptyToItemsTransition needs statics to send to client
	// Create a context that includes statics for item tree building.
	itemCtx := contextWithStatics(ctx)

	for i := 0; i < collection.Len(); i++ {
		item := collection.Index(i).Interface()

		var itemTree *TreeNode
		var err error

		if hasVarDecls {
			itemTree, err = executeRangeBodyWithVars(node, i, item, data, keyGen, itemCtx)
		} else {
			varCtx := &varContext{
				parent: data,
				vars:   newOrderedVars(),
				dot:    item,
			}
			itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, itemCtx)
		}

		if err != nil {
			return nil, fmt.Errorf("range item %d error: %w", i, err)
		}

		// Collect item with its statics
		itemsWithStatics = append(itemsWithStatics, rangeItemWithStatics{
			tree:    itemTree,
			statics: itemTree.Statics,
			hash:    hashStatics(itemTree.Statics),
		})
	}

	return buildRangeTreeWithStatics(itemsWithStatics, ctx)
}

// handleMapRange processes map range iterations.
// Similar to handleSliceRange but iterates over map keys.
// Note: Map iteration order is non-deterministic in Go.
func handleMapRange(node *parse.RangeNode, collection reflect.Value, data interface{}, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Collect all items with their statics
	keys := collection.MapKeys()
	itemsWithStatics := make([]rangeItemWithStatics, 0, len(keys))

	// IMPORTANT: Range items ALWAYS need statics for internal use (same as handleSliceRange)
	itemCtx := contextWithStatics(ctx)

	for i, key := range keys {
		item := collection.MapIndex(key).Interface()

		var itemTree *TreeNode
		var err error

		if hasVarDecls {
			itemTree, err = executeRangeBodyWithVars(node, key.Interface(), item, data, keyGen, itemCtx)
		} else {
			varCtx := &varContext{
				parent: data,
				vars:   newOrderedVars(),
				dot:    item,
			}
			itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, itemCtx)
		}

		if err != nil {
			return nil, fmt.Errorf("range item %d error: %w", i, err)
		}

		// Collect item with its statics
		itemsWithStatics = append(itemsWithStatics, rangeItemWithStatics{
			tree:    itemTree,
			statics: itemTree.Statics,
			hash:    hashStatics(itemTree.Statics),
		})
	}

	return buildRangeTreeWithStatics(itemsWithStatics, ctx)
}

// buildRangeTreeWithStatics constructs the final range tree with per-item statics support.
// For homogeneous ranges (all items same statics), uses single Statics array.
// For heterogeneous ranges (items have different statics), uses StaticsMap with _sk keys.
func buildRangeTreeWithStatics(items []rangeItemWithStatics, ctx *Context) (*TreeNode, error) {
	if len(items) == 0 {
		// Empty range
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

	// Build item trees
	itemTrees := make([]interface{}, len(items))

	if isHomogeneous {
		// Detect ID key from first item's statics
		idKey := detectIDKey(items[0].statics)

		// If no explicit key attribute found, inject auto-generated keys.
		// This ensures both server and client use the same key for item matching.
		if !hasExplicitKeyAttribute(items[0].statics) {
			for _, item := range items {
				hash := generateItemHash(item.tree)
				item.tree.SetDynamic("_k", hash)
			}
			idKey = "_k"
		}

		// All items share the same statics - use original format
		for i, item := range items {
			itemTrees[i] = extractItemDynamics(item.tree)
		}

		rangeTree := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			rangeTree.Statics = items[0].statics
		}
		// Always set Range.Statics for internal use (diff operations),
		// even when TreeNode.Statics is nil (not included in wire format).
		rangeTree.Range = NewRangeData(itemTrees, items[0].statics)
		rangeTree.Metadata = NewTreeMetadata(idKey)
		return rangeTree, nil
	}

	// Heterogeneous ranges (items with different statics) are handled the same as homogeneous.
	// Use first item's statics as representative. The fingerprint-based diff will detect
	// structure changes and trigger full tree sends when item structures change.
	// This simplifies the code - no StaticsMap complexity.

	// Detect ID key from first item's statics (for metadata)
	idKey := detectIDKey(items[0].statics)

	// If no explicit key attribute found, inject auto-generated keys.
	// This ensures both server and client use the same key for item matching.
	if !hasExplicitKeyAttribute(items[0].statics) {
		for _, item := range items {
			hash := generateItemHash(item.tree)
			item.tree.SetDynamic("_k", hash)
		}
		idKey = "_k"
	}

	// Extract dynamics from each item
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

// executeRangeBodyWithVars executes a range body with variable declarations.
// The indexOrKey parameter is either an int (for slices/arrays) or the key (for maps).
func executeRangeBodyWithVars(node *parse.RangeNode, indexOrKey interface{}, item interface{}, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	return executeRangeBodyWithVarsAndInherited(node, indexOrKey, item, data, nil, keyGen, ctx)
}

// executeRangeBodyWithVarsAndInherited executes a range body with both range-declared
// variables and inherited variables from parent scope.
func executeRangeBodyWithVarsAndInherited(node *parse.RangeNode, indexOrKey interface{}, item interface{}, data interface{}, inheritedVars *orderedVars, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Create a variable context
	varCtx := &varContext{
		parent: data,
		vars:   newOrderedVars(),
		dot:    item,
	}

	// Copy inherited variables first (parent scope vars like $c)
	if inheritedVars != nil {
		inheritedVars.Range(func(key string, value interface{}) {
			varCtx.vars.Set(key, value)
		})
	}

	// Populate variables from range declarations (these override inherited if same name).
	// Strip $ prefix: Go's parser stores "$v" but evaluateActionWithVars
	// builds search patterns as "$" + varName, so vars must be stored without $.
	if len(node.Pipe.Decl) == 1 {
		// {{range $v := ...}} - single variable (value)
		varName := strings.TrimPrefix(node.Pipe.Decl[0].Ident[0], "$")
		varCtx.vars.Set(varName, item)
	} else if len(node.Pipe.Decl) >= 2 {
		// {{range $i, $v := ...}} or {{range $k, $v := ...}}
		indexKeyVar := strings.TrimPrefix(node.Pipe.Decl[0].Ident[0], "$")
		valueVar := strings.TrimPrefix(node.Pipe.Decl[1].Ident[0], "$")
		varCtx.vars.Set(indexKeyVar, indexOrKey)
		varCtx.vars.Set(valueVar, item)
	}

	// Walk the range body AST with the variable context
	return buildTreeFromASTWithVars(node.List, varCtx, keyGen, ctx)
}

// handleRangeNodeWithInheritedVars handles range nodes while propagating inherited
// variables from the parent scope. This enables variables like $c defined outside
// a range block to be accessible inside the range body and nested ranges.
func handleRangeNodeWithInheritedVars(node *parse.RangeNode, parentVarCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Extract collection - evaluate pipe against the dot context
	collection, err := extractRangeCollectionFromVarCtx(node, parentVarCtx, ctx)
	if err != nil {
		return nil, err
	}

	collectionValue := reflect.ValueOf(collection)
	if isEmpty(collectionValue) {
		if node.ElseList != nil {
			return buildTreeFromASTWithVars(node.ElseList, parentVarCtx, keyGen, ctx)
		}
		emptyRange := NewTreeNode()
		if ctx.ShouldIncludeStatics() {
			emptyRange.Statics = []string{""}
		}
		emptyRange.Range = NewRangeData([]interface{}{}, nil)
		return emptyRange, nil
	}

	kind := collectionValue.Kind()
	if kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
		return nil, fmt.Errorf("range over non-iterable type: %v", kind)
	}

	hasVarDecls := len(node.Pipe.Decl) > 0

	if kind == reflect.Map {
		return handleMapRangeWithInheritedVars(node, collectionValue, parentVarCtx, hasVarDecls, keyGen, ctx)
	}

	return handleSliceRangeWithInheritedVars(node, collectionValue, parentVarCtx, hasVarDecls, keyGen, ctx)
}

// extractRangeCollectionFromVarCtx extracts the range collection, handling both
// simple expressions and variable references in the pipe.
func extractRangeCollectionFromVarCtx(node *parse.RangeNode, varCtx *varContext, ctx *Context) (interface{}, error) {
	if len(node.Pipe.Decl) > 0 {
		if len(node.Pipe.Cmds) > 0 {
			lastCmd := node.Pipe.Cmds[len(node.Pipe.Cmds)-1]
			if len(lastCmd.Args) > 0 {
				collectionExpr := lastCmd.Args[0].String()
				// Check if expression uses a variable (e.g., $c.Items)
				usesVar := false
				varCtx.vars.Range(func(varName string, _ interface{}) {
					if strings.Contains(collectionExpr, "$"+varName) {
						usesVar = true
					}
				})
				if usesVar {
					// Evaluate using variable context
					result := evaluateActionWithVars("{{"+collectionExpr+"}}", varCtx, ctx)
					// The result is a string, but we need the actual value
					// Use evaluatePipe with var-transformed expression
					transformedExpr := collectionExpr
					execData := make(map[string]interface{})
					varCtx.vars.Range(func(varName string, varValue interface{}) {
						if strings.Contains(collectionExpr, "$"+varName) {
							fieldName := strings.ToUpper(varName[:1]) + varName[1:]
							transformedExpr = strings.ReplaceAll(transformedExpr, "$"+varName, "."+fieldName)
							execData[fieldName] = varValue
						}
					})
					_ = result // discard string result
					return evaluatePipeWithCache(ctx.TemplateName, transformedExpr, execData, ctx)
				}
				return evaluatePipeWithCache(ctx.TemplateName, collectionExpr, varCtx.dot, ctx)
			}
		}
		return nil, fmt.Errorf("range with declarations has no collection expression")
	}

	pipeStr := formatPipe(node.Pipe)
	// Check if pipe references variables
	usesVar := false
	varCtx.vars.Range(func(varName string, _ interface{}) {
		if strings.Contains(pipeStr, "$"+varName) {
			usesVar = true
		}
	})
	if usesVar {
		transformedExpr := pipeStr
		execData := make(map[string]interface{})
		varCtx.vars.Range(func(varName string, varValue interface{}) {
			if strings.Contains(pipeStr, "$"+varName) {
				fieldName := strings.ToUpper(varName[:1]) + varName[1:]
				transformedExpr = strings.ReplaceAll(transformedExpr, "$"+varName, "."+fieldName)
				execData[fieldName] = varValue
			}
		})
		return evaluatePipeWithCache(ctx.TemplateName, transformedExpr, execData, ctx)
	}
	return evaluatePipeWithCache(ctx.TemplateName, pipeStr, varCtx.dot, ctx)
}

// handleSliceRangeWithInheritedVars processes slice range with inherited variables.
func handleSliceRangeWithInheritedVars(node *parse.RangeNode, collection reflect.Value, parentVarCtx *varContext, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	itemsWithStatics := make([]rangeItemWithStatics, 0, collection.Len())
	itemCtx := contextWithStatics(ctx)

	for i := 0; i < collection.Len(); i++ {
		item := collection.Index(i).Interface()
		var itemTree *TreeNode
		var err error

		if hasVarDecls {
			itemTree, err = executeRangeBodyWithVarsAndInherited(node, i, item, parentVarCtx.dot, &parentVarCtx.vars, keyGen, itemCtx)
		} else {
			// No range-specific vars, but still pass inherited vars
			varCtx := &varContext{
				parent: parentVarCtx.dot,
				vars:   newOrderedVars(),
				dot:    item,
			}
			parentVarCtx.vars.Range(func(key string, value interface{}) {
				varCtx.vars.Set(key, value)
			})
			itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, itemCtx)
		}

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

// handleMapRangeWithInheritedVars processes map range with inherited variables.
func handleMapRangeWithInheritedVars(node *parse.RangeNode, collection reflect.Value, parentVarCtx *varContext, hasVarDecls bool, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	keys := collection.MapKeys()
	itemsWithStatics := make([]rangeItemWithStatics, 0, len(keys))
	itemCtx := contextWithStatics(ctx)

	for i, key := range keys {
		item := collection.MapIndex(key).Interface()
		var itemTree *TreeNode
		var err error

		if hasVarDecls {
			itemTree, err = executeRangeBodyWithVarsAndInherited(node, key.Interface(), item, parentVarCtx.dot, &parentVarCtx.vars, keyGen, itemCtx)
		} else {
			varCtx := &varContext{
				parent: parentVarCtx.dot,
				vars:   newOrderedVars(),
				dot:    item,
			}
			parentVarCtx.vars.Range(func(key string, value interface{}) {
				varCtx.vars.Set(key, value)
			})
			itemTree, err = buildTreeFromASTWithVars(node.List, varCtx, keyGen, itemCtx)
		}

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

// extractItemDynamics extracts only the dynamics from an item tree,
// avoiding unnecessary map allocations by reusing the existing dynamics map.
func extractItemDynamics(itemTree *TreeNode) *TreeNode {
	// Return a tree node with only dynamics (no statics, no range, no metadata)
	// This is more efficient than creating a new map and copying
	return &TreeNode{
		Dynamics: itemTree.Dynamics,
	}
}

// hasExplicitKeyAttribute checks if any key attribute is present in the statics.
// Returns true if id=, data-key=, key=, or similar attributes are found.
//
// NOTE: The keyAttrs list must stay synchronized with:
//   - detectIDKey() in this file
//   - internal/keys.DefaultKeyAttributes
func hasExplicitKeyAttribute(statics []string) bool {
	if len(statics) == 0 {
		return false
	}

	// Key attributes to check (must match detectIDKey and internal/keys.DefaultKeyAttributes)
	keyAttrs := []string{
		"id=\"",
		"data-key=\"",
		"key=\"",
		"data-lvt-key=\"",
		"lvt-key=\"",
		"data-id=\"",
		"x-key=\"",
		"v-key=\"",
	}

	for _, static := range statics {
		for _, attr := range keyAttrs {
			if strings.Contains(static, attr) {
				return true
			}
		}
	}
	return false
}

// detectIDKey detects which position in the dynamics contains the item ID
// by scanning the statics array for key attribute patterns.
// Returns the position as a string (e.g., "1" for the second dynamic position).
// Returns "0" as default if no key attribute is found.
//
// Searches for key attributes in priority order: id, data-key, key, data-lvt-key,
// lvt-key, data-id, x-key (Alpine.js), v-key (Vue.js).
func detectIDKey(statics []string) string {
	if len(statics) == 0 {
		return "0"
	}

	// Key attributes to search for (in priority order)
	// Using a single slice for both prefix and full attribute reduces allocations
	keyAttrs := []string{
		"id=\"",
		"data-key=\"",
		"key=\"",
		"data-lvt-key=\"",
		"lvt-key=\"",
		"data-id=\"",
		"x-key=\"", // Alpine.js compatibility
		"v-key=\"", // Vue.js compatibility
	}

	// Scan through statics array - single pass with early exit
	for i, static := range statics {
		// Find earliest matching attribute in this static string
		minPos := -1
		matchedIdx := -1

		for attrIdx, attr := range keyAttrs {
			if pos := strings.Index(static, attr); pos != -1 {
				// Found a match - check if it's earlier than previous matches
				if minPos == -1 || pos < minPos {
					minPos = pos
					matchedIdx = attrIdx
					// If we found id= (highest priority), no need to check others
					if attrIdx == 0 {
						break
					}
				}
			}
		}

		if matchedIdx != -1 {
			// The dynamic value after this static is the ID
			// Position i in statics means dynamic at position i
			return fmt.Sprintf("%d", i)
		}
	}

	// Default to position 0 if no key attribute found
	return "0"
}

// hashPrefixLength is the number of characters to use from the generated hash
// for compact item identifiers. 12 hex characters = 48 bits of entropy.
//
// This provides sufficient uniqueness for single-session DOM item matching:
// - Birthday paradox collision probability: ~0.0001% at 10,000 items
// - Collision becomes likely (~50%) only around 16 million items
// - For typical web apps with <1000 range items, collisions are negligible
const hashPrefixLength = 12

// generateItemHash creates a stable hash for a range item based on its content.
// This is used when no explicit key attribute is provided in the template.
// Uses FNV-1a hash for fast, non-cryptographic content fingerprinting.
//
// NOTE: This function duplicates internal/diff.GenerateItemHash intentionally.
// The parse package cannot import diff (circular dependency), and the
// implementations must stay synchronized. Both use FNV-1a with hashPrefixLength=12.
func generateItemHash(item *TreeNode) string {
	if item == nil {
		return ""
	}

	// Create a canonical JSON representation for hashing
	// Sort keys to ensure deterministic ordering
	keys := make([]string, 0, len(item.Dynamics))
	for k := range item.Dynamics {
		// Skip internal/reserved fields
		if k != "_k" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// Build canonical representation
	var parts []string
	for _, k := range keys {
		val, _ := item.GetDynamic(k)
		valJSON, err := json.Marshal(val)
		if err != nil {
			// Fallback to Go syntax representation for more stable output
			// than %v (which has non-deterministic map ordering)
			parts = append(parts, fmt.Sprintf("%s:%#v", k, val))
		} else {
			parts = append(parts, fmt.Sprintf("%s:%s", k, string(valJSON)))
		}
	}

	// Hash the canonical representation using FNV-1a
	content := strings.Join(parts, "|")
	hasher := fnv.New64a()
	hasher.Write([]byte(content))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Return first hashPrefixLength characters for compactness
	if len(hash) >= hashPrefixLength {
		return hash[:hashPrefixLength]
	}
	return hash
}
