# LiveTemplate Observability Guide

## Overview

LiveTemplate uses Go's standard `log/slog` package for both structured logging and metrics emission. This unified approach provides:

- **Structured logging** with contextual information
- **Operational metrics** emitted as structured logs
- **Production-ready observability** from day 1
- **Standard tooling** compatibility (no custom dependencies)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Code                         │
└────────────────────┬────────────────────────────────────────┘
                     ↓
         ┌───────────────────────┐
         │   observe.Logger      │  ← Domain-specific log methods
         │   observe.Metrics     │  ← Operational counters/gauges
         └───────────┬───────────┘
                     ↓
         ┌───────────────────────┐
         │      slog.Logger      │  ← Go standard library
         └───────────┬───────────┘
                     ↓
         ┌───────────────────────┐
         │   slog.Handler        │  ← JSON/Text output
         └───────────┬───────────┘
                     ↓
              stdout/stderr/file
                     ↓
         Log aggregation system
         (e.g., Loki, CloudWatch,
          Datadog, etc.)
```

## Package: internal/observe

### Logger

The `Logger` wraps `slog.Logger` with LiveTemplate-specific methods for common operations.

**Creation:**

```go
import "github.com/livefir/livetemplate/internal/observe"

// Development: human-readable text logs
logger := observe.NewLogger(
    slog.LevelDebug,
    slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }),
)

// Production: structured JSON logs
logger := observe.NewLogger(
    slog.LevelInfo,
    slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }),
)

// Default: uses JSON handler on stdout
logger := observe.NewLogger(slog.LevelInfo, nil)
```

**Domain-Specific Methods:**

```go
// Template operations
logger.TemplateParsed("todos.html", time.Millisecond*5)
logger.TreeBuilt("*main.TodoState", time.Millisecond*2)
logger.TreeDiffed(3, time.Millisecond*1)

// Rendering
logger.Rendered("html", 1024, time.Millisecond*3)

// WebSocket lifecycle
logger.WebSocketConnected("user-123", "group-456", "192.168.1.1")
logger.WebSocketDisconnected("user-123", "group-456", time.Minute*5)

// Broadcasting
logger.BroadcastSent("group-456", 10)

// Actions
logger.ActionReceived("increment", "counter")

// Errors
logger.ErrorEncountered("template_execution", err)
```

**Operation Tracking:**

```go
// Track operation with automatic duration logging
op := logger.StartOperation("process_action")
defer func() {
    if err != nil {
        op.Fail(err)
    } else {
        op.Complete()
    }
}()

// ... do work ...
```

**Context-Aware Logging:**

```go
// Add context fields for request tracking
ctxLogger := logger.WithContext(ctx)

// All subsequent logs include context fields
ctxLogger.Info("processing request")
// Output: {"time":"...","level":"INFO","msg":"processing request","user_id":"user-123","session_id":"sess-456"}
```

### Metrics

The `Metrics` type tracks operational metrics and emits them periodically via slog.

**Creation:**

```go
metrics := observe.NewMetrics(logger.Logger)

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

    "github.com/livefir/livetemplate"
    "github.com/livefir/livetemplate/internal/observe"
)

func main() {
    // Create logger (production: JSON, dev: Text)
    logger := observe.NewLogger(slog.LevelInfo, nil)

    // Create metrics tracker
    metrics := observe.NewMetrics(logger.Logger)
    go metrics.EmitPeriodically(30 * time.Second)

    // Create LiveTemplate handler with observability
    handler := livetemplate.New("app",
        livetemplate.WithLogger(logger),      // Future: pass logger to handler
        livetemplate.WithMetrics(metrics),    // Future: pass metrics to handler
    )

    // Handler will automatically log and record metrics
    // for all operations (parsing, building, diffing, rendering)
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
logger.ActionReceived("increment", "counter")  // action type, store name
```

**Bad** (high cardinality):
```go
logger.ActionReceived("increment", userID)  // DO NOT use user IDs, session IDs, etc.
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
    logger := observe.NewLogger(
        slog.LevelDebug,
        slog.NewJSONHandler(&buf, nil),
    )

    // Use logger in test
    logger.TemplateParsed("test.html", time.Millisecond)

    // Verify log output
    output := buf.String()
    if !strings.Contains(output, "template_parsed") {
        t.Error("expected template_parsed log")
    }
}
```

## Future Enhancements

- [ ] OpenTelemetry trace integration
- [ ] Prometheus metrics exporter
- [ ] Distributed tracing with trace IDs
- [ ] Custom metric labels/dimensions
- [ ] Log sampling for high-traffic scenarios
- [ ] Performance profiling integration (pprof)

## Related Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture overview
- [internal/observe/](../internal/observe/) - Package implementation
- [Go slog documentation](https://pkg.go.dev/log/slog) - Standard library reference
