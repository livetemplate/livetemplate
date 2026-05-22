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
	publishesSent     atomic.Int64
	errorsEncountered atomic.Int64

	// WebSocket async sending counters
	wsBufferFull       atomic.Int64 // Total count of buffer overflow events
	wsSlowClientCloses atomic.Int64 // Total count of connections closed due to slow clients
	wsWriteErrors      atomic.Int64 // Total count of WebSocket write errors
	wsDispatchDropped  atomic.Int64 // Total count of publish dispatch drops (channel full)

	// Wire format metrics (fingerprint-based diff tracking)
	fullTreeSends         atomic.Int64 // Total sends with statics (structure changed or first render)
	dynamicsOnlySends     atomic.Int64 // Total sends without statics (structure unchanged)
	fingerprintMismatches atomic.Int64 // Total fingerprint mismatches (structure changes detected)

	// Gauges (atomic for thread safety)
	activeConnections atomic.Int64
	activeGroups      atomic.Int64

	// WebSocket async sending gauges
	wsSendBufferSize atomic.Int64 // Current total messages queued across all connections

	// Histograms (track duration distributions)
	templateDurations *DurationHistogram
	buildDurations    *DurationHistogram
	diffDurations     *DurationHistogram
	actionDurations   *DurationHistogram

	// Wire format histogram (track payload sizes)
	updatePayloadBytes *SizeHistogram
}

// NewMetrics creates a new metrics tracker.
func NewMetrics(logger *slog.Logger) *Metrics {
	return &Metrics{
		logger:             logger,
		templateDurations:  NewDurationHistogram(),
		buildDurations:     NewDurationHistogram(),
		diffDurations:      NewDurationHistogram(),
		actionDurations:    NewDurationHistogram(),
		updatePayloadBytes: NewSizeHistogram(),
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

// PublishSent increments the publishes-sent counter (peer-fan-out via ctx.Publish).
func (m *Metrics) PublishSent() {
	m.publishesSent.Add(1)
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

// WebSocket async sending operations

// WSBufferFull increments the buffer overflow counter.
func (m *Metrics) WSBufferFull() {
	m.wsBufferFull.Add(1)
}

// WSSlowClientClose increments the slow client close counter.
func (m *Metrics) WSSlowClientClose() {
	m.wsSlowClientCloses.Add(1)
}

// WSWriteError increments the write error counter.
func (m *Metrics) WSWriteError() {
	m.wsWriteErrors.Add(1)
}

// WSSetBufferSize sets the current total buffer size across all connections.
func (m *Metrics) WSSetBufferSize(size int64) {
	m.wsSendBufferSize.Store(size)
}

// WSAddBufferSize adds to the current buffer size (when message queued).
func (m *Metrics) WSAddBufferSize(delta int64) {
	m.wsSendBufferSize.Add(delta)
}

// WSDispatchDropped increments the publish dispatch drop counter.
func (m *Metrics) WSDispatchDropped() {
	m.wsDispatchDropped.Add(1)
}

// Wire format metrics operations

// FullTreeSent records when a full tree (with statics) is sent to the client.
// This happens on first render or when the structure fingerprint changes.
func (m *Metrics) FullTreeSent() {
	m.fullTreeSends.Add(1)
}

// DynamicsOnlySent records when only dynamics (no statics) are sent.
// This happens when structure is unchanged (fingerprints match).
func (m *Metrics) DynamicsOnlySent() {
	m.dynamicsOnlySends.Add(1)
}

// FingerprintMismatch records when a structure change is detected via fingerprint comparison.
func (m *Metrics) FingerprintMismatch() {
	m.fingerprintMismatches.Add(1)
}

// UpdatePayloadSent records the size of an update payload sent to the client.
func (m *Metrics) UpdatePayloadSent(sizeBytes int) {
	m.updatePayloadBytes.Record(int64(sizeBytes))
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
		"publishes_sent", m.publishesSent.Load(),
		"errors_encountered", m.errorsEncountered.Load(),

		// WebSocket async sending counters
		"ws_buffer_full", m.wsBufferFull.Load(),
		"ws_slow_client_closes", m.wsSlowClientCloses.Load(),
		"ws_write_errors", m.wsWriteErrors.Load(),

		// Wire format counters (fingerprint-based diff tracking)
		"full_tree_sends", m.fullTreeSends.Load(),
		"dynamics_only_sends", m.dynamicsOnlySends.Load(),
		"fingerprint_mismatches", m.fingerprintMismatches.Load(),

		// Gauges
		"active_connections", m.activeConnections.Load(),
		"active_groups", m.activeGroups.Load(),
		"ws_send_buffer_size", m.wsSendBufferSize.Load(),

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

		// Histogram percentiles (update payload sizes)
		"payload_p50_bytes", m.updatePayloadBytes.Percentile(50),
		"payload_p95_bytes", m.updatePayloadBytes.Percentile(95),
		"payload_p99_bytes", m.updatePayloadBytes.Percentile(99),
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

// SizeHistogram tracks byte size distribution using a simple ring buffer.
// Similar to DurationHistogram but for payload sizes instead of durations.
// Thread-safe: uses mutex to protect samples array from concurrent access.
type SizeHistogram struct {
	mu      sync.RWMutex // Protects samples array
	samples []int64
	pos     atomic.Int64
	size    int
}

// NewSizeHistogram creates a new size histogram.
// Keeps the last 1000 samples for percentile calculation.
func NewSizeHistogram() *SizeHistogram {
	return &SizeHistogram{
		samples: make([]int64, 1000),
		size:    1000,
	}
}

// Record adds a size sample (in bytes) to the histogram.
func (h *SizeHistogram) Record(sizeBytes int64) {
	pos := int(h.pos.Add(1) % int64(h.size))
	h.mu.Lock()
	h.samples[pos] = sizeBytes
	h.mu.Unlock()
}

// Percentile returns the approximate percentile value in bytes.
// p should be between 0 and 100 (e.g., 50 for median, 95 for p95).
func (h *SizeHistogram) Percentile(p int) int64 {
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
	nonZero := make([]int64, 0, count)
	for _, s := range samplesCopy {
		if s > 0 {
			nonZero = append(nonZero, s)
		}
	}

	// Sort samples (reuse quickSort from DurationHistogram)
	quickSort(nonZero, 0, len(nonZero)-1)

	// Calculate percentile index
	index := (p * len(nonZero)) / 100
	if index >= len(nonZero) {
		index = len(nonZero) - 1
	}

	return nonZero[index]
}
