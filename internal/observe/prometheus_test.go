package observe

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// mockLimitsGetter implements LimitsGetter for testing.
type mockLimitsGetter struct {
	activeConnections   int64
	connectionsRejected int64
}

func (m *mockLimitsGetter) ActiveConnections() int64 {
	return m.activeConnections
}

func (m *mockLimitsGetter) ConnectionsRejected() int64 {
	return m.connectionsRejected
}

func TestPrometheusExporter_WriteMetrics(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)

	// Set some metric values
	metrics.ActionProcessed()
	metrics.ActionProcessed()
	metrics.TemplateExecuted(10 * time.Millisecond)
	metrics.TemplateExecuted(20 * time.Millisecond)
	metrics.TreeBuilt(5 * time.Millisecond)
	metrics.TreeDiffed(3 * time.Millisecond)
	metrics.BroadcastSent()
	metrics.ErrorEncountered()
	metrics.ConnectionAdded()
	metrics.ConnectionAdded()
	metrics.GroupCreated()

	// Create limits getter with test data
	limitsGetter := &mockLimitsGetter{
		activeConnections:   2,
		connectionsRejected: 5,
	}

	exporter := NewPrometheusExporter(metrics, limitsGetter)

	var buf bytes.Buffer
	err := exporter.WriteMetrics(&buf)
	if err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	output := buf.String()

	// Verify key metrics are present
	requiredMetrics := []string{
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

	for _, metric := range requiredMetrics {
		if !strings.Contains(output, metric) {
			t.Errorf("Output missing metric: %s", metric)
		}
	}

	// Verify HELP and TYPE directives are present
	if !strings.Contains(output, "# HELP") {
		t.Error("Output missing HELP directives")
	}
	if !strings.Contains(output, "# TYPE") {
		t.Error("Output missing TYPE directives")
	}

	// Verify counter type
	if !strings.Contains(output, "# TYPE livetemplate_actions_processed_total counter") {
		t.Error("Actions metric should be counter type")
	}

	// Verify gauge type
	if !strings.Contains(output, "# TYPE livetemplate_connections_active gauge") {
		t.Error("Connections metric should be gauge type")
	}

	// Verify summary type for histograms
	if !strings.Contains(output, "# TYPE livetemplate_template_duration_seconds summary") {
		t.Error("Template duration should be summary type")
	}

	// Verify quantile labels are present
	if !strings.Contains(output, `quantile="0.5"`) {
		t.Error("Missing p50 quantile")
	}
	if !strings.Contains(output, `quantile="0.95"`) {
		t.Error("Missing p95 quantile")
	}
	if !strings.Contains(output, `quantile="0.99"`) {
		t.Error("Missing p99 quantile")
	}

	// Verify values are correct
	parsedMetrics := ParsePrometheusMetrics(output)

	if parsedMetrics["livetemplate_actions_processed_total"] != 2 {
		t.Errorf("Expected 2 actions processed, got %f", parsedMetrics["livetemplate_actions_processed_total"])
	}

	if parsedMetrics["livetemplate_connections_active"] != 2 {
		t.Errorf("Expected 2 active connections, got %f", parsedMetrics["livetemplate_connections_active"])
	}

	if parsedMetrics["livetemplate_connections_rejected_total"] != 5 {
		t.Errorf("Expected 5 rejected connections, got %f", parsedMetrics["livetemplate_connections_rejected_total"])
	}

	if parsedMetrics["livetemplate_broadcasts_sent_total"] != 1 {
		t.Errorf("Expected 1 broadcast sent, got %f", parsedMetrics["livetemplate_broadcasts_sent_total"])
	}
}

func TestPrometheusExporter_NoLimitsGetter(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)

	// Create exporter without limits getter
	exporter := NewPrometheusExporter(metrics, nil)

	var buf bytes.Buffer
	err := exporter.WriteMetrics(&buf)
	if err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	output := buf.String()

	// Should not have connection limit metrics
	if strings.Contains(output, "livetemplate_connections_rejected_total") {
		t.Error("Should not export rejected connections without limits getter")
	}

	// Should still have other metrics
	if !strings.Contains(output, "livetemplate_connections_active") {
		t.Error("Should still export active connections")
	}
}

func TestPrometheusExporter_Snapshot(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)

	// Set some values
	metrics.ActionProcessed()
	metrics.ActionProcessed()
	metrics.ActionProcessed()
	metrics.TemplateExecuted(15 * time.Millisecond)
	metrics.ConnectionAdded()
	metrics.ConnectionAdded()
	metrics.GroupCreated()

	limitsGetter := &mockLimitsGetter{
		activeConnections:   2,
		connectionsRejected: 3,
	}

	exporter := NewPrometheusExporter(metrics, limitsGetter)
	snapshot := exporter.Snapshot()

	// Verify snapshot values
	if snapshot.ActionsProcessed != 3 {
		t.Errorf("Expected 3 actions, got %d", snapshot.ActionsProcessed)
	}

	if snapshot.TemplatesExecuted != 1 {
		t.Errorf("Expected 1 template execution, got %d", snapshot.TemplatesExecuted)
	}

	if snapshot.ActiveConnections != 2 {
		t.Errorf("Expected 2 active connections, got %d", snapshot.ActiveConnections)
	}

	if snapshot.ActiveGroups != 1 {
		t.Errorf("Expected 1 active group, got %d", snapshot.ActiveGroups)
	}

	if snapshot.ConnectionsRejected != 3 {
		t.Errorf("Expected 3 rejected connections, got %d", snapshot.ConnectionsRejected)
	}
}

func TestPrometheusExporter_HistogramQuantiles(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)

	// Record various durations
	durations := []time.Duration{
		1 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
		20 * time.Millisecond,
		25 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, d := range durations {
		metrics.TemplateExecuted(d)
	}

	exporter := NewPrometheusExporter(metrics, nil)

	var buf bytes.Buffer
	err := exporter.WriteMetrics(&buf)
	if err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	output := buf.String()

	// Verify quantiles are in seconds (not milliseconds)
	// Should contain decimal values like 0.015000 (15ms in seconds)
	if !strings.Contains(output, "0.") {
		t.Error("Histogram values should be in seconds (decimal format)")
	}

	// Verify all quantiles are present
	if !strings.Contains(output, `{quantile="0.5"}`) {
		t.Error("Missing p50 quantile")
	}
	if !strings.Contains(output, `{quantile="0.9"}`) {
		t.Error("Missing p90 quantile")
	}
	if !strings.Contains(output, `{quantile="0.95"}`) {
		t.Error("Missing p95 quantile")
	}
	if !strings.Contains(output, `{quantile="0.99"}`) {
		t.Error("Missing p99 quantile")
	}
}

func TestPrometheusExporter_MetricNaming(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)
	exporter := NewPrometheusExporter(metrics, nil)

	names := exporter.GetMetricNames()

	// Verify all names follow Prometheus conventions
	for _, name := range names {
		// Should have namespace prefix
		if !strings.HasPrefix(name, "livetemplate_") {
			t.Errorf("Metric %s missing namespace prefix", name)
		}

		// Should use underscores (not hyphens or camelCase)
		if strings.Contains(name, "-") {
			t.Errorf("Metric %s uses hyphens instead of underscores", name)
		}

		// Counters should end with _total
		if strings.Contains(name, "processed") || strings.Contains(name, "executed") ||
			strings.Contains(name, "built") || strings.Contains(name, "diffed") ||
			strings.Contains(name, "sent") || strings.Contains(name, "rejected") ||
			strings.Contains(name, "errors") {
			if !strings.HasSuffix(name, "_total") {
				t.Errorf("Counter metric %s should end with _total", name)
			}
		}

		// Duration metrics should use seconds
		if strings.Contains(name, "duration") {
			if !strings.Contains(name, "_seconds") {
				t.Errorf("Duration metric %s should use _seconds suffix", name)
			}
		}
	}
}

func TestPrometheusExporter_EmptyMetrics(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)
	exporter := NewPrometheusExporter(metrics, nil)

	var buf bytes.Buffer
	err := exporter.WriteMetrics(&buf)
	if err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	output := buf.String()

	// Should still produce valid output with zero values
	if output == "" {
		t.Error("Output should not be empty")
	}

	// Verify zero values are exported
	parsedMetrics := ParsePrometheusMetrics(output)
	if parsedMetrics["livetemplate_actions_processed_total"] != 0 {
		t.Error("Empty metrics should have zero values")
	}
}

func TestPrometheusExporter_ConcurrentAccess(t *testing.T) {
	logger := slog.Default()
	metrics := NewMetrics(logger)
	exporter := NewPrometheusExporter(metrics, nil)

	// Simulate concurrent scrapes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			var buf bytes.Buffer
			_ = exporter.WriteMetrics(&buf)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or produce corrupted output
}

func TestParsePrometheusMetrics(t *testing.T) {
	input := `# HELP test_metric A test metric
# TYPE test_metric counter
test_metric 42

# HELP another_metric Another metric
# TYPE another_metric gauge
another_metric 123

test_metric_with_labels{label="value"} 99
`

	metrics := ParsePrometheusMetrics(input)

	if metrics["test_metric"] != 42 {
		t.Errorf("Expected test_metric=42, got %f", metrics["test_metric"])
	}

	if metrics["another_metric"] != 123 {
		t.Errorf("Expected another_metric=123, got %f", metrics["another_metric"])
	}

	if metrics["test_metric_with_labels{label=\"value\"}"] != 99 {
		t.Errorf("Expected metric with labels=99, got %f", metrics["test_metric_with_labels{label=\"value\"}"])
	}
}
