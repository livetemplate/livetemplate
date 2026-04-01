package livetemplate

import (
	"encoding/json"
	"testing"
)

// ============================================================================
// Controller+State Pattern Tests
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
// Selective Persistence (lvt:"persist") Tests
// ============================================================================

type testPersistState struct {
	Filter string   `json:"filter" lvt:"persist"`
	Page   int      `json:"page" lvt:"persist"`
	Items  []string `json:"items"`
	Count  int      `json:"count"`
}

func TestPersistFields_Detection(t *testing.T) {
	state := AsState(&testPersistState{})
	js := state.(*jsonState[testPersistState])

	if !js.HasPersistFields() {
		t.Fatal("Expected HasPersistFields() = true")
	}
	if len(js.persistFields) != 2 {
		t.Fatalf("Expected 2 persist fields, got %d", len(js.persistFields))
	}
	if js.persistFields[0].jsonName != "filter" {
		t.Errorf("First persist field jsonName: got %q, want %q", js.persistFields[0].jsonName, "filter")
	}
	if js.persistFields[1].jsonName != "page" {
		t.Errorf("Second persist field jsonName: got %q, want %q", js.persistFields[1].jsonName, "page")
	}
}

func TestPersistFields_NoPersistTags(t *testing.T) {
	type EphemeralState struct {
		Items []string `json:"items"`
		Count int      `json:"count"`
	}
	state := AsState(&EphemeralState{})
	js := state.(*jsonState[EphemeralState])

	if js.HasPersistFields() {
		t.Fatal("Expected HasPersistFields() = false for state without persist tags")
	}
}

func TestPersistFields_ExtractAndInject(t *testing.T) {
	state := AsState(&testPersistState{})
	js := state.(*jsonState[testPersistState])

	// Create a state with all fields populated
	original := testPersistState{
		Filter: "active",
		Page:   3,
		Items:  []string{"todo1", "todo2"},
		Count:  2,
	}

	// Extract persist fields — should only include Filter and Page
	data, err := js.ExtractPersistFields(original)
	if err != nil {
		t.Fatalf("ExtractPersistFields failed: %v", err)
	}

	// Verify JSON only contains persist fields
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to parse extracted JSON: %v", err)
	}
	if _, ok := m["filter"]; !ok {
		t.Error("Expected 'filter' in extracted data")
	}
	if _, ok := m["page"]; !ok {
		t.Error("Expected 'page' in extracted data")
	}
	if _, ok := m["items"]; ok {
		t.Error("'items' should NOT be in extracted data (not tagged persist)")
	}
	if _, ok := m["count"]; ok {
		t.Error("'count' should NOT be in extracted data (not tagged persist)")
	}

	// Inject persist fields into a zero-value state
	restored, err := js.InjectPersistFields(data)
	if err != nil {
		t.Fatalf("InjectPersistFields failed: %v", err)
	}

	restoredState := restored.(testPersistState)

	// Persist fields should be restored
	if restoredState.Filter != "active" {
		t.Errorf("Filter: got %q, want %q", restoredState.Filter, "active")
	}
	if restoredState.Page != 3 {
		t.Errorf("Page: got %d, want %d", restoredState.Page, 3)
	}

	// Non-persist fields should be zero
	if restoredState.Items != nil {
		t.Errorf("Items should be nil (zero value), got %v", restoredState.Items)
	}
	if restoredState.Count != 0 {
		t.Errorf("Count should be 0 (zero value), got %d", restoredState.Count)
	}
}

func TestPersistFields_ExtractEmpty(t *testing.T) {
	type EphemeralState struct {
		Value int `json:"value"`
	}
	state := AsState(&EphemeralState{})
	js := state.(*jsonState[EphemeralState])

	data, err := js.ExtractPersistFields(EphemeralState{Value: 42})
	if err != nil {
		t.Fatalf("ExtractPersistFields failed: %v", err)
	}
	if data != nil {
		t.Errorf("Expected nil data for state without persist tags, got %s", string(data))
	}
}

func TestPersistFields_InjectEmpty(t *testing.T) {
	type EphemeralState struct {
		Value int `json:"value"`
	}
	state := AsState(&EphemeralState{})
	js := state.(*jsonState[EphemeralState])

	restored, err := js.InjectPersistFields(nil)
	if err != nil {
		t.Fatalf("InjectPersistFields failed: %v", err)
	}
	restoredState := restored.(EphemeralState)
	if restoredState.Value != 0 {
		t.Errorf("Expected zero value, got %d", restoredState.Value)
	}
}

func TestPersistFields_MarshalBinaryStillFull(t *testing.T) {
	original := &testPersistState{
		Filter: "active",
		Page:   3,
		Items:  []string{"todo1", "todo2"},
		Count:  2,
	}
	state := AsState(original)

	// MarshalBinary should still serialize ALL fields (used for cloning)
	data, err := state.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to parse MarshalBinary output: %v", err)
	}

	// All fields should be present in full marshal
	for _, key := range []string{"filter", "page", "items", "count"} {
		if _, ok := m[key]; !ok {
			t.Errorf("Expected '%s' in MarshalBinary output", key)
		}
	}
}

func TestPersistFields_JSONTagWithOptions(t *testing.T) {
	type StateWithOptions struct {
		Filter string `json:"filter,omitempty" lvt:"persist"`
		NoJSON string `json:"-" lvt:"persist"`
		Plain  string `lvt:"persist"`
	}

	state := AsState(&StateWithOptions{})
	js := state.(*jsonState[StateWithOptions])

	// json:"-" fields are skipped (can't round-trip), so only 2 persist fields
	if len(js.persistFields) != 2 {
		t.Fatalf("Expected 2 persist fields (json:\"-\" skipped), got %d", len(js.persistFields))
	}

	// "filter,omitempty" should extract "filter" as json name
	if js.persistFields[0].jsonName != "filter" {
		t.Errorf("First field jsonName: got %q, want %q", js.persistFields[0].jsonName, "filter")
	}

	// No json tag should use field name
	if js.persistFields[1].jsonName != "Plain" {
		t.Errorf("Second field jsonName: got %q, want %q", js.persistFields[1].jsonName, "Plain")
	}
}

func TestPersistableStateInterface(t *testing.T) {
	// Verify jsonState implements persistableState
	state := AsState(&testPersistState{})
	ps, ok := state.(persistableState)
	if !ok {
		t.Fatal("jsonState should implement persistableState")
	}
	if !ps.HasPersistFields() {
		t.Fatal("Expected HasPersistFields() = true")
	}
}

func TestValidatePersistTag(t *testing.T) {
	type TypoState struct {
		Value int `json:"value" lvt:"Persist"` // case typo
	}

	// Should not panic but should log warning (we just verify it doesn't crash)
	state := AsState(&TypoState{})
	js := state.(*jsonState[TypoState])

	// Typo "Persist" != "persist", so no persist fields detected
	if js.HasPersistFields() {
		t.Error("Case-sensitive: 'Persist' should not match 'persist'")
	}
}
