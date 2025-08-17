package livetemplate

import (
	"context"
	"html/template"
	"time"

	"github.com/livefir/livetemplate/internal/app"
)

// Application provides secure multi-tenant isolation with JWT-based authentication
type Application struct {
	internal *app.Application
	config   *ApplicationConfig
}

// ApplicationConfig contains configuration for the public Application
type ApplicationConfig struct {
	MaxMemoryMB    int
	MetricsEnabled bool
}

// ApplicationOption configures an Application instance
type ApplicationOption func(*Application) error

// NewApplication creates a new isolated Application instance
func NewApplication(options ...ApplicationOption) (*Application, error) {
	// Initialize with default configuration
	publicApp := &Application{
		config: &ApplicationConfig{
			MaxMemoryMB:    100,
			MetricsEnabled: true,
		},
	}

	// Apply public options to collect configuration
	for _, option := range options {
		if err := option(publicApp); err != nil {
			return nil, err
		}
	}

	// Convert public options to internal options
	var internalOptions []app.Option
	if publicApp.config.MaxMemoryMB != 100 {
		internalOptions = append(internalOptions, app.WithMaxMemoryMB(publicApp.config.MaxMemoryMB))
	}
	if !publicApp.config.MetricsEnabled {
		internalOptions = append(internalOptions, app.WithMetricsEnabled(false))
	}

	// Create internal application with collected configuration
	internal, err := app.NewApplication(internalOptions...)
	if err != nil {
		return nil, err
	}

	publicApp.internal = internal
	return publicApp, nil
}

// WithMaxPages sets the maximum number of pages
func WithMaxPages(maxPages int) ApplicationOption {
	return func(a *Application) error {
		// Configuration will be applied when creating internal application
		return nil
	}
}

// WithPageTTL sets the page time-to-live
func WithPageTTL(ttl time.Duration) ApplicationOption {
	return func(a *Application) error {
		// Configuration will be applied when creating internal application
		return nil
	}
}

// WithMaxMemoryMB sets the maximum memory usage in MB
func WithMaxMemoryMB(memoryMB int) ApplicationOption {
	return func(a *Application) error {
		a.config.MaxMemoryMB = memoryMB
		return nil
	}
}

// WithApplicationMetricsEnabled configures metrics collection for the application
func WithApplicationMetricsEnabled(enabled bool) ApplicationOption {
	return func(a *Application) error {
		a.config.MetricsEnabled = enabled
		return nil
	}
}

// ApplicationPageOption configures a page created by an Application
type ApplicationPageOption func(*ApplicationPage) error

// ApplicationPage represents a page managed by an Application with JWT token support
type ApplicationPage struct {
	internal *app.Page
}

// NewApplicationPage creates a new isolated page session with JWT token
func (a *Application) NewApplicationPage(tmpl *template.Template, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error) {
	// Convert public options to internal options
	var internalOptions []app.PageOption

	// Create internal page
	internal, err := a.internal.NewPage(tmpl, data, internalOptions...)
	if err != nil {
		return nil, err
	}

	publicPage := &ApplicationPage{internal: internal}

	// Apply public options
	for _, option := range options {
		if err := option(publicPage); err != nil {
			_ = publicPage.Close() // Cleanup on error
			return nil, err
		}
	}

	return publicPage, nil
}

// GetApplicationPage retrieves a page by JWT token with application boundary enforcement
func (a *Application) GetApplicationPage(token string) (*ApplicationPage, error) {
	internal, err := a.internal.GetPage(token)
	if err != nil {
		return nil, err
	}

	return &ApplicationPage{internal: internal}, nil
}

// GetPageCount returns the total number of active pages
func (a *Application) GetPageCount() int {
	return a.internal.GetPageCount()
}

// CleanupExpiredPages removes expired pages and returns count
func (a *Application) CleanupExpiredPages() int {
	return a.internal.CleanupExpiredPages()
}

// GetApplicationMetrics returns application metrics
func (a *Application) GetApplicationMetrics() ApplicationMetrics {
	internal := a.internal.GetMetrics()

	return ApplicationMetrics{
		ApplicationID:      internal.ApplicationID,
		PagesCreated:       internal.PagesCreated,
		PagesDestroyed:     internal.PagesDestroyed,
		ActivePages:        internal.ActivePages,
		MaxConcurrentPages: internal.MaxConcurrentPages,
		TokensGenerated:    internal.TokensGenerated,
		TokensVerified:     internal.TokensVerified,
		TokenFailures:      internal.TokenFailures,
		FragmentsGenerated: internal.FragmentsGenerated,
		GenerationErrors:   internal.GenerationErrors,
		MemoryUsage:        internal.MemoryUsage,
		MemoryUsagePercent: internal.MemoryUsagePercent,
		MemoryStatus:       internal.MemoryStatus,
		RegistryCapacity:   internal.RegistryCapacity,
		Uptime:             internal.Uptime,
		StartTime:          internal.StartTime,
	}
}

// Close releases all application resources
func (a *Application) Close() error {
	return a.internal.Close()
}

// ApplicationPage methods

// Render generates the complete HTML output for the current page state
func (p *ApplicationPage) Render() (string, error) {
	return p.internal.Render()
}

// RenderFragments generates fragment updates for the given new data
func (p *ApplicationPage) RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error) {
	internalFragments, err := p.internal.RenderFragments(ctx, newData)
	if err != nil {
		return nil, err
	}

	// Convert to existing Fragment type format
	fragments := make([]*Fragment, len(internalFragments))
	for i, frag := range internalFragments {
		fragments[i] = &Fragment{
			ID:       frag.ID,
			Strategy: frag.Strategy,
			Action:   frag.Action,
			Data:     frag.Data,
			Metadata: convertInternalMetadata(frag.Metadata),
		}
	}

	return fragments, nil
}

// GetToken returns the JWT token for this page
func (p *ApplicationPage) GetToken() string {
	return p.internal.GetToken()
}

// SetData updates the page data state
func (p *ApplicationPage) SetData(data interface{}) error {
	return p.internal.SetData(data)
}

// GetData returns the current page data
func (p *ApplicationPage) GetData() interface{} {
	return p.internal.GetData()
}

// GetTemplate returns the page template
func (p *ApplicationPage) GetTemplate() *template.Template {
	return p.internal.GetTemplate()
}

// GetApplicationPageMetrics returns page-specific metrics
func (p *ApplicationPage) GetApplicationPageMetrics() ApplicationPageMetrics {
	internal := p.internal.GetMetrics()

	return ApplicationPageMetrics{
		PageID:                internal.PageID,
		ApplicationID:         internal.ApplicationID,
		CreatedAt:             internal.CreatedAt,
		LastAccessed:          internal.LastAccessed,
		Age:                   internal.Age,
		IdleTime:              internal.IdleTime,
		MemoryUsage:           internal.MemoryUsage,
		FragmentCacheSize:     internal.FragmentCacheSize,
		TotalGenerations:      internal.TotalGenerations,
		SuccessfulGenerations: internal.SuccessfulGenerations,
		FailedGenerations:     internal.FailedGenerations,
		AverageGenerationTime: internal.AverageGenerationTime,
		ErrorRate:             internal.ErrorRate,
	}
}

// Close releases page resources and removes from application
func (p *ApplicationPage) Close() error {
	return p.internal.Close()
}

// Public API types for Application

// ApplicationMetrics contains application performance data
type ApplicationMetrics struct {
	ApplicationID      string        `json:"application_id"`
	PagesCreated       int64         `json:"pages_created"`
	PagesDestroyed     int64         `json:"pages_destroyed"`
	ActivePages        int64         `json:"active_pages"`
	MaxConcurrentPages int64         `json:"max_concurrent_pages"`
	TokensGenerated    int64         `json:"tokens_generated"`
	TokensVerified     int64         `json:"tokens_verified"`
	TokenFailures      int64         `json:"token_failures"`
	FragmentsGenerated int64         `json:"fragments_generated"`
	GenerationErrors   int64         `json:"generation_errors"`
	MemoryUsage        int64         `json:"memory_usage"`
	MemoryUsagePercent float64       `json:"memory_usage_percent"`
	MemoryStatus       string        `json:"memory_status"`
	RegistryCapacity   float64       `json:"registry_capacity"`
	Uptime             time.Duration `json:"uptime"`
	StartTime          time.Time     `json:"start_time"`
}

// ApplicationPageMetrics contains page performance data for Application-managed pages
type ApplicationPageMetrics struct {
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

// convertInternalMetadata converts internal metadata to existing FragmentMetadata format
func convertInternalMetadata(internal *app.Metadata) *FragmentMetadata {
	if internal == nil {
		return nil
	}

	// Parse the generation time string back to duration
	genTime, _ := time.ParseDuration(internal.GenerationTime)

	return &FragmentMetadata{
		GenerationTime:   genTime,
		OriginalSize:     internal.OriginalSize,
		CompressedSize:   internal.CompressedSize,
		CompressionRatio: internal.CompressionRatio,
		Strategy:         internal.Strategy,
		Confidence:       internal.Confidence,
		FallbackUsed:     internal.FallbackUsed,
	}
}
