# LiveTemplate Counter Example

A real-time counter application demonstrating LiveTemplate's tree-based optimization over WebSocket.

## Features

- **Real-time updates**: Counter changes are sent via WebSocket using LiveTemplate's differential updates
- **Minimal bandwidth**: Only the changed values are transmitted, not the entire HTML
- **No custom JavaScript**: Uses only the LiveTemplate client library
- **Template-based**: HTML is generated from Go templates with conditional rendering
- **WebSocket communication**: Interactive buttons trigger server-side state changes

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

- **Single Template Instance**: One parsed template used for both HTTP and WebSocket
- **State Management**: `CounterState` struct tracks counter value, status, and metadata
- **HTTP Handler**: Serves initial HTML with `Execute()`
- **WebSocket Handler**: Sends initial tree on connect, then differential updates with `ExecuteUpdates()`
- **Conditional Logic**: Template shows different messages based on counter value (positive/negative/zero)

**Note**: For production with multiple concurrent users, each WebSocket connection should have its own template instance to avoid state conflicts.

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
- **Automatic WebSocket connection** to `/ws` endpoint
- **Automatic reconnection** on disconnect (configurable)
- **Automatic DOM updates** when updates arrive
- **Event delegation** - works with dynamically updated elements

#### Supported lvt-* attributes:
- `lvt-click` - Handle click events
- `lvt-submit` - Handle form submissions (prevents default)
- `lvt-change` - Handle input change events
- `lvt-input` - Handle input events (real-time)
- `lvt-keydown` - Handle keydown events
- `lvt-keyup` - Handle keyup events
- `lvt-data-*` - Include custom data in messages

### LiveTemplate Integration

- **Tree-based Updates**: Only changed dynamic values are sent over the wire
- **Static Content Caching**: HTML structure is cached client-side
- **Differential Updates**: Bandwidth savings of 90%+ compared to full page refreshes
- **Conditional Rendering**: Template conditionals are handled automatically

## Architecture

```
Browser                    WebSocket                    Go Server
┌─────────────────┐        ┌─────────┐                ┌──────────────────┐
│ counter.tmpl    │        │         │                │ CounterState     │
│ (rendered HTML) │        │         │                │ - Counter: int   │
│                 │        │         │                │ - Status: string │
│ [+1] [-1] [Reset]◄──────►│   /ws   │◄──────────────►│ - LastUpdated   │
│                 │        │         │                │ - SessionID      │
│ LiveTemplate    │        │         │                │                  │
│ Client JS       │        │         │                │ Template Engine  │
└─────────────────┘        └─────────┘                └──────────────────┘
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
- **CORS**: Configured to allow all origins for development (change for production)
- **Template Path**: Reads from `examples/counter/counter.tmpl`
- **Client Library**: Serves `client/dist/livetemplate-client.browser.js` (browser-compatible IIFE bundle)
- **Building Client**: Run `cd client && npm run build` to regenerate the browser bundle
- **Error Handling**: WebSocket reconnection and error logging included