# Sequential User Activity Testing Framework

## Design Document

### Overview

This document describes a testing framework for LiveTemplate that enables testing of sequential user activities (e.g., "two adds, three mark completes") with support for:

1. **Deterministic sequential tests** - Pre-defined action sequences
2. **Property-based/fuzz testing** - Random action sequences with invariant validation
3. **Browser-less testing** - Fast, unit-test level testing using direct dispatch
4. **Browser-based testing** - Full E2E sanity tests using chromedp

### Goals

- Test realistic user workflows (not just single actions)
- Detect state corruption from action sequences
- Find edge cases through randomized sequences
- Keep test suite fast (majority non-browser)
- Provide regression protection for complex interactions

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Sequential Testing Framework                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐  │
│  │   Scenario   │───▶│   Executor   │───▶│  State Validators    │  │
│  │  Definition  │    │              │    │                      │  │
│  └──────────────┘    └──────────────┘    └──────────────────────┘  │
│         │                   │                      │                │
│         ▼                   ▼                      ▼                │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐  │
│  │   Action     │    │   Direct     │    │  Invariant Checks    │  │
│  │  Generator   │    │   Dispatch   │    │  - State valid       │  │
│  │  (Fuzz)      │    │              │    │  - Tree consistent   │  │
│  └──────────────┘    └──────────────┘    │  - No panics         │  │
│                             │            └──────────────────────┘  │
│                             ▼                                       │
│                      ┌──────────────┐                               │
│                      │   Browser    │ (optional, sanity only)       │
│                      │   Executor   │                               │
│                      └──────────────┘                               │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Core Components

### 1. Action Definition

```go
// Action represents a user action in a test scenario
type Action struct {
    Name string                 // Action method name (e.g., "add", "toggle")
    Data map[string]interface{} // Action payload
}

// Scenario is a sequence of actions with expected outcomes
type Scenario struct {
    Name        string
    InitState   interface{}           // Initial state
    Actions     []Action              // Sequence of actions
    Validate    func(state interface{}) error // Post-scenario validation
}
```

### 2. State Snapshot for Assertions

```go
// StateSnapshot captures state at a point in time for comparison
type StateSnapshot struct {
    State     interface{}
    Tree      map[string]interface{}
    Timestamp time.Time
}

// StateHistory tracks all state transitions
type StateHistory struct {
    Initial   StateSnapshot
    Snapshots []StateSnapshot
    Actions   []Action
}
```

### 3. Invariant Validators

```go
// Invariant is a property that must hold after every action
type Invariant func(before, after StateSnapshot, action Action) error

// Built-in invariants
var (
    // State must be JSON serializable (detects leaked dependencies)
    InvariantSerializable Invariant

    // Count fields must be non-negative
    InvariantNonNegativeCounts Invariant

    // Tree must be valid structure
    InvariantValidTree Invariant

    // No panics during execution
    InvariantNoPanic Invariant
)
```

---

## Test Execution Modes

### Mode 1: Direct Dispatch (Fast, Primary)

Tests run without HTTP/WebSocket overhead. Used for:
- Unit tests
- Fuzz testing
- CI/CD

```go
func TestSequence_DirectDispatch(t *testing.T) {
    ctrl := &TodoController{}
    state := &TodoState{Items: []Todo{}}

    executor := NewDirectExecutor(ctrl, state)

    scenario := Scenario{
        Actions: []Action{
            {Name: "add", Data: map[string]interface{}{"title": "Item 1"}},
            {Name: "add", Data: map[string]interface{}{"title": "Item 2"}},
            {Name: "toggle", Data: map[string]interface{}{"index": 0}},
            {Name: "toggle", Data: map[string]interface{}{"index": 1}},
            {Name: "toggle", Data: map[string]interface{}{"index": 0}},
        },
        Validate: func(state interface{}) error {
            s := state.(*TodoState)
            if len(s.Items) != 2 {
                return fmt.Errorf("expected 2 items, got %d", len(s.Items))
            }
            if s.Items[0].Done {
                return errors.New("item 0 should not be done (toggled twice)")
            }
            if !s.Items[1].Done {
                return errors.New("item 1 should be done")
            }
            return nil
        },
    }

    err := executor.Run(scenario)
    if err != nil {
        t.Fatal(err)
    }
}
```

### Mode 2: Template Update Verification

Tests that tree updates are generated correctly after sequences:

```go
func TestSequence_TreeUpdates(t *testing.T) {
    tmpl := Must(New("todos"))
    tmpl.Parse(`<ul>{{range .Items}}<li>{{.Title}}: {{.Done}}</li>{{end}}</ul>`)

    ctrl := &TodoController{}
    state := &TodoState{}

    executor := NewTemplateExecutor(tmpl, ctrl, state)
    executor.WithInvariants(
        InvariantValidTree,
        InvariantSerializable,
    )

    // After each action, executor renders and validates tree
    err := executor.RunSequence([]Action{
        {Name: "add", Data: map[string]interface{}{"title": "A"}},
        {Name: "add", Data: map[string]interface{}{"title": "B"}},
        {Name: "remove", Data: map[string]interface{}{"index": 0}},
    })

    if err != nil {
        t.Fatal(err)
    }
}
```

### Mode 3: HTTP Handler Testing

Tests the full HTTP request cycle without a browser:

```go
func TestSequence_HTTPHandler(t *testing.T) {
    tmpl := Must(New("counter"))
    tmpl.Parse(`<div>{{.Count}}</div>`)

    ctrl := &CounterController{}
    state := AsState(&CounterState{Count: 0})
    handler := tmpl.Handle(ctrl, state)

    executor := NewHTTPExecutor(handler)

    // Simulates form POST requests
    err := executor.RunSequence([]Action{
        {Name: "increment", Data: nil},
        {Name: "increment", Data: nil},
        {Name: "add", Data: map[string]interface{}{"amount": 5}},
    })

    // Verify final response contains expected value
    if !strings.Contains(executor.LastResponse(), "7") {
        t.Error("expected count to be 7")
    }
}
```

### Mode 4: Browser Testing (Sanity Only)

Reserved for critical path validation. Expensive but comprehensive:

```go
// +build browser

func TestSequence_Browser(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping browser test in short mode")
    }

    srv := startTestServer(t)
    defer srv.Close()

    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    executor := NewBrowserExecutor(ctx, srv.URL)

    err := executor.RunSequence([]BrowserAction{
        {Type: "click", Selector: "#add-btn"},
        {Type: "input", Selector: "#title", Value: "Test item"},
        {Type: "click", Selector: "#submit"},
        {Type: "click", Selector: ".todo-item:first-child .toggle"},
    })

    // Verify DOM state
    var checked bool
    err = chromedp.Run(ctx,
        chromedp.EvaluateAsDevTools(`document.querySelector('.todo-item:first-child input').checked`, &checked),
    )
    if !checked {
        t.Error("first item should be checked")
    }
}
```

---

## Fuzz Testing

### Action Generator

```go
// ActionGenerator creates valid random actions for a controller
type ActionGenerator struct {
    controller interface{}
    stateType  reflect.Type
    rng        *rand.Rand

    // Optional constraints
    weights    map[string]float64    // Action frequency weights
    dataGens   map[string]DataGen    // Custom data generators per action
}

// DataGen generates valid data for an action
type DataGen func(rng *rand.Rand, currentState interface{}) map[string]interface{}

// Generate creates a random valid action
func (g *ActionGenerator) Generate(currentState interface{}) Action {
    // 1. Get available actions for controller
    // 2. Weight selection based on config
    // 3. Generate valid data using dataGens or reflection
    // 4. Return action
}
```

### Fuzz Test Runner

```go
func FuzzTodoApp(f *testing.F) {
    ctrl := &TodoController{}

    // Action generator with custom data generators
    gen := NewActionGenerator(ctrl, reflect.TypeOf(TodoState{}))
    gen.SetWeight("add", 0.4)      // 40% adds
    gen.SetWeight("toggle", 0.3)   // 30% toggles
    gen.SetWeight("remove", 0.2)   // 20% removes
    gen.SetWeight("clear", 0.1)    // 10% clears

    gen.SetDataGen("add", func(rng *rand.Rand, state interface{}) map[string]interface{} {
        return map[string]interface{}{
            "title": randomString(rng, 1, 50),
        }
    })

    gen.SetDataGen("toggle", func(rng *rand.Rand, state interface{}) map[string]interface{} {
        s := state.(*TodoState)
        if len(s.Items) == 0 {
            return nil // Skip if no items
        }
        return map[string]interface{}{
            "index": rng.Intn(len(s.Items)),
        }
    })

    // Fuzz with random sequences
    f.Fuzz(func(t *testing.T, seed int64) {
        rng := rand.New(rand.NewSource(seed))
        state := &TodoState{}

        executor := NewDirectExecutor(ctrl, state)
        executor.WithInvariants(
            InvariantSerializable,
            InvariantNoPanic,
            InvariantNonNegativeCounts,
        )

        // Generate 10-100 random actions
        numActions := 10 + rng.Intn(91)
        actions := make([]Action, 0, numActions)

        for i := 0; i < numActions; i++ {
            action := gen.Generate(state)
            if action.Name == "" {
                continue // Skip invalid actions
            }
            actions = append(actions, action)

            // Execute to update state for next generation
            newState, _ := executor.ExecuteOne(action)
            state = newState.(*TodoState)
        }

        // Run full sequence with invariant checking
        state = &TodoState{} // Reset
        err := executor.RunSequence(actions)
        if err != nil {
            t.Fatalf("sequence failed at seed %d: %v", seed, err)
        }
    })
}
```

### Property-Based Testing with rapid

```go
import "pgregory.net/rapid"

func TestTodoProperties(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        ctrl := &TodoController{}
        state := &TodoState{}
        executor := NewDirectExecutor(ctrl, state)

        // Generate action sequence
        actions := rapid.SliceOfN(
            rapid.Custom(func(t *rapid.T) Action {
                actionType := rapid.SampledFrom([]string{"add", "toggle", "remove"}).Draw(t, "action")

                switch actionType {
                case "add":
                    return Action{
                        Name: "add",
                        Data: map[string]interface{}{
                            "title": rapid.String().Draw(t, "title"),
                        },
                    }
                case "toggle":
                    s := executor.CurrentState().(*TodoState)
                    if len(s.Items) == 0 {
                        return Action{Name: "add", Data: map[string]interface{}{"title": "x"}}
                    }
                    return Action{
                        Name: "toggle",
                        Data: map[string]interface{}{
                            "index": rapid.IntRange(0, len(s.Items)-1).Draw(t, "index"),
                        },
                    }
                case "remove":
                    s := executor.CurrentState().(*TodoState)
                    if len(s.Items) == 0 {
                        return Action{}
                    }
                    return Action{
                        Name: "remove",
                        Data: map[string]interface{}{
                            "index": rapid.IntRange(0, len(s.Items)-1).Draw(t, "index"),
                        },
                    }
                }
                return Action{}
            }),
            1, 50,
        ).Draw(t, "actions")

        err := executor.RunSequence(actions)
        if err != nil {
            t.Fatal(err)
        }

        // Property: item count matches adds minus removes
        finalState := executor.CurrentState().(*TodoState)

        adds := countActions(actions, "add")
        removes := countActions(actions, "remove")
        expected := adds - removes

        if len(finalState.Items) != expected {
            t.Errorf("item count mismatch: got %d, expected %d (adds=%d, removes=%d)",
                len(finalState.Items), expected, adds, removes)
        }
    })
}
```

---

## Test Helpers

### Common Scenarios

```go
// Package seqtest provides pre-built test scenarios

// TodoScenarios provides common todo app test sequences
var TodoScenarios = []Scenario{
    {
        Name: "two_adds",
        Actions: []Action{
            {Name: "add", Data: map[string]interface{}{"title": "First"}},
            {Name: "add", Data: map[string]interface{}{"title": "Second"}},
        },
    },
    {
        Name: "add_toggle_add",
        Actions: []Action{
            {Name: "add", Data: map[string]interface{}{"title": "A"}},
            {Name: "toggle", Data: map[string]interface{}{"index": 0}},
            {Name: "add", Data: map[string]interface{}{"title": "B"}},
        },
    },
    {
        Name: "three_marks",
        Actions: []Action{
            {Name: "add", Data: map[string]interface{}{"title": "1"}},
            {Name: "add", Data: map[string]interface{}{"title": "2"}},
            {Name: "add", Data: map[string]interface{}{"title": "3"}},
            {Name: "toggle", Data: map[string]interface{}{"index": 0}},
            {Name: "toggle", Data: map[string]interface{}{"index": 1}},
            {Name: "toggle", Data: map[string]interface{}{"index": 2}},
        },
    },
    {
        Name: "add_remove_add",
        Actions: []Action{
            {Name: "add", Data: map[string]interface{}{"title": "temp"}},
            {Name: "remove", Data: map[string]interface{}{"index": 0}},
            {Name: "add", Data: map[string]interface{}{"title": "final"}},
        },
    },
    {
        Name: "toggle_untoggle",
        Actions: []Action{
            {Name: "add", Data: map[string]interface{}{"title": "item"}},
            {Name: "toggle", Data: map[string]interface{}{"index": 0}},
            {Name: "toggle", Data: map[string]interface{}{"index": 0}},
        },
    },
}

// CounterScenarios provides common counter test sequences
var CounterScenarios = []Scenario{
    {
        Name: "ten_increments",
        Actions: repeatAction(Action{Name: "increment"}, 10),
    },
    {
        Name: "increment_decrement_mix",
        Actions: []Action{
            {Name: "increment"},
            {Name: "increment"},
            {Name: "decrement"},
            {Name: "increment"},
        },
    },
}
```

### Assertion Helpers

```go
// AssertStateField verifies a specific field value
func AssertStateField[T any](t *testing.T, state interface{}, field string, expected T) {
    v := reflect.ValueOf(state)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    f := v.FieldByName(field)
    if !f.IsValid() {
        t.Fatalf("field %q not found in state", field)
    }
    actual := f.Interface()
    if !reflect.DeepEqual(actual, expected) {
        t.Errorf("state.%s = %v, want %v", field, actual, expected)
    }
}

// AssertSliceLen verifies slice field length
func AssertSliceLen(t *testing.T, state interface{}, field string, expected int) {
    v := reflect.ValueOf(state)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    f := v.FieldByName(field)
    if f.Kind() != reflect.Slice {
        t.Fatalf("field %q is not a slice", field)
    }
    if f.Len() != expected {
        t.Errorf("len(state.%s) = %d, want %d", field, f.Len(), expected)
    }
}

// AssertNoError fails if error occurred
func AssertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

---

## Implementation Plan

### Phase 1: Core Framework (Week 1)

**Files to create:**
- `seqtest/action.go` - Action, Scenario types
- `seqtest/executor.go` - DirectExecutor implementation
- `seqtest/invariants.go` - Built-in invariants
- `seqtest/snapshot.go` - State snapshot and history

**Tasks:**
1. Define core types (Action, Scenario, Invariant)
2. Implement DirectExecutor using DispatchWithState
3. Implement state snapshot mechanism
4. Add built-in invariants (serializable, no-panic, valid-tree)
5. Write unit tests for framework itself

### Phase 2: Template Integration (Week 2)

**Files to create:**
- `seqtest/template_executor.go` - Executor with tree validation
- `seqtest/tree_invariants.go` - Tree structure invariants

**Tasks:**
1. Implement TemplateExecutor that renders after each action
2. Add tree validation invariants
3. Verify update format (statics stripped on updates)
4. Add comparison helpers for tree diffs

### Phase 3: Fuzz Testing (Week 3)

**Files to create:**
- `seqtest/generator.go` - ActionGenerator
- `seqtest/fuzz/fuzz.go` - Go fuzz test integration
- `seqtest/rapid/rapid.go` - rapid integration (optional)

**Tasks:**
1. Implement ActionGenerator with weighted selection
2. Add data generator registration
3. Integrate with Go's testing/fuzzing
4. Create example fuzz tests for todo app
5. Add seed replay for failing sequences

### Phase 4: HTTP & Browser Testing (Week 4)

**Files to create:**
- `seqtest/http_executor.go` - HTTP handler testing
- `seqtest/browser_executor.go` - chromedp integration

**Tasks:**
1. Implement HTTPExecutor with httptest
2. Add session/cookie handling for multi-request sequences
3. Implement BrowserExecutor with chromedp
4. Add browser action types (click, input, wait)
5. Create sanity test suite

### Phase 5: Test Suite & Documentation (Week 5)

**Files to create:**
- `seqtest/scenarios/` - Pre-built scenario packages
- `seqtest/doc.go` - Package documentation

**Tasks:**
1. Create scenario packages (todo, counter, form)
2. Add table-driven test helpers
3. Write comprehensive documentation
4. Add examples in _test.go files
5. Create migration guide from existing tests

---

## Usage Examples

### Basic Sequential Test

```go
func TestTodoSequence(t *testing.T) {
    ctrl := &TodoController{DB: testDB}

    seqtest.Run(t, ctrl, &TodoState{},
        seqtest.Action("add", "title", "Buy milk"),
        seqtest.Action("add", "title", "Walk dog"),
        seqtest.Action("toggle", "index", 0),
        seqtest.Expect(func(s *TodoState) error {
            if len(s.Items) != 2 {
                return fmt.Errorf("want 2 items, got %d", len(s.Items))
            }
            if !s.Items[0].Done {
                return errors.New("first item should be done")
            }
            return nil
        }),
    )
}
```

### Table-Driven Sequential Tests

```go
func TestTodoScenarios(t *testing.T) {
    ctrl := &TodoController{DB: testDB}

    for _, scenario := range seqtest.TodoScenarios {
        t.Run(scenario.Name, func(t *testing.T) {
            executor := seqtest.NewDirectExecutor(ctrl, &TodoState{})
            executor.WithInvariants(seqtest.DefaultInvariants...)

            err := executor.Run(scenario)
            if err != nil {
                t.Fatal(err)
            }
        })
    }
}
```

### Fuzz Test

```go
func FuzzTodoApp(f *testing.F) {
    // Add seed corpus
    f.Add(int64(12345))
    f.Add(int64(67890))

    f.Fuzz(func(t *testing.T, seed int64) {
        ctrl := &TodoController{}

        err := seqtest.FuzzRun(t, seed, ctrl, &TodoState{},
            seqtest.WithMaxActions(100),
            seqtest.WithActionWeights(map[string]float64{
                "add":    0.4,
                "toggle": 0.3,
                "remove": 0.2,
                "clear":  0.1,
            }),
        )
        if err != nil {
            t.Fatalf("fuzz failed at seed %d: %v", seed, err)
        }
    })
}
```

### Browser Sanity Test

```go
// +build browser

func TestTodoE2E(t *testing.T) {
    if testing.Short() {
        t.Skip("browser tests disabled in short mode")
    }

    srv := startTodoServer(t)
    defer srv.Close()

    seqtest.RunBrowser(t, srv.URL,
        seqtest.BrowserAction("navigate", "/todos"),
        seqtest.BrowserAction("type", "#new-todo", "Test item"),
        seqtest.BrowserAction("click", "#add-btn"),
        seqtest.BrowserAction("wait", ".todo-item"),
        seqtest.BrowserAction("click", ".todo-item .toggle"),
        seqtest.BrowserExpect("checked", ".todo-item input[type=checkbox]"),
    )
}
```

---

## Invariants Reference

| Invariant | Description | Use Case |
|-----------|-------------|----------|
| `InvariantSerializable` | State can be JSON marshaled | Detect leaked dependencies |
| `InvariantNoPanic` | No panics during execution | Basic stability |
| `InvariantValidTree` | Tree structure is valid | Template updates |
| `InvariantNonNegativeCounts` | Count fields >= 0 | Counter/quantity fields |
| `InvariantIdempotent` | Same action twice = same result | Toggles, sets |
| `InvariantMonotonic` | Field only increases | Sequence numbers |
| `InvariantBounded` | Field stays in range | Percentages, indices |

---

## Configuration

### Environment Variables

```bash
# Enable browser tests
LVT_BROWSER_TESTS=1

# Set fuzz iterations
LVT_FUZZ_ITERATIONS=10000

# Browser test timeout
LVT_BROWSER_TIMEOUT=30s

# Parallel browser instances
LVT_BROWSER_PARALLEL=4
```

### Test Tags

```go
// +build !browser      // Exclude browser tests by default
// +build browser       // Include only in browser test runs
// +build fuzz          // Long-running fuzz tests
```

---

## Metrics & Reporting

### Test Output

```
=== RUN   TestTodoSequence
    executor.go:45: Action 1/5: add {title: "Buy milk"}
    executor.go:45: Action 2/5: add {title: "Walk dog"}
    executor.go:45: Action 3/5: toggle {index: 0}
    executor.go:45: Action 4/5: toggle {index: 1}
    executor.go:45: Action 5/5: toggle {index: 0}
    executor.go:78: Sequence completed in 1.2ms
    executor.go:79: State transitions: 5
    executor.go:80: Invariant checks: 25 (5 actions × 5 invariants)
--- PASS: TestTodoSequence (0.00s)
```

### Fuzz Test Coverage

```
=== FUZZ  FuzzTodoApp
    fuzz.go:123: Explored 10,000 sequences
    fuzz.go:124: Unique action combinations: 847
    fuzz.go:125: Max sequence length tested: 100
    fuzz.go:126: Coverage: 94.2% of action method code
    fuzz.go:127: No failures found
--- PASS: FuzzTodoApp (12.34s)
```

---

## Integration with Existing Tests

The framework complements existing tests:

| Test Type | Existing | Sequential Framework |
|-----------|----------|---------------------|
| Unit tests | `dispatch_test.go` | `seqtest.DirectExecutor` |
| Template tests | `template_test.go` | `seqtest.TemplateExecutor` |
| E2E benchmarks | `e2e_bench_test.go` | `seqtest.Scenario` |
| Browser tests | `lvt/e2e/` | `seqtest.BrowserExecutor` |

Migration is incremental - existing tests continue working while new sequential tests are added.
