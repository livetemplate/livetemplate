# LiveTemplate JavaScript Client

JavaScript client library for LiveTemplate, providing real-time DOM updates via WebSocket with tree-based diff optimization.

## 📁 Project Structure

```
client/
├── livetemplate-client.js    # Main client library (ES6)
├── dist/                     # Built distributions
│   ├── livetemplate-client.js      # Development build (unminified + sourcemap)
│   └── livetemplate-client.min.js  # Production build (minified)
├── tests/                    # Test suite
│   ├── LiveTemplateClient.unit.test.js  # Unit tests
│   ├── setup.js             # Jest test setup
│   └── README.md             # Testing documentation
├── coverage/                 # Test coverage reports
├── package.json              # NPM configuration
├── jest.config.js           # Jest test configuration
└── README.md                # This file
```

## 🚀 Quick Start

### Installation

The client can be used in two ways:

#### 1. ES6 Module (Recommended)
```javascript
import LiveTemplateClient from './livetemplate-client.js';
```

#### 2. Browser Global (IIFE)
```html
<script src="dist/livetemplate-client.min.js"></script>
<script>
  // LiveTemplateClient is now available globally
  const client = new LiveTemplateClient();
</script>
```

### Auto-Initialization

The client auto-initializes when:
1. DOM is ready
2. A `<meta name="livetemplate-token" content="{{.Token}}">` tag is found

#### Template Setup
Add this to your HTML template:
```html
<meta name="livetemplate-token" content="{{.Token}}">
```

#### Action Buttons
Use data attributes for automatic action handling:
```html
<!-- Simple action -->
<button data-lvt-action="increment">+</button>

<!-- Action with element capture -->
<input id="todo-input" type="text">
<button data-lvt-action="addTodo" data-lvt-element="todo-input">Add Todo</button>

<!-- Action with JSON parameters -->
<button data-lvt-action="deleteItem" data-lvt-params='{"id": 123}'>Delete</button>
```

## 🔧 Development

### Building

```bash
# Production build (minified)
npm run build

# Development build (with sourcemap)  
npm run build:dev

# Watch mode for development
npm run watch
```

### Testing

```bash
# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Run tests in watch mode
npm run test:watch
```

**Current Test Coverage**: 37% statements, 36% branches, 45% functions
- ✅ 26 unit tests covering core functionality
- ✅ WebSocket connection management
- ✅ Fragment reconstruction logic
- ✅ Static cache operations
- ✅ Error handling and edge cases

## 🎯 Features

- 🚀 **Tree-based optimization** - 92%+ bandwidth reduction
- 💾 **Static content caching** - Reuse cached HTML segments  
- 🔄 **Automatic reconnection** - Exponential backoff on connection loss
- 🎯 **Smart element targeting** - `lvt-id` and fallback to `id`
- 📦 **Morphdom integration** - Efficient DOM updates preserving state
- 🛡️ **Error resilience** - Graceful handling of malformed data

## 📡 Tree-Based Updates

LiveTemplate uses tree-based optimization for minimal data transfer:

```javascript
// Server sends minimal diff data
{
  "s": ["<p>Hello ", "!</p>"],  // Static HTML segments (cached)
  "0": "World"                  // Dynamic values by position
}

// Client reconstructs: "<p>Hello World!</p>"
```

## 🔌 Manual Usage

```javascript
const client = new LiveTemplateClient({
  wsUrl: 'ws://localhost:8080/ws',
  onOpen: () => console.log('Connected'),
  onError: (error) => console.error('Error:', error),
  onFragmentUpdate: (fragment, element) => {
    console.log(`Updated fragment ${fragment.id}`);
  }
});

// Connect with page token
client.connect('your-page-token');

// Send actions
client.sendAction('updateCounter', { value: 42 });

// Disconnect
client.disconnect();
```

## 📚 API Reference

### Constructor Options
- `wsUrl` - WebSocket URL (auto-detected from location)
- `maxReconnectAttempts` - Max reconnection attempts (default: 5)
- `reconnectDelay` - Initial reconnect delay in ms (default: 1000)
- `onOpen` - Connection opened callback
- `onClose` - Connection closed callback  
- `onError` - Error callback
- `onFragmentUpdate` - Fragment update callback

### Methods
- `connect(token)` - Connect with page token
- `disconnect()` - Close connection and cleanup
- `sendAction(action, data)` - Send action to server
- `applyFragments(fragments)` - Apply fragment updates to DOM

## 🌐 Browser Support

- Modern browsers with WebSocket support
- ES6+ features (use build for older browsers)
- DOM manipulation APIs (querySelector, morphdom)

## 📄 Dependencies

- **morphdom** - Efficient DOM diffing and patching
- **esbuild** - Fast JavaScript bundler (dev dependency)
- **jest** - JavaScript testing framework (dev dependency)

## 🏗️ Architecture

```
LiveTemplate Client
├── WebSocket Connection (/ws?token=...)  
├── Static Cache (Map<fragmentId, statics[]>)
├── Fragment Processing (diff.Update format)
└── DOM Updates (morphdom)
```

The client is designed to work exclusively with Go's `diff.Update` structure, providing maximum efficiency and tree-based optimization for real-time web applications.