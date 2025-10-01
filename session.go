package livetemplate

import "sync"

// SessionStore manages state for HTTP connections
type SessionStore interface {
	Get(sessionID string) interface{}
	Set(sessionID string, state interface{})
	Delete(sessionID string)
}

// MemorySessionStore is a simple in-memory session store
type MemorySessionStore struct {
	sessions map[string]interface{}
	mu       sync.RWMutex
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]interface{}),
	}
}

// Get retrieves a session
func (s *MemorySessionStore) Get(sessionID string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

// Set stores a session
func (s *MemorySessionStore) Set(sessionID string, state interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = state
}

// Delete removes a session
func (s *MemorySessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}
