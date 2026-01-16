# Code Structure

This document provides a comprehensive map of the LiveTemplate codebase, explaining what each file does and how they fit together.

## Table of Contents

- [Project Overview](#project-overview)
- [Core Library Files](#core-library-files)
- [Supporting Files](#supporting-files)
- [Test Files](#test-files)
- [CLI Tool](#cli-tool)
- [Client Library](#client-library)
- [File Dependencies](#file-dependencies)
- [Entry Points](#entry-points)

## Project Overview

```
livetemplate/
├── *.go                    # Core library (21 files)
├── client/                 # TypeScript client library
│   ├── livetemplate-client.ts
│   └── livetemplate-client.test.ts
├── cmd/lvt/                # CLI tool for code generation
│   ├── main.go
│   ├── commands/
│   └── internal/
├── examples/               # Example applications
│   ├── counter/
│   └── todos/
├── testdata/               # Test fixtures and golden files
├── docs/                   # Documentation
└── scripts/                # Development scripts
```

## Core Library Files

### Public API Layer

#### template.go (~1,400 lines)
**Purpose:** Main API entry point and orchestrator for the library

**Key Types:**
- `Template` - Core template management type
- `Config` - Template configuration options
- `UpdateResponse` - Wrapper for tree updates
- `ResponseMetadata` - Action metadata

**Key Functions:**
- `New(name string, opts ...Option) *Template` - Create new template
- `Execute(wr io.Writer, data interface{}) error` - Full HTML render
- `ExecuteUpdates(wr io.Writer, data interface{}) error` - Tree updates
- `Handle(store Store) http.Handler` - WebSocket/HTTP handler
- `ParseFiles(filenames ...string) (*Template, error)` - Parse templates

**Dependencies:**
- tree.go (tree operations)
- internal/parse/ (template parsing)
- internal/build/ (tree building)
- internal/diff/ (tree comparison)
- internal/observe/ (observability)
- mount.go (HTTP handlers)
- session.go (session management)

**Used By:** All user applications

**Architecture Note:** v1.0 refactored template.go from 2,500 to 1,400 lines by extracting parsing, building, and diffing logic into internal packages while maintaining 100% backward compatibility.

---

#### action.go (~270 lines)
**Purpose:** Action protocol and data binding

**Key Types:**
- `ActionData` - Type-safe data extraction
- `FieldError` - Validation error
- `MultiError` - Collection of field errors

**Key Functions:**
- `Bind(v interface{}) error` - Unmarshal to struct
- `BindAndValidate(v, validator) error` - Bind + validate
- `GetString/GetInt/GetFloat/GetBool(key)` - Type-safe getters
- `ValidationToMultiError(err) MultiError` - Convert validator errors

**Internal Functions:**
- `parseAction(action string) (controller, actualAction)` - Parse "controller.action"
- `parseActionFromHTTP(r *http.Request) (message, error)` - HTTP parser
- `parseActionFromWebSocket(data []byte) (message, error)` - WS parser
- `writeUpdateWebSocket(conn, update) error` - WS writer

**Dependencies:** None (self-contained)

**Used By:** template.go, mount.go, user applications

---

#### mount.go (~500 lines)
**Purpose:** HTTP/WebSocket handlers and Controller+State pattern

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

**Dependencies:**
- template.go (Template type)
- context.go (Context type)
- state.go (State interface, AsState wrapper)
- session.go (SessionStore)

**Used By:** User applications (via Template.Handle())

---

#### session.go (~150 lines)
**Purpose:** Session management for HTTP requests

**Key Types:**
- `SessionStore` interface - Session storage abstraction
- `MemorySessionStore` - In-memory implementation

**Key Functions:**
- `NewMemorySessionStore() SessionStore` - Create memory store
- `GetSession(r *http.Request) (Session, error)` - Get session
- `SaveSession(w, r, session) error` - Save session

**Dependencies:** None

**Used By:** mount.go

---

### Template Processing Layer

#### tree.go (~400 lines)
**Purpose:** Tree operations and key generation

**Key Types (Private):**
- `treeNode` - map[string]interface{} representing tree structure
- `keyGenerator` - Sequential key generation
- `keyAttributeConfig` - Configuration for key attributes

**Key Functions:**
- `parseTemplateToTree(templateStr, data, keyGen) (treeNode, error)` - Entry point
- `calculateFingerprint(tree) string` - MD5 hash for change detection
- `newKeyGenerator() *keyGenerator` - Create key generator
- `generateRandomID() string` - Random wrapper ID generation
- `injectWrapperDiv(html, wrapperID, loadingDisabled) string` - Wrapper injection
- `extractTemplateContent(input, wrapperID) string` - Extract wrapped content
- `normalizeTemplateSpacing(templateStr) string` - Normalize {{}} spacing

**Tree Format:**
```go
treeNode{
    "s": []string{"<div>", "</div>"},  // Statics
    "0": "dynamic value",               // Dynamic at position 0
    "1": nestedTreeNode,                // Nested tree
}
```

**Dependencies:**
- internal/parse/ (AST parser)
- internal/build/ (tree building)

**Used By:** template.go

---

### Internal Packages (v1.0 Architecture)

#### internal/parse/ (~1,320 lines)
**Purpose:** AST-based template parser - parses Go templates into tree structures

**Files:**
- `parser.go` (280 lines) - Main parser entry point
- `constructs.go` (290 lines) - Construct type definitions
- `compile.go` (320 lines) - Compilation logic (structure definition)
- `hydrate.go` (260 lines) - Hydration logic (data filling)
- `helpers.go` (170 lines) - Utility functions

**Key Types:**
- `Construct` interface - Common interface for all template constructs
- `FieldConstruct` - Simple field replacement `{{.Field}}`
- `ConditionalConstruct` - If/else branches `{{if .Cond}}...{{else}}...{{end}}`
- `RangeConstruct` - Iteration `{{range .Items}}...{{end}}`
- `WithConstruct` - Context switching `{{with .Item}}...{{end}}`
- `TemplateInvokeConstruct` - Template invocation `{{template "name" .}}`

**Key Functions:**
- `ParseTemplateToTreeAST(templateStr, data, keyGen) (TreeNode, error)` - Main parser
- `buildTreeFromAST(node, data, keyGen) (TreeNode, error)` - Recursive AST walk
- `compileConstruct(construct) error` - Compile phase
- `hydrateConstruct(construct, data) error` - Hydration phase

**How It Works:**
1. Parse template using stdlib html/template
2. Walk AST to identify template constructs
3. Compile constructs (define structure)
4. Hydrate constructs (fill with data)
5. Build tree with statics and dynamics separated

**Dependencies:**
- internal/build/ (tree types)
- template_flatten.go (template composition)

**Used By:** tree.go, template.go

---

#### internal/build/ (~570 lines)
**Purpose:** Tree construction and operations

**Files:**
- `builder.go` (210 lines) - Tree building orchestration
- `tree_ops.go` (180 lines) - Tree manipulation operations
- `fingerprint.go` (90 lines) - Change detection via structure fingerprinting
- `types.go` (40 lines) - Core tree types

**Key Types:**
- `TreeNode` - map[string]interface{} representing tree structure
- `RangeData` - Range metadata (item keys, construct ID)
- `TreeMetadata` - Metadata annotations for trees

**Key Functions:**
- `BuildTree(statics []string, dynamics []interface{}) TreeNode` - Construct tree
- `CalculateFingerprint(tree TreeNode) string` - MD5 hash for change detection
- `MergeTree(target, source TreeNode)` - Merge operations
- `CloneTree(tree TreeNode) TreeNode` - Deep copy

**Dependencies:** None (foundational)

**Used By:** internal/parse/, internal/diff/, tree.go, template.go

---

#### internal/diff/ (~1,570 lines)
**Purpose:** Tree comparison and minimal update generation

**Files:**
- `tree_compare.go` (420 lines) - Main tree comparison logic
- `range_ops.go` (400 lines) - Range differential operations
- `prepare.go` (62 lines) - Client preparation (static stripping)
- `helpers.go` (661 lines) - Comparison helper functions
- `types.go` (27 lines) - Type aliases for compatibility

**Key Functions:**
- `CompareTreesAndGetChangesWithPath(old, new, insideNewStructure, path, rangeMatches) TreeNode` - Main comparison orchestrator
- `ClientNeedsStatics(oldTree, newTree) bool` - Fingerprint-based structure comparison
- `GenerateRangeDifferentialOperations(oldTree, newTree, clientHasStatics) []interface{}` - Range diff orchestrator
- `PrepareTreeForClient(node, clientHasStatics) interface{}` - Strip statics for wire transmission

**Architecture Pattern:** Orchestrator → Coordinator → Helper
- **Orchestrators** (~30 lines): High-level flow control
- **Coordinators** (20-50 lines): Coordinate one aspect (e.g., removals, insertions)
- **Helpers** (<20 lines): Pure functions with single responsibility

**Range Operations Generated:**
- `["u", "item-id", updates]` - Update existing item
- `["i", "after-id", "position", data]` - Insert new item
- `["r", "item-id"]` - Remove item
- `["o", ["id1", "id2", ...]]` - Reorder items

**Wire Format Optimization:**
- First render: Full tree WITH statics
- Updates: ONLY changed dynamics, NO statics (client has cached them)
- Result: ~90% size reduction for updates

**Dependencies:**
- internal/build/ (tree types)

**Used By:** template.go

---

#### internal/observe/ (~449 lines)
**Purpose:** Production-ready observability with structured logging and metrics

**Files:**
- `logger.go` (180 lines) - Structured logging with slog
- `metrics.go` (210 lines) - Operational metrics
- `context.go` (59 lines) - Context enrichment

**Key Types:**
- `Logger` - Wrapper around slog.Logger with domain-specific methods
- `Metrics` - Operational metrics collector

**Key Functions:**
- `NewLogger(level, handler) *Logger` - Create structured logger
- `NewMetrics(logger) *Metrics` - Create metrics collector
- `EmitPeriodically(interval)` - Background metrics emission
- `RecordTemplateExecution(duration)` - Record timing
- `RecordWebSocketConnection()` - Record connection
- `RecordAction(action, duration)` - Record action metrics

**Metrics Tracked:**
- Template executions (count, duration)
- WebSocket connections (active, total)
- Actions processed (by type)
- Errors (by type)
- Update sizes (bytes)

**Log Levels:**
- Debug: Template operations, tree generation
- Info: Connections, configuration
- Warn: Validation failures, retries
- Error: Fatal errors, panics

**Dependencies:** None (uses stdlib slog)

**Used By:** template.go, mount.go, examples/

---

#### template_flatten.go (~400 lines)
**Purpose:** Template composition resolver

**Key Functions:**
- `flattenTemplate(tmpl *template.Template) (string, error)` - Flatten template
- `hasTemplateComposition(tmpl) bool` - Check for {{template}} calls
- `resolveTemplateInvocations(node, tmpl, result) error` - Resolve invocations
- `getTemplateByName(tmpl, name) (*template.Template, error)` - Find template

**How It Works:**
1. Detect {{template "name" .}} invocations
2. Inline the referenced template's content
3. Recursively resolve nested invocations
4. Return flattened template string

**Dependencies:** None (self-contained)

**Used By:** tree_ast.go

---

#### template_discovery.go (~100 lines)
**Purpose:** Auto-discovery of template files

**Key Functions:**
- `discoverTemplateFiles() ([]string, error)` - Find template files
- `findTemplateFile(name) string` - Find specific template

**Search Locations:**
- Current directory
- ./templates/
- ./views/
- ./web/templates/
- ./web/views/

**Search Extensions:**
- .tmpl
- .html
- .gotmpl

**Dependencies:** None

**Used By:** template.go (New function)

---

### Supporting Files

#### errors.go (~50 lines)
**Purpose:** Error handling utilities

**Key Functions:**
- Error wrapping and formatting
- Validation error helpers

**Dependencies:** None

**Used By:** template.go, mount.go

---

#### html_minify.go (~100 lines)
**Purpose:** HTML minification (optional optimization)

**Key Functions:**
- `minifyHTML(html string) string` - Remove unnecessary whitespace

**Dependencies:** None

**Used By:** template.go (conditionally)

---

## Test Files

### E2E Tests

#### e2e_test.go (~1,800 lines)
**Purpose:** End-to-end rendering sequences with golden file validation

**Test Scenarios:**
- Complete rendering sequence (todos)
- Simple counter updates
- Component-based templates
- Range operations (add, remove, reorder)
- No-change updates
- Performance benchmarks

**Golden Files:** testdata/e2e/*.json, *.html

---

### Integration Tests

#### template_test.go (~800 lines)
**Purpose:** Core template functionality tests

**Test Coverage:**
- Template parsing
- Tree generation
- Update generation
- Error handling
- Configuration options

---

#### focus_preservation_test.go (~300 lines)
**Purpose:** Browser E2E test for focus preservation

**Tests:**
- Input focus maintained during updates
- Scroll position preserved
- Form state persistence

**Uses:** chromedp for browser automation

---

#### loading_indicator_test.go (~200 lines)
**Purpose:** Browser E2E test for loading indicators

**Tests:**
- Loading indicator shown/hidden correctly
- Timing and transitions
- User experience

**Uses:** chromedp

---

### Unit Tests

#### tree_invariant_test.go (~400 lines)
**Purpose:** Tree structure invariant validation

**Tests:**
- Tree structure correctness
- Statics/dynamics separation
- Key uniqueness
- Fingerprint consistency

---

#### tree_fuzz_test.go (~200 lines)
**Purpose:** Fuzz testing for template parser

**Tests:**
- Random template inputs
- Parser robustness
- Crash prevention

---

#### tree_deep_nesting_test.go (~150 lines)
**Purpose:** Deep nesting scenarios

**Tests:**
- Deeply nested conditionals
- Nested ranges
- Performance with deep structures

---

#### tree_nested_conditionals_test.go (~150 lines)
**Purpose:** Complex conditional logic

**Tests:**
- If/else chains
- Nested if statements
- Edge cases

---

#### key_injection_test.go (~200 lines)
**Purpose:** Key generation and stability tests

**Tests:**
- Key uniqueness
- Key stability across renders
- Key generation patterns

---

#### template_flatten_test.go (~300 lines)
**Purpose:** Template composition tests

**Tests:**
- Template invocations
- Nested templates
- Recursive resolution

---

### Test Helpers

#### tree_test_helpers.go (~100 lines)
**Purpose:** Shared test utilities

**Functions:**
- Tree comparison helpers
- JSON normalization
- Test data generation

---

## CLI Tool

Located in `cmd/lvt/`:

```
cmd/lvt/
├── main.go                 # CLI entry point
├── commands/               # CLI commands
│   ├── new.go              # Create new apps
│   ├── gen.go              # Generate resources
│   ├── kits.go             # Kit management
│   └── serve.go            # Development server
├── internal/
│   ├── generator/          # Code generation engine
│   ├── kits/               # Kit system
│   │   ├── loader.go       # Kit loading
│   │   ├── types.go        # Kit types
│   │   └── system/         # Built-in kits (Tailwind, Bulma, Pico, None)
│   ├── config/             # Configuration management
│   └── serve/              # Development server
└── e2e/                    # E2E tests for CLI
    └── tutorial_test.go    # Tutorial walkthrough test
```

**Key Features:**
- App scaffolding with CSS framework selection
- CRUD generation with forms, tables, validation
- Component system (reusable UI blocks)
- Kit system (CSS framework integrations)
- Hot reload development server

See [CLI Documentation](user-guide.md) for details.

---

## Client Library

Located in `client/`:

```
client/
├── livetemplate-client.ts          # Main client implementation
├── livetemplate-client.test.ts     # Jest tests
├── package.json
├── tsconfig.json
└── dist/                           # Built output
    └── livetemplate-client.min.js
```

**Key Features:**
- WebSocket connection with auto-reconnect
- HTTP fallback
- Event delegation (`lvt-*` attributes)
- Tree-based DOM updates
- Focus preservation
- Loading indicators
- Form lifecycle events

**Size:** ~15KB minified

---

## File Dependencies

### Dependency Graph (v1.0)

```
User Application
    ↓
template.go (Public API & Orchestrator)
    ↓
    ├→ tree.go (Tree operations)
    │    ↓
    │    ├→ internal/parse/ (AST parser)
    │    │    ↓
    │    │    ├→ internal/build/ (Tree building)
    │    │    └→ template_flatten.go (Composition)
    │    │
    │    └→ internal/build/ (Tree building)
    │
    ├→ internal/diff/ (Tree comparison)
    │    ↓
    │    └→ internal/build/ (Tree types)
    │
    ├→ internal/observe/ (Observability)
    │
    ├→ action.go (Actions & data binding)
    │
    ├→ mount.go (HTTP/WebSocket handlers)
    │    ↓
    │    ├→ session.go (Session management)
    │    ├→ context.go (Context type)
    │    ├→ state.go (State interface)
    │    └→ internal/observe/ (Logging)
    │
    └→ template_discovery.go (File discovery)
```

### Import Relationships

**Level 0 (No dependencies - Foundational):**
- action.go
- errors.go
- html_minify.go
- session.go
- template_discovery.go
- template_flatten.go
- internal/build/ (tree types and operations)
- internal/observe/ (observability)

**Level 1 (Depends on Level 0 - Internal Packages):**
- internal/parse/ (uses internal/build/, template_flatten.go)
- internal/diff/ (uses internal/build/)

**Level 2 (Depends on Level 1 - Core Logic):**
- tree.go (uses internal/parse/, internal/build/)

**Level 3 (Depends on Level 2 - Public API):**
- template.go (uses tree.go, internal/parse/, internal/build/, internal/diff/, internal/observe/, action.go, session.go)
- mount.go (uses action.go, session.go, internal/observe/)

**Top Level:**
- User applications (use template.go, action.go, mount.go)

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

### For Contributors (v1.0)

**Adding New Template Features:**
1. Start in internal/parse/constructs.go (define construct type)
2. Implement Parse, Compile, Hydrate methods
3. Add parser logic in internal/parse/parser.go
4. Add compilation in internal/parse/compile.go
5. Add hydration in internal/parse/hydrate.go
6. Add tests in template_test.go
7. Update internal/build/ if tree structure changes

**Modifying Tree Structure:**
1. Start in internal/build/types.go (TreeNode definition)
2. Update internal/build/tree_ops.go (operations)
3. Update internal/parse/ (tree building)
4. Update internal/diff/ (comparison logic)
5. Update client/livetemplate-client.ts (tree consumption)
6. Add tests in tree_invariant_test.go

**Improving Diff Algorithm:**
1. Start in internal/diff/tree_compare.go (orchestrator)
2. Add coordinator functions for specific scenarios
3. Add helper functions in internal/diff/helpers.go
4. Follow orchestrator → coordinator → helper pattern
5. Ensure functions are <50 lines
6. Add tests in e2e_test.go

**Adding HTTP/WebSocket Features:**
1. Start in mount.go
2. Update action.go if protocol changes
3. Add observability in internal/observe/
4. Add tests in e2e_test.go

**Adding Observability:**
1. Add metrics in internal/observe/metrics.go
2. Add logging in internal/observe/logger.go
3. Integrate in template.go or mount.go
4. Test in examples/observability/

---

## Quick Reference

### Where to Find Things (v1.0)

| What | Where |
|------|-------|
| Public API | template.go |
| Controller+State pattern | mount.go, state.go, context.go |
| Action data binding | action.go |
| Template parsing | internal/parse/ |
| Tree building | internal/build/ |
| Tree comparison | internal/diff/ |
| Observability | internal/observe/ |
| Tree operations | tree.go |
| HTTP handlers | mount.go |
| Session management | session.go |
| Template discovery | template_discovery.go |
| Composition resolver | template_flatten.go |
| E2E tests | e2e_test.go |
| Client library | client/livetemplate-client.ts |
| CLI tool | cmd/lvt/ |

### Component Size Summary (v1.0)

| Component | Lines | Purpose |
|-----------|-------|---------|
| internal/diff/ | ~1,570 | Tree comparison & updates |
| template.go | ~1,400 | Main API (orchestrator) |
| e2e_test.go | ~1,800 | E2E tests |
| internal/parse/ | ~1,320 | Template parsing (AST) |
| template_test.go | ~800 | Unit tests |
| internal/build/ | ~570 | Tree building |
| mount.go | ~500 | HTTP/WS handlers |
| internal/observe/ | ~449 | Observability |
| tree.go | ~400 | Tree operations |
| template_flatten.go | ~400 | Composition |
| action.go | ~270 | Actions |
| session.go | ~150 | Sessions |
| template_discovery.go | ~100 | Discovery |

**Total:** ~8,000 lines of core library code + ~3,000 lines of tests + ~3,900 lines in internal packages

**Architecture Note:** v1.0 refactored monolithic template.go (2,500 lines) into orchestrator (1,400 lines) + internal packages (3,900 lines) with zero breaking changes.

---

## Navigation Tips (v1.0)

**For New Users:**
1. **Start with template.go** - Understand the public API
2. **Then mount.go, state.go, context.go** - Learn the Controller+State pattern
3. **Then examples/** - See real usage (counter, todos, observability)
4. **Then action.go** - Understand action data binding
5. **Check e2e_test.go** - See comprehensive test scenarios

**For Contributors:**
1. **Read ARCHITECTURE.md** - Understand v1.0 design principles
2. **Read internal/parse/parser.go** - See how templates are parsed
3. **Read internal/build/builder.go** - See how trees are built
4. **Read internal/diff/tree_compare.go** - See orchestrator → coordinator → helper pattern
5. **Check CLAUDE.md** - Development guidelines and conventions
6. **Run tests** - `go test -v ./...`

For guided walkthrough, see [CODE_TOUR.md](CODE_TOUR.md)

For architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md)

For contribution guide, see [CONTRIBUTING.md](../CONTRIBUTING.md)
