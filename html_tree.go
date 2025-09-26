package livetemplate

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"html/template"
	"regexp"
	"strings"
)

// HTMLTreeNode represents a complete HTML tree structure with static and dynamic parts
type HTMLTreeNode struct {
	Type       string                 `json:"type,omitempty"`     // "element", "text", "dynamic"
	Tag        string                 `json:"tag,omitempty"`      // HTML tag name (for elements)
	Attrs      map[string]interface{} `json:"attrs,omitempty"`    // Attributes (can be static or dynamic)
	Children   []interface{}          `json:"children,omitempty"` // Child nodes (HTMLTreeNode or string)
	Content    string                 `json:"content,omitempty"`  // Text content (for text nodes)
	Dynamic    bool                   `json:"dynamic,omitempty"`  // Whether this node is dynamic
	Expression string                 `json:"expr,omitempty"`     // Template expression (for dynamic nodes)
	Value      interface{}            `json:"value,omitempty"`    // Evaluated value (for dynamic nodes)
}

// parseHTMLTemplateToFullTree parses an HTML template into a complete tree structure
// that preserves all HTML elements and identifies dynamic parts
func parseHTMLTemplateToFullTree(templateStr string, data interface{}) (*HTMLTreeNode, error) {
	// First, execute the template to get the rendered HTML
	tmpl, err := template.New("temp").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var renderedBuf bytes.Buffer
	if err := tmpl.Execute(&renderedBuf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	renderedHTML := renderedBuf.String()

	// Parse both the template and rendered HTML to build the tree
	tree, err := buildFullHTMLTree(templateStr, renderedHTML, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTML tree: %w", err)
	}

	return tree, nil
}

// buildFullHTMLTree builds a complete HTML tree from template and rendered HTML
func buildFullHTMLTree(templateStr, renderedHTML string, data interface{}) (*HTMLTreeNode, error) {
	// Parse the rendered HTML to get the structure
	doc, err := html.Parse(strings.NewReader(renderedHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find the body element
	body := findBodyElement(doc)
	if body == nil {
		return nil, fmt.Errorf("no body element found")
	}

	// Build tree from the body content
	tree := buildTreeFromNode(body, templateStr, data)
	return tree, nil
}

// findBodyElement finds the body element in an HTML document
func findBodyElement(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findBodyElement(c); result != nil {
			return result
		}
	}
	return nil
}

// buildTreeFromNode builds a tree from an HTML node
func buildTreeFromNode(n *html.Node, templateStr string, data interface{}) *HTMLTreeNode {
	switch n.Type {
	case html.ElementNode:
		node := &HTMLTreeNode{
			Type:  "element",
			Tag:   n.Data,
			Attrs: make(map[string]interface{}),
		}

		// Process attributes
		for _, attr := range n.Attr {
			// Check if attribute contains template expression
			if containsTemplateExpr(attr.Val) {
				node.Attrs[attr.Key] = map[string]interface{}{
					"dynamic": true,
					"expr":    extractTemplateExpr(attr.Val),
					"value":   attr.Val,
				}
			} else {
				node.Attrs[attr.Key] = attr.Val
			}
		}

		// Process children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			child := buildTreeFromNode(c, templateStr, data)
			if child != nil {
				node.Children = append(node.Children, child)
			}
		}

		return node

	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text == "" {
			return nil // Skip empty text nodes
		}

		// Check if text contains template expression
		if containsTemplateExpr(text) {
			return &HTMLTreeNode{
				Type:    "dynamic",
				Content: text,
				Dynamic: true,
				Value:   text,
			}
		}

		return &HTMLTreeNode{
			Type:    "text",
			Content: text,
		}

	default:
		return nil
	}
}

// containsTemplateExpr checks if a string contains template expressions
func containsTemplateExpr(s string) bool {
	// For now, just check if the string was likely generated from a template
	// In a real implementation, we'd need to track the original template
	// This is a simplified check
	return false // Will be enhanced later
}

// extractTemplateExpr extracts template expression from a string
func extractTemplateExpr(s string) string {
	// This would extract the original template expression
	// For now, return empty
	return ""
}

// convertFullTreeToSegmentTree converts a full HTML tree to the segment-based format
// This maintains backward compatibility with the current client expectations
func convertFullTreeToSegmentTree(fullTree *HTMLTreeNode) TreeNode {
	result := make(TreeNode)
	var statics []string
	var dynamics []interface{}

	// Flatten the tree into static and dynamic segments
	flattenTree(fullTree, &statics, &dynamics)

	// Build the segment tree
	if len(statics) > 0 {
		result["s"] = statics
	}

	for i, dynamic := range dynamics {
		result[fmt.Sprintf("%d", i)] = dynamic
	}

	return result
}

// flattenTree flattens a full tree into static and dynamic segments
func flattenTree(node *HTMLTreeNode, statics *[]string, dynamics *[]interface{}) {
	if node == nil {
		return
	}

	switch node.Type {
	case "element":
		// Add opening tag as static
		tag := "<" + node.Tag
		for key, val := range node.Attrs {
			if strVal, ok := val.(string); ok {
				tag += fmt.Sprintf(` %s="%s"`, key, strVal)
			}
		}
		tag += ">"
		*statics = append(*statics, tag)

		// Process children
		for _, child := range node.Children {
			switch c := child.(type) {
			case *HTMLTreeNode:
				flattenTree(c, statics, dynamics)
			case string:
				*statics = append(*statics, c)
			}
		}

		// Add closing tag as static
		*statics = append(*statics, "</"+node.Tag+">")

	case "text":
		*statics = append(*statics, node.Content)

	case "dynamic":
		// Add empty static for this position
		*statics = append(*statics, "")
		// Add dynamic value
		*dynamics = append(*dynamics, node.Value)
	}
}

// Enhanced tree generation that captures full HTML structure
func (t *Template) generateFullHTMLTree(html string, data interface{}) (*HTMLTreeNode, error) {
	// Extract body content from wrapper if present
	// var contentToAnalyze string
	// if t.wrapperID != "" {
	// 	contentToAnalyze = extractTemplateContent(html, t.wrapperID)
	// } else {
	// 	contentToAnalyze = html
	// }

	// Get the original template content
	var templateContent string
	if t.wrapperID != "" {
		templateContent = extractTemplateBodyContent(t.templateStr)
	} else {
		templateContent = t.templateStr
	}

	// Build full HTML tree
	tree, err := parseHTMLTemplateToFullTree(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template to tree: %w", err)
	}

	return tree, nil
}

// templateExpressionRegex matches Go template expressions
var templateExpressionRegex = regexp.MustCompile(`\{\{.*?\}\}`)

// enhancedContainsTemplateExpr checks if a string contains template expressions
func enhancedContainsTemplateExpr(original, rendered string) bool {
	// Check if the original template had expressions at this position
	return templateExpressionRegex.MatchString(original)
}

// mapTemplateToRendered maps template positions to rendered HTML positions
// This helps identify which parts of the rendered HTML came from template expressions
func mapTemplateToRendered(templateStr, renderedHTML string) map[int]int {
	// This is a simplified mapping - a full implementation would need
	// to track positions through template execution
	mapping := make(map[int]int)

	// For now, return empty mapping
	// A real implementation would track how template positions map to output
	return mapping
}
