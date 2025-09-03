package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"time"

	"github.com/livefir/livetemplate/internal/strategy"
)

// Fragment represents a generated update fragment with strategy-specific data
type Fragment struct {
	ID       string            `json:"id"`
	Strategy string            `json:"strategy"` // "static_dynamic", "markers", "granular", "replacement"
	Action   string            `json:"action"`   // Strategy-specific action
	Data     interface{}       `json:"data"`     // Strategy-specific payload
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
	template *template.Template

	// Current data state
	data             interface{}
	currentDataMutex sync.RWMutex

	// Update generation pipeline - now using tree-based generator
	treeGenerator *strategy.SimpleTreeGenerator

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
		template:      tmpl,
		data:          data,
		treeGenerator: strategy.NewSimpleTreeGenerator(),
		enableMetrics: true,
		created:       time.Now(),
	}

	// Apply options
	for _, option := range options {
		if err := option(page); err != nil {
			return nil, fmt.Errorf("failed to apply page option: %w", err)
		}
	}

	return page, nil
}

// WithMetricsEnabled configures whether metrics collection is enabled
func WithMetricsEnabled(enabled bool) PageOption {
	return func(p *Page) error {
		p.enableMetrics = enabled
		// Tree generator doesn't have configurable metrics yet
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

	return buf.String(), nil
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

	// Extract template source - simplified for tree generator
	templateSource := p.template.Name()
	if templateSource == "" {
		templateSource = "template"
	}

	// Generate fragments using the tree generator
	oldData := p.data
	fragmentID := fmt.Sprintf("fragment_%d", time.Now().UnixNano())

	var startTime time.Time
	if config.IncludeMetadata {
		startTime = time.Now()
	}

	treeResult, err := p.treeGenerator.GenerateFromTemplateSource(templateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("tree fragment generation failed: %w", err)
	}

	// Create fragment from tree result
	fragment := &Fragment{
		ID:       fragmentID,
		Strategy: "tree_based",
		Action:   "update_tree",
		Data:     treeResult,
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
			Strategy:         1, // Tree-based strategy
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

// GetMetrics returns current fragment generation metrics
func (p *Page) GetMetrics() *UpdateGeneratorMetrics {
	// Tree generator doesn't expose detailed metrics yet
	// Return empty metrics structure for backward compatibility
	return &UpdateGeneratorMetrics{
		TotalGenerations:      0,
		SuccessfulGenerations: 0,
		FailedGenerations:     0,
		StrategyUsage:         make(map[string]int64),
		AverageGenerationTime: 0,
		TotalBandwidthSaved:   0,
		FallbackRate:          0,
		ErrorRate:             0,
		LastReset:             p.created,
	}
}

// ResetMetrics resets all fragment generation metrics
func (p *Page) ResetMetrics() {
	// Tree generator doesn't have metrics to reset yet
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

// Close cleans up page resources
func (p *Page) Close() error {
	// For the basic API, minimal cleanup is needed
	// In the full multi-tenant version, this would clean up tokens, registry entries, etc.
	p.currentDataMutex.Lock()
	defer p.currentDataMutex.Unlock()

	p.data = nil
	return nil
}

// Helper functions
