---
applyTo: "*_test.go"
---

# Testing Instructions for StateTemplate

## Table-Driven Test Standards

When creating or modifying tests in StateTemplate, follow these specific patterns:

### Test Suite Structure

```go
func TestSuiteName(t *testing.T) {
    tests := []struct {
        name     string
        template string
        data     interface{}
        expected string
    }{
        // Test cases here
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Test Case Naming

- Use descriptive names that explain the scenario being tested
- Format: `"action_type with condition"` (e.g., `"comment action with single line"`)
- Include edge cases: `"empty data"`, `"nil values"`, `"malformed template"`

### Template Action Test Categories

- **Comment Tests**: Template comments and their handling
- **Variable Tests**: Variable assignment and scoping
- **Pipeline Tests**: Function chains and data transformation
- **Conditional Tests**: If/else logic and branching
- **Loop Tests**: Range and with statements
- **Function Tests**: Built-in and custom function calls
- **Comparison Tests**: Equality and logical operations
- **Block Tests**: Template composition and inheritance

### Test Data Patterns

Use realistic data structures that reflect real-world usage:

```go
data := map[string]interface{}{
    "Name":  "John",
    "Items": []string{"item1", "item2"},
    "User":  struct{Name string}{Name: "Alice"},
}
```

### Error Testing

Include negative test cases for:

- Malformed templates
- Missing data fields
- Type mismatches
- Invalid template syntax

### Fragment Testing Specifics

When testing fragment extraction and updates:

- Verify fragment boundaries are correctly identified
- Test dependency tracking for data changes
- Validate minimal update generation
- Check fragment type categorization (simple, conditional, range, block)

### TDD Workflow

1. Write failing test that describes expected behavior
2. Run test to confirm it fails
3. Implement minimal code to make test pass
4. Refactor while keeping tests green
5. Add edge cases and error scenarios
