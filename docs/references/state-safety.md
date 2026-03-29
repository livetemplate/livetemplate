# State Safety Reference

LiveTemplate ensures safe state management through two dimensions: **purity enforcement** (preventing dependency types in state) and **session isolation** (preventing cross-user data leakage). For the full Controller+State pattern, see the [Controller+State Pattern Reference](controller-pattern.md).

---

## State Purity

State purity is enforced through four layers. Each layer catches different classes of mistakes.

### Layer 1: Compile-Time Type Separation

Controllers and state are separate types. Action methods receive state by value and return a modified copy:

```go
func (c *Controller) Action(state State, ctx *livetemplate.Context) (State, error)
```

The controller is a singleton holding shared dependencies (DB, Logger). State is pure data cloned per session. Because state is passed by value, mutations in the action body don't affect the original — the framework only applies the returned copy.

### Layer 2: Runtime Dependency Detection

`AsState[T]()` validates the state type at registration time using `validatePureState[T]()`. This performs a recursive descent through:

- Direct struct fields
- Nested structs and pointer-to-struct fields
- Slice, array, and map element types

If a dependency type is found, `AsState` **panics immediately** with an actionable message:

```
livetemplate.AsState: field DB appears to be a dependency (*sql.DB) - move to controller
```

#### Detected Dependency Patterns

| Pattern | Category |
|---------|----------|
| `*sql.DB` | Database |
| `*sql.Tx` | Database |
| `*sql.Conn` | Database |
| `*slog.Logger` | Logging |
| `*log.Logger` | Logging |
| `*http.Client` | Network |
| `*redis.Client` | Cache |
| `io.Writer` | I/O |
| `io.Reader` | I/O |

Detection is heuristic — it matches these 9 known dependency patterns. Custom wrappers (e.g., `type AppDB struct{ *sql.DB }`) and other third-party types (e.g., `*pgxpool.Pool`) are **not caught** by this layer. Use Layer 4 (test helper) for stricter coverage.

### Layer 3: Serialization Boundary

Each new session gets a deep copy of state via JSON marshal/unmarshal in `cloneStateTyped()`. This catches non-serializable fields that pass Layer 2:

- Functions and closures
- Channels
- Unexported fields (not visible to `encoding/json`)
- Circular references

If state contains any of these, the JSON round-trip fails at runtime when the first session is created.

### Layer 4: Test Helper

`AssertPureState[T](t)` runs the same validation as Layer 2 but fails the test instead of panicking:

```go
func TestState(t *testing.T) {
    livetemplate.AssertPureState[TodoState](t)
}
```

Add this to every state type's test file. It catches dependency leakage in CI before the code reaches production.

### What Happens on Violation

| Violation | When Detected | Outcome |
|-----------|---------------|---------|
| Dependency type in state struct | `AsState[T]()` at handler registration | **Panic** with field name and type |
| Non-serializable field (func, chan) | First session clone at runtime | JSON marshal error |
| Dependency in test | `AssertPureState[T](t)` in test suite | Test failure with field name and type |

---

## Session Isolation

Session isolation ensures that user A's state is never visible to user B.

### GroupID as Isolation Boundary

Every request — HTTP and WebSocket — goes through the `Authenticator` to compute a `groupID`:

```
Authenticator.Identify(r)           → userID
Authenticator.GetSessionGroup(r, userID) → groupID
```

The `groupID` is the authorization boundary for all state access. Users cannot specify a `groupID` directly in the URL or headers — the `Authenticator` computes it from the request's identity (cookies, auth headers).

The built-in `AnonymousAuthenticator` generates a random 256-bit `groupID` per browser (base64-encoded, stored in a `livetemplate-id` cookie). The `BasicAuthenticator` maps `groupID = userID` for authenticated users.

### Per-Session State Cloning

Each session group gets an independent state clone via JSON serialization/deserialization. No shared pointers exist across sessions. Each WebSocket connection also gets its own template clone (`Template.Clone()`) for independent diff state, preventing one tab's updates from corrupting another tab's tree comparison.

### SessionStore Keying

State persistence is keyed by `groupID`:

- **MemorySessionStore**: Go map keyed by `groupID`
- **RedisSessionStore**: Redis hash at `livetemplate:session:{groupID}`

No API exists to access another group's state. State is deserialized fresh on each `Get()`, preventing reference sharing across requests.

### Broadcast Scoping

Both `SharedState` auto-broadcast and explicit `BroadcastAction()` are scoped to the sender's `groupID`. The connection registry filters recipients via `GetByGroup(groupID)` — messages only reach connections in the same group. Different groups are never informed of each other's updates.

### HTTP Request Isolation

In the HTTP path (non-WebSocket), each session group has a per-group `httpTemplateCacheEntry` with its own mutex. Each POST request clones state, processes the action, and persists the result back to the group's SessionStore entry. The mutex serializes concurrent requests for the same group, preventing data races.

### Summary

| Component | Isolation Key | Cross-User Leakage |
|-----------|---------------|-------------------|
| SessionStore | `groupID` | Not possible (keyed by groupID) |
| ConnectionRegistry | `groupID` (dual-indexed) | Not possible (lookup filtered) |
| Template | Per-connection clone | Not possible (independent instances) |
| State | Per-group clone via JSON | Not possible (deep copy) |
| HTTP cache | Per-group entry + mutex | Not possible (separate entries) |
| Broadcast | `groupID` in registry | Not possible (filtered by group) |

---

## Limitations

- **Dependency detection is heuristic**: Only catches 9 known dependency patterns (stdlib types like `*sql.DB` plus common third-party types like `*redis.Client`). Custom wrappers and other third-party types require `AssertPureState[T]()` in tests.
- **Warning — Session isolation depends on Authenticator**: A custom `Authenticator` that returns the same `groupID` for different users would break isolation. Use the built-in authenticators or ensure `GetSessionGroup` maps distinct users to distinct groups.
- **JSON serialization overhead**: State cloning involves a JSON round-trip per session. Keep state structs small for best performance.

See [Current Limitations](current-limitations.md) for the full limitations reference.

---

## See Also

- [Controller+State Pattern](controller-pattern.md) — Full pattern reference with lifecycle methods and examples
- [Current Limitations](current-limitations.md) — All known limitations and workarounds
- [Session Reference](session.md) — Session stores and connection management
- [Authentication Reference](authentication.md) — Authenticator interface and session grouping
