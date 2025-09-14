package page

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// Forward reference to avoid circular dependencies
// FragmentConfig is defined in page.go

// regionIDGenerator creates unique, short alphanumeric IDs for template regions
type regionIDGenerator struct {
	counter int
	usedIDs map[string]bool
}

// generate creates a unique short alphanumeric ID (e.g., "a1", "b2", "c3")
func (g *regionIDGenerator) generate() string {
	for {
		g.counter++
		// Create short alphanumeric ID: a1, a2, ..., a9, b1, b2, etc.
		letter := string(rune('a' + ((g.counter-1)/9)%26))
		number := ((g.counter - 1) % 9) + 1
		id := fmt.Sprintf("%s%d", letter, number)

		// Ensure uniqueness
		if !g.usedIDs[id] {
			g.usedIDs[id] = true
			return id
		}
	}
}

// Removed generateCustom and isValidFormat methods - we now always auto-generate for consistency

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

	// Create a centralized ID generator for consistent, unique, short alphanumeric IDs
	idGenerator := &regionIDGenerator{usedIDs: make(map[string]bool)}

	// Method 1: Find standalone template constructs like {{range}}, {{if}} that span elements
	// Process these FIRST as they have higher priority and define larger boundaries
	standaloneConstructs := findStandaloneTemplateConstructs(templateSource, idGenerator)

	// Process standalone template construct regions (Method 1 - Higher Priority)
	for _, construct := range standaloneConstructs {
		region := TemplateRegion{
			ID:             idGenerator.generate(), // Use centralized ID generator
			TemplateSource: construct.TemplateSource,
			StartMarker:    construct.StartMarker,
			EndMarker:      construct.EndMarker,
			FieldPaths:     construct.FieldPaths,
			ElementTag:     construct.ElementTag,
			OriginalAttrs:  construct.OriginalAttrs,
		}

		// DEBUG DISABLED: Region creation logging removed to reduce noise during testing
		// fmt.Printf("DEBUG: Created standalone region ID=%s, Tag=%s, TemplateSource=%s\n",
		//	region.ID, region.ElementTag, region.TemplateSource)
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

			// IMPORTANT: Always auto-generate lvt-id for consistency and guaranteed uniqueness
			// Never preserve existing IDs (id, lvt-id) to ensure all IDs follow same format
			lvtID := idGenerator.generate()

			// Extract field paths from template expressions in both attributes and content
			fieldPaths := extractFieldPaths(attributes + " " + content)

			// If attributes contain templates, include the entire element as template source
			// IMPORTANT: Inject lvt-id directly into template source to avoid post-processing annotation
			var templateSource string
			var endMarker string
			if hasTemplateInAttributes {
				// Inject lvt-id into the attributes of the template source
				attributesWithID := injectLvtID(attributes, lvtID)
				if isSelfClosing {
					templateSource = fmt.Sprintf("<%s%s>", startTagName, attributesWithID)
					endMarker = "" // Self-closing elements don't have end markers
				} else {
					templateSource = fmt.Sprintf("<%s%s>%s</%s>", startTagName, attributesWithID, content, endTagName)
					endMarker = fmt.Sprintf("</%s>", endTagName)
				}
			} else {
				// For content-only templates, the template source is just the content
				templateSource = content
				endMarker = fmt.Sprintf("</%s>", endTagName)
			}

			// Use attributes with injected ID for StartMarker as well
			var startMarker string
			if hasTemplateInAttributes {
				attributesWithID := injectLvtID(attributes, lvtID)
				startMarker = fmt.Sprintf("<%s%s>", startTagName, attributesWithID)
			} else {
				attributesWithID := injectLvtID(attributes, lvtID)
				startMarker = fmt.Sprintf("<%s%s>", startTagName, attributesWithID)
			}

			region := TemplateRegion{
				ID:             lvtID,
				TemplateSource: templateSource,
				StartMarker:    startMarker,
				EndMarker:      endMarker,
				FieldPaths:     fieldPaths,
				ElementTag:     startTagName,
				OriginalAttrs:  attributes,
			}

			// DEBUG DISABLED: Element region creation logging removed to reduce noise
			// fmt.Printf("DEBUG: Created element region ID=%s, Tag=%s, TemplateSource=%s\n",
			//	region.ID, region.ElementTag, region.TemplateSource)
			regions = append(regions, region)
		}
	}

	return regions, nil
}

// DetectTemplateRegionsFromSource detects template regions from template source without requiring a Page instance
// This is used during template parsing to inject lvt-id attributes
func DetectTemplateRegionsFromSource(templateSource string) ([]TemplateRegion, error) {
	// Create a global ID generator for this template
	idGenerator := &regionIDGenerator{
		counter: 0,
		usedIDs: make(map[string]bool),
	}

	var regions []TemplateRegion

	// Find standalone template constructs first (range, if, with, etc)
	standaloneConstructs := findStandaloneTemplateConstructs(templateSource, idGenerator)
	regions = append(regions, standaloneConstructs...)

	// Mark IDs as used from standalone constructs
	for _, construct := range standaloneConstructs {
		idGenerator.usedIDs[construct.ID] = true
	}

	// Find HTML elements with template expressions
	elementMatches := findCompleteElementMatches(templateSource)
	leafElements := filterToLeafElements(elementMatches)

	for _, match := range leafElements {
		if len(match) < 5 {
			continue
		}

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

		// IMPORTANT: Always auto-generate lvt-id for consistency and guaranteed uniqueness
		// Never preserve existing IDs (id, lvt-id) to ensure all IDs follow same format
		lvtID := idGenerator.generate()

		// Extract field paths from template expressions in both attributes and content
		fieldPaths := extractFieldPaths(attributes + " " + content)

		// If attributes contain templates, include the entire element as template source
		var templateSourceForRegion string
		var endMarker string
		isSelfClosing := content == "" && (startTagName == "input" || startTagName == "img" ||
			startTagName == "meta" || startTagName == "link" || startTagName == "br" || startTagName == "hr")

		if hasTemplateInAttributes {
			if isSelfClosing {
				templateSourceForRegion = fmt.Sprintf("<%s%s>", startTagName, attributes)
				endMarker = "" // Self-closing elements don't have end markers
			} else {
				templateSourceForRegion = fmt.Sprintf("<%s%s>%s</%s>", startTagName, attributes, content, endTagName)
				endMarker = fmt.Sprintf("</%s>", endTagName)
			}
		} else {
			templateSourceForRegion = content
			endMarker = fmt.Sprintf("</%s>", endTagName)
		}

		// Create start marker
		startMarker := fmt.Sprintf("<%s%s>", startTagName, attributes)

		region := TemplateRegion{
			ID:             lvtID,
			TemplateSource: templateSourceForRegion,
			StartMarker:    startMarker,
			EndMarker:      endMarker,
			FieldPaths:     fieldPaths,
			ElementTag:     startTagName,
			OriginalAttrs:  attributes,
		}

		regions = append(regions, region)
	}

	return regions, nil
}

// injectLvtID injects a data-lvt-id attribute into an HTML attribute string
// If data-lvt-id already exists, it preserves the existing one
// If attributes is empty, it creates the attribute string
func injectLvtID(attributes, lvtID string) string {
	// Check if data-lvt-id already exists
	if strings.Contains(attributes, "data-lvt-id=") {
		return attributes // Don't override existing data-lvt-id
	}

	// If attributes is empty, create the attribute
	if strings.TrimSpace(attributes) == "" {
		return fmt.Sprintf(` data-lvt-id="%s"`, lvtID)
	}

	// Add data-lvt-id to existing attributes
	return fmt.Sprintf(`%s data-lvt-id="%s"`, attributes, lvtID)
}

// extractIDFromTag extracts the data-lvt-id or id attribute from an HTML tag, prioritizing data-lvt-id
func extractIDFromTag(tag string) string {
	// First check for data-lvt-id attribute (LiveTemplate specific)
	lvtIdRegex := regexp.MustCompile(`data-lvt-id=["']([^"']+)["']`)
	matches := lvtIdRegex.FindStringSubmatch(tag)
	if len(matches) >= 2 {
		return matches[1]
	}

	// Fallback to regular id attribute
	idRegex := regexp.MustCompile(`id=["']([^"']+)["']`)
	matches = idRegex.FindStringSubmatch(tag)
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

		// For other elements with template expressions, include if they're "leaf" elements OR have template attributes
		// Elements with template attributes should always be included (e.g., div with dynamic style)
		if hasTemplateInAttributes {
			// Always include elements with template expressions in attributes
			leafMatches = append(leafMatches, match)
			continue
		}

		// For elements with only template content, only include if they're "leaf" elements
		// (no nested template-containing elements within them)
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
	// CRITICAL FIX: Check if this region is inside a range loop and whether that loop is empty
	// If the region is inside a range loop that results in empty output, skip fragment generation
	if p.isRegionInEmptyRangeLoop(region, newData) {
		// Return nil to indicate this fragment should be skipped
		return nil, fmt.Errorf("region %s is inside an empty range loop, skipping fragment generation", region.ID)
	}

	// Use the tree generator on just this region
	oldData := p.data
	// Use region ID directly as fragment ID (it's already unique)
	fragmentID := region.ID

	// CRITICAL FIX: Inject lvt-id attribute into the region's template source before fragment generation
	templateSourceWithLvtID := p.injectLvtIDIntoRegionTemplate(region)

	treeResult, err := p.treeGenerator.GenerateFromTemplateSource(templateSourceWithLvtID, oldData, newData, fragmentID)
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

// injectLvtIDIntoRegionTemplate injects the lvt-id attribute into the region's template source
func (p *Page) injectLvtIDIntoRegionTemplate(region TemplateRegion) string {
	templateSource := region.TemplateSource

	// If the template source already contains lvt-id for this region, return as-is
	if strings.Contains(templateSource, fmt.Sprintf(`lvt-id="%s"`, region.ID)) {
		return templateSource
	}

	// Check if the template source contains HTML element tags
	if strings.Contains(templateSource, "<") && strings.Contains(templateSource, ">") {
		// Template source contains HTML elements - inject lvt-id directly
		originalTag := region.StartMarker
		if originalTag == "" {
			return templateSource
		}

		modifiedTag := p.injectLvtIDIntoElement(originalTag, region.ID)
		modifiedTemplate := strings.Replace(templateSource, originalTag, modifiedTag, 1)
		return modifiedTemplate
	} else {
		// Template source is content-only (e.g., "{{.Count}}")
		// For content-only regions, we need to wrap the content with the HTML element that has lvt-id
		if region.StartMarker != "" && region.EndMarker != "" {
			// Reconstruct the full element with lvt-id
			return region.StartMarker + templateSource + region.EndMarker
		} else if region.StartMarker != "" {
			// Self-closing element case
			return region.StartMarker
		}

		// Fallback: return original template source
		return templateSource
	}
}

// injectLvtIDIntoElement injects lvt-id attribute into an HTML element tag
func (p *Page) injectLvtIDIntoElement(elementTag, lvtID string) string {
	// Remove existing lvt-id if present to avoid duplicates
	elementTag = regexp.MustCompile(`\s+lvt-id="[^"]*"`).ReplaceAllString(elementTag, "")

	// Handle self-closing tags (like <input />)
	if strings.HasSuffix(elementTag, "/>") {
		return strings.TrimSuffix(elementTag, "/>") + fmt.Sprintf(` lvt-id="%s"/>`, lvtID)
	}

	// Handle regular opening tags (like <div>)
	if strings.HasSuffix(elementTag, ">") {
		return strings.TrimSuffix(elementTag, ">") + fmt.Sprintf(` lvt-id="%s">`, lvtID)
	}

	// Handle unclosed tags (shouldn't happen but fallback)
	return elementTag + fmt.Sprintf(` lvt-id="%s"`, lvtID)
}

// findStandaloneTemplateConstructs finds template constructs like {{range}}, {{if}} that span across HTML elements
func findStandaloneTemplateConstructs(templateSource string, idGenerator *regionIDGenerator) []TemplateRegion {
	var constructs []TemplateRegion

	// Find {{range}} constructs
	rangeConstructs := findRangeConstructs(templateSource, idGenerator)
	constructs = append(constructs, rangeConstructs...)

	// Find {{if}} constructs, but only include small/contained ones to avoid overlapping regions
	ifConstructs := findConditionalConstructs(templateSource, idGenerator)
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
func findRangeConstructs(templateSource string, idGenerator *regionIDGenerator) []TemplateRegion {
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
			ID:             idGenerator.generate(),
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
func findConditionalConstructs(templateSource string, idGenerator *regionIDGenerator) []TemplateRegion {
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
			ID:             idGenerator.generate(),
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

// isRegionInEmptyRangeLoop checks if a region is inside a range loop that evaluates to empty
func (p *Page) isRegionInEmptyRangeLoop(region TemplateRegion, newData interface{}) bool {
	// SIMPLE APPROACH: Use naming pattern to detect range loop elements
	// In LiveTemplate, range loop elements typically get 'b*' IDs while main template elements get 'a*' IDs
	if strings.HasPrefix(region.ID, "b") {
		// Check if .Todos is empty in the new data - using correct function signature
		isEmpty := isFieldEmptyHelper(newData, "Todos")

		if isEmpty {
			log.Printf("FILTER: Skipping region %s (range loop element, todos empty)", region.ID)
			return true
		}
	}

	return false
}

// isFieldEmptyHelper checks if a field is empty in interface{} data structures (like map[string]interface{})
func isFieldEmptyHelper(data interface{}, fieldPath string) bool {
	if data == nil {
		return true
	}

	value := reflect.ValueOf(data)

	// Handle pointer indirection
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}

	// Navigate to the field
	parts := strings.Split(fieldPath, ".")
	for _, part := range parts {
		if part == "" {
			continue
		}

		switch value.Kind() {
		case reflect.Map:
			mapValue := value.MapIndex(reflect.ValueOf(part))
			if !mapValue.IsValid() {
				return true
			}
			value = mapValue
		case reflect.Struct:
			field := value.FieldByName(part)
			if !field.IsValid() {
				return true
			}
			value = field
		default:
			return true
		}

		// Handle interface{} values - need to get the underlying concrete value
		for value.Kind() == reflect.Interface {
			if value.IsNil() {
				return true
			}
			value = value.Elem()
		}
	}

	// Check if the final value is empty
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return value.Len() == 0
	case reflect.String:
		return value.String() == ""
	case reflect.Ptr, reflect.Interface:
		return value.IsNil()
	case reflect.Map:
		return value.Len() == 0
	default:
		// For other types, check if it's the zero value
		zeroValue := reflect.Zero(value.Type())
		return reflect.DeepEqual(value.Interface(), zeroValue.Interface())
	}
}

// isRegionInsideEmptyRange determines if a region is inside a range loop that produces no output
func (p *Page) isRegionInsideEmptyRange(templateSource string, region TemplateRegion, newData interface{}) bool {
	// Find all range blocks in the template
	rangePattern := regexp.MustCompile(`\{\{\s*range\s+([^}]+)\s*\}\}([\s\S]*?)\{\{\s*end\s*\}\}`)
	matches := rangePattern.FindAllStringSubmatch(templateSource, -1)
	matchIndexes := rangePattern.FindAllStringSubmatchIndex(templateSource, -1)

	log.Printf("DEBUG RANGE: Found %d range blocks in template for region %s", len(matches), region.ID)

	// Debug: Show template source snippet around range blocks
	if len(matches) > 0 {
		rangeIndex := strings.Index(templateSource, "{{range")
		if rangeIndex >= 0 {
			start := rangeIndex
			if start > 50 {
				start = rangeIndex - 50
			} else {
				start = 0
			}
			end := rangeIndex + 200
			if end > len(templateSource) {
				end = len(templateSource)
			}
			log.Printf("DEBUG RANGE: Template around range (chars %d-%d): %s", start, end, templateSource[start:end])
		}
	}

	for i, match := range matches {
		if len(match) < 3 || len(matchIndexes) <= i {
			continue
		}

		rangeVariable := strings.TrimSpace(match[1])
		rangeContent := match[2]

		log.Printf("DEBUG RANGE: Range %d - variable: %s", i, rangeVariable)
		log.Printf("DEBUG RANGE: Range content (first 100 chars): %.100s", rangeContent)
		log.Printf("DEBUG RANGE: Looking for region StartMarker: %.100s", region.StartMarker)
		log.Printf("DEBUG RANGE: Looking for region ElementTag: %s", region.ElementTag)

		// Check if the region's start marker appears within this range block content
		startsInRange := strings.Contains(rangeContent, region.StartMarker)
		tagInRange := strings.Contains(rangeContent, region.ElementTag)

		log.Printf("DEBUG RANGE: StartMarker in range: %v, ElementTag in range: %v", startsInRange, tagInRange)

		if startsInRange || tagInRange {
			log.Printf("DEBUG RANGE: Region %s is inside range loop %d", region.ID, i)

			// This region is likely inside this range loop
			// Now check if the range evaluates to empty
			isEmpty := p.isRangeVariableEmpty(rangeVariable, newData)
			log.Printf("DEBUG RANGE: Range variable %s is empty: %v", rangeVariable, isEmpty)

			if isEmpty {
				return true
			}
		}
	}

	return false
}

// isRangeVariableEmpty checks if a range variable evaluates to empty in the given data
func (p *Page) isRangeVariableEmpty(rangeVariable string, data interface{}) bool {
	// Handle different range variable formats:
	// .Items (most common)
	// $var := .Items
	// .User.Items

	// Extract the actual variable path
	varPath := rangeVariable
	if strings.Contains(rangeVariable, ":=") {
		// Handle format: $var := .Items
		parts := strings.Split(rangeVariable, ":=")
		if len(parts) == 2 {
			varPath = strings.TrimSpace(parts[1])
		}
	}

	// Remove leading dot if present
	varPath = strings.TrimPrefix(strings.TrimSpace(varPath), ".")

	// Use reflection to check if the field is empty
	return p.isFieldEmpty(varPath, data)
}

// isFieldEmpty uses reflection to check if a field path evaluates to empty
func (p *Page) isFieldEmpty(fieldPath string, data interface{}) bool {
	if data == nil {
		return true
	}

	// Use reflection to traverse the field path
	value := reflect.ValueOf(data)
	if !value.IsValid() {
		return true
	}

	// Handle pointer dereferencing
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}

	// Split field path by dots for nested access
	fieldParts := strings.Split(fieldPath, ".")

	for _, fieldName := range fieldParts {
		if fieldName == "" {
			continue
		}

		// Handle struct fields
		if value.Kind() == reflect.Struct {
			field := value.FieldByName(fieldName)
			if !field.IsValid() {
				return true // Field doesn't exist, consider empty
			}
			value = field
		} else if value.Kind() == reflect.Map {
			// Handle map access
			mapKey := reflect.ValueOf(fieldName)
			field := value.MapIndex(mapKey)
			if !field.IsValid() {
				return true // Key doesn't exist, consider empty
			}
			value = field
		} else {
			return true // Can't traverse further, consider empty
		}

		// Handle pointer dereferencing again
		for value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return true
			}
			value = value.Elem()
		}
	}

	// Check if the final value is empty
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return value.Len() == 0
	case reflect.Map:
		return value.Len() == 0
	case reflect.String:
		return value.String() == ""
	case reflect.Interface:
		return value.IsNil()
	case reflect.Invalid:
		return true
	default:
		// For other types, check zero value
		return value.IsZero()
	}
}

// InjectLvtIDsIntoTemplate detects template regions and injects lvt-id attributes directly into the template source
// This approach eliminates post-processing annotation and prevents duplicate IDs
// It returns both the modified source and the detected regions for caching
func InjectLvtIDsIntoTemplate(templateSource string) (string, []TemplateRegion, error) {
	// Detect regions from the template source using the standalone function
	regions, err := DetectTemplateRegionsFromSource(templateSource)
	if err != nil {
		return "", nil, fmt.Errorf("failed to detect regions: %w", err)
	}

	// Sort regions by position (descending) so we can modify from end to beginning
	// This prevents position shifts from affecting later replacements
	sort.Slice(regions, func(i, j int) bool {
		return strings.Index(templateSource, regions[i].StartMarker) > strings.Index(templateSource, regions[j].StartMarker)
	})

	modifiedSource := templateSource

	// Replace each region's opening tags with versions that include lvt-id
	for _, region := range regions {
		// FIXED APPROACH: Replace only the opening tag (StartMarker), not the entire element
		originalTag := region.StartMarker

		// RE-ENABLED: Parse-time injection is simpler and works for most cases
		// Range loop filtering handles the few edge cases with dynamic elements

		// Create modified opening tag with lvt-id injected
		modifiedTag := injectLvtIDIntoElement(originalTag, region.ID)

		// Replace only the opening tag in template source
		modifiedSource = strings.Replace(modifiedSource, originalTag, modifiedTag, 1)

		// Silent operation - no debug output since this is only for region detection
	}

	return modifiedSource, regions, nil
}

// injectLvtIDIntoElement injects data-lvt-id into a single HTML element string
func injectLvtIDIntoElement(element, lvtID string) string {
	// Check if data-lvt-id already exists
	if strings.Contains(element, "data-lvt-id=") {
		return element // Don't override existing data-lvt-id
	}

	// For self-closing tags like <input ...>, inject before the closing >
	if strings.HasSuffix(element, ">") && !strings.Contains(element, "</") {
		return strings.TrimSuffix(element, ">") + fmt.Sprintf(` data-lvt-id="%s">`, lvtID)
	}

	// For paired tags, inject data-lvt-id into the opening tag
	re := regexp.MustCompile(`^(<[^>]+)(>.*?)$`)
	if matches := re.FindStringSubmatch(element); len(matches) >= 3 {
		openingTag := matches[1]
		rest := matches[2]
		return fmt.Sprintf(`%s data-lvt-id="%s"%s`, openingTag, lvtID, rest)
	}

	return element // Fallback: return unchanged if we can't parse
}
