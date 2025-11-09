package livetemplate

import (
	"context"
	"encoding/gob"
	"sync"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/testutil"
	"github.com/redis/go-redis/v9"
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
	store.Set(context.Background(), "group-1", stores)

	// Get
	retrieved := store.Get(context.Background(), "group-1")

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

	retrieved := store.Get(context.Background(), "non-existent")

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
	store.Set(context.Background(), "group-1", stores)
	if store.Get(context.Background(), "group-1") == nil {
		t.Fatal("Failed to set group")
	}

	// Delete
	store.Delete(context.Background(), "group-1")

	// Verify deleted
	if store.Get(context.Background(), "group-1") != nil {
		t.Error("Get() after Delete() returned non-nil, expected nil")
	}
}

// TestMemorySessionStore_List tests listing all group IDs
func TestMemorySessionStore_List(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Initially empty
	list := store.List(context.Background())
	if len(list) != 0 {
		t.Errorf("List() returned %d groups, want 0", len(list))
	}

	// Add groups
	store.Set(context.Background(), "group-1", Stores{"a": &testStore{value: 1}})
	store.Set(context.Background(), "group-2", Stores{"b": &testStore{value: 2}})
	store.Set(context.Background(), "group-3", Stores{"c": &testStore{value: 3}})

	// List should have all 3
	list = store.List(context.Background())
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
	store.Set(context.Background(), "group-1", stores1)

	// Update with new stores
	stores2 := Stores{
		"counter": &testStore{value: 2},
	}
	store.Set(context.Background(), "group-1", stores2)

	// Verify updated value
	retrieved := store.Get(context.Background(), "group-1")
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
				store.Set(context.Background(), groupID, stores)
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
				_ = store.Get(context.Background(), groupID)
			}
		}(i)
	}

	// Concurrent list operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = store.List(context.Background())
			}
		}()
	}

	// Wait for all goroutines
	wg.Wait()

	// Verify store is still functional
	testStores := Stores{"test": &testStore{value: 999}}
	store.Set(context.Background(), "test-group", testStores)

	retrieved := store.Get(context.Background(), "test-group")
	if retrieved == nil {
		t.Error("Store corrupted after concurrent access")
	}
}

// TestMemorySessionStore_LastAccessTracking tests that Get and Set update last access time
func TestMemorySessionStore_LastAccessTracking(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	stores := Stores{"counter": &testStore{value: 1}}
	store.Set(context.Background(), "group-1", stores)

	// Get initial last access time
	store.mu.RLock()
	lastAccess1 := store.lastAccess["group-1"]
	store.mu.RUnlock()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Get should update last access
	store.Get(context.Background(), "group-1")

	store.mu.RLock()
	lastAccess2 := store.lastAccess["group-1"]
	store.mu.RUnlock()

	if !lastAccess2.After(lastAccess1) {
		t.Error("Get() did not update last access time")
	}

	// Wait a bit more
	time.Sleep(10 * time.Millisecond)

	// Set should also update last access
	store.Set(context.Background(), "group-1", stores)

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
	store.Set(context.Background(), "group-1", Stores{"a": &testStore{value: 1}})
	store.Set(context.Background(), "group-2", Stores{"b": &testStore{value: 2}})

	// Verify both exist
	if store.Get(context.Background(), "group-1") == nil || store.Get(context.Background(), "group-2") == nil {
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
				store.Get(context.Background(), "group-1") // Keep group-1 alive
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
	if store.Get(context.Background(), "group-1") == nil {
		t.Error("group-1 was cleaned up even though it was being accessed")
	}

	// Verify group-2 was cleaned up (not accessed)
	if store.Get(context.Background(), "group-2") != nil {
		t.Error("group-2 was not cleaned up despite being inactive")
	}
}

// TestMemorySessionStore_Close tests graceful shutdown
func TestMemorySessionStore_Close(t *testing.T) {
	store := NewMemorySessionStore()

	// Add some data
	store.Set(context.Background(), "group-1", Stores{"a": &testStore{value: 1}})

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

// testStore is a simple Store implementation for testing (memory store tests)
type testStore struct {
	value int
}

func (s *testStore) Change(ctx *ActionContext) error {
	s.value++
	return nil
}

// =============================================================================
// Redis Session Store Tests
// =============================================================================

// TestStore is a Store implementation for testing with gob serialization
type TestStore struct {
	Value   int
	Message string
}

func (t *TestStore) Change(ctx *ActionContext) error {
	t.Value++
	return nil
}

// Register TestStore with gob for serialization
func init() {
	gob.Register(&TestStore{})
}

// getTestRedisClient returns a Redis client for testing using testcontainers
func getTestRedisClient(t *testing.T) redis.UniversalClient {
	return testutil.GetTestRedisClient(t)
}

func TestRedisSessionStore_SetAndGet(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client, WithSessionTTL(1*time.Hour))

	// Create test stores
	stores := Stores{
		"counter": &TestStore{Value: 42, Message: "hello"},
	}

	// Set stores
	store.Set(context.Background(), "test-group-1", stores)

	// Get stores back
	retrieved := store.Get(context.Background(), "test-group-1")
	if retrieved == nil {
		t.Fatal("Expected stores to be retrieved, got nil")
	}

	// Verify the store was correctly serialized and deserialized
	counterStore, ok := retrieved["counter"].(*TestStore)
	if !ok {
		t.Fatalf("Expected *TestStore, got %T", retrieved["counter"])
	}

	if counterStore.Value != 42 {
		t.Errorf("Expected Value=42, got %d", counterStore.Value)
	}

	if counterStore.Message != "hello" {
		t.Errorf("Expected Message='hello', got '%s'", counterStore.Message)
	}
}

func TestRedisSessionStore_GetNonExistent(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Get non-existent group
	retrieved := store.Get(context.Background(), "non-existent")
	if retrieved != nil {
		t.Errorf("Expected nil for non-existent group, got %v", retrieved)
	}
}

func TestRedisSessionStore_Delete(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Create and set stores
	stores := Stores{
		"test": &TestStore{Value: 100},
	}
	store.Set(context.Background(), "test-group", stores)

	// Verify it exists
	if store.Get(context.Background(), "test-group") == nil {
		t.Fatal("Store should exist after Set")
	}

	// Delete it
	store.Delete(context.Background(), "test-group")

	// Verify it's gone
	if store.Get(context.Background(), "test-group") != nil {
		t.Error("Store should be nil after Delete")
	}
}

func TestRedisSessionStore_List(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Create multiple groups
	stores1 := Stores{"test": &TestStore{Value: 1}}
	stores2 := Stores{"test": &TestStore{Value: 2}}
	stores3 := Stores{"test": &TestStore{Value: 3}}

	store.Set(context.Background(), "group-1", stores1)
	store.Set(context.Background(), "group-2", stores2)
	store.Set(context.Background(), "group-3", stores3)

	// List all groups
	groups := store.List(context.Background())

	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Verify groups are present
	groupMap := make(map[string]bool)
	for _, group := range groups {
		groupMap[group] = true
	}

	if !groupMap["group-1"] || !groupMap["group-2"] || !groupMap["group-3"] {
		t.Errorf("Expected groups [group-1, group-2, group-3], got %v", groups)
	}
}

func TestRedisSessionStore_TTLRefresh(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	// Use short TTL for testing
	store := NewRedisSessionStore(client, WithSessionTTL(2*time.Second))

	stores := Stores{"test": &TestStore{Value: 1}}
	store.Set(context.Background(), "test-group", stores)

	// Get the initial TTL
	ctx := context.Background()
	key := sessionKeyPrefix + "test-group"
	initialTTL := client.TTL(ctx, key).Val()

	if initialTTL <= 0 {
		t.Fatal("Expected positive TTL after Set")
	}

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Access the store (should refresh TTL)
	store.Get(context.Background(), "test-group")

	// Wait for async refresh
	time.Sleep(100 * time.Millisecond)

	// Get the new TTL
	newTTL := client.TTL(ctx, key).Val()

	// New TTL should be close to the original (refreshed)
	// Allow some margin for timing
	if newTTL < initialTTL-1*time.Second {
		t.Errorf("Expected TTL to be refreshed, initial=%v, new=%v", initialTTL, newTTL)
	}
}

func TestRedisSessionStore_MultipleStores(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Create stores with multiple entries
	stores := Stores{
		"counter": &TestStore{Value: 1, Message: "first"},
		"timer":   &TestStore{Value: 2, Message: "second"},
		"config":  &TestStore{Value: 3, Message: "third"},
	}

	store.Set(context.Background(), "multi-group", stores)

	// Retrieve and verify
	retrieved := store.Get(context.Background(), "multi-group")
	if retrieved == nil {
		t.Fatal("Expected stores to be retrieved")
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 stores, got %d", len(retrieved))
	}

	// Verify each store
	for name, expected := range stores {
		actual, ok := retrieved[name].(*TestStore)
		if !ok {
			t.Errorf("Store %s has wrong type", name)
			continue
		}

		expectedStore := expected.(*TestStore)
		if actual.Value != expectedStore.Value {
			t.Errorf("Store %s: expected Value=%d, got %d", name, expectedStore.Value, actual.Value)
		}

		if actual.Message != expectedStore.Message {
			t.Errorf("Store %s: expected Message='%s', got '%s'", name, expectedStore.Message, actual.Message)
		}
	}
}

func TestRedisSessionStore_EmptyStores(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Set empty stores
	emptyStores := Stores{}
	store.Set(context.Background(), "empty-group", emptyStores)

	// Retrieve
	retrieved := store.Get(context.Background(), "empty-group")
	if retrieved == nil {
		t.Error("Expected empty stores map, got nil")
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected 0 stores, got %d", len(retrieved))
	}
}

func TestRedisSessionStore_NilStores(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Set nil stores (should handle gracefully)
	store.Set(context.Background(), "nil-group", nil)

	// Retrieve should return nil
	retrieved := store.Get(context.Background(), "nil-group")
	if retrieved != nil {
		t.Errorf("Expected nil for nil stores, got %v", retrieved)
	}
}

func TestRedisSessionStore_Ping(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Ping should succeed
	if err := store.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestRedisSessionStore_ConcurrentAccess(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Create initial stores
	stores := Stores{"test": &TestStore{Value: 0}}
	store.Set(context.Background(), "concurrent-group", stores)

	// Concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(val int) {
			// Set
			s := Stores{"test": &TestStore{Value: val}}
			store.Set(context.Background(), "concurrent-group", s)

			// Get
			retrieved := store.Get(context.Background(), "concurrent-group")
			if retrieved == nil {
				t.Error("Expected stores, got nil")
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Final verification
	final := store.Get(context.Background(), "concurrent-group")
	if final == nil {
		t.Fatal("Expected final stores to exist")
	}
}

func TestRedisSessionStore_Options(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	// Test with custom options
	store := NewRedisSessionStore(client,
		WithSessionTTL(30*time.Minute),
		WithMaxRetries(5),
		WithRetryDelay(200*time.Millisecond),
	)

	if store.ttl != 30*time.Minute {
		t.Errorf("Expected TTL=30m, got %v", store.ttl)
	}

	if store.maxRetries != 5 {
		t.Errorf("Expected maxRetries=5, got %d", store.maxRetries)
	}

	if store.retryDelay != 200*time.Millisecond {
		t.Errorf("Expected retryDelay=200ms, got %v", store.retryDelay)
	}
}

func TestRedisSessionStore_ListEmpty(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// List should return empty slice when no groups exist
	groups := store.List(context.Background())
	if groups == nil {
		t.Error("Expected empty slice, got nil")
	}

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}
}

func TestRedisSessionStore_UpdateExisting(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Set initial stores
	initial := Stores{"test": &TestStore{Value: 1, Message: "initial"}}
	store.Set(context.Background(), "update-group", initial)

	// Update with new values
	updated := Stores{"test": &TestStore{Value: 2, Message: "updated"}}
	store.Set(context.Background(), "update-group", updated)

	// Retrieve and verify updated values
	retrieved := store.Get(context.Background(), "update-group")
	if retrieved == nil {
		t.Fatal("Expected stores after update")
	}

	testStore := retrieved["test"].(*TestStore)
	if testStore.Value != 2 {
		t.Errorf("Expected Value=2, got %d", testStore.Value)
	}

	if testStore.Message != "updated" {
		t.Errorf("Expected Message='updated', got '%s'", testStore.Message)
	}
}
