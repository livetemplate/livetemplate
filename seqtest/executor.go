package seqtest

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate"
)

// Executor is the interface for running action sequences
type Executor interface {
	// Run executes a complete scenario
	Run(scenario Scenario) error

	// RunSequence executes a slice of actions
	RunSequence(actions []Action) error

	// ExecuteOne executes a single action and returns the new state
	ExecuteOne(action Action) (interface{}, error)

	// CurrentState returns the current state
	CurrentState() interface{}

	// History returns the state history
	History() *StateHistory

	// WithInvariants adds invariants to check after each action
	WithInvariants(invariants ...Invariant) Executor

	// Reset resets the executor to initial state
	Reset()
}

// DirectExecutor executes actions using DispatchWithState directly
// This is the fastest executor, suitable for unit tests and fuzz testing
type DirectExecutor struct {
	controller   interface{}
	initialState interface{}
	currentState interface{}
	history      *StateHistory
	invariants   []Invariant

	mu sync.Mutex
}

// NewDirectExecutor creates a new DirectExecutor
func NewDirectExecutor(controller interface{}, initialState interface{}) *DirectExecutor {
	// Clone initial state to preserve original
	clonedState := cloneState(initialState)

	return &DirectExecutor{
		controller:   controller,
		initialState: initialState,
		currentState: clonedState,
		history:      NewStateHistory(clonedState),
		invariants:   DefaultInvariants,
	}
}

// WithInvariants adds invariants to check after each action
func (e *DirectExecutor) WithInvariants(invariants ...Invariant) Executor {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.invariants = append(e.invariants, invariants...)
	return e
}

// CurrentState returns the current state
func (e *DirectExecutor) CurrentState() interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentState
}

// History returns the state history
func (e *DirectExecutor) History() *StateHistory {
	return e.history
}

// Reset resets the executor to initial state
func (e *DirectExecutor) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentState = cloneState(e.initialState)
	e.history = NewStateHistory(e.currentState)
}

// Run executes a complete scenario
func (e *DirectExecutor) Run(scenario Scenario) error {
	e.Reset()

	// Use custom setup if provided
	if scenario.Setup != nil {
		ctrl, state := scenario.Setup()
		if ctrl != nil {
			e.controller = ctrl
		}
		if state != nil {
			e.currentState = state
			e.history = NewStateHistory(state)
		}
	}

	// Add scenario-specific invariants
	invariants := e.invariants
	if len(scenario.Invariants) > 0 {
		invariants = append(invariants, scenario.Invariants...)
	}

	// Execute actions
	for i, action := range scenario.Actions {
		if err := e.executeWithInvariants(i, action, invariants); err != nil {
			return fmt.Errorf("action %d (%s) failed: %w", i, action.Name, err)
		}
	}

	// Run validation if provided
	if scenario.Validate != nil {
		if err := scenario.Validate(e.currentState); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return nil
}

// RunSequence executes a slice of actions
func (e *DirectExecutor) RunSequence(actions []Action) error {
	return e.Run(Scenario{Actions: actions})
}

// ExecuteOne executes a single action and returns the new state
func (e *DirectExecutor) ExecuteOne(action Action) (interface{}, error) {
	return e.executeAction(action)
}

// executeAction dispatches an action and updates state
func (e *DirectExecutor) executeAction(action Action) (interface{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ctx := livetemplate.NewContext(context.Background(), action.Name, action.Data)

	newState, err := livetemplate.DispatchWithState(e.controller, e.currentState, ctx)
	if err != nil {
		return e.currentState, err
	}

	e.currentState = newState
	return newState, nil
}

// executeWithInvariants executes an action and checks invariants
func (e *DirectExecutor) executeWithInvariants(index int, action Action, invariants []Invariant) error {
	// Capture state before
	e.mu.Lock()
	stateBefore := e.currentState
	e.mu.Unlock()

	beforeSnapshot := StateSnapshot{
		State:     stateBefore,
		Timestamp: time.Now(),
	}

	start := time.Now()

	// Execute action with panic recovery
	var newState interface{}
	var actionErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				actionErr = fmt.Errorf("panic during action %s: %v", action.Name, r)
			}
		}()
		newState, actionErr = e.ExecuteOne(action)
	}()

	duration := time.Since(start)

	// Capture state after (even on error, state might have changed)
	e.mu.Lock()
	stateAfter := e.currentState
	e.mu.Unlock()

	afterSnapshot := StateSnapshot{
		State:     stateAfter,
		Timestamp: time.Now(),
	}

	// Record in history
	e.history.Record(action, stateAfter, nil)

	// Return action error if any
	if actionErr != nil {
		return actionErr
	}

	// Check invariants
	for _, inv := range invariants {
		if err := inv(beforeSnapshot, afterSnapshot, action); err != nil {
			return fmt.Errorf("invariant violation after action %d (%s): %w",
				index, action.Name, err)
		}
	}

	_ = duration // Could be used for metrics
	_ = newState

	return nil
}

// cloneState creates a deep copy of state via JSON roundtrip
func cloneState(state interface{}) interface{} {
	if state == nil {
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return state // Return original if marshal fails
	}

	// Create new instance of same type
	stateType := reflect.TypeOf(state)
	var newState interface{}

	if stateType.Kind() == reflect.Ptr {
		newPtr := reflect.New(stateType.Elem())
		if err := json.Unmarshal(data, newPtr.Interface()); err != nil {
			return state
		}
		newState = newPtr.Interface()
	} else {
		newPtr := reflect.New(stateType)
		if err := json.Unmarshal(data, newPtr.Interface()); err != nil {
			return state
		}
		newState = newPtr.Elem().Interface()
	}

	return newState
}

// Run is a convenience function to run a scenario with a controller and state
func Run(scenario Scenario, controller interface{}, initialState interface{}) error {
	executor := NewDirectExecutor(controller, initialState)
	return executor.Run(scenario)
}

// RunSequence is a convenience function to run actions with a controller and state
func RunSequence(controller interface{}, initialState interface{}, actions ...Action) error {
	executor := NewDirectExecutor(controller, initialState)
	return executor.RunSequence(actions)
}
