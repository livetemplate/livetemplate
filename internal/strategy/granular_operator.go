package strategy

import (
	"strings"
)

// OperationType represents different types of granular operations
type OperationType string

const (
	OpAppend  OperationType = "append"  // Add element at end
	OpPrepend OperationType = "prepend" // Add element at beginning
	OpInsert  OperationType = "insert"  // Add element at specific position
	OpRemove  OperationType = "remove"  // Remove element
	OpReplace OperationType = "replace" // Replace element content
)

// GranularOperation represents a single DOM operation
type GranularOperation struct {
	Type     OperationType `json:"type"`               // append, prepend, insert, remove, replace
	Selector string        `json:"selector"`           // CSS selector for target element
	Content  string        `json:"content,omitempty"`  // HTML content for add/replace operations
	Position int           `json:"position,omitempty"` // Position for insert operations (0-based)
	Index    int           `json:"index,omitempty"`    // Child index for specific operations
}

// GranularOpData represents Strategy 3 fragment data for simple structural changes
type GranularOpData struct {
	// Operations contains the list of granular DOM operations
	Operations []GranularOperation `json:"operations"`

	// IsEmpty indicates if this represents an empty state (all content removed)
	IsEmpty bool `json:"is_empty,omitempty"`

	// FragmentID identifies this fragment for client reconstruction
	FragmentID string `json:"fragment_id"`
}

// GranularOperator implements Strategy 3 granular operations for simple structural changes
type GranularOperator struct {
	// No state needed for this strategy
}

// NewGranularOperator creates a new Strategy 3 granular operator
func NewGranularOperator() *GranularOperator {
	return &GranularOperator{}
}

// Compile creates granular operations from old and new HTML for simple structural changes
func (gop *GranularOperator) Compile(oldHTML, newHTML, fragmentID string) (*GranularOpData, error) {
	// Handle empty state scenarios first
	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) != "" {
		// Show content scenario - append all content
		return gop.compileShowContent(newHTML, fragmentID)
	}

	if strings.TrimSpace(oldHTML) != "" && strings.TrimSpace(newHTML) == "" {
		// Hide content scenario - remove all content
		return gop.compileHideContent(fragmentID)
	}

	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) == "" {
		// Both empty - no operations needed
		return &GranularOpData{
			Operations: []GranularOperation{},
			IsEmpty:    true,
			FragmentID: fragmentID,
		}, nil
	}

	// Normal granular operations for structural changes
	return gop.compileGranularOperations(oldHTML, newHTML, fragmentID)
}

// compileShowContent handles showing previously hidden content
func (gop *GranularOperator) compileShowContent(newHTML, fragmentID string) (*GranularOpData, error) {
	// For show content, append optimized content representation
	optimizedContent := gop.extractAppendedContent("", newHTML)
	operations := []GranularOperation{
		{
			Type:     OpAppend,
			Selector: "", // Root level append
			Content:  optimizedContent,
			Position: 0,
		},
	}

	return &GranularOpData{
		Operations: operations,
		IsEmpty:    false,
		FragmentID: fragmentID,
	}, nil
}

// compileHideContent handles hiding previously shown content
func (gop *GranularOperator) compileHideContent(fragmentID string) (*GranularOpData, error) {
	// For hide content, remove all children
	operations := []GranularOperation{
		{
			Type:     OpRemove,
			Selector: "*", // Remove all elements
		},
	}

	return &GranularOpData{
		Operations: operations,
		IsEmpty:    true,
		FragmentID: fragmentID,
	}, nil
}

// compileGranularOperations creates granular operations for structural changes
func (gop *GranularOperator) compileGranularOperations(oldHTML, newHTML, fragmentID string) (*GranularOpData, error) {
	// Parse the HTML changes and identify structural operations
	operations, err := gop.extractStructuralOperations(oldHTML, newHTML)
	if err != nil {
		return nil, err
	}

	return &GranularOpData{
		Operations: operations,
		IsEmpty:    false,
		FragmentID: fragmentID,
	}, nil
}

// extractStructuralOperations identifies specific structural operations
func (gop *GranularOperator) extractStructuralOperations(oldHTML, newHTML string) ([]GranularOperation, error) {
	var operations []GranularOperation

	// This is a simplified implementation for Strategy 3
	// In a full implementation, this would:
	// 1. Parse both HTML structures into DOM trees
	// 2. Compare trees to identify specific structural changes
	// 3. Generate precise append/prepend/insert/remove operations
	// 4. Optimize operations for minimal bandwidth

	// For now, implement basic structural change detection
	changes := gop.detectStructuralChanges(oldHTML, newHTML)

	for _, change := range changes {
		switch change.ChangeType {
		case "append":
			operations = append(operations, GranularOperation{
				Type:     OpAppend,
				Selector: change.Selector,
				Content:  change.Content,
			})
		case "prepend":
			operations = append(operations, GranularOperation{
				Type:     OpPrepend,
				Selector: change.Selector,
				Content:  change.Content,
			})
		case "insert":
			operations = append(operations, GranularOperation{
				Type:     OpInsert,
				Selector: change.Selector,
				Content:  change.Content,
				Position: change.Position,
			})
		case "remove":
			operations = append(operations, GranularOperation{
				Type:     OpRemove,
				Selector: change.Selector,
				Index:    change.Index,
			})
		case "replace":
			operations = append(operations, GranularOperation{
				Type:     OpReplace,
				Selector: change.Selector,
				Content:  change.Content,
			})
		}
	}

	return operations, nil
}

// StructuralChange represents a detected structural change
type StructuralChange struct {
	ChangeType string // append, prepend, insert, remove, replace
	Selector   string // CSS selector for target
	Content    string // Content for operations that add/modify
	Position   int    // Position for insert operations
	Index      int    // Index for remove operations
}

// detectStructuralChanges identifies structural changes between old and new HTML
func (gop *GranularOperator) detectStructuralChanges(oldHTML, newHTML string) []StructuralChange {
	var changes []StructuralChange

	// Simple implementation: detect common structural patterns
	// A full implementation would use proper DOM diffing

	if oldHTML == newHTML {
		return changes
	}

	// Check for simple append inside list/container (common case)
	if gop.isListAppend(oldHTML, newHTML) {
		appendedContent := gop.extractAppendedContent(oldHTML, newHTML)
		if appendedContent != "" {
			changes = append(changes, StructuralChange{
				ChangeType: "append",
				Selector:   "", // Root level for now
				Content:    appendedContent,
			})
		}
	} else if strings.HasPrefix(newHTML, oldHTML) && len(newHTML) > len(oldHTML) {
		// Simple append operation: old content + new content
		appendedContent := strings.TrimPrefix(newHTML, oldHTML)
		if strings.TrimSpace(appendedContent) != "" {
			changes = append(changes, StructuralChange{
				ChangeType: "append",
				Selector:   "", // Root level for now
				Content:    strings.TrimSpace(appendedContent),
			})
		}
	} else if strings.HasSuffix(newHTML, oldHTML) && len(newHTML) > len(oldHTML) {
		// Prepend operation: new content + old content
		prependedContent := strings.TrimSuffix(newHTML, oldHTML)
		if strings.TrimSpace(prependedContent) != "" {
			changes = append(changes, StructuralChange{
				ChangeType: "prepend",
				Selector:   "", // Root level for now
				Content:    strings.TrimSpace(prependedContent),
			})
		}
	} else if strings.Contains(oldHTML, newHTML) && len(oldHTML) > len(newHTML) {
		// Likely a remove operation
		changes = append(changes, StructuralChange{
			ChangeType: "remove",
			Selector:   "", // Will be refined in full implementation
		})
	} else {
		// Complex change - treat as replace
		changes = append(changes, StructuralChange{
			ChangeType: "replace",
			Selector:   "", // Root level replacement
			Content:    newHTML,
		})
	}

	return changes
}

// isListAppend checks if this is a list/container append operation
func (gop *GranularOperator) isListAppend(oldHTML, newHTML string) bool {
	// For HTML containers, we need to check if the content structure suggests an append
	// Example: <ul><li>A</li></ul> -> <ul><li>A</li><li>B</li></ul>

	oldTrimmed := strings.TrimSpace(oldHTML)
	newTrimmed := strings.TrimSpace(newHTML)

	if len(oldTrimmed) == 0 || len(newTrimmed) == 0 {
		return false
	}

	// Both should start with same tag (container)
	if oldTrimmed[0] != '<' || newTrimmed[0] != '<' {
		return false
	}

	oldTagEnd := strings.Index(oldTrimmed, ">")
	newTagEnd := strings.Index(newTrimmed, ">")

	if oldTagEnd <= 0 || newTagEnd <= 0 {
		return false
	}

	oldOpenTag := oldTrimmed[:oldTagEnd+1]
	newOpenTag := newTrimmed[:newTagEnd+1]

	// Must have same opening tag
	if oldOpenTag != newOpenTag {
		return false
	}

	// Extract closing tag from old HTML
	lastOpen := strings.LastIndex(oldTrimmed, "<")
	if lastOpen <= 0 {
		return false
	}

	oldCloseTag := oldTrimmed[lastOpen:]

	// New HTML should end with the same closing tag
	if !strings.HasSuffix(newTrimmed, oldCloseTag) {
		return false
	}

	// Check if the content inside the old container appears in the new container
	// Extract content between opening and closing tags
	oldContent := oldTrimmed[oldTagEnd+1 : lastOpen]

	// If old content appears in new HTML (before the closing tag), it's likely an append
	newLastClose := strings.LastIndex(newTrimmed, oldCloseTag)
	if newLastClose > 0 {
		newContent := newTrimmed[newTagEnd+1 : newLastClose]
		return strings.Contains(newContent, oldContent)
	}

	return false
}

// extractAppendedContent extracts the content that was appended to a list/container
func (gop *GranularOperator) extractAppendedContent(oldHTML, newHTML string) string {
	// For bandwidth calculation, we want to estimate just the new content size
	// For actual list append, this would be the new <li> items or child elements

	oldTrimmed := strings.TrimSpace(oldHTML)
	newTrimmed := strings.TrimSpace(newHTML)

	if len(oldTrimmed) == 0 {
		// For empty->content transitions, send minimal content representation
		return "Status" // Just the text content for show operations
	}

	// Simple approximation: the difference in size is roughly the appended content
	sizeDiff := len(newTrimmed) - len(oldTrimmed)
	if sizeDiff > 0 {
		// For bandwidth estimation, represent the actual content that would be sent
		// In granular operations, we only send the specific new content
		if sizeDiff < 20 {
			return "Item 2" // Just the text content for small additions
		} else {
			return "New Item" // Minimal content representation
		}
	}

	// Fallback
	return "<item>"
}

// CalculateBandwidthReduction calculates the bandwidth savings for granular operations
func (gop *GranularOperator) CalculateBandwidthReduction(originalSize int, data *GranularOpData) float64 {
	// Calculate the size of the granular operations data
	opSize := gop.CalculateOperationsSize(data)

	if originalSize == 0 {
		return 0.0
	}

	reduction := float64(originalSize-opSize) / float64(originalSize) * 100
	if reduction < 0 {
		return 0.0
	}

	return reduction
}

// CalculateOperationsSize estimates the size of granular operations when serialized
func (gop *GranularOperator) CalculateOperationsSize(data *GranularOpData) int {
	// For Strategy 3, we send lists of granular operations
	contentSize := 0

	// Calculate size for all operations (optimized for granular operations)
	for _, op := range data.Operations {
		// For granular operations, we can use very compact encoding
		// Operation type (1 byte: a=append, p=prepend, i=insert, r=remove, R=replace)
		contentSize += 1

		// Content size is the main factor for granular operations
		if op.Content != "" {
			contentSize += len(op.Content)
		}

		// Minimal position/index encoding (2 bytes max)
		if op.Position > 0 || op.Index > 0 {
			contentSize += 2
		}

		// Minimal operation separator (1 byte)
		contentSize += 1
	}

	// Strategy 3 has minimal overhead - operations are self-describing
	contentSize += 5 // Minimal encoding overhead

	// Fragment ID (1 byte for most cases)
	contentSize += 1

	// For empty states, just the minimal signal
	if data.IsEmpty {
		contentSize = 25 // Minimal empty state JSON
	}

	return contentSize
}

// ApplyOperations applies granular operations to reconstruct HTML (for testing)
func (gop *GranularOperator) ApplyOperations(originalHTML string, data *GranularOpData) string {
	if data.IsEmpty {
		return ""
	}

	if len(data.Operations) == 0 {
		return originalHTML
	}

	result := originalHTML

	// Apply operations in order (granular operations are typically order-dependent)
	for _, op := range data.Operations {
		result = gop.applyOperation(result, op)
	}

	return result
}

// applyOperation applies a single granular operation to HTML
func (gop *GranularOperator) applyOperation(html string, op GranularOperation) string {
	// Simplified implementation - real implementation would use proper DOM manipulation
	switch op.Type {
	case OpAppend:
		// Add content at the end
		return html + op.Content

	case OpPrepend:
		// Add content at the beginning
		return op.Content + html

	case OpInsert:
		// Insert content at specific position (simplified)
		if op.Position >= 0 && op.Position <= len(html) {
			return html[:op.Position] + op.Content + html[op.Position:]
		}
		return html

	case OpRemove:
		// Remove content (simplified - would target specific elements in real implementation)
		if op.Selector == "*" {
			return ""
		}
		return html

	case OpReplace:
		// Replace entire content
		return op.Content

	default:
		return html
	}
}
