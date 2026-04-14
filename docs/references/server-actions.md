# Server Actions Reference

Server actions let you push updates from server-side code to connected clients. Use them for timers, webhooks, background job notifications, real-time data feeds, and any scenario where the server initiates a UI update.

## Overview

LiveTemplate supports two types of updates:

| Type | Trigger | Scope | Use Case |
|------|---------|-------|----------|
| **Client Action** | User interaction (click, submit) | Same session group | Form submissions, button clicks |
| **Server Action** | Server-side code | Same session group | Timers, webhooks, background jobs |

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
    // TriggerAction dispatches the action to the controller,
    // then sends the updated template to ALL connections for this user.
    TriggerAction(action string, data map[string]interface{}) error
}
```

**Key Points:**
- `TriggerAction()` calls your action method just like client-initiated actions
- Updates are sent to ALL connections in the current session group
  (typically all tabs of the browser session; for authenticated flows
  the group mapping depends on the `Authenticator` — see [API Reference](api-reference.md#session))
- Scoped to a session group only — cannot target other groups or other users
- Thread-safe - can be called from any goroutine

## Getting the Session Reference

Access the Session through the `OnConnect` lifecycle method on your controller:

```go
type TimerController struct {
    session livetemplate.Session
    mu      sync.Mutex
}

func (c *TimerController) OnConnect(state TimerState, ctx *livetemplate.Context) (TimerState, error) {
    c.mu.Lock()
    c.session = ctx.Session()
    c.mu.Unlock()

    // Start background timer
    go c.runTimer(ctx)
    return state, nil
}

func (c *TimerController) OnDisconnect() {
    c.mu.Lock()
    c.session = nil
    c.mu.Unlock()
}
```

**Lifecycle:**

```
1. WebSocket connection established
   └─► OnConnect(state, ctx) called
       └─► Store ctx.Session() for later use

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

### Complete Timer Example

```go
type TimerState struct {
    Seconds int
}

type TimerController struct {
    session livetemplate.Session
    mu      sync.Mutex
}

func (c *TimerController) OnConnect(state TimerState, ctx *livetemplate.Context) (TimerState, error) {
    c.mu.Lock()
    c.session = ctx.Session()
    c.mu.Unlock()

    // Start background timer
    go c.runTimer(ctx)
    return state, nil
}

func (c *TimerController) OnDisconnect() {
    c.mu.Lock()
    c.session = nil
    c.mu.Unlock()
}

func (c *TimerController) runTimer(ctx context.Context) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return // Connection closed
        case <-ticker.C:
            c.mu.Lock()
            session := c.session
            c.mu.Unlock()

            if session != nil {
                session.TriggerAction("tick", nil)
            }
        }
    }
}

func (c *TimerController) Tick(state TimerState, ctx *livetemplate.Context) (TimerState, error) {
    state.Seconds++
    return state, nil
}

func (c *TimerController) Reset(state TimerState, ctx *livetemplate.Context) (TimerState, error) {
    state.Seconds = 0
    return state, nil
}
```

## Common Patterns

### Timer/Tick Updates

Periodic updates (dashboards, live data, countdowns):

```go
func (c *Controller) OnConnect(state State, ctx *livetemplate.Context) (State, error) {
    c.session = ctx.Session()
    go c.runTicker(ctx)
    return state, nil
}

func (c *Controller) runTicker(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if c.session != nil {
                c.session.TriggerAction("refresh", nil)
            }
        }
    }
}

func (c *Controller) Refresh(state State, ctx *livetemplate.Context) (State, error) {
    state.Data = c.fetchLatestData()
    return state, nil
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
func (c *AuthController) OnConnect(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
    c.session = ctx.Session()

    if state.IsLoggedIn {
        // Send welcome after short delay (let page render first)
        go func() {
            time.Sleep(500 * time.Millisecond)
            if c.session != nil {
                c.session.TriggerAction("serverWelcome", map[string]interface{}{
                    "message": fmt.Sprintf("Welcome back, %s!", state.Username),
                })
            }
        }()
    }

    return state, nil
}

func (c *AuthController) ServerWelcome(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
    state.WelcomeMessage = ctx.GetString("message")
    return state, nil
}
```

### Background Job Completion

Notify users when async jobs finish. Use proper cleanup with context cancellation:

```go
type ExportState struct {
    ExportStatus string
}

type ExportController struct {
    session      livetemplate.Session
    cancelExport context.CancelFunc
    mu           sync.Mutex
}

func (c *ExportController) OnConnect(state ExportState, ctx *livetemplate.Context) (ExportState, error) {
    c.mu.Lock()
    c.session = ctx.Session()
    c.mu.Unlock()
    return state, nil
}

func (c *ExportController) OnDisconnect() {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Cancel any running export when user disconnects
    if c.cancelExport != nil {
        c.cancelExport()
        c.cancelExport = nil
    }
    c.session = nil
}

func (c *ExportController) StartExport(state ExportState, ctx *livetemplate.Context) (ExportState, error) {
    // Create cancellable context for the background job
    jobCtx, cancel := context.WithCancel(context.Background())

    c.mu.Lock()
    c.cancelExport = cancel
    c.mu.Unlock()

    go func() {
        defer cancel() // Clean up when done

        result, err := performLongRunningExport(jobCtx)

        // Check if cancelled before notifying
        select {
        case <-jobCtx.Done():
            return // User disconnected, don't notify
        default:
        }

        c.mu.Lock()
        session := c.session
        c.mu.Unlock()

        if session != nil {
            if err != nil {
                session.TriggerAction("exportFailed", map[string]interface{}{
                    "error": err.Error(),
                })
            } else {
                session.TriggerAction("exportComplete", map[string]interface{}{
                    "downloadURL": result.URL,
                })
            }
        }
    }()

    state.ExportStatus = "Processing..."
    return state, nil
}

func (c *ExportController) ExportComplete(state ExportState, ctx *livetemplate.Context) (ExportState, error) {
    state.ExportStatus = "Complete"
    state.DownloadURL = ctx.GetString("downloadURL")
    return state, nil
}

func (c *ExportController) ExportFailed(state ExportState, ctx *livetemplate.Context) (ExportState, error) {
    state.ExportStatus = "Failed: " + ctx.GetString("error")
    return state, nil
}
```

### Real-time Notifications

Push notifications from any part of your application:

```go
// Global session registry (thread-safe)
var userSessions = sync.Map{}

func (c *Controller) OnConnect(state State, ctx *livetemplate.Context) (State, error) {
    c.session = ctx.Session()
    userSessions.Store(ctx.UserID(), c.session)
    return state, nil
}

func (c *Controller) OnDisconnect() {
    userSessions.Delete(c.userID)
    c.session = nil
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
type Controller struct {
    session livetemplate.Session
    mu      sync.Mutex
}

func (c *Controller) OnConnect(state State, ctx *livetemplate.Context) (State, error) {
    c.mu.Lock()
    c.session = ctx.Session()
    c.mu.Unlock()
    return state, nil
}

func (c *Controller) OnDisconnect() {
    c.mu.Lock()
    c.session = nil
    c.mu.Unlock()
}

func (c *Controller) triggerFromBackground() {
    c.mu.Lock()
    session := c.session
    c.mu.Unlock()

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
- Safe to expose to controller logic

**Why this design?**

- Simpler mental model - "push to myself"
- No accidental cross-user data leaks
- No authorization checks needed in controller code
- For admin broadcasts, use database + polling or dedicated admin endpoints

## Multi-Tab/Multi-Device Behavior

When a user has multiple tabs or devices connected:

**Client Action (from Tab 1):**
```
User clicks button in Tab 1
    └─► Tab 1's action method called
        └─► Tab 1 receives update
        └─► Tab 2, Tab 3 receive Sync() dispatch (if controller implements Sync)
```

> When the controller implements a `Sync()` method, other tabs automatically receive a Sync dispatch after each action. Without `Sync()`, use `ctx.BroadcastAction("ActionName", nil)` for explicit cross-tab sync.

**Server Action (TriggerAction):**
```
Background job completes
    └─► session.TriggerAction("jobComplete", data)
        └─► ALL tabs receive the action via action method
        └─► ALL tabs are updated simultaneously
```

## Distributed Deployments

In multi-instance deployments, `TriggerAction()` automatically publishes to Redis so all instances can update their local connections. See the [PubSub Reference](pubsub.md) for setup, channel schema, and subscription lifecycle.

## See Also

- [Controller+State Pattern](controller-pattern.md) - Core architecture pattern
- [Session Reference](session.md) - Session stores and connection management
- [Authentication Reference](authentication.md) - User identification and custom authenticators
- [Scaling Guide](../guides/SCALING.md) - Horizontal scaling with Redis
