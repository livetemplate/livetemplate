package parse

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"sync"
	"testing"
	"text/template/parse"
)

// TestParse_SimpleTemplate tests parsing a simple template.
func TestParse_SimpleTemplate(t *testing.T) {
	tmpl, err := Parse("<div>{{.Name}}</div>", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template")
	}

	if tmpl.ast == nil {
		t.Error("Expected non-nil AST")
	}
}

// TestParse_WithFuncMap tests parsing with function map.
func TestParse_WithFuncMap(t *testing.T) {
	funcMap := template.FuncMap{
		"upper": func(s string) string { return s },
	}

	tmpl, err := Parse("{{.Name | upper}}", funcMap)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template")
	}
}

// TestParse_InvalidSyntax tests parsing with syntax errors.
func TestParse_InvalidSyntax(t *testing.T) {
	_, err := Parse("{{.Name", nil)
	if err == nil {
		t.Error("Expected error for invalid syntax")
	}
}

// TestParse_EmptyTemplate tests parsing an empty template.
func TestParse_EmptyTemplate(t *testing.T) {
	tmpl, err := Parse("", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected non-nil template for empty string")
	}
}

// TestBuildTree_SimpleField tests building tree from simple field.
func TestBuildTree_SimpleField(t *testing.T) {
	tmpl, err := Parse("{{.Name}}", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]any{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree.Dynamics["0"] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics["0"])
	}
}

// TestBuildTree_NestedFields tests building tree with nested data.
func TestBuildTree_NestedFields(t *testing.T) {
	tmpl, err := Parse("{{.User.Name}}", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]any{
		"User": map[string]any{
			"Name": "John",
		},
	}
	ctx := &Context{IncludeStatics: true}

	tree, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if tree.Dynamics["0"] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics["0"])
	}
}

// TestBuildTreeFromAST_TextNode tests text node handling.
func TestBuildTreeFromAST_TextNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("Hello World")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	tree, err := buildTreeFromAST(tmpl.Tree.Root, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("buildTreeFromAST failed: %v", err)
	}

	if len(tree.Statics) == 0 {
		t.Error("Expected statics for text node")
	}
}

// TestBuildTreeFromAST_ActionNode tests action node handling.
func TestBuildTreeFromAST_ActionNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]any{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := buildTreeFromAST(tmpl.Tree.Root, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("buildTreeFromAST failed: %v", err)
	}

	if tree.Dynamics["0"] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics["0"])
	}
}

// TestBuildTreeFromAST_CommentNode tests comment handling.
func TestBuildTreeFromAST_CommentNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{/* comment */}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctx := &Context{IncludeStatics: true}
	tree, err := buildTreeFromAST(tmpl.Tree.Root, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("buildTreeFromAST failed: %v", err)
	}

	// Comments should produce empty tree
	if tree.HasDynamics() {
		t.Error("Expected no dynamics for comment node")
	}
}

// TestBuildTreeFromList_SingleNode tests list with single node.
func TestBuildTreeFromList_SingleNode(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]any{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := buildTreeFromList(tmpl.Tree.Root, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("buildTreeFromList failed: %v", err)
	}

	if tree.Dynamics["0"] != "John" {
		t.Errorf("Expected dynamics['0'] = 'John', got: %v", tree.Dynamics["0"])
	}
}

// TestBuildTreeFromList_MultipleNodes tests list with multiple nodes.
func TestBuildTreeFromList_MultipleNodes(t *testing.T) {
	tmpl, err := template.New("test").Parse("<div>{{.Name}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]any{"Name": "John"}
	ctx := &Context{IncludeStatics: true}

	tree, err := buildTreeFromList(tmpl.Tree.Root, data, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("buildTreeFromList failed: %v", err)
	}

	// Should have merged statics from multiple nodes
	if len(tree.Statics) < 2 {
		t.Errorf("Expected at least 2 statics, got: %d", len(tree.Statics))
	}
}

// TestBuildTreeFromList_EmptyList tests empty list handling.
func TestBuildTreeFromList_EmptyList(t *testing.T) {
	ctx := &Context{IncludeStatics: true}
	tree, err := buildTreeFromList(nil, nil, newMockKeyGen(), ctx)
	if err != nil {
		t.Fatalf("buildTreeFromList failed: %v", err)
	}

	// Empty list should return tree with single empty static
	if len(tree.Statics) != 1 {
		t.Errorf("Expected 1 static for empty list, got: %d", len(tree.Statics))
	}
}

// TestEvaluatePipe_Simple tests simple dot access.
func TestEvaluatePipe_Simple(t *testing.T) {
	data := map[string]any{"Name": "John"}
	ctx := &Context{}

	result, err := evaluatePipe(".Name", data, ctx)
	if err != nil {
		t.Fatalf("evaluatePipe failed: %v", err)
	}

	if result != "John" {
		t.Errorf("Expected 'John', got: %v", result)
	}
}

// TestEvaluatePipe_Complex tests complex pipeline.
func TestEvaluatePipe_Complex(t *testing.T) {
	data := map[string]any{
		"Items": []string{"a", "b", "c"},
	}
	ctx := &Context{}

	result, err := evaluatePipe(".Items", data, ctx)
	if err != nil {
		t.Fatalf("evaluatePipe failed: %v", err)
	}

	// Should return the slice
	if reflect.TypeOf(result).Kind() != reflect.Slice {
		t.Errorf("Expected slice, got: %T", result)
	}
}

// TestEvaluatePipe_WithFuncs tests pipeline with functions.
func TestEvaluatePipe_WithFuncs(t *testing.T) {
	funcMap := template.FuncMap{
		"upper": func(s string) string { return s + "_UPPER" },
	}

	data := map[string]any{"Name": "John"}
	ctx := &Context{FuncMap: funcMap}

	// Note: evaluatePipe uses the capture mechanism
	result, err := evaluatePipe(".Name", data, ctx)
	if err != nil {
		t.Fatalf("evaluatePipe failed: %v", err)
	}

	if result != "John" {
		t.Errorf("Expected 'John', got: %v", result)
	}
}

// TestFormatPipe tests pipe formatting.
func TestFormatPipe(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.Name}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actionNode := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	pipeStr := formatPipe(actionNode.Pipe)

	if pipeStr != ".Name" {
		t.Errorf("Expected '.Name', got: %q", pipeStr)
	}
}

// TestIsZeroValue_AllTypes tests zero value detection for all types.
func TestIsZeroValue_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"nil", nil, true},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"zero string", "", true},
		{"non-zero string", "hello", false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"nil slice", []string(nil), true},
		{"empty slice", []string{}, true},
		{"non-empty slice", []string{"a"}, false},
		{"nil map", map[string]string(nil), true},
		{"empty map", map[string]string{}, true},
		{"non-empty map", map[string]string{"a": "b"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.value)
			got := isZeroValue(v)
			if got != tt.want {
				t.Errorf("isZeroValue(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestGetSortedKeys_Performance tests that getSortedKeys is efficient.
func TestGetSortedKeys_Performance(t *testing.T) {
	// Create a map with many numeric keys
	m := make(map[string]any)
	for i := range 100 {
		m[fmt.Sprintf("%d", i)] = i
	}

	keys := getSortedKeys(m)

	// Verify we got all keys
	if len(keys) != 100 {
		t.Errorf("Expected 100 keys, got %d", len(keys))
	}

	// Verify sorted order
	for i := 0; i < len(keys)-1; i++ {
		var curr, next int
		if _, err := fmt.Sscanf(keys[i], "%d", &curr); err != nil {
			t.Errorf("Failed to parse key %q: %v", keys[i], err)
		}
		if _, err := fmt.Sscanf(keys[i+1], "%d", &next); err != nil {
			t.Errorf("Failed to parse key %q: %v", keys[i+1], err)
		}
		if curr > next {
			t.Errorf("Keys not sorted: %d > %d at position %d", curr, next, i)
		}
	}
}

// TestGetSortedKeys_EmptyMap tests empty map handling.
func TestGetSortedKeys_EmptyMap(t *testing.T) {
	m := make(map[string]any)
	keys := getSortedKeys(m)

	if keys != nil {
		t.Errorf("Expected nil for empty map, got %v", keys)
	}
}

// TestGetSortedKeys_Order tests specific ordering.
func TestGetSortedKeys_Order(t *testing.T) {
	m := map[string]any{
		"0":  "a",
		"10": "b",
		"2":  "c",
		"1":  "d",
	}

	keys := getSortedKeys(m)

	expected := []string{"0", "1", "2", "10"}
	if len(keys) != len(expected) {
		t.Fatalf("Expected %d keys, got %d", len(expected), len(keys))
	}

	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("Expected key %q at position %d, got %q", expected[i], i, key)
		}
	}
}

// TestBuildTreeFromList_ErrorContext tests error reporting with context.
func TestBuildTreeFromList_ErrorContext(t *testing.T) {
	// This tests that errors from child nodes include context.
	// We test with a template invocation which should fail with an error message.
	tmpl, err := template.New("test").Parse("{{template \"missing\" .}}")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	data := map[string]any{}
	ctx := &Context{IncludeStatics: true}

	_, err = buildTreeFromList(tmpl.Tree.Root, data, newMockKeyGen(), ctx)
	if err == nil {
		t.Error("Expected error for template invocation")
	}

	// Error should contain child node information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "child node") {
		t.Errorf("Expected error to contain 'child node', got: %v", errMsg)
	}
}

// TestGetOrParseTemplate_Caching tests that template caching works.
func TestGetOrParseTemplate_Caching(t *testing.T) {
	cache := &sync.Map{}
	cacheKey := "test-key"
	templateStr := "{{.Name}}"
	funcs := template.FuncMap{}

	// First call - should parse and cache
	tmpl1, err := getOrParseTemplate(cache, cacheKey, templateStr, funcs)
	if err != nil {
		t.Fatalf("First parse failed: %v", err)
	}

	// Second call - should hit cache
	tmpl2, err := getOrParseTemplate(cache, cacheKey, templateStr, funcs)
	if err != nil {
		t.Fatalf("Second parse failed: %v", err)
	}

	// Both templates should work
	if tmpl1 == nil || tmpl2 == nil {
		t.Error("Expected non-nil templates")
	}

	// Verify cache was used
	if _, ok := cache.Load(cacheKey); !ok {
		t.Error("Expected template to be cached")
	}

	// Verify that returned templates are different instances (cloned)
	// This is important for concurrent execution safety
	if tmpl1 == tmpl2 {
		t.Error("Expected different template instances from cloning, got same instance")
	}

	// Verify both templates can execute independently
	data := map[string]any{"Name": "Test"}
	var buf1, buf2 strings.Builder
	if err := tmpl1.Execute(&buf1, data); err != nil {
		t.Errorf("Template 1 execute failed: %v", err)
	}
	if err := tmpl2.Execute(&buf2, data); err != nil {
		t.Errorf("Template 2 execute failed: %v", err)
	}
	if buf1.String() != buf2.String() {
		t.Errorf("Template outputs differ: %q vs %q", buf1.String(), buf2.String())
	}
}

// TestGetOrParseTemplate_ConcurrentAccess tests that concurrent access to template cache is safe.
func TestGetOrParseTemplate_ConcurrentAccess(t *testing.T) {
	cache := &sync.Map{}
	cacheKey := "concurrent-test"
	templateStr := "{{.Value}}"
	funcs := template.FuncMap{}

	// Run many goroutines accessing the cache concurrently
	const numGoroutines = 100
	const numIterations = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numIterations)

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numIterations {
				tmpl, err := getOrParseTemplate(cache, cacheKey, templateStr, funcs)
				if err != nil {
					errChan <- fmt.Errorf("goroutine %d iteration %d: parse failed: %w", id, j, err)
					return
				}

				// Execute template to ensure it's valid
				data := map[string]any{"Value": fmt.Sprintf("g%d-i%d", id, j)}
				var buf strings.Builder
				if err := tmpl.Execute(&buf, data); err != nil {
					errChan <- fmt.Errorf("goroutine %d iteration %d: execute failed: %w", id, j, err)
					return
				}

				expected := fmt.Sprintf("g%d-i%d", id, j)
				if buf.String() != expected {
					errChan <- fmt.Errorf("goroutine %d iteration %d: got %q, want %q", id, j, buf.String(), expected)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Errorf("Concurrent access had %d errors:", len(errs))
		for _, err := range errs {
			t.Errorf("  - %v", err)
		}
	}

	// Verify cache was populated
	if _, ok := cache.Load(cacheKey); !ok {
		t.Error("Expected template to be cached after concurrent access")
	}
}
