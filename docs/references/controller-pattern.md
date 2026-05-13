# Controller+State Pattern Reference

The Controller+State pattern separates concerns in LiveTemplate applications:
- **Controller**: Singleton that holds dependencies (DB, logger, clients) - never cloned
- **State**: Pure data that is cloned per session - automatically serialized

This separation ensures dependencies are shared correctly while state is isolated per user session.

## Overview

```go
// CONTROLLER: Singleton, holds dependencies, never cloned
type TodoController struct {
    DB     *sql.DB
    Logger *slog.Logger
}

// STATE: Pure data, cloned per session
type TodoState struct {
    Items  []Todo
    Filter string
}

// Mount handler (controller, state wrapper)
handler := tmpl.Handle(controller, livetemplate.AsState(&TodoState{}))
```

## Action Methods

Actions are automatically dispatched to methods matching the action name:

```go
// Template: <button name="add">  OR  lvt-on:click="add"
// Dispatches to: Add() method

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    title := ctx.GetString("title")
    state.Items = append(state.Items, Todo{Title: title})
    return state, nil
}
```

**Key features:**
- No boilerplate switch statements
- Method names are discoverable by IDE
- Type-safe action handlers
- Cached method lookups (O(1) after first call)

### Implicit Action Methods

LiveTemplate provides two conventional method names that are auto-routed without any explicit attributes:

**`Submit()`** — Called when a form submits with no explicit action routing (`lvt-form:action`, `button name`, or `form name`):

```go
// Template: <form method="POST"><button type="submit">Save</button></form>
// Auto-routes to Submit() because no action is specified

func (c *Controller) Submit(state State, ctx *livetemplate.Context) (State, error) {
    var input struct {
        Title string `validate:"required,min=3"`
    }
    if err := ctx.BindAndValidate(&input, validate); err != nil {
        return state, err
    }
    // process...
    return state, nil
}
```

**`Change()`** — Called when a form input with a dynamic value changes. The server detects this method and sends `capabilities: ["change"]` in the initial render; the client auto-wires debounced input events (300ms default). No `lvt-*` attributes needed. Optional:

```go
// Auto-routes when an input with value="{{.Field}}" changes
func (c *Controller) Change(state State, ctx *livetemplate.Context) (State, error) {
    if ctx.Has("Name") { state.Name = ctx.GetString("Name") }
    return state, nil
}
```

### Standard HTML Action Routing

Actions can be routed using standard HTML attributes instead of `lvt-*`:

| HTML Pattern | Routed To |
|-------------|-----------|
| `<form>` (no attributes) | `Submit()` |
| `<button name="save">` | `Save()` |
| `<form name="search">` | `Search()` (JS client only; `form.name` is not sent in a non-JS POST) |
| `lvt-form:action="create"` | `Create()` (explicit Tier 2 routing) |
| `lvt-on:click="delete"` | `Delete()` (for non-form interactions) |

### Method Signature

All action methods follow the same signature:

```go
func (c *ControllerType) ActionName(state StateType, ctx *livetemplate.Context) (StateType, error)
```

- **Receiver**: Pointer to Controller type
- **First param**: State value (copied per session)
- **Second param**: Context with action data and HTTP utilities
- **Return**: Modified state and optional error

### Naming Conventions

| Action Name | Method Name |
|-------------|-------------|
| `increment` | `Increment()` |
| `addItem` | `AddItem()` |
| `add_item` | `AddItem()` |
| `setUserProfile` | `SetUserProfile()` |

Action names are case-insensitive and support both camelCase and snake_case.

## Lifecycle Methods

LiveTemplate provides lifecycle hooks for initialization and connection management.

### Mount

Called on every HTTP request (GET and POST) and every WebSocket connect (new and reconnect):

```go
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // Load initial data
    items, err := c.DB.GetTodosForUser(ctx.UserID())
    if err != nil {
        return state, fmt.Errorf("failed to load todos: %w", err)
    }
    state.Items = items
    return state, nil
}
```

**Use cases:**
- Load initial data from database
- Set up computed fields
- Initialize state based on user context

**Guarding side effects per connect kind:** Mount runs on HTTP GET, HTTP POST actions, WebSocket new-connect, and WebSocket reconnect. Use `ctx.IsInitialMount()` to limit one-time setup (e.g. starting a background goroutine) to initial page loads, and `ctx.IsReconnect()` to detect a WebSocket reconnect that restored persisted state:

```go
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    if ctx.IsInitialMount() {
        // Only on initial HTTP GET — not POST actions, not WS connects.
        state.Loading = true
        go c.warmCacheFor(ctx.UserID())
    }
    return state, nil
}
```

```go
func (c *ChatController) OnConnect(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    if ctx.IsReconnect() {
        // Network blipped — re-announce presence, skip re-fetches the prior
        // connection already completed.
        state.SystemMessages = append(state.SystemMessages, "[reconnected]")
    }
    return state, nil
}
```

The older `ctx.Action() == ""` idiom still works (it returns true for GET, internal navigate POSTs, and WS connects/reconnects), but the new helpers disambiguate the four lifecycle paths and make intent obvious to readers.

### OnConnect

Called when a WebSocket connection is established:

```go
func (c *TodoController) OnConnect(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    c.Logger.Info("WebSocket connected", "user", ctx.UserID())

    // Store session for server-initiated updates
    session := ctx.Session()
    if session != nil {
        go c.sendWelcomeMessage(session)
    }

    return state, nil
}
```

**Use cases:**
- Store session reference for server-initiated updates
- Start background jobs
- Subscribe to real-time data feeds

### OnDisconnect

Called when a WebSocket connection is closed:

```go
func (c *TodoController) OnDisconnect() {
    c.Logger.Info("WebSocket disconnected")
}
```

**Use cases:**
- Clean up session references
- Cancel background jobs
- Unsubscribe from data feeds

## Context API

For the complete Context API (data extraction, HTTP operations, struct binding), see [API Reference — Context](api-reference.md#context).

## Error Handling

For validation errors, field errors, and template error display, see [Error Handling Reference](error-handling.md).

## Common Patterns

### Counter with Dependencies

```go
type CounterState struct {
    Count int
}

type CounterController struct {
    Logger *slog.Logger
}

func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Count++
    c.Logger.Info("counter incremented", slog.Int("count", state.Count))
    return state, nil
}

func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    if state.Count > 0 {
        state.Count--
    }
    return state, nil
}
```

### CRUD with Database

```go
type TodoState struct {
    Items []Todo
}

type Todo struct {
    ID        string
    Title     string
    Completed bool
}

type TodoController struct {
    DB *sql.DB
}

func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    items, err := c.DB.GetTodos()
    if err != nil {
        return state, fmt.Errorf("failed to load todos: %w", err)
    }
    state.Items = items
    return state, nil
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    title := strings.TrimSpace(ctx.GetString("title"))
    if title == "" {
        return state, livetemplate.NewFieldError("title", errors.New("title required"))
    }

    todo := Todo{
        ID:    uuid.New().String(),
        Title: title,
    }

    if err := c.DB.InsertTodo(todo); err != nil {
        return state, fmt.Errorf("database error")
    }

    state.Items = append(state.Items, todo)
    return state, nil
}

func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    id := ctx.GetString("id")

    for i := range state.Items {
        if state.Items[i].ID == id {
            state.Items[i].Completed = !state.Items[i].Completed
            return state, c.DB.UpdateTodo(state.Items[i])
        }
    }

    return state, fmt.Errorf("todo not found")
}

func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    id := ctx.GetString("id")

    for i, todo := range state.Items {
        if todo.ID == id {
            if err := c.DB.DeleteTodo(id); err != nil {
                return state, fmt.Errorf("database error")
            }
            state.Items = append(state.Items[:i], state.Items[i+1:]...)
            return state, nil
        }
    }

    return state, fmt.Errorf("todo not found")
}
```

### Server-Initiated Updates

```go
type NotificationState struct {
    Messages []string
}

type NotificationController struct {
    sessions sync.Map // userID -> Session
}

func (c *NotificationController) OnConnect(state NotificationState, ctx *livetemplate.Context) (NotificationState, error) {
    if session := ctx.Session(); session != nil {
        c.sessions.Store(ctx.UserID(), session)
    }
    return state, nil
}

func (c *NotificationController) OnDisconnect() {
    // Session cleanup handled by LiveTemplate
}

// Call from anywhere in your application
func (c *NotificationController) NotifyUser(userID, message string) {
    if session, ok := c.sessions.Load(userID); ok {
        session.(livetemplate.Session).TriggerAction("addMessage", map[string]interface{}{
            "message": message,
        })
    }
}

func (c *NotificationController) AddMessage(state NotificationState, ctx *livetemplate.Context) (NotificationState, error) {
    message := ctx.GetString("message")
    state.Messages = append(state.Messages, message)
    return state, nil
}
```

### Cross-Tab Updates with BroadcastAction

In per-connection state mode (the default), use `ctx.BroadcastAction()` to dispatch a named action to all other connections in the session group. Each receiving connection runs the action with its own state, preserving per-connection fields.

```go
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    c.mu.Lock()
    c.messages = append(c.messages, Message{User: state.CurrentUser, Text: ctx.GetString("message")})
    c.mu.Unlock()
    state.Messages = c.copyMessages()
    ctx.BroadcastAction("RefreshMessages", nil) // dispatches to other connections
    return state, nil
}

func (c *ChatController) RefreshMessages(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.Messages = c.copyMessages() // each connection's CurrentUser is preserved
    return state, nil
}
```

## Registration

```go
// Create controller with dependencies
controller := &TodoController{
    DB:     db,
    Logger: logger,
}

// Create initial state
initialState := &TodoState{
    Items: []Todo{},
}

// Create template
tmpl := livetemplate.New("todos")

// Register handler
handler := tmpl.Handle(controller, livetemplate.AsState(initialState))

// Mount to HTTP server
http.Handle("/", handler)
```

## Upload Access

For file upload configuration and handling, see [Upload Reference](uploads.md).

## Testing

Use `AssertPureState[T]()` as a sanity check to catch common dependency types accidentally added to state structs (this is a heuristic, not a comprehensive serializability check):

```go
func TestState(t *testing.T) {
    // Fails if TodoState contains *sql.DB, *slog.Logger, etc.
    livetemplate.AssertPureState[TodoState](t)
}
```

## See Also

- [Server Actions Reference](server-actions.md) - Server-initiated updates with TriggerAction
- [Session Reference](session.md) - State safety, session stores, and connection management
- [Error Handling Reference](error-handling.md) - Detailed error handling patterns
- [Authentication Reference](authentication.md) - User identification and session grouping
- [Upload Reference](uploads.md) - File upload configuration and handling
