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
	done := make(chan bool, 1)
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
	done := make(chan bool, 1)
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
	done := make(chan bool, 1)
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
	done := make(chan bool, 1)
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

func TestRedisBroadcaster_ReconnectPreservesDynamicSubscriptions(t *testing.T) {
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

	// Subscribe to dynamic channels before reconnect
	if err := subscriber.SubscribeToGroup("g1"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}
	if err := subscriber.SubscribeToUser("u1"); err != nil {
		t.Fatalf("SubscribeToUser failed: %v", err)
	}
	if err := subscriber.SubscribeToServerAction("u1"); err != nil {
		t.Fatalf("SubscribeToServerAction failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Force a reconnect by closing the internal pubsub
	subscriber.mu.Lock()
	if subscriber.pubsub != nil {
		_ = subscriber.pubsub.Close()
	}
	subscriber.mu.Unlock()

	// Wait for reconnect to complete
	time.Sleep(200 * time.Millisecond)

	// Publish to all channel types after reconnect
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

func TestRedisBroadcaster_SubscribeDedup(t *testing.T) {
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

	go func() {
		if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error {
			return nil
		}); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// First subscription should succeed
	if err := broadcaster.SubscribeToGroup("g1"); err != nil {
		t.Fatalf("First SubscribeToGroup failed: %v", err)
	}

	// Second subscription to same group should be a no-op (dedup)
	if err := broadcaster.SubscribeToGroup("g1"); err != nil {
		t.Fatalf("Second SubscribeToGroup failed: %v", err)
	}

	// Different group should succeed
	if err := broadcaster.SubscribeToGroup("g2"); err != nil {
		t.Fatalf("SubscribeToGroup for g2 failed: %v", err)
	}

	// Verify subscribedChannels has exactly 2 group entries
	broadcaster.mu.RLock()
	count := len(broadcaster.subscribedChannels)
	broadcaster.mu.RUnlock()

	if count != 2 {
		t.Errorf("Expected 2 entries in subscribedChannels, got %d", count)
	}

	// Same dedup for user channels
	if err := broadcaster.SubscribeToUser("u1"); err != nil {
		t.Fatalf("First SubscribeToUser failed: %v", err)
	}
	if err := broadcaster.SubscribeToUser("u1"); err != nil {
		t.Fatalf("Second SubscribeToUser failed: %v", err)
	}

	broadcaster.mu.RLock()
	count = len(broadcaster.subscribedChannels)
	broadcaster.mu.RUnlock()

	// 2 groups + 1 user = 3
	if count != 3 {
		t.Errorf("Expected 3 entries in subscribedChannels, got %d", count)
	}
}

func TestRedisBroadcaster_CrossInstanceGroupBroadcast(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	// Instance A: publisher
	instanceA := NewRedisBroadcaster(client)
	defer func() {
		if err := instanceA.Close(); err != nil {
			t.Errorf("Failed to close instanceA: %v", err)
		}
	}()

	// Instance B: subscriber with dynamic group subscription
	instanceB := NewRedisBroadcaster(client)
	defer func() {
		if err := instanceB.Close(); err != nil {
			t.Errorf("Failed to close instanceB: %v", err)
		}
	}()

	var received sync.WaitGroup
	received.Add(1)

	var receivedMsg *BroadcastMessage
	go func() {
		if err := instanceB.Subscribe(func(msg *BroadcastMessage) error {
			receivedMsg = msg
			received.Done()
			return nil
		}); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Instance B subscribes to group channel dynamically
	if err := instanceB.SubscribeToGroup("tenant-42"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Instance A publishes to that group
	if err := instanceA.PublishToGroup("tenant-42", []byte(`{"update": "new-data"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}

	done := make(chan bool, 1)
	go func() {
		received.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: instance B did not receive group broadcast from instance A")
	}

	if receivedMsg == nil {
		t.Fatal("No message received")
	}
	if receivedMsg.Scope != ScopeGroup {
		t.Errorf("Expected Scope=ScopeGroup, got '%s'", receivedMsg.Scope)
	}
	if receivedMsg.GroupID != "tenant-42" {
		t.Errorf("Expected GroupID='tenant-42', got '%s'", receivedMsg.GroupID)
	}
}

func TestRedisBroadcaster_UnsubscribedGroupNotReceived(t *testing.T) {
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

	subscriber := NewRedisBroadcaster(client)
	defer func() {
		if err := subscriber.Close(); err != nil {
			t.Errorf("Failed to close subscriber: %v", err)
		}
	}()

	var mu sync.Mutex
	var received []*BroadcastMessage

	go func() {
		if err := subscriber.Subscribe(func(msg *BroadcastMessage) error {
			mu.Lock()
			received = append(received, msg)
			mu.Unlock()
			return nil
		}); err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Subscribe to group-A only, NOT group-B
	if err := subscriber.SubscribeToGroup("group-A"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish to group-B (subscriber should NOT receive this)
	if err := publisher.PublishToGroup("group-B", []byte(`{"test": "should-not-arrive"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}

	// Publish to group-A (subscriber should receive this)
	if err := publisher.PublishToGroup("group-A", []byte(`{"test": "should-arrive"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}

	// Wait for messages to propagate
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("Expected 1 message (group-A only), got %d", len(received))
	}
	if received[0].GroupID != "group-A" {
		t.Errorf("Expected GroupID='group-A', got '%s'", received[0].GroupID)
	}
}
