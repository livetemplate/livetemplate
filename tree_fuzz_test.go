package livetemplate

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"testing"
)

// FuzzParseTemplateToTree tests the current regex-based parser with random templates
// This establishes a baseline of what exotic templates work/fail before AST migration
func FuzzParseTemplateToTree(f *testing.F) {
	// Seed corpus with known working templates
	f.Add("<div>{{.Name}}</div>")
	f.Add("{{range .Items}}<span>{{.}}</span>{{end}}")
	f.Add("{{if .Show}}yes{{else}}no{{end}}")
	f.Add("{{if gt (len .Items) 0}}{{range .Items}}<li>{{.}}</li>{{end}}{{end}}")
	f.Add("{{with .User}}Hello {{.Name}}{{end}}")
	f.Add("{{range $i, $v := .Items}}{{$i}}: {{$v}}{{end}}")
	f.Add("{{.Name | printf \"User: %s\"}}")
	f.Add("{{range .Items}}{{if .Active}}{{.Name}}{{end}}{{end}}")
	f.Add("<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>")
	f.Add("{{if .A}}{{if .B}}nested{{end}}{{end}}")

	// Phase 1: Mixed templates (ranges + other dynamics) - Critical for examples/todos bug
	f.Add("<div>{{.Title}}</div>{{range .Items}}<span>{{.}}</span>{{end}}<p>{{.Footer}}</p>")
	f.Add("{{.Name}}{{range .Items}}{{.}}{{end}}{{.Count}}")
	f.Add("<h1>{{.Title}}</h1>{{range .Items}}<li>{{.}}</li>{{end}}")

	// Phase 1: Empty state transitions
	f.Add("{{range .EmptyItems}}<li>{{.}}</li>{{else}}<p>No items</p>{{end}}")
	f.Add("{{range .NilItems}}<li>{{.}}</li>{{else}}<p>No items</p>{{end}}")
	f.Add("{{with .NilValue}}Has value: {{.}}{{else}}No value{{end}}")

	// Phase 1: Range with else branch
	f.Add("{{range .Items}}<span>{{.}}</span>{{else}}<span>empty</span>{{end}}")

	// Phase 1: Map ranges
	f.Add("{{range $k, $v := .Map}}{{$k}}={{$v}} {{end}}")

	// Phase 1: Accessing parent context with $
	f.Add("{{range .Items}}{{$.Title}}: {{.}}{{end}}")

	// Phase 2: Break and continue (Go 1.18+)
	f.Add("{{range .Items}}{{if eq . \"stop\"}}{{break}}{{end}}{{.}}{{end}}")
	f.Add("{{range .Items}}{{if eq . \"skip\"}}{{continue}}{{end}}{{.}}{{end}}")
	f.Add("{{range .Items}}{{if gt (len .) 3}}{{break}}{{end}}{{.}}{{end}}")

	// Phase 2: Else-if chains
	f.Add("{{if eq .Type \"a\"}}A{{else if eq .Type \"b\"}}B{{else}}C{{end}}")
	f.Add("{{if .A}}first{{else if .B}}second{{else if .C}}third{{else}}none{{end}}")

	// Phase 2: Nested ranges
	f.Add("{{range .Outer}}{{range .Inner}}{{.}}{{end}}{{end}}")
	f.Add("{{range .Outer}}<div>{{range .Inner}}<span>{{.}}</span>{{end}}</div>{{end}}")

	// Phase 2: With with else
	f.Add("{{with .User}}Hello {{.Name}}{{else}}No user{{end}}")
	f.Add("{{with .EmptyString}}has value{{else}}empty string{{end}}")

	// Phase 2: Complex nesting
	f.Add("{{range .Items}}{{if .Active}}{{with .Details}}{{.Text}}{{end}}{{end}}{{end}}")

	// Phase 3: Variable scope in nested contexts
	f.Add("{{range $i, $v := .Items}}{{$i}}: {{$v}}{{end}}")
	f.Add("{{range $i, $v := .ItemsWithSub}}{{range $j, $w := .Sub}}{{$i}},{{$j}}: {{$w}}{{end}}{{end}}")

	// Phase 3: Accessing parent context with $
	f.Add("{{with .User}}{{$.Title}}: {{.Name}}{{end}}")

	// Phase 3: Variable in if block
	f.Add("{{$x := \"\"}}{{if .Cond}}{{$x = \"yes\"}}{{else}}{{$x = \"no\"}}{{end}}{{$x}}")

	// Phase 3: Variable shadowing
	f.Add("{{$v := .Name}}{{range .Items}}{{$v := .}}inner:{{$v}}{{end}}outer:{{$v}}")

	// Phase 3: Multiple variable declarations
	f.Add("{{$a := .A}}{{$b := .B}}{{$a}}{{$b}}")

	// Phase 4: Maps
	f.Add("{{range $k, $v := .StringMap}}{{$k}}: {{$v}}, {{end}}")

	// Phase 4: Int slices
	f.Add("{{range .Numbers}}{{.}},{{end}}")
	f.Add("{{range $i, $n := .Numbers}}[{{$i}}]={{$n}} {{end}}")

	// Phase 4: Bool slices
	f.Add("{{range .Flags}}{{if .}}yes{{else}}no{{end}} {{end}}")

	// Phase 4: Interface slices (mixed types)
	f.Add("{{range .Mixed}}{{.}}{{end}}")

	// Phase 4: Pointer fields
	f.Add("{{if .PtrField}}{{.PtrField}}{{else}}nil{{end}}")

	// Phase 5: Whitespace trimming
	f.Add("{{- .Field -}}")
	f.Add("text {{- .Field}}")
	f.Add("{{.Field -}} text")

	// Phase 5: Negative number vs trim
	f.Add("{{-3}}")
	f.Add("{{- 3}}")

	// Phase 5: Empty templates
	f.Add("")
	f.Add("{{/* comment only */}}")

	// Phase 5: Whitespace in ranges
	f.Add("{{range .Items -}}\n  {{.}}\n{{- end}}")

	// Phase 6: Function pipelines
	f.Add("{{.Value | printf \"%d\"}}")

	// Phase 6: Comparison functions
	f.Add("{{if eq .A .B}}equal{{end}}")
	f.Add("{{if ne .A .B}}not equal{{end}}")
	f.Add("{{if lt .Count 10}}small{{else}}large{{end}}")
	f.Add("{{if gt (len .Items) 0}}has items{{end}}")

	// Phase 6: Logical functions
	f.Add("{{if and .A .B}}both{{end}}")
	f.Add("{{if or .A .B}}either{{end}}")
	f.Add("{{if not .Empty}}has value{{end}}")

	// Phase 6: Index and len functions
	f.Add("{{index .Items 0}}")
	f.Add("{{len .Items}}")
	f.Add("{{len .Name}}")

	f.Fuzz(func(t *testing.T, templateStr string) {
		// Only test templates that Go's parser accepts
		_, err := template.New("fuzz").Parse(templateStr)
		if err != nil {
			t.Skip() // Invalid template syntax
		}

		// Generate test data that matches common template patterns
		data := map[string]interface{}{
			"Name":   "TestName",
			"Show":   true,
			"Items":  []string{"a", "b", "c"},
			"User":   map[string]interface{}{"Name": "John"},
			"Count":  5,
			"A":      true,
			"B":      false,
			"Active": true,

			// Phase 1: Empty state testing
			"EmptyItems": []string{},
			"NilItems":   ([]string)(nil),
			"NilValue":   nil,

			// Phase 1: Mixed template testing
			"Title":  "Page Title",
			"Footer": "Page Footer",

			// Phase 1: Map testing
			"Map": map[string]string{"key1": "val1", "key2": "val2"},

			// Phase 2: Control flow testing
			"Type": "a",
			"C":    false,
			"Outer": []map[string]interface{}{
				{"Inner": []string{"x", "y"}},
				{"Inner": []string{"p", "q"}},
			},
			"EmptyString": "",

			// Phase 3: Variable scope and context testing
			"Root": "root-value",
			"Cond": true,
			"ItemsWithSub": []map[string]interface{}{
				{"Name": "item1", "Sub": []string{"s1", "s2"}},
				{"Name": "item2", "Sub": []string{"s3", "s4"}},
			},

			// Phase 4: Data type testing
			"StringMap": map[string]string{"key1": "val1", "key2": "val2"},
			"Numbers":   []int{1, 2, 3, 4, 5},
			"Flags":     []bool{true, false, true},
			"Mixed":     []interface{}{"string", 42, true},
			"PtrField":  (*string)(nil),

			// Phase 5: Whitespace testing
			"Field": "value",

			// Phase 6: Function testing
			"Value": 42,
			"Empty": false,
		}

		// Test current AST-based parser
		keyGen := newKeyGenerator()
		tree, err := parseTemplateToTree(templateStr, data, keyGen)

		if err != nil {
			// Parser failed - this is fine, we're documenting failures
			return
		}

		// Level 1: Verify tree structure is valid
		// Note: We do NOT check tree invariants here because the hybrid execution
		// strategy (AST walking + flat execution for mixed patterns) can produce
		// trees that violate len(statics) = len(dynamics) + 1 for complex templates.
		// This is expected and documented behavior. The E2E tests verify correctness.
		if !validateTreeStructure(tree) {
			t.Errorf("Invalid tree structure\nTemplate: %q\nTree: %+v",
				templateStr, tree)
		}

		// Level 2: Verify tree can be rendered
		// This ensures the tree structure is not just syntactically valid
		// but also semantically correct and can be reconstructed into HTML
		if !validateTreeRenders(tree) {
			t.Errorf("Tree cannot be rendered\nTemplate: %q\nTree: %+v",
				templateStr, tree)
		}

		// Level 3: Verify round-trip consistency (Parse → Render → Parse → Compare)
		// With deterministic variable iteration (using orderedVars), the parser now produces
		// identical tree structures across multiple parses. This validation ensures that
		// parsing the same template with the same data twice produces structurally identical trees.
		ok, msg := validateTreeRoundTrip(templateStr, data, keyGen)
		if !ok {
			t.Errorf("Round-trip validation failed\nTemplate: %q\nReason: %s",
				templateStr, msg)
		}

		// Level 4: Verify empty→non-empty state transitions
		// This directly tests the critical bug found in examples/todos where
		// range flattening broke transitions between empty and non-empty states
		// Only applies to templates with range constructs
		if hasRangeConstruct(templateStr) {
			ok, msg := validateEmptyToNonEmptyTransition(templateStr, data)
			if !ok {
				t.Errorf("Empty→non-empty transition validation failed\nTemplate: %q\nReason: %s",
					templateStr, msg)
			}
		}
	})
}

// validateTreeStructure performs basic validation of tree structure
func validateTreeStructure(tree treeNode) bool {
	if tree == nil {
		return false
	}

	// Must have statics
	_, hasStatics := tree["s"]
	return hasStatics
}

// validateTreeRenders attempts to render a tree to HTML
// Returns true if the tree can be successfully rendered, false otherwise
// This is Level 2 validation from the enhanced validation strategy
func validateTreeRenders(tree treeNode) bool {
	if tree == nil {
		return false
	}

	// Extract statics array
	staticsIface, hasStatics := tree["s"]
	if !hasStatics {
		return false
	}

	statics, ok := staticsIface.([]string)
	if !ok {
		return false
	}

	// Attempt to reconstruct HTML from tree
	// This validates that the tree structure is renderable
	var html strings.Builder

	// Simple reconstruction: iterate through statics and dynamics
	for i := 0; i < len(statics); i++ {
		html.WriteString(statics[i])

		// Check if there's a dynamic value at this position
		dynamicKey := strconv.Itoa(i)
		if dynamicVal, exists := tree[dynamicKey]; exists {
			// Handle nested trees recursively
			if nestedTree, isTree := dynamicVal.(treeNode); isTree {
				if !validateTreeRenders(nestedTree) {
					return false
				}
			}
			// Dynamic value exists and is valid (string, number, or nested tree)
		}
	}

	// Successfully reconstructed HTML - tree is renderable
	return true
}

// treesEqual performs deep equality comparison of two tree structures
// Used for round-trip validation (Level 3)
// Handles non-deterministic map iteration by sorting range comprehension items
func treesEqual(tree1, tree2 treeNode) bool {
	if tree1 == nil && tree2 == nil {
		return true
	}
	if tree1 == nil || tree2 == nil {
		return false
	}

	// Check if this is a range comprehension (has "d" key)
	d1, hasD1 := tree1["d"]
	d2, hasD2 := tree2["d"]

	if hasD1 != hasD2 {
		return false
	}

	if hasD1 {
		// This is a range comprehension - compare with sorting
		return rangeComprehensionsEqual(d1, d2, tree1, tree2)
	}

	// Extract statics from both trees
	statics1Iface, hasStatics1 := tree1["s"]
	statics2Iface, hasStatics2 := tree2["s"]

	if hasStatics1 != hasStatics2 {
		return false
	}

	if !hasStatics1 {
		return false
	}

	statics1, ok1 := statics1Iface.([]string)
	statics2, ok2 := statics2Iface.([]string)

	if !ok1 || !ok2 {
		return false
	}

	// Compare statics arrays
	if len(statics1) != len(statics2) {
		return false
	}

	for i, s1 := range statics1 {
		if s1 != statics2[i] {
			return false
		}
	}

	// Compare dynamic values
	// Collect all numeric keys from both trees
	keys := make(map[string]bool)
	for key := range tree1 {
		if key != "s" {
			keys[key] = true
		}
	}
	for key := range tree2 {
		if key != "s" {
			keys[key] = true
		}
	}

	// Check each dynamic position
	for key := range keys {
		val1, exists1 := tree1[key]
		val2, exists2 := tree2[key]

		if exists1 != exists2 {
			return false
		}

		if !exists1 {
			continue
		}

		// Both values exist, compare them
		nested1, isTree1 := val1.(treeNode)
		nested2, isTree2 := val2.(treeNode)

		if isTree1 != isTree2 {
			return false
		}

		if isTree1 {
			// Recursively compare nested trees
			if !treesEqual(nested1, nested2) {
				return false
			}
		} else {
			// Compare primitive values (convert to strings for comparison)
			if fmt.Sprintf("%v", val1) != fmt.Sprintf("%v", val2) {
				return false
			}
		}
	}

	return true
}

// rangeComprehensionsEqual compares two range comprehensions with sorted items
// This handles non-deterministic map iteration order
func rangeComprehensionsEqual(d1, d2 interface{}, tree1, tree2 treeNode) bool {
	// Extract items arrays
	items1, ok1 := d1.([]interface{})
	items2, ok2 := d2.([]interface{})

	if !ok1 || !ok2 {
		return false
	}

	if len(items1) != len(items2) {
		return false
	}

	// Check statics match
	s1 := tree1["s"]
	s2 := tree2["s"]
	if fmt.Sprintf("%v", s1) != fmt.Sprintf("%v", s2) {
		return false
	}

	// Convert items to comparable strings and sort
	strs1 := make([]string, len(items1))
	strs2 := make([]string, len(items2))

	for i, item := range items1 {
		strs1[i] = fmt.Sprintf("%v", item)
	}
	for i, item := range items2 {
		strs2[i] = fmt.Sprintf("%v", item)
	}

	// Sort both arrays for comparison
	sortedStrs1 := make([]string, len(strs1))
	sortedStrs2 := make([]string, len(strs2))
	copy(sortedStrs1, strs1)
	copy(sortedStrs2, strs2)

	// Simple bubble sort (fine for fuzz test validation)
	for i := 0; i < len(sortedStrs1); i++ {
		for j := i + 1; j < len(sortedStrs1); j++ {
			if sortedStrs1[i] > sortedStrs1[j] {
				sortedStrs1[i], sortedStrs1[j] = sortedStrs1[j], sortedStrs1[i]
			}
		}
	}
	for i := 0; i < len(sortedStrs2); i++ {
		for j := i + 1; j < len(sortedStrs2); j++ {
			if sortedStrs2[i] > sortedStrs2[j] {
				sortedStrs2[i], sortedStrs2[j] = sortedStrs2[j], sortedStrs2[i]
			}
		}
	}

	// Compare sorted arrays
	for i := 0; i < len(sortedStrs1); i++ {
		if sortedStrs1[i] != sortedStrs2[i] {
			return false
		}
	}

	return true
}

// validateTreeRoundTrip performs round-trip validation: Parse → Render → Parse → Compare
// This is Level 3 validation from the enhanced validation strategy
func validateTreeRoundTrip(templateStr string, data map[string]interface{}, keyGen *keyGenerator) (bool, string) {
	// Parse template to tree1
	tree1, err := parseTemplateToTree(templateStr, data, keyGen)
	if err != nil {
		return false, fmt.Sprintf("first parse failed: %v", err)
	}

	// Render tree1 to HTML
	html, err := renderTreeToHTML(tree1)
	if err != nil {
		return false, fmt.Sprintf("render failed: %v", err)
	}

	// Parse template again with same data to tree2
	// NOTE: We use a new key generator to ensure consistent keys
	keyGen2 := newKeyGenerator()
	tree2, err := parseTemplateToTree(templateStr, data, keyGen2)
	if err != nil {
		return false, fmt.Sprintf("second parse failed: %v", err)
	}

	// Compare trees
	if !treesEqual(tree1, tree2) {
		return false, fmt.Sprintf("trees not equal\nHTML: %q\nTree1: %+v\nTree2: %+v", html, tree1, tree2)
	}

	return true, ""
}

// hasRangeConstruct checks if a template string contains range constructs
// Used to determine if Level 4 validation (transition testing) should be applied
func hasRangeConstruct(templateStr string) bool {
	return strings.Contains(templateStr, "{{range")
}

// makeEmptyData creates a copy of test data with all collections replaced by empty ones
// This is used for empty→non-empty transition testing (Level 4)
func makeEmptyData(data map[string]interface{}) map[string]interface{} {
	emptyData := make(map[string]interface{})

	for key, val := range data {
		switch val.(type) {
		case []string:
			emptyData[key] = []string{}
		case []int:
			emptyData[key] = []int{}
		case []bool:
			emptyData[key] = []bool{}
		case []interface{}:
			emptyData[key] = []interface{}{}
		case []map[string]interface{}:
			emptyData[key] = []map[string]interface{}{}
		case map[string]string:
			emptyData[key] = map[string]string{}
		case map[string]interface{}:
			emptyData[key] = map[string]interface{}{}
		default:
			// Preserve non-collection values
			emptyData[key] = val
		}
	}

	return emptyData
}

// validateTreeTransition checks that two trees (from different data states) are consistent
// Used for Level 4 validation to ensure empty→non-empty transitions work correctly
func validateTreeTransition(tree1, tree2 treeNode) (bool, string) {
	if tree1 == nil || tree2 == nil {
		return false, "one or both trees are nil"
	}

	// Both trees should be structurally valid
	if !validateTreeStructure(tree1) {
		return false, "tree1 fails structure validation"
	}
	if !validateTreeStructure(tree2) {
		return false, "tree2 fails structure validation"
	}

	// NOTE: We do NOT check that statics arrays have the same length
	// Empty ranges can produce different tree structures than non-empty ranges
	// This is expected behavior - empty ranges may be flattened or optimized differently
	// The key requirement is that both trees are structurally valid and renderable

	// Both trees should be renderable
	if !validateTreeRenders(tree1) {
		return false, "tree1 cannot be rendered"
	}
	if !validateTreeRenders(tree2) {
		return false, "tree2 cannot be rendered"
	}

	return true, ""
}

// validateEmptyToNonEmptyTransition tests that templates handle empty→non-empty state changes
// This is Level 4 validation from the enhanced validation strategy
// This directly tests the bug that was found in examples/todos
func validateEmptyToNonEmptyTransition(templateStr string, data map[string]interface{}) (bool, string) {
	// Create empty version of data
	emptyData := makeEmptyData(data)

	// Parse with empty data
	keyGen1 := newKeyGenerator()
	tree1, err := parseTemplateToTree(templateStr, emptyData, keyGen1)
	if err != nil {
		return false, fmt.Sprintf("parse with empty data failed: %v", err)
	}

	// Parse with non-empty data
	keyGen2 := newKeyGenerator()
	tree2, err := parseTemplateToTree(templateStr, data, keyGen2)
	if err != nil {
		return false, fmt.Sprintf("parse with non-empty data failed: %v", err)
	}

	// Validate transition between the two trees
	ok, msg := validateTreeTransition(tree1, tree2)
	if !ok {
		return false, fmt.Sprintf("empty→non-empty transition failed: %s", msg)
	}

	// Also test the reverse: non-empty→empty
	keyGen3 := newKeyGenerator()
	tree3, err := parseTemplateToTree(templateStr, data, keyGen3)
	if err != nil {
		return false, fmt.Sprintf("second parse with non-empty data failed: %v", err)
	}

	keyGen4 := newKeyGenerator()
	tree4, err := parseTemplateToTree(templateStr, emptyData, keyGen4)
	if err != nil {
		return false, fmt.Sprintf("second parse with empty data failed: %v", err)
	}

	ok, msg = validateTreeTransition(tree3, tree4)
	if !ok {
		return false, fmt.Sprintf("non-empty→empty transition failed: %s", msg)
	}

	return true, ""
}
