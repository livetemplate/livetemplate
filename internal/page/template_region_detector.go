package page

import (
	"fmt"
	"regexp"
	"strings"
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

	// Extract the full template source
	templateSource, err := p.extractTemplateSourceFromTemplate(p.template)
	if err != nil {
		return nil, fmt.Errorf("failed to extract template source: %w", err)
	}

	// Find HTML elements that contain template expressions
	regions := []TemplateRegion{}

	// Updated regex to capture any HTML element
	// Captures: <tag attributes>content</tag>
	elementRegex := regexp.MustCompile(`<(\w+)([^>]*)>([^<]*)</(\w+)>`)
	matches := elementRegex.FindAllStringSubmatch(templateSource, -1)

	for i, match := range matches {
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

			// Extract ID from tag attributes if available, otherwise generate globally unique ID
			extractedID := extractIDFromTag(attributes)
			var lvtID string
			if extractedID != "" {
				lvtID = extractedID
			} else {
				// Generate region ID using region index
				lvtID = fmt.Sprintf("region_%d", i)
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

// generateRegionFragment creates a fragment update for a specific template region (with metadata)
func (p *Page) generateRegionFragment(region TemplateRegion, newData interface{}) (*Fragment, error) {
	config := &FragmentConfig{IncludeMetadata: true} // Legacy behavior includes metadata
	return p.generateRegionFragmentWithConfig(region, newData, config)
}

// generateRegionFragmentWithConfig creates a fragment update for a specific template region with config
func (p *Page) generateRegionFragmentWithConfig(region TemplateRegion, newData interface{}, config *FragmentConfig) (*Fragment, error) {
	// Use the tree generator on just this region
	oldData := p.data
	fragmentID := fmt.Sprintf("region_%s", region.ID)

	treeResult, err := p.treeGenerator.GenerateFromTemplateSource(region.TemplateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate region fragment: %w", err)
	}

	fragment := &Fragment{
		ID:       fragmentID,
		Strategy: "tree_based_region",
		Action:   "update_region",
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
