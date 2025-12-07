# Controller + State Pattern Design

**Status**: Proposal
**Created**: 2024-12-06
**Author**: Adnaan
**Proposal Number**: 002

## Summary

Refactor LiveTemplate to separate Controller (dependencies, singleton) from State (pure data, cloned per session). Inspired by Phoenix LiveView's socket.assigns pattern.

## Problem

Current `cloneStore()` copies all exported fields including dependencies, causing:
- Security issues: Session-specific data (OAuth tokens, caches) accidentally shared
- Architectural ambiguity: No clear contract for what gets cloned vs shared
- Developer footguns: Easy to accidentally put dependencies in cloned state

## Solution

### Core Architecture

```go
// CONTROLLER: Singleton, holds dependencies, NEVER cloned
type TodoController struct {
    DB     *sql.DB
    Logger *slog.Logger
}

// STATE: Pure data, cloned per session, serializable
type TodoState struct {
    Items  []Todo
    Filter string
    Flash  string `lvt:"temp"`  // Reset after each render
}

// Helper methods on State (for templates)
func (s *TodoState) IsEmpty() bool { return len(s.Items) == 0 }
```

### State Interface

The State interface uses serialization as its marker - if a struct can serialize, it's pure data.

```go
// Framework provides State interface (serialization = marker)
type State interface {
    encoding.BinaryMarshaler
    encoding.BinaryUnmarshaler
    inner() any  // Returns the underlying value for framework use
}
```

#### Option 1: Generic Wrapper (Zero Boilerplate)

For most use cases, wrap your plain struct with `AsState()`:

```go
// User code - plain struct, no interfaces needed
type TodoState struct {
    Items  []Todo
    Filter string
    Flash  string `lvt:"temp"`  // Reset after each render
}

// Helper methods on State (accessible in templates)
func (s *TodoState) IsEmpty() bool {
    return len(s.Items) == 0
}

func (s *TodoState) FilteredItems() []Todo {
    if s.Filter == "" {
        return s.Items
    }
    var filtered []Todo
    for _, item := range s.Items {
        if strings.Contains(strings.ToLower(item.Title), strings.ToLower(s.Filter)) {
            filtered = append(filtered, item)
        }
    }
    return filtered
}

// Mount with wrapper - AsState handles serialization
handler := tmpl.Handle(&TodoController{DB: db}, livetemplate.AsState(&TodoState{}))
```

Framework implementation of `AsState`:

```go
// Generic wrapper for zero-boilerplate usage
func AsState[T any](s *T) State {
    return &jsonState[T]{value: s}
}

type jsonState[T any] struct {
    value *T
}

func (s *jsonState[T]) MarshalBinary() ([]byte, error) {
    return json.Marshal(s.value)
}

func (s *jsonState[T]) UnmarshalBinary(data []byte) error {
    return json.Unmarshal(data, s.value)
}

func (s *jsonState[T]) inner() any {
    return s.value
}
```

#### Option 2: Explicit Implementation (Custom Serialization)

For advanced use cases requiring custom serialization (e.g., msgpack, protobuf):

```go
type TodoState struct {
    Items  []Todo
    Filter string
}

// Implement State interface explicitly
func (s *TodoState) MarshalBinary() ([]byte, error) {
    // Custom serialization (e.g., msgpack for performance)
    return msgpack.Marshal(s)
}

func (s *TodoState) UnmarshalBinary(data []byte) error {
    return msgpack.Unmarshal(data, s)
}

func (s *TodoState) inner() any {
    return s
}

// Mount directly - no wrapper needed
handler := tmpl.Handle(&TodoController{DB: db}, &TodoState{})
```

### Unified Context

```go
// livetemplate.Context - single type for all lifecycle methods
// Defined in the livetemplate package
type Context struct {
    context.Context                     // Embedded stdlib context
    action  string                      // Action name (empty for Mount)
    data    *ActionData                 // Form/request data
    userID  string                      // Authenticated user
    session Session                     // For server-initiated actions
}

func (c *Context) Action() string              { return c.action }
func (c *Context) UserID() string              { return c.userID }
func (c *Context) GetString(key string) string { /* ... */ }
func (c *Context) GetInt(key string) int       { /* ... */ }
func (c *Context) GetFloat(key string) float64 { /* ... */ }
func (c *Context) GetBool(key string) bool     { /* ... */ }
func (c *Context) Has(key string) bool         { /* ... */ }
func (c *Context) Session() Session            { return c.session }
```

**Usage in user code:**
```go
import "github.com/livetemplate/livetemplate"

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    title := ctx.GetString("title")
    userID := ctx.UserID()
    // ...
}
```

### Lifecycle Methods

All lifecycle methods follow a consistent pattern with the Controller as receiver and State as the first parameter.

#### Lifecycle Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         REQUEST LIFECYCLE                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  HTTP GET (Initial Page Load)                                           │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │ New Session │───►│   Mount()   │───►│   Render    │                 │
│  │  Created?   │yes │ Load data   │    │  Template   │                 │
│  └──────┬──────┘    └─────────────┘    └─────────────┘                 │
│         │no                                                             │
│         ▼                                                               │
│  ┌─────────────┐    ┌─────────────┐                                    │
│  │   Restore   │───►│   Render    │                                    │
│  │    State    │    │  Template   │                                    │
│  └─────────────┘    └─────────────┘                                    │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  WebSocket Connect                                                      │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │   Restore   │───►│ OnConnect() │───►│ Send Initial│                 │
│  │    State    │    │ (optional)  │    │    Tree     │                 │
│  └─────────────┘    └─────────────┘    └─────────────┘                 │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  User Action (Button Click, Form Submit)                                │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │  Dispatch   │───►│  Action()   │───►│ Send Update │                 │
│  │   Action    │    │ Modify state│    │    Tree     │                 │
│  └─────────────┘    └─────────────┘    └─────────────┘                 │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  WebSocket Disconnect                                                   │
│  ┌─────────────┐    ┌─────────────┐                                    │
│  │   Client    │───►│OnDisconnect│                                    │
│  │ Disconnects │    │ (optional)  │                                    │
│  └─────────────┘    └─────────────┘                                    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 1. Mount

**Signature:**
```go
func (c *Controller) Mount(state StateType, ctx *livetemplate.Context) (StateType, error)
```

**When Called:**
- Once per session creation (first HTTP GET for a new session)
- NOT called on WebSocket reconnect (state is restored instead)
- NOT called on subsequent HTTP GETs (state is restored instead)

**Purpose:**
- Initialize state with data from database
- Set up initial values
- Load user-specific data

**Return Pattern:**
- State is passed by value (immutable input)
- Return modified state and nil error on success
- Return original state and error on failure

**Example:**
```go
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // Load user's todos from database
    todos, err := c.DB.GetTodosByUser(ctx.UserID())
    if err != nil {
        return state, fmt.Errorf("failed to load todos: %w", err)
    }
    state.Items = todos

    // Load user preferences
    prefs, _ := c.DB.GetUserPrefs(ctx.UserID())
    state.Filter = prefs.DefaultFilter

    c.Logger.Info("session mounted",
        slog.String("user_id", ctx.UserID()),
        slog.Int("todo_count", len(todos)))

    return state, nil
}
```

---

#### 2. OnConnect

**Signature:**
```go
func (c *Controller) OnConnect(state StateType, ctx *livetemplate.Context) (StateType, error)
```

**When Called:**
- Every time a WebSocket connection is established
- After state is restored from session store
- Includes initial connect and reconnects

**Purpose:**
- Subscribe to PubSub channels
- Start background timers
- Refresh potentially stale data
- Set up server-initiated actions

**Example:**
```go
func (c *TodoController) OnConnect(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // Subscribe to real-time updates for this user
    c.PubSub.Subscribe(fmt.Sprintf("user:%s:todos", ctx.UserID()))

    // Optionally refresh stale data (if session is old)
    if state.LastUpdated.Before(time.Now().Add(-5 * time.Minute)) {
        todos, err := c.DB.GetTodosByUser(ctx.UserID())
        if err == nil {
            state.Items = todos
            state.LastUpdated = time.Now()
        }
    }

    // Start a ticker for server-initiated updates
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                ctx.Session().TriggerAction("refresh_stats", nil)
            }
        }
    }()

    return state, nil
}
```

---

#### 3. Action Methods

**Signature:**
```go
func (c *Controller) ActionName(state StateType, ctx *livetemplate.Context) (StateType, error)
```

**When Called:**
- When user triggers an action (button click, form submit)
- Action name is matched to method name (case-insensitive, snake_case → PascalCase)

**Naming Convention:**
| Action Name | Method Name |
|-------------|-------------|
| `add` | `Add()` |
| `delete_todo` | `DeleteTodo()` |
| `setFilter` | `SetFilter()` |
| `toggle_completed` | `ToggleCompleted()` |

**Purpose:**
- Handle user interactions
- Modify state
- Perform database operations
- Return errors for validation

**Example:**
```go
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    title := strings.TrimSpace(ctx.GetString("title"))

    // Validation - return original state with error
    if title == "" {
        return state, livetemplate.FieldError{
            Field:   "title",
            Message: "Title is required",
        }
    }

    // Database operation
    todo, err := c.DB.CreateTodo(ctx.UserID(), title)
    if err != nil {
        c.Logger.Error("failed to create todo", slog.String("error", err.Error()))
        return state, fmt.Errorf("failed to save todo")
    }

    // Update state and return
    state.Items = append(state.Items, todo)
    state.Flash = "Todo added successfully!"

    return state, nil
}

func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    id := ctx.GetString("id")

    // Database operation
    if err := c.DB.DeleteTodo(id); err != nil {
        return state, fmt.Errorf("failed to delete todo")
    }

    // Update state
    for i, todo := range state.Items {
        if todo.ID == id {
            state.Items = append(state.Items[:i], state.Items[i+1:]...)
            break
        }
    }

    return state, nil
}

func (c *TodoController) ToggleCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    id := ctx.GetString("id")

    for i, todo := range state.Items {
        if todo.ID == id {
            state.Items[i].Completed = !state.Items[i].Completed
            c.DB.UpdateTodo(state.Items[i])
            break
        }
    }

    return state, nil
}

func (c *TodoController) SetFilter(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.Filter = ctx.GetString("filter")
    return state, nil
}
```

---

#### 4. OnDisconnect

**Signature:**
```go
func (c *Controller) OnDisconnect()
```

**When Called:**
- When WebSocket connection closes (client navigates away, network issues, etc.)
- NOT called on HTTP requests

**Purpose:**
- Cleanup resources
- Unsubscribe from PubSub
- Stop background goroutines
- Log disconnect events

**Note:** This method does NOT receive state or context because the connection is already closed.

**Example:**
```go
func (c *TodoController) OnDisconnect() {
    c.Logger.Info("client disconnected")
    // Note: PubSub cleanup is typically handled automatically by the framework
    // based on the connection context being cancelled
}
```

---

#### 5. Server-Initiated Actions

**Triggered via:**
```go
ctx.Session().TriggerAction(actionName string, data map[string]any) error
```

**When Used:**
- Background job completion
- Real-time updates from external sources
- Timer-based updates
- Webhook handlers

**Example:**
```go
// Called from OnConnect or external webhook
func (c *TodoController) RefreshStats(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    stats, err := c.DB.GetTodoStats(ctx.UserID())
    if err != nil {
        return state, err
    }
    state.TotalCount = stats.Total
    state.CompletedCount = stats.Completed
    return state, nil
}

// Triggered from a webhook handler elsewhere in your app
func (h *WebhookHandler) HandleTodoCreated(userID string, todo Todo) {
    // Trigger action for all of user's sessions
    h.LiveTemplate.TriggerAction(userID, "external_todo_added", map[string]any{
        "todo": todo,
    })
}

// The action handler
func (c *TodoController) ExternalTodoAdded(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // Data passed from TriggerAction
    todoData := ctx.Get("todo").(map[string]any)
    todo := Todo{
        ID:    todoData["id"].(string),
        Title: todoData["title"].(string),
    }
    state.Items = append(state.Items, todo)
    state.Flash = "New todo synced from external source"
    return state, nil
}
```

---

#### Lifecycle Method Summary

| Method | Signature | Required | When Called |
|--------|-----------|----------|-------------|
| `Mount` | `(state StateType, ctx *livetemplate.Context) (StateType, error)` | Yes | Once per session creation |
| `OnConnect` | `(state StateType, ctx *livetemplate.Context) (StateType, error)` | No | Every WebSocket connect |
| `ActionName` | `(state StateType, ctx *livetemplate.Context) (StateType, error)` | No | User/server triggers action |
| `OnDisconnect` | `()` | No | WebSocket closes |

**Key Points:**
- `StateType` is your concrete state type (e.g., `TodoState`)
- State is passed **by value** (immutable input)
- Modified state is **returned** (functional pattern)
- All methods except OnDisconnect return `(StateType, error)`
- On error, return original state with error; framework won't update state

#### Error Handling

All lifecycle methods (except `OnDisconnect`) can return errors:

```go
// Field-specific error (shown next to form field)
return livetemplate.FieldError{
    Field:   "email",
    Message: "Invalid email format",
}

// Multiple field errors
return livetemplate.MultiError{
    {Field: "email", Message: "Invalid email"},
    {Field: "password", Message: "Too short"},
}

// General error (shown as flash/alert)
return fmt.Errorf("database connection failed")
```

### Mounting API

```go
tmpl := livetemplate.New("todos.html")

// Basic
handler := tmpl.Handle(&TodoController{DB: db}, livetemplate.AsState(&TodoState{}))

// With options
handler := tmpl.Handle(
    &TodoController{DB: db},
    livetemplate.AsState(&TodoState{}),
    livetemplate.WithStore(livetemplate.RedisStore(client)),
    livetemplate.WithSessionTTL(1 * time.Hour),
)

http.Handle("/todos", handler)
```

### Template Access

```html
<!-- State is template root -->
{{if .Flash}}
    <div class="alert">{{.Flash}}</div>
{{end}}

{{if .IsEmpty}}
    <p>No todos yet</p>
{{else}}
    {{range .Items}}
        <li>{{.Title}}</li>
    {{end}}
{{end}}
```

### State Lifetime

| Field Type | Behavior |
|------------|----------|
| Regular | Persisted to session store |
| `lvt:"temp"` | Reset after each render |

### Persistence (Configurable)

```go
// Memory store (Phoenix-like, state in memory)
livetemplate.WithStore(livetemplate.MemoryStore())

// Redis store (distributed, survives restarts)
livetemplate.WithStore(livetemplate.RedisStore(client))
```

Lifecycle is consistent regardless of store choice:
- Mount() called once per session creation
- OnConnect() called on each WebSocket connection
- State persisted/restored according to store implementation

## Security Properties

| Threat | Mitigation |
|--------|------------|
| Cross-session data leakage | State cannot contain dependencies (enforced by serialization) |
| Auth context sharing | Dependencies on Controller, not State |
| Accidental pointer sharing | Only State cloned, Controller is singleton |

## Files to Modify

- `mount.go` - New Handle() signature, lifecycle dispatch
- `state.go` - AsState wrapper, State interface
- `context.go` - Unified Context type (new file)
- `session_stores.go` - State serialization changes
- `dispatch.go` - Method signature changes (state as first param)

## Breaking Changes (Clean Break)

This is a complete API replacement. No backward compatibility, no deprecation period.

**Removed:**
- `Handle(store)` - old single-argument signature
- `lvt:"state"` tag - no longer needed
- `cloneStore()` - reflection-based cloning eliminated
- `ActionContext` - replaced by unified `Context`
- `StoreInitializer.Init()` - replaced by `Mount()`

**New:**
- `Handle(controller, AsState(state), ...options)` - explicit separation
- `AsState[T]()` - generic wrapper for state serialization
- `*livetemplate.Context` - unified context for all lifecycle methods
- Method signature: `Method(state StateType, ctx *livetemplate.Context) (StateType, error)`
- Lifecycle methods: `Mount`, `OnConnect`, `OnDisconnect`

**Rationale:** Library is pre-1.0 with no external users. Clean break is simpler than maintaining two APIs.

## Complete Example

A full working example showing all concepts together:

```go
package main

import (
    "database/sql"
    "fmt"
    "log/slog"
    "net/http"
    "strings"
    "time"

    "github.com/livetemplate/livetemplate"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN TYPES
// ════════════════════════════════════════════════════════════════════════════

type Todo struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Completed bool      `json:"completed"`
    CreatedAt time.Time `json:"created_at"`
}

// ════════════════════════════════════════════════════════════════════════════
// STATE - Pure data, cloned per session
// ════════════════════════════════════════════════════════════════════════════

type TodoState struct {
    Items       []Todo    `json:"items"`
    Filter      string    `json:"filter"`       // "all", "active", "completed"
    LastUpdated time.Time `json:"last_updated"`
    Flash       string    `json:"flash" lvt:"temp"` // Reset after each render
}

// Helper methods for templates
func (s *TodoState) IsEmpty() bool {
    return len(s.Items) == 0
}

func (s *TodoState) FilteredItems() []Todo {
    switch s.Filter {
    case "active":
        var items []Todo
        for _, t := range s.Items {
            if !t.Completed {
                items = append(items, t)
            }
        }
        return items
    case "completed":
        var items []Todo
        for _, t := range s.Items {
            if t.Completed {
                items = append(items, t)
            }
        }
        return items
    default:
        return s.Items
    }
}

func (s *TodoState) ActiveCount() int {
    count := 0
    for _, t := range s.Items {
        if !t.Completed {
            count++
        }
    }
    return count
}

func (s *TodoState) CompletedCount() int {
    return len(s.Items) - s.ActiveCount()
}

// ════════════════════════════════════════════════════════════════════════════
// CONTROLLER - Singleton, holds dependencies
// ════════════════════════════════════════════════════════════════════════════

type TodoController struct {
    DB     *sql.DB
    Logger *slog.Logger
}

// Mount - Called once when session is created
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    c.Logger.Info("mounting session", slog.String("user_id", ctx.UserID()))

    // Load todos from database
    rows, err := c.DB.Query("SELECT id, title, completed, created_at FROM todos WHERE user_id = ?", ctx.UserID())
    if err != nil {
        return state, err
    }
    defer rows.Close()

    for rows.Next() {
        var t Todo
        if err := rows.Scan(&t.ID, &t.Title, &t.Completed, &t.CreatedAt); err != nil {
            return state, err
        }
        state.Items = append(state.Items, t)
    }

    state.Filter = "all"
    state.LastUpdated = time.Now()

    return state, nil
}

// OnConnect - Called on every WebSocket connect
func (c *TodoController) OnConnect(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    c.Logger.Info("client connected", slog.String("user_id", ctx.UserID()))

    // Refresh if data is stale (older than 5 minutes)
    if time.Since(state.LastUpdated) > 5*time.Minute {
        c.Logger.Info("refreshing stale data")
        // Re-run mount logic to refresh
        state.Items = nil
        return c.Mount(state, ctx)
    }

    return state, nil
}

// OnDisconnect - Called when WebSocket closes
func (c *TodoController) OnDisconnect() {
    c.Logger.Info("client disconnected")
}

// ════════════════════════════════════════════════════════════════════════════
// ACTION METHODS
// ════════════════════════════════════════════════════════════════════════════

// Add - Create a new todo
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    title := strings.TrimSpace(ctx.GetString("title"))

    if title == "" {
        return state, livetemplate.FieldError{
            Field:   "title",
            Message: "Title cannot be empty",
        }
    }

    if len(title) > 200 {
        return state, livetemplate.FieldError{
            Field:   "title",
            Message: "Title must be less than 200 characters",
        }
    }

    // Insert into database
    id := generateID()
    _, err := c.DB.Exec(
        "INSERT INTO todos (id, user_id, title, completed, created_at) VALUES (?, ?, ?, ?, ?)",
        id, ctx.UserID(), title, false, time.Now(),
    )
    if err != nil {
        c.Logger.Error("failed to insert todo", slog.String("error", err.Error()))
        return state, fmt.Errorf("failed to save todo")
    }

    // Update state
    state.Items = append(state.Items, Todo{
        ID:        id,
        Title:     title,
        Completed: false,
        CreatedAt: time.Now(),
    })
    state.Flash = "Todo added!"

    return state, nil
}

// Toggle - Toggle todo completion status
func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    id := ctx.GetString("id")

    for i := range state.Items {
        if state.Items[i].ID == id {
            state.Items[i].Completed = !state.Items[i].Completed

            _, err := c.DB.Exec(
                "UPDATE todos SET completed = ? WHERE id = ?",
                state.Items[i].Completed, id,
            )
            if err != nil {
                return state, fmt.Errorf("failed to update todo")
            }
            break
        }
    }

    return state, nil
}

// Delete - Remove a todo
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    id := ctx.GetString("id")

    _, err := c.DB.Exec("DELETE FROM todos WHERE id = ?", id)
    if err != nil {
        return state, fmt.Errorf("failed to delete todo")
    }

    for i, t := range state.Items {
        if t.ID == id {
            state.Items = append(state.Items[:i], state.Items[i+1:]...)
            break
        }
    }

    return state, nil
}

// SetFilter - Change the filter view
func (c *TodoController) SetFilter(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    filter := ctx.GetString("filter")
    if filter != "all" && filter != "active" && filter != "completed" {
        filter = "all"
    }
    state.Filter = filter
    return state, nil
}

// ClearCompleted - Remove all completed todos
func (c *TodoController) ClearCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    _, err := c.DB.Exec("DELETE FROM todos WHERE user_id = ? AND completed = true", ctx.UserID())
    if err != nil {
        return state, fmt.Errorf("failed to clear completed")
    }

    var active []Todo
    for _, t := range state.Items {
        if !t.Completed {
            active = append(active, t)
        }
    }
    state.Items = active
    state.Flash = "Cleared completed todos"

    return state, nil
}

// ════════════════════════════════════════════════════════════════════════════
// MAIN
// ════════════════════════════════════════════════════════════════════════════

func main() {
    // Setup dependencies
    db, _ := sql.Open("sqlite3", "todos.db")
    logger := slog.Default()

    // Create template
    tmpl := livetemplate.New("todos.html")

    // Mount controller with state
    handler := tmpl.Handle(
        &TodoController{DB: db, Logger: logger},
        livetemplate.AsState(&TodoState{}),
        livetemplate.WithStore(livetemplate.MemoryStore()),
        livetemplate.WithSessionTTL(24 * time.Hour),
    )

    // Serve
    http.Handle("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

### Template (todos.html)

```html
<!DOCTYPE html>
<html>
<head>
    <title>Todos</title>
    <script src="/livetemplate.js"></script>
</head>
<body>
    <div id="app">
        {{if .Flash}}
            <div class="flash">{{.Flash}}</div>
        {{end}}

        <h1>Todos</h1>

        <form lvt-action="add">
            <input type="text" name="title" placeholder="What needs to be done?" />
            <button type="submit">Add</button>
            {{if .Errors.title}}
                <span class="error">{{.Errors.title}}</span>
            {{end}}
        </form>

        {{if not .IsEmpty}}
            <ul class="todo-list">
                {{range .FilteredItems}}
                    <li class="{{if .Completed}}completed{{end}}">
                        <input type="checkbox"
                               lvt-action="toggle"
                               lvt-data-id="{{.ID}}"
                               {{if .Completed}}checked{{end}} />
                        <span>{{.Title}}</span>
                        <button lvt-action="delete" lvt-data-id="{{.ID}}">×</button>
                    </li>
                {{end}}
            </ul>

            <footer>
                <span>{{.ActiveCount}} items left</span>

                <div class="filters">
                    <button lvt-action="set_filter" lvt-data-filter="all"
                            class="{{if eq .Filter "all"}}selected{{end}}">All</button>
                    <button lvt-action="set_filter" lvt-data-filter="active"
                            class="{{if eq .Filter "active"}}selected{{end}}">Active</button>
                    <button lvt-action="set_filter" lvt-data-filter="completed"
                            class="{{if eq .Filter "completed"}}selected{{end}}">Completed</button>
                </div>

                {{if gt .CompletedCount 0}}
                    <button lvt-action="clear_completed">Clear completed</button>
                {{end}}
            </footer>
        {{else}}
            <p class="empty">No todos yet. Add one above!</p>
        {{end}}
    </div>
</body>
</html>
```

### Test

```go
func TestTodoState_IsPure(t *testing.T) {
    // Verifies TodoState contains no dependencies
    livetemplate.AssertPureState[TodoState](t)
}

func TestTodoController_Add(t *testing.T) {
    db := setupTestDB(t)
    ctrl := &TodoController{DB: db, Logger: slog.Default()}
    state := TodoState{}  // Value type, not pointer
    ctx := livetemplate.NewTestContext("add", map[string]any{
        "title": "Buy groceries",
    })

    newState, err := ctrl.Add(state, ctx)

    assert.NoError(t, err)
    assert.Len(t, newState.Items, 1)
    assert.Equal(t, "Buy groceries", newState.Items[0].Title)
    assert.Equal(t, "Todo added!", newState.Flash)

    // Original state is unchanged (immutability)
    assert.Len(t, state.Items, 0)
}

func TestTodoController_Add_ValidationError(t *testing.T) {
    ctrl := &TodoController{DB: nil, Logger: slog.Default()}
    state := TodoState{}
    ctx := livetemplate.NewTestContext("add", map[string]any{
        "title": "",  // Empty title
    })

    newState, err := ctrl.Add(state, ctx)

    assert.Error(t, err)
    fieldErr, ok := err.(livetemplate.FieldError)
    assert.True(t, ok)
    assert.Equal(t, "title", fieldErr.Field)

    // State unchanged on error
    assert.Equal(t, state, newState)
}
```

## Progress

- [x] Phase 1: Understanding
- [x] Phase 2: Exploration
- [x] Phase 3: Design Presentation
- [x] Phase 4: Design Documentation
- [ ] Phase 5: Worktree Setup
- [ ] Phase 6: Core Library Implementation
- [ ] Phase 7: Update Dependent Repos
  - [ ] `examples/` - Update all example apps to new API
  - [ ] `livepage` - Update to new Controller+State pattern
  - [ ] `lvt` - Update CLI tool and generated code templates
