package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Collector provides simple built-in metrics collection with no external dependencies
type Collector struct {
	applicationMetrics *ApplicationMetrics
	operationCounters  map[string]*int64
	mu                 sync.RWMutex
	startTime          time.Time
}

// ApplicationMetrics tracks application-level performance data
type ApplicationMetrics struct {
	// Page management
	PagesCreated       int64 `json:"pages_created"`
	PagesDestroyed     int64 `json:"pages_destroyed"`
	ActivePages        int64 `json:"active_pages"`
	MaxConcurrentPages int64 `json:"max_concurrent_pages"`

	// Token operations
	TokensGenerated int64 `json:"tokens_generated"`
	TokensVerified  int64 `json:"tokens_verified"`
	TokenFailures   int64 `json:"token_failures"`

	// Fragment generation
	FragmentsGenerated int64 `json:"fragments_generated"`
	GenerationErrors   int64 `json:"generation_errors"`

	// Memory and performance
	TotalMemoryUsage  int64 `json:"total_memory_usage"`
	AveragePageMemory int64 `json:"average_page_memory"`

	// Cleanup operations
	CleanupOperations   int64 `json:"cleanup_operations"`
	ExpiredPagesRemoved int64 `json:"expired_pages_removed"`

	// Uptime
	StartTime time.Time     `json:"start_time"`
	Uptime    time.Duration `json:"uptime"`
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		applicationMetrics: &ApplicationMetrics{
			StartTime: time.Now(),
		},
		operationCounters: make(map[string]*int64),
		startTime:         time.Now(),
	}
}

// IncrementPageCreated records a new page creation
func (c *Collector) IncrementPageCreated() {
	atomic.AddInt64(&c.applicationMetrics.PagesCreated, 1)
	currentActive := atomic.AddInt64(&c.applicationMetrics.ActivePages, 1)

	// Update max concurrent if needed
	for {
		max := atomic.LoadInt64(&c.applicationMetrics.MaxConcurrentPages)
		if currentActive <= max {
			break
		}
		if atomic.CompareAndSwapInt64(&c.applicationMetrics.MaxConcurrentPages, max, currentActive) {
			break
		}
	}
}

// IncrementPageDestroyed records a page destruction
func (c *Collector) IncrementPageDestroyed() {
	atomic.AddInt64(&c.applicationMetrics.PagesDestroyed, 1)
	atomic.AddInt64(&c.applicationMetrics.ActivePages, -1)
}

// IncrementTokenGenerated records a token generation
func (c *Collector) IncrementTokenGenerated() {
	atomic.AddInt64(&c.applicationMetrics.TokensGenerated, 1)
}

// IncrementTokenVerified records a successful token verification
func (c *Collector) IncrementTokenVerified() {
	atomic.AddInt64(&c.applicationMetrics.TokensVerified, 1)
}

// IncrementTokenFailure records a token verification failure
func (c *Collector) IncrementTokenFailure() {
	atomic.AddInt64(&c.applicationMetrics.TokenFailures, 1)
}

// IncrementFragmentGenerated records a fragment generation
func (c *Collector) IncrementFragmentGenerated() {
	atomic.AddInt64(&c.applicationMetrics.FragmentsGenerated, 1)
}

// IncrementGenerationError records a fragment generation error
func (c *Collector) IncrementGenerationError() {
	atomic.AddInt64(&c.applicationMetrics.GenerationErrors, 1)
}

// UpdateMemoryUsage updates memory usage metrics
func (c *Collector) UpdateMemoryUsage(totalMemory, averagePageMemory int64) {
	atomic.StoreInt64(&c.applicationMetrics.TotalMemoryUsage, totalMemory)
	atomic.StoreInt64(&c.applicationMetrics.AveragePageMemory, averagePageMemory)
}

// IncrementCleanupOperation records a cleanup operation
func (c *Collector) IncrementCleanupOperation(expiredPagesRemoved int64) {
	atomic.AddInt64(&c.applicationMetrics.CleanupOperations, 1)
	atomic.AddInt64(&c.applicationMetrics.ExpiredPagesRemoved, expiredPagesRemoved)
}

// IncrementCustomCounter increments a custom named counter
func (c *Collector) IncrementCustomCounter(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if counter, exists := c.operationCounters[name]; exists {
		atomic.AddInt64(counter, 1)
	} else {
		var newCounter int64 = 1
		c.operationCounters[name] = &newCounter
	}
}

// GetMetrics returns current application metrics
func (c *Collector) GetMetrics() ApplicationMetrics {
	// Update uptime
	c.applicationMetrics.Uptime = time.Since(c.startTime)

	// Return a copy with current atomic values
	return ApplicationMetrics{
		PagesCreated:        atomic.LoadInt64(&c.applicationMetrics.PagesCreated),
		PagesDestroyed:      atomic.LoadInt64(&c.applicationMetrics.PagesDestroyed),
		ActivePages:         atomic.LoadInt64(&c.applicationMetrics.ActivePages),
		MaxConcurrentPages:  atomic.LoadInt64(&c.applicationMetrics.MaxConcurrentPages),
		TokensGenerated:     atomic.LoadInt64(&c.applicationMetrics.TokensGenerated),
		TokensVerified:      atomic.LoadInt64(&c.applicationMetrics.TokensVerified),
		TokenFailures:       atomic.LoadInt64(&c.applicationMetrics.TokenFailures),
		FragmentsGenerated:  atomic.LoadInt64(&c.applicationMetrics.FragmentsGenerated),
		GenerationErrors:    atomic.LoadInt64(&c.applicationMetrics.GenerationErrors),
		TotalMemoryUsage:    atomic.LoadInt64(&c.applicationMetrics.TotalMemoryUsage),
		AveragePageMemory:   atomic.LoadInt64(&c.applicationMetrics.AveragePageMemory),
		CleanupOperations:   atomic.LoadInt64(&c.applicationMetrics.CleanupOperations),
		ExpiredPagesRemoved: atomic.LoadInt64(&c.applicationMetrics.ExpiredPagesRemoved),
		StartTime:           c.applicationMetrics.StartTime,
		Uptime:              time.Since(c.startTime),
	}
}

// GetCustomCounters returns all custom counters
func (c *Collector) GetCustomCounters() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]int64)
	for name, counter := range c.operationCounters {
		result[name] = atomic.LoadInt64(counter)
	}
	return result
}

// Reset resets all metrics to zero
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset application metrics
	atomic.StoreInt64(&c.applicationMetrics.PagesCreated, 0)
	atomic.StoreInt64(&c.applicationMetrics.PagesDestroyed, 0)
	atomic.StoreInt64(&c.applicationMetrics.ActivePages, 0)
	atomic.StoreInt64(&c.applicationMetrics.MaxConcurrentPages, 0)
	atomic.StoreInt64(&c.applicationMetrics.TokensGenerated, 0)
	atomic.StoreInt64(&c.applicationMetrics.TokensVerified, 0)
	atomic.StoreInt64(&c.applicationMetrics.TokenFailures, 0)
	atomic.StoreInt64(&c.applicationMetrics.FragmentsGenerated, 0)
	atomic.StoreInt64(&c.applicationMetrics.GenerationErrors, 0)
	atomic.StoreInt64(&c.applicationMetrics.TotalMemoryUsage, 0)
	atomic.StoreInt64(&c.applicationMetrics.AveragePageMemory, 0)
	atomic.StoreInt64(&c.applicationMetrics.CleanupOperations, 0)
	atomic.StoreInt64(&c.applicationMetrics.ExpiredPagesRemoved, 0)

	// Reset custom counters
	c.operationCounters = make(map[string]*int64)

	// Reset start time
	c.startTime = time.Now()
	c.applicationMetrics.StartTime = time.Now()
}

// GetErrorRate returns the error rate for fragment generation
func (c *Collector) GetErrorRate() float64 {
	generated := atomic.LoadInt64(&c.applicationMetrics.FragmentsGenerated)
	errors := atomic.LoadInt64(&c.applicationMetrics.GenerationErrors)

	if generated == 0 {
		return 0.0
	}

	return float64(errors) / float64(generated+errors) * 100.0
}

// GetTokenSuccessRate returns the success rate for token operations
func (c *Collector) GetTokenSuccessRate() float64 {
	verified := atomic.LoadInt64(&c.applicationMetrics.TokensVerified)
	failures := atomic.LoadInt64(&c.applicationMetrics.TokenFailures)

	total := verified + failures
	if total == 0 {
		return 100.0 // No operations means 100% success rate
	}

	return float64(verified) / float64(total) * 100.0
}

// GetMemoryEfficiency returns memory usage per active page
func (c *Collector) GetMemoryEfficiency() float64 {
	totalMemory := atomic.LoadInt64(&c.applicationMetrics.TotalMemoryUsage)
	activePages := atomic.LoadInt64(&c.applicationMetrics.ActivePages)

	if activePages == 0 {
		return 0.0
	}

	return float64(totalMemory) / float64(activePages)
}
