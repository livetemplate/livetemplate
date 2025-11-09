# LiveTemplate Architecture

## Overview

LiveTemplate is a reactive web framework for Go that uses tree-based DOM diffing to send minimal updates to clients. This document explains the system architecture, design decisions, and operational flow.

**New contributor?** Start with the [Contributor Walkthrough](guides/new-contributor-walkthrough.md) for a hands-on introduction to the 5-phase architecture with code examples and test links.

## System Flow

```
┌────────────────────────────────────────────────────────────────┐
│                     User Action (Browser)                       │
└──────────────────────────┬─────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│  Phase 1: PARSE                                                 │
│  Package: internal/parse/                                       │
│  Input:   Template string                                       │
│  Output:  Parsed AST                                            │
│  Job:     Convert Go template syntax to executable form         │
└──────────────────────────┬─────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│  Phase 2: BUILD                                                 │
│  Package: internal/build/                                       │
│  Input:   Parsed AST + Data                                     │
│  Output:  Tree structure                                        │
│  Job:     Generate tree separating statics from dynamics        │
└──────────────────────────┬─────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│  Phase 3: DIFF                                                  │
│  Package: internal/diff/                                        │
│  Input:   Old tree + New tree                                   │
│  Output:  Minimal changes                                       │
│  Job:     Calculate what changed                                │
└──────────────────────────┬─────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│  Phase 4: RENDER                                                │
│  Package: internal/render/                                      │
│  Input:   Tree or Changes                                       │
│  Output:  HTML or JSON                                          │
│  Job:     Format for wire transmission                          │
└──────────────────────────┬─────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│  Phase 5: SEND                                                  │
│  Package: internal/send/                                        │
│  Input:   Rendered output                                       │
│  Output:  HTTP response / WebSocket message                     │
│  Job:     Deliver updates to client                             │
└────────────────────────────────────────────────────────────────┘
                           ↓
                    Client applies update
```

## Package Structure

### Operational Phases (Main Flow)

#### `internal/parse/`
**Responsibility:** Parse Go templates into executable AST

**Files:**
- `parse.go` - Main parsing logic
- `field.go` - `{{.Field}}` handling
- `conditional.go` - `{{if}}{{else}}{{end}}` handling
- `range.go` - `{{range}}{{end}}` handling
- `with.go` - `{{with}}{{end}}` handling
- `invoke.go` - `{{template}}` handling

**Key Functions:**
- `Parse(text string, funcMap template.FuncMap) (*Template, error)`

#### `internal/build/`
**Responsibility:** Build tree structures from AST and data

**Files:**
- `types.go` - TreeNode type definitions
- `node.go` - Tree node operations, fingerprinting
- `build.go` - Tree building from AST + data
- `walk.go` - Tree traversal utilities

**Key Types:**
- `TreeNode` - Tree structure with statics and dynamics
- `RangeData` - Range iteration data
- `TreeMetadata` - Tree metadata

**Key Functions:**
- `Build(tmpl *parse.Template, data interface{}, keyGen *keys.Generator) (*TreeNode, error)`

#### `internal/diff/`
**Responsibility:** Compute minimal differences between trees

**Files:**
- `diff.go` - Main diffing algorithm
- `range.go` - Range-specific diffing
- `optimize.go` - Diff optimizations

**Key Functions:**
- `Compute(old, new *build.TreeNode, logger *observe.Logger) *build.TreeNode`
- `ComputeRangeDiff(old, new *build.TreeNode) []RangeOperation`

#### `internal/render/`
**Responsibility:** Render trees and diffs to output formats

**Files:**
- `html.go` - Tree → HTML rendering
- `json.go` - Diff → JSON rendering
- `wrapper.go` - HTML wrapper injection

**Key Functions:**
- `HTML(tree *build.TreeNode, wrapperID string) (string, error)`
- `JSON(changes *build.TreeNode) ([]byte, error)`

#### `internal/send/`
**Responsibility:** HTTP/WebSocket communication with clients

**Files:**
- `handler.go` - Main HTTP handler
- `websocket.go` - WebSocket upgrade and handling
- `broadcast.go` - Broadcasting to multiple clients

**Key Types:**
- `Handler` - HTTP handler implementing ServeHTTP
- `Connection` - WebSocket connection wrapper

### Supporting Packages

#### `internal/keys/`
**Responsibility:** Generate stable keys for range items

**Key Functions:**
- `New() *Generator`
- `(*Generator).Next() string`

#### `internal/context/`
**Responsibility:** Action context and data extraction

**Key Types:**
- `Context` - Action context with data access methods

**Key Functions:**
- `(*Context).String(key string) string`
- `(*Context).Int(key string) int`
- `(*Context).Bind(v interface{}) error`

#### `internal/session/`
**Responsibility:** Session management

**Key Types:**
- `Store` interface
- `Manager` - Session manager

#### `internal/observe/`
**Responsibility:** Structured logging and metrics (slog-based)

**Key Types:**
- `Logger` - Structured logger with domain methods
- `Metrics` - Operational metrics tracker

**Key Functions:**
- `NewLogger(level slog.Level, handler slog.Handler) *Logger`
- `NewMetrics(logger *slog.Logger) *Metrics`

#### `internal/util/`
**Responsibility:** Generic utility functions

**Key Functions:**
- `Map[T, U](slice []T, fn func(T) U) []U`
- `Filter[T](slice []T, fn func(T) bool) []T`
- `Keys[K, V](m map[K]V) []K`

## Design Decisions

### Why Tree-Based Updates?

**Problem:** Sending full HTML on every update wastes bandwidth

**Solution:** Separate static HTML (structure) from dynamic data (values)

**Result:**
- First render: Send complete tree with statics `["<div>", "</div>"]` + dynamics
- Updates: Send only changed dynamics, client has statics cached
- Bandwidth reduction: 50-90% for typical templates

**Example:**
```json
// First render
{
  "s": ["<div>Counter: ", "</div>"],
  "0": "5"
}

// Update (client has statics)
{
  "0": "6"
}
```

### Why HTTP-First (WebSocket Optional)?

**Decision:** All features work over HTTP, WebSocket only for broadcasts

**Rationale:**
- ✅ HTTP works everywhere (no proxy/firewall issues)
- ✅ Simpler deployment and debugging
- ✅ Stateless until you need state
- ✅ WebSocket adds complexity, only needed for server-initiated updates

**When to use WebSocket:**
- Real-time collaboration
- Live notifications
- Server-initiated broadcasts
- Multiple simultaneous updates

### Why Server-Side State?

**Decision:** State lives in Go, not split between client/server

**Benefits:**
- ✅ Type safety via Go's type system
- ✅ No synchronization issues
- ✅ Simpler mental model
- ✅ Backend logic stays in backend
- ✅ No need for API layer

**Trade-offs:**
- ❌ Not suitable for offline-first apps
- ❌ Requires server round-trip for interactions
- ✅ But: Perfect for admin dashboards, internal tools, forms

### Why Operational Phase Packages?

**Decision:** Packages named `parse`, `build`, `diff`, `render`, `send`

**Benefits:**
- ✅ Self-documenting code structure
- ✅ Import list shows execution flow
- ✅ New developers can follow the pipeline
- ✅ Clear separation of concerns
- ✅ Easy to find code (operation → package)

**Example:**
```go
import (
    "github.com/livetemplate/livetemplate/internal/parse"   // Phase 1
    "github.com/livetemplate/livetemplate/internal/build"   // Phase 2
    "github.com/livetemplate/livetemplate/internal/diff"    // Phase 3
    "github.com/livetemplate/livetemplate/internal/render"  // Phase 4
    "github.com/livetemplate/livetemplate/internal/send"    // Phase 5
)
```

## Code Composition Patterns

### Orchestrator → Coordinator → Helper Pattern

Large operations are broken into three levels:

**1. Orchestrator (25-35 lines)**
- Public API entry point
- Calls coordinators in sequence
- Handles errors and logging
- Example: `diff.Compute()`

**2. Coordinators (20-30 lines)**
- Handle one aspect of the operation
- Call multiple helpers
- Example: `computeUpdates()`, `computeInserts()`

**3. Helpers (<15 lines)**
- Do ONE thing well
- Pure functions when possible
- Example: `extractItemKey()`, `haveSameKeys()`

### Example: Range Diffing

```go
// Orchestrator (25 lines)
func ComputeRangeDiff(old, new *build.Tree) []RangeOperation {
    oldItems, oldKeys := extractRangeItems(old)
    newItems, newKeys := extractRangeItems(new)

    if isPureReorder(oldItems, newItems, oldKeys, newKeys) {
        return []RangeOperation{createReorderOp(newKeys)}
    }

    ops := []RangeOperation{}
    ops = append(ops, computeUpdates(oldItems, newItems, oldKeys, newKeys)...)
    ops = append(ops, computeInserts(oldItems, newItems, oldKeys, newKeys)...)
    ops = append(ops, computeRemoves(oldItems, newItems, oldKeys, newKeys)...)
    return ops
}

// Coordinator (20-30 lines each)
func computeUpdates(oldItems, newItems []interface{}, oldKeys, newKeys []string) []RangeOperation
func computeInserts(oldItems, newItems []interface{}, oldKeys, newKeys []string) []RangeOperation
func computeRemoves(oldItems, newItems []interface{}, oldKeys, newKeys []string) []RangeOperation

// Helpers (<15 lines each)
func extractRangeItems(tree *build.Tree) ([]interface{}, []string)
func extractItemKey(item interface{}) string
func isPureReorder(oldItems, newItems []interface{}, oldKeys, newKeys []string) bool
func haveSameKeys(keys1, keys2 []string) bool
```

**Benefits:**
- Easy to understand (each function fits on screen)
- Easy to test (each function independently testable)
- Easy to modify (change one aspect without affecting others)
- Self-documenting (function names explain intent)

## Performance Characteristics

### Template Parsing
- **Frequency:** Once per template (cached)
- **Complexity:** O(n) where n = template size
- **Optimization:** Parsed templates are reused

### Tree Building
- **Frequency:** Every render
- **Complexity:** O(n) where n = data size
- **Optimization:** Incremental fingerprinting

### Tree Diffing
- **Frequency:** Every update (not first render)
- **Complexity:** O(n) where n = tree node count
- **Optimization:** Early exit on no changes, range operation detection

### Rendering
- **Frequency:** Every render/update
- **Complexity:** O(n) where n = output size
- **Optimization:** Pre-allocated buffers

## Wire Format Optimization

### First Render
Client receives complete tree with statics:
```json
{
  "s": ["<div class='card'>", "<span>", "</span>", "</div>"],
  "0": "Title",
  "1": "Description"
}
```

Client caches statics by structure signature.

### Subsequent Updates
Client receives only changed dynamics:
```json
{
  "1": "Updated description"
}
```

Client uses cached statics + new dynamics to rebuild DOM.

**Bandwidth savings:** ~90% for typical updates (statics are the largest part)

## Security Considerations

### HTML Escaping
- All dynamic content goes through `html/template` for automatic escaping
- No direct HTML injection possible
- User content is always escaped

### WebSocket Authentication
- Configurable `Authenticator` interface
- Anonymous mode for development
- User-based session isolation for production

### Session Management
- Sessions isolated by user ID
- Session groups for multi-tab support
- Automatic cleanup on disconnect

## Observability

### Structured Logging (slog)
All logs are structured JSON:
```json
{
  "time": "2025-10-31T12:34:56Z",
  "level": "INFO",
  "msg": "template_executed",
  "template": "todos",
  "data_type": "*main.TodoState",
  "duration_ms": 5,
  "user_id": "user-123",
  "session_id": "sess-456"
}
```

### Metrics
Emitted periodically via slog:
- Counters: actions_processed, templates_executed, etc.
- Gauges: active_connections, active_groups
- Histograms: p50/p95/p99 latencies

See [OBSERVABILITY.md](OBSERVABILITY.md) for complete guide.

## Future Optimizations

### Planned
- [ ] Streaming updates for large datasets
- [ ] Client-side caching improvements
- [ ] Binary diff format (more compact than JSON)
- [ ] Diff batching (combine multiple small updates)

### Under Consideration
- [ ] Multi-user broadcast optimization
- [ ] Partial tree invalidation
- [ ] Template compilation cache
- [ ] WebSocket message compression

## Related Documentation

- [OBSERVABILITY.md](OBSERVABILITY.md) - Logging and metrics guide
- [MIGRATION.md](MIGRATION.md) - Alpha → v1.0 migration guide
- [CODE_STRUCTURE.md](CODE_STRUCTURE.md) - Codebase organization
- [API Reference](https://pkg.go.dev/github.com/livetemplate/livetemplate) - Go package docs
