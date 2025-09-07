package session

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		want time.Duration
	}{
		{
			name: "with custom TTL",
			ttl:  12 * time.Hour,
			want: 12 * time.Hour,
		},
		{
			name: "with zero TTL uses default",
			ttl:  0,
			want: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.ttl)
			if m == nil {
				t.Fatal("expected manager, got nil")
			}
			if m.ttl != tt.want {
				t.Errorf("ttl = %v, want %v", m.ttl, tt.want)
			}
			if m.sessions == nil {
				t.Error("sessions map not initialized")
			}
		})
	}
}

func TestCreateSession(t *testing.T) {
	m := NewManager(1 * time.Hour)

	sess, err := m.CreateSession("app123", "page456", "cache789")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if sess.ID == "" {
		t.Error("expected session ID, got empty string")
	}
	if sess.AppID != "app123" {
		t.Errorf("AppID = %s, want app123", sess.AppID)
	}
	if sess.PageID != "page456" {
		t.Errorf("PageID = %s, want page456", sess.PageID)
	}
	if sess.CacheToken != "cache789" {
		t.Errorf("CacheToken = %s, want cache789", sess.CacheToken)
	}
	if sess.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
	if sess.LastAccess.IsZero() {
		t.Error("LastAccess not set")
	}

	// Verify session is stored
	stored, exists := m.sessions[sess.ID]
	if !exists {
		t.Error("session not stored in manager")
	}
	if stored != sess {
		t.Error("stored session doesn't match returned session")
	}
}

func TestGetSession(t *testing.T) {
	m := NewManager(1 * time.Hour)

	// Create a session
	sess, err := m.CreateSession("app123", "page456", "cache789")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Test getting existing session
	retrieved, exists := m.GetSession(sess.ID)
	if !exists {
		t.Error("expected session to exist")
	}
	if retrieved.ID != sess.ID {
		t.Errorf("retrieved ID = %s, want %s", retrieved.ID, sess.ID)
	}
	if retrieved.AppID != sess.AppID {
		t.Errorf("retrieved AppID = %s, want %s", retrieved.AppID, sess.AppID)
	}

	// Test getting non-existent session
	_, exists = m.GetSession("nonexistent")
	if exists {
		t.Error("expected no session for non-existent ID")
	}
}

func TestSessionExpiration(t *testing.T) {
	// Use very short TTL for testing
	m := NewManager(50 * time.Millisecond)

	sess, err := m.CreateSession("app123", "page456", "cache789")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Session should exist immediately
	_, exists := m.GetSession(sess.ID)
	if !exists {
		t.Error("session should exist immediately after creation")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Session should be expired and removed
	_, exists = m.GetSession(sess.ID)
	if exists {
		t.Error("session should be expired and removed")
	}

	// Verify it's actually removed from the map
	m.mu.RLock()
	_, stillInMap := m.sessions[sess.ID]
	m.mu.RUnlock()
	if stillInMap {
		t.Error("expired session still in map")
	}
}

func TestSessionLastAccessUpdate(t *testing.T) {
	m := NewManager(1 * time.Hour)

	sess, err := m.CreateSession("app123", "page456", "cache789")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	originalAccess := sess.LastAccess

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Get session should update LastAccess
	retrieved, exists := m.GetSession(sess.ID)
	if !exists {
		t.Fatal("session should exist")
	}

	if !retrieved.LastAccess.After(originalAccess) {
		t.Error("LastAccess should be updated after GetSession")
	}
}

func TestDeleteSession(t *testing.T) {
	m := NewManager(1 * time.Hour)

	sess, err := m.CreateSession("app123", "page456", "cache789")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session exists
	_, exists := m.GetSession(sess.ID)
	if !exists {
		t.Error("session should exist before deletion")
	}

	// Delete the session
	m.DeleteSession(sess.ID)

	// Verify session is deleted
	_, exists = m.GetSession(sess.ID)
	if exists {
		t.Error("session should not exist after deletion")
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	m := NewManager(100 * time.Millisecond) // Longer TTL to avoid race conditions

	// Create multiple sessions
	sess1, _ := m.CreateSession("app1", "page1", "cache1")
	sess2, _ := m.CreateSession("app2", "page2", "cache2")
	sess3, _ := m.CreateSession("app3", "page3", "cache3")

	// Access sess1 to keep it fresh
	m.GetSession(sess1.ID)

	// Wait for some time, but not enough to expire sess1
	time.Sleep(60 * time.Millisecond)

	// Access sess1 again to update its LastAccess - keeps it fresh
	m.GetSession(sess1.ID)

	// Wait for sess2 and sess3 to expire (they weren't accessed)
	time.Sleep(60 * time.Millisecond) // Now sess2 and sess3 are over 120ms old, sess1 is ~60ms

	// Run cleanup
	count := m.CleanupExpiredSessions()

	// Should have cleaned up 2 sessions (sess2 and sess3)
	if count != 2 {
		t.Errorf("CleanupExpiredSessions returned %d, want 2", count)
	}

	// sess1 should still exist
	_, exists := m.GetSession(sess1.ID)
	if !exists {
		t.Error("sess1 should still exist after cleanup")
	}

	// sess2 and sess3 should not exist
	_, exists = m.GetSession(sess2.ID)
	if exists {
		t.Error("sess2 should not exist after cleanup")
	}

	_, exists = m.GetSession(sess3.ID)
	if exists {
		t.Error("sess3 should not exist after cleanup")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(1 * time.Hour)

	// Create initial session
	sess, err := m.CreateSession("app", "page", "cache")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = m.GetSession(sess.ID)
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				_, _ = m.CreateSession("app", "page", "cache")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Should not panic or deadlock
	_, exists := m.GetSession(sess.ID)
	if !exists {
		t.Error("original session should still exist")
	}
}

func TestGenerateSessionID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs and check for uniqueness
	for i := 0; i < 100; i++ {
		id, err := generateSessionID()
		if err != nil {
			t.Fatalf("generateSessionID failed: %v", err)
		}

		if id == "" {
			t.Error("generated empty session ID")
		}

		// Check length (32 bytes = 64 hex characters)
		if len(id) != 64 {
			t.Errorf("session ID length = %d, want 64", len(id))
		}

		// Check uniqueness
		if ids[id] {
			t.Errorf("duplicate session ID generated: %s", id)
		}
		ids[id] = true
	}
}
