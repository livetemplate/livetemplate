# Observability Example

This example demonstrates how to add production-ready observability to a LiveTemplate application using the v1.0 observability features.

## What's Included

- **Structured Logging**: Using Go's standard `log/slog` package
- **Operational Metrics**: Counters, gauges, and histograms with percentile tracking
- **Development vs Production**: Different log formats for different environments

## Features Demonstrated

### 1. Structured Logging

All template operations are automatically logged:
- Template parsing
- Tree building
- Tree diffing
- WebSocket connections
- Action processing
- Broadcast events

### 2. Operational Metrics

Automatically tracked metrics:
- **Counters**: `templates_executed`, `actions_processed`, `broadcasts_sent`
- **Gauges**: `active_connections`, `active_groups`
- **Histograms**: `template_duration_ms`, `build_duration_ms`, `diff_duration_ms` (with p50, p95, p99)

### 3. Environment-Aware Output

- **Development**: Human-readable text format with DEBUG level
- **Production**: JSON format for log aggregators with INFO level

## Running the Example

### Development Mode (Text Logs)

```bash
# Uses defaults: LVT_LOG_LEVEL=info, LVT_METRICS_ENABLED=true
go run main.go

# Or explicitly set log level to debug
LVT_LOG_LEVEL=debug go run main.go
```

Expected output:
```
time=2025-10-31T12:00:00.000Z level=INFO msg="LiveTemplate Counter Server starting with observability enabled" log_level=info metrics_enabled=true dev_mode=false
time=2025-10-31T12:00:00.100Z level=INFO msg="Server starting" port=8080 url=http://localhost:8080
time=2025-10-31T12:00:00.100Z level=INFO msg="Metrics will be emitted every 30 seconds"
```

### Production Mode (JSON Logs)

```bash
# Production environment with structured JSON logging
ENV=production LVT_LOG_LEVEL=info go run main.go

# Production with warnings only
ENV=production LVT_LOG_LEVEL=warn go run main.go
```

Expected output:
```json
{"time":"2025-10-31T12:00:00Z","level":"INFO","msg":"LiveTemplate Counter Server starting with observability enabled","log_level":"info","metrics_enabled":true,"dev_mode":false}
{"time":"2025-10-31T12:00:00Z","level":"INFO","msg":"Server starting","port":"8080","url":"http://localhost:8080"}
{"time":"2025-10-31T12:00:00Z","level":"INFO","msg":"Metrics will be emitted every 30 seconds"}
```

## Usage

1. Start the server:
   ```bash
   go run main.go
   ```

2. Open http://localhost:8080 in your browser

3. Click the increment/decrement buttons

4. Watch the console for:
   - Action processing logs
   - WebSocket connection events
   - Periodic metrics snapshots (every 30s)

## Example Log Output

### When a User Connects

```
time=2025-10-31T12:00:05.123Z level=INFO msg="WebSocket connected" connection_id=abc123
time=2025-10-31T12:00:05.125Z level=INFO msg="Template parsed" template=counter duration_ms=0.5
time=2025-10-31T12:00:05.127Z level=INFO msg="Tree built" duration_ms=1.2
```

### When a User Clicks Increment

```
time=2025-10-31T12:00:10.456Z level=INFO msg="Action received" action=increment
time=2025-10-31T12:00:10.458Z level=INFO msg="Tree diffed" duration_ms=0.8
time=2025-10-31T12:00:10.459Z level=INFO msg="Broadcast sent" group=default recipients=1
```

### Periodic Metrics (Every 30s)

```
time=2025-10-31T12:00:30.000Z level=INFO msg="Metrics snapshot" templates_executed=42 template_duration_p50=1.2 template_duration_p95=3.5 template_duration_p99=5.0 active_connections=3 actions_processed=156
```

## Comparison with Basic Example

### Basic Example (examples/counter)

```go
func main() {
    state := &CounterState{...}
    tmpl := livetemplate.New("counter")
    http.Handle("/", tmpl.Handle(state))
    http.ListenAndServe(":8080", nil)
}
```

### With Observability (this example)

```go
func main() {
    // Add logging
    logger := observe.NewLogger(slog.NewJSONHandler(...))

    // Add metrics
    metrics := observe.NewMetrics()
    metrics.StartEmission(logger, 30)
    defer metrics.StopEmission()

    // Same application code
    state := &CounterState{...}
    tmpl := livetemplate.New("counter")
    http.Handle("/", tmpl.Handle(state))
    http.ListenAndServe(":8080", nil)
}
```

**Result**: Automatic logging and metrics with <0.1% overhead!

## Performance Impact

- **Overhead**: <0.1% of request time
- **Memory**: Negligible (metrics stored in-memory with bounded size)
- **No external dependencies**: Uses only Go standard library

## Integration with Monitoring Systems

The JSON log output can be integrated with:

- **Log Aggregators**: Elasticsearch, Splunk, CloudWatch Logs
- **APM Tools**: Datadog, New Relic, Application Insights
- **Metrics Systems**: Prometheus (via log scraping), Graphite

Example Elasticsearch query:
```json
{
  "query": {
    "bool": {
      "must": [
        { "match": { "msg": "Action received" }},
        { "range": { "time": { "gte": "now-1h" }}}
      ]
    }
  }
}
```

## Customization

### Custom Log Levels

```go
handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelWarn, // Only warnings and errors
})
```

### Custom Metrics Interval

```go
metrics.StartEmission(logger, 60) // Emit every 60 seconds
```

### Custom slog Handler

```go
// Use any slog-compatible handler
handler := myCustomHandler{}
logger := observe.NewLogger(handler)
```

## See Also

- [OBSERVABILITY.md](../../docs/OBSERVABILITY.md) - Complete observability guide
- [MIGRATION.md](../../docs/MIGRATION.md) - Migration guide to v1.0
- [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) - Internal architecture
