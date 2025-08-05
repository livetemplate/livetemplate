# StateTemplate Examples Guide

This guide provides practical examples for using StateTemplate in real-world applications.

## Quick Start

### Basic Template Rendering

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

    log.Println(html)
}
```

### Real-time Updates

```go
package main

import (
    "log"
    "time"
    "statetemplate"
)

func main() {
    renderer := statetemplate.NewRenderer()

    // Parse template with fragments
    template := `
    <div>
        <h1>{{.Title}}</h1>
        <div>Count: {{.Counter}}</div>
        <div>Time: {{.Timestamp}}</div>
    </div>`

    renderer.Parse(template)

    // Start real-time processing
    renderer.Start()
    defer renderer.Stop()

    // Listen for updates
    updateChan := renderer.GetUpdateChannel()
    go func() {
        for update := range updateChan {
            log.Printf("Fragment %s updated: %s", update.FragmentID, update.Action)
            // In real app: send to WebSocket clients
        }
    }()

    // Set initial data
    data := map[string]interface{}{
        "Title":     "Real-time Demo",
        "Counter":   0,
        "Timestamp": time.Now().Format("15:04:05"),
    }

    html, _ := renderer.SetInitialData(data)
    log.Println("Initial HTML:", html)

    // Simulate updates
    for i := 1; i <= 5; i++ {
        time.Sleep(2 * time.Second)

        data["Counter"] = i
        data["Timestamp"] = time.Now().Format("15:04:05")

        renderer.SendUpdate(data)
    }
}
```

## File-based Templates

### Using Template Files

```go
package main

import (
    "log"
    "statetemplate"
)

func main() {
    renderer := statetemplate.NewRenderer()

    // Parse templates from files
    err := renderer.ParseFiles("header.html", "content.html", "footer.html")
    if err != nil {
        log.Fatal(err)
    }

    // Or use glob patterns
    err = renderer.ParseGlob("templates/*.html")
    if err != nil {
        log.Fatal(err)
    }

    data := map[string]interface{}{
        "User": map[string]string{
            "Name":  "John Doe",
            "Email": "john@example.com",
        },
        "Posts": []map[string]string{
            {"Title": "First Post", "Content": "Hello world"},
            {"Title": "Second Post", "Content": "More content"},
        },
    }

    html, err := renderer.SetInitialData(data)
    if err != nil {
        log.Fatal(err)
    }

    log.Println(html)
}
```

### Embedded Templates

```go
package main

import (
    "embed"
    "log"
    "statetemplate"
)

//go:embed templates
var templateFS embed.FS

func main() {
    renderer := statetemplate.NewRenderer()

    // Parse from embedded filesystem
    err := renderer.ParseFS(templateFS, "templates/*.html")
    if err != nil {
        log.Fatal(err)
    }

    data := map[string]interface{}{
        "Title": "Embedded Templates",
        "Items": []string{"Item 1", "Item 2", "Item 3"},
    }

    html, err := renderer.SetInitialData(data)
    if err != nil {
        log.Fatal(err)
    }

    log.Println(html)
}
```

## WebSocket Integration

### Complete Web Application

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/gorilla/websocket"
    "statetemplate"
)

type AppData struct {
    Counter   int    `json:"counter"`
    Message   string `json:"message"`
    Timestamp string `json:"timestamp"`
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Allow all origins in development
    },
}

func main() {
    // Create renderer
    renderer := statetemplate.NewRenderer()

    // Parse template
    template := `
    <!DOCTYPE html>
    <html>
    <head>
        <title>StateTemplate Demo</title>
    </head>
    <body>
        <div id="app">
            <h1>{{.Message}}</h1>
            <div id="counter">Count: {{.Counter}}</div>
            <div id="time">{{.Timestamp}}</div>
            <button onclick="updateCounter()">Update</button>
        </div>

        <script>
            const ws = new WebSocket('ws://localhost:8080/ws');

            ws.onmessage = function(event) {
                const message = JSON.parse(event.data);

                if (message.type === 'initial') {
                    document.getElementById('app').innerHTML = message.html;
                } else if (message.type === 'update') {
                    const element = document.getElementById(message.fragment_id);
                    if (element) {
                        if (message.action === 'replace') {
                            element.outerHTML = message.html;
                        }
                        // Handle other actions as needed
                    }
                }
            };

            function updateCounter() {
                fetch('/api/increment', { method: 'POST' });
            }
        </script>
    </body>
    </html>`

    renderer.Parse(template)

    // Start real-time processing
    renderer.Start()
    defer renderer.Stop()

    // Initialize data
    appData := &AppData{
        Counter:   0,
        Message:   "Welcome to StateTemplate!",
        Timestamp: time.Now().Format("15:04:05"),
    }

    initialHTML, _ := renderer.SetInitialData(appData)

    // WebSocket handler
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Println("Upgrade error:", err)
            return
        }
        defer conn.Close()

        // Send initial HTML
        conn.WriteJSON(map[string]string{
            "type": "initial",
            "html": initialHTML,
        })

        // Forward real-time updates
        updateChan := renderer.GetUpdateChannel()
        for update := range updateChan {
            conn.WriteJSON(map[string]interface{}{
                "type":        "update",
                "fragment_id": update.FragmentID,
                "html":        update.HTML,
                "action":      update.Action,
            })
        }
    })

    // API endpoint to increment counter
    http.HandleFunc("/api/increment", func(w http.ResponseWriter, r *http.Request) {
        appData.Counter++
        appData.Timestamp = time.Now().Format("15:04:05")
        renderer.SendUpdate(appData)
        w.WriteHeader(http.StatusOK)
    })

    // Serve static files
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/" {
            w.Header().Set("Content-Type", "text/html")
            w.Write([]byte(initialHTML))
        }
    })

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Range Fragment Examples

### Dynamic Lists

```go
package main

import (
    "log"
    "statetemplate"
)

type Task struct {
    ID       string `json:"id"`
    Title    string `json:"title"`
    Priority int    `json:"priority"`
    URL      string `json:"url"`
}

func main() {
    renderer := statetemplate.NewRenderer()

    // Template with range
    template := `
    <div>
        <h1>Task List</h1>
        <ul class="task-list">
            {{range .Tasks}}
            <li data-id="{{.ID}}" class="task-item">
                <a href="{{.URL}}">{{.Title}} (Priority: {{.Priority}})</a>
            </li>
            {{end}}
        </ul>
    </div>`

    renderer.Parse(template)
    renderer.Start()
    defer renderer.Stop()

    // Listen for updates
    updateChan := renderer.GetUpdateChannel()
    go func() {
        for update := range updateChan {
            log.Printf("Range update - Action: %s, Fragment: %s", update.Action, update.FragmentID)
            if update.RangeInfo != nil {
                log.Printf("  Item Key: %s, Reference: %s",
                    update.RangeInfo.ItemKey, update.RangeInfo.ReferenceID)
            }
        }
    }()

    // Initial data
    data := map[string]interface{}{
        "Tasks": []Task{
            {ID: "1", Title: "Fix bug", Priority: 1, URL: "/task/1"},
            {ID: "2", Title: "Add feature", Priority: 2, URL: "/task/2"},
            {ID: "3", Title: "Write docs", Priority: 3, URL: "/task/3"},
        },
    }

    html, _ := renderer.SetInitialData(data)
    log.Println("Initial HTML generated")

    // Add new task
    data["Tasks"] = append(data["Tasks"].([]Task), Task{
        ID: "4", Title: "Review code", Priority: 1, URL: "/task/4",
    })
    renderer.SendUpdate(data)

    // Remove a task
    tasks := data["Tasks"].([]Task)
    data["Tasks"] = append(tasks[:1], tasks[2:]...) // Remove middle task
    renderer.SendUpdate(data)
}
```

## Template Features

### Block Templates

```go
// Template with named blocks
template := `
{{block "header" .}}
    <header>
        <h1>{{.Title}}</h1>
        <nav>{{.Navigation}}</nav>
    </header>
{{end}}

{{block "content" .}}
    <main>
        <p>{{.Content}}</p>
        <div>Users: {{len .Users}}</div>
    </main>
{{end}}

{{block "footer" .}}
    <footer>{{.Footer}}</footer>
{{end}}`

// Blocks become fragment IDs: "header", "content", "footer"
```

### Conditional Fragments

```go
// Template with conditionals
template := `
<div>
    {{if .User}}
        <p>Welcome, {{.User.Name}}!</p>
        {{if gt .User.Points 100}}
            <span class="badge">VIP Member</span>
        {{end}}
    {{else}}
        <p>Please log in</p>
    {{end}}

    {{with .Notification}}
        <div class="alert">{{.Message}}</div>
    {{end}}
</div>`

// Conditional blocks are automatically detected and wrapped
```

## Best Practices

### Performance Tips

1. **Parse templates once** at startup, not per request
2. **Use embedded templates** for better performance
3. **Structure data efficiently** - avoid deep nesting when possible
4. **Start/Stop renderer** appropriately to manage goroutines

### Error Handling

```go
renderer := statetemplate.NewRenderer()

// Always check parse errors
if err := renderer.ParseFiles("template.html"); err != nil {
    log.Fatalf("Template parse error: %v", err)
}

// Handle data rendering errors
html, err := renderer.SetInitialData(data)
if err != nil {
    log.Printf("Render error: %v", err)
    // Fallback handling
}

// SendUpdate never blocks or panics
renderer.SendUpdate(newData) // Safe to call
```

### Thread Safety

```go
// Multiple goroutines can safely call SendUpdate
go func() {
    for {
        time.Sleep(time.Second)
        renderer.SendUpdate(getLatestData())
    }
}()

go func() {
    for {
        time.Sleep(2 * time.Second)
        renderer.SendUpdate(getOtherData())
    }
}()

// Single goroutine should consume updates
updateChan := renderer.GetUpdateChannel()
for update := range updateChan {
    // Process updates sequentially
    handleUpdate(update)
}
```

## Integration Patterns

### With Popular WebSocket Libraries

**Gorilla WebSocket:**

```go
import "github.com/gorilla/websocket"

// See complete example above
```

**Gin Framework:**

```go
import "github.com/gin-gonic/gin"

func setupWebSocket(renderer *statetemplate.Renderer) gin.HandlerFunc {
    return gin.WrapH(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        conn, _ := upgrader.Upgrade(w, r, nil)
        defer conn.Close()

        // Forward updates
        for update := range renderer.GetUpdateChannel() {
            conn.WriteJSON(update)
        }
    }))
}
```

### With Server-Sent Events

```go
func setupSSE(renderer *statetemplate.Renderer) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        for update := range renderer.GetUpdateChannel() {
            data, _ := json.Marshal(update)
            fmt.Fprintf(w, "data: %s\n\n", data)
            w.(http.Flusher).Flush()
        }
    }
}
```

## Migration from Earlier Versions

If upgrading from an earlier version:

```go
// Old API (if you had it)
// renderer := statetemplate.NewRenderer(config)
// renderer.AddTemplate("name", content)

// New API
renderer := statetemplate.NewRenderer()
renderer.Parse(content) // or ParseFiles, ParseGlob, ParseFS

// Update types changed
// Update -> Update
// Renderer -> Renderer
```

For more examples, see the `examples/` directory in the repository.
