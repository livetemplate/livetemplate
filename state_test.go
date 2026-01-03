package livetemplate

import (
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
)

// testStateProfile is a simple state struct
type testStateProfile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// testStateSettings is another simple state struct
type testStateSettings struct {
	Theme    string `json:"theme"`
	Language string `json:"language"`
}

// testController has state-tagged fields and dependencies
type testController struct {
	Profile  *testStateProfile  `lvt:"state"`
	Settings *testStateSettings `lvt:"state"`
	Logger   *slog.Logger       // Not tagged - should not be persisted
	DBConn   string             // Not tagged - should not be persisted
}

// testNoTagsController has no state tags - should use full serialization
type testNoTagsController struct {
	Counter int
	Items   []string
}

// testBinaryState implements BinaryMarshaler/BinaryUnmarshaler
type testBinaryState struct {
	Value int
}

func (s *testBinaryState) MarshalBinary() ([]byte, error) {
	return json.Marshal(s)
}

func (s *testBinaryState) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, s)
}

// testBinaryController has a custom binary serializable field
type testBinaryController struct {
	State  *testBinaryState `lvt:"state"`
	Logger *slog.Logger
}

func TestHasStateFields(t *testing.T) {
	tests := []struct {
		name     string
		store    interface{}
		expected bool
	}{
		{
			name: "controller with state tags",
			store: &testController{
				Profile:  &testStateProfile{Name: "Test"},
				Settings: &testStateSettings{Theme: "dark"},
			},
			expected: true,
		},
		{
			name: "controller without state tags",
			store: &testNoTagsController{
				Counter: 5,
				Items:   []string{"a", "b"},
			},
			expected: false,
		},
		{
			name: "binary controller with state tags",
			store: &testBinaryController{
				State: &testBinaryState{Value: 42},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasStateFields(tt.store)
			if result != tt.expected {
				t.Errorf("HasStateFields() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractState(t *testing.T) {
	t.Run("extracts tagged fields only", func(t *testing.T) {
		controller := &testController{
			Profile:  &testStateProfile{Name: "John", Email: "john@example.com"},
			Settings: &testStateSettings{Theme: "dark", Language: "en"},
			Logger:   slog.Default(),
			DBConn:   "postgres://localhost/db",
		}

		state := ExtractState(controller)

		if state == nil {
			t.Fatal("ExtractState() returned nil")
		}

		// Should have Profile and Settings
		if len(state) != 2 {
			t.Errorf("Expected 2 fields, got %d", len(state))
		}

		// Check Profile is extracted
		profile, ok := state["Profile"].(*testStateProfile)
		if !ok {
			t.Errorf("Profile not found or wrong type: %T", state["Profile"])
		} else {
			if profile.Name != "John" || profile.Email != "john@example.com" {
				t.Errorf("Profile values incorrect: %+v", profile)
			}
		}

		// Check Settings is extracted
		settings, ok := state["Settings"].(*testStateSettings)
		if !ok {
			t.Errorf("Settings not found or wrong type: %T", state["Settings"])
		} else {
			if settings.Theme != "dark" || settings.Language != "en" {
				t.Errorf("Settings values incorrect: %+v", settings)
			}
		}

		// Logger and DBConn should NOT be extracted
		if _, ok := state["Logger"]; ok {
			t.Error("Logger should not be extracted")
		}
		if _, ok := state["DBConn"]; ok {
			t.Error("DBConn should not be extracted")
		}
	})

	t.Run("returns nil for store without tags", func(t *testing.T) {
		controller := &testNoTagsController{Counter: 5}
		state := ExtractState(controller)
		if state != nil {
			t.Errorf("ExtractState() should return nil for untagged store, got %v", state)
		}
	})
}

func TestInjectState(t *testing.T) {
	t.Run("injects state into tagged fields", func(t *testing.T) {
		controller := &testController{
			Logger: slog.Default(), // Pre-existing dependency
			DBConn: "original",
		}

		state := map[string]interface{}{
			"Profile":  &testStateProfile{Name: "Jane", Email: "jane@example.com"},
			"Settings": &testStateSettings{Theme: "light", Language: "fr"},
		}

		err := InjectState(controller, state)
		if err != nil {
			t.Fatalf("InjectState() error: %v", err)
		}

		// Check Profile was injected
		if controller.Profile == nil || controller.Profile.Name != "Jane" {
			t.Errorf("Profile not injected correctly: %+v", controller.Profile)
		}

		// Check Settings was injected
		if controller.Settings == nil || controller.Settings.Theme != "light" {
			t.Errorf("Settings not injected correctly: %+v", controller.Settings)
		}

		// Dependencies should remain unchanged
		if controller.Logger == nil {
			t.Error("Logger should remain set")
		}
		if controller.DBConn != "original" {
			t.Error("DBConn should remain unchanged")
		}
	})

	t.Run("handles missing fields gracefully", func(t *testing.T) {
		controller := &testController{}

		// Only inject Profile, not Settings
		state := map[string]interface{}{
			"Profile": &testStateProfile{Name: "Only Profile"},
		}

		err := InjectState(controller, state)
		if err != nil {
			t.Fatalf("InjectState() error: %v", err)
		}

		if controller.Profile == nil || controller.Profile.Name != "Only Profile" {
			t.Errorf("Profile not injected: %+v", controller.Profile)
		}

		// Settings should remain nil
		if controller.Settings != nil {
			t.Error("Settings should remain nil")
		}
	})
}

func TestSerializeDeserializeState(t *testing.T) {
	t.Run("roundtrip with JSON serialization", func(t *testing.T) {
		original := &testController{
			Profile:  &testStateProfile{Name: "Test User", Email: "test@example.com"},
			Settings: &testStateSettings{Theme: "dark", Language: "en"},
		}

		// Extract state
		state := ExtractState(original)
		if state == nil {
			t.Fatal("ExtractState() returned nil")
		}

		// Serialize
		data, err := SerializeState(state)
		if err != nil {
			t.Fatalf("SerializeState() error: %v", err)
		}

		if len(data) == 0 {
			t.Fatal("SerializeState() returned empty data")
		}

		// Deserialize
		restored := &testController{}
		restoredState, err := DeserializeState(data, restored)
		if err != nil {
			t.Fatalf("DeserializeState() error: %v", err)
		}

		// Inject into new controller
		err = InjectState(restored, restoredState)
		if err != nil {
			t.Fatalf("InjectState() error: %v", err)
		}

		// Verify
		if restored.Profile == nil {
			t.Fatal("Restored Profile is nil")
		}
		if restored.Profile.Name != "Test User" || restored.Profile.Email != "test@example.com" {
			t.Errorf("Profile mismatch: got %+v", restored.Profile)
		}

		if restored.Settings == nil {
			t.Fatal("Restored Settings is nil")
		}
		if restored.Settings.Theme != "dark" || restored.Settings.Language != "en" {
			t.Errorf("Settings mismatch: got %+v", restored.Settings)
		}
	})

	t.Run("roundtrip with BinaryMarshaler", func(t *testing.T) {
		original := &testBinaryController{
			State: &testBinaryState{Value: 42},
		}

		// Extract state
		state := ExtractState(original)
		if state == nil {
			t.Fatal("ExtractState() returned nil")
		}

		// Serialize
		data, err := SerializeState(state)
		if err != nil {
			t.Fatalf("SerializeState() error: %v", err)
		}

		// Deserialize
		restored := &testBinaryController{}
		restoredState, err := DeserializeState(data, restored)
		if err != nil {
			t.Fatalf("DeserializeState() error: %v", err)
		}

		// Inject
		err = InjectState(restored, restoredState)
		if err != nil {
			t.Fatalf("InjectState() error: %v", err)
		}

		// Verify
		if restored.State == nil {
			t.Fatal("Restored State is nil")
		}
		if restored.State.Value != 42 {
			t.Errorf("State.Value mismatch: got %d, want 42", restored.State.Value)
		}
	})

	t.Run("empty state", func(t *testing.T) {
		data, err := SerializeState(nil)
		if err != nil {
			t.Fatalf("SerializeState(nil) error: %v", err)
		}
		if data != nil {
			t.Error("SerializeState(nil) should return nil")
		}

		result, err := DeserializeState(nil, &testController{})
		if err != nil {
			t.Fatalf("DeserializeState(nil) error: %v", err)
		}
		if result != nil {
			t.Error("DeserializeState(nil) should return nil")
		}
	})
}

func TestStateFieldCache(t *testing.T) {
	// Clear cache before test
	stateFieldCache = sync.Map{}

	controller := &testController{}

	// First call - should populate cache
	fields1 := getStateFieldInfo(controller)

	// Second call - should use cache
	fields2 := getStateFieldInfo(controller)

	// Both should return same results
	if len(fields1) != len(fields2) {
		t.Errorf("Cache inconsistency: %d vs %d fields", len(fields1), len(fields2))
	}

	if len(fields1) != 2 {
		t.Errorf("Expected 2 state fields, got %d", len(fields1))
	}
}

// testTypoController has a typo in state tag (uppercase State)
type testTypoController struct {
	Counter int `lvt:"State"` // Typo: should be "state"
	Valid   int `lvt:"state"` // Correct
}

// testUnknownTagController has an unknown lvt tag value
type testUnknownTagController struct {
	Counter int `lvt:"custom"` // Unknown value
}

func TestValidateStateTag(t *testing.T) {
	t.Run("detects case variation", func(t *testing.T) {
		// Clear cache to ensure fresh validation
		stateFieldCache = sync.Map{}

		controller := &testTypoController{Counter: 10, Valid: 5}
		fields := getStateFieldInfo(controller)

		// Only the correctly tagged field should be extracted
		if len(fields) != 1 {
			t.Errorf("Expected 1 state field (only Valid), got %d", len(fields))
		}

		if len(fields) > 0 && fields[0].Name != "Valid" {
			t.Errorf("Expected field 'Valid', got '%s'", fields[0].Name)
		}
	})

	t.Run("logs unknown tag values", func(t *testing.T) {
		// Clear cache
		stateFieldCache = sync.Map{}

		controller := &testUnknownTagController{Counter: 10}
		fields := getStateFieldInfo(controller)

		// No fields should be extracted (unknown tag value is not "state")
		if len(fields) != 0 {
			t.Errorf("Expected 0 state fields, got %d", len(fields))
		}
	})
}

// ============================================================================
// New Controller+State Pattern Tests (Task 1)
// ============================================================================

func TestAsState_MarshalUnmarshal(t *testing.T) {
	type TodoState struct {
		Items []string
		Count int
	}

	original := &TodoState{Items: []string{"buy milk"}, Count: 1}
	state := AsState(original)

	// Marshal
	data, err := state.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	// Unmarshal into new instance
	restored := &TodoState{}
	restoredState := AsState(restored)
	if err := restoredState.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	// Verify
	if restored.Count != original.Count {
		t.Errorf("Count mismatch: got %d, want %d", restored.Count, original.Count)
	}
	if len(restored.Items) != len(original.Items) {
		t.Errorf("Items length mismatch: got %d, want %d", len(restored.Items), len(original.Items))
	}
	if len(restored.Items) > 0 && restored.Items[0] != original.Items[0] {
		t.Errorf("Items[0] mismatch: got %q, want %q", restored.Items[0], original.Items[0])
	}
}

func TestAsState_Inner(t *testing.T) {
	type MyState struct{ Value int }
	original := &MyState{Value: 42}
	state := AsState(original)

	inner := state.Inner()
	if inner != original {
		t.Error("Inner() should return original pointer")
	}
}

func TestAsState_EmptyState(t *testing.T) {
	type EmptyState struct{}
	empty := &EmptyState{}
	state := AsState(empty)

	data, err := state.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary on empty state failed: %v", err)
	}

	restored := &EmptyState{}
	restoredState := AsState(restored)
	if err := restoredState.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary on empty state failed: %v", err)
	}
}

func TestAsState_ComplexNestedState(t *testing.T) {
	type Item struct {
		ID    int
		Name  string
		Tags  []string
		Props map[string]interface{}
	}

	type ComplexState struct {
		Items    []*Item
		Metadata map[string]string
		Flags    []bool
	}

	original := &ComplexState{
		Items: []*Item{
			{ID: 1, Name: "First", Tags: []string{"a", "b"}, Props: map[string]interface{}{"key": "value"}},
			{ID: 2, Name: "Second", Tags: nil, Props: nil},
		},
		Metadata: map[string]string{"version": "1.0"},
		Flags:    []bool{true, false, true},
	}

	state := AsState(original)

	data, err := state.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	restored := &ComplexState{}
	restoredState := AsState(restored)
	if err := restoredState.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	// Verify nested data
	if len(restored.Items) != 2 {
		t.Fatalf("Items count mismatch: got %d, want 2", len(restored.Items))
	}
	if restored.Items[0].Name != "First" {
		t.Errorf("Items[0].Name mismatch: got %q, want %q", restored.Items[0].Name, "First")
	}
	if restored.Metadata["version"] != "1.0" {
		t.Errorf("Metadata[version] mismatch: got %q", restored.Metadata["version"])
	}
}

// ============================================================================
// ClearTransientFields Tests
// ============================================================================

// testTransientState has a mix of transient and non-transient fields
type testTransientState struct {
	SearchQuery   string  `json:"search_query"`                    // Not transient
	EditingID     string  `json:"editing_id" lvt:"transient"`      // Transient
	EditingItem   *string `json:"editing_item" lvt:"transient"`    // Transient pointer
	Counter       int     `json:"counter"`                         // Not transient
	TransientInt  int     `json:"transient_int" lvt:"transient"`   // Transient int
	TransientList []int   `json:"transient_list" lvt:"transient"`  // Transient slice
}

func TestClearTransientFields_StructPointer(t *testing.T) {
	item := "test-item"
	state := &testTransientState{
		SearchQuery:   "hello",
		EditingID:     "post-123",
		EditingItem:   &item,
		Counter:       42,
		TransientInt:  100,
		TransientList: []int{1, 2, 3},
	}

	result := ClearTransientFields(state)

	// Should return same pointer type
	resultPtr, ok := result.(*testTransientState)
	if !ok {
		t.Fatalf("Expected *testTransientState, got %T", result)
	}

	// Should be the same pointer (modified in place)
	if resultPtr != state {
		t.Error("Expected same pointer to be returned for pointer input")
	}

	// Transient fields should be cleared
	if state.EditingID != "" {
		t.Errorf("EditingID should be cleared, got %q", state.EditingID)
	}
	if state.EditingItem != nil {
		t.Errorf("EditingItem should be nil, got %v", state.EditingItem)
	}
	if state.TransientInt != 0 {
		t.Errorf("TransientInt should be 0, got %d", state.TransientInt)
	}
	if state.TransientList != nil {
		t.Errorf("TransientList should be nil, got %v", state.TransientList)
	}

	// Non-transient fields should be preserved
	if state.SearchQuery != "hello" {
		t.Errorf("SearchQuery should be preserved, got %q", state.SearchQuery)
	}
	if state.Counter != 42 {
		t.Errorf("Counter should be preserved, got %d", state.Counter)
	}
}

func TestClearTransientFields_StructValue(t *testing.T) {
	item := "test-item"
	state := testTransientState{
		SearchQuery:   "hello",
		EditingID:     "post-123",
		EditingItem:   &item,
		Counter:       42,
		TransientInt:  100,
		TransientList: []int{1, 2, 3},
	}

	result := ClearTransientFields(state)

	// Should return value type (not pointer)
	resultVal, ok := result.(testTransientState)
	if !ok {
		t.Fatalf("Expected testTransientState, got %T", result)
	}

	// Transient fields should be cleared in result
	if resultVal.EditingID != "" {
		t.Errorf("EditingID should be cleared, got %q", resultVal.EditingID)
	}
	if resultVal.EditingItem != nil {
		t.Errorf("EditingItem should be nil, got %v", resultVal.EditingItem)
	}

	// Non-transient fields should be preserved
	if resultVal.SearchQuery != "hello" {
		t.Errorf("SearchQuery should be preserved, got %q", resultVal.SearchQuery)
	}
	if resultVal.Counter != 42 {
		t.Errorf("Counter should be preserved, got %d", resultVal.Counter)
	}

	// Original value should be unchanged (Go passes by value)
	if state.EditingID != "post-123" {
		t.Errorf("Original EditingID should be unchanged, got %q", state.EditingID)
	}
}

func TestClearTransientFields_WithStateInterface(t *testing.T) {
	item := "test-item"
	originalState := &testTransientState{
		SearchQuery:   "hello",
		EditingID:     "post-123",
		EditingItem:   &item,
		Counter:       42,
		TransientInt:  100,
		TransientList: []int{1, 2, 3},
	}

	// Wrap in State interface (like jsonState does)
	wrapped := AsState(originalState)

	result := ClearTransientFields(wrapped)

	// Result should be pointer to the struct (unwrapped from State)
	resultPtr, ok := result.(*testTransientState)
	if !ok {
		t.Fatalf("Expected *testTransientState, got %T", result)
	}

	// Transient fields should be cleared
	if resultPtr.EditingID != "" {
		t.Errorf("EditingID should be cleared, got %q", resultPtr.EditingID)
	}

	// Non-transient fields should be preserved
	if resultPtr.SearchQuery != "hello" {
		t.Errorf("SearchQuery should be preserved, got %q", resultPtr.SearchQuery)
	}
}

func TestClearTransientFields_NilPointer(t *testing.T) {
	var state *testTransientState = nil

	result := ClearTransientFields(state)

	// Should return nil pointer without panicking
	// Note: result is interface{} containing (*testTransientState)(nil)
	resultPtr, ok := result.(*testTransientState)
	if !ok {
		t.Fatalf("Expected *testTransientState type, got %T", result)
	}
	if resultPtr != nil {
		t.Errorf("Expected nil pointer, got %v", resultPtr)
	}
}

func TestClearTransientFields_NonStruct(t *testing.T) {
	// Test with non-struct types - should return unchanged
	intVal := 42
	result := ClearTransientFields(intVal)
	if result != intVal {
		t.Errorf("Expected %d, got %v", intVal, result)
	}

	strVal := "hello"
	result = ClearTransientFields(strVal)
	if result != strVal {
		t.Errorf("Expected %q, got %v", strVal, result)
	}
}

func TestClearTransientFields_NoTransientFields(t *testing.T) {
	type noTransientState struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	state := &noTransientState{Name: "test", Count: 10}
	result := ClearTransientFields(state)

	resultPtr, ok := result.(*noTransientState)
	if !ok {
		t.Fatalf("Expected *noTransientState, got %T", result)
	}

	// All fields should be preserved
	if resultPtr.Name != "test" {
		t.Errorf("Name should be preserved, got %q", resultPtr.Name)
	}
	if resultPtr.Count != 10 {
		t.Errorf("Count should be preserved, got %d", resultPtr.Count)
	}
}

func TestClearTransientFields_AllTransientFields(t *testing.T) {
	type allTransientState struct {
		Field1 string `lvt:"transient"`
		Field2 int    `lvt:"transient"`
	}

	state := &allTransientState{Field1: "test", Field2: 42}
	result := ClearTransientFields(state)

	resultPtr, ok := result.(*allTransientState)
	if !ok {
		t.Fatalf("Expected *allTransientState, got %T", result)
	}

	// All fields should be cleared
	if resultPtr.Field1 != "" {
		t.Errorf("Field1 should be cleared, got %q", resultPtr.Field1)
	}
	if resultPtr.Field2 != 0 {
		t.Errorf("Field2 should be cleared, got %d", resultPtr.Field2)
	}
}

func TestClearTransientFields_MapAndSliceTypes(t *testing.T) {
	type complexTransientState struct {
		RegularMap    map[string]int   `json:"regular_map"`
		TransientMap  map[string]int   `json:"transient_map" lvt:"transient"`
		RegularSlice  []string         `json:"regular_slice"`
		TransientSlice []string        `json:"transient_slice" lvt:"transient"`
	}

	state := &complexTransientState{
		RegularMap:     map[string]int{"a": 1},
		TransientMap:   map[string]int{"b": 2},
		RegularSlice:   []string{"x", "y"},
		TransientSlice: []string{"z"},
	}

	result := ClearTransientFields(state)
	resultPtr := result.(*complexTransientState)

	// Transient fields should be nil
	if resultPtr.TransientMap != nil {
		t.Errorf("TransientMap should be nil, got %v", resultPtr.TransientMap)
	}
	if resultPtr.TransientSlice != nil {
		t.Errorf("TransientSlice should be nil, got %v", resultPtr.TransientSlice)
	}

	// Regular fields should be preserved
	if resultPtr.RegularMap["a"] != 1 {
		t.Errorf("RegularMap should be preserved, got %v", resultPtr.RegularMap)
	}
	if len(resultPtr.RegularSlice) != 2 {
		t.Errorf("RegularSlice should be preserved, got %v", resultPtr.RegularSlice)
	}
}
