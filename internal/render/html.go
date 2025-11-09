// Package render provides HTML rendering utilities for LiveTemplate.
package render

import (
	"fmt"
	htmlescape "html"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const (
	// keyStatics is the tree key for static HTML fragments.
	keyStatics = "s"
	// keyDynamics is the tree key for dynamic range items.
	keyDynamics = "d"
)

// voidElements is the set of HTML void (self-closing) elements.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// Node recursively renders an HTML node and its children to a string builder.
func Node(w *strings.Builder, n *html.Node) {
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
		if !IsVoidElement(n.Data) {
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				Node(w, child)
			}
			w.WriteString("</")
			w.WriteString(n.Data)
			w.WriteString(">")
		}
	}
}

// IsVoidElement checks if an HTML element is void (self-closing).
func IsVoidElement(tagName string) bool {
	return voidElements[strings.ToLower(tagName)]
}

// TreeToHTML renders a tree structure to HTML (used in tests).
func TreeToHTML(tree map[string]interface{}) (string, error) {
	// Check if this is a range comprehension (has keyDynamics key with items)
	if itemsRaw, hasD := tree[keyDynamics]; hasD {
		return rangeComprehensionToHTML(tree, itemsRaw)
	}

	statics, ok := tree[keyStatics].([]string)
	if !ok || len(statics) == 0 {
		return "", fmt.Errorf("invalid tree: missing or empty statics array")
	}

	var result strings.Builder
	if err := renderTreeWithStatics(statics, tree, &result); err != nil {
		return "", err
	}
	return result.String(), nil
}

// renderTreeWithStatics interleaves static HTML fragments with dynamic values.
// It handles nested trees and properly escapes HTML in dynamic values.
func renderTreeWithStatics(statics []string, dynamics map[string]interface{}, result *strings.Builder) error {
	for i, static := range statics {
		result.WriteString(static)

		// After each static (except the last), add the corresponding dynamic
		if i < len(statics)-1 {
			dynKey := strconv.Itoa(i)
			if dynValue, exists := dynamics[dynKey]; exists {
				// Handle nested trees (like ranges)
				if nestedTree, ok := dynValue.(map[string]interface{}); ok {
					nestedHTML, err := TreeToHTML(nestedTree)
					if err != nil {
						return fmt.Errorf("rendering nested tree at position %d: %w", i, err)
					}
					result.WriteString(nestedHTML)
				} else {
					// Simple value - convert to string and escape HTML
					result.WriteString(htmlescape.EscapeString(fmt.Sprintf("%v", dynValue)))
				}
			}
		}
	}
	return nil
}

// rangeComprehensionToHTML renders a range comprehension tree to HTML.
// A range comprehension tree has the structure:
//
//	{
//	  "s": ["<div>", "</div>"],  // Static template for each item
//	  "d": [                      // Dynamic items array
//	    {"0": "value1", "1": "value2"},
//	    {"0": "value3", "1": "value4"},
//	  ]
//	}
//
// This renders each item using the static template, interleaving dynamic values.
func rangeComprehensionToHTML(tree map[string]interface{}, itemsRaw interface{}) (string, error) {
	// Get statics for the range items
	statics, ok := tree[keyStatics].([]string)
	if !ok {
		return "", fmt.Errorf("range comprehension: missing statics array")
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
		return "", fmt.Errorf("range comprehension: unexpected items type %T", itemsRaw)
	}

	var result strings.Builder

	// Render each item using the statics as template
	for itemIdx, itemRaw := range items {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("range comprehension: item %d is not a map (got %T)", itemIdx, itemRaw)
		}

		if err := renderTreeWithStatics(statics, itemMap, &result); err != nil {
			return "", fmt.Errorf("range comprehension: rendering item %d: %w", itemIdx, err)
		}
	}

	return result.String(), nil
}
