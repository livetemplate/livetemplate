package page

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Forward reference to avoid circular dependencies
// FragmentConfig is defined in page.go

// TemplateRegion represents a dynamic region within a full HTML template
type TemplateRegion struct {
	ID             string   // Generated LiveTemplate identifier (e.g., "lvt_0")
	TemplateSource string   // The template fragment (e.g., "Hello {{.Counter}}")
	StartMarker    string   // HTML element start (e.g., "<div>")
	EndMarker      string   // HTML element end (e.g., "</div>")
	FieldPaths     []string // Template fields used (e.g., [".Counter"])
	ElementTag     string   // The HTML tag name (e.g., "div", "span")
	OriginalAttrs  string   // Original attributes to preserve
}

// detectTemplateRegions analyzes a full HTML template and identifies dynamic regions
func (p *Page) detectTemplateRegions() ([]TemplateRegion, error) {
	if p.template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	// Use the original template source (not the potentially contaminated one from template object)
	var templateSource string
	if p.templateSource != "" {
		// Use the pre-stored original template source
		templateSource = p.templateSource
	} else {
		// Fallback: extract from template (may contain security pipelines)
		var err error
		templateSource, err = p.extractTemplateSourceFromTemplate(p.template)
		if err != nil {
			return nil, fmt.Errorf("failed to extract template source: %w", err)
		}
	}

	// Find dynamic regions - both HTML elements with template expressions AND standalone template constructs
	regions := []TemplateRegion{}

	// Method 1: Find standalone template constructs like {{range}}, {{if}} that span elements
	// Process these FIRST as they have higher priority and define larger boundaries
	standaloneConstructs := findStandaloneTemplateConstructs(templateSource)

	regionIndex := 0

	// Process standalone template construct regions (Method 1 - Higher Priority)
	for _, construct := range standaloneConstructs {
		regionIndex++

		region := TemplateRegion{
			ID:             generateGloballyUniqueFragmentID(),
			TemplateSource: construct.TemplateSource,
			StartMarker:    construct.StartMarker,
			EndMarker:      construct.EndMarker,
			FieldPaths:     construct.FieldPaths,
			ElementTag:     construct.ElementTag,
			OriginalAttrs:  construct.OriginalAttrs,
		}

		regions = append(regions, region)
	}

	// Method 2: Find HTML elements that contain template expressions by parsing properly
	matches := findCompleteElementMatches(templateSource)
	// Filter to only get leaf elements (elements that don't contain other template-containing elements)
	leafMatches := filterToLeafElements(matches)

	// Helper function to check if an element is already covered by existing regions
	isElementCoveredByExistingRegions := func(elementContent string, elementStart int) bool {
		// Check if this element is inside any of the already created regions
		for _, region := range regions {
			// Check if the element content appears within this region's template source
			if strings.Contains(region.TemplateSource, elementContent) {
				return true
			}
		}
		return false
	}

	// Process HTML element-based regions (Method 2 - Lower Priority)
	for _, match := range leafMatches {
		if len(match) >= 5 {
			startTagName := match[1]
			attributes := match[2]
			content := match[3]
			endTagName := match[4]

			// Ensure start and end tags match
			if startTagName != endTagName {
				continue
			}

			// Check if either attributes or content contain template expressions
			templatePattern := regexp.MustCompile(`\{\{[^}]+\}\}`)
			hasTemplateInAttributes := templatePattern.MatchString(attributes)
			hasTemplateInContent := templatePattern.MatchString(content)

			if !hasTemplateInAttributes && !hasTemplateInContent {
				continue
			}

			// Skip regions that are already covered by standalone constructs - prevents overlap
			// Handle self-closing elements correctly
			var fullElement string
			isSelfClosing := content == "" && (startTagName == "input" || startTagName == "img" ||
				startTagName == "meta" || startTagName == "link" || startTagName == "br" || startTagName == "hr")
			if isSelfClosing {
				fullElement = fmt.Sprintf("<%s%s>", startTagName, attributes)
			} else {
				fullElement = fmt.Sprintf("<%s%s>%s</%s>", startTagName, attributes, content, endTagName)
			}
			if isElementCoveredByExistingRegions(fullElement, 0) {
				continue
			}

			// Extract ID from tag attributes if available, otherwise generate ID
			extractedID := extractIDFromTag(attributes)
			var lvtID string
			if extractedID != "" {
				lvtID = extractedID
			} else {
				// Generate globally unique random ID instead of predictable sequential ID
				lvtID = generateGloballyUniqueFragmentID()
				regionIndex++
			}

			// Extract field paths from template expressions in both attributes and content
			fieldPaths := extractFieldPaths(attributes + " " + content)

			// If attributes contain templates, include the entire element as template source
			var templateSource string
			var endMarker string
			if hasTemplateInAttributes {
				if isSelfClosing {
					templateSource = fmt.Sprintf("<%s%s>", startTagName, attributes)
					endMarker = "" // Self-closing elements don't have end markers
				} else {
					templateSource = fmt.Sprintf("<%s%s>%s</%s>", startTagName, attributes, content, endTagName)
					endMarker = fmt.Sprintf("</%s>", endTagName)
				}
			} else {
				templateSource = content
				endMarker = fmt.Sprintf("</%s>", endTagName)
			}

			region := TemplateRegion{
				ID:             lvtID,
				TemplateSource: templateSource,
				StartMarker:    fmt.Sprintf("<%s%s>", startTagName, attributes),
				EndMarker:      endMarker,
				FieldPaths:     fieldPaths,
				ElementTag:     startTagName,
				OriginalAttrs:  attributes,
			}

			regions = append(regions, region)
		}
	}

	return regions, nil
}

// extractIDFromTag extracts the ID attribute from an HTML tag
func extractIDFromTag(tag string) string {
	idRegex := regexp.MustCompile(`id=["']([^"']+)["']`)
	matches := idRegex.FindStringSubmatch(tag)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractFieldPaths finds all template field expressions in content
func extractFieldPaths(content string) []string {
	// Match all field references (including in complex expressions like if, range, etc.)
	fieldRegex := regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)`)
	matches := fieldRegex.FindAllStringSubmatch(content, -1)

	var fields []string
	fieldCounts := make(map[string]int) // Track field path counts for proper deduplication logic

	for _, match := range matches {
		if len(match) >= 2 {
			fullPath := match[1]
			// Extract only the first part of nested field paths
			// For "User.Name", extract "User"; for "Name", extract "Name"
			firstField := fullPath
			if dotIndex := strings.Index(fullPath, "."); dotIndex >= 0 {
				firstField = fullPath[:dotIndex]
			}

			rootField := "." + firstField
			originalField := "." + fullPath

			// Count occurrences of the original full field path
			fieldCounts[originalField]++

			// For nested fields, add the root field for each occurrence of different nested paths
			// For non-nested fields, deduplicate identical references
			if strings.Contains(fullPath, ".") {
				// Nested field: add root field for each different nested path
				fields = append(fields, rootField)
			} else {
				// Non-nested field: deduplicate identical references
				if fieldCounts[originalField] == 1 {
					fields = append(fields, rootField)
				}
			}
		}
	}

	return fields
}

// findCompleteElementMatches finds properly matched HTML elements with their complete content
// Handles both paired elements (<div>content</div>) and self-closing elements (<input/>)
func findCompleteElementMatches(templateSource string) [][]string {
	var matches [][]string
	templatePattern := regexp.MustCompile(`\{\{[^}]+\}\}`)

	// Method 1: Find all opening tags (both self-closing and regular)
	openTagRegex := regexp.MustCompile(`<(\w+)([^>]*?)(/?)>`)
	allTags := openTagRegex.FindAllStringSubmatch(templateSource, -1)
	allTagIndexes := openTagRegex.FindAllStringSubmatchIndex(templateSource, -1)

	for i, tagMatch := range allTags {
		if len(tagMatch) < 4 || len(allTagIndexes) <= i {
			continue
		}

		tagName := tagMatch[1]
		attributes := tagMatch[2]
		selfCloseSlash := tagMatch[3]

		openStart := allTagIndexes[i][0]
		openEnd := allTagIndexes[i][1]

		hasTemplateInAttributes := templatePattern.MatchString(attributes)

		// Check if this is a self-closing element
		isSelfClosing := selfCloseSlash == "/" ||
			tagName == "input" || tagName == "img" || tagName == "meta" ||
			tagName == "link" || tagName == "br" || tagName == "hr"

		if isSelfClosing {
			// Handle self-closing elements
			if hasTemplateInAttributes {
				fullMatch := templateSource[openStart:openEnd]
				match := []string{fullMatch, tagName, attributes, "", tagName}
				matches = append(matches, match)
			}
			continue
		}

		// Handle regular paired elements - find matching closing tag
		closeIndex := findMatchingCloseTag(templateSource, openEnd, tagName)
		if closeIndex == -1 {
			continue // No matching close tag found
		}

		// Extract the content between open and close tags
		content := templateSource[openEnd:closeIndex]
		hasTemplateInContent := templatePattern.MatchString(content)

		// Include element if it has template expressions in attributes OR content
		if hasTemplateInAttributes || hasTemplateInContent {
			closeTag := "</" + tagName + ">"
			fullMatch := templateSource[openStart : closeIndex+len(closeTag)]
			match := []string{fullMatch, tagName, attributes, content, tagName}
			matches = append(matches, match)
		}
	}

	return matches
}

// filterToLeafElements filters matches to only include leaf elements (no nested template-containing elements)
// Enhanced to be more inclusive for form elements and UI components
func filterToLeafElements(matches [][]string) [][]string {
	var leafMatches [][]string
	templatePattern := regexp.MustCompile(`\{\{[^}]+\}\}`)

	for i, match := range matches {
		if len(match) < 5 {
			continue
		}

		fullMatch := match[0]
		tagName := match[1]
		attributes := match[2]
		content := match[3]

		// Check if this element contains template expressions
		hasTemplateInAttributes := templatePattern.MatchString(attributes)
		hasTemplateInContent := templatePattern.MatchString(content)

		if !hasTemplateInAttributes && !hasTemplateInContent {
			continue // Skip elements without templates
		}

		// Always include interactive elements that have template expressions
		// These are typically leaf elements for user interaction
		isInteractiveElement := tagName == "input" || tagName == "button" || tagName == "select" ||
			tagName == "textarea" || tagName == "img" ||
			strings.Contains(attributes, `data-lvt-action`)

		if isInteractiveElement {
			leafMatches = append(leafMatches, match)
			continue
		}

		// For other elements with template expressions, only include if they're "leaf" elements
		// (no nested template-containing elements within them)

		// For other elements, check if any smaller template-containing element is nested within
		isLeaf := true

		for j, otherMatch := range matches {
			if i == j || len(otherMatch) < 5 {
				continue
			}

			otherFull := otherMatch[0]
			otherContent := otherMatch[3]
			otherAttrs := otherMatch[2]

			// Check if other element has templates
			otherHasTemplate := templatePattern.MatchString(otherContent) || templatePattern.MatchString(otherAttrs)

			// If this match contains another template-containing element, and the other element is smaller,
			// then this match is not a leaf element
			if otherHasTemplate && strings.Contains(fullMatch, otherFull) &&
				fullMatch != otherFull && len(otherFull) < len(fullMatch) {
				isLeaf = false
				break
			}
		}

		if isLeaf {
			leafMatches = append(leafMatches, match)
		}
	}

	return leafMatches
}

// findMatchingCloseTag finds the position of the matching closing tag, accounting for nested tags
func findMatchingCloseTag(source string, startPos int, tagName string) int {
	openTag := "<" + tagName
	closeTag := "</" + tagName + ">"

	pos := startPos
	depth := 1 // We're already inside one tag

	for pos < len(source) && depth > 0 {
		// Look for next occurrence of either open or close tag
		nextOpen := strings.Index(source[pos:], openTag)
		nextClose := strings.Index(source[pos:], closeTag)

		// Adjust positions to be absolute
		if nextOpen != -1 {
			nextOpen += pos
		}
		if nextClose != -1 {
			nextClose += pos
		}

		// Determine which comes first
		if nextClose == -1 {
			return -1 // No closing tag found
		}

		if nextOpen != -1 && nextOpen < nextClose {
			// Found opening tag before closing tag
			// Check if it's actually an opening tag (not part of attribute)
			if isActualOpenTag(source, nextOpen, tagName) {
				depth++
			}
			pos = nextOpen + len(openTag)
		} else {
			// Found closing tag
			depth--
			if depth == 0 {
				return nextClose // Found our matching close tag
			}
			pos = nextClose + len(closeTag)
		}
	}

	return -1 // No matching close tag found
}

// isActualOpenTag checks if a position is actually an opening tag (not part of an attribute)
func isActualOpenTag(source string, pos int, tagName string) bool {
	// Simple check: ensure it's followed by space or >
	tagEnd := pos + len("<"+tagName)
	if tagEnd >= len(source) {
		return false
	}

	nextChar := source[tagEnd]
	return nextChar == ' ' || nextChar == '>' || nextChar == '\n' || nextChar == '\t'
}

// generateRegionFragment creates a fragment update for a specific template region (with metadata)
func (p *Page) generateRegionFragment(region TemplateRegion, newData interface{}) (*Fragment, error) {
	config := &FragmentConfig{IncludeMetadata: true} // Legacy behavior includes metadata
	return p.generateRegionFragmentWithConfig(region, newData, config)
}

// generateRegionFragmentWithConfig creates a fragment update for a specific template region with config
func (p *Page) generateRegionFragmentWithConfig(region TemplateRegion, newData interface{}, config *FragmentConfig) (*Fragment, error) {
	// Use the tree generator on just this region
	oldData := p.data
	// Use region ID directly as fragment ID (it's already unique)
	fragmentID := region.ID

	treeResult, err := p.treeGenerator.GenerateFromTemplateSource(region.TemplateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate region fragment: %w", err)
	}

	fragment := &Fragment{
		ID:       fragmentID,
		Data:     treeResult,
		Metadata: nil, // Will be set conditionally below
	}

	// Add metadata only if requested
	if config.IncludeMetadata {
		fragment.Metadata = &Metadata{
			Strategy:     2, // Region-based strategy
			Confidence:   1.0,
			FallbackUsed: false,
		}
	}

	return fragment, nil
}

// generateGloballyUniqueFragmentID creates a cryptographically secure, globally unique fragment ID
func generateGloballyUniqueFragmentID() string {
	// Generate 8 random bytes (64 bits of entropy) for shorter but still globally unique IDs
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (should never happen in practice)
		return fmt.Sprintf("f%x", time.Now().UnixNano()&0xFFFFFFFFFFFFFF)
	}

	// Convert to hex string for a clean, globally unique ID (16 characters)
	// Add "f" prefix to ensure it starts with a letter for valid HTML/CSS IDs
	return "f" + hex.EncodeToString(bytes)
}

// findStandaloneTemplateConstructs finds template constructs like {{range}}, {{if}} that span across HTML elements
func findStandaloneTemplateConstructs(templateSource string) []TemplateRegion {
	var constructs []TemplateRegion

	// Find {{range}} constructs
	rangeConstructs := findRangeConstructs(templateSource)
	constructs = append(constructs, rangeConstructs...)

	// Find {{if}} constructs, but only include small/contained ones to avoid overlapping regions
	ifConstructs := findConditionalConstructs(templateSource)
	// Filter out large conditional constructs that likely span multiple elements
	filteredIfConstructs := filterSmallConditionalConstructs(ifConstructs)
	constructs = append(constructs, filteredIfConstructs...)

	return constructs
}

// filterSmallConditionalConstructs filters out large conditional constructs that could cause overlapping regions
func filterSmallConditionalConstructs(constructs []TemplateRegion) []TemplateRegion {
	var filtered []TemplateRegion

	for _, construct := range constructs {
		// Calculate size of the template source content
		contentSize := len(construct.TemplateSource)

		// Only allow small conditional constructs that are likely contained within single elements
		// The todo count div is around 80-120 characters, so we allow up to 200 chars
		if contentSize <= 200 {
			filtered = append(filtered, construct)
		}
	}

	return filtered
}

// findRangeConstructs specifically finds {{range}} template constructs
func findRangeConstructs(templateSource string) []TemplateRegion {
	var constructs []TemplateRegion

	// Pattern to match {{range ...}} ... {{end}} blocks
	rangePattern := regexp.MustCompile(`\{\{\s*range\s+([^}]+)\s*\}\}([\s\S]*?)\{\{\s*end\s*\}\}`)
	matches := rangePattern.FindAllStringSubmatch(templateSource, -1)
	matchIndexes := rangePattern.FindAllStringSubmatchIndex(templateSource, -1)

	for i, match := range matches {
		if len(match) < 3 || len(matchIndexes) <= i {
			continue
		}

		rangeExpression := strings.TrimSpace(match[1]) // e.g., ".Todos" or "$item := .Items"
		// rangeContent := match[2]                    // Content between {{range}} and {{end}} - not used yet

		// Extract field path from range expression
		fieldPaths := extractFieldPathsFromRangeExpression(rangeExpression)

		// Find the HTML container element that surrounds this range construct
		// Look backwards from the start of {{range}} to find opening tag
		// Look forwards from {{end}} to find closing tag
		rangeStart := matchIndexes[i][0]
		rangeEnd := matchIndexes[i][1]

		containerStart, containerTag, containerAttrs := findContainerElementStart(templateSource, rangeStart)
		containerEnd := findContainerElementEnd(templateSource, rangeEnd, containerTag)

		if containerStart == -1 || containerEnd == -1 {
			// No suitable container found, skip this construct
			continue
		}

		// Build template source for the entire range construct including container
		fullTemplateSource := templateSource[containerStart:containerEnd]

		construct := TemplateRegion{
			ID:             generateGloballyUniqueFragmentID(), // Will be set by caller
			TemplateSource: fullTemplateSource,
			StartMarker:    fmt.Sprintf("<%s%s>", containerTag, containerAttrs),
			EndMarker:      fmt.Sprintf("</%s>", containerTag),
			FieldPaths:     fieldPaths,
			ElementTag:     containerTag,
			OriginalAttrs:  containerAttrs,
		}

		constructs = append(constructs, construct)
	}

	return constructs
}

// findConditionalConstructs specifically finds {{if}} template constructs
func findConditionalConstructs(templateSource string) []TemplateRegion {
	var constructs []TemplateRegion

	// Pattern to match {{if ...}} ... {{end}} blocks
	ifPattern := regexp.MustCompile(`\{\{\s*if\s+([^}]+)\s*\}\}([\s\S]*?)\{\{\s*end\s*\}\}`)
	matches := ifPattern.FindAllStringSubmatch(templateSource, -1)
	matchIndexes := ifPattern.FindAllStringSubmatchIndex(templateSource, -1)

	for i, match := range matches {
		if len(match) < 3 || len(matchIndexes) <= i {
			continue
		}

		ifExpression := strings.TrimSpace(match[1]) // e.g., ".ShowError"
		ifContent := match[2]                       // Content between {{if}} and {{end}}

		// Extract field path from if expression
		fieldPaths := extractFieldPathsFromIfExpression(ifExpression)

		// For conditional constructs, find the immediate HTML container that wraps the conditional content
		// Look for HTML element that contains the {{if}} ... {{end}} block
		ifStart := matchIndexes[i][0]
		ifEnd := matchIndexes[i][1]

		// Find the element that directly contains this conditional construct
		containerStart, containerTag, containerAttrs := findImmediateContainerForConditional(templateSource, ifStart, ifEnd, ifContent)

		if containerStart == -1 {
			// No suitable container found, skip this construct
			continue
		}

		// Find the matching closing tag for this container
		containerEnd := findContainerElementEnd(templateSource, ifEnd, containerTag)

		if containerEnd == -1 {
			// No matching closing tag found, skip this construct
			continue
		}

		// Build template source for the entire if construct including container
		fullTemplateSource := templateSource[containerStart:containerEnd]

		construct := TemplateRegion{
			ID:             generateGloballyUniqueFragmentID(),
			TemplateSource: fullTemplateSource,
			StartMarker:    fmt.Sprintf("<%s%s>", containerTag, containerAttrs),
			EndMarker:      fmt.Sprintf("</%s>", containerTag),
			FieldPaths:     fieldPaths,
			ElementTag:     containerTag,
			OriginalAttrs:  containerAttrs,
		}

		constructs = append(constructs, construct)
	}

	return constructs
}

// extractFieldPathsFromIfExpression extracts field paths from if expressions
func extractFieldPathsFromIfExpression(ifExpr string) []string {
	// Handle different if expression formats:
	// ".ShowError" -> [".ShowError"]
	// "not .Active" -> [".Active"]
	// ".Count gt 0" -> [".Count"]
	// "and .A .B" -> [".A", ".B"]

	var fieldPaths []string

	// Simple regex to find field references (starting with .)
	fieldPattern := regexp.MustCompile(`\.[A-Za-z][A-Za-z0-9_]*`)
	matches := fieldPattern.FindAllString(ifExpr, -1)

	// Deduplicate field paths
	seen := make(map[string]bool)
	for _, match := range matches {
		if !seen[match] {
			fieldPaths = append(fieldPaths, match)
			seen[match] = true
		}
	}

	return fieldPaths
}

// extractFieldPathsFromRangeExpression extracts field paths from range expressions
func extractFieldPathsFromRangeExpression(rangeExpr string) []string {
	// Handle different range expression formats:
	// ".Todos" -> [".Todos"]
	// "$item := .Items" -> [".Items"]
	// "$i, $item := .Users" -> [".Users"]

	if strings.Contains(rangeExpr, ":=") {
		// Assignment format: extract right side
		parts := strings.Split(rangeExpr, ":=")
		if len(parts) >= 2 {
			fieldPath := strings.TrimSpace(parts[1])
			if strings.HasPrefix(fieldPath, ".") {
				return []string{fieldPath}
			}
		}
	} else if strings.HasPrefix(rangeExpr, ".") {
		// Direct field reference
		return []string{rangeExpr}
	}

	return []string{}
}

// findContainerElementStart finds the HTML element that contains a template construct
func findContainerElementStart(templateSource string, constructPos int) (int, string, string) {
	// Look backwards from constructPos to find the most recent opening tag
	before := templateSource[:constructPos]

	// Find all opening tags before the construct
	openTagPattern := regexp.MustCompile(`<(\w+)([^>]*)>`)
	matches := openTagPattern.FindAllStringSubmatch(before, -1)
	indexes := openTagPattern.FindAllStringSubmatchIndex(before, -1)

	if len(matches) == 0 || len(indexes) == 0 {
		return -1, "", ""
	}

	// Get the last (most recent) opening tag
	lastMatch := matches[len(matches)-1]
	lastIndex := indexes[len(indexes)-1]

	if len(lastMatch) >= 3 {
		tagName := lastMatch[1]
		attributes := lastMatch[2]
		startPos := lastIndex[0]

		return startPos, tagName, attributes
	}

	return -1, "", ""
}

// findImmediateContainerForConditional finds the HTML element that directly contains a conditional construct
// This looks for the element that wraps the conditional content, not just the last opening tag before it
func findImmediateContainerForConditional(templateSource string, ifStart, ifEnd int, ifContent string) (int, string, string) {
	// Strategy: Look for HTML elements that have the conditional construct within their content
	// Pattern: <tag ...>...{{if}}...{{end}}...</tag>

	// Find all HTML elements in the template source
	// We need to find matching open/close tags manually since Go regex doesn't support backreferences
	var matches [][]string
	var matchIndexes [][]int

	// Find all opening tags first
	openTagPattern := regexp.MustCompile(`<(\w+)([^>]*)>`)
	openMatches := openTagPattern.FindAllStringSubmatch(templateSource, -1)
	openIndexes := openTagPattern.FindAllStringSubmatchIndex(templateSource, -1)

	// For each opening tag, find its matching closing tag
	for i, openMatch := range openMatches {
		if len(openMatch) < 3 || len(openIndexes) <= i {
			continue
		}

		tagName := openMatch[1]
		attributes := openMatch[2]
		openStart := openIndexes[i][0]
		openEnd := openIndexes[i][1]

		// Find the matching closing tag
		closeIndex := findMatchingCloseTag(templateSource, openEnd, tagName)
		if closeIndex == -1 {
			continue // No matching close tag
		}

		// Extract content between open and close tags
		content := templateSource[openEnd:closeIndex]
		closeEnd := closeIndex + len("</"+tagName+">")

		// Create match similar to what the regex would produce
		fullMatch := templateSource[openStart:closeEnd]
		match := []string{fullMatch, tagName, attributes, content}
		matches = append(matches, match)

		// Create index match
		matchIndex := []int{openStart, closeEnd, 0, 0} // We only need the full match positions
		matchIndexes = append(matchIndexes, matchIndex)
	}

	// Look for elements that contain the conditional construct within their boundaries
	conditionalText := templateSource[ifStart:ifEnd]

	for i, match := range matches {
		if len(match) < 4 || len(matchIndexes) <= i {
			continue
		}

		elementStart := matchIndexes[i][0]
		elementEnd := matchIndexes[i][1]
		tagName := match[1]
		attributes := match[2]
		elementContent := match[3]

		// Check if this element contains the conditional construct
		if elementStart <= ifStart && ifEnd <= elementEnd && strings.Contains(elementContent, conditionalText) {
			// This element contains the conditional construct
			// Make sure it's not too broad (avoid large containers like body, html)
			if tagName != "body" && tagName != "html" && tagName != "main" {
				return elementStart, tagName, attributes
			}
		}
	}

	// Fallback to the old method if no suitable container found
	return findContainerElementStart(templateSource, ifStart)
}

// findContainerElementEnd finds the closing tag for a container element
func findContainerElementEnd(templateSource string, constructEndPos int, tagName string) int {
	// Look forwards from constructEndPos to find the matching closing tag
	after := templateSource[constructEndPos:]
	closeTag := "</" + tagName + ">"

	closeIndex := strings.Index(after, closeTag)
	if closeIndex != -1 {
		return constructEndPos + closeIndex + len(closeTag)
	}

	return -1
}
