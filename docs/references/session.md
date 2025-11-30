# Session Reference

Session management in LiveTemplate handles state sharing, connection management, and server-initiated actions. This guide covers session stores, the Session API, connection management, and WebSocket configuration.

## Overview

LiveTemplate provides two types of updates:
1. **Automatic Session Syncing** - Tabs in the same browser automatically stay in sync (no code needed)
2. **Server-Initiated Actions** - Push updates from server-side code (timers, webhooks, background jobs)

### Quick Concepts

- **Session groups**: Isolation boundaries for shared state (all connections with same `groupID` share `Stores`)
- **Stores**: Application state shared within a group
- **Connections**: Individual WebSocket connections within a group
- **Session API**: Interface for server-initiated actions

## Automatic Session Syncing (Default Behavior)

When a user performs an action that modifies state, **all tabs in the same browser session automatically receive updates**. This happens with zero configuration:

```go
type ChatState struct {
    Messages []Message
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    s.Messages = append(s.Messages, newMessage)
    return nil  // All tabs in same browser update automatically!
}
```

**How it works:**
- Each browser gets a unique session ID (via cookie: `livetemplate-id`)
- All tabs in the same browser share this session ID
- State changes automatically broadcast to all tabs in the same session
- No manual code required

**Example:** Chat app where multiple tabs stay in sync:
```go
// Tab 1: User sends message
// Tab 2, Tab 3: Automatically see the new message
```

## Server-Initiated Actions

For pushing updates from the server (timers, webhooks, background jobs), implement `SessionAware`:

```go
type TimerStore struct {
    Seconds int
    session livetemplate.Session
    mu      sync.Mutex
}

// OnConnect is called when a WebSocket connection is established
func (s *TimerStore) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.mu.Lock()
    s.session = session
    s.mu.Unlock()

    // Start background timer
    go s.runTimer(ctx)
    return nil
}

// OnDisconnect is called when the WebSocket connection closes
func (s *TimerStore) OnDisconnect() {
    s.mu.Lock()
    s.session = nil
    s.mu.Unlock()
}

func (s *TimerStore) runTimer(ctx context.Context) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.mu.Lock()
            session := s.session
            s.mu.Unlock()

            if session != nil {
                // Trigger action from server - calls Change() and updates all user's tabs
                session.TriggerAction("tick", nil)
            }
        }
    }
}

func (s *TimerStore) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "tick":
        s.Seconds++
    case "reset":
        s.Seconds = 0
    }
    return nil
}
```

## Session Interface

```go
type Session interface {
    // TriggerAction triggers Store.Change() with the given action and data,
    // then sends the updated template to ALL connections for this user.
    TriggerAction(action string, data map[string]interface{}) error
}
```

**Key Points:**
- `TriggerAction()` calls `Store.Change()` just like client-initiated actions
- Updates are sent to ALL of the user's connections (all tabs/devices)
- Scoped to the current user only - cannot target other users
- Thread-safe - can be called from any goroutine

## SessionAware Interface

```go
type SessionAware interface {
    OnConnect(ctx context.Context, session Session) error
    OnDisconnect()
}
```

**Lifecycle:**
1. WebSocket connection established -> `OnConnect()` called with `Session`
2. Store the `Session` for later use (e.g., in background goroutines)
3. WebSocket connection closed -> `OnDisconnect()` called
4. Clean up any references to `Session`

**Context (`ctx`):**
- Contains cancellation signal - cancelled when WebSocket disconnects
- Use for background goroutines to know when to stop
- Pass to database calls for timeout/cancellation support

## Common Patterns

### Timer/Tick Updates

```go
func (s *Store) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.session = session
    go s.runTicker(ctx)
    return nil
}

func (s *Store) runTicker(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if s.session != nil {
                s.session.TriggerAction("refresh", nil)
            }
        }
    }
}
```

### Webhook-Triggered Updates

```go
// HTTP handler receives webhook
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var payload WebhookPayload
    json.NewDecoder(r.Body).Decode(&payload)

    // Store session reference from OnConnect
    // Session scoped to specific user, so webhook needs to know target user
    if session := getUserSession(payload.UserID); session != nil {
        session.TriggerAction("notification", map[string]interface{}{
            "message": payload.Message,
        })
    }

    w.WriteHeader(http.StatusOK)
}
```

### Welcome Message After Connect

```go
func (s *AuthStore) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.session = session

    // Send welcome after short delay (page needs to fully render)
    if s.IsLoggedIn {
        go func() {
            time.Sleep(500 * time.Millisecond)
            session.TriggerAction("serverWelcome", map[string]interface{}{
                "message": fmt.Sprintf("Welcome back, %s!", s.Username),
            })
        }()
    }

    return nil
}

func (s *AuthStore) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "serverWelcome":
        s.WelcomeMessage = ctx.GetString("message")
    // ... other actions
    }
    return nil
}
```

### Background Job Completion

```go
func (s *Store) submitJob(ctx *livetemplate.ActionContext) error {
    // Start background job
    go func() {
        result := performLongRunningTask()

        // Notify user when done
        if s.session != nil {
            s.session.TriggerAction("jobComplete", map[string]interface{}{
                "result": result,
            })
        }
    }()

    return nil
}
```

## Thread Safety

Session methods are thread-safe and can be called from any goroutine:

```go
// Safe: Multiple goroutines using session
go func() { session.TriggerAction("update1", nil) }()
go func() { session.TriggerAction("update2", nil) }()
```

However, you should protect access to the session field itself:

```go
type Store struct {
    session livetemplate.Session
    mu      sync.Mutex
}

func (s *Store) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.mu.Lock()
    s.session = session
    s.mu.Unlock()
    return nil
}

func (s *Store) OnDisconnect() {
    s.mu.Lock()
    s.session = nil
    s.mu.Unlock()
}

func (s *Store) triggerFromBackground() {
    s.mu.Lock()
    session := s.session
    s.mu.Unlock()

    if session != nil {
        session.TriggerAction("update", nil)
    }
}
```

## Security Model

**Session is scoped to the current user only:**
- `TriggerAction()` affects ALL connections for THIS user
- There is no way to target other users
- Prevents unauthorized cross-user actions
- Safe to expose to store logic

**Why this design?**
- Simpler mental model - "push to myself"
- No accidental cross-user data leaks
- No authorization checks needed in store code
- For admin broadcasts, use database + polling or dedicated admin endpoints

## Multi-Tab Behavior

When a user has multiple tabs open:

1. **Client Action (Tab 1)**: User clicks button in Tab 1
   - Tab 1's store.Change() is called
   - Tab 1 receives update
   - Tab 2, Tab 3 automatically receive update (auto-broadcast)

2. **Server Action (TriggerAction)**: Background job completes
   - ALL tabs receive the action via Change()
   - ALL tabs are updated simultaneously

## SessionStore Interface

The `SessionStore` interface manages session groups, where each group contains Stores shared across connections.

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

**Configuration Options:**

```go
store := livetemplate.NewMemorySessionStore(
    livetemplate.WithCleanupTTL(12*time.Hour),      // Default: 24 hours
    livetemplate.WithCleanupInterval(30*time.Minute), // Default: 1 hour
)
defer store.Close() // Stop cleanup goroutine on shutdown
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithCleanupTTL(ttl)` | 24 hours | Time-to-live for inactive groups |
| `WithCleanupInterval(interval)` | 1 hour | How often cleanup runs |

**Usage:**

```go
tmpl := livetemplate.New("app",
    livetemplate.WithSessionStore(store),
)
```

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

**Configuration Options:**

```go
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

store := livetemplate.NewRedisSessionStore(client,
    livetemplate.WithSessionTTL(24*time.Hour),  // Default: 24 hours
    livetemplate.WithMaxRetries(5),              // Default: 3
    livetemplate.WithRetryDelay(200*time.Millisecond), // Default: 100ms
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

**Key Methods:**

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

## Distributed Deployments (Redis)

In multi-instance deployments with Redis PubSub configured, `TriggerAction()` automatically publishes to Redis so all instances can update their local connections for the user:

```go
// Instance 1: User connects here
session.TriggerAction("update", nil)

// Instance 2: If user has tabs here, they also receive the update
```

This happens transparently - no code changes needed.

Configure with:
```go
tmpl := livetemplate.New("app",
    livetemplate.WithPubSubBroadcaster(redisBroadcaster),
)
```

## Migration from Broadcaster (Deprecated)

The old `Broadcaster` API is deprecated. Here's how to migrate:

**Before (deprecated):**
```go
type Store struct {
    broadcaster livetemplate.Broadcaster
}

func (s *Store) OnConnect(ctx context.Context, b livetemplate.Broadcaster) error {
    s.broadcaster = b
    return nil
}

func (s *Store) pushUpdate() {
    s.broadcaster.Send()  // Re-renders and sends entire template
}
```

**After (recommended):**
```go
type Store struct {
    session livetemplate.Session
}

func (s *Store) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.session = session
    return nil
}

func (s *Store) pushUpdate() {
    // Trigger action that modifies state and sends update
    s.session.TriggerAction("refresh", nil)
}

func (s *Store) Change(ctx *livetemplate.ActionContext) error {
    if ctx.Action == "refresh" {
        // Update state here
        s.Data = fetchLatestData()
    }
    return nil
}
```

**Key Differences:**

| Broadcaster (old) | Session (new) |
|-------------------|---------------|
| `Send()` re-renders immediately | `TriggerAction()` calls `Change()` first |
| Single connection only | ALL user's connections |
| Direct state manipulation | Action-based state changes |
| No distributed support | Redis PubSub support |

## Examples

See these examples for complete implementations:
- `examples/login/` - Authentication with server-initiated welcome message
- `examples/chat/` - Real-time chat with auto-sync
- `examples/counter/` - Simple counter demonstrating multi-tab sync

## See Also

- [Authentication Reference](authentication.md) - User identification and custom authenticators
- [Scaling Guide](../SCALING.md) - Horizontal scaling with Redis
- [Error Handling](error-handling.md) - Validation and error display
