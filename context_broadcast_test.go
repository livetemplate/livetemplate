package livetemplate

import (
	"context"
	"testing"
)

func TestBroadcastAction_AccumulatesRequests(t *testing.T) {
	ctx := NewContext(context.Background(), "TestAction", nil)

	ctx.BroadcastAction("RefreshMessages", nil)
	ctx.BroadcastAction("UpdateUsers", map[string]interface{}{"count": 5})

	broadcasts := ctx.pendingBroadcasts()
	if len(broadcasts) != 2 {
		t.Fatalf("expected 2 broadcasts, got %d", len(broadcasts))
	}

	if broadcasts[0].Action != "RefreshMessages" {
		t.Errorf("expected action RefreshMessages, got %s", broadcasts[0].Action)
	}
	if broadcasts[0].Data != nil {
		t.Errorf("expected nil data, got %v", broadcasts[0].Data)
	}

	if broadcasts[1].Action != "UpdateUsers" {
		t.Errorf("expected action UpdateUsers, got %s", broadcasts[1].Action)
	}
	if broadcasts[1].Data["count"] != 5 {
		t.Errorf("expected count=5, got %v", broadcasts[1].Data["count"])
	}
}

func TestBroadcastAction_PendingBroadcastsClearsAfterRead(t *testing.T) {
	ctx := NewContext(context.Background(), "TestAction", nil)

	ctx.BroadcastAction("Refresh", nil)
	broadcasts := ctx.pendingBroadcasts()
	if len(broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(broadcasts))
	}

	// Second call should return empty
	broadcasts = ctx.pendingBroadcasts()
	if len(broadcasts) != 0 {
		t.Fatalf("expected 0 broadcasts after clear, got %d", len(broadcasts))
	}
}

func TestBroadcastAction_NoBroadcastsReturnsEmpty(t *testing.T) {
	ctx := NewContext(context.Background(), "TestAction", nil)

	broadcasts := ctx.pendingBroadcasts()
	if broadcasts != nil {
		t.Fatalf("expected nil broadcasts, got %v", broadcasts)
	}
}

func TestBroadcastAction_PreservedThroughWithMethods(t *testing.T) {
	ctx := NewContext(context.Background(), "TestAction", nil)
	ctx.BroadcastAction("First", nil)

	// WithUserID creates a shallow copy — broadcasts are a slice reference
	// but BroadcastAction appends to the original's backing array.
	// After WithUserID, the new context has its own broadcasts slice.
	ctx2 := ctx.WithUserID("alice")
	ctx2.BroadcastAction("Second", nil)

	// Original context should have only 1 broadcast
	b1 := ctx.pendingBroadcasts()
	if len(b1) != 1 {
		t.Fatalf("original ctx expected 1 broadcast, got %d", len(b1))
	}

	// Derived context should have 2 (copied 1 + appended 1)
	b2 := ctx2.pendingBroadcasts()
	if len(b2) != 2 {
		t.Fatalf("derived ctx expected 2 broadcasts, got %d", len(b2))
	}
}
