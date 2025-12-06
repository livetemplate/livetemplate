# Proposal: Controller Dependency Injection

**Status**: Draft
**Created**: 2024-12-06
**Author**: Adnaan

## Summary

The current controller cloning mechanism copies all exported fields via reflection, including dependencies like database connections. This creates architectural ambiguity around dependency lifecycle, sharing semantics, and resource management.

## Background

### Current Behavior

When a new session connects, livetemplate clones the template controller:

```go
// mount.go:1096-1119
func cloneStore(store interface{}) interface{} {
    newStore := reflect.New(storeType).Interface()
    copyStruct(newStore, store)  // Copies ALL exported fields

    if initializer, ok := newStore.(StoreInitializer); ok {
        initializer.Init()
    }
    return newStore
}
```

For a controller like:

```go
type TodoController struct {
    Title string
    Prefs *TodoPrefs  `lvt:"state"`
    View  *TodoView
    DB    *db.Queries `json:"-"`  // Dependency
}
```

The result is:

```
Template.DB = 0x140001234  →  Clone A.DB = 0x140001234  (same pointer)
                           →  Clone B.DB = 0x140001234  (same pointer)
```

All clones share the **same** dependency instance.

### Why This Matters

For `*sql.DB` (connection pool), sharing is correct and efficient. But the framework provides no mechanism to distinguish between:

1. **Shared dependencies** - should be copied (DB pool, config, logger)
2. **Per-session dependencies** - should be freshly created (transactions, session-scoped caches)
3. **Non-copyable dependencies** - should not be copied at all (open file handles, locks)

## Problem Statement

### 1. Implicit Sharing with No Contract

Developers have no way to declare dependency sharing semantics:

```go
type Controller struct {
    DB      *sql.DB       // Shared? Per-session? Framework doesn't know
    Cache   *SessionCache // Should NOT be shared, but will be
    Tx      *sql.Tx       // Dangerous to share, but will be
    Logger  *slog.Logger  // Shared is fine, but what about session context?
}
```

### 2. No Lifecycle Management

- **Acquisition**: Dependencies are copied, not acquired. No factory/provider pattern.
- **Release**: No cleanup hook when session ends. Resources may leak.
- **Refresh**: No way to get a fresh dependency mid-session.

### 3. Private Fields Can't Be Dependencies

Go reflection cannot access unexported fields, so:

```go
type Controller struct {
    db *sql.DB  // Private: NOT copied, clone gets nil
}
```

This forces dependencies to be exported, but exported fields are also candidates for state serialization, requiring careful use of `json:"-"` tags.

### 4. Init() is Insufficient

While `Init()` is called after cloning, it receives a controller with dependencies already (shallow) copied:

```go
func (c *Controller) Init() error {
    // c.DB is already set (copied from template)
    // No context about whether this is initial load or session restore
    // No access to dependency providers
}
```

### 5. Mixing Concerns

A single struct conflates three distinct concepts:

| Concept | Lifecycle | Serialized? | Current Tag |
|---------|-----------|-------------|-------------|
| State (user preferences) | Per-session, persisted | Yes | `lvt:"state"` |
| Computed (derived data) | Per-request, not persisted | Sometimes | None |
| Dependencies (resources) | Shared or per-session | No | `json:"-"` |

## Use Cases Not Well Supported

### 1. Database Transaction Per Session

```go
// Desired: Each session gets its own transaction
type OrderController struct {
    Tx *sql.Tx  // Should be per-session, not shared
}

// Current: All sessions share one transaction (broken)
```

### 2. Session-Scoped Caching

```go
// Desired: Each session has isolated cache
type SearchController struct {
    Cache *lru.Cache  // Should be per-session
}

// Current: All sessions share cache (data leakage)
```

### 3. Authenticated Resources

```go
// Desired: Per-session authenticated client
type APIController struct {
    Client *AuthenticatedHTTPClient  // Has user's OAuth token
}

// Current: All sessions share client (security issue)
```

### 4. Connection Cleanup

```go
// Desired: Close connection when session ends
type StreamController struct {
    Conn *websocket.Conn  // Needs cleanup on disconnect
}

// Current: No automatic cleanup, resource leak
```

## Potential Solutions

### Option A: Dependency Tags

Introduce struct tags to declare dependency semantics:

```go
type Controller struct {
    // Shared across all sessions (copied as pointer)
    DB *sql.DB `lvt:"dep,shared"`

    // Fresh instance per session (requires factory)
    Cache *lru.Cache `lvt:"dep,session"`

    // Not a dependency, regular state
    Prefs *UserPrefs `lvt:"state"`
}
```

**Pros**: Declarative, backward compatible
**Cons**: Requires factory registration, complex tag parsing

### Option B: Dependency Provider Interface

Controllers declare their dependencies via interface:

```go
type DependencyAware interface {
    // Called during clone to inject dependencies
    InjectDependencies(provider DependencyProvider) error
}

type DependencyProvider interface {
    // Get shared dependency by key
    GetShared(key string) (interface{}, error)

    // Create session-scoped dependency
    CreateSession(key string) (interface{}, error)
}

func (c *Controller) InjectDependencies(p DependencyProvider) error {
    c.db = p.GetShared("db").(*sql.DB)
    c.cache = p.CreateSession("cache").(*lru.Cache)
    return nil
}
```

**Pros**: Explicit, testable, supports both patterns
**Cons**: More boilerplate, requires provider setup

### Option C: Factory Functions

Register factory functions for session-scoped dependencies:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithDependency("db", func() interface{} {
        return sharedDB  // Return same instance
    }),
    livetemplate.WithSessionDependency("cache", func(sessionID string) interface{} {
        return lru.New(100)  // Fresh per session
    }),
)

type Controller struct {
    DB    *sql.DB   `lvt:"inject:db"`
    Cache *lru.Cache `lvt:"inject:cache"`
}
```

**Pros**: Clean separation, framework handles lifecycle
**Cons**: Magic injection, harder to trace

### Option D: Explicit Composition

Separate dependencies from state entirely:

```go
// Dependencies are not part of controller struct
type TodoController struct {
    Prefs *TodoPrefs `lvt:"state"`
    View  *TodoView
}

// Dependencies passed via context or method parameter
func (c *TodoController) Add(ctx *ActionContext) error {
    db := ctx.Dependency("db").(*sql.DB)
    // or
    db := GetDB(ctx.Context())
}
```

**Pros**: Clear separation, no cloning issues
**Cons**: Verbose, context threading, loses struct cohesion

### Option E: Lifecycle Hooks

Add explicit lifecycle methods:

```go
type LifecycleAware interface {
    // Called when creating new session
    OnSessionCreate(ctx context.Context) error

    // Called when session ends (disconnect, timeout)
    OnSessionDestroy(ctx context.Context) error
}

func (c *Controller) OnSessionCreate(ctx context.Context) error {
    c.tx, _ = db.BeginTx(ctx, nil)  // Fresh transaction
    return nil
}

func (c *Controller) OnSessionDestroy(ctx context.Context) error {
    return c.tx.Rollback()  // Cleanup
}
```

**Pros**: Explicit lifecycle, supports cleanup
**Cons**: Doesn't solve sharing semantics

## Recommendation

A combination of **Option B (Dependency Provider)** and **Option E (Lifecycle Hooks)** provides the most complete solution:

1. **DependencyProvider** for acquisition semantics (shared vs per-session)
2. **OnSessionDestroy** for cleanup
3. Keep private fields for dependencies (don't rely on reflection copying)
4. **Init()** continues to work for computed field initialization

This approach:
- Makes sharing semantics explicit
- Supports both shared and per-session dependencies
- Enables proper resource cleanup
- Maintains backward compatibility (existing code keeps working)
- Keeps dependencies private (cleaner API)

## Migration Path

1. **Phase 1**: Add optional `DependencyProvider` interface, existing behavior unchanged
2. **Phase 2**: Add `OnSessionDestroy` hook
3. **Phase 3**: Deprecate dependency copying, recommend provider pattern
4. **Phase 4**: Document best practices, update examples

## Questions for Discussion

1. Should dependencies be injected via struct tags or explicit interface?
2. How should dependency providers be registered with the template?
3. Should there be a distinction between "request-scoped" and "session-scoped" dependencies?
4. How does this interact with Redis session restore?
5. Should the framework provide common dependency providers (DB pool, logger)?

## Related

- `mount.go:1096-1119` - cloneStore implementation
- `mount.go:1029-1094` - hydrateStores for state restoration
- `state.go` - lvt:state tag processing
- `examples/todos` - current pattern with shared DB

## Appendix: Current Workarounds

### Workaround 1: Global Dependencies

```go
var globalDB *sql.DB

func (c *Controller) getDB() *sql.DB {
    return globalDB
}
```

**Problems**: Hidden dependency, hard to test, no lifecycle

### Workaround 2: Init() Acquisition

```go
func (c *Controller) Init() error {
    c.cache = lru.New(100)  // Fresh per clone
    return nil
}
```

**Problems**: Can't distinguish initial vs restore, no cleanup

### Workaround 3: Lazy Initialization

```go
func (c *Controller) getCache() *lru.Cache {
    if c.cache == nil {
        c.cache = lru.New(100)
    }
    return c.cache
}
```

**Problems**: Race conditions, no cleanup, scattered initialization
