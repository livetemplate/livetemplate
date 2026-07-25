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

**Heads-up — `ctx.IsReconnect()` on the first WS:** When a browser does the normal flow (HTTP GET → server-renders → WebSocket connects), the framework persists state at the end of the HTTP-path Mount, then *restores* that state when the WebSocket opens. The WS-path `OnConnect` therefore sees `ctx.IsReconnect() == true` even though no prior WebSocket connection ever existed. If your `OnConnect` needs to distinguish "brand-new session" from "WS resumed after a blip", pair `IsReconnect()` with `IsNewConnect()` or gate on per-session state. See the `IsReconnect` godoc for the full semantics.

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

## State methods in templates

Exported zero-arg methods on your State type are callable from templates just like
fields — `{{.ActiveCount}}` resolves to `ActiveCount()`'s return value. Methods
returning `(T, error)` are omitted (with a warning) when the error is non-nil.

Because LiveTemplate converts State to a map before rendering, these methods are
*precomputed* — but only for methods your templates actually reference. A method that
appears nowhere in any template is never called, so an expensive or side-effecting
helper you keep on State but don't render costs nothing.

Two caveats:

- **Conditional references still run.** The scoping is by name, not by reachability:
  `{{if .Show}}{{.Expensive}}{{end}}` still calls `Expensive()` on every render even
  when `.Show` is false, because the name appears in the template text. Keep genuinely
  expensive work in an action method, not a render-time State method.
- **Prefer methods without side effects.** Precompute timing is an implementation
  detail; a State method should compute a view of the state, not mutate anything or
  perform I/O.

### Methods that take arguments (view helpers)

Sometimes you want a method that takes a key and returns a per-item view — e.g. a
CSS class for a given row, or a formatted label for a given id. **Put it on a struct
field, not on the top-level State**, and call it through that field:

```go
type RowViews struct{ selected map[string]bool }

func (v RowViews) Class(id string) string {
    if v.selected[id] {
        return "row selected"
    }
    return "row"
}

type BoardState struct {
    Rows  []Row
    Views RowViews // a "view helper" sub-struct
}
```

```html
{{range .Rows}}
  <div class="{{$.Views.Class .ID}}">{{.Label}}</div>
{{end}}
```

This works in both the initial HTTP render and WebSocket updates, and replaces the
common workaround of precomputing a `map[string]string` in an action and looking it
up by key in the template. It is the same shape as the framework's built-in
`{{.lvt.AriaInvalid "field"}}`.

**Why the field matters — top-level arg-methods are not supported.** LiveTemplate
converts State to a map before rendering (to inject the `{{.lvt}}` namespace), and
Go's `html/template` cannot call an argument-accepting method once its receiver is a
map key — `{{.Class .ID}}` directly on State fails with *"Class is not a method but
has arguments"*. A struct **field** is stored in the map as a struct value, so both
renderers call its methods natively. Zero-arg State methods are unaffected (they are
precomputed, as above); only *argument-accepting* methods need the field.

## Context API

For the complete Context API (data extraction, HTTP operations, struct binding), see [API Reference — Context](api-reference.md#context).

### Request context

`*livetemplate.Context` **embeds `context.Context`** — it carries the request/connection
context (cancellation, deadline, request-scoped values). So `ctx` *is* a `context.Context`:
pass it directly to any context-aware call (database queries, outbound HTTP, tracing) instead
of `context.Background()`.

```go
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // ✅ ctx is a context.Context — propagates cancellation when the request/connection ends
    if err := c.DB.InsertTodo(ctx, todo); err != nil {
        return state, err
    }
    return state, nil
}
```

Reaching for `context.Background()` inside an action instead silently discards that
cancellation and any deadline or trace attached to the request:

```go
// ❌ throws away request-scoped cancellation, deadline, and tracing
if err := c.DB.InsertTodo(context.Background(), todo); err != nil { ... }
```

Use `context.Background()` only for work that must deliberately **outlive** the request (e.g.
a fire-and-forget goroutine you start from the action) — not for the action's own calls.

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
    // Pass ctx (a context.Context) so the query is cancelled if the request ends.
    items, err := c.DB.GetTodos(ctx)
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

    if err := c.DB.InsertTodo(ctx, todo); err != nil {
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

> `TriggerAction` is also the mechanism behind the server-owned loading pattern (set `Loading=true`, spawn a goroutine, trigger a second action to clear it). See [Loading States §7.3](../guides/progressive-complexity.md#73-server-owned-loading-tier-1) in the Progressive Complexity Guide.

### Cross-Tab Updates with Subscribe + Publish

Peer fan-out is opt-in. Each connection that wants to receive peer updates subscribes to a topic in `Mount`; actions that mutate shared state publish to that topic, and every subscribed peer dispatches the named action with its own state.

The canonical "broadcast to my own session" pattern uses `ctx.SelfTopic()` — a reserved-namespace topic (`lvt:session:<groupID>`) that resolves to this session's own connections. `SelfTopic()` is ACL-exempt by construction; you can always `Subscribe(SelfTopic())` without a `WithTopicACL` rule.

```go
func (c *ChatController) Mount(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    // Opt this connection in to peer fan-out for the session.
    // SelfTopic() is ACL-exempt, idempotent across re-Mounts.
    _ = ctx.Subscribe(ctx.SelfTopic())
    return state, nil
}

func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    c.mu.Lock()
    c.messages = append(c.messages, Message{User: state.CurrentUser, Text: ctx.GetString("message")})
    c.mu.Unlock()
    state.Messages = c.copyMessages()
    ctx.Publish(ctx.SelfTopic(), "RefreshMessages", nil) // fans out to subscribed peers
    return state, nil
}

func (c *ChatController) RefreshMessages(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.Messages = c.copyMessages() // each connection's CurrentUser is preserved
    return state, nil
}
```

**The published action reloads data only — it is not a re-Mount.** Notice `RefreshMessages` re-reads the messages and nothing else. Reach for the tempting shortcut and it bites you:

```go
// ❌ Don't do this. A fan-out tick is neither a page load nor a connect.
func (c *ChatController) RefreshMessages(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    return c.Mount(state, ctx)
}
```

`Mount` does **connect-time** work on top of loading: it `Subscribe`s to topics and runs any `IsInitialMount()`/`IsReconnect()`/`IsNewConnect()`-guarded setup (analytics, background goroutines, presence). A fan-out dispatch is an *action* — its connect-kind is `ConnectKindAction`, so all three of those helpers return `false`. Delegating the action to `Mount` therefore means any connect-kind-guarded setup **silently never runs**, and the unguarded `Subscribe` re-runs on every single broadcast — a cheap no-op for `ctx.SelfTopic()` (ACL-exempt and idempotent, as here), but a real per-tick ACL re-check for a developer topic like `Subscribe("room/" + id)`. Keep the two jobs separate:

- **`Mount`** — subscribe + load. Runs on HTTP GET/POST, WS connect, WS reconnect.
- **The published action** (`RefreshMessages`) — load only. Runs on every fan-out tick.

The shortcut *appears* to work when `Mount` is a pure data-load with no `Subscribe` or guarded setup (its fields just get re-set to the same values) — which is exactly why it's a trap: it breaks silently the day `Mount` grows a connect-time concern. So split them unconditionally, even while `Mount` is still a plain load; don't wait for the break. The runnable [`live-dashboard`](https://github.com/livetemplate/docs/tree/main/examples/live-dashboard) example follows the split from the start: `Mount` subscribes then snapshots; its `Refresh` action snapshots only.

**Ordering.** `Publish` queues onto a per-action drain. Call it **after** every `ctx.With*()` shallow-copy mutation; publishes queued before a `With*()` are stranded on the pre-copy Context and won't propagate.

**Cap.** A single action can enqueue at most `MaxPublishesPerAction` (declared in `topic_context.go`) `Publish` calls before subsequent calls become hard errors.

### Subscribing to ACL-Gated Developer Topics

Beyond `SelfTopic()`, controllers can subscribe to developer-defined topics (e.g. `"room/lobby"`). Developer topics are **deny-by-default** — every `Subscribe(developerTopic)` runs the `WithTopicACL(fn)` hook, and a denied call returns `*TopicForbiddenError`. Two patterns matter for controllers:

**(1) Guard with `IsInitialMount` when the ACL may deny.** `Mount` runs on HTTP GET, HTTP POST, WS connect, and WS reconnect. A denied `Subscribe` on the HTTP GET path surfaces as **HTTP 500** — the page aborts before the WS can exercise the keep-open path. Guard side-effects (including potentially-denied Subscribes) with the connect-kind helpers so the GET render does not call `Subscribe` until the WS is the lifecycle owner:

```go
func (c *RoomController) Mount(state RoomState, ctx *livetemplate.Context) (RoomState, error) {
    _ = ctx.Subscribe(ctx.SelfTopic()) // always safe, ACL-exempt
    if !ctx.IsInitialMount() {
        // Only subscribe to the gated topic on WS connect / reconnect /
        // POST actions — never on the initial HTTP GET render.
        if err := ctx.Subscribe("room/" + state.RoomID); err != nil {
            // Propagate the error to surface lvt:error on the client (see below).
            return state, err
        }
    }
    return state, nil
}
```

**(2) Propagate the error to surface `lvt:error`.** When a controller propagates a `*TopicForbiddenError` from `Mount` on the WS-connect path, the server emits a `{"type":"error","code":"topic_forbidden","topic":<denied>}` envelope, logs a structured warning, and **keeps the connection open** — adopting the controller's returned state and continuing the normal mount lifecycle (`persistState`, `OnConnect`, initial-tree send). The client dispatches a `lvt:error` `CustomEvent` on the `[data-lvt-id]` wrapper element. See the [Client Error Envelope section in the PubSub reference](pubsub.md#client-error-envelope-lvterror) for the wire-level contract.

**(3) Don't swallow the error if you want it surfaced.** A controller that swallows the denied Subscribe — `_ = ctx.Subscribe("denied"); return s, nil` — emits **no envelope**. The Phase 4 contract is on the propagated-error scenario only; if you want the client to see `lvt:error`, return the error. Returning `nil` is a quiet allow-and-continue.

**(4) Don't mutate `s` before a may-deny `Subscribe`.** Because keep-open adopts the controller's returned `newState`, a partial mutation before a denied Subscribe persists silently (it isn't rolled back). The rule of thumb:

> To surface the envelope, propagate the error (`return s, err`). To keep state clean, don't mutate `s` before a `Subscribe` that may be denied.

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

### Serving static assets alongside the app

The handler returned by `Handle` is mounted at `/`, where Go's `ServeMux` treats it as a **catch-all**: every request that doesn't match a more specific pattern falls through to it (including the WebSocket upgrade). To serve static files — images, CSS, downloads — next to your app, register them under a **more specific prefix** on the same mux. Longest-prefix-match routes those requests to the file server and everything else to the app:

```go
handler := tmpl.Handle(controller, livetemplate.AsState(initialState))

// Files under /assets/… are served from ./assets. http.Dir already rejects
// "../" traversal, so http.FileServer needs no extra guard.
http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

// The app owns everything else.
http.Handle("/", handler)
```

The prefix just has to be distinct from your app's own routes — `ServeMux` does the rest. This composition is plain `net/http`; the framework adds nothing here on purpose.

**When a prefix isn't enough.** If your app renders content that references assets at **arbitrary top-level paths** — e.g. user-authored HTML with `<img src="diagram.png">` resolving to `/diagram.png` — those paths can collide with app routes, and `ServeMux` can't disambiguate them by prefix. That case needs a fall-through wrapper: *serve the file if it exists under a safe root, otherwise defer to the app handler.* Keep it a small wrapper in your own `main` rather than reaching for a framework primitive — the security-relevant policy (which extensions to serve, embedded vs. disk, symlink resolution) is app-specific, and `http.Dir` / `http.FileServer` already supply the traversal-safe file access it builds on.

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
