package strategy

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
)

// treeNode represents the tree-based static/dynamic structure
type treeNode map[string]interface{}

// parseTemplateToTree parses a Go template and data into a tree-based structure
func parseTemplateToTree(templateStr string, data interface{}) (treeNode, error) {
	// Try the original fine-grained approach first
	tree, err := parseWithOriginalApproach(templateStr, data)
	if err != nil {
		// If original approach fails (likely due to variables), fall back to tree-based differ
		differ, diffErr := newInternalDiffer(templateStr)
		if diffErr != nil {
			return nil, fmt.Errorf("both approaches failed - original: %w, diff: %w", err, diffErr)
		}
		return differ.generateTreeInternal(data)
	}
	return tree, nil
}

// parseWithOriginalApproach uses the original fine-grained static/dynamic separation
func parseWithOriginalApproach(templateStr string, data interface{}) (treeNode, error) {
	// Step 1: Parse template to identify static/dynamic parts by position
	staticParts, dynamicExprs := splitTemplateByExpressions(templateStr)

	if len(dynamicExprs) == 0 {
		// No dynamic content, return as single static part
		tmpl, err := template.New("test").Parse(templateStr)
		if err != nil {
			return nil, fmt.Errorf("template parse error: %w", err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("template execute error: %w", err)
		}
		return treeNode{"s": []string{buf.String()}}, nil
	}

	// Step 2: Evaluate each dynamic expression against data
	tree := treeNode{"s": staticParts}

	for i, expr := range dynamicExprs {
		value, err := evaluateTemplateExpression(expr, data)
		if err != nil {
			// Return error to trigger fallback to diff-based approach
			return nil, fmt.Errorf("error evaluating expression %s: %w", expr, err)
		}
		tree[strconv.Itoa(i)] = value
	}

	return tree, nil
}

// parseTemplateToTreeWithDiff generates tree structure by diffing old and new data
// This is used for fragment updates where we have previous state
func parseTemplateToTreeWithDiff(templateStr string, oldData, newData interface{}) (treeNode, error) {
	// For fragment updates, try to detect if we can use fine-grained approach
	// by comparing if both old and new can be parsed with original approach
	_, oldErr := parseWithOriginalApproach(templateStr, oldData)
	newTree, newErr := parseWithOriginalApproach(templateStr, newData)
	
	if oldErr == nil && newErr == nil {
		// Both can be parsed with original approach, return the new tree
		// This preserves fine-grained static/dynamic separation for fragments
		return newTree, nil
	}
	
	// If either fails, fall back to tree-based differ
	differ, err := newInternalDiffer(templateStr)
	if err != nil {
		return nil, err
	}
	// Prime the differ with old data
	_, err = differ.GenerateTree(oldData)
	if err != nil {
		return nil, err
	}
	// Generate tree with new data (will use diff internally)
	return differ.generateTreeInternal(newData)
}

// reconstructHTML rebuilds HTML from tree structure
func reconstructHTML(tree treeNode) string {
	return reconstructNode(tree)
}

func reconstructNode(node interface{}) string {
	switch v := node.(type) {
	case string:
		return v
	case treeNode:
		// Try []string first
		if statics, ok := v["s"].([]string); ok {
			var html strings.Builder
			for i, static := range statics {
				html.WriteString(static)

				// Add dynamic part if it exists
				if dynValue, exists := v[strconv.Itoa(i)]; exists {
					html.WriteString(reconstructNode(dynValue))
				}
			}
			return html.String()
		}
		
		// Try []interface{} (from JSON unmarshaling)
		if staticsInterface, ok := v["s"].([]interface{}); ok {
			var html strings.Builder
			for i, staticInterface := range staticsInterface {
				if static, ok := staticInterface.(string); ok {
					html.WriteString(static)

					// Add dynamic part if it exists
					if dynValue, exists := v[strconv.Itoa(i)]; exists {
						html.WriteString(reconstructNode(dynValue))
					}
				}
			}
			return html.String()
		}

		return ""
	case []interface{}:
		// Array of nodes (from range loops)
		var result strings.Builder
		for _, item := range v {
			result.WriteString(reconstructNode(item))
		}
		return result.String()
	case map[string]interface{}:
		// This handles treeNode from JSON unmarshaling (treeNode is an alias for map[string]interface{})
		return reconstructNode(treeNode(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// splitTemplateByExpressions splits template into alternating static/dynamic parts
func splitTemplateByExpressions(templateStr string) ([]string, []string) {
	// Find template constructs that span multiple expressions
	constructs := findTemplateConstructs(templateStr)

	if len(constructs) == 0 {
		return []string{templateStr}, []string{}
	}

	var staticParts []string
	var dynamicExprs []string

	lastEnd := 0
	for _, construct := range constructs {
		// Add static part before this construct
		beforePart := templateStr[lastEnd:construct.Start]
		staticParts = append(staticParts, beforePart)

		// Add complete dynamic construct
		dynamicPart := templateStr[construct.Start:construct.End]
		dynamicExprs = append(dynamicExprs, dynamicPart)

		lastEnd = construct.End
	}

	// Add final static part
	afterPart := templateStr[lastEnd:]
	staticParts = append(staticParts, afterPart)

	return staticParts, dynamicExprs
}

// templateConstruct represents a complete template construct (field, conditional, range, etc.)
type templateConstruct struct {
	Start int
	End   int
	Type  string // "field", "conditional", "range", "function"
}

// findTemplateConstructs finds complete template constructs including multi-part ones
func findTemplateConstructs(templateStr string) []templateConstruct {
	var constructs []templateConstruct

	// Find all individual {{...}} expressions first
	re := regexp.MustCompile(`\{\{[^}]*\}\}`)
	matches := re.FindAllStringSubmatchIndex(templateStr, -1)

	i := 0
	for i < len(matches) {
		match := matches[i]
		expr := templateStr[match[0]:match[1]]

		if strings.HasPrefix(expr, "{{if ") || strings.HasPrefix(expr, "{{with ") {
			// Find matching {{end}}
			endIdx := findMatchingEnd(matches, i, templateStr)
			if endIdx != -1 {
				constructs = append(constructs, templateConstruct{
					Start: match[0],
					End:   matches[endIdx][1],
					Type:  "conditional",
				})
				i = endIdx + 1
			} else {
				// Treat as simple field if no matching end
				constructs = append(constructs, templateConstruct{
					Start: match[0],
					End:   match[1],
					Type:  "field",
				})
				i++
			}
		} else if strings.HasPrefix(expr, "{{range ") {
			// Find matching {{end}}
			endIdx := findMatchingEnd(matches, i, templateStr)
			if endIdx != -1 {
				constructs = append(constructs, templateConstruct{
					Start: match[0],
					End:   matches[endIdx][1],
					Type:  "range",
				})
				i = endIdx + 1
			} else {
				// Treat as simple field if no matching end
				constructs = append(constructs, templateConstruct{
					Start: match[0],
					End:   match[1],
					Type:  "field",
				})
				i++
			}
		} else if strings.HasPrefix(expr, "{{else}}") || strings.HasPrefix(expr, "{{end}}") {
			// Skip - these are handled as part of constructs above
			i++
		} else {
			// Simple field or function
			constructs = append(constructs, templateConstruct{
				Start: match[0],
				End:   match[1],
				Type:  "field",
			})
			i++
		}
	}

	return constructs
}

// findMatchingEnd finds the matching {{end}} for a {{if}} or {{range}}
func findMatchingEnd(matches [][]int, startIdx int, templateStr string) int {
	depth := 1

	for i := startIdx + 1; i < len(matches); i++ {
		match := matches[i]
		expr := templateStr[match[0]:match[1]]

		if strings.HasPrefix(expr, "{{if ") || strings.HasPrefix(expr, "{{range ") || strings.HasPrefix(expr, "{{with ") {
			depth++
		} else if strings.HasPrefix(expr, "{{end}}") {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1 // No matching end found
}

// evaluateTemplateExpression evaluates a single template expression against data
func evaluateTemplateExpression(expr string, data interface{}) (interface{}, error) {
	// Check if this is a range construct that needs special handling
	if strings.HasPrefix(expr, "{{range ") {
		// Detect complex range constructs that should fall back to diff-based approach
		if strings.Contains(expr, "{{else}}") || strings.Contains(expr, "$") {
			// Complex range with else clause or variables - force fallback
			return nil, fmt.Errorf("complex range construct not supported by fine-grained approach")
		}
		return evaluateRangeExpression(expr, data)
	}

	// For non-range expressions, evaluate normally
	tmpl, err := template.New("expr").Parse(expr)
	if err != nil {
		return "", fmt.Errorf("parse expression error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute expression error: %w", err)
	}

	return buf.String(), nil
}

// evaluateRangeExpression handles range constructs by parsing them into arrays of tree nodes
func evaluateRangeExpression(rangeExpr string, data interface{}) (interface{}, error) {
	// Parse the range expression to extract the field path and template content
	fieldPath, templateContent, err := parseRangeExpression(rangeExpr)
	if err != nil {
		return nil, err
	}

	// Get the array data to iterate over
	arrayData, err := extractFieldValue(fieldPath, data)
	if err != nil {
		return []interface{}{}, nil // Return empty array if field doesn't exist
	}

	// Handle empty arrays
	arraySlice, ok := arrayData.([]interface{})
	if !ok {
		// Try to convert different slice types
		if convertedSlice, converted := convertToInterfaceSlice(arrayData); converted {
			arraySlice = convertedSlice
		} else {
			return []interface{}{}, nil
		}
	}

	if len(arraySlice) == 0 {
		return []interface{}{}, nil
	}

	// Parse the template content for each item
	var result []interface{}
	for _, item := range arraySlice {
		// Parse the template content as a sub-tree
		itemTree, err := parseTemplateToTree(templateContent, item)
		if err != nil {
			return nil, fmt.Errorf("error parsing range item template: %w", err)
		}
		result = append(result, itemTree)
	}

	return result, nil
}

// parseRangeExpression extracts field path and template content from range expression
func parseRangeExpression(rangeExpr string) (string, string, error) {
	// First extract the field path from the opening {{range}}
	startRe := regexp.MustCompile(`\{\{range\s+([^}]+)\}\}`)
	matches := startRe.FindStringSubmatch(rangeExpr)
	if len(matches) < 2 {
		return "", "", fmt.Errorf("invalid range expression: no opening {{range}}")
	}

	rangeClause := strings.TrimSpace(matches[1])
	
	// Parse range clause to extract the actual field path
	// Handles: .Items, $v := .Items, $i, $v := .Items
	var fieldPath string
	if strings.Contains(rangeClause, ":=") {
		// Variable assignment form
		parts := strings.Split(rangeClause, ":=")
		fieldPath = strings.TrimSpace(parts[len(parts)-1])
	} else {
		// Direct field form
		fieldPath = rangeClause
	}

	// Find all {{...}} expressions to properly handle nested structures
	allExprRe := regexp.MustCompile(`\{\{[^}]*\}\}`)
	allMatches := allExprRe.FindAllStringSubmatchIndex(rangeExpr, -1)

	if len(allMatches) < 2 {
		return "", "", fmt.Errorf("invalid range expression: missing {{end}}")
	}

	// Find the matching {{end}} for the opening {{range}}
	depth := 0
	var endIndex int

	for _, match := range allMatches {
		expr := rangeExpr[match[0]:match[1]]

		if strings.HasPrefix(expr, "{{range ") || strings.HasPrefix(expr, "{{if ") || strings.HasPrefix(expr, "{{with ") {
			depth++
		} else if strings.HasPrefix(expr, "{{end}}") {
			depth--
			if depth == 0 {
				endIndex = match[0]
				break
			}
		}
	}

	if depth != 0 {
		return "", "", fmt.Errorf("invalid range expression: unmatched {{range}}")
	}

	// Extract template content between {{range ...}} and matching {{end}}
	startOfContent := allMatches[0][1] // End of first {{range ...}}
	templateContent := rangeExpr[startOfContent:endIndex]

	return fieldPath, templateContent, nil
}

// extractFieldValue extracts field value from data using field path like ".Items"
func extractFieldValue(fieldPath string, data interface{}) (interface{}, error) {
	if !strings.HasPrefix(fieldPath, ".") {
		return nil, fmt.Errorf("invalid field path: %s", fieldPath)
	}

	fieldName := strings.TrimPrefix(fieldPath, ".")

	// Handle data as map
	if dataMap, ok := data.(map[string]interface{}); ok {
		if value, exists := dataMap[fieldName]; exists {
			return value, nil
		}
	}

	return nil, fmt.Errorf("field not found: %s", fieldPath)
}

// convertToInterfaceSlice converts various slice types to []interface{}
func convertToInterfaceSlice(data interface{}) ([]interface{}, bool) {
	switch v := data.(type) {
	case []string:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	case []map[string]interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	default:
		return nil, false
	}
}

// No marker-based functions needed - using direct template evaluation

// compareTreeNodes compares two tree structures for equality
func compareTreeNodes(a, b treeNode) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valueA := range a {
		valueB, exists := b[key]
		if !exists {
			return false
		}

		if !compareValues(valueA, valueB) {
			return false
		}
	}

	return true
}

func compareValues(a, b interface{}) bool {
	// Handle different types
	switch va := a.(type) {
	case treeNode:
		if vb, ok := b.(treeNode); ok {
			return compareTreeNodes(va, vb)
		}
		return false
	case map[string]interface{}:
		if vb, ok := b.(map[string]interface{}); ok {
			return compareTreeNodes(treeNode(va), treeNode(vb))
		}
		return false
	case []interface{}:
		if vb, ok := b.([]interface{}); ok {
			if len(va) != len(vb) {
				return false
			}
			for i := range va {
				if !compareValues(va[i], vb[i]) {
					return false
				}
			}
			return true
		}
		// Also handle []string (reverse comparison)
		if vb, ok := b.([]string); ok {
			if len(va) != len(vb) {
				return false
			}
			for i := range va {
				if vaStr, ok := va[i].(string); ok {
					if vaStr != vb[i] {
						return false
					}
				} else {
					return false
				}
			}
			return true
		}
		return false
	case []string:
		if vb, ok := b.([]string); ok {
			if len(va) != len(vb) {
				return false
			}
			for i := range va {
				if va[i] != vb[i] {
					return false
				}
			}
			return true
		}
		// Also handle []interface{} (from JSON unmarshaling)
		if vb, ok := b.([]interface{}); ok {
			if len(va) != len(vb) {
				return false
			}
			for i := range va {
				if vbStr, ok := vb[i].(string); ok {
					if va[i] != vbStr {
						return false
					}
				} else {
					return false
				}
			}
			return true
		}
		return false
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}
