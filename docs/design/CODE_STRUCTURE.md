# Code Structure

This document provides a comprehensive map of the LiveTemplate codebase, explaining what each file does and how they fit together.

## Table of Contents

- [Project Overview](#project-overview)
- [Core Library Files](#core-library-files)
- [Internal Packages](#internal-packages)
- [Top-Level Packages](#top-level-packages)
- [Test Files](#test-files)
- [File Dependencies](#file-dependencies)
- [Entry Points](#entry-points)

## Project Overview

```
livetemplate/
├── *.go                    # Core library (15 files)
├── internal/
│   ├── build/              # Tree types, fingerprinting, wrapper injection
│   ├── compat/             # Backward compatibility wrappers
│   ├── context/            # Template execution context
│   ├── diff/               # Tree comparison and update generation
│   ├── discovery/          # Template file auto-discovery
│   ├── fuzz/               # Fuzz testing framework
│   ├── keys/               # Range item key generation
│   ├── observe/            # Metrics and Prometheus export
│   ├── parse/              # Template parsing into tree structures
│   ├── render/             # HTML rendering and minification
│   ├── send/               # Message parsing and serialization
│   ├── session/            # WebSocket connection registry
│   ├── testutil/           # Test utilities (Redis helpers)
│   ├── upload/             # File upload infrastructure
│   ├── uploadtypes/        # Upload type definitions
│   └── util/               # String utility functions
├── pubsub/                 # Redis pub/sub broadcasting
├── testdata/               # Test fixtures, golden files, fuzz corpus
└── docs/                   # Documentation
```

## Core Library Files

### Public API Layer

#### template.go (1,655 lines)
**Purpose:** Main API entry point and orchestrator for the library

**Key Types:**
- `Template` - Core template management type

**Key Functions:**
- `New(name string, opts ...Option) *Template` - Create new template
- `Execute(wr io.Writer, data interface{}) error` - Full HTML render
- `ExecuteUpdates(wr io.Writer, data interface{}) error` - Tree updates
- `Handle(controller, state, ...options) LiveHandler` - WebSocket/HTTP handler (embeds `http.Handler`, adds `Shutdown(ctx)` and `MetricsHandler()`)
- `ParseFiles(filenames ...string) (*Template, error)` - Parse templates

**Dependencies:**
- internal/parse/ (template parsing)
- internal/build/ (tree building)
- internal/diff/ (tree comparison)
- internal/observe/ (observability)
- mount.go (HTTP handlers)
- session_stores.go (session management)

**Used By:** All user applications

---

#### mount.go (2,024 lines)
**Purpose:** HTTP/WebSocket handlers, Controller+State pattern, and connection lifecycle

**Key Functions:**
- `Handle(controller, AsState(state), ...options) http.Handler` - Create handler with controller and state
- HTTP handlers (handleHTTPRequest, handleAction)
- WebSocket handlers (handleWebSocket, message loops)

**Key Features:**
- Session management (per-connection state)
- State cloning via `AsState()` (isolation between sessions)
- Error handling (validation errors, panics)
- Broadcasting support
- Controller lifecycle methods (Mount, OnConnect, OnDisconnect)
- Graceful shutdown with connection draining

**Dependencies:**
- template.go (Template type)
- context.go (Context type)
- state.go (State interface, AsState wrapper)
- session_stores.go (SessionStore)
- internal/session/ (ConnectionRegistry)

**Used By:** User applications (via Template.Handle())

---

#### session_stores.go (905 lines)
**Purpose:** Session store abstraction for single-instance and distributed deployments

**Key Types:**
- `SessionStore` interface - Session storage abstraction
- `MemorySessionStore` - In-memory implementation for single-instance
- `RedisSessionStore` - Redis-backed implementation for multi-instance

**Key Functions:**
- `NewMemorySessionStore() *MemorySessionStore`
- `NewRedisSessionStore(client *redis.Client, ...options) *RedisSessionStore`

**Dependencies:** `github.com/redis/go-redis/v9`

**Used By:** mount.go

---

#### state.go (451 lines)
**Purpose:** State interface and generic session state management

**Key Types:**
- `StateWrapper` interface - State cloning and serialization contract
- `AsState[T]()` - Generic wrapper that marks a struct as session state

**Key Functions:**
- `AsState[T any](initial *T) *TypedState[T]` - Create typed state wrapper

**Used By:** mount.go, user applications

---

#### action.go (329 lines)
**Purpose:** Action protocol and data binding

**Key Types:**
- `ActionData` - Type-safe data extraction
- `FieldError` - Validation error
- `MultiError` - Collection of field errors

**Key Functions:**
- `Bind(v interface{}) error` - Unmarshal to struct
- `BindAndValidate(v, validator) error` - Bind + validate
- `GetString/GetInt/GetFloat/GetBool(key)` - Type-safe getters

**Dependencies:** None (self-contained)

**Used By:** context.go, mount.go, user applications

---

#### config.go (312 lines)
**Purpose:** Template and handler configuration via options pattern

**Key Types:**
- `TemplateConfig` - Template customization options

**Key Functions:**
- Configuration options: `WithDevMode()`, `WithCompressHTML()`, `WithMaxConnections()`, etc.
- Environment variable loading (`LVT_*` prefix)

**Used By:** template.go, mount.go

---

#### health.go (303 lines)
**Purpose:** Health check endpoints for Kubernetes probes

**Key Types:**
- `HealthHandler` - HTTP handler for health endpoints
- `HealthChecker` interface - Custom dependency health checks

**Key Functions:**
- `NewHealthHandler(checkers ...HealthChecker) *HealthHandler`
- `Live(w, r)` - Liveness probe (`/health/live`)
- `Ready(w, r)` - Readiness probe (`/health/ready`)

**Used By:** User applications (Kubernetes deployments)

---

#### context.go (278 lines)
**Purpose:** Unified context for lifecycle and action methods

**Key Types:**
- `Context` - Unified context for all lifecycle and action methods

**Key Functions:**
- `Action() string` - Get current action name
- `UserID() string` - Get authenticated user ID
- `GetString(key) string` - Type-safe string extraction
- `GetInt(key) int` - Type-safe int extraction
- `BindAndValidate(v, validator) error` - Bind + validate

**Used By:** mount.go, user controller methods

---

#### dispatch.go (214 lines)
**Purpose:** Reflection-based action method dispatch

**Key Functions:**
- Method lookup and caching per controller type
- Dispatch actions to controller methods by name

**Used By:** mount.go

---

#### auth.go (199 lines)
**Purpose:** Authentication interface and default implementation

**Key Types:**
- `Authenticator` interface - User identification contract
- `DefaultAuthenticator` - Cookie-based session authentication

**Used By:** mount.go

---

#### lifecycle.go (147 lines)
**Purpose:** Controller lifecycle method detection and invocation

**Key Functions:**
- Detection and invocation of `Mount()`, `OnConnect()`, `OnDisconnect()` methods

**Used By:** mount.go

---

#### s3_presigner.go (121 lines)
**Purpose:** S3 presigned URL generation for file uploads

**Used By:** upload.go

---

#### formvalidation.go (196 lines)
**Purpose:** Form schema extraction and validation

**Key Types:**
- `FormRule` - Single validation rule from HTML
- `FormSchema` - Collection of form rules

**Key Functions:**
- `ExtractFormSchema()` - Extract rules from template statics

**Used By:** mount.go

---

#### ws.go (120 lines)
**Purpose:** WebSocket interface abstraction

**Key Types/Interfaces:**
- `WSConn` - WebSocket connection (ReadMessage, WriteMessage, Close)
- `WSUpgrader` - Upgrade HTTP to WebSocket
- `WSCloseError` - WebSocket close error
- Constants: `WSTextMessage`, `WSBinaryMessage`, close codes

**Key Functions:**
- `WSFormatCloseMessage()`, `WSIsUnexpectedCloseError()`, `WSIsUpgrade()`

**Used By:** mount.go

---

#### ws_gorilla.go (120 lines)
**Purpose:** Gorilla WebSocket implementation of WSUpgrader/WSConn interfaces

**Key Types:**
- `GorillaUpgrader` - gorilla/websocket implementation
- `GorillaOption` - Configuration option

**Used By:** template.go (default upgrader)

---

#### testing.go (87 lines)
**Purpose:** Test helpers for library users

**Key Functions:**
- `AssertPureState[T](t)` - Verify state struct contains no dependency types

**Used By:** User test files

---

#### upload.go (82 lines)
**Purpose:** File upload public API

**Used By:** mount.go, user applications

---

#### upload_init.go (22 lines)
**Purpose:** Upload subsystem initialization

**Used By:** mount.go

---

## Internal Packages

### Phase 1: Parse — `internal/parse/` (~2,200 lines)

**Purpose:** Parse Go templates into tree structures with separated statics and dynamics

**Files:**
| File | Lines | Purpose |
|------|-------|---------|
| `api.go` | 45 | Public API: `Parse()` and `BuildTree()` |
| `eval.go` | 825 | Custom AST evaluator (walks PipeNode/CommandNode via reflection) |
| `walker.go` | 201 | Unified AST walker with optional variable context |
| `range.go` | 171 | `{{range}}` handling with key generation |
| `keys.go` | 130 | Key detection and content hashing for range items |
| `helpers.go` | 131 | Shared utilities (isZeroValue, formatPipe, getSortedKeys) |
| `vars.go` | 90 | Variable context (orderedVars, varContext) |
| `conditional.go` | 57 | `{{if}}{{else}}{{end}}` handling |
| `errors.go` | 42 | Structured ParseError type |
| `flatten.go` | 405 | Template composition (`{{template "name" .}}` inlining) |
| `with.go` | 30 | `{{with}}{{end}}` handling |
| `field.go` | 29 | `{{.Field}}` and action handling |
| `types.go` | 27 | Shared type definitions and KeyGenerator interface |

**Key Functions:**
- `Parse(templateStr string, funcMap template.FuncMap) (*Template, error)` - Parse template string
- `BuildTree(tmpl *Template, data interface{}, keyGen KeyGenerator, ctx *Context) (*TreeNode, error)` - Build tree from AST + data

**How It Works:**
1. Parse template using stdlib `html/template`
2. Walk AST with unified walker (`walkAST`) that handles variable context
3. Evaluate expressions directly via custom AST evaluator (no re-parsing)
4. Flatten template compositions (inline `{{template}}` calls)
5. Build tree with statics and dynamics separated

---

### Phase 2: Build — `internal/build/` (1,090 lines)

**Purpose:** Tree type definitions, fingerprinting, wrapper injection, and HTML utilities

**Files:**
| File | Lines | Purpose |
|------|-------|---------|
| `types.go` | 580 | `TreeNode` struct, `RangeData`, tree operations |
| `wrapper.go` | 263 | Wrapper div injection and extraction |
| `fingerprint.go` | 97 | FNV-1a structure fingerprinting |
| `html_segmentation.go` | 94 | HTML segmentation for statics extraction |
| `html_diff.go` | 56 | HTML-level diff utilities |
| `aria_inject.go` | 113 | Aria attribute injection for accessibility |

**Key Types:**
- `TreeNode` (struct) - Core tree structure with statics and dynamics
- `RangeData` - Range metadata (item keys, construct ID)
- `TreeMetadata` - Metadata annotations

**Key Functions:**
- `CalculateStructureFingerprint(tree *TreeNode) string` - FNV-1a hash of static structure
- `TreeNode.GetStructureFingerprint() string` - Cached fingerprint accessor

---

### Phase 3: Diff — `internal/diff/` (1,972 lines)

**Purpose:** Tree comparison and minimal update generation

**Files:**
| File | Lines | Purpose |
|------|-------|---------|
| `helpers_value.go` | 117 | Value inspection and range detection |
| `helpers_compare.go` | 68 | Deep comparison (DeepEqual, TreeNodeEqual) |
| `helpers_keys.go` | 162 | Key extraction, hashing, position detection |
| `helpers_range.go` | 348 | Range analysis (reordering, insertion patterns) |
| `range_ops.go` | 521 | Range differential operations (insert, remove, update, reorder) |
| `tree_compare.go` | 495 | Main comparison orchestrator |
| `prepare.go` | 86 | Wire format preparation (strip statics when cached) |
| `types.go` | 17 | Type aliases for compatibility |

**Key Functions:**
- `CompareTreesAndGetChangesWithPath(old, new, insideNewStructure, path, rangeMatches)` - Main comparison
- `ClientNeedsStatics(oldTree, newTree *build.TreeNode) bool` - Fingerprint-based comparison
- `GenerateRangeDifferentialOperations(oldValue, newValue interface{}, stripStatics bool) []interface{}` - Range diff
- `PrepareTreeForClient(node interface{}, clientHasStatics bool) interface{}` - Strip statics for wire transmission

**Architecture Pattern:** Orchestrator → Coordinator → Helper
- **Orchestrators** (~30 lines): High-level flow control
- **Coordinators** (20-50 lines): Coordinate one aspect (e.g., removals, insertions)
- **Helpers** (<20 lines): Pure functions with single responsibility

**Range Operations Generated:**
- `["u", "item-id", updates]` - Update existing item
- `["i", "after-id", "position", data]` - Insert new item
- `["r", "item-id"]` - Remove item
- `["o", ["id1", "id2", ...]]` - Reorder items

---

### Phase 4: Render — `internal/render/` (217 lines)

**Purpose:** HTML rendering and minification

**Files:**
| File | Lines | Purpose |
|------|-------|---------|
| `html.go` | 154 | Tree → HTML rendering, void element detection |
| `minify.go` | 63 | HTML whitespace minification |

**Key Functions:**
- `Node(w *strings.Builder, n *html.Node)` - Render HTML node to builder
- `TreeToHTML(tree map[string]interface{}) (string, error)` - Convert tree to HTML
- `IsVoidElement(tagName string) bool` - Check if element is self-closing
- `MinifyHTML(htmlContent string) string` - Remove unnecessary whitespace

---

### Phase 5: Send — `internal/send/` (270 lines)

**Purpose:** Action message parsing, update response wrapping, and JSON serialization

**Files:**
| File | Lines | Purpose |
|------|-------|---------|
| `message.go` | 272 | Parse actions from HTTP requests and WebSocket messages |
| `response.go` | 55 | Update response wrapping |
| `json.go` | 30 | Ordered JSON serialization |

**Key Functions:**
- `ParseActionFromHTTP(r *http.Request) (ActionMessage, error)` - Parse action from HTTP
- `PrepareUpdate(tree, errors, action) *UpdateResponse` - Wrap update for client
- `SerializeUpdate(resp *UpdateResponse) ([]byte, error)` - Serialize to JSON
- `MarshalOrderedJSON(tree interface{}) ([]byte, error)` - Deterministic JSON output

---

### Supporting Packages

#### `internal/keys/` (241 lines)
**Purpose:** Sequential key generation for range items

**Files:**
- `generator.go` (201) - Key generator with thread-safe counter
- `loader.go` (40) - Key loading utilities

#### `internal/context/` (507 lines)
**Purpose:** Template execution context and data utilities

**Files:**
- `context.go` (412) - TemplateContext for error handling and dev mode
- `data.go` (95) - Data extraction utilities

#### `internal/session/` (695 lines)
**Purpose:** WebSocket connection registry with async write pump

**Files:**
- `registry.go` (510) - Connection type, ConnectionRegistry with dual indexing
- `limits.go` (185) - ConnectionLimits for per-instance and per-group limits

**Key Types:**
- `Connection` - WebSocket connection with dedicated writePump goroutine
- `ConnectionRegistry` - Thread-safe registry indexed by groupID and userID
- `ConnectionLimits` - Global and per-group connection limits

#### `internal/observe/` (785 lines)
**Purpose:** Operational metrics and Prometheus export

**Files:**
- `metrics.go` (401) - Counters, gauges, and histograms for all operations
- `prometheus.go` (356) - PrometheusExporter for `/metrics` endpoint
- `doc.go` (28) - Package documentation

**Key Types:**
- `Metrics` - Operational metrics collector
- `PrometheusExporter` - Prometheus-compatible exporter

#### `internal/compat/` (142 lines)
**Purpose:** Backward compatibility wrappers for tree operations

#### `internal/discovery/` (118 lines)
**Purpose:** Template file auto-discovery from conventional directories

#### `internal/upload/` (1,037 lines)
**Purpose:** File upload infrastructure

**Files:**
- `registry.go` (244) - Upload registry for tracking uploads
- `protocol.go` (219) - Upload protocol handling
- `multipart.go` (197) - Multipart form parsing
- `tempfile.go` (189) - Temporary file management
- `validate.go` (109) - Upload validation rules
- `accessor.go` (79) - Upload accessor interface

#### `internal/uploadtypes/` (52 lines)
**Purpose:** Upload type definitions shared across packages

#### `internal/testutil/` (153 lines)
**Purpose:** Test utilities (Redis test helpers)

#### `internal/util/` (45 lines)
**Purpose:** String utility functions

**Key Functions:**
- `FindCommonPrefix(s1, s2 string) string`
- `FindCommonSuffix(s1, s2 string) string`

#### `internal/fuzz/` (6,599 lines)
**Purpose:** Fuzz testing framework for tree diff correctness validation

**Subpackages:**
- `app/` - Application state types and mutation generators (10 files)
- `generators/` - Random state and template generators (2 files)
- `invariants/` - Invariant verifiers: update minimality, key stability, tree structure, no data loss (3 files)
- `mutations/` - Mutation types and application logic (2 files)

---

## Top-Level Packages

### `pubsub/` (716 lines)
**Purpose:** Redis pub/sub broadcasting for distributed multi-instance deployments

**Files:**
- `redis.go` (592) - Redis-based pub/sub broadcaster with subscription tracking and reconnect replay
- `types.go` (124) - Message types, `Broadcaster` interface, and `DynamicSubscriber` optional interface

---

## Test Files

### Core Tests

| File | Lines | Purpose |
|------|-------|---------|
| `tree_test.go` | 3,659 | Tree generation and invariant tests |
| `template_test.go` | 3,383 | Core template functionality |
| `fuzz_diff_test.go` | 3,082 | Fuzz-based diff testing |
| `handle_test.go` | 1,195 | HTTP/WebSocket handler tests |
| `e2e_update_spec_test.go` | 953 | Tree update specification compliance |
| `state_test.go` | 748 | State cloning and serialization |
| `config_test.go` | 639 | Configuration option tests |
| `session_test.go` | 530 | Session store tests |
| `fuzz_ts_oracle_test.go` | 500 | TypeScript oracle validation |
| `upload_test.go` | 487 | File upload tests |
| `auth_test.go` | 344 | Authentication tests |
| `health_test.go` | 347 | Health check endpoint tests |
| `s3_presigner_test.go` | 331 | S3 presigner tests |
| `dispatch_test.go` | 229 | Action dispatch tests |
| `lifecycle_test.go` | 220 | Lifecycle method tests |
| `template_bench_test.go` | 215 | Template benchmarks |
| `component_templates_test.go` | 205 | Component template tests |
| `context_test.go` | 199 | Context tests |
| `e2e_bench_test.go` | 197 | E2E benchmarks |
| `shutdown_test.go` | 195 | Graceful shutdown tests |
| `metrics_test.go` | 123 | Metrics tests |
| `error_bench_test.go` | 81 | Error handling benchmarks |
| `testing_test.go` | 56 | Test helper tests |
| `tree_bench_test.go` | 42 | Tree operation benchmarks |

### Browser E2E Tests

Browser-based chromedp E2E tests are maintained in the lvt repository:
- Location: `github.com/livetemplate/lvt/e2e/livetemplate_core_test.go`
- Tests validate: complete rendering sequences, loading indicators, focus preservation, etc.

### Test Data

- `testdata/fixtures/` - Template fixtures for unit tests
- `testdata/golden/` - Golden files for snapshot testing
- `testdata/fuzz/` - Fuzz test corpus

---

## File Dependencies

### Dependency Graph

```
User Application
    ↓
template.go (Public API & Orchestrator)
    ↓
    ├→ internal/parse/ (Template parsing + composition)
    │    ↓
    │    └→ internal/build/ (Tree types)
    │
    ├→ internal/build/ (Fingerprinting, wrapper injection)
    │
    ├→ internal/diff/ (Tree comparison)
    │    ↓
    │    └→ internal/build/ (Tree types)
    │
    ├→ internal/observe/ (Metrics)
    │
    ├→ internal/discovery/ (Template file discovery)
    │
    ├→ action.go (Action data binding)
    │
    ├→ mount.go (HTTP/WebSocket handlers)
    │    ↓
    │    ├→ context.go (Context type)
    │    ├→ state.go (State interface)
    │    ├→ dispatch.go (Action dispatch)
    │    ├→ lifecycle.go (Lifecycle methods)
    │    ├→ auth.go (Authentication)
    │    ├→ session_stores.go (Session storage)
    │    ├→ internal/session/ (Connection registry)
    │    ├→ internal/send/ (Message parsing)
    │    └→ internal/observe/ (Logging)
    │
    └→ config.go (Configuration)
```

### Import Relationships

**Level 0 (No internal dependencies — Foundational):**
- action.go
- internal/build/ (tree types and operations)
- internal/keys/ (key generation)
- internal/util/ (string utilities)

**Level 1 (Depends on Level 0 — Internal Packages):**
- internal/parse/ (uses internal/build/)
- internal/diff/ (uses internal/build/)
- internal/render/ (uses internal/build/)
- internal/send/ (standalone)
- internal/observe/ (standalone)

**Level 2 (Depends on Level 1 — Public API Support):**
- context.go, state.go, dispatch.go, lifecycle.go, auth.go
- session_stores.go (uses internal/session/)
- config.go

**Level 3 (Depends on Level 2 — Public API):**
- template.go (uses internal/parse/, internal/build/, internal/diff/, internal/observe/, config.go)
- mount.go (uses context.go, state.go, dispatch.go, session_stores.go, internal/session/, internal/send/)

**Top Level:**
- User applications (use template.go, mount.go)

---

## Entry Points

### For Library Users

**Creating a Template:**
```go
tmpl := livetemplate.New("counter")  // template.go
```

**Defining Controller and State:**
```go
// Controller holds dependencies (singleton, never cloned)
type CounterController struct {
    DB *sql.DB  // Dependencies go here
}

// State holds data (cloned per session)
type CounterState struct {
    Count int
}
```

**Handling Requests:**
```go
controller := &CounterController{DB: db}
state := &CounterState{}
http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(state)))  // mount.go
```

**Implementing Action Methods:**
```go
// Action "increment" maps to method Increment()
func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Count++
    return state, nil
}
```

### For Contributors

**Adding New Template Features:**
1. Start in `internal/parse/` (define construct handling)
2. Add parser logic in `internal/parse/parse.go`
3. Add tests in `template_test.go` and `tree_test.go`
4. Update `internal/build/` if tree structure changes
5. Update `internal/diff/` if comparison logic changes

**Modifying Tree Structure:**
1. Start in `internal/build/types.go` (TreeNode definition)
2. Update `internal/parse/` (tree building)
3. Update `internal/diff/` (comparison logic)
4. Add tests in `tree_test.go`

**Improving Diff Algorithm:**
1. Start in `internal/diff/tree_compare.go` (orchestrator)
2. Add coordinator functions for specific scenarios
3. Add helper functions in the appropriate `internal/diff/helpers_*.go` file
4. Follow orchestrator → coordinator → helper pattern
5. Add tests in `e2e_update_spec_test.go`

**Adding HTTP/WebSocket Features:**
1. Start in `mount.go`
2. Update `internal/send/` if protocol changes
3. Add observability in `internal/observe/`
4. Add tests in `handle_test.go`

---

## Quick Reference

### Where to Find Things

| What | Where |
|------|-------|
| Public API | `template.go` |
| Controller+State pattern | `mount.go`, `state.go`, `context.go` |
| Action data binding | `action.go` |
| Template parsing | `internal/parse/` |
| Tree types and fingerprinting | `internal/build/` |
| Tree comparison | `internal/diff/` |
| HTML rendering | `internal/render/` |
| Message serialization | `internal/send/` |
| Observability and metrics | `internal/observe/` |
| HTTP/WebSocket handlers | `mount.go` |
| Session storage | `session_stores.go` |
| Connection registry | `internal/session/` |
| File uploads | `upload.go`, `internal/upload/` |
| Health checks | `health.go` |
| Configuration | `config.go` |
| Authentication | `auth.go` |
| Redis pub/sub | `pubsub/` |
| Fuzz testing | `internal/fuzz/` |
| CLI tool | Separate repo: `livetemplate/lvt` |
| TypeScript client | Separate repo: `livetemplate/client` |

### Component Size Summary

| Component | Lines | Purpose |
|-----------|-------|---------|
| `internal/fuzz/` | 6,599 | Fuzz testing framework |
| `internal/parse/` | 2,149 | Template parsing (AST) |
| `mount.go` | 2,024 | HTTP/WS handlers |
| `internal/diff/` | 1,972 | Tree comparison & updates |
| `template.go` | 1,655 | Main API (orchestrator) |
| `internal/build/` | 1,090 | Tree types & fingerprinting |
| `internal/upload/` | 1,037 | File upload infrastructure |
| `session_stores.go` | 905 | Session storage |
| `internal/observe/` | 785 | Observability |
| `internal/session/` | 695 | Connection registry |
| `pubsub/` | 675 | Redis pub/sub |
| `internal/context/` | 507 | Execution context |
| `state.go` | 451 | State management |
| `action.go` | 329 | Actions & data binding |
| `config.go` | 312 | Configuration |
| `health.go` | 303 | Health checks |
| `context.go` | 278 | Context type |
| `internal/send/` | 270 | Message serialization |
| `internal/keys/` | 241 | Key generation |
| `internal/render/` | 217 | HTML rendering |
| `dispatch.go` | 214 | Action dispatch |
| `auth.go` | 199 | Authentication |
| `internal/testutil/` | 153 | Test utilities |
| `lifecycle.go` | 147 | Lifecycle methods |
| `internal/compat/` | 142 | Backward compatibility |
| `s3_presigner.go` | 121 | S3 presigning |
| `internal/discovery/` | 118 | Template discovery |
| `testing.go` | 87 | Test helpers |
| `upload.go` | 82 | Upload API |
| `internal/uploadtypes/` | 52 | Upload types |
| `internal/util/` | 45 | String utilities |
| `upload_init.go` | 22 | Upload init |

**Total:** ~7,130 lines of root-level library code + ~16,070 lines in internal packages + 675 lines in pubsub

---

## Navigation Tips

**For New Users:**
1. **Start with `template.go`** — Understand the public API
2. **Then `mount.go`, `state.go`, `context.go`** — Learn the Controller+State pattern
3. **Then `action.go`** — Understand action data binding
4. **Check test files** — See comprehensive test scenarios

**For Contributors:**
1. **Read `ARCHITECTURE.md`** — Understand the 5-phase design
2. **Read `internal/parse/parse.go`** — See how templates are parsed
3. **Read `internal/build/types.go`** — See tree structure definition
4. **Read `internal/diff/tree_compare.go`** — See orchestrator → coordinator → helper pattern
5. **Check `CLAUDE.md`** — Development guidelines and conventions
6. **Run tests** — `go test -v ./...`

For architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md)

For contribution guide, see [CONTRIBUTING.md](../CONTRIBUTING.md)
