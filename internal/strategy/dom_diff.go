package strategy

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// DOMDiffer implements Phoenix LiveView-inspired DOM diffing
type DOMDiffer struct {
	// Store rendered HTML for comparison
	lastHTML string
}

// NewDOMDiffer creates a new DOM-based fragment generator
func NewDOMDiffer() *DOMDiffer {
	return &DOMDiffer{}
}

// PatchOperation represents a single DOM modification
type PatchOperation struct {
	Type     string      `json:"type"`            // "setAttribute", "removeAttribute", "setTextContent", "replaceElement"
	Selector string      `json:"selector"`        // CSS-like selector or element path
	Key      string      `json:"key,omitempty"`   // For attributes
	Value    interface{} `json:"value,omitempty"` // New value
	HTML     string      `json:"html,omitempty"`  // For element replacement
}

// DOMPatch represents a collection of operations to update the DOM
type DOMPatch struct {
	FragmentID string           `json:"fragment_id"`
	Operations []PatchOperation `json:"operations"`
}

// GenerateFromTemplateSource creates DOM patches by diffing rendered HTML
func (d *DOMDiffer) GenerateFromTemplateSource(templateSource string, oldData, newData interface{}, fragmentID string) (*DOMPatch, error) {
	// Parse the template
	tmpl, err := template.New("dom_diff").Parse(templateSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	// Render old HTML (or use cached version)
	var oldHTML string
	if d.lastHTML != "" {
		oldHTML = d.lastHTML
	} else if oldData != nil {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, oldData); err != nil {
			return nil, fmt.Errorf("failed to render old template: %v", err)
		}
		oldHTML = buf.String()
	}

	// Render new HTML
	var newBuf bytes.Buffer
	if err := tmpl.Execute(&newBuf, newData); err != nil {
		return nil, fmt.Errorf("failed to render new template: %v", err)
	}
	newHTML := newBuf.String()

	// Store for next comparison
	d.lastHTML = newHTML

	// If this is the first render, return the full HTML as a replacement
	if oldHTML == "" {
		return &DOMPatch{
			FragmentID: fragmentID,
			Operations: []PatchOperation{
				{
					Type:     "replaceElement",
					Selector: "body", // Or could be a specific container
					HTML:     newHTML,
				},
			},
		}, nil
	}

	// Parse both HTML strings into DOM trees
	oldTree, err := d.parseHTML(oldHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old HTML: %v", err)
	}

	newTree, err := d.parseHTML(newHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new HTML: %v", err)
	}

	// Generate diff operations
	operations := d.diffNodes(oldTree, newTree, "")

	return &DOMPatch{
		FragmentID: fragmentID,
		Operations: operations,
	}, nil
}

// parseHTML parses HTML string into a DOM tree
func (d *DOMDiffer) parseHTML(htmlStr string) (*html.Node, error) {
	// Wrap in a container to handle fragments
	wrappedHTML := fmt.Sprintf("<div>%s</div>", htmlStr)
	doc, err := html.Parse(strings.NewReader(wrappedHTML))
	if err != nil {
		return nil, err
	}

	// Find the body > div container
	var container *html.Node
	var findContainer func(*html.Node)
	findContainer = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Div {
			container = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findContainer(c)
			if container != nil {
				return
			}
		}
	}
	findContainer(doc)

	if container == nil {
		return nil, fmt.Errorf("could not find container div")
	}

	return container, nil
}

// diffNodes compares two DOM nodes and generates patch operations
func (d *DOMDiffer) diffNodes(oldNode, newNode *html.Node, pathPrefix string) []PatchOperation {
	var operations []PatchOperation

	if oldNode == nil && newNode == nil {
		return operations
	}

	// Node was removed
	if oldNode != nil && newNode == nil {
		operations = append(operations, PatchOperation{
			Type:     "removeElement",
			Selector: pathPrefix,
		})
		return operations
	}

	// Node was added
	if oldNode == nil && newNode != nil {
		operations = append(operations, PatchOperation{
			Type:     "insertElement",
			Selector: pathPrefix,
			HTML:     d.nodeToHTML(newNode),
		})
		return operations
	}

	// Different node types or tag names - replace entirely
	if oldNode.Type != newNode.Type ||
		(oldNode.Type == html.ElementNode && oldNode.DataAtom != newNode.DataAtom) {
		operations = append(operations, PatchOperation{
			Type:     "replaceElement",
			Selector: pathPrefix,
			HTML:     d.nodeToHTML(newNode),
		})
		return operations
	}

	// Handle text nodes
	if oldNode.Type == html.TextNode {
		if oldNode.Data != newNode.Data {
			operations = append(operations, PatchOperation{
				Type:     "setTextContent",
				Selector: pathPrefix,
				Value:    newNode.Data,
			})
		}
		return operations
	}

	// Handle element nodes - check attributes
	if oldNode.Type == html.ElementNode {
		currentPath := pathPrefix
		if currentPath == "" {
			currentPath = strings.ToLower(oldNode.Data)
		}

		// Compare attributes
		operations = append(operations, d.diffAttributes(oldNode, newNode, currentPath)...)

		// Compare children
		operations = append(operations, d.diffChildren(oldNode, newNode, currentPath)...)
	}

	return operations
}

// diffAttributes compares attributes between two element nodes
func (d *DOMDiffer) diffAttributes(oldNode, newNode *html.Node, selector string) []PatchOperation {
	var operations []PatchOperation

	// Build attribute maps for easier comparison
	oldAttrs := make(map[string]string)
	newAttrs := make(map[string]string)

	for _, attr := range oldNode.Attr {
		oldAttrs[attr.Key] = attr.Val
	}
	for _, attr := range newNode.Attr {
		newAttrs[attr.Key] = attr.Val
	}

	// Check for removed attributes
	for key := range oldAttrs {
		if _, exists := newAttrs[key]; !exists {
			operations = append(operations, PatchOperation{
				Type:     "removeAttribute",
				Selector: selector,
				Key:      key,
			})
		}
	}

	// Check for new or changed attributes
	for key, newVal := range newAttrs {
		if oldVal, exists := oldAttrs[key]; !exists || oldVal != newVal {
			operations = append(operations, PatchOperation{
				Type:     "setAttribute",
				Selector: selector,
				Key:      key,
				Value:    newVal,
			})
		}
	}

	return operations
}

// diffChildren compares child nodes between two elements
func (d *DOMDiffer) diffChildren(oldNode, newNode *html.Node, pathPrefix string) []PatchOperation {
	var operations []PatchOperation

	// Collect children into slices for easier comparison
	var oldChildren []*html.Node
	var newChildren []*html.Node

	for c := oldNode.FirstChild; c != nil; c = c.NextSibling {
		// Skip whitespace-only text nodes for cleaner diffs
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		oldChildren = append(oldChildren, c)
	}

	for c := newNode.FirstChild; c != nil; c = c.NextSibling {
		// Skip whitespace-only text nodes for cleaner diffs
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		newChildren = append(newChildren, c)
	}

	// Simple approach: compare by index
	// TODO: Implement more sophisticated matching (like React's key-based diffing)
	maxLen := len(oldChildren)
	if len(newChildren) > maxLen {
		maxLen = len(newChildren)
	}

	for i := 0; i < maxLen; i++ {
		childPath := fmt.Sprintf("%s > :nth-child(%d)", pathPrefix, i+1)

		var oldChild, newChild *html.Node
		if i < len(oldChildren) {
			oldChild = oldChildren[i]
		}
		if i < len(newChildren) {
			newChild = newChildren[i]
		}

		operations = append(operations, d.diffNodes(oldChild, newChild, childPath)...)
	}

	return operations
}

// nodeToHTML converts a DOM node back to HTML string
func (d *DOMDiffer) nodeToHTML(node *html.Node) string {
	var buf bytes.Buffer
	_ = html.Render(&buf, node)
	return buf.String()
}

// String provides a readable representation of the patch
func (p *DOMPatch) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Patch for fragment: %s", p.FragmentID))

	for i, op := range p.Operations {
		var detail string
		switch op.Type {
		case "setAttribute":
			detail = fmt.Sprintf("Set %s='%v'", op.Key, op.Value)
		case "removeAttribute":
			detail = fmt.Sprintf("Remove %s", op.Key)
		case "setTextContent":
			detail = fmt.Sprintf("Set text: '%v'", op.Value)
		case "replaceElement":
			detail = fmt.Sprintf("Replace with: %s", op.HTML)
		case "insertElement":
			detail = fmt.Sprintf("Insert: %s", op.HTML)
		case "removeElement":
			detail = "Remove element"
		default:
			detail = fmt.Sprintf("Unknown: %v", op.Value)
		}

		lines = append(lines, fmt.Sprintf("  %d. [%s] %s -> %s",
			i+1, op.Type, op.Selector, detail))
	}

	return strings.Join(lines, "\n")
}
