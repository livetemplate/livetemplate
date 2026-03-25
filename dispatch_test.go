package livetemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Increment", "increment"},
		{"AddItem", "add_item"},
		{"UpdateUserProfile", "update_user_profile"},
		{"HTTPHandler", "h_t_t_p_handler"}, // edge case - consecutive caps
		{"", ""},
		{"a", "a"},
		{"A", "a"},
		{"PasswordConfirmation", "password_confirmation"},
		{"PhoneNumber", "phone_number"},
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"Title", "Title"},
		{"Email", "Email"},
		{"PhoneNumber", "Phone Number"},
		{"PasswordConfirmation", "Password Confirmation"},
		{"URLField", "URL Field"},
		{"A", "A"},
	}

	for _, tt := range tests {
		result := formatFieldName(tt.input)
		if result != tt.expected {
			t.Errorf("formatFieldName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidationToMultiError(t *testing.T) {
	validate := validator.New()

	type TestInput struct {
		Email                string `validate:"required,email"`
		Website              string `validate:"required,url"`
		PasswordConfirmation string `validate:"required,eqfield=Password"`
		Password             string `validate:"required,min=8"`
	}

	input := TestInput{
		Email:                "not-an-email",
		Website:              "not-a-url",
		PasswordConfirmation: "short",
		Password:             "short",
	}

	err := validate.Struct(input)
	if err == nil {
		t.Fatal("expected validation error")
	}

	multiErr := ValidationToMultiError(err)

	// Check that we got field errors
	if len(multiErr) == 0 {
		t.Fatal("expected at least one field error")
	}

	// Verify field names are snake_case
	fieldMap := make(map[string]string)
	for _, fe := range multiErr {
		fieldMap[fe.Field] = fe.Message
	}

	// email field should use snake_case
	if msg, ok := fieldMap["email"]; ok {
		if msg != "Email must be a valid email address" {
			t.Errorf("email message = %q, want %q", msg, "Email must be a valid email address")
		}
	}

	// website field should use snake_case
	if msg, ok := fieldMap["website"]; ok {
		if msg != "Website must be a valid URL" {
			t.Errorf("website message = %q, want %q", msg, "Website must be a valid URL")
		}
	}

	// password_confirmation should use snake_case (multi-word)
	if msg, ok := fieldMap["password_confirmation"]; ok {
		if msg != "Password Confirmation must match Password" {
			t.Errorf("password_confirmation message = %q, want %q", msg, "Password Confirmation must match Password")
		}
	}

	// password should use snake_case and have min message
	if msg, ok := fieldMap["password"]; ok {
		if msg != "Password must be at least 8 characters" {
			t.Errorf("password message = %q, want %q", msg, "Password must be at least 8 characters")
		}
	}
}

func TestMethodNameToActions(t *testing.T) {
	tests := []struct {
		methodName string
		expected   []string
	}{
		{"Increment", []string{"increment", "Increment"}},
		{"AddItem", []string{"addItem", "add_item", "AddItem"}},
	}

	for _, tt := range tests {
		result := methodNameToActions(tt.methodName)
		if len(result) != len(tt.expected) {
			t.Errorf("methodNameToActions(%q) = %v, expected %v", tt.methodName, result, tt.expected)
			continue
		}
		for i, exp := range tt.expected {
			found := false
			for _, r := range result {
				if r == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("methodNameToActions(%q)[%d] missing %q, got %v", tt.methodName, i, exp, result)
			}
		}
	}
}

// ============================================================================
// Controller+State Pattern Tests
// ============================================================================

// Test state for dispatch pattern
type testCounterState struct {
	Count int
}

// Test controller for dispatch pattern
type testCounterController struct {
	IncrementCalled bool
	AddCalled       bool
}

// Method signature: (state, ctx) -> (state, error)
func (c *testCounterController) Increment(state testCounterState, ctx *Context) (testCounterState, error) {
	c.IncrementCalled = true
	state.Count++
	return state, nil
}

func (c *testCounterController) Add(state testCounterState, ctx *Context) (testCounterState, error) {
	c.AddCalled = true
	amount := ctx.GetInt("amount")
	if amount == 0 {
		amount = 1
	}
	state.Count += amount
	return state, nil
}

func (c *testCounterController) FailingAction(state testCounterState, ctx *Context) (testCounterState, error) {
	return state, errors.New("action failed")
}

// Invalid signature - should be ignored
func (c *testCounterController) WrongSig(state testCounterState) (testCounterState, error) {
	return state, nil
}

func TestDispatchWithState_Increment(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 5}
	ctx := NewContext(context.TODO(), "increment", nil)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}

	result, ok := newState.(testCounterState)
	if !ok {
		t.Fatalf("Expected testCounterState, got %T", newState)
	}

	if result.Count != 6 {
		t.Errorf("Count = %d, want 6", result.Count)
	}

	if !ctrl.IncrementCalled {
		t.Error("Increment was not called on controller")
	}
}

func TestDispatchWithState_WithData(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 10}
	data := map[string]interface{}{"amount": float64(5)} // JSON numbers are float64
	ctx := NewContext(context.TODO(), "add", data)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}

	result := newState.(testCounterState)
	if result.Count != 15 {
		t.Errorf("Count = %d, want 15", result.Count)
	}
}

func TestDispatchWithState_MethodNotFound(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "nonexistent", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound, got %v", err)
	}
}

func TestDispatchWithState_EmptyAction(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for empty action, got %v", err)
	}
}

func TestDispatchWithState_NilContext(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}

	_, err := DispatchWithState(ctrl, state, nil)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for nil context, got %v", err)
	}
}

func TestDispatchWithState_MethodReturnsError(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "failingAction", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if err == nil {
		t.Fatal("Expected error from method")
	}
	if err.Error() != "action failed" {
		t.Errorf("Expected 'action failed', got %q", err.Error())
	}
}

func TestDispatchWithState_SnakeCase(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 0}
	ctx := NewContext(context.TODO(), "failing_action", nil)

	_, err := DispatchWithState(ctrl, state, ctx)
	if err == nil || err.Error() != "action failed" {
		t.Errorf("Expected snake_case action to work, got %v", err)
	}
}

func TestDispatchWithState_StateUnchangedOnError(t *testing.T) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 100}
	ctx := NewContext(context.TODO(), "failingAction", nil)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	// State should be returned as-is even on error
	result := newState.(testCounterState)
	if result.Count != 100 {
		t.Errorf("State should be unchanged, Count = %d", result.Count)
	}
}

// ============================================================================
// Progressive Complexity: Default Action & Standard HTML Routing Tests
// ============================================================================

type testFormState struct {
	Title  string
	Filter string
}

type testFormController struct {
	SubmitCalled bool
	DeleteCalled bool
	FilterCalled bool
}

func (c *testFormController) Submit(state testFormState, ctx *Context) (testFormState, error) {
	c.SubmitCalled = true
	state.Title = ctx.GetString("Title")
	return state, nil
}

func (c *testFormController) Delete(state testFormState, ctx *Context) (testFormState, error) {
	c.DeleteCalled = true
	return state, nil
}

func (c *testFormController) Filter(state testFormState, ctx *Context) (testFormState, error) {
	c.FilterCalled = true
	state.Filter = ctx.GetString("filter")
	return state, nil
}

func TestDispatchWithState_DefaultSubmitAction(t *testing.T) {
	ctrl := &testFormController{}
	state := testFormState{}
	data := map[string]interface{}{"Title": "Buy milk"}
	ctx := NewContext(context.TODO(), "submit", data)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}

	result := newState.(testFormState)
	if result.Title != "Buy milk" {
		t.Errorf("Title = %q, want %q", result.Title, "Buy milk")
	}
	if !ctrl.SubmitCalled {
		t.Error("Submit was not called")
	}
}

func TestDispatchWithState_ButtonNameActionRouting(t *testing.T) {
	ctrl := &testFormController{}
	state := testFormState{}
	data := map[string]interface{}{"id": "123"}
	ctx := NewContext(context.TODO(), "delete", data)

	_, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}
	if !ctrl.DeleteCalled {
		t.Error("Delete was not called via button name='action' value='delete'")
	}
}

func TestDispatchWithState_FormNameRouting(t *testing.T) {
	ctrl := &testFormController{}
	state := testFormState{}
	data := map[string]interface{}{"filter": "active"}
	ctx := NewContext(context.TODO(), "filter", data)

	newState, err := DispatchWithState(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("DispatchWithState failed: %v", err)
	}

	result := newState.(testFormState)
	if result.Filter != "active" {
		t.Errorf("Filter = %q, want %q", result.Filter, "active")
	}
	if !ctrl.FilterCalled {
		t.Error("Filter was not called via form name routing")
	}
}

func TestDispatchWithState_NoSubmitMethodReturnsError(t *testing.T) {
	ctrl := &testCounterController{} // has no Submit() method
	state := testCounterState{}
	ctx := NewContext(context.TODO(), "submit", map[string]interface{}{"Title": "test"})

	_, err := DispatchWithState(ctrl, state, ctx)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("Expected ErrMethodNotFound for controller without Submit(), got %v", err)
	}
}

func BenchmarkDispatchWithState_Cached(b *testing.B) {
	ctrl := &testCounterController{}
	state := testCounterState{Count: 0}
	ctx := NewContext(context.TODO(), "increment", nil)

	// Warm up cache
	_, _ = DispatchWithState(ctrl, state, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DispatchWithState(ctrl, state, ctx)
	}
}
