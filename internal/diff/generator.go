package diff

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// Generator implements the tree generator interface using the unified diff approach
// This eliminates HTML intrinsics knowledge and uses template-based parsing for maximum efficiency
type Generator struct {
	differ *Tree
}

// NewGenerator creates a new unified tree generator
func NewGenerator() *Generator {
	return &Generator{
		differ: NewTree(),
	}
}

// GenerateFromTemplateSource creates unified tree updates from template source
// This matches the SimpleTreeGenerator interface for seamless integration
func (g *Generator) GenerateFromTemplateSource(templateSource string, oldData, newData interface{}, fragmentID string) (*Update, error) {
	// Extract actual template source from template object if needed
	actualTemplateSource := templateSource

	// If templateSource doesn't contain template expressions, treat it as pure static HTML
	if !strings.Contains(templateSource, "{{") {
		// Pure static template - return as single static segment
		return &Update{
			S:        []string{templateSource},
			Dynamics: make(map[string]any),
			H:        "", // No hash needed for static-only content
		}, nil
	}

	// Generate the unified tree update with fragment ID for proper static caching
	update, err := g.differ.GenerateWithFragmentID(actualTemplateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unified tree update: %w", err)
	}

	return update, nil
}

// GenerateFromTemplate creates unified tree updates from a template object
// This is an additional method to handle template objects directly
func (g *Generator) GenerateFromTemplate(tmpl *template.Template, oldData, newData interface{}, fragmentID string) (*Update, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}

	// Extract template source - this is tricky with Go's template package
	// We'll need to render the template to get comparable output
	templateSource := g.extractTemplateSource(tmpl)

	if templateSource == "" {
		return nil, fmt.Errorf("unable to extract template source from template")
	}

	return g.GenerateFromTemplateSource(templateSource, oldData, newData, fragmentID)
}

// extractTemplateSource attempts to extract template source from template object
// This is a workaround since Go's template package doesn't expose source directly
func (g *Generator) extractTemplateSource(tmpl *template.Template) string {
	_ = tmpl // Use parameter to avoid unused warning
	// In a real implementation, we would need to store the original template source
	// or use reflection to extract it from the template object
	// For now, return empty string to indicate we need the source provided separately
	return ""
}

// ClearCache clears the internal cache
func (g *Generator) ClearCache() {
	g.differ = NewTree() // Reset the differ to clear cache
}

// HasCachedStructure checks if structure is cached for a specific fragment
func (g *Generator) HasCachedStructure(fragmentID string) bool {
	// Check if fragment has cached statics
	fragmentCache, exists := g.differ.fragmentCache[fragmentID]
	return exists && fragmentCache.hasStatics
}

// GeneratorMetrics provides metrics for the unified approach
type GeneratorMetrics struct {
	TotalGenerations      int64         `json:"total_generations"`
	SuccessfulGenerations int64         `json:"successful_generations"`
	FailedGenerations     int64         `json:"failed_generations"`
	AverageGenerationTime time.Duration `json:"average_generation_time"`
	TotalBandwidthSaved   int64         `json:"total_bandwidth_saved"`
	CacheHitRate          float64       `json:"cache_hit_rate"`
}

// GetMetrics returns current metrics (placeholder for future implementation)
func (g *Generator) GetMetrics() *GeneratorMetrics {
	return &GeneratorMetrics{
		TotalGenerations:      0,
		SuccessfulGenerations: 0,
		FailedGenerations:     0,
		AverageGenerationTime: 0,
		TotalBandwidthSaved:   0,
		CacheHitRate:          0,
	}
}
