# LiveTemplate Architecture

This document describes the architecture and design of LiveTemplate, a high-performance Go library for building reactive web applications.

## Table of Contents

- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [System Architecture](#system-architecture)
- [Data Flow](#data-flow)
- [Key Components](#key-components)
- [Template Processing Pipeline](#template-processing-pipeline)
- [Tree Structure](#tree-structure)
- [Update Generation](#update-generation)
- [Client-Server Protocol](#client-server-protocol)
- [Performance Optimizations](#performance-optimizations)
- [Design Decisions](#design-decisions)

## Overview

LiveTemplate enables building reactive web applications by:
1. Separating static and dynamic content in templates
2. Generating minimal tree-based updates
3. Sending only changed data over the wire (WebSocket or HTTP)
4. Applying updates efficiently on the client side

This approach is inspired by Phoenix LiveView but implemented in Go with zero dependencies on JavaScript frameworks.

## Core Concepts

### 1. Store Pattern

State management follows the Store pattern:

```go
type Store interface {
    Change(ctx *ActionContext) error
}
```

- User defines a struct that implements `Store`
- Actions are method-like calls (e.g., "increment", "save")
- State changes trigger automatic re-renders
- No explicit state management code needed

### 2. Tree-based Representation

Templates are parsed into a tree structure that separates:
- **Statics**: Template structure (HTML tags, static text) - sent once, cached client-side
- **Dynamics**: Data values that change - sent on every update

```json
{
  "s": ["<div>", "</div>"],
  "0": "Dynamic content here"
}
```

### 3. Minimal Updates

After the first render:
- Only changed dynamic values are sent
- Statics are referenced by ID (already cached on client)
- Nested trees allow surgical updates to specific parts of the DOM

### 4. Action Protocol

User interactions trigger actions:

```html
<button lvt-click="increment">+</button>
```

1. Client captures click event
2. Sends `{"action": "increment", "data": {...}}` to server
3. Server calls `store.Change(ctx)`
4. Server re-renders and sends tree update
5. Client applies update to DOM

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client (Browser)                     │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  livetemplate-client.ts (TypeScript)                   │ │
│  │  - Event listeners (lvt-click, lvt-submit, etc.)      │ │
│  │  - WebSocket/HTTP communication                       │ │
│  │  - Tree cache (statics)                               │ │
│  │  - DOM updates (tree application)                     │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              ▲ │
                              │ │ WebSocket/HTTP
                              │ │ Tree Updates
                              │ ▼
┌─────────────────────────────────────────────────────────────┐
│                         Server (Go)                          │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  mount.go - HTTP/WebSocket Handlers                   │ │
│  │  - Session management                                 │ │
│  │  - Action routing (parseAction)                       │ │
│  │  - Store lifecycle                                    │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  template.go - Template Management                    │ │
│  │  - Template parsing and caching                       │ │
│  │  - Update generation (ExecuteToUpdate)               │ │
│  │  - Tree diffing and fingerprinting                   │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  tree_ast.go - AST Parser                             │ │
│  │  - Parse Go templates to tree structure              │ │
│  │  - Construct compilation (fields, ranges, etc.)      │ │
│  │  - Hydration (fill constructs with data)             │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  tree.go - Tree Operations                            │ │
│  │  - Key generation (sequential wrapper keys)          │ │
│  │  - Fingerprint calculation (change detection)        │ │
│  │  - Tree normalization                                 │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  User Store (implements Store interface)              │ │
│  │  - Application state                                  │ │
│  │  - Business logic in Change() method                 │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

### First Render (Initial Load)

```
1. HTTP GET /
   └─> mount.go: Handle()
       └─> template.go: ExecuteToHTML()
           ├─> tree_ast.go: parseTemplateToTree()
           │   ├─> Compile constructs
           │   └─> Hydrate with data
           ├─> tree.go: Generate keys
           ├─> tree.go: Calculate fingerprint
           └─> Inject wrapper div with ID
       └─> Return HTML with embedded tree data
```

### Subsequent Updates (WebSocket)

```
1. Client: User clicks button (lvt-click="increment")
   │
2. Client: Send {"action": "increment", "data": {...}}
   │
3. Server: mount.go receives WebSocket message
   ├─> parseActionFromWebSocket()
   ├─> store.Change(ctx)  // User's business logic
   └─> template.ExecuteToUpdate(store)
       ├─> parseTemplateToTree() with new data
       ├─> Diff: Compare new tree vs lastTree
       ├─> Generate minimal update (only changes)
       └─> Return UpdateResponse
   │
4. Server: Send tree update via WebSocket
   │
5. Client: Receive update
   ├─> Resolve statics from cache
   ├─> Build DOM from tree
   └─> Apply to specific wrapper div
```

## Key Components

### 1. Template (`template.go`)

**Purpose:** Main API entry point for template management

**Key Methods:**
- `New(name string) *Template` - Create template from file
- `ExecuteToHTML(data) (string, error)` - First render (full HTML)
- `ExecuteToUpdate(data) (*UpdateResponse, error)` - Subsequent renders (tree update)
- `Handle(store Store) http.Handler` - WebSocket/HTTP handler

**State:**
- `lastTree` - Previous render's tree (for diffing)
- `lastData` - Previous render's data (for caching)
- `keyGen` - Key generator for current template instance
- `wrapperID` - Unique ID for targeting updates

### 2. AST Parser (`tree_ast.go`)

**Purpose:** Parse Go templates into tree structures

**Key Types:**
- `Construct` interface - Represents template elements
  - `FieldConstruct` - `{{.Field}}`
  - `ConditionalConstruct` - `{{if}}...{{else}}...{{end}}`
  - `RangeConstruct` - `{{range}}...{{end}}`
  - `WithConstruct` - `{{with}}...{{end}}`
  - `TemplateInvokeConstruct` - `{{template "name" .}}`

**Processing:**
1. Parse template into html/template AST
2. Walk AST nodes
3. Compile constructs (identify static/dynamic boundaries)
4. Hydrate constructs (fill with actual data)
5. Build tree structure

### 3. Tree Operations (`tree.go`)

**Purpose:** Low-level tree manipulation and utilities

**Key Functions:**
- `newKeyGenerator()` - Create key generator
- `calculateFingerprint(tree)` - MD5 hash for change detection
- `injectWrapperDiv(html, id)` - Wrap content for targeting
- `extractTemplateContent(html, id)` - Extract wrapped content

**Tree Structure:**
```go
type treeNode map[string]interface{}
// "s" -> []string (statics)
// "0", "1", "2", ... -> interface{} (dynamics)
```

### 4. Mount & Session (`mount.go`)

**Purpose:** HTTP/WebSocket handling and session management

**Key Functions:**
- `Handle(store Store) http.Handler` - Create handler for store
- Session management via gorilla/sessions
- Store cloning (each session gets its own store instance)
- Action routing and error handling

**Store Lifecycle:**
1. Clone user's store for new session
2. Call `Init()` if store implements `StoreInitializer`
3. Handle actions via `Change(ctx)`
4. Re-render and send updates

### 5. Action System (`action.go`)

**Purpose:** Action protocol and data binding

**Key Types:**
- `ActionContext` - Context for Change() method
  - `Action` - Action name (e.g., "increment")
  - `Data` - ActionData wrapper
- `ActionData` - Data extraction and validation
  - `Bind(v interface{})` - Unmarshal to struct
  - `BindAndValidate(v, validator)` - Bind + validate
  - `GetString/GetInt/GetFloat/GetBool(key)` - Type-safe accessors

**Multi-store Actions:**
- Single store: `"increment"` → `Change(ctx)` where `ctx.Action == "increment"`
- Multi-store: `"counter.increment"` → Routes to `stores["counter"]`

### 6. Client Library (`client/livetemplate-client.ts`)

**Purpose:** Browser-side event handling and DOM updates

**Key Features:**
- Event binding (`lvt-click`, `lvt-submit`, etc.)
- WebSocket/HTTP communication
- Tree cache (statics cached in memory)
- Efficient DOM updates (only changed elements)
- Focus preservation
- Loading indicators

## Template Processing Pipeline

### Compilation Phase (One-time)

```
Template String
    │
    ▼
html/template Parse
    │
    ▼
Walk AST Nodes
    │
    ▼
Identify Constructs
    │
    ├─> FieldConstruct: {{.Name}}
    ├─> ConditionalConstruct: {{if .Active}}...{{end}}
    ├─> RangeConstruct: {{range .Items}}...{{end}}
    ├─> WithConstruct: {{with .User}}...{{end}}
    └─> TemplateInvokeConstruct: {{template "header" .}}
    │
    ▼
Build Construct Tree
```

### Hydration Phase (Every Render)

```
Construct Tree + Data
    │
    ▼
Hydrate Constructs
    │
    ├─> Field: Extract value from data
    ├─> Conditional: Evaluate condition, select branch
    ├─> Range: Iterate items, hydrate body for each
    ├─> With: Change context, hydrate body
    └─> Template: Invoke nested template
    │
    ▼
Generate Tree Structure
    │
    ├─> Statics: ["<div>", "</div>"]
    └─> Dynamics: {"0": value, "1": nested_tree}
```

## Tree Structure

### Simple Example

**Template:**
```html
<div>Hello {{.Name}}</div>
```

**Tree:**
```json
{
  "s": ["<div>Hello ", "</div>"],
  "0": "World"
}
```

### Nested Example

**Template:**
```html
<div>
  {{if .ShowMessage}}
    <p>{{.Message}}</p>
  {{end}}
</div>
```

**Tree (when ShowMessage=true):**
```json
{
  "s": ["<div>\n  ", "\n</div>"],
  "0": {
    "s": ["<p>", "</p>"],
    "0": "Hello"
  }
}
```

### Range Example

**Template:**
```html
<ul>
  {{range .Items}}
    <li>{{.}}</li>
  {{end}}
</ul>
```

**Tree:**
```json
{
  "s": ["<ul>\n  ", "\n</ul>"],
  "0": [
    {"s": ["<li>", "</li>"], "0": "Item 1"},
    {"s": ["<li>", "</li>"], "0": "Item 2"},
    {"s": ["<li>", "</li>"], "0": "Item 3"}
  ]
}
```

## Update Generation

### Diffing Strategy

1. **Fingerprint Comparison**
   - Calculate MD5 hash of tree (statics + dynamics)
   - If fingerprint unchanged, return minimal "no change" response
   - If changed, perform tree diff

2. **Tree Diff**
   - Compare new tree vs last tree
   - Identify changed dynamic values
   - Generate minimal update containing only changes

3. **Range Operations**
   For lists, special operations optimize updates:
   - `["u", "id", updates]` - Update existing item
   - `["i", "after-id", "position", data]` - Insert new item
   - `["r", "id"]` - Remove item
   - `["o", ["id1", "id2", ...]]` - Reorder items

### Update Format

**Full tree (first render):**
```json
{
  "tree": {
    "s": ["<div>", "</div>"],
    "0": "Initial value"
  },
  "meta": {
    "wrapper_id": "lvt-abc123",
    "timestamp": "2025-10-19T10:00:00Z"
  }
}
```

**Minimal update (subsequent renders):**
```json
{
  "tree": {
    "0": "Updated value"
  },
  "meta": {
    "wrapper_id": "lvt-abc123",
    "timestamp": "2025-10-19T10:00:01Z"
  }
}
```

## Client-Server Protocol

### WebSocket Protocol

**Client → Server (Action):**
```json
{
  "action": "increment",
  "data": {
    "amount": 1,
    "user_id": "123"
  }
}
```

**Server → Client (Update):**
```json
{
  "tree": { /* tree update */ },
  "meta": { /* metadata */ }
}
```

### HTTP Fallback

For browsers without WebSocket support:
1. Client POSTs action to `/action` endpoint
2. Server processes action and responds with tree update
3. Client applies update

Same protocol format, different transport.

## Performance Optimizations

### 1. Static Content Caching

- Statics sent once on first render
- Client caches by tree structure hash
- Subsequent updates reference cached statics
- Reduces bandwidth by 50-90% for typical apps

### 2. Key Generation

- Sequential integer keys (1, 2, 3, ...)
- Minimal overhead (simple counter increment)
- Stable within a single render
- No complex hashing or UUID generation

### 3. Fingerprinting

- MD5 hash of entire tree
- Early exit if tree unchanged
- Prevents unnecessary tree diff
- O(1) comparison vs O(n) diff

### 4. Tree Diffing

- O(n) complexity for most operations
- Only compares changed subtrees
- Leverages structural sharing

### 5. Memory Management

- Templates are long-lived (parsed once)
- Trees are ephemeral (generated per render)
- Stores are per-session (cloned on creation)

## Design Decisions

### Why AST-based Parser?

- **Correctness:** Handles all Go template features correctly
- **Maintainability:** Uses stdlib `html/template` parser
- **Future-proof:** Automatically supports new template features
- **Performance:** One-time compilation, fast hydration

Previous regex-based parser was removed due to incorrect handling of nested constructs.

### Why Tree Structure?

- **Minimal updates:** Only send changed values
- **Client-side caching:** Statics cached in memory
- **Surgical DOM updates:** Update specific elements without full re-render
- **Bandwidth efficiency:** 50-90% reduction vs full HTML

### Why Sequential Keys?

- **Simplicity:** No complex key generation logic
- **Performance:** O(1) key generation (counter increment)
- **Universality:** Works with any data type
- **Stability:** Keys consistent within a render

Alternative approaches (content-based hashing, explicit user keys) were more complex without clear benefits.

### Why Store Pattern?

- **Simplicity:** Single `Change()` method for all actions
- **Type-safety:** User controls their own types
- **Flexibility:** No framework-imposed constraints
- **Testability:** Pure Go functions, easy to test

### Why No Client-side State?

- **Server authority:** Single source of truth
- **Simplified logic:** All business logic in Go
- **Security:** Validation and authorization on server
- **Consistency:** No client-server sync issues

Client only handles UI events and DOM updates.

### Why WebSocket Primary, HTTP Fallback?

- **Real-time:** WebSocket enables instant updates
- **Efficiency:** Persistent connection, no handshake overhead
- **Compatibility:** HTTP fallback for older browsers
- **Broadcasting:** WebSocket enables multi-user features

## Future Considerations

Potential improvements for future versions:

1. **Streaming Updates:** For large lists, stream tree updates incrementally
2. **Partial Hydration:** Only hydrate changed subtrees
3. **Advanced Diffing:** More sophisticated algorithms for complex trees
4. **Client-side Optimizations:** Virtual DOM, request coalescing
5. **Server-side Caching:** Cache compiled constructs across requests

---

For implementation details, see:
- [CODE_TOUR.md](CODE_TOUR.md) - Guided code walkthrough
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development guidelines
