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

// mockMetrics is a MetricsRecorder that counts dispatch drops for assertion.
type mockMetrics struct {
	dispatchDropped int
}

func (m *mockMetrics) WSBufferFull()           {}
func (m *mockMetrics) WSSlowClientClose()      {}
func (m *mockMetrics) WSWriteError()           {}
func (m *mockMetrics) WSAddBufferSize(_ int64) {}
func (m *mockMetrics) WSDispatchDropped()      { m.dispatchDropped++ }

func TestEnqueueDispatch_DropIncrementsMetric(t *testing.T) {
	metrics := &mockMetrics{}
	conn := &Connection{
		GroupID:      "group-1",
		DispatchChan: make(chan *DispatchRequest, 1),
		metrics:      metrics,
	}

	// First enqueue fills the buffer; second is dropped and must increment the metric.
	conn.EnqueueDispatch(&DispatchRequest{Action: "First"})
	conn.EnqueueDispatch(&DispatchRequest{Action: "Second"})

	if metrics.dispatchDropped != 1 {
		t.Errorf("expected WSDispatchDropped to be called once, got %d", metrics.dispatchDropped)
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

	if cap(conn.DispatchChan) != defaultDispatchBufferSize {
		t.Errorf("DispatchChan capacity should be %d (default), got %d", defaultDispatchBufferSize, cap(conn.DispatchChan))
	}
}
