package livetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
)

// FullTreeNode represents a complete tree with HTML structure preserved
type FullTreeNode struct {
	Type     string        `json:"t,omitempty"` // "e" (element), "t" (text), "d" (dynamic)
	Tag      string        `json:"tag,omitempty"`
	Attrs    [][]string    `json:"a,omitempty"`    // [[key, value], ...]
	Children []interface{} `json:"c,omitempty"`    // Can be FullTreeNode or string
	Static   string        `json:"s,omitempty"`    // Static text content
	Dynamic  interface{}   `json:"d,omitempty"`    // Dynamic value
	Template string        `json:"tmpl,omitempty"` // Original template expression
}

// ParseTemplateToFullTree parses a template string into a complete tree structure
// that preserves HTML structure and separates static from dynamic content
func ParseTemplateToFullTree(templateStr string, data interface{}) (*FullTreeNode, error) {
	// Normalize template spacing to handle formatter-added spaces
	templateStr = normalizeTemplateSpacing(templateStr)

	// Parse and execute the template to get values
	tmpl, err := template.New("temp").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}
	renderedHTML := buf.String()

	// Build a mapping of template expressions to their rendered values
	exprMap := buildExpressionMapping(templateStr, renderedHTML, data)

	// Parse the template structure
	root, err := parseTemplateStructure(templateStr, exprMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template structure: %w", err)
	}

	return root, nil
}

// parseTemplateStructure parses template HTML into a tree structure
func parseTemplateStructure(templateStr string, exprMap map[string]interface{}) (*FullTreeNode, error) {
	// For the body content, create a root node
	root := &FullTreeNode{
		Type:     "e",
		Tag:      "root",
		Children: []interface{}{},
	}

	// Parse the template content
	pos := 0
	stack := []*FullTreeNode{root}

	for pos < len(templateStr) {
		// Look for next HTML tag or template expression
		nextTag := strings.Index(templateStr[pos:], "<")
		nextExpr := strings.Index(templateStr[pos:], "{{")

		// Process text before any tag/expression
		var textEnd int
		if nextTag == -1 && nextExpr == -1 {
			textEnd = len(templateStr)
		} else if nextTag == -1 {
			textEnd = pos + nextExpr
		} else if nextExpr == -1 {
			textEnd = pos + nextTag
		} else {
			textEnd = pos + min(nextTag, nextExpr)
		}

		if textEnd > pos {
			text := templateStr[pos:textEnd]
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				current := stack[len(stack)-1]
				current.Children = append(current.Children, &FullTreeNode{
					Type:   "t",
					Static: text,
				})
			}
			pos = textEnd
		}

		if pos >= len(templateStr) {
			break
		}

		// Process tag or expression
		if strings.HasPrefix(templateStr[pos:], "{{") {
			// Template expression
			end := strings.Index(templateStr[pos:], "}}")
			if end == -1 {
				return nil, fmt.Errorf("unclosed template expression at position %d", pos)
			}
			end += pos + 2

			expr := templateStr[pos:end]
			current := stack[len(stack)-1]

			// Check if this is a control structure
			if strings.HasPrefix(expr, "{{if ") || strings.HasPrefix(expr, "{{range ") {
				// Find the matching end
				endPos := findMatchingEndForTemplate(templateStr, pos, expr)
				if endPos == -1 {
					return nil, fmt.Errorf("no matching end for %s", expr)
				}

				// Process the entire block as a dynamic node
				// blockContent := templateStr[pos:endPos] // May use later for block parsing
				node := &FullTreeNode{
					Type:     "d",
					Template: expr,
					Dynamic:  exprMap[expr],
				}

				current.Children = append(current.Children, node)
				pos = endPos
			} else {
				// Simple expression
				node := &FullTreeNode{
					Type:     "d",
					Template: expr,
					Dynamic:  exprMap[expr],
				}
				current.Children = append(current.Children, node)
				pos = end
			}
		} else if strings.HasPrefix(templateStr[pos:], "<") {
			// HTML tag
			if strings.HasPrefix(templateStr[pos:], "</") {
				// Closing tag
				end := strings.Index(templateStr[pos:], ">")
				if end == -1 {
					return nil, fmt.Errorf("unclosed tag at position %d", pos)
				}
				end += pos + 1

				// Pop from stack
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
				pos = end
			} else if strings.HasPrefix(templateStr[pos:], "<!") {
				// DOCTYPE or comment - skip
				end := strings.Index(templateStr[pos:], ">")
				if end == -1 {
					pos = len(templateStr)
				} else {
					pos = pos + end + 1
				}
			} else {
				// Opening tag
				tagEnd := strings.Index(templateStr[pos:], ">")
				if tagEnd == -1 {
					return nil, fmt.Errorf("unclosed tag at position %d", pos)
				}
				tagEnd += pos + 1

				tagStr := templateStr[pos:tagEnd]
				tagName, attrs, selfClosing := parseTag(tagStr)

				node := &FullTreeNode{
					Type:     "e",
					Tag:      tagName,
					Attrs:    attrs,
					Children: []interface{}{},
				}

				current := stack[len(stack)-1]
				current.Children = append(current.Children, node)

				// If not self-closing and not void element, push to stack
				if !selfClosing && !isVoidElement(tagName) {
					stack = append(stack, node)
				}

				pos = tagEnd
			}
		} else {
			pos++
		}
	}

	return root, nil
}

// parseTag parses an HTML tag string to extract tag name and attributes
func parseTag(tagStr string) (string, [][]string, bool) {
	tagStr = strings.TrimPrefix(tagStr, "<")
	tagStr = strings.TrimSuffix(tagStr, ">")
	selfClosing := strings.HasSuffix(tagStr, "/")
	if selfClosing {
		tagStr = strings.TrimSuffix(tagStr, "/")
	}

	parts := strings.Fields(tagStr)
	if len(parts) == 0 {
		return "", nil, selfClosing
	}

	tagName := strings.ToLower(parts[0])
	var attrs [][]string

	// Simple attribute parsing
	for i := 1; i < len(parts); i++ {
		if strings.Contains(parts[i], "=") {
			kv := strings.SplitN(parts[i], "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				val := strings.Trim(kv[1], `"'`)
				attrs = append(attrs, []string{key, val})
			}
		}
	}

	return tagName, attrs, selfClosing
}

// isVoidElement checks if an element is a void element (self-closing in HTML)
func isVoidElement(tag string) bool {
	voidElements := map[string]bool{
		"area": true, "base": true, "br": true, "col": true,
		"embed": true, "hr": true, "img": true, "input": true,
		"link": true, "meta": true, "param": true, "source": true,
		"track": true, "wbr": true,
	}
	return voidElements[tag]
}

// findMatchingEndForTemplate finds the matching {{end}} for a control structure
func findMatchingEndForTemplate(templateStr string, start int, openingExpr string) int {
	depth := 1
	pos := start + len(openingExpr)

	for pos < len(templateStr) {
		nextOpen := strings.Index(templateStr[pos:], "{{if ")
		if nextOpen == -1 {
			nextOpen = strings.Index(templateStr[pos:], "{{range ")
		}
		nextEnd := strings.Index(templateStr[pos:], "{{end}}")

		if nextEnd == -1 {
			return -1
		}

		if nextOpen != -1 && nextOpen < nextEnd {
			depth++
			pos = pos + nextOpen + 5
		} else {
			depth--
			if depth == 0 {
				return pos + nextEnd + 7
			}
			pos = pos + nextEnd + 7
		}
	}

	return -1
}

// buildExpressionMapping builds a map of template expressions to their rendered values
func buildExpressionMapping(templateStr, renderedHTML string, data interface{}) map[string]interface{} {
	mapping := make(map[string]interface{})

	// Execute the template to get the full rendered output
	tmpl, err := template.New("mapping").Parse(templateStr)
	if err != nil {
		return mapping
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return mapping
	}
	fullRendered := buf.String()

	// Find all template expressions and try to map them to rendered content
	re := regexp.MustCompile(`\{\{[^}]+\}\}`)
	matches := re.FindAllString(templateStr, -1)

	// For each expression, evaluate it and map to rendered value
	for _, expr := range matches {
		// Skip control structure keywords but include their content
		cleaned := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(expr, "}}"), "{{"))
		if strings.HasPrefix(cleaned, "range ") || strings.HasPrefix(cleaned, "with ") ||
			cleaned == "else" || cleaned == "end" {
			continue
		}

		// For if statements, we want the evaluated result, not the expression itself
		if strings.HasPrefix(cleaned, "if ") {
			value := evaluateExpression(expr, data)
			mapping[expr] = value
		} else {
			// Simple expressions
			value := evaluateExpression(expr, data)
			mapping[expr] = value
		}
	}

	// Generic approach: extract dynamic content from rendered HTML
	// This should work with any template structure, not hardcoded field names
	_ = fullRendered // Future: implement generic dynamic content extraction

	return mapping
}

// evaluateExpression evaluates a template expression with given data
func evaluateExpression(expr string, data interface{}) interface{} {
	// Clean the expression
	cleaned := strings.TrimPrefix(strings.TrimSuffix(expr, "}}"), "{{")
	cleaned = strings.TrimSpace(cleaned)

	// For complex expressions, we need to execute them properly
	// Create a temporary template to evaluate just this expression
	tmplStr := expr
	tmpl, err := template.New("expr").Parse(tmplStr)
	if err != nil {
		return expr // Return original on error
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return expr // Return original on error
	}

	result := buf.String()

	// For template control structures, return the original expression
	if strings.HasPrefix(cleaned, "if ") || strings.HasPrefix(cleaned, "range ") ||
		strings.HasPrefix(cleaned, "else") || cleaned == "end" {
		return expr
	}

	return result
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ConvertFullTreeToSimpleTree converts a full tree to the simple tree format
// This provides backward compatibility with the current client
func ConvertFullTreeToSimpleTree(fullTree *FullTreeNode) TreeNode {
	result := make(TreeNode)
	var statics []string
	dynamics := make(map[string]interface{})
	dynamicIndex := 0

	// Process the tree to extract statics and dynamics
	processNode(fullTree, &statics, dynamics, &dynamicIndex)

	// Build result
	if len(statics) > 0 {
		result["s"] = statics
	}

	// Sort keys to ensure deterministic output
	keys := make([]string, 0, len(dynamics))
	for k := range dynamics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		result[k] = dynamics[k]
	}

	return result
}

// processNode processes a node to extract static and dynamic parts
// maintaining the invariant: len(statics) == len(dynamics) + 1
func processNode(node *FullTreeNode, statics *[]string, dynamics map[string]interface{}, index *int) {
	if node == nil {
		return
	}

	switch node.Type {
	case "e": // Element
		// Build opening tag
		tag := "<" + node.Tag
		for _, attr := range node.Attrs {
			if len(attr) >= 2 {
				tag += fmt.Sprintf(` %s="%s"`, attr[0], attr[1])
			}
		}
		tag += ">"

		// Append to the last static segment instead of creating a new one
		if len(*statics) > 0 {
			(*statics)[len(*statics)-1] += tag
		} else {
			*statics = append(*statics, tag)
		}

		// Process children
		for _, child := range node.Children {
			switch c := child.(type) {
			case *FullTreeNode:
				processNode(c, statics, dynamics, index)
			case string:
				// Append to the last static segment
				if len(*statics) > 0 {
					(*statics)[len(*statics)-1] += c
				} else {
					*statics = append(*statics, c)
				}
			}
		}

		// Closing tag - append to last static segment
		if !isVoidElement(node.Tag) {
			if len(*statics) > 0 {
				(*statics)[len(*statics)-1] += "</" + node.Tag + ">"
			} else {
				*statics = append(*statics, "</"+node.Tag+">")
			}
		}

	case "t": // Text
		// Append to the last static segment
		if len(*statics) > 0 {
			(*statics)[len(*statics)-1] += node.Static
		} else {
			*statics = append(*statics, node.Static)
		}

	case "d": // Dynamic
		// Add a new static segment for content that comes after this dynamic
		dynamics[fmt.Sprintf("%d", *index)] = node.Dynamic
		*statics = append(*statics, "") // This will be the static content after this dynamic
		*index++
	}
}
