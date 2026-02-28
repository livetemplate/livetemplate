# Proposal: Async WebSocket Message Sending

**Status:** Proposed
**Author:** Claude
**Date:** 2025-01-22
**Related Issues:** Flaky delete tests under parallel execution (e2e test failures)

## Problem Statement

Under high load with parallel test execution, WebSocket message delivery experiences blocking that causes test timeouts and potential production performance issues.

### Current Behavior

The current synchronous `Send()` implementation blocks the calling goroutine until `WriteMessage()` completes:

```go
func (c *Connection) Send(messageType int, data []byte) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.Conn == nil {
        return nil
    }

    return c.Conn.WriteMessage(messageType, data)  // BLOCKS HERE
}
```

### Root Cause

From [gorilla/websocket documentation](https://github.com/gorilla/websocket/issues/675):
> "WriteMessage will eventually block waiting for space in the operating system transmit buffer. If the receiving end isn't consuming messages fast enough, WriteMessage will block."

This blocking behavior causes cascading delays:

1. **Slow Chrome containers** (especially headless in Docker) don't consume WebSocket messages quickly
2. **WriteMessage blocks** waiting for the OS transmit buffer to have space
3. **HTTP handlers are blocked** because they call Send() synchronously
4. **Other operations are delayed** because the mutex is held during the blocked write
5. **Tests timeout** waiting for DOM updates that haven't been sent yet

### Evidence

- E2E tests pass individually but fail ~33% of the time under parallel execution (`-p 4`)
- Delete operations succeed on server-side but browser doesn't receive update
- Screenshots show deleted rows, but tests timeout waiting for them
- Full test suite fails consistently, smaller parallel runs pass
- Web search confirms this is a known issue with gorilla/websocket under load

## Proposed Solution

Implement **asynchronous message sending** using buffered channels and dedicated write goroutines per connection.

### Architecture

```go
type Connection struct {
    Conn     *websocket.Conn
    GroupID  string
    UserID   string
    Template interface{}
    Stores   interface{}
    Uploads  interface{}
    mu       sync.Mutex

    // NEW: Async sending infrastructure
    sendChan   chan *wsMessage // Buffered channel for queued messages
    done       chan struct{}    // Signal for graceful shutdown
    pumpExited chan struct{}    // Signals writePump has exited cleanly
    closeOnce  sync.Once        // Prevents double-close race conditions
}

type wsMessage struct {
    messageType int
    data        []byte
}

// Send queues message for async delivery (non-blocking)
func (c *Connection) Send(messageType int, data []byte) error {
    select {
    case c.sendChan <- &wsMessage{messageType, data}:
        return nil // Message queued successfully
    case <-c.done:
        return fmt.Errorf("connection closed")
    default:
        // Buffer full - client is too slow, close connection
        go c.Close()
        return fmt.Errorf("client too slow, closing connection")
    }
}

// writePump runs as a background goroutine per connection
// Dequeues messages from sendChan and writes to WebSocket
func (c *Connection) writePump() {
    defer func() {
        close(c.pumpExited)  // Signal that writePump has exited
        c.Close()            // Ensure connection is closed
    }()

    for {
        select {
        case msg := <-c.sendChan:
            c.mu.Lock()
            err := c.Conn.WriteMessage(msg.messageType, msg.data)
            c.mu.Unlock()

            if err != nil {
                slog.Warn("WebSocket write failed, closing connection",
                    slog.String("error", err.Error()),
                    slog.String("group_id", c.GroupID),
                    slog.String("user_id", c.UserID))
                return
            }
        case <-c.done:
            // Drain remaining messages before closing
            c.drainSendChannel()
            return
        }
    }
}

// Close closes the WebSocket connection safely
// Uses sync.Once to prevent double-close race conditions
func (c *Connection) Close() error {
    c.closeOnce.Do(func() {
        // 1. Signal writePump to stop (if not already signaled)
        select {
        case <-c.done:
            // Already closed
        default:
            close(c.done)
        }

        // 2. Wait for writePump to exit (with timeout)
        select {
        case <-c.pumpExited:
            // writePump exited cleanly
        case <-time.After(5 * time.Second):
            slog.Warn("writePump drain timeout, forcing close",
                slog.String("group_id", c.GroupID),
                slog.String("user_id", c.UserID))
        }

        // 3. Close the WebSocket connection
        c.mu.Lock()
        defer c.mu.Unlock()
        if c.Conn != nil {
            c.Conn.Close()
        }
    })
    return nil
}

// drainSendChannel attempts to send remaining queued messages
func (c *Connection) drainSendChannel() {
    for {
        select {
        case msg := <-c.sendChan:
            c.mu.Lock()
            _ = c.Conn.WriteMessage(msg.messageType, msg.data)
            c.mu.Unlock()
        default:
            return
        }
    }
}
```

### Lifecycle Management

```go
// Register adds a connection and starts the write pump
func (r *ConnectionRegistry) Register(conn *Connection, bufferSize int) {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Initialize async sending infrastructure
    conn.sendChan = make(chan *wsMessage, bufferSize) // Configurable buffer size from mountConfig
    conn.done = make(chan struct{})
    conn.pumpExited = make(chan struct{})

    // Start write pump goroutine
    go conn.writePump()

    // Add to indexes
    r.byGroup[conn.GroupID] = append(r.byGroup[conn.GroupID], conn)
    r.byUser[conn.UserID] = append(r.byUser[conn.UserID], conn)
}

// Called from mount.go when accepting WebSocket connection
func (m *mount) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // ... existing WebSocket upgrade code ...

    conn := &Connection{
        Conn:    wsConn,
        GroupID: groupID,
        UserID:  userID,
        // ...
    }

    // Pass buffer size from mount config
    m.registry.Register(conn, m.config.wsBufferSize)
}

// Unregister removes a connection and stops the write pump
// This is called when removing from registry, Close() handles the actual shutdown
func (r *ConnectionRegistry) Unregister(conn *Connection) {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Close the connection (idempotent due to sync.Once)
    conn.Close()

    // Remove from indexes
    r.removeFromIndexes(conn)
}
```

## Design Decisions

### 1. Buffer Size: 50 messages

**Rationale:**
- Small enough: Low memory overhead (~500KB per 100 connections with 10KB messages)
- Large enough: Handles burst traffic without dropping messages
- Typical usage: Most connections queue < 5 messages at a time

**Calculation:**
- 100 concurrent connections
- 50 message buffer each
- Average 10KB per message
- Total: 100 × 50 × 10KB = 50MB

### 2. Backpressure Strategy: Close Connection

When buffer is full (client too slow):

```go
default:
    // Buffer full - client is too slow, close connection
    go c.Close()
    return fmt.Errorf("client too slow, closing connection")
}
```

**Why close instead of drop/block:**
- **Better than dropping**: Client reconnects and gets fresh state
- **Better than blocking**: Returns handler to original problem
- **Matches LiveTemplate semantics**: Full state updates on reconnect
- **Fails fast**: Slow clients are identified and handled quickly

**Alternative strategies considered:**
- Drop oldest message: Client sees stale state, confusing UX
- Drop newest message: Lose important updates
- Block on full buffer: Defeats the purpose of async sending
- Expand buffer dynamically: Unbounded memory growth risk

### 3. One Goroutine Per Connection

**Cost:** ~2KB per goroutine (Go runtime overhead)
**Benefit:** Clean isolation, follows gorilla/websocket best practices

**Scaling:**
- 1,000 connections = ~2MB goroutine overhead
- 10,000 connections = ~20MB goroutine overhead
- Acceptable for most deployments

### 4. Graceful Shutdown with Channel Draining

On connection close, attempt to send remaining queued messages:

```go
func (c *Connection) drainSendChannel() {
    for {
        select {
        case msg := <-c.sendChan:
            c.mu.Lock()
            _ = c.Conn.WriteMessage(msg.messageType, msg.data)
            c.mu.Unlock()
        default:
            return
        }
    }
}
```

**Rationale:**
- Best-effort delivery of pending messages
- Prevents message loss on clean disconnects
- Non-blocking (uses `default` case)

### 5. Goroutine Lifecycle Management

**Critical Implementation Details:**

1. **Initialize all channels before starting writePump**:
   - `sendChan` - buffered channel for messages
   - `done` - signals graceful shutdown
   - `pumpExited` - signals writePump has exited cleanly

2. **writePump cleanup sequence**:
   - Use `defer` to ensure `pumpExited` is always closed
   - Signal completion before calling `Close()`
   - Ensures goroutines never leak

3. **Close() coordination**:
   - Use `sync.Once` to prevent double-close
   - Wait for `pumpExited` with timeout (5 seconds)
   - Close WebSocket only after writePump exits

4. **Unregister() delegation**:
   - Calls `Close()` to trigger shutdown
   - `Close()` is idempotent via `sync.Once`
   - Safe to call multiple times

## Pros and Cons

### Pros ✅

1. **Non-blocking handler execution**: HTTP handlers return immediately
2. **Isolation of slow clients**: One slow client doesn't affect others
3. **Better performance under load**: Server can handle more concurrent requests
4. **Graceful degradation**: Slow clients are closed cleanly
5. **Industry best practice**: Standard pattern for production WebSocket servers
6. **Matches gorilla/websocket recommendations**: One write goroutine per connection

### Cons ⚠️

1. **Message delivery not guaranteed**: Send() returns before actual delivery
2. **Increased memory usage**: ~50-100KB per connection for buffer + goroutine
3. **Error handling complexity**: Write errors happen in background
4. **Testing complexity**: Async behavior requires synchronization in tests
5. **Connection lifecycle management**: Must ensure goroutines exit cleanly

### Why Cons Are Acceptable for LiveTemplate

1. **Full state updates**: LiveTemplate sends complete state, not deltas. Dropped messages are less critical because next update includes all state.
2. **Reconnect semantics**: Client reconnects on close and receives fresh state.
3. **Memory overhead is small**: 50-100KB per connection is negligible for modern servers.
4. **Error handling is simpler**: Just close connection on write error, client reconnects.
5. **Testing**: Can add synchronization helpers for tests if needed.

## Configuration

### Buffer Size

The send buffer size should be **configurable** to allow tuning based on deployment characteristics.

**Configuration via Functional Options:**

```go
// Functional option for configuring WebSocket buffer size
func WithWebSocketBufferSize(size int) MountOption {
    return func(mc *mountConfig) {
        mc.wsBufferSize = size
    }
}

// Usage in application code
handler := livetemplate.Mount(
    template,
    store,
    livetemplate.WithWebSocketBufferSize(100), // Custom buffer size
)
```

**Implementation in mountConfig:**

```go
type mountConfig struct {
    // ... existing fields
    wsBufferSize int // WebSocket send buffer size per connection
}

// Default configuration
func defaultMountConfig() *mountConfig {
    return &mountConfig{
        // ... existing defaults
        wsBufferSize: 50, // Default: 50 messages
    }
}
```

**Environment Variable Override:**

For deployment flexibility without code changes:

```go
func defaultMountConfig() *mountConfig {
    bufferSize := 50 // Default
    if envSize := os.Getenv("LVT_WS_BUFFER_SIZE"); envSize != "" {
        if size, err := strconv.Atoi(envSize); err == nil && size > 0 {
            bufferSize = size
        }
    }

    return &mountConfig{
        // ... existing defaults
        wsBufferSize: bufferSize,
    }
}
```

**Usage Examples:**

```go
// Use default (50 messages)
handler := livetemplate.Mount(template, store)

// High-throughput deployment (100 messages)
handler := livetemplate.Mount(template, store,
    livetemplate.WithWebSocketBufferSize(100))

// Memory-constrained environment (10 messages)
handler := livetemplate.Mount(template, store,
    livetemplate.WithWebSocketBufferSize(10))

// Override via environment variable
// LVT_WS_BUFFER_SIZE=200 ./myapp
handler := livetemplate.Mount(template, store) // Uses 200 from env
```

**Configuration Priority:**

1. **Functional option** (highest priority) - Explicit in code
2. **Environment variable** - Deployment-time override
3. **Default value** (50) - Fallback if neither specified

**Default Value Rationale:**

- **50 messages** is the default
- Small enough: Low memory overhead (~500KB per 100 connections)
- Large enough: Handles typical burst traffic
- High-traffic sites: Use `WithWebSocketBufferSize(100-200)`
- Memory-constrained: Use `WithWebSocketBufferSize(10-25)`

## Metrics and Observability

### Prometheus Metrics

Add the following metrics to `internal/observe/metrics.go`:

```go
// WebSocket async sending metrics
var (
    // Current buffer usage per connection (gauge)
    websocketSendBufferSize = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "livetemplate_websocket_send_buffer_size",
            Help: "Current number of queued messages in send buffer",
        },
        []string{"group_id"},
    )

    // Total count of buffer overflow events (counter)
    websocketBufferFullTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "livetemplate_websocket_buffer_full_total",
            Help: "Total number of times send buffer was full (client too slow)",
        },
        []string{"group_id"},
    )

    // Total count of connections closed due to slow clients (counter)
    websocketSlowClientClosesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "livetemplate_websocket_slow_client_closes_total",
            Help: "Total number of connections closed due to slow message consumption",
        },
        []string{"group_id"},
    )

    // Total count of write errors (counter)
    websocketWriteErrorsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "livetemplate_websocket_write_errors_total",
            Help: "Total number of WebSocket write errors",
        },
        []string{"group_id", "error_type"},
    )
)
```

### Metric Updates in Code

**In Send() method**:
```go
func (c *Connection) Send(messageType int, data []byte) error {
    select {
    case c.sendChan <- &wsMessage{messageType, data}:
        websocketSendBufferSize.WithLabelValues(c.GroupID).Set(float64(len(c.sendChan)))
        return nil
    case <-c.done:
        return fmt.Errorf("connection closed")
    default:
        websocketBufferFullTotal.WithLabelValues(c.GroupID).Inc()
        websocketSlowClientClosesTotal.WithLabelValues(c.GroupID).Inc()
        go c.Close()
        return fmt.Errorf("client too slow, closing connection")
    }
}
```

**In writePump() method**:
```go
if err != nil {
    websocketWriteErrorsTotal.WithLabelValues(c.GroupID, "write_failed").Inc()
    slog.Warn("WebSocket write failed, closing connection", ...)
    return
}
websocketSendBufferSize.WithLabelValues(c.GroupID).Set(float64(len(c.sendChan)))
```

### Alerting Guidelines

**Recommended Alerts:**

1. **High Buffer Overflow Rate**:
   ```promql
   rate(livetemplate_websocket_buffer_full_total[5m]) > 0.1
   ```
   - Indicates many slow clients
   - May need to increase buffer size or investigate client performance

2. **High Slow Client Close Rate**:
   ```promql
   rate(livetemplate_websocket_slow_client_closes_total[5m]) > 1
   ```
   - Frequent client disconnections
   - Check network conditions or client-side issues

3. **High Write Error Rate**:
   ```promql
   rate(livetemplate_websocket_write_errors_total[5m]) > 0.5
   ```
   - Network issues or connection problems
   - Investigate infrastructure

### Logging

**Structured Logging with Context:**

All WebSocket operations should log with consistent fields:
- `group_id` - Session group
- `user_id` - User identifier
- `buffer_size` - Current buffer length
- `error` - Error message if applicable

**Example Log Entries:**

```
INFO  WebSocket write pump started group_id=abc123 user_id=user456
WARN  WebSocket write failed, closing connection group_id=abc123 user_id=user456 error="write: broken pipe"
WARN  Client too slow, buffer full group_id=abc123 user_id=user456 buffer_size=50
WARN  writePump drain timeout, forcing close group_id=abc123 user_id=user456
```

## Implementation Plan

### Phase 1: Core Infrastructure (2-3 days)

1. Add new fields to `Connection` struct:
   - `sendChan chan *wsMessage`
   - `done chan struct{}`
   - `pumpExited chan struct{}`
   - `closeOnce sync.Once`

2. Implement goroutine lifecycle methods:
   - `writePump()` with proper defer cleanup
   - `drainSendChannel()` for graceful shutdown
   - `Close()` with sync.Once and timeout coordination

3. Update connection management:
   - `Send()` to use async channel send
   - `Register()` to initialize channels and start writePump
   - `Unregister()` to delegate to `Close()`

### Phase 2: Configuration and Metrics (1-2 days)

1. Add configuration support:
   - Add `wsBufferSize` field to `mountConfig` (default: 50)
   - Create `WithWebSocketBufferSize(int)` functional option
   - Support `LVT_WS_BUFFER_SIZE` environment variable in `defaultMountConfig()`
   - Pass buffer size to `Register()` from mount handler

2. Add Prometheus metrics to `internal/observe/metrics.go`:
   - `websocketSendBufferSize` gauge
   - `websocketBufferFullTotal` counter
   - `websocketSlowClientClosesTotal` counter
   - `websocketWriteErrorsTotal` counter

3. Integrate metrics into Send() and writePump()

4. Add structured logging with consistent fields

### Phase 3: Testing (2-3 days)

**Unit Tests**:
1. Test `writePump()` delivers messages correctly
2. Test buffer overflow triggers connection close
3. Test graceful shutdown drains messages
4. Test `Close()` is idempotent (can call multiple times)
5. **Test goroutine leak prevention** (CRITICAL):
   - Run 1,000,000+ connection open/close cycles
   - Verify goroutine count returns to baseline
   - Use `runtime.NumGoroutine()` to track
   - Fail if goroutines leak

**Integration Tests**:
1. Test concurrent sends to multiple connections
2. Test slow client scenario (simulated)
3. Test registry cleanup on unregister

**E2E Tests**:
1. Run existing test suite with `-p 4` (20 runs)
2. Run stress test with `-p 8` (10 runs)
3. Verify 100% pass rate (no flakiness)
4. Verify delete operations work consistently

**Benchmarks**:
1. Compare throughput vs synchronous implementation
2. Measure latency distribution
3. Test memory usage under load

### Phase 4: Documentation (1 day)

1. Update godoc comments:
   - `Connection.Send()` - Document async behavior
   - `Connection.Close()` - Document idempotency
   - `writePump()` - Document lifecycle

2. Update CLAUDE.md:
   - Document async WebSocket architecture
   - Explain goroutine lifecycle
   - Add metrics and monitoring guidance

3. Create migration notes (if needed for downstream users)

## Testing Strategy

### Unit Tests

```go
// Test async message delivery
func TestWritePumpDeliversMessages(t *testing.T) {
    // 1. Create mock WebSocket connection
    // 2. Register connection (starts writePump)
    // 3. Send multiple messages via Send()
    // 4. Verify all messages written to WebSocket
    // 5. Close connection
    // 6. Verify writePump exits cleanly
}

// Test buffer overflow handling
func TestBufferOverflowClosesConnection(t *testing.T) {
    // 1. Create connection with small buffer (size=2)
    // 2. Block WebSocket writes (simulate slow client)
    // 3. Send messages until buffer full
    // 4. Verify next Send() returns error
    // 5. Verify connection Close() was called
    // 6. Verify metrics incremented (buffer_full, slow_client_closes)
}

// Test graceful shutdown
func TestGracefulShutdownDrainsMessages(t *testing.T) {
    // 1. Queue several messages in sendChan
    // 2. Call Close()
    // 3. Verify all queued messages are sent before close
    // 4. Verify pumpExited channel is closed
    // 5. Verify WebSocket closed after drain
}

// Test idempotent Close()
func TestCloseIsIdempotent(t *testing.T) {
    // 1. Create and register connection
    // 2. Call Close() multiple times concurrently
    // 3. Verify WebSocket Close() called exactly once
    // 4. Verify no panics or errors
}

// CRITICAL: Test goroutine leak prevention
func TestNoGoroutineLeaks(t *testing.T) {
    // 1. Record baseline goroutine count
    baseline := runtime.NumGoroutine()

    // 2. Create and close 10,000 connections
    for i := 0; i < 10000; i++ {
        conn := createTestConnection()
        registry.Register(conn)
        conn.Send(websocket.TextMessage, []byte("test"))
        registry.Unregister(conn)
    }

    // 3. Force GC and wait for cleanup
    runtime.GC()
    time.Sleep(100 * time.Millisecond)

    // 4. Verify goroutine count back to baseline (+/- 5 tolerance)
    final := runtime.NumGoroutine()
    if abs(final - baseline) > 5 {
        t.Fatalf("Goroutine leak detected: baseline=%d final=%d", baseline, final)
    }
}

// Test for extended leak test (run separately)
func TestExtendedGoroutineLeaks(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping extended leak test in short mode")
    }

    baseline := runtime.NumGoroutine()

    // Run 1,000,000 connection cycles
    for i := 0; i < 1000000; i++ {
        conn := createTestConnection()
        registry.Register(conn)
        conn.Send(websocket.TextMessage, []byte("test"))
        registry.Unregister(conn)

        // Periodic check every 100k cycles
        if i % 100000 == 0 {
            runtime.GC()
            current := runtime.NumGoroutine()
            if current > baseline + 100 {
                t.Fatalf("Goroutine leak at cycle %d: baseline=%d current=%d",
                    i, baseline, current)
            }
        }
    }

    runtime.GC()
    time.Sleep(1 * time.Second)

    final := runtime.NumGoroutine()
    if abs(final - baseline) > 10 {
        t.Fatalf("Goroutine leak after 1M cycles: baseline=%d final=%d",
            baseline, final)
    }
}
```

### Integration Tests

```go
// Test concurrent sends to multiple connections
func TestConcurrentSends(t *testing.T) {
    // 1. Create 100 connections
    // 2. Send 1000 messages concurrently to each
    // 3. Verify all messages delivered
    // 4. No data races or deadlocks
}

// Test slow client simulation
func TestSlowClientHandling(t *testing.T) {
    // 1. Create connection with instrumented WebSocket
    // 2. Add artificial delay to WriteMessage (100ms)
    // 3. Send messages rapidly
    // 4. Verify buffer fills up
    // 5. Verify connection closed when buffer full
}
```

### E2E Tests

Run existing browser-based E2E test suite:

```bash
# Standard parallelism (current failure mode)
go test -v -p 4 -count 20 ./...

# Stress test with high parallelism
go test -v -p 8 -count 10 ./...

# Extended soak test
go test -v -p 4 -count 100 -timeout 30m ./...
```

**Success Criteria:**
- 100% pass rate (0 failures out of 20 runs at `-p 4`)
- 100% pass rate (0 failures out of 10 runs at `-p 8`)
- Delete operations work consistently
- No test timeouts

### Performance Benchmarks

```go
func BenchmarkAsyncSendThroughput(b *testing.B) {
    conn := createBenchConnection()
    data := []byte("test message")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        conn.Send(websocket.TextMessage, data)
    }
    b.StopTimer()

    conn.Close()
}

func BenchmarkSyncSendThroughput(b *testing.B) {
    // Baseline: current synchronous implementation
    conn := createSyncConnection()
    data := []byte("test message")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        conn.SendSync(websocket.TextMessage, data)
    }
    b.StopTimer()

    conn.Close()
}

func BenchmarkConcurrentConnections(b *testing.B) {
    // Test scalability with many connections
    numConns := 1000
    conns := make([]*Connection, numConns)

    for i := 0; i < numConns; i++ {
        conns[i] = createBenchConnection()
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        idx := i % numConns
        conns[idx].Send(websocket.TextMessage, []byte("test"))
    }
    b.StopTimer()
}
```

### Test Helpers for Async Verification

```go
// Helper to wait for message delivery in tests
func waitForMessageDelivery(conn *Connection, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if len(conn.sendChan) == 0 {
            return nil // All messages sent
        }
        time.Sleep(1 * time.Millisecond)
    }
    return fmt.Errorf("timeout waiting for message delivery")
}

// Helper to verify goroutine cleanup
func verifyGoroutineCleanup(t *testing.T, baseline int, tolerance int) {
    runtime.GC()
    time.Sleep(100 * time.Millisecond)

    current := runtime.NumGoroutine()
    if abs(current - baseline) > tolerance {
        t.Errorf("Goroutine leak: baseline=%d current=%d", baseline, current)
    }
}
```

## Migration Guide

### For Existing Code

**No changes required!** The `Send()` API remains the same:

```go
conn.Send(websocket.TextMessage, data)
```

### For Test Code

If tests need to verify message delivery, add synchronization:

```go
// Wait for message to be sent
time.Sleep(10 * time.Millisecond)

// Or use a custom helper
waitForMessageDelivery(conn)
```

## Alternatives Considered

### 1. Add Write Deadline

**Tried:** Set write deadline to prevent indefinite blocking
**Result:** Made tests fail 100% of the time
**Why it failed:** Write deadlines are persistent and broke subsequent operations

### 2. Reduce Parallelism (-p 2)

**Trade-off:** Tests run slower but pass reliably
**Status:** Temporary workaround, not a solution
**Why not sufficient:** Doesn't solve production performance issues

### 3. Increase Timeouts

**Tried:** Increased from 60s to 120s
**Result:** Tests still fail, just take longer
**Why not sufficient:** Masks the problem, doesn't fix root cause

### 4. Accept Flakiness

**Trade-off:** Keep current implementation, retry failed tests in CI
**Status:** Current state (undesirable)
**Why unacceptable:** Production could experience same blocking issues

## Performance Considerations

### Memory Impact

- **Per connection:** ~52KB (50 messages × 1KB avg + 2KB goroutine)
- **100 connections:** ~5MB
- **1,000 connections:** ~50MB
- **10,000 connections:** ~500MB

### CPU Impact

- **Goroutine scheduling:** Minimal (Go runtime is efficient)
- **Channel operations:** O(1) for send/receive
- **Lock contention:** Reduced (shorter critical sections)

### Latency Impact

- **Best case:** Same as synchronous (immediate delivery)
- **Average case:** +100μs (channel enqueue/dequeue)
- **Worst case:** Message queued but client slow (would block synchronously)

## Deployment Strategy

Since LiveTemplate is **pre-v1.0**, this change can be deployed directly without feature flags or gradual rollout:

### Implementation Branch

1. Create feature branch `async-websocket-sends`
2. Implement all phases as outlined above
3. Run comprehensive test suite:
   - Unit tests (including 1M+ cycle leak test)
   - Integration tests
   - E2E tests with `-p 4` and `-p 8` (20+ runs each)
   - Benchmarks (compare vs current sync implementation)

### Merge Criteria

**All must pass before merging to main:**

1. ✅ 100% E2E test pass rate (0 failures in 20 runs at `-p 4`)
2. ✅ No goroutine leaks (verified with extended leak test)
3. ✅ Memory stable (verified with benchmarks)
4. ✅ Throughput improved or unchanged vs sync implementation
5. ✅ All unit and integration tests passing
6. ✅ Code review complete

### Post-Merge Monitoring

After merging to main, monitor in production:

1. **Metrics Dashboard**:
   - Track `websocket_send_buffer_size`
   - Alert on `websocket_buffer_full_total` spike
   - Monitor `websocket_slow_client_closes_total`

2. **Performance**:
   - Monitor CPU usage (expect slight decrease)
   - Monitor memory usage (expect slight increase)
   - Track P99 WebSocket send latency

3. **Errors**:
   - Monitor `websocket_write_errors_total`
   - Track connection close rate
   - Check for goroutine count growth

### Rollback Plan

If issues arise post-merge:

1. **Quick Rollback**: Revert the commit (since pre-v1.0)
2. **Investigate**: Check metrics and logs for root cause
3. **Fix**: Address issue in new branch
4. **Re-merge**: After fixing and re-testing

## Success Metrics

1. **E2E tests pass consistently** under `-p 4` parallelism (100% pass rate over 20 runs)
2. **No goroutine leaks** after 1,000,000 connection open/close cycles
3. **Memory usage stable** over 24-hour stress test
4. **Throughput increased** by >20% under concurrent load
5. **P99 latency unchanged** or improved vs synchronous implementation

## References

- [Gorilla WebSocket Issue #675: Slow sending of messages](https://github.com/gorilla/websocket/issues/675)
- [Gorilla WebSocket Issue #228: HOL blocking](https://github.com/gorilla/websocket/issues/228)
- [Gorilla WebSocket Concurrency Documentation](https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency)
- [E2E Test Failure Investigation](/tmp/delete_test_breakthrough.md)

## Resolved Design Questions

1. **Should buffer size be configurable?**
   - ✅ **YES** - Configurable via `WithWebSocketBufferSize()` functional option
   - Environment variable override: `LVT_WS_BUFFER_SIZE`
   - Default: 50 messages
   - Allows tuning for different deployment scenarios
   - Follows idiomatic Go patterns for optional configuration

2. **Should we add metrics for monitoring?**
   - ✅ **YES** - Full Prometheus metrics integration in `internal/observe/metrics.go`
   - Metrics: buffer size, buffer full count, slow client closes, write errors
   - Enables production monitoring and alerting

3. **Should we support graceful connection draining?**
   - ✅ **YES** - Implemented via `drainSendChannel()` with 5-second timeout
   - Best-effort delivery of pending messages on close
   - Prevents message loss on clean disconnects

4. **How to prevent goroutine leaks?**
   - ✅ **SOLVED** - Multi-layer approach:
     - `defer close(pumpExited)` in writePump
     - `sync.Once` for idempotent Close()
     - `pumpExited` channel for shutdown coordination
     - Comprehensive leak tests (1M+ cycles)

5. **Should we add circuit breaker for repeatedly failing connections?**
   - ❌ **NO** - Not in initial version
   - Current approach (close on buffer full) is sufficient
   - Can add later if production data shows need

## Implementation Timeline

### Total Estimate: 6-8 days

**Detailed Breakdown:**

- **Phase 1: Core Infrastructure** (2-3 days)
  - Connection struct updates with new fields
  - writePump, drainSendChannel, Close implementations
  - Register/Unregister coordination updates

- **Phase 2: Configuration & Metrics** (1-2 days)
  - Buffer size configuration
  - Prometheus metrics integration
  - Structured logging

- **Phase 3: Testing** (2-3 days)
  - Unit tests (including leak tests)
  - Integration tests
  - E2E verification (20+ runs at `-p 4`, 10+ runs at `-p 8`)
  - Performance benchmarks

- **Phase 4: Documentation** (1 day)
  - Godoc updates
  - CLAUDE.md updates
  - Migration notes

### Merge Checklist

Before merging `async-websocket-sends` → `main`:

- [ ] All unit tests passing
- [ ] Goroutine leak test passing (1M+ cycles)
- [ ] E2E tests 100% pass rate at `-p 4` (20 runs)
- [ ] E2E tests 100% pass rate at `-p 8` (10 runs)
- [ ] Benchmarks show throughput ≥ sync implementation
- [ ] Memory usage stable under load
- [ ] Code review complete
- [ ] Documentation updated

## Conclusion

This proposal addresses the root cause of WebSocket message delivery delays under high load by implementing industry-standard async sending with buffered channels. The benefits (non-blocking handlers, isolation, better performance) outweigh the costs (memory, complexity) for LiveTemplate's use case of full state updates.

**Key Improvements in This Version:**

1. ✅ **Goroutine Leak Prevention**: Added `pumpExited` channel and `sync.Once` for safe lifecycle management
2. ✅ **Configuration**: Made buffer size configurable via `TemplateConfig` and environment variable
3. ✅ **Observability**: Comprehensive Prometheus metrics and structured logging
4. ✅ **Testing**: Detailed goroutine leak tests with 1M+ connection cycles
5. ✅ **Pre-v1.0 Deployment**: Simplified deployment strategy (no feature flags needed)

**Recommendation:** Implement this proposal to fix e2e test flakiness and improve production performance under load. The design is production-ready with comprehensive safeguards against common async pitfalls.
