package livetemplate

import (
	"context"
	"sync"
	"time"
)

// SessionStore manages session groups, where each group contains Stores shared across connections.
//
// A session group is the fundamental isolation boundary: all connections with the same groupID
// share the same Stores instance. Different groupIDs have completely isolated state.
//
// For anonymous users: groupID is typically a browser-based identifier (all tabs share state).
// For authenticated users: groupID is typically the userID (each user has isolated state).
//
// Thread-safety: All implementations must be safe for concurrent access from multiple goroutines.
type SessionStore interface {
	// Get retrieves the Stores for a session group.
	// Returns nil if the group doesn't exist.
	Get(groupID string) Stores

	// Set stores Stores for a session group.
	// Creates a new group if it doesn't exist, updates if it does.
	Set(groupID string, stores Stores)

	// Delete removes a session group and all its state.
	Delete(groupID string)

	// List returns all active session group IDs.
	// Used for broadcasting and cleanup operations.
	List() []string
}

// MemorySessionStore is an in-memory session store with automatic cleanup.
//
// Features:
// - Thread-safe for concurrent access
// - Tracks last access time for each group
// - Automatic cleanup of inactive groups (configurable TTL)
// - Suitable for single-instance deployments
//
// For multi-instance deployments, use a persistent SessionStore (e.g., Redis).
type MemorySessionStore struct {
	groups     map[string]Stores    // groupID → Stores
	lastAccess map[string]time.Time // groupID → last access timestamp
	mu         sync.RWMutex         // Protects groups and lastAccess
	cleanupTTL time.Duration        // Time to live for inactive groups
	stopCh     chan struct{}        // Signal to stop cleanup goroutine
	ctx        context.Context      // Context for cleanup goroutine
	cancel     context.CancelFunc   // Cancel function for cleanup
}

// SessionStoreOption configures MemorySessionStore
type SessionStoreOption func(*MemorySessionStore)

// WithCleanupTTL sets the time-to-live for inactive session groups.
// Groups not accessed within this duration will be automatically cleaned up.
// Default: 24 hours
func WithCleanupTTL(ttl time.Duration) SessionStoreOption {
	return func(s *MemorySessionStore) {
		s.cleanupTTL = ttl
	}
}

// NewMemorySessionStore creates a new in-memory session store with automatic cleanup.
//
// Default configuration:
// - Cleanup TTL: 24 hours
// - Cleanup interval: 1 hour
//
// The cleanup goroutine runs in the background and removes session groups that
// haven't been accessed within the TTL period. This prevents memory leaks from
// abandoned sessions.
//
// Call Close() to stop the cleanup goroutine when shutting down.
func NewMemorySessionStore(opts ...SessionStoreOption) *MemorySessionStore {
	ctx, cancel := context.WithCancel(context.Background())

	s := &MemorySessionStore{
		groups:     make(map[string]Stores),
		lastAccess: make(map[string]time.Time),
		cleanupTTL: 24 * time.Hour, // Default: 24 hours
		stopCh:     make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Start cleanup goroutine
	go s.cleanupLoop()

	return s
}

// Get retrieves the Stores for a session group.
// Updates the last access time for the group.
func (s *MemorySessionStore) Get(groupID string) Stores {
	s.mu.Lock()
	defer s.mu.Unlock()

	stores := s.groups[groupID]
	if stores != nil {
		s.lastAccess[groupID] = time.Now()
	}
	return stores
}

// Set stores Stores for a session group.
// Updates the last access time for the group.
func (s *MemorySessionStore) Set(groupID string, stores Stores) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.groups[groupID] = stores
	s.lastAccess[groupID] = time.Now()
}

// Delete removes a session group and all its state.
func (s *MemorySessionStore) Delete(groupID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.groups, groupID)
	delete(s.lastAccess, groupID)
}

// List returns all active session group IDs.
func (s *MemorySessionStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groupIDs := make([]string, 0, len(s.groups))
	for groupID := range s.groups {
		groupIDs = append(groupIDs, groupID)
	}
	return groupIDs
}

// Close stops the cleanup goroutine.
// Should be called when shutting down the application.
func (s *MemorySessionStore) Close() {
	s.cancel()
	<-s.stopCh
}

// cleanupLoop runs in the background and removes inactive session groups.
func (s *MemorySessionStore) cleanupLoop() {
	defer close(s.stopCh)

	ticker := time.NewTicker(1 * time.Hour) // Cleanup interval
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes session groups that haven't been accessed within the TTL period.
func (s *MemorySessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for groupID, lastAccess := range s.lastAccess {
		if now.Sub(lastAccess) > s.cleanupTTL {
			delete(s.groups, groupID)
			delete(s.lastAccess, groupID)
		}
	}
}
