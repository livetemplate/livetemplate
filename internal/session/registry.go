// Package session provides connection and session management for LiveTemplate.
// It handles WebSocket connection tracking, registration, and lookup operations.
package session

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection represents a WebSocket connection with associated metadata.
//
// Each connection belongs to a session group (via groupID) and has a user identity (userID).
// Multiple connections can share the same groupID (multi-tab) or userID (multi-device).
//
// The Template field is per-connection because ExecuteUpdates() maintains state (lastTree, lastData)
// for tree diffing, which must be independent for each connection.
//
// Type Safety Note: Template, Stores, and Uploads are interface{} to avoid circular imports with the parent
// livetemplate package. Consumers should use type assertions with the safe pattern:
//
//	tmpl, ok := conn.Template.(*livetemplate.Template)
//	if !ok {
//	    return fmt.Errorf("invalid template type")
//	}
//
// Expected types:
//   - Template: *livetemplate.Template
//   - Stores: livetemplate.Stores (map[string]Store)
//   - Uploads: *upload.Registry
type Connection struct {
	Conn     *websocket.Conn // WebSocket connection
	GroupID  string          // Session group ID (shared state boundary)
	UserID   string          // User identity ("" for anonymous)
	Template interface{}     // Per-connection template for tree diffing (*livetemplate.Template)
	Stores   interface{}     // Reference to shared stores from session group (livetemplate.Stores)
	Uploads  interface{}     // Per-connection upload registry (*upload.Registry)
	mu       sync.Mutex      // Protects writes to Conn

	// Async sending infrastructure
	sendChan   chan *wsMessage // Buffered channel for queued messages
	done       chan struct{}   // Signal for graceful shutdown
	pumpExited chan struct{}   // Signals writePump has exited cleanly
	closeOnce  sync.Once       // Prevents double-close race conditions
	metrics    MetricsRecorder // Optional: metrics recorder for observability
}

// wsMessage represents a WebSocket message to be sent asynchronously.
type wsMessage struct {
	messageType int
	data        []byte
}

// Send queues a message for async delivery to this connection.
// Thread-safe: multiple goroutines can call Send concurrently.
// Returns nil if message queued successfully.
// Returns error if connection closed or buffer full (client too slow).
//
// Note: Send returns immediately without waiting for actual delivery.
// Actual message transmission happens in the background writePump goroutine.
func (c *Connection) Send(messageType int, data []byte) error {
	// If channels not initialized, fall back to sync send (for tests)
	if c.sendChan == nil {
		// Allow nil Conn for testing (mock connections)
		if c.Conn == nil {
			return nil
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.Conn.WriteMessage(messageType, data)
	}

	// Check if connection is already closed (priority check)
	select {
	case <-c.done:
		return ErrConnectionClosed
	default:
	}

	// Try to queue message for async delivery
	select {
	case c.sendChan <- &wsMessage{messageType, data}:
		// Record buffer size metric
		if c.metrics != nil {
			c.metrics.WSAddBufferSize(1)
		}
		return nil // Message queued successfully
	case <-c.done:
		return ErrConnectionClosed
	default:
		// Buffer full - client is too slow, close connection
		if c.metrics != nil {
			c.metrics.WSBufferFull()
			c.metrics.WSSlowClientClose()
		}
		go c.Close()
		return ErrClientTooSlow
	}
}

var (
	// ErrConnectionClosed is returned when attempting to send on a closed connection
	ErrConnectionClosed = &ConnectionError{"connection closed"}
	// ErrClientTooSlow is returned when send buffer is full (client not consuming fast enough)
	ErrClientTooSlow = &ConnectionError{"client too slow, closing connection"}
)

// ConnectionError represents a connection-level error
type ConnectionError struct {
	msg string
}

func (e *ConnectionError) Error() string {
	return e.msg
}

// Close closes the WebSocket connection safely.
// Thread-safe: safe to call concurrently with Send and multiple times.
// Uses sync.Once to prevent double-close race conditions.
//
// Close sequence:
// 1. Signal writePump to stop (via done channel)
// 2. Wait for writePump to exit (with 5-second timeout)
// 3. Close the WebSocket connection
func (c *Connection) Close() error {
	// For uninitialized async infrastructure (no Register() called)
	if c.done == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.Conn != nil {
			return c.Conn.Close()
		}
		return nil
	}

	c.closeOnce.Do(func() {
		// 1. Signal writePump to stop (if not already signaled)
		select {
		case <-c.done:
			// Already closed
		default:
			close(c.done)
		}

		// 2. Wait for writePump to exit (with timeout)
		select {
		case <-c.pumpExited:
			// writePump exited cleanly
		case <-time.After(5 * time.Second):
			slog.Warn("writePump drain timeout, forcing close",
				slog.String("group_id", c.GroupID),
				slog.String("user_id", c.UserID))
		}

		// 3. Close the WebSocket connection
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.Conn != nil {
			c.Conn.Close()
		}
	})
	return nil
}

// writePump runs as a background goroutine per connection.
// Dequeues messages from sendChan and writes to WebSocket.
// Ensures goroutines never leak via defer cleanup.
func (c *Connection) writePump() {
	defer func() {
		close(c.pumpExited) // Signal that writePump has exited
		c.Close()           // Ensure connection is closed
	}()

	for {
		select {
		case msg := <-c.sendChan:
			// Skip write if connection is nil (for testing)
			if c.Conn == nil {
				continue
			}

			c.mu.Lock()
			err := c.Conn.WriteMessage(msg.messageType, msg.data)
			c.mu.Unlock()

			if err != nil {
				if c.metrics != nil {
					c.metrics.WSWriteError()
				}
				slog.Warn("WebSocket write failed, closing connection",
					slog.String("error", err.Error()),
					slog.Int("message_type", msg.messageType),
					slog.String("group_id", c.GroupID),
					slog.String("user_id", c.UserID))
				return
			}

			// Message sent successfully, decrement buffer size
			if c.metrics != nil {
				c.metrics.WSAddBufferSize(-1)
			}
		case <-c.done:
			// Drain remaining messages before closing
			c.drainSendChannel()
			return
		}
	}
}

// drainSendChannel attempts to send remaining queued messages.
// Non-blocking (uses default case) for graceful shutdown.
func (c *Connection) drainSendChannel() {
	for {
		select {
		case msg := <-c.sendChan:
			// Skip write if connection is nil (for testing)
			if c.Conn != nil {
				c.mu.Lock()
				_ = c.Conn.WriteMessage(msg.messageType, msg.data)
				c.mu.Unlock()
			}
			// Decrement buffer size metric for drained message
			if c.metrics != nil {
				c.metrics.WSAddBufferSize(-1)
			}
		default:
			return
		}
	}
}

// MetricsRecorder is an interface for recording WebSocket metrics.
// Implemented by *observe.Metrics in the main package.
type MetricsRecorder interface {
	WSBufferFull()
	WSSlowClientClose()
	WSWriteError()
	WSAddBufferSize(delta int64)
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
	metrics MetricsRecorder          // Optional: metrics recorder for observability
}

// NewConnectionRegistry creates a new empty connection registry.
func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		byGroup: make(map[string][]*Connection),
		byUser:  make(map[string][]*Connection),
	}
}

// SetMetrics sets the metrics recorder for observability.
// Optional: if not set, metrics are not recorded.
func (r *ConnectionRegistry) SetMetrics(m MetricsRecorder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = m
}

// Register adds a connection to the registry and starts its write pump.
//
// The connection is indexed by both groupID and userID for efficient lookups.
// Initializes async sending infrastructure and starts background writePump goroutine.
//
// bufferSize: Number of messages to buffer per connection (typically 10-200).
//
// If the connection is already registered, this is a no-op (idempotent).
func (r *ConnectionRegistry) Register(conn *Connection, bufferSize int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize async sending infrastructure
	conn.sendChan = make(chan *wsMessage, bufferSize)
	conn.done = make(chan struct{})
	conn.pumpExited = make(chan struct{})
	conn.metrics = r.metrics // Set metrics from registry

	// Start write pump goroutine
	go conn.writePump()

	// Add to byGroup index
	r.byGroup[conn.GroupID] = append(r.byGroup[conn.GroupID], conn)

	// Add to byUser index
	r.byUser[conn.UserID] = append(r.byUser[conn.UserID], conn)
}

// Unregister removes a connection from the registry and stops its write pump.
//
// Removes the connection from both indexes (byGroup and byUser).
// Closes the connection (idempotent due to sync.Once).
// If the connection is not found, this is a no-op (idempotent).
//
// Should be called when a WebSocket connection closes to prevent memory leaks.
func (r *ConnectionRegistry) Unregister(conn *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close the connection (triggers writePump shutdown)
	// This is idempotent due to sync.Once, safe to call multiple times
	conn.Close()

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

// GetByGroupExcept returns all connections for a session group except the specified one.
//
// Used for automatic broadcasting to avoid sending duplicate updates to the connection
// that triggered the action (it receives update through normal response flow).
//
// Returns a copy of the slice to prevent external modification.
// Returns empty slice if the group has no other connections.
//
// Example: Broadcast to other tabs after state change:
//
//	otherConns := registry.GetByGroupExcept("anon-abc123", currentConn)
//	for _, conn := range otherConns {
//	    conn.Send(websocket.TextMessage, update)
//	}
func (r *ConnectionRegistry) GetByGroupExcept(groupID string, excludeConn *Connection) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conns := r.byGroup[groupID]
	if conns == nil {
		return []*Connection{}
	}

	// Filter out the excluded connection
	result := make([]*Connection, 0, len(conns)-1)
	for _, conn := range conns {
		if conn != excludeConn {
			result = append(result, conn)
		}
	}
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

	// Pre-calculate total size to avoid multiple allocations
	total := 0
	for _, conns := range r.byGroup {
		total += len(conns)
	}

	result := make([]*Connection, 0, total)
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
