package livetemplate

import (
	"context"
	"errors"
	"testing"
)

// ============================================================================
// Task 4: Mount Lifecycle Tests
// ============================================================================

type testTodoState struct {
	Items  []string
	Loaded bool
}

type testTodoController struct {
	MountCalled bool
	MountError  error
}

func (c *testTodoController) Mount(state testTodoState, ctx *Context) (testTodoState, error) {
	c.MountCalled = true
	if c.MountError != nil {
		return state, c.MountError
	}
	state.Items = []string{"item1", "item2"}
	state.Loaded = true
	return state, nil
}

func TestCallMount_Success(t *testing.T) {
	ctrl := &testTodoController{}
	initialState := testTodoState{}
	ctx := NewContext(context.Background(), "", nil)

	newState, err := callMount(ctrl, initialState, ctx)
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	if !ctrl.MountCalled {
		t.Error("Mount was not called")
	}

	result := newState.(testTodoState)
	if !result.Loaded {
		t.Error("State.Loaded should be true")
	}
	if len(result.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(result.Items))
	}
}

func TestCallMount_Error(t *testing.T) {
	expectedErr := errors.New("mount failed")
	ctrl := &testTodoController{MountError: expectedErr}
	initialState := testTodoState{}
	ctx := NewContext(context.Background(), "", nil)

	_, err := callMount(ctrl, initialState, ctx)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// Controller without Mount method
type testNoMountController struct{}

func TestCallMount_NoMethod(t *testing.T) {
	ctrl := &testNoMountController{}
	state := testTodoState{Loaded: false}
	ctx := NewContext(context.Background(), "", nil)

	newState, err := callMount(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("callMount should not error when no Mount method: %v", err)
	}

	// State should be unchanged
	result := newState.(testTodoState)
	if result.Loaded {
		t.Error("State should be unchanged when no Mount method")
	}
}

// Controller with wrong Mount signature
type testWrongMountController struct{}

func (c *testWrongMountController) Mount() error {
	return nil
}

func TestCallMount_WrongSignature(t *testing.T) {
	ctrl := &testWrongMountController{}
	state := testTodoState{}
	ctx := NewContext(context.Background(), "", nil)

	newState, err := callMount(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("callMount should not error for wrong signature: %v", err)
	}

	// State should be unchanged (Mount with wrong sig not called)
	result := newState.(testTodoState)
	if result.Loaded {
		t.Error("State should be unchanged when Mount has wrong signature")
	}
}

// ============================================================================
// Task 5: OnConnect Lifecycle Tests
// ============================================================================

type testChatState struct {
	Connected    bool
	ConnectionID string
}

type testChatController struct {
	OnConnectCalled bool
	OnConnectError  error
}

func (c *testChatController) OnConnect(state testChatState, ctx *Context) (testChatState, error) {
	c.OnConnectCalled = true
	if c.OnConnectError != nil {
		return state, c.OnConnectError
	}
	state.Connected = true
	state.ConnectionID = "conn-123"
	return state, nil
}

func TestCallOnConnect_Success(t *testing.T) {
	ctrl := &testChatController{}
	state := testChatState{}
	ctx := NewContext(context.Background(), "", nil)

	newState, err := callOnConnect(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	if !ctrl.OnConnectCalled {
		t.Error("OnConnect was not called")
	}

	result := newState.(testChatState)
	if !result.Connected {
		t.Error("State.Connected should be true")
	}
}

func TestCallOnConnect_NoMethod(t *testing.T) {
	ctrl := &testNoMountController{} // Reuse - has no OnConnect
	state := testChatState{}
	ctx := NewContext(context.Background(), "", nil)

	newState, err := callOnConnect(ctrl, state, ctx)
	if err != nil {
		t.Fatalf("callOnConnect should not error when no method: %v", err)
	}

	result := newState.(testChatState)
	if result.Connected {
		t.Error("State should be unchanged")
	}
}

// ============================================================================
// Task 6: OnDisconnect Lifecycle Tests
// ============================================================================

type testStreamController struct {
	DisconnectCalled bool
}

func (c *testStreamController) OnDisconnect() {
	c.DisconnectCalled = true
}

func TestCallOnDisconnect_Success(t *testing.T) {
	ctrl := &testStreamController{}

	callOnDisconnect(ctrl)

	if !ctrl.DisconnectCalled {
		t.Error("OnDisconnect was not called")
	}
}

func TestCallOnDisconnect_NoMethod(t *testing.T) {
	ctrl := &testNoMountController{} // Has no OnDisconnect

	// Should not panic
	callOnDisconnect(ctrl)
}

// Controller with wrong OnDisconnect signature
type testWrongDisconnectController struct {
	Called bool
}

func (c *testWrongDisconnectController) OnDisconnect(state int) int {
	c.Called = true
	return state
}

func TestCallOnDisconnect_WrongSignature(t *testing.T) {
	ctrl := &testWrongDisconnectController{}

	// Should not panic, should not call method
	callOnDisconnect(ctrl)

	if ctrl.Called {
		t.Error("OnDisconnect with wrong signature should not be called")
	}
}
