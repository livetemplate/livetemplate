# Server Actions Reference

Server actions let you push updates from server-side code to connected clients. Use them for timers, webhooks, background job notifications, real-time data feeds, and any scenario where the server initiates a UI update.

## Overview

LiveTemplate supports two types of updates:

| Type | Trigger | Scope | Use Case |
|------|---------|-------|----------|
| **Client Action** | User interaction (click, submit) | Same session group | Form submissions, button clicks |
| **Server Action** | Server-side code | All user's connections | Timers, webhooks, background jobs |

Server actions use the `Session` interface to trigger updates:

```go
// From any goroutine - timer, webhook handler, background job
session.TriggerAction("notification", map[string]interface{}{
    "message": "Your export is ready!",
})
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

To receive the `Session` reference, implement `SessionAware` on your store:

```go
type SessionAware interface {
    OnConnect(ctx context.Context, session Session) error
    OnDisconnect()
}
```

**Lifecycle:**

```
1. WebSocket connection established
   └─► OnConnect(ctx, session) called
       └─► Store the session for later use

2. Connection active
   └─► Use session.TriggerAction() from background goroutines

3. WebSocket connection closed
   └─► OnDisconnect() called
       └─► Clean up session reference
```

**Context (`ctx`):**
- Contains cancellation signal - cancelled when WebSocket disconnects
- Use for background goroutines to know when to stop
- Pass to database calls for timeout/cancellation support

### Complete Example

```go
type TimerStore struct {
    Seconds int
    session livetemplate.Session
    mu      sync.Mutex
}

func (s *TimerStore) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.mu.Lock()
    s.session = session
    s.mu.Unlock()

    // Start background timer
    go s.runTimer(ctx)
    return nil
}

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
            return // Connection closed
        case <-ticker.C:
            s.mu.Lock()
            session := s.session
            s.mu.Unlock()

            if session != nil {
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

## Common Patterns

### Timer/Tick Updates

Periodic updates (dashboards, live data, countdowns):

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

External events pushing updates to users:

```go
// HTTP handler receives webhook from external service
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var payload WebhookPayload
    json.NewDecoder(r.Body).Decode(&payload)

    // Get session for target user (stored during OnConnect)
    if session := getUserSession(payload.UserID); session != nil {
        session.TriggerAction("notification", map[string]interface{}{
            "message": payload.Message,
            "type":    "webhook",
        })
    }

    w.WriteHeader(http.StatusOK)
}
```

### Welcome Message After Connect

Greet users after page loads:

```go
func (s *AuthStore) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.session = session

    if s.IsLoggedIn {
        // Send welcome after short delay (let page render first)
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
    }
    return nil
}
```

### Background Job Completion

Notify users when async jobs finish:

```go
func (s *Store) Change(ctx *livetemplate.ActionContext) error {
    if ctx.Action == "startExport" {
        // Start background job
        go func() {
            result, err := performLongRunningExport()

            if s.session != nil {
                if err != nil {
                    s.session.TriggerAction("exportFailed", map[string]interface{}{
                        "error": err.Error(),
                    })
                } else {
                    s.session.TriggerAction("exportComplete", map[string]interface{}{
                        "downloadURL": result.URL,
                    })
                }
            }
        }()

        s.ExportStatus = "Processing..."
    }
    return nil
}
```

### Real-time Notifications

Push notifications from any part of your application:

```go
// Global session registry (thread-safe)
var userSessions = sync.Map{}

func (s *Store) OnConnect(ctx context.Context, session livetemplate.Session) error {
    s.session = session
    userSessions.Store(s.UserID, session)
    return nil
}

func (s *Store) OnDisconnect() {
    userSessions.Delete(s.UserID)
    s.session = nil
}

// Call from anywhere in your application
func NotifyUser(userID string, message string) {
    if session, ok := userSessions.Load(userID); ok {
        session.(livetemplate.Session).TriggerAction("notification", map[string]interface{}{
            "message": message,
        })
    }
}
```

## Thread Safety

Session methods are thread-safe and can be called from any goroutine:

```go
// Safe: Multiple goroutines using session concurrently
go func() { session.TriggerAction("update1", nil) }()
go func() { session.TriggerAction("update2", nil) }()
```

However, you must protect access to the session field itself:

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

## Multi-Tab/Multi-Device Behavior

When a user has multiple tabs or devices connected:

**Client Action (from Tab 1):**
```
User clicks button in Tab 1
    └─► Tab 1's store.Change() called
        └─► Tab 1 receives update
        └─► Tab 2, Tab 3 automatically receive update (auto-broadcast)
```

**Server Action (TriggerAction):**
```
Background job completes
    └─► session.TriggerAction("jobComplete", data)
        └─► ALL tabs receive the action via Change()
        └─► ALL tabs are updated simultaneously
```

## Distributed Deployments

In multi-instance deployments with Redis PubSub configured, `TriggerAction()` automatically publishes to Redis so all instances can update their local connections for the user:

```go
// Configure Redis PubSub
tmpl := livetemplate.New("app",
    livetemplate.WithPubSubBroadcaster(redisBroadcaster),
)

// Instance 1: User connects here
session.TriggerAction("update", nil)

// Instance 2: If user has tabs here, they also receive the update
// (Happens transparently via Redis PubSub)
```

This happens transparently - no code changes needed in your stores.

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
    s.session.TriggerAction("refresh", nil)
}

func (s *Store) Change(ctx *livetemplate.ActionContext) error {
    if ctx.Action == "refresh" {
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

- [Session Reference](session.md) - Session stores and connection management
- [Authentication Reference](authentication.md) - User identification and custom authenticators
- [Scaling Guide](../SCALING.md) - Horizontal scaling with Redis
