package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/livefir/livetemplate/internal/app"
)

// Application provides secure multi-tenant isolation with JWT-based authentication
type Application struct {
	internal  *app.Application
	config    *ApplicationConfig
	templates map[string]*template.Template // Template registry for reuse
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
		templates: make(map[string]*template.Template), // Initialize template registry
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

// WithCacheInfo sets the client cache information for the page
func WithCacheInfo(cacheInfo *ClientCacheInfo) ApplicationPageOption {
	return func(p *ApplicationPage) error {
		p.cacheInfo = cacheInfo
		return nil
	}
}

// ApplicationPage represents a page managed by an Application with JWT token support
type ApplicationPage struct {
	internal  *app.Page
	cacheInfo *ClientCacheInfo
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

// RegisterTemplate registers a template with a name for reuse
func (a *Application) RegisterTemplate(name string, tmpl *template.Template) error {
	if tmpl == nil {
		return fmt.Errorf("template cannot be nil")
	}
	if name == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	a.templates[name] = tmpl
	return nil
}

// RegisterTemplateFromFile parses and registers a template from a file
func (a *Application) RegisterTemplateFromFile(name string, filepath string) error {
	tmpl, err := template.ParseFiles(filepath)
	if err != nil {
		return fmt.Errorf("failed to parse template file %s: %w", filepath, err)
	}

	return a.RegisterTemplate(name, tmpl)
}

// NewPageFromTemplate creates a new page using a registered template
func (a *Application) NewPageFromTemplate(templateName string, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error) {
	tmpl, exists := a.templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template %q not registered", templateName)
	}

	return a.NewApplicationPage(tmpl, data, options...)
}

// NewPage creates a new page using a registered template (simplified name)
func (a *Application) NewPage(templateName string, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error) {
	return a.NewPageFromTemplate(templateName, data, options...)
}

// GetRegisteredTemplates returns the names of all registered templates
func (a *Application) GetRegisteredTemplates() []string {
	names := make([]string, 0, len(a.templates))
	for name := range a.templates {
		names = append(names, name)
	}
	return names
}

// GetApplicationPage retrieves a page by JWT token with application boundary enforcement
func (a *Application) GetApplicationPage(token string, options ...ApplicationPageOption) (*ApplicationPage, error) {
	internal, err := a.internal.GetPage(token)
	if err != nil {
		return nil, err
	}

	publicPage := &ApplicationPage{internal: internal}

	// Apply options
	for _, option := range options {
		if err := option(publicPage); err != nil {
			return nil, err
		}
	}

	return publicPage, nil
}

// GetPage retrieves a page by JWT token (simplified name)
func (a *Application) GetPage(token string, options ...ApplicationPageOption) (*ApplicationPage, error) {
	return a.GetApplicationPage(token, options...)
}

// GetPageFromRequest extracts token from request and cache info from query params
// This is the most convenient method for WebSocket handlers
func (a *Application) GetPageFromRequest(r *http.Request) (*ApplicationPage, error) {
	// Extract token from query param
	token := r.URL.Query().Get("token")
	if token == "" {
		return nil, fmt.Errorf("no token provided in request")
	}

	// Parse cache information from URL query parameters
	cacheInfo := ParseCacheInfoFromURL(r.URL.Query())

	// Get page with cache context
	return a.GetPage(token, WithCacheInfo(cacheInfo))
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

// ClientCacheInfo contains information about what the client has cached
type ClientCacheInfo struct {
	HasCache        bool            `json:"has_cache"`
	CachedFragments map[string]bool `json:"cached_fragments"`
}

// RenderFragments generates fragment updates for the given new data
// Automatically uses the page's cache information if set
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
			Data:     frag.Data,
			Metadata: convertInternalMetadata(frag.Metadata),
		}
	}

	// Apply client cache filtering if cache info is set on the page
	if p.cacheInfo != nil && p.cacheInfo.HasCache {
		fragments = p.filterStaticsFromFragments(fragments, p.cacheInfo.CachedFragments)
	}

	return fragments, nil
}

// filterStaticsFromFragments removes statics from fragments that client already has cached
func (p *ApplicationPage) filterStaticsFromFragments(fragments []*Fragment, cachedFragments map[string]bool) []*Fragment {
	var filtered []*Fragment

	for _, frag := range fragments {
		// Create a copy of the fragment
		newFrag := &Fragment{
			ID:       frag.ID,
			Data:     frag.Data,
			Metadata: frag.Metadata,
		}

		// If client has this fragment cached, remove statics from data
		if cachedFragments[frag.ID] {
			if treeData, ok := frag.Data.(map[string]interface{}); ok {
				// Create new tree data without statics
				newTreeData := make(map[string]interface{})
				for k, v := range treeData {
					if k != "s" { // Remove statics ("s" field)
						newTreeData[k] = v
					}
				}
				newFrag.Data = newTreeData
			}
		}

		filtered = append(filtered, newFrag)
	}

	return filtered
}

// GetToken returns the JWT token for this page
func (p *ApplicationPage) GetToken() string {
	return p.internal.GetToken()
}

// SetData updates the page data state
func (p *ApplicationPage) SetData(data interface{}) error {
	return p.internal.SetData(data)
}

// SetCacheInfo updates the client cache information for this page
func (p *ApplicationPage) SetCacheInfo(cacheInfo *ClientCacheInfo) {
	p.cacheInfo = cacheInfo
}

// GetCacheInfo returns the current cache information for this page
func (p *ApplicationPage) GetCacheInfo() *ClientCacheInfo {
	return p.cacheInfo
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

// ParseCacheInfoFromURL extracts cache information from URL query parameters
func ParseCacheInfoFromURL(queryValues url.Values) *ClientCacheInfo {
	hasCache := queryValues.Get("has_cache") == "true"
	if !hasCache {
		return &ClientCacheInfo{HasCache: false, CachedFragments: make(map[string]bool)}
	}

	cachedFragments := make(map[string]bool)
	if fragmentsList := queryValues.Get("cached_fragments"); fragmentsList != "" {
		for _, fragID := range strings.Split(fragmentsList, ",") {
			if fragID != "" {
				cachedFragments[fragID] = true
			}
		}
	}

	return &ClientCacheInfo{
		HasCache:        true,
		CachedFragments: cachedFragments,
	}
}

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
