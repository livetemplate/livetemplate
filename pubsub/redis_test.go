package pubsub

import (
	"sync"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// getTestRedisClient returns a Redis client for testing using testcontainers.
func getTestRedisClient(t *testing.T) redis.UniversalClient {
	return testutil.GetTestRedisClient(t)
}

func TestRedisBroadcaster_PublishGlobal(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	payload := []byte(`{"action": "test"}`)
	err := broadcaster.PublishGlobal(payload)
	if err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}
}

func TestRedisBroadcaster_PublishToGroup(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	payload := []byte(`{"action": "test"}`)
	err := broadcaster.PublishToGroup("group-123", payload)
	if err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}
}

func TestRedisBroadcaster_PublishToUser(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	payload := []byte(`{"action": "test"}`)
	err := broadcaster.PublishToUser("user-123", payload)
	if err != nil {
		t.Fatalf("PublishToUser failed: %v", err)
	}
}

func TestRedisBroadcaster_SubscribeAndReceive(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster1 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster1.Close(); err != nil {
			t.Errorf("Failed to close broadcaster1: %v", err)
		}
	}()

	broadcaster2 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster2.Close(); err != nil {
			t.Errorf("Failed to close broadcaster2: %v", err)
		}
	}()

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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster1 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster1.Close(); err != nil {
			t.Errorf("Failed to close broadcaster1: %v", err)
		}
	}()

	broadcaster2 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster2.Close(); err != nil {
			t.Errorf("Failed to close broadcaster2: %v", err)
		}
	}()

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

	// Publish to group (arrives via global channel, no dynamic subscription needed)
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster1 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster1.Close(); err != nil {
			t.Errorf("Failed to close broadcaster1: %v", err)
		}
	}()

	broadcaster2 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster2.Close(); err != nil {
			t.Errorf("Failed to close broadcaster2: %v", err)
		}
	}()

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

	// Publish to user (arrives via global channel, no dynamic subscription needed)
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster1 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster1.Close(); err != nil {
			t.Errorf("Failed to close broadcaster1: %v", err)
		}
	}()

	broadcaster2 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster2.Close(); err != nil {
			t.Errorf("Failed to close broadcaster2: %v", err)
		}
	}()

	broadcaster3 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster3.Close(); err != nil {
			t.Errorf("Failed to close broadcaster3: %v", err)
		}
	}()

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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	err := broadcaster.PublishToGroup("", []byte("test"))
	if err == nil {
		t.Error("Expected error for empty groupID")
	}
}

func TestRedisBroadcaster_EmptyUserID(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	err := broadcaster.PublishToUser("", []byte("test"))
	if err == nil {
		t.Error("Expected error for empty userID")
	}
}

func TestRedisBroadcaster_ServerActionViaSingleChannel(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster1 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster1.Close(); err != nil {
			t.Errorf("Failed to close broadcaster1: %v", err)
		}
	}()

	broadcaster2 := NewRedisBroadcaster(client)
	defer func() {
		if err := broadcaster2.Close(); err != nil {
			t.Errorf("Failed to close broadcaster2: %v", err)
		}
	}()

	var received sync.WaitGroup
	received.Add(1)

	var receivedMsg *ServerActionMessage
	if err := broadcaster2.SubscribeServerActions(func(msg *ServerActionMessage) error {
		receivedMsg = msg
		received.Done()
		return nil
	}); err != nil {
		t.Fatalf("SubscribeServerActions failed: %v", err)
	}

	go func() {
		if err := broadcaster2.Subscribe(func(msg *BroadcastMessage) error {
			return nil
		}); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	if err := broadcaster1.PublishServerAction("user-789", "refresh", map[string]interface{}{"key": "value"}); err != nil {
		t.Fatalf("PublishServerAction failed: %v", err)
	}

	done := make(chan bool)
	go func() {
		received.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for server action message")
	}

	if receivedMsg == nil {
		t.Fatal("No server action message received")
	}

	if receivedMsg.UserID != "user-789" {
		t.Errorf("Expected UserID='user-789', got '%s'", receivedMsg.UserID)
	}

	if receivedMsg.Action != "refresh" {
		t.Errorf("Expected Action='refresh', got '%s'", receivedMsg.Action)
	}
}

func TestRedisBroadcaster_ReconnectPreservesAllMessageTypes(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	publisher := NewRedisBroadcaster(client)
	defer func() {
		if err := publisher.Close(); err != nil {
			t.Errorf("Failed to close publisher: %v", err)
		}
	}()

	subscriber := NewRedisBroadcaster(client, WithReconnectDelay(10*time.Millisecond))
	defer func() {
		if err := subscriber.Close(); err != nil {
			t.Errorf("Failed to close subscriber: %v", err)
		}
	}()

	var mu sync.Mutex
	var broadcastMsgs []*BroadcastMessage
	var serverActionMsgs []*ServerActionMessage

	if err := subscriber.SubscribeServerActions(func(msg *ServerActionMessage) error {
		mu.Lock()
		serverActionMsgs = append(serverActionMsgs, msg)
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("SubscribeServerActions failed: %v", err)
	}

	go func() {
		if err := subscriber.Subscribe(func(msg *BroadcastMessage) error {
			mu.Lock()
			broadcastMsgs = append(broadcastMsgs, msg)
			mu.Unlock()
			return nil
		}); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Force a reconnect by closing the internal pubsub
	subscriber.mu.Lock()
	if subscriber.pubsub != nil {
		_ = subscriber.pubsub.Close()
	}
	subscriber.mu.Unlock()

	// Wait for reconnect to complete
	time.Sleep(200 * time.Millisecond)

	// Publish all message types after reconnect
	if err := publisher.PublishGlobal([]byte(`{"test": "global"}`)); err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}
	if err := publisher.PublishToGroup("g1", []byte(`{"test": "group"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}
	if err := publisher.PublishToUser("u1", []byte(`{"test": "user"}`)); err != nil {
		t.Fatalf("PublishToUser failed: %v", err)
	}
	if err := publisher.PublishServerAction("u1", "tick", nil); err != nil {
		t.Fatalf("PublishServerAction failed: %v", err)
	}

	// Wait for messages
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(broadcastMsgs) != 3 {
		t.Fatalf("Expected 3 broadcast messages (global+group+user), got %d", len(broadcastMsgs))
	}

	scopes := map[BroadcastScope]bool{}
	for _, msg := range broadcastMsgs {
		scopes[msg.Scope] = true
	}
	if !scopes[ScopeGlobal] {
		t.Error("Missing global broadcast after reconnect")
	}
	if !scopes[ScopeGroup] {
		t.Error("Missing group broadcast after reconnect")
	}
	if !scopes[ScopeUser] {
		t.Error("Missing user broadcast after reconnect")
	}

	if len(serverActionMsgs) != 1 {
		t.Fatalf("Expected 1 server action message, got %d", len(serverActionMsgs))
	}
	if serverActionMsgs[0].Action != "tick" {
		t.Errorf("Expected action='tick', got '%s'", serverActionMsgs[0].Action)
	}
}
