package build

import (
	"fmt"
	htmlescape "html"
	"strings"

	"golang.org/x/net/html"
)

// voidElements is the set of HTML void (self-closing) elements.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// RenderNode recursively renders an HTML node and its children to a string builder.
func RenderNode(w *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		w.WriteString(n.Data)
	case html.ElementNode:
		w.WriteString("<")
		w.WriteString(n.Data)
		for _, attr := range n.Attr {
			w.WriteString(" ")
			w.WriteString(attr.Key)
			w.WriteString(`="`)
			w.WriteString(htmlescape.EscapeString(attr.Val))
			w.WriteString(`"`)
		}
		w.WriteString(">")
		if !IsVoidHTMLElement(n.Data) {
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				RenderNode(w, child)
			}
			w.WriteString("</")
			w.WriteString(n.Data)
			w.WriteString(">")
		}
	}
}

// IsVoidHTMLElement checks if an HTML element is void (self-closing).
func IsVoidHTMLElement(tagName string) bool {
	return voidElements[strings.ToLower(tagName)]
}

// RenderTreeToHTML renders a tree structure to HTML (used in tests).
func RenderTreeToHTML(tree map[string]interface{}) (string, error) {
	// Check if this is a range comprehension (has "d" key with items)
	if itemsRaw, hasD := tree["d"]; hasD {
		return RenderRangeComprehensionToHTML(tree, itemsRaw)
	}

	statics, ok := tree["s"].([]string)
	if !ok || len(statics) == 0 {
		return "", fmt.Errorf("invalid tree: no statics")
	}

	var result strings.Builder

	// Interleave statics and dynamics
	dynamicIndex := 0
	for i, static := range statics {
		result.WriteString(static)

		// After each static (except the last), add the corresponding dynamic
		if i < len(statics)-1 {
			dynKey := fmt.Sprintf("%d", dynamicIndex)
			if dynValue, exists := tree[dynKey]; exists {
				// Handle nested trees (like ranges)
				if nestedTree, ok := dynValue.(map[string]interface{}); ok {
					nestedHTML, err := RenderTreeToHTML(nestedTree)
					if err != nil {
						return "", err
					}
					result.WriteString(nestedHTML)
				} else {
					// Simple value - convert to string and escape HTML
					result.WriteString(htmlescape.EscapeString(fmt.Sprintf("%v", dynValue)))
				}
			}
			dynamicIndex++
		}
	}

	return result.String(), nil
}

// RenderRangeComprehensionToHTML renders a range comprehension (with "d" and "s" keys) to HTML.
func RenderRangeComprehensionToHTML(tree map[string]interface{}, itemsRaw interface{}) (string, error) {
	// Get statics for the range items
	statics, ok := tree["s"].([]string)
	if !ok {
		return "", fmt.Errorf("range comprehension missing statics")
	}

	// Convert items to []interface{}
	var items []interface{}
	switch v := itemsRaw.(type) {
	case []interface{}:
		items = v
	case []map[string]interface{}:
		items = make([]interface{}, len(v))
		for i, item := range v {
			items[i] = item
		}
	default:
		return "", fmt.Errorf("unexpected items type: %T", itemsRaw)
	}

	var result strings.Builder

	// Render each item using the statics as template
	for _, itemRaw := range items {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Interleave statics and item dynamics
		for i, static := range statics {
			result.WriteString(static)

			// After each static (except the last), add the corresponding dynamic
			if i < len(statics)-1 {
				dynKey := fmt.Sprintf("%d", i)
				if dynValue, exists := itemMap[dynKey]; exists {
					// Recursively render nested trees
					if nestedTree, ok := dynValue.(map[string]interface{}); ok {
						nestedHTML, err := RenderTreeToHTML(nestedTree)
						if err != nil {
							return "", err
						}
						result.WriteString(nestedHTML)
					} else {
						// Simple value - convert to string and escape HTML
						result.WriteString(htmlescape.EscapeString(fmt.Sprintf("%v", dynValue)))
					}
				}
			}
		}
	}

	return result.String(), nil
}
