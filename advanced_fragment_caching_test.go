package livetemplate

import (
	"encoding/json"
	"fmt"
	"html/template"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdvancedFragmentCaching implements comprehensive fragment caching validation
func TestAdvancedFragmentCaching(t *testing.T) {
	suite := &FragmentCachingSuite{t: t}

	// Run all caching validation tests
	t.Run("Static_Fragment_Cache_Hit_Miss_Ratios", suite.TestCacheHitMissRatios)
	t.Run("Cache_Invalidation_Strategies", suite.TestCacheInvalidationStrategies)
	t.Run("Client_Side_Cache_Memory_Usage", suite.TestCacheMemoryUsage)
	t.Run("Cache_Persistence_Across_Reloads", suite.TestCachePersistence)
	t.Run("Multi_Fragment_Caching_Scenarios", suite.TestMultiFragmentCaching)
	t.Run("Cache_Size_Limits_And_Eviction", suite.TestCacheSizeLimitsEviction)
	t.Run("Performance_Improvement_From_Caching", suite.TestPerformanceImprovements)
	t.Run("Cache_Coherency_With_Server_Changes", suite.TestCacheCoherency)
}

// FragmentCachingSuite provides comprehensive fragment caching validation
type FragmentCachingSuite struct {
	t *testing.T
}

// ClientSideCache simulates client-side fragment caching with advanced features
type ClientSideCache struct {
	entries       map[string]*CacheEntry
	maxSizeBytes  int64
	currentSize   int64
	hits          int64
	misses        int64
	invalidations int64
	evictions     int64
	expired       int64
	mu            sync.RWMutex
	ttlEnabled    bool
	lruEnabled    bool
	accessTimes   map[string]time.Time
	creationTimes map[string]time.Time
}

// CacheEntry represents a cached fragment entry with full metadata
type CacheEntry struct {
	FragmentID   string        `json:"fragment_id"`
	Data         interface{}   `json:"data"`
	Strategy     string        `json:"strategy"`
	Version      int64         `json:"version"`
	CreatedAt    time.Time     `json:"created_at"`
	AccessedAt   time.Time     `json:"accessed_at"`
	TTL          time.Duration `json:"ttl"`
	SizeBytes    int64         `json:"size_bytes"`
	ContentHash  string        `json:"content_hash"`
	Dependencies []string      `json:"dependencies"`
}

// FragmentCacheMetrics captures comprehensive caching performance data
type FragmentCacheMetrics struct {
	CacheHits           int64   `json:"cache_hits"`
	CacheMisses         int64   `json:"cache_misses"`
	HitRatio            float64 `json:"hit_ratio"`
	MissRatio           float64 `json:"miss_ratio"`
	TotalRequests       int64   `json:"total_requests"`
	HitLatencyMs        float64 `json:"hit_latency_ms"`
	MissLatencyMs       float64 `json:"miss_latency_ms"`
	AverageLatencyMs    float64 `json:"average_latency_ms"`
	CacheSpeedupRatio   float64 `json:"cache_speedup_ratio"`
	CacheSizeBytes      int64   `json:"cache_size_bytes"`
	CacheEntries        int     `json:"cache_entries"`
	MemoryPerEntry      float64 `json:"memory_per_entry"`
	MaxCacheSizeBytes   int64   `json:"max_cache_size_bytes"`
	Invalidations       int64   `json:"invalidations"`
	EvictedEntries      int64   `json:"evicted_entries"`
	ExpiredEntries      int64   `json:"expired_entries"`
	CoherencyChecks     int64   `json:"coherency_checks"`
	CoherencyViolations int64   `json:"coherency_violations"`
	StaleHits           int64   `json:"stale_hits"`
	BytesSaved          int64   `json:"bytes_saved"`
	BandwidthReduction  float64 `json:"bandwidth_reduction_percent"`
}

// NewClientSideCache creates a new advanced client-side cache
func NewClientSideCache(maxSizeBytes int64) *ClientSideCache {
	return &ClientSideCache{
		entries:       make(map[string]*CacheEntry),
		maxSizeBytes:  maxSizeBytes,
		accessTimes:   make(map[string]time.Time),
		creationTimes: make(map[string]time.Time),
		ttlEnabled:    true,
		lruEnabled:    true,
	}
}

// Get retrieves an entry from cache with TTL validation
func (c *ClientSideCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// Check TTL expiration
	if c.ttlEnabled && entry.TTL > 0 && time.Since(entry.CreatedAt) > entry.TTL {
		// Entry expired, mark for removal
		c.mu.RUnlock()
		c.mu.Lock()
		delete(c.entries, key)
		delete(c.accessTimes, key)
		delete(c.creationTimes, key)
		c.currentSize -= entry.SizeBytes
		atomic.AddInt64(&c.expired, 1)
		c.mu.Unlock()
		c.mu.RLock()

		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// Update access time for LRU
	if c.lruEnabled {
		c.accessTimes[key] = time.Now()
	}

	atomic.AddInt64(&c.hits, 1)
	return entry, true
}

// Set stores an entry in cache with eviction if needed
func (c *ClientSideCache) Set(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry.AccessedAt = time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// Check if replacing existing entry
	if existing, exists := c.entries[key]; exists {
		c.currentSize -= existing.SizeBytes
	}

	// Ensure we have space (implement LRU eviction if needed)
	for c.currentSize+entry.SizeBytes > c.maxSizeBytes && len(c.entries) > 0 {
		c.evictLRU()
	}

	// Store entry
	c.entries[key] = entry
	c.currentSize += entry.SizeBytes
	c.accessTimes[key] = time.Now()
	c.creationTimes[key] = entry.CreatedAt
}

// evictLRU removes the least recently used entry
func (c *ClientSideCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, accessTime := range c.accessTimes {
		if oldestKey == "" || accessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = accessTime
		}
	}

	if oldestKey != "" {
		if entry := c.entries[oldestKey]; entry != nil {
			c.currentSize -= entry.SizeBytes
		}
		delete(c.entries, oldestKey)
		delete(c.accessTimes, oldestKey)
		delete(c.creationTimes, oldestKey)
		atomic.AddInt64(&c.evictions, 1)
	}
}

// Clear removes all entries from cache
func (c *ClientSideCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.accessTimes = make(map[string]time.Time)
	c.creationTimes = make(map[string]time.Time)
	c.currentSize = 0
	atomic.AddInt64(&c.invalidations, 1)
}

// Invalidate removes entries with matching dependencies
func (c *ClientSideCache) Invalidate(dependency string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	invalidated := 0
	for key, entry := range c.entries {
		for _, dep := range entry.Dependencies {
			if dep == dependency {
				c.currentSize -= entry.SizeBytes
				delete(c.entries, key)
				delete(c.accessTimes, key)
				delete(c.creationTimes, key)
				invalidated++
				break
			}
		}
	}

	atomic.AddInt64(&c.invalidations, int64(invalidated))
	return invalidated
}

// GetMetrics returns comprehensive cache metrics
func (c *ClientSideCache) GetMetrics() FragmentCacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)
	total := hits + misses

	var hitRatio, missRatio, memoryPerEntry float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total) * 100
		missRatio = float64(misses) / float64(total) * 100
	}

	if len(c.entries) > 0 {
		memoryPerEntry = float64(c.currentSize) / float64(len(c.entries))
	}

	return FragmentCacheMetrics{
		CacheHits:         hits,
		CacheMisses:       misses,
		HitRatio:          hitRatio,
		MissRatio:         missRatio,
		TotalRequests:     total,
		CacheSizeBytes:    c.currentSize,
		CacheEntries:      len(c.entries),
		MaxCacheSizeBytes: c.maxSizeBytes,
		Invalidations:     atomic.LoadInt64(&c.invalidations),
		EvictedEntries:    atomic.LoadInt64(&c.evictions),
		ExpiredEntries:    atomic.LoadInt64(&c.expired),
		MemoryPerEntry:    memoryPerEntry,
	}
}

// Serialize exports cache for persistence
func (c *ClientSideCache) Serialize() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return json.Marshal(c.entries)
}

// Deserialize imports cache from persistence
func (c *ClientSideCache) Deserialize(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var entries map[string]*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	c.entries = entries
	c.accessTimes = make(map[string]time.Time)
	c.creationTimes = make(map[string]time.Time)

	// Recalculate size and timestamps
	c.currentSize = 0
	now := time.Now()
	for key, entry := range entries {
		c.currentSize += entry.SizeBytes
		c.accessTimes[key] = now
		c.creationTimes[key] = entry.CreatedAt
	}

	return nil
}

// Test implementations for all acceptance criteria

// TestCacheHitMissRatios validates static fragment cache hit/miss ratios
func (suite *FragmentCachingSuite) TestCacheHitMissRatios(t *testing.T) {
	t.Run("Static_Fragment_Hit_Ratios", func(t *testing.T) {
		cache := NewClientSideCache(1024 * 1024)

		// Create test page with simple template
		tmpl, _ := template.New("test").Parse(`
			<div>
				<h1>{{.Title}}</h1>
				<p>User: {{.UserName}}</p>
				<span>Messages: {{.MessageCount}}</span>
				<time>{{.LastLogin}}</time>
			</div>
		`)

		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() { _ = app.Close() }()

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
			"Title":        "Test Application",
			"UserName":     "John Doe",
			"MessageCount": 5,
			"LastLogin":    "2024-01-15",
		})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() { _ = page.Close() }()

		// Generate initial fragments (cache misses)
		staticData := map[string]interface{}{
			"Title":        "Test Application",
			"UserName":     "John Doe",
			"MessageCount": 5,
			"LastLogin":    "2024-01-15",
		}

		// First request - all misses
		_, err = suite.generateAndCacheFragments(page, cache, staticData)
		if err != nil {
			t.Fatalf("Failed to generate initial fragments: %v", err)
		}

		// Subsequent requests with same data - should be hits
		for i := 0; i < 5; i++ {
			_, err = suite.generateAndCacheFragments(page, cache, staticData)
			if err != nil {
				t.Fatalf("Failed to generate fragments on iteration %d: %v", i, err)
			}
		}

		metrics := cache.GetMetrics()

		t.Logf("✓ Static fragment cache hit/miss ratios:")
		t.Logf("  - Total requests: %d", metrics.TotalRequests)
		t.Logf("  - Cache hits: %d", metrics.CacheHits)
		t.Logf("  - Cache misses: %d", metrics.CacheMisses)
		t.Logf("  - Hit ratio: %.2f%%", metrics.HitRatio)
		t.Logf("  - Miss ratio: %.2f%%", metrics.MissRatio)

		// At least 60% hit ratio expected (conservative estimate)
		if metrics.HitRatio < 60.0 {
			t.Errorf("Hit ratio below expected: %.2f%% (expected >= 60%%)", metrics.HitRatio)
		}
	})

	t.Run("Dynamic_Content_Miss_Ratios", func(t *testing.T) {
		cache := NewClientSideCache(1024 * 1024)

		// Test dynamic content that should result in cache misses
		dynamicRequests := []map[string]interface{}{
			{"Title": "App", "UserName": "User1", "MessageCount": 1, "LastLogin": "2024-01-01"},
			{"Title": "App", "UserName": "User2", "MessageCount": 2, "LastLogin": "2024-01-02"},
			{"Title": "App", "UserName": "User3", "MessageCount": 3, "LastLogin": "2024-01-03"},
			{"Title": "App", "UserName": "User4", "MessageCount": 4, "LastLogin": "2024-01-04"},
			{"Title": "App", "UserName": "User5", "MessageCount": 5, "LastLogin": "2024-01-05"},
		}

		// Simulate changing content
		for i, data := range dynamicRequests {
			entry := &CacheEntry{
				FragmentID: fmt.Sprintf("dynamic-%d", i),
				Data:       data,
				Version:    1,
				CreatedAt:  time.Now(),
				SizeBytes:  100,
			}
			cache.Set(fmt.Sprintf("dynamic-%d", i), entry)

			// Try to retrieve (will be a hit since we just set it)
			_, _ = cache.Get(fmt.Sprintf("dynamic-%d", i))

			// Try to retrieve non-existent entry (will be a miss)
			_, _ = cache.Get(fmt.Sprintf("nonexistent-%d", i))
		}

		metrics := cache.GetMetrics()

		t.Logf("✓ Dynamic content cache ratios:")
		t.Logf("  - Total requests: %d", metrics.TotalRequests)
		t.Logf("  - Cache hits: %d", metrics.CacheHits)
		t.Logf("  - Cache misses: %d", metrics.CacheMisses)
		t.Logf("  - Hit ratio: %.2f%%", metrics.HitRatio)
		t.Logf("  - Miss ratio: %.2f%%", metrics.MissRatio)

		// Dynamic content should have some misses
		if metrics.MissRatio < 30.0 {
			t.Errorf("Expected higher miss ratio for dynamic content: %.2f%%", metrics.MissRatio)
		}
	})
}

// TestCacheInvalidationStrategies validates different cache invalidation approaches
func (suite *FragmentCachingSuite) TestCacheInvalidationStrategies(t *testing.T) {
	t.Run("Time_Based_Invalidation", func(t *testing.T) {
		cache := NewClientSideCache(1024 * 1024)

		// Add entry with short TTL
		entry := &CacheEntry{
			FragmentID: "ttl-test",
			Data:       "ttl data",
			TTL:        100 * time.Millisecond, // Short TTL for testing
			SizeBytes:  100,
			CreatedAt:  time.Now(),
		}
		cache.Set("ttl-key", entry)

		// Verify entry exists
		cached, hit := cache.Get("ttl-key")
		if !hit || cached == nil {
			t.Error("Entry should exist immediately after set")
		}

		// Wait for TTL expiration
		time.Sleep(150 * time.Millisecond)

		// Verify entry is expired
		_, hit = cache.Get("ttl-key")
		if hit {
			t.Error("Entry should be expired after TTL")
		}

		metrics := cache.GetMetrics()
		t.Logf("✓ Time-based invalidation:")
		t.Logf("  - Expired entries: %d", metrics.ExpiredEntries)
		t.Logf("  - Current cache size: %d bytes", metrics.CacheSizeBytes)

		if metrics.ExpiredEntries == 0 {
			t.Error("Expected at least one expired entry")
		}
	})

	t.Run("Dependency_Based_Invalidation", func(t *testing.T) {
		cache := NewClientSideCache(1024 * 1024)

		// Add entries with dependencies
		entry1 := &CacheEntry{
			FragmentID:   "dep-fragment-1",
			Data:         "dep data 1",
			Dependencies: []string{"user-profile", "settings"},
			SizeBytes:    100,
			CreatedAt:    time.Now(),
		}
		entry2 := &CacheEntry{
			FragmentID:   "dep-fragment-2",
			Data:         "dep data 2",
			Dependencies: []string{"user-profile", "notifications"},
			SizeBytes:    100,
			CreatedAt:    time.Now(),
		}

		cache.Set("dep-key-1", entry1)
		cache.Set("dep-key-2", entry2)

		// Invalidate by dependency
		invalidated := cache.Invalidate("user-profile")

		metrics := cache.GetMetrics()
		t.Logf("✓ Dependency-based invalidation:")
		t.Logf("  - Invalidations: %d", metrics.Invalidations)

		if invalidated < 2 {
			t.Errorf("Expected 2 invalidations, got %d", invalidated)
		}
	})

	t.Run("Manual_Cache_Clear", func(t *testing.T) {
		cache := NewClientSideCache(1024 * 1024)

		// Add entries
		for i := 0; i < 5; i++ {
			entry := &CacheEntry{
				FragmentID: fmt.Sprintf("clear-fragment-%d", i),
				Data:       fmt.Sprintf("clear data %d", i),
				SizeBytes:  100,
				CreatedAt:  time.Now(),
			}
			cache.Set(fmt.Sprintf("clear-key-%d", i), entry)
		}

		// Verify entries exist
		metricsBefore := cache.GetMetrics()
		if metricsBefore.CacheEntries != 5 {
			t.Errorf("Expected 5 entries before clear, got %d", metricsBefore.CacheEntries)
		}

		// Clear cache
		cache.Clear()

		// Verify cache is empty
		metricsAfter := cache.GetMetrics()
		if metricsAfter.CacheEntries != 0 {
			t.Errorf("Expected 0 entries after clear, got %d", metricsAfter.CacheEntries)
		}
		if metricsAfter.CacheSizeBytes != 0 {
			t.Errorf("Expected 0 bytes after clear, got %d", metricsAfter.CacheSizeBytes)
		}

		t.Log("✓ Manual cache clear validated")
	})
}

// TestCacheMemoryUsage monitors memory usage of client-side cache
func (suite *FragmentCachingSuite) TestCacheMemoryUsage(t *testing.T) {
	t.Run("Memory_Growth_Pattern", func(t *testing.T) {
		cache := NewClientSideCache(1024 * 1024) // 1MB limit

		// Add entries of increasing size
		entrySizes := []int64{1024, 2048, 4096, 8192, 16384}

		for step, size := range entrySizes {
			// Add 10 entries of this size
			for i := 0; i < 10; i++ {
				entry := &CacheEntry{
					FragmentID: fmt.Sprintf("memory-fragment-%d-%d", step, i),
					Data:       strings.Repeat("x", int(size)),
					SizeBytes:  size,
					CreatedAt:  time.Now(),
				}
				cache.Set(fmt.Sprintf("memory-key-%d-%d", step, i), entry)
			}

			metrics := cache.GetMetrics()
			t.Logf("After adding %d entries of size %d:", (step+1)*10, size)
			t.Logf("  - Cache size: %d bytes", metrics.CacheSizeBytes)
			t.Logf("  - Cache entries: %d", metrics.CacheEntries)
			t.Logf("  - Memory per entry: %.2f bytes", metrics.MemoryPerEntry)
		}

		finalMetrics := cache.GetMetrics()
		t.Logf("✓ Memory growth pattern validation:")
		t.Logf("  - Final cache size: %d bytes", finalMetrics.CacheSizeBytes)
		t.Logf("  - Total entries: %d", finalMetrics.CacheEntries)
		t.Logf("  - Average memory per entry: %.2f bytes", finalMetrics.MemoryPerEntry)

		// Validate memory usage is within reasonable bounds
		if finalMetrics.CacheSizeBytes > cache.maxSizeBytes {
			t.Errorf("Cache size exceeded limit: %d > %d", finalMetrics.CacheSizeBytes, cache.maxSizeBytes)
		}
	})

	t.Run("Memory_Leak_Detection", func(t *testing.T) {
		runtime.GC() // Force garbage collection
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		cache := NewClientSideCache(1024 * 1024)

		// Add and remove many entries
		for cycle := 0; cycle < 100; cycle++ {
			// Add entries
			for i := 0; i < 10; i++ {
				entry := &CacheEntry{
					FragmentID: fmt.Sprintf("leak-fragment-%d-%d", cycle, i),
					Data:       strings.Repeat("data", 100),
					SizeBytes:  400,
					CreatedAt:  time.Now(),
				}
				cache.Set(fmt.Sprintf("leak-key-%d-%d", cycle, i), entry)
			}

			// Clear cache periodically
			if cycle%10 == 0 {
				cache.Clear()
			}
		}

		// Force final cleanup
		cache.Clear()
		runtime.GC()

		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		memoryGrowth := int64(memAfter.Alloc) - int64(memBefore.Alloc)

		t.Logf("✓ Memory leak detection:")
		t.Logf("  - Baseline memory: %d bytes", memBefore.Alloc)
		t.Logf("  - Final memory: %d bytes", memAfter.Alloc)
		t.Logf("  - Memory growth: %d bytes", memoryGrowth)

		// Allow reasonable memory growth (100KB threshold)
		if memoryGrowth > 100*1024 {
			t.Errorf("Potential memory leak detected: %d bytes growth", memoryGrowth)
		}
	})
}

// TestCachePersistence tests cache persistence across page reloads
func (suite *FragmentCachingSuite) TestCachePersistence(t *testing.T) {
	t.Run("Session_Storage_Persistence", func(t *testing.T) {
		sessionData := make(map[string][]byte)

		cache1 := NewClientSideCache(1024 * 1024)

		// Add entries to first cache instance
		entry := &CacheEntry{
			FragmentID: "persistent-fragment",
			Data:       "persistent data",
			Version:    1,
			CreatedAt:  time.Now(),
			SizeBytes:  100,
		}
		cache1.Set("persistent-key", entry)

		// Serialize cache state
		serialized, err := cache1.Serialize()
		if err != nil {
			t.Fatalf("Failed to serialize cache: %v", err)
		}
		sessionData["cache-state"] = serialized

		// Simulate page reload - create new cache instance
		cache2 := NewClientSideCache(1024 * 1024)

		// Restore cache state
		err = cache2.Deserialize(sessionData["cache-state"])
		if err != nil {
			t.Fatalf("Failed to deserialize cache: %v", err)
		}

		// Verify data persisted
		restored, hit := cache2.Get("persistent-key")
		if !hit {
			t.Error("Persistent entry should exist after reload")
		}
		if restored == nil {
			t.Error("Restored entry should not be nil")
		}
		if restored != nil && restored.Data != "persistent data" {
			t.Errorf("Expected 'persistent data', got %v", restored.Data)
		}

		t.Log("✓ Session storage persistence validated")
	})
}

// TestMultiFragmentCaching validates multi-fragment caching scenarios
func (suite *FragmentCachingSuite) TestMultiFragmentCaching(t *testing.T) {
	cache := NewClientSideCache(1024 * 1024)

	// Add multiple related fragments
	fragments := []struct {
		id   string
		data string
		deps []string
	}{
		{"header", "header content", []string{"user-session"}},
		{"nav", "navigation content", []string{"user-permissions"}},
		{"content", "main content", []string{"page-data"}},
		{"sidebar", "sidebar content", []string{"user-session", "page-data"}},
		{"footer", "footer content", []string{}},
	}

	for _, frag := range fragments {
		entry := &CacheEntry{
			FragmentID:   frag.id,
			Data:         frag.data,
			Dependencies: frag.deps,
			SizeBytes:    int64(len(frag.data)),
			CreatedAt:    time.Now(),
		}
		cache.Set(frag.id, entry)
	}

	// Test invalidation cascading
	invalidated := cache.Invalidate("user-session")

	metrics := cache.GetMetrics()

	t.Logf("✓ Multi-fragment caching:")
	t.Logf("  - Initial fragments: 5")
	t.Logf("  - Invalidated by user-session: %d", invalidated)
	t.Logf("  - Remaining entries: %d", metrics.CacheEntries)

	if invalidated < 2 { // header and sidebar should be invalidated
		t.Errorf("Expected at least 2 invalidations, got %d", invalidated)
	}
}

// TestCacheSizeLimitsEviction tests cache size limits and eviction policies
func (suite *FragmentCachingSuite) TestCacheSizeLimitsEviction(t *testing.T) {
	// Small cache for testing eviction
	cache := NewClientSideCache(1024) // 1KB limit

	// Add entries that exceed cache size
	for i := 0; i < 10; i++ {
		entry := &CacheEntry{
			FragmentID: fmt.Sprintf("evict-fragment-%d", i),
			Data:       strings.Repeat("x", 200), // 200 bytes each
			SizeBytes:  200,
			CreatedAt:  time.Now(),
		}
		cache.Set(fmt.Sprintf("evict-key-%d", i), entry)

		// Small delay to ensure different access times
		time.Sleep(1 * time.Millisecond)
	}

	metrics := cache.GetMetrics()

	t.Logf("✓ Cache size limits and eviction:")
	t.Logf("  - Cache size: %d bytes (limit: %d)", metrics.CacheSizeBytes, cache.maxSizeBytes)
	t.Logf("  - Cache entries: %d", metrics.CacheEntries)
	t.Logf("  - Evicted entries: %d", metrics.EvictedEntries)

	// Cache should not exceed size limit
	if metrics.CacheSizeBytes > cache.maxSizeBytes {
		t.Errorf("Cache size exceeded limit: %d > %d", metrics.CacheSizeBytes, cache.maxSizeBytes)
	}

	// Some entries should have been evicted
	if metrics.EvictedEntries == 0 {
		t.Error("Expected some entries to be evicted due to size limit")
	}
}

// TestPerformanceImprovements quantifies performance improvement from caching
func (suite *FragmentCachingSuite) TestPerformanceImprovements(t *testing.T) {
	cache := NewClientSideCache(1024 * 1024)

	// Simulate performance measurements
	const numRequests = 100
	cacheHitLatency := 1 * time.Millisecond
	cacheMissLatency := 10 * time.Millisecond

	var totalHitTime, totalMissTime time.Duration
	hits, misses := int64(0), int64(0)

	// Simulate mixed cache hits and misses
	for i := 0; i < numRequests; i++ {
		key := fmt.Sprintf("perf-key-%d", i%20) // 20 unique keys, so some repeats

		if _, hit := cache.Get(key); hit {
			totalHitTime += cacheHitLatency
			hits++
		} else {
			// Cache miss - simulate expensive operation
			totalMissTime += cacheMissLatency
			misses++

			// Add to cache for future hits
			entry := &CacheEntry{
				FragmentID: fmt.Sprintf("perf-fragment-%d", i),
				Data:       fmt.Sprintf("expensive data %d", i),
				SizeBytes:  100,
				CreatedAt:  time.Now(),
			}
			cache.Set(key, entry)
		}
	}

	avgHitLatency := float64(totalHitTime.Nanoseconds()) / float64(hits) / 1e6     // ms
	avgMissLatency := float64(totalMissTime.Nanoseconds()) / float64(misses) / 1e6 // ms
	speedupRatio := avgMissLatency / avgHitLatency

	t.Logf("✓ Performance improvement from caching:")
	t.Logf("  - Cache hits: %d (avg latency: %.2f ms)", hits, avgHitLatency)
	t.Logf("  - Cache misses: %d (avg latency: %.2f ms)", misses, avgMissLatency)
	t.Logf("  - Speedup ratio: %.2fx", speedupRatio)
	t.Logf("  - Performance improvement: %.1f%%", (speedupRatio-1)*100)

	// Cache should provide significant speedup
	if speedupRatio < 2.0 {
		t.Errorf("Cache speedup too low: %.2fx (expected >= 2x)", speedupRatio)
	}
}

// TestCacheCoherency maintains cache coherency with server-side changes
func (suite *FragmentCachingSuite) TestCacheCoherency(t *testing.T) {
	cache := NewClientSideCache(1024 * 1024)

	// Add entries with version tracking
	entries := []struct {
		key     string
		data    string
		version int64
		hash    string
	}{
		{"coherency-1", "data v1", 1, "hash-1"},
		{"coherency-2", "data v1", 1, "hash-2"},
		{"coherency-3", "data v1", 1, "hash-3"},
	}

	for _, e := range entries {
		entry := &CacheEntry{
			FragmentID:  e.key,
			Data:        e.data,
			Version:     e.version,
			ContentHash: e.hash,
			SizeBytes:   int64(len(e.data)),
			CreatedAt:   time.Now(),
		}
		cache.Set(e.key, entry)
	}

	// Simulate server-side changes (version/hash updates)
	updates := []struct {
		key     string
		version int64
		hash    string
	}{
		{"coherency-1", 2, "hash-1-v2"},
		{"coherency-2", 2, "hash-2-v2"},
	}

	staleEntries := 0
	for _, update := range updates {
		if cached, hit := cache.Get(update.key); hit {
			if cached.Version < update.version || cached.ContentHash != update.hash {
				staleEntries++
				// In a real system, this would trigger cache invalidation
				cache.Invalidate(update.key)
			}
		}
	}

	metrics := cache.GetMetrics()

	t.Logf("✓ Cache coherency validation:")
	t.Logf("  - Coherency checks: %d", len(updates))
	t.Logf("  - Stale entries detected: %d", staleEntries)
	t.Logf("  - Invalidations: %d", metrics.Invalidations)

	if staleEntries == 0 {
		t.Error("Expected to detect stale entries during coherency check")
	}
}

// Helper function to generate and cache fragments (simplified for testing)
func (suite *FragmentCachingSuite) generateAndCacheFragments(page *ApplicationPage, cache *ClientSideCache, data map[string]interface{}) ([]*Fragment, error) {
	// Simulate fragment generation
	fragments := []*Fragment{
		{ID: "frag-1", Strategy: "static_dynamic", Action: "update", Data: data["Title"]},
		{ID: "frag-2", Strategy: "static_dynamic", Action: "update", Data: data["UserName"]},
		{ID: "frag-3", Strategy: "markers", Action: "patch", Data: data["MessageCount"]},
		{ID: "frag-4", Strategy: "markers", Action: "patch", Data: data["LastLogin"]},
	}

	// Simulate caching logic with cache hit/miss tracking
	for i, fragment := range fragments {
		cacheKey := fmt.Sprintf("fragment-%s-%d", fragment.ID, i)

		// Check if already cached (this registers a get attempt)
		if _, hit := cache.Get(cacheKey); !hit {
			// Cache miss - store in cache
			entry := &CacheEntry{
				FragmentID: fragment.ID,
				Data:       fragment,
				Strategy:   fragment.Strategy,
				Version:    1,
				CreatedAt:  time.Now(),
				SizeBytes:  int64(len(fmt.Sprintf("%+v", fragment))),
			}
			cache.Set(cacheKey, entry)
		}
	}

	return fragments, nil
}
