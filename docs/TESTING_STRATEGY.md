# Comprehensive Testing Strategy for StateTemplate

## Overview

This document outlines the comprehensive testing strategy for the StateTemplate library, ensuring all Go template actions are thoroughly tested for both initial rendering and real-time updates.

## Testing Architecture

### 1. Test Structure

```
tests/
├── realtime_renderer_comprehensive_test.go  # Main test file
├── testdata/                                # Template files organized by action
│   ├── comments/
│   │   ├── single_line.tmpl
│   │   ├── multi_line.tmpl
│   │   └── nested.tmpl
│   ├── variables/
│   │   ├── simple.tmpl
│   │   └── scoping.tmpl
│   ├── pipelines/
│   │   ├── basic.tmpl
│   │   └── chained.tmpl
│   ├── conditionals/
│   │   ├── if_basic.tmpl
│   │   ├── if_else.tmpl
│   │   └── nested.tmpl
│   ├── range/
│   │   ├── simple.tmpl
│   │   ├── with_index.tmpl
│   │   └── empty.tmpl
│   ├── with/
│   │   ├── basic.tmpl
│   │   └── nil_fallback.tmpl
│   ├── functions/
│   │   ├── builtin.tmpl
│   │   └── comparisons.tmpl
│   ├── blocks/
│   │   ├── basic.tmpl
│   │   └── template_include.tmpl
│   ├── mixed/
│   │   ├── conditionals_range.tmpl
│   │   └── with_vars_pipes.tmpl
│   └── comprehensive/
│       └── all_actions.tmpl
```

### 2. Test Data Structures

**TestData**: Comprehensive data structure covering all possible template scenarios:

- Basic fields (string, int, float64, bool)
- Collections (slices of strings, structs, numbers)
- Nested objects for `with` operations
- Complex structures for real-world scenarios

**TestCase**: Standardized test case structure:

- Template file reference (in testdata/)
- Initial data and expected HTML
- Update data and expected RealtimeUpdates
- Error conditions and validation

### 3. Test Categories

#### A. Constructor Tests

- `TestNewRealtimeRenderer`: Validates renderer initialization with different configurations

#### B. Individual Action Tests

Each Go template action gets its own test suite:

1. **Comments** (`TestTemplateAction_Comments`)

   - Single line comments
   - Multi-line comments
   - Nested comments
   - Comments within control structures

2. **Variables** (`TestTemplateAction_Variables`)

   - Simple variable assignment
   - Variable scoping within ranges
   - Variable updates and fragment targeting

3. **Pipelines** (`TestTemplateAction_Pipelines`)

   - Basic pipeline operations
   - Chained pipelines
   - Built-in function usage
   - Custom function integration

4. **Conditionals** (`TestTemplateAction_Conditionals`)

   - If conditions (true/false)
   - If-else statements
   - Nested conditionals
   - Complex boolean expressions

5. **Range/Loops** (`TestTemplateAction_Range`)

   - Simple range over slices
   - Range with index and value
   - Range over structs
   - Empty range with else clause
   - Granular list updates (add/remove/modify)

6. **With** (`TestTemplateAction_With`)

   - With existing objects
   - With nil objects and else clause
   - Nested with statements

7. **Functions** (`TestTemplateAction_Functions`)

   - Built-in functions (len, index, printf)
   - Comparison functions (eq, ne, lt, gt)
   - String functions (upper, lower)
   - Math functions (add, sub, mul, div)

8. **Blocks/Templates** (`TestTemplateAction_Blocks`)
   - Block definitions and usage
   - Template inclusions
   - Template inheritance patterns

#### C. Combination Tests

9. **Mixed Combinations** (`TestTemplateAction_MixedCombinations`)
   - Conditionals with ranges
   - With statements with variables and pipelines
   - Nested combinations of multiple actions

#### D. Comprehensive Tests

10. **All Actions Combined** (`TestTemplateAction_AllActionsCombined`)

- Single template using all Go template actions
- Complex real-world scenarios
- Full integration testing

### 4. Testing Methodology

#### Two-Phase Testing

Each test case validates both:

1. **Initial Rendering Phase**:

   - Template parsing and registration
   - Initial data setting
   - Complete HTML output validation
   - Fragment ID generation and wrapping

2. **Real-time Update Phase**:
   - Data updates through SendUpdate()
   - RealtimeUpdate generation
   - Fragment-specific updates
   - Granular change detection

#### Validation Approach

- **HTML Comparison**: Normalized whitespace comparison for initial rendering
- **Fragment Updates**: Validate RealtimeUpdate structure and content
- **Error Handling**: Test error conditions and recovery
- **Performance**: Basic performance validation for fragment operations

### 5. Template File Organization

Templates are organized by action type in the `testdata/` directory:

- Enables real file loading (more realistic than inline strings)
- Clear separation of concerns
- Reusable templates across multiple test scenarios
- Version control for template changes

### 6. Expected Benefits

#### Comprehensive Coverage

- All Go template actions tested individually
- Real-world combinations and edge cases
- Both initial rendering and real-time updates

#### Maintainability

- Clear test organization by action type
- File-based templates for easy modification
- Standardized test case structure
- Consistent validation patterns

#### Debugging Support

- Detailed error messages with expected vs actual
- Template file references for easy debugging
- Fragment ID tracking and validation
- Update sequence verification

#### Regression Prevention

- Comprehensive baseline for all template actions
- Real-time update validation
- Error condition testing
- Performance regression detection

### 7. Running the Tests

```go
// Run all tests
go test -v ./...

// Run specific test suite
go test -v -run TestTemplateAction_Comments

// Run comprehensive test only
go test -v -run TestTemplateAction_AllActionsCombined

// Run with coverage
go test -v -cover ./...
```

### 8. Future Enhancements

- **Performance Benchmarks**: Add benchmark tests for fragment operations
- **Concurrency Tests**: Test multi-goroutine scenarios
- **Memory Usage**: Validate memory efficiency
- **WebSocket Integration**: End-to-end client-server testing
- **Template Validation**: Static analysis of template correctness

This testing strategy ensures that StateTemplate's real-time rendering capabilities are thoroughly validated across all Go template features, providing confidence in both initial rendering and live update functionality.
