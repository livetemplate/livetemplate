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
│  Output:  HTML (for testing and validation)                     │
│  Job:     HTML rendering and minification                       │
└──────────────────────────┬─────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│  Phase 5: SEND                                                  │
│  Package: internal/send/                                        │
│  Input:   Tree updates + action data                            │
│  Output:  Serialized JSON messages                              │
│  Job:     Message parsing and serialization                     │
└────────────────────────────────────────────────────────────────┘
                           ↓
                    Client applies update
```

## Package Structure

### Operational Phases (Main Flow)

#### `internal/parse/` — 8 files, 2,149 lines
**Responsibility:** Parse Go templates into executable AST

**Files:**
- `parse.go` (558) - Main parser entry point, `Parse()` and `BuildTree()`
- `field.go` (167) - `{{.Field}}` handling
- `conditional.go` (262) - `{{if}}{{else}}{{end}}` handling
- `range.go` (513) - `{{range}}{{end}}` handling
- `with.go` (35) - `{{with}}{{end}}` handling
- `flatten.go` (405) - Template composition (`{{template "name" .}}` inlining)
- `types.go` (27) - Shared type definitions
- `var_context.go` (182) - Variable context for template scoping

**Key Functions:**
- `Parse(templateStr string, funcMap template.FuncMap) (*Template, error)`
- `BuildTree(tmpl *Template, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error)`

#### `internal/build/` — 5 files, 1,090 lines
**Responsibility:** Tree types, fingerprinting, wrapper injection, and HTML diffing

**Files:**
- `types.go` (580) - `TreeNode` struct, `RangeData`, `TreeMetadata`, tree operations
- `wrapper.go` (263) - Wrapper div injection and HTML content extraction
- `fingerprint.go` (97) - Structure fingerprinting for diff optimization
- `html_diff.go` (56) - HTML-level diff utilities
- `html_segmentation.go` (94) - HTML segmentation for statics extraction

**Key Types:**
- `TreeNode` (struct) - Tree structure with statics and dynamics
- `RangeData` - Range metadata (item keys, construct ID)
- `TreeMetadata` - Metadata annotations for trees

**Key Functions:**
- `CalculateStructureFingerprint(tree *TreeNode) string` - MD5 hash of static structure
- `TreeNode.GetStructureFingerprint() string` - Cached structure fingerprint accessor

#### `internal/diff/` — 5 files, 1,972 lines
**Responsibility:** Compute minimal differences between trees

**Files:**
- `tree_compare.go` (495) - Main tree comparison algorithm
- `range_ops.go` (521) - Range-specific diffing operations
- `helpers.go` (853) - Utility functions for diff operations
- `prepare.go` (86) - Wire format preparation (strip statics when cached)
- `types.go` (17) - Type aliases for compatibility

**Key Functions:**
- `CompareTreesAndGetChangesWithPath(old, new *build.TreeNode, ...) *build.TreeNode`
- `ClientNeedsStatics(oldTree, newTree *build.TreeNode) bool` - Fingerprint-based comparison
- `GenerateRangeDifferentialOperations(oldValue, newValue interface{}, stripStatics bool) []interface{}`
- `PrepareTreeForClient(node interface{}, clientHasStatics bool) interface{}`

#### `internal/render/` — 2 files, 217 lines
**Responsibility:** HTML rendering and minification

**Files:**
- `html.go` (154) - Tree → HTML rendering, void element detection
- `minify.go` (63) - HTML minification

**Key Functions:**
- `Node(w *strings.Builder, n *html.Node)` - Render HTML node to builder
- `TreeToHTML(tree map[string]interface{}) (string, error)` - Convert tree to HTML string
- `IsVoidElement(tagName string) bool` - Check if element is self-closing
- `MinifyHTML(htmlContent string) string` - Remove unnecessary whitespace

#### `internal/send/` — 3 files, 270 lines
**Responsibility:** Action message parsing, update serialization, and JSON encoding

**Files:**
- `message.go` (185) - Action message parsing from HTTP/WebSocket
- `response.go` (55) - Update response wrapping
- `json.go` (30) - Ordered JSON serialization

**Key Functions:**
- `ParseActionFromHTTP(r *http.Request) (ActionMessage, error)` - Parse action from HTTP request
- `PrepareUpdate(tree interface{}, errors map[string]string, action string) *UpdateResponse`
- `SerializeUpdate(resp *UpdateResponse) ([]byte, error)` - Serialize update to JSON
- `MarshalOrderedJSON(tree interface{}) ([]byte, error)` - Deterministic JSON output

### Supporting Packages

#### `internal/keys/` — 2 files, 241 lines
**Responsibility:** Generate stable keys for range items

**Key Functions:**
- `New() *Generator`
- `(*Generator).Next() string`

#### `internal/context/` — 2 files, 507 lines
**Responsibility:** Template execution context and data utilities

**Files:**
- `context.go` (412) - TemplateContext for error handling and dev mode
- `data.go` (95) - Data extraction utilities

#### `internal/session/` — 2 files, 695 lines
**Responsibility:** WebSocket connection registry and connection limits

**Files:**
- `registry.go` (510) - Connection type, ConnectionRegistry with dual indexing (groupID + userID)
- `limits.go` (185) - ConnectionLimits for per-instance and per-group limits

**Key Types:**
- `Connection` - WebSocket connection with async write pump
- `ConnectionRegistry` - Thread-safe registry with dual indexing
- `ConnectionLimits` - Global and per-group connection limits

#### `internal/observe/` — 3 files, 785 lines
**Responsibility:** Operational metrics and Prometheus export

**Files:**
- `metrics.go` (401) - Metrics struct with counters, gauges, and histograms
- `prometheus.go` (356) - PrometheusExporter for `/metrics` endpoint
- `doc.go` (28) - Package documentation

**Key Types:**
- `Metrics` - Operational metrics collector
- `PrometheusExporter` - Prometheus-compatible metrics exporter

**Key Functions:**
- `NewMetrics(logger *slog.Logger) *Metrics`
- `NewPrometheusExporter(metrics *Metrics, limitsGetter LimitsGetter) *PrometheusExporter`

#### `internal/util/` — 1 file, 45 lines
**Responsibility:** String utility functions

**Key Functions:**
- `FindCommonPrefix(s1, s2 string) string`
- `FindCommonSuffix(s1, s2 string) string`

#### `internal/compat/` — 1 file, 142 lines
**Responsibility:** Backward compatibility wrappers for tree operations

#### `internal/discovery/` — 1 file, 118 lines
**Responsibility:** Template file auto-discovery from conventional directories

#### `internal/upload/` — 6 files, 1,037 lines
**Responsibility:** File upload infrastructure (multipart parsing, temp files, validation)

**Files:**
- `registry.go` (244) - Upload registry for tracking uploads
- `protocol.go` (219) - Upload protocol handling
- `multipart.go` (197) - Multipart form parsing
- `tempfile.go` (189) - Temporary file management
- `validate.go` (109) - Upload validation rules
- `accessor.go` (79) - Upload accessor interface

#### `internal/uploadtypes/` — 1 file, 52 lines
**Responsibility:** Upload type definitions shared across packages

#### `internal/testutil/` — 1 file, 153 lines
**Responsibility:** Test utilities (Redis test helpers)

#### `internal/fuzz/` — 4 subpackages, 18 files, 6,599 lines
**Responsibility:** Fuzz testing framework for tree diff validation

**Subpackages:**
- `app/` - Application state types and mutation generators
- `generators/` - Random state and template generators
- `invariants/` - Invariant verifiers (minimality, key stability, structure, data loss)
- `mutations/` - Mutation types and application logic

### Top-Level Package

#### `pubsub/` — 2 files, 675 lines
**Responsibility:** Redis pub/sub broadcasting for distributed deployments

**Files:**
- `redis.go` (561) - Redis-based pub/sub broadcaster
- `types.go` (114) - Pub/sub message types and interfaces

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
- **Optimization:** Structure fingerprinting for O(1) statics decision

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

Client caches statics for reuse in subsequent updates.

### Subsequent Updates
Server uses **fingerprint-based comparison** to determine if client needs statics:

```go
// Simple 4-case logic (inspired by Phoenix LiveView)
func ClientNeedsStatics(oldTree, newTree *TreeNode) bool {
    if oldTree == nil { return true }   // First render
    if newTree == nil { return false }  // Removal
    return CalculateStructureFingerprint(oldTree) != CalculateStructureFingerprint(newTree)
}
```

When structure is unchanged, client receives only changed dynamics:
```json
{
  "1": "Updated description"
}
```

Client uses cached statics + new dynamics to rebuild DOM.

**Bandwidth savings:** ~90% for typical updates (statics are the largest part)

### Fingerprint-Based Optimization
The fingerprint approach (v0.8.0+) replaces the previous registry-based tracking:
- **O(1) comparison** instead of path-based registry lookups
- **Simpler logic**: 4 cases vs 49+ state transitions
- **No registry state**: Server compares fingerprints directly
- **"When in doubt, send full tree"**: Safe fallback for edge cases

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
- [MIGRATION.md](implementation-plans/MIGRATION.md) - Alpha → v1.0 migration guide
- [CODE_STRUCTURE.md](CODE_STRUCTURE.md) - Codebase organization
- [API Reference](https://pkg.go.dev/github.com/livetemplate/livetemplate) - Go package docs
