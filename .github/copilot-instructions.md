# StateTemplate - Real-time Go Template Rendering Library

## Project Overview

StateTemplate is a high-performance Go library for real-time HTML template rendering with granular fragment updates. It enables live updates to specific parts of rendered templates without full page reloads, making it ideal for building responsive web applications with WebSocket integration.

The library processes Go templates into trackable fragments, monitors data changes, and generates minimal update payloads for efficient real-time synchronization between server and client.

## Architecture Components

### Core Components

- **RealtimeRenderer** (`realtime_renderer.go`): Main orchestrator managing template parsing, fragment tracking, and real-time updates
- **TemplateTracker** (`template_tracker.go`): Monitors data dependencies and detects changes using reflection
- **FragmentExtractor** (`fragment_extractor.go`): Extracts and categorizes template fragments (simple, conditional, range, block)
- **TemplateAnalyzer** (`template_analyzer.go`): Provides advanced template analysis and optimization

### Key Data Structures

- `RealtimeRenderer`: Central coordinator with template maps, fragment stores, and update channels
- `TemplateFragment`: Represents extractable template segments with dependency tracking
- `RangeFragment`: Special handling for loop constructs with granular item updates
- `RealtimeUpdate`: Update payload structure for WebSocket transmission

## Folder Structure

```
statetemplate/
├── realtime_renderer.go           # Main renderer orchestrator
├── template_tracker.go            # Data change tracking and dependency analysis
├── fragment_extractor.go          # Fragment extraction and categorization
├── template_analyzer.go           # Advanced template analysis
├── template_actions_tdd_test.go   # Comprehensive TDD test suite
├── examples/                      # Usage examples and demos
│   ├── simple/                    # Basic usage patterns
│   └── advanced/                  # Complex scenarios and integrations
├── docs/                          # Documentation
│   └── ARCHITECTURE.md            # Detailed architectural documentation
└── .github/                       # GitHub configuration
    └── copilot-instructions.md    # Repository instructions for Copilot
```

## Coding Standards and Best Practices

### Documentation Organization

- **MANDATORY**: All documentation and markdown files (except README.md) MUST be created in the `docs/` directory
- When creating new documentation, ALWAYS use `docs/` as the target location
- When editing existing documentation outside `docs/`, move it to `docs/` first
- This includes: architectural docs, API docs, design documents, technical specifications, implementation guides
- The only markdown file allowed in the root directory is `README.md`

### Code Organization

- **MANDATORY - NO cmd/ DIRECTORY**: Never create a `cmd/` directory for examples or debug code. All code must go in designated locations:
  - **Examples/Demos**: MUST be in `examples/` directory with descriptive subdirectory names
  - **Debug code**: MUST be in test files as debug test functions, not separate executables
  - **Transient debug code**: Delete immediately after debugging - do not commit
- **MANDATORY - Examples Organization**:
  - All examples and demo code MUST be in `examples/` directory only
  - Use descriptive subdirectory names: `examples/simple/`, `examples/realtime/`, etc.
  - E2E tests for examples MUST be in `examples/e2e/` directory
  - No example or demo code in any other directories (not in `cmd/`, not in root, not in test files)
  - Convert any debug executables to either examples or test cases
- **MANDATORY - Debug Code Policy**:
  - Debug code should be temporary test functions in `*_test.go` files
  - Use names like `TestDebug_SpecificIssue` for temporary debugging
  - Delete debug test functions after debugging is complete
  - Never create separate debug executables or main.go files for debugging

### Milestone Success Criteria

- **MANDATORY - MILESTONE SUCCESS CRITERIA**: 
  - No milestone can be marked as successful unless `./scripts/validate-ci.sh` passes completely without any issues
  - This includes: all tests passing, code formatting, go vet, golangci-lint, and go mod tidy
  - Any code changes MUST be validated by running `./scripts/validate-ci.sh` before considering work complete
  - Git pre-commit hooks automatically enforce this, but manual validation is also required

### Go Code Conventions

- Follow standard Go formatting with `gofmt`
- Use descriptive variable names reflecting template rendering concepts
- Implement comprehensive error handling with structured error types
- Use table-driven tests for systematic coverage of template actions
- Employ TDD methodology for new features and bug fixes

### Template Fragment Patterns

- Simple fragments: `{{.Field}}` for direct field access
- Conditional fragments: `{{if .Condition}}...{{end}}` with dependency tracking
- Range fragments: `{{range .Items}}...{{end}}` with granular item updates
- Block fragments: `{{block "name" .}}...{{end}}` for template composition

### Test Organization

- Use table-driven test suites with descriptive names (e.g., `CommentTestSuite`, `PipelineTestSuite`)
- Structure test cases with `name`, `template`, `data`, and `expected` fields
- Group related template actions into cohesive test suites
- Include edge cases and error scenarios in test coverage

## Libraries and Dependencies

### Core Dependencies

- **Go standard library**: `text/template`, `html/template` for template processing
- **Reflection**: `reflect` package for dynamic data analysis and change detection
- **Concurrency**: Goroutines and channels for real-time update processing
- **WebSocket support**: Compatible with standard WebSocket libraries

### Development Dependencies

- **Testing**: Go's built-in testing framework with table-driven patterns
- **Benchmarking**: Performance testing for fragment extraction and rendering
- **Documentation**: Markdown for architectural documentation and examples

## Real-time Update Flow

### Template Processing Pipeline

1. **Registration**: Templates parsed and analyzed for extractable fragments
2. **Dependency Mapping**: Data fields mapped to dependent template fragments
3. **Change Detection**: Data updates monitored through reflection-based tracking
4. **Fragment Updates**: Only changed fragments re-rendered for efficiency
5. **WebSocket Delivery**: Minimal update payloads sent to connected clients

### Fragment Types and Handling

- **Simple fragments**: Direct field substitutions with straightforward dependency tracking
- **Conditional fragments**: If/with blocks that may appear or disappear based on data
- **Range fragments**: Loop constructs requiring granular item-level tracking
- **Block fragments**: Named template sections for composition and inheritance

## Performance Considerations

### Optimization Strategies

- **Fragment caching**: Cache rendered fragments to avoid re-computation
- **Batch updates**: Group multiple changes into single WebSocket messages
- **Differential rendering**: Only process and send changed template portions
- **Memory pooling**: Reuse buffers and data structures for high-throughput scenarios

### Concurrency Safety

- Be aware that some operations may not be fully thread-safe
- Use proper synchronization when accessing shared template and fragment state
- Consider goroutine lifecycle management for long-running real-time scenarios

## Common Template Actions and Patterns

When working with template actions, refer to the comprehensive test suite in `template_actions_tdd_test.go`:

### Supported Template Actions

- **Comments**: `{{/* comment */}}` - ignored in output but tracked for completeness
- **Variables**: `{{$var := .Field}}` - local variable assignment with scope tracking
- **Pipelines**: `{{.Field | func}}` - function application chains
- **Conditionals**: `{{if}}/{{else}}/{{end}}` - branching logic with dependency analysis
- **Loops**: `{{range}}/{{with}}` - iteration and context switching
- **Functions**: `{{call}}/{{len}}/{{index}}` - built-in and custom function invocation
- **Comparisons**: `{{eq}}/{{ne}}/{{lt}}/{{gt}}` - logical operations
- **Blocks**: `{{block}}/{{template}}` - template composition and inheritance

### Fragment Extraction Patterns

- Identify boundaries of extractable template segments
- Track data dependencies for each fragment type
- Handle nested structures and complex template hierarchies
- Optimize for minimal update payloads in real-time scenarios

## Security Considerations

### Template Security

- Relies on Go's built-in template auto-escaping for HTML safety
- XSS prevention through automatic escaping of dynamic content
- Template injection protection via Go template sandbox restrictions

### WebSocket Security

- No built-in authentication (application responsibility)
- No built-in rate limiting for real-time updates
- Minimal input validation on incoming data updates

## Testing Guidelines

### Table-Driven Test Structure

- Organize tests into logical suites based on template action types
- Use consistent test case structure: `name`, `template`, `data`, `expected`
- Include both positive and negative test scenarios
- Test edge cases like empty data, nil values, and malformed templates

### TDD Development Process

- Write failing tests first to define expected behavior
- Implement minimal code to make tests pass
- Refactor for maintainability while keeping tests green
- Add comprehensive test coverage for new template actions and fragment types

## Integration Patterns

### WebSocket Integration

- Design for minimal update payloads to reduce bandwidth
- Handle connection lifecycle events (connect, disconnect, reconnect)
- Implement proper error handling for network failures
- Consider template rendering optimization strategies

### Real-time Application Patterns

- Use fragment IDs for precise HTML updates
- Implement efficient data change detection
- Handle race conditions between data updates and template rendering
- Design for horizontal scaling with multiple server instances
