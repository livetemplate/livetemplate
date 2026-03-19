# LiveTemplate - Development Guidelines

## Project Overview

LiveTemplate is a high-performance Go library for building reactive web applications. It provides an API similar to `html/template` but with the additional capability of generating minimal, tree-based updates that can be efficiently transmitted to clients.

**Related repositories:**
- **CLI Tool (lvt)**: Code generator and development server — maintained at `github.com/livetemplate/lvt`
- **TypeScript Client**: Browser-side update handler — maintained at `github.com/livetemplate/client`

## Version 0.7.0 - Controller+State Pattern

**Breaking Change Notice:** v0.7.0 introduces the Controller+State pattern, separating dependencies from session data.

### Why This Change?

The previous `cloneStore()` approach copied ALL exported fields, causing:
- **Security issues**: Session-specific data (OAuth tokens, caches) accidentally shared across users
- **Architectural ambiguity**: No clear contract for what gets cloned vs shared
- **Developer footguns**: Easy to accidentally put dependencies in cloned state

### New Pattern

```go
// CONTROLLER: Singleton, holds dependencies, NEVER cloned
type TodoController struct {
    DB     *sql.DB
    Logger *slog.Logger
}

// STATE: Pure data, cloned per session, serializable
type TodoState struct {
    Items  []Todo
    Filter string
}

// Action methods receive state and return modified state
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    todo := c.DB.InsertTodo(ctx.GetString("title"))
    state.Items = append(state.Items, todo)
    return state, nil
}

// Mount handler with explicit separation
handler := tmpl.Handle(controller, livetemplate.AsState(&TodoState{}))
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Controller** | Singleton holding dependencies (DB, Logger, API clients). Never cloned. |
| **State** | Pure data struct cloned per session. Must be serializable (no pointers to dependencies). |
| **AsState[T]()** | Generic wrapper that marks a struct as session state. Handles serialization automatically. |
| **Context** | Unified context for all lifecycle and action methods. Replaces ActionContext. |

### Lifecycle Methods

```go
// Called once when session is created
func (c *Controller) Mount(state State, ctx *Context) (State, error)

// Called on each WebSocket connect (optional)
func (c *Controller) OnConnect(state State, ctx *Context) (State, error)

// Called on disconnect (optional)
func (c *Controller) OnDisconnect()
```

### Migration from Old API

| Old Pattern | New Pattern |
|-------------|-------------|
| `type Store struct { DB *sql.DB; Items []Todo }` | Separate into Controller (DB) and State (Items) |
| `func (s *Store) Action(ctx *ActionContext) error` | `func (c *Controller) Action(state State, ctx *Context) (State, error)` |
| `tmpl.Handle(&Store{})` | `tmpl.Handle(&Controller{}, AsState(&State{}))` |
| `ctx.Action` | `ctx.Action()` |
| `ctx.Data` | `ctx.GetString()`, `ctx.GetInt()`, `ctx.BindAndValidate()` |

### Testing Helper

Use `AssertPureState[T]()` in tests to catch common mistakes:

```go
func TestState(t *testing.T) {
    // Fails if State contains dependency types (DB, Logger, etc.)
    livetemplate.AssertPureState[TodoState](t)
}
```

### Performance Considerations

- **State serialization**: Every session clone involves JSON marshal/unmarshal. Keep state small.
- **Method caching**: Reflection-based dispatch caches method lookups per type.
- **First invocation**: Slightly slower due to cache population; subsequent calls are fast.

---

## 5-Phase Architecture (Current)

The library is organized into 5 operational phases: **Parse → Build → Diff → Render → Send**

Each phase has its own internal package with clear responsibilities:
- `internal/parse/` - Template parsing into tree structures
- `internal/build/` - Tree construction, fingerprinting, operations
- `internal/diff/` - Tree comparison and update generation
- `internal/render/` - HTML rendering utilities
- `internal/send/` - Message formatting and serialization

Additional supporting packages:
- `internal/keys/` - Key generation for range items (`Generator` type)
- `internal/session/` - Connection registry and async WebSocket handling
- `internal/observe/` - Logging and metrics

## Core Architecture

### Main Package Files (Public API)

The main `livetemplate` package provides a clean, minimal public API:

1. **Template Engine (`template.go`)**:
   - Main entry point providing `html/template` compatible API
   - Manages template parsing, execution, and update generation
   - Handles wrapper ID injection for update targeting
   - Orchestrates internal packages for parsing, building, and diffing

2. **Mount Handler (`mount.go`)**:
   - LiveHandler interface for HTTP/WebSocket handling
   - Broadcaster and BroadcastAware interfaces for server-initiated updates
   - WebSocket connection lifecycle management
   - Auto-broadcasting to session groups
   - Dynamic pub/sub subscription via `DynamicSubscriber` type assertion during WebSocket setup

3. **Session Stores (`session_stores.go`)**:
   - SessionStore interface for session group management
   - MemorySessionStore for single-instance deployments
   - RedisSessionStore for distributed deployments
   - Automatic cleanup and TTL management

4. **Health Checks (`health.go`)**:
   - HealthHandler for liveness and readiness probes
   - HealthChecker interface for custom health checks
   - Built-in session store and Redis health checkers
   - Kubernetes-ready health endpoints

5. **Types (`types.go`)**:
   - TreeNode, RangeData, TreeMetadata type re-exports
   - Clean public API for tree-based operations
   - Backward-compatible type aliases

6. **Context (`context.go`)**:
   - Unified Context for all lifecycle and action methods
   - ActionData for form/JSON data handling
   - FieldError and MultiError for validation
   - Methods: `Action()`, `UserID()`, `GetString()`, `GetInt()`, `BindAndValidate()`

7. **Authentication (`auth.go`)**:
   - Authenticator interface for user identification
   - DefaultAuthenticator with cookie-based sessions

8. **Configuration (`config.go`)**:
   - TemplateConfig for template customization
   - DevMode, CompressHTML, and other options

### Internal Packages (5-Phase Architecture)

**Phase 1: Parse** (`internal/parse/`)
- Parses Go templates into tree structures using a custom AST evaluator
- Handles template constructs (fields, conditionals, ranges, with, template invokes)
- Components: api.go, walker.go, eval.go, field.go, conditional.go, range.go, with.go, vars.go, keys.go, helpers.go, errors.go, flatten.go, types.go
- Single unified AST walker with optional variable context

**Phase 2: Build** (`internal/build/`)
- Tree construction and operations
- Components: fingerprint.go, html_diff.go, html_segmentation.go, types.go, wrapper.go
- Handles tree creation, HTML diffing, segmentation, and change detection
- Wrapper div injection and HTML content extraction

**Phase 3: Diff** (`internal/diff/`)
- Tree comparison and update generation
- Components: tree_compare.go, range_ops.go, prepare.go, helpers_value.go, helpers_compare.go, helpers_keys.go, helpers_range.go, types.go
- Architecture: Hierarchical delegation pattern
  - Orchestrator: `CompareTreesAndGetChangesWithPath()` - entry point
  - Delegators: `handle*()` functions for specialized cases
  - Coordinator: `GenerateRangeDifferentialOperations()` for range ops

**Phase 4: Render** (`internal/render/`)
- HTML rendering utilities
- Components: html.go, minify.go
- Functions: `Node()`, `TreeToHTML()`, `IsVoidElement()`
- Converts tree structures to HTML strings; includes HTML minification

**Phase 5: Send** (`internal/send/`)
- Message formatting and serialization
- Components: json.go, message.go, response.go
- Action message parsing (HTTP/WebSocket)
- Update response wrapping, JSON serialization, and custom JSON encoding
- Functions: `ParseActionFromHTTP()`, `PrepareUpdate()`, `SerializeUpdate()`

**Supporting Packages:**

9. **Key Generation (`internal/keys/`)**:
   - Sequential key generation for range items
   - Components: generator.go, loader.go
   - Type: `Generator` (renamed from KeyGenerator)
   - Thread-safe counter with overflow protection; loader for key persistence

10. **Observability (`internal/observe/`)**:
    - Operational metrics with Prometheus export
    - Components: metrics.go, prometheus.go
    - Exposed via public `handler.MetricsHandler()` API

11. **Session Management (`internal/session/`)**: ⚡ **NEW: Async WebSocket Architecture**
    - **Async Sending Infrastructure**: Channel-based message queuing with background writePump goroutines
    - **Connection Types**: Connection, ConnectionRegistry, ConnectionLimits
    - **WebSocket Concurrency**: Each connection has dedicated writePump goroutine for async sends
    - **Dual Indexing**: By groupID (session groups) and userID (multi-device)
    - **Graceful Shutdown**: 5-second drain timeout with sync.Once protection
    - **Performance**: 165M concurrent sends/sec, 54.7M queued sends/sec
    - **Memory**: ~980 bytes per connection (measured)
    - **Backpressure**: Closes slow clients when buffer full (fail-fast)
    - **Thread-safety**: Lock-free sends, mutex-protected registry operations

    **Async Send Flow:**
    ```
    1. Send(messageType, data) → Queue message to buffered channel
    2. writePump goroutine → Dequeue and write to WebSocket
    3. Close() → Signal done → Wait for pump exit → Close WebSocket
    ```

    **Configuration:**
    - Buffer size: `LVT_WS_BUFFER_SIZE` env var (default: 50)
    - Config option: `WithWebSocketBufferSize(int)`
    - Metrics: `wsBufferFull`, `wsSlowClientCloses`, `wsWriteErrors`, `wsSendBufferSize`

12. **Execution Context (`internal/context/`)**:
    - TemplateContext for error handling and dev mode
    - Template execution utilities
    - Error propagation to client

13. **Compatibility (`internal/compat/`)**:
    - Compatibility layer for tree structures
    - Components: tree.go

14. **Template Discovery (`internal/discovery/`)**:
    - Automatic template file discovery
    - Components: discovery.go

15. **File Uploads (`internal/upload/`)**:
    - File upload handling infrastructure
    - Components: accessor.go, multipart.go, protocol.go, registry.go, tempfile.go, validate.go

16. **Upload Types (`internal/uploadtypes/`)**:
    - Upload type definitions shared across packages
    - Components: types.go

17. **Test Utilities (`internal/testutil/`)**:
    - Shared test helpers (e.g., Redis test setup)
    - Components: redis.go

18. **String Utilities (`internal/util/`)**:
    - General-purpose string utilities
    - Components: strings.go

19. **Fuzz Testing (`internal/fuzz/`)**:
    - Fuzz testing infrastructure for tree invariant validation
    - Subpackages: app/ (mutations, templates, types), generators/ (state, template), invariants/ (verifier, helpers, TS oracle), mutations/ (apply, types)

## Key Data Structures

### TreeNode
```go
type TreeNode struct {
    Statics     []string                // Static HTML parts (key: "s")
    Dynamics    map[string]interface{}   // Dynamic content (keys: "0", "1", etc.)
    Fingerprint string                   // Structure hash (key: "f")
    Range       *RangeData              // Range operation data (key: "d")
    Metadata    *TreeMetadata           // Additional metadata (key: "m")
}
```
- Core structure for representing static/dynamic content
- Custom JSON marshaling maintains wire format compatibility (numeric keys for dynamics)
- Can be nested for complex templates

### Template
```go
type Template struct {
    name           string
    templateStr    string
    tmpl           *template.Template
    wrapperID      string
    funcs          template.FuncMap
    mu             sync.RWMutex
    lastData       interface{}
    lastHTML       string
    lastTree       *treeNode
    initialTree    *treeNode
    hasInitialTree bool
    keyGen         *keyGenerator
    config         Config
    uploadRegistry interface{}
}
```

### Key Constructs
- `FieldConstruct`: Simple field replacement `{{.Field}}`
- `ConditionalConstruct`: If/else branches `{{if .Cond}}...{{else}}...{{end}}`
- `RangeConstruct`: Iteration `{{range .Items}}...{{end}}`
- `WithConstruct`: Context switching `{{with .Item}}...{{end}}`
- `TemplateInvokeConstruct`: Template invocation `{{template "name" .}}`

## Testing Strategy

### Test Files Structure
- `template_test.go`: Core template functionality tests (includes key injection tests)
- `tree_test.go`: Tree structure invariant validation
- `e2e_update_spec_test.go`: Tree update specification compliance tests
- Internal package tests: `internal/*/`

**Browser-based E2E Tests:**
Browser-based chromedp E2E tests are maintained in the lvt repository:
- Location: `github.com/livetemplate/lvt/e2e/livetemplate_core_test.go`
- These tests validate the library from a black-box perspective using real browser automation
- Tests include: complete rendering sequences, loading indicators, focus preservation, etc.

### Test Data
- `testdata/fixtures/`: Template fixtures for unit tests
- `testdata/golden/`: Golden files for snapshot testing
- `testdata/fuzz/`: Fuzz test corpus

### Running Tests
```bash
# Run all tests
go test -v ./...

# Run specific test categories
go test -run TestTemplate -v          # Template engine tests
go test -run TestTreeInvariant -v     # Tree invariant tests
go test -run TestKeyInjection -v      # Key injection tests
go test -run TestE2EUpdateSpec -v     # Update spec compliance tests

# Run with timeout
go test -v ./... -timeout=30s
```

## Development Conventions

### Release Process
- **Never create git tags manually** - Always use `release.sh` script for releases

### Code Style
1. **No unnecessary comments** - Code should be self-documenting
2. **Follow existing patterns** - Check neighboring code for conventions
3. **Use existing utilities** - Don't reinvent the wheel
4. **Maintain idiomatic Go** - Follow Go best practices

### Template Processing Flow
1. **Parse**: Template string → Compiled template structure
2. **Execute**: First render generates initial tree with statics and dynamics
3. **Update**: Subsequent renders generate minimal update trees
4. **Diff**: Compare trees to produce update operations

### Key Generation Strategy
- Uses wrapper-based approach with sequential key generation
- Keys are stable within a single render
- Supports any data type without special handling
- Keys reset between renders for consistency

### Best Practices: `data-key` Attributes in Range Templates

When rendering lists with `{{range}}`, the diff engine needs stable keys to track items across renders. By default, LiveTemplate auto-generates hash-based keys from item content. Explicit `data-key` attributes give you control:

```html
{{range .Items}}
<div data-key="{{.ID}}">{{.Name}}</div>
{{end}}
```

**When to use explicit `data-key`:**
- Items have a natural unique identifier (database ID, UUID)
- List items are reordered frequently (drag-and-drop, sorting)
- Items may have identical content but different identity

**When auto-generated keys suffice:**
- Small static lists (<100 items) with no reordering
- Items are always appended/removed, never reordered
- Content is unique across all items

**Key stability matters for performance:** Stable keys enable the diff engine to emit minimal `["u", key, changes]` operations instead of removing and re-inserting items. Unstable keys (e.g., using array index) cause unnecessary DOM thrashing on the client.

## Important Implementation Details

### Wire Format Optimization
The library implements a critical optimization per the tree-update-specification.md:
- **First Render**: Tree includes complete static structure (`"s"` arrays) + all dynamics
- **Updates**: Tree includes ONLY changed dynamics, NO statics (client has them cached)

This is implemented by `PrepareTreeForClient(node, clientHasStatics)` which:
1. Takes a fully-built tree (WITH statics, needed for comparison consistency)
2. Returns a wire-format tree (WITHOUT statics if client has cached them)
3. Implements spec requirement: "Updates MUST include ONLY changed dynamics"

**Key Architectural Points**:
- Trees are ALWAYS generated WITH statics (ensures consistent comparison)
- Comparison logic (`compareTreesAndGetChanges`) determines what changed
- `PrepareTreeForClient` removes statics before wire transmission
- This is NOT a "reactive fix" - it's the correct implementation of specification
- Result: Updates are ~10% the size of full renders (statics are largest part)

### Fingerprint-Based Structure Comparison

The system uses FNV-1a structure fingerprints to decide whether statics need to be resent. This replaced an earlier per-path `ClientStructureRegistry` approach (removed in [PR #86](https://github.com/livetemplate/livetemplate/pull/86)) that was more complex and harder to debug.

**What gets fingerprinted** (`internal/build/fingerprint.go`):
- Statics arrays (the HTML template parts between dynamic slots)
- Dynamic key positions (e.g., "there's a dynamic at position 0, 1, 2")
- Nested TreeNode structure (recursively)
- Range statics (item template structure)
- NOT dynamic values — two trees with identical structure but different content produce the same fingerprint

**Decision flow** (`internal/diff/tree_compare.go`):
```go
func ClientNeedsStatics(oldTree, newTree *TreeNode) bool {
    // First render: no previous tree → must send statics
    if oldTree == nil { return true }
    // Removal: new tree is gone → no statics needed
    if newTree == nil { return false }
    // Compare structure fingerprints for non-nil trees
    oldFP := oldTree.GetStructureFingerprint()
    newFP := newTree.GetStructureFingerprint()
    // Same fingerprint → client has statics cached → send dynamics only
    // Different fingerprint → structure changed → send full tree with statics
    return oldFP != newFP
}
```

The lazy-cached fingerprint enables O(1) structure comparison, avoiding re-computation on subsequent comparisons.

**Key functions**:
- `CalculateStructureFingerprint(tree)` — Computes FNV-1a 128-bit truncated to 64 bits (16 hex chars) of static structure (`internal/build/fingerprint.go`)
- `TreeNode.GetStructureFingerprint()` — Lazy-computes and caches fingerprint on first access (`internal/build/types.go`)
- `ClientNeedsStatics(oldTree, newTree)` — Returns true if fingerprints differ (`internal/diff/tree_compare.go`)
- `PrepareTreeForClient(tree, clientHasStatics)` — Strips statics from wire format when cached (`internal/diff/prepare.go`)

### Wrapper ID Injection
- All templates get a wrapper div with unique ID (`lvt-[random]`)
- Full HTML documents: Wrapper injected around body content
- Fragments: Entire content wrapped
- Used for targeting updates on client side

### Tree Update Format
```json
{
  "s": ["<div>", "</div>"],     // Static parts (cached client-side)
  "0": "Dynamic content",        // Dynamic value at position 0
  "1": {                         // Nested tree for complex structures
    "s": ["<span>", "</span>"],
    "0": "Nested dynamic"
  }
}
```

After first render, client caches the `"s"` arrays. Subsequent updates omit them:
```json
{
  "0": "Updated content"         // Only changed dynamic, no statics
}
```

### Range Operations
For list updates, special operations are used:
- `["u", "item-id", updates]`: Update existing item
- `["i", "after-id", "position", data]`: Insert new item
- `["r", "item-id"]`: Remove item
- `["o", ["id1", "id2", ...]]`: Reorder items

## Pre-commit Hook
The repository has a pre-commit hook that:
1. Auto-formats Go code using `go fmt`
2. Runs all tests with 30-second timeout
3. Blocks commits if tests fail
4. Automatically stages formatted files

## Common Tasks

### Adding New Template Construct
1. Add handling logic in the appropriate `internal/parse/` file (e.g., `field.go`, `conditional.go`, `range.go`, `with.go`)
2. For new construct types, create a new file in `internal/parse/` following existing patterns
3. Add parser logic in `internal/parse/parse.go`
4. Update type definitions in `internal/parse/types.go` if needed
5. Write tests in appropriate test files
6. Ensure backward compatibility if modifying existing constructs

### Debugging Tree Generation
1. Use `TreeNode.GetStructureFingerprint()` to track structural changes
2. Check `lastTree` vs current tree in Template
3. Validate tree structure with `validateTreeStructure()`
4. Use golden files for regression testing

### Updating Client Library
The TypeScript client is maintained in a separate repository at `github.com/livetemplate/client` (locally `../client`).
1. Make changes in the client repository
2. Ensure compatibility with tree format
3. Test with browser test suite
4. Update TypeScript types if needed

## Performance Considerations

1. **Tree Diffing**: O(n) complexity for most operations
2. **Memory**: Trees are kept in memory for diffing
3. **Fingerprinting**: FNV-1a hashing for change detection
4. **Key Generation**: Sequential integers for minimal overhead

## Security Notes

1. **HTML Escaping**: Uses `html/template` for automatic escaping
2. **No Direct HTML**: All content goes through template engine
3. **Wrapper IDs**: Random generation prevents conflicts

## Troubleshooting

### Test Failures
- Check golden files in `testdata/golden/` and `testdata/fixtures/`
- Verify tree structure matches expected format
- Ensure key generation is consistent
- Check for HTML escaping issues
- For browser E2E test failures, see lvt repository

### Tree Generation Issues
- Validate template syntax
- Check construct parsing order
- Verify hydration logic matches compilation
- Test with simpler templates first

### Async WebSocket Issues

**Connection Closes Unexpectedly:**
- Check metrics for `wsSlowClientCloses` counter
- Client may be too slow (buffer full)
- Increase buffer size with `LVT_WS_BUFFER_SIZE` or `WithWebSocketBufferSize()`
- Default buffer: 50 messages

**Goroutine Leaks:**
- Ensure all connections are unregistered via `registry.Unregister(conn)`
- `Unregister()` calls `Close()` which signals writePump to exit
- Check `pumpExited` channel closes (5-second timeout)
- Run tests with `go test -race` to detect race conditions

**High Memory Usage:**
- Each connection: ~980 bytes base + (buffer size × avg message size)
- Default 50-buffer: ~1KB per connection + message overhead
- Reduce buffer size for memory-constrained environments
- Monitor `wsSendBufferSize` gauge metric

**Messages Not Delivered:**
- Check `wsWriteErrors` metric for write failures
- Verify client WebSocket is connected and reading
- Check for `ErrConnectionClosed` or `ErrClientTooSlow` errors
- Review server logs for "WebSocket write failed" warnings

**Performance Tuning:**
- Benchmark results: 165M concurrent sends/sec, 54.7M queued sends/sec
- For high-throughput: Increase buffer size (100-1000)
- For low-latency: Decrease buffer size (10-20)
- Monitor `wsBufferFull` metric to detect backpressure
- Use Prometheus metrics at `/metrics` endpoint for observability

### PubSub / Cross-Instance Broadcasting Issues

**Cross-instance messages not received:**
- Verify `WithPubSubBroadcaster(broadcaster)` is configured
- Check Redis connectivity; all instances must use the same Redis
- Subscriptions are created automatically during WebSocket setup via `DynamicSubscriber` type assertion

**Subscriptions lost after Redis reconnection:**
- Should auto-recover: `reconnect()` replays all tracked channels from `subscribedChannels` map
- Check logs for `"Reconnected successfully"` with `dynamic_channels` count
- If `dynamic_channels=0`, no dynamic subscriptions were active

**See also:** [PubSub Reference](docs/references/pubsub.md)

## Future Improvements
- Consider adding more sophisticated diffing algorithms
- Optimize memory usage for large trees
- Add metrics and profiling hooks
- Enhance client-side caching strategies

