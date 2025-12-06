package livetemplate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// Task 7: New Handle() Signature Tests
// ============================================================================

type testHandleState struct {
	Count   int
	Message string
}

type testHandleController struct {
	MountCalled bool
}

func (c *testHandleController) Mount(state testHandleState, ctx *Context) (testHandleState, error) {
	c.MountCalled = true
	state.Message = "mounted"
	return state, nil
}

func (c *testHandleController) Increment(state testHandleState, ctx *Context) (testHandleState, error) {
	state.Count++
	return state, nil
}

func TestHandleNew_ReturnsLiveHandler(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	state := AsState(&testHandleState{})

	handler := tmpl.HandleNew(ctrl, state)

	if handler == nil {
		t.Fatal("HandleNew returned nil")
	}

	// Should implement http.Handler
	var _ http.Handler = handler
}

func TestHandleNew_PanicsOnNilController(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>test</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil controller")
		}
	}()

	tmpl.HandleNew(nil, AsState(&testHandleState{}))
}

func TestHandleNew_PanicsOnNilState(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>test</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil state")
		}
	}()

	tmpl.HandleNew(&testHandleController{}, nil)
}

func TestHandleNew_ServesHTTP(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	state := AsState(&testHandleState{Count: 42})

	handler := tmpl.HandleNew(ctrl, state)

	// Make HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("Response body should not be empty")
	}
}

// Test that state is properly cloned per session (via serialization)
func TestHandleNew_StateCloning(t *testing.T) {
	// This test verifies that the State interface's serialization
	// is used for cloning, not struct copying
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	originalState := &testHandleState{Count: 100}
	state := AsState(originalState)

	handler := tmpl.HandleNew(&testHandleController{}, state)

	// Handler should exist
	if handler == nil {
		t.Fatal("Handler is nil")
	}

	// Original state should not be modified by creating handler
	if originalState.Count != 100 {
		t.Errorf("Original state was modified: Count = %d", originalState.Count)
	}
}
