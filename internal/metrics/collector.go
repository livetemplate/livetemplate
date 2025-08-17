package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
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

	// Internal tracking for derived metrics
	totalHTMLDiffTime    int64 // Cumulative time in nanoseconds
	totalOriginalBytes   int64 // Cumulative original size
	totalCompressedBytes int64 // Cumulative compressed size
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

	// HTML diffing performance metrics
	HTMLDiffsPerformed      int64         `json:"html_diffs_performed"`
	HTMLDiffErrors          int64         `json:"html_diff_errors"`
	HTMLDiffTotalTime       int64         `json:"html_diff_total_time_ns"`
	HTMLDiffAverageTime     time.Duration `json:"html_diff_average_time"`
	HTMLDiffAccuracyScore   float64       `json:"html_diff_accuracy_score"`
	ChangePatternDetections int64         `json:"change_pattern_detections"`

	// Strategy usage distribution (four-tier system)
	StaticDynamicUsage    int64 `json:"static_dynamic_usage"` // Strategy 1: Text-only changes
	MarkerUsage           int64 `json:"marker_usage"`         // Strategy 2: Attribute changes
	GranularUsage         int64 `json:"granular_usage"`       // Strategy 3: Structural changes
	ReplacementUsage      int64 `json:"replacement_usage"`    // Strategy 4: Complex changes
	StrategySelectionTime int64 `json:"strategy_selection_time_ns"`

	// Bandwidth savings metrics
	OriginalBytes       int64   `json:"original_bytes"`
	CompressedBytes     int64   `json:"compressed_bytes"`
	TotalBytesSaved     int64   `json:"total_bytes_saved"`
	BandwidthSavingsPct float64 `json:"bandwidth_savings_pct"`

	// Strategy efficiency metrics
	StaticDynamicSavings    int64   `json:"static_dynamic_savings"` // Bytes saved by Strategy 1
	MarkerSavings           int64   `json:"marker_savings"`         // Bytes saved by Strategy 2
	GranularSavings         int64   `json:"granular_savings"`       // Bytes saved by Strategy 3
	ReplacementSavings      int64   `json:"replacement_savings"`    // Bytes saved by Strategy 4
	AverageCompressionRatio float64 `json:"average_compression_ratio"`

	// Uptime
	StartTime time.Time     `json:"start_time"`
	Uptime    time.Duration `json:"uptime"`
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		applicationMetrics: &ApplicationMetrics{
			StartTime:               time.Now(),
			HTMLDiffAccuracyScore:   0.0,
			BandwidthSavingsPct:     0.0,
			AverageCompressionRatio: 1.0,
		},
		operationCounters:    make(map[string]*int64),
		startTime:            time.Now(),
		totalHTMLDiffTime:    0,
		totalOriginalBytes:   0,
		totalCompressedBytes: 0,
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

// HTML Diffing Metrics

// RecordHTMLDiffPerformed records a successful HTML diff operation
func (c *Collector) RecordHTMLDiffPerformed(duration time.Duration, accuracyScore float64) {
	atomic.AddInt64(&c.applicationMetrics.HTMLDiffsPerformed, 1)
	atomic.AddInt64(&c.applicationMetrics.HTMLDiffTotalTime, duration.Nanoseconds())
	atomic.AddInt64(&c.totalHTMLDiffTime, duration.Nanoseconds())

	// Update accuracy score (simple moving average approximation)
	performed := atomic.LoadInt64(&c.applicationMetrics.HTMLDiffsPerformed)
	if performed > 0 {
		currentScore := c.applicationMetrics.HTMLDiffAccuracyScore
		newScore := ((currentScore * float64(performed-1)) + accuracyScore) / float64(performed)
		c.applicationMetrics.HTMLDiffAccuracyScore = newScore
	}
}

// RecordHTMLDiffError records an HTML diff operation error
func (c *Collector) RecordHTMLDiffError() {
	atomic.AddInt64(&c.applicationMetrics.HTMLDiffErrors, 1)
}

// RecordChangePatternDetection records a successful change pattern detection
func (c *Collector) RecordChangePatternDetection() {
	atomic.AddInt64(&c.applicationMetrics.ChangePatternDetections, 1)
}

// Strategy Usage Metrics

// RecordStrategyUsage records usage of a specific strategy with timing
func (c *Collector) RecordStrategyUsage(strategy string, selectionTime time.Duration) {
	atomic.AddInt64(&c.applicationMetrics.StrategySelectionTime, selectionTime.Nanoseconds())

	switch strategy {
	case "static_dynamic":
		atomic.AddInt64(&c.applicationMetrics.StaticDynamicUsage, 1)
	case "markers":
		atomic.AddInt64(&c.applicationMetrics.MarkerUsage, 1)
	case "granular":
		atomic.AddInt64(&c.applicationMetrics.GranularUsage, 1)
	case "replacement":
		atomic.AddInt64(&c.applicationMetrics.ReplacementUsage, 1)
	default:
		// Unknown strategy, use custom counter
		c.IncrementCustomCounter("strategy_" + strategy)
	}
}

// Bandwidth Savings Metrics

// RecordBandwidthSavings records bandwidth savings for a fragment update
func (c *Collector) RecordBandwidthSavings(originalSize, compressedSize int64, strategy string) {
	atomic.AddInt64(&c.applicationMetrics.OriginalBytes, originalSize)
	atomic.AddInt64(&c.applicationMetrics.CompressedBytes, compressedSize)
	atomic.AddInt64(&c.totalOriginalBytes, originalSize)
	atomic.AddInt64(&c.totalCompressedBytes, compressedSize)

	bytesSaved := originalSize - compressedSize
	if bytesSaved > 0 {
		atomic.AddInt64(&c.applicationMetrics.TotalBytesSaved, bytesSaved)

		// Record strategy-specific savings
		switch strategy {
		case "static_dynamic":
			atomic.AddInt64(&c.applicationMetrics.StaticDynamicSavings, bytesSaved)
		case "markers":
			atomic.AddInt64(&c.applicationMetrics.MarkerSavings, bytesSaved)
		case "granular":
			atomic.AddInt64(&c.applicationMetrics.GranularSavings, bytesSaved)
		case "replacement":
			atomic.AddInt64(&c.applicationMetrics.ReplacementSavings, bytesSaved)
		}
	}
}

// UpdateBandwidthMetrics recalculates bandwidth savings percentage and compression ratio
func (c *Collector) UpdateBandwidthMetrics() {
	totalOriginal := atomic.LoadInt64(&c.applicationMetrics.OriginalBytes)
	totalCompressed := atomic.LoadInt64(&c.applicationMetrics.CompressedBytes)

	if totalOriginal > 0 {
		// Calculate bandwidth savings percentage
		savingsPct := float64(totalOriginal-totalCompressed) / float64(totalOriginal) * 100.0
		c.applicationMetrics.BandwidthSavingsPct = savingsPct

		// Calculate average compression ratio
		compressionRatio := float64(totalCompressed) / float64(totalOriginal)
		c.applicationMetrics.AverageCompressionRatio = compressionRatio
	}
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

	// Update derived metrics
	c.UpdateBandwidthMetrics()

	// Calculate average HTML diff time
	totalDiffs := atomic.LoadInt64(&c.applicationMetrics.HTMLDiffsPerformed)
	var avgDiffTime time.Duration
	if totalDiffs > 0 {
		totalTime := atomic.LoadInt64(&c.applicationMetrics.HTMLDiffTotalTime)
		avgDiffTime = time.Duration(totalTime / totalDiffs)
	}

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

		// HTML diffing metrics
		HTMLDiffsPerformed:      atomic.LoadInt64(&c.applicationMetrics.HTMLDiffsPerformed),
		HTMLDiffErrors:          atomic.LoadInt64(&c.applicationMetrics.HTMLDiffErrors),
		HTMLDiffTotalTime:       atomic.LoadInt64(&c.applicationMetrics.HTMLDiffTotalTime),
		HTMLDiffAverageTime:     avgDiffTime,
		HTMLDiffAccuracyScore:   c.applicationMetrics.HTMLDiffAccuracyScore,
		ChangePatternDetections: atomic.LoadInt64(&c.applicationMetrics.ChangePatternDetections),

		// Strategy usage metrics
		StaticDynamicUsage:    atomic.LoadInt64(&c.applicationMetrics.StaticDynamicUsage),
		MarkerUsage:           atomic.LoadInt64(&c.applicationMetrics.MarkerUsage),
		GranularUsage:         atomic.LoadInt64(&c.applicationMetrics.GranularUsage),
		ReplacementUsage:      atomic.LoadInt64(&c.applicationMetrics.ReplacementUsage),
		StrategySelectionTime: atomic.LoadInt64(&c.applicationMetrics.StrategySelectionTime),

		// Bandwidth savings metrics
		OriginalBytes:       atomic.LoadInt64(&c.applicationMetrics.OriginalBytes),
		CompressedBytes:     atomic.LoadInt64(&c.applicationMetrics.CompressedBytes),
		TotalBytesSaved:     atomic.LoadInt64(&c.applicationMetrics.TotalBytesSaved),
		BandwidthSavingsPct: c.applicationMetrics.BandwidthSavingsPct,

		// Strategy efficiency metrics
		StaticDynamicSavings:    atomic.LoadInt64(&c.applicationMetrics.StaticDynamicSavings),
		MarkerSavings:           atomic.LoadInt64(&c.applicationMetrics.MarkerSavings),
		GranularSavings:         atomic.LoadInt64(&c.applicationMetrics.GranularSavings),
		ReplacementSavings:      atomic.LoadInt64(&c.applicationMetrics.ReplacementSavings),
		AverageCompressionRatio: c.applicationMetrics.AverageCompressionRatio,

		StartTime: c.applicationMetrics.StartTime,
		Uptime:    time.Since(c.startTime),
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

	// Reset HTML diffing metrics
	atomic.StoreInt64(&c.applicationMetrics.HTMLDiffsPerformed, 0)
	atomic.StoreInt64(&c.applicationMetrics.HTMLDiffErrors, 0)
	atomic.StoreInt64(&c.applicationMetrics.HTMLDiffTotalTime, 0)
	c.applicationMetrics.HTMLDiffAccuracyScore = 0.0
	atomic.StoreInt64(&c.applicationMetrics.ChangePatternDetections, 0)

	// Reset strategy usage metrics
	atomic.StoreInt64(&c.applicationMetrics.StaticDynamicUsage, 0)
	atomic.StoreInt64(&c.applicationMetrics.MarkerUsage, 0)
	atomic.StoreInt64(&c.applicationMetrics.GranularUsage, 0)
	atomic.StoreInt64(&c.applicationMetrics.ReplacementUsage, 0)
	atomic.StoreInt64(&c.applicationMetrics.StrategySelectionTime, 0)

	// Reset bandwidth savings metrics
	atomic.StoreInt64(&c.applicationMetrics.OriginalBytes, 0)
	atomic.StoreInt64(&c.applicationMetrics.CompressedBytes, 0)
	atomic.StoreInt64(&c.applicationMetrics.TotalBytesSaved, 0)
	c.applicationMetrics.BandwidthSavingsPct = 0.0
	atomic.StoreInt64(&c.applicationMetrics.StaticDynamicSavings, 0)
	atomic.StoreInt64(&c.applicationMetrics.MarkerSavings, 0)
	atomic.StoreInt64(&c.applicationMetrics.GranularSavings, 0)
	atomic.StoreInt64(&c.applicationMetrics.ReplacementSavings, 0)
	c.applicationMetrics.AverageCompressionRatio = 1.0

	// Reset internal tracking
	c.totalHTMLDiffTime = 0
	c.totalOriginalBytes = 0
	c.totalCompressedBytes = 0

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

// Strategy Metrics Utility Functions

// GetStrategyDistribution returns the percentage distribution of strategies
func (c *Collector) GetStrategyDistribution() map[string]float64 {
	static := atomic.LoadInt64(&c.applicationMetrics.StaticDynamicUsage)
	marker := atomic.LoadInt64(&c.applicationMetrics.MarkerUsage)
	granular := atomic.LoadInt64(&c.applicationMetrics.GranularUsage)
	replacement := atomic.LoadInt64(&c.applicationMetrics.ReplacementUsage)

	total := static + marker + granular + replacement
	if total == 0 {
		return map[string]float64{
			"static_dynamic": 0.0,
			"markers":        0.0,
			"granular":       0.0,
			"replacement":    0.0,
		}
	}

	return map[string]float64{
		"static_dynamic": float64(static) / float64(total) * 100.0,
		"markers":        float64(marker) / float64(total) * 100.0,
		"granular":       float64(granular) / float64(total) * 100.0,
		"replacement":    float64(replacement) / float64(total) * 100.0,
	}
}

// GetStrategyEfficiencyRatios returns efficiency ratios for each strategy
func (c *Collector) GetStrategyEfficiencyRatios() map[string]float64 {
	static := atomic.LoadInt64(&c.applicationMetrics.StaticDynamicUsage)
	marker := atomic.LoadInt64(&c.applicationMetrics.MarkerUsage)
	granular := atomic.LoadInt64(&c.applicationMetrics.GranularUsage)
	replacement := atomic.LoadInt64(&c.applicationMetrics.ReplacementUsage)

	staticSavings := atomic.LoadInt64(&c.applicationMetrics.StaticDynamicSavings)
	markerSavings := atomic.LoadInt64(&c.applicationMetrics.MarkerSavings)
	granularSavings := atomic.LoadInt64(&c.applicationMetrics.GranularSavings)
	replacementSavings := atomic.LoadInt64(&c.applicationMetrics.ReplacementSavings)

	ratios := make(map[string]float64)

	if static > 0 {
		ratios["static_dynamic"] = float64(staticSavings) / float64(static)
	}
	if marker > 0 {
		ratios["markers"] = float64(markerSavings) / float64(marker)
	}
	if granular > 0 {
		ratios["granular"] = float64(granularSavings) / float64(granular)
	}
	if replacement > 0 {
		ratios["replacement"] = float64(replacementSavings) / float64(replacement)
	}

	return ratios
}

// GetHTMLDiffSuccessRate returns the success rate of HTML diff operations
func (c *Collector) GetHTMLDiffSuccessRate() float64 {
	performed := atomic.LoadInt64(&c.applicationMetrics.HTMLDiffsPerformed)
	errors := atomic.LoadInt64(&c.applicationMetrics.HTMLDiffErrors)

	total := performed + errors
	if total == 0 {
		return 100.0 // No operations means 100% success rate
	}

	return float64(performed) / float64(total) * 100.0
}

// GetAverageStrategySelectionTime returns average time to select strategy
func (c *Collector) GetAverageStrategySelectionTime() time.Duration {
	totalTime := atomic.LoadInt64(&c.applicationMetrics.StrategySelectionTime)
	totalSelections := atomic.LoadInt64(&c.applicationMetrics.StaticDynamicUsage) +
		atomic.LoadInt64(&c.applicationMetrics.MarkerUsage) +
		atomic.LoadInt64(&c.applicationMetrics.GranularUsage) +
		atomic.LoadInt64(&c.applicationMetrics.ReplacementUsage)

	if totalSelections == 0 {
		return 0
	}

	return time.Duration(totalTime / totalSelections)
}

// Prometheus Export Functionality

// PrometheusMetrics represents metrics in Prometheus format
type PrometheusMetrics struct {
	Metrics []PrometheusMetric `json:"metrics"`
}

type PrometheusMetric struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"` // counter, gauge, histogram
	Help   string            `json:"help"`
	Value  interface{}       `json:"value"`
	Labels map[string]string `json:"labels,omitempty"`
}

// ExportPrometheusMetrics returns metrics in Prometheus format
func (c *Collector) ExportPrometheusMetrics() *PrometheusMetrics {
	metrics := c.GetMetrics()

	promMetrics := &PrometheusMetrics{
		Metrics: []PrometheusMetric{
			// Page management metrics
			{Name: "livetemplate_pages_created_total", Type: "counter", Help: "Total number of pages created", Value: metrics.PagesCreated},
			{Name: "livetemplate_pages_destroyed_total", Type: "counter", Help: "Total number of pages destroyed", Value: metrics.PagesDestroyed},
			{Name: "livetemplate_active_pages", Type: "gauge", Help: "Current number of active pages", Value: metrics.ActivePages},
			{Name: "livetemplate_max_concurrent_pages", Type: "gauge", Help: "Maximum concurrent pages observed", Value: metrics.MaxConcurrentPages},

			// Token metrics
			{Name: "livetemplate_tokens_generated_total", Type: "counter", Help: "Total JWT tokens generated", Value: metrics.TokensGenerated},
			{Name: "livetemplate_tokens_verified_total", Type: "counter", Help: "Total JWT tokens verified", Value: metrics.TokensVerified},
			{Name: "livetemplate_token_failures_total", Type: "counter", Help: "Total JWT token verification failures", Value: metrics.TokenFailures},

			// Fragment generation metrics
			{Name: "livetemplate_fragments_generated_total", Type: "counter", Help: "Total fragments generated", Value: metrics.FragmentsGenerated},
			{Name: "livetemplate_generation_errors_total", Type: "counter", Help: "Total fragment generation errors", Value: metrics.GenerationErrors},

			// Memory metrics
			{Name: "livetemplate_memory_usage_bytes", Type: "gauge", Help: "Total memory usage in bytes", Value: metrics.TotalMemoryUsage},
			{Name: "livetemplate_average_page_memory_bytes", Type: "gauge", Help: "Average memory per page in bytes", Value: metrics.AveragePageMemory},

			// HTML diffing metrics
			{Name: "livetemplate_html_diffs_performed_total", Type: "counter", Help: "Total HTML diff operations performed", Value: metrics.HTMLDiffsPerformed},
			{Name: "livetemplate_html_diff_errors_total", Type: "counter", Help: "Total HTML diff operation errors", Value: metrics.HTMLDiffErrors},
			{Name: "livetemplate_html_diff_duration_seconds", Type: "gauge", Help: "Average HTML diff operation duration", Value: metrics.HTMLDiffAverageTime.Seconds()},
			{Name: "livetemplate_html_diff_accuracy_score", Type: "gauge", Help: "HTML diff accuracy score (0-1)", Value: metrics.HTMLDiffAccuracyScore},
			{Name: "livetemplate_change_pattern_detections_total", Type: "counter", Help: "Total change pattern detections", Value: metrics.ChangePatternDetections},

			// Strategy usage metrics
			{Name: "livetemplate_strategy_usage_total", Type: "counter", Help: "Strategy usage count", Value: metrics.StaticDynamicUsage, Labels: map[string]string{"strategy": "static_dynamic"}},
			{Name: "livetemplate_strategy_usage_total", Type: "counter", Help: "Strategy usage count", Value: metrics.MarkerUsage, Labels: map[string]string{"strategy": "markers"}},
			{Name: "livetemplate_strategy_usage_total", Type: "counter", Help: "Strategy usage count", Value: metrics.GranularUsage, Labels: map[string]string{"strategy": "granular"}},
			{Name: "livetemplate_strategy_usage_total", Type: "counter", Help: "Strategy usage count", Value: metrics.ReplacementUsage, Labels: map[string]string{"strategy": "replacement"}},

			// Bandwidth savings metrics
			{Name: "livetemplate_original_bytes_total", Type: "counter", Help: "Total original bytes before compression", Value: metrics.OriginalBytes},
			{Name: "livetemplate_compressed_bytes_total", Type: "counter", Help: "Total compressed bytes after optimization", Value: metrics.CompressedBytes},
			{Name: "livetemplate_bytes_saved_total", Type: "counter", Help: "Total bytes saved through optimization", Value: metrics.TotalBytesSaved},
			{Name: "livetemplate_bandwidth_savings_percent", Type: "gauge", Help: "Bandwidth savings percentage", Value: metrics.BandwidthSavingsPct},
			{Name: "livetemplate_compression_ratio", Type: "gauge", Help: "Average compression ratio", Value: metrics.AverageCompressionRatio},

			// Strategy efficiency metrics
			{Name: "livetemplate_strategy_bytes_saved_total", Type: "counter", Help: "Bytes saved by strategy", Value: metrics.StaticDynamicSavings, Labels: map[string]string{"strategy": "static_dynamic"}},
			{Name: "livetemplate_strategy_bytes_saved_total", Type: "counter", Help: "Bytes saved by strategy", Value: metrics.MarkerSavings, Labels: map[string]string{"strategy": "markers"}},
			{Name: "livetemplate_strategy_bytes_saved_total", Type: "counter", Help: "Bytes saved by strategy", Value: metrics.GranularSavings, Labels: map[string]string{"strategy": "granular"}},
			{Name: "livetemplate_strategy_bytes_saved_total", Type: "counter", Help: "Bytes saved by strategy", Value: metrics.ReplacementSavings, Labels: map[string]string{"strategy": "replacement"}},

			// Cleanup metrics
			{Name: "livetemplate_cleanup_operations_total", Type: "counter", Help: "Total cleanup operations", Value: metrics.CleanupOperations},
			{Name: "livetemplate_expired_pages_removed_total", Type: "counter", Help: "Total expired pages removed", Value: metrics.ExpiredPagesRemoved},

			// Uptime metric
			{Name: "livetemplate_uptime_seconds", Type: "gauge", Help: "Application uptime in seconds", Value: metrics.Uptime.Seconds()},
		},
	}

	return promMetrics
}

// ExportPrometheusText returns metrics in Prometheus text format
func (c *Collector) ExportPrometheusText() string {
	promMetrics := c.ExportPrometheusMetrics()
	var builder strings.Builder

	for _, metric := range promMetrics.Metrics {
		// Write help comment
		builder.WriteString(fmt.Sprintf("# HELP %s %s\n", metric.Name, metric.Help))

		// Write type comment
		builder.WriteString(fmt.Sprintf("# TYPE %s %s\n", metric.Name, metric.Type))

		// Write metric value with labels
		if len(metric.Labels) > 0 {
			labelPairs := make([]string, 0, len(metric.Labels))
			for key, value := range metric.Labels {
				labelPairs = append(labelPairs, fmt.Sprintf(`%s="%s"`, key, value))
			}
			builder.WriteString(fmt.Sprintf("%s{%s} %v\n", metric.Name, strings.Join(labelPairs, ","), metric.Value))
		} else {
			builder.WriteString(fmt.Sprintf("%s %v\n", metric.Name, metric.Value))
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

// ExportPrometheusJSON returns metrics in JSON format compatible with Prometheus
func (c *Collector) ExportPrometheusJSON() (string, error) {
	promMetrics := c.ExportPrometheusMetrics()
	bytes, err := json.MarshalIndent(promMetrics, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal Prometheus metrics: %w", err)
	}
	return string(bytes), nil
}
