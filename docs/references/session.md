# Session Reference

Session infrastructure in LiveTemplate handles state storage, connection management, and WebSocket configuration. This guide covers session stores, connection management, and performance tuning for production deployments.

For pushing updates from server-side code, see [Server Actions Reference](server-actions.md).

## Overview

### Key Concepts

- **Session groups**: Isolation boundaries for shared state. All connections with the same `groupID` share the same state instance.
- **State**: Application state cloned per session group via `AsState()`
- **Connections**: Individual WebSocket connections within a group
- **Session store**: Persistence layer for session groups (in-memory or Redis)

### Per-Connection State (Default)

Since v0.9, each WebSocket connection owns its state independently. Actions update only the calling connection's state. `OnConnect()` initializes per-connection state (e.g., resetting `CurrentUser` to empty for each new tab). Cross-tab sync requires explicit `ctx.BroadcastAction("ActionName", data)`, which dispatches the named action to all other connections in the session group, preserving each connection's per-connection fields.

**Reconnect behavior:** State is persisted to SessionStore after every action (both HTTP and WebSocket). On reconnect, the new connection loads the last-persisted state from SessionStore, clears any transient fields (`lvt:"transient"`), then calls `OnConnect()` to re-initialize per-connection fields. The key difference from `WithSharedState()` is that per-connection mode does NOT auto-broadcast to other tabs — cross-tab sync requires explicit `ctx.BroadcastAction()`.

### State Persistence Matrix

| Operation | Per-Connection Mode | SharedState Mode |
|-----------|-------------------|-----------------|
| Mount() (new session) | Persisted | Persisted |
| OnConnect() (reconnect) | Not persisted | Not persisted |
| HTTP POST action | Persisted | Persisted + auto-broadcast |
| WebSocket action | Persisted | Persisted + auto-broadcast |
| Dispatched action (BroadcastAction) | Persisted | N/A (uses auto-broadcast) |
| Server action (TriggerAction) | Persisted (since v0.9.0) | Persisted |
| Auto-broadcast to other tabs | No | Yes |

### Shared State Mode (Opt-In)

> **Note:** This behavior requires `WithSharedState()`. It is no longer the default.

When using `WithSharedState()`, all tabs in the same browser session automatically receive updates after every action:

```go
func (c *ChatController) SendMessage(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    newMessage := ctx.GetString("message")
    state.Messages = append(state.Messages, newMessage)
    return state, nil  // All tabs in same browser update automatically (SharedState mode only)
}
```

**How it works in SharedState mode:**
- Each browser gets a unique session ID (via cookie: `livetemplate-id`)
- All tabs in the same browser share this session ID (`groupID`)
- State changes automatically broadcast to all tabs in the same session group

## Session Store Interface

The session store interface manages session groups:

```go
type SessionStore interface {
    // Get retrieves the state for a session group.
    // Returns nil if the group doesn't exist.
    Get(ctx context.Context, groupID string) interface{}

    // Set stores state for a session group.
    // Creates a new group if it doesn't exist, updates if it does.
    Set(ctx context.Context, groupID string, state interface{})

    // Delete removes a session group and all its state.
    Delete(ctx context.Context, groupID string)

    // List returns all active session group IDs.
    List(ctx context.Context) []string
}
```

### SingleStoreSetter

An optimization interface for updating a single named store within a session group without replacing the entire state. Both `MemorySessionStore` and `RedisSessionStore` implement this interface. Note: `MemorySessionStore.SetStore()` is a no-op since in-memory references are already updated in-place; the optimization primarily benefits `RedisSessionStore` where it avoids re-serializing all stores on every action:

```go
type SingleStoreSetter interface {
    SetStore(ctx context.Context, groupID string, storeName string, store interface{})
}
```

### MemorySessionStore

In-memory session store for single-instance deployments.

**Features:**
- Thread-safe for concurrent access
- Tracks last access time for each group
- Automatic cleanup of inactive groups (configurable TTL)

**Configuration:**

```go
store := livetemplate.NewMemorySessionStore(
    livetemplate.WithCleanupTTL(12*time.Hour),       // Default: 24 hours
    livetemplate.WithCleanupInterval(30*time.Minute), // Default: 1 hour
)
defer store.Close() // Stop cleanup goroutine on shutdown

tmpl := livetemplate.New("app",
    livetemplate.WithSessionStore(store),
)
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithCleanupTTL(ttl)` | 24 hours | Time-to-live for inactive groups |
| `WithCleanupInterval(interval)` | 1 hour | How often cleanup runs |

### RedisSessionStore

Redis-backed session store for distributed/multi-instance deployments.

**Features:**
- Suitable for horizontal scaling
- Automatic TTL refresh on access
- Connection retry with exponential backoff
- Serialization using gob encoding

**Redis Key Schema:**
- `livetemplate:session:{groupID}` -> Gob-encoded Stores
- `livetemplate:session:{groupID}:access` -> Last access timestamp

**Configuration:**

```go
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

store := livetemplate.NewRedisSessionStore(client,
    livetemplate.WithSessionTTL(24*time.Hour),         // Default: 24 hours
    livetemplate.WithMaxRetries(5),                     // Default: 3
    livetemplate.WithRetryDelay(200*time.Millisecond), // Default: 100ms
)

tmpl := livetemplate.New("app",
    livetemplate.WithSessionStore(store),
)
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithSessionTTL(ttl)` | 24 hours | TTL for sessions in Redis |
| `WithMaxRetries(n)` | 3 | Retry attempts for Redis operations |
| `WithRetryDelay(delay)` | 100ms | Base delay for exponential backoff |

**Important: Register Custom Types**

Custom state types MUST be registered with `gob.Register()` before use. Register ALL types that will be serialized, including nested structs and slice element types:

```go
type User struct {
    ID   string
    Name string
}

type MyState struct {
    Value    int
    Users    []User          // Nested type - must also register
    Metadata map[string]any  // Maps with interface values need care
}

func init() {
    // Register the state type AND all nested types
    gob.Register(&MyState{})
    gob.Register(&User{})        // Required for []User slice
    gob.Register(map[string]any{}) // If using interface{} maps
}
```

> **Common Pitfall:** Forgetting to register nested types causes silent serialization failures. If state doesn't persist across Redis, check gob registration.

**Health Checks:**

```go
// Check Redis connectivity
if err := store.Ping(); err != nil {
    log.Printf("Redis unhealthy: %v", err)
}

// With context timeout
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
if err := store.PingContext(ctx); err != nil {
    log.Printf("Redis unhealthy: %v", err)
}
```

## Connection Management

### Connection Type

Represents a single WebSocket connection with metadata:

```go
type Connection struct {
    Conn     *websocket.Conn // The WebSocket connection
    GroupID  string          // Session group ID (shared state boundary)
    UserID   string          // User identity ("" for anonymous)
    Template interface{}     // Per-connection template for tree diffing
    State    interface{}     // Reference to shared state from session group
    Uploads  interface{}     // Per-connection upload registry
}
```

**Key Methods:**
- `Send(messageType, data) error` - Thread-safe async send (non-blocking)
- `Close() error` - Thread-safe graceful shutdown

**Async Send Architecture:**

```
Send(msgType, data) [called from handler]
    |
[Non-blocking] Queue to sendChan (buffered channel)
    |
writePump goroutine (one per connection)
    |
Dequeue from sendChan
    |
conn.WriteMessage (protected by mutex)
    |
WebSocket
```

### ConnectionRegistry

Efficient lookup for broadcasting with dual indexing:

```go
type ConnectionRegistry struct {
    byGroup map[string][]*Connection  // groupID -> connections
    byUser  map[string][]*Connection  // userID -> connections
}
```

| Method | Description |
|--------|-------------|
| `Register(conn, bufferSize)` | Add connection and start writePump |
| `Unregister(conn)` | Remove connection, trigger graceful shutdown |
| `GetByGroup(groupID)` | All connections in a session group (multi-tab) |
| `GetByUser(userID)` | All connections for a user (multi-device) |
| `GetAll()` | All active connections |
| `Count()` | Total connection count |
| `GroupCount()` | Number of session groups |
| `UserCount()` | Number of unique users |

### ConnectionLimits

Resource protection with two-level limits:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithMaxConnections(10000),      // Global limit
    livetemplate.WithMaxConnectionsPerGroup(10), // Per-group limit
)
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithMaxConnections(max)` | 0 (unlimited) | Global connection limit |
| `WithMaxConnectionsPerGroup(max)` | 0 (unlimited) | Per-group limit (prevents single-user DOS) |

## WebSocket Configuration

### Buffer Size

Configure the message buffer per connection:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithWebSocketBufferSize(100), // Default: 50
)
```

Or via environment variable:
```bash
export LVT_WS_BUFFER_SIZE=100
```

**Buffer Size Recommendations:**

| Traffic Level | Buffer Size | Notes |
|---------------|-------------|-------|
| Low/memory-constrained | 10-25 | Minimal memory footprint |
| Normal | 50 (default) | Good balance |
| High/burst-heavy | 100-200 | Handles traffic spikes |

> **Memory Warning:** Buffer size directly affects memory usage per connection. Each buffered message slot reserves memory even when empty. Plan for: `connections × buffer_size × avg_message_size`.

**Memory calculation:**
- Base overhead: ~980 bytes per connection
- Buffer overhead: buffer size × average message size
- Example: 50-buffer at 1KB average = ~50KB per connection
- **1,000 connections with default buffer ≈ 50MB**
- **10,000 connections with 200 buffer ≈ 2GB**

For memory-constrained environments, prefer smaller buffers (10-25) and rely on backpressure to handle slow clients.

### HTTP-Only Mode

Disable WebSocket for HTTP-only operation:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithWebSocketDisabled(),
)
```

### Rate Limiting

Limit message rate per connection using a token bucket algorithm:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithMessageRateLimit(10, 20), // 10 msg/sec, burst of 20
)
```

| Parameter | Description | Default |
|-----------|-------------|---------|
| `messagesPerSecond` (`float64`) | Sustained message rate | `10` |
| `burstCapacity` (`int`) | Maximum burst size above sustained rate | `20` |

Set `messagesPerSecond = 0` to disable rate limiting (not recommended for production).

## Error Handling

### Connection Errors

```go
var (
    // Returned when Send() is called on a closed connection
    ErrConnectionClosed = errors.New("connection closed")

    // Returned when buffer is full (client not consuming fast enough)
    // Connection will be closed automatically
    ErrClientTooSlow = errors.New("client too slow")
)
```

**Handling slow clients:**
- When buffer is full, connection is closed (fail-fast)
- Prevents memory buildup from slow clients
- Monitor `wsBufferFull` and `wsSlowClientCloses` metrics

## Performance Characteristics

| Metric | Value |
|--------|-------|
| Concurrent sends/sec | 165M (lock-free) |
| Queued sends/sec | 54.7M (through buffer) |
| Memory per connection | ~980 bytes base |

**Memory calculation:**
- Base overhead: ~980 bytes per connection
- Buffer overhead: buffer size x avg message size
- Example: 50-buffer at 1KB = ~50KB per connection
- 1000 connections = ~50MB total

## See Also

- [Server Actions Reference](server-actions.md) - TriggerAction API for server-initiated updates
- [Authentication Reference](authentication.md) - User identification and custom authenticators
- [Scaling Guide](../guides/SCALING.md) - Horizontal scaling with Redis
