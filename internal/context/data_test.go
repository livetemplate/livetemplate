package context

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"testing"
)

// Test types for method preservation

type CounterState struct {
	Value int
}

func (s CounterState) Doubled() int {
	return s.Value * 2
}

func (s CounterState) Display() string {
	return "counter"
}

type TodoItem struct {
	Title string
	Done  bool
}

type TodoState struct {
	Items  []TodoItem
	Filter string
}

func (s TodoState) ActiveCount() int {
	count := 0
	for _, item := range s.Items {
		if !item.Done {
			count++
		}
	}
	return count
}

func (s TodoState) FilteredItems() []TodoItem {
	if s.Filter == "" || s.Filter == "all" {
		return s.Items
	}
	var result []TodoItem
	for _, item := range s.Items {
		if s.Filter == "active" && !item.Done {
			result = append(result, item)
		} else if s.Filter == "done" && item.Done {
			result = append(result, item)
		}
	}
	return result
}

type StateWithPointerMethod struct {
	Name string
}

func (s *StateWithPointerMethod) Greeting() string {
	return "Hello, " + s.Name
}

func TestBuildDataMap_AllowSetScopesMethods(t *testing.T) {
	state := CounterState{Value: 5}

	// Only Doubled is referenced: Display must be skipped, the field stays.
	dm := BuildDataMap(state, nil, false, nil, map[string]struct{}{"Doubled": {}}, false).(map[string]interface{})
	if got, ok := dm["Doubled"].(int); !ok || got != 10 {
		t.Errorf("Doubled should be precomputed, got %v", dm["Doubled"])
	}
	if _, exists := dm["Display"]; exists {
		t.Error("Display is not in the allow-set and must be skipped")
	}
	if v, ok := dm["Value"]; !ok || v != 5 {
		t.Errorf("field Value must always be present, got %v", dm["Value"])
	}
}

func TestBuildDataMap_EmptyAllowSetSkipsAllMethods(t *testing.T) {
	state := CounterState{Value: 5}
	dm := BuildDataMap(state, nil, false, nil, map[string]struct{}{}, false).(map[string]interface{})
	if _, exists := dm["Doubled"]; exists {
		t.Error("empty allow-set must skip all methods (Doubled)")
	}
	if _, exists := dm["Display"]; exists {
		t.Error("empty allow-set must skip all methods (Display)")
	}
	if v, ok := dm["Value"]; !ok || v != 5 {
		t.Errorf("fields are unaffected by the allow-set, got Value=%v", dm["Value"])
	}
}

func TestBuildDataMap_NilAllowSetPrecomputesAll(t *testing.T) {
	state := CounterState{Value: 5}
	dm := BuildDataMap(state, nil, false, nil, nil, false).(map[string]interface{})
	if _, ok := dm["Doubled"]; !ok {
		t.Error("nil allow-set must precompute all methods (Doubled missing)")
	}
	if _, ok := dm["Display"]; !ok {
		t.Error("nil allow-set must precompute all methods (Display missing)")
	}
}

func TestBuildDataMap_StructMethodsPreserved(t *testing.T) {
	state := CounterState{Value: 5}
	result := BuildDataMap(state, nil, false, nil, nil, false)

	dataMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result)
	}

	// Field should be present
	if v, ok := dataMap["Value"]; !ok || v != 5 {
		t.Errorf("Expected Value=5, got %v", dataMap["Value"])
	}

	// Methods should be present as precomputed return values
	if got, ok := dataMap["Doubled"].(int); !ok || got != 10 {
		t.Errorf("Doubled should be precomputed int 10, got %v (%T)", dataMap["Doubled"], dataMap["Doubled"])
	}
	if got, ok := dataMap["Display"].(string); !ok || got != "counter" {
		t.Errorf("Display should be precomputed string, got %v (%T)", dataMap["Display"], dataMap["Display"])
	}
}

func TestBuildDataMap_StructMethodsWorkInTemplates(t *testing.T) {
	state := CounterState{Value: 5}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)

	tmpl, err := template.New("test").Parse(`Value={{.Value}} Doubled={{.Doubled}}`)
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, dataMap); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	got := buf.String()
	if got != "Value=5 Doubled=10" {
		t.Errorf("Template output = %q, want %q", got, "Value=5 Doubled=10")
	}
}

func TestBuildDataMap_TodoMethodsInTemplates(t *testing.T) {
	state := TodoState{
		Items: []TodoItem{
			{Title: "Task 1", Done: false},
			{Title: "Task 2", Done: true},
			{Title: "Task 3", Done: false},
		},
		Filter: "active",
	}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)

	tmpl, err := template.New("test").Parse(`Active={{.ActiveCount}} Items={{len .FilteredItems}}`)
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, dataMap); err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	got := buf.String()
	if got != "Active=2 Items=2" {
		t.Errorf("Template output = %q, want %q", got, "Active=2 Items=2")
	}
}

func TestBuildDataMap_PointerMethodsPreserved(t *testing.T) {
	state := &StateWithPointerMethod{Name: "World"}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)

	dm, ok := dataMap.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", dataMap)
	}

	if _, ok := dm["Greeting"]; !ok {
		t.Fatal("Pointer receiver method Greeting not found in data map")
	}

	if got, ok := dm["Greeting"].(string); !ok || got != "Hello, World" {
		t.Errorf("Greeting should be precomputed string %q, got %v (%T)", "Hello, World", dm["Greeting"], dm["Greeting"])
	}
}

type JSONTagCollision struct {
	Computed string `json:"Doubled"`
}

func (s JSONTagCollision) Doubled() string {
	return "from-method"
}

func TestBuildDataMap_FieldTakesPrecedenceOverMethod(t *testing.T) {
	// A JSON tag can alias a field to a name that matches a method.
	// The field value (via JSON tag) should win.
	state := JSONTagCollision{Computed: "from-field"}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)

	dm := dataMap.(map[string]interface{})
	if v, ok := dm["Doubled"].(string); !ok || v != "from-field" {
		t.Errorf("JSON-tagged field should take precedence over method, got %v", dm["Doubled"])
	}
}

type FallibleState struct {
	OK bool
}

func (s FallibleState) Name() (string, error) {
	if !s.OK {
		return "", fmt.Errorf("not ready")
	}
	return "ready", nil
}

func TestBuildDataMap_ErrorMethodOmittedWhenNonNil(t *testing.T) {
	// When a method returns (T, error) and the error is non-nil,
	// the method is silently omitted from the map.
	state := FallibleState{OK: false}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)
	dm := dataMap.(map[string]interface{})

	if _, exists := dm["Name"]; exists {
		t.Error("Method returning non-nil error should be omitted from map")
	}
}

func TestBuildDataMap_ErrorMethodIncludedWhenNil(t *testing.T) {
	state := FallibleState{OK: true}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)
	dm := dataMap.(map[string]interface{})

	if v, ok := dm["Name"].(string); !ok || v != "ready" {
		t.Errorf("Method returning nil error should be included, got %v", dm["Name"])
	}
}

type PanickingState struct {
	Safe string
}

func (s PanickingState) Boom() string {
	panic("kaboom")
}

func (s PanickingState) OK() string {
	return "fine"
}

func TestBuildDataMap_PanickingMethodSkipped(t *testing.T) {
	state := PanickingState{Safe: "value"}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)
	dm := dataMap.(map[string]interface{})

	if _, exists := dm["Boom"]; exists {
		t.Error("Panicking method should be omitted from map")
	}
	if v, ok := dm["OK"].(string); !ok || v != "fine" {
		t.Errorf("Non-panicking method should be present, got %v", dm["OK"])
	}
	if v, ok := dm["Safe"].(string); !ok || v != "value" {
		t.Errorf("Field should be present, got %v", dm["Safe"])
	}
}

func TestBuildDataMap_LvtContextAlwaysPresent(t *testing.T) {
	state := CounterState{Value: 1}
	dataMap := BuildDataMap(state, nil, true, nil, nil, false)

	dm := dataMap.(map[string]interface{})
	lvt, ok := dm[TemplateContextKey]
	if !ok {
		t.Fatal("lvt context should be present")
	}
	if _, ok := lvt.(*TemplateContext); !ok {
		t.Errorf("lvt should be *TemplateContext, got %T", lvt)
	}
}

func TestBuildDataMap_MethodsWithPointerReceiverOnValueInput(t *testing.T) {
	// Pass by value — pointer receiver methods should still be captured
	// because we create a pointer via reflect.New
	state := StateWithPointerMethod{Name: "Value"}
	dataMap := BuildDataMap(state, nil, false, nil, nil, false)

	dm := dataMap.(map[string]interface{})
	if _, ok := dm["Greeting"]; !ok {
		t.Fatal("Pointer receiver method should be captured even when passed by value")
	}
}

// MixedReceiverState has both a value-receiver and a pointer-receiver zero-arg
// method, to exercise the receiver-kind bookkeeping in getMethodMeta.
type MixedReceiverState struct {
	Name string
}

func (s MixedReceiverState) ValueGreeting() string { return "value " + s.Name }

func (s *MixedReceiverState) PtrGreeting() string { return "ptr " + s.Name }

// TestGetMethodMeta_ValueIndexReceiverKinds locks the mechanism behind the
// reflect.New optimization: value-receiver methods carry a real ValueIndex,
// pointer-receiver methods carry -1.
func TestGetMethodMeta_ValueIndexReceiverKinds(t *testing.T) {
	meta := getMethodMeta(reflect.PointerTo(reflect.TypeOf(MixedReceiverState{})))
	byName := make(map[string]methodMeta, len(meta))
	for _, m := range meta {
		byName[m.Name] = m
	}

	if vm, ok := byName["ValueGreeting"]; !ok || vm.ValueIndex < 0 {
		t.Errorf("ValueGreeting should be value-receiver (ValueIndex >= 0), got %+v (ok=%v)", vm, ok)
	}
	if pm, ok := byName["PtrGreeting"]; !ok || pm.ValueIndex != -1 {
		t.Errorf("PtrGreeting should be pointer-receiver (ValueIndex == -1), got %+v (ok=%v)", pm, ok)
	}
}

// TestBuildDataMap_MixedReceiversOnValueInput is the correctness trap: a value
// input that has at least one pointer-receiver method must allocate the pointer
// and route ALL calls through it, so the value-receiver method still resolves.
func TestBuildDataMap_MixedReceiversOnValueInput(t *testing.T) {
	dm := BuildDataMap(MixedReceiverState{Name: "x"}, nil, false, nil, nil, false).(map[string]interface{})

	if got := dm["ValueGreeting"]; got != "value x" {
		t.Errorf("ValueGreeting: got %v, want %q", got, "value x")
	}
	if got := dm["PtrGreeting"]; got != "ptr x" {
		t.Errorf("PtrGreeting: got %v, want %q", got, "ptr x")
	}
}
