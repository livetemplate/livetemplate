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

- **Template**: Uses `counter.tmpl` matching the E2E test pattern
- **State Management**: `CounterState` struct tracks counter value, status, and metadata
- **WebSocket Handler**: Receives action messages and sends LiveTemplate updates
- **Conditional Logic**: Template shows different messages based on counter value (positive/negative/zero)

### Client Side (JavaScript)

- **LiveTemplate Client**: Loads `livetemplate-client.js` for HTML patching
- **WebSocket Connection**: Connects to `/ws` endpoint for real-time communication
- **Interactive Buttons**: Send simple text messages (`increment`, `decrement`, `reset`)
- **Automatic Updates**: Received updates are applied to the DOM automatically

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