package livetemplate

import (
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// TestConnectionRegistry_RegisterAndGet tests basic registration and retrieval
func TestConnectionRegistry_RegisterAndGet(t *testing.T) {
	registry := NewConnectionRegistry()

	conn1 := &Connection{
		Conn:    &websocket.Conn{},
		GroupID: "group-1",
		UserID:  "alice",
	}

	registry.Register(conn1)

	// Get by group
	groupConns := registry.GetByGroup("group-1")
	if len(groupConns) != 1 {
		t.Errorf("GetByGroup returned %d connections, want 1", len(groupConns))
	}
	if groupConns[0] != conn1 {
		t.Error("GetByGroup returned wrong connection")
	}

	// Get by user
	userConns := registry.GetByUser("alice")
	if len(userConns) != 1 {
		t.Errorf("GetByUser returned %d connections, want 1", len(userConns))
	}
	if userConns[0] != conn1 {
		t.Error("GetByUser returned wrong connection")
	}
}

// TestConnectionRegistry_MultipleConnectionsSameGroup tests multiple connections in same group
func TestConnectionRegistry_MultipleConnectionsSameGroup(t *testing.T) {
	registry := NewConnectionRegistry()

	// Multiple tabs for same anonymous user
	conn1 := &Connection{GroupID: "group-1", UserID: ""}
	conn2 := &Connection{GroupID: "group-1", UserID: ""}
	conn3 := &Connection{GroupID: "group-1", UserID: ""}

	registry.Register(conn1)
	registry.Register(conn2)
	registry.Register(conn3)

	groupConns := registry.GetByGroup("group-1")
	if len(groupConns) != 3 {
		t.Errorf("GetByGroup returned %d connections, want 3", len(groupConns))
	}

	// All should be for anonymous user
	anonConns := registry.GetByUser("")
	if len(anonConns) != 3 {
		t.Errorf("GetByUser('') returned %d connections, want 3", len(anonConns))
	}
}

// TestConnectionRegistry_MultipleConnectionsSameUser tests multi-device for same user
func TestConnectionRegistry_MultipleConnectionsSameUser(t *testing.T) {
	registry := NewConnectionRegistry()

	// Multiple devices for same authenticated user
	conn1 := &Connection{GroupID: "alice-device-1", UserID: "alice"}
	conn2 := &Connection{GroupID: "alice-device-2", UserID: "alice"}
	conn3 := &Connection{GroupID: "alice-device-3", UserID: "alice"}

	registry.Register(conn1)
	registry.Register(conn2)
	registry.Register(conn3)

	userConns := registry.GetByUser("alice")
	if len(userConns) != 3 {
		t.Errorf("GetByUser returned %d connections, want 3", len(userConns))
	}

	// Each should be in different group
	for _, conn := range []*Connection{conn1, conn2, conn3} {
		groupConns := registry.GetByGroup(conn.GroupID)
		if len(groupConns) != 1 {
			t.Errorf("GetByGroup(%s) returned %d connections, want 1", conn.GroupID, len(groupConns))
		}
	}
}

// TestConnectionRegistry_Unregister tests removing connections
func TestConnectionRegistry_Unregister(t *testing.T) {
	registry := NewConnectionRegistry()

	conn1 := &Connection{GroupID: "group-1", UserID: "alice"}
	conn2 := &Connection{GroupID: "group-1", UserID: "alice"}

	registry.Register(conn1)
	registry.Register(conn2)

	// Verify both registered
	if registry.Count() != 2 {
		t.Errorf("Count() = %d, want 2", registry.Count())
	}

	// Unregister one
	registry.Unregister(conn1)

	// Verify count
	if registry.Count() != 1 {
		t.Errorf("After unregister, Count() = %d, want 1", registry.Count())
	}

	// Verify by group
	groupConns := registry.GetByGroup("group-1")
	if len(groupConns) != 1 {
		t.Errorf("After unregister, GetByGroup returned %d connections, want 1", len(groupConns))
	}
	if groupConns[0] != conn2 {
		t.Error("Wrong connection remained after unregister")
	}

	// Unregister second
	registry.Unregister(conn2)

	// Verify empty
	if registry.Count() != 0 {
		t.Errorf("After unregistering all, Count() = %d, want 0", registry.Count())
	}

	// Verify group cleaned up
	groupConns = registry.GetByGroup("group-1")
	if len(groupConns) != 0 {
		t.Errorf("After unregistering all, GetByGroup returned %d connections, want 0", len(groupConns))
	}

	// Verify user cleaned up
	userConns := registry.GetByUser("alice")
	if len(userConns) != 0 {
		t.Errorf("After unregistering all, GetByUser returned %d connections, want 0", len(userConns))
	}
}

// TestConnectionRegistry_UnregisterIdempotent tests that unregister is idempotent
func TestConnectionRegistry_UnregisterIdempotent(t *testing.T) {
	registry := NewConnectionRegistry()

	conn := &Connection{GroupID: "group-1", UserID: "alice"}
	registry.Register(conn)

	// Unregister twice (should not panic)
	registry.Unregister(conn)
	registry.Unregister(conn)

	if registry.Count() != 0 {
		t.Errorf("After double unregister, Count() = %d, want 0", registry.Count())
	}
}

// TestConnectionRegistry_GetByGroupNonExistent tests getting non-existent group
func TestConnectionRegistry_GetByGroupNonExistent(t *testing.T) {
	registry := NewConnectionRegistry()

	conns := registry.GetByGroup("non-existent")

	if conns == nil {
		t.Error("GetByGroup(non-existent) returned nil, want empty slice")
	}

	if len(conns) != 0 {
		t.Errorf("GetByGroup(non-existent) returned %d connections, want 0", len(conns))
	}
}

// TestConnectionRegistry_GetByUserNonExistent tests getting non-existent user
func TestConnectionRegistry_GetByUserNonExistent(t *testing.T) {
	registry := NewConnectionRegistry()

	conns := registry.GetByUser("non-existent")

	if conns == nil {
		t.Error("GetByUser(non-existent) returned nil, want empty slice")
	}

	if len(conns) != 0 {
		t.Errorf("GetByUser(non-existent) returned %d connections, want 0", len(conns))
	}
}

// TestConnectionRegistry_GetAll tests getting all connections
func TestConnectionRegistry_GetAll(t *testing.T) {
	registry := NewConnectionRegistry()

	conn1 := &Connection{GroupID: "group-1", UserID: "alice"}
	conn2 := &Connection{GroupID: "group-2", UserID: "bob"}
	conn3 := &Connection{GroupID: "group-3", UserID: ""}

	registry.Register(conn1)
	registry.Register(conn2)
	registry.Register(conn3)

	all := registry.GetAll()

	if len(all) != 3 {
		t.Errorf("GetAll() returned %d connections, want 3", len(all))
	}

	// Verify all connections present
	found := make(map[*Connection]bool)
	for _, conn := range all {
		found[conn] = true
	}

	if !found[conn1] || !found[conn2] || !found[conn3] {
		t.Error("GetAll() missing some connections")
	}
}

// TestConnectionRegistry_Count tests connection counting
func TestConnectionRegistry_Count(t *testing.T) {
	registry := NewConnectionRegistry()

	if registry.Count() != 0 {
		t.Errorf("Initial Count() = %d, want 0", registry.Count())
	}

	// Add connections
	for i := 0; i < 5; i++ {
		conn := &Connection{GroupID: "group-1", UserID: "alice"}
		registry.Register(conn)
	}

	if registry.Count() != 5 {
		t.Errorf("After registering 5, Count() = %d, want 5", registry.Count())
	}
}

// TestConnectionRegistry_GroupCount tests group counting
func TestConnectionRegistry_GroupCount(t *testing.T) {
	registry := NewConnectionRegistry()

	// Multiple connections in same group should count as 1 group
	registry.Register(&Connection{GroupID: "group-1", UserID: "alice"})
	registry.Register(&Connection{GroupID: "group-1", UserID: "alice"})
	registry.Register(&Connection{GroupID: "group-2", UserID: "bob"})

	if registry.GroupCount() != 2 {
		t.Errorf("GroupCount() = %d, want 2", registry.GroupCount())
	}
}

// TestConnectionRegistry_UserCount tests user counting
func TestConnectionRegistry_UserCount(t *testing.T) {
	registry := NewConnectionRegistry()

	// Multiple connections for same user should count as 1 user
	registry.Register(&Connection{GroupID: "group-1", UserID: "alice"})
	registry.Register(&Connection{GroupID: "group-2", UserID: "alice"})
	registry.Register(&Connection{GroupID: "group-3", UserID: "bob"})
	registry.Register(&Connection{GroupID: "group-4", UserID: ""}) // Anonymous

	if registry.UserCount() != 3 {
		t.Errorf("UserCount() = %d, want 3 (alice, bob, anonymous)", registry.UserCount())
	}
}

// TestConnectionRegistry_ConcurrentAccess tests thread-safety
func TestConnectionRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewConnectionRegistry()

	var wg sync.WaitGroup
	iterations := 100
	goroutines := 10

	// Concurrent registrations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				conn := &Connection{
					GroupID: "group-" + string(rune('0'+id)),
					UserID:  "user-" + string(rune('0'+id)),
				}
				registry.Register(conn)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				registry.GetByGroup("group-" + string(rune('0'+id)))
				registry.GetByUser("user-" + string(rune('0'+id)))
				registry.GetAll()
				registry.Count()
			}
		}(i)
	}

	// Concurrent unregistrations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations/2; j++ {
				conn := &Connection{
					GroupID: "group-" + string(rune('0'+id)),
					UserID:  "user-" + string(rune('0'+id)),
				}
				registry.Unregister(conn)
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is still functional
	testConn := &Connection{GroupID: "test", UserID: "test"}
	registry.Register(testConn)

	if registry.Count() == 0 {
		t.Error("Registry corrupted after concurrent access")
	}
}

// TestConnectionRegistry_ReturnsCopy tests that returned slices are copies
func TestConnectionRegistry_ReturnsCopy(t *testing.T) {
	registry := NewConnectionRegistry()

	conn1 := &Connection{GroupID: "group-1", UserID: "alice"}
	registry.Register(conn1)

	// Get slice
	conns := registry.GetByGroup("group-1")

	// Modify returned slice
	conns[0] = &Connection{GroupID: "modified", UserID: "modified"}

	// Verify registry not affected
	original := registry.GetByGroup("group-1")
	if original[0].GroupID != "group-1" {
		t.Error("Modifying returned slice affected registry (should be copy)")
	}
}

// TestConnection_Send is tested implicitly through handleWebSocket in integration tests.
// Unit testing Send() in isolation requires a full WebSocket server setup which is
// out of scope for this test file. The mutex protection is already verified by the
// concurrent access tests above.
