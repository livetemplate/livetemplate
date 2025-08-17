package page

import (
	"fmt"
	"sync"
	"time"
)

// Registry provides thread-safe storage for Page instances with TTL cleanup
type Registry struct {
	pages         map[string]*Page
	pagesByApp    map[string]map[string]*Page // applicationID -> pageID -> Page
	mu            sync.RWMutex
	maxPages      int
	defaultTTL    time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// RegistryConfig defines PageRegistry configuration
type RegistryConfig struct {
	MaxPages        int           // Maximum pages to store
	DefaultTTL      time.Duration // Default TTL for pages
	CleanupInterval time.Duration // How often to run cleanup
}

// DefaultRegistryConfig returns secure default configuration
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxPages:        1000,
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 5 * time.Minute,
	}
}

// NewRegistry creates a new PageRegistry with automatic cleanup
func NewRegistry(config *RegistryConfig) *Registry {
	if config == nil {
		config = DefaultRegistryConfig()
	}

	registry := &Registry{
		pages:       make(map[string]*Page),
		pagesByApp:  make(map[string]map[string]*Page),
		maxPages:    config.MaxPages,
		defaultTTL:  config.DefaultTTL,
		stopCleanup: make(chan struct{}),
	}

	// Start automatic cleanup - initialize ticker first, then start goroutine
	registry.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go registry.runCleanup()

	return registry
}

// Store adds a page to the registry
func (r *Registry) Store(page *Page) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check capacity
	if len(r.pages) >= r.maxPages {
		return fmt.Errorf("registry at capacity (%d pages)", r.maxPages)
	}

	// Store in main index
	r.pages[page.ID] = page

	// Store in application index
	if r.pagesByApp[page.ApplicationID] == nil {
		r.pagesByApp[page.ApplicationID] = make(map[string]*Page)
	}
	r.pagesByApp[page.ApplicationID][page.ID] = page

	return nil
}

// Get retrieves a page by ID with application boundary check
func (r *Registry) Get(pageID, applicationID string) (*Page, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	page, exists := r.pages[pageID]
	if !exists {
		return nil, fmt.Errorf("page not found: %s", pageID)
	}

	// Enforce application isolation
	if page.ApplicationID != applicationID {
		return nil, fmt.Errorf("cross-application access denied")
	}

	// Check if page is expired
	if page.IsExpired(r.defaultTTL) {
		return nil, fmt.Errorf("page expired: %s", pageID)
	}

	// Update last accessed time
	page.UpdateLastAccessed()

	return page, nil
}

// Remove deletes a page from the registry
func (r *Registry) Remove(pageID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	page, exists := r.pages[pageID]
	if !exists {
		return false
	}

	// Remove from main index
	delete(r.pages, pageID)

	// Remove from application index
	if appPages, exists := r.pagesByApp[page.ApplicationID]; exists {
		delete(appPages, pageID)

		// Clean up empty application index
		if len(appPages) == 0 {
			delete(r.pagesByApp, page.ApplicationID)
		}
	}

	return true
}

// GetByApplication returns all pages for an application
func (r *Registry) GetByApplication(applicationID string) map[string]*Page {
	r.mu.RLock()
	defer r.mu.RUnlock()

	appPages, exists := r.pagesByApp[applicationID]
	if !exists {
		return make(map[string]*Page)
	}

	// Return copy to prevent external modification
	result := make(map[string]*Page)
	for id, page := range appPages {
		if !page.IsExpired(r.defaultTTL) {
			result[id] = page
		}
	}

	return result
}

// GetPageCount returns the total number of pages
func (r *Registry) GetPageCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pages)
}

// GetApplicationCount returns the number of applications with pages
func (r *Registry) GetApplicationCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pagesByApp)
}

// CleanupExpired removes expired pages and returns count
func (r *Registry) CleanupExpired() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0

	// Find expired pages
	var expiredPageIDs []string
	for pageID, page := range r.pages {
		if page.IsExpired(r.defaultTTL) {
			expiredPageIDs = append(expiredPageIDs, pageID)
		}
	}

	// Remove expired pages
	for _, pageID := range expiredPageIDs {
		page := r.pages[pageID]

		// Remove from main index
		delete(r.pages, pageID)

		// Remove from application index
		if appPages, exists := r.pagesByApp[page.ApplicationID]; exists {
			delete(appPages, pageID)

			// Clean up empty application index
			if len(appPages) == 0 {
				delete(r.pagesByApp, page.ApplicationID)
			}
		}

		count++
	}

	return count
}

// GetMetrics returns registry metrics
func (r *Registry) GetMetrics() RegistryMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := RegistryMetrics{
		TotalPages:   len(r.pages),
		Applications: len(r.pagesByApp),
		MaxCapacity:  r.maxPages,
		CapacityUsed: float64(len(r.pages)) / float64(r.maxPages),
		DefaultTTL:   r.defaultTTL,
	}

	// Calculate pages per application
	if len(r.pagesByApp) > 0 {
		total := 0
		for _, appPages := range r.pagesByApp {
			total += len(appPages)
		}
		metrics.AvgPagesPerApp = float64(total) / float64(len(r.pagesByApp))
	}

	return metrics
}

// RegistryMetrics contains registry performance data
type RegistryMetrics struct {
	TotalPages     int           `json:"total_pages"`
	Applications   int           `json:"applications"`
	MaxCapacity    int           `json:"max_capacity"`
	CapacityUsed   float64       `json:"capacity_used"`
	AvgPagesPerApp float64       `json:"avg_pages_per_app"`
	DefaultTTL     time.Duration `json:"default_ttl"`
}

// Close stops the cleanup goroutine and releases resources
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cleanupTicker != nil {
		r.cleanupTicker.Stop()
		r.cleanupTicker = nil

		// Close channel if not already closed
		select {
		case <-r.stopCleanup:
			// Channel already closed
		default:
			close(r.stopCleanup)
		}
	}

	// Clear all pages
	r.pages = make(map[string]*Page)
	r.pagesByApp = make(map[string]map[string]*Page)

	return nil
}

// runCleanup performs periodic cleanup in background
func (r *Registry) runCleanup() {
	for {
		// Check if ticker is initialized to prevent nil pointer dereference
		if r.cleanupTicker == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		select {
		case <-r.cleanupTicker.C:
			r.CleanupExpired()
		case <-r.stopCleanup:
			return
		}
	}
}
