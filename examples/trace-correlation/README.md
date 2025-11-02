## Trace Correlation Example

This example demonstrates **distributed tracing** and **log correlation** using trace IDs. In production systems with multiple services and high request volume, trace IDs are essential for debugging and understanding request flows.

## Why Trace IDs Matter

Without trace IDs, debugging production issues is nearly impossible:

❌ **Without Trace IDs:**
```
2025-11-02 10:00:01 Processing action: increment
2025-11-02 10:00:01 Processing action: decrement
2025-11-02 10:00:01 Counter incremented to 5
2025-11-02 10:00:01 Counter decremented to 3
```
**Problem**: Which increment goes with which decrement? Impossible to tell with concurrent requests.

✅ **With Trace IDs:**
```
2025-11-02 10:00:01 [trace_id=a1b2c3d4] Processing action: increment
2025-11-02 10:00:01 [trace_id=e5f6g7h8] Processing action: decrement
2025-11-02 10:00:01 [trace_id=a1b2c3d4] Counter incremented to 5
2025-11-02 10:00:01 [trace_id=e5f6g7h8] Counter decremented to 3
```
**Solution**: Each request has a unique ID. Easy to correlate all logs for a single request.

## Running the Example

### Basic Usage

```bash
go run main.go
```

The server starts on port 8080. Watch the logs to see trace IDs in action.

### With Structured JSON Logging

```bash
ENV=production go run main.go
```

Produces JSON logs suitable for log aggregators (Elasticsearch, Splunk, Datadog):

```json
{"time":"2025-11-02T10:00:00Z","level":"INFO","msg":"Handling request","trace_id":"a1b2c3d4e5f6g7h8","method":"GET","path":"/"}
```

### With Debug Logging

```bash
LVT_LOG_LEVEL=debug go run main.go
```

Shows additional debug logs, all correlated by trace ID.

## Testing Trace Correlation

### 1. Browser Test (Automatic Trace IDs)

```bash
# Start server
go run main.go

# Open browser
open http://localhost:8080
```

**Expected**: Every page load and action gets a unique trace ID. Check your terminal logs.

### 2. cURL Test (View Trace ID)

```bash
# Get trace ID from response header
curl -v http://localhost:8080/health
```

**Expected output:**
```
< HTTP/1.1 200 OK
< Content-Type: application/json
< X-Trace-ID: a1b2c3d4e5f6g7h8
<
{"status":"healthy","timestamp":"2025-11-02T10:00:00Z"}
```

The trace ID appears in:
1. Response header (`X-Trace-ID`)
2. Server logs (correlated with this request)

### 3. cURL Test (Custom Trace ID)

```bash
# Send custom trace ID for cross-service tracing
curl -H 'X-Trace-ID: my-custom-trace-123' http://localhost:8080/health
```

**Expected**: Server preserves your trace ID and uses it in all logs.

**Use case**: When Service A calls Service B, it passes its trace ID. This enables end-to-end tracing across your entire system.

### 4. Load Test (Multiple Concurrent Requests)

```bash
# Generate concurrent requests with different trace IDs
for i in {1..10}; do
  curl -H "X-Trace-ID: load-test-$i" http://localhost:8080/health &
done
wait
```

**Expected**: Logs show 10 different trace IDs, each correlating all logs for that request.

## How It Works

### Trace ID Generation

Trace IDs are 16-character hexadecimal strings (64 bits):

```go
traceID := observe.GenerateTraceID()
// Example: "a1b2c3d4e5f6g7h8"
```

**Characteristics:**
- Globally unique (collision probability: ~1 in 18 quintillion)
- URL-safe (only hex characters)
- Compact (16 bytes)
- High entropy (cryptographically random)

### Trace ID Flow

```
Client Request
     │
     ├─> 1. TraceMiddleware extracts or generates trace ID
     │
     ├─> 2. Trace ID added to request context
     │
     ├─> 3. Trace ID added to response header (X-Trace-ID)
     │
     ├─> 4. Handler uses observe.LoggerWithTraceID(logger, ctx)
     │
     └─> 5. All logs for this request include trace_id field
```

### Code Architecture

**Middleware (Automatic)**:
```go
// Wraps handler with automatic trace ID injection
mux.Handle("/", observe.TraceMiddleware(yourHandler))
```

**Logger Integration (Automatic)**:
```go
// Creates logger with trace ID from context
traceLogger := observe.LoggerWithTraceID(logger, r.Context())

// All logs from this logger include trace_id
traceLogger.Info("Processing request", "user_id", user.ID)
// Output: {"time":"...","level":"INFO","msg":"Processing request","trace_id":"a1b2c3d4","user_id":"user-123"}
```

**Manual Access (When Needed)**:
```go
// Get trace ID from context
traceID := observe.GetTraceID(ctx)

// Use in custom logging
log.Printf("[trace_id=%s] Custom message", traceID)
```

## Distributed Tracing (Multi-Service)

When you have multiple services, trace IDs enable end-to-end tracing:

```
User Request → Service A → Service B → Database
               ├─────────────┼──────────┘
               Same Trace ID: abc123
```

### Service A (Caller)

```go
func callServiceB(ctx context.Context) error {
    // Get trace ID from context
    traceID := observe.GetTraceID(ctx)

    // Create request to Service B
    req, _ := http.NewRequest("GET", "http://service-b/api", nil)

    // Propagate trace ID
    req.Header.Set("X-Trace-ID", traceID)

    // Make request
    resp, err := http.DefaultClient.Do(req)
    // ...
}
```

### Service B (Receiver)

```go
func main() {
    // Use TraceMiddleware - automatically extracts X-Trace-ID header
    mux.Handle("/api", observe.TraceMiddleware(apiHandler))
    http.ListenAndServe(":8080", mux)
}
```

**Result**: Same trace ID flows through both services. All logs correlated.

## Integration with Log Aggregators

### Elasticsearch

JSON logs with trace_id field enable queries like:

```json
GET /logs/_search
{
  "query": {
    "term": { "trace_id": "a1b2c3d4e5f6g7h8" }
  },
  "sort": [ { "time": "asc" } ]
}
```

Returns all logs for this request across all services, in chronological order.

### Splunk

```
index=app trace_id="a1b2c3d4e5f6g7h8" | sort _time
```

### Datadog

```
trace_id:a1b2c3d4e5f6g7h8
```

### CloudWatch Logs Insights

```
fields @timestamp, @message, trace_id
| filter trace_id = "a1b2c3d4e5f6g7h8"
| sort @timestamp asc
```

## Best Practices

### 1. Always Use TraceMiddleware

```go
// ✅ GOOD: Automatic trace ID injection
mux.Handle("/api", observe.TraceMiddleware(apiHandler))

// ❌ BAD: Missing trace IDs
mux.Handle("/api", apiHandler)
```

### 2. Use LoggerWithTraceID for Structured Logs

```go
// ✅ GOOD: Trace ID automatically included
traceLogger := observe.LoggerWithTraceID(logger, ctx)
traceLogger.Info("Processing order", "order_id", order.ID)

// ❌ BAD: Manual trace ID formatting
log.Printf("[trace_id=%s] Processing order %s", traceID, order.ID)
```

Why? Structured logs are:
- Parseable by log aggregators
- Queryable (filter by trace_id field)
- Consistent format across services

### 3. Propagate Trace IDs Across Services

```go
// ✅ GOOD: Trace ID passed to downstream service
req.Header.Set("X-Trace-ID", observe.GetTraceID(ctx))

// ❌ BAD: New trace ID for each service
// (breaks end-to-end tracing)
```

### 4. Include Trace IDs in Error Responses

```go
// ✅ GOOD: Client can correlate errors with server logs
w.Header().Set("X-Trace-ID", observe.GetTraceID(ctx))
http.Error(w, "Internal error", 500)

// Client sees: X-Trace-ID: a1b2c3d4
// Client reports: "Error with trace ID a1b2c3d4"
// You search logs: trace_id="a1b2c3d4" -> find root cause
```

### 5. Log at Service Boundaries

```go
// ✅ GOOD: Log when entering/exiting service
traceLogger.Info("Request received")
// ... process request ...
traceLogger.Info("Response sent", "status", 200)

// Enables timing analysis:
// - How long did this request take?
// - Where was time spent?
```

## Common Patterns

### Pattern 1: Correlation Across HTTP Handlers

```go
func handlerA(w http.ResponseWriter, r *http.Request) {
    logger := observe.LoggerWithTraceID(baseLogger, r.Context())
    logger.Info("Handler A started")

    // Call handler B
    handlerB(r.Context())

    logger.Info("Handler A completed")
}

func handlerB(ctx context.Context) {
    logger := observe.LoggerWithTraceID(baseLogger, ctx)
    logger.Info("Handler B processing")
    // All logs have same trace_id
}
```

### Pattern 2: Background Jobs with Trace Context

```go
// Start background job with trace context
func processAsync(ctx context.Context, data Data) {
    go func() {
        logger := observe.LoggerWithTraceID(baseLogger, ctx)
        logger.Info("Background job started")
        // Process data
        logger.Info("Background job completed")
    }()
}
```

All logs from background job have same trace ID as original request.

### Pattern 3: Database Query Tracing

```go
func queryDatabase(ctx context.Context, query string) error {
    traceID := observe.GetTraceID(ctx)
    logger := observe.LoggerWithTraceID(baseLogger, ctx)

    start := time.Now()
    rows, err := db.QueryContext(ctx, query)
    duration := time.Since(start)

    logger.Info("Database query executed",
        "query", query,
        "duration_ms", duration.Milliseconds(),
        "rows", countRows(rows),
    )
    // Log aggregator can find: all DB queries for trace_id=abc123

    return err
}
```

## Performance Considerations

### Trace ID Generation Cost

- **CPU**: ~200ns per trace ID (negligible)
- **Memory**: 16 bytes per trace ID
- **Throughput**: >5 million trace IDs/second on typical hardware

### Logging Overhead

```go
// Fast: Structured logging with trace ID
logger.Info("Message", "trace_id", traceID)  // ~1-2 microseconds

// Slow: String formatting
log.Printf("[trace_id=%s] Message", traceID)  // ~5-10 microseconds

// Slowest: JSON marshaling on every log
// (avoid this in hot paths)
```

**Recommendation**: Use structured logging (slog) with trace IDs. Cost is negligible compared to benefits.

## Troubleshooting

### Issue: Trace IDs Not Appearing in Logs

**Cause**: Not using `LoggerWithTraceID` or `WithContext`

**Fix**:
```go
// Add this
traceLogger := observe.LoggerWithTraceID(logger, ctx)
traceLogger.Info("Message")  // ✅ Includes trace_id
```

### Issue: Different Trace IDs Across Services

**Cause**: Not propagating X-Trace-ID header

**Fix**:
```go
// Service A
traceID := observe.GetTraceID(ctx)
req.Header.Set("X-Trace-ID", traceID)

// Service B
mux.Handle("/", observe.TraceMiddleware(handler))  // Auto-extracts header
```

### Issue: Trace IDs in Logs but Can't Query

**Cause**: Using unstructured logs (log.Printf)

**Fix**: Switch to structured logging (slog):
```go
// ❌ Unstructured
log.Printf("[trace_id=%s] Message", traceID)

// ✅ Structured (queryable)
logger.Info("Message", "trace_id", traceID)
```

## Production Deployment

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  ENV: "production"
  LVT_LOG_LEVEL: "info"
```

All logs will be JSON format with trace_id field.

### Log Aggregation Pipeline

```
Application → Stdout/JSON → Fluentd/Vector → Elasticsearch → Kibana
                                                           ↓
                                                  Query by trace_id
```

### Alert on Specific Trace

```
// Datadog Alert
traces.trace_id:abc123 AND status:error
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | empty | Set to `production` for JSON logging |
| `LVT_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `PORT` | `8080` | HTTP server port |

See [CONFIGURATION.md](../../docs/CONFIGURATION.md) for complete reference.

## Related Documentation

- [Observability Example](../observability/) - Metrics and structured logging
- [Graceful Shutdown Example](../graceful-shutdown/) - Production deployment patterns
- [Configuration Guide](../../docs/CONFIGURATION.md) - Environment variables

## Summary

This example demonstrates **production-grade request tracing**:

1. ✅ Automatic trace ID generation and injection
2. ✅ HTTP header propagation (X-Trace-ID)
3. ✅ Structured logging integration
4. ✅ Cross-service tracing support
5. ✅ Log aggregator compatibility

Use trace IDs in production to:
- Debug issues across distributed systems
- Measure end-to-end request latency
- Correlate errors with specific requests
- Track requests through multiple services

**Without trace IDs, production debugging is guesswork. With trace IDs, it's systematic.**
