---
applyTo: "**"
---

# Fragment Extraction Instructions

## FragmentExtractor Implementation Guidelines

### Fragment Type Classification

#### Simple Fragments

- Direct field access: `{{.Field}}`
- Single data dependency
- Straightforward replacement updates
- No conditional rendering logic

#### Conditional Fragments

- If/with blocks: `{{if .Condition}}...{{end}}`
- May appear or disappear based on data
- Track condition dependencies
- Handle nested conditional structures

#### Range Fragments

- Loop constructs: `{{range .Items}}...{{end}}`
- Granular item-level tracking required
- Support for item addition, removal, reordering
- Maintain item index relationships

#### Block Fragments

- Named template sections: `{{block "name" .}}...{{end}}`
- Template composition and inheritance
- Cross-template dependencies
- Hierarchical fragment relationships

### Fragment Extraction Patterns

#### Boundary Detection

- Identify start and end positions of extractable segments
- Handle nested template structures correctly
- Account for whitespace and formatting preservation
- Validate fragment completeness and syntax

#### Dependency Analysis

- Map template variables to data structure fields
- Track deep object path dependencies
- Identify circular dependency risks
- Build dependency graphs for change propagation

#### Fragment Identification

- Generate unique, stable fragment IDs
- Maintain ID consistency across template updates
- Support fragment renaming and restructuring
- Handle fragment merging and splitting scenarios

### Performance Considerations

#### Extraction Optimization

- Cache fragment analysis results
- Minimize template re-parsing overhead
- Use efficient string parsing algorithms
- Parallelize independent extraction operations

#### Memory Management

- Pool fragment objects for reuse
- Minimize memory allocation in hot paths
- Clean up unused fragment references
- Implement fragment garbage collection

### Error Handling

- Validate template syntax during extraction
- Handle malformed template structures gracefully
- Provide detailed error context for debugging
- Implement fallback strategies for extraction failures

### Integration Points

- Coordinate with TemplateTracker for dependency mapping
- Provide fragment metadata to RealtimeRenderer
- Support TemplateAnalyzer optimization hints
- Enable efficient fragment update generation
