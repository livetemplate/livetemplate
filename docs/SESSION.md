# Session API

LiveTemplate provides two types of updates:
1. **Automatic Session Syncing** - Tabs in the same browser automatically stay in sync (no code needed)
2. **Server-Initiated Actions** - Push updates from server-side code (timers, webhooks, background jobs)

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
- No manual code required

**Example:** Chat app where multiple tabs stay in sync:
```go
// Tab 1: User sends message
// Tab 2, Tab 3: Automatically see the new message
```

See `examples/chat/` for a complete demonstration.

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
1. WebSocket connection established → `OnConnect()` called with `Session`
2. Store the `Session` for later use (e.g., in background goroutines)
3. WebSocket connection closed → `OnDisconnect()` called
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

## Distributed Deployments (Redis)

In multi-instance deployments with Redis PubSub configured, `TriggerAction()` automatically publishes to Redis so all instances can update their local connections for the user:

```go
// Instance 1: User connects here
session.TriggerAction("update", nil)

// Instance 2: If user has tabs here, they also receive the update
```

This happens transparently - no code changes needed.

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

- [Authentication Guide](./AUTHENTICATION.md)
- [Scaling Guide](./SCALING.md)
- [API Reference](./API.md)
