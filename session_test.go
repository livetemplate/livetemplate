package livetemplate

import (
	"context"
	"encoding/gob"
	"sync"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/session"
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

// =============================================================================
// SingleStoreSetter Interface Tests
// =============================================================================

// TestMemorySessionStore_SingleStoreSetter verifies MemorySessionStore implements SingleStoreSetter
func TestMemorySessionStore_SingleStoreSetter(t *testing.T) {
	var _ SingleStoreSetter = (*MemorySessionStore)(nil)
}

// TestRedisSessionStore_SingleStoreSetter verifies RedisSessionStore implements SingleStoreSetter
func TestRedisSessionStore_SingleStoreSetter(t *testing.T) {
	var _ SingleStoreSetter = (*RedisSessionStore)(nil)
}

// TestMemorySessionStore_SetStore tests SetStore no-op behavior
func TestMemorySessionStore_SetStore(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Set up initial stores
	testStores := Stores{
		"counter": &testStore{value: 10},
	}
	store.Set(context.Background(), "group-1", testStores)

	// Modify the store directly (simulating what happens after handleAction)
	counterStore := testStores["counter"].(*testStore)
	counterStore.value = 20

	// Call SetStore (should be a no-op for memory store, just update lastAccess)
	store.SetStore(context.Background(), "group-1", "counter", counterStore)

	// Verify the store still has the modified value
	retrieved := store.Get(context.Background(), "group-1")
	if retrieved == nil {
		t.Fatal("Expected stores to exist")
	}

	retrievedCounter := retrieved["counter"].(*testStore)
	if retrievedCounter.value != 20 {
		t.Errorf("Expected value=20, got %d", retrievedCounter.value)
	}
}

// TestRedisSessionStore_SetStore tests single store update
func TestRedisSessionStore_SetStore(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Set up initial stores with multiple entries
	stores := Stores{
		"counter": &TestStore{Value: 10, Message: "initial"},
		"timer":   &TestStore{Value: 100, Message: "timer"},
	}
	store.Set(context.Background(), "setstore-group", stores)

	// Update only the counter store
	updatedCounter := &TestStore{Value: 20, Message: "updated"}
	store.SetStore(context.Background(), "setstore-group", "counter", updatedCounter)

	// Retrieve and verify
	retrieved := store.Get(context.Background(), "setstore-group")
	if retrieved == nil {
		t.Fatal("Expected stores to exist")
	}

	// Counter should be updated
	counter := retrieved["counter"].(*TestStore)
	if counter.Value != 20 || counter.Message != "updated" {
		t.Errorf("Counter not updated: Value=%d, Message=%s", counter.Value, counter.Message)
	}

	// Timer should be unchanged
	timer := retrieved["timer"].(*TestStore)
	if timer.Value != 100 || timer.Message != "timer" {
		t.Errorf("Timer should be unchanged: Value=%d, Message=%s", timer.Value, timer.Message)
	}
}

// =============================================================================
// State Tag Serialization Tests (lvt:"state")
// =============================================================================

// StateTagTestProfile is a test state struct
type StateTagTestProfile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// StateTagTestController has state-tagged fields
type StateTagTestController struct {
	Profile *StateTagTestProfile `lvt:"state"`
	Counter int                  `lvt:"state"` // Non-pointer state field
	DBConn  string               // Not tagged - should not be persisted
}

func init() {
	// Register types for gob serialization
	gob.Register(&StateTagTestController{})
	gob.Register(&StateTagTestProfile{})
}

// TestRedisSessionStore_StateTagSerialization tests that only state-tagged fields are serialized
func TestRedisSessionStore_StateTagSerialization(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)

	// Create controller with state tags
	controller := &StateTagTestController{
		Profile: &StateTagTestProfile{Name: "Test User", Email: "test@example.com"},
		Counter: 42,
		DBConn:  "postgres://secret:password@localhost/db", // Should NOT be persisted
	}

	stores := Stores{"controller": controller}
	store.Set(context.Background(), "state-tag-group", stores)

	// Retrieve - the returned store will be a StateData wrapper
	// because state-tagged fields are serialized differently
	retrieved := store.Get(context.Background(), "state-tag-group")
	if retrieved == nil {
		t.Fatal("Expected stores to exist")
	}

	// Check if it's StateData (state-only serialization) or full store
	retrievedController := retrieved["controller"]
	if retrievedController == nil {
		t.Fatal("Expected controller store to exist")
	}

	// If it's StateData, we need to verify the raw bytes contain state
	if sd := GetStateData(retrievedController); sd != nil {
		// This is StateData - state-only serialization worked
		if len(sd.Raw) == 0 {
			t.Error("StateData.Raw is empty")
		}

		// Verify we can deserialize the state
		stateMap, err := DeserializeState(sd.Raw, &StateTagTestController{})
		if err != nil {
			t.Fatalf("Failed to deserialize state: %v", err)
		}

		// Verify Profile is in state
		if _, ok := stateMap["Profile"]; !ok {
			t.Error("Profile should be in state map")
		}

		// Verify Counter is in state
		if _, ok := stateMap["Counter"]; !ok {
			t.Error("Counter should be in state map")
		}
	}
}

// TestIsStateData tests the IsStateData helper
func TestIsStateData(t *testing.T) {
	sd := &StateData{Raw: []byte(`{"v":1,"fields":{}}`)}

	if !IsStateData(sd) {
		t.Error("IsStateData should return true for *StateData")
	}

	if IsStateData("not state data") {
		t.Error("IsStateData should return false for string")
	}

	if IsStateData(nil) {
		t.Error("IsStateData should return false for nil")
	}
}

// TestGetStateData tests the GetStateData helper
func TestGetStateData(t *testing.T) {
	sd := &StateData{Raw: []byte(`test`)}

	result := GetStateData(sd)
	if result != sd {
		t.Error("GetStateData should return the same StateData pointer")
	}

	if GetStateData("not state data") != nil {
		t.Error("GetStateData should return nil for non-StateData")
	}
}

// =============================================================================
// Session (Server-Initiated Actions) Tests
// =============================================================================

// SessionTestStore is a test store that implements SessionAware
type SessionTestStore struct {
	Counter   int
	session   Session
	onConnect func(ctx context.Context, session Session) error
	mu        sync.Mutex
}

func (s *SessionTestStore) Change(ctx *ActionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch ctx.Action {
	case "increment":
		s.Counter++
	case "decrement":
		s.Counter--
	case "tick":
		s.Counter++
	case "reset":
		s.Counter = 0
	}
	return nil
}

func (s *SessionTestStore) OnConnect(ctx context.Context, sess Session) error {
	s.session = sess
	if s.onConnect != nil {
		return s.onConnect(ctx, sess)
	}
	return nil
}

func (s *SessionTestStore) OnDisconnect() {
	s.session = nil
}

func (s *SessionTestStore) GetCounter() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Counter
}

// TestSession_TriggerAction tests basic TriggerAction functionality
func TestSession_TriggerAction(t *testing.T) {
	tmpl := Must(New("session-action-test"))
	if _, err := tmpl.Parse("<p>Counter: {{.Counter}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	store := &SessionTestStore{Counter: 0}
	handler := tmpl.Handle(store)

	// Create mock connection
	conn := createSessionTestConnection(t, "user1", "group1", tmpl)

	// Register connection
	h := handler.(*liveHandler)
	h.registry.Register(conn, 50)

	// Store the stores in the connection
	conn.Stores = Stores{"": store}

	// Create a session for this user
	sess := &liveSession{
		userID:  "user1",
		handler: h,
	}

	// Trigger action
	err := sess.TriggerAction("increment", nil)
	if err != nil {
		t.Errorf("TriggerAction failed: %v", err)
	}

	// Verify counter was incremented
	if store.GetCounter() != 1 {
		t.Errorf("Expected counter to be 1, got %d", store.GetCounter())
	}
}

// TestSession_TriggerActionMultipleConnections tests TriggerAction with multiple connections
func TestSession_TriggerActionMultipleConnections(t *testing.T) {
	tmpl := Must(New("session-multi-action-test"))
	if _, err := tmpl.Parse("<p>Counter: {{.Counter}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&SessionTestStore{Counter: 0})
	h := handler.(*liveHandler)

	// Create two stores for two connections (simulating cloned stores)
	store1 := &SessionTestStore{Counter: 0}
	store2 := &SessionTestStore{Counter: 0}

	// Create mock connections for same user
	conn1 := createSessionTestConnection(t, "user1", "group1", tmpl)
	conn1.Stores = Stores{"": store1}
	h.registry.Register(conn1, 50)

	conn2 := createSessionTestConnection(t, "user1", "group2", tmpl)
	conn2.Stores = Stores{"": store2}
	h.registry.Register(conn2, 50)

	// Create a session for this user
	sess := &liveSession{
		userID:  "user1",
		handler: h,
	}

	// Trigger action - should affect both connections
	err := sess.TriggerAction("increment", nil)
	if err != nil {
		t.Errorf("TriggerAction failed: %v", err)
	}

	// Both stores should be incremented
	if store1.GetCounter() != 1 {
		t.Errorf("Expected store1 counter to be 1, got %d", store1.GetCounter())
	}
	if store2.GetCounter() != 1 {
		t.Errorf("Expected store2 counter to be 1, got %d", store2.GetCounter())
	}
}

// TestSession_TriggerActionNoConnections tests TriggerAction with no connections
func TestSession_TriggerActionNoConnections(t *testing.T) {
	tmpl := Must(New("session-empty-action-test"))
	if _, err := tmpl.Parse("<p>Counter: {{.Counter}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&SessionTestStore{Counter: 0})
	h := handler.(*liveHandler)

	// Create session for user with no connections
	sess := &liveSession{
		userID:  "user-no-connections",
		handler: h,
	}

	// Should not error when no connections exist
	err := sess.TriggerAction("increment", nil)
	if err != nil {
		t.Errorf("TriggerAction with no connections should not error, got: %v", err)
	}
}

// TestSession_TriggerActionConcurrent tests concurrent TriggerAction calls
func TestSession_TriggerActionConcurrent(t *testing.T) {
	tmpl := Must(New("session-concurrent-action-test"))
	if _, err := tmpl.Parse("<p>Counter: {{.Counter}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&SessionTestStore{Counter: 0})
	h := handler.(*liveHandler)

	// Create store
	store := &SessionTestStore{Counter: 0}

	// Create connection
	conn := createSessionTestConnection(t, "user1", "group1", tmpl)
	conn.Stores = Stores{"": store}
	h.registry.Register(conn, 50)

	// Create session
	sess := &liveSession{
		userID:  "user1",
		handler: h,
	}

	// Concurrent triggers
	var wg sync.WaitGroup
	triggers := 10

	for i := 0; i < triggers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sess.TriggerAction("increment", nil); err != nil {
				t.Errorf("Concurrent TriggerAction failed: %v", err)
			}
		}()
	}

	wg.Wait()

	// Counter should be incremented `triggers` times
	if store.GetCounter() != triggers {
		t.Errorf("Expected counter to be %d, got %d", triggers, store.GetCounter())
	}
}

// TestSessionAware_Integration tests the full SessionAware integration
func TestSessionAware_Integration(t *testing.T) {
	tmpl := Must(New("session-aware-integration-test"))
	if _, err := tmpl.Parse("<p>Counter: {{.Counter}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Create a store that uses Session in OnConnect
	sessionReceived := make(chan Session, 1)
	store := &SessionTestStore{
		Counter: 0,
		onConnect: func(ctx context.Context, sess Session) error {
			sessionReceived <- sess
			return nil
		},
	}

	handler := tmpl.Handle(store)
	h := handler.(*liveHandler)

	// Create connection (simulating WebSocket connect)
	conn := createSessionTestConnection(t, "user1", "group1", tmpl)
	conn.Stores = Stores{"": store}
	h.registry.Register(conn, 50)

	// Create context for OnConnect
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create session
	sess := &liveSession{
		userID:  "user1",
		handler: h,
	}

	// Call OnConnect (simulating what handleWebSocket does)
	if err := store.OnConnect(ctx, sess); err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	// Verify session was received
	select {
	case received := <-sessionReceived:
		if received == nil {
			t.Error("Expected non-nil session in OnConnect")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for session in OnConnect")
	}

	// Use the stored session to trigger an action
	if store.session != nil {
		err := store.session.TriggerAction("increment", nil)
		if err != nil {
			t.Errorf("TriggerAction from stored session failed: %v", err)
		}

		// Verify counter was incremented
		if store.GetCounter() != 1 {
			t.Errorf("Expected counter to be 1 after TriggerAction, got %d", store.GetCounter())
		}
	} else {
		t.Error("Expected session to be stored in OnConnect")
	}
}

// TestSession_TriggerActionWithData tests TriggerAction with action data
func TestSession_TriggerActionWithData(t *testing.T) {
	tmpl := Must(New("session-data-action-test"))
	if _, err := tmpl.Parse("<p>Counter: {{.Counter}}</p>"); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	store := &SessionTestStore{Counter: 0}
	handler := tmpl.Handle(store)
	h := handler.(*liveHandler)

	// Create connection
	conn := createSessionTestConnection(t, "user1", "group1", tmpl)
	conn.Stores = Stores{"": store}
	h.registry.Register(conn, 50)

	// Create session
	sess := &liveSession{
		userID:  "user1",
		handler: h,
	}

	// Trigger action with data (data is available in ctx.Data in Change)
	err := sess.TriggerAction("increment", map[string]interface{}{
		"amount": 5,
	})
	if err != nil {
		t.Errorf("TriggerAction with data failed: %v", err)
	}

	// Verify action was triggered (basic store just increments by 1)
	if store.GetCounter() != 1 {
		t.Errorf("Expected counter to be 1, got %d", store.GetCounter())
	}
}

// TestSession_Interface verifies that liveSession implements Session
func TestSession_Interface(t *testing.T) {
	var _ Session = (*liveSession)(nil)
}

// TestSessionAware_Interface verifies that SessionTestStore implements SessionAware
func TestSessionAware_Interface(t *testing.T) {
	var _ SessionAware = (*SessionTestStore)(nil)
}

// createSessionTestConnection creates a mock connection for session testing
func createSessionTestConnection(t *testing.T, userID, groupID string, tmpl *Template) *session.Connection {
	t.Helper()

	// Clone template for this connection
	connTmpl, err := tmpl.Clone()
	if err != nil {
		t.Fatalf("Failed to clone template: %v", err)
	}

	return &session.Connection{
		Conn:     nil, // Nil Conn triggers test mode in sendUpdate
		UserID:   userID,
		GroupID:  groupID,
		Template: connTmpl,
		Stores:   make(Stores),
	}
}

// =============================================================================
// Redis v2 Integration Tests (P0: Roundtrip verification)
// =============================================================================

// RedisV2TestController has mixed state/non-state fields for v2 format testing
type RedisV2TestController struct {
	// State fields - should be serialized with "s:" prefix
	Title   string `json:"title" lvt:"state"`
	Counter int    `json:"counter" lvt:"state"`

	// Non-state fields - should NOT be serialized
	DBConn  string `json:"-"` // Simulates database connection
	Logger  string `json:"-"` // Simulates logger dependency
}

func init() {
	gob.Register(&RedisV2TestController{})
}

// TestRedisSessionStore_V2RoundtripStateOnly tests the complete roundtrip for state-only stores
// This verifies: Set → Redis v2 hash → Get → StateData wrapper → Hydration
func TestRedisSessionStore_V2RoundtripStateOnly(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)
	ctx := context.Background()
	groupID := "v2-roundtrip-state-test"

	// Create controller with state tags
	controller := &RedisV2TestController{
		Title:   "Test Title",
		Counter: 42,
		DBConn:  "postgres://secret@localhost/db", // Should NOT be persisted
		Logger:  "zap:production",                 // Should NOT be persisted
	}

	stores := Stores{"main": controller}
	store.Set(ctx, groupID, stores)

	// Verify the data is stored in Redis with v2 hash format
	key := sessionKeyPrefix + groupID
	hashData, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get hash data: %v", err)
	}

	// Check metadata field exists and is v2
	metaJSON, ok := hashData[metaField]
	if !ok {
		t.Fatal("Expected _meta field in hash")
	}
	if !containsString(metaJSON, `"version":"2"`) {
		t.Errorf("Expected v2 metadata, got: %s", metaJSON)
	}

	// Check main store field exists and has "s:" prefix (state-only format)
	mainData, ok := hashData["main"]
	if !ok {
		t.Fatal("Expected 'main' field in hash")
	}
	if !hasPrefix(mainData, "s:") {
		t.Errorf("Expected 's:' prefix for state-only format, got prefix: %s", mainData[:min(10, len(mainData))])
	}

	// Retrieve stores
	retrieved := store.Get(ctx, groupID)
	if retrieved == nil {
		t.Fatal("Expected stores to be retrieved")
	}

	// Should be StateData wrapper (for state-only serialization)
	mainStore := retrieved["main"]
	if mainStore == nil {
		t.Fatal("Expected main store to exist")
	}

	sd := GetStateData(mainStore)
	if sd == nil {
		t.Fatalf("Expected StateData wrapper, got %T", mainStore)
	}

	// Verify state can be deserialized
	stateMap, err := DeserializeState(sd.Raw, &RedisV2TestController{})
	if err != nil {
		t.Fatalf("Failed to deserialize state: %v", err)
	}

	// Verify state fields are present
	if stateMap["Title"] != "Test Title" {
		t.Errorf("Expected Title='Test Title', got %v", stateMap["Title"])
	}
	// Counter could be int or float64 depending on deserialization path
	counterVal := stateMap["Counter"]
	var counterInt int
	switch v := counterVal.(type) {
	case float64:
		counterInt = int(v)
	case int:
		counterInt = v
	default:
		t.Fatalf("Unexpected Counter type: %T", counterVal)
	}
	if counterInt != 42 {
		t.Errorf("Expected Counter=42, got %d", counterInt)
	}

	// Verify non-state fields are NOT in state map
	if _, ok := stateMap["DBConn"]; ok {
		t.Error("DBConn should NOT be in state map (not tagged with lvt:state)")
	}
	if _, ok := stateMap["Logger"]; ok {
		t.Error("Logger should NOT be in state map (not tagged with lvt:state)")
	}
}

// TestRedisSessionStore_V2RoundtripGobFormat tests roundtrip for stores without state tags
func TestRedisSessionStore_V2RoundtripGobFormat(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)
	ctx := context.Background()
	groupID := "v2-roundtrip-gob-test"

	// Create store WITHOUT state tags (should use Gob format)
	testStore := &TestStore{Value: 100, Message: "gob test"}

	stores := Stores{"counter": testStore}
	store.Set(ctx, groupID, stores)

	// Verify the data is stored with "g:" prefix (Gob format)
	key := sessionKeyPrefix + groupID
	hashData, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get hash data: %v", err)
	}

	counterData, ok := hashData["counter"]
	if !ok {
		t.Fatal("Expected 'counter' field in hash")
	}
	if !hasPrefix(counterData, "g:") {
		t.Errorf("Expected 'g:' prefix for Gob format, got prefix: %s", counterData[:min(10, len(counterData))])
	}

	// Retrieve stores
	retrieved := store.Get(ctx, groupID)
	if retrieved == nil {
		t.Fatal("Expected stores to be retrieved")
	}

	// Should be *TestStore (not StateData wrapper, since no state tags)
	counterStore, ok := retrieved["counter"].(*TestStore)
	if !ok {
		t.Fatalf("Expected *TestStore, got %T", retrieved["counter"])
	}

	if counterStore.Value != 100 {
		t.Errorf("Expected Value=100, got %d", counterStore.Value)
	}
	if counterStore.Message != "gob test" {
		t.Errorf("Expected Message='gob test', got '%s'", counterStore.Message)
	}
}

// TestRedisSessionStore_V2SetStoreUpdatesOnly tests that SetStore updates only one field in the hash
func TestRedisSessionStore_V2SetStoreUpdatesOnly(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisSessionStore(client)
	ctx := context.Background()
	groupID := "v2-setstore-test"

	// Create initial stores
	stores := Stores{
		"counter": &TestStore{Value: 10, Message: "first"},
		"timer":   &TestStore{Value: 100, Message: "timer"},
	}
	store.Set(ctx, groupID, stores)

	// Update only counter
	updatedCounter := &TestStore{Value: 20, Message: "updated"}
	store.SetStore(ctx, groupID, "counter", updatedCounter)

	// Verify Redis hash
	key := sessionKeyPrefix + groupID
	hashData, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get hash data: %v", err)
	}

	// Should have 3 fields: _meta, counter, timer
	if len(hashData) != 3 {
		t.Errorf("Expected 3 hash fields, got %d", len(hashData))
	}

	// Retrieve and verify
	retrieved := store.Get(ctx, groupID)
	if retrieved == nil {
		t.Fatal("Expected stores")
	}

	counter := retrieved["counter"].(*TestStore)
	if counter.Value != 20 || counter.Message != "updated" {
		t.Errorf("Counter not updated correctly: Value=%d, Message=%s", counter.Value, counter.Message)
	}

	timer := retrieved["timer"].(*TestStore)
	if timer.Value != 100 || timer.Message != "timer" {
		t.Errorf("Timer should be unchanged: Value=%d, Message=%s", timer.Value, timer.Message)
	}
}

// Helper functions for tests
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
