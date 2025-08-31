# LiveTemplate API Reference

## Overview

LiveTemplate provides a simple, efficient API for ultra-efficient HTML template rendering with **tree-based optimization**. The library generates minimal update structures similar to Phoenix LiveView, achieving 90%+ bandwidth savings through intelligent static/dynamic content separation.

> **Current Version**: Simplified tree-based architecture with single Page API

## Core Architecture

### Single Page Model
- **Pages** are template instances with tree-based optimization
- **Simple API** with minimal configuration required
- **Memory management** with automatic cleanup
- **Tree-based strategy** handles all template patterns automatically

## Public API

### Page Management

#### `NewPage(tmpl *template.Template, data interface{}, options ...PageOption) (*Page, error)`

Creates a new page instance with the provided template and initial data.

```go
page, err := livetemplate.NewPage(tmpl, initialData)
if err != nil {
    log.Fatal(err)
}
defer page.Close()
```

**Page Options:**
- `WithMetricsEnabled(bool)` - Enable metrics collection (default: true)
- `WithFallbackEnabled(bool)` - Enable fallback strategies (no-op in tree-based system)
- `WithMaxGenerationTime(duration)` - Set generation timeout (no-op in tree-based system)

### Page Operations

#### `Page.Render() (string, error)`

Generates the complete HTML output for the current page state.

```go
html, err := page.Render()
if err != nil {
    log.Printf("Render failed: %v", err)
    return
}
fmt.Println(html)
```

#### `Page.RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error)`

Generates efficient fragment updates using tree-based optimization.

```go
fragments, err := page.RenderFragments(context.Background(), newData)
if err != nil {
    log.Printf("Fragment generation failed: %v", err)
    return
}

for _, fragment := range fragments {
    // Process fragment update - always tree-based strategy
    fmt.Printf("Strategy: %s, Action: %s\n", fragment.Strategy, fragment.Action)
    // fragment.Strategy will always be "tree_based"
    // fragment.Data contains tree structure with statics and dynamics
}
```

#### `Page.UpdateData(newData interface{}) interface{}`

Updates the page data and returns the current state.

```go
currentData := page.UpdateData(newData)
```

#### `Page.GetData() interface{}`

Returns the current page data.

```go
data := page.GetData()
```

#### `Page.GetTemplate() *template.Template`

Returns the page template.

```go
tmpl := page.GetTemplate()
```

#### `Page.SetTemplate(tmpl *template.Template) error`

Updates the page template.

```go
err := page.SetTemplate(newTemplate)
```

#### `Page.GetMetrics() *UpdateGeneratorMetrics`

Returns page-specific performance metrics.

```go
metrics := page.GetMetrics()
fmt.Printf("Total generations: %d\n", metrics.TotalGenerations)
```

#### `Page.ResetMetrics()`

Resets all fragment generation metrics.

```go
page.ResetMetrics()
```

#### `Page.GetCreatedTime() time.Time`

Returns when the page was created.

```go
created := page.GetCreatedTime()
```

#### `Page.Close() error`

Releases page resources and performs cleanup.

```go
err := page.Close()
```

## Data Types

### Fragment

Represents an update fragment with tree-based optimization data.

```go
type Fragment struct {
    ID       string            `json:"id"`       // Unique fragment identifier
    Strategy string            `json:"strategy"` // Always "tree_based"
    Action   string            `json:"action"`   // Always "update_tree"
    Data     interface{}       `json:"data"`     // Tree structure
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
    Strategy         int           `json:"strategy_number"`      // Always 1 (tree-based)
    Confidence       float64       `json:"confidence"`           // Always 1.0
    FallbackUsed     bool          `json:"fallback_used"`        // Always false
}
```

### UpdateGeneratorMetrics

Tracks performance of the update generation pipeline.

```go
type UpdateGeneratorMetrics struct {
    TotalGenerations      int64            `json:"total_generations"`
    SuccessfulGenerations int64            `json:"successful_generations"`
    FailedGenerations     int64            `json:"failed_generations"`
    StrategyUsage         map[string]int64 `json:"strategy_usage"`       // Always "tree_based": count
    AverageGenerationTime time.Duration    `json:"average_generation_time"`
    TotalBandwidthSaved   int64            `json:"total_bandwidth_saved"`
    FallbackRate          float64          `json:"fallback_rate"`        // Always 0.0
    ErrorRate             float64          `json:"error_rate"`
    LastReset             time.Time        `json:"last_reset"`
}
```

## Tree-Based Optimization

LiveTemplate uses **tree-based optimization** - a single unified strategy that adapts to all template patterns, achieving 90%+ bandwidth savings through intelligent static/dynamic content separation.

### Single Strategy: Tree-Based (90%+ reduction)

**For all template patterns**: Static content cached client-side, only dynamic values transmitted

```go
// Fragment.Strategy = "tree_based"
// Fragment.Action = "update_tree"
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
- **Tree-based optimization**: 90%+ bandwidth savings for all template patterns
- **Generation speed**: Sub-microsecond performance (236μs for full test suite)
- **Consistency**: Same high performance across all template constructs
- **Memory efficiency**: Minimal allocation with structure reuse

### Memory Management
- **Lightweight pages**: ~4MB typical memory usage per page
- **Automatic cleanup**: Resource management in Close() method
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
    data := getPageData(r)
    page, err := livetemplate.NewPage(template, data)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    defer page.Close()
    
    html, err := page.Render()
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
}
```

### WebSocket Real-time Updates
```go
func websocketHandler(w http.ResponseWriter, r *http.Request) {
    initialData := getInitialData(r)
    page, err := livetemplate.NewPage(template, initialData)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    defer page.Close()
    
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    
    // Send initial HTML
    html, _ := page.Render()
    conn.WriteMessage(websocket.TextMessage, []byte(html))
    
    // Handle updates
    for {
        var newData map[string]interface{}
        if err := conn.ReadJSON(&newData); err != nil {
            break
        }
        
        fragments, err := page.RenderFragments(context.Background(), newData)
        if err != nil {
            log.Printf("Fragment generation failed: %v", err)
            continue
        }
        
        for _, fragment := range fragments {
            if err := conn.WriteJSON(fragment); err != nil {
                log.Printf("Failed to send fragment: %v", err)
                return
            }
        }
    }
}
```

### Template Updates
```go
func updateTemplate(page *livetemplate.Page, newTemplateText string) error {
    tmpl, err := template.New("updated").Parse(newTemplateText)
    if err != nil {
        return fmt.Errorf("template parse error: %w", err)
    }
    
    if err := page.SetTemplate(tmpl); err != nil {
        return fmt.Errorf("template update error: %w", err)
    }
    
    return nil
}
```

## Error Handling

### Common Error Scenarios
- `"template cannot be nil"` - Nil template passed to NewPage or SetTemplate
- `"template execution failed"` - Template rendering error
- `"tree fragment generation failed"` - Fragment generation error

### Best Practices
- Always check errors from `NewPage()` and other operations
- Use `defer page.Close()` to ensure resource cleanup
- Handle template execution errors gracefully
- Monitor metrics for performance issues

## Concurrency

### Thread Safety
- All public API methods are **thread-safe**
- Multiple goroutines can safely access the same page
- Metrics collection is thread-safe with proper synchronization
- Template updates are synchronized

### Performance Considerations
- Page instances are lightweight - create as needed
- Tree generation is optimized for high throughput
- Memory cleanup is automatic on Close()
- Concurrent access is optimized with read-write mutexes

## Testing

### Unit Testing with LiveTemplate
```go
func TestPageRendering(t *testing.T) {
    tmpl := template.Must(template.New("test").Parse(`<p>{{.Name}}</p>`))
    data := map[string]interface{}{"Name": "Test"}
    
    page, err := livetemplate.NewPage(tmpl, data)
    if err != nil {
        t.Fatalf("Failed to create page: %v", err)
    }
    defer page.Close()
    
    html, err := page.Render()
    if err != nil {
        t.Fatalf("Failed to render: %v", err)
    }
    
    expected := `<p>Test</p>`
    if html != expected {
        t.Errorf("Expected %q, got %q", expected, html)
    }
}
```

### Fragment Testing
```go
func TestFragmentGeneration(t *testing.T) {
    tmpl := template.Must(template.New("test").Parse(`<p>{{.Name}}</p>`))
    initialData := map[string]interface{}{"Name": "Alice"}
    updatedData := map[string]interface{}{"Name": "Bob"}
    
    page, err := livetemplate.NewPage(tmpl, initialData)
    if err != nil {
        t.Fatalf("Failed to create page: %v", err)
    }
    defer page.Close()
    
    fragments, err := page.RenderFragments(context.Background(), updatedData)
    if err != nil {
        t.Fatalf("Failed to generate fragments: %v", err)
    }
    
    if len(fragments) != 1 {
        t.Fatalf("Expected 1 fragment, got %d", len(fragments))
    }
    
    fragment := fragments[0]
    if fragment.Strategy != "tree_based" {
        t.Errorf("Expected strategy 'tree_based', got %q", fragment.Strategy)
    }
    
    if fragment.Action != "update_tree" {
        t.Errorf("Expected action 'update_tree', got %q", fragment.Action)
    }
}
```

---

For implementation details and architecture information, see:
- **ARCHITECTURE.md**: Core architecture and components
- **CI_CD_PIPELINE.md**: Testing and validation pipeline
- **HLD.md**: High-level design decisions