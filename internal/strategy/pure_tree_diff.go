package strategy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
)

// PureTreeDiff generates minimal tree-based updates by diffing rendered HTML
// NO HTML intrinsics knowledge, only sends actual changes
type PureTreeDiff struct {
	lastTree *html.Node
	lastHTML string
}

// NewPureTreeDiff creates a new pure tree differ
func NewPureTreeDiff() *PureTreeDiff {
	return &PureTreeDiff{}
}

// MinimalTreeUpdate contains only the changes needed for update
type MinimalTreeUpdate struct {
	FragmentID string       `json:"fragment_id"`
	Type       string       `json:"type"` // "full", "partial", "none"
	Changes    []TreeChange `json:"changes,omitempty"`
	FullHTML   string       `json:"full_html,omitempty"` // Only for first render
}

// TreeChange represents a single change in the tree
type TreeChange struct {
	Path     []int  `json:"path"`                // Path to node [0,1,2] means root->child[0]->child[1]->child[2]
	Type     string `json:"type"`                // "text", "attr", "add", "remove", "replace"
	Key      string `json:"key,omitempty"`       // For attributes
	Value    string `json:"value,omitempty"`     // New value
	OldValue string `json:"old_value,omitempty"` // For debugging
	HTML     string `json:"html,omitempty"`      // For add/replace operations
}

// GenerateMinimalUpdate creates the smallest possible update by diffing HTML trees
func (d *PureTreeDiff) GenerateMinimalUpdate(templateSource string, oldData, newData any, fragmentID string) (*MinimalTreeUpdate, error) {
	// Parse and render the template with new data
	tmpl, err := template.New("pure_diff").Parse(templateSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	var newBuf bytes.Buffer
	if err := tmpl.Execute(&newBuf, newData); err != nil {
		return nil, fmt.Errorf("failed to render template: %v", err)
	}
	newHTML := newBuf.String()

	// Parse new HTML into tree
	newTree, err := d.parseHTMLToTree(newHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new HTML: %v", err)
	}

	// First render - send full HTML
	if d.lastTree == nil {
		d.lastTree = newTree
		d.lastHTML = newHTML
		return &MinimalTreeUpdate{
			FragmentID: fragmentID,
			Type:       "full",
			FullHTML:   newHTML,
		}, nil
	}

	// Check if HTML is identical (no changes)
	if d.lastHTML == newHTML {
		return &MinimalTreeUpdate{
			FragmentID: fragmentID,
			Type:       "none",
		}, nil
	}

	// Generate minimal diff between trees
	changes := d.diffTrees(d.lastTree, newTree, []int{})

	// Store new state
	d.lastTree = newTree
	d.lastHTML = newHTML

	return &MinimalTreeUpdate{
		FragmentID: fragmentID,
		Type:       "partial",
		Changes:    changes,
	}, nil
}

// parseHTMLToTree parses HTML string into a DOM tree
func (d *PureTreeDiff) parseHTMLToTree(htmlStr string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}

	// Find the body node (or first element for fragments)
	var findFirstElement func(*html.Node) *html.Node
	findFirstElement = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findFirstElement(c); result != nil {
				return result
			}
		}
		return nil
	}

	return findFirstElement(doc), nil
}

// diffTrees compares two trees and returns minimal changes
func (d *PureTreeDiff) diffTrees(oldNode, newNode *html.Node, path []int) []TreeChange {
	var changes []TreeChange

	// Both nil - no changes
	if oldNode == nil && newNode == nil {
		return changes
	}

	// Node removed
	if oldNode != nil && newNode == nil {
		return []TreeChange{{
			Path: path,
			Type: "remove",
		}}
	}

	// Node added
	if oldNode == nil && newNode != nil {
		return []TreeChange{{
			Path: path,
			Type: "add",
			HTML: d.nodeToHTML(newNode),
		}}
	}

	// Different node types - replace
	if oldNode.Type != newNode.Type {
		return []TreeChange{{
			Path: path,
			Type: "replace",
			HTML: d.nodeToHTML(newNode),
		}}
	}

	// Text nodes - check if text changed
	if oldNode.Type == html.TextNode {
		oldText := strings.TrimSpace(oldNode.Data)
		newText := strings.TrimSpace(newNode.Data)
		if oldText != newText {
			changes = append(changes, TreeChange{
				Path:     path,
				Type:     "text",
				Value:    newText,
				OldValue: oldText, // For debugging
			})
		}
		return changes
	}

	// Element nodes - check tag and attributes
	if oldNode.Type == html.ElementNode {
		// Different tags - replace entire element
		if oldNode.Data != newNode.Data {
			return []TreeChange{{
				Path: path,
				Type: "replace",
				HTML: d.nodeToHTML(newNode),
			}}
		}

		// Check attributes
		changes = append(changes, d.diffAttributes(oldNode, newNode, path)...)

		// Check children
		changes = append(changes, d.diffChildren(oldNode, newNode, path)...)
	}

	return changes
}

// diffAttributes finds attribute changes
func (d *PureTreeDiff) diffAttributes(oldNode, newNode *html.Node, path []int) []TreeChange {
	var changes []TreeChange

	oldAttrs := make(map[string]string)
	newAttrs := make(map[string]string)

	for _, attr := range oldNode.Attr {
		oldAttrs[attr.Key] = attr.Val
	}
	for _, attr := range newNode.Attr {
		newAttrs[attr.Key] = attr.Val
	}

	// Find changes
	for key, newVal := range newAttrs {
		if oldVal, exists := oldAttrs[key]; !exists || oldVal != newVal {
			changes = append(changes, TreeChange{
				Path:     path,
				Type:     "attr",
				Key:      key,
				Value:    newVal,
				OldValue: oldVal,
			})
		}
	}

	// Find removed attributes
	for key := range oldAttrs {
		if _, exists := newAttrs[key]; !exists {
			changes = append(changes, TreeChange{
				Path:  path,
				Type:  "attr",
				Key:   key,
				Value: "", // Empty means remove
			})
		}
	}

	return changes
}

// diffChildren compares child nodes
func (d *PureTreeDiff) diffChildren(oldNode, newNode *html.Node, path []int) []TreeChange {
	var changes []TreeChange

	// Get meaningful children (skip whitespace-only text nodes)
	oldChildren := d.getMeaningfulChildren(oldNode)
	newChildren := d.getMeaningfulChildren(newNode)

	maxLen := len(oldChildren)
	if len(newChildren) > maxLen {
		maxLen = len(newChildren)
	}

	for i := 0; i < maxLen; i++ {
		childPath := append(append([]int{}, path...), i)

		var oldChild, newChild *html.Node
		if i < len(oldChildren) {
			oldChild = oldChildren[i]
		}
		if i < len(newChildren) {
			newChild = newChildren[i]
		}

		changes = append(changes, d.diffTrees(oldChild, newChild, childPath)...)
	}

	return changes
}

// getMeaningfulChildren returns children excluding whitespace-only text nodes
func (d *PureTreeDiff) getMeaningfulChildren(node *html.Node) []*html.Node {
	var children []*html.Node
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		// Skip whitespace-only text nodes
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		children = append(children, c)
	}
	return children
}

// nodeToHTML converts a node to HTML string
func (d *PureTreeDiff) nodeToHTML(node *html.Node) string {
	var buf bytes.Buffer
	_ = html.Render(&buf, node)
	return buf.String()
}

// GetUpdateSize calculates the size of the update in bytes
func (u *MinimalTreeUpdate) GetUpdateSize() int {
	size := 0

	if u.FullHTML != "" {
		size += len(u.FullHTML)
	}

	for _, change := range u.Changes {
		// Rough estimate of JSON size
		size += 20 // Base overhead
		size += len(change.Type)
		size += len(change.Key)
		size += len(change.Value)
		size += len(change.HTML)
		size += len(change.Path) * 4 // Path array
	}

	return size
}

// String provides readable representation
func (u *MinimalTreeUpdate) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("MinimalTreeUpdate for fragment: %s", u.FragmentID))
	lines = append(lines, fmt.Sprintf("  Type: %s", u.Type))

	if u.Type == "full" {
		preview := u.FullHTML
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		lines = append(lines, fmt.Sprintf("  Full HTML: %s", preview))
	} else if u.Type == "partial" {
		lines = append(lines, fmt.Sprintf("  Changes: %d", len(u.Changes)))
		for i, change := range u.Changes {
			pathStr := fmt.Sprintf("%v", change.Path)
			switch change.Type {
			case "text":
				lines = append(lines, fmt.Sprintf("    %d. [text] %s: '%s' → '%s'",
					i+1, pathStr, change.OldValue, change.Value))
			case "attr":
				lines = append(lines, fmt.Sprintf("    %d. [attr] %s: %s='%s' (was '%s')",
					i+1, pathStr, change.Key, change.Value, change.OldValue))
			case "add":
				preview := change.HTML
				if len(preview) > 50 {
					preview = preview[:50] + "..."
				}
				lines = append(lines, fmt.Sprintf("    %d. [add] %s: %s",
					i+1, pathStr, preview))
			case "remove":
				lines = append(lines, fmt.Sprintf("    %d. [remove] %s",
					i+1, pathStr))
			case "replace":
				preview := change.HTML
				if len(preview) > 50 {
					preview = preview[:50] + "..."
				}
				lines = append(lines, fmt.Sprintf("    %d. [replace] %s: %s",
					i+1, pathStr, preview))
			}
		}
	}

	return strings.Join(lines, "\n")
}

// GetChecksum returns a checksum of the current tree for validation
func (d *PureTreeDiff) GetChecksum() string {
	if d.lastHTML == "" {
		return ""
	}
	hash := md5.Sum([]byte(d.lastHTML))
	return hex.EncodeToString(hash[:])
}
