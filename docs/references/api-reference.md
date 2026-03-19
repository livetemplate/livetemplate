# Go Library API Reference

> **Scope:** This reference documents the **`livetemplate` Go library** (`github.com/livetemplate/livetemplate`). For the CLI tool, see the [lvt repository](https://github.com/livetemplate/lvt).

## Quick Start

```go
package main

import (
    "log"
    "net/http"

    "github.com/livetemplate/livetemplate"
)

type AppController struct {
    // Dependencies (singleton, never cloned)
}

type AppState struct {
    Count int
}

func (c *AppController) Increment(state AppState, ctx *livetemplate.Context) (AppState, error) {
    state.Count++
    return state, nil
}

func main() {
    tmpl, err := livetemplate.New("app")
    if err != nil {
        log.Fatal(err)
    }
    tmpl.Parse(`<h1>Count: {{.Count}}</h1><button lvt-click="increment">+</button>`)

    handler := tmpl.Handle(&AppController{}, livetemplate.AsState(&AppState{}))
    http.Handle("/", handler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## Template

The `Template` type manages template parsing, execution, and tree-based update generation.

### Creating Templates

```go
func New(name string, opts ...Option) (*Template, error)
```

Creates a new template with the given name and options. Auto-discovers template files in the caller's directory.

```go
tmpl, err := livetemplate.New("todos",
    livetemplate.WithDevMode(true),
    livetemplate.WithSessionStore(redisStore),
)
```

### Parsing

```go
func (t *Template) Parse(text string) (*Template, error)
```

Parses a template string. Supports `{{define}}`, `{{block}}`, and `{{template}}` composition via automatic flattening.

```go
func (t *Template) ParseFiles(filenames ...string) (*Template, error)
```

Parses template files. The first file is the main template; additional files provide definitions.

```go
func (t *Template) ParseGlob(pattern string) (*Template, error)
```

Parses all template files matching the glob pattern.

```go
func (t *Template) Funcs(funcMap template.FuncMap) *Template
```

Registers custom template functions. Must be called before `Parse` or `ParseFiles`.

### Mounting

```go
func (t *Template) Handle(controller interface{}, state State, opts ...HandleOption) LiveHandler
```

Creates an HTTP/WebSocket handler from a controller and initial state. The controller is a singleton holding dependencies; state is cloned per session.

```go
handler := tmpl.Handle(&TodoController{DB: db}, livetemplate.AsState(&TodoState{}))
```

**HandleOption:**

| Option | Description |
|--------|-------------|
| `WithStore(store SessionStore)` | Override the session store for this handler |

---

## Controller+State Pattern

### State Interface

```go
type State interface {
    encoding.BinaryMarshaler
    encoding.BinaryUnmarshaler
    Inner() any
}
```

State represents serializable session data. Use `AsState[T]()` instead of implementing directly.

### AsState

```go
func AsState[T any](s *T) State
```

Wraps a plain struct pointer to satisfy the `State` interface using JSON serialization.

```go
state := livetemplate.AsState(&TodoState{Items: []Todo{}})
```

### Lifecycle Methods

Controllers may implement these optional lifecycle methods:

```go
// Called once when a new session is created
func (c *Controller) Mount(state S, ctx *livetemplate.Context) (S, error)

// Called on each WebSocket connect (including reconnects)
func (c *Controller) OnConnect(state S, ctx *livetemplate.Context) (S, error)

// Called when a WebSocket disconnects
func (c *Controller) OnDisconnect()
```

### Action Methods

Action methods handle user interactions. They receive the current state and return the modified state:

```go
func (c *Controller) ActionName(state S, ctx *livetemplate.Context) (S, error)
```

Action dispatch is automatic: `lvt-click="addItem"` dispatches to `AddItem()`.

### Struct Tags

> **Deprecated:** Struct tags are legacy from the pre-v0.7.0 API where controllers and state
> were a single struct. In the current Controller+State pattern, use `AsState[T]()` to
> designate the entire state struct — no field-level tags needed. These tags remain for
> backward compatibility but will be removed in a future release.

| Tag | Description |
|-----|-------------|
| `lvt:"state"` | Mark fields for selective persistence |
| `lvt:"transient"` | Field is cleared on page reload/reconnect |

### Testing

```go
func AssertPureState[T any](t *testing.T)
```

Validates that a state type contains only serializable data. Catches accidental inclusion of `*sql.DB`, `*slog.Logger`, etc.

```go
func TestTodoState(t *testing.T) {
    livetemplate.AssertPureState[TodoState](t)
}
```

---

## Context

`Context` is the unified context for all lifecycle and action methods. It embeds `context.Context`.

### Constructor

```go
func NewContext(ctx context.Context, action string, data map[string]interface{}) *Context
```

Primarily useful in tests. In production, Context is created internally and passed to controller methods.

### Action and Identity

| Method | Signature | Description |
|--------|-----------|-------------|
| `Action` | `() string` | Returns the action name that triggered this context |
| `UserID` | `() string` | Returns the authenticated user's ID |
| `Session` | `() Session` | Returns the Session for server-initiated actions |
| `IsHTTP` | `() bool` | Whether this is an HTTP (not WebSocket) context |

### Data Extraction

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetString` | `(key string) string` | Get a string value from action data |
| `GetInt` | `(key string) int` | Get an integer value |
| `GetFloat` | `(key string) float64` | Get a float value |
| `GetBool` | `(key string) bool` | Get a boolean value |
| `Has` | `(key string) bool` | Check if a key exists in action data |
| `Get` | `(key string) interface{}` | Get a raw value |
| `Bind` | `(v interface{}) error` | Unmarshal action data into a struct |
| `BindAndValidate` | `(v interface{}, validate *validator.Validate) error` | Bind and validate in one step |

### HTTP Operations

These methods return `ErrNoHTTPContext` when called from a WebSocket action.

| Method | Signature | Description |
|--------|-----------|-------------|
| `SetCookie` | `(cookie *http.Cookie) error` | Set an HTTP cookie |
| `DeleteCookie` | `(name string) error` | Delete an HTTP cookie |
| `GetCookie` | `(name string) (*http.Cookie, error)` | Get an HTTP cookie |
| `Redirect` | `(url string, code int) error` | Send an HTTP redirect (3xx only, relative paths only) |

### Upload Access

| Method | Signature | Description |
|--------|-----------|-------------|
| `HasUploads` | `(name string) bool` | Check if uploads exist for a field |
| `GetCompletedUploads` | `(name string) []*UploadEntry` | Get completed upload entries |

### Flash Messages

```go
func (c *Context) SetFlash(key, message string)
```

Sets a flash message available in templates via `.lvt.Flash(key)`. Common keys: `"success"`, `"error"`, `"info"`, `"warning"`. Flash messages are cleared after each render.

### Context Builders

| Method | Signature | Description |
|--------|-----------|-------------|
| `WithUserID` | `(userID string) *Context` | Returns new Context with user ID |
| `WithSession` | `(session Session) *Context` | Returns new Context with session |
| `WithHTTP` | `(w http.ResponseWriter, r *http.Request) *Context` | Returns new Context with HTTP |
| `WithAction` | `(action string) *Context` | Returns new Context with action name |
| `WithData` | `(data map[string]interface{}) *Context` | Returns new Context with data |
| `WithUploads` | `(uploads UploadAccessor) *Context` | Returns new Context with uploads |
| `WithFlashSetter` | `(setter FlashSetter) *Context` | Returns new Context with flash setter |

---

## Session

```go
type Session interface {
    TriggerAction(action string, data map[string]interface{}) error
}
```

Enables server-initiated actions for the current user's connections (all tabs/devices).

Use cases: timers, background job notifications, webhook-triggered updates.

Session is accessed by implementing the `SessionAware` interface on your controller. The `ctx.Session()` method exists on `*Context` but is not populated by the framework in action handlers — use `SessionAware.OnConnect` to receive a `Session` handle for background goroutines that need to trigger actions asynchronously.

**Note:** The `SessionAware.OnConnect(ctx context.Context, session Session) error` method is distinct from the lifecycle `OnConnect(state S, ctx *Context) (S, error)`. The lifecycle version handles state updates on WebSocket connect. The `SessionAware` version provides a `Session` handle for background goroutines that need to trigger actions asynchronously. A controller can implement both.

```go
// Controller implements SessionAware to receive the session on connect
type TimerController struct {
    session livetemplate.Session
}

func (c *TimerController) OnConnect(ctx context.Context, session livetemplate.Session) error {
    c.session = session
    go func() {
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                c.session.TriggerAction("tick", nil)
            case <-ctx.Done():
                return
            }
        }
    }()
    return nil
}
```

---

## LiveHandler

```go
type LiveHandler interface {
    http.Handler
    Shutdown(ctx context.Context) error
    MetricsHandler() http.Handler
}
```

Returned by `Template.Handle()`. Serves both HTTP and WebSocket requests.

| Method | Description |
|--------|-------------|
| `ServeHTTP` | Handles HTTP requests and WebSocket upgrades |
| `Shutdown` | Gracefully drains connections with context timeout |
| `MetricsHandler` | Returns Prometheus metrics endpoint handler |

---

## Authentication

### Authenticator Interface

```go
type Authenticator interface {
    Identify(r *http.Request) (userID string, err error)
    GetSessionGroup(r *http.Request, userID string) (groupID string, err error)
}
```

Maps HTTP requests to user IDs and session groups.

### Built-in Authenticators

**AnonymousAuthenticator** (default): Browser-based session grouping via persistent cookie. All tabs in the same browser share state.

```go
// Used by default - no configuration needed
```

**BasicAuthenticator**: HTTP Basic Auth with user-provided validation.

> **Security:** Basic Auth transmits credentials in base64 (not encrypted). Always use HTTPS
> in production. The BasicAuthenticator provides **no built-in rate limiting or account lockout**, so you must enforce brute-force protection (for example via a WAF, reverse proxy, or auth service) before using it in production.
> Consider session-based auth for browser-facing applications.

```go
func NewBasicAuthenticator(validateFunc func(username, password string) (bool, error)) *BasicAuthenticator
```

```go
auth := livetemplate.NewBasicAuthenticator(func(user, pass string) (bool, error) {
    return db.ValidateUser(user, pass)
})
tmpl, _ := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
```

---

## Session Stores

### SessionStore Interface

```go
type SessionStore interface {
    Get(ctx context.Context, groupID string) interface{}
    Set(ctx context.Context, groupID string, state interface{})
    Delete(ctx context.Context, groupID string)
    List(ctx context.Context) []string
}
```

### SingleStoreSetter Interface

Optional optimization for targeted persistence of individual stores:

```go
type SingleStoreSetter interface {
    SetStore(ctx context.Context, groupID string, storeName string, store interface{})
}
```

### MemorySessionStore

In-memory store with automatic cleanup. Suitable for single-instance deployments.

```go
func NewMemorySessionStore(opts ...SessionStoreOption) *MemorySessionStore
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithCleanupTTL(ttl)` | 24h | TTL for inactive session groups |
| `WithCleanupInterval(interval)` | 1h | Cleanup goroutine interval |

```go
store := livetemplate.NewMemorySessionStore(
    livetemplate.WithCleanupTTL(1 * time.Hour),
)
defer store.Close()
```

### RedisSessionStore

Redis-backed store for distributed deployments. Supports Redis, Redis Cluster, Ring, and Sentinel.

```go
func NewRedisSessionStore(client redis.UniversalClient, opts ...RedisSessionStoreOption) *RedisSessionStore
```

| Option | Default | Description |
|--------|---------|-------------|
| `WithSessionTTL(ttl)` | 24h | Session expiry in Redis |
| `WithMaxRetries(n)` | 3 | Retry attempts with exponential backoff |
| `WithRetryDelay(delay)` | 100ms | Base delay between retries |

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
store := livetemplate.NewRedisSessionStore(client,
    livetemplate.WithSessionTTL(24 * time.Hour),
)
defer store.Close()
```

Both implementations satisfy `SingleStoreSetter`.

---

## Configuration

### Environment Variables

Load configuration from environment variables with `LVT_` prefix:

```go
func LoadEnvConfig() (*EnvConfig, error)
```

```go
config, err := livetemplate.LoadEnvConfig()
if err != nil {
    log.Fatal(err)
}
tmpl, _ := livetemplate.New("app", config.ToOptions()...)
```

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LVT_MAX_CONNECTIONS` | int64 | 0 (unlimited) | Max concurrent WebSocket connections |
| `LVT_MAX_CONNECTIONS_PER_GROUP` | int64 | 0 (unlimited) | Max connections per session group |
| `LVT_ALLOWED_ORIGINS` | string | "" | Comma-separated allowed WebSocket origins |
| `LVT_DEV_MODE` | bool | false | Enable development mode |
| `LVT_WEBSOCKET_DISABLED` | bool | false | HTTP-only mode |
| `LVT_LOADING_DISABLED` | bool | false | Disable automatic loading indicator |
| `LVT_TEMPLATE_BASE_DIR` | string | "" | Base directory for template discovery |
| `LVT_SHUTDOWN_TIMEOUT` | duration | 30s | Graceful shutdown timeout (reserved, not yet applied) |
| `LVT_LOG_LEVEL` | string | "info" | Log level (reserved, not yet applied) |
| `LVT_METRICS_ENABLED` | bool | true | Enable Prometheus metrics (reserved, not yet applied) |
| `LVT_PROGRESSIVE_ENHANCEMENT` | bool | true | Enable non-JS form submission |
| `LVT_WS_BUFFER_SIZE` | int | 50 | WebSocket send buffer size per connection |

### Template Option Functions

Options passed to `New()`:

| Option | Signature | Description |
|--------|-----------|-------------|
| `WithDevMode` | `(enabled bool)` | Enable development mode |
| `WithSessionStore` | `(store SessionStore)` | Set session store |
| `WithAuthenticator` | `(auth Authenticator)` | Set authenticator |
| `WithAllowedOrigins` | `(origins []string)` | Allowed WebSocket origins |
| `WithPermissiveOriginCheck` | `()` | Bypass origin check (dev only) |
| `WithMaxConnections` | `(max int64)` | Max WebSocket connections |
| `WithMaxConnectionsPerGroup` | `(max int64)` | Max connections per group |
| `WithWebSocketDisabled` | `()` | HTTP-only mode |
| `WithWebSocketBufferSize` | `(size int)` | WebSocket send buffer size |
| `WithLoadingDisabled` | `()` | Disable loading indicator |
| `WithMessageRateLimit` | `(messagesPerSecond float64, burstCapacity int)` | WebSocket rate limiting |
| `WithCookieMaxAge` | `(maxAge time.Duration)` | Session cookie max age |
| `WithUpgrader` | `(upgrader *websocket.Upgrader)` | Custom WebSocket upgrader |
| `WithParseFiles` | `(files ...string)` | Explicit template files |
| `WithTemplateBaseDir` | `(dir string)` | Template discovery base dir |
| `WithIgnoreTemplateDirs` | `(dirs ...string)` | Skip directories during discovery |
| `WithUpload` | `(name string, config UploadConfig)` | Configure upload field |
| `WithPubSubBroadcaster` | `(broadcaster pubsub.Broadcaster)` | Enable distributed broadcasting |
| `WithComponentTemplates` | `(sets ...*TemplateSet)` | Register component templates |
| `WithProgressiveEnhancement` | `(enabled bool)` | Non-JS form submission support |

---

## Uploads

### WithUpload

Configure upload fields at template creation:

```go
tmpl, _ := livetemplate.New("profile",
    livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
        Accept:      []string{"image/*"},
        MaxFileSize: 5 * 1024 * 1024, // 5MB
        MaxEntries:  1,
        AutoUpload:  true,
    }),
)
```

### UploadConfig

```go
type UploadConfig struct {
    Accept      []string  // Allowed MIME types or extensions
    MaxEntries  int       // Max concurrent files (0 = unlimited)
    MaxFileSize int64     // Max file size in bytes (0 = unlimited)
    AutoUpload  bool      // Start upload on file selection
    ChunkSize   int       // WebSocket chunk size (default: 256KB)
    External    Presigner // Optional presigner for direct-to-storage uploads
}
```

### UploadEntry

```go
type UploadEntry struct {
    ID          string
    ClientName  string    // Original filename
    ClientType  string    // MIME type
    ClientSize  int64     // File size in bytes
    Progress    int       // 0-100
    Valid       bool      // Passed validation
    Done        bool      // Upload completed
    Error       string    // Error message
    TempPath    string    // Server-side temp file (server uploads)
    BytesRecv   int64     // Bytes received
    ExternalRef string    // Storage reference (external uploads)
    CreatedAt   time.Time
    CompletedAt time.Time
}
```

### External Uploads (S3)

```go
type Presigner interface {
    Presign(entry *UploadEntry) (UploadMeta, error)
}

type UploadMeta struct {
    Uploader string            // Provider name (e.g., "s3")
    URL      string            // Presigned upload URL
    Fields   map[string]string // Form fields for multipart POST
    Headers  map[string]string // HTTP headers for PUT
}
```

**S3Presigner:**

```go
func NewS3Presigner(cfg S3Config) (*S3Presigner, error)

type S3Config struct {
    Bucket          string
    Region          string
    AccessKeyID     string        // Optional: uses default chain if empty
    SecretAccessKey string
    Endpoint        string        // Custom endpoint (MinIO, LocalStack)
    Expiry          time.Duration // Default: 15 minutes
    KeyPrefix       string        // S3 key prefix
}
```

For complete upload documentation, see [Upload Reference](uploads.md).

---

## Health Checks

### HealthHandler

```go
func NewHealthHandler(timeout time.Duration) *HealthHandler
```

Provides Kubernetes-ready health endpoints:

```go
health := livetemplate.NewHealthHandler(5 * time.Second)
health.RegisterChecker("database", dbChecker)
health.RegisterChecker("redis", livetemplate.NewRedisHealthChecker(redisStore))

mux.HandleFunc("/health/live", health.Live)
mux.HandleFunc("/health/ready", health.Ready)
```

### HealthChecker Interface

```go
type HealthChecker interface {
    Check(ctx context.Context) error
}
```

### Built-in Checkers

```go
func NewSessionStoreHealthChecker(store SessionStore) *SessionStoreHealthChecker
func NewRedisHealthChecker(store *RedisSessionStore) *RedisHealthChecker
```

---

## PubSub (Distributed Broadcasting)

Package `pubsub` provides cross-instance messaging for horizontally scaled deployments. See the [PubSub Reference](pubsub.md) for the complete API including `Broadcaster`, `DynamicSubscriber`, broadcast scopes, channel schema, and subscription lifecycle.

---

## Error Types

### Sentinel Errors

| Error | Description |
|-------|-------------|
| `ErrNoHTTPContext` | HTTP method called from WebSocket action |
| `ErrInvalidRedirectCode` | Redirect status code is not 3xx |
| `ErrInvalidRedirectURL` | Redirect URL is not a valid relative path |
| `ErrMethodNotFound` | No controller method matches the action name |

### FieldError

```go
type FieldError struct {
    Field   string
    Message string
}

func NewFieldError(field string, err error) FieldError
```

### MultiError

```go
type MultiError []FieldError
```

A collection of field errors. Implements the `error` interface. Return from action methods to display per-field validation errors in templates.

### ValidationToMultiError

```go
func ValidationToMultiError(err error) MultiError
```

Converts `go-playground/validator` errors to `MultiError`.

---

## See Also

- [Controller+State Pattern](controller-pattern.md) - Architecture guide
- [Upload Reference](uploads.md) - File upload system
- [Configuration](CONFIGURATION.md) - Environment and option configuration
- [Authentication](authentication.md) - Auth setup
- [Session Management](session.md) - Session stores and persistence
- [Server Actions](server-actions.md) - Broadcasting and server-initiated updates
- [Error Handling](error-handling.md) - Validation and error display
- [Client Attributes](client-attributes.md) - Template attribute reference
