# LiveTemplate Architecture

## Overview

LiveTemplate is a real-time template rendering library for Go that enables granular, fragment-based updates for web applications. It extends Go's standard `html/template` package with advanced fragment extraction, dependency tracking, and real-time update capabilities.

> **Note**: This document describes the current architecture of LiveTemplate v1.x. For API changes and migration guides, see [API_DESIGN.md](API_DESIGN.md).

## Core Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        WEB[Web Application]
        WS[WebSocket Client]
    end

    subgraph "LiveTemplate Core"
        RT[Renderer]
        TT[TemplateTracker]
        FE[FragmentExtractor]
        TA[TemplateAnalyzer]
    end

    subgraph "Template Processing"
        TP[Template Parser]
        FG[Fragment Generator]
        DT[Dependency Tracker]
    end

    subgraph "Update Pipeline"
        UC[Update Channel]
        UP[Update Processor]
        FU[Fragment Updater]
    end

    WEB --> RT
    WS --> RT
    RT --> TT
    RT --> FE
    RT --> TA
    TT --> TP
    FE --> FG
    TA --> DT
    RT --> UC
    UC --> UP
    UP --> FU
    FU --> WS
```

## Core Components

### 1. Renderer

**Location**: `realtime_renderer.go`
**Purpose**: Main orchestrator for real-time template rendering

**Key Responsibilities**:

- Template management and parsing
- Fragment creation and tracking
- Real-time update processing
- WebSocket-compatible output generation

```go
type Renderer struct {
    templates       map[string]*template.Template
    fragmentStore   map[string][]*templateFragment
    rangeFragments  map[string][]*rangeFragment
    currentData     interface{}
    updateChan      chan interface{}
    outputChan      chan Update
    // ... other fields
}
```

### 2. templateTracker

**Location**: `template_tracker.go`
**Purpose**: Tracks data dependencies and changes

**Key Features**:

- Deep field dependency analysis
- Change detection through reflection
- Dependency graph construction

### 3. fragmentExtractor

**Location**: `fragment_extractor.go`
**Purpose**: Extracts fragments from templates

**Fragment Types**:

- **Simple Fragments**: Single field outputs (`{{.Field}}`)
- **Conditional Fragments**: If/with blocks
- **Range Fragments**: Loop constructs with granular item tracking
- **Block Fragments**: Named template blocks

### 4. advancedTemplateAnalyzer

**Location**: `advanced_analyzer.go`
**Purpose**: Advanced template analysis and optimization

## Data Flow Architecture

### Initial Rendering Flow

```mermaid
sequenceDiagram
    participant Client
    participant Renderer
    participant TemplateParser
    participant fragmentExtractor
    participant templateTracker

    Client->>Renderer: Parse/ParseFiles/ParseGlob/ParseFS
    Renderer->>TemplateParser: Parse template
    TemplateParser->>fragmentExtractor: Extract fragments
    fragmentExtractor->>Renderer: Return fragments
    Renderer->>templateTracker: Register dependencies

    Client->>Renderer: SetInitialData(data)
    Renderer->>Renderer: renderFullHTML()
    Renderer->>Renderer: wrapRenderedFragments()
    Renderer->>Client: Return wrapped HTML
```

### Real-time Update Flow

```mermaid
sequenceDiagram
    participant Client
    participant Renderer
    participant UpdateProcessor
    participant FragmentUpdater
    participant WebSocketClient

    Client->>Renderer: SendUpdate(newData)
    Renderer->>UpdateProcessor: Process data change
    UpdateProcessor->>UpdateProcessor: Compare old vs new data
    UpdateProcessor->>FragmentUpdater: Identify affected fragments
    FragmentUpdater->>FragmentUpdater: Render fragment updates
    FragmentUpdater->>WebSocketClient: Send Update
    WebSocketClient->>Client: Apply DOM updates
```

## Fragment Architecture

### Fragment Types and Processing

```mermaid
graph LR
    subgraph "Fragment Types"
        SF[Simple Fragments]
        CF[Conditional Fragments]
        RF[Range Fragments]
        BF[Block Fragments]
        IF[Include Fragments]
    end

    subgraph "Processing Pipeline"
        FE[Fragment Extraction]
        DM[Dependency Mapping]
        ID[ID Generation]
        HW[HTML Wrapping]
    end

    subgraph "Update Types"
        REP[Replace]
        APP[Append]
        PRE[Prepend]
        REM[Remove]
        INS[Insert]
    end

    SF --> FE
    CF --> FE
    RF --> FE
    BF --> FE
    IF --> FE

    FE --> DM
    DM --> ID
    ID --> HW

    HW --> REP
    HW --> APP
    HW --> PRE
    HW --> REM
    HW --> INS
```

### Range Fragment Architecture

Range fragments enable granular list operations with enhanced capabilities for sorting, reordering, and dynamic list management:

```go
type rangeFragment struct {
    *templateFragment
    RangePath    string                // "Navigation.MainItems"
    ItemTemplate string                // Template for individual items
    Items        map[string]*rangeItem // Current items by key
    ContainerID  string                // Container element ID
}

type rangeItem struct {
    ID    string      // Unique fragment ID
    Index int         // Array position
    Key   string      // Unique key (URL, ID field)
    Data  interface{} // Item data
    HTML  string      // Rendered HTML
}
```

#### Enhanced Range Operations

LiveTemplate supports advanced range operations with stable, key-based fragment IDs:

- **Stable IDs**: Range items use key-based IDs (`container-item-/task/1`) instead of index-based ones
- **Precise Positioning**: `insertafter` and `insertbefore` actions for exact placement
- **Minimal DOM Changes**: Only affected items are updated, preserving scroll and focus states

#### Supported Range Actions

| Action         | Description                             | Use Case                     |
| -------------- | --------------------------------------- | ---------------------------- |
| `insertafter`  | Insert element after reference element  | Precise positioning, sorting |
| `insertbefore` | Insert element before reference element | Precise positioning, sorting |
| `prepend`      | Insert at beginning of container        | Move to first position       |
| `append`       | Insert at end of container              | Move to last position        |
| `remove`       | Remove element                          | Delete items                 |
| `replace`      | Replace element content                 | Update existing items        |

#### Range Update Examples

**Sorting Lists:**

```go
// Before: [Task A (Priority 3), Task B (Priority 1), Task C (Priority 2)]
// After:  [Task B (Priority 1), Task C (Priority 2), Task A (Priority 3)]
// Result: Precise insertafter/insertbefore actions to reorder items
```

**Dynamic Insertion:**

```go
// Insert new task at specific position
// Result: insertafter action with RangeInfo.ReferenceID for positioning
```

## Update System Architecture

### Update Processing Pipeline

```mermaid
graph TD
    ND[New Data] --> DC[Data Comparison]
    DC --> CF[Changed Fields]
    CF --> FA[Find Affected Fragments]
    FA --> FT{Fragment Type}

    FT -->|Simple| SF[Simple Fragment Update]
    FT -->|Conditional| CF2[Conditional Fragment Update]
    FT -->|Range| RF[Range Fragment Update]
    FT -->|Block| BF[Block Fragment Update]

    SF --> RU[Update]
    CF2 --> RU
    RF --> RU2[Multiple Updates]
    BF --> RU

    RU --> WS[WebSocket]
    RU2 --> WS
    WS --> DOM[DOM Updates]
```

### Update Structure

```go
type Update struct {
    FragmentID  string `json:"fragment_id"`
    HTML        string `json:"html"`
    Action      string `json:"action"` // replace, append, remove, etc.
    *RangeInfo  `json:"range,omitempty"`
}

type RangeInfo struct {
    ItemKey     string `json:"item_key"`
    ReferenceID string `json:"reference_id,omitempty"`
}
```

## Template Analysis Engine

### Dependency Detection

```mermaid
graph LR
    subgraph "Template Analysis"
        TA[Template AST]
        PE[Pipeline Extraction]
        FD[Function Detection]
        VT[Variable Tracking]
    end

    subgraph "Dependency Types"
        SF[Simple Fields]
        NF[Nested Fields]
        CF[Computed Fields]
        FF[Function Fields]
    end

    TA --> PE
    PE --> FD
    FD --> VT

    PE --> SF
    PE --> NF
    VT --> CF
    FD --> FF
```

### Advanced Features

1. **Block Fragment Detection**: Identifies named template blocks
2. **Conditional Logic Analysis**: Tracks if/with/range dependencies
3. **Function Call Analysis**: Detects template functions (eq, gt, len, etc.)
4. **Variable Scope Tracking**: Handles template variable assignments
5. **Cross-Template Dependencies**: Tracks template inclusions

## File Structure

```text
livetemplate/
├── realtime_renderer.go     # Main renderer orchestrator
├── template_tracker.go      # Data change tracking
├── fragment_extractor.go    # Fragment extraction logic
├── advanced_analyzer.go     # Advanced template analysis
├── examples/                # Usage examples and demos
│   ├── comprehensive-demo/  # Complete application examples
│   ├── range-demo/          # Range fragment demonstrations
│   └── e2e/                 # End-to-end tests
├── docs/                    # Documentation
│   ├── ARCHITECTURE.md      # This file
│   ├── API_DESIGN.md        # Public API design
│   └── README.md            # Examples guide
├── testdata/                # Test template files
└── scripts/                 # Build and validation scripts
```

## Technical Debt Analysis

### High Priority Issues

1. **Fragment ID Generation**

   - **Issue**: Current ID generation is timestamp-based, may not be deterministic
   - **Impact**: Difficult to test, potential conflicts in high-concurrency scenarios
   - **Recommendation**: Implement content-based hashing for deterministic IDs

2. **Complex Fragment Detection Logic**

   - **Issue**: Multiple overlapping approaches for fragment detection
   - **Location**: `realtime_renderer.go:createSimpleFragments()`
   - **Impact**: Hard to maintain, potential for inconsistent behavior
   - **Recommendation**: Consolidate into unified fragment detection strategy

3. **Memory Management**

   - **Issue**: Fragment store and range fragments not cleaned up
   - **Impact**: Memory leaks in long-running applications
   - **Recommendation**: Implement fragment cleanup and lifecycle management

4. **Error Handling**
   - **Issue**: Inconsistent error handling across components
   - **Impact**: Hard to debug, potential for silent failures
   - **Recommendation**: Standardize error handling with structured logging

### Medium Priority Issues

1. **Concurrency Safety**

   - **Issue**: Some operations may not be fully thread-safe
   - **Location**: Fragment updates and template parsing
   - **Recommendation**: Comprehensive concurrency audit and mutex improvements

2. **Template Parsing Performance**

   - **Issue**: Templates re-parsed on each addition
   - **Impact**: Performance degradation with many templates
   - **Recommendation**: Implement template caching and parsing optimization

3. **Dependency Resolution Complexity**
   - **Issue**: Complex nested dependency resolution logic
   - **Location**: `template_analyzer.go`
   - **Recommendation**: Simplify with clearer algorithms and better testing

### Low Priority Issues

1. **Code Duplication**

   - **Issue**: Similar patterns repeated across fragment types
   - **Recommendation**: Extract common patterns into shared utilities

2. **Configuration Management**
   - **Issue**: Limited configuration options for advanced use cases
   - **Recommendation**: Expand configuration system for customization

## Test Coverage Gaps

### Critical Coverage Gaps

1. **Concurrency Testing**

   - **Missing**: Multi-threaded access to Renderer
   - **Required**: Race condition detection, concurrent update handling

2. **Error Scenarios**

   - **Missing**: Malformed template handling, invalid data structures
   - **Required**: Comprehensive error path testing

3. **Performance Testing**

   - **Missing**: Large dataset handling, memory usage patterns
   - **Required**: Benchmark tests for scalability validation

4. **Fragment Edge Cases**
   - **Missing**: Deeply nested structures, circular references
   - **Required**: Complex data structure handling tests

### Moderate Coverage Gaps

1. **Cross-Template Dependencies**

   - **Missing**: Template inclusion and inheritance scenarios
   - **Required**: Template composition testing

2. **Range Fragment Operations**

   - **Missing**: Complex list operations (reordering, batch updates)
   - **Required**: Comprehensive range operation testing

3. **WebSocket Integration**
   - **Missing**: End-to-end WebSocket communication testing
   - **Required**: Integration tests with actual WebSocket clients

### Current Test Coverage

- **Template Actions**: ✅ Comprehensive (via `examples/e2e/template_actions_tdd_test.go`)
- **Parse APIs**: ✅ Complete coverage (via `parse_api_test.go`)
- **Fragment Extraction**: ✅ Good coverage
- **Real-time Updates**: ✅ Basic scenarios covered
- **Integration**: ✅ Core functionality tested via examples
- **Performance**: ❌ No benchmarks
- **Concurrency**: ❌ Limited testing
- **Error Handling**: ❌ Insufficient coverage

### Testing Strategy

LiveTemplate uses a comprehensive table-driven testing approach:

1. **Test Organization**: Tests organized by template action type
2. **Real Files**: Tests use actual template files from `testdata/` directory
3. **TDD Methodology**: Test-driven development with failing tests first
4. **Embedded FS**: Tests include embedded filesystem scenarios
5. **E2E Testing**: End-to-end tests in `examples/e2e/` directory

## Performance Characteristics

### Scalability Considerations

1. **Fragment Count**: Performance degrades with >1000 fragments per template
2. **Update Frequency**: Optimized for <100 updates/second per renderer
3. **Data Size**: Efficient with objects <10MB, memory usage scales linearly
4. **Template Complexity**: Nested depth >5 levels may impact performance

### Optimization Opportunities

1. **Fragment Caching**: Cache rendered fragments to avoid re-computation
2. **Batch Updates**: Group multiple updates into single WebSocket message
3. **Differential Rendering**: Only render changed portions of fragments
4. **Memory Pooling**: Reuse buffers and data structures

## Security Considerations

### Template Security

- **HTML Escaping**: Relies on Go's template auto-escaping
- **XSS Prevention**: Automatic escaping prevents most XSS attacks
- **Template Injection**: Limited by Go template sandbox

### WebSocket Security

- **Authentication**: No built-in authentication (application responsibility)
- **Rate Limiting**: No built-in rate limiting for updates
- **Data Validation**: Minimal input validation on updates

## Future Architecture Enhancements

### Planned Go Library Improvements

1. **Plugin System**: Allow custom fragment processors
2. **Template Inheritance**: Support for template hierarchies
3. **Caching Layer**: Redis/Memcached integration for fragment caching
4. **Monitoring**: Built-in metrics and observability
5. **Template Validation**: Static analysis and validation tools

### Experimental Features

1. **Server-Sent Events**: Alternative to WebSocket for updates
2. **GraphQL Integration**: Fragment-based GraphQL subscriptions
3. **Template Compilation**: Compile templates to native Go code
4. **Performance Profiling**: Advanced template rendering profiling

## Conclusion

LiveTemplate provides a solid foundation for real-time web applications with its fragment-based architecture. The main strengths are its granular update capabilities and comprehensive template action support. Key areas for improvement include error handling, performance optimization, and test coverage expansion.

The architecture is designed for extensibility and can accommodate future enhancements while maintaining backward compatibility.
