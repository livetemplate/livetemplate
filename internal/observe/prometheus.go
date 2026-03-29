package observe

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// PrometheusExporter exports metrics in Prometheus text format.
//
// The exporter follows Prometheus naming conventions:
// - Use underscores for word separation (not camelCase or hyphens)
// - Suffix counters with _total
// - Use base units (seconds, bytes) not scaled units (milliseconds, kilobytes)
// - Include namespace prefix (livetemplate_)
//
// Thread-safe: safe for concurrent scrapes.
type PrometheusExporter struct {
	metrics      *Metrics
	limitsGetter LimitsGetter // Interface to get connection limit stats
	mu           sync.RWMutex // Protects metrics snapshot
}

// LimitsGetter provides connection limit statistics.
// Implemented by *ConnectionLimits in the main package.
type LimitsGetter interface {
	ActiveConnections() int64
	ConnectionsRejected() int64
}

// NewPrometheusExporter creates a Prometheus exporter.
//
// Parameters:
//   - metrics: The metrics tracker to export
//   - limitsGetter: Optional connection limits provider (can be nil)
func NewPrometheusExporter(metrics *Metrics, limitsGetter LimitsGetter) *PrometheusExporter {
	return &PrometheusExporter{
		metrics:      metrics,
		limitsGetter: limitsGetter,
	}
}

// WriteMetrics writes all metrics in Prometheus text format to the writer.
//
// Format: https://prometheus.io/docs/instrumenting/exposition_formats/
//
// Example output:
//
//	# HELP livetemplate_connections_active Current number of active WebSocket connections
//	# TYPE livetemplate_connections_active gauge
//	livetemplate_connections_active 42
//
//	# HELP livetemplate_actions_processed_total Total number of actions processed
//	# TYPE livetemplate_actions_processed_total counter
//	livetemplate_actions_processed_total 1234
func (e *PrometheusExporter) WriteMetrics(w io.Writer) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var sb strings.Builder

	// Connection metrics (gauges)
	e.writeGauge(&sb, "livetemplate_connections_active",
		"Current number of active WebSocket connections",
		e.metrics.activeConnections.Load())

	e.writeGauge(&sb, "livetemplate_groups_active",
		"Current number of active session groups",
		e.metrics.activeGroups.Load())

	// Connection limit metrics (from limits.go)
	if e.limitsGetter != nil {
		e.writeCounter(&sb, "livetemplate_connections_rejected_total",
			"Total number of connections rejected due to limits",
			e.limitsGetter.ConnectionsRejected())
	}

	// Action metrics (counters)
	e.writeCounter(&sb, "livetemplate_actions_processed_total",
		"Total number of actions processed",
		e.metrics.actionsProcessed.Load())

	e.writeCounter(&sb, "livetemplate_templates_executed_total",
		"Total number of template executions",
		e.metrics.templatesExecuted.Load())

	e.writeCounter(&sb, "livetemplate_trees_built_total",
		"Total number of trees built",
		e.metrics.treesBuilt.Load())

	e.writeCounter(&sb, "livetemplate_trees_diffed_total",
		"Total number of tree diffs performed",
		e.metrics.treesDiffed.Load())

	e.writeCounter(&sb, "livetemplate_broadcasts_sent_total",
		"Total number of broadcasts sent",
		e.metrics.broadcastsSent.Load())

	e.writeCounter(&sb, "livetemplate_errors_total",
		"Total number of errors encountered",
		e.metrics.errorsEncountered.Load())

	// WebSocket async sending metrics
	e.writeCounter(&sb, "livetemplate_websocket_buffer_full_total",
		"Total number of times send buffer was full (client too slow)",
		e.metrics.wsBufferFull.Load())

	e.writeCounter(&sb, "livetemplate_websocket_slow_client_closes_total",
		"Total number of connections closed due to slow message consumption",
		e.metrics.wsSlowClientCloses.Load())

	e.writeCounter(&sb, "livetemplate_websocket_write_errors_total",
		"Total number of WebSocket write errors",
		e.metrics.wsWriteErrors.Load())

	e.writeGauge(&sb, "livetemplate_websocket_send_buffer_size",
		"Current total number of queued messages across all connections",
		e.metrics.wsSendBufferSize.Load())

	e.writeCounter(&sb, "livetemplate_websocket_dispatch_dropped_total",
		"Total number of broadcast dispatch drops (dispatch channel full)",
		e.metrics.wsDispatchDropped.Load())

	// Wire format metrics (fingerprint-based diff tracking)
	e.writeCounter(&sb, "livetemplate_full_tree_sends_total",
		"Total number of sends with statics (first render or structure changed)",
		e.metrics.fullTreeSends.Load())

	e.writeCounter(&sb, "livetemplate_dynamics_only_sends_total",
		"Total number of sends without statics (structure unchanged)",
		e.metrics.dynamicsOnlySends.Load())

	e.writeCounter(&sb, "livetemplate_fingerprint_mismatches_total",
		"Total number of structure fingerprint mismatches detected",
		e.metrics.fingerprintMismatches.Load())

	// Update payload size histogram (bytes)
	e.writeSizeHistogram(&sb, "livetemplate_update_payload_bytes",
		"Distribution of update payload sizes in bytes",
		e.metrics.updatePayloadBytes)

	// Duration histograms (converted to seconds for Prometheus convention)
	// Template execution durations
	e.writeHistogram(&sb, "livetemplate_template_duration_seconds",
		"Template execution duration distribution",
		e.metrics.templateDurations)

	// Tree building durations
	e.writeHistogram(&sb, "livetemplate_build_duration_seconds",
		"Tree building duration distribution",
		e.metrics.buildDurations)

	// Tree diffing durations
	e.writeHistogram(&sb, "livetemplate_diff_duration_seconds",
		"Tree diffing duration distribution",
		e.metrics.diffDurations)

	// Action processing durations
	e.writeHistogram(&sb, "livetemplate_action_duration_seconds",
		"Action processing duration distribution",
		e.metrics.actionDurations)

	_, err := w.Write([]byte(sb.String()))
	return err
}

// writeGauge writes a gauge metric.
func (e *PrometheusExporter) writeGauge(sb *strings.Builder, name, help string, value int64) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s gauge\n", name)
	fmt.Fprintf(sb, "%s %d\n\n", name, value)
}

// writeCounter writes a counter metric.
func (e *PrometheusExporter) writeCounter(sb *strings.Builder, name, help string, value int64) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s counter\n", name)
	fmt.Fprintf(sb, "%s %d\n\n", name, value)
}

// writeHistogram writes histogram metrics (quantiles).
//
// Prometheus convention: Use summary type for pre-calculated quantiles.
// For better performance, we export quantiles rather than raw buckets.
func (e *PrometheusExporter) writeHistogram(sb *strings.Builder, name, help string, hist *DurationHistogram) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s summary\n", name)

	// Calculate common quantiles
	quantiles := []struct {
		q     int
		label string
	}{
		{50, "0.5"},
		{90, "0.9"},
		{95, "0.95"},
		{99, "0.99"},
	}

	// Write quantile values (convert milliseconds to seconds)
	for _, q := range quantiles {
		valueMs := hist.Percentile(q.q)
		valueSec := float64(valueMs) / 1000.0
		fmt.Fprintf(sb, "%s{quantile=\"%s\"} %.6f\n", name, q.label, valueSec)
	}

	// Calculate and write sum and count for summary type
	// Note: Our histogram doesn't track these, so we approximate
	// For production use with full histogram support, track sum/count separately
	fmt.Fprintf(sb, "%s_sum 0\n", name)     // Approximation: not tracked
	fmt.Fprintf(sb, "%s_count 0\n\n", name) // Approximation: not tracked
}

// writeSizeHistogram writes a size histogram metrics (quantiles in bytes).
// Unlike duration histograms, size histograms report values directly in bytes.
func (e *PrometheusExporter) writeSizeHistogram(sb *strings.Builder, name, help string, hist *SizeHistogram) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s summary\n", name)

	// Calculate common quantiles
	quantiles := []struct {
		q     int
		label string
	}{
		{50, "0.5"},
		{90, "0.9"},
		{95, "0.95"},
		{99, "0.99"},
	}

	// Write quantile values (bytes, no conversion needed)
	for _, q := range quantiles {
		valueBytes := hist.Percentile(q.q)
		fmt.Fprintf(sb, "%s{quantile=\"%s\"} %d\n", name, q.label, valueBytes)
	}

	fmt.Fprintf(sb, "%s_sum 0\n", name)     // Approximation: not tracked
	fmt.Fprintf(sb, "%s_count 0\n\n", name) // Approximation: not tracked
}

// GetMetrics returns the underlying Metrics tracker.
// Use this to record wire format metrics from send paths.
func (e *PrometheusExporter) GetMetrics() *Metrics {
	return e.metrics
}

// MetricsSnapshot represents a point-in-time snapshot of metrics.
// Useful for testing and inspection.
type MetricsSnapshot struct {
	// Counters
	ActionsProcessed  int64
	TemplatesExecuted int64
	TreesBuilt        int64
	TreesDiffed       int64
	BroadcastsSent    int64
	ErrorsEncountered int64

	// Gauges
	ActiveConnections   int64
	ActiveGroups        int64
	ConnectionsRejected int64

	// Histogram percentiles (milliseconds)
	TemplateP50 int64
	TemplateP95 int64
	TemplateP99 int64
	BuildP50    int64
	BuildP95    int64
	DiffP50     int64
	DiffP95     int64
	ActionP50   int64
	ActionP95   int64
	ActionP99   int64
}

// Snapshot returns a point-in-time snapshot of all metrics.
func (e *PrometheusExporter) Snapshot() MetricsSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	snapshot := MetricsSnapshot{
		ActionsProcessed:  e.metrics.actionsProcessed.Load(),
		TemplatesExecuted: e.metrics.templatesExecuted.Load(),
		TreesBuilt:        e.metrics.treesBuilt.Load(),
		TreesDiffed:       e.metrics.treesDiffed.Load(),
		BroadcastsSent:    e.metrics.broadcastsSent.Load(),
		ErrorsEncountered: e.metrics.errorsEncountered.Load(),
		ActiveConnections: e.metrics.activeConnections.Load(),
		ActiveGroups:      e.metrics.activeGroups.Load(),

		TemplateP50: e.metrics.templateDurations.Percentile(50),
		TemplateP95: e.metrics.templateDurations.Percentile(95),
		TemplateP99: e.metrics.templateDurations.Percentile(99),
		BuildP50:    e.metrics.buildDurations.Percentile(50),
		BuildP95:    e.metrics.buildDurations.Percentile(95),
		DiffP50:     e.metrics.diffDurations.Percentile(50),
		DiffP95:     e.metrics.diffDurations.Percentile(95),
		ActionP50:   e.metrics.actionDurations.Percentile(50),
		ActionP95:   e.metrics.actionDurations.Percentile(95),
		ActionP99:   e.metrics.actionDurations.Percentile(99),
	}

	if e.limitsGetter != nil {
		snapshot.ConnectionsRejected = e.limitsGetter.ConnectionsRejected()
	}

	return snapshot
}

// ParsePrometheusMetrics parses Prometheus text format into a map.
// Useful for testing.
func ParsePrometheusMetrics(text string) map[string]float64 {
	metrics := make(map[string]float64)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		metricName := parts[0]
		var value float64
		_, _ = fmt.Sscanf(parts[1], "%f", &value)
		metrics[metricName] = value
	}

	return metrics
}

// GetMetricNames returns all metric names in sorted order.
// Useful for documentation and testing.
func (e *PrometheusExporter) GetMetricNames() []string {
	names := []string{
		"livetemplate_connections_active",
		"livetemplate_groups_active",
		"livetemplate_connections_rejected_total",
		"livetemplate_actions_processed_total",
		"livetemplate_templates_executed_total",
		"livetemplate_trees_built_total",
		"livetemplate_trees_diffed_total",
		"livetemplate_broadcasts_sent_total",
		"livetemplate_errors_total",
		"livetemplate_template_duration_seconds",
		"livetemplate_build_duration_seconds",
		"livetemplate_diff_duration_seconds",
		"livetemplate_action_duration_seconds",
	}
	sort.Strings(names)
	return names
}
