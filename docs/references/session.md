# Session Reference

Session infrastructure in LiveTemplate handles state storage, connection management, and WebSocket configuration. This guide covers session stores, connection management, and performance tuning for production deployments.

For pushing updates from server-side code, see [Server Actions Reference](server-actions.md).

## Overview

### Key Concepts

- **Session groups**: Isolation boundaries for shared state. All connections with the same `groupID` share the same `Stores` instance.
- **Stores**: Application state (`map[string]Store`) shared within a session group
- **Connections**: Individual WebSocket connections within a group
- **SessionStore**: Persistence layer for session groups (in-memory or Redis)

### Automatic Session Syncing

When a user performs an action, all tabs in the same browser session automatically receive updates. This happens with zero configuration:

```go
func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    s.Messages = append(s.Messages, newMessage)
    return nil  // All tabs in same browser update automatically
}
```

**How it works:**
- Each browser gets a unique session ID (via cookie: `livetemplate-id`)
- All tabs in the same browser share this session ID (`groupID`)
- State changes automatically broadcast to all tabs in the same session group

## SessionStore Interface

The `SessionStore` interface manages session groups:

```go
type SessionStore interface {
    // Get retrieves the Stores for a session group.
    // Returns nil if the group doesn't exist.
    Get(ctx context.Context, groupID string) Stores

    // Set stores Stores for a session group.
    // Creates a new group if it doesn't exist, updates if it does.
    Set(ctx context.Context, groupID string, stores Stores)

    // Delete removes a session group and all its state.
    Delete(ctx context.Context, groupID string)

    // List returns all active session group IDs.
    List(ctx context.Context) []string
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

Custom Store types MUST be registered with `gob.Register()` before use:

```go
type MyStore struct {
    Value int
}

func (m *MyStore) Change(ctx *livetemplate.ActionContext) error {
    return nil
}

func init() {
    gob.Register(&MyStore{})
}
```

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
    Stores   interface{}     // Reference to shared stores from session group
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

**Memory estimate:** 50 buffered messages at ~1KB avg = ~50KB per connection

### HTTP-Only Mode

Disable WebSocket for HTTP-only operation:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithWebSocketDisabled(),
)
```

### Rate Limiting

Limit message rate per connection:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithMessageRateLimit(10, 20), // 10 msg/sec, burst of 20
)
```

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
- [Scaling Guide](../SCALING.md) - Horizontal scaling with Redis
