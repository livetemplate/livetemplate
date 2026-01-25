// Package seqtest provides a framework for testing sequential user activities
// in LiveTemplate applications. It supports deterministic sequences, fuzz testing,
// and both browser-less and browser-based execution modes.
package seqtest

import (
	"fmt"
	"time"
)

// Action represents a single user action in a test sequence
type Action struct {
	Name string                 // Action method name (e.g., "add", "toggle", "increment")
	Data map[string]interface{} // Action payload data
}

// String returns a human-readable representation of the action
func (a Action) String() string {
	if len(a.Data) == 0 {
		return a.Name
	}
	return fmt.Sprintf("%s(%v)", a.Name, a.Data)
}

// NewAction creates an action with the given name and key-value pairs
// Example: NewAction("add", "title", "Buy milk", "priority", 1)
func NewAction(name string, kvPairs ...interface{}) Action {
	data := make(map[string]interface{})
	for i := 0; i+1 < len(kvPairs); i += 2 {
		if key, ok := kvPairs[i].(string); ok {
			data[key] = kvPairs[i+1]
		}
	}
	return Action{Name: name, Data: data}
}

// Scenario defines a test scenario with a sequence of actions and validation
type Scenario struct {
	Name       string                            // Scenario name for test output
	Setup      func() (interface{}, interface{}) // Optional: returns (controller, initialState)
	Actions    []Action                          // Sequence of actions to execute
	Validate   func(state interface{}) error     // Post-scenario validation
	Invariants []Invariant                       // Per-action invariants to check
}

// StepResult captures the outcome of a single action execution
type StepResult struct {
	Index       int           // Action index in sequence
	Action      Action        // The action executed
	StateBefore interface{}   // State before action
	StateAfter  interface{}   // State after action
	Error       error         // Error from action (if any)
	Duration    time.Duration // Execution time
}

// SequenceResult captures the outcome of an entire sequence execution
type SequenceResult struct {
	Scenario        string
	Steps           []StepResult
	FinalState      interface{}
	TotalDuration   time.Duration
	InvariantError  error // First invariant violation (if any)
	ValidationError error // Validation function error (if any)
}

// Success returns true if the sequence completed without errors
func (r *SequenceResult) Success() bool {
	if r.InvariantError != nil || r.ValidationError != nil {
		return false
	}
	for _, step := range r.Steps {
		if step.Error != nil {
			return false
		}
	}
	return true
}

// FirstError returns the first error encountered, or nil if successful
func (r *SequenceResult) FirstError() error {
	for _, step := range r.Steps {
		if step.Error != nil {
			return fmt.Errorf("action %d (%s): %w", step.Index, step.Action.Name, step.Error)
		}
	}
	if r.InvariantError != nil {
		return fmt.Errorf("invariant violation: %w", r.InvariantError)
	}
	if r.ValidationError != nil {
		return fmt.Errorf("validation failed: %w", r.ValidationError)
	}
	return nil
}

// RepeatAction creates a slice with the same action repeated n times
func RepeatAction(action Action, n int) []Action {
	actions := make([]Action, n)
	for i := range actions {
		actions[i] = action
	}
	return actions
}

// CycleActions creates a sequence that cycles through actions n times
func CycleActions(actions []Action, n int) []Action {
	result := make([]Action, 0, len(actions)*n)
	for i := 0; i < n; i++ {
		result = append(result, actions...)
	}
	return result
}
