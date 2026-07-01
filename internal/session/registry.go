// Package session provides connection and session management for LiveTemplate.
// It handles WebSocket connection tracking, registration, and lookup operations.
package session

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// WSConn is the interface for a WebSocket connection used by the session package.
// This mirrors the WSConn interface in the root livetemplate package, defined here
// to avoid circular imports.
type WSConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
}

// Connection represents a WebSocket connection with associated metadata.
//
// Each connection belongs to a session group (via groupID) and has a user identity (userID).
// Multiple connections can share the same groupID (multi-tab) or userID (multi-device).
//
// The Template field is per-connection because ExecuteUpdates() maintains state (lastTree, lastData)
// for tree diffing, which must be independent for each connection.
//
// Type Safety Note: Template, State, and Uploads are interface{} to avoid circular imports with the parent
// livetemplate package. Consumers should use type assertions with the safe pattern:
//
//	tmpl, ok := conn.Template.(*livetemplate.Template)
//	if !ok {
//	    return fmt.Errorf("invalid template type")
//	}
//
// Expected types:
//   - Template: *livetemplate.Template
//   - State: the per-connection typed State value, updated per-action (for peer fan-out)
//   - Uploads: *upload.Registry
type Connection struct {
	Conn     WSConn      // WebSocket connection
	GroupID  string      // Session group ID (shared state boundary)
	UserID   string      // User identity ("" for anonymous)
	Template interface{} // Per-connection template for tree diffing (*livetemplate.Template)
	State    interface{} // Per-connection typed State snapshot, updated per-action (for peer fan-out).
	Uploads  interface{} // Per-connection upload registry (*upload.Registry)
	mu       sync.Mutex  // Protects writes to Conn

	// Async sending infrastructure
	sendChan   chan *wsMessage // Buffered channel for queued messages
	done       chan struct{}   // Signal for graceful shutdown
	pumpExited chan struct{}   // Signals writePump has exited cleanly
	closeOnce  sync.Once       // Prevents double-close race conditions
	metrics    MetricsRecorder // Optional: metrics recorder for observability

	// Dispatch channel for broadcast actions from other connections.
	// Actions enqueued here are processed by the connection's select-based event loop.
	DispatchChan chan *DispatchRequest

	// subscribedTopics is the GC root Unregister() walks to evict this conn from
	// byTopic/byTopicPattern (no reverse index — topics are many-to-many).
	// Accessed only under ConnectionRegistry.mu, so it needs no separate lock.
	subscribedTopics map[string]struct{}
}

// wsMessage represents a WebSocket message to be sent asynchronously.
type wsMessage struct {
	messageType int
	data        []byte
}

// DispatchKind classifies a DispatchRequest. It is a forward-compatible
// placeholder: v1 has exactly one kind, KindAction, which is the zero value so
// every existing DispatchRequest literal (which never sets Kind) keeps its
// current named-action semantics with no change. Future dispatch shapes add
// non-zero kinds here without breaking the wire or callers.
type DispatchKind int

const (
	// KindAction is a named-action dispatch resolved to a controller method by
	// name (the only kind in v1; the zero value, so it is backward-compatible).
	KindAction DispatchKind = iota
)

// DispatchRequest represents an action to dispatch on a connection's event loop.
// Used by topic Publish and Session.TriggerAction to fan out actions to other
// connections (any topic subscriber, or every connection in a session group).
type DispatchRequest struct {
	Action string
	Data   map[string]interface{}
	Kind   DispatchKind
}

// Done returns a channel that is closed when the connection is shutting down.
func (c *Connection) Done() <-chan struct{} {
	return c.done
}

// EnqueueDispatch queues an action for dispatch on this connection's event loop.
// Non-blocking: drops the request if the channel is full, closed, or not initialized.
func (c *Connection) EnqueueDispatch(req *DispatchRequest) {
	if c.DispatchChan == nil {
		return
	}
	select {
	case <-c.done:
		return // connection shutting down
	default:
	}
	select {
	case c.DispatchChan <- req:
		if c.metrics != nil {
			c.metrics.PublishSent()
		}
	case <-c.done:
		return // connection closed between checks
	default:
		if c.metrics != nil {
			c.metrics.WSDispatchDropped()
		}
		slog.Warn("dispatch channel full, dropping broadcast action",
			slog.String("action", req.Action),
			slog.String("group_id", c.GroupID))
	}
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
		go func() {
			if err := c.Close(); err != nil {
				slog.Warn("failed to close slow client connection",
					slog.Any("error", err),
					slog.String("group_id", c.GroupID),
					slog.String("user_id", c.UserID))
			}
		}()
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
			if err := c.Conn.Close(); err != nil {
				slog.Debug("WebSocket close returned error (may be expected if already closed)",
					slog.Any("error", err),
					slog.String("group_id", c.GroupID))
			}
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
		if err := c.Close(); err != nil {
			slog.Debug("connection close in writePump returned error",
				slog.Any("error", err),
				slog.String("group_id", c.GroupID))
		}
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
					slog.Any("error", err),
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
	WSDispatchDropped()
	PublishSent()
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
	byGroup            map[string][]*Connection // groupID → connections
	byUser             map[string][]*Connection // userID → connections  (empty string for anonymous)
	byTopic            map[string][]*Connection // exact pub/sub topic → connections
	byTopicPattern     map[string][]*Connection // wildcard topic pattern (contains "*") → connections
	mu                 sync.RWMutex             // Protects all four maps
	metrics            MetricsRecorder          // Optional: metrics recorder for observability
	dispatchBufferSize int                      // Dispatch channel buffer size (0 = use default)
}

const defaultDispatchBufferSize = 16

// NewConnectionRegistry creates a new empty connection registry.
func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		byGroup:        make(map[string][]*Connection),
		byUser:         make(map[string][]*Connection),
		byTopic:        make(map[string][]*Connection),
		byTopicPattern: make(map[string][]*Connection),
	}
}

// SetDispatchBufferSize sets the buffer size for the dispatch channel.
// Must be called before any connections are registered.
// Default: 16.
func (r *ConnectionRegistry) SetDispatchBufferSize(size int) {
	r.dispatchBufferSize = size
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
	dispatchBuf := r.dispatchBufferSize
	if dispatchBuf <= 0 {
		dispatchBuf = defaultDispatchBufferSize
	}
	conn.DispatchChan = make(chan *DispatchRequest, dispatchBuf)

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

	// Remove from indexes FIRST so no new dispatches target this connection.
	// Then close the connection (triggers writePump shutdown via done channel).
	// DispatchChan is NOT closed — senders use the done channel to detect shutdown.
	// This avoids send-on-closed-channel panics in concurrent EnqueueDispatch calls.

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

	// Topics are many-to-many, so unlike byGroup/byUser there is no O(1)
	// reverse lookup — walk the connection's own subscription set instead.
	for topic := range conn.subscribedTopics {
		index := r.byTopic
		if isPatternTopic(topic) {
			index = r.byTopicPattern
		}
		index[topic] = removeConnection(index[topic], conn)
		if len(index[topic]) == 0 {
			delete(index, topic)
		}
	}
	conn.subscribedTopics = nil

	// Close the connection AFTER removing from indexes.
	// This signals the done channel, which EnqueueDispatch checks before sending.
	if err := conn.Close(); err != nil {
		slog.Debug("connection close during unregister returned error",
			slog.Any("error", err),
			slog.String("group_id", conn.GroupID))
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

	// Filter out the excluded connection. When excludeConn is nil (e.g., HTTP path),
	// all connections are returned since no registered connection is nil.
	result := make([]*Connection, 0, len(conns)-1)
	for _, conn := range conns {
		if conn != excludeConn {
			result = append(result, conn)
		}
	}
	return result
}

// isPatternTopic reports whether topic is a wildcard pattern (contains "*")
// rather than an exact topic, selecting which index it belongs in.
func isPatternTopic(topic string) bool {
	return strings.Contains(topic, "*")
}

// SubscribeConnectionToTopic adds conn to the registry index for topic.
//
// Exact topics (no "*") go in byTopic; wildcard patterns in byTopicPattern.
// Idempotent set semantics: a repeat subscribe is a no-op (the index slice
// holds conn at most once per topic) — membership, deliberately not a
// ref-count. conn.subscribedTopics is lazily allocated here.
//
// Liveness short-circuit: if conn is already shutting down, drop the subscribe
// silently (mirrors EnqueueDispatch). Unregister() closes conn.done via
// conn.Close() *while holding r.mu*, so a closed done observed under this same
// lock means Unregister has run (or is committed) for conn — re-inserting here
// would resurrect a dead connection into byTopic/byTopicPattern that
// Unregister's topic-GC can never reclaim (the conn never Unregisters again).
// The subscription is re-established on WS reconnect via Mount, so silent-drop
// is the correct, lower-surprise policy (proposal Phase 1 decision).
//
// Returns true exactly on the per-connection 0→1 transition (this conn was not
// already subscribed and is now). Callers relaying to a cross-instance
// transport (Phase 2 Redis) act only on a true return so the instance-wide
// channel refcount is incremented exactly once per connection-topic — a
// liveness short-circuit or an idempotent repeat returns false.
func (r *ConnectionRegistry) SubscribeConnectionToTopic(conn *Connection, topic string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-conn.done:
		return false // connection shutting down — see Liveness short-circuit above
	default:
	}

	if conn.subscribedTopics == nil {
		conn.subscribedTopics = make(map[string]struct{})
	}
	if _, already := conn.subscribedTopics[topic]; already {
		return false // idempotent: already subscribed
	}
	conn.subscribedTopics[topic] = struct{}{}

	index := r.byTopic
	if isPatternTopic(topic) {
		index = r.byTopicPattern
	}
	index[topic] = append(index[topic], conn)
	return true
}

// UnsubscribeConnectionFromTopic removes conn from the registry index for topic.
//
// No-op if conn was not subscribed to topic. Emptied index entries are deleted
// to prevent map-key leaks (same discipline as byGroup/byUser in Unregister).
//
// Returns true exactly on the per-connection 1→0 transition (this conn was
// subscribed and is now removed). Callers relaying to a cross-instance
// transport act only on a true return, pairing it one-to-one with the true
// returned by SubscribeConnectionToTopic so the instance-wide channel refcount
// stays balanced. A not-subscribed no-op returns false.
func (r *ConnectionRegistry) UnsubscribeConnectionFromTopic(conn *Connection, topic string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, subscribed := conn.subscribedTopics[topic]; !subscribed {
		return false
	}
	delete(conn.subscribedTopics, topic)

	index := r.byTopic
	if isPatternTopic(topic) {
		index = r.byTopicPattern
	}
	index[topic] = removeConnection(index[topic], conn)
	if len(index[topic]) == 0 {
		delete(index, topic)
	}
	return true
}

// SubscribedTopics returns a snapshot of the topics conn is currently
// subscribed to. Used by the cross-instance relay teardown on disconnect to
// release exactly the channels still held (any explicitly Unsubscribed earlier
// were already removed from the set and relay-released then). Safe to call
// before Unregister (which nils the set); the returned slice is a copy.
func (r *ConnectionRegistry) SubscribedTopics(conn *Connection) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(conn.subscribedTopics) == 0 {
		return nil
	}
	topics := make([]string, 0, len(conn.subscribedTopics))
	for topic := range conn.subscribedTopics {
		topics = append(topics, topic)
	}
	return topics
}

// GetByTopicExcept returns the deduped union of connections subscribed to a
// concrete (publishable, no-"*") topic, excluding excludeConn (the publisher).
//
// Union = byTopic[concrete] ∪ { conns in byTopicPattern[p] : match(p, concrete) },
// deduplicated by *Connection identity, returned as a defensive copy. The
// pattern scan is a linear O(P) pass over distinct patterns — there is no
// trie/radix index, by design (proposal §2 "Matcher").
//
// match is dependency-injected: the segment matcher lives in the root package,
// which internal/session cannot import without an import cycle, so it is passed
// in. nil is safe ONLY when no pattern subscribers are registered — a
// time-of-call property (a nil-passing caller is safe until the first pattern
// subscriber, then panics). Callers that may face pattern subscribers must
// always pass a non-nil matcher; passing nil with patterns present panics by
// design (a loud programmer error, never a silent exact-only degradation).
func (r *ConnectionRegistry) GetByTopicExcept(concrete string, excludeConn *Connection, match func(pattern, concrete string) bool) []*Connection {
	if isPatternTopic(concrete) {
		// concrete must be a publishable topic; a "*" here would silently
		// mis-resolve (pattern-keyed exact lookup + self-match against every
		// indexed pattern). Loud over silent-wrong, symmetric with the
		// nil-match guard below.
		panic("session: GetByTopicExcept concrete topic must not contain \"*\" (publish to exact topics only)")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if match == nil && len(r.byTopicPattern) > 0 {
		// Loud, diagnosable programmer error (vs. an opaque nil-func panic
		// inside the loop). Nil match is safe only with zero pattern
		// subscribers — see TestGetByTopicExcept_NilMatchSafeWhenNoPatterns.
		panic("session: GetByTopicExcept requires a non-nil match when pattern subscribers are indexed (callers must pass the segment matcher)")
	}

	seen := make(map[*Connection]struct{})
	result := make([]*Connection, 0, len(r.byTopic[concrete]))

	add := func(conns []*Connection) {
		for _, conn := range conns {
			if conn == excludeConn {
				continue
			}
			if _, dup := seen[conn]; dup {
				continue
			}
			seen[conn] = struct{}{}
			result = append(result, conn)
		}
	}

	add(r.byTopic[concrete])
	for pattern, conns := range r.byTopicPattern {
		if match(pattern, concrete) {
			add(conns)
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
