package seqtest

import (
	"encoding/json"
	"time"
)

// StateSnapshot captures the state at a point in time
type StateSnapshot struct {
	State     interface{}            // The state value
	Tree      map[string]interface{} // Template tree (if applicable)
	Timestamp time.Time              // When snapshot was taken
}

// Clone creates a deep copy of the snapshot's state via JSON roundtrip
func (s *StateSnapshot) Clone() (interface{}, error) {
	data, err := json.Marshal(s.State)
	if err != nil {
		return nil, err
	}

	// Create new instance of same type
	clone := cloneInterface(s.State)
	if err := json.Unmarshal(data, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// StateHistory tracks all state transitions during a sequence
type StateHistory struct {
	Initial   StateSnapshot
	Snapshots []StateSnapshot
	Actions   []Action
}

// NewStateHistory creates a new history starting with the given initial state
func NewStateHistory(initialState interface{}) *StateHistory {
	return &StateHistory{
		Initial: StateSnapshot{
			State:     initialState,
			Timestamp: time.Now(),
		},
		Snapshots: make([]StateSnapshot, 0),
		Actions:   make([]Action, 0),
	}
}

// Record adds a new state transition to the history
func (h *StateHistory) Record(action Action, newState interface{}, tree map[string]interface{}) {
	h.Actions = append(h.Actions, action)
	h.Snapshots = append(h.Snapshots, StateSnapshot{
		State:     newState,
		Tree:      tree,
		Timestamp: time.Now(),
	})
}

// Len returns the number of recorded transitions
func (h *StateHistory) Len() int {
	return len(h.Actions)
}

// At returns the state snapshot after the i-th action (0-indexed)
func (h *StateHistory) At(i int) *StateSnapshot {
	if i < 0 || i >= len(h.Snapshots) {
		return nil
	}
	return &h.Snapshots[i]
}

// Before returns the state before the i-th action
func (h *StateHistory) Before(i int) *StateSnapshot {
	if i <= 0 {
		return &h.Initial
	}
	if i > len(h.Snapshots) {
		return nil
	}
	return &h.Snapshots[i-1]
}

// Final returns the final state snapshot
func (h *StateHistory) Final() *StateSnapshot {
	if len(h.Snapshots) == 0 {
		return &h.Initial
	}
	return &h.Snapshots[len(h.Snapshots)-1]
}

// cloneInterface creates a new pointer to the same type as v
func cloneInterface(v interface{}) interface{} {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	// Create a map to unmarshal into (generic approach)
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}
