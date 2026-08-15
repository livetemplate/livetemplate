# Session Reference

Session infrastructure in LiveTemplate handles state storage, connection management, and WebSocket configuration. Below: the state safety guarantees, the session stores, connection management, and what to tune before production.

For pushing updates from server-side code, see [Server Actions Reference](server-actions.md).

## Overview

### Key Concepts

- **Session groups**: Isolation boundaries for shared state. All connections with the same `groupID` share the same state instance.
- **State**: Application state cloned per session group via `AsState()`
- **Connections**: Individual WebSocket connections within a group
- **Session store**: Persistence layer for session groups (in-memory or Redis)

### State Persistence

State persistence is controlled at the field level using the `lvt:"persist"` struct tag. Fields tagged with `lvt:"persist"` are serialized to SessionStore after every successful action (WebSocket, HTTP POST, dispatched, server-initiated). Fields without the tag are ephemeral -- they start at their zero value on each page load and are populated by `Mount()` from the database, URL params, or other sources.

The SessionStore is only consulted when at least one field in the state struct has the `lvt:"persist"` tag. If no fields are tagged, the state is fully ephemeral -- equivalent to the old `WithEphemeralState()` behavior -- and the SessionStore is never read from or written to.

```go
type TodoState struct {
    // Persisted -- survives page refresh
    Filter string `json:"filter" lvt:"persist"`
    Page   int    `json:"page" lvt:"persist"`

    // Ephemeral -- loaded fresh by Mount()
    Items      []Todo `json:"items"`
    TotalCount int    `json:"total_count"`
}
```

In this example, `Filter` and `Page` survive page refreshes because they are persisted to the SessionStore. `Items` and `TotalCount` are ephemeral -- they start as zero values and are loaded fresh by `Mount()` on every request, ensuring data is always current from the database.

**Why selective persistence?** LiveTemplate supports both WebSocket and HTTP-only modes. In WebSocket mode, state naturally lives in memory for the lifetime of the connection. In HTTP-only mode (progressive enhancement), each request is stateless -- without persistence, form submissions via POST-Redirect-GET would lose UI state like filters and pagination. Selective persistence lets you persist only the lightweight UI state that needs to survive page refreshes, while keeping database-sourced data ephemeral and always fresh from `Mount()`.

**Mount() lifecycle:** `Mount()` is called on every HTTP request (GET and POST) and every WebSocket connect (new and reconnect). It receives the current state (with persisted fields restored, ephemeral fields at zero value) and returns refreshed state. This ensures data is always fresh from the database. Keep Mount cheap -- it runs on every request.

**Mount on POST:** Since Mount runs before actions on HTTP POST, it must populate ephemeral fields (e.g., `state.Items = db.GetItems(state.Filter)`). Persisted fields like `Filter` and `Page` are already restored from the SessionStore before Mount receives the state. Guard side effects with `ctx.IsInitialMount()` (preferred — true only on HTTP GET) or the older `ctx.Action() == ""` (true on GET, internal navigate POSTs, and WS connects/reconnects).

### State Persistence Matrix

Fields tagged with `lvt:"persist"` follow this persistence schedule. Untagged fields are always ephemeral (zero value on load, populated by `Mount()`). If no fields have the `lvt:"persist"` tag, the SessionStore is never consulted.

| Operation | `lvt:"persist"` fields | Untagged fields |
|-----------|----------------------|-----------------|
| Mount() (new session) | Persisted | Zero value, loaded by Mount() |
| Mount() (HTTP GET) | Restored from store, then persisted | Zero value, loaded by Mount() |
| Mount() (HTTP POST) | Restored from store, then persisted | Zero value, loaded by Mount() |
| Mount() (WS reconnect) | Restored from store, then persisted | Zero value, loaded by Mount() |
| OnConnect() (WS connect) | Not persisted | Not persisted |
| HTTP POST action | Persisted on success | In-memory only, discarded after response |
| WebSocket action | Persisted on success | In-memory for connection lifetime |
| Dispatched action | Persisted on success | In-memory for connection lifetime |
| Server action | Persisted (once per group) | In-memory for connection lifetime |
| Page refresh | Restored from store | Zero value, loaded by Mount() |

### Persistence and Path Navigation

`lvt:"persist"` fields survive a **same-URL page refresh** (F5) and a **WebSocket reconnect**. They do **not** survive navigation to a different path:

- Persistence is keyed by `groupID`, not by URL path.
- When a GET request arrives on a path different from the group's last-seen path, LiveTemplate resets to fresh state for the new URL (persist fields go back to their zero values, then `Mount()` runs) and **overwrites** the group's stored persist fields with that fresh state.
- Because the store is overwritten on navigation, navigating **back** to the original path does **not** restore the previously persisted values — they were replaced. Persist fields only ever round-trip across a refresh/reconnect on the *same* path.

This matches the pre-`lvt:"persist"` behavior where a path change always meant fresh state. Treat `lvt:"persist"` as "survives refresh," not "survives navigation."

### What the session store is not for

`lvt:"persist"` and the `SessionStore` provide **session continuity** — carrying UI state across a refresh or reconnect within a session group. They are not a durable application datastore:

- The default `MemorySessionStore` is in-process. A **server relaunch loses every session**; the browser's cookie survives but now misses the fresh empty store, so persist fields fall back to their zero values and `Mount()` repopulates them. `RedisSessionStore` survives a relaunch, but its purpose is shared continuity across instances, not per-user durable storage.
- Persistence is keyed by the cookie-derived `groupID`. State stored there is scoped to *that browser session*, not to a user — the same person on a second device (a different cookie) gets a different group.

So data that must **outlive a process relaunch** or mean the same thing **across a user's sessions and devices** — durable user preferences, saved documents, anything you'd call "app data" — belongs in your own store (a file, your database), loaded in `Mount()`. Keep only genuine session-continuity state (filters, pagination, an in-progress draft) on `lvt:"persist"`. Mixing the two is the trap: a preference parked in the session silently resets on the next relaunch. The framework deliberately owns session continuity and leaves durable app data to the app.

### Explicit Peer Refresh

```go
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    _ = ctx.Subscribe(ctx.SelfTopic()) // opt in to peer fan-out for this session
    state.Items = c.DB.GetItems(ctx.UserID())
    return state, nil
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    c.DB.AddItem(ctx.UserID(), ctx.GetString("text"))
    state.Items = c.DB.GetItems(ctx.UserID())
    ctx.Publish(ctx.SelfTopic(), "RefreshTodos", nil)
    return state, nil
}

func (c *TodoController) RefreshTodos(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.Items = c.DB.GetItems(ctx.UserID())
    return state, nil
}
```

**How it works:**
- Each browser gets a unique session ID (via cookie: `livetemplate-id`)
- All tabs in the same browser share this session ID (`groupID`); `ctx.SelfTopic()` resolves to the reserved-namespace topic `lvt:session:<groupID>` for that session
- `Mount` subscribes the calling connection to `SelfTopic()` — this opt-in is what makes the connection a peer-receiver; without it, nothing reaches the tab
- After a successful action, queued `ctx.Publish` calls fan out to every subscriber of the named topic in the group
- Each peer connection runs the named action with its own state, reloading from the database
- If no peer subscribed (or the action did not `Publish`), no cross-tab dispatch occurs

## State Safety

LiveTemplate ensures safe state management through two dimensions: **purity enforcement** (preventing dependency types in state) and **session isolation** (preventing cross-user data leakage). For the full Controller+State pattern, see the [Controller+State Pattern Reference](controller-pattern.md).

### State Purity

State purity is enforced through four layers. Each layer catches different classes of mistakes.

#### Layer 1: Compile-Time Type Separation

Controllers and state are separate types. Action methods receive state by value and return a modified copy:

```go
func (c *Controller) Action(state State, ctx *livetemplate.Context) (State, error)
```

The controller is a singleton holding shared dependencies (DB, Logger). State is pure data cloned per session. Because state is passed by value, mutations in the action body don't affect the original — the framework only applies the returned copy.

#### Layer 2: Runtime Dependency Detection

`AsState[T]()` validates the state type at registration time using `validatePureState[T]()`. This performs a recursive descent through:

- Direct struct fields
- Nested structs and pointer-to-struct fields
- Slice, array, and map element types

If a dependency type is found, `AsState` **panics immediately** with an actionable message:

```
livetemplate.AsState: field DB appears to be a dependency (*sql.DB) - move to controller
```

**Detected Dependency Patterns:**

| Pattern | Category |
|---------|----------|
| `*sql.DB` | Database |
| `*sql.Tx` | Database |
| `*sql.Conn` | Database |
| `*slog.Logger` | Logging |
| `*log.Logger` | Logging |
| `*http.Client` | Network |
| `*redis.Client` | Cache |
| `io.Writer` | I/O |
| `io.Reader` | I/O |

Detection is heuristic — it matches these 9 known dependency patterns by comparing the reflected type string (for example, `*sql.DB`). Struct wrappers that embed these types (e.g., `type AppDB struct{ *sql.DB }`) are still detected because validation descends into nested struct fields. Defined pointer types (e.g., `type AppDB *sql.DB`) and other third-party types (e.g., `*pgxpool.Pool`) are **not** caught by this layer. Layer 4 (the test helper) runs the same validation logic but reports failures as test errors instead of panics.

#### Layer 3: Serialization Boundary

Each new session gets a deep copy of state via JSON marshal/unmarshal. This catches non-serializable fields that pass Layer 2:

- Functions and closures
- Channels
- Unexported fields (ignored by `encoding/json`; data in these fields is not cloned or persisted)
- Circular references

If state contains functions, channels, or circular references, the JSON round-trip fails at runtime when the first session is created; unexported fields are silently omitted from the cloned and persisted state.

#### Layer 4: Test Helper

`AssertPureState[T](t)` runs the same validation as Layer 2 but fails the test instead of panicking:

```go
func TestState(t *testing.T) {
    livetemplate.AssertPureState[TodoState](t)
}
```

Add this to every state type's test file. It catches dependency leakage in CI before the code reaches production.

#### What Happens on Violation

| Violation | When Detected | Outcome |
|-----------|---------------|---------|
| Dependency type in state struct | `AsState[T]()` at handler registration | **Panic** with field name and type |
| Non-serializable field (func, chan) | First session clone at runtime | JSON marshal error |
| Dependency in test | `AssertPureState[T](t)` in test suite | Test failure with field name and type |

### Session Isolation

Session isolation ensures that user A's state is never visible to user B.

#### GroupID as Isolation Boundary

Every request — HTTP and WebSocket — goes through the `Authenticator` to compute a `groupID`:

```
Authenticator.Identify(r)           → userID
Authenticator.GetSessionGroup(r, userID) → groupID
```

The `groupID` is the authorization boundary for all state access. Users cannot specify a `groupID` directly in the URL or headers — the `Authenticator` computes it from the request's identity (cookies, auth headers).

The built-in `AnonymousAuthenticator` generates a random 256-bit `groupID` per browser (base64-encoded, stored in a `livetemplate-id` cookie). The `BasicAuthenticator` maps `groupID = userID` for authenticated users.

#### Per-Session State Cloning

Each session group gets an independent state clone via JSON serialization/deserialization. No shared pointers exist across sessions. Each WebSocket connection also gets its own template clone (`Template.Clone()`) for independent diff state, preventing one tab's updates from corrupting another tab's tree comparison.

#### SessionStore Keying

State persistence is keyed by `groupID`. No API exists to access another group's state:

- **MemorySessionStore**: Go map keyed by `groupID`
- **RedisSessionStore**: Redis hash at `livetemplate:session:{groupID}`

State is deserialized fresh on each `Get()`, preventing reference sharing across requests.

#### Broadcast Scoping

`SelfTopic()` resolves to `lvt:session:<groupID>` — scoped to the sender's own `groupID`. A `ctx.Publish(ctx.SelfTopic(), ...)` therefore reaches only connections in that same group (and only those that subscribed). The [ConnectionRegistry](#connection-registry) filters recipients via `GetByTopicExcept(topic, sender)`; different groups receive different `SelfTopic()` strings and are never informed of each other's updates.

#### HTTP Request Isolation

In the HTTP path (non-WebSocket), each session group has a per-group `httpTemplateCacheEntry` with its own mutex. Each POST request clones state, processes the action, and persists the result back to the group's SessionStore entry. The mutex serializes concurrent requests for the same group, preventing data races.

#### Isolation Summary

| Component | Isolation Key | Cross-User Leakage |
|-----------|---------------|-------------------|
| SessionStore | `groupID` | Not possible (keyed by groupID) |
| ConnectionRegistry | `groupID` (dual-indexed) | Not possible (lookup filtered) |
| Template | Per-connection clone | Not possible (independent instances) |
| State | Per-group clone via JSON | Not possible (deep copy) |
| HTTP cache | Per-group entry + mutex | Not possible (separate entries) |
| Broadcast | `groupID` in registry | Not possible (filtered by group) |

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

**`[]byte` contract for selective persistence:** when the framework persists `lvt:"persist"` fields, it always passes `[]byte` (JSON) to `Set()` and requires `Get()` to return that same `[]byte` (or `nil`) unchanged. A custom `SessionStore` that transforms the value on the round-trip — for example, JSON-unmarshalling it into a `map` — breaks persistence: persist fields silently fall back to fresh state on every request (a warning is logged, no error returned). Both built-in stores honor this automatically; see the `SessionStore` doc comment for the full contract.

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

## Limitations

- **Dependency detection is heuristic**: Only catches 9 known dependency patterns (stdlib types like `*sql.DB` plus common third-party types like `*redis.Client`). `AssertPureState[T](t)` uses the same `validatePureState` heuristics as `AsState`—it helps you catch the *same* issues earlier in CI / tests without panicking, but it does not broaden detection. Truly unknown custom wrappers or third-party types will not be flagged unless you extend the framework's pattern list or add custom validation.
- **Warning — Session isolation depends on Authenticator**: A custom `Authenticator` that returns the same `groupID` for different users would break isolation. Use the built-in authenticators or ensure `GetSessionGroup` maps distinct users to distinct groups.
- **JSON serialization overhead**: State cloning involves a JSON round-trip per session. Keep state structs small for best performance.

See [Current Limitations](current-limitations.md) for the full limitations reference.

## See Also

- [Controller+State Pattern](controller-pattern.md) - Full pattern reference with lifecycle methods and examples
- [Server Actions Reference](server-actions.md) - TriggerAction API for server-initiated updates
- [Authentication Reference](authentication.md) - User identification and custom authenticators
- [Current Limitations](current-limitations.md) - All known limitations and workarounds
- [Scaling Guide](../guides/SCALING.md) - Horizontal scaling with Redis
