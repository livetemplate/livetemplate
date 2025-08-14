package diff

import (
	"fmt"
	"strings"
)

// PatternType represents different patterns of HTML changes
type PatternType string

const (
	PatternStaticDynamic PatternType = "static-dynamic" // Best for strategy 1
	PatternMarkerizable  PatternType = "markerizable"   // Best for strategy 2
	PatternGranular      PatternType = "granular"       // Best for strategy 3
	PatternReplacement   PatternType = "replacement"    // Fallback to strategy 4
	PatternUnknown       PatternType = "unknown"
)

// StrategyRecommendation represents a recommended strategy
type StrategyRecommendation struct {
	Strategy int // 1-4 corresponding to the four-tier strategy
	Pattern  PatternType
	Reason   string
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
	// Handle empty inputs
	if strings.TrimSpace(oldHTML) == "" || strings.TrimSpace(newHTML) == "" {
		return nil, fmt.Errorf("empty HTML input not allowed")
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
	hasAttribute := false
	hasStructural := false

	for _, change := range changes {
		switch change.Type {
		case ChangeTextOnly:
			hasText = true
		case ChangeAttribute:
			hasAttribute = true
		case ChangeStructure:
			hasStructural = true
		}
	}

	// Rule-based deterministic selection:
	if hasStructural {
		// Any structural change → Strategy 3 or 4
		if hasText || hasAttribute {
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

	if hasAttribute {
		// Attribute changes (with/without text, no structural) → Strategy 2
		return &StrategyRecommendation{
			Strategy: 2,
			Pattern:  PatternMarkerizable,
			Reason:   "Attribute changes optimal for marker compilation",
		}
	}

	if hasText {
		// Pure text-only changes (no attribute, no structural) → Strategy 1
		return &StrategyRecommendation{
			Strategy: 1,
			Pattern:  PatternStaticDynamic,
			Reason:   "Text-only changes optimal for static/dynamic strategy",
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
