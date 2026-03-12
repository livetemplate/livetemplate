package parse

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template/parse"
	"unicode"
	"unicode/utf8"
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

// sortedVarNames returns variable names from vars sorted by descending length.
// This ensures longer names are matched first, preventing partial matches
// (e.g., $col is processed before $c).
func sortedVarNames(vars *orderedVars) []string {
	var names []string
	vars.Range(func(key string, _ interface{}) {
		names = append(names, key)
	})
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})
	return names
}

// capitalizeFieldName converts a variable name to a capitalized field name.
// Uses UTF8-safe rune decoding to correctly handle multi-byte characters.
// Examples: "show" -> "Show", "isActive" -> "IsActive", "ñame" -> "Ñame"
func capitalizeFieldName(varName string) string {
	if len(varName) == 0 {
		return varName
	}
	r, size := utf8.DecodeRuneInString(varName)
	return string(unicode.ToUpper(r)) + varName[size:]
}

// mergeFieldsIntoMap copies all accessible fields from value into the target map.
// For structs, copies all exported fields. For maps, copies all entries.
// Skips keys that already exist in target to avoid silent overwrites.
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
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			if _, exists := target[keyStr]; !exists {
				target[keyStr] = v.MapIndex(key).Interface()
			}
		}

	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath == "" {
				fieldValue := v.Field(i)
				if _, exists := target[field.Name]; !exists {
					target[field.Name] = fieldValue.Interface()
				}
			}
		}

	default:
		return nil
	}

	return nil
}

// buildExecData constructs the transformed expression string and synthetic execution
// data map for evaluating a Go template expression containing variable references.
// It handles:
//   - UTF8-safe capitalization of variable names ($var -> .Var)
//   - Descending-length variable name ordering to prevent partial matches ($col before $c)
//   - Root variable ($. / standalone $) detection and transformation
//   - Dot context field merging so expressions like {{$c.Method .Type}} work
func buildExecData(expr string, varCtx *varContext) (string, map[string]interface{}, error) {
	sortedNames := sortedVarNames(&varCtx.vars)
	transformedExpr := expr
	execData := make(map[string]interface{})

	for _, varName := range sortedNames {
		if strings.Contains(expr, "$"+varName) {
			fieldName := capitalizeFieldName(varName)
			transformedExpr = strings.ReplaceAll(transformedExpr, "$"+varName, "."+fieldName)
			varValue, _ := varCtx.vars.Get(varName)
			execData[fieldName] = varValue
		}
	}

	if detectsRootVariable(expr, varCtx.vars) {
		// NOTE: "RootData" is a reserved synthetic key. If template data has a field
		// named "RootData", it will be shadowed by the root variable substitution.
		transformedExpr = strings.ReplaceAll(transformedExpr, "$.", ".RootData.")
		execData["RootData"] = varCtx.parent
	}

	// Merge dot fields so mixed-context expressions (e.g. {{$c.Method .Type}}) work.
	// Skip when dot is nil to avoid unnecessary reflection.
	if varCtx.dot != nil {
		if err := mergeFieldsIntoMap(varCtx.dot, execData); err != nil {
			return "", nil, fmt.Errorf("failed to merge dot fields: %w", err)
		}
	}

	return transformedExpr, execData, nil
}

// transformAndEvalWithVars evaluates an expression that may contain variable references.
// It transforms $varName.Field into .VarName.Field with a synthetic data map,
// then evaluates via evaluatePipeWithCache. If no variables are referenced, it
// evaluates directly against the dot context.
// Variable names are processed in descending length order to prevent partial matches
// (e.g., $col is replaced before $c).
func transformAndEvalWithVars(expr string, varCtx *varContext, ctx *Context) (interface{}, error) {
	sortedNames := sortedVarNames(&varCtx.vars)
	usesVar := false
	for _, varName := range sortedNames {
		if strings.Contains(expr, "$"+varName) {
			usesVar = true
			break
		}
	}
	usesRoot := detectsRootVariable(expr, varCtx.vars)

	if !usesVar && !usesRoot {
		return evaluatePipeWithCache(ctx.TemplateName, expr, varCtx.dot, ctx)
	}

	transformedExpr, execData, err := buildExecData(expr, varCtx)
	if err != nil {
		return nil, err
	}
	return evaluatePipeWithCache(ctx.TemplateName, transformedExpr, execData, ctx)
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

	case *parse.CommentNode:
		return NewTreeNode(), nil

	case *parse.TemplateNode:
		return nil, fmt.Errorf("template invocation found - should be flattened: %s", n.Name)

	default:
		return nil, fmt.Errorf("unhandled node type in varCtx: %T", n)
	}
}

// buildTreeFromListWithVars processes a list of nodes with variable context.
func buildTreeFromListWithVars(node *parse.ListNode, varCtx *varContext, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	if node == nil || len(node.Nodes) == 0 {
		return createEmptyTree(ctx), nil
	}

	// Check if any child has variable declarations; if so, delegate to the
	// declaration-aware path which registers variables into varCtx.
	if listHasVarDeclarations(node) {
		return buildTreeFromListWithDeclVars(node, varCtx, keyGen, ctx)
	}

	var statics []string
	tree := NewTreeNode()
	dynamicIndex := 0
	statics = append(statics, "")

	for i, child := range node.Nodes {
		childTree, err := buildTreeFromASTWithVars(child, varCtx, keyGen, ctx)
		if err != nil {
			return nil, fmt.Errorf("child node %d (%T): %w", i, child, err)
		}

		// Handle range comprehension (has Range field)
		if childTree.HasRange() {
			if len(node.Nodes) == 1 {
				return childTree, nil
			}
			tree.SetDynamic(fmt.Sprintf("%d", dynamicIndex), childTree)
			dynamicIndex++
			statics = append(statics, "")
			continue
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
