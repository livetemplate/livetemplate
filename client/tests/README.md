# LiveTemplate Client Tests

This directory contains comprehensive unit and integration tests for the LiveTemplate JavaScript client library.

## Test Structure

### Unit Tests
- **LiveTemplateClient.unit.test.js**: Core functionality tests for the LiveTemplate client
  - Constructor and configuration
  - WebSocket connection management  
  - Fragment reconstruction logic
  - Static cache management
  - Action sending
  - Message parsing
  - Error handling and validation

## Test Coverage

Current test coverage: **37% statements, 36% branches, 45% functions**

The tests focus on:
- ✅ Core API functionality
- ✅ Message parsing and fragment handling
- ✅ Static cache operations
- ✅ WebSocket connection lifecycle
- ✅ Error handling and edge cases

## Running Tests

```bash
# Run all tests
npm test

# Run tests with coverage
npm run test:coverage

# Run tests in watch mode
npm run test:watch
```

## Test Framework

- **Jest**: Testing framework with built-in assertions, mocking, and coverage
- **jsdom**: DOM environment for testing browser functionality
- **Babel**: ES6+ transpilation for test files

## Test Philosophy

The tests are designed to:
1. **Validate core functionality** without requiring real DOM manipulation
2. **Mock external dependencies** (WebSocket, DOM) for isolated testing
3. **Cover edge cases** and error scenarios
4. **Ensure backward compatibility** as the library evolves

## Coverage Areas

### Covered ✅
- Fragment reconstruction from diff updates
- Static content caching and reuse
- WebSocket message parsing (array and object formats)
- Action data collection and sending
- Error handling for malformed data
- Connection lifecycle management

### Future Coverage Opportunities
- Real DOM manipulation with morphdom
- WebSocket integration with actual server
- Auto-initialization functionality
- Browser event handling
- Performance benchmarks

## Test Data Formats

The tests validate the core LiveTemplate diff.Update format:

```javascript
// Simple text update
{
  s: ['<p>Hello ', '!</p>'],    // Static segments
  '0': 'World'                  // Dynamic values by position
}

// Multi-dynamic update  
{
  s: ['<div>', ' - ', '</div>'],
  '0': 'First',
  '1': 'Second'
}
```

## Mocking Strategy

- **WebSocket**: Mocked to avoid network dependencies
- **DOM APIs**: Selectively mocked to test logic without full DOM
- **morphdom**: Mocked for unit tests, real for integration tests
- **console**: Mocked to reduce test output noise

## Adding New Tests

When adding new functionality:

1. **Add unit tests first** for core logic
2. **Mock external dependencies** to isolate functionality  
3. **Test error conditions** and edge cases
4. **Validate input/output formats** match the LiveTemplate protocol
5. **Update coverage expectations** as needed