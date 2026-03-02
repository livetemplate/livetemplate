package parse

import (
	"fmt"
	"text/template/parse"
)

// varContext holds variable bindings for template execution.
// It maintains three pieces of context:
//   - parent: The original data passed to the template
//   - vars: Variable bindings created by range/with ($index, $value, etc.)
//   - dot: The current dot context (may differ from parent in nested scopes)
type varContext struct {
	parent interface{} // Original data
	vars   orderedVars // Variable bindings ($index, $todo, etc.)
	dot    interface{} // Current dot context
}

// varPair represents a single variable binding with a name and value.
type varPair struct {
	key   string
	value interface{}
}

// orderedVars is a deterministic map-like structure that preserves insertion order.
// This ensures that variable iteration is deterministic for reproducible tree generation.
// It uses both a slice (for order preservation) and a map (for O(1) lookup performance).
type orderedVars struct {
	pairs []varPair
	index map[string]int // Maps key to position in pairs slice
}

const (
	// defaultVarCapacity is the initial capacity for variable storage.
	// Most range loops use 1-2 variables ($index, $value or $key, $value).
	defaultVarCapacity = 2
)

// newOrderedVars creates an empty orderedVars with optimized capacity.
func newOrderedVars() orderedVars {
	return orderedVars{
		pairs: make([]varPair, 0, defaultVarCapacity),
		index: make(map[string]int, defaultVarCapacity),
	}
}

// Set adds or updates a key-value pair.
// The key should be a valid Go template variable name (matching $[a-zA-Z_][a-zA-Z0-9_]*).
// Empty keys are silently ignored to prevent errors during template parsing.
func (ov *orderedVars) Set(key string, value interface{}) {
	// Reject empty keys to prevent invalid variable bindings
	if key == "" {
		return
	}

	// Check if key exists in index - O(1) lookup
	if pos, exists := ov.index[key]; exists {
		// Update existing value
		ov.pairs[pos].value = value
		return
	}

	// Key doesn't exist - append it and update index
	pos := len(ov.pairs)
	ov.pairs = append(ov.pairs, varPair{key: key, value: value})
	ov.index[key] = pos
}

// Get retrieves a value by key. Returns (value, true) if found, (nil, false) otherwise.
// Uses O(1) map lookup for optimal performance.
func (ov orderedVars) Get(key string) (interface{}, bool) {
	pos, exists := ov.index[key]
	if !exists {
		return nil, false
	}
	return ov.pairs[pos].value, true
}

// Len returns the number of key-value pairs.
func (ov orderedVars) Len() int {
	return len(ov.pairs)
}

// Range iterates over all key-value pairs in insertion order.
// Calls fn for each pair. If fn panics, iteration stops and the panic propagates.
func (ov orderedVars) Range(fn func(key string, value interface{})) {
	for _, pair := range ov.pairs {
		fn(pair.key, pair.value)
	}
}

// createEmptyTree creates a tree node representing empty content.
// If statics should be included, returns a tree with a single empty static string.
func createEmptyTree(ctx *Context) *TreeNode {
	if ctx.ShouldIncludeStatics() {
		return NewTreeNodeWithStatics([]string{""})
	}
	return NewTreeNode()
}

// buildTreeFromASTWithVars is like buildTreeFromAST but handles variable references.
func buildTreeFromASTWithVars(node parse.Node, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	if node == nil {
		return createEmptyTree(ctx), nil
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
		// Nested range - propagate inherited variables through
		return handleRangeNodeWithInheritedVars(n, varCtx, keyGen, ctx)

	case *parse.WithNode:
		// With block - propagate inherited variables through
		return handleWithNodeWithVars(n, varCtx, keyGen, ctx)

	default:
		return nil, fmt.Errorf("unhandled node type in varCtx: %T", n)
	}
}

// buildTreeFromListWithVars processes a list of nodes with variable context.
func buildTreeFromListWithVars(node *parse.ListNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	if node == nil || len(node.Nodes) == 0 {
		return createEmptyTree(ctx), nil
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

		// Copy dynamic values from child, renumbering them
		childKeys := getSortedKeys(childTree.Dynamics)
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
