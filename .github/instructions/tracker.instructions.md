---
applyTo: "*tracker*.go"
---

# Template Tracker Instructions

## TemplateTracker Implementation Guidelines

### Core Responsibilities

- Monitor data structure changes through reflection
- Build and maintain dependency graphs
- Detect field-level modifications efficiently
- Trigger fragment updates based on data changes

### Change Detection Strategies

#### Reflection-Based Monitoring

- Use reflect package for dynamic data analysis
- Handle various Go data types (structs, maps, slices, interfaces)
- Deep comparison for nested data structures
- Efficient diff generation for large objects

#### Dependency Graph Management

- Map data fields to dependent template fragments
- Handle complex object relationships
- Support circular dependency detection
- Optimize graph traversal for change propagation

#### Performance Optimization

- Cache reflection metadata for frequently accessed types
- Use type-specific optimized comparison functions
- Minimize reflection overhead in change detection
- Implement incremental update strategies

### Data Structure Handling

#### Primitive Types

- Direct value comparison for basic types
- Handle nil pointer dereferences safely
- Support type coercion where appropriate
- Preserve original data type information

#### Complex Types

- Recursive comparison for nested structures
- Map key addition, removal, and value changes
- Slice item modifications, additions, removals
- Interface type changes and nil handling

#### Special Cases

- Handle unexported struct fields appropriately
- Support custom comparison methods where available
- Deal with function types and channels gracefully
- Manage time.Time and other complex standard types

### Integration Patterns

#### Fragment Coordination

- Notify FragmentExtractor of relevant data changes
- Provide change context for minimal update generation
- Support batch change notifications for efficiency
- Handle change ordering and dependency resolution

#### Real-time Processing

- Implement efficient change polling or event-driven updates
- Support configurable update intervals
- Handle high-frequency data changes gracefully
- Provide backpressure mechanisms for update flooding

### Error Handling and Edge Cases

- Handle reflection panics gracefully
- Deal with concurrent data modifications
- Support data structure evolution and schema changes
- Provide debugging information for tracking issues

### Concurrency Safety

- Implement proper synchronization for shared state
- Handle concurrent read/write access to tracked data
- Use appropriate locking strategies for performance
- Support goroutine-safe tracking operations
