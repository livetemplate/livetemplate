package strategy

import (
	"strings"
)

// ReplacementData represents Strategy 4 fragment data for complex structural changes
type ReplacementData struct {
	// Content contains the complete HTML content to replace the fragment
	Content string `json:"content"`

	// IsEmpty indicates if this represents an empty state (complete removal)
	IsEmpty bool `json:"is_empty,omitempty"`

	// FragmentID identifies this fragment for client reconstruction
	FragmentID string `json:"fragment_id"`

	// Reason explains why fragment replacement was chosen (for debugging/optimization)
	Reason string `json:"reason,omitempty"`

	// Complexity indicates the complexity level that triggered fallback to Strategy 4
	Complexity string `json:"complexity,omitempty"` // "mixed", "recursive", "unpredictable", "unknown"
}

// FragmentReplacer implements Strategy 4 fragment replacement for complex structural changes
type FragmentReplacer struct {
	// compressionEnabled indicates if content compression should be applied
	compressionEnabled bool
}

// NewFragmentReplacer creates a new Strategy 4 fragment replacer
func NewFragmentReplacer() *FragmentReplacer {
	return &FragmentReplacer{
		compressionEnabled: true, // Enable compression by default for bandwidth optimization
	}
}

// Compile creates a replacement fragment from old and new HTML for complex structural changes
func (fr *FragmentReplacer) Compile(oldHTML, newHTML, fragmentID string) (*ReplacementData, error) {
	// Handle empty state scenarios first
	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) != "" {
		// Show content scenario
		return fr.compileShowContent(newHTML, fragmentID)
	}

	if strings.TrimSpace(oldHTML) != "" && strings.TrimSpace(newHTML) == "" {
		// Hide content scenario
		return fr.compileHideContent(fragmentID)
	}

	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) == "" {
		// Both empty - no replacement needed
		return &ReplacementData{
			Content:    "",
			IsEmpty:    true,
			FragmentID: fragmentID,
			Reason:     "Both old and new content are empty",
			Complexity: "none",
		}, nil
	}

	// Normal fragment replacement for complex structural changes
	return fr.compileReplacement(oldHTML, newHTML, fragmentID)
}

// compileShowContent handles showing previously hidden content
func (fr *FragmentReplacer) compileShowContent(newHTML, fragmentID string) (*ReplacementData, error) {
	// For show content, replace with the complete new content
	content := newHTML
	if fr.compressionEnabled {
		content = fr.optimizeContent(newHTML)
	}

	return &ReplacementData{
		Content:    content,
		IsEmpty:    false,
		FragmentID: fragmentID,
		Reason:     "Show previously hidden content",
		Complexity: "empty-to-content",
	}, nil
}

// compileHideContent handles hiding previously shown content
func (fr *FragmentReplacer) compileHideContent(fragmentID string) (*ReplacementData, error) {
	// For hide content, replace with empty content
	return &ReplacementData{
		Content:    "",
		IsEmpty:    true,
		FragmentID: fragmentID,
		Reason:     "Hide previously shown content",
		Complexity: "content-to-empty",
	}, nil
}

// compileReplacement creates replacement data for complex structural changes
func (fr *FragmentReplacer) compileReplacement(oldHTML, newHTML, fragmentID string) (*ReplacementData, error) {
	// Analyze the complexity of the change to provide context
	complexity := fr.analyzeComplexity(oldHTML, newHTML)
	reason := fr.generateReason(oldHTML, newHTML, complexity)

	// Apply content optimization if enabled
	content := newHTML
	if fr.compressionEnabled {
		content = fr.optimizeContent(newHTML)
	}

	return &ReplacementData{
		Content:    content,
		IsEmpty:    false,
		FragmentID: fragmentID,
		Reason:     reason,
		Complexity: complexity,
	}, nil
}

// analyzeComplexity analyzes the complexity of the change that required fallback to Strategy 4
func (fr *FragmentReplacer) analyzeComplexity(oldHTML, newHTML string) string {
	oldTrimmed := strings.TrimSpace(oldHTML)
	newTrimmed := strings.TrimSpace(newHTML)

	// Check for different types of complexity in priority order
	if strings.Contains(oldTrimmed, "{{") || strings.Contains(newTrimmed, "{{") {
		return "template-functions" // Contains template functions
	}

	if fr.hasRecursiveStructure(oldTrimmed, newTrimmed) {
		return "recursive-structure" // Deeply nested or recursive changes
	}

	if fr.hasUnpredictableChanges(oldTrimmed, newTrimmed) {
		return "unpredictable" // Changes that don't follow predictable patterns
	}

	// Check for complex structural changes before mixed changes
	// Complex structural means significant structural reorganization
	if fr.hasComplexStructuralChanges(oldTrimmed, newTrimmed) {
		return "complex-structural" // Complex structural reorganization
	}

	if fr.hasMixedChanges(oldTrimmed, newTrimmed) {
		return "mixed-changes" // Multiple types of changes (text + attribute + structural)
	}

	return "complex-structural" // General complex structural changes
}

// hasMixedChanges checks if the change involves multiple types of modifications
// but is not unpredictable or complex-structural
func (fr *FragmentReplacer) hasMixedChanges(oldHTML, newHTML string) bool {
	// Only classify as mixed-changes if it's not already classified as unpredictable or complex-structural

	// Simple heuristic: if there are significant differences in multiple aspects
	hasTagChanges := fr.hasTagStructureChanges(oldHTML, newHTML)
	hasAttributeChanges := fr.hasAttributeChanges(oldHTML, newHTML)
	hasTextChanges := fr.hasTextChanges(oldHTML, newHTML)

	// Mixed changes: at least 2 different types of changes
	changeTypes := 0
	if hasTagChanges {
		changeTypes++
	}
	if hasAttributeChanges {
		changeTypes++
	}
	if hasTextChanges {
		changeTypes++
	}

	// Only return true for mixed changes if it has multiple change types
	// but doesn't meet the criteria for unpredictable or complex-structural
	if changeTypes >= 2 {
		// Make sure it's not already classified as unpredictable or complex-structural
		similarity := fr.calculateSimilarity(oldHTML, newHTML)

		// If similarity is very low, it might be unpredictable (will be caught earlier)
		if similarity < 0.3 {
			return false // Let unpredictable classification handle this
		}

		// If it has significant depth changes, it might be complex-structural (will be caught earlier)
		oldDepth := fr.calculateNestingDepth(oldHTML)
		newDepth := fr.calculateNestingDepth(newHTML)
		if (newDepth - oldDepth) >= 2 {
			return false // Let complex-structural classification handle this
		}

		return true // It's genuinely mixed changes
	}

	return false
}

// hasTagStructureChanges checks if there are structural tag changes
func (fr *FragmentReplacer) hasTagStructureChanges(oldHTML, newHTML string) bool {
	// Count opening tags in both versions
	oldTagCount := strings.Count(oldHTML, "<") - strings.Count(oldHTML, "</")
	newTagCount := strings.Count(newHTML, "<") - strings.Count(newHTML, "</")

	// Significant difference in tag structure
	return abs(oldTagCount-newTagCount) > 1
}

// hasAttributeChanges checks if there are attribute changes
func (fr *FragmentReplacer) hasAttributeChanges(oldHTML, newHTML string) bool {
	// Simple heuristic: different number of attributes or different attribute patterns
	oldAttrCount := strings.Count(oldHTML, "=")
	newAttrCount := strings.Count(newHTML, "=")

	return abs(oldAttrCount-newAttrCount) > 0
}

// hasTextChanges checks if there are text content changes
func (fr *FragmentReplacer) hasTextChanges(oldHTML, newHTML string) bool {
	// Extract text content (simplified)
	oldText := fr.extractTextContent(oldHTML)
	newText := fr.extractTextContent(newHTML)

	return oldText != newText
}

// hasRecursiveStructure checks if the change involves deeply nested structures
func (fr *FragmentReplacer) hasRecursiveStructure(oldHTML, newHTML string) bool {
	// Check nesting depth
	maxOldDepth := fr.calculateNestingDepth(oldHTML)
	maxNewDepth := fr.calculateNestingDepth(newHTML)

	// Consider it recursive if deep nesting (>4 levels) or significant depth change
	return maxOldDepth > 4 || maxNewDepth > 4 || abs(maxOldDepth-maxNewDepth) > 2
}

// hasUnpredictableChanges checks if changes don't follow predictable patterns
func (fr *FragmentReplacer) hasUnpredictableChanges(oldHTML, newHTML string) bool {
	// Changes are unpredictable if they don't have clear patterns
	// This is a heuristic - real implementation could be more sophisticated

	// Check for complete tag type change (e.g., table → form)
	oldRootTag := fr.extractRootTag(oldHTML)
	newRootTag := fr.extractRootTag(newHTML)

	if oldRootTag != "" && newRootTag != "" && oldRootTag != newRootTag {
		// Different root tags suggest unpredictable change
		// But first check if it's actually complex structural (which should take precedence)

		// If there's significant nesting depth change, it's complex structural, not unpredictable
		oldDepth := fr.calculateNestingDepth(oldHTML)
		newDepth := fr.calculateNestingDepth(newHTML)
		if (newDepth - oldDepth) >= 1 {
			return false // Let complex-structural handle this
		}

		// Check for semantic tag category changes (which indicate unpredictable rewrites)
		if fr.isSemanticTagCategoryChange(oldRootTag, newRootTag) {
			// Semantic category changes with different content = unpredictable
			oldText := fr.extractTextContent(oldHTML)
			newText := fr.extractTextContent(newHTML)

			if oldText != newText && !strings.Contains(newText, oldText) {
				return true // Complete rewrite with semantic change = unpredictable
			}
		}
	}

	return false
}

// isSemanticTagCategoryChange checks if tags represent fundamentally different semantic categories
func (fr *FragmentReplacer) isSemanticTagCategoryChange(oldTag, newTag string) bool {
	// Define semantic categories
	formTags := map[string]bool{"form": true, "input": true, "button": true, "select": true, "textarea": true}
	tableTags := map[string]bool{"table": true, "tr": true, "td": true, "th": true, "thead": true, "tbody": true}
	listTags := map[string]bool{"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true}
	contentTags := map[string]bool{"div": true, "section": true, "article": true, "header": true, "footer": true, "main": true, "aside": true}

	// Check if tags are from different semantic categories
	oldInForms := formTags[oldTag]
	newInForms := formTags[newTag]
	oldInTables := tableTags[oldTag]
	newInTables := tableTags[newTag]
	oldInLists := listTags[oldTag]
	newInLists := listTags[newTag]
	oldInContent := contentTags[oldTag]
	newInContent := contentTags[newTag]

	// Same category = not unpredictable
	if (oldInForms && newInForms) || (oldInTables && newInTables) ||
		(oldInLists && newInLists) || (oldInContent && newInContent) {
		return false
	}

	// Different categories = potentially unpredictable
	return true
}

// hasComplexStructuralChanges checks for complex structural reorganization
func (fr *FragmentReplacer) hasComplexStructuralChanges(oldHTML, newHTML string) bool {
	// Complex structural changes involve significant nesting changes
	// but are not necessarily unpredictable (they follow structural patterns)

	oldDepth := fr.calculateNestingDepth(oldHTML)
	newDepth := fr.calculateNestingDepth(newHTML)

	// Significant increase in nesting depth suggests complex structural change
	depthIncrease := newDepth - oldDepth
	if depthIncrease >= 1 { // Lowered threshold from 2 to 1
		// Also check that it's a significant structural change
		if fr.hasTagStructureChanges(oldHTML, newHTML) {
			return true
		}
	}

	// Check for significant structural reorganization even without depth change
	// if there are many structural changes
	if fr.hasTagStructureChanges(oldHTML, newHTML) {
		// Count how many tags changed
		oldTagCount := strings.Count(oldHTML, "<") - strings.Count(oldHTML, "</")
		newTagCount := strings.Count(newHTML, "<") - strings.Count(newHTML, "</")

		// If there's a significant difference in tag structure, it's complex structural
		if abs(oldTagCount-newTagCount) >= 2 {
			return true
		}
	}

	return false
}

// extractRootTag extracts the first HTML tag from the content
func (fr *FragmentReplacer) extractRootTag(html string) string {
	html = strings.TrimSpace(html)
	if html == "" || !strings.HasPrefix(html, "<") {
		return ""
	}

	endPos := strings.Index(html, ">")
	if endPos == -1 {
		return ""
	}

	tagContent := html[1:endPos]
	// Remove attributes, just get tag name
	spacePos := strings.Index(tagContent, " ")
	if spacePos != -1 {
		tagContent = tagContent[:spacePos]
	}

	return tagContent
}

// generateReason generates a human-readable reason for why Strategy 4 was chosen
func (fr *FragmentReplacer) generateReason(oldHTML, newHTML, complexity string) string {
	switch complexity {
	case "template-functions":
		return "Complex template functions require full replacement for compatibility"
	case "mixed-changes":
		return "Mixed structural, attribute, and text changes require full replacement"
	case "recursive-structure":
		return "Deep recursive structure changes require full replacement"
	case "unpredictable":
		return "Unpredictable change patterns require full replacement for reliability"
	case "complex-structural":
		return "Complex structural changes require full replacement"
	default:
		return "Complex changes require full fragment replacement"
	}
}

// optimizeContent applies basic content optimizations for bandwidth reduction
func (fr *FragmentReplacer) optimizeContent(content string) string {
	// Basic optimizations that don't change functionality
	optimized := content

	// Remove redundant whitespace (but preserve single spaces)
	optimized = strings.ReplaceAll(optimized, "\n", "")
	optimized = strings.ReplaceAll(optimized, "\t", "")

	// Remove extra spaces between tags
	for strings.Contains(optimized, ">  <") {
		optimized = strings.ReplaceAll(optimized, ">  <", "> <")
	}
	for strings.Contains(optimized, "> <") {
		optimized = strings.ReplaceAll(optimized, "> <", "><")
	}

	return strings.TrimSpace(optimized)
}

// Helper functions for complexity analysis

// extractTextContent extracts text content from HTML (simplified)
func (fr *FragmentReplacer) extractTextContent(html string) string {
	// Simple text extraction - remove all tags
	result := html
	inTag := false
	var text strings.Builder

	for _, char := range result {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			text.WriteRune(char)
		}
	}

	return strings.TrimSpace(text.String())
}

// calculateNestingDepth calculates the maximum nesting depth of HTML tags
func (fr *FragmentReplacer) calculateNestingDepth(html string) int {
	depth := 0
	maxDepth := 0
	inTag := false
	var tagBuilder strings.Builder

	for _, char := range html {
		if char == '<' {
			inTag = true
			tagBuilder.Reset()
		} else if char == '>' {
			inTag = false
			tag := tagBuilder.String()

			if strings.HasPrefix(tag, "/") {
				// Closing tag
				depth--
			} else if !strings.HasSuffix(tag, "/") && !strings.Contains(tag, " ") {
				// Opening tag (not self-closing, simplified check)
				depth++
				if depth > maxDepth {
					maxDepth = depth
				}
			}
		} else if inTag {
			tagBuilder.WriteRune(char)
		}
	}

	return maxDepth
}

// calculateSimilarity calculates a simple similarity score between two strings
func (fr *FragmentReplacer) calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 && len(s2) == 0 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Simple character-based similarity using common characters
	charCount1 := make(map[rune]int)
	charCount2 := make(map[rune]int)

	for _, char := range s1 {
		charCount1[char]++
	}

	for _, char := range s2 {
		charCount2[char]++
	}

	commonChars := 0
	totalChars := 0

	// Count common characters
	for char, count1 := range charCount1 {
		count2, exists := charCount2[char]
		if exists {
			if count1 < count2 {
				commonChars += count1
			} else {
				commonChars += count2
			}
		}
		totalChars += count1
	}

	for char, count2 := range charCount2 {
		if _, exists := charCount1[char]; !exists {
			totalChars += count2
		}
	}

	if totalChars == 0 {
		return 1.0
	}

	return float64(commonChars*2) / float64(totalChars)
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CalculateBandwidthReduction calculates the bandwidth savings for fragment replacement
func (fr *FragmentReplacer) CalculateBandwidthReduction(originalSize int, data *ReplacementData) float64 {
	// Calculate the size of the replacement data
	replacementSize := fr.calculateReplacementSize(data)

	if originalSize == 0 {
		return 0.0
	}

	// For Strategy 4, we compare against full page reload (not just the fragment)
	// Calculate estimated full page size based on fragment size and typical web page structure
	estimatedFullPageSize := fr.estimateFullPageSize(originalSize)

	reduction := float64(estimatedFullPageSize-replacementSize) / float64(estimatedFullPageSize) * 100
	if reduction < 0 {
		return 0.0
	}

	return reduction
}

// estimateFullPageSize estimates the full page size based on fragment size
func (fr *FragmentReplacer) estimateFullPageSize(fragmentSize int) int {
	// Base page structure (HTML, head, meta tags, CSS, JS)
	basePageSize := 2000 // Conservative estimate for minimal page structure

	// Fragment represents part of the body content
	// Typical fragments are 10-30% of total page content
	// So if fragment is X, full page content is roughly 3-10x larger
	contentMultiplier := 5.0 // Conservative middle ground

	// Add typical additional page assets
	typicalCSSSize := 1500  // CSS overhead
	typicalJSSize := 2000   // JavaScript overhead
	typicalImageSize := 500 // Image references and small images

	estimatedContentSize := float64(fragmentSize) * contentMultiplier
	totalPageSize := basePageSize + int(estimatedContentSize) + typicalCSSSize + typicalJSSize + typicalImageSize

	return totalPageSize
}

// calculateReplacementSize estimates the size of replacement data when transmitted
func (fr *FragmentReplacer) calculateReplacementSize(data *ReplacementData) int {
	size := 0

	// Content size (the main component)
	size += len(data.Content)

	// Metadata overhead (fragment ID, reason, complexity)
	size += len(data.FragmentID) + 10
	size += len(data.Reason) + 10
	size += len(data.Complexity) + 10

	// JSON structure overhead
	size += 20 // Minimal JSON structure

	// For empty states, just the signal
	if data.IsEmpty {
		size = 15 // Just the empty state signal
	}

	return size
}

// ApplyReplacement applies fragment replacement to reconstruct HTML (for testing)
func (fr *FragmentReplacer) ApplyReplacement(originalHTML string, data *ReplacementData) string {
	if data.IsEmpty {
		return ""
	}

	// For Strategy 4, we completely replace the content
	return data.Content
}

// SetCompressionEnabled enables or disables content compression
func (fr *FragmentReplacer) SetCompressionEnabled(enabled bool) {
	fr.compressionEnabled = enabled
}

// IsCompressionEnabled returns whether content compression is enabled
func (fr *FragmentReplacer) IsCompressionEnabled() bool {
	return fr.compressionEnabled
}
