package memory

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Manager provides memory management and resource limits
type Manager struct {
	maxMemoryBytes   int64
	currentUsage     int64
	pageMemoryUsage  map[string]int64 // pageID -> memory usage
	componentUsage   map[string]int64 // component -> memory usage
	memoryThresholds *Thresholds
	statistics       *Statistics
	callbacks        *PressureCallbacks
	cleanupTicker    *time.Ticker
	stopCleanup      chan struct{}
	mu               sync.RWMutex
	config           *Config
	started          bool
}

// Config defines memory manager configuration
type Config struct {
	MaxMemoryMB          int           // Maximum memory in MB
	WarningThresholdPct  int           // Warning threshold percentage
	CriticalThresholdPct int           // Critical threshold percentage
	CleanupInterval      time.Duration // How often to check memory usage
	EnableGCTuning       bool          // Enable garbage collection tuning
	LeakDetectionEnabled bool          // Enable memory leak detection
	ComponentTracking    bool          // Enable component-level tracking
}

// Thresholds defines memory usage thresholds
type Thresholds struct {
	WarningBytes  int64 // Warning threshold in bytes
	CriticalBytes int64 // Critical threshold in bytes
}

// Statistics tracks memory usage patterns over time
type Statistics struct {
	TotalAllocations    int64         // Total number of allocations
	TotalDeallocations  int64         // Total number of deallocations
	PeakUsage           int64         // Peak memory usage seen
	LeakDetectionCount  int64         // Number of potential leaks detected
	GCTriggerCount      int64         // Number of times GC was triggered
	PressureEvents      int64         // Number of pressure events
	LastPressureEvent   time.Time     // Last time pressure was detected
	AveragePageLifetime time.Duration // Average page lifetime
	StartTime           time.Time     // When tracking started
}

// PressureCallbacks defines callback functions for memory pressure events
type PressureCallbacks struct {
	OnWarning  func(status Status) // Called when warning threshold is reached
	OnCritical func(status Status) // Called when critical threshold is reached
	OnRecovery func(status Status) // Called when pressure subsides
}

// ComponentType defines different component types for memory tracking
type ComponentType string

const (
	ComponentPage     ComponentType = "page"
	ComponentTemplate ComponentType = "template"
	ComponentFragment ComponentType = "fragment"
	ComponentRegistry ComponentType = "registry"
	ComponentToken    ComponentType = "token"
	ComponentMetrics  ComponentType = "metrics"
)

// DefaultConfig returns secure default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxMemoryMB:          100, // 100MB default limit
		WarningThresholdPct:  75,  // 75% warning
		CriticalThresholdPct: 90,  // 90% critical
		CleanupInterval:      1 * time.Minute,
		EnableGCTuning:       true,
		LeakDetectionEnabled: true,
		ComponentTracking:    true,
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
		componentUsage:  make(map[string]int64),
		config:          config,
		memoryThresholds: &Thresholds{
			WarningBytes:  (maxBytes * int64(config.WarningThresholdPct)) / 100,
			CriticalBytes: (maxBytes * int64(config.CriticalThresholdPct)) / 100,
		},
		statistics: &Statistics{
			StartTime: time.Now(),
		},
		callbacks:   &PressureCallbacks{},
		stopCleanup: make(chan struct{}),
		started:     false,
	}

	// Start background monitoring if cleanup interval is set
	if config.CleanupInterval > 0 {
		manager.Start()
	}

	return manager
}

// Start begins background memory monitoring
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return
	}

	m.cleanupTicker = time.NewTicker(m.config.CleanupInterval)
	m.started = true

	go m.monitorMemoryPressure()
}

// Stop halts background memory monitoring
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
	close(m.stopCleanup)
	m.started = false
}

// monitorMemoryPressure runs background monitoring
func (m *Manager) monitorMemoryPressure() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.checkMemoryPressure()
			m.detectMemoryLeaks()
			m.updateStatistics()
		case <-m.stopCleanup:
			return
		}
	}
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

	// Update statistics
	atomic.AddInt64(&m.statistics.TotalAllocations, 1)

	// Update peak usage if necessary
	if newUsage > atomic.LoadInt64(&m.statistics.PeakUsage) {
		atomic.StoreInt64(&m.statistics.PeakUsage, newUsage)
	}

	// Track component usage
	if m.config.ComponentTracking {
		m.updateComponentUsage(string(ComponentPage), estimatedSize)
	}

	return nil
}

// DeallocatePage releases memory for a page
func (m *Manager) DeallocatePage(pageID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if usage, exists := m.pageMemoryUsage[pageID]; exists {
		atomic.AddInt64(&m.currentUsage, -usage)
		delete(m.pageMemoryUsage, pageID)

		// Update statistics
		atomic.AddInt64(&m.statistics.TotalDeallocations, 1)

		// Track component usage
		if m.config.ComponentTracking {
			m.updateComponentUsage(string(ComponentPage), -usage)
		}
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

// checkMemoryPressure monitors memory usage and triggers callbacks
func (m *Manager) checkMemoryPressure() {
	status := m.GetMemoryStatus()
	previousLevel := m.getPreviousMemoryLevel()

	// Trigger callbacks based on level changes
	switch status.Level {
	case "CRITICAL":
		if previousLevel != "CRITICAL" {
			atomic.AddInt64(&m.statistics.PressureEvents, 1)
			m.statistics.LastPressureEvent = time.Now()
			if m.callbacks.OnCritical != nil {
				go m.callbacks.OnCritical(status)
			}
			if m.config.EnableGCTuning {
				runtime.GC() // Force garbage collection
				atomic.AddInt64(&m.statistics.GCTriggerCount, 1)
			}
		}
	case "WARNING":
		if previousLevel == "OK" {
			if m.callbacks.OnWarning != nil {
				go m.callbacks.OnWarning(status)
			}
		}
	case "OK":
		if previousLevel == "WARNING" || previousLevel == "CRITICAL" {
			if m.callbacks.OnRecovery != nil {
				go m.callbacks.OnRecovery(status)
			}
		}
	}
}

// detectMemoryLeaks checks for potential memory leaks
func (m *Manager) detectMemoryLeaks() {
	if !m.config.LeakDetectionEnabled {
		return
	}

	m.mu.RLock()
	allocs := atomic.LoadInt64(&m.statistics.TotalAllocations)
	deallocs := atomic.LoadInt64(&m.statistics.TotalDeallocations)
	m.mu.RUnlock()

	// Simple leak detection: significant imbalance between allocs and deallocs
	if allocs > 0 && deallocs > 0 {
		ratio := float64(deallocs) / float64(allocs)
		if ratio < 0.8 { // Less than 80% of allocations have been deallocated
			atomic.AddInt64(&m.statistics.LeakDetectionCount, 1)
		}
	}
}

// updateStatistics updates memory usage statistics
func (m *Manager) updateStatistics() {
	// Update average page lifetime calculation
	m.mu.RLock()
	pageCount := len(m.pageMemoryUsage)
	m.mu.RUnlock()

	if pageCount > 0 {
		uptime := time.Since(m.statistics.StartTime)
		allocs := atomic.LoadInt64(&m.statistics.TotalAllocations)
		if allocs > 0 {
			m.statistics.AveragePageLifetime = time.Duration(int64(uptime) / allocs)
		}
	}
}

// getPreviousMemoryLevel returns the previous memory level for comparison
func (m *Manager) getPreviousMemoryLevel() string {
	// This would typically store the previous state
	// For simplicity, we'll recalculate based on current usage
	currentUsage := atomic.LoadInt64(&m.currentUsage)
	if currentUsage >= m.memoryThresholds.CriticalBytes {
		return "CRITICAL"
	} else if currentUsage >= m.memoryThresholds.WarningBytes {
		return "WARNING"
	}
	return "OK"
}

// updateComponentUsage updates memory usage for a specific component
func (m *Manager) updateComponentUsage(component string, delta int64) {
	current := m.componentUsage[component]
	m.componentUsage[component] = current + delta
	if m.componentUsage[component] < 0 {
		m.componentUsage[component] = 0
	}
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
		ComponentUsage:    make(map[string]int64),
		Statistics:        *m.statistics,
	}

	// Copy component usage
	if m.config.ComponentTracking {
		for component, usage := range m.componentUsage {
			status.ComponentUsage[component] = usage
		}
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
	CurrentUsage      int64            `json:"current_usage"`
	MaxMemory         int64            `json:"max_memory"`
	UsagePercentage   float64          `json:"usage_percentage"`
	Level             string           `json:"level"` // "OK", "WARNING", "CRITICAL"
	ActivePages       int              `json:"active_pages"`
	AveragePageMemory int64            `json:"average_page_memory"`
	WarningThreshold  int64            `json:"warning_threshold"`
	CriticalThreshold int64            `json:"critical_threshold"`
	ComponentUsage    map[string]int64 `json:"component_usage"`
	Statistics        Statistics       `json:"statistics"`
	GCStats           runtime.MemStats `json:"gc_stats,omitempty"`
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
	m.componentUsage = make(map[string]int64)

	// Reset statistics
	m.statistics = &Statistics{
		StartTime: time.Now(),
	}
}

// SetPressureCallbacks registers callback functions for memory pressure events
func (m *Manager) SetPressureCallbacks(callbacks *PressureCallbacks) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = callbacks
}

// AllocateComponent allocates memory for a specific component type
func (m *Manager) AllocateComponent(componentType ComponentType, componentID string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if allocation would exceed memory limit
	newUsage := atomic.LoadInt64(&m.currentUsage) + size
	if newUsage > m.maxMemoryBytes {
		return fmt.Errorf("component allocation would exceed limit: %d + %d > %d",
			atomic.LoadInt64(&m.currentUsage), size, m.maxMemoryBytes)
	}

	// Update usage tracking
	atomic.AddInt64(&m.currentUsage, size)
	atomic.AddInt64(&m.statistics.TotalAllocations, 1)

	// Track component usage
	if m.config.ComponentTracking {
		m.updateComponentUsage(string(componentType), size)
	}

	return nil
}

// DeallocateComponent releases memory for a specific component
func (m *Manager) DeallocateComponent(componentType ComponentType, componentID string, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.AddInt64(&m.currentUsage, -size)
	atomic.AddInt64(&m.statistics.TotalDeallocations, 1)

	// Track component usage
	if m.config.ComponentTracking {
		m.updateComponentUsage(string(componentType), -size)
	}
}

// GetDetailedStatus returns comprehensive memory status with GC info
func (m *Manager) GetDetailedStatus() Status {
	status := m.GetMemoryStatus()

	// Add GC statistics if requested
	var gcStats runtime.MemStats
	runtime.ReadMemStats(&gcStats)
	status.GCStats = gcStats

	return status
}

// ForceCleanup provides emergency memory cleanup interface
func (m *Manager) ForceCleanup() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return list of pages that should be cleaned up
	// Start with highest memory usage pages
	pages := m.GetTopMemoryPages(len(m.pageMemoryUsage))
	var candidatePages []string

	for _, page := range pages {
		candidatePages = append(candidatePages, page.PageID)
	}

	return candidatePages
}

// GetMemoryEfficiency calculates memory efficiency metrics
func (m *Manager) GetMemoryEfficiency() MemoryEfficiency {
	m.mu.RLock()
	defer m.mu.RUnlock()

	efficiency := MemoryEfficiency{
		MemoryUtilization: float64(atomic.LoadInt64(&m.currentUsage)) / float64(m.maxMemoryBytes) * 100,
		AllocationRate:    float64(atomic.LoadInt64(&m.statistics.TotalAllocations)),
		DeallocationRate:  float64(atomic.LoadInt64(&m.statistics.TotalDeallocations)),
		LeakPotential:     atomic.LoadInt64(&m.statistics.LeakDetectionCount),
	}

	// Calculate efficiency score (0-100)
	if efficiency.AllocationRate > 0 {
		efficiency.EfficiencyScore = (efficiency.DeallocationRate / efficiency.AllocationRate) * 100
		if efficiency.EfficiencyScore > 100 {
			efficiency.EfficiencyScore = 100
		}
	}

	return efficiency
}

// MemoryEfficiency contains memory efficiency metrics
type MemoryEfficiency struct {
	MemoryUtilization float64 `json:"memory_utilization"`
	AllocationRate    float64 `json:"allocation_rate"`
	DeallocationRate  float64 `json:"deallocation_rate"`
	LeakPotential     int64   `json:"leak_potential"`
	EfficiencyScore   float64 `json:"efficiency_score"`
}

// Close shuts down the memory manager
func (m *Manager) Close() error {
	if m.started {
		m.Stop()
	}
	m.Reset()
	return nil
}
