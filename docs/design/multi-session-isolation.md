# Multi-Session Data Isolation Design

**Status**: Draft
**Author**: LiveTemplate Team
**Created**: 2025-10-19
**Last Updated**: 2025-10-19 (All review feedback incorporated)

## Table of Contents
1. [Problem Statement](#problem-statement)
2. [Current Architecture](#current-architecture)
3. [Goals and Non-Goals](#goals-and-non-goals)
4. [Proposed Solution](#proposed-solution)
5. [Architecture](#architecture)
6. [API Design](#api-design)
7. [Examples](#examples)
8. [Migration Path](#migration-path)
9. [Alternatives Considered](#alternatives-considered)
10. [Implementation Plan](#implementation-plan)

---

## Problem Statement

LiveTemplate currently treats each WebSocket connection as an independent entity with its own isolated state. While this works for simple single-tab applications, it creates confusion and limitations in real-world usage. Users expect their browser tabs to share state (like how Gmail shows the same inbox in multiple tabs), and multi-user applications need to isolate data by user identity. This design addresses these fundamental architectural gaps by introducing session grouping and authentication-aware state management.

### Current Limitations

**Problem 1: No Multi-Tab Data Sharing for Anonymous Users**
```go
// Current behavior: Each WebSocket connection gets independent state
tmpl := livetemplate.New("counter")
http.Handle("/", tmpl.Handle(&CounterState{Counter: 0}))

// User opens Tab 1: Counter = 0
// User clicks increment in Tab 1: Counter = 1
// User opens Tab 2: Counter = 0 (independent!)
// ❌ Tabs don't share state - confusing UX
```

**Problem 2: No User-Based Data Isolation**
```go
// Current: All users share the same state
// User A sees User B's data
// No way to isolate data by user
// ❌ Cannot build multi-user applications
```

**Problem 3: No Authorized Broadcasting**
```go
// Need to broadcast updates to specific users
// Example: Admin creates announcement → notify all users
// Example: New message → notify only chat room members
// ❌ No mechanism to broadcast with authorization
```

### Use Cases

1. **Anonymous Multi-Tab Sharing** (most common)
   - User opens app in multiple browser tabs
   - Tabs should share same data automatically
   - Different browsers should have independent data

2. **Authenticated Multi-User Apps**
   - Each user has their own isolated data
   - Multiple tabs for same user share data
   - Different users never see each other's data

3. **Server-Initiated Broadcasting**
   - Admin broadcasts system notifications
   - Background jobs update UI for specific users
   - Real-time collaboration features

---

## Current Architecture

### WebSocket Flow
```
Request → WebSocket Upgrade
       → Clone Template (per-connection state)
       → Clone Stores (per-connection state)
       → Handle Messages
```

Each WebSocket connection gets:
- Its own template instance (for tree diffing)
- Its own store instances (independent data)
- No sharing between connections

### HTTP Flow
```
Request → Get Session ID (from cookie/header)
       → Get Session State (from SessionStore)
       → Handle GET/POST
       → Save Session State
```

HTTP uses session cookies, but:
- SessionStore stores arbitrary `interface{}`
- Not integrated with WebSocket flow
- No concept of "session groups"

### Key Files
- `template.go`: Template creation and configuration
- `mount.go`: HTTP/WebSocket handler logic
- `session.go`: SessionStore interface (HTTP-only)
- `action.go`: Store interface and action handling

---

## Goals and Non-Goals

### Goals

1. **Zero-Config Default Behavior**
   - Anonymous users automatically get browser-based session grouping
   - Multiple tabs share data by default
   - In-memory storage by default
   - No breaking changes to existing apps

2. **Flexible Authentication**
   - Pluggable authentication system
   - Support anonymous, basic auth, JWT, OAuth, custom
   - Authentication controls user identity and session grouping

3. **Authorized Broadcasting**
   - Server can push updates to specific users
   - Filter recipients by authorization callback
   - Broadcast to all connections in a session group

4. **Pluggable Persistence**
   - In-memory storage by default
   - Support Redis, database, or custom backends
   - Enable multi-instance deployments

### Non-Goals

1. **Distributed Locking**
   - Not solving distributed consensus
   - Each session group is self-contained
   - Cross-group coordination out of scope

2. **Session Migration**
   - Not moving sessions between servers
   - Sticky sessions assumed for WebSocket
   - Session persistence for reconnection, not migration

3. **Fine-Grained Permissions**
   - Not building a permissions system
   - Simple user ID-based filtering
   - Apps implement their own authorization logic

---

## Proposed Solution

### Core Concepts

**1. Session Groups**

A session group is the fundamental concept that enables state sharing across connections while maintaining isolation between different users.

**What is a session group?**
- A collection of WebSocket/HTTP connections that share the same state (Stores)
- Identified by a unique `groupID` string
- Each group has its own independent Stores instance

**Why do we need session groups?**

Without session groups, every connection is independent. Consider a user with 3 browser tabs:
- Tab 1 increments counter to 5
- Tab 2 shows counter = 0 (different connection = different state)
- Tab 3 shows counter = 0 (confusing!)

With session groups:
- Tab 1, 2, 3 all share `groupID = "browser-abc123"`
- All tabs share the same Stores instance
- Tab 1 increments counter → all tabs see counter = 5
- User experiences seamless multi-tab behavior

**Example: Anonymous User Multi-Tab Sharing**
```go
// Anonymous user opens Tab 1
// Server creates: groupID = "anon-abc123"
// SessionStore.Set("anon-abc123", &CounterState{Count: 0})

// User increments in Tab 1
// State updated: CounterState{Count: 1}

// User opens Tab 2 (same browser, same cookie)
// Server reads cookie: groupID = "anon-abc123"
// SessionStore.Get("anon-abc123") → returns same CounterState{Count: 1}
// Tab 2 immediately shows Count: 1 ✅
```

**Example: Authenticated User Isolation**
```go
// User "alice" logs in from Tab 1
// groupID = "alice"
// SessionStore.Set("alice", &ChatState{Messages: []})

// Alice opens Tab 2
// groupID = "alice" (same user)
// Both tabs share same ChatState

// User "bob" logs in
// groupID = "bob" (different user)
// SessionStore.Set("bob", &ChatState{Messages: []})
// Bob's data completely isolated from Alice ✅
```

**Relationship: userID and groupID**

The Authenticator controls the mapping between users and session groups via two methods:
- `Identify(r)` → returns `userID` (who you are)
- `GetSessionGroup(r, userID)` → returns `groupID` (which session group you belong to)

**Default mappings in our implementation:**

| Scenario | userID | groupID | Behavior |
|----------|--------|---------|----------|
| Anonymous | `""` | `"cookie-abc123"` | Browser-based grouping |
| User Alice | `"alice"` | `"alice"` | User-based grouping |
| User Bob | `"bob"` | `"bob"` | Isolated from Alice |

**Key point:** In the default implementation, `groupID = userID` for authenticated users. This ensures each user has isolated data.

**Why separate userID and groupID?**

The separation provides flexibility:
- **userID**: Identity (who you are)
- **groupID**: State isolation boundary (which data you see)

For most apps: `groupID = userID` (simple, 1:1 mapping)

For advanced apps: custom mapping enables collaboration or multi-context sessions

**Advanced scenarios (not in v1, but architecture allows):**

*Scenario 1: Collaborative Workspaces - Multiple users share one session group*
```go
// Multiple users share one session group
func (a *WorkspaceAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    workspaceID := getWorkspaceFromURL(r) // e.g., "workspace-123"
    return workspaceID, nil
}

// Result:
// Alice: userID="alice", groupID="workspace-123"
// Bob: userID="bob", groupID="workspace-123"
// Both see same shared state (Google Docs-style collaboration)
```

*Scenario 2: Multi-Context Sessions - One user has multiple session groups*
```go
// Same user, different contexts (e.g., admin panel vs public view)
func (a *MultiContextAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    context := r.Header.Get("X-Context") // "admin" or "public"
    return fmt.Sprintf("%s-%s", userID, context), nil
}

// Result:
// Admin viewing admin panel: userID="admin", groupID="admin-admin"
// Admin viewing public site: userID="admin", groupID="admin-public"
// Isolated state for each context
```

**Can multiple userIDs share one groupID?**

Yes, in advanced scenarios (collaborative workspaces), but not in the default implementation. The architecture is designed to support this flexibility, though v1 focuses on the simple 1:1 mapping.

**2. Authentication**
- Authenticator identifies users from requests
- Returns `userID` ("" for anonymous)
- Maps users to session groups via `GetSessionGroup()`

**3. Default Behavior**
- Anonymous users: `groupID` = persistent browser cookie
- All tabs in same browser share same `groupID`
- Different browsers get different `groupID`

**4. Orthogonal Concerns**
- Authentication (who is the user?) - default: anonymous
- Session Storage (where to persist?) - default: in-memory
- These are independent, configurable options

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                         Request                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Authenticator                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Anonymous   │  │  Basic Auth  │  │   JWT Auth   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
                    userID, groupID
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   SessionStore                               │
│                                                               │
│   groupID → Stores (shared across connections)              │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Memory     │  │    Redis     │  │   Database   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              ConnectionRegistry                              │
│                                                               │
│  Tracks: groupID → [connections]                            │
│          userID → [connections]                             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Broadcasting                                │
│                                                               │
│  Filter by userID → Find connections → Send updates         │
└─────────────────────────────────────────────────────────────┘
```

---

## Architecture

### 1. Authenticator Interface

```go
// Authenticator identifies users and maps them to session groups
type Authenticator interface {
    // Identify returns the user ID from the request
    // Returns "" for anonymous users
    // Returns error if authentication fails (e.g., invalid credentials)
    Identify(r *http.Request) (userID string, err error)

    // GetSessionGroup returns the session group ID for this user
    // Multiple requests with same groupID share state
    // For anonymous: typically returns browser-based identifier
    // For authenticated: typically returns userID
    GetSessionGroup(r *http.Request, userID string) (groupID string, err error)
}
```

**Design Rationale:**
- Separates "who are you?" from "which session group?"
- Allows flexibility: one user could have multiple session groups
- Example: Admin user might have admin panel session + regular session

### 2. AnonymousAuthenticator (Default)

```go
type AnonymousAuthenticator struct{}

func (a *AnonymousAuthenticator) Identify(r *http.Request) (string, error) {
    return "", nil // Always anonymous
}

func (a *AnonymousAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    // Check for existing session ID cookie
    cookie, err := r.Cookie("livetemplate-id")
    if err == nil {
        return cookie.Value, nil
    }

    // Generate new session ID
    return generateRandomID(), nil
}
```

**Cookie Management:**
- Cookie name: `livetemplate-id` (persistent session identifier)
- Long-lived: 1 year expiration
- Security: `HttpOnly`, `SameSite=Lax`
- Set by both HTTP and WebSocket handlers
- Persists across browser restarts

**Design Rationale:**
- Zero configuration - automatically enabled
- Browser-based grouping feels natural to users
- Cookie approach works with standard browsers
- No server-side state needed for anonymous users

### 3. SessionStore Interface (Refactored)

**Before** (per-HTTP-request):
```go
type SessionStore interface {
    Get(sessionID string) interface{}
    Set(sessionID string, state interface{})
    Delete(sessionID string)
}
```

**After** (per-session-group):
```go
type SessionStore interface {
    Get(groupID string) Stores
    Set(groupID string, stores Stores)
    Delete(groupID string)
    List() []string // Returns all active groupIDs
}
```

**Design Rationale:**
- Type-safe: stores `Stores` instead of `interface{}`
- Group-centric: explicitly stores by groupID
- Supports broadcasting: `List()` enables finding all groups
- Backward compatible: old HTTP session concept maps cleanly

### 4. ConnectionRegistry

```go
type Connection struct {
    Conn     *websocket.Conn
    GroupID  string
    UserID   string
    Template *Template  // Per-connection template for tree diffing
    Stores   Stores     // Shared stores from session group
}

type ConnectionRegistry struct {
    byGroup map[string][]*Connection  // For group-based broadcasting
    byUser  map[string][]*Connection  // For user-based broadcasting
    mu      sync.RWMutex
}
```

**Design Rationale:**
- Dual indexing: by group and by user
- Enables efficient broadcasting to either dimension
- Thread-safe: supports concurrent WebSocket connections
- Auto-cleanup on disconnect prevents memory leaks

### 5. Broadcasting

```go
// On liveHandler
func (h *liveHandler) Broadcast(filter func(userID string) bool) error
func (h *liveHandler) BroadcastToUsers(userIDs []string) error
func (h *liveHandler) BroadcastToGroup(groupID string) error

// On Template (public API)
func (t *Template) Broadcast(filter func(userID string) bool) error
func (t *Template) BroadcastToUsers(userIDs ...string) error
```

**Design Rationale:**
- Simple callback filtering: flexible and easy to use
- User-centric: filter by userID (most common case)
- Group-aware: can broadcast to session groups directly
- Error handling: logs failures but continues to other connections

---

## API Design

### Default Usage (Zero Config)

```go
// Anonymous users, in-memory storage, multi-tab sharing
tmpl := livetemplate.New("counter")
http.Handle("/", tmpl.Handle(state))

// ✅ Multiple tabs automatically share data
// ✅ Different browsers get independent data
// ✅ Zero configuration needed
```

### Custom Authentication

```go
// Basic authentication
auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
    return db.ValidateUser(username, password)
})

tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
http.Handle("/", tmpl.Handle(state))
```

### Custom Session Storage

```go
// Redis-backed session storage for multi-instance deployment
redisStore := NewRedisSessionStore("redis://localhost:6379")

tmpl := livetemplate.New("app", livetemplate.WithSessionStore(redisStore))
http.Handle("/", tmpl.Handle(state))
```

### Combined Custom Config

```go
// JWT auth + Redis storage
jwtAuth := NewJWTAuthenticator(secretKey)
redisStore := NewRedisSessionStore("redis://localhost:6379")

tmpl := livetemplate.New("app",
    livetemplate.WithAuthenticator(jwtAuth),
    livetemplate.WithSessionStore(redisStore))

http.Handle("/", tmpl.Handle(state))
```

### Broadcasting

```go
// Broadcast to all authenticated users
tmpl.Broadcast(func(userID string) bool {
    return userID != "" // Only authenticated users
})

// Broadcast to specific users
tmpl.BroadcastToUsers("user123", "user456")

// Broadcast to admins only
tmpl.Broadcast(func(userID string) bool {
    return isAdmin(userID)
})

// Broadcast from background goroutine
go func() {
    time.Sleep(5 * time.Second)
    tmpl.Broadcast(func(userID string) bool {
        return true // All users
    })
}()
```

---

## Examples

### Example 1: Anonymous Counter (Default)

```go
package main

import (
    "net/http"
    "github.com/livefir/livetemplate"
)

type CounterState struct {
    Counter int `json:"counter"`
}

func (s *CounterState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "increment":
        s.Counter++
    case "decrement":
        s.Counter--
    }
    return nil
}

func main() {
    state := &CounterState{Counter: 0}
    tmpl := livetemplate.New("counter")
    http.Handle("/", tmpl.Handle(state))
    http.ListenAndServe(":8080", nil)
}
```

**Behavior:**
- User opens Tab 1, Tab 2 in Chrome → same counter
- User opens Safari → different counter
- Zero configuration needed

### Example 2: Authenticated Chat

```go
package main

import (
    "net/http"
    "github.com/livefir/livetemplate"
)

type ChatState struct {
    Messages []Message
    Username string
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "send":
        text := ctx.GetString("text")
        s.Messages = append(s.Messages, Message{
            User: s.Username,
            Text: text,
        })
    }
    return nil
}

func main() {
    // Basic authentication
    auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
        return validateUser(username, password)
    })

    tmpl := livetemplate.New("chat", livetemplate.WithAuthenticator(auth))

    http.Handle("/", tmpl.Handle(&ChatState{}))
    http.ListenAndServe(":8080", nil)
}
```

**Behavior:**
- User A and User B login → separate chat histories
- User A opens multiple tabs → same chat history
- Each user completely isolated

### Example 3: Admin Dashboard with Broadcasting

```go
package main

import (
    "net/http"
    "time"
    "github.com/livefir/livetemplate"
)

type DashboardState struct {
    Notifications []string
    IsAdmin       bool
}

func (s *DashboardState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "broadcast":
        if !s.IsAdmin {
            return fmt.Errorf("unauthorized")
        }
        message := ctx.GetString("message")
        // Add to all users' notifications
        // This is handled by broadcasting below
    }
    return nil
}

func main() {
    auth := NewRoleBasedAuthenticator()
    tmpl := livetemplate.New("dashboard", livetemplate.WithAuthenticator(auth))

    http.Handle("/", tmpl.Handle(&DashboardState{}))

    // Background job: broadcast notifications
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            // Broadcast to all users
            tmpl.Broadcast(func(userID string) bool {
                return true // All users
            })
        }
    }()

    http.ListenAndServe(":8080", nil)
}
```

**Behavior:**
- Admin sends announcement → all users receive update
- Background jobs can push updates to UI
- Filter by user role for targeted notifications

---

## Migration Path

### Backward Compatibility

**Existing apps continue to work unchanged:**

```go
// Old code (still works)
tmpl := livetemplate.New("counter")
http.Handle("/", tmpl.Handle(state))
```

**What changes:**
- Before: Each WebSocket connection gets independent state
- After: Connections from same browser share state
- **This is the desired behavior!** Users expect multi-tab sharing

### Breaking Changes

**None.** The new default behavior is strictly better:
- Anonymous users get browser-based grouping automatically
- No API changes required
- Existing apps get multi-tab sharing for free

### Deprecations

**Old SessionStore usage (HTTP-only):**
- `SessionStore` interface changes to store `Stores` instead of `interface{}`
- Old HTTP session cookies still work
- Migration: Update custom SessionStore implementations

---

## Alternatives Considered

### Alternative 1: Explicit Session Group Configuration

```go
// Rejected: Too much configuration
tmpl := livetemplate.New("app",
    livetemplate.WithSessionGrouping(livetemplate.BrowserBased),
    livetemplate.WithSessionPersistence(livetemplate.InMemory))
```

**Reason for rejection:** Adds complexity for the common case. Default behavior should "just work."

### Alternative 2: Always Per-Connection State

```go
// Rejected: Keep current behavior, add explicit grouping API
tmpl.LinkConnections(conn1, conn2) // Manual grouping
```

**Reason for rejection:** Requires users to manage connections explicitly. Not ergonomic.

### Alternative 3: Authentication Controls Session Storage

```go
// Rejected: Authenticator includes session store
type Authenticator interface {
    Identify(r *http.Request) (userID string, err error)
    GetSessionStore() SessionStore
}
```

**Reason for rejection:** Couples orthogonal concerns. Anonymous users might need Redis storage for multi-instance deployments.

### Alternative 4: Middleware-Based Authentication

```go
// Rejected: Middleware wraps handler
authMiddleware := livetemplate.NewAuthMiddleware(jwtAuth)
http.Handle("/", authMiddleware.Wrap(tmpl.Handle(state)))
```

**Reason for rejection:** Less integrated, harder to configure session storage together.

---

## Implementation Plan

### Phase 1: Authentication Infrastructure
**Files:** `auth.go` (new)

**Tasks:**
- [ ] Define `Authenticator` interface
- [ ] Implement `AnonymousAuthenticator` (default)
- [ ] Implement `BasicAuthenticator`
- [ ] Write unit tests for authenticators
- [ ] Document authentication system

**Estimated Effort:** 1 session

---

### Phase 2: Refactor SessionStore
**Files:** `session.go` (modify)

**Tasks:**
- [ ] Update `SessionStore` interface to store `Stores`
- [ ] Update `MemorySessionStore` implementation
- [ ] Add `List()` method for broadcasting support
- [ ] Write unit tests for SessionStore
- [ ] Ensure backward compatibility

**Estimated Effort:** 1 session

---

### Phase 3: Connection Registry
**Files:** `connection_registry.go` (new)

**Tasks:**
- [ ] Define `Connection` struct
- [ ] Implement `ConnectionRegistry` with dual indexing
- [ ] Add `Register()`, `Unregister()` methods
- [ ] Add `GetByUser()`, `GetByGroup()`, `GetAll()` methods
- [ ] Write unit tests for registry
- [ ] Test concurrent access patterns

**Estimated Effort:** 1 session

---

### Phase 4: Update Template Configuration
**Files:** `template.go` (modify)

**Tasks:**
- [ ] Add `Authenticator` field to `Config`
- [ ] Update `New()` to default to `AnonymousAuthenticator`
- [ ] Add `WithAuthenticator()` option
- [ ] Ensure `WithSessionStore()` works with new interface
- [ ] Write unit tests for config options

**Estimated Effort:** 0.5 session

---

### Phase 5: Integrate with Mount Handler
**Files:** `mount.go` (modify)

**Tasks:**
- [ ] Add `ConnectionRegistry` to `liveHandler`
- [ ] Update `handleWebSocket()` to use Authenticator
- [ ] Set browser-id cookie for anonymous users
- [ ] Use SessionStore to get/set session group stores
- [ ] Register/unregister connections in registry
- [ ] Update `handleHTTP()` similarly
- [ ] Write integration tests

**Estimated Effort:** 2 sessions

---

### Phase 6: Broadcasting System
**Files:** `broadcast.go` (new), `template.go` (modify)

**Tasks:**
- [ ] Implement `Broadcast()` with filter callback
- [ ] Implement `BroadcastToUsers()`
- [ ] Implement `BroadcastToGroup()`
- [ ] Add public API on Template
- [ ] Handle connection failures gracefully
- [ ] Write tests for broadcasting
- [ ] Test concurrent broadcasts

**Estimated Effort:** 1.5 sessions

---

### Phase 7: End-to-End Testing
**Files:** `test_multi_session.go` (new)

**Tasks:**
- [ ] Test anonymous multi-tab sharing
- [ ] Test different browsers get different data
- [ ] Test authenticated user isolation
- [ ] Test broadcasting to filtered users
- [ ] Test broadcasting to specific users
- [ ] Test session persistence (in-memory)
- [ ] Test WebSocket + HTTP interaction

**Estimated Effort:** 1.5 sessions

---

### Phase 8: Example Applications
**Files:** `examples/authenticated_chat/`, `examples/admin_dashboard/`

**Tasks:**
- [ ] Create authenticated chat example
- [ ] Create admin dashboard with broadcasting
- [ ] Update counter example docs (works by default!)
- [ ] Add README for each example
- [ ] Test examples manually

**Estimated Effort:** 1 session

---

### Phase 9: Documentation
**Files:** `docs/authentication.md`, `docs/broadcasting.md`, `README.md`

**Tasks:**
- [ ] Write authentication guide
- [ ] Write broadcasting guide
- [ ] Write session groups explanation
- [ ] Update main README with new features
- [ ] Add API documentation
- [ ] Create migration guide

**Estimated Effort:** 1 session

---

### Phase 10: Optional Extensions
**Files:** `session_store_redis.go` (new)

**Tasks:**
- [ ] Implement RedisSessionStore example
- [ ] Write persistence guide
- [ ] Document multi-instance deployment
- [ ] Add JWT authenticator example

**Estimated Effort:** 1-2 sessions (optional)

---

### Total Estimated Effort
**Core Implementation:** 8-9 sessions
**Optional Extensions:** 1-2 sessions
**Total:** 9-11 sessions

### Dependencies
- Phases 1-4 can be done in parallel
- Phase 5 depends on 1-4
- Phase 6 depends on 5
- Phases 7-10 depend on 6

---

## Success Criteria

### Functional Requirements
- ✅ Anonymous users share data across browser tabs by default
- ✅ Different browsers have independent data
- ✅ Custom authentication works (Basic, JWT, etc.)
- ✅ Broadcasting to filtered users works
- ✅ In-memory session storage is default
- ✅ Custom session storage (Redis) is possible

### Non-Functional Requirements
- ✅ Zero breaking changes to existing apps
- ✅ Zero configuration for common case
- ✅ Thread-safe for concurrent connections
- ✅ Performance: no significant overhead
- ✅ Documentation: comprehensive guides

### Testing Requirements
- ✅ Unit tests for all new components
- ✅ Integration tests for WebSocket + HTTP flows
- ✅ E2E tests for multi-tab scenarios
- ✅ Example apps demonstrate key use cases

---

## Questions and Open Issues

### Q1: Session Expiration
**Question:** Should session groups expire after inactivity?

**Options:**
- In-memory: TTL-based cleanup (e.g., 24 hours)
- Redis: Use Redis TTL
- No expiration: Manual cleanup only

**Decision:** TBD - depends on typical usage patterns

### Q2: Cross-Instance Broadcasting
**Question:** How to broadcast across multiple server instances?

**Options:**
- Redis Pub/Sub
- Message queue (RabbitMQ, Kafka)
- Out of scope for v1

**Decision:** Out of scope for v1. Sticky sessions assumed.

### Q3: Connection Limit per Session Group
**Question:** Should we limit connections per session group?

**Options:**
- Unlimited (current)
- Configurable limit (e.g., 10 tabs max)
- Automatic cleanup of stale connections

**Decision:** TBD - start with unlimited, monitor usage

---

## References

- [WebSocket RFC 6455](https://tools.ietf.org/html/rfc6455)
- [HTTP Cookie RFC 6265](https://tools.ietf.org/html/rfc6265)
- [JWT RFC 7519](https://tools.ietf.org/html/rfc7519)
- [Gorilla WebSocket](https://github.com/gorilla/websocket)

---

## Appendix A: Key Interfaces

```go
// Authenticator identifies users and maps to session groups
type Authenticator interface {
    Identify(r *http.Request) (userID string, err error)
    GetSessionGroup(r *http.Request, userID string) (groupID string, err error)
}

// SessionStore manages session group state
type SessionStore interface {
    Get(groupID string) Stores
    Set(groupID string, stores Stores)
    Delete(groupID string)
    List() []string
}

// Broadcasting methods
func (h *liveHandler) Broadcast(filter func(userID string) bool) error
func (h *liveHandler) BroadcastToUsers(userIDs []string) error
func (h *liveHandler) BroadcastToGroup(groupID string) error
```

---

## Appendix B: Data Flow Diagrams

### Anonymous User Flow
```
1. Browser Tab 1 → GET /
2. Server checks for "livetemplate-id" cookie
3. Not found → Generate sessionID = "abc123"
4. Set cookie: livetemplate-id=abc123
5. SessionStore.Get("abc123") → nil
6. Create new Stores, SessionStore.Set("abc123", stores)
7. Return HTML

8. Browser Tab 1 → WebSocket upgrade
9. Read cookie: sessionID = "abc123"
10. SessionStore.Get("abc123") → existing stores
11. Register connection in registry
12. User increments counter → stores updated

13. Browser Tab 2 → WebSocket upgrade
14. Read cookie: sessionID = "abc123" (same!)
15. SessionStore.Get("abc123") → same stores
16. Tab 2 sees counter value from Tab 1
```

### Authenticated User Flow
```
1. Browser → POST /login (username=alice, password=secret)
2. Authenticator.Identify() → userID="alice"
3. Authenticator.GetSessionGroup() → groupID="alice"
4. SessionStore.Get("alice") → nil
5. Create new Stores for Alice
6. SessionStore.Set("alice", stores)

7. Alice's Tab 1 → WebSocket upgrade
8. Authenticator.Identify() → userID="alice"
9. SessionStore.Get("alice") → Alice's stores
10. Register connection (groupID="alice", userID="alice")

11. Alice's Tab 2 → WebSocket upgrade
12. Same flow → same stores (shared state)

13. Bob logs in → userID="bob", groupID="bob"
14. SessionStore.Get("bob") → separate stores
15. Alice and Bob have completely isolated data
```

### Broadcasting Flow
```
1. Admin triggers broadcast: tmpl.Broadcast(func(uid string) bool { return true })
2. Registry.GetAll() → [conn1, conn2, conn3, ...]
3. For each connection:
   a. Apply filter: filter(conn.UserID) → true/false
   b. If true: render template with conn.Stores
   c. Send update to conn.Conn (WebSocket)
4. All authorized users receive update
```

---

**End of Design Document**
