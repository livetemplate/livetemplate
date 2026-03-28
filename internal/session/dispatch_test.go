package session

import (
	"testing"
	"time"
)

func TestEnqueueDispatch_DeliversToChannel(t *testing.T) {
	conn := &Connection{
		GroupID:      "group-1",
		DispatchChan: make(chan *DispatchRequest, 10),
	}

	conn.EnqueueDispatch(&DispatchRequest{
		Action: "RefreshMessages",
		Data:   map[string]interface{}{"key": "value"},
	})

	select {
	case req := <-conn.DispatchChan:
		if req.Action != "RefreshMessages" {
			t.Errorf("expected action RefreshMessages, got %s", req.Action)
		}
		if req.Data["key"] != "value" {
			t.Errorf("expected data key=value, got %v", req.Data["key"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch request")
	}
}

func TestEnqueueDispatch_DropsWhenFull(t *testing.T) {
	conn := &Connection{
		GroupID:      "group-1",
		DispatchChan: make(chan *DispatchRequest, 1),
	}

	// Fill the channel
	conn.EnqueueDispatch(&DispatchRequest{Action: "First"})

	// This should be dropped (non-blocking)
	conn.EnqueueDispatch(&DispatchRequest{Action: "Second"})

	// Only first should be in the channel
	select {
	case req := <-conn.DispatchChan:
		if req.Action != "First" {
			t.Errorf("expected First, got %s", req.Action)
		}
	default:
		t.Fatal("expected a message in channel")
	}

	// Channel should be empty now
	select {
	case req := <-conn.DispatchChan:
		t.Fatalf("expected empty channel, got %s", req.Action)
	default:
		// expected
	}
}

func TestEnqueueDispatch_NilChannelIsNoOp(t *testing.T) {
	conn := &Connection{
		GroupID: "group-1",
		// DispatchChan is nil (not registered)
	}

	// Should not panic
	conn.EnqueueDispatch(&DispatchRequest{Action: "Test"})
}

func TestRegister_InitializesDispatchChan(t *testing.T) {
	registry := NewConnectionRegistry()
	conn := &Connection{
		GroupID: "group-1",
		UserID:  "user-1",
	}

	registry.Register(conn, 10)
	defer registry.Unregister(conn)

	if conn.DispatchChan == nil {
		t.Fatal("DispatchChan should be initialized after Register")
	}

	if cap(conn.DispatchChan) != 10 {
		t.Errorf("DispatchChan capacity should be 10, got %d", cap(conn.DispatchChan))
	}
}
