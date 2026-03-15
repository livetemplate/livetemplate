package parse

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template/parse"
)

// formatPipe converts a pipe node to a string representation.
func formatPipe(pipe *parse.PipeNode) string {
	if pipe == nil {
		return ""
	}
	return strings.TrimSpace(pipe.String())
}

// createEmptyTree creates a tree node representing empty content.
func createEmptyTree(ctx *Context) *TreeNode {
	if ctx.ShouldIncludeStatics() {
		return NewTreeNodeWithStatics([]string{""})
	}
	return NewTreeNode()
}

// contextWithStatics returns a context that always includes statics for internal use.
// Range items ALWAYS need statics collected internally for:
// - detectIDKey() to find key attribute position in statics
// - Range.Statics for diff operations
// - handleEmptyToItemsTransition to send statics to client
func contextWithStatics(ctx *Context) *Context {
	if ctx == nil {
		return &Context{
			IsFirstRender:  true,
			IncludeStatics: true,
		}
	}
	if ctx.ShouldIncludeStatics() {
		return ctx
	}
	newCtx := *ctx
	newCtx.IncludeStatics = true
	newCtx.IsFirstRender = true
	return &newCtx
}

// getSortedKeys returns keys from a map sorted numerically.
func getSortedKeys(m map[string]interface{}) []string {
	if len(m) == 0 {
		return nil
	}
	type kv struct {
		key   string
		num   int
		valid bool
	}
	pairs := make([]kv, 0, len(m))
	for k := range m {
		var num int
		_, err := fmt.Sscanf(k, "%d", &num)
		pairs = append(pairs, kv{key: k, num: num, valid: err == nil})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].valid && pairs[j].valid {
			return pairs[i].num < pairs[j].num
		}
		if pairs[i].valid {
			return true
		}
		if pairs[j].valid {
			return false
		}
		return pairs[i].key < pairs[j].key
	})
	keys := make([]string, len(pairs))
	for i, pair := range pairs {
		keys[i] = pair.key
	}
	return keys
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
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
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		return real(c) == 0 && imag(c) == 0
	case reflect.Chan, reflect.Func:
		return v.IsNil()
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZeroValue(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		return v.IsZero()
	default:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}

// listHasVarDeclarations checks if any child node in a list is a variable declaration.
func listHasVarDeclarations(node *parse.ListNode) bool {
	for _, child := range node.Nodes {
		if actionNode, ok := child.(*parse.ActionNode); ok &&
			actionNode.Pipe != nil && len(actionNode.Pipe.Decl) > 0 {
			return true
		}
	}
	return false
}
