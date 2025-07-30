# State Template - Live Template Update System

This Go package provides a sophisticated system for tracking dependencies between HTML templates and data structures, and efficiently determining which templates need re-rendering when data changes.

## Features

- **Automatic Dependency Detection**: Analyzes Go HTML templates to extract field dependencies
- **Change Detection**: Compares data structures to detect which fields have changed
- **Efficient Re-rendering**: Only re-renders templates that depend on changed data fields
- **Real-time Updates**: Processes data updates through channels for live applications
- **Advanced AST Analysis**: Uses template AST parsing for accurate dependency tracking

## How It Works

1. **Template Registration**: Register your HTML templates with the system
2. **Dependency Analysis**: The system analyzes each template to determine which data fields it depends on
3. **Data Updates**: Send data updates through a channel
4. **Change Detection**: The system compares new data with previous data to detect changes
5. **Template Notification**: Receive notifications about which templates need re-rendering

## Usage

### Basic Setup

```go
package main

import (
    "html/template"
    "github.com/livefir/statetemplate"
)

func main() {
    // Create template tracker
    tracker := statetemplate.NewTemplateTracker()
    
    // Define your template
    tmpl := template.Must(template.New("example").Parse(`
        <div>
            <h1>{{.Title}}</h1>
            <p>Welcome, {{.User.Name}}!</p>
        </div>
    `))
    
    // Register template
    tracker.AddTemplate("example", tmpl)
    
    // Set up channels
    dataChannel := make(chan statetemplate.DataUpdate)
    updateChannel := make(chan statetemplate.TemplateUpdate)
    
    // Start live update processor
    go tracker.StartLiveUpdates(dataChannel, updateChannel)
    
    // Handle updates
    go func() {
        for update := range updateChannel {
            fmt.Printf("Re-render templates: %v\n", update.TemplateNames)
            fmt.Printf("Changed fields: %v\n", update.ChangedFields)
        }
    }()
    
    // Send data updates
    dataChannel <- statetemplate.DataUpdate{
        Data: struct {
            Title string
            User  struct { Name string }
        }{
            Title: "My App",
            User:  struct{ Name string }{Name: "John"},
        },
    }
}
```

### Advanced Usage with AST Analysis

For more accurate dependency detection, use the advanced analyzer:

```go
// Create advanced analyzer
analyzer := statetemplate.NewAdvancedTemplateAnalyzer()

// Use advanced analysis when adding templates
analyzer.UpdateTemplateTracker(tracker, "advanced", template)
```

## API Reference

### Core Types

#### `TemplateTracker`
The main component that manages templates and tracks dependencies.

#### `DataUpdate`
```go
type DataUpdate struct {
    Data interface{}
}
```

#### `TemplateUpdate`
```go
type TemplateUpdate struct {
    TemplateNames []string
    ChangedFields []string
}
```

### Key Methods

#### `NewTemplateTracker() *TemplateTracker`
Creates a new template tracker.

#### `AddTemplate(name string, tmpl *template.Template)`
Registers a template with basic dependency analysis.

#### `StartLiveUpdates(dataChannel <-chan DataUpdate, updateChannel chan<- TemplateUpdate)`
Starts processing data updates and sending template update notifications.

### Advanced Features

#### `AdvancedTemplateAnalyzer`
Provides sophisticated template AST analysis for more accurate dependency detection.

#### `UpdateTemplateTracker(tt *TemplateTracker, name string, tmpl *template.Template)`
Registers a template using advanced AST analysis.

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

## Running the Demo

### Option 1: Run the executable demo
```bash
cd /Users/adnaan/code/livefir/statetemplate
go run cmd/demo/main.go
```

### Option 2: Run examples
```bash
# Run the comprehensive example
go run examples/example.go
```

### Option 3: Run tests
```bash
go test -v
```

This will run a complete demonstration showing:
1. Template dependency detection
2. Data updates simulation
3. Real-time template update notifications

## Testing

Run the test suite:

```bash
go test -v
```

## Use Cases

- **Live Web Applications**: Real-time UI updates based on data changes
- **Server-Side Rendering**: Efficient partial page updates
- **Progressive Web Apps**: Optimized template rendering
- **Real-time Dashboards**: Live data visualization updates
- **Chat Applications**: Message and user state updates

## Integration

This system can be integrated with:
- WebSocket connections for real-time updates
- HTTP SSE (Server-Sent Events) for live streaming
- Message queues for distributed updates
- Database change notifications
- File system watchers

The system is designed to be the foundation for building efficient, real-time web applications with minimal unnecessary re-rendering.
