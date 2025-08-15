package strategy

import (
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

// StaticDynamicGenerator implements Strategy 1 fragment generation
type StaticDynamicGenerator struct {
	// Add any configuration if needed
}

// NewStaticDynamicGenerator creates a new Strategy 1 generator
func NewStaticDynamicGenerator() *StaticDynamicGenerator {
	return &StaticDynamicGenerator{}
}

// Generate creates a static/dynamic fragment from old and new HTML
func (g *StaticDynamicGenerator) Generate(oldHTML, newHTML, fragmentID string) (*StaticDynamicData, error) {
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
	// For show content, we just need to send the new HTML as a single static segment
	return &StaticDynamicData{
		Statics:    []string{newHTML},
		Dynamics:   map[int]string{},
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
	// For now, implement a simple version that works for basic text changes
	// This can be enhanced with more sophisticated diff algorithms later

	// If the HTML structure is the same, find text differences
	if g.hasSameStructure(oldHTML, newHTML) {
		return g.extractTextOnlyChanges(oldHTML, newHTML)
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

	for i, static := range data.Statics {
		// Insert dynamic content first if it exists for this position
		if dynamic, exists := data.Dynamics[i]; exists {
			result.WriteString(dynamic)
		}

		// Then add the static content
		result.WriteString(static)
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
		} else if !inTag && char != ' ' && char != '\t' && char != '\n' {
			text.WriteRune(char)
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
