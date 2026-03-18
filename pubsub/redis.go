package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Redis channel schema:
// All message types (global, group, user, server_action) route through a single channel.
// Scope-based filtering happens at the application layer via the connection registry.
const channelGlobal = "livetemplate:broadcast:global"

// RedisBroadcaster implements distributed broadcasting using Redis Pub/Sub.
//
// Features:
// - Publishes messages to Redis channels for cross-instance distribution
// - Subscribes to channels and fans out messages to local connections
// - Local-first optimization: skips Redis for same-instance broadcasts
// - Automatic reconnection on Redis failures
// - Metrics for broadcast latency tracking
type RedisBroadcaster struct {
	client              redis.UniversalClient
	pubsub              *redis.PubSub
	instanceID          string
	handler             MessageHandler
	serverActionHandler ServerActionHandler
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	mu                  sync.RWMutex
	closed              bool
	reconnectDelay      time.Duration // Delay before reconnecting after subscription failure (default: 1s)
}

// RedisBroadcasterOption configures RedisBroadcaster.
type RedisBroadcasterOption func(*RedisBroadcaster)

// WithReconnectDelay sets the delay before reconnecting after a subscription failure.
// Lower values = faster reconnection but more aggressive retries.
// Higher values = less load on Redis but slower recovery.
// Default: 1 second
func WithReconnectDelay(delay time.Duration) RedisBroadcasterOption {
	return func(b *RedisBroadcaster) {
		b.reconnectDelay = delay
	}
}

// NewRedisBroadcaster creates a new Redis-backed broadcaster.
//
// The client parameter can be any redis.UniversalClient (single-node, cluster, ring, sentinel).
//
// Example:
//
//	client := redis.NewClient(&redis.Options{
//	    Addr: "localhost:6379",
//	})
//	broadcaster := pubsub.NewRedisBroadcaster(client)
func NewRedisBroadcaster(client redis.UniversalClient, opts ...RedisBroadcasterOption) *RedisBroadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	b := &RedisBroadcaster{
		client:         client,
		instanceID:     uuid.New().String(),
		ctx:            ctx,
		cancel:         cancel,
		reconnectDelay: 1 * time.Second, // Default: 1 second
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	return b
}

// PublishGlobal publishes a message to all instances.
//
// The message will be received by all instances and fanned out to all their local connections.
func (b *RedisBroadcaster) PublishGlobal(payload []byte) error {
	msg := &BroadcastMessage{
		Type:       "broadcast",
		Scope:      ScopeGlobal,
		Payload:    payload,
		Timestamp:  time.Now(),
		InstanceID: b.instanceID,
	}

	return b.publish(channelGlobal, msg)
}

// PublishToGroup publishes a message to all instances for a specific group.
//
// The message will be received by all instances and fanned out to connections in the target group.
func (b *RedisBroadcaster) PublishToGroup(groupID string, payload []byte) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	msg := &BroadcastMessage{
		Type:       "broadcast",
		Scope:      ScopeGroup,
		GroupID:    groupID,
		Payload:    payload,
		Timestamp:  time.Now(),
		InstanceID: b.instanceID,
	}

	return b.publish(channelGlobal, msg)
}

// PublishToUser publishes a message to all instances for a specific user.
//
// The message will be received by all instances and fanned out to connections for the target user.
func (b *RedisBroadcaster) PublishToUser(userID string, payload []byte) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	msg := &BroadcastMessage{
		Type:       "broadcast",
		Scope:      ScopeUser,
		UserID:     userID,
		Payload:    payload,
		Timestamp:  time.Now(),
		InstanceID: b.instanceID,
	}

	return b.publish(channelGlobal, msg)
}

// PublishServerAction publishes a server-initiated action to all instances for a user.
//
// This dispatches to the matching store method on receiving instances, enabling
// server-initiated actions to work across a distributed deployment.
func (b *RedisBroadcaster) PublishServerAction(userID string, action string, data map[string]interface{}) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	if action == "" {
		return fmt.Errorf("action cannot be empty")
	}

	msg := &ServerActionMessage{
		Type:       "server_action",
		UserID:     userID,
		Action:     action,
		Data:       data,
		Timestamp:  time.Now(),
		InstanceID: b.instanceID,
	}

	return b.publishServerAction(channelGlobal, msg)
}

// publishServerAction serializes and publishes a server action message to a Redis channel.
func (b *RedisBroadcaster) publishServerAction(channel string, msg *ServerActionMessage) error {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("broadcaster is closed")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal server action message: %w", err)
	}

	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	if err := b.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	return nil
}

// publish serializes and publishes a message to a Redis channel.
func (b *RedisBroadcaster) publish(channel string, msg *BroadcastMessage) error {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("broadcaster is closed")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal broadcast message: %w", err)
	}

	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	if err := b.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	return nil
}

// Subscribe starts listening for broadcast messages.
//
// The handler will be called for each received message and should fan out
// the message to relevant local connections.
//
// This method blocks until Close() is called or an error occurs.
func (b *RedisBroadcaster) Subscribe(handler MessageHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	if b.handler != nil {
		b.mu.Unlock()
		return fmt.Errorf("already subscribed")
	}
	b.handler = handler

	// All messages route through single channel; scope-based filtering happens at application layer
	b.pubsub = b.client.Subscribe(b.ctx, channelGlobal)
	b.mu.Unlock()

	// Wait for subscription confirmation
	if _, err := b.pubsub.Receive(b.ctx); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	slog.Info("Subscribed to channel",
		slog.String("component", "redis_broadcaster"),
		slog.String("channel", channelGlobal),
		slog.String("instance_id", b.instanceID))

	// Start message processing goroutine
	b.wg.Add(1)
	go b.processMessages()

	return nil
}

// SubscribeServerActions starts listening for server action messages.
//
// The handler will be called for each received server action message.
// It should trigger the action on relevant local connections.
func (b *RedisBroadcaster) SubscribeServerActions(handler ServerActionHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	if b.serverActionHandler != nil {
		b.mu.Unlock()
		return fmt.Errorf("already subscribed to server actions")
	}
	b.serverActionHandler = handler
	b.mu.Unlock()

	slog.Info("Registered server action handler",
		slog.String("component", "redis_broadcaster"),
		slog.String("instance_id", b.instanceID))
	return nil
}

// processMessages continuously receives and processes messages from Redis.
func (b *RedisBroadcaster) processMessages() {
	defer b.wg.Done()

	// Get channel reference safely with read lock
	b.mu.RLock()
	if b.pubsub == nil {
		b.mu.RUnlock()
		slog.Error("Pubsub is nil, cannot process messages",
			slog.String("component", "redis_broadcaster"),
			slog.String("instance_id", b.instanceID))
		return
	}
	ch := b.pubsub.Channel()
	b.mu.RUnlock()

	for {
		select {
		case <-b.ctx.Done():
			slog.Info("Stopping message processing",
				slog.String("component", "redis_broadcaster"),
				slog.String("instance_id", b.instanceID))
			return

		case redisMsg, ok := <-ch:
			if !ok {
				slog.Warn("Channel closed, attempting reconnect",
					slog.String("component", "redis_broadcaster"))
				if err := b.reconnect(); err != nil {
					slog.Error("Reconnect failed",
						slog.String("component", "redis_broadcaster"),
						slog.Any("error", err))
					return
				}
				// Update channel reference after reconnect
				b.mu.RLock()
				if b.pubsub != nil {
					ch = b.pubsub.Channel()
				}
				b.mu.RUnlock()
				continue
			}

			if err := b.handleMessage(redisMsg); err != nil {
				slog.Error("Failed to handle message",
					slog.String("component", "redis_broadcaster"),
					slog.Any("error", err))
			}
		}
	}
}

// handleMessage deserializes and processes a single message.
func (b *RedisBroadcaster) handleMessage(redisMsg *redis.Message) error {
	// First, try to determine message type from a partial parse
	var typeCheck struct {
		Type       string `json:"type"`
		InstanceID string `json:"instanceID"`
	}
	if err := json.Unmarshal([]byte(redisMsg.Payload), &typeCheck); err != nil {
		return fmt.Errorf("failed to unmarshal message type: %w", err)
	}

	// Local-first optimization: skip messages from same instance
	if typeCheck.InstanceID == b.instanceID {
		return nil
	}

	// Route based on message type
	switch typeCheck.Type {
	case "server_action":
		return b.handleServerActionMessage(redisMsg)
	default:
		return b.handleBroadcastMessage(redisMsg)
	}
}

// handleBroadcastMessage processes a broadcast message.
func (b *RedisBroadcaster) handleBroadcastMessage(redisMsg *redis.Message) error {
	var msg BroadcastMessage
	if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal broadcast message: %w", err)
	}

	// Call the handler to fan out to local connections
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no broadcast handler registered")
	}

	if err := handler(&msg); err != nil {
		return fmt.Errorf("broadcast handler failed: %w", err)
	}

	return nil
}

// handleServerActionMessage processes a server action message.
func (b *RedisBroadcaster) handleServerActionMessage(redisMsg *redis.Message) error {
	var msg ServerActionMessage
	if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal server action message: %w", err)
	}

	// Call the server action handler
	b.mu.RLock()
	handler := b.serverActionHandler
	b.mu.RUnlock()

	if handler == nil {
		slog.Warn("No server action handler registered, ignoring message",
			slog.String("component", "redis_broadcaster"))
		return nil
	}

	if err := handler(&msg); err != nil {
		return fmt.Errorf("server action handler failed: %w", err)
	}

	return nil
}

// reconnect attempts to re-establish the Redis subscription after a failure.
func (b *RedisBroadcaster) reconnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("broadcaster is closed")
	}

	// Close old subscription
	if b.pubsub != nil {
		_ = b.pubsub.Close()
	}

	// Wait before reconnecting
	time.Sleep(b.reconnectDelay)

	// Create new subscription
	b.pubsub = b.client.Subscribe(b.ctx, channelGlobal)

	// Wait for confirmation
	if _, err := b.pubsub.Receive(b.ctx); err != nil {
		return fmt.Errorf("failed to resubscribe: %w", err)
	}

	slog.Info("Reconnected successfully",
		slog.String("component", "redis_broadcaster"))
	return nil
}

// Close stops the broadcaster and cleans up resources.
func (b *RedisBroadcaster) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	// Cancel context to stop message processing
	b.cancel()

	// Wait for goroutines to finish
	b.wg.Wait()

	// Close pubsub with write lock
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pubsub != nil {
		if err := b.pubsub.Close(); err != nil {
			return fmt.Errorf("failed to close pubsub: %w", err)
		}
		b.pubsub = nil
	}

	slog.Info("Closed",
		slog.String("component", "redis_broadcaster"),
		slog.String("instance_id", b.instanceID))
	return nil
}
