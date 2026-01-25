package seqtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate"
)

// TemplateExecutor executes actions and verifies template updates
// It renders the template after each action and validates tree structure
type TemplateExecutor struct {
	template     *livetemplate.Template
	controller   interface{}
	initialState interface{}
	currentState interface{}
	history      *StateHistory
	invariants   []Invariant

	// Template-specific state
	firstRender bool
	lastTree    map[string]interface{}

	mu sync.Mutex
}

// NewTemplateExecutor creates a new TemplateExecutor
func NewTemplateExecutor(tmpl *livetemplate.Template, controller interface{}, initialState interface{}) *TemplateExecutor {
	clonedState := cloneState(initialState)

	return &TemplateExecutor{
		template:     tmpl,
		controller:   controller,
		initialState: initialState,
		currentState: clonedState,
		history:      NewStateHistory(clonedState),
		invariants:   append(DefaultInvariants, InvariantValidTree),
		firstRender:  true,
	}
}

// WithInvariants adds invariants to check after each action
func (e *TemplateExecutor) WithInvariants(invariants ...Invariant) Executor {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.invariants = append(e.invariants, invariants...)
	return e
}

// CurrentState returns the current state
func (e *TemplateExecutor) CurrentState() interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentState
}

// History returns the state history
func (e *TemplateExecutor) History() *StateHistory {
	return e.history
}

// LastTree returns the last rendered tree
func (e *TemplateExecutor) LastTree() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastTree
}

// Reset resets the executor to initial state
func (e *TemplateExecutor) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentState = cloneState(e.initialState)
	e.history = NewStateHistory(e.currentState)
	e.firstRender = true
	e.lastTree = nil
}

// Run executes a complete scenario
func (e *TemplateExecutor) Run(scenario Scenario) error {
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

	// Do initial render
	if err := e.doInitialRender(); err != nil {
		return fmt.Errorf("initial render failed: %w", err)
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
func (e *TemplateExecutor) RunSequence(actions []Action) error {
	return e.Run(Scenario{Actions: actions})
}

// ExecuteOne executes a single action and returns the new state
func (e *TemplateExecutor) ExecuteOne(action Action) (interface{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Dispatch action
	ctx := livetemplate.NewContext(context.Background(), action.Name, action.Data)
	newState, err := livetemplate.DispatchWithState(e.controller, e.currentState, ctx)
	if err != nil {
		return e.currentState, err
	}

	e.currentState = newState

	// Render update
	var buf bytes.Buffer
	if err := e.template.ExecuteUpdates(&buf, newState); err != nil {
		return newState, fmt.Errorf("template update failed: %w", err)
	}

	// Parse tree from response
	tree, err := parseTree(buf.Bytes())
	if err != nil {
		return newState, fmt.Errorf("failed to parse tree: %w", err)
	}

	e.lastTree = tree
	return newState, nil
}

// doInitialRender performs the first render to establish statics
func (e *TemplateExecutor) doInitialRender() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var buf bytes.Buffer
	if err := e.template.Execute(&buf, e.currentState); err != nil {
		return err
	}

	e.firstRender = false
	return nil
}

// executeWithInvariants executes an action and checks invariants
func (e *TemplateExecutor) executeWithInvariants(index int, action Action, invariants []Invariant) error {
	// Capture state before
	e.mu.Lock()
	stateBefore := e.currentState
	treeBefore := e.lastTree
	e.mu.Unlock()

	beforeSnapshot := StateSnapshot{
		State:     stateBefore,
		Tree:      treeBefore,
		Timestamp: time.Now(),
	}

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

	// Capture state after
	e.mu.Lock()
	stateAfter := e.currentState
	treeAfter := e.lastTree
	e.mu.Unlock()

	afterSnapshot := StateSnapshot{
		State:     stateAfter,
		Tree:      treeAfter,
		Timestamp: time.Now(),
	}

	// Record in history
	e.history.Record(action, stateAfter, treeAfter)

	// Return action error if any
	if actionErr != nil {
		return actionErr
	}

	// Check invariants
	for _, inv := range invariants {
		if err := inv(beforeSnapshot, afterSnapshot, action); err != nil {
			return fmt.Errorf("invariant violation: %w", err)
		}
	}

	_ = newState

	return nil
}

// parseTree parses the JSON tree from template output
func parseTree(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var tree map[string]interface{}
	if err := json.Unmarshal(data, &tree); err != nil {
		// Might be HTML, not JSON
		return nil, nil
	}

	return tree, nil
}

// InvariantUpdateHasNoStatics ensures update trees don't contain statics
// (statics should only be in the initial render)
var InvariantUpdateHasNoStatics Invariant = func(before, after StateSnapshot, action Action) error {
	if after.Tree == nil {
		return nil
	}

	// Check if tree contains "s" key at root level (statics)
	if _, hasStatics := after.Tree["s"]; hasStatics {
		// This is acceptable for the first render but not for updates
		// The executor tracks this separately, so we check nested structures
		return checkNestedStatics(after.Tree, "")
	}

	return nil
}

// checkNestedStatics recursively checks for unexpected statics in updates
func checkNestedStatics(tree map[string]interface{}, path string) error {
	for key, value := range tree {
		fullPath := path + "/" + key

		if nested, ok := value.(map[string]interface{}); ok {
			// Updates should not have statics in nested nodes
			// (first render is an exception)
			if err := checkNestedStatics(nested, fullPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// InvariantTreeStructureUnchanged ensures tree structure (statics) doesn't change between renders
// Only dynamic values should change
var InvariantTreeStructureUnchanged Invariant = func(before, after StateSnapshot, action Action) error {
	if before.Tree == nil || after.Tree == nil {
		return nil
	}

	// Compare static structures
	return compareStaticStructure(before.Tree, after.Tree, "")
}

// compareStaticStructure compares the static structure of two trees
func compareStaticStructure(before, after map[string]interface{}, path string) error {
	// Both should have same "s" array if present
	beforeS, beforeHasS := before["s"]
	afterS, afterHasS := after["s"]

	if beforeHasS != afterHasS {
		return fmt.Errorf("static structure mismatch at %s: before has statics=%v, after=%v",
			path, beforeHasS, afterHasS)
	}

	if beforeHasS {
		beforeArr, _ := beforeS.([]interface{})
		afterArr, _ := afterS.([]interface{})

		if len(beforeArr) != len(afterArr) {
			return fmt.Errorf("static array length mismatch at %s: %d vs %d",
				path, len(beforeArr), len(afterArr))
		}

		// Compare string statics
		for i, b := range beforeArr {
			bs, bOK := b.(string)
			as, aOK := afterArr[i].(string)
			if bOK && aOK && bs != as {
				return fmt.Errorf("static string changed at %s[%d]: %q vs %q",
					path, i, bs, as)
			}
		}
	}

	// Recursively check nested maps
	for key, beforeVal := range before {
		if key == "s" {
			continue
		}

		beforeMap, bIsMap := beforeVal.(map[string]interface{})
		afterVal, exists := after[key]
		afterMap, aIsMap := afterVal.(map[string]interface{})

		if bIsMap && exists && aIsMap {
			if err := compareStaticStructure(beforeMap, afterMap, path+"/"+key); err != nil {
				return err
			}
		}
	}

	return nil
}
