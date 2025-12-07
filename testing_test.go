package livetemplate

import (
	"database/sql"
	"log/slog"
	"testing"
)

// PureState contains only serializable data - should pass
type PureState struct {
	Count   int
	Name    string
	Items   []string
	Mapping map[string]int
}

// ImpureState contains a dependency - should fail
type ImpureState struct {
	Count  int
	Logger *slog.Logger
}

// NestedImpureState has a dependency in a nested struct
type NestedImpureState struct {
	Data struct {
		Count int
		DB    *sql.DB
	}
}

func TestAssertPureState_PureState(t *testing.T) {
	// This should pass without error
	AssertPureState[PureState](t)
}

func TestAssertPureState_DetectsSlogLogger(t *testing.T) {
	err := validatePureState[ImpureState]()
	if err == nil {
		t.Error("Expected error for state with *slog.Logger")
	}
	if err != nil && err.Error() != "field Logger appears to be a dependency (*slog.Logger) - move to controller" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestAssertPureState_DetectsNestedDependency(t *testing.T) {
	err := validatePureState[NestedImpureState]()
	if err == nil {
		t.Error("Expected error for state with nested *sql.DB")
	}
}

func TestAssertPureState_PointerToPureState(t *testing.T) {
	// Pointers to pure state should also pass
	AssertPureState[*PureState](t)
}
