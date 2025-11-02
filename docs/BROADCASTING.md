# Broadcasting API

LiveTemplate provides two types of broadcasting:
1. **Automatic Session Syncing** - Tabs in the same browser automatically stay in sync (no code needed)
2. **Manual Broadcasting** - Explicit control for cross-session scenarios

## Automatic Session Syncing (Default Behavior)

When a user performs an action that modifies state, **all tabs in the same browser session automatically receive updates**. This happens with zero configuration:

```go
type ChatState struct {
    Messages []Message
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    s.Messages = append(s.Messages, newMessage)
    return nil  // All tabs in same browser update automatically! ✨
}
```

**How it works:**
- Each browser gets a unique session ID (via cookie: `livetemplate-id`)
- All tabs in the same browser share this session ID
- State changes automatically broadcast to all tabs in the same session
- No manual broadcasting code required

**Example:** Chat app where multiple tabs stay in sync:
```go
// Tab 1: User sends message
// Tab 2, Tab 3: Automatically see the new message
```

See `examples/chat/` for a complete demonstration.

## Manual Broadcasting

For cross-session scenarios (system announcements, user notifications, pub/sub topics), use the `LiveHandler` interface:

```go
// Create template and handler
tmpl := livetemplate.New("app")
handler := tmpl.Handle(&AppState{})  // Returns LiveHandler

// Broadcast to all connections (all browsers, all sessions)
handler.Broadcast(data)

// Broadcast to specific users across all their sessions
handler.BroadcastToUsers([]string{"user-123", "user-456"}, data)

// Broadcast to specific session group or topic
handler.BroadcastToGroup("topic:crypto-prices", data)
```

## LiveHandler Interface

```go
type LiveHandler interface {
    http.Handler

    // Broadcast sends updates to all connected clients
    Broadcast(data interface{}) error

    // BroadcastToUsers sends updates to specific users across all their connections
    BroadcastToUsers(userIDs []string, data interface{}) error

    // BroadcastToGroup sends updates to all connections in a session group
    BroadcastToGroup(groupID string, data interface{}) error
}
```

## Broadcasting Methods

### Broadcast()

Sends updates to **all connected clients** across all session groups.

**Use Cases:**
- System-wide announcements
- Global data updates (stock prices, weather, etc.)
- Admin broadcasts

**Example:**
```go
// In a background goroutine
go func() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        data := fetchLatestData()
        handler.Broadcast(data)
    }
}()
```

**Behavior:**
- Sends to ALL active WebSocket connections
- Each connection uses its own cloned template for tree diffing
- Errors logged but don't stop other sends
- Returns error if any sends fail (check logs for details)

### BroadcastToUsers()

Sends updates to **specific users** across all their active connections.

**Use Cases:**
- User-specific notifications
- Multi-device updates (desktop + mobile)
- Targeted messaging

**Example:**
```go
// Notify users about a new message
func notifyNewMessage(handler livetemplate.LiveHandler, recipients []string) {
    notification := &Notification{
        Message: "You have a new message",
        Time:    time.Now(),
    }
    handler.BroadcastToUsers(recipients, notification)
}
```

**Behavior:**
- Sends to all connections for specified userIDs
- One user may have multiple connections (different tabs/devices)
- Empty userIDs list returns error
- Non-existent users silently skipped (no error)

### BroadcastToGroup()

Sends updates to **all connections in a session group**. Typically used for pub/sub topics or channels, not multi-tab syncing (which is automatic).

**Use Cases:**
- Pub/sub topics (e.g., "topic:crypto-prices", "topic:sports")
- Chat rooms (e.g., "room:lobby", "room:support")
- Collaborative workspaces (e.g., "workspace:123")

**Example:**
```go
// Publish price update to crypto topic subscribers
func publishCryptoPrice(handler livetemplate.LiveHandler, price CryptoPrice) {
    handler.BroadcastToGroup("topic:crypto-prices", price)
}

// Custom authenticator for topic subscriptions
type TopicAuthenticator struct{}

func (a *TopicAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    topic := r.URL.Query().Get("topic")
    return "topic:" + topic, nil  // Users subscribe by connecting with ?topic=crypto-prices
}
```

**Behavior:**
- Sends to all connections with matching groupID
- Empty groupID returns error
- Non-existent group silently skipped (no error)

## Authentication & Session Groups

Session groups determine which tabs automatically stay in sync:

### Anonymous Users (Default)

```go
// Default: AnonymousAuthenticator
tmpl := livetemplate.New("app")
handler := tmpl.Handle(&state)

// Each browser gets unique groupID (via cookie)
// All tabs in same browser share groupID
```

**Session Grouping:**
- Browser A, Tab 1: `groupID = session-abc` (from cookie)
- Browser A, Tab 2: `groupID = session-abc` (same cookie → same state, auto-sync)
- Browser B, Tab 1: `groupID = session-xyz` (different cookie → isolated state)

**Automatic Syncing:**
- Tabs 1 & 2 in Browser A automatically sync (same groupID)
- Browser B is isolated (different groupID)

**Manual Broadcast Behavior:**
- `Broadcast()` → All tabs in all browsers
- `BroadcastToUsers()` → N/A (users are anonymous)
- `BroadcastToGroup("session-abc")` → Both tabs in Browser A (rarely needed, already auto-synced)

### Authenticated Users

```go
auth := livetemplate.NewBasicAuthenticator(validateUser)
tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
handler := tmpl.Handle(&state)
```

**Session Grouping (BasicAuthenticator uses userID as groupID):**
- User "alice", Desktop: `groupID = alice`
- User "alice", Mobile: `groupID = alice` (same groupID → auto-sync across devices!)
- User "bob", Desktop: `groupID = bob` (different user → isolated)

**Automatic Syncing:**
- Alice's desktop and mobile automatically sync (same groupID)
- Bob is isolated (different groupID)

**Manual Broadcast Behavior:**
- `Broadcast()` → All devices for all users
- `BroadcastToUsers(["alice"])` → Desktop + Mobile for alice (rarely needed, already auto-synced)
- `BroadcastToGroup("alice")` → Same as BroadcastToUsers for BasicAuthenticator

## Thread Safety

All broadcasting methods are **thread-safe** and can be called concurrently:

```go
// Safe: Multiple goroutines broadcasting
go handler.Broadcast(data1)
go handler.BroadcastToUsers(users, data2)
go handler.BroadcastToGroup(group, data3)
```

The ConnectionRegistry uses `sync.RWMutex` for safe concurrent access.

## Error Handling

### Partial Failures

Broadcasting continues even if individual sends fail:

```go
// 3 connections: A, B, C
// B fails (connection closed)
// A and C still receive the update
// Error returned: "broadcast failed for 1/3 connections"
```

Check logs for details:
```
2025/10/20 12:34:56 Broadcast: Failed to send to connection user-123: websocket: close sent
```

### Best Practices

```go
// Always check errors in production
if err := handler.Broadcast(data); err != nil {
    log.Printf("Broadcast error: %v", err)
    // Optional: retry logic, alerting, etc.
}

// Empty checks return errors
if err := handler.BroadcastToUsers([]string{}, data); err != nil {
    // Error: "no user IDs provided"
}

if err := handler.BroadcastToGroup("", data); err != nil {
    // Error: "group ID cannot be empty"
}
```

## Performance Considerations

### Tree Diffing Per Connection

Each connection maintains its own template state:

```go
// Connection A: lastData = {Count: 5}
// Connection B: lastData = {Count: 10}

handler.Broadcast(&State{Count: 15})

// Connection A: sends update from 5→15
// Connection B: sends update from 10→15
// Different tree diffs for same broadcast!
```

This ensures:
- Independent state tracking
- Efficient updates (only what changed)
- No shared state conflicts

### Broadcasting Frequency

**Guidelines:**
- **High frequency** (<100ms): Use only for critical real-time data
- **Medium frequency** (1-5s): Suitable for most live updates
- **Low frequency** (>5s): Recommended for background sync

**Example:**
```go
// Good: Throttled updates
ticker := time.NewTicker(1 * time.Second)
for range ticker.C {
    handler.Broadcast(data)
}

// Bad: Unthrottled updates in tight loop
for {
    handler.Broadcast(data)  // DON'T DO THIS
}
```

### Connection Limits

**Considerations:**
- Each connection uses memory for WebSocket + template state
- Typical limit: 1000-10000 concurrent connections per server
- For higher scale, use horizontal scaling with Redis SessionStore

## Common Patterns

### Background Job Broadcasting

```go
func startBackgroundUpdates(handler livetemplate.LiveHandler) {
    go func() {
        for {
            time.Sleep(10 * time.Second)

            // Fetch latest data
            data := fetchFromDatabase()

            // Broadcast to all
            if err := handler.Broadcast(data); err != nil {
                log.Printf("Broadcast failed: %v", err)
            }
        }
    }()
}
```

### Webhook Broadcasting

```go
func handleWebhook(w http.ResponseWriter, r *http.Request, handler livetemplate.LiveHandler) {
    // Parse webhook payload
    var payload WebhookData
    json.NewDecoder(r.Body).Decode(&payload)

    // Broadcast to affected users
    handler.BroadcastToUsers(payload.UserIDs, payload.Data)

    w.WriteHeader(http.StatusOK)
}
```

### Room-Based Broadcasting

```go
type ChatRoom struct {
    RoomID  string
    Handler livetemplate.LiveHandler
}

func (r *ChatRoom) SendMessage(msg Message) {
    // Broadcast to all users in this room
    r.Handler.BroadcastToGroup(r.RoomID, msg)
}
```

### Conditional Broadcasting

```go
// Broadcast only to premium users
func broadcastToPremium(handler livetemplate.LiveHandler, premiumUsers []string, data interface{}) {
    if len(premiumUsers) > 0 {
        handler.BroadcastToUsers(premiumUsers, data)
    }
}
```

## Testing

### Unit Testing

Broadcasting works in tests with nil WebSocket connections:

```go
func TestBroadcast(t *testing.T) {
    tmpl := livetemplate.New("test")
    handler := tmpl.Handle(&State{})

    // Broadcast with no connections (safe)
    err := handler.Broadcast(&State{Value: 42})
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
}
```

### Integration Testing

See `broadcast_test.go` for comprehensive examples:
- TestLiveHandler_Broadcast - all connections
- TestLiveHandler_BroadcastToUsers - specific users
- TestLiveHandler_BroadcastConcurrent - concurrent broadcasting

## Examples

### Real-Time Chat

See `examples/chat/` for a complete multi-user chat application demonstrating:
- Message broadcasting to all users
- User presence tracking
- Multi-tab session sharing

### Live Dashboard

```go
type DashboardState struct {
    Metrics map[string]int
    Alerts  []Alert
}

func (s *DashboardState) Change(ctx *livetemplate.ActionContext) error {
    // Handle user actions
    return nil
}

func main() {
    tmpl := livetemplate.New("dashboard")
    handler := tmpl.Handle(&DashboardState{})

    // Background: Update metrics every 5 seconds
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        for range ticker.C {
            metrics := fetchMetrics()
            handler.Broadcast(&DashboardState{Metrics: metrics})
        }
    }()

    http.Handle("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

## Migration Guide

### From Manual WebSocket Management

**Before:**
```go
// Manual WebSocket tracking
var connections []*websocket.Conn
mu sync.Mutex

// Manual broadcasting
for _, conn := range connections {
    conn.WriteJSON(data)
}
```

**After:**
```go
handler := tmpl.Handle(&state)
handler.Broadcast(data)  // That's it!
```

## Broadcast Guarantees & Semantics

### Single-Instance Deployments

In single-instance deployments (without Redis Pub/Sub), broadcasts have **at-least-once** delivery semantics:

- **Guaranteed Delivery**: All active local connections receive the broadcast
- **Synchronous**: Broadcast completes before returning (blocking)
- **Failure Handling**: Individual connection failures are logged but don't stop other sends
- **Ordering**: Messages are delivered in the order they were broadcast

### Multi-Instance Deployments (Redis Pub/Sub)

When using distributed broadcasting with Redis Pub/Sub, the system provides **at-most-once** delivery semantics:

#### Delivery Guarantees

**What is Guaranteed:**
- ✅ Messages published to Redis will be delivered to all subscribed instances
- ✅ Local connections on each instance will receive the broadcast
- ✅ Local-first optimization: same-instance connections receive immediate delivery
- ✅ Message ordering within a single publisher is preserved

**What is NOT Guaranteed:**
- ❌ **No Delivery Confirmation**: Redis Pub/Sub doesn't confirm message delivery
- ❌ **No Message Persistence**: Messages are lost if no instances are subscribed
- ❌ **No Retry on Failure**: If an instance is disconnected, it misses messages
- ❌ **No Global Ordering**: Messages from different instances may arrive in different orders

#### Why At-Most-Once?

Redis Pub/Sub is designed for real-time, fire-and-forget messaging:

1. **No Message Queue**: Messages are not persisted - they're delivered to active subscribers only
2. **No ACKs**: Subscribers don't acknowledge receipt
3. **No Replay**: Missed messages cannot be replayed
4. **Performance Trade-off**: This design enables low-latency, high-throughput broadcasting

#### Best Practices for Multi-Instance Broadcasting

**✅ DO:**
- Use broadcasts for **ephemeral updates** (UI refreshes, live data)
- Implement **client-side state reconciliation** when reconnecting
- Design for **eventual consistency** across instances
- Use broadcasts for **real-time notifications** that can be missed
- Rely on the database as the **source of truth**

**❌ DON'T:**
- Don't rely on broadcasts for **critical state changes**
- Don't assume **all instances receive all messages**
- Don't use broadcasts for **transactions** or **workflows**
- Don't expect **message ordering** across instances

#### Example: Handling Missed Broadcasts

```go
// GOOD: Client reconciles state on reconnect
type ChatStore struct {
    Messages []Message
    LastSync time.Time
}

func (s *ChatStore) OnReconnect(ctx *ActionContext) error {
    // Fetch any messages missed during disconnect
    newMessages := fetchMessagesAfter(s.LastSync)
    s.Messages = append(s.Messages, newMessages...)
    s.LastSync = time.Now()
    return nil
}

// Broadcasts are used for real-time updates only
// Database is the source of truth for missed messages
```

#### Monitoring & Observability

**Recommended Metrics:**
- Broadcast success rate (local connections)
- Redis publish latency
- Redis connection health
- Message fan-out time (instance receive → local delivery)

**Logging:**
- Individual connection failures are logged (non-fatal)
- Redis publish failures are logged (degrades to local-only)
- Reconnection events are logged

### Comparison Table

| Aspect | Single-Instance | Multi-Instance (Redis) |
|--------|----------------|----------------------|
| Delivery Guarantee | At-least-once | At-most-once |
| Message Persistence | N/A | None |
| Network Partition | N/A | Messages lost |
| Latency | <1ms | 5-50ms (p95) |
| Ordering | Guaranteed | Per-publisher only |
| Failure Mode | Log errors | Degrade to local |

### Design Recommendations

**For Critical Operations:**
```go
// Use database + broadcasts for critical data
func (s *Store) UpdateCriticalData(ctx *ActionContext) error {
    // 1. Update database (source of truth)
    if err := db.Update(s.Data); err != nil {
        return err
    }

    // 2. Broadcast for real-time UI updates (best-effort)
    ctx.Broadcast(s)

    // 3. Clients reconcile with database on reconnect
    return nil
}
```

**For Real-Time Updates:**
```go
// Broadcasts are perfect for real-time, non-critical updates
func (s *LiveDashboard) RefreshMetrics(ctx *ActionContext) error {
    s.Metrics = fetchLatestMetrics()
    ctx.Broadcast(s) // Fire-and-forget, eventual consistency is fine
    return nil
}
```

## See Also

- [Multi-Session Isolation Design](./design/multi-session-isolation.md)
- [Authentication Guide](./AUTHENTICATION.md)
- [Examples](../examples/)
- [API Reference](./API.md)
