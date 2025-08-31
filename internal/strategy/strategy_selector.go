package strategy

import (
	"fmt"
	"html/template"
)

// StrategyType represents the type of strategy to use
type StrategyType int

const (
	// TreeBasedStrategy uses tree-based optimization for maximum efficiency
	TreeBasedStrategy StrategyType = iota
	// FragmentReplacementStrategy uses complete fragment replacement as fallback
	FragmentReplacementStrategy
)

// String returns string representation of strategy type
func (st StrategyType) String() string {
	switch st {
	case TreeBasedStrategy:
		return "TreeBased"
	case FragmentReplacementStrategy:
		return "FragmentReplacement"
	default:
		return "Unknown"
	}
}

// StrategySelector chooses the optimal strategy for a given template
type StrategySelector struct {
	treeGenerator    *SimpleTreeGenerator
	fragmentReplacer *FragmentReplacer
	templateParser   *TemplateParser
}

// NewStrategySelector creates a new strategy selector
func NewStrategySelector() *StrategySelector {
	return &StrategySelector{
		treeGenerator:    NewSimpleTreeGenerator(),
		fragmentReplacer: NewFragmentReplacer(),
		templateParser:   NewTemplateParser(),
	}
}

// SelectStrategy determines the best strategy for a template
func (ss *StrategySelector) SelectStrategy(templateSource string) (StrategyType, error) {
	// Parse template to understand complexity
	boundaries, err := ss.templateParser.ParseBoundaries(templateSource)
	if err != nil {
		return FragmentReplacementStrategy, fmt.Errorf("failed to parse template: %v", err)
	}

	// Simple decision logic: try tree-based first, fallback if complex
	if ss.isSuitableForTreeStrategy(boundaries) {
		return TreeBasedStrategy, nil
	}

	return FragmentReplacementStrategy, nil
}

// GenerateUpdate generates an update using the appropriate strategy
func (ss *StrategySelector) GenerateUpdate(templateSource string, tmpl *template.Template, oldData, newData interface{}, fragmentID string) (interface{}, StrategyType, error) {
	strategy, err := ss.SelectStrategy(templateSource)
	if err != nil {
		return nil, FragmentReplacementStrategy, err
	}

	switch strategy {
	case TreeBasedStrategy:
		treeData, err := ss.treeGenerator.GenerateFromTemplateSource(templateSource, oldData, newData, fragmentID)
		if err != nil {
			// Fallback to fragment replacement if tree generation fails
			replacement, fallbackErr := ss.fragmentReplacer.GenerateReplacement(tmpl, newData, fragmentID)
			if fallbackErr != nil {
				return nil, FragmentReplacementStrategy, fmt.Errorf("both strategies failed - tree: %v, replacement: %v", err, fallbackErr)
			}
			return replacement, FragmentReplacementStrategy, nil
		}
		return treeData, TreeBasedStrategy, nil

	case FragmentReplacementStrategy:
		replacement, err := ss.fragmentReplacer.GenerateReplacement(tmpl, newData, fragmentID)
		if err != nil {
			return nil, FragmentReplacementStrategy, fmt.Errorf("fragment replacement failed: %v", err)
		}
		return replacement, FragmentReplacementStrategy, nil

	default:
		return nil, FragmentReplacementStrategy, fmt.Errorf("unknown strategy type: %v", strategy)
	}
}

// isSuitableForTreeStrategy determines if template is suitable for tree-based optimization
func (ss *StrategySelector) isSuitableForTreeStrategy(boundaries []TemplateBoundary) bool {
	for _, boundary := range boundaries {
		switch boundary.Type {
		case StaticContent, SimpleField, Comment, TemplateDefinition:
			// These are compatible with tree-based strategy
			continue
		case ConditionalIf, RangeLoop:
			// These are supported but make it more complex - still try tree-based
			continue
		case ContextWith, Variable, TemplateInvocation, BlockDefinition, Pipeline, Function, LoopControl, Complex:
			// These constructs are too complex for tree-based optimization
			return false
		default:
			// Unknown construct - use fallback
			return false
		}
	}
	return true
}

// GetTreeGenerator returns the tree generator for direct use
func (ss *StrategySelector) GetTreeGenerator() *SimpleTreeGenerator {
	return ss.treeGenerator
}

// GetFragmentReplacer returns the fragment replacer for direct use
func (ss *StrategySelector) GetFragmentReplacer() *FragmentReplacer {
	return ss.fragmentReplacer
}

// ClearCaches clears all strategy caches
func (ss *StrategySelector) ClearCaches() {
	ss.treeGenerator.ClearCache()
}

// GetStrategyStats returns statistics about strategy usage
func (ss *StrategySelector) GetStrategyStats(templateSources []string) (map[StrategyType]int, error) {
	stats := make(map[StrategyType]int)

	for _, templateSource := range templateSources {
		strategy, err := ss.SelectStrategy(templateSource)
		if err != nil {
			continue // Skip templates that can't be analyzed
		}
		stats[strategy]++
	}

	return stats, nil
}
