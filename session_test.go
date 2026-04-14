package livetemplate

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// =============================================================================
// Memory Session Store Tests
// =============================================================================

// TestMemorySessionStore_SetAndGet tests basic set/get operations
func TestMemorySessionStore_SetAndGet(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Create test state
	state := &testStore{value: 42}

	// Set
	store.Set(context.Background(), "group-1", state)

	// Get
	retrieved := store.Get(context.Background(), "group-1")

	if retrieved == nil {
		t.Fatal("Get() returned nil, expected state")
	}

	counterStore, ok := retrieved.(*testStore)
	if !ok {
		t.Fatalf("Retrieved state is not *testStore, got %T", retrieved)
	}

	if counterStore.value != 42 {
		t.Errorf("Retrieved state value = %d, want 42", counterStore.value)
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

	// Create test state
	state := &testStore{value: 100}

	// Set and verify
	store.Set(context.Background(), "group-1", state)
	if store.Get(context.Background(), "group-1") == nil {
		t.Fatal("State was not set")
	}

	// Delete
	store.Delete(context.Background(), "group-1")

	// Verify deleted
	if got := store.Get(context.Background(), "group-1"); got != nil {
		t.Errorf("Get() after Delete() = %v, want nil", got)
	}
}

// TestMemorySessionStore_List tests listing all groups
func TestMemorySessionStore_List(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Set multiple groups
	store.Set(context.Background(), "group-1", &testStore{value: 1})
	store.Set(context.Background(), "group-2", &testStore{value: 2})
	store.Set(context.Background(), "group-3", &testStore{value: 3})

	// List
	groups := store.List(context.Background())

	if len(groups) != 3 {
		t.Errorf("List() returned %d groups, want 3", len(groups))
	}

	// Verify all expected groups are present
	expected := map[string]bool{"group-1": true, "group-2": true, "group-3": true}
	for _, g := range groups {
		if !expected[g] {
			t.Errorf("Unexpected group %q in list", g)
		}
		delete(expected, g)
	}

	if len(expected) > 0 {
		t.Errorf("Missing groups: %v", expected)
	}
}

// TestMemorySessionStore_Update tests updating existing state
func TestMemorySessionStore_Update(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	// Set initial state
	state1 := &testStore{value: 10}
	store.Set(context.Background(), "group-1", state1)

	// Update state
	state2 := &testStore{value: 20}
	store.Set(context.Background(), "group-1", state2)

	// Verify update
	retrieved := store.Get(context.Background(), "group-1")
	if retrieved == nil {
		t.Fatal("Get() returned nil")
	}

	counterStore, ok := retrieved.(*testStore)
	if !ok {
		t.Fatalf("Retrieved state is not *testStore, got %T", retrieved)
	}

	if counterStore.value != 20 {
		t.Errorf("Retrieved state value = %d, want 20", counterStore.value)
	}
}

// TestMemorySessionStore_ConcurrentAccess tests thread safety
func TestMemorySessionStore_ConcurrentAccess(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				groupID := "group-" + string(rune('a'+id))
				state := &testStore{value: id*1000 + j}
				store.Set(context.Background(), groupID, state)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				groupID := "group-" + string(rune('a'+id))
				_ = store.Get(context.Background(), groupID)
			}
		}(i)
	}

	wg.Wait()

	// Verify we have the expected groups
	groups := store.List(context.Background())
	if len(groups) != numGoroutines {
		t.Errorf("List() returned %d groups, want %d", len(groups), numGoroutines)
	}
}

// TestMemorySessionStore_Cleanup tests automatic cleanup of expired sessions
func TestMemorySessionStore_Cleanup(t *testing.T) {
	ttl := 100 * time.Millisecond
	cleanupInterval := 50 * time.Millisecond
	store := NewMemorySessionStore(WithCleanupTTL(ttl), WithCleanupInterval(cleanupInterval))
	defer store.Close()

	// Add sessions
	store.Set(context.Background(), "group-1", &testStore{value: 1})
	store.Set(context.Background(), "group-2", &testStore{value: 2})

	// Verify sessions exist
	if store.Get(context.Background(), "group-1") == nil {
		t.Fatal("group-1 should exist")
	}

	// Wait for TTL to expire plus two cleanup intervals (to ensure cleanup ran)
	time.Sleep(ttl + 2*cleanupInterval + 50*time.Millisecond)

	// Verify sessions are cleaned up
	groups := store.List(context.Background())
	if len(groups) != 0 {
		t.Errorf("Expected 0 groups after cleanup, got %d", len(groups))
	}
}

// TestMemorySessionStore_Close tests proper shutdown
func TestMemorySessionStore_Close(t *testing.T) {
	store := NewMemorySessionStore()

	// Add some data
	store.Set(context.Background(), "group-1", &testStore{value: 1})

	// Close should not panic
	store.Close()
}

// TestMemorySessionStore_WithCleanupTTL tests the WithCleanupTTL option
func TestMemorySessionStore_WithCleanupTTL(t *testing.T) {
	customTTL := 30 * time.Minute
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

// testStore is a simple state for testing (memory store tests)
type testStore struct {
	value int
}

// =============================================================================
// Redis Session Store Tests
// =============================================================================

// TestStore is a state for testing with gob serialization
type TestStore struct {
	Value   int    `json:"value"`
	Message string `json:"message"`
}

// Register TestStore with gob for serialization
func init() {
	gob.Register(&TestStore{})
	gob.Register(&testStore{})
}

// getTestRedisClient returns a Redis client for testing using testcontainers
func getTestRedisClient(t *testing.T) redis.UniversalClient {
	return testutil.GetTestRedisClient(t)
}

func TestRedisSessionStore_SetAndGet(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	store := NewRedisSessionStore(client, WithSessionTTL(1*time.Hour))

	// Store raw JSON bytes (as mount.go does with persist fields)
	stateJSON := []byte(`{"value":42,"message":"hello"}`)
	store.Set(context.Background(), "test-group-1", stateJSON)

	// Get returns raw JSON bytes
	retrieved := store.Get(context.Background(), "test-group-1")
	if retrieved == nil {
		t.Fatal("Expected state to be retrieved, got nil")
	}

	data, ok := retrieved.([]byte)
	if !ok {
		t.Fatalf("Expected []byte, got %T", retrieved)
	}

	var result TestStore
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if result.Value != 42 || result.Message != "hello" {
		t.Errorf("Unexpected values: Value=%d, Message=%s", result.Value, result.Message)
	}
}

func TestRedisSessionStore_GetNonExistent(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	store := NewRedisSessionStore(client)

	// Get non-existent group
	retrieved := store.Get(context.Background(), "non-existent")
	if retrieved != nil {
		t.Errorf("Expected nil for non-existent group, got %v", retrieved)
	}
}

func TestRedisSessionStore_Delete(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	store := NewRedisSessionStore(client)

	// Set state
	state := &TestStore{Value: 99, Message: "test"}
	store.Set(context.Background(), "test-group-delete", state)

	// Verify it exists
	if store.Get(context.Background(), "test-group-delete") == nil {
		t.Fatal("State was not set")
	}

	// Delete
	store.Delete(context.Background(), "test-group-delete")

	// Verify deleted
	if got := store.Get(context.Background(), "test-group-delete"); got != nil {
		t.Errorf("Get() after Delete() = %v, want nil", got)
	}
}

func TestRedisSessionStore_List(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	store := NewRedisSessionStore(client)

	// Clean up any existing test keys first
	ctx := context.Background()
	store.Delete(ctx, "test-list-1")
	store.Delete(ctx, "test-list-2")
	store.Delete(ctx, "test-list-3")

	// Set multiple groups
	store.Set(ctx, "test-list-1", &TestStore{Value: 1})
	store.Set(ctx, "test-list-2", &TestStore{Value: 2})
	store.Set(ctx, "test-list-3", &TestStore{Value: 3})

	// List all groups
	groups := store.List(ctx)

	// Should have at least our 3 test groups
	found := make(map[string]bool)
	for _, g := range groups {
		found[g] = true
	}

	expected := []string{"test-list-1", "test-list-2", "test-list-3"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("Missing expected group %q in list", e)
		}
	}

	// Cleanup
	store.Delete(ctx, "test-list-1")
	store.Delete(ctx, "test-list-2")
	store.Delete(ctx, "test-list-3")
}

func TestRedisSessionStore_ConcurrentAccess(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	store := NewRedisSessionStore(client)

	var wg sync.WaitGroup
	numGoroutines := 5
	numOperations := 20

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				groupID := "redis-concurrent-" + string(rune('a'+id))
				state := &TestStore{Value: id*1000 + j, Message: "test"}
				store.Set(context.Background(), groupID, state)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				groupID := "redis-concurrent-" + string(rune('a'+id))
				_ = store.Get(context.Background(), groupID)
			}
		}(i)
	}

	wg.Wait()

	// Cleanup
	for i := 0; i < numGoroutines; i++ {
		groupID := "redis-concurrent-" + string(rune('a'+i))
		store.Delete(context.Background(), groupID)
	}
}

func TestRedisSessionStore_Ping(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	store := NewRedisSessionStore(client)

	err := store.Ping()
	if err != nil {
		t.Errorf("Ping() failed: %v", err)
	}
}

func TestRedisSessionStore_Options(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	customTTL := 2 * time.Hour
	store := NewRedisSessionStore(client,
		WithSessionTTL(customTTL),
		WithMaxRetries(5),
		WithRetryDelay(100*time.Millisecond),
	)

	if store.ttl != customTTL {
		t.Errorf("WithSessionTTL() set TTL to %v, want %v", store.ttl, customTTL)
	}

	if store.maxRetries != 5 {
		t.Errorf("WithRetryPolicy() set maxRetries to %d, want 5", store.maxRetries)
	}
}

// =============================================================================
// Session Interface Tests
// =============================================================================

func TestSession_Interface(t *testing.T) {
	// Verify Session interface exists
	var _ = (Session)(nil)
}
