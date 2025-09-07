# LiveTemplate API Reference

## Overview

LiveTemplate provides a secure, multi-tenant API for ultra-efficient HTML template rendering with **tree-based optimization**. The library generates minimal update structures similar to Phoenix LiveView, achieving 92%+ bandwidth savings through intelligent static/dynamic content separation.

> **Current Version**: Multi-tenant architecture with Application/Page API and session-based authentication

## Core Architecture

### Application/Page Model
- **Applications** provide multi-tenant isolation with session-based authentication
- **Pages** are template instances within applications with tree-based optimization  
- **Data Models** with action methods for clean business logic separation
- **Tree-based strategy** handles all template patterns automatically

## Public API

### Application Management

#### `NewApplication() (*Application, error)`

Creates a new isolated application instance with session-based authentication.

```go
app, err := livetemplate.NewApplication()
if err != nil {
    log.Fatal(err)
}
defer app.Close()
```

#### `Application.ParseFiles(filenames ...string) (*template.Template, error)`

Parses template files and automatically registers them for reuse. Template name is derived from first filename without extension.

```go
// Registers template as "index" (filename without extension)
_, err := app.ParseFiles("templates/index.html")
if err != nil {
    log.Fatal(err)
}
```

#### `Application.NewPage(templateName string, data interface{}) (*ApplicationPage, error)`

Creates a new page instance using a registered template.

```go
page, err := app.NewPage("index", initialData)
if err != nil {
    log.Fatal(err)
}
```

#### `Application.GetPage(r *http.Request) (*ApplicationPage, error)`

Retrieves a page from HTTP request with session authentication.

```go
page, err := app.GetPage(r)
if err != nil {
    log.Printf("Authentication failed: %v", err)
    return
}
```

### Page Operations

#### `ApplicationPage.ServeHTTP(w http.ResponseWriter, data interface{}) error`

Renders and serves the complete HTML page with the provided data.

```go
data := map[string]interface{}{"Counter": 42, "Color": "#ff6b6b"}
err := page.ServeHTTP(w, data)
if err != nil {
    log.Printf("Serve failed: %v", err)
    return
}
```

#### `ApplicationPage.GetToken() string`

Returns the session token for the page (used for WebSocket authentication).

```go
token := page.GetToken()
fmt.Printf("Page token: %s", token)
```

#### `ApplicationPage.RegisterDataModel(model interface{}) error`

Registers a data model with action methods for handling user interactions.

```go
type Counter struct {
    Value int `json:"Counter"`
}

func (c *Counter) Increment(ctx *livetemplate.ActionContext) error {
    c.Value++
    return ctx.Data(map[string]interface{}{"Counter": c.Value})
}

// Register the model
err := page.RegisterDataModel(counter)
if err != nil {
    log.Fatal(err)
}
```

#### `Application.HandleAction(r *http.Request) error`

Processes action requests from client (typically called via WebSocket).

```go
// WebSocket message handling
for {
    var actionMsg livetemplate.ActionMessage
    if err := conn.ReadJSON(&actionMsg); err != nil {
        break
    }
    
    if err := app.HandleAction(r); err != nil {
        log.Printf("Action failed: %v", err)
    }
}
```

#### `Application.Close() error`

Releases all application resources and performs cleanup.

```go
err := app.Close()
if err != nil {
    log.Printf("Cleanup failed: %v", err)
}
```

## Data Types

### Fragment

Represents an update fragment with tree-based optimization data.

```go
type Fragment struct {
    ID       string            `json:"id"`       // Unique fragment identifier
    Data     interface{}       `json:"data"`     // Tree structure (SimpleTreeData)
    Metadata *FragmentMetadata `json:"metadata,omitempty"` // Performance info
}
```

### FragmentMetadata

Contains performance and optimization information.

```go
type FragmentMetadata struct {
    GenerationTime   time.Duration `json:"generation_time"`
    OriginalSize     int           `json:"original_size"`
    CompressedSize   int           `json:"compressed_size"`
    CompressionRatio float64       `json:"compression_ratio"`
    Strategy         int           `json:"strategy_number"`
    Confidence       float64       `json:"confidence"`
    FallbackUsed     bool          `json:"fallback_used"`
}
```

### ActionContext

Provides context for data model action methods.

```go
type ActionContext struct {
    // Contains request context and response handling
}

// Set response data for the action
func (ctx *ActionContext) Data(data interface{}) error
```

### ActionMessage

Represents an action message from the client.

```go
type ActionMessage struct {
    Action string                 `json:"action"`
    Data   map[string]interface{} `json:"data,omitempty"`
}
```

## Tree-Based Optimization

LiveTemplate uses **tree-based optimization** - a single unified strategy that adapts to all template patterns, achieving 92%+ bandwidth savings through intelligent static/dynamic content separation.

### Single Strategy: Tree-Based (92%+ reduction)

**For all template patterns**: Static content cached client-side, only dynamic values transmitted

```go
// Fragment.Data contains tree structure:
// {
//   "s": ["<p>Hello ", "!</p>"],  // Static segments (cached client-side)
//   "0": "Alice"                   // Dynamic field 0
// }
```

### Tree Structure Examples

**Simple Field Template**:
```html
<p>Hello {{.Name}}!</p>
```
**Generated Fragment Data**:
```json
{
  "s": ["<p>Hello ", "!</p>"],
  "0": "Alice"
}
```

**Conditional Template**:
```html
<div>{{if .Show}}Welcome {{.Name}}!{{end}}</div>
```
**Generated Fragment Data (Show=true)**:
```json
{
  "s": ["<div>", "</div>"],
  "0": {
    "s": ["Welcome ", "!"],
    "0": "John"
  }
}
```

**Range Template**:
```html
<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>
```
**Generated Fragment Data**:
```json
{
  "s": ["<ul>", "</ul>"],
  "0": [
    {"s": ["<li>", "</li>"], "0": "Item A"},
    {"s": ["<li>", "</li>"], "0": "Item B"},
    {"s": ["<li>", "</li>"], "0": "Item C"}
  ]
}
```

### Template Construct Support
- **Simple Fields**: `{{.Name}}` - Direct value substitution
- **Conditionals**: `{{if .Active}}...{{else}}...{{end}}` - Branch selection  
- **Ranges**: `{{range .Items}}...{{end}}` - List iteration with individual tracking
- **Nested Structures**: Complex combinations with proper hierarchical parsing
- **Static Content**: Preserved and cached client-side for maximum efficiency

## Performance Characteristics

### Performance Results
- **Tree-based optimization**: 92%+ bandwidth savings for all template patterns
- **Generation speed**: Sub-millisecond performance for typical updates
- **Consistency**: Same high performance across all template constructs
- **Memory efficiency**: Minimal allocation with structure reuse

### Memory Management
- **Lightweight applications**: Long-lived service instances
- **Session cleanup**: Automatic TTL-based expiration
- **Tree caching**: Minimal overhead with shared static segments
- **No memory leaks**: Comprehensive resource cleanup

### Optimization Benefits
- **Single strategy**: No complex selection logic overhead
- **Client-side caching**: Static content cached, only dynamics transmitted
- **Phoenix LiveView compatible**: Client structures mirror LiveView format
- **Predictable performance**: Consistent behavior across all template patterns

## Usage Patterns

### Basic HTTP Handler
```go
func pageHandler(w http.ResponseWriter, r *http.Request) {
    // Create application (typically done once at startup)
    app, err := livetemplate.NewApplication()
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    defer app.Close()
    
    // Parse and register template
    _, err = app.ParseFiles("template.html")
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    // Create page and serve
    page, err := app.NewPage("template", getPageData(r))
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    err = page.ServeHTTP(w, getPageData(r))
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
}
```

### WebSocket Real-time Updates
```go
func websocketHandler(app *livetemplate.Application, w http.ResponseWriter, r *http.Request) {
    // Get page from request (handles authentication)
    page, err := app.GetPage(r)
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    
    // Handle action messages
    for {
        var actionMsg livetemplate.ActionMessage
        if err := conn.ReadJSON(&actionMsg); err != nil {
            break
        }
        
        // Process action (triggers data model methods)
        if err := app.HandleAction(r); err != nil {
            log.Printf("Action failed: %v", err)
            continue
        }
        
        // Response fragments are automatically sent to client
    }
}
```

### Data Model with Actions
```go
type Counter struct {
    Value int `json:"Counter"`
    mu    sync.RWMutex
}

// Action method - automatically registered
func (c *Counter) Increment(ctx *livetemplate.ActionContext) error {
    c.mu.Lock()
    c.Value++
    c.mu.Unlock()
    
    // Return updated data
    return ctx.Data(map[string]interface{}{
        "Counter": c.Value,
    })
}

// Register the model
page.RegisterDataModel(counter)
```

## Error Handling

### Common Error Scenarios
- `"failed to create application"` - Application creation error
- `"template not found"` - Template name not registered
- `"action not found"` - Action method not found on data model
- `"session not found"` - Invalid or expired session token

### Best Practices
- Always check errors from `NewApplication()` and other operations
- Use `defer app.Close()` to ensure resource cleanup
- Register templates once at startup with `ParseFiles()`
- Use data models with action methods for clean separation
- Handle session authentication errors gracefully

## Concurrency

### Thread Safety
- All public API methods are **thread-safe**
- Multiple goroutines can safely access the same application/page
- Session management is thread-safe with proper synchronization
- Data model action methods should handle their own synchronization

### Performance Considerations
- Application instances should be long-lived (one per service)
- Pages are lightweight - create as needed
- Tree generation is optimized for high throughput
- Session cleanup is automatic with TTL
- Concurrent access is optimized with read-write mutexes

## Testing

### Unit Testing with LiveTemplate
```go
func TestPageRendering(t *testing.T) {
    app, err := livetemplate.NewApplication()
    if err != nil {
        t.Fatalf("Failed to create app: %v", err)
    }
    defer app.Close()
    
    // Create template file for testing
    tmpFile, _ := os.CreateTemp("", "test*.html")
    tmpFile.WriteString(`<p>{{.Name}}</p>`)
    tmpFile.Close()
    defer os.Remove(tmpFile.Name())
    
    _, err = app.ParseFiles(tmpFile.Name())
    if err != nil {
        t.Fatalf("Failed to parse template: %v", err)
    }
    
    data := map[string]interface{}{"Name": "Test"}
    page, err := app.NewPage(filepath.Base(tmpFile.Name()), data)
    if err != nil {
        t.Fatalf("Failed to create page: %v", err)
    }
    
    // Test would verify ServeHTTP output
}
```

### Action Testing
```go
type TestCounter struct {
    Value int `json:"Counter"`
}

func (c *TestCounter) Increment(ctx *livetemplate.ActionContext) error {
    c.Value++
    return ctx.Data(map[string]interface{}{"Counter": c.Value})
}

func TestActionHandling(t *testing.T) {
    app, err := livetemplate.NewApplication()
    if err != nil {
        t.Fatalf("Failed to create app: %v", err)
    }
    defer app.Close()
    
    // Setup template and page...
    counter := &TestCounter{Value: 0}
    page.RegisterDataModel(counter)
    
    // Test action handling through HTTP request simulation
    // This would test the complete action workflow
}
```

---

For implementation details and architecture information, see:
- **ARCHITECTURE.md**: Core architecture and components
- **HLD.md**: High-level design decisions
- **TEMPLATE_METHODS.md**: Template parsing API reference