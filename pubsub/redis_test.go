package pubsub

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// errTestHandlerNilMsg is returned by test MessageHandler closures when the
// broadcast layer delivers a nil message. The defensive return turns these
// closures into real contract validators (and stops unparam from flagging
// them as always-nil-result).
var errTestHandlerNilMsg = errors.New("test handler received nil message")

// waitForTimeout is the default deadline for waitFor. 2s is generous enough
// for slow CI runners while still failing fast on a real bug; all callers
// today share the same budget so it lives in one place.
const waitForTimeout = 2 * time.Second

// subscribeAckWindow is the brief delay after SubscribeToGroup/SubscribeToUser/
// SubscribeToServerAction/SubscribeToGroupAction returns. Unlike top-level
// Subscribe() (which calls pubsub.Receive() to wait for Redis to ACK the
// SUBSCRIBE), subscribeTo() only writes the SUBSCRIBE bytes — Redis processes
// them async on its end. A test that publishes on the new channel without
// this gap can race the SUBSCRIBE round-trip on a separate connection and
// silently drop the message. 50ms is plenty above Redis's sub-ms processing
// time (the original tests used 100ms; this trims that excess while keeping
// a comfortable margin). Worth a follow-up to consider adding Receive() to
// subscribeTo for symmetry with Subscribe — that would let the helper drop
// to zero.
const subscribeAckWindow = 50 * time.Millisecond

// waitFor replaces time.Sleep-based synchronization when an observable state
// change (e.g. reconnect completion, message arrival) is the real trigger.
func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(waitForTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

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
		if msg == nil {
			return errTestHandlerNilMsg
		}
		receivedMsg = msg
		received.Done()
		return nil
	}

	// Subscribe synchronously — Subscribe() blocks until the Redis SUBSCRIBE
	// is confirmed, so no sleep is needed afterward.
	if err := broadcaster2.Subscribe(handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

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
		if msg == nil {
			return errTestHandlerNilMsg
		}
		receivedCount++
		return nil
	}

	// Subscribe synchronously — Subscribe() blocks until Redis SUBSCRIBE confirms.
	if err := broadcaster.Subscribe(handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish from same instance
	payload := []byte(`{"test": "data"}`)
	if err := broadcaster.PublishGlobal(payload); err != nil {
		t.Fatalf("PublishGlobal failed: %v", err)
	}

	// Negative assertion: this test verifies a message is NOT received (local
	// optimization filters own-instance messages). A bounded sleep is the
	// honest way to assert "nothing happened within N ms" — there is no
	// state change to poll for.
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
		if msg == nil {
			return errTestHandlerNilMsg
		}
		receivedMsg = msg
		received.Done()
		return nil
	}

	// Subscribe synchronously — Subscribe() blocks until the Redis SUBSCRIBE
	// is confirmed, so no sleep is needed afterward.
	if err := broadcaster2.Subscribe(handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := broadcaster2.SubscribeToGroup("group-123"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}

	// Wait for Redis to ACK the SUBSCRIBE (subscribeTo doesn't Receive — see
	// subscribeAckWindow). Without this gap, a publish on a separate connection
	// can race the SUBSCRIBE registration and the message is silently dropped.
	time.Sleep(subscribeAckWindow)

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
		if msg == nil {
			return errTestHandlerNilMsg
		}
		receivedMsg = msg
		received.Done()
		return nil
	}

	// Subscribe() blocks until Redis ACKs the global SUBSCRIBE.
	if err := broadcaster2.Subscribe(handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// SubscribeToUser writes SUBSCRIBE bytes synchronously; the ACK is gated
	// by the subscribeAckWindow sleep below.
	if err := broadcaster2.SubscribeToUser("user-456"); err != nil {
		t.Fatalf("SubscribeToUser failed: %v", err)
	}

	// Wait for Redis to ACK the SUBSCRIBE (see subscribeAckWindow).
	time.Sleep(subscribeAckWindow)

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
		if msg == nil {
			return errTestHandlerNilMsg
		}
		received.Done()
		return nil
	}

	// Subscribe both synchronously — Subscribe() blocks until Redis confirms.
	if err := broadcaster2.Subscribe(handler); err != nil {
		t.Fatalf("broadcaster2 Subscribe failed: %v", err)
	}
	if err := broadcaster3.Subscribe(handler); err != nil {
		t.Fatalf("broadcaster3 Subscribe failed: %v", err)
	}

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

	// Subscribe synchronously before closing.
	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error {
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

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

	if err := subscriber.Subscribe(func(msg *BroadcastMessage) error {
		mu.Lock()
		broadcastMsgs = append(broadcastMsgs, msg)
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Subscribe to dynamic channels before reconnect (all synchronous).
	if err := subscriber.SubscribeToGroup("g1"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}
	if err := subscriber.SubscribeToUser("u1"); err != nil {
		t.Fatalf("SubscribeToUser failed: %v", err)
	}
	if err := subscriber.SubscribeToServerAction("u1"); err != nil {
		t.Fatalf("SubscribeToServerAction failed: %v", err)
	}

	// Force a reconnect by closing the internal pubsub. Capture the pre-close
	// pubsub pointer so we can poll for it being swapped — checking only
	// "pubsub != nil" would race with the test (b.pubsub still references the
	// dead connection until reconnect() reassigns it).
	subscriber.mu.Lock()
	oldPubsub := subscriber.pubsub
	if subscriber.pubsub != nil {
		_ = subscriber.pubsub.Close()
	}
	subscriber.mu.Unlock()

	// Poll for reconnect completion: b.pubsub has been swapped to a new
	// connection AND the reconnecting flag has cleared.
	if !waitFor(func() bool {
		subscriber.mu.RLock()
		defer subscriber.mu.RUnlock()
		return subscriber.pubsub != nil && subscriber.pubsub != oldPubsub && !subscriber.reconnecting
	}) {
		t.Fatalf("reconnect did not complete within %v", waitForTimeout)
	}

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

	// Poll for all expected messages to arrive.
	if !waitFor(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(broadcastMsgs) >= 3 && len(serverActionMsgs) >= 1
	}) {
		mu.Lock()
		gotB, gotS := len(broadcastMsgs), len(serverActionMsgs)
		mu.Unlock()
		t.Fatalf("timeout waiting for messages after reconnect: broadcasts=%d (want 3), serverActions=%d (want 1)", gotB, gotS)
	}

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

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error {
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

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
	if err := instanceB.Subscribe(func(msg *BroadcastMessage) error {
		receivedMsg = msg
		received.Done()
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := instanceB.SubscribeToGroup("tenant-42"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}

	// Wait for Redis to ACK the SUBSCRIBE (see subscribeAckWindow).
	time.Sleep(subscribeAckWindow)

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

	if err := subscriber.Subscribe(func(msg *BroadcastMessage) error {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Subscribe to group-A only, NOT group-B.
	if err := subscriber.SubscribeToGroup("group-A"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}

	// Wait for Redis to ACK the SUBSCRIBE (see subscribeAckWindow).
	time.Sleep(subscribeAckWindow)

	// Publish to group-B first, then group-A. Both publishes use the same
	// Redis connection ordering, so by the time group-A's message arrives at
	// the subscriber, group-B's would have arrived too if it were going to.
	// Thus polling for the group-A arrival lets us deterministically assert
	// group-B was filtered out.
	if err := publisher.PublishToGroup("group-B", []byte(`{"test": "should-not-arrive"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}
	if err := publisher.PublishToGroup("group-A", []byte(`{"test": "should-arrive"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}

	// Poll for the group-A message to land — its arrival is the synchronization
	// point that guarantees group-B's would have arrived too if not filtered.
	if !waitFor(func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, m := range received {
			if m.GroupID == "group-A" {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("group-A message did not arrive within %v", waitForTimeout)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("Expected 1 message (group-A only), got %d", len(received))
	}
	if received[0].GroupID != "group-A" {
		t.Errorf("Expected GroupID='group-A', got '%s'", received[0].GroupID)
	}
}

func TestSubscribeTo_RetrySucceedsAfterTransientFailure(t *testing.T) {
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

	handler := func(msg *BroadcastMessage) error { return nil }
	if err := broadcaster.Subscribe(handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Save the real pubsub, then nil it out to simulate transient failure
	broadcaster.mu.Lock()
	realPubSub := broadcaster.pubsub
	broadcaster.pubsub = nil
	broadcaster.mu.Unlock()

	// Restore immediately in a goroutine — the first trySubscribe attempt sees nil
	// and fails, then this goroutine restores pubsub before the 100ms retry delay elapses
	restored := make(chan struct{})
	go func() {
		broadcaster.mu.Lock()
		broadcaster.pubsub = realPubSub
		broadcaster.mu.Unlock()
		close(restored)
	}()
	defer func() { <-restored }()

	err := broadcaster.subscribeTo("test:retry-channel", "test")
	if err != nil {
		t.Fatalf("subscribeTo should have succeeded on retry, got: %v", err)
	}

	broadcaster.mu.RLock()
	_, subscribed := broadcaster.subscribedChannels["test:retry-channel"]
	broadcaster.mu.RUnlock()
	if !subscribed {
		t.Fatal("channel should be tracked in subscribedChannels after successful retry")
	}
}

func TestSubscribeTo_ExhaustsRetriesWhenPubSubNil(t *testing.T) {
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

	// Don't call Subscribe — pubsub stays nil, all 3 attempts should fail
	err := broadcaster.subscribeTo("test:fail-channel", "test")
	if err == nil {
		t.Fatal("subscribeTo should fail when pubsub is permanently nil")
	}

	if !strings.Contains(err.Error(), "attempts") {
		t.Fatalf("error should mention exhausted attempts, got: %v", err)
	}
}

func TestSubscribeTo_RespectsContextCancellation(t *testing.T) {
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

	// Cancel context immediately so the retry wait exits early
	broadcaster.cancel()

	start := time.Now()
	err := broadcaster.subscribeTo("test:cancel-channel", "test")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("subscribeTo should fail when context is cancelled")
	}

	if !strings.Contains(err.Error(), "context cancelled") {
		t.Fatalf("error should mention context cancellation, got: %v", err)
	}

	// Should exit quickly — not wait for all retry delays
	if elapsed > 500*time.Millisecond {
		t.Fatalf("context cancellation should short-circuit retries, took %v", elapsed)
	}
}

func TestSubscribeTo_DeduplicatesAlreadySubscribed(t *testing.T) {
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

	handler := func(msg *BroadcastMessage) error { return nil }
	if err := broadcaster.Subscribe(handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	channel := "test:dedup-channel"
	if err := broadcaster.subscribeTo(channel, "test"); err != nil {
		t.Fatalf("first subscribeTo failed: %v", err)
	}

	// Second call should succeed immediately (dedup)
	if err := broadcaster.subscribeTo(channel, "test"); err != nil {
		t.Fatalf("duplicate subscribeTo should succeed (dedup), got: %v", err)
	}
}

// TestSubscribeTo_DoesNotBlockConcurrentOperations verifies the fix for #215:
// subscribeTo() must NOT hold b.mu across the Redis SUBSCRIBE network call,
// so a slow Subscribe on one channel must not block concurrent publish or
// subscribe operations. The test uses subscribeHook to make one Subscribe
// pause inside trySubscribe, then asserts the racing Publish and the racing
// SubscribeToGroup on a different channel both complete quickly.
func TestSubscribeTo_DoesNotBlockConcurrentOperations(t *testing.T) {
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

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error {
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Inject a hook that holds inside trySubscribe (after lock release, before
	// the Redis network call) until we explicitly release it.
	release := make(chan struct{})
	hookEntered := make(chan struct{})
	var hookOnce sync.Once
	prevHook := setSubscribeHook(func() {
		// trySubscribe can run concurrently from reconnect()'s channel
		// replay on the processMessages loop and the test's explicit
		// goroutine, so the close must be exactly-once (a select/default
		// guard is not atomic across goroutines).
		hookOnce.Do(func() { close(hookEntered) })
		<-release
	})
	defer setSubscribeHook(prevHook)

	// Start the slow subscribe in a goroutine.
	slowDone := make(chan error, 1)
	go func() {
		slowDone <- broadcaster.SubscribeToGroup("slow-group")
	}()

	// Wait for the slow subscribe to enter the hook (i.e. it has released
	// the lock and is sitting in the network-call window).
	select {
	case <-hookEntered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("slow Subscribe never reached the hook")
	}

	// Concurrent Publish must NOT block on the held-during-Subscribe lock.
	// With the bug, this would block until the slow Subscribe completes.
	pubDone := make(chan error, 1)
	go func() {
		pubDone <- broadcaster.PublishGlobal([]byte(`{"test": "concurrent"}`))
	}()

	select {
	case err := <-pubDone:
		if err != nil {
			t.Fatalf("PublishGlobal failed: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		close(release)
		t.Fatal("PublishGlobal blocked while a peer SubscribeToGroup was in flight (#215 regression)")
	}

	// Concurrent SubscribeToGroup on a *different* channel must also not block.
	// Note: this call still goes through the same hook (the fix preserves
	// Subscribe-on-different-channel as concurrent at the b.mu level — the hook
	// stalls in the network-call window, not under the lock — so we expect
	// this goroutine to also reach hookEntered without contending on b.mu).
	// We verify the lock-side property by having a fast operation that only
	// touches b.mu (PublishGlobal above) succeed; that's the load-bearing assertion.

	// Release the held subscribe and let it complete.
	close(release)

	select {
	case err := <-slowDone:
		if err != nil {
			t.Fatalf("SubscribeToGroup failed after release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow SubscribeToGroup did not complete after release")
	}
}

// TestReconnect_DoesNotBlockConcurrentPublish verifies the fix for #215 on
// the reconnect() path: the lock must be released across the reconnect-delay
// sleep and the Redis SUBSCRIBE/Receive calls. With the bug, a 1s default
// reconnectDelay stalled all publishes for the full second.
//
// Synchronization uses reconnectHook (signalled when reconnect() has released
// its initial lock and is about to enter the sleep window) rather than
// time-based sleep, so the test is deterministic on slow CI runners.
func TestReconnect_DoesNotBlockConcurrentPublish(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	// Use a long reconnect delay to make the bug observable. With the fix,
	// publish during the sleep window completes in milliseconds.
	broadcaster := NewRedisBroadcaster(client, WithReconnectDelay(800*time.Millisecond))
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error {
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Hook fires after reconnect() has released its initial lock and is about
	// to enter the sleep window. Use it to deterministically time the publish
	// rather than guessing with time.Sleep.
	hookEntered := make(chan struct{})
	var hookOnce sync.Once
	prevHook := setReconnectHook(func() {
		// reconnect() can run concurrently from the processMessages loop and
		// the test's explicit goroutine, so the close must be exactly-once
		// (a select/default guard is not atomic across goroutines).
		hookOnce.Do(func() { close(hookEntered) })
	})
	defer setReconnectHook(prevHook)

	// Trigger a reconnect from a goroutine.
	reconnectDone := make(chan error, 1)
	go func() {
		reconnectDone <- broadcaster.reconnect()
	}()

	// Wait for reconnect() to enter the no-lock sleep window.
	select {
	case <-hookEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("reconnect never reached the hook (lock-release phase did not run)")
	}

	// Publish must not block on the lock while reconnect() is sleeping.
	start := time.Now()
	if err := broadcaster.PublishGlobal([]byte(`{"test": "during-reconnect"}`)); err != nil {
		t.Fatalf("PublishGlobal failed during reconnect: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Fatalf("PublishGlobal blocked for %v during reconnect sleep (#215 regression — should be <200ms)", elapsed)
	}

	// reconnect() must still complete successfully.
	select {
	case err := <-reconnectDone:
		if err != nil {
			t.Fatalf("reconnect failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not complete")
	}
}

// TestReconnect_InterruptibleByContextCancel verifies that Close() (which
// cancels the broadcaster context) short-circuits a long reconnectDelay
// rather than blocking for the full delay.
//
// Uses reconnectHook to deterministically signal that reconnect() has entered
// its sleep window, then calls Close() (preferred over the bare cancel() so
// b.closed is set, matching production semantics).
func TestReconnect_InterruptibleByContextCancel(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	broadcaster := NewRedisBroadcaster(client, WithReconnectDelay(5*time.Second))

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error { return nil }); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	hookEntered := make(chan struct{})
	var hookOnce sync.Once
	prevHook := setReconnectHook(func() {
		// reconnect() can run concurrently from the processMessages loop and
		// the test's explicit goroutine, so the close must be exactly-once
		// (a select/default guard is not atomic across goroutines).
		hookOnce.Do(func() { close(hookEntered) })
	})
	defer setReconnectHook(prevHook)

	reconnectDone := make(chan error, 1)
	go func() {
		reconnectDone <- broadcaster.reconnect()
	}()

	// Wait for reconnect to enter its sleep, then trigger Close to cancel
	// the context. Close() also sets b.closed = true (matches production
	// semantics; using the bare cancel() left b.closed = false).
	select {
	case <-hookEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("reconnect never reached the hook")
	}
	closeDone := make(chan error, 1)
	go func() {
		// Best-effort: Close may report an error because the racing reconnect
		// can tear down the pubsub from underneath it. The interrupt itself
		// is the load-bearing assertion below.
		closeDone <- broadcaster.Close()
	}()

	select {
	case <-reconnectDone:
		// Expected: reconnect returns promptly when ctx is cancelled.
	case <-time.After(1 * time.Second):
		t.Fatal("reconnect did not honor context cancellation; would have slept 5s")
	}

	// Drain Close to avoid leaking the goroutine.
	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not return")
	}
}

// TestSubscribeTo_SucceedsDuringReconnectWindow verifies the fix for the
// follow-up issue surfaced in PR #389 review: with b.pubsub == nil during
// reconnect, a concurrent SubscribeToGroup must not fail-fast inside its
// 3×100ms retry budget. The extended retry window (bounded by reconnectDelay)
// keeps retrying until reconnect installs the new pubsub, then succeeds.
func TestSubscribeTo_SucceedsDuringReconnectWindow(t *testing.T) {
	client := getTestRedisClient(t)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
	}()

	// 600ms > 200ms naive retry budget; with the fix, subscribeTo waits for
	// the reconnect window before giving up.
	broadcaster := NewRedisBroadcaster(client, WithReconnectDelay(600*time.Millisecond))
	defer func() {
		if err := broadcaster.Close(); err != nil {
			t.Errorf("Failed to close broadcaster: %v", err)
		}
	}()

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error {
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	hookEntered := make(chan struct{})
	var hookOnce sync.Once
	prevHook := setReconnectHook(func() {
		// reconnect() can run concurrently from the processMessages loop and
		// the test's explicit goroutine, so the close must be exactly-once
		// (a select/default guard is not atomic across goroutines).
		hookOnce.Do(func() { close(hookEntered) })
	})
	defer setReconnectHook(prevHook)

	// Start reconnect; pubsub is nilled inside the lock then released before
	// the hook fires.
	reconnectDone := make(chan error, 1)
	go func() {
		reconnectDone <- broadcaster.reconnect()
	}()

	// Wait until reconnect is in its sleep window — this is the worst case
	// for SubscribeToGroup because pubsub is observably nil for the full
	// reconnectDelay.
	select {
	case <-hookEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("reconnect never reached the hook")
	}

	// SubscribeToGroup MUST succeed even though it lands during the
	// pubsub-is-nil window. With the bug (3×100ms fail-fast), this would
	// return "not subscribed" well before reconnect installs the new pubsub.
	subDone := make(chan error, 1)
	go func() {
		subDone <- broadcaster.SubscribeToGroup("late-subscribe-group")
	}()

	select {
	case err := <-subDone:
		if err != nil {
			t.Fatalf("SubscribeToGroup failed during reconnect window: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SubscribeToGroup did not complete within reconnect window")
	}

	// reconnect must complete successfully too.
	select {
	case err := <-reconnectDone:
		if err != nil {
			t.Fatalf("reconnect failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not complete")
	}
}

// TestRedisBroadcaster_SubscribeRefcount_Increments verifies that repeated
// SubscribeTo calls for the same channel bump a per-channel refcount rather
// than silently deduplicating. Without refcounting, the first connection to
// disconnect would tear down the shared subscription for everyone (#214).
func TestRedisBroadcaster_SubscribeRefcount_Increments(t *testing.T) {
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

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error { return nil }); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := broadcaster.SubscribeToGroup("g1"); err != nil {
			t.Fatalf("SubscribeToGroup attempt %d failed: %v", i+1, err)
		}
	}

	broadcaster.mu.RLock()
	count := broadcaster.subscribedChannels[channelGroup+"g1"]
	broadcaster.mu.RUnlock()
	if count != 3 {
		t.Errorf("Expected refcount=3 after 3 SubscribeToGroup calls, got %d", count)
	}
}

// TestRedisBroadcaster_SubscribeRefcount_DecrementsKeepsEntry verifies that
// the channel stays in subscribedChannels (and the underlying Redis SUBSCRIBE
// stays live) until the refcount drops to zero.
func TestRedisBroadcaster_SubscribeRefcount_DecrementsKeepsEntry(t *testing.T) {
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

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error { return nil }); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := broadcaster.SubscribeToGroup("g1"); err != nil {
		t.Fatalf("first SubscribeToGroup failed: %v", err)
	}
	if err := broadcaster.SubscribeToGroup("g1"); err != nil {
		t.Fatalf("second SubscribeToGroup failed: %v", err)
	}

	if err := broadcaster.UnsubscribeFromGroup("g1"); err != nil {
		t.Fatalf("UnsubscribeFromGroup failed: %v", err)
	}

	broadcaster.mu.RLock()
	count, present := broadcaster.subscribedChannels[channelGroup+"g1"]
	broadcaster.mu.RUnlock()
	if !present {
		t.Fatal("channel should still be tracked after partial unsubscribe (refcount > 0)")
	}
	if count != 1 {
		t.Errorf("Expected refcount=1 after 2 subscribes + 1 unsubscribe, got %d", count)
	}
}

// TestRedisBroadcaster_SubscribeRefcount_ZeroRemovesEntry verifies the 1→0
// transition: the entry is removed from subscribedChannels and a Redis
// UNSUBSCRIBE is issued so the broadcaster no longer pays for traffic on
// that channel.
func TestRedisBroadcaster_SubscribeRefcount_ZeroRemovesEntry(t *testing.T) {
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

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error { return nil }); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := broadcaster.SubscribeToGroup("g-removable"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}
	if err := broadcaster.SubscribeToGroup("g-removable"); err != nil {
		t.Fatalf("second SubscribeToGroup failed: %v", err)
	}

	if err := broadcaster.UnsubscribeFromGroup("g-removable"); err != nil {
		t.Fatalf("first UnsubscribeFromGroup failed: %v", err)
	}
	if err := broadcaster.UnsubscribeFromGroup("g-removable"); err != nil {
		t.Fatalf("second UnsubscribeFromGroup failed: %v", err)
	}

	broadcaster.mu.RLock()
	_, present := broadcaster.subscribedChannels[channelGroup+"g-removable"]
	broadcaster.mu.RUnlock()
	if present {
		t.Fatal("channel should be removed from subscribedChannels after refcount reaches 0")
	}

	// A subsequent SubscribeToGroup on the same channel must succeed and
	// re-establish a refcount of 1 (i.e. trigger a fresh SUBSCRIBE rather
	// than piggybacking on a stale entry).
	if err := broadcaster.SubscribeToGroup("g-removable"); err != nil {
		t.Fatalf("re-SubscribeToGroup after teardown failed: %v", err)
	}
	broadcaster.mu.RLock()
	count := broadcaster.subscribedChannels[channelGroup+"g-removable"]
	broadcaster.mu.RUnlock()
	if count != 1 {
		t.Errorf("Expected fresh refcount=1 after re-subscribe, got %d", count)
	}
}

// TestRedisBroadcaster_UnsubscribeOnNeverSubscribed verifies that calling
// Unsubscribe* on a channel that was never subscribed is a benign no-op.
// Disconnect-time cleanup in the WebSocket handler relies on this so that
// partial-failure setup paths (Subscribe returned error) don't underflow the
// refcount when the deferred unsubscribe still fires.
func TestRedisBroadcaster_UnsubscribeOnNeverSubscribed(t *testing.T) {
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

	if err := broadcaster.UnsubscribeFromGroup("never-subscribed"); err != nil {
		t.Errorf("UnsubscribeFromGroup on unknown channel should be a no-op, got: %v", err)
	}
	if err := broadcaster.UnsubscribeFromUser("never-subscribed-user"); err != nil {
		t.Errorf("UnsubscribeFromUser on unknown channel should be a no-op, got: %v", err)
	}
	if err := broadcaster.UnsubscribeFromServerAction("never-subscribed-user"); err != nil {
		t.Errorf("UnsubscribeFromServerAction on unknown channel should be a no-op, got: %v", err)
	}
	if err := broadcaster.UnsubscribeFromGroupAction("never-subscribed"); err != nil {
		t.Errorf("UnsubscribeFromGroupAction on unknown channel should be a no-op, got: %v", err)
	}
}

// TestRedisBroadcaster_UnsubscribeAllScopes verifies that Unsubscribe*
// methods work for all four channel scopes (group, user, server action,
// group action), tearing each down independently.
func TestRedisBroadcaster_UnsubscribeAllScopes(t *testing.T) {
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

	if err := broadcaster.Subscribe(func(msg *BroadcastMessage) error { return nil }); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if err := broadcaster.SubscribeToGroup("g-all"); err != nil {
		t.Fatalf("SubscribeToGroup failed: %v", err)
	}
	if err := broadcaster.SubscribeToUser("u-all"); err != nil {
		t.Fatalf("SubscribeToUser failed: %v", err)
	}
	if err := broadcaster.SubscribeToServerAction("u-all"); err != nil {
		t.Fatalf("SubscribeToServerAction failed: %v", err)
	}
	if err := broadcaster.SubscribeToGroupAction("g-all"); err != nil {
		t.Fatalf("SubscribeToGroupAction failed: %v", err)
	}

	broadcaster.mu.RLock()
	if len(broadcaster.subscribedChannels) != 4 {
		broadcaster.mu.RUnlock()
		t.Fatalf("Expected 4 tracked channels, got %d", len(broadcaster.subscribedChannels))
	}
	broadcaster.mu.RUnlock()

	if err := broadcaster.UnsubscribeFromGroup("g-all"); err != nil {
		t.Fatalf("UnsubscribeFromGroup failed: %v", err)
	}
	if err := broadcaster.UnsubscribeFromUser("u-all"); err != nil {
		t.Fatalf("UnsubscribeFromUser failed: %v", err)
	}
	if err := broadcaster.UnsubscribeFromServerAction("u-all"); err != nil {
		t.Fatalf("UnsubscribeFromServerAction failed: %v", err)
	}
	if err := broadcaster.UnsubscribeFromGroupAction("g-all"); err != nil {
		t.Fatalf("UnsubscribeFromGroupAction failed: %v", err)
	}

	broadcaster.mu.RLock()
	remaining := len(broadcaster.subscribedChannels)
	broadcaster.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("Expected 0 tracked channels after unsubscribing all, got %d", remaining)
	}
}

// TestRedisBroadcaster_RefcountSurvivesUnsubscribeUntilZero is the integration
// scenario for #214: two "connections" share a group channel; one disconnects
// (1 unsubscribe), and the channel must remain live for the other; only the
// final disconnect tears it down.
func TestRedisBroadcaster_RefcountSurvivesUnsubscribeUntilZero(t *testing.T) {
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

	var receivedCount int
	var receivedMu sync.Mutex
	if err := subscriber.Subscribe(func(msg *BroadcastMessage) error {
		if msg == nil {
			return errTestHandlerNilMsg
		}
		receivedMu.Lock()
		receivedCount++
		receivedMu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Two "connections" both subscribe to the same group channel. Only the
	// first 0→1 transition issues a real Redis SUBSCRIBE; the second is a
	// refcount bump that doesn't touch the network.
	if err := subscriber.SubscribeToGroup("shared"); err != nil {
		t.Fatalf("first SubscribeToGroup failed: %v", err)
	}
	if err := subscriber.SubscribeToGroup("shared"); err != nil {
		t.Fatalf("second SubscribeToGroup failed: %v", err)
	}

	// Wait for Redis to ACK the first SUBSCRIBE (see subscribeAckWindow).
	time.Sleep(subscribeAckWindow)

	// First "connection" disconnects.
	if err := subscriber.UnsubscribeFromGroup("shared"); err != nil {
		t.Fatalf("first UnsubscribeFromGroup failed: %v", err)
	}

	// Channel must still be live: a publish should still reach the subscriber.
	if err := publisher.PublishToGroup("shared", []byte(`{"k":"v"}`)); err != nil {
		t.Fatalf("PublishToGroup failed: %v", err)
	}

	// Poll for the broadcast to arrive — refcount > 0 means the channel is
	// still live, so the publish must reach us.
	if !waitFor(func() bool {
		receivedMu.Lock()
		defer receivedMu.Unlock()
		return receivedCount >= 1
	}) {
		receivedMu.Lock()
		gotAfterPartial := receivedCount
		receivedMu.Unlock()
		t.Fatalf("Subscriber should still receive after partial unsubscribe (refcount > 0); receivedCount=%d", gotAfterPartial)
	}

	// Second disconnect: refcount → 0, channel torn down.
	if err := subscriber.UnsubscribeFromGroup("shared"); err != nil {
		t.Fatalf("second UnsubscribeFromGroup failed: %v", err)
	}

	subscriber.mu.RLock()
	_, present := subscriber.subscribedChannels[channelGroup+"shared"]
	subscriber.mu.RUnlock()
	if present {
		t.Fatal("channel must be removed from subscribedChannels after final unsubscribe")
	}
}
