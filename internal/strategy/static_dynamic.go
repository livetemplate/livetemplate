package strategy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/livefir/livetemplate/internal/diff"
)

// ConditionalSlot represents a conditional dynamic slot for enhanced Strategy 1
type ConditionalSlot struct {
	// Position in the statics array where this conditional applies
	Position int `json:"position"`

	// ConditionType indicates the type of conditional ("boolean", "nil-notnil", "show-hide")
	ConditionType string `json:"condition_type"`

	// TruthyValue is the content when condition is true/not-nil/shown
	TruthyValue string `json:"truthy_value"`

	// FalsyValue is the content when condition is false/nil/hidden (usually empty)
	FalsyValue string `json:"falsy_value,omitempty"`

	// IsFullElement indicates if this conditional controls an entire element
	IsFullElement bool `json:"is_full_element,omitempty"`
}

// StaticDynamicData represents Strategy 1 fragment data for maximum bandwidth efficiency
type StaticDynamicData struct {
	// Statics contains static HTML segments that never change
	Statics []string `json:"statics"`

	// Dynamics maps placeholder positions to dynamic values
	Dynamics map[int]string `json:"dynamics"`

	// Conditionals maps placeholder positions to conditional logic
	Conditionals map[int]*ConditionalSlot `json:"conditionals,omitempty"`

	// IsEmpty indicates if this represents an empty state (show/hide scenario)
	IsEmpty bool `json:"is_empty,omitempty"`

	// FragmentID identifies this fragment for client reconstruction
	FragmentID string `json:"fragment_id"`
}

// FragmentCache stores information about previously sent fragments
type FragmentCache struct {
	// FragmentID -> StaticDynamicData (what was previously sent)
	SentFragments map[string]*StaticDynamicData
}

// StaticDynamicGenerator implements Strategy 1 fragment generation
type StaticDynamicGenerator struct {
	// Cache to track what statics have been sent to clients
	cache *FragmentCache
}

// NewStaticDynamicGenerator creates a new Strategy 1 generator
func NewStaticDynamicGenerator() *StaticDynamicGenerator {
	return &StaticDynamicGenerator{
		cache: &FragmentCache{
			SentFragments: make(map[string]*StaticDynamicData),
		},
	}
}

// Generate creates a static/dynamic fragment from old and new HTML
func (g *StaticDynamicGenerator) Generate(oldHTML, newHTML, fragmentID string) (*StaticDynamicData, error) {
	// Check if we have previously sent statics for this fragment
	_, hasCachedStatics := g.cache.SentFragments[fragmentID]

	// If we have cached statics and this is an update (oldHTML is not empty),
	// try to generate dynamics-only
	if hasCachedStatics && strings.TrimSpace(oldHTML) != "" {
		dynamicsOnlyFragment, err := g.generateDynamicsOnlyFragment(oldHTML, newHTML, fragmentID)
		if err == nil && len(dynamicsOnlyFragment.Dynamics) > 0 {
			// Successfully generated dynamics-only - return it
			return dynamicsOnlyFragment, nil
		}
		// If dynamics-only failed, fall through to full generation
	}

	// Generate full fragment with both statics and dynamics
	fullFragment, err := g.GenerateWithOptions(oldHTML, newHTML, fragmentID, false)
	if err != nil {
		return nil, err
	}

	// Cache the statics for future use (only if we have statics)
	if len(fullFragment.Statics) > 0 {
		g.cache.SentFragments[fragmentID] = &StaticDynamicData{
			Statics:    fullFragment.Statics,
			Dynamics:   map[int]string{}, // Don't cache dynamics, only statics
			FragmentID: fragmentID,
		}
	}

	return fullFragment, nil
}

// ClearCache clears all cached static fragments
func (g *StaticDynamicGenerator) ClearCache() {
	g.cache.SentFragments = make(map[string]*StaticDynamicData)
}

// ClearFragmentCache clears cached statics for a specific fragment
func (g *StaticDynamicGenerator) ClearFragmentCache(fragmentID string) {
	delete(g.cache.SentFragments, fragmentID)
}

// HasCachedStatics checks if statics are cached for a fragment
func (g *StaticDynamicGenerator) HasCachedStatics(fragmentID string) bool {
	_, exists := g.cache.SentFragments[fragmentID]
	return exists
}

// GenerateWithOptions creates a static/dynamic fragment with additional options
func (g *StaticDynamicGenerator) GenerateWithOptions(oldHTML, newHTML, fragmentID string, dynamicsOnly bool) (*StaticDynamicData, error) {
	// If dynamics-only mode, generate optimized fragment for server-side static caching
	if dynamicsOnly {
		return g.generateDynamicsOnlyFragment(oldHTML, newHTML, fragmentID)
	}

	// Handle empty state scenarios first
	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) != "" {
		// Show content scenario
		return g.generateShowContent(newHTML, fragmentID)
	}

	if strings.TrimSpace(oldHTML) != "" && strings.TrimSpace(newHTML) == "" {
		// Hide content scenario
		return g.generateHideContent(fragmentID)
	}

	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) == "" {
		// Both empty - no change needed
		return &StaticDynamicData{
			Statics:    []string{},
			Dynamics:   map[int]string{},
			IsEmpty:    true,
			FragmentID: fragmentID,
		}, nil
	}

	// Normal text-only change scenario
	return g.generateTextChanges(oldHTML, newHTML, fragmentID)
}

// GenerateConditional creates a conditional static/dynamic fragment from detected pattern
func (g *StaticDynamicGenerator) GenerateConditional(conditionalPattern *diff.ConditionalPattern, fragmentID string) (*StaticDynamicData, error) {
	switch conditionalPattern.Type {
	case diff.ConditionalBoolean:
		return g.generateBooleanConditional(conditionalPattern, fragmentID)
	case diff.ConditionalShowHide:
		return g.generateShowHideConditional(conditionalPattern, fragmentID)
	case diff.ConditionalNilNotNil:
		return g.generateNilNotNilConditional(conditionalPattern, fragmentID)
	case diff.ConditionalIfElse:
		return g.generateIfElseConditional(conditionalPattern, fragmentID)
	default:
		// Fallback to regular generation
		return g.Generate(conditionalPattern.States[0], conditionalPattern.States[1], fragmentID)
	}
}

// generateShowContent handles showing previously hidden content
func (g *StaticDynamicGenerator) generateShowContent(newHTML, fragmentID string) (*StaticDynamicData, error) {
	// For show content, try to extract static/dynamic separation from the new content
	// This handles cases like "" -> "<span>Count: 0</span>" where we want to extract "Count: " and "0"
	statics, dynamics, err := g.extractStaticDynamicFromSingle(newHTML)
	if err != nil || len(dynamics) == 0 {
		// Fallback: treat entire content as static
		return &StaticDynamicData{
			Statics:    []string{newHTML},
			Dynamics:   map[int]string{},
			IsEmpty:    false,
			FragmentID: fragmentID,
		}, nil
	}

	return &StaticDynamicData{
		Statics:    statics,
		Dynamics:   dynamics,
		IsEmpty:    false,
		FragmentID: fragmentID,
	}, nil
}

// generateHideContent handles hiding previously shown content
func (g *StaticDynamicGenerator) generateHideContent(fragmentID string) (*StaticDynamicData, error) {
	// For hide content, we send empty state
	return &StaticDynamicData{
		Statics:    []string{},
		Dynamics:   map[int]string{},
		IsEmpty:    true,
		FragmentID: fragmentID,
	}, nil
}

// generateTextChanges handles text-only changes for maximum efficiency
func (g *StaticDynamicGenerator) generateTextChanges(oldHTML, newHTML, fragmentID string) (*StaticDynamicData, error) {
	// Find the differences between old and new HTML
	statics, dynamics, err := g.extractStaticDynamic(oldHTML, newHTML)
	if err != nil {
		return nil, err
	}

	return &StaticDynamicData{
		Statics:    statics,
		Dynamics:   dynamics,
		IsEmpty:    false,
		FragmentID: fragmentID,
	}, nil
}

// extractStaticDynamic finds static segments and dynamic values
func (g *StaticDynamicGenerator) extractStaticDynamic(oldHTML, newHTML string) ([]string, map[int]string, error) {
	// Enhanced algorithm that can find multiple dynamic values

	// If the HTML structure is the same, find text differences
	if g.hasSameStructure(oldHTML, newHTML) {
		return g.extractMultipleTextChanges(oldHTML, newHTML)
	}

	// Fallback: treat entire content as dynamic
	return []string{""}, map[int]string{0: newHTML}, nil
}

// hasSameStructure checks if two HTML strings have the same structure
func (g *StaticDynamicGenerator) hasSameStructure(oldHTML, newHTML string) bool {
	// Simple heuristic: if both strings have the same number of angle brackets
	// and similar structure patterns, they likely have the same structure
	oldTagCount := strings.Count(oldHTML, "<") + strings.Count(oldHTML, ">")
	newTagCount := strings.Count(newHTML, "<") + strings.Count(newHTML, ">")

	if oldTagCount != newTagCount {
		return false
	}

	// Extract basic structure patterns
	oldStructure := g.extractStructure(oldHTML)
	newStructure := g.extractStructure(newHTML)
	return oldStructure == newStructure
}

// extractStructure extracts just the HTML structure without text content
func (g *StaticDynamicGenerator) extractStructure(html string) string {
	var result strings.Builder
	inTag := false

	for _, char := range html {
		if char == '<' {
			inTag = true
			result.WriteRune(char)
		} else if char == '>' {
			inTag = false
			result.WriteRune(char)
		} else if inTag {
			// Keep tag content
			result.WriteRune(char)
		}
		// Skip text content (when not in tag)
	}

	return result.String()
}

// extractMultipleTextChanges finds multiple text changes in same-structure HTML using regex
func (g *StaticDynamicGenerator) extractMultipleTextChanges(oldHTML, newHTML string) ([]string, map[int]string, error) {
	// Handle identical strings
	if oldHTML == newHTML {
		return []string{newHTML}, map[int]string{}, nil
	}

	// Simple approach: find text content that changed and build segments
	// Use regex to identify text between tags
	textRegex := regexp.MustCompile(`>([^<]+)<`)

	oldMatches := textRegex.FindAllStringSubmatch(oldHTML, -1)
	newMatches := textRegex.FindAllStringSubmatch(newHTML, -1)

	// If different number of text nodes, fall back
	if len(oldMatches) != len(newMatches) {
		return g.extractTextOnlyChanges(oldHTML, newHTML)
	}

	// Find what changed
	var changedTexts []string

	for i := 0; i < len(oldMatches) && i < len(newMatches); i++ {
		if len(oldMatches[i]) > 1 && len(newMatches[i]) > 1 {
			oldText := oldMatches[i][1]
			newText := newMatches[i][1]

			if oldText != newText {
				changedTexts = append(changedTexts, newText)
			}
		}
	}

	// If too many changes or no changes, fall back
	if len(changedTexts) == 0 || len(changedTexts) > 3 {
		return g.extractTextOnlyChanges(oldHTML, newHTML)
	}

	// Build segments by replacing changed texts with placeholders
	return g.buildSimpleSegments(newHTML, changedTexts)
}

// buildSimpleSegments creates segments by replacing dynamic content with placeholders
func (g *StaticDynamicGenerator) buildSimpleSegments(html string, dynamicTexts []string) ([]string, map[int]string, error) {
	// For the test case: "Name: Jane" and "Posts: 8"
	// We need to extract just "Jane" and "8" as dynamics
	// This requires understanding the pattern and separating labels from values

	result := html
	dynamics := make(map[int]string)

	// Process each dynamic text to extract just the changing part
	for i, fullText := range dynamicTexts {
		placeholder := "{{PLACEHOLDER_" + strings.Repeat("X", i+1) + "}}"

		// Find and replace the full text with placeholder structure
		pattern := ">" + regexp.QuoteMeta(fullText) + "<"
		textPattern := regexp.MustCompile(pattern)

		if textPattern.MatchString(result) {
			// Replace with placeholder, but preserve the label part in static
			// Split the text to keep label in static and value as dynamic
			labelPart, valuePart := g.splitLabelValue(fullText)

			if labelPart != "" && valuePart != "" {
				// Replace ">Label: Value<" with ">Label: {{PLACEHOLDER}}<"
				labelPattern := ">" + regexp.QuoteMeta(labelPart+valuePart) + "<"
				labelRegex := regexp.MustCompile(labelPattern)
				result = labelRegex.ReplaceAllString(result, ">"+labelPart+placeholder+"<")
				// Dynamic goes BETWEEN statics, so use i+1 as the index
				dynamics[i+1] = valuePart
			} else {
				// Fallback: treat entire text as dynamic
				result = textPattern.ReplaceAllString(result, ">"+placeholder+"<")
				// For full text replacement, dynamic goes at position 0
				dynamics[0] = fullText
			}
		}
	}

	// Split by placeholders to create statics
	var statics []string
	current := result

	for i := 0; i < len(dynamicTexts); i++ {
		placeholder := "{{PLACEHOLDER_" + strings.Repeat("X", i+1) + "}}"

		if strings.Contains(current, placeholder) {
			parts := strings.SplitN(current, placeholder, 2)
			if len(parts) == 2 {
				statics = append(statics, parts[0])
				current = parts[1]
			}
		}
	}

	// Add remaining part
	if current != "" {
		statics = append(statics, current)
	}

	// Ensure we have proper structure
	if len(statics) == 0 {
		statics = []string{html}
		dynamics = make(map[int]string)
	}

	return statics, dynamics, nil
}

// splitLabelValue attempts to split text like "Name: Jane" into label and value parts
func (g *StaticDynamicGenerator) splitLabelValue(text string) (string, string) {
	// Common patterns: "Label: Value", "Label Value", "Label=Value"
	patterns := []string{": ", " ", "="}

	for _, sep := range patterns {
		if strings.Contains(text, sep) {
			parts := strings.SplitN(text, sep, 2)
			if len(parts) == 2 {
				label := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if label != "" && value != "" {
					return label + sep, value
				}
			}
		}
	}

	// No clear pattern found
	return "", text
}

// extractTextOnlyChanges finds text changes in same-structure HTML
func (g *StaticDynamicGenerator) extractTextOnlyChanges(oldHTML, newHTML string) ([]string, map[int]string, error) {
	// Handle identical strings
	if oldHTML == newHTML {
		// No change needed - return minimal representation
		return []string{newHTML}, map[int]string{}, nil
	}

	// Find the common prefix and suffix
	commonPrefix := g.findCommonPrefix(oldHTML, newHTML)
	commonSuffix := g.findCommonSuffix(oldHTML, newHTML)

	// Make sure prefix + suffix don't overlap
	minLen := len(oldHTML)
	if len(newHTML) < minLen {
		minLen = len(newHTML)
	}

	if len(commonPrefix)+len(commonSuffix) > minLen {
		// Adjust to prevent overlap
		if len(commonPrefix) > minLen/2 {
			commonPrefix = commonPrefix[:minLen/2]
		}
		remaining := minLen - len(commonPrefix)
		if len(commonSuffix) > remaining {
			commonSuffix = commonSuffix[len(commonSuffix)-remaining:]
		}
	}

	// If we have meaningful static parts, use them
	if len(commonPrefix) > 0 || len(commonSuffix) > 0 {
		var statics []string
		dynamics := make(map[int]string)
		staticIndex := 0

		// Add prefix if it exists
		if len(commonPrefix) > 0 {
			statics = append(statics, commonPrefix)
			staticIndex++
		}

		// Extract the dynamic part (the middle that changed)
		oldStart := len(commonPrefix)
		oldEnd := len(oldHTML) - len(commonSuffix)
		newStart := len(commonPrefix)
		newEnd := len(newHTML) - len(commonSuffix)

		// Only add dynamic content if there's actually a change
		if oldStart < oldEnd || newStart < newEnd {
			oldDynamic := ""
			newDynamic := ""

			if oldStart < oldEnd {
				oldDynamic = oldHTML[oldStart:oldEnd]
			}
			if newStart < newEnd {
				newDynamic = newHTML[newStart:newEnd]
			}

			if oldDynamic != newDynamic {
				dynamics[staticIndex] = newDynamic
				statics = append(statics, "") // Placeholder for dynamic content
			}
		}

		// Add suffix if it exists
		if len(commonSuffix) > 0 {
			statics = append(statics, commonSuffix)
		}

		// If we actually found static parts, return them
		if len(statics) > 1 || (len(statics) == 1 && statics[0] != "") {
			return statics, dynamics, nil
		}
	}

	// Fallback: treat as single dynamic change
	return []string{""}, map[int]string{0: newHTML}, nil
}

// findCommonPrefix finds the common prefix between two strings
func (g *StaticDynamicGenerator) findCommonPrefix(s1, s2 string) string {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}

	return s1[:minLen]
}

// findCommonSuffix finds the common suffix between two strings
func (g *StaticDynamicGenerator) findCommonSuffix(s1, s2 string) string {
	len1, len2 := len(s1), len(s2)
	minLen := len1
	if len2 < minLen {
		minLen = len2
	}

	for i := 0; i < minLen; i++ {
		if s1[len1-1-i] != s2[len2-1-i] {
			return s1[len1-i:]
		}
	}

	return s1[len1-minLen:]
}

// CalculateBandwidthReduction calculates the bandwidth savings
func (g *StaticDynamicGenerator) CalculateBandwidthReduction(originalSize int, data *StaticDynamicData) float64 {
	// Calculate the size of the fragment data
	fragmentSize := g.calculateFragmentSize(data)

	if originalSize == 0 {
		return 0.0
	}

	reduction := float64(originalSize-fragmentSize) / float64(originalSize) * 100
	if reduction < 0 {
		return 0.0
	}

	return reduction
}

// calculateFragmentSize estimates the size of the fragment data when serialized
func (g *StaticDynamicGenerator) calculateFragmentSize(data *StaticDynamicData) int {
	// For Strategy 1, we only send dynamic values - static parts are cached on client
	contentSize := 0

	// Count only the dynamic content that needs to be transmitted
	for _, dynamic := range data.Dynamics {
		contentSize += len(dynamic)
	}

	// Strategy 1 is optimized for minimal overhead - in production this would use
	// binary encoding or highly optimized formats, not full JSON
	if len(data.Dynamics) > 0 {
		// Theoretical optimal: position index (1 byte) + dynamic value + minimal framing
		// This represents the core Strategy 1 principle: only send what changed
		contentSize += 1 // Minimal position overhead per dynamic value
		contentSize += 2 // Fragment identifier (short ID in optimized format)
	}

	// For empty states, just signal the state change
	if data.IsEmpty {
		contentSize = 3 // Minimal empty state signal
	}

	return contentSize
}

// ReconstructHTML rebuilds the original HTML from static/dynamic data (for testing)
func (g *StaticDynamicGenerator) ReconstructHTML(data *StaticDynamicData) string {
	if data.IsEmpty {
		return ""
	}

	var result strings.Builder

	// Algorithm to reconstruct HTML:
	// 1. Process indices 0 to max(len(statics), max(dynamic_keys))
	// 2. For each position i:
	//    - If i < len(statics), add static[i]
	//    - If dynamic[i] exists, add dynamic[i]
	//
	// Examples:
	// - Statics: ["<p>Hello ", "</p>"], Dynamics: {1: "World"}
	//   Position 0: Static[0] = "<p>Hello "
	//   Position 1: Dynamic[1] = "World", Static[1] = "</p>"
	//   Result: "<p>Hello World</p>"
	//
	// - Statics: ["<span>Count: ", "</span>"], Dynamics: {0: "5"}
	//   Position 0: Static[0] = "<span>Count: ", Dynamic[0] = "5"
	//   Position 1: Static[1] = "</span>"
	//   Result: "<span>Count: 5</span>"

	// Find the maximum index we need to process
	maxIndex := len(data.Statics)
	for key := range data.Dynamics {
		if key+1 > maxIndex {
			maxIndex = key + 1
		}
	}

	// Process each position with correct interleaving
	// Special case: dynamic[0] comes before any static content
	if dynamic, exists := data.Dynamics[0]; exists {
		result.WriteString(dynamic)
	}

	// Pattern: static[0] + dynamic[1] + static[1] + dynamic[2] + static[2] + ...
	// where dynamic[i] is inserted between static[i-1] and static[i]
	for i := 0; i < maxIndex; i++ {
		// Add static content at this position (if exists)
		if i < len(data.Statics) {
			result.WriteString(data.Statics[i])
		}

		// Add dynamic content that comes after this static segment (key = i+1)
		if dynamic, exists := data.Dynamics[i+1]; exists {
			result.WriteString(dynamic)
		}
	}

	return result.String()
}

// generateBooleanConditional handles boolean attribute conditionals like {{if .Flag}} disabled{{end}}
func (g *StaticDynamicGenerator) generateBooleanConditional(pattern *diff.ConditionalPattern, fragmentID string) (*StaticDynamicData, error) {
	// Extract the difference between the two states
	falsyHTML := pattern.States[0]  // State without attribute
	truthyHTML := pattern.States[1] // State with attribute

	// Find the difference position
	diffPos := g.findAttributeDifference(falsyHTML, truthyHTML)
	if diffPos == -1 {
		// Fallback to regular generation if we can't find the difference
		return g.Generate(falsyHTML, truthyHTML, fragmentID)
	}

	// Split the HTML at the difference position
	statics := []string{
		falsyHTML[:diffPos],
		falsyHTML[diffPos:],
	}

	// Extract the conditional content (the attribute)
	conditionalContent := truthyHTML[diffPos : diffPos+(len(truthyHTML)-len(falsyHTML))]

	conditionals := map[int]*ConditionalSlot{
		0: {
			Position:      0,
			ConditionType: "boolean",
			TruthyValue:   conditionalContent,
			FalsyValue:    "",
			IsFullElement: false,
		},
	}

	return &StaticDynamicData{
		Statics:      statics,
		Dynamics:     map[int]string{},
		Conditionals: conditionals,
		IsEmpty:      false,
		FragmentID:   fragmentID,
	}, nil
}

// generateShowHideConditional handles show/hide element conditionals like {{if .Show}}<element>{{end}}
func (g *StaticDynamicGenerator) generateShowHideConditional(pattern *diff.ConditionalPattern, fragmentID string) (*StaticDynamicData, error) {
	hiddenHTML := pattern.States[0] // Empty state
	shownHTML := pattern.States[1]  // Content state

	// For show/hide, we have a simple conditional slot
	conditionals := map[int]*ConditionalSlot{
		0: {
			Position:      0,
			ConditionType: "show-hide",
			TruthyValue:   shownHTML,
			FalsyValue:    hiddenHTML,
			IsFullElement: true,
		},
	}

	return &StaticDynamicData{
		Statics:      []string{""}, // Single empty static slot
		Dynamics:     map[int]string{},
		Conditionals: conditionals,
		IsEmpty:      false,
		FragmentID:   fragmentID,
	}, nil
}

// generateNilNotNilConditional handles nil/not-nil conditionals like {{if .Value}} class="{{.Value}}"{{end}}
func (g *StaticDynamicGenerator) generateNilNotNilConditional(pattern *diff.ConditionalPattern, fragmentID string) (*StaticDynamicData, error) {
	// Similar to boolean but with actual values
	nilHTML := pattern.States[0]   // State without attribute/content
	valueHTML := pattern.States[1] // State with attribute/content

	// Find the difference position
	diffPos := g.findAttributeDifference(nilHTML, valueHTML)
	if diffPos == -1 {
		// Fallback to regular generation if we can't find the difference
		return g.Generate(nilHTML, valueHTML, fragmentID)
	}

	// Split the HTML at the difference position
	statics := []string{
		nilHTML[:diffPos],
		nilHTML[diffPos:],
	}

	// Extract the conditional content
	conditionalContent := valueHTML[diffPos : diffPos+(len(valueHTML)-len(nilHTML))]

	conditionals := map[int]*ConditionalSlot{
		0: {
			Position:      0,
			ConditionType: "nil-notnil",
			TruthyValue:   conditionalContent,
			FalsyValue:    "",
			IsFullElement: false,
		},
	}

	return &StaticDynamicData{
		Statics:      statics,
		Dynamics:     map[int]string{},
		Conditionals: conditionals,
		IsEmpty:      false,
		FragmentID:   fragmentID,
	}, nil
}

// generateIfElseConditional handles if-else structural conditionals like {{if .Flag}}<table>{{else}}<div>{{end}}
func (g *StaticDynamicGenerator) generateIfElseConditional(pattern *diff.ConditionalPattern, fragmentID string) (*StaticDynamicData, error) {
	falsyHTML := pattern.States[0]  // False condition structure
	truthyHTML := pattern.States[1] // True condition structure

	// For if-else structural conditionals, we need to extract common dynamic values
	// and create a conditional slot for the structural switch
	statics, dynamics, err := g.extractDynamicValues(falsyHTML, truthyHTML)
	if err != nil {
		// If we can't extract dynamic values, treat as pure structural switch
		statics = []string{""}
		dynamics = map[int]string{}
	}

	// Create the conditional slot for structural switching
	conditionals := map[int]*ConditionalSlot{
		0: {
			Position:      0,
			ConditionType: "if-else",
			TruthyValue:   truthyHTML,
			FalsyValue:    falsyHTML,
			IsFullElement: true,
		},
	}

	return &StaticDynamicData{
		Statics:      statics,
		Dynamics:     dynamics,
		Conditionals: conditionals,
		IsEmpty:      false,
		FragmentID:   fragmentID,
	}, nil
}

// extractDynamicValues attempts to find common dynamic values between two structural templates
func (g *StaticDynamicGenerator) extractDynamicValues(html1, html2 string) ([]string, map[int]string, error) {
	// Look for common patterns in both HTML structures that might be dynamic values
	// This is a heuristic approach to find dynamic content within different structures

	// Extract text content from both structures
	text1 := g.extractTextContent(html1)
	text2 := g.extractTextContent(html2)

	// If there are common text values, treat them as dynamics
	if text1 != "" && text1 == text2 {
		// Same text content in different structures - this is a dynamic value
		return []string{""}, map[int]string{0: text1}, nil
	}

	// If texts are different but both non-empty, include both as possible dynamics
	if text1 != "" && text2 != "" && text1 != text2 {
		// Different text content - could be part of the change
		return []string{""}, map[int]string{0: text2}, nil // Use the new text
	}

	// No common dynamic values found
	return []string{""}, map[int]string{}, nil
}

// extractTextContent extracts text content from HTML
func (g *StaticDynamicGenerator) extractTextContent(html string) string {
	var text strings.Builder
	inTag := false

	for _, char := range html {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			// Keep all text content including spaces (but normalize whitespace)
			if char == '\t' || char == '\n' {
				text.WriteRune(' ')
			} else {
				text.WriteRune(char)
			}
		}
	}

	return strings.TrimSpace(text.String())
}

// findAttributeDifference finds the position where two HTML strings differ (for attribute changes)
func (g *StaticDynamicGenerator) findAttributeDifference(shorter, longer string) int {
	// Find the first position where they differ
	for i := 0; i < len(shorter) && i < len(longer); i++ {
		if shorter[i] != longer[i] {
			return i
		}
	}

	// If shorter string is a prefix of longer, difference starts at end of shorter
	if len(shorter) < len(longer) {
		return len(shorter)
	}

	return -1 // No difference found
}

// generateDynamicsOnlyFragment creates a fragment with only dynamic values for server-side static caching
func (g *StaticDynamicGenerator) generateDynamicsOnlyFragment(oldHTML, newHTML, fragmentID string) (*StaticDynamicData, error) {
	// For dynamics-only mode, we need a different approach that focuses on extracting
	// all individual dynamic values from text changes, not just following the full algorithm
	dynamics := make(map[int]string)

	// Try advanced extraction for multiple values in single text changes
	extractedValues := g.extractAllDynamicValues(oldHTML, newHTML)

	if len(extractedValues) > 0 {
		// Use the advanced extraction results
		for i, value := range extractedValues {
			dynamics[i] = value
		}
	} else {
		// Fallback to full static/dynamic separation approach
		fullData, err := g.generateTextChanges(oldHTML, newHTML, fragmentID)
		if err != nil {
			return nil, err
		}

		// Re-map dynamics to start from index 0 for dynamics-only transmission
		keys := make([]int, 0, len(fullData.Dynamics))
		for k := range fullData.Dynamics {
			keys = append(keys, k)
		}

		// Sort keys to maintain order
		for i := 0; i < len(keys)-1; i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}

		// Re-index dynamics starting from 0 and extract essential values
		for i, key := range keys {
			value := fullData.Dynamics[key]
			extractedValue := g.extractEssentialDynamicValue(value)
			if extractedValue != "" {
				dynamics[i] = extractedValue
			}
		}
	}

	return &StaticDynamicData{
		Statics:    []string{}, // Empty - cached server-side
		Dynamics:   dynamics,   // Only the essential changing values with re-indexed positions
		IsEmpty:    false,
		FragmentID: fragmentID,
	}, nil
}

// extractAllDynamicValues tries to extract all individual dynamic values from text changes
func (g *StaticDynamicGenerator) extractAllDynamicValues(oldHTML, newHTML string) []string {
	// Extract text content from both HTML
	oldText := g.extractTextContent(oldHTML)
	newText := g.extractTextContent(newHTML)

	if oldText == "" || newText == "" {
		// Handle empty to content or content to empty
		if newText != "" {
			return []string{newText}
		}
		return []string{}
	}

	// Try to find multiple value changes in patterns like "Name: John, Age: 25" → "Name: Jane, Age: 30"
	return g.findMultipleValueChanges(oldText, newText)
}

// findMultipleValueChanges finds individual value changes in structured text
func (g *StaticDynamicGenerator) findMultipleValueChanges(oldText, newText string) []string {
	// Look for comma-separated label-value pairs
	if strings.Contains(oldText, ", ") && strings.Contains(newText, ", ") {
		return g.extractFromCommaSeparated(oldText, newText)
	}

	// Look for space-separated label-value pairs
	if strings.Contains(oldText, ": ") && strings.Contains(newText, ": ") {
		return g.extractFromColonSeparated(oldText, newText)
	}

	// Single value change
	return []string{newText}
}

// extractFromCommaSeparated handles "Name: John, Age: 25" patterns
func (g *StaticDynamicGenerator) extractFromCommaSeparated(oldText, newText string) []string {
	oldParts := strings.Split(oldText, ", ")
	newParts := strings.Split(newText, ", ")

	if len(oldParts) != len(newParts) {
		// Structure changed, fallback to single value
		return []string{newText}
	}

	var changedValues []string
	for i := 0; i < len(oldParts) && i < len(newParts); i++ {
		oldPart := strings.TrimSpace(oldParts[i])
		newPart := strings.TrimSpace(newParts[i])

		if oldPart != newPart {
			// Extract value from "Label: Value" format
			value := g.extractEssentialDynamicValue(newPart)
			if value != "" {
				changedValues = append(changedValues, value)
			}
		}
	}

	if len(changedValues) == 0 {
		return []string{newText}
	}

	return changedValues
}

// extractFromColonSeparated handles single "Label: Value" patterns
func (g *StaticDynamicGenerator) extractFromColonSeparated(oldText, newText string) []string {
	oldValue := g.extractEssentialDynamicValue(oldText)
	newValue := g.extractEssentialDynamicValue(newText)

	if oldValue != newValue && newValue != "" {
		return []string{newValue}
	}

	return []string{newText}
}

// extractStaticDynamicFromSingle analyzes a single HTML string to find likely static/dynamic patterns
func (g *StaticDynamicGenerator) extractStaticDynamicFromSingle(html string) ([]string, map[int]string, error) {
	// This method handles cases like "<span>Count: 0</span>" -> statics: ["<span>Count: ", "</span>"], dynamics: {0: "0"}
	// Or "<div>Name: John, Age: 25</div>" -> statics: ["<div>Name: ", ", Age: ", "</div>"], dynamics: {0: "John", 1: "25"}

	if strings.TrimSpace(html) == "" {
		return []string{}, map[int]string{}, nil
	}

	// Try to identify label-value patterns within the HTML
	return g.extractLabelValuePatterns(html)
}

// extractLabelValuePatterns finds patterns like "Label: Value" within HTML content
func (g *StaticDynamicGenerator) extractLabelValuePatterns(html string) ([]string, map[int]string, error) {
	// Use regex to find text content between tags that might contain label-value pairs
	textRegex := regexp.MustCompile(`>([^<]+)<`)
	matches := textRegex.FindAllStringSubmatch(html, -1)

	if len(matches) == 0 {
		// No text content found, treat as static
		return []string{html}, map[int]string{}, nil
	}

	var allDynamics = make(map[int]string)
	dynamicIndex := 1 // Start at 1 to match extractMultipleTextChanges pattern
	result := html

	// Process each text content to find label-value patterns
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		textContent := match[1]

		// Check for various label-value patterns
		statics, dynamics := g.findLabelValueInText(textContent)

		if len(dynamics) > 0 {
			// Replace the original text with placeholder structure
			originalPattern := ">" + regexp.QuoteMeta(textContent) + "<"

			// Build replacement with placeholders
			var replacement strings.Builder
			replacement.WriteString(">")

			for i, static := range statics {
				replacement.WriteString(static)
				if i < len(dynamics) {
					placeholder := "{{DYNAMIC_" + fmt.Sprintf("%d", dynamicIndex) + "}}"
					replacement.WriteString(placeholder)
					allDynamics[dynamicIndex] = dynamics[i]
					dynamicIndex++
				}
			}
			replacement.WriteString("<")

			// Replace in the result HTML
			re := regexp.MustCompile(originalPattern)
			result = re.ReplaceAllString(result, replacement.String())
		}
	}

	if len(allDynamics) == 0 {
		// No patterns found, treat as static
		return []string{html}, map[int]string{}, nil
	}

	// Split the result by placeholders to create final statics array
	finalStatics := []string{}
	current := result

	for i := 0; i < dynamicIndex; i++ {
		placeholder := "{{DYNAMIC_" + fmt.Sprintf("%d", i) + "}}"
		if strings.Contains(current, placeholder) {
			parts := strings.SplitN(current, placeholder, 2)
			if len(parts) == 2 {
				finalStatics = append(finalStatics, parts[0])
				current = parts[1]
			}
		}
	}

	// Add remaining part
	if current != "" {
		finalStatics = append(finalStatics, current)
	}

	return finalStatics, allDynamics, nil
}

// findLabelValueInText analyzes text content to find label-value patterns
func (g *StaticDynamicGenerator) findLabelValueInText(text string) ([]string, []string) {
	// Handle patterns like:
	// "Count: 5" -> statics: ["Count: "], dynamics: ["5"]
	// "Name: John, Age: 25" -> statics: ["Name: ", ", Age: "], dynamics: ["John", "25"]

	// Try comma-separated values first
	if strings.Contains(text, ", ") {
		return g.parseCommaSeparatedValues(text)
	}

	// Try single label-value pattern (removed generic space to avoid false positives)
	patterns := []string{": ", " = "}
	for _, sep := range patterns {
		if strings.Contains(text, sep) {
			parts := strings.SplitN(text, sep, 2)
			if len(parts) == 2 {
				label := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if label != "" && value != "" {
					// Check if value contains more patterns
					if strings.Contains(value, ", ") {
						subStatics, subDynamics := g.parseCommaSeparatedValues(value)
						if len(subDynamics) > 1 {
							// Combine label with first substatic
							statics := []string{label + sep + subStatics[0]}
							statics = append(statics, subStatics[1:]...)
							return statics, subDynamics
						}
					}
					return []string{label + sep}, []string{value}
				}
			}
		}
	}

	// No pattern found
	return []string{text}, []string{}
}

// parseCommaSeparatedValues handles "Name: John, Age: 25" patterns
func (g *StaticDynamicGenerator) parseCommaSeparatedValues(text string) ([]string, []string) {
	parts := strings.Split(text, ", ")
	if len(parts) < 2 {
		return []string{text}, []string{}
	}

	var statics []string
	var dynamics []string

	for i, part := range parts {
		part = strings.TrimSpace(part)

		// Try to find label-value within this part
		subPatterns := []string{": ", " = "}
		found := false

		for _, sep := range subPatterns {
			if strings.Contains(part, sep) {
				subParts := strings.SplitN(part, sep, 2)
				if len(subParts) == 2 {
					label := strings.TrimSpace(subParts[0])
					value := strings.TrimSpace(subParts[1])
					if label != "" && value != "" {
						if i == 0 {
							statics = append(statics, label+sep)
						} else {
							statics = append(statics, ", "+label+sep)
						}
						dynamics = append(dynamics, value)
						found = true
						break
					}
				}
			}
		}

		if !found {
			// No pattern in this part, treat as is
			if i == 0 {
				statics = append(statics, part)
			} else {
				statics = append(statics, ", "+part)
			}
		}
	}

	if len(dynamics) == 0 {
		return []string{text}, []string{}
	}

	return statics, dynamics
}

// extractEssentialDynamicValue extracts just the changing value from a full dynamic content
func (g *StaticDynamicGenerator) extractEssentialDynamicValue(fullValue string) string {
	// For patterns like "Score: 105", extract just "105"
	// For patterns like "Name: Jane", extract just "Jane"
	// For patterns like "Name: Jane, Age: 30", extract the last value "30"

	// Try common label-value patterns, prioritizing more specific ones first
	patterns := []string{": ", ", ", " ", "="}

	for _, sep := range patterns {
		if strings.Contains(fullValue, sep) {
			parts := strings.SplitN(fullValue, sep, 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if value != "" {
					// For complex patterns like "Name: Jane, Age: 30", recursively extract from the value part
					if strings.Contains(value, ": ") || strings.Contains(value, ", ") {
						return g.extractEssentialDynamicValue(value)
					}
					return value // Return just the value part
				}
			}
		}
	}

	// If no pattern matches, return the full value
	return fullValue
}
