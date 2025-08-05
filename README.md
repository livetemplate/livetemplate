# StateTemplate - Real-time Go Template Rendering Library

StateTemplate is a high-performance Go library for real-time HTML template rendering with granular fragment updates. It enables live updates to specific parts of rendered templates without full page reloads, making it ideal for building responsive web applications with WebSocket integration.

## 🚀 Quick Start

```bash
# Clone and setup
git clone <repository-url>
cd statetemplate

# Run tests (no bash scripts needed)
go test -v

# Run examples
go run examples/simple/main.go
go run examples/files/main.go
go run examples/fragments/main.go
go run examples/e2e/main.go
```

## Features

- **Fragment-based Updates**: Extract and track template fragments for granular updates
- **Real-time Rendering**: WebSocket-compatible updates with minimal payloads
- **Change Detection**: Efficient data monitoring through reflection-based tracking
- **Template Composition**: Support for blocks, conditionals, ranges, and nested templates
- **Performance Optimized**: Fragment caching and batch updates for high throughput
- **Type Safety**: Full Go type system integration with comprehensive error handling

## How It Works

1. **Template Registration**: Register templates with automatic fragment extraction
2. **Fragment Analysis**: System categorizes fragments (simple, conditional, range, block)
3. **Dependency Tracking**: Map data dependencies to specific template fragments
4. **Real-time Updates**: Monitor data changes and generate minimal update payloads
5. **WebSocket Integration**: Send targeted fragment updates to connected clients

## Architecture

The library consists of four main components:

- **Renderer**: Main orchestrator managing template parsing and real-time updates
- **TemplateTracker**: Monitors data dependencies and detects changes using reflection
- **FragmentExtractor**: Extracts and categorizes template fragments for granular updates
- **TemplateAnalyzer**: Provides advanced template analysis and optimization

For detailed architecture documentation, see [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Usage

### Basic Real-time Rendering

```go
package main

import (
    "html/template"
    "github.com/livefir/statetemplate"
)

func main() {
    // Create renderer
    tmpl := template.Must(template.New("example").Parse(`
        <div>
            <h1>{{.Title}}</h1>
            <p>Welcome, {{.User.Name}}!</p>
            {{range .Items}}
                <span>{{.}}</span>
            {{end}}
        </div>
    `))

    config := statetemplate.Config{
        WrapperTagName: "div",
        IDPrefix:       "fragment-",
    }

    renderer := statetemplate.NewRenderer("main", tmpl, config)

    // Initial render
    data := struct {
        Title string
        User  struct { Name string }
        Items []string
    }{
        Title: "My App",
        User:  struct{ Name string }{Name: "John"},
        Items: []string{"item1", "item2"},
    }

    // Create StateTemplate instance
    st := statetemplate.New(tmpl,
        statetemplate.WithExpiration(24*time.Hour),
    )

    // For initial page load - get full HTML with session info
    initial, err := st.NewSession(context.Background(), data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Initial HTML: %s\n", initial.HTML)
    fmt.Printf("Session ID: %s\n", initial.SessionID)

    // For real-time updates - get existing session and stream updates
    session, err := st.GetSession(initial.SessionID, initial.Token)
    if err != nil {
        panic(err)
    }
    defer session.Close()

    // Listen for real-time fragment updates
    go func() {
        for update := range session.Updates() {
            // Send to WebSocket clients
            fmt.Printf("Fragment %s: %s\n", update.FragmentID, update.HTML)
        }
    }()

    // Trigger updates with new data
    newData := struct {
        Title string
        User  struct { Name string }
        Items []string
    }{
        Title: "Updated App",
        User:  struct{ Name string }{Name: "Jane"},
        Items: []string{"item1", "item2", "item3"},
    }

    // This sends fragment updates to session.Updates() channel
    if err := session.Render(context.Background(), newData); err != nil {
        panic(err)
    }
}
```

### Fragment Extraction and Tracking

StateTemplate automatically extracts different types of fragments:

```go
// Simple fragments: {{.Field}}
renderer.ExtractSimpleFragments(template)

// Conditional fragments: {{if .Condition}}...{{end}}
renderer.ExtractConditionalFragments(template)

// Range fragments: {{range .Items}}...{{end}}
renderer.ExtractRangeFragments(template)

// Block fragments: {{block "name" .}}...{{end}}
renderer.ExtractBlockFragments(template)
```

## API Reference

### Core Types

#### `StateTemplate`

The main entry point that manages sessions and template rendering.

#### `Session`

Represents an isolated rendering session for a single client.

#### `Config`

```go
type Config struct {
    Expiration      time.Duration // Session expiration (default: 24h)
    Store           sessionStore  // Session storage backend
    SigningKey      []byte        // For secure session tokens
    MaxSessions     int           // Maximum concurrent sessions
    CleanupInterval time.Duration // Cleanup frequency (default: 5m)
}
```

#### `Update`

```go
type Update struct {
    FragmentID string    `json:"fragment_id"`
    HTML       string    `json:"html"`
    Action     string    `json:"action"` // "replace", "append", "remove"
    Timestamp  time.Time `json:"timestamp"`
}
```

#### `InitialRender`

```go
type InitialRender struct {
    HTML      string `json:"html"`       // Full annotated HTML
    SessionID string `json:"session_id"` // For subsequent updates
    Token     string `json:"token"`      // For session authentication
}
```

### Key Methods

#### `New(templates *template.Template, options ...Option) *StateTemplate`

Creates a new StateTemplate instance with configuration options.

#### `NewSession(ctx context.Context, data interface{}) (*InitialRender, error)`

Creates a new session and renders the complete template with initial data. Use this for initial page loads.

#### `GetSession(sessionID, token string) (*Session, error)`

Retrieves an existing session for real-time updates. Use this for WebSocket connections.

#### `Render(ctx context.Context, data interface{}) error`

Processes new data and triggers fragment updates. Updates are sent to the `Updates()` channel.

#### `Updates() <-chan Update`

Returns a channel that streams incremental fragment updates. Use this for real-time connections.

#### `Close() error`

Closes the session and releases resources.

## Usage Patterns

### Pattern 1: Initial Page Load (HTTP Handler)

Use `NewSession()` for server-side rendering of the initial page:

```go
func pageHandler(w http.ResponseWriter, r *http.Request) {
    st := statetemplate.New(templates,
        statetemplate.WithExpiration(24*time.Hour),
    )

    data := getPageData(r)

    // Get full HTML for initial page load
    initial, err := st.NewSession(ctx, data)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // Render page template with session info for JavaScript
    pageTemplate.Execute(w, struct {
        Content   string
        SessionID string
        Token     string
    }{
        Content:   initial.HTML,
        SessionID: initial.SessionID,
        Token:     initial.Token,
    })
}
```

### Pattern 2: Real-time Updates (WebSocket Handler)

Use `GetSession()` + `Updates()` for incremental updates:

```go
func websocketHandler(w http.ResponseWriter, r *http.Request) {
    sessionID := r.URL.Query().Get("session_id")
    token := r.URL.Query().Get("token")

    // Get existing session from initial page load
    session, err := st.GetSession(sessionID, token)
    if err != nil {
        http.Error(w, "Invalid session", 401)
        return
    }
    defer session.Close()

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    // Stream incremental updates
    go func() {
        for update := range session.Updates() {
            conn.WriteJSON(update)
        }
    }()

    // Process data changes
    for {
        var newData PageData
        if err := conn.ReadJSON(&newData); err != nil {
            break
        }

        // Trigger fragment updates
        if err := session.Render(ctx, &newData); err != nil {
            log.Printf("Render error: %v", err)
        }
    }
}
```

### Fragment Types

StateTemplate supports four types of template fragments:

#### Simple Fragments

- Direct field access: `{{.Field}}`
- Single data dependency with straightforward updates

#### Conditional Fragments

- If/with blocks: `{{if .Condition}}...{{end}}`
- May appear or disappear based on data conditions

#### Range Fragments

- Loop constructs: `{{range .Items}}...{{end}}`
- Granular item-level tracking for additions, removals, reordering

#### Block Fragments

- Named template sections: `{{block "name" .}}...{{end}}`
- Template composition and inheritance support

#### `UpdateTemplateTracker(tt *TemplateTracker, name string, tmpl *template.Template)`

Registers a template using advanced AST analysis.

## WebSocket Integration

For real-time web applications, StateTemplate generates WebSocket-compatible updates:

```go
// Set up WebSocket handler
func handleWebSocket(conn *websocket.Conn) {
    // Create renderer and channels
    renderer := statetemplate.NewRenderer("main", template, config)
    updateChan := make(chan statetemplate.DataUpdate)
    fragmentChan := make(chan statetemplate.Update)

    go renderer.StartUpdates(updateChan, fragmentChan)

    // Forward real-time updates to WebSocket
    go func() {
        for update := range fragmentChan {
            conn.WriteJSON(map[string]interface{}{
                "type":       "fragment_update",
                "fragmentId": update.FragmentID,
                "html":       update.HTML,
                "action":     update.Action,
            })
        }
    }()
}
```

For comprehensive documentation and examples, see [`docs/EXAMPLES.md`](docs/EXAMPLES.md).

## Example Data Structures

```go
type User struct {
    ID    int
    Name  string
    Email string
}

type AppData struct {
    Title       string
    CurrentUser *User
    UserCount   int
    Articles    []*Article
}
```

## Template Examples

### Header Template

```html
<header>
  <h1>{{.Title}}</h1>
  {{if .CurrentUser}}
  <p>Welcome, {{.CurrentUser.Name}}!</p>
  {{end}}
</header>
```

**Dependencies**: `Title`, `CurrentUser`, `CurrentUser.Name`

### Sidebar Template

```html
<aside>
  <p>Users: {{.UserCount}}</p>
  {{if .CurrentUser}}
  <p>Your ID: {{.CurrentUser.ID}}</p>
  {{end}}
</aside>
```

**Dependencies**: `UserCount`, `CurrentUser`, `CurrentUser.ID`

## Change Detection

The system performs deep comparison of data structures:

- **Primitive fields**: Direct value comparison
- **Struct fields**: Recursive field-by-field comparison
- **Pointer fields**: Handles nil pointers correctly
- **Nested structures**: Tracks changes at any depth

## Performance Considerations

- **Efficient Comparison**: Only compares fields that templates actually use
- **Minimal Re-rendering**: Only notifies about templates that need updates
- **Memory Efficient**: Doesn't store large data structures, just tracks changes
- **Concurrent Safe**: Thread-safe operations with proper synchronization

## Testing

Run the comprehensive test suite:

```bash
# Run all tests
go test -v

# Run specific test suites
go test -v -run "TestRenderer"
go test -v -run "TestTemplateTracker"
go test -v -run "TestFragmentExtractor"

# Run examples with timeout
timeout 3s go run examples/simple/main.go
timeout 3s go run examples/e2e/main.go
```

The test suite uses table-driven tests for comprehensive coverage of template actions and fragment types.

## Use Cases

- **Real-time Web Applications**: Live UI updates with WebSocket integration
- **Progressive Web Apps**: Efficient fragment-based updates for smooth UX
- **Live Dashboards**: Real-time data visualization with minimal bandwidth
- **Chat Applications**: Message and user state updates with granular control
- **E-commerce Sites**: Dynamic cart, inventory, and pricing updates
- **Content Management**: Live preview and collaborative editing features

## Project Structure

```text
statetemplate/
├── renderer.go                # Main renderer orchestrator
├── template_tracker.go        # Data change tracking
├── fragment_extractor.go      # Fragment extraction and categorization
├── template_analyzer.go       # Advanced template analysis
├── examples/                  # Usage examples and demos
├── docs/                      # Comprehensive documentation
├── scripts/                   # Development and validation scripts
└── testdata/                  # Test templates and data
```

## Integration

StateTemplate integrates seamlessly with:

- **WebSocket connections** for real-time bi-directional updates
- **HTTP Server-Sent Events (SSE)** for live streaming
- **Message queues** (Redis, RabbitMQ) for distributed updates
- **Database change notifications** (PostgreSQL LISTEN/NOTIFY)
- **File system watchers** for development and content updates

The library provides the foundation for building efficient, real-time web applications with minimal client-side complexity and optimal bandwidth usage.

## Documentation

- [`docs/SESSION_DESIGN.md`](docs/SESSION_DESIGN.md) - Session-based architecture design and implementation
- [`docs/SESSION_IMPLEMENTATION.md`](docs/SESSION_IMPLEMENTATION.md) - Session API implementation guide
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) - Detailed architectural overview
- [`docs/EXAMPLES.md`](docs/EXAMPLES.md) - WebSocket integration guide
- [`examples/README.md`](examples/README.md) - Usage examples and patterns
