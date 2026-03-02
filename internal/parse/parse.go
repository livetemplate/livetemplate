// Package parse handles template parsing and AST-based tree building.
// It walks Go template parse trees directly instead of using regex extraction.
package parse

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/template/parse"
)

// captureResultFuncName is a unique function name used internally to capture
// pipeline evaluation results. Generated lazily on first use with random suffix to
// prevent collision with user-defined functions.
var captureResultFuncName string
var captureResultFuncNameOnce sync.Once

// pipeTemplateCache caches parsed pipe templates to avoid repeated parsing.
// Key is the template string, value is the parsed *template.Template.
// Uses sync.Map for efficient concurrent access.
var pipeTemplateCache sync.Map

// astTemplateCache caches parsed templates for AST node handlers (actions, conditionals).
// Key is the template string, value is the parsed *template.Template.
// Uses sync.Map for efficient concurrent access.
var astTemplateCache sync.Map

// executionCache caches pipeline evaluation results to avoid redundant template execution.
// Key is hash of (templateName + pipeStr + dataHash), value is the evaluation result.
// Uses sync.Map for efficient concurrent access.
// Cache is invalidated when data changes (detected via hash comparison).
var executionCache sync.Map

// initCaptureFunc initializes the capture function name with a unique random suffix.
// This is called lazily on first use to avoid panicking during package initialization.
// If random generation fails, falls back to a pseudo-random approach.
func initCaptureFunc() {
	captureResultFuncNameOnce.Do(func() {
		// Try to generate a unique function name with random suffix
		randBytes := make([]byte, 8)
		if _, err := rand.Read(randBytes); err != nil {
			// Fallback to address-based unique identifier if crypto/rand fails
			// This is extremely unlikely but ensures library never panics
			// Use pointer address which is unique per process
			captureResultFuncName = fmt.Sprintf("__lvt_internal_capture_%p__", &captureResultFuncName)
		} else {
			captureResultFuncName = fmt.Sprintf("__lvt_internal_capture_%s__", hex.EncodeToString(randBytes))
		}
	})
}

// Template represents a parsed template with its AST and associated data.
type Template struct {
	name string
	ast  *parse.Tree
}

// Parse parses a template string into an executable template structure.
//
// This function parses Go template syntax into an Abstract Syntax Tree (AST) that can
// be used with BuildTree to generate tree structures for efficient client-side updates.
//
// Parameters:
//   - templateStr: The template string using Go template syntax ({{.Field}}, {{range}}, etc.)
//   - funcMap: Optional map of custom functions available in the template
//
// Returns:
//   - *Template: Parsed template containing the AST, ready for BuildTree
//   - error: Parse errors if template syntax is invalid
//
// Note: Template composition ({{template "name" .}} calls) must be flattened before
// calling Parse. Use the template flattening utilities in the parent package.
//
// Example:
//
//	tmpl, err := Parse("<div>{{.Name}}</div>", nil)
//	if err != nil {
//	    return err
//	}
//	tree, err := BuildTree(tmpl, data, keyGen, ctx)
func Parse(templateStr string, funcMap template.FuncMap) (*Template, error) {
	// Create context for parsing
	ctx := &Context{FuncMap: funcMap}

	// Parse template to get AST
	tmpl, err := newTemplateWithFuncs("temp", ctx).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
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
//
// This is the core function that replaces regex-based expression extraction by directly
// walking the Go template parse tree (AST) and evaluating expressions against the provided data.
//
// Parameters:
//   - tmpl: The parsed template from Parse() containing the AST
//   - data: The data context for template evaluation (typically a struct or map)
//   - keyGen: Generator for unique keys in range constructs (e.g., for list items)
//   - ctx: Evaluation context containing:
//   - FuncMap: Custom template functions available during evaluation
//   - IncludeStatics: Whether to include static HTML in the tree (true for first render,
//     false for updates to reduce payload size)
//
// Returns:
//   - *TreeNode: A tree structure containing:
//   - Statics: Array of static HTML strings (if ctx.IncludeStatics is true)
//   - Dynamics: Map of dynamic values keyed by position ("0", "1", etc.)
//   - Range: Range metadata for list/map iterations (if template contains {{range}})
//   - error: Any error encountered during AST walking or expression evaluation
//
// The tree structure represents the template as alternating static and dynamic parts:
//
//	Tree{ Statics: ["<div>", "</div>"], Dynamics: {"0": "value"} }
//	represents: <div>value</div>
//
// For first renders, include statics for complete HTML. For updates, omit statics
// (client caches them) to send only changed dynamics.
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

	for i, child := range node.Nodes {
		childTree, err := buildTreeFromAST(child, data, keyGen, ctx)
		if err != nil {
			return nil, fmt.Errorf("child node %d (%T): %w", i, child, err)
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

// keyValuePair holds a key and its numeric value for sorting.
type keyValuePair struct {
	key   string
	num   int
	valid bool // true if key was successfully parsed as numeric
}

// getSortedKeys returns keys from a map sorted numerically.
// Non-numeric keys are pushed to the end and sorted lexicographically.
func getSortedKeys(m map[string]interface{}) []string {
	if len(m) == 0 {
		return nil
	}

	// Create pairs of key and numeric value
	pairs := make([]keyValuePair, 0, len(m))
	for k := range m {
		var num int
		_, err := fmt.Sscanf(k, "%d", &num)
		pairs = append(pairs, keyValuePair{
			key:   k,
			num:   num,
			valid: err == nil,
		})
	}

	// Sort pairs: valid numeric keys first (by value), then invalid keys (lexicographically)
	sort.Slice(pairs, func(i, j int) bool {
		// Both valid - compare numerically
		if pairs[i].valid && pairs[j].valid {
			return pairs[i].num < pairs[j].num
		}
		// Only i valid - i comes first
		if pairs[i].valid {
			return true
		}
		// Only j valid - j comes first
		if pairs[j].valid {
			return false
		}
		// Both invalid - compare lexicographically
		return pairs[i].key < pairs[j].key
	})

	// Extract sorted keys
	keys := make([]string, len(pairs))
	for i, pair := range pairs {
		keys[i] = pair.key
	}
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

// getOrParseTemplate retrieves a cached template or parses a new one.
// Templates are cloned before applying FuncMap to allow concurrent execution with different functions.
// The cache parameter specifies which cache to use (astTemplateCache or pipeTemplateCache).
func getOrParseTemplate(cache *sync.Map, cacheKey, templateStr string, funcs template.FuncMap) (*template.Template, error) {
	// Try to get from cache
	if cached, ok := cache.Load(cacheKey); ok {
		cachedTmpl, ok := cached.(*template.Template)
		if !ok {
			// Cache corruption - entry exists but is not a template
			// Fall through to parse to recover
			goto parse
		}

		// Clone the cached template and apply the FuncMap
		clone, err := cachedTmpl.Clone()
		if err != nil {
			// If clone fails, fall back to parsing
			goto parse
		}
		// Apply FuncMap
		if len(funcs) > 0 {
			clone.Funcs(funcs)
		}
		return clone, nil
	}

parse:
	// Parse new template with FuncMap
	tmpl := template.New(cacheKey)
	if len(funcs) > 0 {
		tmpl = tmpl.Funcs(funcs)
	}
	parsedTmpl, err := tmpl.Parse(templateStr)
	if err != nil {
		return nil, err
	}

	// Store in cache for future use (without FuncMap, will be applied on clone)
	baseTmpl, err := template.New(cacheKey).Parse(templateStr)
	if err == nil {
		cache.Store(cacheKey, baseTmpl)
	}

	return parsedTmpl, nil
}

// getFuncMapFromContext extracts the function map from a context, returning nil if empty.
// This centralizes the nil-checking logic used throughout template parsing.
func getFuncMapFromContext(ctx *Context) template.FuncMap {
	if ctx != nil && ctx.FuncMap != nil && len(ctx.FuncMap) > 0 {
		return ctx.FuncMap
	}
	return nil
}

// getOrParseASTTemplate retrieves a cached AST node template or parses a new one.
// This is used by action and conditional handlers to avoid repeated parsing.
// Templates are cloned before applying FuncMap to allow concurrent execution with different functions.
func getOrParseASTTemplate(cacheKey, templateStr string, ctx *Context) (*template.Template, error) {
	return getOrParseTemplate(&astTemplateCache, cacheKey, templateStr, getFuncMapFromContext(ctx))
}

// evaluatePipe evaluates a pipe expression against data.
func evaluatePipe(pipeStr string, data interface{}, ctx *Context) (interface{}, error) {
	// Initialize capture function name lazily
	initCaptureFunc()

	if pipeStr == "." {
		return data, nil
	}

	var (
		captured   interface{}
		didCapture bool
	)

	// Build function map with capture helper and user-provided functions
	funcs := make(template.FuncMap)
	if userFuncs := getFuncMapFromContext(ctx); userFuncs != nil {
		for name, fn := range userFuncs {
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

// hashData generates a stable hash string from data for cache key generation.
// Uses SHA256 for cryptographic strength and JSON serialization for stability.
// Returns empty string on error (cache miss is safe fallback).
func hashData(data interface{}) string {
	// Handle nil data
	if data == nil {
		return "nil"
	}

	// Serialize data to JSON for stable hash
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		// If serialization fails, return empty string (cache will miss)
		return ""
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// evaluatePipeWithCache evaluates a pipe expression with result caching.
// Caches results based on (templateName + pipeStr + dataHash) to avoid redundant execution.
// Falls back to uncached evaluation if cache operations fail.
func evaluatePipeWithCache(templateName, pipeStr string, data interface{}, ctx *Context) (interface{}, error) {
	// Generate cache key from template name, pipe string, and data hash
	dataHash := hashData(data)
	cacheKey := templateName + ":" + pipeStr + ":" + dataHash

	// Try to get from cache
	if cached, ok := executionCache.Load(cacheKey); ok {
		return cached, nil
	}

	// Cache miss - evaluate the pipe
	result, err := evaluatePipe(pipeStr, data, ctx)
	if err != nil {
		return nil, err
	}

	// Store in cache for future use
	executionCache.Store(cacheKey, result)

	return result, nil
}

// InvalidateExecutionCache clears the execution cache.
// Called when data changes to ensure fresh evaluation.
func InvalidateExecutionCache() {
	executionCache = sync.Map{}
}

// getOrParsePipeTemplate retrieves a cached template or parses a new one.
// The cache key includes a prefix to distinguish between different template types.
// Templates are cloned before applying FuncMap to allow concurrent execution with different functions.
func getOrParsePipeTemplate(cacheKey, templateStr string, funcs template.FuncMap) (*template.Template, error) {
	return getOrParseTemplate(&pipeTemplateCache, cacheKey, templateStr, funcs)
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
