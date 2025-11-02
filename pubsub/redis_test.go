package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// getTestRedisClient returns a Redis client for testing.
func getTestRedisClient(t *testing.T) redis.UniversalClient {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for tests
	})

	// Check if Redis is available
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test:", err)
	}

	// Flush the test database
	client.FlushDB(ctx)

	return client
}

func TestRedisBroadcaster_PublishGlobal(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)
	defer broadcaster.Close()

	payload := []byte(`{"action": "test"}`)
	err := broadcaster.PublishGlobal(payload)
	if err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}
}

func TestRedisBroadcaster_PublishToGroup(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)
	defer broadcaster.Close()

	payload := []byte(`{"action": "test"}`)
	err := broadcaster.PublishToGroup("group-123", payload)
	if err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}
}

func TestRedisBroadcaster_PublishToUser(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)
	defer broadcaster.Close()

	payload := []byte(`{"action": "test"}`)
	err := broadcaster.PublishToUser("user-123", payload)
	if err != nil {
		t.Fatalf("PublishToUser failed: %v", err)
	}
}

func TestRedisBroadcaster_SubscribeAndReceive(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster1 := NewRedisBroadcaster(client)
	defer broadcaster1.Close()

	broadcaster2 := NewRedisBroadcaster(client)
	defer broadcaster2.Close()

	// Set up receiver
	var received sync.WaitGroup
	received.Add(1)

	var receivedMsg *BroadcastMessage
	handler := func(msg *BroadcastMessage) error {
		receivedMsg = msg
		received.Done()
		return nil
	}

	// Start subscriber
	go func() {
		if err := broadcaster2.Subscribe(handler); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	// Give subscriber time to start
	time.Sleep(100 * time.Millisecond)

	// Publish from broadcaster1
	payload := []byte(`{"test": "data"}`)
	if err := broadcaster1.PublishGlobal(payload); err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}

	// Wait for message with timeout
	done := make(chan bool)
	go func() {
		received.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for broadcast message")
	}

	// Verify message
	if receivedMsg == nil {
		t.Fatal("No message received")
	}

	if receivedMsg.Type != "broadcast" {
		t.Errorf("Expected Type='broadcast', got '%s'", receivedMsg.Type)
	}

	if receivedMsg.Scope != ScopeGlobal {
		t.Errorf("Expected Scope=ScopeGlobal, got '%s'", receivedMsg.Scope)
	}

	if string(receivedMsg.Payload) != string(payload) {
		t.Errorf("Expected Payload=%s, got %s", string(payload), string(receivedMsg.Payload))
	}

	if receivedMsg.InstanceID == "" {
		t.Error("Expected InstanceID to be set")
	}
}

func TestRedisBroadcaster_LocalOptimization(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)
	defer broadcaster.Close()

	// Set up receiver
	var receivedCount int
	handler := func(msg *BroadcastMessage) error {
		receivedCount++
		return nil
	}

	// Start subscriber
	go func() {
		if err := broadcaster.Subscribe(handler); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	// Give subscriber time to start
	time.Sleep(100 * time.Millisecond)

	// Publish from same instance
	payload := []byte(`{"test": "data"}`)
	if err := broadcaster.PublishGlobal(payload); err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Should not receive own message (local-first optimization)
	if receivedCount != 0 {
		t.Errorf("Expected receivedCount=0 (local optimization), got %d", receivedCount)
	}
}

func TestRedisBroadcaster_GroupBroadcast(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster1 := NewRedisBroadcaster(client)
	defer broadcaster1.Close()

	broadcaster2 := NewRedisBroadcaster(client)
	defer broadcaster2.Close()

	// Set up receiver
	var received sync.WaitGroup
	received.Add(1)

	var receivedMsg *BroadcastMessage
	handler := func(msg *BroadcastMessage) error {
		receivedMsg = msg
		received.Done()
		return nil
	}

	// Start subscriber
	go func() {
		if err := broadcaster2.Subscribe(handler); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	// Give subscriber time to start
	time.Sleep(100 * time.Millisecond)

	// Subscribe to specific group
	if err := broadcaster2.SubscribeToGroup("group-123"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}

	// Give subscription time to register
	time.Sleep(100 * time.Millisecond)

	// Publish to group
	payload := []byte(`{"test": "group-data"}`)
	if err := broadcaster1.PublishToGroup("group-123", payload); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}

	// Wait for message with timeout
	done := make(chan bool)
	go func() {
		received.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for group broadcast message")
	}

	// Verify message
	if receivedMsg == nil {
		t.Fatal("No message received")
	}

	if receivedMsg.Scope != ScopeGroup {
		t.Errorf("Expected Scope=ScopeGroup, got '%s'", receivedMsg.Scope)
	}

	if receivedMsg.GroupID != "group-123" {
		t.Errorf("Expected GroupID='group-123', got '%s'", receivedMsg.GroupID)
	}
}

func TestRedisBroadcaster_UserBroadcast(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster1 := NewRedisBroadcaster(client)
	defer broadcaster1.Close()

	broadcaster2 := NewRedisBroadcaster(client)
	defer broadcaster2.Close()

	// Set up receiver
	var received sync.WaitGroup
	received.Add(1)

	var receivedMsg *BroadcastMessage
	handler := func(msg *BroadcastMessage) error {
		receivedMsg = msg
		received.Done()
		return nil
	}

	// Start subscriber
	go func() {
		if err := broadcaster2.Subscribe(handler); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	// Give subscriber time to start
	time.Sleep(100 * time.Millisecond)

	// Subscribe to specific user
	if err := broadcaster2.SubscribeToUser("user-456"); err != nil {
		t.Fatalf("SubscribeToUser failed: %v", err)
	}

	// Give subscription time to register
	time.Sleep(100 * time.Millisecond)

	// Publish to user
	payload := []byte(`{"test": "user-data"}`)
	if err := broadcaster1.PublishToUser("user-456", payload); err != nil {
		t.Fatalf("PublishToUser failed: %v", err)
	}

	// Wait for message with timeout
	done := make(chan bool)
	go func() {
		received.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for user broadcast message")
	}

	// Verify message
	if receivedMsg == nil {
		t.Fatal("No message received")
	}

	if receivedMsg.Scope != ScopeUser {
		t.Errorf("Expected Scope=ScopeUser, got '%s'", receivedMsg.Scope)
	}

	if receivedMsg.UserID != "user-456" {
		t.Errorf("Expected UserID='user-456', got '%s'", receivedMsg.UserID)
	}
}

func TestRedisBroadcaster_MultipleSubscribers(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster1 := NewRedisBroadcaster(client)
	defer broadcaster1.Close()

	broadcaster2 := NewRedisBroadcaster(client)
	defer broadcaster2.Close()

	broadcaster3 := NewRedisBroadcaster(client)
	defer broadcaster3.Close()

	// Set up receivers
	var received sync.WaitGroup
	received.Add(2)

	handler := func(msg *BroadcastMessage) error {
		received.Done()
		return nil
	}

	// Start subscribers
	go func() {
		_ = broadcaster2.Subscribe(handler)
	}()
	go func() {
		_ = broadcaster3.Subscribe(handler)
	}()

	// Give subscribers time to start
	time.Sleep(100 * time.Millisecond)

	// Publish from broadcaster1
	payload := []byte(`{"test": "multi"}`)
	if err := broadcaster1.PublishGlobal(payload); err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}

	// Wait for messages with timeout
	done := make(chan bool)
	go func() {
		received.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - both subscribers received the message
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for broadcast to multiple subscribers")
	}
}

func TestRedisBroadcaster_Close(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)

	// Subscribe first
	go func() {
		_ = broadcaster.Subscribe(func(msg *BroadcastMessage) error {
			return nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	// Close broadcaster
	if err := broadcaster.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Try to publish after close (should fail)
	err := broadcaster.PublishGlobal([]byte("test"))
	if err == nil {
		t.Error("Expected error when publishing after close")
	}
}

func TestRedisBroadcaster_EmptyGroupID(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)
	defer broadcaster.Close()

	err := broadcaster.PublishToGroup("", []byte("test"))
	if err == nil {
		t.Error("Expected error for empty groupID")
	}
}

func TestRedisBroadcaster_EmptyUserID(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	broadcaster := NewRedisBroadcaster(client)
	defer broadcaster.Close()

	err := broadcaster.PublishToUser("", []byte("test"))
	if err == nil {
		t.Error("Expected error for empty userID")
	}
}
