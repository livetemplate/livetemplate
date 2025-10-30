# LiveTemplate TypeScript Client

A TypeScript client for consuming LiveTemplate tree-based updates, implementing Phoenix LiveView-style optimization.

## 🚀 Features

- **Tree-based Updates**: Consume optimized JSON updates from LiveTemplate server
- **Static Structure Caching**: Cache static HTML structure client-side for maximum efficiency
- **Phoenix LiveView Compatible**: Only dynamic values transmitted after initial render
- **Bandwidth Optimization**: 75%+ reduction in update payload sizes
- **Type Safety**: Full TypeScript support with type definitions

## 📦 Installation

```bash
npm install
npm run build
```

## 🧪 Testing

The client includes comprehensive tests to validate the optimization effectiveness:

```bash
# Run optimization validation tests
npm run test:optimization

# Run HTML reconstruction tests
npm run test:reconstruction

# Run all tests
npm run test:all
```

## 📊 Test Results

Current optimization performance:

- **Update 1**: 168 bytes (first update after initial render)
- **Update 2**: 128 bytes (subsequent optimized update)
- **Bandwidth Savings**: ~75.3% vs full HTML updates
- **Static Structure**: Successfully excluded from updates ✅

## 🛠️ Usage

### Basic Client Usage

```typescript
import { LiveTemplateClient } from "./livetemplate-client";

const client = new LiveTemplateClient();

// Apply initial update (includes static structure)
const initialResult = client.applyUpdate({
  s: ["<h1>", "</h1><p>Count: ", "</p>"], // Static HTML segments
  "0": "Hello World", // Dynamic content
  "1": "42", // Dynamic content
});

console.log(initialResult.html); // "<h1>Hello World</h1><p>Count: 42</p>"

// Apply subsequent update (only changed dynamic values)
const updateResult = client.applyUpdate({
  "1": "43", // Only the changed value
});

console.log(updateResult.html); // "<h1>Hello World</h1><p>Count: 43</p>"
console.log(updateResult.changed); // true
```

### Loading Updates from Files

```typescript
import { loadAndApplyUpdate } from "./livetemplate-client";

const client = new LiveTemplateClient();

// Load update from JSON file
const result = await loadAndApplyUpdate(client, "update_01.json");
console.log(result.html);
```

### HTML Comparison

```typescript
import { compareHTML } from "./livetemplate-client";

const comparison = compareHTML(expectedHTML, actualHTML);
if (comparison.match) {
  console.log("✅ HTML matches!");
} else {
  console.log("❌ Differences found:", comparison.differences);
}
```

## 🏗️ Architecture

### Tree-Based Updates

LiveTemplate uses a tree-based approach where:

1. **Static Structure** (`"s"` key): HTML segments sent once and cached client-side
2. **Dynamic Values** (numbered keys): Only the values that change between updates
3. **Segment Interleaving**: Client reconstructs HTML by interleaving static + dynamic

Example update structure:

```json
{
  "s": ["<h1>", "</h1><div>Count: ", "</div>"], // Static HTML (sent once)
  "0": "Task Manager", // Dynamic: page title
  "1": "42" // Dynamic: counter value
}
```

### Optimization Strategy

Following Phoenix LiveView's approach:

- **Initial Render**: Full HTML + cached static structure
- **Subsequent Updates**: Only changed dynamic values (75%+ bandwidth savings)
- **Client Reconstruction**: Merge updates with cached structure
- **DOM Morphing**: Let morphdom handle efficient DOM updates

## 🧪 Test Data

The test suite validates optimization using real E2E test data:

- `testdata/e2e/update_01_add_todos.json` - First optimized update (168 bytes)
- `testdata/e2e/update_02_remove_todo.json` - Subsequent update (128 bytes)
- `testdata/e2e/rendered_*.html` - Expected HTML output for comparison

## 🔧 API Reference

### LiveTemplateClient

#### `applyUpdate(update: TreeNode): UpdateResult`

Apply a tree-based update to the client state.

- **Parameters**: `update` - Tree update object from LiveTemplate server
- **Returns**: `{ html: string, changed: boolean }`

#### `reset(): void`

Reset client state (useful for testing).

#### `getState(): { static: string[] | null, dynamic: object }`

Get current cached state for debugging.

### Utility Functions

#### `loadAndApplyUpdate(client, path): Promise<UpdateResult>`

Load update from JSON file and apply to client.

#### `compareHTML(expected, actual): { match: boolean, differences: string[] }`

Compare two HTML strings, ignoring whitespace differences.

## 🚀 Performance

Optimization results with real E2E test data:

| Update Type  | Size (bytes) | Bandwidth Savings |
| ------------ | ------------ | ----------------- |
| Full HTML    | ~600         | 0% (baseline)     |
| Optimized #1 | 168          | 72%               |
| Optimized #2 | 128          | 79%               |
| **Average**  | **148**      | **~75.3%**        |

## 📈 Future Enhancements

- Browser-based DOM morphing integration
- WebSocket client for real-time updates
- React/Vue.js integration hooks
- Advanced diff algorithms for complex nested structures
- Performance monitoring and metrics
