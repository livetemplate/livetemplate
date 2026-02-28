# Proposal: Persistence Bug Fix & Design Improvements

## 1. Critical Bug Fix: Session Persistence

### The Problem

Currently, `livetemplate` fails to persist state changes when using `RedisSessionStore` (or any external `SessionStore` implementation).

In `mount.go`, the request lifecycle is:

1.  **Load:** `SessionStore.Get(ctx, groupID)` retrieves the session state.
    - For `RedisSessionStore`, this deserializes a _copy_ of the data from Redis.
2.  **Modify:** `store.Change(ctx)` modifies this local copy in memory.
3.  **Render:** The modified state is used to render the template update.
4.  **End:** The request ends **without calling `SessionStore.Set`**.

**Consequences:**

- **Data Loss:** Since `RedisSessionStore` works with a copy, the modifications are discarded at the end of the request.
- **Sync Failure:** If a user opens a second tab, it fetches the _stale_ state from Redis, not the updated state from the first tab.
- **Restart Data Loss:** If the server restarts, all in-memory progress is lost because it was never written back to Redis.

### The Fix

We must ensure that `SessionStore.Set` is called whenever the state is successfully modified.

#### Implementation Plan

Modify `livetemplate/mount.go`.

1.  **Update `handleAction` signature:**
    It needs access to the `groupID` to save the session.

2.  **Add Persistence Logic:**
    After `store.Change(actionCtx)` returns `nil` (success), immediately save the state.

```go
// In mount.go

func (h *liveHandler) handleAction(ctx context.Context, msg message, state *connState, ...) error {
    // ... existing setup ...

    // 1. Execute Change
    err := store.Change(actionCtx)
    if err != nil {
        // ... handle error ...
        return nil // or err
    }

    // 2. PROPOSED FIX: Persist State
    // Only persist if no error occurred
    if h.config.SessionStore != nil {
        // Note: This saves ALL stores. See "Scalability" below for optimization.
        h.config.SessionStore.Set(ctx, state.groupID, state.stores)
    }

    return nil
}
```

_Note: This must be applied to both the WebSocket loop and the HTTP POST handler._

---

## 2. Design Improvements

### A. Scalability: Granular Persistence

**Problem:**
The current `SessionStore` interface treats the entire `Stores` map as a single blob.

- **Inefficient I/O:** Changing one integer in a `Counter` store requires serializing and writing the entire session (which might include a large `Todo` list) to Redis.
- **Race Conditions:** In a distributed environment, two concurrent requests modifying different parts of the state will overwrite each other's changes (Last-Write-Wins on the whole blob).

**Proposed Solution: Dirty Tracking & Granular Keys**

1.  **Dirty Tracking:**
    Modify `Store` to indicate if it has changed.

    ```go
    type Store interface {
        Change(ctx *ActionContext) error
    }
    // Optional interface
    type DirtyTracker interface {
        IsDirty() bool
        ClearDirty()
    }
    ```

    Only call `Set` for stores that are "dirty".

2.  **Redis Hash Structure:**
    Instead of storing the session as one key `livetemplate:session:{id}`, use a Redis Hash:

    - Key: `livetemplate:session:{id}`
    - Fields: `counter` -> `blob`, `todos` -> `blob`

    This allows updating just the `counter` store without overwriting the `todos` store.

### B. Maintainability: Method Dispatch (No More "God Methods")

**Problem:**
The `Change` method pattern encourages massive `switch` statements that violate the Open/Closed principle and are hard to read.

```go
// Current Pattern
func (s *Store) Change(ctx *ActionContext) error {
    switch ctx.Action {
    case "increment": ...
    case "decrement": ...
    case "reset": ...
    // ... 100 lines later ...
    }
}
```

**Proposed Solution: Reflection-based or Map-based Dispatch**

Introduce a `Dispatcher` helper that routes actions to methods.

```go
// Proposed Pattern
type CounterStore struct {
    Count int
}

// Methods match Action names: "increment" -> Increment
func (s *CounterStore) Increment(ctx *ActionContext) error {
    s.Count++
    return nil
}

func (s *CounterStore) Change(ctx *ActionContext) error {
    // Reusable dispatcher
    return livetemplate.Dispatch(s, ctx)
}
```

This breaks the "God Method" into small, testable methods.

### C. Code Legibility: Separation of Concerns (MVC)

**Problem:**
`Store` structs currently mix **Data** (state fields) with **Behavior** (logic). This makes it hard to serialize data (you serialize the logic too?) and hard to reason about the data shape.

**Proposed Solution: Split State and Controller**

Separate the data structure from the action handler.

```go
// 1. The Data (Model) - Pure data, easy to serialize
type CounterState struct {
    Count int `json:"count"`
}

// 2. The Logic (Controller)
type CounterController struct {
    State *CounterState
}

func (c *CounterController) Increment(ctx *ActionContext) error {
    c.State.Count++
    return nil
}
```

The `livetemplate` framework would hold the `Controller`, which holds the `State`. Persistence would only save the `State`.

### D. Type Safety: Typed Actions

**Problem:**
Actions are strings (`"counter.increment"`). Renaming an action requires a global find-and-replace and is error-prone.

**Proposed Solution:**
Generate constants or use a type-safe builder.

```go
// Generated or defined constants
const (
    ActionIncrement = "increment"
    ActionDecrement = "decrement"
)

// In Template
// <button lvt-click="{{.Actions.Increment}}">+</button>
```

This ensures compile-time safety for action names.
