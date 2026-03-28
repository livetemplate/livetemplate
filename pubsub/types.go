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

	// PublishServerAction publishes a server-initiated action to all instances for a user.
	// This dispatches to the matching store method on receiving instances.
	PublishServerAction(userID string, action string, data map[string]interface{}) error

	// Subscribe starts listening for broadcast messages and calls the handler
	// The handler is responsible for fan-out to local connections
	Subscribe(handler MessageHandler) error

	// SubscribeServerActions starts listening for server action messages
	// The handler is responsible for triggering actions on local connections
	SubscribeServerActions(handler ServerActionHandler) error

	// PublishGroupAction publishes a group-scoped action to all instances.
	// Each receiving instance dispatches the action on all local connections
	// in the target group via their DispatchChan.
	PublishGroupAction(groupID string, action string, data map[string]interface{}) error

	// SubscribeGroupActions starts listening for group action messages.
	// The handler dispatches actions to local connections in the target group.
	SubscribeGroupActions(handler GroupActionHandler) error

	// Close stops the broadcaster and cleans up resources
	Close() error
}

// DynamicSubscriber allows subscribing to scoped channels at runtime.
// Implementations that support per-scope channels (for transport-level
// data isolation) should implement this interface. The handler layer
// type-asserts and calls these methods during WebSocket connection setup.
type DynamicSubscriber interface {
	SubscribeToGroup(groupID string) error
	SubscribeToUser(userID string) error
	SubscribeToServerAction(userID string) error
	SubscribeToGroupAction(groupID string) error
}

// MessageHandler is called when a broadcast message is received.
// It should fan out the message to relevant local connections.
type MessageHandler func(msg *BroadcastMessage) error

// ServerActionMessage represents a server-initiated action sent over Redis Pub/Sub.
//
// This message type dispatches to the matching store method on receiving instances,
// enabling server-initiated actions to work across a distributed deployment.
type ServerActionMessage struct {
	// Type identifies the message type (always "server_action")
	Type string `json:"type"`

	// UserID identifies the target user (all connections for this user)
	UserID string `json:"userID"`

	// Action is the action name to trigger (e.g., "tick", "refresh")
	Action string `json:"action"`

	// Data contains the action data (may be nil)
	Data map[string]interface{} `json:"data,omitempty"`

	// Timestamp indicates when the message was published
	Timestamp time.Time `json:"timestamp"`

	// InstanceID identifies the instance that published this message
	InstanceID string `json:"instanceID"`
}

// ServerActionHandler is called when a server action message is received.
// It should trigger the action on relevant local connections.
type ServerActionHandler func(msg *ServerActionMessage) error

// GroupActionMessage represents a group-scoped action sent over Redis Pub/Sub.
// Unlike ServerActionMessage (user-scoped), this targets all connections in a
// specific session group across all instances. Used by BroadcastAction to deliver
// explicit cross-connection broadcasts in per-connection state mode.
type GroupActionMessage struct {
	Type       string                 `json:"type"`
	GroupID    string                 `json:"groupID"`
	Action     string                 `json:"action"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	InstanceID string                 `json:"instanceID"`
}

// GroupActionHandler is called when a group action message is received.
// It should dispatch the action to relevant local connections in the group.
type GroupActionHandler func(msg *GroupActionMessage) error
