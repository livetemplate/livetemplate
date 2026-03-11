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
// Template: lvt-click="add"
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

Called once when a new session is created:

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

The `*livetemplate.Context` provides access to action data and HTTP utilities.

### Data Extraction

| Method | Return Type | Description |
|--------|-------------|-------------|
| `Action()` | `string` | Get action name (empty for lifecycle methods like Mount/OnConnect) |
| `Get(key)` | `interface{}` | Get raw value (for non-primitive types) |
| `GetString(key)` | `string` | Get string value (empty if missing) |
| `GetInt(key)` | `int` | Get integer value (0 if missing/invalid) |
| `GetFloat(key)` | `float64` | Get float value (0 if missing/invalid) |
| `GetBool(key)` | `bool` | Get boolean value (false if missing) |
| `Has(key)` | `bool` | Check if key exists |
| `Bind(v)` | `error` | Unmarshal action data into a struct |
| `BindAndValidate(v, validate)` | `error` | Bind + validate using `*validator.Validate` from go-playground/validator |

Example:

```go
func (c *Controller) UpdateSettings(state State, ctx *livetemplate.Context) (State, error) {
    state.Theme = ctx.GetString("theme")
    state.FontSize = ctx.GetInt("fontSize")
    state.DarkMode = ctx.GetBool("darkMode")
    return state, nil
}
```

### Struct Binding

Bind form data directly to a struct:

```go
type CreateUserInput struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func (c *Controller) CreateUser(state State, ctx *livetemplate.Context) (State, error) {
    var input CreateUserInput
    if err := ctx.Bind(&input); err != nil {
        return state, err
    }

    // Use input.Name, input.Email, input.Age
    return state, c.DB.CreateUser(input)
}
```

### Validation

Use with `go-playground/validator`:

```go
type RegisterInput struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Name     string `json:"name" validate:"required"`
}

func (c *Controller) Register(state State, ctx *livetemplate.Context) (State, error) {
    var input RegisterInput
    validate := validator.New()

    if err := ctx.BindAndValidate(&input, validate); err != nil {
        return state, err // Returns field-specific error messages
    }

    return state, c.createAccount(input)
}
```

### Session Access

```go
func (c *Controller) SendNotification(state State, ctx *livetemplate.Context) (State, error) {
    session := ctx.Session()
    if session != nil {
        // Use session for server-initiated updates
        go session.TriggerAction("notification", map[string]interface{}{
            "message": "Update complete!",
        })
    }
    return state, nil
}
```

### HTTP Methods (HTTP POST only)

For HTTP POST actions (not WebSocket), you can access HTTP primitives:

```go
func (c *Controller) Login(state State, ctx *livetemplate.Context) (State, error) {
    if !ctx.IsHTTP() {
        return state, errors.New("login requires HTTP")
    }

    // Set cookie
    ctx.SetCookie(&http.Cookie{
        Name:     "session_token",
        Value:    token,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
    })

    // Redirect after login
    return state, ctx.Redirect("/dashboard", http.StatusSeeOther)
}
```

| Method | Description |
|--------|-------------|
| `IsHTTP()` | Check if HTTP context available |
| `SetCookie(cookie)` | Set response cookie |
| `GetCookie(name)` | Get request cookie |
| `DeleteCookie(name)` | Delete cookie |
| `Redirect(url, code)` | Redirect response |

## Error Handling

### Field Errors

Return a single field error:

```go
func (c *Controller) UpdateEmail(state State, ctx *livetemplate.Context) (State, error) {
    email := ctx.GetString("email")

    if !isValidEmail(email) {
        return state, livetemplate.NewFieldError("email",
            errors.New("please enter a valid email address"))
    }

    state.Email = email
    return state, nil
}
```

### Multiple Field Errors

Return multiple field errors at once:

```go
func (c *Controller) Register(state State, ctx *livetemplate.Context) (State, error) {
    var errs livetemplate.MultiError

    email := ctx.GetString("email")
    password := ctx.GetString("password")

    if !isValidEmail(email) {
        errs = append(errs, livetemplate.NewFieldError("email",
            errors.New("invalid email format")))
    }

    if len(password) < 8 {
        errs = append(errs, livetemplate.NewFieldError("password",
            errors.New("password must be at least 8 characters")))
    }

    if len(errs) > 0 {
        return state, errs
    }

    return state, c.createUser(email, password)
}
```

### Template Error Display

```html
{{if .lvt.HasError "email"}}
    <span class="error">{{.lvt.Error "email"}}</span>
{{end}}

{{if .lvt.Errors}}
    <div class="alert">{{range .lvt.Errors}}{{.}}{{end}}</div>
{{end}}
```

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

When templates are configured with `WithUpload(...)`, completed uploads are accessible via Context:

```go
func (c *Controller) SaveProfile(state State, ctx *livetemplate.Context) (State, error) {
    if ctx.HasUploads("avatar") {
        for _, entry := range ctx.GetCompletedUploads("avatar") {
            state.AvatarPath = entry.TempPath
        }
    }
    return state, nil
}
```

| Method | Return Type | Description |
|--------|-------------|-------------|
| `HasUploads(name)` | `bool` | Check if uploads exist for field |
| `GetCompletedUploads(name)` | `[]*UploadEntry` | Get completed upload entries |

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
- [Session Reference](session.md) - Session stores and connection management
- [Error Handling Reference](error-handling.md) - Detailed error handling patterns
- [Authentication Reference](authentication.md) - User identification and session grouping
- [Upload Reference](uploads.md) - File upload configuration and handling
