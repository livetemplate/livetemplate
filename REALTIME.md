# 🌐 Real-time Web Rendering API

The StateTemplate library now includes a powerful **Real-time Web Rendering API** that enables targeted fragment updates for modern web applications. This new API is perfect for WebSocket-based real-time applications where you need to update specific parts of the page without full re-renders.

## 🎯 Key Features

### ✨ **Fragment-Based Updates**
- Automatically extracts template fragments for granular updates
- Each fragment gets a unique ID for targeted DOM updates
- Preserves block names as fragment IDs when possible
- Wraps fragments with configurable HTML tags (default: `<div>`)

### 🔄 **Live Change Detection**
- Deep data structure comparison to detect changes
- Only sends updates for fragments that actually changed
- Supports nested data structures and complex field dependencies
- No unnecessary updates - highly efficient

### 📡 **WebSocket-Ready Output**
- JSON-serializable update messages
- Fragment ID, HTML content, and action type included
- Perfect for real-time web applications
- Easy integration with WebSocket servers

## 📋 API Reference

### **RealtimeRenderer**

```go
type RealtimeRenderer struct {
    // Internal fields...
}

// Configuration for the renderer
type RealtimeConfig struct {
    WrapperTag     string // HTML tag to wrap fragments (default: "div")
    IDPrefix       string // Prefix for fragment IDs (default: "fragment-")
    PreserveBlocks bool   // Whether to preserve block names as IDs
}

// Update message sent to clients
type RealtimeUpdate struct {
    FragmentID string `json:"fragment_id"` // ID of element to update
    HTML       string `json:"html"`        // New HTML content
    Action     string `json:"action"`      // "replace", "append", etc.
}
```

### **Core Methods**

#### `NewRealtimeRenderer(config *RealtimeConfig) *RealtimeRenderer`
Creates a new real-time renderer with optional configuration.

```go
renderer := statetemplate.NewRealtimeRenderer(&statetemplate.RealtimeConfig{
    WrapperTag:     "div",
    IDPrefix:       "fragment-",
    PreserveBlocks: true,
})
```

#### `AddTemplate(name, content string) error`
Adds an HTML template for real-time rendering with automatic fragment extraction.

```go
template := `<div>
    Current Count: {{.Counter.Value}}
    {{block "header" .}}
        <h1>{{.Site.Name}}</h1>
    {{end}}
</div>`

err := renderer.AddTemplate("main", template)
```

#### `SetInitialData(data interface{}) (string, error)`
Sets initial data and returns complete HTML for page load.

```go
initialData := &MyData{Counter: &Counter{Value: 42}}
fullHTML, err := renderer.SetInitialData(initialData)
// Use fullHTML for initial page render
```

#### `Start()` and `Stop()`
Starts/stops the background update processor.

```go
renderer.Start()
defer renderer.Stop()
```

#### `GetUpdateChannel() <-chan RealtimeUpdate`
Returns channel for receiving real-time updates.

```go
updateChan := renderer.GetUpdateChannel()
for update := range updateChan {
    // Send update to WebSocket clients
    sendToClients(update)
}
```

#### `SendUpdate(newData interface{})`
Sends new data that may trigger fragment updates.

```go
newData := &MyData{Counter: &Counter{Value: 43}}
renderer.SendUpdate(newData)
```

## 🚀 Usage Example

### **Complete Web Application Integration**

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "github.com/gorilla/websocket"
    "github.com/livefir/statetemplate"
)

type AppData struct {
    Counter *Counter `json:"counter"`
    Message string   `json:"message"`
}

type Counter struct {
    Value int    `json:"value"`
    Label string `json:"label"`
}

func main() {
    // Create renderer
    renderer := statetemplate.NewRealtimeRenderer(nil)
    
    // Add template
    template := `<div>
        <div id="counter">Count: {{.Counter.Value}}</div>
        <div id="message">{{.Message}}</div>
        {{block "status" .}}
            <p>Status: {{.Counter.Label}}</p>
        {{end}}
    </div>`
    
    renderer.AddTemplate("app", template)
    
    // Set initial data
    initialData := &AppData{
        Counter: &Counter{Value: 0, Label: "Ready"},
        Message: "Welcome!",
    }
    
    fullHTML, _ := renderer.SetInitialData(initialData)
    renderer.Start()
    defer renderer.Stop()
    
    // WebSocket handler
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, _ := websocket.Upgrade(w, r, nil)
        defer conn.Close()
        
        // Send initial HTML
        conn.WriteJSON(map[string]string{
            "type": "initial",
            "html": fullHTML,
        })
        
        // Forward real-time updates
        updateChan := renderer.GetUpdateChannel()
        for update := range updateChan {
            conn.WriteJSON(map[string]interface{}{
                "type": "update",
                "fragmentId": update.FragmentID,
                "html": update.HTML,
                "action": update.Action,
            })
        }
    })
    
    // API endpoint to trigger updates
    http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
        newData := &AppData{
            Counter: &Counter{Value: 42, Label: "Updated"},
            Message: "Data changed!",
        }
        renderer.SendUpdate(newData)
        w.WriteHeader(200)
    })
    
    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}
```

### **Client-Side JavaScript**

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    
    if (message.type === 'initial') {
        // Set initial HTML
        document.getElementById('app').innerHTML = message.html;
    } else if (message.type === 'update') {
        // Update specific fragment
        const element = document.getElementById(message.fragmentId);
        if (element) {
            element.outerHTML = message.html;
        }
    }
};

// Trigger updates
function updateData() {
    fetch('/update', { method: 'POST' });
}
```

## 🎨 Template Features

### **Block Preservation**
Named blocks become fragment IDs:

```html
{{block "header" .}}
    <h1>{{.Title}}</h1>
{{end}}
```
Results in: `<div id="header">...</div>`

### **Automatic Fragment Detection**
Templates are automatically analyzed for update targets:

```html
<div>
    Current: {{.Counter.Value}}    <!-- Fragment 1 -->
    Status: {{.Status.Message}}    <!-- Fragment 2 -->
</div>
```

### **Nested Data Support**
Complex data structures work seamlessly:

```go
type AppData struct {
    User *User `json:"user"`
    Stats *Stats `json:"stats"`
}

// Changes to User.Name or Stats.Count are detected automatically
```

## 🔧 Configuration Options

### **Custom Wrapper Tags**
```go
config := &statetemplate.RealtimeConfig{
    WrapperTag: "section",  // Use <section> instead of <div>
    IDPrefix: "live-",      // Fragment IDs: live-header, live-sidebar
    PreserveBlocks: true,   // Keep block names as IDs
}
```

### **Fragment ID Strategies**
- **Block Names**: `{{block "header" .}}` → `id="header"`
- **Generated IDs**: `fragment-main-12345` for unnamed fragments
- **Custom Prefix**: Configure with `IDPrefix` option

## 📊 Performance Benefits

- **Minimal DOM Updates**: Only changed fragments are updated
- **Efficient Change Detection**: Deep comparison only for template dependencies
- **No Full Re-renders**: Preserve scroll position, form state, focus
- **Bandwidth Efficient**: Send only changed HTML content
- **Real-time Responsive**: Sub-millisecond update detection

## 🛡️ Production Considerations

### **Error Handling**
```go
// Graceful handling of template errors
if err := renderer.AddTemplate("main", template); err != nil {
    log.Printf("Template error: %v", err)
    // Fallback behavior
}
```

### **Rate Limiting**
```go
// Channel has built-in buffering to prevent blocking
updateChan := renderer.GetUpdateChannel()

// For high-frequency updates, consider debouncing
debouncer := time.NewTicker(100 * time.Millisecond)
for {
    select {
    case update := <-updateChan:
        // Batch updates within 100ms window
        handleUpdate(update)
    case <-debouncer.C:
        // Send batched updates
    }
}
```

### **Memory Management**
```go
// Always clean up resources
renderer.Start()
defer renderer.Stop()  // Closes goroutines and channels
```

## 🧪 Testing Integration

The realtime renderer includes comprehensive test coverage:

```bash
# Run all realtime tests
go test ./examples/e2e -run TestRealtime -v

# Run benchmarks
go test ./examples/e2e -bench=BenchmarkRealtime -v
```

## 🎯 Use Cases

- **Live Dashboards**: Real-time metrics and charts
- **Chat Applications**: Message updates without page refresh
- **Gaming Interfaces**: Live score updates, player status
- **Admin Panels**: Live user activity, system status
- **E-commerce**: Live inventory, price changes
- **Collaborative Editing**: Live document updates

## 🔄 Migration from Basic TemplateTracker

Existing `TemplateTracker` code can be enhanced with real-time capabilities:

```go
// Before: Basic template tracking
tracker := statetemplate.NewTemplateTracker()
tracker.AddTemplate("main", tmpl)

// After: Real-time web rendering
renderer := statetemplate.NewRealtimeRenderer(nil)
renderer.AddTemplate("main", templateContent)
renderer.Start()

// Get both initial HTML AND live updates
fullHTML, _ := renderer.SetInitialData(data)
updateChan := renderer.GetUpdateChannel()
```

The Real-time Web API provides a seamless upgrade path for existing applications while enabling powerful new real-time capabilities! 🚀
