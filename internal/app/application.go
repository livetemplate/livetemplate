package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"sync"
	"time"

	"github.com/livefir/livetemplate/internal/memory"
	"github.com/livefir/livetemplate/internal/metrics"
	"github.com/livefir/livetemplate/internal/page"
	"github.com/livefir/livetemplate/internal/token"
)

// Application provides secure multi-tenant isolation with JWT-based authentication
type Application struct {
	id            string
	tokenService  *token.TokenService
	pageRegistry  *page.Registry
	memoryManager *memory.Manager
	metrics       *metrics.Collector
	config        *Config
	mu            sync.RWMutex
	closed        bool
}

// Config defines Application configuration
type Config struct {
	// Page management
	MaxPages        int           // Default: 1000
	PageTTL         time.Duration // Default: 1 hour
	CleanupInterval time.Duration // Default: 5 minutes

	// Token configuration
	TokenTTL time.Duration // Default: 24 hours

	// Memory limits
	MaxMemoryMB int // Default: 100MB

	// Security settings
	EnableMetrics bool // Default: true
}

// DefaultConfig returns secure default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxPages:        1000,
		PageTTL:         1 * time.Hour,
		CleanupInterval: 5 * time.Minute,
		TokenTTL:        24 * time.Hour,
		MaxMemoryMB:     100,
		EnableMetrics:   true,
	}
}

// Option configures an Application instance
type Option func(*Application) error

// NewApplication creates a new isolated Application instance
func NewApplication(options ...Option) (*Application, error) {
	// Generate unique application ID
	appID, err := generateApplicationID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate application ID: %w", err)
	}

	// Create default configuration
	config := DefaultConfig()

	// Create temporary app to apply options to config
	tempApp := &Application{
		config: config,
	}

	// Apply options to update configuration BEFORE creating components
	for _, option := range options {
		if err := option(tempApp); err != nil {
			return nil, fmt.Errorf("failed to apply application option: %w", err)
		}
	}

	// Create token service with final config
	tokenConfig := &token.Config{
		TTL:               config.TokenTTL,
		NonceWindow:       5 * time.Minute,
		MaxNoncePerWindow: 1000,
	}
	tokenService, err := token.NewTokenService(tokenConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create token service: %w", err)
	}

	// Create page registry with final config
	registryConfig := &page.RegistryConfig{
		MaxPages:        config.MaxPages,
		DefaultTTL:      config.PageTTL,
		CleanupInterval: config.CleanupInterval,
	}
	pageRegistry := page.NewRegistry(registryConfig)

	// Create memory manager with final config
	memoryConfig := &memory.Config{
		MaxMemoryMB:          config.MaxMemoryMB,
		WarningThresholdPct:  75,
		CriticalThresholdPct: 90,
		CleanupInterval:      1 * time.Minute,
		EnableGCTuning:       true,
		LeakDetectionEnabled: true,
		ComponentTracking:    true,
	}
	memoryManager := memory.NewManager(memoryConfig)

	// Create metrics collector with final config
	var metricsCollector *metrics.Collector
	if config.EnableMetrics {
		metricsCollector = metrics.NewCollector()
	}

	app := &Application{
		id:            appID,
		tokenService:  tokenService,
		pageRegistry:  pageRegistry,
		memoryManager: memoryManager,
		metrics:       metricsCollector,
		config:        config,
		closed:        false,
	}

	return app, nil
}

// WithMaxPages sets the maximum number of pages
func WithMaxPages(maxPages int) Option {
	return func(app *Application) error {
		if maxPages <= 0 {
			return fmt.Errorf("maxPages must be positive")
		}
		app.config.MaxPages = maxPages
		return nil
	}
}

// WithPageTTL sets the page time-to-live
func WithPageTTL(ttl time.Duration) Option {
	return func(app *Application) error {
		if ttl <= 0 {
			return fmt.Errorf("TTL must be positive")
		}
		app.config.PageTTL = ttl
		return nil
	}
}

// WithMaxMemoryMB sets the maximum memory usage in MB
func WithMaxMemoryMB(memoryMB int) Option {
	return func(app *Application) error {
		if memoryMB <= 0 {
			return fmt.Errorf("memory limit must be positive")
		}
		app.config.MaxMemoryMB = memoryMB
		return nil
	}
}

// WithMetricsEnabled configures metrics collection
func WithMetricsEnabled(enabled bool) Option {
	return func(app *Application) error {
		app.config.EnableMetrics = enabled
		if !enabled {
			app.metrics = nil
		}
		return nil
	}
}

// NewPage creates a new isolated page session
func (app *Application) NewPage(tmpl *template.Template, data interface{}, options ...PageOption) (*Page, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	if app.closed {
		return nil, fmt.Errorf("application is closed")
	}

	// Estimate memory usage for the new page
	estimatedSize := estimatePageMemoryUsage(tmpl, data)

	// Check memory limits
	if !app.memoryManager.CanAllocate(estimatedSize) {
		return nil, fmt.Errorf("insufficient memory for new page")
	}

	// Create internal page
	pageConfig := page.DefaultConfig()
	internalPage, err := page.NewPage(app.id, tmpl, data, pageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Allocate memory for the page
	if err := app.memoryManager.AllocatePage(internalPage.ID, estimatedSize); err != nil {
		return nil, fmt.Errorf("failed to allocate memory: %w", err)
	}

	// Store page in registry
	if err := app.pageRegistry.Store(internalPage); err != nil {
		app.memoryManager.DeallocatePage(internalPage.ID) // Cleanup memory
		return nil, fmt.Errorf("failed to store page: %w", err)
	}

	// Generate token for page access
	tokenString, err := app.tokenService.GenerateToken(app.id, internalPage.ID)
	if err != nil {
		app.pageRegistry.Remove(internalPage.ID)
		app.memoryManager.DeallocatePage(internalPage.ID)
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Update metrics
	if app.metrics != nil {
		app.metrics.IncrementPageCreated()
		app.metrics.IncrementTokenGenerated()
	}

	// Create public page wrapper
	publicPage := &Page{
		internal: internalPage,
		token:    tokenString,
		app:      app,
	}

	// Apply page options
	for _, option := range options {
		if err := option(publicPage); err != nil {
			_ = publicPage.Close() // Cleanup on error
			return nil, fmt.Errorf("failed to apply page option: %w", err)
		}
	}

	return publicPage, nil
}

// GetPage retrieves a page by token with application boundary enforcement
func (app *Application) GetPage(tokenString string) (*Page, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	if app.closed {
		return nil, fmt.Errorf("application is closed")
	}

	// Verify token and extract claims
	claims, err := app.tokenService.VerifyToken(tokenString)
	if err != nil {
		if app.metrics != nil {
			app.metrics.IncrementTokenFailure()
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Enforce application isolation
	if claims.ApplicationID != app.id {
		if app.metrics != nil {
			app.metrics.IncrementTokenFailure()
		}
		return nil, fmt.Errorf("cross-application access denied")
	}

	// Retrieve page from registry
	internalPage, err := app.pageRegistry.Get(claims.PageID, app.id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve page: %w", err)
	}

	// Update metrics
	if app.metrics != nil {
		app.metrics.IncrementTokenVerified()
	}

	// Create public page wrapper
	return &Page{
		internal: internalPage,
		token:    tokenString,
		app:      app,
	}, nil
}

// GetPageCount returns the total number of active pages
func (app *Application) GetPageCount() int {
	app.mu.RLock()
	defer app.mu.RUnlock()

	if app.closed {
		return 0
	}

	return app.pageRegistry.GetPageCount()
}

// CleanupExpiredPages removes expired pages and returns count
func (app *Application) CleanupExpiredPages() int {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.closed {
		return 0
	}

	// Cleanup expired pages from registry
	expiredCount := app.pageRegistry.CleanupExpired()

	// Cleanup expired nonces from token service
	app.tokenService.CleanupExpiredNonces()

	// Update metrics
	if app.metrics != nil {
		app.metrics.IncrementCleanupOperation(int64(expiredCount))
	}

	return expiredCount
}

// GetMetrics returns application metrics
func (app *Application) GetMetrics() ApplicationMetrics {
	app.mu.RLock()
	defer app.mu.RUnlock()

	if app.closed || app.metrics == nil {
		return ApplicationMetrics{}
	}

	// Get metrics from collector
	metricsData := app.metrics.GetMetrics()

	// Get memory status
	memoryStatus := app.memoryManager.GetMemoryStatus()

	// Get registry metrics
	registryMetrics := app.pageRegistry.GetMetrics()

	return ApplicationMetrics{
		ApplicationID:      app.id,
		PagesCreated:       metricsData.PagesCreated,
		PagesDestroyed:     metricsData.PagesDestroyed,
		ActivePages:        metricsData.ActivePages,
		MaxConcurrentPages: metricsData.MaxConcurrentPages,
		TokensGenerated:    metricsData.TokensGenerated,
		TokensVerified:     metricsData.TokensVerified,
		TokenFailures:      metricsData.TokenFailures,
		FragmentsGenerated: metricsData.FragmentsGenerated,
		GenerationErrors:   metricsData.GenerationErrors,
		MemoryUsage:        memoryStatus.CurrentUsage,
		MemoryUsagePercent: memoryStatus.UsagePercentage,
		MemoryStatus:       memoryStatus.Level,
		RegistryCapacity:   float64(registryMetrics.TotalPages) / float64(registryMetrics.MaxCapacity),
		Uptime:             metricsData.Uptime,
		StartTime:          metricsData.StartTime,
	}
}

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

// Close releases all application resources
func (app *Application) Close() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.closed {
		return nil
	}
	app.closed = true

	var errors []error

	// Close page registry
	if err := app.pageRegistry.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close page registry: %w", err))
	}

	// Reset memory manager
	app.memoryManager.Reset()

	// Reset metrics
	if app.metrics != nil {
		app.metrics.Reset()
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during close: %v", errors)
	}

	return nil
}

// Helper functions

// generateApplicationID creates a cryptographically secure application ID
func generateApplicationID() (string, error) {
	bytes := make([]byte, 16) // 128-bit ID
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// estimatePageMemoryUsage provides rough estimate of page memory requirements
func estimatePageMemoryUsage(tmpl *template.Template, data interface{}) int64 {
	// More realistic estimation for memory pressure testing
	var size int64 = 2048 // Base overhead for page structure (increased)

	// Template size estimation
	if tmpl != nil {
		size += int64(len(tmpl.Name()) * 4)        // Unicode estimation (increased)
		size += int64(len(tmpl.Templates()) * 500) // Template overhead (increased)
	}

	// Data size estimation (more realistic)
	dataSize := estimateDataSize(data)
	size += dataSize * 3 // Account for internal processing overhead

	// Fragment cache overhead (increased for realistic memory usage)
	size += 4096

	// If data is very large, add significant processing overhead
	if dataSize > 10240 { // 10KB
		size += dataSize / 2 // 50% overhead for large data
	}

	return size
}

// estimateDataSize provides rough estimate of data memory usage
func estimateDataSize(data interface{}) int64 {
	if data == nil {
		return 0
	}

	// Very rough estimation based on data type
	switch v := data.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case map[string]interface{}:
		size := int64(0)
		for key, value := range v {
			size += int64(len(key))
			size += estimateDataSize(value)
		}
		return size
	case []interface{}:
		size := int64(0)
		for _, value := range v {
			size += estimateDataSize(value)
		}
		return size
	default:
		return 100 // Default estimation for unknown types
	}
}
