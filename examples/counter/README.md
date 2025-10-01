# LiveTemplate Counter Example

A real-time counter application demonstrating LiveTemplate's reactive state management and tree-based optimization.

## Features

- **Reactive state**: Changes to state automatically generate and broadcast updates
- **Transport-agnostic**: Works over WebSocket or plain HTTP/AJAX
- **Minimal bandwidth**: Only the changed values are transmitted, not the entire HTML
- **No custom JavaScript**: Uses only the LiveTemplate client library
- **Template-based**: HTML is generated from Go templates with conditional rendering
- **Simple API**: Mount a store with a single function call

## Running the Example

1. **Start the server:**

   From project root:
   ```bash
   go run examples/counter/main.go
   ```

   Or from the counter directory:
   ```bash
   cd examples/counter
   go run main.go
   ```

   With custom port:
   ```bash
   PORT=8081 go run main.go
   ```

2. **Open your browser:**
   Navigate to `http://localhost:8080`

3. **Interact with the counter:**
   - Click **+1** to increment the counter
   - Click **-1** to decrement the counter
   - Click **Reset** to reset to zero
   - Watch the conditional text change based on the counter value

## How It Works

### Server Side (Go)

The server is extremely simple with the new reactive API:

```go
type CounterState struct {
    Counter     int    `json:"counter"`
    Status      string `json:"status"`
    // ... other fields
}

// Implement the Store interface
func (s *CounterState) Change(action string, data map[string]interface{}) {
    switch action {
    case "increment":
        s.Counter++
    case "decrement":
        s.Counter--
    case "reset":
        s.Counter = 0
    }

    // Update derived state
    s.Status = getStatus(s.Counter)
    s.LastUpdated = formatTime()
}

func main() {
    tmpl := livetemplate.New("counter")
    tmpl.ParseFiles("counter.tmpl")

    state := &CounterState{Counter: 0, Status: "zero"}

    // Mount handles everything: WebSocket, HTTP, state cloning, updates
    http.Handle("/live", livetemplate.Mount(tmpl, state))
    http.ListenAndServe(":8080", nil)
}
```

**Key concepts:**
- **Store Interface**: Any struct with a `Change(action string, data map[string]interface{})` method
- **Auto Updates**: Mount automatically generates and sends updates after Change() is called
- **Auto Cloning**: Each WebSocket connection gets its own cloned state
- **Session Management**: HTTP connections automatically get session-based state persistence
- **Transport Detection**: Mount auto-detects WebSocket vs HTTP requests

### Client Side (JavaScript)

**Zero-config integration** - just add one script tag:

```html
<!-- In your template -->
<button lvt-click="increment">+1</button>
<button lvt-click="decrement">-1</button>
<button lvt-click="reset">Reset</button>

<!-- Auto-initializing client library -->
<script src="livetemplate-client.js"></script>
```

That's it! No JavaScript code needed. The client library auto-initializes and handles:
- **Declarative event binding** via `lvt-*` attributes (`lvt-click`, `lvt-submit`, `lvt-change`, `lvt-input`, etc.)
- **Automatic WebSocket connection** to `/live` endpoint
- **Automatic reconnection** on disconnect (configurable)
- **Automatic DOM updates** when updates arrive
- **Event delegation** - works with dynamically updated elements

#### Sending Actions with Data

Actions can include multiple values from forms, inputs, and custom data attributes:

```html
<!-- Simple action -->
<button lvt-click="increment">+1</button>

<!-- Action with input value -->
<input type="number" lvt-change="setValue">

<!-- Action with form data (all fields sent in data map) -->
<form lvt-submit="addTodo">
    <input name="title" type="text">
    <input name="priority" type="number">
    <button>Add</button>
</form>

<!-- Action with custom data attributes -->
<button lvt-click="delete" lvt-data-id="123" lvt-data-confirm="true">
    Delete Item
</button>
```

All values are collected into a `data` map and passed to the store's `Change()` method:
```go
func (s *Store) Change(action string, data map[string]interface{}) {
    id := livetemplate.GetInt(data, "id")
    title := livetemplate.GetString(data, "title")
    // ...
}
```

#### Supported lvt-* attributes:
- `lvt-click` - Handle click events
- `lvt-submit` - Handle form submissions (prevents default, sends all form fields)
- `lvt-change` - Handle input change events (sends input value as "value")
- `lvt-input` - Handle input events for real-time updates (sends input value as "value")
- `lvt-keydown` - Handle keydown events
- `lvt-keyup` - Handle keyup events
- `lvt-data-*` - Include custom data in the data map (e.g., `lvt-data-id="123"`)
- `lvt-value-*` - Include explicit multiple values (e.g., `lvt-value-quantity="5"`)

### LiveTemplate Integration

- **Tree-based Updates**: Only changed dynamic values are sent over the wire
- **Static Content Caching**: HTML structure is cached client-side
- **Differential Updates**: Bandwidth savings of 90%+ compared to full page refreshes
- **Conditional Rendering**: Template conditionals are handled automatically

## Architecture

```
Browser                    WebSocket/HTTP              Go Server
┌─────────────────┐        ┌──────────┐               ┌──────────────────┐
│ counter.tmpl    │        │          │               │ CounterState     │
│ (rendered HTML) │        │          │               │   implements     │
│                 │        │          │               │   Store          │
│ [+1] [-1] [Reset]◄──────►│  /live   │◄─────────────►│                  │
│                 │        │          │               │ Change(action,   │
│ LiveTemplate    │        │          │               │   data)          │
│ Client JS       │        │          │               │                  │
│                 │        │          │               │ Mount()          │
│ lvt-* attrs     │        │ Auto-    │               │ - Clones state   │
│ - click         │        │ detects  │               │ - Generates      │
│ - submit        │        │ transport│               │   updates        │
│ - change        │        │          │               │ - Broadcasts     │
└─────────────────┘        └──────────┘               └──────────────────┘
```

## Example Update Payloads

**Initial State (counter = 0):**
```json
{
  "s": ["<!DOCTYPE html><html>...", "...</html>"],
  "0": "Live Counter",
  "1": "0",
  "2": "zero",
  "3": "Counter is zero",
  "4": "2025-09-30 00:20:00",
  "5": "session-1727654400"
}
```

**After Increment (only changed values):**
```json
{
  "1": "1",
  "2": "positive",
  "3": "Counter is positive",
  "4": "2025-09-30 00:20:05"
}
```

This demonstrates LiveTemplate's bandwidth efficiency - subsequent updates contain only the 4 changed dynamic values instead of the full HTML document.

## Template Structure

The template follows the same pattern as `testdata/e2e/counter/input.tmpl`:

- **Title**: Dynamic page title
- **Counter Display**: Shows current counter value
- **Status**: Shows "positive", "negative", or "zero"
- **Conditional Text**: Different messages based on counter value
- **Interactive Controls**: Buttons for user actions
- **Metadata**: Last updated timestamp and session ID

## Development Notes

- **Port**: Defaults to `:8080`, can be overridden with `PORT` environment variable
- **Endpoint**: `/live` handles both WebSocket upgrades and HTTP POST requests
- **Template Path**: Reads from `examples/counter/counter.tmpl`
- **Client Library**: Serves `client/dist/livetemplate-client.browser.js` via `internal/testing.ServeClientLibrary()` (development only - use CDN in production)
- **Building Client**: Run `cd client && npm run build` to regenerate the browser bundle
- **State Isolation**: Each WebSocket connection gets its own cloned state
- **Session Management**: HTTP connections use cookie-based sessions for state persistence
- **Error Handling**: Automatic WebSocket reconnection and comprehensive error logging

## Multiple Stores

For applications with multiple state objects, use `MountStores()` with dot notation:

```go
stores := livetemplate.Stores{
    "counter": &CounterState{},
    "user":    &UserState{},
}

http.Handle("/live", livetemplate.MountStores(tmpl, stores))
```

Then use store prefixes in actions:
```html
<button lvt-click="counter.increment">+1</button>
<button lvt-click="user.logout">Logout</button>
```