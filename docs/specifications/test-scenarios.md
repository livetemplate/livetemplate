# LiveTemplate Test Scenarios Specification

Version: 1.0.0
Last Updated: 2025-10-22
Status: Draft

## 1. Introduction

This document defines comprehensive test scenarios for validating LiveTemplate's tree update generation system. It covers single-step operations, multi-step user journeys, edge cases, and performance scenarios.

## 2. Test Categories

### 2.1 Categories Overview

1. **Unit Tests**: Individual tree operations
2. **Integration Tests**: Template construct combinations
3. **User Journey Tests**: Real-world usage patterns
4. **Fuzz Tests**: Random operation sequences
5. **Performance Tests**: Large-scale scenarios
6. **Regression Tests**: Known bug scenarios

## 3. Single-Step Test Cases

### 3.1 Basic Field Updates

#### Test: Simple String Field Update
```go
Initial: {Name: "Alice"}
Update:  {Name: "Bob"}

Expected Tree (first):
{
  "s": ["<div>Hello ", "</div>"],
  "0": "Alice"
}

Expected Update:
{
  "0": "Bob"
}
```

#### Test: Numeric Field Update
```go
Initial: {Count: 5}
Update:  {Count: 10}

Expected Update:
{
  "0": "10"  // Only the changed value
}
```

#### Test: Boolean Field Toggle
```go
Initial: {Active: true}
Update:  {Active: false}

Expected Update:
{
  "0": "inactive"  // Rendered result of {{if .Active}}active{{else}}inactive{{end}}
}
```

### 3.2 Empty State Transitions

#### Test: Empty to Content
```go
Initial: {Items: []}
Update:  {Items: ["First"]}

Expected Tree (empty):
{
  "s": ["", ""],
  "0": {
    "s": [""],
    "d": []
  }
}

Expected Update (with content):
{
  "0": {
    "d": [{"0": "First"}]  // No statics needed, client has structure
  }
}
```

#### Test: Content to Empty
```go
Initial: {Items: ["A", "B"]}
Update:  {Items: []}

Expected Update:
{
  "0": {
    "d": []  // Empty array, statics retained
  }
}
```

### 3.3 Conditional Transitions

#### Test: If Branch Change
```go
Template: {{if .Premium}}Pro User{{else}}Free User{{end}}
Initial: {Premium: false}
Update:  {Premium: true}

Expected Update:
{
  "0": "Pro User"  // Only the new branch content
}
```

#### Test: Condition Appears
```go
Template: {{if .ShowBanner}}Important!{{end}}
Initial: {ShowBanner: false}
Update:  {ShowBanner: true}

Expected Update:
{
  "0": "Important!"  // Dynamic appears
}
```

### 3.4 Range Operations

#### Test: Single Item Add
```go
Initial: {Items: ["A", "B"]}
Update:  {Items: ["A", "B", "C"]}

Expected Update:
{
  "0": [
    ["i", "item-b", "end", {"0": "item-c", "1": "C"}]
  ]
}
```

#### Test: Single Item Remove
```go
Initial: {Items: ["A", "B", "C"]}
Update:  {Items: ["A", "C"]}

Expected Update:
{
  "0": [
    ["r", "item-b"]
  ]
}
```

#### Test: Single Item Update
```go
Initial: {Items: [{ID: "1", Name: "Old"}]}
Update:  {Items: [{ID: "1", Name: "New"}]}

Expected Update:
{
  "0": [
    ["u", "1", {"1": "New"}]
  ]
}
```

#### Test: Items Reorder
```go
Initial: {Items: ["A", "B", "C"]}
Update:  {Items: ["C", "A", "B"]}

Expected Update:
{
  "0": [
    ["o", ["item-c", "item-a", "item-b"]]
  ]
}
```

## 4. Multi-Step User Journeys

### 4.1 Todo Application Journey

```yaml
Journey: Todo_Application_Workflow
Steps:
  1. Visit:
     Action: Initial page load
     State: {Todos: []}
     Validate:
       - Tree has statics
       - Empty range with "d": []

  2. Add_First_Todo:
     Action: Add todo "Learn Go"
     State: {Todos: [{ID: "1", Text: "Learn Go"}]}
     Validate:
       - Insert operation only
       - No full list sent

  3. Add_Second_Todo:
     Action: Add todo "Build app"
     State: {Todos: [..., {ID: "2", Text: "Build app"}]}
     Validate:
       - Single insert operation
       - Position after first item

  4. Complete_First:
     Action: Mark first as complete
     State: {Todos[0].Complete: true}
     Validate:
       - Update operation for item "1"
       - Only changed field sent

  5. Delete_Second:
     Action: Remove second todo
     State: {Todos: [first_only]}
     Validate:
       - Remove operation ["r", "2"]
       - No other data sent

  6. Add_Multiple:
     Action: Add 3 todos at once
     State: {Todos: [...4 items]}
     Validate:
       - Three insert operations
       - Each with correct position

  7. Reorder_All:
     Action: Drag to reorder
     State: {Todos: [reordered]}
     Validate:
       - Single order operation
       - ["o", [new_order]]

  8. Clear_All:
     Action: Clear completed
     State: {Todos: []}
     Validate:
       - Multiple remove operations
       - OR empty "d": []
```

### 4.2 Chat Application Journey

```yaml
Journey: Chat_Real_Time
Steps:
  1. Join_Chat:
     Action: Initial connection
     State: {Messages: [], Users: ["self"]}
     Validate:
       - Full tree with statics
       - Both lists empty/minimal

  2. First_Message:
     Action: Send "Hello"
     State: {Messages: [{User: "self", Text: "Hello"}]}
     Validate:
       - Insert operation for message
       - Timestamp included

  3. Other_User_Joins:
     Action: "Alice" joins
     State: {Users: ["self", "Alice"]}
     Validate:
       - Insert operation for user
       - OR full user list update

  4. Receive_Message:
     Action: Alice sends message
     State: {Messages: [..., {User: "Alice", Text: "Hi"}]}
     Validate:
       - Insert at end
       - Message structure correct

  5. Edit_Message:
     Action: Edit own message
     State: {Messages[0].Text: "Hello everyone"}
     Validate:
       - Update operation
       - Only text field changes

  6. User_Typing:
     Action: Show typing indicator
     State: {TypingUsers: ["Alice"]}
     Validate:
       - Field update
       - Efficient indicator toggle

  7. Load_History:
     Action: Load previous messages
     State: {Messages: [old_msgs + current]}
     Validate:
       - Multiple insert operations
       - Prepended to list

  8. User_Leaves:
     Action: Alice disconnects
     State: {Users: ["self"]}
     Validate:
       - Remove operation
       - OR user list update
```

### 4.3 Dashboard Journey

```yaml
Journey: Analytics_Dashboard
Steps:
  1. Load_Dashboard:
     Action: Initial load
     State: {Widgets: [], Loading: true}
     Validate:
       - Loading state shown
       - Empty widgets

  2. Data_Arrives:
     Action: Metrics loaded
     State: {Widgets: [w1, w2, w3], Loading: false}
     Validate:
       - Multiple widgets appear
       - Loading disappears

  3. Real_Time_Update:
     Action: Metric changes
     State: {Widgets[0].Value: new_value}
     Validate:
       - Single widget update
       - Specific field only

  4. Add_Widget:
     Action: User adds widget
     State: {Widgets: [..., new_widget]}
     Validate:
       - Insert operation
       - Widget fully defined

  5. Configure_Widget:
     Action: Change widget settings
     State: {Widgets[3].Config: updated}
     Validate:
       - Update operation
       - Config nested update

  6. Rearrange_Layout:
     Action: Drag widgets
     State: {Widgets: [reordered]}
     Validate:
       - Order operation
       - Positions preserved

  7. Filter_Data:
     Action: Apply time filter
     State: {All_widgets_update}
     Validate:
       - Multiple updates
       - Each widget affected

  8. Remove_Widget:
     Action: Delete widget
     State: {Widgets: [fewer]}
     Validate:
       - Remove operation
       - Clean removal
```

## 5. Edge Cases

### 5.1 Rapid Updates

#### Test: Debounced Input
```yaml
Scenario: User types quickly
Updates:
  - {Search: "a"}     @ 0ms
  - {Search: "ab"}    @ 50ms
  - {Search: "abc"}   @ 100ms
  - {Search: "abcd"}  @ 150ms

Expected:
  - Each update contains only "0": "new_value"
  - No statics resent
  - Updates queued properly
```

#### Test: Concurrent Operations
```yaml
Scenario: Multiple async actions
Actions:
  - Add item A
  - Delete item B
  - Update item C
  - All within 10ms

Expected:
  - Three separate operations
  - Order preserved
  - No conflicts
```

### 5.2 Large Scale

#### Test: Large List (1000 items)
```yaml
Initial: Generate 1000 items
Operations:
  - Add item at position 500
  - Remove item at position 250
  - Update item at position 750
  - Reorder subsection

Validate:
  - Operations remain granular
  - No full list resends
  - Performance < 100ms
```

#### Test: Deep Nesting (10 levels)
```yaml
Template: Nested divs with conditions
Structure:
  {{if .L1}}
    {{if .L2}}
      {{if .L3}}
        ...10 levels deep
      {{end}}
    {{end}}
  {{end}}

Validate:
  - Tree structure maintains depth
  - Updates affect only changed level
  - No structure corruption
```

### 5.3 Special Characters

#### Test: HTML in Content
```yaml
Data: {Text: "<script>alert('xss')</script>"}
Expected:
  - Properly escaped in tree
  - &lt;script&gt; in output
```

#### Test: Unicode Content
```yaml
Data: {Text: "Hello 世界 🌍"}
Expected:
  - Correct encoding
  - No data loss
```

### 5.4 Whitespace Handling

#### Test: Trim Operators
```yaml
Template: {{- .Field -}}
Expected:
  - Whitespace removed
  - Tree structure correct
```

## 6. Performance Scenarios

### 6.1 Benchmarks

| Scenario | Items | Target Time | Max Memory |
|----------|-------|-------------|------------|
| Small list update | 10 | < 1ms | < 1KB |
| Medium list update | 100 | < 5ms | < 10KB |
| Large list update | 1000 | < 50ms | < 100KB |
| Deep nesting | 10 levels | < 10ms | < 5KB |
| Complex template | 50 fields | < 20ms | < 20KB |

### 6.2 Memory Tests

#### Test: Memory Leak Detection
```yaml
Scenario: 10000 update cycles
Monitor:
  - Memory growth
  - Goroutine leaks
  - Tree cache size

Expected:
  - Stable memory usage
  - No goroutine accumulation
  - Cache bounded
```

## 7. Regression Tests

### 7.1 Known Issues

#### Test: Mixed Template Fix
```yaml
Issue: Templates with ranges + other dynamics failed
Template:
  {{.Title}}
  {{range .Items}}{{.}}{{end}}
  {{.Footer}}

Validate:
  - All three dynamics work
  - Updates independent
```

#### Test: Empty Range Transition
```yaml
Issue: Empty to non-empty range lost statics
Transition:
  From: {Items: []}
  To: {Items: ["A"]}

Validate:
  - Statics included when needed
  - Structure preserved
```

## 8. Fuzz Test Scenarios

### 8.1 Random User Activity

```go
type FuzzActivity struct {
    Operations []string{
        "visit",
        "add_item",
        "remove_item",
        "update_item",
        "reorder_items",
        "toggle_condition",
        "change_field",
        "clear_all",
    }

    Constraints:
    - Valid state transitions only
    - Max 100 operations per journey
    - Random delays between operations
}
```

### 8.2 Property-Based Tests

```yaml
Properties:
  1. First_Update_Has_Statics:
     For any template T and data D:
     First tree MUST contain "s" key

  2. Subsequent_Updates_Minimal:
     For any change C:
     Update size <= size(changed_data) * 1.1

  3. Operations_Granular:
     For any list operation O:
     Operation affects only target items

  4. Fingerprint_Deterministic:
     Same data → Same fingerprint

  5. Round_Trip_Preservation:
     Tree → HTML → Parse → Tree (identical)
```

## 9. Validation Criteria

### 9.1 Correctness Metrics

- **Specification Compliance**: 100% adherence
- **Update Efficiency**: < 10% overhead
- **Operation Granularity**: 100% granular
- **Statics Redundancy**: 0% after first render

### 9.2 Performance Metrics

- **Tree Generation**: O(n) complexity verified
- **Diff Computation**: O(m) for m changes
- **Memory Usage**: Linear with template size
- **Update Latency**: < 10ms p99

## 10. Test Implementation

### 10.1 Test Framework Structure

```go
type TestScenario struct {
    Name        string
    Template    string
    Journey     []Step
    Validators  []Validator
}

type Step struct {
    Action      string
    Data        interface{}
    Expected    TreeNode
}

type Validator interface {
    Validate(actual, expected TreeNode) error
}
```

### 10.2 Golden Files

Location: `testdata/scenarios/`

Structure:
```
scenarios/
├── todo_journey/
│   ├── step_01_initial.golden.json
│   ├── step_02_add_first.golden.json
│   └── ...
├── chat_journey/
└── edge_cases/
```

## 11. Continuous Testing

### 11.1 CI Pipeline

1. **Unit Tests**: Run on every commit
2. **Integration Tests**: Run on PR
3. **Journey Tests**: Run on PR
4. **Fuzz Tests**: Nightly (8 hours)
5. **Performance Tests**: Weekly

### 11.2 Test Coverage Requirements

- **Line Coverage**: > 95%
- **Branch Coverage**: > 90%
- **Scenario Coverage**: 100% of documented
- **Fuzz Iterations**: > 10M without failure

## 12. Debugging Support

### 12.1 Test Failure Output

When test fails, output must include:
1. Template source
2. Input data
3. Expected tree
4. Actual tree
5. Diff visualization
6. Update sequence history

### 12.2 Replay Capability

Failed fuzzing sequences must be:
1. Saved as regression tests
2. Minimized to smallest failing case
3. Reproducible deterministically

## Appendix A: Common Patterns

### Pattern: List CRUD
```yaml
Create: Insert operation
Read: Initial tree with "d"
Update: Update operation on item
Delete: Remove operation
```

### Pattern: Toggle UI
```yaml
Show: Condition true, content appears
Hide: Condition false, empty string
Toggle: Only dynamic changes
```

### Pattern: Form States
```yaml
Empty: Initial state
Dirty: Fields have values
Submitting: Loading indicator
Success: Clear form, show message
Error: Show validation errors
```

## Appendix B: Test Data Generators

```go
// Generate random todo items
func GenerateTodos(count int) []Todo

// Generate user activity sequence
func GenerateUserJourney(length int) []Activity

// Generate nested structure
func GenerateNestedData(depth int) interface{}

// Generate large dataset
func GenerateBulkData(size int) []interface{}
```