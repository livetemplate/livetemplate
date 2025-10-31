package parse

import (
	"fmt"
	"text/template/parse"
)

// varContext holds variable bindings for template execution.
type varContext struct {
	parent interface{} // Original data
	vars   orderedVars // Variable bindings ($index, $todo, etc.)
	dot    interface{} // Current dot context
}

// orderedVars is a deterministic map-like structure that preserves insertion order.
// This ensures that variable iteration is deterministic for reproducible tree generation.
type orderedVars []struct {
	key   string
	value interface{}
}

// newOrderedVars creates an empty orderedVars.
func newOrderedVars() orderedVars {
	return make(orderedVars, 0, 2) // Most ranges have 1-2 variables
}

// Set adds or updates a key-value pair.
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

// Get retrieves a value by key.
func (ov orderedVars) Get(key string) (interface{}, bool) {
	for _, pair := range ov {
		if pair.key == key {
			return pair.value, true
		}
	}
	return nil, false
}

// Len returns the number of key-value pairs.
func (ov orderedVars) Len() int {
	return len(ov)
}

// Range iterates over all key-value pairs in insertion order.
func (ov orderedVars) Range(fn func(key string, value interface{})) {
	for _, pair := range ov {
		fn(pair.key, pair.value)
	}
}

// buildTreeFromASTWithVars is like buildTreeFromAST but handles variable references.
func buildTreeFromASTWithVars(node parse.Node, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
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

// buildTreeFromListWithVars processes a list of nodes with variable context.
func buildTreeFromListWithVars(node *parse.ListNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
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
