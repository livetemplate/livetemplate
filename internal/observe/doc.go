// Package observe provides operational metrics for LiveTemplate.
//
// # Metrics
//
// Track operational metrics with automatic periodic emission:
//
//	metrics := observe.NewMetrics(slog.Default())
//	go metrics.EmitPeriodically(60 * time.Second)
//
//	metrics.ActionProcessed()
//	metrics.TemplateExecuted(5 * time.Millisecond)
//
// Metrics are emitted as structured logs every interval.
//
// # Prometheus Exporter
//
// Export metrics in Prometheus text format for scraping:
//
//	exporter := observe.NewPrometheusExporter(metrics, limits)
//	http.Handle("/metrics", exporter)
//
// # Logging
//
// LiveTemplate uses Go's standard log/slog package directly for all structured
// logging. See the slog package documentation for configuration options.
//
// See OBSERVABILITY.md in docs/ for complete guide.
package observe
