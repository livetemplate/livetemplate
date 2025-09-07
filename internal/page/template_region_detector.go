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

	// Find HTML elements that contain template expressions
	regions := []TemplateRegion{}

	// Find HTML elements that contain template expressions by parsing properly
	matches := findCompleteElementMatches(templateSource)

	// Debug info removed for cleaner output

	// Filter to only get leaf elements (elements that don't contain other template-containing elements)
	leafMatches := filterToLeafElements(matches)

	regionIndex := 0
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
			if hasTemplateInAttributes {
				templateSource = fmt.Sprintf("<%s%s>%s</%s>", startTagName, attributes, content, endTagName)
			} else {
				templateSource = content
			}

			region := TemplateRegion{
				ID:             lvtID,
				TemplateSource: templateSource,
				StartMarker:    fmt.Sprintf("<%s%s>", startTagName, attributes),
				EndMarker:      fmt.Sprintf("</%s>", endTagName),
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
func findCompleteElementMatches(templateSource string) [][]string {
	var matches [][]string

	// Find all opening tags
	openTagRegex := regexp.MustCompile(`<(\w+)([^>]*)>`)
	openMatches := openTagRegex.FindAllStringSubmatch(templateSource, -1)
	openIndexes := openTagRegex.FindAllStringSubmatchIndex(templateSource, -1)

	for i, openMatch := range openMatches {
		if len(openMatch) < 3 {
			continue
		}

		tagName := openMatch[1]
		attributes := openMatch[2]
		openStart := openIndexes[i][0]
		openEnd := openIndexes[i][1]

		// Find the matching closing tag
		closeTag := "</" + tagName + ">"
		closeIndex := findMatchingCloseTag(templateSource, openEnd, tagName)

		if closeIndex == -1 {
			continue // No matching close tag found
		}

		// Extract the content between open and close tags
		content := templateSource[openEnd:closeIndex]

		// Create match array similar to regex: [fullMatch, openTag, attributes, content, closeTag]
		fullMatch := templateSource[openStart : closeIndex+len(closeTag)]
		match := []string{fullMatch, tagName, attributes, content, tagName}
		matches = append(matches, match)
	}

	return matches
}

// filterToLeafElements filters matches to only include leaf elements (no nested template-containing elements)
func filterToLeafElements(matches [][]string) [][]string {
	var leafMatches [][]string
	templatePattern := regexp.MustCompile(`\{\{[^}]+\}\}`)

	for i, match := range matches {
		if len(match) < 5 {
			continue
		}

		fullMatch := match[0]
		attributes := match[2]
		content := match[3]

		// Check if this element contains template expressions
		hasTemplateInAttributes := templatePattern.MatchString(attributes)
		hasTemplateInContent := templatePattern.MatchString(content)

		if !hasTemplateInAttributes && !hasTemplateInContent {
			continue // Skip elements without templates
		}

		// Check if any other match is contained within this one
		// If so, this is not a leaf element
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

			if otherHasTemplate && strings.Contains(fullMatch, otherFull) && fullMatch != otherFull {
				// This match contains another template-containing element
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
