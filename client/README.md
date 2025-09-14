# LiveTemplate JavaScript Client

A unified WebSocket-based client for LiveTemplate tree-based diff updates. Works exclusively with the `diff.Update` format from the Go backend.

## Features

- ✅ **Unified**: Single client for all LiveTemplate updates
- ✅ **Tree-based**: Works with `diff.Update` structure only
- ✅ **Efficient**: Caches static segments for bandwidth optimization
- ✅ **Modern**: ES6 modules with morphdom for DOM updates
- ✅ **Bundled**: Includes all dependencies in a single file

## Quick Start

### Use the Pre-built Bundle

```html
<script src="/dist/livetemplate-client.min.js"></script>
<script>
  const client = new LiveTemplateClient();
  const token = document.querySelector('meta[name="page-token"]').content;
  client.connect(token);
</script>
```

### WebSocket Connection

The client automatically connects to `/ws` on the current host:

```javascript
const client = new LiveTemplateClient({
  onOpen: () => console.log('Connected!'),
  onFragmentUpdate: (fragment, element) => {
    console.log('Fragment updated:', fragment.id);
  }
});

client.connect(pageToken);
```

### Sending Actions

```javascript
// Simple action
client.sendAction('increment');

// Action with data
client.sendAction('update_user', { name: 'Alice', age: 30 });
```

## How It Works

1. **Static Caching**: First update includes static HTML segments (`s` array)
2. **Dynamic Updates**: Subsequent updates only send dynamic values
3. **Reconstruction**: Client reconstructs full content from cached statics + new dynamics
4. **DOM Updates**: Uses morphdom to efficiently update only changed elements

### Fragment Format

The client expects fragments in this format:

```javascript
{
  "1": {           // Fragment ID
    "s": ["<div style=\"color: ", ";\">Hello ", " World</div>"],  // Static segments (cached)
    "0": "#ff6b6b", // Dynamic value at position 0
    "1": "42"       // Dynamic value at position 1  
  }
}
```

## Building

```bash
# Development build (with sourcemap)
npm run build:dev

# Production build (minified)
npm run build

# Watch mode
npm run watch
```

## API Reference

### Constructor Options

```javascript
new LiveTemplateClient({
  wsUrl: 'ws://localhost:8080/ws',  // Custom WebSocket URL
  maxReconnectAttempts: 5,          // Reconnection limit
  reconnectDelay: 1000,             // Base reconnection delay (ms)
  onOpen: () => {},                 // Connection opened callback
  onClose: (event) => {},           // Connection closed callback  
  onError: (error) => {},           // Error callback
  onFragmentUpdate: (fragment, element) => {} // Fragment update callback
});
```

### Methods

- `connect(token)` - Connect with page token
- `sendAction(action, data)` - Send action to server
- `disconnect()` - Close connection and clear cache

### Static Caching

The client automatically caches static HTML segments from the server:
- First fragment update: Contains both static segments and dynamic values
- Subsequent updates: Only dynamic values (92%+ bandwidth savings)
- Cache is scoped per fragment ID for isolation

## Architecture

```
LiveTemplate Client
├── WebSocket Connection (/ws?token=...)  
├── Static Cache (Map<fragmentId, statics[]>)
├── Fragment Processing (diff.Update format)
└── DOM Updates (morphdom)
```

The client is designed to work exclusively with Go's `diff.Update` structure, providing maximum efficiency and simplicity.