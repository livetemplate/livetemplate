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
go run examples/realtime/main.go
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

- **RealtimeRenderer**: Main orchestrator managing template parsing and real-time updates
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
    // Create realtime renderer
    tmpl := template.Must(template.New("example").Parse(`
        <div>
            <h1>{{.Title}}</h1>
            <p>Welcome, {{.User.Name}}!</p>
            {{range .Items}}
                <span>{{.}}</span>
            {{end}}
        </div>
    `))

    config := statetemplate.RealtimeConfig{
        WrapperTagName: "div",
        IDPrefix:       "fragment-",
    }

    renderer := statetemplate.NewRealtimeRenderer("main", tmpl, config)

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

    initialHTML, err := renderer.ProcessInitialData(data)
    if err != nil {
        panic(err)
    }

    // Set up real-time updates
    updateChan := make(chan statetemplate.DataUpdate)
    realtimeChan := make(chan statetemplate.RealtimeUpdate)

    go renderer.StartRealtimeUpdates(updateChan, realtimeChan)

    // Handle real-time updates (for WebSocket)
    go func() {
        for update := range realtimeChan {
            // Send to WebSocket clients
            fmt.Printf("Fragment %s: %s\n", update.FragmentID, update.HTML)
        }
    }()

    // Send data updates
    updateChan <- statetemplate.DataUpdate{
        Data: struct {
            Title string
            User  struct { Name string }
            Items []string
        }{
            Title: "Updated App",
            User:  struct{ Name string }{Name: "Jane"},
            Items: []string{"item1", "item2", "item3"},
        },
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

#### `RealtimeRenderer`

The main component that orchestrates template parsing, fragment extraction, and real-time updates.

#### `RealtimeConfig`

```go
type RealtimeConfig struct {
    WrapperTagName string // HTML tag for fragment wrapping
    IDPrefix       string // Prefix for generated fragment IDs
}
```

#### `RealtimeUpdate`

```go
type RealtimeUpdate struct {
    FragmentID string // Unique fragment identifier
    HTML       string // Updated HTML content
    Action     string // Update action (replace, append, remove)
}
```

#### `DataUpdate`

```go
type DataUpdate struct {
    Data interface{} // New data state
}
```

### Key Methods

#### `NewRealtimeRenderer(name string, tmpl *template.Template, config RealtimeConfig) *RealtimeRenderer`

Creates a new realtime renderer with fragment extraction and tracking.

#### `ProcessInitialData(data interface{}) (string, error)`

Renders the complete template with initial data and sets up fragment tracking.

#### `StartRealtimeUpdates(updateChan <-chan DataUpdate, realtimeChan chan<- RealtimeUpdate)`

Starts processing data updates and generating minimal fragment updates for real-time synchronization.

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
    renderer := statetemplate.NewRealtimeRenderer("main", template, config)
    updateChan := make(chan statetemplate.DataUpdate)
    realtimeChan := make(chan statetemplate.RealtimeUpdate)

    go renderer.StartRealtimeUpdates(updateChan, realtimeChan)

    // Forward real-time updates to WebSocket
    go func() {
        for update := range realtimeChan {
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

For comprehensive documentation and examples, see [`docs/REALTIME.md`](docs/REALTIME.md).

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
go test -v -run "TestRealtimeRenderer"
go test -v -run "TestTemplateTracker"
go test -v -run "TestFragmentExtractor"

# Run examples with timeout
timeout 3s go run examples/simple/main.go
timeout 3s go run examples/realtime/main.go
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
├── realtime_renderer.go       # Main renderer orchestrator
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

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) - Detailed architectural overview
- [`docs/REALTIME.md`](docs/REALTIME.md) - WebSocket integration guide
- [`docs/ENHANCED_INTERFACE_IMPLEMENTATION_SUMMARY.md`](docs/ENHANCED_INTERFACE_IMPLEMENTATION_SUMMARY.md) - Implementation details
- [`examples/README.md`](examples/README.md) - Usage examples and patterns
