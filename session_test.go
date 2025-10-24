package livetemplate

import (
	"sync"
	"testing"
	"time"
)

// TestMemorySessionStore_SetAndGet tests basic set/get operations
func TestMemorySessionStore_SetAndGet(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Create test stores
	stores := Stores{
		"counter": &testStore{value: 42},
	}

	// Set
	store.Set("group-1", stores)

	// Get
	retrieved := store.Get("group-1")

	if retrieved == nil {
		t.Fatal("Get() returned nil, expected stores")
	}

	if len(retrieved) != 1 {
		t.Errorf("Get() returned %d stores, want 1", len(retrieved))
	}

	counterStore, ok := retrieved["counter"].(*testStore)
	if !ok {
		t.Fatal("Retrieved store is not *testStore")
	}

	if counterStore.value != 42 {
		t.Errorf("Retrieved store value = %d, want 42", counterStore.value)
	}
}

// TestMemorySessionStore_GetNonExistent tests getting a non-existent group
func TestMemorySessionStore_GetNonExistent(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	retrieved := store.Get("non-existent")

	if retrieved != nil {
		t.Errorf("Get(non-existent) = %v, want nil", retrieved)
	}
}

// TestMemorySessionStore_Delete tests deletion of session groups
func TestMemorySessionStore_Delete(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	stores := Stores{
		"counter": &testStore{value: 42},
	}

	// Set and verify
	store.Set("group-1", stores)
	if store.Get("group-1") == nil {
		t.Fatal("Failed to set group")
	}

	// Delete
	store.Delete("group-1")

	// Verify deleted
	if store.Get("group-1") != nil {
		t.Error("Get() after Delete() returned non-nil, expected nil")
	}
}

// TestMemorySessionStore_List tests listing all group IDs
func TestMemorySessionStore_List(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Initially empty
	list := store.List()
	if len(list) != 0 {
		t.Errorf("List() returned %d groups, want 0", len(list))
	}

	// Add groups
	store.Set("group-1", Stores{"a": &testStore{value: 1}})
	store.Set("group-2", Stores{"b": &testStore{value: 2}})
	store.Set("group-3", Stores{"c": &testStore{value: 3}})

	// List should have all 3
	list = store.List()
	if len(list) != 3 {
		t.Errorf("List() returned %d groups, want 3", len(list))
	}

	// Verify all groups are present (order doesn't matter)
	groupMap := make(map[string]bool)
	for _, id := range list {
		groupMap[id] = true
	}

	expectedGroups := []string{"group-1", "group-2", "group-3"}
	for _, expected := range expectedGroups {
		if !groupMap[expected] {
			t.Errorf("List() missing expected group: %s", expected)
		}
	}
}

// TestMemorySessionStore_Update tests updating existing groups
func TestMemorySessionStore_Update(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Initial stores
	stores1 := Stores{
		"counter": &testStore{value: 1},
	}
	store.Set("group-1", stores1)

	// Update with new stores
	stores2 := Stores{
		"counter": &testStore{value: 2},
	}
	store.Set("group-1", stores2)

	// Verify updated value
	retrieved := store.Get("group-1")
	counterStore := retrieved["counter"].(*testStore)

	if counterStore.value != 2 {
		t.Errorf("After update, value = %d, want 2", counterStore.value)
	}
}

// TestMemorySessionStore_ConcurrentAccess tests thread-safety
func TestMemorySessionStore_ConcurrentAccess(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	var wg sync.WaitGroup
	iterations := 100
	goroutines := 10

	// Concurrent writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				groupID := "group-" + string(rune('0'+id))
				stores := Stores{
					"counter": &testStore{value: j},
				}
				store.Set(groupID, stores)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				groupID := "group-" + string(rune('0'+id))
				_ = store.Get(groupID)
			}
		}(i)
	}

	// Concurrent list operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = store.List()
			}
		}()
	}

	// Wait for all goroutines
	wg.Wait()

	// Verify store is still functional
	testStores := Stores{"test": &testStore{value: 999}}
	store.Set("test-group", testStores)

	retrieved := store.Get("test-group")
	if retrieved == nil {
		t.Error("Store corrupted after concurrent access")
	}
}

// TestMemorySessionStore_LastAccessTracking tests that Get and Set update last access time
func TestMemorySessionStore_LastAccessTracking(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	stores := Stores{"counter": &testStore{value: 1}}
	store.Set("group-1", stores)

	// Get initial last access time
	store.mu.RLock()
	lastAccess1 := store.lastAccess["group-1"]
	store.mu.RUnlock()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Get should update last access
	store.Get("group-1")

	store.mu.RLock()
	lastAccess2 := store.lastAccess["group-1"]
	store.mu.RUnlock()

	if !lastAccess2.After(lastAccess1) {
		t.Error("Get() did not update last access time")
	}

	// Wait a bit more
	time.Sleep(10 * time.Millisecond)

	// Set should also update last access
	store.Set("group-1", stores)

	store.mu.RLock()
	lastAccess3 := store.lastAccess["group-1"]
	store.mu.RUnlock()

	if !lastAccess3.After(lastAccess2) {
		t.Error("Set() did not update last access time")
	}
}

// TestMemorySessionStore_Cleanup tests automatic cleanup of inactive groups
func TestMemorySessionStore_Cleanup(t *testing.T) {
	// Create store with short TTL for testing
	store := NewMemorySessionStore(WithCleanupTTL(50 * time.Millisecond))
	defer store.Close()

	// Add groups
	store.Set("group-1", Stores{"a": &testStore{value: 1}})
	store.Set("group-2", Stores{"b": &testStore{value: 2}})

	// Verify both exist
	if store.Get("group-1") == nil || store.Get("group-2") == nil {
		t.Fatal("Failed to set groups")
	}

	// Keep accessing group-1 (should not be cleaned up)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				store.Get("group-1") // Keep group-1 alive
			case <-done:
				return
			}
		}
	}()

	// Wait for cleanup to run (group-2 should be cleaned up, group-1 should remain)
	time.Sleep(100 * time.Millisecond)
	close(done)

	// Manually trigger cleanup to ensure it runs
	store.cleanup()

	// Verify group-1 still exists (kept alive by periodic access)
	if store.Get("group-1") == nil {
		t.Error("group-1 was cleaned up even though it was being accessed")
	}

	// Verify group-2 was cleaned up (not accessed)
	if store.Get("group-2") != nil {
		t.Error("group-2 was not cleaned up despite being inactive")
	}
}

// TestMemorySessionStore_Close tests graceful shutdown
func TestMemorySessionStore_Close(t *testing.T) {
	store := NewMemorySessionStore()

	// Add some data
	store.Set("group-1", Stores{"a": &testStore{value: 1}})

	// Close should not panic
	store.Close()

	// Verify cleanup goroutine stopped (stopCh should be closed)
	select {
	case <-store.stopCh:
		// Good, channel is closed
	case <-time.After(1 * time.Second):
		t.Error("Close() did not stop cleanup goroutine within timeout")
	}

	// Context should be cancelled
	select {
	case <-store.ctx.Done():
		// Good, context is cancelled
	default:
		t.Error("Close() did not cancel context")
	}
}

// TestMemorySessionStore_WithCleanupTTL tests custom TTL configuration
func TestMemorySessionStore_WithCleanupTTL(t *testing.T) {
	customTTL := 2 * time.Hour
	store := NewMemorySessionStore(WithCleanupTTL(customTTL))
	defer store.Close()

	if store.cleanupTTL != customTTL {
		t.Errorf("WithCleanupTTL() set TTL to %v, want %v", store.cleanupTTL, customTTL)
	}
}

// TestSessionStore_Interface verifies that MemorySessionStore implements SessionStore
func TestSessionStore_Interface(t *testing.T) {
	var _ SessionStore = (*MemorySessionStore)(nil)
}

// testStore is a simple Store implementation for testing
type testStore struct {
	value int
}

func (s *testStore) Change(ctx *ActionContext) error {
	s.value++
	return nil
}
