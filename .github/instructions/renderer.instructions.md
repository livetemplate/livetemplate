---
applyTo: "**"
---

# Renderer Component Instructions

## RealtimeRenderer Implementation Guidelines

### Core Responsibilities

- Template registration and parsing
- Fragment extraction and tracking
- Real-time update coordination
- WebSocket-compatible output generation

### Key Data Structures

```go
type RealtimeRenderer struct {
    templates       map[string]*template.Template
    fragmentTracker *FragmentExtractor
    tracker         *TemplateTracker
    currentData     interface{}
    updateChan      chan interface{}
    outputChan      chan RealtimeUpdate
    fragmentStore   map[string][]*TemplateFragment
    rangeFragments  map[string][]*RangeFragment
}
```

### Implementation Patterns

#### Template Registration

- Parse templates immediately upon registration
- Extract fragments during registration phase
- Build dependency maps for change tracking
- Validate template syntax and structure

#### Fragment Management

- Categorize fragments by type (simple, conditional, range, block)
- Maintain fragment-to-data dependency mappings
- Store rendered fragment cache for performance
- Handle fragment lifecycle (creation, update, removal)

#### Real-time Update Processing

- Monitor data changes through TemplateTracker
- Generate minimal update payloads
- Batch related updates for efficiency
- Maintain update ordering for consistency

#### Concurrency Considerations

- Use proper mutex protection for shared state
- Handle goroutine lifecycle management
- Implement graceful shutdown procedures
- Avoid race conditions in fragment updates

### Performance Optimization

- Cache parsed templates to avoid re-parsing
- Pool buffers for rendering operations
- Minimize reflection usage in hot paths
- Batch multiple updates into single WebSocket messages

### Error Handling

- Provide detailed error context for template parsing failures
- Handle missing data gracefully with default values
- Log fragment extraction issues for debugging
- Implement recovery mechanisms for update failures

### WebSocket Integration

- Generate updates in WebSocket-compatible format
- Handle client connection lifecycle events
- Implement proper backpressure handling
- Support multiple concurrent client connections
