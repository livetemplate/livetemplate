package livetemplate

import (
	"context"
	"encoding/gob"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestStore is a simple store implementation for testing
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

// getTestRedisClient returns a Redis client for testing.
// Uses miniredis for in-memory Redis simulation if available,
// otherwise skips tests that require Redis.
func getTestRedisClient(t *testing.T) redis.UniversalClient {
	// For now, we'll use a real Redis connection for testing
	// In a CI environment, this should use miniredis or similar
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for tests to avoid conflicts
	})

	// Check if Redis is available
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test:", err)
	}

	// Flush the test database before each test
	client.FlushDB(ctx)

	return client
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
	store.Set("test-group-1", stores)

	// Get stores back
	retrieved := store.Get("test-group-1")
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
	retrieved := store.Get("non-existent")
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
	store.Set("test-group", stores)

	// Verify it exists
	if store.Get("test-group") == nil {
		t.Fatal("Store should exist after Set")
	}

	// Delete it
	store.Delete("test-group")

	// Verify it's gone
	if store.Get("test-group") != nil {
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

	store.Set("group-1", stores1)
	store.Set("group-2", stores2)
	store.Set("group-3", stores3)

	// List all groups
	groups := store.List()

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
	store.Set("test-group", stores)

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
	store.Get("test-group")

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

	store.Set("multi-group", stores)

	// Retrieve and verify
	retrieved := store.Get("multi-group")
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
	store.Set("empty-group", emptyStores)

	// Retrieve
	retrieved := store.Get("empty-group")
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
	store.Set("nil-group", nil)

	// Retrieve should return nil
	retrieved := store.Get("nil-group")
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
	store.Set("concurrent-group", stores)

	// Concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(val int) {
			// Set
			s := Stores{"test": &TestStore{Value: val}}
			store.Set("concurrent-group", s)

			// Get
			retrieved := store.Get("concurrent-group")
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
	final := store.Get("concurrent-group")
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
	groups := store.List()
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
	store.Set("update-group", initial)

	// Update with new values
	updated := Stores{"test": &TestStore{Value: 2, Message: "updated"}}
	store.Set("update-group", updated)

	// Retrieve and verify updated values
	retrieved := store.Get("update-group")
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
