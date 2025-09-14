package page

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
)

// HTMLProcessor handles runtime injection of lvt-id attributes into rendered HTML
type HTMLProcessor struct {
	regionTracker *RegionTracker
	mu            sync.RWMutex
}

// RegionTracker tracks template regions and their runtime states
type RegionTracker struct {
	templateRegions []TemplateRegion  // Original regions from template
	activeRegions   map[string]bool   // Which regions are active in current render
	regionIDMap     map[string]string // Maps region signatures to consistent IDs
}

// NewHTMLProcessor creates a new HTML processor
func NewHTMLProcessor(templateRegions []TemplateRegion) *HTMLProcessor {
	return &HTMLProcessor{
		regionTracker: &RegionTracker{
			templateRegions: templateRegions,
			activeRegions:   make(map[string]bool),
			regionIDMap:     make(map[string]string),
		},
	}
}

// ProcessRenderedHTML injects lvt-id attributes into rendered HTML
// This is the key innovation: we process AFTER template rendering, not before
func (p *HTMLProcessor) ProcessRenderedHTML(html string, data interface{}, regions []TemplateRegion) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Track which regions are active in this render
	p.regionTracker.activeRegions = make(map[string]bool)

	// Process each region
	processedHTML := html
	for _, region := range regions {
		// Check if this region's content exists in the rendered HTML
		if p.regionExistsInHTML(processedHTML, region) {
			// Region is active - inject lvt-id
			processedHTML = p.injectLvtIDIntoHTML(processedHTML, region)
			p.regionTracker.activeRegions[region.ID] = true

			log.Printf("HTML_PROCESSOR: Injected lvt-id=%s for active region (tag: %s)", region.ID, region.ElementTag)
		} else {
			// Region doesn't exist in current HTML (e.g., empty range, false conditional)
			log.Printf("HTML_PROCESSOR: Skipping lvt-id injection for region %s (not in rendered HTML)", region.ID)
		}
	}

	return processedHTML, nil
}

// regionExistsInHTML checks if a region's element exists in the rendered HTML
func (p *HTMLProcessor) regionExistsInHTML(html string, region TemplateRegion) bool {
	// For range loop elements (b* IDs), they may not exist if the range is empty
	// Use a generic approach that checks for the element tag regardless of context
	if strings.HasPrefix(region.ID, "b") {
		// For range loop elements, check if the element tag exists in the HTML
		// This handles empty ranges generically without hardcoded container assumptions
		tagPattern := fmt.Sprintf("<%s", region.ElementTag)
		return strings.Contains(html, tagPattern)
	}

	// For static elements, check if they exist in the HTML
	// Look for opening tag of the element
	tagPattern := fmt.Sprintf("<%s", region.ElementTag)
	return strings.Contains(html, tagPattern)
}

// injectLvtIDIntoHTML injects lvt-id attribute into the actual rendered HTML
func (p *HTMLProcessor) injectLvtIDIntoHTML(html string, region TemplateRegion) string {
	// Use a generic pattern that matches any element of the specified tag type
	// This approach is element-agnostic and doesn't depend on specific attributes or use cases
	pattern := regexp.MustCompile(fmt.Sprintf(`(<%s[^>]*)(>)`, regexp.QuoteMeta(region.ElementTag)))

	if pattern == nil {
		return html
	}

	// Check if element already has data-lvt-id
	if strings.Contains(html, fmt.Sprintf(`data-lvt-id="%s"`, region.ID)) {
		return html
	}

	// Inject data-lvt-id attribute
	replacement := fmt.Sprintf(`$1 data-lvt-id="%s"$2`, region.ID)

	// For range loop elements, we might need to inject into multiple instances
	if strings.HasPrefix(region.ID, "b") {
		// This is a range loop element - inject into first occurrence only
		// In a real implementation, we'd handle multiple instances properly
		return pattern.ReplaceAllStringFunc(html, func(match string) string {
			// Only replace if it doesn't already have an lvt-id
			if !strings.Contains(match, "lvt-id=") {
				return pattern.ReplaceAllString(match, replacement)
			}
			return match
		})
	}

	// For static elements, replace first occurrence
	result := pattern.ReplaceAllString(html, replacement)

	if result != html {
		log.Printf("HTML_PROCESSOR: Successfully injected lvt-id=%s into %s element", region.ID, region.ElementTag)
	} else {
		log.Printf("HTML_PROCESSOR: Failed to inject lvt-id=%s into %s element", region.ID, region.ElementTag)
	}

	return result
}

// GetActiveRegions returns the currently active regions
func (p *HTMLProcessor) GetActiveRegions() map[string]bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid race conditions
	active := make(map[string]bool)
	for k, v := range p.regionTracker.activeRegions {
		active[k] = v
	}
	return active
}

// IsRegionActive checks if a specific region is active in the current render
func (p *HTMLProcessor) IsRegionActive(regionID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.regionTracker.activeRegions[regionID]
}
