# LiveTemplate Todo App Example

A real-time todo application demonstrating LiveTemplate's reactive state management with [Pico CSS](https://picocss.com/) for semantic, class-less styling.

## Features

- **Add todos** - Create new tasks via form submission
- **Toggle completion** - Mark tasks as done/undone with checkboxes
- **Delete todos** - Remove individual tasks
- **Clear completed** - Bulk remove all completed tasks
- **Live statistics** - Real-time total, completed, and remaining counts
- **Reactive updates** - Changes automatically broadcast to all connected clients
- **Semantic CSS** - Beautiful UI using Pico CSS without custom classes
- **Transport-agnostic** - Works over WebSocket or plain HTTP/AJAX

## Running the Example

1. **Start the server:**

   From project root:
   ```bash
   go run examples/todos/main.go
   ```

   Or from the todos directory:
   ```bash
   cd examples/todos
   go run main.go
   ```

   With custom port:
   ```bash
   PORT=8081 go run main.go
   ```

2. **Open your browser:**
   Navigate to `http://localhost:8080`

3. **Interact with todos:**
   - Type a task in the input field and click "Add"
   - Check/uncheck boxes to toggle completion
   - Click "Delete" to remove a task
   - Click "Clear Completed" to remove all done tasks
   - Watch statistics update in real-time

## How It Works

### Server Side (Go)

Simple reactive API with array state management:

```go
type TodoItem struct {
    ID        string
    Text      string
    Completed bool
}

type TodoState struct {
    Title          string
    Todos          []TodoItem
    TotalCount     int
    CompletedCount int
    RemainingCount int
}

// Implement the Store interface
func (s *TodoState) Change(action string, data map[string]interface{}) {
    switch action {
    case "add":
        text := livetemplate.GetString(data, "text")
        s.Todos = append(s.Todos, TodoItem{
            ID:   fmt.Sprintf("todo-%d", time.Now().UnixNano()),
            Text: text,
        })
    case "toggle":
        id := livetemplate.GetString(data, "id")
        // Find and toggle todo
    case "delete":
        id := livetemplate.GetString(data, "id")
        // Remove todo from slice
    case "clear_completed":
        // Filter out completed todos
    }

    s.updateStats()
}

func main() {
    state := &TodoState{Title: "Todo App", Todos: []TodoItem{}}

    // Auto-discovers todos.tmpl in current directory
    tmpl := livetemplate.New("todos")

    // Handle() auto-configures: WebSocket, HTTP, state cloning, updates
    http.Handle("/", tmpl.Handle(state))
    http.ListenAndServe(":8080", nil)
}
```

**Key concepts:**
- **Array State Management**: Todos stored as slice, automatically tracked
- **Auto-discovery**: Automatically finds and parses `todos.tmpl`
- **Auto Updates**: Handle() automatically generates and sends updates after Change()
- **Auto Cloning**: Each WebSocket connection gets its own state copy
- **Session Management**: HTTP connections get session-based persistence

### Client Side (Pico CSS + LiveTemplate)

**Zero JavaScript needed** - Pico CSS + LiveTemplate handles everything:

```html
<!-- Pico CSS via CDN - no custom classes needed! -->
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">

<main class="container">
    <article>
        <!-- Add Todo Form -->
        <form lvt-submit="add">
            <fieldset role="group">
                <input type="text" name="text" placeholder="What needs to be done?" required>
                <button type="submit">Add</button>
            </fieldset>
        </form>

        <!-- Statistics Table -->
        <table>
            <tbody>
                <tr><td>Total</td><td><strong>{{.TotalCount}}</strong></td></tr>
                <tr><td>Completed</td><td><strong>{{.CompletedCount}}</strong></td></tr>
                <tr><td>Remaining</td><td><strong>{{.RemainingCount}}</strong></td></tr>
            </tbody>
        </table>

        <!-- Todo List -->
        {{range .Todos}}
        <tr data-key="{{.ID}}">
            <td>
                <input type="checkbox" {{if .Completed}}checked{{end}}
                       lvt-change="toggle" lvt-data-id="{{.ID}}">
            </td>
            <td>{{.Text}}</td>
            <td>
                <button lvt-click="delete" lvt-data-id="{{.ID}}">Delete</button>
            </td>
        </tr>
        {{end}}
    </article>
</main>

<!-- Auto-initializing client library -->
<script src="livetemplate-client.js"></script>
```

**Pico CSS features used:**
- `<main class="container">` - Responsive centered layout
- `<article>` - Card-style sections with automatic spacing
- `<form>` with `<fieldset role="group">` - Inline form controls
- `<table>` - Clean, styled tables
- `<button>` - Semantic button styling (primary, secondary variants)
- Semantic HTML elements automatically styled

**LiveTemplate features:**
- `lvt-submit` - Form submission with all field values
- `lvt-change` - Checkbox change events
- `lvt-click` - Button click events
- `lvt-data-*` - Custom data passed to actions
- `data-key` - Item tracking for range updates

## Architecture

```
Browser                    WebSocket/HTTP              Go Server
┌─────────────────┐        ┌──────────┐               ┌──────────────────┐
│ todos.tmpl      │        │          │               │ TodoState        │
│ (Pico CSS)      │        │          │               │   Todos []Item   │
│                 │        │          │               │   Stats          │
│ [Add] [Delete]  │◄──────►│  /       │◄─────────────►│                  │
│ [✓] Checkboxes  │        │          │               │ Change(action,   │
│                 │        │          │               │   data)          │
│ LiveTemplate    │        │          │               │                  │
│ Client JS       │        │ Auto-    │               │ Handle()         │
│                 │        │ detects  │               │ - Clones state   │
│ lvt-* attrs     │        │ transport│               │ - Generates      │
│ - submit        │        │          │               │   updates        │
│ - click         │        │          │               │ - Broadcasts     │
│ - change        │        │          │               │                  │
└─────────────────┘        └──────────┘               └──────────────────┘
```

## Example Update Payloads

**Initial State (no todos):**
```json
{
  "s": ["<main class=\"container\">...", "...</main>"],
  "0": "Todo App",
  "1": "No tasks yet. Add one above!"
}
```

**After Adding Todo (only changed values):**
```json
{
  "1": "<table><tbody><tr data-key=\"todo-1234\">...",
  "2": "1",
  "3": "0",
  "4": "1"
}
```

This demonstrates LiveTemplate's bandwidth efficiency - subsequent updates contain only changed dynamic values, not the full HTML.

## Testing

### WebSocket Integration Test

```bash
go test -v -run TestWebSocketBasic
```

Tests:
- WebSocket connection establishment
- Add todo action
- Toggle completion action
- Response validation

### Browser E2E Test

```bash
go test -v -run TestTodosE2E
```

Tests:
- Initial page load with Pico CSS
- WebSocket connectivity
- LiveTemplate wrapper preservation
- Semantic HTML structure

Requires Docker for Chrome headless testing.

## Development Notes

- **Port**: Defaults to `:8080`, override with `PORT` environment variable
- **Endpoint**: `/` handles both WebSocket upgrades and HTTP POST requests
- **Template Path**: Reads from `examples/todos/todos.tmpl`
- **Client Library**: Serves via `internal/testing.ServeClientLibrary()` (dev only)
- **State Isolation**: Each WebSocket connection gets its own todo list
- **Session Management**: HTTP connections persist state via cookies
- **Pico CSS**: Loaded from CDN, no build step required

## Why Pico CSS?

- **Zero configuration** - Works with semantic HTML
- **No custom classes** - `<article>`, `<table>`, `<form>` are pre-styled
- **Responsive** - Mobile-friendly by default
- **Dark mode** - Automatic theme switching
- **Accessible** - Proper ARIA roles and keyboard navigation
- **Minimal** - ~10KB gzipped

Perfect for LiveTemplate demos where you want beautiful UIs without CSS complexity!
