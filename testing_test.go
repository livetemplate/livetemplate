package livetemplate

import (
	"database/sql"
	"html/template"
	"log/slog"
	"strings"
	"testing"

	lvtcontext "github.com/livetemplate/livetemplate/internal/context"
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

// StateWithMethods has computed properties via methods — still pure state
type StateWithMethods struct {
	Items  []string
	Filter string
}

func (s StateWithMethods) ActiveCount() int {
	count := 0
	for _, item := range s.Items {
		if item != "" {
			count++
		}
	}
	return count
}

func (s StateWithMethods) FilteredItems() []string {
	if s.Filter == "" {
		return s.Items
	}
	var result []string
	for _, item := range s.Items {
		if strings.Contains(item, s.Filter) {
			result = append(result, item)
		}
	}
	return result
}

func TestAsState_PanicsOnImpureState(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for impure state")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "livetemplate.AsState") {
			t.Errorf("Panic message should mention AsState, got: %s", msg)
		}
		if !strings.Contains(msg, "Logger") {
			t.Errorf("Panic message should mention the offending field, got: %s", msg)
		}
	}()
	AsState(&ImpureState{})
}

func TestAsState_PanicsOnNestedImpureState(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for nested impure state")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Data.DB") {
			t.Errorf("Panic message should mention nested field path, got: %s", msg)
		}
	}()
	AsState(&NestedImpureState{})
}

type PointerNestedImpureState struct {
	Cfg *struct {
		DB *sql.DB
	}
}

func TestAsState_PanicsOnPointerToNestedImpureState(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for pointer-to-struct with dependency")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Cfg.DB") {
			t.Errorf("Panic message should mention nested field path through pointer, got: %s", msg)
		}
	}()
	AsState(&PointerNestedImpureState{})
}

type SliceImpureState struct {
	Loggers []*slog.Logger
}

func TestAsState_PanicsOnSliceOfDependency(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for slice of dependency type")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Loggers") {
			t.Errorf("Panic message should mention the field, got: %s", msg)
		}
	}()
	AsState(&SliceImpureState{})
}

type MapImpureState struct {
	Connections map[string]*sql.DB
}

func TestAsState_PanicsOnMapOfDependency(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for map of dependency type")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Connections") {
			t.Errorf("Panic message should mention the field, got: %s", msg)
		}
	}()
	AsState(&MapImpureState{})
}

type SelfRefState struct {
	Next  *SelfRefState
	Value string
}

func TestAsState_SelfReferentialStateDoesNotStackOverflow(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Self-referential pure state should not panic, got: %v", r)
		}
	}()
	s := AsState(&SelfRefState{Value: "root"})
	if s == nil {
		t.Fatal("Expected non-nil State")
	}
}

func TestAsState_PureStateDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Pure state should not panic, got: %v", r)
		}
	}()
	s := AsState(&PureState{Count: 1, Name: "test"})
	if s == nil {
		t.Fatal("Expected non-nil State")
	}
}

func TestAssertPureState_WithMethods(t *testing.T) {
	// State structs with methods should pass — methods are not dependencies
	AssertPureState[StateWithMethods](t)
}

func TestStateMethodsInTemplates(t *testing.T) {
	// End-to-end: AssertPureState passes AND methods work in templates via BuildDataMap
	AssertPureState[StateWithMethods](t)

	state := StateWithMethods{
		Items:  []string{"alpha", "beta", ""},
		Filter: "a",
	}
	dataMap := lvtcontext.BuildDataMap(state, nil, false, nil, nil, false)

	tmpl, err := template.New("test").Parse(
		`Count={{.ActiveCount}} Filtered={{len .FilteredItems}} Items={{len .Items}}`)
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, dataMap); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	got := buf.String()
	want := "Count=2 Filtered=2 Items=3"
	if got != want {
		t.Errorf("Template output = %q, want %q", got, want)
	}
}
