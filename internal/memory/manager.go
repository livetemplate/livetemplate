package memory

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Manager provides memory management and resource limits
type Manager struct {
	maxMemoryBytes   int64
	currentUsage     int64
	pageMemoryUsage  map[string]int64 // pageID -> memory usage
	memoryThresholds *Thresholds
	mu               sync.RWMutex
	config           *Config
}

// Config defines memory manager configuration
type Config struct {
	MaxMemoryMB          int           // Maximum memory in MB
	WarningThresholdPct  int           // Warning threshold percentage
	CriticalThresholdPct int           // Critical threshold percentage
	CleanupInterval      time.Duration // How often to check memory usage
}

// Thresholds defines memory usage thresholds
type Thresholds struct {
	WarningBytes  int64 // Warning threshold in bytes
	CriticalBytes int64 // Critical threshold in bytes
}

// DefaultConfig returns secure default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxMemoryMB:          100, // 100MB default limit
		WarningThresholdPct:  75,  // 75% warning
		CriticalThresholdPct: 90,  // 90% critical
		CleanupInterval:      1 * time.Minute,
	}
}

// NewManager creates a new memory manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	maxBytes := int64(config.MaxMemoryMB * 1024 * 1024)

	manager := &Manager{
		maxMemoryBytes:  maxBytes,
		currentUsage:    0,
		pageMemoryUsage: make(map[string]int64),
		config:          config,
		memoryThresholds: &Thresholds{
			WarningBytes:  (maxBytes * int64(config.WarningThresholdPct)) / 100,
			CriticalBytes: (maxBytes * int64(config.CriticalThresholdPct)) / 100,
		},
	}

	return manager
}

// AllocatePage attempts to allocate memory for a new page
func (m *Manager) AllocatePage(pageID string, estimatedSize int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if allocation would exceed memory limit
	newUsage := atomic.LoadInt64(&m.currentUsage) + estimatedSize
	if newUsage > m.maxMemoryBytes {
		return fmt.Errorf("memory allocation would exceed limit: %d + %d > %d",
			atomic.LoadInt64(&m.currentUsage), estimatedSize, m.maxMemoryBytes)
	}

	// Record page memory usage
	m.pageMemoryUsage[pageID] = estimatedSize
	atomic.AddInt64(&m.currentUsage, estimatedSize)

	return nil
}

// DeallocatePage releases memory for a page
func (m *Manager) DeallocatePage(pageID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if usage, exists := m.pageMemoryUsage[pageID]; exists {
		atomic.AddInt64(&m.currentUsage, -usage)
		delete(m.pageMemoryUsage, pageID)
	}
}

// UpdatePageUsage updates memory usage for an existing page
func (m *Manager) UpdatePageUsage(pageID string, newSize int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldSize, exists := m.pageMemoryUsage[pageID]
	if !exists {
		return fmt.Errorf("page not found: %s", pageID)
	}

	// Calculate change in memory usage
	deltaSize := newSize - oldSize
	newTotalUsage := atomic.LoadInt64(&m.currentUsage) + deltaSize

	// Check if update would exceed memory limit
	if newTotalUsage > m.maxMemoryBytes {
		return fmt.Errorf("memory update would exceed limit: %d + %d > %d",
			atomic.LoadInt64(&m.currentUsage), deltaSize, m.maxMemoryBytes)
	}

	// Update usage
	m.pageMemoryUsage[pageID] = newSize
	atomic.AddInt64(&m.currentUsage, deltaSize)

	return nil
}

// GetMemoryStatus returns current memory usage status
func (m *Manager) GetMemoryStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	currentUsage := atomic.LoadInt64(&m.currentUsage)

	status := Status{
		CurrentUsage:      currentUsage,
		MaxMemory:         m.maxMemoryBytes,
		UsagePercentage:   float64(currentUsage) / float64(m.maxMemoryBytes) * 100,
		ActivePages:       len(m.pageMemoryUsage),
		WarningThreshold:  m.memoryThresholds.WarningBytes,
		CriticalThreshold: m.memoryThresholds.CriticalBytes,
	}

	// Determine status level
	if currentUsage >= m.memoryThresholds.CriticalBytes {
		status.Level = "CRITICAL"
	} else if currentUsage >= m.memoryThresholds.WarningBytes {
		status.Level = "WARNING"
	} else {
		status.Level = "OK"
	}

	// Calculate average memory per page
	if len(m.pageMemoryUsage) > 0 {
		status.AveragePageMemory = currentUsage / int64(len(m.pageMemoryUsage))
	}

	return status
}

// Status contains memory usage information
type Status struct {
	CurrentUsage      int64   `json:"current_usage"`
	MaxMemory         int64   `json:"max_memory"`
	UsagePercentage   float64 `json:"usage_percentage"`
	Level             string  `json:"level"` // "OK", "WARNING", "CRITICAL"
	ActivePages       int     `json:"active_pages"`
	AveragePageMemory int64   `json:"average_page_memory"`
	WarningThreshold  int64   `json:"warning_threshold"`
	CriticalThreshold int64   `json:"critical_threshold"`
}

// IsAtCapacity checks if memory is at or near capacity
func (m *Manager) IsAtCapacity() bool {
	currentUsage := atomic.LoadInt64(&m.currentUsage)
	return currentUsage >= m.memoryThresholds.CriticalBytes
}

// IsNearCapacity checks if memory usage is approaching capacity
func (m *Manager) IsNearCapacity() bool {
	currentUsage := atomic.LoadInt64(&m.currentUsage)
	return currentUsage >= m.memoryThresholds.WarningBytes
}

// GetAvailableMemory returns available memory in bytes
func (m *Manager) GetAvailableMemory() int64 {
	currentUsage := atomic.LoadInt64(&m.currentUsage)
	available := m.maxMemoryBytes - currentUsage
	if available < 0 {
		return 0
	}
	return available
}

// CanAllocate checks if a given size can be allocated
func (m *Manager) CanAllocate(size int64) bool {
	currentUsage := atomic.LoadInt64(&m.currentUsage)
	return currentUsage+size <= m.maxMemoryBytes
}

// GetPageMemoryUsage returns memory usage for a specific page
func (m *Manager) GetPageMemoryUsage(pageID string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usage, exists := m.pageMemoryUsage[pageID]
	return usage, exists
}

// GetTopMemoryPages returns pages using the most memory
func (m *Manager) GetTopMemoryPages(limit int) []PageMemoryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Convert to slice for sorting
	pages := make([]PageMemoryInfo, 0, len(m.pageMemoryUsage))
	for pageID, usage := range m.pageMemoryUsage {
		pages = append(pages, PageMemoryInfo{
			PageID: pageID,
			Usage:  usage,
		})
	}

	// Simple sort by usage (descending)
	for i := 0; i < len(pages)-1; i++ {
		for j := 0; j < len(pages)-i-1; j++ {
			if pages[j].Usage < pages[j+1].Usage {
				pages[j], pages[j+1] = pages[j+1], pages[j]
			}
		}
	}

	// Return top N pages
	if limit > len(pages) {
		limit = len(pages)
	}
	return pages[:limit]
}

// PageMemoryInfo contains memory usage information for a page
type PageMemoryInfo struct {
	PageID string `json:"page_id"`
	Usage  int64  `json:"usage"`
}

// GetTotalPages returns the number of pages being tracked
func (m *Manager) GetTotalPages() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pageMemoryUsage)
}

// Reset clears all memory tracking
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.StoreInt64(&m.currentUsage, 0)
	m.pageMemoryUsage = make(map[string]int64)
}

// ForceCleanup provides emergency memory cleanup interface
func (m *Manager) ForceCleanup() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return list of pages that should be cleaned up
	// In production, this would integrate with the page registry
	var candidatePages []string

	// Start with highest memory usage pages
	for pageID := range m.pageMemoryUsage {
		candidatePages = append(candidatePages, pageID)
	}

	return candidatePages
}
