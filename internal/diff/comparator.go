package diff

import (
	"fmt"
)

// ChangeType represents the type of change detected
type ChangeType string

const (
	ChangeNone      ChangeType = "none"
	ChangeTextOnly  ChangeType = "text-only"
	ChangeAttribute ChangeType = "attribute"
	ChangeStructure ChangeType = "structure"
	ChangeComplex   ChangeType = "complex"
)

// DOMChange represents a change between two DOM nodes
type DOMChange struct {
	Type        ChangeType
	Path        string
	OldValue    string
	NewValue    string
	Description string
}

// DOMComparator handles comparison between two DOM trees
type DOMComparator struct {
	parser *DOMParser
}

// NewDOMComparator creates a new DOM comparator
func NewDOMComparator() *DOMComparator {
	return &DOMComparator{
		parser: NewDOMParser(),
	}
}

// Compare compares two DOM trees and returns the differences
func (c *DOMComparator) Compare(oldHTML, newHTML string) ([]DOMChange, error) {
	oldNode, err := c.parser.ParseFragment(oldHTML)
	if err != nil {
		return nil, err
	}

	newNode, err := c.parser.ParseFragment(newHTML)
	if err != nil {
		return nil, err
	}

	// Normalize both trees
	oldNormalized := c.parser.NormalizeNode(oldNode)
	newNormalized := c.parser.NormalizeNode(newNode)

	var changes []DOMChange
	c.compareNodes(oldNormalized, newNormalized, &changes)

	return changes, nil
}

// compareNodes recursively compares two DOM nodes
func (c *DOMComparator) compareNodes(oldNode, newNode *DOMNode, changes *[]DOMChange) {
	// Handle nil cases
	if oldNode == nil && newNode == nil {
		return
	}

	if oldNode == nil {
		// Node was added
		*changes = append(*changes, DOMChange{
			Type:        ChangeStructure,
			Path:        newNode.GetPath(),
			OldValue:    "",
			NewValue:    c.nodeToString(newNode),
			Description: "Node added",
		})
		return
	}

	if newNode == nil {
		// Node was removed
		*changes = append(*changes, DOMChange{
			Type:        ChangeStructure,
			Path:        oldNode.GetPath(),
			OldValue:    c.nodeToString(oldNode),
			NewValue:    "",
			Description: "Node removed",
		})
		return
	}

	// Compare node types first
	if oldNode.Type != newNode.Type {
		*changes = append(*changes, DOMChange{
			Type:        ChangeStructure,
			Path:        oldNode.GetPath(),
			OldValue:    c.nodeToString(oldNode),
			NewValue:    c.nodeToString(newNode),
			Description: "Node type changed",
		})
		return
	}

	// For element nodes, compare tag names (data field)
	if oldNode.IsElementNode() && oldNode.Data != newNode.Data {
		*changes = append(*changes, DOMChange{
			Type:        ChangeStructure,
			Path:        oldNode.GetPath(),
			OldValue:    c.nodeToString(oldNode),
			NewValue:    c.nodeToString(newNode),
			Description: "Element tag changed",
		})
		return
	}

	// For text nodes, compare content
	if oldNode.IsTextNode() {
		if oldNode.Data != newNode.Data {
			changeType := ChangeTextOnly

			// Check if this is just whitespace changes
			if c.normalizeWhitespace(oldNode.Data) == c.normalizeWhitespace(newNode.Data) {
				changeType = ChangeNone
			}

			if changeType != ChangeNone {
				*changes = append(*changes, DOMChange{
					Type:        changeType,
					Path:        oldNode.GetPath(),
					OldValue:    oldNode.Data,
					NewValue:    newNode.Data,
					Description: "Text content changed",
				})
			}
		}
		return
	}

	// For element nodes, compare attributes
	c.compareAttributes(oldNode, newNode, changes)

	// Compare children
	c.compareChildren(oldNode, newNode, changes)
}

// compareAttributes compares the attributes of two nodes
func (c *DOMComparator) compareAttributes(oldNode, newNode *DOMNode, changes *[]DOMChange) {
	// Find added and changed attributes
	for key, newValue := range newNode.Attributes {
		if oldValue, exists := oldNode.Attributes[key]; exists {
			if oldValue != newValue {
				*changes = append(*changes, DOMChange{
					Type:        ChangeAttribute,
					Path:        oldNode.GetPath() + "/@" + key,
					OldValue:    oldValue,
					NewValue:    newValue,
					Description: "Attribute value changed",
				})
			}
		} else {
			*changes = append(*changes, DOMChange{
				Type:        ChangeAttribute,
				Path:        oldNode.GetPath() + "/@" + key,
				OldValue:    "",
				NewValue:    newValue,
				Description: "Attribute added",
			})
		}
	}

	// Find removed attributes
	for key, oldValue := range oldNode.Attributes {
		if _, exists := newNode.Attributes[key]; !exists {
			*changes = append(*changes, DOMChange{
				Type:        ChangeAttribute,
				Path:        oldNode.GetPath() + "/@" + key,
				OldValue:    oldValue,
				NewValue:    "",
				Description: "Attribute removed",
			})
		}
	}
}

// compareChildren compares the children of two nodes
func (c *DOMComparator) compareChildren(oldNode, newNode *DOMNode, changes *[]DOMChange) {
	oldChildren := oldNode.Children
	newChildren := newNode.Children

	// Simple comparison for now - can be enhanced with more sophisticated matching
	maxLen := len(oldChildren)
	if len(newChildren) > maxLen {
		maxLen = len(newChildren)
	}

	for i := 0; i < maxLen; i++ {
		var oldChild, newChild *DOMNode

		if i < len(oldChildren) {
			oldChild = oldChildren[i]
		}
		if i < len(newChildren) {
			newChild = newChildren[i]
		}

		c.compareNodes(oldChild, newChild, changes)
	}

	// Detect if this is a complex change (multiple children affected)
	if len(oldChildren) != len(newChildren) {
		*changes = append(*changes, DOMChange{
			Type:        ChangeStructure,
			Path:        oldNode.GetPath(),
			OldValue:    c.formatChildCount(len(oldChildren)),
			NewValue:    c.formatChildCount(len(newChildren)),
			Description: "Number of children changed",
		})
	}
}

// nodeToString converts a node to a readable string representation
func (c *DOMComparator) nodeToString(node *DOMNode) string {
	if node == nil {
		return "<nil>"
	}

	if node.IsTextNode() {
		return node.Data
	}

	result := "<" + node.Data
	for key, value := range node.Attributes {
		result += " " + key + `="` + value + `"`
	}
	result += ">"

	return result
}

// normalizeWhitespace normalizes whitespace for comparison
func (c *DOMComparator) normalizeWhitespace(text string) string {
	// Simple normalization - can be enhanced
	return text
}

// formatChildCount formats child count for display
func (c *DOMComparator) formatChildCount(count int) string {
	if count == 1 {
		return "1 child"
	}
	return fmt.Sprintf("%d children", count)
}

// ClassifyChanges analyzes a set of changes and classifies the overall change pattern
func (c *DOMComparator) ClassifyChanges(changes []DOMChange) ChangeType {
	if len(changes) == 0 {
		return ChangeNone
	}

	hasTextOnly := false
	hasAttribute := false
	hasStructure := false

	for _, change := range changes {
		switch change.Type {
		case ChangeTextOnly:
			hasTextOnly = true
		case ChangeAttribute:
			hasAttribute = true
		case ChangeStructure:
			hasStructure = true
		}
	}

	// Determine overall classification
	if hasStructure {
		if hasTextOnly || hasAttribute {
			return ChangeComplex
		}
		return ChangeStructure
	}

	if hasAttribute && hasTextOnly {
		return ChangeComplex
	}

	if hasAttribute {
		return ChangeAttribute
	}

	if hasTextOnly {
		return ChangeTextOnly
	}

	return ChangeNone
}
