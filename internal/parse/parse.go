// Package parse handles template parsing and AST-based tree building.
// It walks Go template parse trees directly instead of using regex extraction.
package parse

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/template/parse"
)

// captureResultFuncName is a unique function name used internally to capture
// pipeline evaluation results. Generated once at init with random suffix to
// prevent collision with user-defined functions.
var captureResultFuncName string

// pipeTemplateCache caches parsed pipe templates to avoid repeated parsing.
// Key is the template string, value is the parsed *template.Template.
// Uses sync.Map for efficient concurrent access.
var pipeTemplateCache sync.Map

func init() {
	// Generate unique function name with random suffix
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		// Fallback to deterministic name if random fails (shouldn't happen)
		captureResultFuncName = "__lvt_internal_capture_result_fallback__"
		return
	}
	captureResultFuncName = fmt.Sprintf("__lvt_internal_capture_%s__", hex.EncodeToString(randBytes))
}

// Template represents a parsed template with its AST and associated data.
type Template struct {
	name string
	ast  *parse.Tree
}

// Parse parses a template string into an executable template structure.
// It normalizes spacing, handles template composition via flattening,
// and returns the AST for tree building.
func Parse(templateStr string, funcMap template.FuncMap) (*Template, error) {
	// Create context for parsing
	ctx := &Context{FuncMap: funcMap}

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

	return &Template{
		name: tmpl.Name(),
		ast:  tmpl.Tree,
	}, nil
}

// BuildTree constructs a tree structure from the parsed AST and data.
// This is the core function that replaces regex-based expression extraction.
func BuildTree(tmpl *Template, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	// Default context if not provided
	if ctx == nil {
		ctx = &Context{}
	}

	// Build tree by walking AST
	return buildTreeFromAST(tmpl.ast.Root, data, keyGen, ctx)
}

// buildTreeFromAST recursively walks the AST and constructs the tree structure.
func buildTreeFromAST(node parse.Node, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
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
		// Comments render nothing
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

// buildTreeFromList processes a list of nodes and merges their trees.
func buildTreeFromList(node *parse.ListNode, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error) {
	if node == nil || len(node.Nodes) == 0 {
		if ctx.ShouldIncludeStatics() {
			return NewTreeNodeWithStatics([]string{""}), nil
		}
		return NewTreeNode(), nil
	}

	// Walk AST and merge trees from all nodes
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
			// as a nested structure. Do NOT merge its statics.
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

		// Copy dynamic values from child, renumbering them
		childKeys := getSortedKeys(childTree.Dynamics)
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

// getSortedKeys returns keys from a map sorted numerically.
func getSortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// Sort numerically using efficient stdlib sort (O(n log n) vs O(n²) bubble sort)
	sort.Slice(keys, func(i, j int) bool {
		var iVal, jVal int
		_, _ = fmt.Sscanf(keys[i], "%d", &iVal)
		_, _ = fmt.Sscanf(keys[j], "%d", &jVal)
		return iVal < jVal
	})
	return keys
}

// newTemplateWithFuncs creates a template preloaded with the context's function map.
func newTemplateWithFuncs(name string, ctx *Context) *template.Template {
	tmpl := template.New(name)
	if ctx != nil && ctx.FuncMap != nil && len(ctx.FuncMap) > 0 {
		tmpl = tmpl.Funcs(ctx.FuncMap)
	}
	return tmpl
}

// evaluatePipe evaluates a pipe expression against data.
func evaluatePipe(pipeStr string, data interface{}, ctx *Context) (interface{}, error) {
	if pipeStr == "." {
		return data, nil
	}

	var (
		captured   interface{}
		didCapture bool
	)

	// Build function map with capture helper and user-provided functions
	funcs := make(template.FuncMap)
	if ctx != nil && ctx.FuncMap != nil {
		for name, fn := range ctx.FuncMap {
			funcs[name] = fn
		}
	}
	funcs[captureResultFuncName] = func(v interface{}) string {
		captured = v
		didCapture = true
		return ""
	}

	// Execute the pipeline through a capture helper
	// Use cached template if available, otherwise parse and cache it
	captureTemplateStr := fmt.Sprintf("{{%s (%s)}}", captureResultFuncName, pipeStr)
	tmpl, err := getOrParsePipeTemplate("pipe:"+pipeStr, captureTemplateStr, funcs)
	if err != nil {
		return nil, err
	}

	if err := tmpl.Execute(&bytes.Buffer{}, data); err != nil {
		return nil, err
	}

	if didCapture {
		return captured, nil
	}

	// Fallback to string representation
	// Use cached template if available, otherwise parse and cache it
	fallbackTemplateStr := fmt.Sprintf("{{%s}}", pipeStr)
	fallbackTmpl, err := getOrParsePipeTemplate("fallback:"+pipeStr, fallbackTemplateStr, funcs)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := fallbackTmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.String(), nil
}

// getOrParsePipeTemplate retrieves a cached template or parses a new one.
// The cache key includes a prefix to distinguish between different template types.
// Templates are cloned before applying FuncMap to allow concurrent execution with different functions.
func getOrParsePipeTemplate(cacheKey, templateStr string, funcs template.FuncMap) (*template.Template, error) {
	// Try to get from cache
	if cached, ok := pipeTemplateCache.Load(cacheKey); ok {
		if cachedTmpl, ok := cached.(*template.Template); ok {
			// Clone the cached template and apply the FuncMap
			// This allows reusing the parsed structure while supporting dynamic functions
			clone, err := cachedTmpl.Clone()
			if err != nil {
				// If clone fails, fall back to parsing
				goto parse
			}
			clone.Funcs(funcs)
			return clone, nil
		}
	}

parse:
	// Parse new template
	tmpl, err := template.New(cacheKey).Funcs(funcs).Parse(templateStr)
	if err != nil {
		return nil, err
	}

	// Store in cache for future use
	// Store the template without FuncMap applied (will be applied on clone)
	baseTmpl, err := template.New(cacheKey).Parse(templateStr)
	if err == nil {
		pipeTemplateCache.Store(cacheKey, baseTmpl)
	}

	return tmpl, nil
}

// formatPipe converts a pipe node to a string representation.
func formatPipe(pipe *parse.PipeNode) string {
	if pipe == nil {
		return ""
	}
	return strings.TrimSpace(pipe.String())
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
		// For arrays, check if all elements are zero
		for i := 0; i < v.Len(); i++ {
			if !isZeroValue(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		// Use IsZero() method (available since Go 1.13) for efficient struct comparison
		return v.IsZero()
	default:
		// Fallback for uncommon types (e.g., UnsafePointer)
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}

// normalizeTemplateSpacing normalizes whitespace in templates.
// TODO: Implement template normalization logic.
func normalizeTemplateSpacing(templateStr string) string {
	return templateStr
}

// hasTemplateComposition checks if a template uses composition.
// TODO: Implement composition detection.
func hasTemplateComposition(tmpl *template.Template) bool {
	return false
}

// flattenTemplate flattens template composition into a single template.
// TODO: Implement template flattening.
func flattenTemplate(tmpl *template.Template) (string, error) {
	return "", fmt.Errorf("template flattening not yet implemented")
}
