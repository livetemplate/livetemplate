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

#### template.go (~2,500 lines)
**Purpose:** Main API entry point for the library

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
- tree_ast.go (AST parsing)
- mount.go (HTTP handlers)
- session.go (session management)

**Used By:** All user applications

---

#### action.go (~270 lines)
**Purpose:** Action protocol and data binding

**Key Types:**
- `Store` interface - User-defined state management
- `StoreInitializer` interface - Optional initialization
- `ActionContext` - Context for Change() method
- `ActionData` - Type-safe data extraction
- `FieldError` - Validation error
- `MultiError` - Collection of field errors

**Key Functions:**
- `Bind(v interface{}) error` - Unmarshal to struct
- `BindAndValidate(v, validator) error` - Bind + validate
- `GetString/GetInt/GetFloat/GetBool(key)` - Type-safe getters
- `ValidationToMultiError(err) MultiError` - Convert validator errors

**Internal Functions:**
- `parseAction(action string) (store, actualAction)` - Parse "store.action"
- `parseActionFromHTTP(r *http.Request) (message, error)` - HTTP parser
- `parseActionFromWebSocket(data []byte) (message, error)` - WS parser
- `writeUpdateWebSocket(conn, update) error` - WS writer

**Dependencies:** None (self-contained)

**Used By:** template.go, mount.go, user applications

---

#### mount.go (~500 lines)
**Purpose:** HTTP/WebSocket handlers and store pattern

**Key Functions:**
- `Handle(store Store) http.Handler` - Single store handler
- `HandleStores(template, stores) http.Handler` - Multi-store handler
- HTTP handlers (handleHTTPRequest, handleAction)
- WebSocket handlers (handleWebSocket, message loops)

**Key Features:**
- Session management (per-connection state)
- Store cloning (isolation between sessions)
- Error handling (validation errors, panics)
- Broadcasting support

**Dependencies:**
- template.go (Template type)
- action.go (Store, ActionContext)
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

#### tree_ast.go (~1,200 lines)
**Purpose:** AST-based template parser (main parser implementation)

**Key Types:**
- `orderedVars` - Deterministic map for variable iteration
- Various construct types (not exported)

**Key Functions:**
- `parseTemplateToTreeAST(templateStr, data, keyGen) (treeNode, error)` - Main parser
- `buildTreeFromAST(node, data, keyGen) (treeNode, error)` - Recursive AST walk
- `buildTreeFromList(node, data, keyGen) (treeNode, error)` - List processing
- `handleActionNode(node, data, keyGen) (treeNode, error)` - {{.Field}} handler
- `handleIfNode(node, data, keyGen) (treeNode, error)` - {{if}} handler
- `handleRangeNode(node, data, keyGen) (treeNode, error)` - {{range}} handler
- `handleWithNode(node, data, keyGen) (treeNode, error)` - {{with}} handler

**How It Works:**
1. Parse template using stdlib html/template
2. Walk AST to identify template constructs
3. Compile constructs (define structure)
4. Hydrate constructs (fill with data)
5. Build tree with statics and dynamics separated

**Dependencies:**
- tree.go (treeNode, keyGenerator)
- template_flatten.go (template composition)

**Used By:** template.go (via parseTemplateToTree)

---

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
- tree_ast.go (AST parser)

**Used By:** template.go, tree_ast.go

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

### Dependency Graph

```
User Application
    ↓
template.go (Public API)
    ↓
    ├→ tree.go (Tree operations)
    │    ↓
    │    └→ tree_ast.go (AST parser)
    │         ↓
    │         └→ template_flatten.go (Composition)
    │
    ├→ action.go (Actions & data binding)
    │
    ├→ mount.go (HTTP/WebSocket handlers)
    │    ↓
    │    ├→ session.go (Session management)
    │    └→ action.go (Store interface)
    │
    └→ template_discovery.go (File discovery)
```

### Import Relationships

**Level 0 (No dependencies):**
- action.go
- errors.go
- html_minify.go
- session.go
- template_discovery.go
- template_flatten.go

**Level 1 (Depends on Level 0):**
- tree.go (uses template_flatten.go)

**Level 2 (Depends on Level 1):**
- tree_ast.go (uses tree.go)

**Level 3 (Depends on Level 2):**
- template.go (uses tree.go, tree_ast.go, action.go, session.go)
- mount.go (uses action.go, session.go)

**Top Level:**
- User applications (use template.go, action.go, mount.go)

---

## Entry Points

### For Library Users

**Creating a Template:**
```go
tmpl := livetemplate.New("counter")  // template.go
```

**Handling Requests:**
```go
http.Handle("/", tmpl.Handle(store))  // mount.go
```

**Implementing State:**
```go
type State struct { ... }
func (s *State) Change(ctx *livetemplate.ActionContext) error {
    // action.go provides ActionContext
}
```

### For Contributors

**Adding New Template Features:**
1. Start in tree_ast.go (AST parsing)
2. Update construct handling
3. Add tests in template_test.go
4. Update tree.go if needed

**Modifying Tree Structure:**
1. Start in tree.go (treeNode definition)
2. Update tree_ast.go (tree building)
3. Update client/livetemplate-client.ts (tree consumption)
4. Add tests in tree_invariant_test.go

**Adding HTTP/WebSocket Features:**
1. Start in mount.go
2. Update action.go if protocol changes
3. Add tests in e2e_test.go

---

## Quick Reference

### Where to Find Things

| What | Where |
|------|-------|
| Public API | template.go |
| Store interface | action.go |
| Template parsing | tree_ast.go |
| Tree operations | tree.go |
| HTTP handlers | mount.go |
| Session management | session.go |
| Template discovery | template_discovery.go |
| Composition resolver | template_flatten.go |
| E2E tests | e2e_test.go |
| Client library | client/livetemplate-client.ts |
| CLI tool | cmd/lvt/ |

### File Size Summary

| File | Lines | Purpose |
|------|-------|---------|
| template.go | ~2,500 | Main API |
| e2e_test.go | ~1,800 | E2E tests |
| tree_ast.go | ~1,200 | AST parser |
| template_test.go | ~800 | Unit tests |
| mount.go | ~500 | HTTP/WS handlers |
| tree.go | ~400 | Tree operations |
| template_flatten.go | ~400 | Composition |
| action.go | ~270 | Actions |
| session.go | ~150 | Sessions |
| template_discovery.go | ~100 | Discovery |

**Total:** ~8,000 lines of core library code + ~3,000 lines of tests

---

## Navigation Tips

1. **Start with template.go** - Understand the public API
2. **Then action.go** - Learn the Store pattern
3. **Then tree_ast.go** - See how templates become trees
4. **Then mount.go** - Understand request handling
5. **Check tests** - See real usage examples

For guided walkthrough, see [CODE_TOUR.md](CODE_TOUR.md)

For architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md)

For contribution guide, see [CONTRIBUTING.md](../CONTRIBUTING.md)
