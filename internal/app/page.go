package app

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/livefir/livetemplate/internal/page"
)

// Page represents a public page wrapper that integrates with Application
type Page struct {
	internal *page.Page
	token    string
	app      *Application
}

// PageOption configures a Page instance
type PageOption func(*Page) error

// Render generates the complete HTML output for the current page state
func (p *Page) Render() (string, error) {
	if p.app.closed {
		return "", fmt.Errorf("application is closed")
	}

	return p.internal.Render()
}

// RenderFragments generates fragment updates for the given new data
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error) {
	if p.app.closed {
		return nil, fmt.Errorf("application is closed")
	}

	// Generate fragments using internal page
	internalFragments, err := p.internal.RenderFragments(ctx, newData)
	if err != nil {
		// Update error metrics
		if p.app.metrics != nil {
			p.app.metrics.IncrementGenerationError()
		}
		return nil, err
	}

	// Convert to public API fragments
	fragments := make([]*Fragment, len(internalFragments))
	for i, frag := range internalFragments {
		fragments[i] = &Fragment{
			ID:       frag.ID,
			Data:     frag.Data,
			Metadata: convertMetadata(frag.Metadata),
		}
	}

	// Update success metrics
	if p.app.metrics != nil {
		for range fragments {
			p.app.metrics.IncrementFragmentGenerated()
		}
	}

	// Update memory usage tracking
	newMemoryUsage := p.internal.GetMemoryUsage()
	_ = p.app.memoryManager.UpdatePageUsage(p.internal.ID, newMemoryUsage)
	// Memory tracking errors are non-critical and don't affect fragment generation

	return fragments, nil
}

// GetToken returns the stable cache token for this page
func (p *Page) GetToken() string {
	return p.token
}

// GetID returns the page ID
func (p *Page) GetID() string {
	return p.internal.GetID()
}

// GenerateSecurityToken creates a fresh JWT token for secure operations
// This prevents replay protection issues by generating a new token each time
func (p *Page) GenerateSecurityToken() (string, error) {
	if p.app.closed {
		return "", fmt.Errorf("application is closed")
	}

	// Generate a new token for the same page ID to avoid replay protection
	return p.app.tokenService.GenerateToken(p.app.id, p.internal.ID)
}

// SetData updates the page data state
func (p *Page) SetData(data interface{}) error {
	if p.app.closed {
		return fmt.Errorf("application is closed")
	}

	err := p.internal.SetData(data)
	if err != nil {
		return err
	}

	// Update memory usage tracking
	newMemoryUsage := p.internal.GetMemoryUsage()
	_ = p.app.memoryManager.UpdatePageUsage(p.internal.ID, newMemoryUsage)
	// Memory tracking errors are non-critical and don't affect data setting

	return nil
}

// GetData returns the current page data
func (p *Page) GetData() interface{} {
	return p.internal.GetData()
}

// GetTemplate returns the page template
func (p *Page) GetTemplate() *template.Template {
	return p.internal.GetTemplate()
}

// GetMetrics returns page-specific metrics
func (p *Page) GetMetrics() PageMetrics {
	internalMetrics := p.internal.GetMetrics()

	return PageMetrics{
		PageID:                internalMetrics.PageID,
		ApplicationID:         internalMetrics.ApplicationID,
		CreatedAt:             internalMetrics.CreatedAt.Format(time.RFC3339),
		LastAccessed:          internalMetrics.LastAccessed.Format(time.RFC3339),
		Age:                   internalMetrics.Age.String(),
		IdleTime:              internalMetrics.IdleTime.String(),
		MemoryUsage:           internalMetrics.MemoryUsage,
		FragmentCacheSize:     internalMetrics.FragmentCacheSize,
		TotalGenerations:      internalMetrics.TotalGenerations,
		SuccessfulGenerations: internalMetrics.SuccessfulGenerations,
		FailedGenerations:     internalMetrics.FailedGenerations,
		AverageGenerationTime: internalMetrics.AverageGenerationTime.String(),
		ErrorRate:             internalMetrics.ErrorRate,
	}
}

// Close releases page resources and removes from application
func (p *Page) Close() error {
	if p.app.closed {
		return fmt.Errorf("application is closed")
	}

	p.app.mu.Lock()
	defer p.app.mu.Unlock()

	// Remove from registry
	removed := p.app.pageRegistry.Remove(p.internal.ID)

	// Deallocate memory
	p.app.memoryManager.DeallocatePage(p.internal.ID)

	// Update metrics
	if removed && p.app.metrics != nil {
		p.app.metrics.IncrementPageDestroyed()
	}

	// Close internal page
	return p.internal.Close()
}

// Fragment represents a generated update fragment for the public API
type Fragment struct {
	ID       string      `json:"id"`
	Data     interface{} `json:"data"`
	Metadata *Metadata   `json:"metadata,omitempty"`
}

// Metadata contains performance information for fragments
type Metadata struct {
	GenerationTime   string  `json:"generation_time"`
	OriginalSize     int     `json:"original_size"`
	CompressedSize   int     `json:"compressed_size"`
	CompressionRatio float64 `json:"compression_ratio"`
	Strategy         int     `json:"strategy_number"`
	Confidence       float64 `json:"confidence"`
	FallbackUsed     bool    `json:"fallback_used"`
}

// PageMetrics contains page performance data for the public API
type PageMetrics struct {
	PageID                string  `json:"page_id"`
	ApplicationID         string  `json:"application_id"`
	CreatedAt             string  `json:"created_at"`
	LastAccessed          string  `json:"last_accessed"`
	Age                   string  `json:"age"`
	IdleTime              string  `json:"idle_time"`
	MemoryUsage           int64   `json:"memory_usage"`
	FragmentCacheSize     int     `json:"fragment_cache_size"`
	TotalGenerations      int64   `json:"total_generations"`
	SuccessfulGenerations int64   `json:"successful_generations"`
	FailedGenerations     int64   `json:"failed_generations"`
	AverageGenerationTime string  `json:"average_generation_time"`
	ErrorRate             float64 `json:"error_rate"`
}

// Helper functions

// convertMetadata converts internal metadata to public API format
func convertMetadata(internal *page.Metadata) *Metadata {
	if internal == nil {
		return nil
	}

	return &Metadata{
		GenerationTime:   internal.GenerationTime.String(),
		OriginalSize:     internal.OriginalSize,
		CompressedSize:   internal.CompressedSize,
		CompressionRatio: internal.CompressionRatio,
		Strategy:         internal.Strategy,
		Confidence:       internal.Confidence,
		FallbackUsed:     internal.FallbackUsed,
	}
}

// SetTemplateSource sets the template source for the page
func (p *Page) SetTemplateSource(templateSource string) {
	if p.internal != nil {
		p.internal.SetTemplateSource(templateSource)
	}
}

// SetTemplateRegions sets the cached template regions for consistent ID generation
func (p *Page) SetTemplateRegions(regions []page.TemplateRegion) {
	if p.internal != nil {
		p.internal.SetTemplateRegions(regions)
	}
}
