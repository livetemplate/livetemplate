package diff

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// PatternType represents different patterns of HTML changes
type PatternType string

const (
	PatternStaticDynamic     PatternType = "static-dynamic"     // Best for strategy 1
	PatternConditionalStatic PatternType = "conditional-static" // Enhanced strategy 1 with conditionals
	PatternMarkerizable      PatternType = "markerizable"       // Best for strategy 2
	PatternGranular          PatternType = "granular"           // Best for strategy 3
	PatternReplacement       PatternType = "replacement"        // Fallback to strategy 4
	PatternUnknown           PatternType = "unknown"
)

// ConditionalType represents types of conditional patterns
type ConditionalType string

const (
	ConditionalBoolean   ConditionalType = "boolean"    // {{if .Flag}} content {{end}}
	ConditionalNilNotNil ConditionalType = "nil-notnil" // {{if .Value}} content {{end}}
	ConditionalShowHide  ConditionalType = "show-hide"  // Complete element show/hide
	ConditionalIfElse    ConditionalType = "if-else"    // {{if .Flag}}<struct1>{{else}}<struct2>{{end}}
)

// ConditionalPattern represents a detected conditional pattern
type ConditionalPattern struct {
	Type          ConditionalType
	States        [2]string // [falsy_state, truthy_state]
	ChangeType    string    // "attribute", "element", "content"
	IsFullElement bool      // True if entire element is conditional
	IsPredictable bool      // True if pattern is predictable/deterministic
}

// StrategyRecommendation represents a recommended strategy
type StrategyRecommendation struct {
	Strategy           int // 1-4 corresponding to the four-tier strategy
	Pattern            PatternType
	Reason             string
	ConditionalPattern *ConditionalPattern // Set if conditional pattern detected
}

// PatternClassifier analyzes HTML changes and recommends the best strategy
type PatternClassifier struct {
	comparator *DOMComparator
}

// NewPatternClassifier creates a new pattern classifier
func NewPatternClassifier() *PatternClassifier {
	return &PatternClassifier{
		comparator: NewDOMComparator(),
	}
}

// ClassifyPattern analyzes changes and recommends the best strategy
func (pc *PatternClassifier) ClassifyPattern(oldHTML, newHTML string) (*StrategyRecommendation, error) {
	// Handle empty inputs - allow them for show/hide patterns
	oldTrimmed := strings.TrimSpace(oldHTML)
	newTrimmed := strings.TrimSpace(newHTML)

	// Check for show/hide patterns first (before rejecting empty inputs)
	if oldTrimmed == "" && newTrimmed != "" {
		// Show pattern: empty → content
		return &StrategyRecommendation{
			Strategy: 1,
			Pattern:  PatternConditionalStatic,
			Reason:   "Show element conditional pattern optimal for enhanced static/dynamic strategy",
			ConditionalPattern: &ConditionalPattern{
				Type:          ConditionalShowHide,
				States:        [2]string{oldHTML, newHTML},
				ChangeType:    "element",
				IsFullElement: true,
				IsPredictable: true,
			},
		}, nil
	}

	if oldTrimmed != "" && newTrimmed == "" {
		// Hide pattern: content → empty
		return &StrategyRecommendation{
			Strategy: 1,
			Pattern:  PatternConditionalStatic,
			Reason:   "Hide element conditional pattern optimal for enhanced static/dynamic strategy",
			ConditionalPattern: &ConditionalPattern{
				Type:          ConditionalShowHide,
				States:        [2]string{newHTML, oldHTML}, // [hidden, shown]
				ChangeType:    "element",
				IsFullElement: true,
				IsPredictable: true,
			},
		}, nil
	}

	// Reject if both are empty
	if oldTrimmed == "" && newTrimmed == "" {
		return nil, fmt.Errorf("both HTML inputs are empty")
	}

	changes, err := pc.comparator.Compare(oldHTML, newHTML)
	if err != nil {
		return nil, err
	}

	// No changes detected
	if len(changes) == 0 {
		return &StrategyRecommendation{
			Strategy: 1, // Default to fastest strategy
			Pattern:  PatternStaticDynamic,
			Reason:   "No changes detected",
		}, nil
	}

	// Check for conditional patterns first (enhanced Strategy 1)
	if conditionalPattern := pc.detectConditionalPattern(oldHTML, newHTML, changes); conditionalPattern != nil {
		return &StrategyRecommendation{
			Strategy:           1,
			Pattern:            PatternConditionalStatic,
			Reason:             fmt.Sprintf("Predictable %s conditional pattern optimal for enhanced static/dynamic strategy", conditionalPattern.Type),
			ConditionalPattern: conditionalPattern,
		}, nil
	}

	// Deterministic rule-based strategy selection
	return pc.selectStrategyDeterministically(changes), nil
}

// selectStrategyDeterministically uses rule-based deterministic selection
func (pc *PatternClassifier) selectStrategyDeterministically(changes []DOMChange) *StrategyRecommendation {
	// Special case: Check for empty state patterns first
	if pc.isEmptyStatePattern(changes) {
		return &StrategyRecommendation{
			Strategy: 1,
			Pattern:  PatternStaticDynamic,
			Reason:   "Empty state transitions optimal for static/dynamic strategy",
		}
	}

	// Analyze change types
	hasText := false
	hasAttributeValue := false     // Value changes within attributes (Strategy 1)
	hasAttributeStructure := false // Adding/removing attributes (Strategy 2)
	hasStructural := false

	for _, change := range changes {
		switch change.Type {
		case ChangeTextOnly:
			hasText = true
		case ChangeAttribute:
			// Distinguish between attribute value changes vs structural attribute changes
			if change.Description == "Attribute value changed" {
				// Changing class="old" to class="new" is just text content change
				hasAttributeValue = true
			} else {
				// Adding/removing attributes is structural change to attributes
				hasAttributeStructure = true
			}
		case ChangeStructure:
			hasStructural = true
		}
	}

	// Rule-based deterministic selection:
	if hasStructural {
		// Any structural change → Strategy 3 or 4
		if hasText || hasAttributeValue || hasAttributeStructure {
			// Structural + content/attribute changes → Strategy 4 (complex replacement)
			return &StrategyRecommendation{
				Strategy: 4,
				Pattern:  PatternReplacement,
				Reason:   "Mixed structural and content changes require replacement",
			}
		}
		// Pure structural changes → Strategy 3 (granular operations)
		return &StrategyRecommendation{
			Strategy: 3,
			Pattern:  PatternGranular,
			Reason:   "Pure structural changes optimal for granular operations",
		}
	}

	if hasAttributeStructure {
		// Attribute structure changes (adding/removing attributes) → Strategy 2
		return &StrategyRecommendation{
			Strategy: 2,
			Pattern:  PatternMarkerizable,
			Reason:   "Attribute structure changes optimal for marker compilation",
		}
	}

	if hasText || hasAttributeValue {
		// Pure text-only changes OR attribute value changes → Strategy 1
		reason := "Text-only changes optimal for static/dynamic strategy"
		if hasAttributeValue {
			reason = "Attribute value changes are text-only optimal for static/dynamic strategy"
		}
		return &StrategyRecommendation{
			Strategy: 1,
			Pattern:  PatternStaticDynamic,
			Reason:   reason,
		}
	}

	// No changes (should not happen as this is checked earlier)
	return &StrategyRecommendation{
		Strategy: 1,
		Pattern:  PatternStaticDynamic,
		Reason:   "No changes detected - static/dynamic default",
	}
}

// isEmptyStatePattern checks if changes represent empty state transitions (show/hide content)
func (pc *PatternClassifier) isEmptyStatePattern(changes []DOMChange) bool {
	// Empty state patterns should be simple show/hide scenarios
	if len(changes) > 3 {
		return false // Too many changes for simple empty state
	}

	// Look for patterns where content is being added to or removed from empty containers
	nodeAddedOrRemoved := false
	childCountChanged := false

	for _, change := range changes {
		if change.Type == ChangeStructure {
			if change.Description == "Node added" || change.Description == "Node removed" {
				nodeAddedOrRemoved = true
			}
			if change.Description == "Number of children changed" {
				// Check if it's transitioning to/from empty
				if strings.Contains(change.OldValue, "0 children") || strings.Contains(change.NewValue, "0 children") {
					childCountChanged = true
				}
			}
		}
	}

	// Empty state pattern: content being added/removed AND child count changing to/from zero
	return nodeAddedOrRemoved && childCountChanged
}

// detectConditionalPattern analyzes changes to detect predictable conditional patterns
func (pc *PatternClassifier) detectConditionalPattern(oldHTML, newHTML string, changes []DOMChange) *ConditionalPattern {
	// Skip if too many changes (not a simple conditional)
	if len(changes) > 3 {
		return nil
	}

	// Check for boolean attribute conditionals
	if pattern := pc.detectBooleanAttributeConditional(oldHTML, newHTML, changes); pattern != nil {
		return pattern
	}

	// Check for show/hide element conditionals
	if pattern := pc.detectShowHideElementConditional(oldHTML, newHTML, changes); pattern != nil {
		return pattern
	}

	// Check for nil/not-nil conditionals
	if pattern := pc.detectNilNotNilConditional(oldHTML, newHTML, changes); pattern != nil {
		return pattern
	}

	// Check for if-else structural conditionals
	if pattern := pc.detectIfElseStructuralConditional(oldHTML, newHTML, changes); pattern != nil {
		return pattern
	}

	return nil
}

// detectBooleanAttributeConditional detects patterns like {{if .Flag}} disabled{{end}}
func (pc *PatternClassifier) detectBooleanAttributeConditional(oldHTML, newHTML string, changes []DOMChange) *ConditionalPattern {
	// Must be exactly one attribute change
	if len(changes) != 1 || changes[0].Type != ChangeAttribute {
		return nil
	}

	change := changes[0]

	// Check for boolean attributes (attributes without values like disabled, checked, hidden)
	if change.Description == "Attribute added" {
		// Check if this looks like a boolean attribute (no value or empty value)
		if pc.isBooleanAttribute(change.Path, change.NewValue) {
			return &ConditionalPattern{
				Type:          ConditionalBoolean,
				States:        [2]string{oldHTML, newHTML}, // [without_attr, with_attr]
				ChangeType:    "attribute",
				IsFullElement: false,
				IsPredictable: true,
			}
		}
	}

	// Check for attribute removal (true → false)
	if change.Description == "Attribute removed" {
		if pc.isBooleanAttribute(change.Path, change.OldValue) {
			return &ConditionalPattern{
				Type:          ConditionalBoolean,
				States:        [2]string{newHTML, oldHTML}, // [without_attr, with_attr]
				ChangeType:    "attribute",
				IsFullElement: false,
				IsPredictable: true,
			}
		}
	}

	return nil
}

// isBooleanAttribute determines if an attribute is a boolean attribute
func (pc *PatternClassifier) isBooleanAttribute(attrPath, attrValue string) bool {
	// Extract attribute name from path
	if !strings.Contains(attrPath, "@") {
		return false
	}

	attrName := attrPath[strings.LastIndex(attrPath, "@")+1:]

	// Common boolean attributes
	booleanAttrs := map[string]bool{
		"disabled": true,
		"checked":  true,
		"hidden":   true,
		"readonly": true,
		"required": true,
		"multiple": true,
		"selected": true,
		"defer":    true,
		"async":    true,
	}

	// Check if it's a known boolean attribute
	if booleanAttrs[attrName] {
		return true
	}

	// Check if the attribute has no value (boolean-style)
	return attrValue == "" || attrValue == attrName
}

// detectShowHideElementConditional detects patterns like {{if .Show}}<element>{{end}}
func (pc *PatternClassifier) detectShowHideElementConditional(oldHTML, newHTML string, changes []DOMChange) *ConditionalPattern {
	// Look for element addition/removal pattern
	hasElementChange := false
	hasChildCountChange := false

	for _, change := range changes {
		if change.Type == ChangeStructure {
			if change.Description == "Node added" || change.Description == "Node removed" {
				hasElementChange = true
			}
			if change.Description == "Number of children changed" {
				hasChildCountChange = true
			}
		}
	}

	if !hasElementChange || !hasChildCountChange {
		return nil
	}

	// Determine which state is empty and which has content
	oldTrimmed := strings.TrimSpace(oldHTML)
	newTrimmed := strings.TrimSpace(newHTML)

	if oldTrimmed == "" && newTrimmed != "" {
		// Show: empty → content
		return &ConditionalPattern{
			Type:          ConditionalShowHide,
			States:        [2]string{oldHTML, newHTML}, // [hidden, shown]
			ChangeType:    "element",
			IsFullElement: true,
			IsPredictable: true,
		}
	}

	if oldTrimmed != "" && newTrimmed == "" {
		// Hide: content → empty
		return &ConditionalPattern{
			Type:          ConditionalShowHide,
			States:        [2]string{newHTML, oldHTML}, // [hidden, shown]
			ChangeType:    "element",
			IsFullElement: true,
			IsPredictable: true,
		}
	}

	return nil
}

// detectIfElseStructuralConditional detects patterns like {{if .Flag}}<table>{{else}}<div>{{end}}
func (pc *PatternClassifier) detectIfElseStructuralConditional(oldHTML, newHTML string, changes []DOMChange) *ConditionalPattern {
	// Check if we have structural changes
	hasStructural := false
	hasAddition := false
	hasRemoval := false

	for _, change := range changes {
		if change.Type == ChangeStructure {
			hasStructural = true
			// Check if this is an addition or removal (not a true replacement)
			if strings.Contains(change.Description, "Node added") {
				hasAddition = true
			}
			if strings.Contains(change.Description, "Node removed") {
				hasRemoval = true
			}
		}
	}

	// Must have structural changes for if-else structural conditionals
	if !hasStructural {
		return nil
	}

	// If this is just adding/removing nodes (not replacing), it's not a conditional
	// Conditionals represent complete structural switches, not incremental changes
	if hasAddition && !hasRemoval {
		return nil // Pure addition - not a conditional
	}
	if hasRemoval && !hasAddition {
		return nil // Pure removal - not a conditional
	}

	// Look for patterns where the structure completely changes but follows a binary pattern
	// This suggests an if-else conditional where different structures are shown
	oldTrimmed := strings.TrimSpace(oldHTML)
	newTrimmed := strings.TrimSpace(newHTML)

	// Both states should have content (not empty)
	if oldTrimmed == "" || newTrimmed == "" {
		return nil
	}

	// Check if this looks like a binary structural switch
	// We analyze the root elements to see if they're completely different structures
	if pc.isBinaryStructuralSwitch(oldHTML, newHTML) {
		return &ConditionalPattern{
			Type:          ConditionalIfElse,
			States:        [2]string{oldHTML, newHTML}, // [falsy_structure, truthy_structure]
			ChangeType:    "structure",
			IsFullElement: true,
			IsPredictable: true,
		}
	}

	return nil
}

// isBinaryStructuralSwitch determines if two HTML fragments represent a binary structural switch
func (pc *PatternClassifier) isBinaryStructuralSwitch(oldHTML, newHTML string) bool {
	// Parse both HTML fragments to analyze their structure
	oldDoc, err := pc.comparator.parser.ParseFragment(oldHTML)
	if err != nil {
		return false
	}

	newDoc, err := pc.comparator.parser.ParseFragment(newHTML)
	if err != nil {
		return false
	}

	// For fragments, get the first element node directly
	oldRoot := pc.getFirstElementNode(oldDoc)
	newRoot := pc.getFirstElementNode(newDoc)

	// Both should have root elements
	if oldRoot == nil || newRoot == nil {
		return false
	}

	// Different tag names could indicate a structural switch
	if oldRoot.Data != newRoot.Data {
		// But only if this looks like a clean structural switch, not a complex change
		// Check if there are too many differences (attributes + text changes)
		if pc.hasTooManyDifferences(oldRoot, newRoot) {
			return false // Too complex to be a simple conditional
		}
		return true
	}

	// Same tag but significantly different structure could also be a switch
	if pc.hasSignificantStructuralDifference(oldRoot, newRoot) {
		return true
	}

	return false
}

// hasSignificantStructuralDifference checks if two elements have significantly different structures
func (pc *PatternClassifier) hasSignificantStructuralDifference(old, new *DOMNode) bool {
	// Count child elements in both
	oldChildCount := pc.countElementChildren(old)
	newChildCount := pc.countElementChildren(new)

	// Significant difference in child count suggests structural switch
	if oldChildCount != newChildCount {
		return true
	}

	// Check if child tag names are different
	oldTags := pc.getChildTagNames(old)
	newTags := pc.getChildTagNames(new)

	if len(oldTags) != len(newTags) {
		return true
	}

	for i, oldTag := range oldTags {
		if i < len(newTags) && oldTag != newTags[i] {
			return true
		}
	}

	return false
}

// countElementChildren counts the number of element children
func (pc *PatternClassifier) countElementChildren(node *DOMNode) int {
	count := 0
	for _, child := range node.Children {
		if child.Type == html.ElementNode {
			count++
		}
	}
	return count
}

// getChildTagNames gets the tag names of element children
func (pc *PatternClassifier) getChildTagNames(node *DOMNode) []string {
	var tags []string
	for _, child := range node.Children {
		if child.Type == html.ElementNode {
			tags = append(tags, child.Data)
		}
	}
	return tags
}

// getFirstElementNode finds the first element node in a fragment
func (pc *PatternClassifier) getFirstElementNode(doc *DOMNode) *DOMNode {
	if doc == nil {
		return nil
	}

	// If this is an element node, return it
	if doc.Type == html.ElementNode {
		return doc
	}

	// Otherwise, search children for the first element node
	for _, child := range doc.Children {
		if child.Type == html.ElementNode {
			return child
		}
	}

	return nil
}

// hasTooManyDifferences checks if two elements have too many differences to be a simple conditional
func (pc *PatternClassifier) hasTooManyDifferences(old, new *DOMNode) bool {
	// Check text content differences
	oldText := strings.TrimSpace(old.GetTextContent())
	newText := strings.TrimSpace(new.GetTextContent())

	// Check attribute differences on the root elements only
	oldAttrs := old.Attributes
	newAttrs := new.Attributes

	// Count significant attribute differences
	attrChanges := 0

	// Check for changed/removed attributes
	for key, oldVal := range oldAttrs {
		if newVal, exists := newAttrs[key]; !exists {
			attrChanges++ // Attribute removed
		} else if oldVal != newVal {
			attrChanges++ // Attribute changed
		}
	}

	// Check for added attributes
	for key := range newAttrs {
		if _, exists := oldAttrs[key]; !exists {
			attrChanges++ // Attribute added
		}
	}

	// A case is too complex if:
	// 1. ANY text changes AND ANY attribute changes (along with structural change)
	// 2. OR if there are many attribute changes alone
	hasAnyTextDifference := oldText != newText
	return (hasAnyTextDifference && attrChanges > 0) || attrChanges > 3
}

// detectNilNotNilConditional detects patterns like {{if .Value}} class="{{.Value}}"{{end}}
func (pc *PatternClassifier) detectNilNotNilConditional(oldHTML, newHTML string, changes []DOMChange) *ConditionalPattern {
	// For attributes with values (non-boolean attributes)
	if len(changes) != 1 || changes[0].Type != ChangeAttribute {
		return nil
	}

	change := changes[0]

	// Check if this looks like a nil/not-nil transition with actual values
	if change.Description == "Attribute added" && change.NewValue != "" {
		// Check that this is NOT a boolean attribute
		if !pc.isBooleanAttribute(change.Path, change.NewValue) {
			// nil → "some value" pattern
			return &ConditionalPattern{
				Type:          ConditionalNilNotNil,
				States:        [2]string{oldHTML, newHTML}, // [nil_state, value_state]
				ChangeType:    "attribute",
				IsFullElement: false,
				IsPredictable: true,
			}
		}
	}

	if change.Description == "Attribute removed" && change.OldValue != "" {
		// Check that this is NOT a boolean attribute
		if !pc.isBooleanAttribute(change.Path, change.OldValue) {
			// "some value" → nil pattern
			return &ConditionalPattern{
				Type:          ConditionalNilNotNil,
				States:        [2]string{newHTML, oldHTML}, // [nil_state, value_state]
				ChangeType:    "attribute",
				IsFullElement: false,
				IsPredictable: true,
			}
		}
	}

	return nil
}
