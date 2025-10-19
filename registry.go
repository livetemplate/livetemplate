package livetemplate

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Connection represents a WebSocket connection with associated metadata.
//
// Each connection belongs to a session group (via groupID) and has a user identity (userID).
// Multiple connections can share the same groupID (multi-tab) or userID (multi-device).
//
// The Template field is per-connection because ExecuteUpdates() maintains state (lastTree, lastData)
// for tree diffing, which must be independent for each connection.
type Connection struct {
	Conn     *websocket.Conn // WebSocket connection
	GroupID  string          // Session group ID (shared state boundary)
	UserID   string          // User identity ("" for anonymous)
	Template *Template       // Per-connection template for tree diffing
	Stores   Stores          // Reference to shared stores from session group
	mu       sync.Mutex      // Protects writes to Conn
}

// Send sends a message to this connection.
// Thread-safe: multiple goroutines can call Send concurrently.
func (c *Connection) Send(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

// ConnectionRegistry tracks all active WebSocket connections with dual indexing.
//
// Dual indexing enables efficient broadcasting:
// - By groupID: Broadcast to all connections in a session group (multi-tab updates)
// - By userID: Broadcast to all connections for a user (multi-device updates)
//
// Thread-safe: safe for concurrent access from multiple goroutines.
//
// Example use cases:
// - GetByGroup("group-123"): Get all tabs for an anonymous user
// - GetByUser("alice"): Get all devices for authenticated user "alice"
// - GetByUser(""): Get all connections for anonymous users
type ConnectionRegistry struct {
	byGroup map[string][]*Connection // groupID → connections
	byUser  map[string][]*Connection // userID → connections  (empty string for anonymous)
	mu      sync.RWMutex             // Protects both maps
}

// NewConnectionRegistry creates a new empty connection registry.
func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		byGroup: make(map[string][]*Connection),
		byUser:  make(map[string][]*Connection),
	}
}

// Register adds a connection to the registry.
//
// The connection is indexed by both groupID and userID for efficient lookups.
// If the connection is already registered, this is a no-op (idempotent).
func (r *ConnectionRegistry) Register(conn *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Add to byGroup index
	r.byGroup[conn.GroupID] = append(r.byGroup[conn.GroupID], conn)

	// Add to byUser index
	r.byUser[conn.UserID] = append(r.byUser[conn.UserID], conn)
}

// Unregister removes a connection from the registry.
//
// Removes the connection from both indexes (byGroup and byUser).
// If the connection is not found, this is a no-op (idempotent).
//
// Should be called when a WebSocket connection closes to prevent memory leaks.
func (r *ConnectionRegistry) Unregister(conn *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from byGroup index
	groupConns := r.byGroup[conn.GroupID]
	r.byGroup[conn.GroupID] = removeConnection(groupConns, conn)

	// Clean up empty slices to prevent memory leaks
	if len(r.byGroup[conn.GroupID]) == 0 {
		delete(r.byGroup, conn.GroupID)
	}

	// Remove from byUser index
	userConns := r.byUser[conn.UserID]
	r.byUser[conn.UserID] = removeConnection(userConns, conn)

	// Clean up empty slices
	if len(r.byUser[conn.UserID]) == 0 {
		delete(r.byUser, conn.UserID)
	}
}

// GetByGroup returns all connections for a session group.
//
// Returns a copy of the slice to prevent external modification.
// Returns empty slice if the group has no connections.
//
// Example: Get all tabs for an anonymous user:
//
//	connections := registry.GetByGroup("anon-abc123")
//	for _, conn := range connections {
//	    conn.Send(websocket.TextMessage, update)
//	}
func (r *ConnectionRegistry) GetByGroup(groupID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conns := r.byGroup[groupID]
	if conns == nil {
		return []*Connection{}
	}

	// Return copy to prevent external modification
	result := make([]*Connection, len(conns))
	copy(result, conns)
	return result
}

// GetByUser returns all connections for a user.
//
// Returns a copy of the slice to prevent external modification.
// Returns empty slice if the user has no connections.
//
// For anonymous users (userID = ""), returns all anonymous connections.
//
// Example: Get all devices for authenticated user:
//
//	connections := registry.GetByUser("alice")
//	for _, conn := range connections {
//	    conn.Send(websocket.TextMessage, notification)
//	}
func (r *ConnectionRegistry) GetByUser(userID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conns := r.byUser[userID]
	if conns == nil {
		return []*Connection{}
	}

	// Return copy to prevent external modification
	result := make([]*Connection, len(conns))
	copy(result, conns)
	return result
}

// GetAll returns all active connections.
//
// Returns a copy of all connections from all groups.
// Useful for broadcasting to everyone.
//
// Example: Broadcast system announcement to all users:
//
//	connections := registry.GetAll()
//	for _, conn := range connections {
//	    conn.Send(websocket.TextMessage, announcement)
//	}
func (r *ConnectionRegistry) GetAll() []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Connection
	for _, conns := range r.byGroup {
		result = append(result, conns...)
	}
	return result
}

// Count returns the total number of active connections.
func (r *ConnectionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, conns := range r.byGroup {
		count += len(conns)
	}
	return count
}

// GroupCount returns the number of session groups.
func (r *ConnectionRegistry) GroupCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byGroup)
}

// UserCount returns the number of unique users (including anonymous as one "user").
func (r *ConnectionRegistry) UserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byUser)
}

// removeConnection removes a specific connection from a slice.
// Returns a new slice without the connection.
func removeConnection(conns []*Connection, target *Connection) []*Connection {
	result := make([]*Connection, 0, len(conns))
	for _, conn := range conns {
		if conn != target {
			result = append(result, conn)
		}
	}
	return result
}
