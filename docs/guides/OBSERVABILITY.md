# LiveTemplate Observability Guide

## Overview

Observability comes from two systems that work together:

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

## Metrics

LiveTemplate automatically tracks operational metrics internally. These metrics are exposed via the public `MetricsHandler()` method on any LiveTemplate handler.

### Prometheus Export

```go
tmpl := livetemplate.Must(livetemplate.New("myapp",
    livetemplate.WithDevMode(false),
))
handler := tmpl.Handle(controller, livetemplate.AsState(&State{}))

mux := http.NewServeMux()
mux.Handle("/", handler)
mux.Handle("/metrics", handler.MetricsHandler()) // Prometheus text format
```

### Available Metrics

> **Breaking change in v0.11.0:** The `livetemplate_broadcasts_*` metric family was renamed to `livetemplate_publishes_*` to reflect the post-v0.10.0 `ctx.Publish` API. The `BroadcastFailures` alert is now `PublishFailures`. See [Metric Migration (v0.10.x → v0.11.0)](#metric-migration-v010x--v0110) below.

**Counters:**
- `livetemplate_actions_processed_total`
- `livetemplate_templates_executed_total`
- `livetemplate_trees_built_total`
- `livetemplate_trees_diffed_total`
- `livetemplate_publishes_sent_total` (peer-fan-out dispatches, counted once per receiving connection — covers `ctx.Publish`, `Session.TriggerAction`, and cross-instance group/topic re-fan-out; counted at enqueue time, so a downstream slow-client close can still drop the resulting send)
- `livetemplate_errors_total`
- `livetemplate_connections_rejected_total`
- `livetemplate_websocket_buffer_full_total`
- `livetemplate_websocket_slow_client_closes_total`
- `livetemplate_websocket_write_errors_total`
- `livetemplate_full_tree_sends_total`
- `livetemplate_dynamics_only_sends_total`
- `livetemplate_fingerprint_mismatches_total`

**Gauges:**
- `livetemplate_connections_active`
- `livetemplate_groups_active`
- `livetemplate_websocket_send_buffer_size`

**Summaries (with quantiles p50/p90/p95/p99):**
- `livetemplate_template_duration_seconds`
- `livetemplate_build_duration_seconds`
- `livetemplate_diff_duration_seconds`
- `livetemplate_action_duration_seconds`
- `livetemplate_update_payload_bytes`

### Metric Migration (v0.10.x → v0.11.0)

v0.11.0 renames the broadcast metric family to reflect the new `ctx.Publish`/`ctx.Subscribe` API. There is no dual-emit period — operators must update dashboards, recording rules, and alert configurations in lockstep with the deploy.

**Renamed metrics:**

| v0.10.x | v0.11.0 |
|---|---|
| `livetemplate_broadcasts_sent_total` | `livetemplate_publishes_sent_total` |

> **Note:** From v0.11.0 through v0.15.0, `livetemplate_publishes_sent_total` (and its predecessor `livetemplate_broadcasts_sent_total`) always reported 0 — the counter was defined but never incremented by any production call site (issue #432). The call site is wired in the first release published after v0.15.0; dashboards and alerts on this metric only produce meaningful data on that version or later. See the CHANGELOG "Fixed" entry referencing #432 for the exact release.

The `livetemplate_websocket_dispatch_dropped_total` name is unchanged; only its help text was updated to use "publish dispatch" instead of "broadcast dispatch".

**Renamed alerts:**

| v0.10.x | v0.11.0 |
|---|---|
| `BroadcastFailures` | `PublishFailures` |

**Quick migration for Prometheus alert rules / Grafana dashboards** (sed one-liners against your YAML/JSON):

```bash
sed -i 's/livetemplate_broadcasts_sent_total/livetemplate_publishes_sent_total/g' /path/to/alerts.yml
sed -i 's/BroadcastFailures/PublishFailures/g' /path/to/alerts.yml
sed -i 's/broadcast_errors_per_minute/publish_errors_per_minute/g' /path/to/alerts.yml
```

**Programmatic Go callers** (internal to livetemplate or out-of-tree forks) using the unexported metric API see two corresponding renames:

| v0.10.x | v0.11.0 |
|---|---|
| `(*observe.Metrics).BroadcastSent()` | `(*observe.Metrics).PublishSent()` |
| `MetricsSnapshot.BroadcastsSent` | `MetricsSnapshot.PublishesSent` |

External users importing the public `MetricsHandler()` API are unaffected.

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
    "net/http"
    "os"

    "github.com/livetemplate/livetemplate"
)

func main() {
    // Configure structured logging (production: JSON, dev: Text)
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })))

    tmpl := livetemplate.Must(livetemplate.New("myapp"))
    handler := tmpl.Handle(controller, livetemplate.AsState(&State{}))

    mux := http.NewServeMux()
    mux.Handle("/", handler)
    mux.Handle("/metrics", handler.MetricsHandler()) // Prometheus endpoint

    http.ListenAndServe(":8080", mux)
}
```

> **Note:** The `internal/observe` package is internal to the library and cannot be imported by external applications (per Go's `internal/` visibility rules). Use the public `MetricsHandler()` API shown above for Prometheus export, and configure `log/slog` at application startup for structured logging.

## Log Levels

- **DEBUG**: Tree building, diffing, rendering details
- **INFO**: Template parsing, actions received, WebSocket lifecycle, publishes, metrics
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

# Publish (peer fan-out) failures
  - name: PublishFailures
    condition: publish_errors_per_minute > 5
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

## Request-ID Correlation

LiveTemplate does not include built-in request-ID middleware. For request tracing and correlation, use a standard middleware from your HTTP router or an OpenTelemetry instrumentation library:

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// Wrap your handler with OpenTelemetry instrumentation
mux.Handle("/", otelhttp.NewHandler(handler, "livetemplate"))
```

Alternatively, add a simple request-ID middleware:

```go
// import "github.com/google/uuid"

type ctxKey struct{}

func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = uuid.NewString()
        }
        w.Header().Set("X-Request-ID", id)
        ctx := context.WithValue(r.Context(), ctxKey{}, id)
        slog.InfoContext(ctx, "request", slog.String("request_id", id), slog.String("path", r.URL.Path))
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Future Enhancements

- [ ] OpenTelemetry trace integration
- [ ] Custom metric labels/dimensions
- [ ] Log sampling for high-traffic scenarios
- [ ] Performance profiling integration (pprof)

## Related Documentation

- [ARCHITECTURE.md](../design/ARCHITECTURE.md) - System architecture overview
- [internal/observe/](../../internal/observe/) - Package implementation
- [Go slog documentation](https://pkg.go.dev/log/slog) - Standard library reference
