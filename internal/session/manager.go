package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session represents a user session
type Session struct {
	ID         string
	PageID     string
	AppID      string
	CreatedAt  time.Time
	LastAccess time.Time
	CacheToken string // Stable token for client-side caching
}

// Manager handles session lifecycle
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewManager creates a new session manager
func NewManager(ttl time.Duration) *Manager {
	if ttl == 0 {
		ttl = 24 * time.Hour // Default 24 hours
	}

	return &Manager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

// CreateSession creates a new session
func (m *Manager) CreateSession(appID, pageID, cacheToken string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:         sessionID,
		PageID:     pageID,
		AppID:      appID,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		CacheToken: cacheToken,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if session has expired
	if time.Since(session.LastAccess) > m.ttl {
		delete(m.sessions, sessionID)
		return nil, false
	}

	// Update last access time
	session.LastAccess = time.Now()
	return session, true
}

// DeleteSession removes a session
func (m *Manager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// CleanupExpiredSessions removes expired sessions
func (m *Manager) CleanupExpiredSessions() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	cutoff := time.Now().Add(-m.ttl)

	for sessionID, session := range m.sessions {
		if session.LastAccess.Before(cutoff) {
			delete(m.sessions, sessionID)
			count++
		}
	}

	return count
}

// generateSessionID creates a cryptographically secure session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 32) // 256-bit session ID
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
