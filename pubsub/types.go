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

	// Close stops the broadcaster and cleans up resources
	Close() error
}

// GroupActionBroadcaster extends Broadcaster with group-scoped action dispatch.
// Implementations that support BroadcastAction for cross-instance delivery should
// implement this interface. The handler layer type-asserts and calls these methods
// during action dispatch and WebSocket connection setup.
type GroupActionBroadcaster interface {
	// PublishGroupAction publishes a group-scoped action to all instances.
	PublishGroupAction(groupID string, action string, data map[string]interface{}) error

	// SubscribeGroupActions starts listening for group action messages.
	SubscribeGroupActions(handler GroupActionHandler) error
}

// DynamicSubscriber allows subscribing to scoped channels at runtime.
// Implementations that support per-scope channels (for transport-level
// data isolation) should implement this interface. The handler layer
// type-asserts and calls these methods during WebSocket connection setup.
//
// Subscribe* methods are reference-counted: each call increments the
// channel's refcount. Implementations only issue the underlying transport
// SUBSCRIBE on the 0→1 transition.
//
// Unsubscribe* methods decrement the corresponding channel's refcount. When
// the refcount drops to 0, the implementation issues the underlying transport
// UNSUBSCRIBE and forgets the channel. Callers must pair every successful
// Subscribe* call with exactly one Unsubscribe* call (typically via deferred
// cleanup on connection disconnect). Calling Unsubscribe* on a channel that
// was never subscribed is a no-op.
type DynamicSubscriber interface {
	SubscribeToGroup(groupID string) error
	SubscribeToUser(userID string) error
	SubscribeToServerAction(userID string) error
	UnsubscribeFromGroup(groupID string) error
	UnsubscribeFromUser(userID string) error
	UnsubscribeFromServerAction(userID string) error
}

// GroupActionSubscriber allows subscribing to group action channels at runtime.
// Checked via type assertion during WebSocket connection setup.
//
// SubscribeToGroupAction / UnsubscribeFromGroupAction are reference-counted
// per the same contract as DynamicSubscriber: callers must pair every
// successful subscribe with exactly one unsubscribe.
type GroupActionSubscriber interface {
	SubscribeToGroupAction(groupID string) error
	UnsubscribeFromGroupAction(groupID string) error
}

// TopicActionBroadcaster extends Broadcaster with Publish/Subscribe topic
// dispatch (the pub/sub topic model). It mirrors GroupActionBroadcaster: the
// handler layer type-asserts and calls these during ctx.Publish /
// handler.Publish fan-out and at handler init. Implementations that do not
// implement it stay single-instance for topics (local registry fan-out only) —
// backward compatible by construction.
//
// Topic messages reuse the GroupActionMessage envelope (Type "topic_action",
// routed by Topic) and the GroupActionHandler signature, so receiving instances
// run the action through the one shared dispatch path.
type TopicActionBroadcaster interface {
	// PublishToTopic publishes a topic-scoped action to all instances over the
	// single exact channel livetemplate:topic:{name}. Publishers always
	// PUBLISH to the exact channel — never to a pattern (Phase 3 wildcard
	// delivery works because Redis pattern-matching connects the exact PUBLISH
	// to a PSUBSCRIBE, not because publishers expand patterns).
	PublishToTopic(topic string, action string, data map[string]interface{}) error

	// SubscribeToTopicActions registers the handler invoked for every received
	// topic action message (from other instances). Mirrors SubscribeGroupActions.
	SubscribeToTopicActions(handler GroupActionHandler) error
}

// TopicChannelSubscriber allows subscribing to per-topic channels at runtime.
// Checked via type assertion when ctx.Subscribe materializes a Connection's
// topic subscription, so a topic published on another instance round-trips
// through Redis to this one.
//
// SubscribeToTopicChannel / UnsubscribeFromTopicChannel are reference-counted
// per the same contract as DynamicSubscriber: every successful subscribe pairs
// with exactly one unsubscribe (multiple local connections subscribing the same
// topic share one underlying Redis SUBSCRIBE; the channel is torn down on the
// 1→0 transition).
//
// Phase 2 issues only exact SUBSCRIBE (concrete topics). Wildcard PSUBSCRIBE is
// the separate TopicPatternSubscriber (Phase 3).
type TopicChannelSubscriber interface {
	SubscribeToTopicChannel(topic string) error
	UnsubscribeFromTopicChannel(topic string) error
}

// TopicPatternSubscriber relays a wildcard topic as one Redis PSUBSCRIBE (refcounted like TopicChannelSubscriber, parallel map).
// SEPARATE optional interface, not a TopicChannelSubscriber extension — exact-only broadcasters stay backward-compatible. Relay invariant + over-delivery handling: see relayTopicSubscribeOne / phase-3.md.
type TopicPatternSubscriber interface {
	SubscribeToTopicPattern(pattern string) error
	UnsubscribeFromTopicPattern(pattern string) error
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
//
// The same envelope also carries Publish/Subscribe topic actions (Type
// "topic_action", routed by Topic instead of GroupID) — reusing one struct keeps
// a single wire format, a single processMessages pump, and a single handleMessage
// type-switch (the design's "adds zero new machinery" property).
//
// Seq is a per-instance monotonic counter, atomically incremented on EVERY
// GroupActionMessage the instance emits (group-action AND topic). Together with
// InstanceID it forms the collision-free dedup id (instanceID, seq). Timestamp
// stays for observability but is NOT the dedup key: same-instance publishes
// within one clock tick share a timestamp, so timestamp-keyed dedup would
// wrongly drop the second.
//
// LOAD-BEARING INVARIANT: sharing one Seq counter across group-action and topic
// messages is safe ONLY while group-action broadcasts stay exact-SUBSCRIBE-only
// (no PSUBSCRIBE). The (instanceID, seq) double-fire dedup fires solely on the
// topic SUBSCRIBE+PSUBSCRIBE double-delivery path (topic subscriptions only),
// so a non-topic message's seq is never compared — making the shared counter
// benign. If wildcard group-action broadcasts are ever added, the
// (instanceID, seq) dedup scope MUST be revisited (e.g. a per-stream counter or
// a type-tagged key) before that change ships.
//
// This envelope is a tagged union, not a product type — fields populated by Type:
//   - "group_action": GroupID set, Topic empty (BroadcastAction cross-instance).
//   - "topic_action":  Topic set, GroupID empty (ctx.Publish / handler.Publish).
//
// Action/Data/Seq/Timestamp/InstanceID are always set; the receiver routes on
// Type and reads only the field valid for that Type.
type GroupActionMessage struct {
	Type    string                 `json:"type"`
	GroupID string                 `json:"groupID"`
	Topic   string                 `json:"topic,omitempty"`
	Action  string                 `json:"action"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Seq     uint64                 `json:"seq"` // intentionally NOT omitempty: seq==0 must mean "pre-upgrade sender", not "absent" — see the rolling-upgrade note above; do not add omitempty

	Timestamp  time.Time `json:"timestamp"`
	InstanceID string    `json:"instanceID"`
}

// GroupActionHandler is called when a group action message is received.
// It should dispatch the action to relevant local connections in the group.
type GroupActionHandler func(msg *GroupActionMessage) error
