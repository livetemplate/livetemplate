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

	// Update generation pipeline
	updateGenerator *strategy.UpdateGenerator

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
		template:        tmpl,
		data:            data,
		updateGenerator: strategy.NewUpdateGenerator(),
		enableMetrics:   true,
		created:         time.Now(),
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
		p.updateGenerator.SetMetricsEnabled(enabled)
		return nil
	}
}

// WithFallbackEnabled configures whether fallback strategies are enabled
func WithFallbackEnabled(enabled bool) PageOption {
	return func(p *Page) error {
		p.updateGenerator.SetFallbackEnabled(enabled)
		return nil
	}
}

// WithMaxGenerationTime sets the maximum time allowed for update generation
func WithMaxGenerationTime(duration time.Duration) PageOption {
	return func(p *Page) error {
		p.updateGenerator.SetMaxGenerationTime(duration)
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

// RenderFragments generates fragment updates for the given new data
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error) {
	p.currentDataMutex.Lock()
	defer p.currentDataMutex.Unlock()

	// Generate fragments using the update generator pipeline
	oldData := p.data
	fragments, err := p.updateGenerator.GenerateUpdate(p.template, oldData, newData)
	if err != nil {
		return nil, fmt.Errorf("fragment generation failed: %w", err)
	}

	// Convert internal fragments to public API fragments
	publicFragments := make([]*Fragment, len(fragments))
	for i, frag := range fragments {
		publicFragments[i] = &Fragment{
			ID:       frag.ID,
			Strategy: frag.Strategy,
			Action:   frag.Action,
			Data:     frag.Data,
			Metadata: convertMetadata(frag.Metadata),
		}
	}

	// Update current data state
	p.data = newData

	return publicFragments, nil
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
	if !p.enableMetrics {
		return &UpdateGeneratorMetrics{}
	}

	metrics := p.updateGenerator.GetMetrics()
	return &UpdateGeneratorMetrics{
		TotalGenerations:      metrics.TotalGenerations,
		SuccessfulGenerations: metrics.SuccessfulGenerations,
		FailedGenerations:     metrics.FailedGenerations,
		StrategyUsage:         copyStrategyUsage(metrics.StrategyUsage),
		AverageGenerationTime: metrics.AverageGenerationTime,
		TotalBandwidthSaved:   metrics.TotalBandwidthSaved,
		FallbackRate:          metrics.FallbackRate,
		ErrorRate:             metrics.ErrorRate,
		LastReset:             metrics.LastReset,
	}
}

// ResetMetrics resets all fragment generation metrics
func (p *Page) ResetMetrics() {
	if p.enableMetrics {
		p.updateGenerator.ResetMetrics()
	}
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

func convertMetadata(internal *strategy.FragmentMetadata) *FragmentMetadata {
	if internal == nil {
		return nil
	}

	return &FragmentMetadata{
		GenerationTime:   internal.GenerationTime,
		OriginalSize:     internal.OriginalSize,
		CompressedSize:   internal.CompressedSize,
		CompressionRatio: internal.CompressionRatio,
		Strategy:         internal.Strategy,
		Confidence:       internal.Confidence,
		FallbackUsed:     internal.FallbackUsed,
	}
}

func copyStrategyUsage(original map[string]int64) map[string]int64 {
	copy := make(map[string]int64)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}
