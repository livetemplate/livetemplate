# LiveTemplate Observability Guide

## Overview

LiveTemplate provides production-ready observability through two complementary systems:

- **Structured logging** via Go's standard `log/slog` package (used directly throughout the codebase)
- **Operational metrics** via `internal/observe` package (counters, gauges, histograms with Prometheus export)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Code                         │
└──────────┬──────────────────────────┬───────────────────────┘
           ↓                          ↓
┌────────────────────┐   ┌───────────────────────┐
│   log/slog         │   │   observe.Metrics     │  ← Operational counters/gauges
│   (structured logs)│   │   PrometheusExporter  │  ← /metrics endpoint
└──────────┬─────────┘   └───────────┬───────────┘
           ↓                          ↓
┌────────────────────┐   ┌───────────────────────┐
│   slog.Handler     │   │   Prometheus scraper   │
│   (JSON/Text)      │   │   or slog emission     │
└──────────┬─────────┘   └───────────────────────┘
           ↓
    stdout/stderr/file
           ↓
    Log aggregation system
    (e.g., Loki, CloudWatch,
     Datadog, etc.)
```

## Structured Logging

LiveTemplate uses Go's standard `log/slog` package directly for all structured logging. No wrapper is needed.

**Configuration:**

```go
import "log/slog"

// Development: human-readable text logs
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})))

// Production: structured JSON logs
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})))
```

All LiveTemplate components log using `slog.Info()`, `slog.Warn()`, `slog.Error()`, and `slog.Debug()` with structured attributes. Configure the default logger at application startup to control output format and level.

## Package: internal/observe

### Metrics

The `Metrics` type tracks operational metrics and emits them periodically via slog.

**Creation:**

```go
metrics := observe.NewMetrics(slog.Default())

// Start periodic emission (every 30 seconds)
go metrics.EmitPeriodically(30 * time.Second)
```

**Recording Metrics:**

```go
// Counters (increment only)
metrics.ActionProcessed()
metrics.TemplateExecuted()
metrics.TreeBuilt()
metrics.TreeDiffed()
metrics.HTMLRendered()
metrics.JSONRendered()
metrics.BroadcastSent()
metrics.WebSocketConnected()
metrics.WebSocketDisconnected()

// Gauges (set absolute value)
metrics.SetActiveConnections(10)
metrics.SetActiveGroups(5)

// Durations (histograms for percentile calculation)
metrics.RecordTemplateDuration(time.Millisecond * 5)
metrics.RecordTreeBuildDuration(time.Millisecond * 2)
metrics.RecordDiffDuration(time.Millisecond * 1)
```

**Emitted Metric Format:**

```json
{
  "time": "2025-10-31T12:34:56Z",
  "level": "INFO",
  "msg": "metrics",
  "actions_processed": 1523,
  "templates_executed": 1523,
  "trees_built": 1523,
  "trees_diffed": 1321,
  "html_rendered": 202,
  "json_rendered": 1321,
  "broadcasts_sent": 45,
  "websocket_connected": 3,
  "websocket_disconnected": 1,
  "active_connections": 10,
  "active_groups": 5,
  "template_duration_p50": 3,
  "template_duration_p95": 8,
  "template_duration_p99": 12,
  "tree_build_duration_p50": 2,
  "tree_build_duration_p95": 5,
  "tree_build_duration_p99": 9,
  "diff_duration_p50": 1,
  "diff_duration_p95": 3,
  "diff_duration_p99": 7
}
```

## Log Output Formats

### Development (Text Handler)

```
time=2025-10-31T12:34:56.789Z level=INFO msg=template_parsed template=todos.html duration_ms=5
time=2025-10-31T12:34:56.790Z level=DEBUG msg=tree_built data_type=*main.TodoState duration_ms=2
time=2025-10-31T12:34:56.791Z level=DEBUG msg=tree_diffed changes=3 duration_ms=1
time=2025-10-31T12:34:56.792Z level=DEBUG msg=rendered format=html bytes=1024 duration_ms=3
time=2025-10-31T12:34:56.793Z level=INFO msg=action_received action=increment store=counter
```

### Production (JSON Handler)

```json
{"time":"2025-10-31T12:34:56.789Z","level":"INFO","msg":"template_parsed","template":"todos.html","duration_ms":5}
{"time":"2025-10-31T12:34:56.790Z","level":"DEBUG","msg":"tree_built","data_type":"*main.TodoState","duration_ms":2}
{"time":"2025-10-31T12:34:56.791Z","level":"DEBUG","msg":"tree_diffed","changes":3,"duration_ms":1}
{"time":"2025-10-31T12:34:56.792Z","level":"DEBUG","msg":"rendered","format":"html","bytes":1024,"duration_ms":3}
{"time":"2025-10-31T12:34:56.793Z","level":"INFO","msg":"action_received","action":"increment","store":"counter"}
```

## Integration Example

```go
package main

import (
    "log/slog"
    "os"
    "time"

    "github.com/livetemplate/livetemplate/internal/observe"
)

func main() {
    // Configure structured logging (production: JSON, dev: Text)
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })))

    // Create metrics tracker
    metrics := observe.NewMetrics(slog.Default())
    go metrics.EmitPeriodically(30 * time.Second)

    // All LiveTemplate components will use slog for logging
    // and metrics for operational counters/gauges
}
```

## Log Levels

- **DEBUG**: Tree building, diffing, rendering details
- **INFO**: Template parsing, actions received, WebSocket lifecycle, broadcasts, metrics
- **WARN**: Recoverable errors, retries, degraded performance
- **ERROR**: Operation failures, unrecoverable errors

**Recommendation:**
- Development: `DEBUG`
- Staging: `INFO`
- Production: `INFO` (switch to `DEBUG` for troubleshooting)

## Metric Collection Best Practices

### 1. Percentiles over Averages

Metrics use histograms with p50/p95/p99 percentiles instead of averages because:
- **Outliers don't skew data**: p95 shows "95% of requests are this fast or faster"
- **SLA compliance**: "99% of requests under 100ms" is more useful than "average 50ms"
- **Tail latency visibility**: p99 exposes slow edge cases

### 2. Emission Frequency

```go
// Low traffic (<100 req/sec): emit every 60s
go metrics.EmitPeriodically(60 * time.Second)

// Medium traffic (100-1000 req/sec): emit every 30s
go metrics.EmitPeriodically(30 * time.Second)

// High traffic (>1000 req/sec): emit every 10s
go metrics.EmitPeriodically(10 * time.Second)
```

### 3. Metric Cardinality

**Good** (low cardinality):
```go
slog.Info("Action received",
    slog.String("action", "increment"),
    slog.String("store", "counter"))
```

**Bad** (high cardinality):
```go
slog.Info("Action received",
    slog.String("action", "increment"),
    slog.String("user_id", userID))  // DO NOT use user IDs, session IDs, etc. in metric labels
```

High-cardinality fields (user IDs, session IDs) should only appear in individual log events, not in metric labels.

## Alerting Patterns

### Key Metrics to Monitor

```yaml
# High error rate
alerts:
  - name: HighErrorRate
    condition: error_logs_per_minute > 10
    severity: warning

  - name: CriticalErrorRate
    condition: error_logs_per_minute > 50
    severity: critical

# Slow template execution
  - name: SlowTemplateExecution
    condition: template_duration_p95 > 100  # ms
    severity: warning

  - name: VerySlowTemplateExecution
    condition: template_duration_p99 > 500  # ms
    severity: critical

# WebSocket connection churn
  - name: HighConnectionChurn
    condition: websocket_disconnected_per_minute > 100
    severity: warning

# Broadcast failures
  - name: BroadcastFailures
    condition: broadcast_errors_per_minute > 5
    severity: critical
```

## Log Aggregation Integration

### Loki (Grafana)

```promql
# Count errors by component
sum by (component) (count_over_time({app="livetemplate",level="ERROR"}[5m]))

# p95 template duration
quantile_over_time(0.95, {app="livetemplate",msg="template_parsed"} | json | unwrap duration_ms [5m])

# Active connections over time
avg_over_time({app="livetemplate",msg="metrics"} | json | unwrap active_connections [1m])
```

### CloudWatch Logs Insights

```sql
-- Error count by component
fields @timestamp, component, error
| filter level = "ERROR"
| stats count() by component

-- p95 template duration
fields @timestamp, duration_ms
| filter msg = "template_parsed"
| stats pct(duration_ms, 95) as p95

-- Active connections
fields @timestamp, active_connections
| filter msg = "metrics"
| stats avg(active_connections) by bin(1m)
```

### Datadog

```
# Error rate
sum:livetemplate.errors{*}.as_count()

# Template duration p95
avg:livetemplate.template.duration{*} by {template}

# Active connections
avg:livetemplate.connections.active{*}
```

## Performance Overhead

The observability system is designed for minimal overhead:

- **Structured logging**: ~1-2μs per log (JSON encoding)
- **Metric recording**: ~50-100ns per counter increment
- **Histogram recording**: ~200-500ns per duration (percentile calculation deferred to emission)
- **Periodic emission**: ~1-5ms every 30s (negligible amortized cost)

**Total overhead**: <0.1% of request processing time for typical workloads.

## Testing with Observability

```go
func TestWithObservability(t *testing.T) {
    // Create test logger that captures output
    var buf bytes.Buffer
    slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })))

    // Trigger operations that log
    slog.Info("template_parsed",
        slog.String("template", "test.html"),
        slog.Duration("duration", time.Millisecond))

    // Verify log output
    output := buf.String()
    if !strings.Contains(output, "template_parsed") {
        t.Error("expected template_parsed log")
    }
}
```

## Future Enhancements

- [ ] OpenTelemetry trace integration
- [ ] Distributed tracing with trace IDs
- [ ] Custom metric labels/dimensions
- [ ] Log sampling for high-traffic scenarios
- [ ] Performance profiling integration (pprof)

## Related Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture overview
- [internal/observe/](../internal/observe/) - Package implementation
- [Go slog documentation](https://pkg.go.dev/log/slog) - Standard library reference
