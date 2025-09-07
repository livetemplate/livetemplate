# LiveTemplate JavaScript Client

A reusable JavaScript client library for connecting to LiveTemplate WebSocket servers and handling real-time fragment updates.

## Features

- WebSocket connection management
- Automatic page token handling
- Fragment update processing with tree-based reconstruction
- Static content caching for optimal performance
- Generic DOM element updating for any template structure
- Configurable connection options and event handlers

## Usage

### Basic Setup

```html
<script src="livetemplate-client.js"></script>
<script>
    const client = new LiveTemplateClient();
    client.connect();
</script>
```

### Advanced Configuration

```javascript
const client = new LiveTemplateClient({
    port: "8080",
    host: "localhost", 
    protocol: "ws",
    endpoint: "/ws",
    onOpen: () => console.log("Connected!"),
    onClose: () => console.log("Disconnected!"),
    onError: (error) => console.error("Error:", error),
    onMessage: (message) => console.log("Message:", message)
});

client.connect();
```

### Sending Actions

```javascript
// Simple action
client.sendAction('increment');

// Action with data
client.sendAction('update_user', { id: 123, name: 'John' });
```

### HTML Template Requirements

Elements that should receive fragment updates must have `lvt-id` attributes:

```html
<div lvt-id="counter-display">{{.Counter}}</div>
<span lvt-id="user-name" class="{{.UserClass}}">{{.UserName}}</span>
```

## API Reference

### Constructor Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | string | `window.location.port` or `"8080"` | WebSocket server port |
| `host` | string | `"localhost"` | WebSocket server host |
| `protocol` | string | `"ws"` | WebSocket protocol (`"ws"` or `"wss"`) |
| `endpoint` | string | `"/ws"` | WebSocket endpoint path |
| `onOpen` | function | Default console log | Connection opened callback |
| `onClose` | function | Default console log | Connection closed callback |
| `onError` | function | Default console error | Connection error callback |
| `onMessage` | function | `null` | Custom message handler |

### Methods

#### `connect()`
Establishes WebSocket connection to the server.

#### `sendAction(action, data = {})`
Sends an action message to the server.

- `action` (string): Action name
- `data` (object): Optional action data

#### `disconnect()`
Closes WebSocket connection and cleans up resources.

#### `isConnected()`
Returns `true` if WebSocket is connected.

#### `getPageToken()`
Returns the current page token received from server.

## How It Works

1. **Connection**: Client connects to WebSocket server and receives a page token
2. **Fragment Updates**: Server sends fragment updates with tree-based data structures
3. **DOM Updates**: Client reconstructs content from static/dynamic parts and updates DOM elements
4. **Caching**: Static content is cached client-side for optimal bandwidth usage

## Examples

See `example.html` for a complete working example.

For a real application example, see the counter app in `../examples/counter/`.