# Controller Pattern Reference

Controllers separate state (data) from logic (behavior), enabling proper dependency injection and cleaner persistence. Use them when stores need database connections, loggers, or other dependencies that shouldn't be serialized to Redis.

## Overview

Actions are automatically dispatched to methods matching the action name:

```go
type Counter struct {
    Count int
}

// Action "increment" → method Increment()
func (c *Counter) Increment(ctx *livetemplate.ActionContext) error {
    c.Count++
    return nil
}

// Action "decrement" → method Decrement()
func (c *Counter) Decrement(ctx *livetemplate.ActionContext) error {
    c.Count--
    return nil
}
```

**Key features:**
- No boilerplate switch statements
- Method names are discoverable by IDE
- Type-safe action handlers
- Cached method lookups (O(1) after first call)

## State vs Controller

For stores with dependencies, separate concerns into two parts:

```go
// State: Pure data, no dependencies, serializable
type CounterState struct {
    Count int `json:"count"`
}

// Controller: Logic + dependencies, wraps state
type CounterController struct {
    State  *CounterState `lvt:"state"` // Persisted to Redis
    DB     *sql.DB                     // NOT persisted
    Logger *slog.Logger                // NOT persisted
}
```

**State characteristics:**
- Contains only data fields
- JSON-serializable
- No pointers to external resources
- Can be safely stored in Redis

**Controller characteristics:**
- Contains state reference(s) tagged with `lvt:"state"`
- Holds dependencies (DB connections, loggers, clients)
- Defines action methods
- Dependencies are cloned from template, not serialized

## State Tags

The `lvt:"state"` struct tag marks fields for persistence:

```go
type UserController struct {
    // These fields ARE persisted to Redis
    Profile  *UserProfile  `lvt:"state"`
    Settings *UserSettings `lvt:"state"`

    // These fields are NOT persisted
    DB       *sql.DB       // Database connection
    Logger   *slog.Logger  // Logger instance
    Client   *http.Client  // HTTP client
}
```

**How it works:**

1. When saving to Redis, only `lvt:"state"` tagged fields are serialized
2. When loading from Redis, state is deserialized and injected
3. Dependencies come from cloning the template controller (not from Redis)

**Serialization flow:**
```
Save: Controller → ExtractState() → Serialize → Redis
Load: Redis → Deserialize → Clone template → InjectState() → Controller
```

## Action Dispatch

Define methods matching action names—routing happens automatically:

```go
// Action "increment" → method Increment()
func (c *CounterController) Increment(ctx *livetemplate.ActionContext) error {
    c.State.Count++
    c.Logger.Info("counter incremented", slog.Int("value", c.State.Count))
    return nil
}

// Action "set_value" → method SetValue()
func (c *CounterController) SetValue(ctx *livetemplate.ActionContext) error {
    c.State.Count = ctx.GetInt("value")
    return nil
}
```

**Naming conventions:**

| Action Name | Method Name |
|-------------|-------------|
| `increment` | `Increment()` |
| `addItem` | `AddItem()` |
| `add_item` | `AddItem()` |
| `setUserProfile` | `SetUserProfile()` |
| `set_user_profile` | `SetUserProfile()` |

**Method signature requirement:**

```go
func (receiver *ControllerType) MethodName(ctx *livetemplate.ActionContext) error
```

- Must be a pointer receiver
- Must accept `*ActionContext` as the only parameter
- Must return `error`
- Methods with wrong signatures are silently ignored

**Performance:** Method lookups are cached by type. After the first call, routing is O(1) with zero reflection overhead.

## ActionContext API

The `ActionContext` provides access to action data sent from the client:

```go
type ActionContext struct {
    Action string        // The action name (e.g., "increment")
    Data   *ActionData   // Form/JSON data from client
    Ctx    context.Context
}
```

### Data Extraction Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `GetString(key)` | `string` | Get string value (empty if missing) |
| `GetInt(key)` | `int` | Get integer value (0 if missing/invalid) |
| `GetFloat(key)` | `float64` | Get float value (0 if missing/invalid) |
| `GetBool(key)` | `bool` | Get boolean value (false if missing) |
| `Has(key)` | `bool` | Check if key exists |

Example:

```go
func (c *Controller) UpdateSettings(ctx *livetemplate.ActionContext) error {
    theme := ctx.GetString("theme")
    fontSize := ctx.GetInt("fontSize")
    darkMode := ctx.GetBool("darkMode")

    if !ctx.Has("theme") {
        return livetemplate.FieldError{Field: "theme", Message: "Theme is required"}
    }

    c.State.Theme = theme
    c.State.FontSize = fontSize
    c.State.DarkMode = darkMode
    return nil
}
```

### Struct Binding

Bind form data directly to a struct:

```go
type CreateUserForm struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func (c *Controller) CreateUser(ctx *livetemplate.ActionContext) error {
    var form CreateUserForm
    if err := ctx.Bind(&form); err != nil {
        return err
    }

    // Use form.Name, form.Email, form.Age
    return c.DB.CreateUser(form)
}
```

### Binding with Validation

Use with a validator (e.g., `go-playground/validator`):

```go
type RegisterForm struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Name     string `json:"name" validate:"required"`
}

func (c *Controller) Register(ctx *livetemplate.ActionContext) error {
    var form RegisterForm
    validate := validator.New()

    if err := ctx.BindAndValidate(&form, validate); err != nil {
        return err // Returns MultiError with field-specific messages
    }

    return c.createAccount(form)
}
```

### HTTP Context

For HTTP POST actions (not WebSocket), you can access HTTP primitives:

```go
func (c *Controller) Login(ctx *livetemplate.ActionContext) error {
    if !ctx.IsHTTP() {
        return errors.New("login requires HTTP")
    }

    // Set cookie
    ctx.SetCookie(&http.Cookie{
        Name:  "session",
        Value: token,
    })

    // Redirect
    ctx.Redirect("/dashboard")

    return nil
}
```

## Initialization

The `StoreInitializer` interface allows setup after a controller is cloned:

```go
type StoreInitializer interface {
    Init() error
}
```

**When Init() is called:**
1. New session created → Template controller cloned → `Init()` called
2. Existing session loaded → Template cloned → State injected → `Init()` called

**Use cases:**
- Load initial data from database
- Set up computed fields
- Validate state consistency

Example:

```go
type TodoController struct {
    State  *TodoState `lvt:"state"`
    DB     *sql.DB
    Logger *slog.Logger
}

func (c *TodoController) Init() error {
    // Load todos from database if state is empty
    if len(c.State.Items) == 0 {
        items, err := c.DB.LoadTodos(c.State.UserID)
        if err != nil {
            return fmt.Errorf("failed to load todos: %w", err)
        }
        c.State.Items = items
    }
    return nil
}
```

## Error Handling

### FieldError

Return a single field error:

```go
func (c *Controller) UpdateEmail(ctx *livetemplate.ActionContext) error {
    email := ctx.GetString("email")

    if !isValidEmail(email) {
        return livetemplate.FieldError{
            Field:   "email",
            Message: "Please enter a valid email address",
        }
    }

    c.State.Email = email
    return nil
}
```

### MultiError

Return multiple field errors at once:

```go
func (c *Controller) Register(ctx *livetemplate.ActionContext) error {
    var errors livetemplate.MultiError

    email := ctx.GetString("email")
    password := ctx.GetString("password")

    if !isValidEmail(email) {
        errors = append(errors, livetemplate.FieldError{
            Field:   "email",
            Message: "Invalid email format",
        })
    }

    if len(password) < 8 {
        errors = append(errors, livetemplate.FieldError{
            Field:   "password",
            Message: "Password must be at least 8 characters",
        })
    }

    if len(errors) > 0 {
        return errors
    }

    return c.createUser(email, password)
}
```

### General Errors

Non-field errors are sent as `_general`:

```go
func (c *Controller) SaveData(ctx *livetemplate.ActionContext) error {
    if err := c.DB.Save(c.State); err != nil {
        // Appears as general error in template
        return fmt.Errorf("failed to save: %w", err)
    }
    return nil
}
```

**Client-side handling:**

```html
{{if .Errors.email}}
  <span class="error">{{.Errors.email}}</span>
{{end}}

{{if .Errors._general}}
  <div class="alert">{{.Errors._general}}</div>
{{end}}
```

## Common Patterns

### Counter with Logging

```go
type CounterState struct {
    Count int `json:"count"`
}

type CounterController struct {
    State  *CounterState `lvt:"state"`
    Logger *slog.Logger
}

func (c *CounterController) Increment(ctx *livetemplate.ActionContext) error {
    c.State.Count++
    c.Logger.Info("incremented", slog.Int("count", c.State.Count))
    return nil
}

func (c *CounterController) Decrement(ctx *livetemplate.ActionContext) error {
    if c.State.Count > 0 {
        c.State.Count--
    }
    return nil
}
```

### Form with Validation

```go
type ContactState struct {
    Name    string
    Email   string
    Message string
    Sent    bool
}

type ContactController struct {
    State  *ContactState `lvt:"state"`
    Mailer *mail.Client
}

func (c *ContactController) Submit(ctx *livetemplate.ActionContext) error {
    var errors livetemplate.MultiError

    c.State.Name = strings.TrimSpace(ctx.GetString("name"))
    c.State.Email = strings.TrimSpace(ctx.GetString("email"))
    c.State.Message = strings.TrimSpace(ctx.GetString("message"))

    if c.State.Name == "" {
        errors = append(errors, livetemplate.FieldError{
            Field: "name", Message: "Name is required",
        })
    }
    if !isValidEmail(c.State.Email) {
        errors = append(errors, livetemplate.FieldError{
            Field: "email", Message: "Valid email is required",
        })
    }
    if len(c.State.Message) < 10 {
        errors = append(errors, livetemplate.FieldError{
            Field: "message", Message: "Message must be at least 10 characters",
        })
    }

    if len(errors) > 0 {
        return errors
    }

    if err := c.Mailer.Send(c.State.Email, c.State.Message); err != nil {
        return fmt.Errorf("failed to send message")
    }

    c.State.Sent = true
    return nil
}
```

### CRUD with Database

```go
type TodoState struct {
    Items []Todo `json:"items"`
}

type Todo struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    Completed bool   `json:"completed"`
}

type TodoController struct {
    State *TodoState `lvt:"state"`
    DB    *sql.DB
}

func (c *TodoController) AddTodo(ctx *livetemplate.ActionContext) error {
    title := strings.TrimSpace(ctx.GetString("title"))
    if title == "" {
        return livetemplate.FieldError{Field: "title", Message: "Title required"}
    }

    todo := Todo{
        ID:    uuid.New().String(),
        Title: title,
    }

    if err := c.DB.InsertTodo(todo); err != nil {
        return fmt.Errorf("database error")
    }

    c.State.Items = append(c.State.Items, todo)
    return nil
}

func (c *TodoController) ToggleTodo(ctx *livetemplate.ActionContext) error {
    id := ctx.GetString("id")

    for i := range c.State.Items {
        if c.State.Items[i].ID == id {
            c.State.Items[i].Completed = !c.State.Items[i].Completed
            return c.DB.UpdateTodo(c.State.Items[i])
        }
    }

    return fmt.Errorf("todo not found")
}

func (c *TodoController) DeleteTodo(ctx *livetemplate.ActionContext) error {
    id := ctx.GetString("id")

    for i, todo := range c.State.Items {
        if todo.ID == id {
            if err := c.DB.DeleteTodo(id); err != nil {
                return fmt.Errorf("database error")
            }
            c.State.Items = append(c.State.Items[:i], c.State.Items[i+1:]...)
            return nil
        }
    }

    return fmt.Errorf("todo not found")
}
```

## Registration

```go
// Simple store (no dependencies)
counter := &Counter{Count: 0}
tmpl.Handle(counter)

// Controller with dependencies
controller := &MyController{
    State:  &MyState{},
    DB:     db,
    Logger: logger,
}
tmpl.Handle(controller)

// Multiple stores
tmpl.Handle(livetemplate.Stores{
    "counter": &Counter{},
    "user":    &UserController{State: &UserState{}, DB: db},
})
```

## See Also

- [Session Reference](session.md) - Session stores and connection management
- [Server Actions Reference](server-actions.md) - Server-initiated updates with SessionAware
- [Error Handling Reference](error-handling.md) - Detailed error handling patterns
