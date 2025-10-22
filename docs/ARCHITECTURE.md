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
│  │  - morphdom for efficient DOM updates                │ │
│  │  - Focus preservation & loading indicators           │ │
│  │  - Form lifecycle events (pending, success, error)   │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              ▲ │
                              │ │ WebSocket/HTTP
                              │ │ Tree Updates + Broadcasts
                              │ ▼
┌─────────────────────────────────────────────────────────────┐
│                         Server (Go)                          │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  mount.go - LiveHandler & HTTP/WebSocket              │ │
│  │  - HTTP/WebSocket connection management               │ │
│  │  - Action routing and error handling                  │ │
│  │  - Store lifecycle (Init, Change, OnConnect, etc.)   │ │
│  │  - Broadcasting (Broadcast, BroadcastToUsers, etc.)  │ │
│  └────────────────────────────────────────────────────────┘ │
│         │                          │                         │
│         ▼                          ▼                         │
│  ┌─────────────────┐      ┌──────────────────────────────┐ │
│  │  registry.go    │      │  auth.go & session.go        │ │
│  │  Connection     │      │  - Authenticator interface   │ │
│  │  Registry       │      │  - AnonymousAuthenticator    │ │
│  │  - byGroup map  │      │  - BasicAuthenticator        │ │
│  │  - byUser map   │      │  - SessionStore (memory)     │ │
│  └─────────────────┘      └──────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  template.go - Template Management                    │ │
│  │  - Template parsing and caching                       │ │
│  │  - Update generation (ExecuteUpdates)                │ │
│  │  - Tree diffing and fingerprinting                   │ │
│  │  - Multi-store template data merging                 │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  tree_ast.go - AST Parser                             │ │
│  │  - Parse Go templates to tree structure              │ │
│  │  - Construct compilation (fields, ranges, etc.)      │ │
│  │  - Hydration (fill constructs with data)             │ │
│  │  - Ordered iteration for deterministic trees         │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  tree.go - Tree Operations                            │ │
│  │  - Key generation (sequential keys)                  │ │
│  │  - Fingerprint calculation (MD5 hash)                │ │
│  │  - Tree normalization                                 │ │
│  │  - Wrapper div injection                              │ │
│  └────────────────────────────────────────────────────────┘ │
│                              │                               │
│                              ▼                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  User Stores (per session group)                      │ │
│  │  - Store interface: Change(ctx)                       │ │
│  │  - Optional: StoreInitializer, BroadcastAware        │ │
│  │  - Shared within session group                        │ │
│  │  - Isolated across session groups                     │ │
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

## Authentication & Session Management

LiveTemplate implements a flexible authentication and session management system based on **session groups**.

### Session Groups

A **session group** is the fundamental isolation boundary for state sharing:
- All connections with the same `groupID` share the same `Stores` instance
- Different `groupID`s have completely isolated state
- Enables multi-tab sync, multi-device sync, and collaborative features

### Authenticator Interface

The `Authenticator` interface determines user identity and session group mapping:

```go
type Authenticator interface {
    // Identify returns the user ID (empty string for anonymous)
    Identify(r *http.Request) (userID string, err error)

    // GetSessionGroup returns the session group ID for this user
    GetSessionGroup(r *http.Request, userID string) (groupID string, err error)
}
```

**Built-in Authenticators:**

1. **AnonymousAuthenticator** (default)
   - Browser-based session grouping via persistent cookie (`livetemplate-id`)
   - All tabs in same browser → same `groupID` → shared state
   - Different browsers → different `groupID` → isolated state
   - Cookie persists for 1 year

2. **BasicAuthenticator**
   - Username/password authentication via HTTP Basic Auth
   - Simple 1:1 mapping: `groupID = userID`
   - Each user gets isolated state across all their devices/tabs
   - Validation via user-provided function

**Custom Authenticators:**

For production use, implement custom authenticators with:
- JWT tokens
- OAuth flows
- Session cookies from existing auth middleware
- Custom session group mapping (e.g., collaborative workspaces)

### SessionStore Interface

The `SessionStore` manages session groups and their associated stores:

```go
type SessionStore interface {
    Get(groupID string) Stores              // Retrieve stores for a group
    Set(groupID string, stores Stores)      // Store stores for a group
    Delete(groupID string)                  // Remove a session group
    List() []string                         // List all active group IDs
}
```

**Built-in Implementation:**

- **MemorySessionStore**: In-memory storage with automatic cleanup
  - TTL-based expiration (default: 24 hours)
  - Background cleanup goroutine
  - Suitable for single-instance deployments
  - Thread-safe for concurrent access

**Future:** Redis-based SessionStore for multi-instance deployments

### Session Lifecycle

```
1. HTTP Request arrives
   ├─> Authenticator.Identify(r) → userID
   └─> Authenticator.GetSessionGroup(r, userID) → groupID

2. SessionStore.Get(groupID) → existing Stores or nil

3. If Stores == nil:
   ├─> Clone user's initial stores
   ├─> Call Init() if store implements StoreInitializer
   ├─> Call OnConnect() if store implements BroadcastAware
   └─> SessionStore.Set(groupID, stores)

4. Handle request with group's stores

5. On WebSocket disconnect:
   └─> Call OnDisconnect() if store implements BroadcastAware
```

## Key Components

### 1. Template (`template.go`)

**Purpose:** Main API entry point for template management

**Key Methods:**
- `New(name string, opts ...TemplateOption) *Template` - Create template with options
- `ExecuteToHTML(data) (string, error)` - First render (full HTML) [Deprecated]
- `ExecuteUpdates(w, data, errors) error` - Generate tree updates (JSON output)
- `Handle(stores ...Store) LiveHandler` - Create handler (returns LiveHandler, not http.Handler)

**Template Options:**
- `WithAuthenticator(auth Authenticator)` - Custom authentication
- `WithSessionStore(store SessionStore)` - Custom session storage
- `WithOriginValidator(validator func(string) bool)` - WebSocket origin validation
- `WithLoadingDisabled()` - Disable loading indicator

**State (per connection):**
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

### 4. LiveHandler & Broadcasting (`mount.go`)

**Purpose:** HTTP/WebSocket handling, session management, and broadcasting

**LiveHandler Interface:**
```go
type LiveHandler interface {
    http.Handler
    Broadcast(data interface{}) error
    BroadcastToUsers(userIDs []string, data interface{}) error
    BroadcastToGroup(groupID string, data interface{}) error
}
```

**Key Features:**
- HTTP initial render + WebSocket upgrade for updates
- Session group management (multiple connections per group)
- Action routing with store namespace support (`store.action`)
- Broadcasting to all connections, specific users, or specific groups
- Automatic multi-tab syncing within session groups

**Store Lifecycle:**
1. Authenticate user and determine session group
2. Get or create stores for the group (from SessionStore)
3. Call `Init()` if store implements `StoreInitializer`
4. Call `OnConnect(ctx, broadcaster)` if store implements `BroadcastAware`
5. Handle actions via `Change(ctx)` with automatic updates to all group connections
6. Call `OnDisconnect()` on connection close

### 5. Connection Registry (`registry.go`)

**Purpose:** Track and manage active WebSocket connections with dual indexing

**Key Types:**
- `Connection`: WebSocket connection with metadata (groupID, userID, template, stores)
- `ConnectionRegistry`: Dual-indexed registry for efficient lookups

**Dual Indexing:**
- `byGroup map[string][]*Connection` - Efficient group broadcasts
- `byUser map[string][]*Connection` - Efficient user broadcasts

**Operations:**
- `Register(conn)` - Add connection to both indexes
- `Unregister(conn)` - Remove from both indexes, cleanup empty maps
- `GetByGroup(groupID)` - Get all connections in a session group
- `GetByUser(userID)` - Get all connections for a user (across groups)

**Use Cases:**
- Multi-tab automatic syncing (same groupID)
- User-specific notifications (by userID)
- Pub/sub topics (custom groupID mapping)

### 6. Broadcaster Interface (`mount.go`)

**Purpose:** Enable server-initiated updates without user actions

**BroadcastAware Interface:**
```go
type BroadcastAware interface {
    OnConnect(ctx context.Context, b Broadcaster) error
    OnDisconnect()
}

type Broadcaster interface {
    Send() error  // Re-render and send update to this connection
}
```

**Use Cases:**
- Live data feeds (stock prices, sports scores)
- Background job status updates
- Real-time notifications
- Collaborative features

**Example:**
```go
type LiveDataStore struct {
    Data string
    broadcaster Broadcaster
}

func (s *LiveDataStore) OnConnect(ctx context.Context, b Broadcaster) error {
    s.broadcaster = b
    go s.pollDataUpdates()  // Start background updates
    return nil
}

func (s *LiveDataStore) pollDataUpdates() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        s.Data = fetchLatestData()
        s.broadcaster.Send()  // Push update to client
    }
}
```

### 7. Action System (`action.go`)

**Purpose:** Action protocol and data binding

**Key Types:**
- `ActionContext` - Context for Change() method
  - `Action` - Action name (e.g., "increment")
  - `Data` - ActionData wrapper
- `ActionData` - Data extraction and validation
  - `Bind(v interface{})` - Unmarshal to struct
  - `BindAndValidate(v, validator)` - Bind + validate with go-playground/validator
  - `GetString/GetInt/GetFloat/GetBool(key)` - Type-safe accessors
  - `Has(key)` - Check if key exists
  - `Raw()` - Access underlying map

**Multi-store Actions:**
- Single store: `"increment"` → `Change(ctx)` where `ctx.Action == "increment"`
- Multi-store: `"counter.increment"` → Routes to `stores["counter"]`

**Error Handling:**
- Validation errors returned from `Change()` are automatically displayed to client
- Uses `ValidationError` and `MultiError` types for structured errors
- Client receives errors in `meta.errors` map (field → error message)

### 8. Client Library (`client/livetemplate-client.ts`)

**Purpose:** Browser-side event handling and DOM updates

**Key Features:**
- **Event binding**: `lvt-click`, `lvt-submit`, `lvt-change`, `lvt-keyup`, etc.
- **WebSocket/HTTP**: Automatic fallback to HTTP if WebSocket unavailable
- **Tree cache**: Statics cached in memory (sent once, reused forever)
- **morphdom**: Efficient DOM patching (minimal reflows)
- **Focus preservation**: Maintains cursor position and selection during updates
- **Loading indicators**: Top progress bar during WebSocket initialization
- **Form lifecycle**: Hooks for `pending`, `success`, `error`, `done` events
- **Rate limiting**: Built-in debounce/throttle for event handlers
- **Infinite scroll**: IntersectionObserver-based infinite scrolling

**Event Attributes:**
- `lvt-click="action"` - Click handler
- `lvt-submit="action"` - Form submission
- `lvt-change="action"` - Input change
- `lvt-keyup="action"` - Keyup handler
- `lvt-debounce="300"` - Debounce delay (ms)
- `lvt-throttle="500"` - Throttle delay (ms)

**Form Lifecycle Events:**
```javascript
form.addEventListener('lvt:pending', () => { /* Show loading */ });
form.addEventListener('lvt:success', () => { /* Clear form */ });
form.addEventListener('lvt:error', (e) => { /* Show errors */ });
form.addEventListener('lvt:done', () => { /* Hide loading */ });
```

**Focus Preservation:**
- Automatically preserves focus on input elements during updates
- Maintains cursor position and text selection
- Only applies to focusable input types (text, textarea, email, etc.)

**Loading Indicator:**
- Animated progress bar at top of page during WebSocket connection
- Automatically removed after first update
- Can be disabled via `WithLoadingDisabled()` option

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

## Broadcasting Architecture

LiveTemplate provides two types of broadcasting for real-time updates:

### 1. Automatic Session Syncing (Default)

**How it works:**
- Each browser gets a unique session group ID (via `livetemplate-id` cookie)
- All tabs in the same browser share this session group ID
- When any tab modifies state via an action, **all tabs in the same session group automatically receive updates**
- This happens with zero configuration or code changes

**Example:**
```go
type ChatState struct {
    Messages []Message
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    s.Messages = append(s.Messages, newMessage)
    return nil  // All tabs in same browser update automatically!
}
```

**Session Grouping for Anonymous Users:**
- Browser A, Tab 1: `groupID = session-abc` (from cookie)
- Browser A, Tab 2: `groupID = session-abc` (same cookie → same state, auto-sync)
- Browser B, Tab 1: `groupID = session-xyz` (different cookie → isolated state)

**Session Grouping for Authenticated Users:**
- User "alice", Desktop: `groupID = alice`
- User "alice", Mobile: `groupID = alice` (same user → auto-sync across devices!)
- User "bob", Desktop: `groupID = bob` (different user → isolated)

### 2. Manual Broadcasting

For cross-session scenarios, use the `LiveHandler` interface:

```go
tmpl := livetemplate.New("app")
handler := tmpl.Handle(&AppState{})  // Returns LiveHandler

// Broadcast to all connections (all browsers, all sessions)
handler.Broadcast(data)

// Broadcast to specific users across all their sessions
handler.BroadcastToUsers([]string{"user-123", "user-456"}, data)

// Broadcast to specific session group or topic
handler.BroadcastToGroup("topic:crypto-prices", data)
```

### Broadcasting Methods

#### Broadcast()
Sends updates to **all connected clients** across all session groups.

**Use Cases:**
- System-wide announcements
- Global data updates (stock prices, weather)
- Admin broadcasts

**Example:**
```go
// Background goroutine pushes live data
go func() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        data := fetchLatestData()
        handler.Broadcast(data)
    }
}()
```

#### BroadcastToUsers()
Sends updates to **specific users** across all their active connections.

**Use Cases:**
- User-specific notifications
- Multi-device updates
- Targeted messaging

**Example:**
```go
notification := &Notification{Message: "You have a new message"}
handler.BroadcastToUsers([]string{"alice", "bob"}, notification)
```

#### BroadcastToGroup()
Sends updates to **all connections in a session group**.

**Use Cases:**
- Pub/sub topics (e.g., `"topic:crypto-prices"`)
- Chat rooms (e.g., `"room:lobby"`)
- Collaborative workspaces (e.g., `"workspace:123"`)

**Example:**
```go
// Custom authenticator for topic subscriptions
type TopicAuthenticator struct{}

func (a *TopicAuthenticator) GetSessionGroup(r *http.Request, userID string) (string, error) {
    topic := r.URL.Query().Get("topic")
    return "topic:" + topic, nil
}

// Publish to all subscribers
handler.BroadcastToGroup("topic:crypto-prices", priceUpdate)
```

### Server-Initiated Updates (BroadcastAware)

For stores that need background updates, implement the `BroadcastAware` interface:

```go
type LiveDataStore struct {
    Data       string
    broadcaster Broadcaster
    stopCh     chan struct{}
}

func (s *LiveDataStore) OnConnect(ctx context.Context, b Broadcaster) error {
    s.broadcaster = b
    s.stopCh = make(chan struct{})

    // Start background updates for this connection
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                s.Data = fetchLatestData()
                s.broadcaster.Send()  // Push to this specific connection
            case <-s.stopCh:
                return
            }
        }
    }()

    return nil
}

func (s *LiveDataStore) OnDisconnect() {
    close(s.stopCh)  // Stop background updates
}
```

**Key Differences:**
- `Broadcaster.Send()`: Updates **one specific connection** (per-connection state)
- `LiveHandler.Broadcast()`: Updates **all connections** (shared state)

**Use Cases for BroadcastAware:**
- Per-user live feeds (different data per connection)
- Background job status (connection-specific)
- Real-time notifications (per-connection)

### Broadcasting Performance

**Tree Diffing Per Connection:**
Each connection maintains independent template state:
- Connection A: `lastData = {Count: 5}`
- Connection B: `lastData = {Count: 10}`
- Broadcast `{Count: 15}` → Different tree diffs for each connection

**Frequency Guidelines:**
- **High frequency** (<100ms): Use only for critical real-time data
- **Medium frequency** (1-5s): Suitable for most live updates
- **Low frequency** (>5s): Recommended for background sync

**Thread Safety:**
All broadcasting methods are thread-safe and can be called concurrently from multiple goroutines.

**Error Handling:**
Broadcasting continues even if individual sends fail. Check logs for details.

### Broadcasting Examples

See the complete chat application in `examples/chat/` demonstrating:
- Message broadcasting to all users
- User presence tracking
- Multi-tab session sharing

For detailed documentation, see [BROADCASTING.md](BROADCASTING.md).

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
- **Multi-tab sync:** WebSocket required for automatic session syncing

### Why Session Groups Instead of Per-Connection State?

- **Multi-tab sync:** Tabs in same browser automatically share state
- **Multi-device sync:** Authenticated users can sync across devices
- **Flexibility:** Custom groupID mapping enables pub/sub, rooms, workspaces
- **Efficiency:** One Stores instance per group, not per connection
- **Broadcasting:** Group-based targeting is natural and efficient

**Trade-offs:**
- Requires connection tracking (ConnectionRegistry)
- Session group management overhead (SessionStore)
- Memory usage scales with active groups, not connections

### Why Dual-Indexed Connection Registry?

- **Efficient broadcasting:** O(1) lookup by groupID or userID
- **Multi-tab support:** Quick access to all tabs in a session
- **User targeting:** Broadcast to all devices for a user
- **Memory overhead:** Minimal (two maps with pointers, no data duplication)

Alternative approaches considered:
- Single map by groupID only: Can't efficiently target specific users
- Single map by userID only: Can't efficiently target session groups
- No indexing: O(n) scan for every broadcast operation

### Why Authenticator Interface?

- **Flexibility:** Support anonymous, authenticated, and custom auth flows
- **Separation of concerns:** Auth logic separate from framework
- **Session group mapping:** Decouples user identity from state grouping
- **Testability:** Easy to mock for testing
- **Extensibility:** Users can implement JWT, OAuth, custom logic

**Default (AnonymousAuthenticator):**
- Zero configuration required
- Persistent browser-based sessions via cookie
- Multi-tab support out of the box

### Why morphdom for DOM Updates?

- **Efficiency:** Only patches changed DOM nodes (minimal reflows)
- **Focus preservation:** Automatically maintains form state
- **Proven:** Battle-tested in Turbo/LiveView ecosystems
- **Small:** Minimal bundle size (~2KB gzipped)

Alternative considered: Direct DOM manipulation (more complex, harder to maintain focus)

### Why Loading Indicators by Default?

- **User feedback:** Visual indication that page is interactive
- **Perceived performance:** Users tolerate delays better with feedback
- **Professional UX:** Matches expectations from modern web apps
- **Optional:** Can be disabled with `WithLoadingDisabled()`

## Future Considerations

Potential improvements for future versions:

1. **Redis SessionStore:** For multi-instance deployments with shared state
2. **Streaming Updates:** For large lists, stream tree updates incrementally
3. **Partial Hydration:** Only hydrate changed subtrees
4. **Advanced Diffing:** More sophisticated algorithms for complex trees
5. **Client-side Optimizations:** Request coalescing, virtual scrolling
6. **Server-side Caching:** Cache compiled constructs across requests
7. **Presence Tracking:** Built-in user presence system (online/offline status)
8. **Binary Protocol:** More efficient serialization than JSON for high-frequency updates

---

## Related Documentation

For more details on specific topics, see:

- **Core Architecture:**
  - [CLAUDE.md](../CLAUDE.md) - Development guidelines and project overview
  - [README.md](../README.md) - Quick start and examples

- **Formal Specifications:**
  - [specifications/tree-update-specification.md](specifications/tree-update-specification.md) - **Formal tree update specification**
  - [specifications/test-scenarios.md](specifications/test-scenarios.md) - **Comprehensive test scenarios**
  - [references/template-support-matrix.md](references/template-support-matrix.md) - Template pattern support

- **Broadcasting & Real-time:**
  - [BROADCASTING.md](BROADCASTING.md) - Comprehensive broadcasting guide
  - [examples/chat/](../examples/chat/) - Multi-user chat example

- **Testing:**
  - `tree_update_fuzz_test.go` - User activity fuzz testing framework
  - `tree_analyzer_enhanced.go` - Specification compliance analyzer
  - `e2e_update_spec_test.go` - Integration test suite

- **CLI Tool:**
  - [cmd/lvt/README.md](../cmd/lvt/README.md) - CLI tool documentation
  - [guides/kit-development.md](guides/kit-development.md) - Creating custom kits
  - [guides/user-guide.md](guides/user-guide.md) - User guide for lvt

- **API Reference:**
  - [references/api-reference.md](references/api-reference.md) - Complete API documentation

- **Design Documents:**
  - [design/IMPLEMENTATION_STATUS.md](design/IMPLEMENTATION_STATUS.md) - Implementation status
  - [proposals/lvt-bind-proposal.md](proposals/lvt-bind-proposal.md) - Data binding proposal
