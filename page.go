package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/livefir/livetemplate/internal/diff"
)

// Fragment represents a generated update fragment with strategy-specific data
type Fragment struct {
	ID       string            `json:"id"`
	Data     interface{}       `json:"data"` // Strategy-specific payload
	Metadata *FragmentMetadata `json:"metadata,omitempty"`
}

// FragmentMetadata contains performance and optimization information
type FragmentMetadata struct {
	GenerationTime   time.Duration `json:"generation_time"`
	OriginalSize     int           `json:"original_size"`
	CompressedSize   int           `json:"compressed_size"`
	CompressionRatio float64       `json:"compression_ratio"`
	Strategy         int           `json:"strategy_number"`
	Confidence       float64       `json:"confidence"`
	FallbackUsed     bool          `json:"fallback_used"`
}

// Page represents a template page instance for rendering and fragment generation
type Page struct {
	// Template for rendering
	template       *template.Template
	templateSource string // Store template source for unified diff

	// Current data state
	data             interface{}
	currentDataMutex sync.RWMutex

	// Update generation pipeline - using unified tree-based generator
	unifiedGenerator *diff.Generator

	// Fragment ID generation
	fragmentIDCounter int // Simple counter for generating fragment IDs

	// Configuration
	enableMetrics bool
	created       time.Time
}

// PageOption configures a Page instance
type PageOption func(*Page) error

// NewPage creates a new Page instance with the given template and initial data
func NewPage(tmpl *template.Template, data interface{}, options ...PageOption) (*Page, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}

	page := &Page{
		template:         tmpl,
		templateSource:   "", // Will need to be provided separately for unified diff
		data:             data,
		unifiedGenerator: diff.NewGenerator(),
		enableMetrics:    true,
		created:          time.Now(),
	}

	// Apply options
	for _, option := range options {
		if err := option(page); err != nil {
			return nil, fmt.Errorf("failed to apply page option: %w", err)
		}
	}

	return page, nil
}

// WithTemplateSource sets the template source for unified diff generation
func WithTemplateSource(source string) PageOption {
	return func(p *Page) error {
		p.templateSource = source
		return nil
	}
}

// WithMetricsEnabled configures whether metrics collection is enabled
func WithMetricsEnabled(enabled bool) PageOption {
	return func(p *Page) error {
		p.enableMetrics = enabled
		return nil
	}
}

// WithFallbackEnabled configures whether fallback strategies are enabled
func WithFallbackEnabled(enabled bool) PageOption {
	return func(p *Page) error {
		// Tree generator doesn't need fallback - it handles all cases
		return nil
	}
}

// WithMaxGenerationTime sets the maximum time allowed for update generation
func WithMaxGenerationTime(duration time.Duration) PageOption {
	return func(p *Page) error {
		// Tree generator doesn't have configurable timeouts yet
		return nil
	}
}

// Render generates the complete HTML output for the current page state
func (p *Page) Render() (string, error) {
	p.currentDataMutex.RLock()
	defer p.currentDataMutex.RUnlock()

	var buf strings.Builder
	err := p.template.Execute(&buf, p.data)
	if err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	// Automatically inject lvt-id attributes with numeric IDs
	htmlWithIds := p.injectLvtIds(buf.String())

	return htmlWithIds, nil
}

// FragmentOption configures fragment generation behavior
type FragmentOption func(*FragmentConfig)

// FragmentConfig controls fragment generation options
type FragmentConfig struct {
	IncludeMetadata bool // Whether to include performance metadata (default: false)
}

// WithMetadata enables metadata collection in fragments
func WithMetadata() FragmentOption {
	return func(config *FragmentConfig) {
		config.IncludeMetadata = true
	}
}

// RenderFragments generates fragment updates for the given new data
func (p *Page) RenderFragments(ctx context.Context, newData interface{}, opts ...FragmentOption) ([]*Fragment, error) {
	// Apply options
	config := &FragmentConfig{
		IncludeMetadata: false, // Default: no metadata for minimal payload
	}
	for _, opt := range opts {
		opt(config)
	}

	return p.renderFragmentsWithConfig(ctx, newData, config)
}

// renderFragmentsWithConfig generates fragments with the specified configuration
func (p *Page) renderFragmentsWithConfig(ctx context.Context, newData interface{}, config *FragmentConfig) ([]*Fragment, error) {
	p.currentDataMutex.Lock()
	defer p.currentDataMutex.Unlock()

	// Try to use unified diff if template source is available, otherwise fall back to basic approach
	if p.templateSource == "" {
		// Fallback: create a simple fragment with the rendered HTML
		// This provides backward compatibility
		return p.renderFragmentsBasic(ctx, newData, config)
	}

	// Generate fragments using the unified tree generator
	oldData := p.data

	// Use a stable numeric fragment ID for consistent caching
	// For a single template page, always use fragment ID "1"
	fragmentID := "1"

	var startTime time.Time
	if config.IncludeMetadata {
		startTime = time.Now()
	}

	unifiedUpdate, err := p.unifiedGenerator.GenerateFromTemplateSource(p.templateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("unified tree generation failed: %w", err)
	}

	// Create fragment from unified update result
	fragment := &Fragment{
		ID:       fragmentID,
		Data:     unifiedUpdate,
		Metadata: nil, // Will be set conditionally below
	}

	// Add metadata only if requested
	if config.IncludeMetadata {
		generationTime := time.Since(startTime)
		fragment.Metadata = &FragmentMetadata{
			GenerationTime:   generationTime,
			OriginalSize:     0,
			CompressedSize:   0,
			CompressionRatio: 0,
			Strategy:         2, // Unified tree diff strategy
			Confidence:       1.0,
			FallbackUsed:     false,
		}
	}

	// Update current data state
	p.data = newData

	return []*Fragment{fragment}, nil
}

// UpdateData updates the page data and returns the current state
func (p *Page) UpdateData(newData interface{}) interface{} {
	p.currentDataMutex.Lock()
	defer p.currentDataMutex.Unlock()

	p.data = newData
	return p.data
}

// GetData returns the current page data
func (p *Page) GetData() interface{} {
	p.currentDataMutex.RLock()
	defer p.currentDataMutex.RUnlock()

	return p.data
}

// GetTemplate returns the page template
func (p *Page) GetTemplate() *template.Template {
	return p.template
}

// SetTemplate updates the page template
func (p *Page) SetTemplate(tmpl *template.Template) error {
	if tmpl == nil {
		return fmt.Errorf("template cannot be nil")
	}

	p.template = tmpl
	return nil
}

// SetTemplateSource sets the template source for unified diff generation
func (p *Page) SetTemplateSource(source string) error {
	p.templateSource = source
	return nil
}

// GetMetrics returns current fragment generation metrics
func (p *Page) GetMetrics() *UpdateGeneratorMetrics {
	// Unified generator metrics
	unifiedMetrics := p.unifiedGenerator.GetMetrics()

	return &UpdateGeneratorMetrics{
		TotalGenerations:      unifiedMetrics.TotalGenerations,
		SuccessfulGenerations: unifiedMetrics.SuccessfulGenerations,
		FailedGenerations:     unifiedMetrics.FailedGenerations,
		StrategyUsage:         map[string]int64{"tree_diff": unifiedMetrics.TotalGenerations},
		AverageGenerationTime: unifiedMetrics.AverageGenerationTime,
		TotalBandwidthSaved:   unifiedMetrics.TotalBandwidthSaved,
		FallbackRate:          0, // Unified approach doesn't use fallbacks
		ErrorRate:             float64(unifiedMetrics.FailedGenerations) / float64(unifiedMetrics.TotalGenerations),
		LastReset:             p.created,
	}
}

// ResetMetrics resets all fragment generation metrics
func (p *Page) ResetMetrics() {
	// Create a new unified generator to reset metrics
	p.unifiedGenerator = diff.NewGenerator()
}

// UpdateGeneratorMetrics tracks performance of the update generation pipeline
type UpdateGeneratorMetrics struct {
	TotalGenerations      int64            `json:"total_generations"`
	SuccessfulGenerations int64            `json:"successful_generations"`
	FailedGenerations     int64            `json:"failed_generations"`
	StrategyUsage         map[string]int64 `json:"strategy_usage"`
	AverageGenerationTime time.Duration    `json:"average_generation_time"`
	TotalBandwidthSaved   int64            `json:"total_bandwidth_saved"`
	FallbackRate          float64          `json:"fallback_rate"`
	ErrorRate             float64          `json:"error_rate"`
	LastReset             time.Time        `json:"last_reset"`
}

// GetCreatedTime returns when the page was created
func (p *Page) GetCreatedTime() time.Time {
	return p.created
}

// renderFragmentsBasic provides backward compatibility when no template source is available
func (p *Page) renderFragmentsBasic(ctx context.Context, newData interface{}, config *FragmentConfig) ([]*Fragment, error) {
	// Basic approach: render full HTML and return as single fragment
	// This maintains compatibility but doesn't provide optimal bandwidth savings

	// Use stable fragment ID for consistency with unified approach
	fragmentID := "1"

	var startTime time.Time
	if config.IncludeMetadata {
		startTime = time.Now()
	}

	// Render new template with new data
	var buf strings.Builder
	err := p.template.Execute(&buf, newData)
	if err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Create a basic fragment with the full HTML
	fragment := &Fragment{
		ID: fragmentID,
		Data: map[string]interface{}{
			"html": buf.String(),
			"type": "full_replace",
		},
		Metadata: nil,
	}

	// Add metadata if requested
	if config.IncludeMetadata {
		generationTime := time.Since(startTime)
		fragment.Metadata = &FragmentMetadata{
			GenerationTime:   generationTime,
			OriginalSize:     0,
			CompressedSize:   0,
			CompressionRatio: 0,
			Strategy:         0, // Basic fallback strategy
			Confidence:       1.0,
			FallbackUsed:     true,
		}
	}

	// Update current data state
	p.data = newData

	return []*Fragment{fragment}, nil
}

// Close cleans up page resources
func (p *Page) Close() error {
	// For the basic API, minimal cleanup is needed
	// In the full multi-tenant version, this would clean up tokens, registry entries, etc.
	p.currentDataMutex.Lock()
	defer p.currentDataMutex.Unlock()

	p.data = nil
	return nil
}

// injectLvtIds automatically injects lvt-id attributes with numeric IDs
func (p *Page) injectLvtIds(html string) string {
	// Find elements that contain template expressions and need fragment tracking
	// For simplicity, we'll look for any elements that might contain dynamic content
	// This is a simple implementation - could be made more sophisticated

	// Reset fragment counter for rendering (keeps consistent IDs)
	lvtIdCounter := 0

	// First, remove ALL existing lvt-id attributes from ALL elements
	// We'll add them back in a controlled manner based on our logic
	html = regexp.MustCompile(`\s*lvt-id="[^"]*"`).ReplaceAllString(html, "")

	// Regex to find elements that contain template expressions or existing lvt-id attributes
	// We'll inject lvt-id into div elements that seem to have dynamic content
	elementRegex := regexp.MustCompile(`<(div|span|p|h[1-6]|section|article|main)([^>]*?)>`)

	result := elementRegex.ReplaceAllStringFunc(html, func(match string) string {
		// Check if this element needs an lvt-id attribute
		// Simple heuristic: if the content contains style attributes or class attributes (dynamic content)
		if strings.Contains(match, "style=") || strings.Contains(match, "class=") {
			lvtIdCounter++
			// Insert lvt-id attribute before the closing >
			return strings.Replace(match, ">", fmt.Sprintf(` lvt-id="%d">`, lvtIdCounter), 1)
		}

		return match
	})

	return result
}

// extractFragmentID extracts the first lvt-id attribute from the template source
func (p *Page) extractFragmentID(templateSource string) string {
	// Look for lvt-id="value" in the template
	re := regexp.MustCompile(`lvt-id="([^"]+)"`)
	matches := re.FindStringSubmatch(templateSource)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Helper functions
