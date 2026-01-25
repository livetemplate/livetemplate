package seqtest_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/livetemplate/livetemplate"
	"github.com/livetemplate/livetemplate/seqtest"
)

// =============================================================================
// Test Controller and State for Todo App
// =============================================================================

type Todo struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type TodoState struct {
	Items []Todo `json:"items"`
}

type TodoController struct{}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	title := ctx.GetString("title")
	if title == "" {
		return state, errors.New("title is required")
	}
	state.Items = append(state.Items, Todo{Title: title, Done: false})
	return state, nil
}

func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	index := ctx.GetInt("index")
	if index < 0 || index >= len(state.Items) {
		return state, fmt.Errorf("invalid index: %d", index)
	}
	state.Items[index].Done = !state.Items[index].Done
	return state, nil
}

func (c *TodoController) Remove(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	index := ctx.GetInt("index")
	if index < 0 || index >= len(state.Items) {
		return state, fmt.Errorf("invalid index: %d", index)
	}
	state.Items = append(state.Items[:index], state.Items[index+1:]...)
	return state, nil
}

func (c *TodoController) ClearCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var remaining []Todo
	for _, item := range state.Items {
		if !item.Done {
			remaining = append(remaining, item)
		}
	}
	state.Items = remaining
	return state, nil
}

// =============================================================================
// Test Controller and State for Counter
// =============================================================================

type CounterState struct {
	Count int `json:"count"`
}

type CounterController struct{}

func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Count++
	return state, nil
}

func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Count--
	return state, nil
}

func (c *CounterController) Add(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	amount := ctx.GetInt("amount")
	state.Count += amount
	return state, nil
}

func (c *CounterController) Set(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Count = ctx.GetInt("value")
	return state, nil
}

// =============================================================================
// Basic Sequential Tests
// =============================================================================

func TestSequential_TwoAdds(t *testing.T) {
	ctrl := &TodoController{}
	state := TodoState{}

	executor := seqtest.NewDirectExecutor(ctrl, state)

	err := executor.RunSequence([]seqtest.Action{
		{Name: "add", Data: map[string]interface{}{"title": "First"}},
		{Name: "add", Data: map[string]interface{}{"title": "Second"}},
	})
	if err != nil {
		t.Fatalf("RunSequence failed: %v", err)
	}

	finalState := executor.CurrentState().(TodoState)
	if len(finalState.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(finalState.Items))
	}
	if finalState.Items[0].Title != "First" {
		t.Errorf("Expected first item 'First', got %q", finalState.Items[0].Title)
	}
	if finalState.Items[1].Title != "Second" {
		t.Errorf("Expected second item 'Second', got %q", finalState.Items[1].Title)
	}
}

func TestSequential_ThreeMarks(t *testing.T) {
	ctrl := &TodoController{}
	state := TodoState{}

	executor := seqtest.NewDirectExecutor(ctrl, state)

	err := executor.RunSequence([]seqtest.Action{
		{Name: "add", Data: map[string]interface{}{"title": "Item 1"}},
		{Name: "add", Data: map[string]interface{}{"title": "Item 2"}},
		{Name: "add", Data: map[string]interface{}{"title": "Item 3"}},
		{Name: "toggle", Data: map[string]interface{}{"index": 0}},
		{Name: "toggle", Data: map[string]interface{}{"index": 1}},
		{Name: "toggle", Data: map[string]interface{}{"index": 2}},
	})
	if err != nil {
		t.Fatalf("RunSequence failed: %v", err)
	}

	finalState := executor.CurrentState().(TodoState)
	if len(finalState.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(finalState.Items))
	}
	for i, item := range finalState.Items {
		if !item.Done {
			t.Errorf("Item %d should be done", i)
		}
	}
}

func TestSequential_ToggleUntoggle(t *testing.T) {
	ctrl := &TodoController{}
	state := TodoState{}

	executor := seqtest.NewDirectExecutor(ctrl, state)

	err := executor.RunSequence([]seqtest.Action{
		{Name: "add", Data: map[string]interface{}{"title": "item"}},
		{Name: "toggle", Data: map[string]interface{}{"index": 0}},
		{Name: "toggle", Data: map[string]interface{}{"index": 0}},
	})
	if err != nil {
		t.Fatalf("RunSequence failed: %v", err)
	}

	finalState := executor.CurrentState().(TodoState)
	if finalState.Items[0].Done {
		t.Error("Item should not be done after toggle-untoggle")
	}
}

func TestSequential_MixedOperations(t *testing.T) {
	ctrl := &TodoController{}
	state := TodoState{}

	scenario := seqtest.Scenario{
		Name: "mixed_operations",
		Actions: []seqtest.Action{
			{Name: "add", Data: map[string]interface{}{"title": "1"}},
			{Name: "add", Data: map[string]interface{}{"title": "2"}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "add", Data: map[string]interface{}{"title": "3"}},
			{Name: "remove", Data: map[string]interface{}{"index": 1}},
			{Name: "toggle", Data: map[string]interface{}{"index": 0}},
			{Name: "add", Data: map[string]interface{}{"title": "4"}},
		},
		Validate: func(state interface{}) error {
			s := state.(TodoState)
			// After: add 1, add 2, toggle 0, add 3, remove 1 (removes "2"), toggle 0, add 4
			// Items: [1 (not done, toggled twice), 3, 4]
			if len(s.Items) != 3 {
				return fmt.Errorf("expected 3 items, got %d", len(s.Items))
			}
			if s.Items[0].Done {
				return errors.New("item 0 should not be done (toggled twice)")
			}
			return nil
		},
	}

	executor := seqtest.NewDirectExecutor(ctrl, state)
	if err := executor.Run(scenario); err != nil {
		t.Fatal(err)
	}
}

// =============================================================================
// Counter Tests
// =============================================================================

func TestSequential_TenIncrements(t *testing.T) {
	ctrl := &CounterController{}
	state := CounterState{Count: 0}

	executor := seqtest.NewDirectExecutor(ctrl, state)

	actions := seqtest.RepeatAction(seqtest.Action{Name: "increment"}, 10)
	if err := executor.RunSequence(actions); err != nil {
		t.Fatal(err)
	}

	finalState := executor.CurrentState().(CounterState)
	if finalState.Count != 10 {
		t.Errorf("Expected count 10, got %d", finalState.Count)
	}
}

func TestSequential_IncrementDecrementMix(t *testing.T) {
	ctrl := &CounterController{}
	state := CounterState{Count: 0}

	executor := seqtest.NewDirectExecutor(ctrl, state)

	err := executor.RunSequence([]seqtest.Action{
		{Name: "increment"},
		{Name: "increment"},
		{Name: "decrement"},
		{Name: "increment"},
		{Name: "decrement"},
		{Name: "decrement"},
		{Name: "increment"},
	})
	if err != nil {
		t.Fatal(err)
	}

	finalState := executor.CurrentState().(CounterState)
	// +1 +1 -1 +1 -1 -1 +1 = 1
	if finalState.Count != 1 {
		t.Errorf("Expected count 1, got %d", finalState.Count)
	}
}

// =============================================================================
// Invariant Tests
// =============================================================================

func TestInvariant_Serializable(t *testing.T) {
	ctrl := &TodoController{}
	state := TodoState{}

	executor := seqtest.NewDirectExecutor(ctrl, state)
	executor.WithInvariants(seqtest.InvariantSerializable)

	err := executor.RunSequence([]seqtest.Action{
		{Name: "add", Data: map[string]interface{}{"title": "test"}},
	})
	if err != nil {
		t.Fatalf("Serializable invariant should pass: %v", err)
	}
}

func TestInvariant_NonNegativeCounts(t *testing.T) {
	ctrl := &CounterController{}
	state := CounterState{Count: 0}

	executor := seqtest.NewDirectExecutor(ctrl, state)
	executor.WithInvariants(seqtest.InvariantNonNegativeCounts)

	// This should pass - count goes to 1
	err := executor.RunSequence([]seqtest.Action{
		{Name: "increment"},
	})
	if err != nil {
		t.Fatalf("Should pass with positive count: %v", err)
	}

	// Reset and try decrement from 0 (goes negative)
	executor.Reset()
	err = executor.RunSequence([]seqtest.Action{
		{Name: "decrement"},
	})
	if err == nil {
		t.Error("Should fail with negative count invariant")
	}
}

func TestInvariant_FieldRange(t *testing.T) {
	ctrl := &CounterController{}
	state := CounterState{Count: 0}

	// Count must be between 0 and 100
	rangeInvariant := seqtest.NewInvariantFieldRange("Count", 0, 100)

	executor := seqtest.NewDirectExecutor(ctrl, state)
	executor.WithInvariants(rangeInvariant)

	// Should pass
	err := executor.RunSequence(seqtest.RepeatAction(seqtest.Action{Name: "increment"}, 50))
	if err != nil {
		t.Fatalf("Should pass within range: %v", err)
	}

	// Should fail (exceed 100)
	executor.Reset()
	err = executor.RunSequence([]seqtest.Action{
		{Name: "set", Data: map[string]interface{}{"value": 101}},
	})
	if err == nil {
		t.Error("Should fail when exceeding range")
	}
}

// =============================================================================
// Fuzz Testing
// =============================================================================

func TestFuzz_TodoApp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuzz test in short mode")
	}

	ctrl := &TodoController{}

	gen := seqtest.NewActionGenerator(ctrl, reflect.TypeOf(TodoState{}))
	gen.WithSeed(12345) // Reproducible

	// Configure weights
	gen.SetWeight("add", 0.5)
	gen.SetWeight("toggle", 0.3)
	gen.SetWeight("remove", 0.15)
	gen.SetWeight("clearCompleted", 0.05)

	// Custom data generators
	gen.SetDataGen("add", func(rng *rand.Rand, state interface{}) map[string]interface{} {
		titles := []string{"Buy milk", "Walk dog", "Write tests", "Review PR", "Deploy"}
		return map[string]interface{}{
			"title": titles[rng.Intn(len(titles))],
		}
	})

	gen.SetDataGen("toggle", seqtest.IndexGen("Items"))
	gen.SetDataGen("remove", seqtest.IndexGen("Items"))

	// Skip toggle/remove when no items
	gen.SetSkipFunc("toggle", func(state interface{}) bool {
		return len(state.(*TodoState).Items) == 0
	})
	gen.SetSkipFunc("remove", func(state interface{}) bool {
		return len(state.(*TodoState).Items) == 0
	})

	// Run fuzz test
	executor := seqtest.NewDirectExecutor(ctrl, &TodoState{})
	executor.WithInvariants(seqtest.InvariantSerializable)

	// Generate 100 actions
	actions := gen.GenerateN(100, executor)

	// Reset and replay with all invariants
	executor.Reset()
	err := executor.RunSequence(actions)
	if err != nil {
		t.Fatalf("Fuzz sequence failed: %v", err)
	}
}

func TestFuzz_Counter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuzz test in short mode")
	}

	ctrl := &CounterController{}

	config := seqtest.DefaultFuzzConfig()
	config.MinActions = 10
	config.MaxActions = 100
	config.Weights = map[string]float64{
		"increment": 0.4,
		"decrement": 0.4,
		"add":       0.15,
		"set":       0.05,
	}
	config.DataGens = map[string]seqtest.DataGen{
		"add": func(rng *rand.Rand, state interface{}) map[string]interface{} {
			return map[string]interface{}{"amount": rng.Intn(10) - 5}
		},
		"set": func(rng *rand.Rand, state interface{}) map[string]interface{} {
			return map[string]interface{}{"value": rng.Intn(100)}
		},
	}

	runner := seqtest.NewFuzzRunner(ctrl, reflect.TypeOf(CounterState{}), config)

	// Run with multiple seeds
	seeds := []int64{1, 42, 123, 456, 789}
	for _, seed := range seeds {
		result, err := runner.Run(seed, &CounterState{Count: 0})
		if err != nil {
			t.Logf("Failing sequence:\n%s", result.ReplaySequence())
			t.Fatalf("Fuzz test failed with seed %d: %v", seed, err)
		}
	}
}

// =============================================================================
// Scenario Tests
// =============================================================================

func TestScenarios_Todo(t *testing.T) {
	// Filter to only scenarios we can test (ones our controller supports)
	testableScenarios := []seqtest.Scenario{
		seqtest.TodoScenarios[0], // two_sequential_adds
		seqtest.TodoScenarios[1], // three_sequential_marks
		seqtest.TodoScenarios[2], // add_toggle_add_toggle
		seqtest.TodoScenarios[3], // add_remove_add
		seqtest.TodoScenarios[4], // toggle_untoggle
		seqtest.TodoScenarios[5], // mixed_operations
	}

	ctrl := &TodoController{}

	for _, scenario := range testableScenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			executor := seqtest.NewDirectExecutor(ctrl, TodoState{})
			executor.WithInvariants(seqtest.DefaultInvariants...)

			err := executor.Run(scenario)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScenarios_Counter(t *testing.T) {
	ctrl := &CounterController{}

	for _, scenario := range seqtest.CounterScenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			executor := seqtest.NewDirectExecutor(ctrl, CounterState{})

			err := executor.Run(scenario)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// =============================================================================
// Action Helper Tests
// =============================================================================

func TestNewAction(t *testing.T) {
	action := seqtest.NewAction("add", "title", "Test", "priority", 1)

	if action.Name != "add" {
		t.Errorf("Expected name 'add', got %q", action.Name)
	}
	if action.Data["title"] != "Test" {
		t.Errorf("Expected title 'Test', got %v", action.Data["title"])
	}
	if action.Data["priority"] != 1 {
		t.Errorf("Expected priority 1, got %v", action.Data["priority"])
	}
}

func TestRepeatAction(t *testing.T) {
	actions := seqtest.RepeatAction(seqtest.Action{Name: "increment"}, 5)

	if len(actions) != 5 {
		t.Errorf("Expected 5 actions, got %d", len(actions))
	}
	for i, action := range actions {
		if action.Name != "increment" {
			t.Errorf("Action %d: expected 'increment', got %q", i, action.Name)
		}
	}
}

func TestCycleActions(t *testing.T) {
	base := []seqtest.Action{
		{Name: "a"},
		{Name: "b"},
	}

	cycled := seqtest.CycleActions(base, 3)

	if len(cycled) != 6 {
		t.Errorf("Expected 6 actions, got %d", len(cycled))
	}

	expected := []string{"a", "b", "a", "b", "a", "b"}
	for i, action := range cycled {
		if action.Name != expected[i] {
			t.Errorf("Action %d: expected %q, got %q", i, expected[i], action.Name)
		}
	}
}

// =============================================================================
// History Tests
// =============================================================================

func TestHistory_Recording(t *testing.T) {
	ctrl := &CounterController{}
	state := CounterState{Count: 0}

	executor := seqtest.NewDirectExecutor(ctrl, state)

	executor.RunSequence([]seqtest.Action{
		{Name: "increment"},
		{Name: "increment"},
		{Name: "decrement"},
	})

	history := executor.History()

	if history.Len() != 3 {
		t.Errorf("Expected 3 recorded transitions, got %d", history.Len())
	}

	// Check initial state
	initial := history.Before(0)
	if initial.State.(CounterState).Count != 0 {
		t.Errorf("Initial state should have count 0")
	}

	// Check after first increment
	after1 := history.At(0)
	if after1.State.(CounterState).Count != 1 {
		t.Errorf("After first action, count should be 1")
	}

	// Check final
	final := history.Final()
	if final.State.(CounterState).Count != 1 {
		t.Errorf("Final count should be 1 (2 inc, 1 dec)")
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestRun_Convenience(t *testing.T) {
	ctrl := &TodoController{}

	err := seqtest.Run(seqtest.Scenario{
		Actions: []seqtest.Action{
			{Name: "add", Data: map[string]interface{}{"title": "Test"}},
		},
		Validate: func(state interface{}) error {
			s := state.(TodoState)
			if len(s.Items) != 1 {
				return errors.New("expected 1 item")
			}
			return nil
		},
	}, ctrl, TodoState{})

	if err != nil {
		t.Fatal(err)
	}
}

func TestRunSequence_Convenience(t *testing.T) {
	ctrl := &CounterController{}

	err := seqtest.RunSequence(ctrl, CounterState{},
		seqtest.Action{Name: "increment"},
		seqtest.Action{Name: "increment"},
	)

	if err != nil {
		t.Fatal(err)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkDirectExecutor_Simple(b *testing.B) {
	ctrl := &CounterController{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor := seqtest.NewDirectExecutor(ctrl, CounterState{})
		executor.RunSequence(seqtest.RepeatAction(seqtest.Action{Name: "increment"}, 100))
	}
}

func BenchmarkDirectExecutor_WithInvariants(b *testing.B) {
	ctrl := &CounterController{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor := seqtest.NewDirectExecutor(ctrl, CounterState{})
		executor.WithInvariants(
			seqtest.InvariantSerializable,
			seqtest.InvariantNonNegativeCounts,
		)
		executor.RunSequence(seqtest.RepeatAction(seqtest.Action{Name: "increment"}, 100))
	}
}

func BenchmarkActionGenerator(b *testing.B) {
	ctrl := &TodoController{}
	gen := seqtest.NewActionGenerator(ctrl, reflect.TypeOf(TodoState{}))
	gen.WithSeed(12345)
	gen.SetDataGen("add", func(rng *rand.Rand, state interface{}) map[string]interface{} {
		return map[string]interface{}{"title": "test"}
	})
	gen.SetDataGen("toggle", seqtest.IndexGen("Items"))
	gen.SetDataGen("remove", seqtest.IndexGen("Items"))
	gen.SetSkipFunc("toggle", func(state interface{}) bool {
		return len(state.(*TodoState).Items) == 0
	})
	gen.SetSkipFunc("remove", func(state interface{}) bool {
		return len(state.(*TodoState).Items) == 0
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor := seqtest.NewDirectExecutor(ctrl, &TodoState{})
		gen.GenerateN(100, executor)
	}
}

// Helper to create context for manual testing
func newTestContext(action string, data map[string]interface{}) *livetemplate.Context {
	return livetemplate.NewContext(context.Background(), action, data)
}
