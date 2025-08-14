package diff

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// DOMNode represents a simplified DOM node for comparison
type DOMNode struct {
	Type       html.NodeType
	Data       string
	Attributes map[string]string
	Children   []*DOMNode
	Parent     *DOMNode
}

// DOMParser handles parsing HTML into comparable tree structures
type DOMParser struct{}

// NewDOMParser creates a new DOM parser
func NewDOMParser() *DOMParser {
	return &DOMParser{}
}

// Parse converts HTML string into a DOMNode tree structure
func (p *DOMParser) Parse(htmlContent string) (*DOMNode, error) {
	reader := strings.NewReader(htmlContent)
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return p.convertNode(doc, nil), nil
}

// ParseFragment parses an HTML fragment (without html/body wrapper)
func (p *DOMParser) ParseFragment(htmlContent string) (*DOMNode, error) {
	if strings.TrimSpace(htmlContent) == "" {
		return nil, fmt.Errorf("empty fragment")
	}

	reader := strings.NewReader(htmlContent)
	nodes, err := html.ParseFragment(reader, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML fragment: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found in fragment")
	}

	// Filter out empty text nodes and extract from html/body wrapper if present
	var validNodes []*html.Node
	for _, node := range nodes {
		extractedNodes := p.extractFromWrappers(node)
		for _, extracted := range extractedNodes {
			if extracted.Type == html.TextNode {
				if strings.TrimSpace(extracted.Data) != "" {
					validNodes = append(validNodes, extracted)
				}
			} else {
				validNodes = append(validNodes, extracted)
			}
		}
	}

	if len(validNodes) == 0 {
		return nil, fmt.Errorf("no valid nodes found in fragment")
	}

	// If single node, return it directly
	if len(validNodes) == 1 {
		return p.convertNode(validNodes[0], nil), nil
	}

	// Multiple nodes, wrap in a virtual container
	container := &DOMNode{
		Type:       html.ElementNode,
		Data:       "fragment-container",
		Attributes: make(map[string]string),
		Children:   make([]*DOMNode, 0, len(validNodes)),
	}

	for _, node := range validNodes {
		child := p.convertNode(node, container)
		container.Children = append(container.Children, child)
	}

	return container, nil
}

// extractFromWrappers extracts content from html/body wrappers that html.ParseFragment adds
func (p *DOMParser) extractFromWrappers(node *html.Node) []*html.Node {
	// If this is an html node, look inside it for body content
	if node.Type == html.ElementNode && node.Data == "html" {
		var result []*html.Node
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "body" {
				// Extract body children
				for bodyChild := child.FirstChild; bodyChild != nil; bodyChild = bodyChild.NextSibling {
					result = append(result, bodyChild)
				}
			} else if child.Type == html.ElementNode && child.Data == "head" {
				// Skip head elements
				continue
			} else {
				// Include other direct html children
				result = append(result, child)
			}
		}
		return result
	}

	// If this is a body node, extract its children
	if node.Type == html.ElementNode && node.Data == "body" {
		var result []*html.Node
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			result = append(result, child)
		}
		return result
	}

	// Otherwise return the node itself
	return []*html.Node{node}
}

// convertNode recursively converts html.Node to DOMNode
func (p *DOMParser) convertNode(node *html.Node, parent *DOMNode) *DOMNode {
	domNode := &DOMNode{
		Type:       node.Type,
		Data:       node.Data,
		Attributes: make(map[string]string),
		Children:   make([]*DOMNode, 0),
		Parent:     parent,
	}

	// Copy attributes
	for _, attr := range node.Attr {
		domNode.Attributes[attr.Key] = attr.Val
	}

	// Convert children
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		childNode := p.convertNode(child, domNode)
		domNode.Children = append(domNode.Children, childNode)
	}

	return domNode
}

// NormalizeNode performs normalization to make nodes more comparable
func (p *DOMParser) NormalizeNode(node *DOMNode) *DOMNode {
	if node == nil {
		return nil
	}

	normalized := &DOMNode{
		Type:       node.Type,
		Data:       node.Data,
		Attributes: make(map[string]string),
		Children:   make([]*DOMNode, 0),
		Parent:     node.Parent,
	}

	// Normalize text nodes by trimming whitespace
	if node.Type == html.TextNode {
		normalized.Data = strings.TrimSpace(node.Data)
		// Skip empty text nodes
		if normalized.Data == "" {
			return nil
		}
	}

	// Copy and normalize attributes (sort them for consistent comparison)
	for key, value := range node.Attributes {
		// Skip certain attributes that shouldn't affect comparison
		if key == "id" && strings.HasPrefix(value, "fragment-") {
			continue // Skip auto-generated fragment IDs
		}
		normalized.Attributes[key] = value
	}

	// Recursively normalize children
	for _, child := range node.Children {
		normalizedChild := p.NormalizeNode(child)
		if normalizedChild != nil {
			normalizedChild.Parent = normalized
			normalized.Children = append(normalized.Children, normalizedChild)
		}
	}

	return normalized
}

// GetPath returns a path string for the node (for debugging and identification)
func (node *DOMNode) GetPath() string {
	if node.Parent == nil {
		return node.Data
	}

	parentPath := node.Parent.GetPath()
	if node.Type == html.TextNode {
		return fmt.Sprintf("%s/text()", parentPath)
	}

	// Find position among siblings of same type
	position := 0
	for _, sibling := range node.Parent.Children {
		if sibling == node {
			break
		}
		if sibling.Type == node.Type && sibling.Data == node.Data {
			position++
		}
	}

	if position > 0 {
		return fmt.Sprintf("%s/%s[%d]", parentPath, node.Data, position+1)
	}
	return fmt.Sprintf("%s/%s", parentPath, node.Data)
}

// IsElementNode returns true if this is an element node
func (node *DOMNode) IsElementNode() bool {
	return node.Type == html.ElementNode
}

// IsTextNode returns true if this is a text node
func (node *DOMNode) IsTextNode() bool {
	return node.Type == html.TextNode
}

// HasAttribute checks if the node has a specific attribute
func (node *DOMNode) HasAttribute(key string) bool {
	_, exists := node.Attributes[key]
	return exists
}

// GetAttribute returns the value of an attribute
func (node *DOMNode) GetAttribute(key string) string {
	return node.Attributes[key]
}

// GetTextContent returns the concatenated text content of the node and its children
func (node *DOMNode) GetTextContent() string {
	if node.IsTextNode() {
		return node.Data
	}

	var text strings.Builder
	for _, child := range node.Children {
		text.WriteString(child.GetTextContent())
	}
	return text.String()
}
