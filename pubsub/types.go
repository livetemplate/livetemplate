package pubsub

import "time"

// BroadcastMessage represents a broadcast message sent over Redis Pub/Sub.
//
// Messages are JSON-encoded and published to Redis channels for distribution
// across multiple application instances in a horizontally scaled deployment.
type BroadcastMessage struct {
	// Type identifies the message type (always "broadcast" for now)
	Type string `json:"type"`

	// GroupID identifies the target session group
	// Empty string means no group targeting (used for global broadcasts)
	GroupID string `json:"groupID,omitempty"`

	// UserID identifies the target user
	// Empty string means anonymous user or no user targeting
	UserID string `json:"userID,omitempty"`

	// ExcludeConnID is the connection ID to exclude from the broadcast
	// Used to avoid sending duplicate updates to the originating connection
	ExcludeConnID string `json:"excludeConnID,omitempty"`

	// Payload contains the actual broadcast data
	// This will be passed to Template.ExecuteUpdates() on receiving instances
	Payload []byte `json:"payload"`

	// Timestamp indicates when the message was published
	Timestamp time.Time `json:"timestamp"`

	// InstanceID identifies the instance that published this message
	// Used for debugging and local-first optimization
	InstanceID string `json:"instanceID"`

	// Scope defines the broadcast scope
	Scope BroadcastScope `json:"scope"`
}

// BroadcastScope defines the targeting scope for a broadcast message.
type BroadcastScope string

const (
	// ScopeGlobal broadcasts to all connections across all instances
	ScopeGlobal BroadcastScope = "global"

	// ScopeGroup broadcasts to all connections in a specific session group
	ScopeGroup BroadcastScope = "group"

	// ScopeUser broadcasts to all connections for specific users
	ScopeUser BroadcastScope = "user"
)

// Broadcaster defines the interface for distributed broadcasting.
//
// Implementations handle publishing messages to remote instances and
// subscribing to messages from other instances for local fan-out.
type Broadcaster interface {
	// PublishGlobal publishes a message to all instances
	PublishGlobal(payload []byte) error

	// PublishToGroup publishes a message to all instances for a specific group
	PublishToGroup(groupID string, payload []byte) error

	// PublishToUser publishes a message to all instances for a specific user
	PublishToUser(userID string, payload []byte) error

	// Subscribe starts listening for broadcast messages and calls the handler
	// The handler is responsible for fan-out to local connections
	Subscribe(handler MessageHandler) error

	// Close stops the broadcaster and cleans up resources
	Close() error
}

// MessageHandler is called when a broadcast message is received.
// It should fan out the message to relevant local connections.
type MessageHandler func(msg *BroadcastMessage) error
