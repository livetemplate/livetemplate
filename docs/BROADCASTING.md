# Broadcasting API

LiveTemplate supports server-initiated updates via the `LiveHandler` interface, enabling real-time broadcasting to connected WebSocket clients.

## Quick Start

```go
// Create template and handler
tmpl := livetemplate.New("app")
handler := tmpl.Handle(&AppState{})  // Returns LiveHandler

// Broadcast to all connections
handler.Broadcast(data)

// Broadcast to specific users
handler.BroadcastToUsers([]string{"user-123", "user-456"}, data)

// Broadcast to session group
handler.BroadcastToGroup("session-abc", data)
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

Sends updates to **all connections in a session group**.

**Use Cases:**
- Multi-tab updates for anonymous users
- Shared session updates
- Room/channel broadcasts

**Example:**
```go
// Update all tabs for a session
func updateSession(handler livetemplate.LiveHandler, groupID string, data interface{}) {
    handler.BroadcastToGroup(groupID, data)
}
```

**Behavior:**
- Sends to all connections with matching groupID
- Empty groupID returns error
- Non-existent group silently skipped (no error)

## Authentication & Session Groups

Broadcasting works with LiveTemplate's authentication system:

### Anonymous Users (Default)

```go
// Default: AnonymousAuthenticator
tmpl := livetemplate.New("app")
handler := tmpl.Handle(&state)

// Each browser gets unique groupID (via cookie)
// All tabs in same browser share groupID
```

**Session Grouping:**
- Browser A, Tab 1: `group-abc` (from cookie)
- Browser A, Tab 2: `group-abc` (same cookie)
- Browser B, Tab 1: `group-xyz` (different cookie)

**Broadcast Behavior:**
- `Broadcast()` → All tabs in all browsers
- `BroadcastToUsers()` → N/A (users are anonymous)
- `BroadcastToGroup("group-abc")` → Both tabs in Browser A

### Authenticated Users

```go
auth := livetemplate.NewBasicAuthenticator(validateUser)
tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
handler := tmpl.Handle(&state)
```

**Session Grouping:**
- User "alice", Desktop: `group-alice-1`
- User "alice", Mobile: `group-alice-2`
- User "bob", Desktop: `group-bob-1`

**Broadcast Behavior:**
- `Broadcast()` → All devices for all users
- `BroadcastToUsers(["alice"])` → Desktop + Mobile for alice
- `BroadcastToGroup("group-alice-1")` → Desktop only for alice

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

### From Mount() to Handle()

**Before:**
```go
http.Handle("/", livetemplate.Mount(tmpl, &state))
```

**After:**
```go
handler := tmpl.Handle(&state)
http.Handle("/", handler)

// Now you can broadcast!
handler.Broadcast(data)
```

## See Also

- [Multi-Session Isolation Design](./design/multi-session-isolation.md)
- [Authentication Guide](./AUTHENTICATION.md)
- [Examples](../examples/)
- [API Reference](./API.md)
