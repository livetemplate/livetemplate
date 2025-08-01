# StateTemplate Client

A TypeScript browser client library for consuming StateTemplate real-time updates and patching HTML using morphdom.

## Features

- 🚀 **Real-time HTML Updates**: Apply granular DOM updates from StateTemplate server
- 🎯 **Targeted Patching**: Update specific fragments without full page reloads
- ⚡ **High Performance**: Uses morphdom for efficient DOM diffing and patching
- 🛡️ **Type Safe**: Full TypeScript support with comprehensive type definitions
- 🧪 **Well Tested**: Comprehensive unit and e2e test coverage
- 🔧 **Flexible API**: Both instance-based and global convenience functions
- 📦 **Multiple Formats**: UMD and ESM builds for different use cases

## Installation

```bash
npm install @statetemplate/client
```

## Quick Start

### Basic Usage

```typescript
import { StateTemplateClient } from '@statetemplate/client';

// Create client instance
const client = new StateTemplateClient({
  debug: true // Enable debug logging
});

// Set initial page content
client.setInitialContent('<div id="counter">Count: 0</div>');

// Apply real-time updates
const update = {
  fragment_id: 'counter',
  html: '<div id="counter">Count: 42</div>',
  action: 'replace'
};

const result = await client.applyUpdate(update);
if (result.success) {
  console.log('Update applied successfully!');
}
```

### Global Convenience API

```typescript
import { initializeGlobalClient, applyUpdate, setInitialContent } from '@statetemplate/client';

// Initialize global client
initializeGlobalClient({ debug: true });

// Use convenience functions
setInitialContent('<div id="app">Loading...</div>');

await applyUpdate({
  fragment_id: 'app',
  html: '<div id="app">Ready!</div>',
  action: 'replace'
});
```

### WebSocket Integration

```typescript
import { StateTemplateClient } from '@statetemplate/client';

const client = new StateTemplateClient();
const ws = new WebSocket('ws://localhost:8080/updates');

ws.onmessage = async (event) => {
  const message = JSON.parse(event.data);
  
  if (message.type === 'initial') {
    client.setInitialContent(message.html);
  } else if (message.type === 'update') {
    await client.applyUpdate({
      fragment_id: message.fragmentId,
      html: message.html,
      action: message.action
    });
  }
};
```

## API Reference

### StateTemplateClient

#### Constructor

```typescript
new StateTemplateClient(config?: ClientConfig)
```

**ClientConfig Options:**

- `debug?: boolean` - Enable debug logging (default: false)
- `morphOptions?: object` - Custom morphdom options

#### Methods

##### `applyUpdate(update: RealtimeUpdate): Promise<UpdateResult>`

Apply a single real-time update to the DOM.

```typescript
const result = await client.applyUpdate({
  fragment_id: 'my-element',
  html: '<div id="my-element">New content</div>',
  action: 'replace'
});
```

**Supported Actions:**

- `replace` - Replace element content using morphdom
- `append` - Append HTML to element
- `prepend` - Prepend HTML to element  
- `remove` - Remove element from DOM

##### `applyUpdates(updates: RealtimeUpdate[]): Promise<UpdateResult[]>`

Apply multiple updates in sequence.

```typescript
const results = await client.applyUpdates([
  { fragment_id: 'counter', html: '<div>Count: 1</div>', action: 'replace' },
  { fragment_id: 'status', html: '<div>Active</div>', action: 'replace' }
]);
```

##### `setInitialContent(html: string, containerId?: string): void`

Set initial HTML content for the page.

```typescript
client.setInitialContent('<h1>Hello World</h1>', 'app');
```

##### `hasElement(fragmentId: string): boolean`

Check if an element exists in the DOM.

```typescript
if (client.hasElement('my-fragment')) {
  // Element exists
}
```

### Types

#### RealtimeUpdate

```typescript
interface RealtimeUpdate {
  fragment_id: string;  // ID of element to update
  html: string;         // New HTML content
  action: string;       // Action to perform
}
```

#### UpdateResult

```typescript
interface UpdateResult {
  success: boolean;     // Whether update succeeded
  fragmentId: string;   // Fragment ID that was updated
  action: string;      // Action that was performed
  error?: Error;       // Error if update failed
  element?: Element;   // Updated element (if successful)
}
```

### Element Selection

The client finds target elements using:

1. **Element ID**: `document.getElementById(fragment_id)`
2. **Data attribute**: `document.querySelector('[data-fragment-id="fragment_id"]')`

## Advanced Usage

### Custom Morphdom Options

```typescript
const client = new StateTemplateClient({
  morphOptions: {
    onBeforeElUpdated: (fromEl, toEl) => {
      // Custom logic before element update
      return true;
    },
    onNodeAdded: (node) => {
      // Handle added nodes
      return node;
    }
  }
});
```

### Error Handling

```typescript
const result = await client.applyUpdate(update);

if (!result.success) {
  console.error('Update failed:', result.error);
  
  if (result.error instanceof UpdateError) {
    console.log('Fragment ID:', result.error.fragmentId);
    console.log('Action:', result.error.action);
  }
}
```

### Batch Processing

```typescript
// Process updates with error handling
const results = await client.applyUpdates(updates);

const failed = results.filter(r => !r.success);
if (failed.length > 0) {
  console.log(`${failed.length} updates failed`);
}
```

## Examples

### Live Dashboard

```typescript
const client = new StateTemplateClient();

// Initialize dashboard
client.setInitialContent(`
  <div class="dashboard">
    <div id="cpu">CPU: 0%</div>
    <div id="memory">Memory: 0%</div>
    <div id="alerts"></div>
  </div>
`);

// Apply metrics updates
await client.applyUpdates([
  { fragment_id: 'cpu', html: '<div id="cpu">CPU: 45%</div>', action: 'replace' },
  { fragment_id: 'memory', html: '<div id="memory">Memory: 62%</div>', action: 'replace' },
  { fragment_id: 'alerts', html: '<div class="alert">High usage!</div>', action: 'append' }
]);
```

### Chat Application

```typescript
const client = new StateTemplateClient();

// Add new message
await client.applyUpdate({
  fragment_id: 'messages',
  html: '<div class="message">User: Hello!</div>',
  action: 'append'
});

// Update user count
await client.applyUpdate({
  fragment_id: 'user-count',
  html: '<span>5 users online</span>',
  action: 'replace'
});
```

## Browser Support

- Chrome 60+
- Firefox 55+
- Safari 12+
- Edge 79+

## Development

```bash
# Install dependencies
npm install

# Run tests
npm test

# Run tests in watch mode
npm run test:watch

# Run e2e tests
npm run test:e2e

# Build library
npm run build

# Lint code
npm run lint
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

## License

MIT

## Related

- [StateTemplate](https://github.com/livefir/statetemplate) - Go library for real-time template rendering
- [morphdom](https://github.com/patrick-steele-idem/morphdom) - DOM diffing and patching library
