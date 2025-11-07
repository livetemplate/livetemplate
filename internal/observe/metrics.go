package observe

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks operational metrics using slog for emission.
// All metrics are thread-safe using atomic operations.
type Metrics struct {
	logger *slog.Logger

	// Counters (atomic for thread safety)
	actionsProcessed  atomic.Int64
	templatesExecuted atomic.Int64
	treesBuilt        atomic.Int64
	treesDiffed       atomic.Int64
	broadcastsSent    atomic.Int64
	errorsEncountered atomic.Int64

	// Gauges (atomic for thread safety)
	activeConnections atomic.Int64
	activeGroups      atomic.Int64

	// Histograms (track duration distributions)
	templateDurations *DurationHistogram
	buildDurations    *DurationHistogram
	diffDurations     *DurationHistogram
	actionDurations   *DurationHistogram
}

// NewMetrics creates a new metrics tracker.
func NewMetrics(logger *slog.Logger) *Metrics {
	return &Metrics{
		logger:            logger,
		templateDurations: NewDurationHistogram(),
		buildDurations:    NewDurationHistogram(),
		diffDurations:     NewDurationHistogram(),
		actionDurations:   NewDurationHistogram(),
	}
}

// Counter operations

// ActionProcessed increments the actions processed counter.
func (m *Metrics) ActionProcessed() {
	m.actionsProcessed.Add(1)
}

// TemplateExecuted records template execution.
func (m *Metrics) TemplateExecuted(duration time.Duration) {
	m.templatesExecuted.Add(1)
	m.templateDurations.Record(duration)
}

// TreeBuilt records tree building.
func (m *Metrics) TreeBuilt(duration time.Duration) {
	m.treesBuilt.Add(1)
	m.buildDurations.Record(duration)
}

// TreeDiffed records tree diffing.
func (m *Metrics) TreeDiffed(duration time.Duration) {
	m.treesDiffed.Add(1)
	m.diffDurations.Record(duration)
}

// BroadcastSent increments the broadcasts sent counter.
func (m *Metrics) BroadcastSent() {
	m.broadcastsSent.Add(1)
}

// ErrorEncountered increments the errors counter.
func (m *Metrics) ErrorEncountered() {
	m.errorsEncountered.Add(1)
}

// Gauge operations

// ConnectionAdded increments active connections.
func (m *Metrics) ConnectionAdded() {
	m.activeConnections.Add(1)
}

// ConnectionRemoved decrements active connections.
func (m *Metrics) ConnectionRemoved() {
	m.activeConnections.Add(-1)
}

// GroupCreated increments active groups.
func (m *Metrics) GroupCreated() {
	m.activeGroups.Add(1)
}

// GroupRemoved decrements active groups.
func (m *Metrics) GroupRemoved() {
	m.activeGroups.Add(-1)
}

// EmitPeriodically emits metrics at the specified interval.
// This should be called in a goroutine: go metrics.EmitPeriodically(60*time.Second)
func (m *Metrics) EmitPeriodically(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		m.emit()
	}
}

// emit logs current metric values via slog.
func (m *Metrics) emit() {
	m.logger.Info("metrics",
		// Counters
		"actions_processed", m.actionsProcessed.Load(),
		"templates_executed", m.templatesExecuted.Load(),
		"trees_built", m.treesBuilt.Load(),
		"trees_diffed", m.treesDiffed.Load(),
		"broadcasts_sent", m.broadcastsSent.Load(),
		"errors_encountered", m.errorsEncountered.Load(),

		// Gauges
		"active_connections", m.activeConnections.Load(),
		"active_groups", m.activeGroups.Load(),

		// Histogram percentiles (template execution)
		"template_p50_ms", m.templateDurations.Percentile(50),
		"template_p95_ms", m.templateDurations.Percentile(95),
		"template_p99_ms", m.templateDurations.Percentile(99),

		// Histogram percentiles (tree building)
		"build_p50_ms", m.buildDurations.Percentile(50),
		"build_p95_ms", m.buildDurations.Percentile(95),

		// Histogram percentiles (tree diffing)
		"diff_p50_ms", m.diffDurations.Percentile(50),
		"diff_p95_ms", m.diffDurations.Percentile(95),

		// Histogram percentiles (action processing)
		"action_p50_ms", m.actionDurations.Percentile(50),
		"action_p95_ms", m.actionDurations.Percentile(95),
	)
}

// DurationHistogram tracks duration distribution using a simple ring buffer.
// This is a simplified histogram suitable for percentile calculations.
// Thread-safe: uses mutex to protect samples array from concurrent access.
type DurationHistogram struct {
	mu      sync.RWMutex // Protects samples array
	samples []int64
	pos     atomic.Int64
	size    int
}

// NewDurationHistogram creates a new duration histogram.
// Keeps the last 1000 samples for percentile calculation.
func NewDurationHistogram() *DurationHistogram {
	return &DurationHistogram{
		samples: make([]int64, 1000),
		size:    1000,
	}
}

// Record adds a duration sample to the histogram.
func (h *DurationHistogram) Record(d time.Duration) {
	pos := int(h.pos.Add(1) % int64(h.size))
	h.mu.Lock()
	h.samples[pos] = d.Milliseconds()
	h.mu.Unlock()
}

// Percentile returns the approximate percentile value.
// p should be between 0 and 100 (e.g., 50 for median, 95 for p95).
//
// Note: This is a simplified implementation. For production use with
// high accuracy requirements, consider using a library like HdrHistogram.
func (h *DurationHistogram) Percentile(p int) int64 {
	if p < 0 || p > 100 {
		return 0
	}

	// Copy samples with read lock to prevent data races
	h.mu.RLock()
	samplesCopy := make([]int64, h.size)
	copy(samplesCopy, h.samples)
	h.mu.RUnlock()

	// Count non-zero samples
	var count int
	for _, s := range samplesCopy {
		if s > 0 {
			count++
		}
	}

	if count == 0 {
		return 0
	}

	// Simple percentile calculation using sorting
	// For better performance, use a sketch algorithm
	nonZero := make([]int64, 0, count)
	for _, s := range samplesCopy {
		if s > 0 {
			nonZero = append(nonZero, s)
		}
	}

	// Sort samples
	quickSort(nonZero, 0, len(nonZero)-1)

	// Calculate percentile index
	index := (p * len(nonZero)) / 100
	if index >= len(nonZero) {
		index = len(nonZero) - 1
	}

	return nonZero[index]
}

// quickSort is a simple in-place quicksort for int64 slices.
func quickSort(arr []int64, low, high int) {
	if low < high {
		p := partition(arr, low, high)
		quickSort(arr, low, p-1)
		quickSort(arr, p+1, high)
	}
}

func partition(arr []int64, low, high int) int {
	pivot := arr[high]
	i := low - 1

	for j := low; j < high; j++ {
		if arr[j] < pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}

	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}
