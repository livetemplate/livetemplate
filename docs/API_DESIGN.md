# StateTemplate API Reference

## Overview

StateTemplate provides a clean, focused API for real-time template rendering. The current public API centers around the `Renderer` type with supporting Parse methods and real-time update capabilities.

## Current Public API

### Core Types

#### `Renderer`

The main entry point for template rendering and real-time updates.

```go
type Renderer struct {
    // Internal fields - not exposed
}
```

#### `Update`

Output type for real-time fragment updates.

```go
type Update struct {
    FragmentID string `json:"fragment_id"` // The ID of the element to update
    HTML       string `json:"html"`        // The new HTML content
    Action     string `json:"action"`      // "replace", "append", "prepend", "remove", etc.
    *RangeInfo `json:"range,omitempty"`   // Range operation info (optional)
}
```

#### `RangeInfo`

Contains range operation details for list updates.

```go
type RangeInfo struct {
    ItemKey     string `json:"item_key"`               // Unique identifier for the item
    ReferenceID string `json:"reference_id,omitempty"` // Reference element for positioning
}
```

### Constructor and Options

#### `NewRenderer(opts ...Option) *Renderer`

Creates a new renderer instance with optional configuration.

```go
renderer := statetemplate.NewRenderer()
// or with options
renderer := statetemplate.NewRenderer(someOption)
```

### Template Parsing Methods

#### `Parse(templateContent string) error`

Parses template content from a string.

```go
err := renderer.Parse(`<h1>{{.Title}}</h1>`)
```

#### `ParseFiles(filenames ...string) error`

Parses templates from files.

```go
err := renderer.ParseFiles("template.html", "layout.html")
```

#### `ParseGlob(pattern string) error`

Parses templates matching a glob pattern.

```go
err := renderer.ParseGlob("templates/*.html")
```

#### `ParseFS(fsys fs.FS, patterns ...string) error`

Parses templates from an embedded filesystem.

```go
//go:embed templates
var templateFS embed.FS

err := renderer.ParseFS(templateFS, "templates/*.html")
```

### Real-time Methods

#### `SetInitialData(data interface{}) (string, error)`

Sets initial data and returns complete HTML for page load.

```go
data := &MyData{Title: "Hello World"}
html, err := renderer.SetInitialData(data)
```

#### `GetUpdateChannel() <-chan Update`

Returns channel for receiving real-time updates.

```go
updateChan := renderer.GetUpdateChannel()
for update := range updateChan {
    // Send to WebSocket clients
    sendToClients(update)
}
```

#### `SendUpdate(newData interface{})`

Sends new data that may trigger fragment updates.

```go
newData := &MyData{Title: "Updated Title"}
renderer.SendUpdate(newData)
```

#### `Start()` and `Stop()`

Controls the background update processor.

```go
renderer.Start()
defer renderer.Stop()
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "log"
    "statetemplate"
)

func main() {
    // Create renderer
    renderer := statetemplate.NewRenderer()

    // Parse template
    err := renderer.Parse(`<h1>{{.Title}}</h1><p>{{.Content}}</p>`)
    if err != nil {
        log.Fatal(err)
    }

    // Set initial data
    data := map[string]interface{}{
        "Title":   "Welcome",
        "Content": "Hello World",
    }

    html, err := renderer.SetInitialData(data)
    if err != nil {
        log.Fatal(err)
    }

    // html contains the wrapped HTML ready for WebSocket updates
    log.Println(html)
}
```

### Real-time Updates

```go
// Start real-time processing
renderer.Start()
defer renderer.Stop()

// Get update channel
updateChan := renderer.GetUpdateChannel()
go func() {
    for update := range updateChan {
        // Send to WebSocket clients
        log.Printf("Fragment %s updated: %s", update.FragmentID, update.Action)
        // sendToWebSocketClients(update)
    }
}()

// Trigger updates
newData := map[string]interface{}{
    "Title":   "Updated Title",
    "Content": "Updated Content",
}
renderer.SendUpdate(newData)
```

### File-based Templates

```go
renderer := statetemplate.NewRenderer()

// Parse from files
err := renderer.ParseFiles("header.html", "content.html", "footer.html")
if err != nil {
    log.Fatal(err)
}

// Or use glob patterns
err = renderer.ParseGlob("templates/*.html")
if err != nil {
    log.Fatal(err)
}
```

### Embedded Templates

```go
//go:embed templates
var templateFS embed.FS

renderer := statetemplate.NewRenderer()
err := renderer.ParseFS(templateFS, "templates/*.html")
if err != nil {
    log.Fatal(err)
}
```

## Implementation Notes

### Internal Types (Not Public API)

The following types are implementation details and are not exported:

- `templateFragment` - Internal fragment representation
- `rangeFragment` - Range-specific fragment handling
- `conditionalFragment` - Conditional block fragments
- `templateTracker` - Data change tracking
- `fragmentExtractor` - Fragment extraction logic
- `advancedTemplateAnalyzer` - Template analysis

### Fragment Actions

The `Update.Action` field can contain:

- `"replace"` - Replace element content
- `"append"` - Add element to end of container
- `"prepend"` - Add element to beginning of container
- `"remove"` - Remove element
- `"insertafter"` - Insert after reference element (requires RangeInfo.ReferenceID)
- `"insertbefore"` - Insert before reference element (requires RangeInfo.ReferenceID)

### Range Operations

For templates with `{{range}}` blocks, StateTemplate automatically generates granular list updates:

```html
{{range .Items}}
<div>{{.Name}}</div>
{{end}}
```

When items are added, removed, or reordered, individual `Update` messages are generated with appropriate actions and range information.

## Migration from Earlier Versions

If you have code referencing older type names:

- `RealtimeRenderer` → `Renderer`
- `RealtimeUpdate` → `Update`
- `RealtimeConfig` → (removed, use Options pattern)
- `AddTemplate()` → Use Parse methods instead

## Error Handling

All Parse methods return errors for:

- Template syntax errors
- File not found errors
- Filesystem access errors

The real-time methods handle errors gracefully:

- `SetInitialData()` returns rendering errors
- `SendUpdate()` never blocks or panics
- Failed fragment updates are logged but don't stop processing

## Performance Considerations

- Parse templates once at startup, not per request
- Use `ParseFS` with embedded templates for better performance
- Start/Stop the renderer appropriately to manage goroutines
- Update channel is buffered but can be overwhelmed with high-frequency updates

## Thread Safety

- All public methods are thread-safe
- Multiple goroutines can call `SendUpdate()` concurrently
- The update channel can be consumed by a single goroutine
- Template parsing should be done during initialization, not concurrently
