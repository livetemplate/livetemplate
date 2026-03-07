# LiveTemplate Test Scenarios Specification

Version: 1.0.0
Last Updated: 2026-03-07
Status: Final

> **Scope:** This is a **test specification** describing expected behavior and coverage targets,
> not an implementation status tracker. For actual test implementations, see:
> `template_test.go` (including key injection tests), `tree_test.go` (tree invariants),
> `e2e_update_spec_test.go`, and `internal/*/` package tests.

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

Expected Update (empty→items uses append with statics+metadata):
{
  "0": [
    ["a", [{"0": "First"}], ["<li>", "</li>"], {"idKey": "0"}]
  ]
}
```

#### Test: Content to Empty
```go
Initial: {Items: ["A", "B"]}
Update:  {Items: []}

Expected Update:
{
  "0": {
    "d": []  // Empty array, statics retained by client
  }
}
```

### 3.3 Conditional Transitions

#### Test: If Branch Change
```go
Template: {{if .Premium}}Pro User{{else}}Free User{{end}}
Initial: {Premium: false}
Update:  {Premium: true}

Expected Update (static-only branch sent as TreeNode with statics):
{
  "0": {"s": ["Pro User"]}
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

#### Test: Single Item Append
```go
Initial: {Items: [{ID: "1", Text: "First"}]}
Update:  {Items: [{ID: "1", Text: "First"}, {ID: "2", Text: "Second"}]}

Expected Update (item at end triggers append):
{
  "0": [
    ["a", [{"0": "2", "1": "Second"}], ["<div data-key=\"", "\">", "</div>"]]
  ]
}
```

#### Test: Single Item Prepend
```go
Initial: {Items: [{ID: "2", Text: "Second"}]}
Update:  {Items: [{ID: "1", Text: "First"}, {ID: "2", Text: "Second"}]}

Expected Update (item at start triggers prepend):
{
  "0": [
    ["p", [{"0": "1", "1": "First"}], ["<div data-key=\"", "\">", "</div>"]]
  ]
}
```

#### Test: Single Item Insert
```go
Initial: {Items: [{ID: "1", Text: "First"}, {ID: "3", Text: "Third"}]}
Update:  {Items: [{ID: "1", Text: "First"}, {ID: "2", Text: "Between"}, {ID: "3", Text: "Third"}]}

Expected Update (item at specific position):
{
  "0": [
    ["i", "1", {"0": "2", "1": "Between"}]
  ]
}
```

#### Test: Single Item Remove
```go
Initial: {Items: [{ID: "1", Text: "First"}, {ID: "2", Text: "Second"}]}
Update:  {Items: [{ID: "1", Text: "First"}]}

Expected Update:
{
  "0": [
    ["r", "2"]
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
Initial: {Items: [{ID: "1"}, {ID: "2"}, {ID: "3"}]}
Update:  {Items: [{ID: "3"}, {ID: "1"}, {ID: "2"}]}

Expected Update (pure reorder, no content changes):
{
  "0": [
    ["o", ["3", "1", "2"]]
  ]
}
```

#### Test: Update + Reorder
```go
Initial: {Items: [{ID: "1", Text: "A"}, {ID: "2", Text: "B"}]}
Update:  {Items: [{ID: "2", Text: "B"}, {ID: "1", Text: "Changed"}]}

Expected Update (content changed AND order changed):
{
  "0": [
    ["u", "1", {"1": "Changed"}],
    ["o", ["2", "1"]]
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
       - Append operation with statics and metadata (empty→items)
       - Format: ["a", items, statics, metadata]
       - No full list sent

  3. Add_Second_Todo:
     Action: Add todo "Build app"
     State: {Todos: [..., {ID: "2", Text: "Build app"}]}
     Validate:
       - Single append operation (item at end)
       - Format: ["a", items, statics]
       - Statics stripped if fingerprint unchanged

  4. Complete_First:
     Action: Mark first as complete
     State: {Todos[0].Done: true}
     Validate:
       - Update operation for item "1"
       - Only changed fields sent

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
       - Append operation with all new items
       - Format: ["a", [item1, item2, item3], statics]

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
       - Append operation with statics+metadata (empty→items)
       - Timestamp included

  3. Other_User_Joins:
     Action: "Alice" joins
     State: {Users: ["self", "Alice"]}
     Validate:
       - Append operation for user
       - OR full user list update

  4. Receive_Message:
     Action: Alice sends message
     State: {Messages: [..., {User: "Alice", Text: "Hi"}]}
     Validate:
       - Append at end
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
       - Prepend operation for history messages
       - Format: ["p", items, statics]

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
       - Append operation with statics+metadata (empty→items)
       - Loading disappears (conditional update)

  3. Real_Time_Update:
     Action: Metric changes
     State: {Widgets[0].Value: new_value}
     Validate:
       - Single widget update operation
       - Specific field only

  4. Add_Widget:
     Action: User adds widget
     State: {Widgets: [..., new_widget]}
     Validate:
       - Append operation
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
       - Multiple update operations
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
  - No statics resent (fingerprint unchanged)
  - Updates queued properly via async WebSocket send
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
  - Structure fingerprints computed correctly at each level
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
Data: {Text: "Hello 世界"}
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

### 5.5 Fingerprint Edge Cases

#### Test: Same Content, Different Structure
```yaml
Old Template: <div>{{.Text}}</div>
New Template: <span>{{.Text}}</span>
Data: {Text: "Same"}

Expected:
  - Structure fingerprints differ
  - Statics re-sent with update
```

#### Test: Different Content, Same Structure
```yaml
Template: <div>{{.Text}}</div>
Old Data: {Text: "Hello"}
New Data: {Text: "World"}

Expected:
  - Structure fingerprints match
  - Only dynamic value sent (no statics)
```

### 5.6 Range Operation Edge Cases

#### Test: Complex Insertion Pattern Fallback
```yaml
Scenario: Many items change keys simultaneously (e.g., close_all action)
Expected:
  - Differential operations return empty
  - Caller falls back to full tree replacement
  - No partial operations (no removes without matching inserts)
```

#### Test: Empty→Items→Empty→Items Cycle
```yaml
Steps:
  1. Start empty
  2. Add items (append with statics+metadata)
  3. Clear all (empty "d": [])
  4. Add items again (append with statics+metadata)

Validate:
  - Each empty→items transition sends statics
  - Client can reconstruct range state each time
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
| Fingerprint comparison | any | O(1) | negligible |

### 6.2 Memory Tests

#### Test: Memory Leak Detection
```yaml
Scenario: 10000 update cycles
Monitor:
  - Memory growth
  - Goroutine leaks
  - Fingerprint cache size

Expected:
  - Stable memory usage
  - No goroutine accumulation
  - Fingerprint cache bounded (one per TreeNode)
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
  - Append operation includes statics
  - Metadata included for item key detection
```

#### Test: Conditional Branch Static-Only Changes
```yaml
Issue: Static-only conditional branches not detected as changes
Template: {{if .Active}}ON{{else}}OFF{{end}}
Transition: Active: true → false

Validate:
  - Fingerprint comparison detects structure change
  - New branch statics sent to client
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
     Same static structure → Same fingerprint
     Different structure → Different fingerprint (with high probability)

  5. Round_Trip_Preservation:
     Tree → HTML → Parse → Tree (identical)

  6. Fingerprint_Ignores_Values:
     Two trees with same structure but different dynamic values
     MUST produce the same structure fingerprint
```

## 9. Validation Criteria

### 9.1 Correctness Metrics

- **Specification Compliance**: 100% adherence
- **Update Efficiency**: < 10% overhead
- **Operation Granularity**: 100% granular (where possible; complex patterns fall back to full replacement)
- **Statics Redundancy**: 0% after first render (when fingerprint unchanged)

### 9.2 Performance Metrics

- **Tree Generation**: O(n) complexity verified
- **Diff Computation**: O(m) for m changes
- **Fingerprint Comparison**: O(1) after initial computation
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
    Expected    *build.TreeNode
}

type Validator interface {
    Validate(actual, expected *build.TreeNode) error
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
Create: Append/prepend/insert operation
Read: Initial tree with "d" and statics
Update: Update operation on item
Delete: Remove operation
```

### Pattern: Toggle UI
```yaml
Show: Condition true, content appears
Hide: Condition false, empty string (or static-only TreeNode)
Toggle: Only dynamic changes (fingerprint may differ for static-only branches)
```

### Pattern: Form States
```yaml
Empty: Initial state
Dirty: Fields have values
Submitting: Loading indicator
Success: Clear form, show message
Error: Show validation errors (via response metadata)
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
