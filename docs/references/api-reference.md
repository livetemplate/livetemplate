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
    tmpl.Parse(`<h1>Count: {{.Count}}</h1><button name="increment">+</button>`)

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

For patterns, examples, and usage guide, see [Controller+State Pattern](controller-pattern.md).

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

### Lifecycle Methods

Controllers may implement these optional lifecycle methods:

```go
// Called on every HTTP request (GET and POST) and every WebSocket connect (new and reconnect)
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

Action dispatch is automatic: `<button name="addItem">` dispatches to `AddItem()`.

### Struct Tags

| Tag | Description |
|-----|-------------|
| `lvt:"persist"` | Field is persisted to SessionStore, survives page refresh. Fields without this tag are ephemeral. |

### Testing

```go
func AssertPureState[T any](t *testing.T)
```

Validates that a state type contains only serializable data (no `*sql.DB`, `*slog.Logger`, etc.).

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
| `SelfTopic` | `() string` | Returns the reserved-namespace topic (`lvt:session:<groupID>`) that identifies this session's own connections. ACL-exempt — always subscribable. |
| `Subscribe` | `(topic string) error` | Subscribes the calling connection to a topic. `SelfTopic()` bypasses the ACL; developer topics run `WithTopicACL`. Returns `*TopicForbiddenError` when the ACL denies. |
| `Unsubscribe` | `(topic string) error` | Removes the calling connection's subscription. |
| `Publish` | `(topic, action string, data map[string]interface{})` | Queues a named action dispatch to every connection subscribed to the topic. Fan-out is opt-in — only Subscribers receive it. Deferred until the current action completes successfully. |

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
func (c *Context) SetFlash(key, message string, opts ...FlashOption)
func (c *Context) ClearFlash(key string)
func FlashExpiry(d time.Duration) FlashOption
```

Manages flash messages available in templates via `.lvt.Flash(key)`. Common keys: `"success"`, `"error"`, `"info"`, `"warning"`.

Flash **persists until explicitly cleared** with `ClearFlash` (or until `FlashExpiry` elapses). Background updates such as `TriggerAction` or scan-loop refreshes do not touch flash. See [Flash Message Lifecycle](error-handling.md#flash-message-lifecycle) for the full lifecycle, multi-tab behavior, and v0.8 → v0.9 migration guidance.

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

Enables server-initiated actions for every connection in the current
session group. Use cases: timers, background job notifications,
webhook-triggered updates.

**Scope** — `Session.TriggerAction` targets a session group (groupID),
not a user identity (userID). For the typical anonymous flow where each
browser session maps to one group via cookie, this is equivalent to
"all tabs of this browser". For authenticated flows the mapping depends
on how the `Authenticator` assigns groupIDs:

- If `GetSessionGroup` returns a stable groupID keyed on userID, all of
  a user's devices share one group and `TriggerAction` fans out to all
  of them.
- If `GetSessionGroup` returns a per-session groupID, each device has
  its own group and `TriggerAction` only fans out within a single
  device's tabs.

Accessed via `ctx.Session()` inside your controller's `OnConnect(state, ctx)`
lifecycle method (or any action method). The returned `Session` handle
can be captured and used from background goroutines. See
[Server Actions](server-actions.md) for examples.

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

**AnonymousAuthenticator** (default): Browser-based session grouping via persistent cookie. All tabs in the same browser share state. No configuration needed.

**BasicAuthenticator**: HTTP Basic Auth with user-provided validation.

> **Security:** Always use HTTPS. No built-in rate limiting or account lockout -- enforce brute-force protection externally.

```go
func NewBasicAuthenticator(validateFunc func(username, password string) (bool, error)) *BasicAuthenticator
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

Both implementations satisfy `SingleStoreSetter`.

---

## Configuration

### Environment Variables

```go
func LoadEnvConfig() (*EnvConfig, error)
```

Load configuration from environment variables with `LVT_` prefix. Call `config.ToOptions()...` to convert to template options.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LVT_MAX_CONNECTIONS` | int64 | 0 (unlimited) | Max concurrent WebSocket connections |
| `LVT_MAX_CONNECTIONS_PER_GROUP` | int64 | 0 (unlimited) | Max connections per session group |
| `LVT_ALLOWED_ORIGINS` | string | "" | Comma-separated allowed WebSocket origins |
| `LVT_DEV_MODE` | bool | false | Enable development mode |
| `LVT_WEBSOCKET_DISABLED` | bool | false | HTTP-only mode |
| `LVT_LOADING_DISABLED` | bool | false | Disable automatic loading indicator |
| `LVT_TEMPLATE_BASE_DIR` | string | "" | Base directory for template discovery |
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
| `WithParseFS` | `(fsys fs.FS, patterns ...string)` | Templates from an `fs.FS` (e.g. `embed.FS`); precedence over `WithParseFiles` |
| `WithTemplateBaseDir` | `(dir string)` | Template discovery base dir |
| `WithIgnoreTemplateDirs` | `(dirs ...string)` | Skip directories during discovery |
| `WithUpload` | `(name string, config UploadConfig)` | Configure upload field |
| `WithPubSubBroadcaster` | `(broadcaster pubsub.Broadcaster)` | Enable cross-instance peer fan-out via Redis Pub/Sub |
| `WithComponentTemplates` | `(sets ...*TemplateSet)` | Register component templates |
| `WithProgressiveEnhancement` | `(enabled bool)` | Non-JS form submission support |

---

## Uploads

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
    Done        bool      // Upload completed
    Error       string    // Error message
    TempPath    string    // Server-side temp file (server uploads)
    ExternalRef string    // Storage reference (external uploads)
}
```

### Presigner

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

For complete upload documentation including S3 configuration, see [Upload Reference](uploads.md).

---

## Health Checks

### HealthHandler

```go
func NewHealthHandler(timeout time.Duration) *HealthHandler
```

Provides Kubernetes-ready `/health/live` and `/health/ready` endpoints. Register checkers via `health.RegisterChecker(name, checker)`.

### HealthChecker Interface

```go
type HealthChecker interface {
    Check(ctx context.Context) error
}
```

Built-in: `NewSessionStoreHealthChecker(store)`, `NewRedisHealthChecker(store)`.

---

## PubSub (Cross-Instance Peer Fan-Out)

Package `pubsub` provides cross-instance messaging for horizontally scaled deployments. See the [PubSub Reference](pubsub.md) for the complete API including `Broadcaster`, `DynamicSubscriber`, the `livetemplate:broadcast:*` channel namespace, channel schema, and subscription lifecycle.

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
- [Server Actions](server-actions.md) - Peer fan-out and server-initiated updates
- [Error Handling](error-handling.md) - Validation and error display
- [Client Attributes](client-attributes.md) - Template attribute reference
