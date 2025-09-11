// Simple Tree-Based Template Strategy - Following LiveView's minimal client structure
package strategy

import (
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"strings"
)

// SimpleTreeData represents the minimal client data structure like LiveView
type SimpleTreeData struct {
	// Static segments
	S []string `json:"s,omitempty"`

	// Dynamic/nested content - can be string values or nested SimpleTreeData
	// Uses string keys "0", "1", "2" etc. for client compatibility
	Dynamics map[string]interface{} `json:",inline"`

	FragmentID string `json:"-"` // Not sent to client

	// Evaluation metadata for incremental updates (not sent to client)
	fieldPaths map[string]string `json:"-"` // dynamic key -> field path
	conditions map[string]string `json:"-"` // dynamic key -> condition expression
}

// SimpleTreeGenerator creates minimal tree structures for maximum client efficiency
type SimpleTreeGenerator struct {
	cache map[string]*SimpleTreeData // FragmentID -> cached static structure
}

// NewSimpleTreeGenerator creates a new simple tree generator
func NewSimpleTreeGenerator() *SimpleTreeGenerator {
	return &SimpleTreeGenerator{
		cache: make(map[string]*SimpleTreeData),
	}
}

// GenerateFromTemplateSource creates simple tree data from template source
func (g *SimpleTreeGenerator) GenerateFromTemplateSource(templateSource string, oldData, newData interface{}, fragmentID string) (*SimpleTreeData, error) {
	// Check if this template contains range constructs
	hasRangeConstructs := strings.Contains(templateSource, "{{range")

	// Check if we have cached structure
	_, hasCached := g.cache[fragmentID]

	// Always use full structure generation for now to fix conditional issues
	// TODO: Fix incremental updates for conditionals
	_ = hasCached
	_ = hasRangeConstructs

	// First render OR templates with range constructs - always use full structure generation
	return g.generateFullStructure(templateSource, newData, fragmentID)
}

// generateFullStructure analyzes template and creates complete structure
func (g *SimpleTreeGenerator) generateFullStructure(templateSource string, data interface{}, fragmentID string) (*SimpleTreeData, error) {
	// Parse template boundaries using simplified parser
	parser := NewTemplateParser()
	boundaries, err := parser.ParseBoundaries(templateSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template boundaries: %v", err)
	}

	// Build simple tree structure
	structure, err := g.buildSimpleTree(boundaries, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build simple tree: %v", err)
	}

	structure.FragmentID = fragmentID

	// Cache the static structure (without dynamic values)
	staticStructure := g.extractStaticStructure(structure)
	g.cache[fragmentID] = staticStructure

	return structure, nil
}

// buildSimpleTree builds the minimal tree structure from boundaries
func (g *SimpleTreeGenerator) buildSimpleTree(boundaries []TemplateBoundary, data interface{}) (*SimpleTreeData, error) {
	tree := &SimpleTreeData{
		S:          []string{},
		Dynamics:   make(map[string]interface{}),
		fieldPaths: make(map[string]string),
		conditions: make(map[string]string),
	}

	dynamicIndex := 0
	i := 0

	for i < len(boundaries) {
		boundary := boundaries[i]

		switch boundary.Type {
		case StaticContent:
			tree.S = append(tree.S, boundary.Content)
			i++

		case SimpleField, Function:
			// Add static up to this point
			if len(tree.S) == dynamicIndex {
				tree.S = append(tree.S, "")
			}

			// Evaluate dynamic value (either field access or function call)
			value, err := g.evaluateFieldPath(boundary.FieldPath, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate %s %s: %v", boundary.Type, boundary.FieldPath, err)
			}

			dynamicKey := fmt.Sprintf("%d", dynamicIndex)
			tree.Dynamics[dynamicKey] = fmt.Sprintf("%v", value)
			tree.fieldPaths[dynamicKey] = boundary.FieldPath // Store field path for incremental updates
			dynamicIndex++
			i++

		case ConditionalIf:
			// Add static up to this point (may be empty string)
			if len(tree.S) == dynamicIndex {
				tree.S = append(tree.S, "")
			}

			// Handle conditional using flat boundaries (from TemplateParser)
			conditionalTree, nextIndex, err := g.buildConditionalTree(boundaries, i, data)
			if err != nil {
				return nil, fmt.Errorf("failed to build conditional tree: %v", err)
			}

			dynamicKey := fmt.Sprintf("%d", dynamicIndex)
			tree.Dynamics[dynamicKey] = conditionalTree
			tree.conditions[dynamicKey] = boundary.FieldPath // Store condition for incremental updates
			dynamicIndex++
			i = nextIndex

		case RangeLoop:
			// Add static up to this point (may be empty string)
			if len(tree.S) == dynamicIndex {
				tree.S = append(tree.S, "")
			}

			// Handle range using flat boundaries (from TemplateParser)
			rangeTree, nextIndex, err := g.buildRangeTree(boundaries, i, data)
			if err != nil {
				return nil, fmt.Errorf("failed to build range tree: %v", err)
			}

			dynamicKey := fmt.Sprintf("%d", dynamicIndex)
			tree.Dynamics[dynamicKey] = rangeTree
			tree.conditions[dynamicKey] = boundary.FieldPath // Store range field path for incremental updates
			dynamicIndex++
			i = nextIndex

		case Comment, TemplateDefinition:
			// Skip
			i++

		case ContextWith:
			// Add static up to this point (may be empty string)
			if len(tree.S) == dynamicIndex {
				tree.S = append(tree.S, "")
			}

			// Handle with context using flat boundaries
			withTree, nextIndex, err := g.buildWithTree(boundaries, i, data)
			if err != nil {
				return nil, fmt.Errorf("failed to build with tree: %v", err)
			}

			dynamicKey := fmt.Sprintf("%d", dynamicIndex)
			tree.Dynamics[dynamicKey] = withTree
			tree.conditions[dynamicKey] = boundary.FieldPath // Store with condition for incremental updates
			dynamicIndex++
			i = nextIndex

		case Variable:
			// Variables affect evaluation context but don't produce direct output
			// For now, we fall back to full template re-rendering for templates with variables
			return nil, fmt.Errorf("variable declarations not yet supported in tree optimization: %s", boundary.Content)

		case Pipeline:
			// Pipelines require complex evaluation - fall back for now
			return nil, fmt.Errorf("pipeline operations not yet supported in tree optimization: %s", boundary.Content)

		case ConditionalEnd, ConditionalElse, RangeEnd, WithEnd, WithElse:
			// These are handled by the structured parsing above, skip them here
			i++

		default:
			// Unsupported - fallback
			return nil, fmt.Errorf("unsupported construct for simple tree: %v", boundary.Type)
		}
	}

	// Add final static if needed
	if len(tree.S) == dynamicIndex {
		tree.S = append(tree.S, "")
	}

	// Ensure there's always at least one dynamic slot for consistency
	// This maintains the expected structure format
	if len(tree.Dynamics) == 0 && len(tree.S) > 0 {
		tree.Dynamics["0"] = ""
	}

	return tree, nil
}

// buildConditionalTree creates conditional structure using flat boundaries
func (g *SimpleTreeGenerator) buildConditionalTree(boundaries []TemplateBoundary, startIndex int, data interface{}) (*SimpleTreeData, int, error) {
	conditionalBoundary := boundaries[startIndex]

	// Extract condition - TemplateParser puts it in FieldPath
	condition := conditionalBoundary.FieldPath
	if condition == "" {
		condition = conditionalBoundary.Condition // fallback for compatibility
	}

	// Evaluate condition to determine which branch is currently active
	branchKey, err := g.evaluateCondition(condition, data)
	if err != nil {
		return nil, startIndex, fmt.Errorf("failed to evaluate condition: %v", err)
	}

	// Find matching {{end}} and collect content
	nestingLevel := 1
	currentIndex := startIndex + 1
	elseIndex := -1
	var contentBoundaries []TemplateBoundary

	for currentIndex < len(boundaries) && nestingLevel > 0 {
		boundary := boundaries[currentIndex]

		switch boundary.Type {
		case ConditionalIf, RangeLoop, ContextWith:
			nestingLevel++
			contentBoundaries = append(contentBoundaries, boundary)
		case ConditionalElse:
			if nestingLevel == 1 {
				elseIndex = currentIndex
			} else {
				contentBoundaries = append(contentBoundaries, boundary)
			}
		case ConditionalEnd:
			nestingLevel--
			if nestingLevel > 0 {
				contentBoundaries = append(contentBoundaries, boundary)
			}
		case Complex:
			// Handle {{end}} which might be classified as Complex
			if strings.Contains(boundary.Content, "end") {
				nestingLevel--
				if nestingLevel > 0 {
					contentBoundaries = append(contentBoundaries, boundary)
				}
			} else {
				contentBoundaries = append(contentBoundaries, boundary)
			}
		default:
			contentBoundaries = append(contentBoundaries, boundary)
		}

		currentIndex++
	}

	// Split content into true and false blocks based on elseIndex
	var trueBoundaries, falseBoundaries []TemplateBoundary
	if elseIndex != -1 {
		// Split at else
		elseRelativeIndex := elseIndex - startIndex - 1
		if elseRelativeIndex > 0 {
			trueBoundaries = contentBoundaries[:elseRelativeIndex]
		}
		if elseRelativeIndex < len(contentBoundaries) {
			falseBoundaries = contentBoundaries[elseRelativeIndex:]
		}
	} else {
		// No else block
		trueBoundaries = contentBoundaries
	}

	// Select the active branch and build its tree structure
	var selectedBranch []TemplateBoundary
	if branchKey == "true" {
		selectedBranch = trueBoundaries
	} else if branchKey == "false" {
		selectedBranch = falseBoundaries
	}

	// Build tree for the selected branch
	if len(selectedBranch) > 0 {
		branchTree, err := g.buildSimpleTree(selectedBranch, data)
		if err != nil {
			return nil, startIndex, fmt.Errorf("failed to build conditional branch: %v", err)
		}
		return branchTree, currentIndex, nil
	} else {
		// Empty branch - return empty structure
		emptyTree := &SimpleTreeData{
			S:          []string{""},
			Dynamics:   make(map[string]interface{}),
			fieldPaths: make(map[string]string),
			conditions: make(map[string]string),
		}
		return emptyTree, currentIndex, nil
	}
}

// buildRangeTree creates range structure
func (g *SimpleTreeGenerator) buildRangeTree(boundaries []TemplateBoundary, startIndex int, data interface{}) (interface{}, int, error) {
	rangeBoundary := boundaries[startIndex]

	// Extract range data field - TemplateParser puts it in FieldPath
	rangeDataField := rangeBoundary.FieldPath
	if rangeDataField == "" {
		rangeDataField = rangeBoundary.Condition // fallback for compatibility
	}
	if strings.Contains(rangeDataField, ":=") {
		parts := strings.Split(rangeDataField, ":=")
		rangeDataField = strings.TrimSpace(parts[1])
	}

	// Evaluate range data
	rangeData, err := g.evaluateFieldPath(rangeDataField, data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to evaluate range data: %v", err)
	}

	// Convert to slice
	items, ok := rangeData.([]interface{})
	if !ok {
		// Try to convert other types to []interface{}
		switch v := rangeData.(type) {
		case []string:
			items = make([]interface{}, len(v))
			for i, s := range v {
				items[i] = s
			}
		default:
			// Use reflection to handle any slice type
			rv := reflect.ValueOf(rangeData)
			if rv.Kind() != reflect.Slice {
				return nil, 0, fmt.Errorf("range data is not iterable: %T", rangeData)
			}

			items = make([]interface{}, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				items[i] = rv.Index(i).Interface()
			}
		}
	}

	// Find range content with proper nesting level tracking
	currentIndex := startIndex + 1
	rangeContent := []TemplateBoundary{}
	nestingLevel := 0 // Track nesting depth

	for currentIndex < len(boundaries) {
		boundary := boundaries[currentIndex]

		// Track nesting level for proper end detection
		if boundary.Type == ConditionalIf || boundary.Type == RangeLoop || boundary.Type == ContextWith {
			nestingLevel++
		} else if boundary.Type == RangeEnd || boundary.Type == ConditionalEnd {
			if nestingLevel == 0 {
				break // This is our range/conditional end
			}
			nestingLevel--
		} else if boundary.Type == Complex {
			// Handle {{end}} which might be classified as Complex
			if strings.Contains(boundary.Content, "{{end}}") {
				if nestingLevel == 0 {
					break // This is our range end
				}
				nestingLevel--
			}
		}

		rangeContent = append(rangeContent, boundary)
		currentIndex++
	}

	// Build structure for each item
	var result interface{}

	if len(items) == 0 {
		// Empty range - return empty structure
		result = &SimpleTreeData{
			S:        []string{""},
			Dynamics: make(map[string]interface{}),
		}
	} else if len(items) == 1 {
		// Single item - return direct structure
		itemTree, err := g.buildSimpleTree(rangeContent, items[0])
		if err != nil {
			return nil, 0, fmt.Errorf("failed to build range item: %v", err)
		}
		result = itemTree
	} else {
		// Multiple items - return array of structures
		itemTrees := make([]interface{}, len(items))
		for i, item := range items {
			itemTree, err := g.buildSimpleTree(rangeContent, item)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to build range item %d: %v", i, err)
			}
			itemTrees[i] = itemTree
		}
		result = itemTrees
	}

	return result, currentIndex + 1, nil
}

// buildWithTree creates with context structure using flat boundaries
func (g *SimpleTreeGenerator) buildWithTree(boundaries []TemplateBoundary, startIndex int, data interface{}) (*SimpleTreeData, int, error) {
	withBoundary := boundaries[startIndex]

	// Extract with context field - TemplateParser puts it in FieldPath
	withField := withBoundary.FieldPath
	if withField == "" {
		withField = withBoundary.Condition // fallback for compatibility
	}

	// Evaluate the with field to get the new context
	withData, err := g.evaluateFieldPath(withField, data)
	if err != nil {
		// Field doesn't exist or evaluation failed - treat as falsy, go to else case
		return g.buildWithElseCase(boundaries, startIndex, data)
	}

	// Check if the value is truthy (non-nil, non-empty)
	if withData == nil || (reflect.ValueOf(withData).Kind() == reflect.Ptr && reflect.ValueOf(withData).IsNil()) {
		// Handle else case if present
		return g.buildWithElseCase(boundaries, startIndex, data)
	}

	// Find matching {{end}} and collect content
	nestingLevel := 1
	currentIndex := startIndex + 1
	elseIndex := -1
	var contentBoundaries []TemplateBoundary

	for currentIndex < len(boundaries) && nestingLevel > 0 {
		boundary := boundaries[currentIndex]

		switch boundary.Type {
		case ConditionalIf, RangeLoop, ContextWith:
			nestingLevel++
			contentBoundaries = append(contentBoundaries, boundary)
		case WithElse:
			if nestingLevel == 1 {
				elseIndex = currentIndex
			} else {
				contentBoundaries = append(contentBoundaries, boundary)
			}
		case ConditionalEnd, WithEnd:
			nestingLevel--
			if nestingLevel > 0 {
				contentBoundaries = append(contentBoundaries, boundary)
			}
		case Complex:
			// Handle {{end}} which might be classified as Complex
			if strings.Contains(boundary.Content, "end") {
				nestingLevel--
				if nestingLevel > 0 {
					contentBoundaries = append(contentBoundaries, boundary)
				}
			} else {
				contentBoundaries = append(contentBoundaries, boundary)
			}
		default:
			contentBoundaries = append(contentBoundaries, boundary)
		}

		currentIndex++
	}

	// Split content into main and else blocks based on elseIndex
	var mainBoundaries []TemplateBoundary
	if elseIndex != -1 {
		// Split at else
		elseRelativeIndex := elseIndex - startIndex - 1
		if elseRelativeIndex > 0 {
			mainBoundaries = contentBoundaries[:elseRelativeIndex]
		}
	} else {
		// No else block
		mainBoundaries = contentBoundaries
	}

	// Build tree for the main block using the with context
	if len(mainBoundaries) > 0 {
		withTree, err := g.buildSimpleTree(mainBoundaries, withData)
		if err != nil {
			return nil, startIndex, fmt.Errorf("failed to build with content: %v", err)
		}
		return withTree, currentIndex, nil
	} else {
		// Empty with block
		emptyTree := &SimpleTreeData{
			S:          []string{""},
			Dynamics:   make(map[string]interface{}),
			fieldPaths: make(map[string]string),
			conditions: make(map[string]string),
		}
		return emptyTree, currentIndex, nil
	}
}

// buildWithElseCase handles the else case for with blocks
func (g *SimpleTreeGenerator) buildWithElseCase(boundaries []TemplateBoundary, startIndex int, data interface{}) (*SimpleTreeData, int, error) {
	// Find matching {{end}} and collect content, looking for else case
	nestingLevel := 1
	currentIndex := startIndex + 1
	elseIndex := -1
	var elseBoundaries []TemplateBoundary

	for currentIndex < len(boundaries) && nestingLevel > 0 {
		boundary := boundaries[currentIndex]

		switch boundary.Type {
		case ConditionalIf, RangeLoop, ContextWith:
			nestingLevel++
		case WithElse:
			if nestingLevel == 1 {
				elseIndex = currentIndex
				// Start collecting else content
				currentIndex++
				continue
			}
		case ConditionalEnd, WithEnd:
			nestingLevel--
			if nestingLevel > 0 && elseIndex != -1 {
				elseBoundaries = append(elseBoundaries, boundary)
			}
		default:
			if elseIndex != -1 {
				elseBoundaries = append(elseBoundaries, boundary)
			}
		}

		currentIndex++
	}

	// Build tree for the else block using original context
	if len(elseBoundaries) > 0 {
		elseTree, err := g.buildSimpleTree(elseBoundaries, data)
		if err != nil {
			return nil, startIndex, fmt.Errorf("failed to build with else content: %v", err)
		}
		return elseTree, currentIndex, nil
	} else {
		// Empty else case
		emptyTree := &SimpleTreeData{
			S:          []string{""},
			Dynamics:   make(map[string]interface{}),
			fieldPaths: make(map[string]string),
			conditions: make(map[string]string),
		}
		return emptyTree, currentIndex, nil
	}
}

// evaluateCondition evaluates conditional expression
func (g *SimpleTreeGenerator) evaluateCondition(condition string, data interface{}) (string, error) {
	// Clean condition - remove leading/trailing spaces
	condition = strings.TrimSpace(condition)

	// Create conditional template - handle different condition formats
	var conditionTemplate string
	if strings.HasPrefix(condition, ".") || strings.Contains(condition, " ") {
		// Field reference (.Field) or function call (gt .Field 0) - use as-is
		conditionTemplate = fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", condition)
	} else {
		// Simple field name without dot - add prefix
		conditionTemplate = fmt.Sprintf("{{if .%s}}true{{else}}false{{end}}", condition)
	}

	tmpl, err := template.New("condition").Parse(conditionTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse condition template: %v", err)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed to execute condition template: %v", err)
	}

	return result.String(), nil
}

// evaluateFieldPath evaluates field path expression
func (g *SimpleTreeGenerator) evaluateFieldPath(fieldPath string, data interface{}) (interface{}, error) {
	// Use simplified template parser for field evaluation
	parser := NewTemplateParser()
	return parser.EvaluateFieldPath(fieldPath, data)
}

// extractStaticStructure extracts only static structure for caching
func (g *SimpleTreeGenerator) extractStaticStructure(tree *SimpleTreeData) *SimpleTreeData {
	staticTree := &SimpleTreeData{
		S:          make([]string, len(tree.S)),
		Dynamics:   make(map[string]interface{}),
		fieldPaths: make(map[string]string),
		conditions: make(map[string]string),
	}

	copy(staticTree.S, tree.S)

	// Copy evaluation metadata for incremental updates
	for key, path := range tree.fieldPaths {
		staticTree.fieldPaths[key] = path
	}
	for key, condition := range tree.conditions {
		staticTree.conditions[key] = condition
	}

	// Recursively extract static structures from nested dynamics
	for key, value := range tree.Dynamics {
		if nestedTree, ok := value.(*SimpleTreeData); ok {
			staticTree.Dynamics[key] = g.extractStaticStructure(nestedTree)
		}
		// Don't include simple string values in static structure
	}

	return staticTree
}

// MarshalJSON customizes JSON marshaling to match LiveView format
func (tree *SimpleTreeData) MarshalJSON() ([]byte, error) {
	// Create map with statics first
	result := make(map[string]interface{})

	if len(tree.S) > 0 {
		result["s"] = tree.S
	}

	// Add dynamics with string keys
	for key, value := range tree.Dynamics {
		result[key] = value
	}

	return json.Marshal(result)
}

// ClearCache clears cached structures
func (g *SimpleTreeGenerator) ClearCache() {
	g.cache = make(map[string]*SimpleTreeData)
}

// HasCachedStructure checks if structure is cached
func (g *SimpleTreeGenerator) HasCachedStructure(fragmentID string) bool {
	_, exists := g.cache[fragmentID]
	return exists
}
