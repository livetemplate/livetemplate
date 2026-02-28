# Proposal: Store Pattern Redesign (v0.5)

**Status:** Draft
**Date:** 2025-12-01
**Target Version:** v0.5.0

## Summary

This proposal outlines a major redesign of the `Store` pattern in LiveTemplate to address critical scalability bottlenecks, maintainability issues, and code legibility concerns. The changes focus on three key areas:

1.  **Scalability:** Moving from coarse-grained "blob" persistence to granular, per-store persistence with dirty tracking.
2.  **Maintainability:** Replacing the monolithic `Change()` "God Method" with automated method dispatch.
3.  **Legibility:** Adopting a clear MVC-like separation between Data (State) and Logic (Controller).

---

## 1. Scalability: Granular Persistence

### The Problem

Currently, `SessionStore` treats the entire session state (a map of all Stores) as a single binary blob.

- **Serialization Overhead:** Modifying a single integer in one store requires serializing the entire session (which may contain large lists or complex objects) using `gob`.
- **Network I/O:** The entire session blob is sent to Redis on every write, and retrieved on every read.
- **Concurrency Conflicts:** In a distributed environment, two concurrent requests modifying different stores (e.g., `Counter` vs `Chat`) will overwrite each other because they both write the full session blob (Last-Write-Wins).

### Proposed Solution

#### 1.1 Dirty Tracking Interface

Introduce an optional interface for Stores to indicate modification.

```go
type DirtyTracker interface {
    // IsDirty returns true if the store's state has changed since load/last save
    IsDirty() bool
    // ClearDirty resets the dirty flag
    ClearDirty()
}
```

The framework will wrap stores or provide a base helper to manage this flag automatically when actions are dispatched.

#### 1.2 Granular Redis Storage

Change the Redis storage schema from a simple Key-Value pair to a Redis Hash.

- **Old Schema:**

  - Key: `livetemplate:session:{groupID}`
  - Value: `GobEncoded(map[string]Store)`

- **New Schema:**
  - Key: `livetemplate:session:{groupID}`
  - Type: `Hash`
  - Fields:
    - `_meta`: Session metadata (created_at, etc.)
    - `counter`: `GobEncoded(CounterState)`
    - `todos`: `GobEncoded(TodoState)`

#### 1.3 Optimized Save Flow

The `SessionStore.Set` method will be updated to support partial updates.

```go
// Pseudo-code for new Save logic
func (s *RedisSessionStore) Save(ctx context.Context, groupID string, stores Stores) error {
    pipe := s.client.Pipeline()

    for name, store := range stores {
        // Only save if dirty (or if it's a new session)
        if tracker, ok := store.(DirtyTracker); !ok || tracker.IsDirty() {
            data, _ := gob.Encode(store)
            pipe.HSet(ctx, "session:"+groupID, name, data)

            if tracker != nil {
                tracker.ClearDirty()
            }
        }
    }

    _, err := pipe.Exec(ctx)
    return err
}
```

**Benefits:**

- **O(1) Writes:** Bandwidth is proportional to the _change_, not the total session size.
- **Conflict Reduction:** Concurrent updates to _different_ stores will no longer conflict in Redis (HSET on different fields is safe).

---

## 2. Maintainability: Method Dispatch

### The Problem

The current `Change(ctx)` method forces developers to write massive `switch` statements. This violates the Open/Closed Principle and leads to "God Methods" that are hard to read and test.

```go
// Current Anti-Pattern
func (s *Store) Change(ctx *ActionContext) error {
    switch ctx.Action {
    case "increment":
        // logic...
    case "decrement":
        // logic...
    case "addItem":
        // logic...
    // ... 500 lines later ...
    }
    return nil
}
```

### Proposed Solution

Introduce a reflection-based dispatcher that routes actions to methods based on naming conventions.

#### 2.1 The Dispatcher

A helper function `livetemplate.Dispatch` will map action strings to methods.

- Action: `"increment"` -> Method: `Increment(ctx *ActionContext) error`
- Action: `"add_item"` -> Method: `AddItem(ctx *ActionContext) error`

#### 2.2 Developer Experience

Developers simply define methods matching their actions.

```go
type CounterStore struct {
    Count int
}

// Action: "increment"
func (c *CounterStore) Increment(ctx *ActionContext) error {
    c.Count++
    return nil
}

// Boilerplate is reduced to one line
func (c *CounterStore) Change(ctx *ActionContext) error {
    return livetemplate.Dispatch(c, ctx)
}
```

**Performance Note:** The Dispatcher will cache method lookups (Type -> Action -> MethodIndex) to ensure zero reflection overhead on the hot path.

---

## 3. Legibility: MVC / Separation of Concerns

### The Problem

`Store` structs currently mix **Data** (state fields) with **Behavior** (logic).

- **Serialization Issues:** When saving a Store to Redis, we only want the data, but the struct often contains private fields, mutexes, or service dependencies that shouldn't be serialized.
- **Testing Difficulty:** It's hard to test logic in isolation without setting up the full persistence machinery.

### Proposed Solution

Formalize the separation of **State** (Data) and **Controller** (Logic).

#### 3.1 The Pattern

```go
// 1. State (The Model)
// Pure data. Easy to serialize. No dependencies.
type CounterState struct {
    Count int `json:"count"`
}

// 2. Controller (The Logic)
// Contains dependencies (db, logger) and operates on State.
type CounterController struct {
    State *CounterState
    DB    *sql.DB      // Not serialized
    Log   *slog.Logger // Not serialized
}

// Methods are defined on the Controller
func (c *CounterController) Increment(ctx *ActionContext) error {
    c.State.Count++
    c.Log.Info("Counter incremented")
    return nil
}
```

#### 3.2 Framework Support

The framework will recognize this pattern. When registering a store, the user provides the Controller.

- **Persistence:** The framework detects the `State` field and only serializes that object to Redis.
- **Dependency Injection:** This structure makes it natural to inject dependencies into the Controller during initialization, which persist across requests (in memory), while the State is swapped out/reloaded from Redis.

### 3.3 Orthogonality & Synergy

It is important to note that **Method Dispatch** (Section 2) and **MVC Separation** (Section 3) are orthogonal concepts:

1.  **Dispatch without MVC:** You can use `Dispatch()` on a simple struct to avoid switch statements.
2.  **MVC without Dispatch:** You can use the Controller/State pattern but still write a manual `Change()` switch statement if you prefer explicit control flow.
3.  **Combined (Recommended):** Using both provides the cleanest architecture: Controllers manage logic and dependencies, State manages data, and Dispatch eliminates boilerplate.

---

## Migration Strategy

To ensure backward compatibility, these changes will be introduced non-destructively:

1.  **Phase 1 (Opt-in):**

    - Add `Dispatch()` helper.
    - Add `DirtyTracker` interface (optional).
    - Existing `Change()` methods continue to work.
    - Redis storage remains blob-based by default.

2.  **Phase 2 (Hybrid Storage):**

    - Update `RedisSessionStore` to support reading both Blobs and Hashes.
    - New sessions use Hashes.
    - Old sessions are lazily migrated to Hashes on next write.

3.  **Phase 3 (Deprecation):**
    - Mark manual `switch` in `Change()` as deprecated in documentation.
    - Encourage MVC pattern for new features.

## Conclusion

This redesign transforms LiveTemplate from a simple prototyping tool into a production-grade framework capable of handling complex, state-heavy applications with efficiency and clean architecture.
