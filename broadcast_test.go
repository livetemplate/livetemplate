package livetemplate

import (
	"sync"
	"testing"
	"time"
)

// BroadcastState is a test store for broadcasting tests
type BroadcastState struct {
	Value int
}

func (s *BroadcastState) Change(ctx *ActionContext) error {
	// No-op for testing - we only use broadcasting
	return nil
}

// TestLiveHandler_Broadcast tests broadcasting to all connections
func TestLiveHandler_Broadcast(t *testing.T) {
	tmpl := New("broadcast-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create mock connections
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)
	conn3 := createMockConnection(t, "user3", "group1", tmpl)

	// Register connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)
	h.registry.Register(conn3)

	// Broadcast
	err := handler.Broadcast(&BroadcastState{Value: 42})
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}

	// Verify registry has all connections
	if h.registry.Count() != 3 {
		t.Errorf("Expected 3 connections in registry, got %d", h.registry.Count())
	}
}

// TestLiveHandler_BroadcastToUsers tests broadcasting to specific users
func TestLiveHandler_BroadcastToUsers(t *testing.T) {
	tmpl := New("broadcast-users-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create mock connections
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)
	conn3 := createMockConnection(t, "user1", "group3", tmpl) // Same user, different group

	// Register connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)
	h.registry.Register(conn3)

	// Broadcast to user1 only
	err := handler.BroadcastToUsers([]string{"user1"}, &BroadcastState{Value: 42})
	if err != nil {
		t.Errorf("BroadcastToUsers failed: %v", err)
	}

	// Verify user1 has 2 connections (conn1 and conn3)
	user1Conns := h.registry.GetByUser("user1")
	if len(user1Conns) != 2 {
		t.Errorf("Expected 2 connections for user1, got %d", len(user1Conns))
	}
}

// TestLiveHandler_BroadcastToGroup tests broadcasting to a specific session group
func TestLiveHandler_BroadcastToGroup(t *testing.T) {
	tmpl := New("broadcast-group-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create mock connections
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)
	conn3 := createMockConnection(t, "user3", "group1", tmpl) // Same group as conn1

	// Register connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)
	h.registry.Register(conn3)

	// Broadcast to group1 only
	err := handler.BroadcastToGroup("group1", &BroadcastState{Value: 42})
	if err != nil {
		t.Errorf("BroadcastToGroup failed: %v", err)
	}

	// Verify group1 has 2 connections (conn1 and conn3)
	group1Conns := h.registry.GetByGroup("group1")
	if len(group1Conns) != 2 {
		t.Errorf("Expected 2 connections for group1, got %d", len(group1Conns))
	}
}

// TestLiveHandler_BroadcastNoConnections tests broadcasting when no connections exist
func TestLiveHandler_BroadcastNoConnections(t *testing.T) {
	tmpl := New("broadcast-empty-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Broadcast with no connections (should not error)
	err := handler.Broadcast(&BroadcastState{Value: 42})
	if err != nil {
		t.Errorf("Broadcast with no connections should not error, got: %v", err)
	}
}

// TestLiveHandler_BroadcastToUsersEmpty tests broadcasting to empty user list
func TestLiveHandler_BroadcastToUsersEmpty(t *testing.T) {
	tmpl := New("broadcast-users-empty-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Broadcast to empty user list (should error)
	err := handler.BroadcastToUsers([]string{}, &BroadcastState{Value: 42})
	if err == nil {
		t.Error("BroadcastToUsers with empty user list should error")
	}
}

// TestLiveHandler_BroadcastToGroupEmpty tests broadcasting to empty group ID
func TestLiveHandler_BroadcastToGroupEmpty(t *testing.T) {
	tmpl := New("broadcast-group-empty-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Broadcast to empty group ID (should error)
	err := handler.BroadcastToGroup("", &BroadcastState{Value: 42})
	if err == nil {
		t.Error("BroadcastToGroup with empty group ID should error")
	}
}

// TestLiveHandler_BroadcastConcurrent tests concurrent broadcasting
func TestLiveHandler_BroadcastConcurrent(t *testing.T) {
	tmpl := New("broadcast-concurrent-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create mock connections
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)

	// Register connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)

	// Concurrent broadcasts
	var wg sync.WaitGroup
	broadcasts := 10

	for i := 0; i < broadcasts; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			err := handler.Broadcast(&BroadcastState{Value: val})
			if err != nil {
				t.Errorf("Concurrent broadcast failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all connections still registered
	if h.registry.Count() != 2 {
		t.Errorf("Expected 2 connections after concurrent broadcasts, got %d", h.registry.Count())
	}
}

// TestLiveHandler_BroadcastToUsersConcurrent tests concurrent user broadcasts
func TestLiveHandler_BroadcastToUsersConcurrent(t *testing.T) {
	tmpl := New("broadcast-users-concurrent-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create mock connections
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)

	// Register connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)

	// Concurrent user broadcasts
	var wg sync.WaitGroup
	broadcasts := 10

	for i := 0; i < broadcasts; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			err := handler.BroadcastToUsers([]string{"user1"}, &BroadcastState{Value: val})
			if err != nil {
				t.Errorf("Concurrent BroadcastToUsers failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify user1 still has 1 connection
	user1Conns := h.registry.GetByUser("user1")
	if len(user1Conns) != 1 {
		t.Errorf("Expected 1 connection for user1, got %d", len(user1Conns))
	}
}

// createMockConnection creates a mock connection for testing
func createMockConnection(t *testing.T, userID, groupID string, tmpl *Template) *Connection {
	t.Helper()

	// Clone template for this connection
	connTmpl, err := tmpl.Clone()
	if err != nil {
		t.Fatalf("Failed to clone template: %v", err)
	}

	return &Connection{
		Conn:     nil, // Nil Conn triggers test mode in sendUpdate
		UserID:   userID,
		GroupID:  groupID,
		Template: connTmpl,
		Stores:   make(Stores),
	}
}

// TestLiveHandler_BroadcastMultipleGroups tests broadcasting to multiple users across groups
func TestLiveHandler_BroadcastMultipleGroups(t *testing.T) {
	tmpl := New("broadcast-multi-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create connections for multiple users across different groups
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)
	conn3 := createMockConnection(t, "user3", "group3", tmpl)
	conn4 := createMockConnection(t, "user1", "group4", tmpl) // user1 in different group

	// Register all connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)
	h.registry.Register(conn3)
	h.registry.Register(conn4)

	// Broadcast to multiple users
	err := handler.BroadcastToUsers([]string{"user1", "user3"}, &BroadcastState{Value: 100})
	if err != nil {
		t.Errorf("BroadcastToUsers failed: %v", err)
	}

	// Verify user1 has 2 connections and user3 has 1
	user1Conns := h.registry.GetByUser("user1")
	if len(user1Conns) != 2 {
		t.Errorf("Expected 2 connections for user1, got %d", len(user1Conns))
	}
	user3Conns := h.registry.GetByUser("user3")
	if len(user3Conns) != 1 {
		t.Errorf("Expected 1 connection for user3, got %d", len(user3Conns))
	}
}

// TestLiveHandler_BroadcastAfterDisconnect tests broadcasting after connection disconnect
func TestLiveHandler_BroadcastAfterDisconnect(t *testing.T) {
	tmpl := New("broadcast-disconnect-test")
	if _, err := tmpl.Parse("<p>Value: {{.Value}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&BroadcastState{Value: 0})

	// Create connections
	conn1 := createMockConnection(t, "user1", "group1", tmpl)
	conn2 := createMockConnection(t, "user2", "group2", tmpl)

	// Register connections
	h := handler.(*liveHandler)
	h.registry.Register(conn1)
	h.registry.Register(conn2)

	// Unregister conn1
	h.registry.Unregister(conn1)

	// Small delay to ensure unregister completes
	time.Sleep(10 * time.Millisecond)

	// Broadcast
	err := handler.Broadcast(&BroadcastState{Value: 42})
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}

	// Verify only conn2 is registered
	if h.registry.Count() != 1 {
		t.Errorf("Expected 1 connection after disconnect, got %d", h.registry.Count())
	}
}
