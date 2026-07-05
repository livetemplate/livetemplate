package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/livetemplate/livetemplate/internal/jsonutil"
	"github.com/redis/go-redis/v9"
)

var _ DynamicSubscriber = (*RedisBroadcaster)(nil)
var _ GroupActionBroadcaster = (*RedisBroadcaster)(nil)
var _ GroupActionSubscriber = (*RedisBroadcaster)(nil)
var _ TopicActionBroadcaster = (*RedisBroadcaster)(nil)
var _ TopicChannelSubscriber = (*RedisBroadcaster)(nil)

// Redis channel schema:
// livetemplate:broadcast:global           -> Global broadcasts (all instances, all connections)
// livetemplate:broadcast:group:{groupID}  -> Group-specific broadcasts
// livetemplate:broadcast:user:{userID}    -> User-specific broadcasts

const (
	channelGlobal       = "livetemplate:broadcast:global"
	channelGroup        = "livetemplate:broadcast:group:"
	channelUser         = "livetemplate:broadcast:user:"
	channelServerAction = "livetemplate:action:user:"       // Server-initiated actions channel
	channelGroupAction  = "livetemplate:groupaction:group:" // Group-scoped broadcast actions channel
	channelTopic        = "livetemplate:topic:"             // Pub/sub topic actions channel (exact SUBSCRIBE; Phase 3 adds PSUBSCRIBE)
)

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
	groupActionHandler  GroupActionHandler
	topicActionHandler  GroupActionHandler // topic messages reuse the GroupActionMessage envelope + handler signature
	// seq is the per-instance monotonic dedup counter, incremented on every
	// GroupActionMessage emitted (group-action AND topic — see the
	// GroupActionMessage doc). atomic.Uint64.Add(1) returns the post-increment
	// value, so a live instance's first emit is seq=1; this instance never
	// emits seq=0.
	//
	// CONSTRAINTS for Phase 3's (instanceID, seq) dedup ring:
	//  1. Values are monotonic PER-INSTANCE only, never per-Type — group and
	//     topic emits interleave and consume from this one counter, so a topic
	//     stream sees gaps. Key on (instanceID, seq); don't assume contiguous
	//     or per-stream-monotonic seq.
	//  2. seq=0 means "pre-upgrade sender" — an instance running code that
	//     predates the Seq field omits it, so it JSON-unmarshals to 0 (Go zero
	//     value). In a mixed-version cluster during a rolling upgrade, EVERY
	//     message from such an instance has seq=0, so a naive (instanceID, 0)
	//     ring key would collapse all-but-one of that instance's messages. The
	//     Phase 3 ring MUST bypass dedup when seq==0 (process unconditionally):
	//     a pre-Phase-2 instance has no topic PSUBSCRIBE, hence no double-fire
	//     to dedup, so processing every seq=0 message is correct and safe.
	seq                atomic.Uint64
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	mu                 sync.RWMutex
	closed             bool
	reconnecting       bool           // True while reconnect() is in its sleep/SUBSCRIBE window (pubsub is nil)
	reconnectDelay     time.Duration  // Delay before reconnecting after subscription failure (default: 1s)
	subscribedChannels map[string]int // Reference counts for dynamic exact-SUBSCRIBE channels (reconnect replay + 0→1 / 1→0 SUBSCRIBE/UNSUBSCRIBE gating)
	subscribedPatterns map[string]int // Parallel to subscribedChannels for PSUBSCRIBE glob channels — replayed via PSubscribe in reconnect(), same 0→1 / 1→0 gating

	// seenRing dedups the SUBSCRIBE+PSUBSCRIBE double-fire by (instanceID, seq); touched only on the serialized processMessages goroutine ⇒ lock-free. See phase-3.md / the seq field constraints.
	seenRing seenRing
}

// seenIDRingSize: double-fire copies arrive back-to-back, so a small bounded window suffices (64 absorbs multi-instance interleave without unbounded growth).
const seenIDRingSize = 64

// seenID is the collision-free dedup key (InstanceID-scoped Seq; not Timestamp — see the GroupActionMessage doc).
type seenID struct {
	instanceID string
	seq        uint64
}

// seenRing is a fixed-size circular set; at N=64 a linear scan beats a map and the single-pump access needs no lock.
type seenRing struct {
	ids  [seenIDRingSize]seenID
	next int  // write cursor (wraps)
	full bool // true once the ring has wrapped at least once
}

// seenThenRecord returns true if id was already present; otherwise records it (evicting oldest once full) and returns false. Single-goroutine, lock-free.
func (r *seenRing) seenThenRecord(id seenID) bool {
	limit := r.next
	if r.full {
		limit = seenIDRingSize
	}
	for i := 0; i < limit; i++ {
		if r.ids[i] == id {
			return true
		}
	}
	r.ids[r.next] = id
	r.next++
	if r.next == seenIDRingSize {
		r.next = 0
		r.full = true
	}
	return false
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
		client:             client,
		instanceID:         uuid.New().String(),
		ctx:                ctx,
		cancel:             cancel,
		reconnectDelay:     1 * time.Second, // Default: 1 second
		subscribedChannels: make(map[string]int),
		subscribedPatterns: make(map[string]int),
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

	return b.publishJSON(channelGlobal, msg)
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

	channel := channelGroup + groupID
	return b.publishJSON(channel, msg)
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

	channel := channelUser + userID
	return b.publishJSON(channel, msg)
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

	return b.publishJSON(channelServerAction+userID, msg)
}

// publishJSON serializes any message as JSON and publishes it to a Redis channel.
func (b *RedisBroadcaster) publishJSON(channel string, msg interface{}) error {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()

	if closed {
		return fmt.Errorf("broadcaster is closed")
	}

	data, err := jsonutil.API.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for channel %s: %w", channel, err)
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
// Subscribe confirms the Redis subscription (blocking until Redis responds),
// then starts a background goroutine for message processing and returns.
// If Redis is unreachable and the client has no DialTimeout configured,
// this blocks indefinitely. Always configure a DialTimeout on the Redis client.
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

	// Subscribe to global channel (all broadcasts)
	// Individual group/user channels will be subscribed dynamically as needed
	b.pubsub = b.client.Subscribe(b.ctx, channelGlobal)
	b.mu.Unlock()

	// Wait for subscription confirmation
	if _, err := b.pubsub.Receive(b.ctx); err != nil {
		b.mu.Lock()
		_ = b.pubsub.Close()
		b.pubsub = nil
		b.handler = nil
		b.mu.Unlock()
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

// RegisterServerActionHandler starts listening for server action messages.
//
// The handler will be called for each received server action message.
// It should trigger the action on relevant local connections.
func (b *RedisBroadcaster) RegisterServerActionHandler(handler ServerActionHandler) error {
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

// SubscribeToServerAction subscribes to server actions for a specific user.
func (b *RedisBroadcaster) SubscribeToServerAction(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	return b.subscribeTo(channelServerAction+userID, "server action", subExact)
}

// SubscribeToGroup subscribes to broadcasts for a specific group.
func (b *RedisBroadcaster) SubscribeToGroup(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}
	return b.subscribeTo(channelGroup+groupID, "group", subExact)
}

// SubscribeToUser subscribes to broadcasts for a specific user.
func (b *RedisBroadcaster) SubscribeToUser(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	return b.subscribeTo(channelUser+userID, "user", subExact)
}

// PublishGroupAction publishes a group-scoped action to all instances.
// Each receiving instance dispatches the action on all local connections
// in the target group via their DispatchChan.
func (b *RedisBroadcaster) PublishGroupAction(groupID string, action string, data map[string]interface{}) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}
	if action == "" {
		return fmt.Errorf("action cannot be empty")
	}

	msg := &GroupActionMessage{
		Type:       "group_action",
		GroupID:    groupID,
		Action:     action,
		Data:       data,
		Seq:        b.seq.Add(1),
		Timestamp:  time.Now(),
		InstanceID: b.instanceID,
	}

	return b.publishJSON(channelGroupAction+groupID, msg)
}

// RegisterGroupActionHandler starts listening for group action messages.
func (b *RedisBroadcaster) RegisterGroupActionHandler(handler GroupActionHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	if b.groupActionHandler != nil {
		b.mu.Unlock()
		return fmt.Errorf("already subscribed to group actions")
	}
	b.groupActionHandler = handler
	b.mu.Unlock()

	slog.Info("Registered group action handler",
		slog.String("component", "redis_broadcaster"),
		slog.String("instance_id", b.instanceID))
	return nil
}

// SubscribeToGroupAction subscribes to group actions for a specific group.
func (b *RedisBroadcaster) SubscribeToGroupAction(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}
	return b.subscribeTo(channelGroupAction+groupID, "group action", subExact)
}

// PublishToTopic publishes a topic-scoped action to all instances over the
// single exact channel livetemplate:topic:{name}. The receiving instance
// resolves the connection set by Topic (not GroupID). Modeled on
// PublishGroupAction; reuses the GroupActionMessage envelope (Type
// "topic_action"). Publishers always PUBLISH to the exact channel — wildcard
// (PSUBSCRIBE) delivery is the subscriber's concern (Phase 3), never a
// publisher pattern expansion.
func (b *RedisBroadcaster) PublishToTopic(topic string, action string, data map[string]interface{}) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if action == "" {
		return fmt.Errorf("action cannot be empty")
	}

	msg := &GroupActionMessage{
		Type:       "topic_action",
		Topic:      topic,
		Action:     action,
		Data:       data,
		Seq:        b.seq.Add(1),
		Timestamp:  time.Now(),
		InstanceID: b.instanceID,
	}

	return b.publishJSON(channelTopic+topic, msg)
}

// RegisterTopicActionHandler registers the handler invoked for every received
// topic action message from other instances. Mirrors RegisterGroupActionHandler.
func (b *RedisBroadcaster) RegisterTopicActionHandler(handler GroupActionHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	if b.topicActionHandler != nil {
		b.mu.Unlock()
		return fmt.Errorf("already subscribed to topic actions")
	}
	b.topicActionHandler = handler
	b.mu.Unlock()

	slog.Info("Registered topic action handler",
		slog.String("component", "redis_broadcaster"),
		slog.String("instance_id", b.instanceID))
	return nil
}

// SubscribeToTopicChannel subscribes to the exact channel for one concrete
// topic. Reference-counted via the shared subscribedChannels map, so multiple
// local connections on the same topic share one Redis SUBSCRIBE and the
// subscription is replayed verbatim by reconnect(). Phase 2 is exact SUBSCRIBE
// only; wildcard PSUBSCRIBE is Phase 3.
func (b *RedisBroadcaster) SubscribeToTopicChannel(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	return b.subscribeTo(channelTopic+topic, "topic", subExact)
}

// UnsubscribeFromTopicChannel decrements the topic channel's refcount,
// issuing the underlying Redis UNSUBSCRIBE on the 1→0 transition.
func (b *RedisBroadcaster) UnsubscribeFromTopicChannel(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	return b.unsubscribeFrom(channelTopic+topic, "topic", subExact)
}

// SubscribeToTopicPattern issues ONE PSUBSCRIBE on the SAME b.pubsub the pump reads (never expand a pattern; never a 2nd PubSub). Refcounted via subscribedPatterns; see TopicPatternSubscriber / phase-3.md.
func (b *RedisBroadcaster) SubscribeToTopicPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	return b.subscribeTo(channelTopic+pattern, "topic pattern", subPattern)
}

// UnsubscribeFromTopicPattern decrements the pattern's refcount, issuing the
// underlying Redis PUNSUBSCRIBE on the 1→0 transition.
func (b *RedisBroadcaster) UnsubscribeFromTopicPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	return b.unsubscribeFrom(channelTopic+pattern, "topic pattern", subPattern)
}

// subscribeHook is a test-only hook invoked from trySubscribe immediately
// before the Redis SUBSCRIBE network call (and outside any lock). It allows
// tests to inject delay or block to verify that mu is not held across the
// network call. Production code leaves this nil.
//
// Access is mutex-guarded: trySubscribe runs on whatever goroutine called
// SubscribeToGroup (a test or background goroutine) and is also reached from
// reconnect()'s channel replay on the processMessages goroutine, while a test
// installs/restores it from the test goroutine, so a bare package global would
// data-race under -race. Use currentSubscribeHook / setSubscribeHook, never the
// variable directly.
var (
	subscribeHookMu sync.Mutex
	subscribeHook   func()
)

// currentSubscribeHook returns the installed test hook (nil in production).
func currentSubscribeHook() func() {
	subscribeHookMu.Lock()
	defer subscribeHookMu.Unlock()
	return subscribeHook
}

// setSubscribeHook installs h and returns the previously installed hook, so a
// test can restore it: prev := setSubscribeHook(fn); defer setSubscribeHook(prev).
func setSubscribeHook(h func()) (prev func()) {
	subscribeHookMu.Lock()
	defer subscribeHookMu.Unlock()
	prev = subscribeHook
	subscribeHook = h
	return prev
}

// reconnectHook is a test-only hook invoked from reconnect() immediately
// after the lock has been released and before the reconnect-delay sleep.
// It allows tests to deterministically wait for reconnect() to enter its
// no-lock window without relying on time-based synchronization.
// Production code leaves this nil.
//
// Access is mutex-guarded: reconnect() (running on the processMessages
// goroutine) reads it while a test installs/restores it from the test
// goroutine, so a bare package global would data-race under -race. Use
// currentReconnectHook / setReconnectHook, never the variable directly.
var (
	reconnectHookMu sync.Mutex
	reconnectHook   func()
)

// currentReconnectHook returns the installed test hook (nil in production).
func currentReconnectHook() func() {
	reconnectHookMu.Lock()
	defer reconnectHookMu.Unlock()
	return reconnectHook
}

// setReconnectHook installs h and returns the previously installed hook, so a
// test can restore it: prev := setReconnectHook(fn); defer setReconnectHook(prev).
func setReconnectHook(h func()) (prev func()) {
	reconnectHookMu.Lock()
	defer reconnectHookMu.Unlock()
	prev = reconnectHook
	reconnectHook = h
	return prev
}

// errPubsubNotReady is a sentinel signaling that pubsub is currently nil
// because reconnect() is in progress. It's distinct from "broadcaster is
// closed" (permanent) and lets subscribeTo() extend its retry window to
// cover the reconnect-delay rather than failing fast in ~200ms.
var errPubsubNotReady = fmt.Errorf("pubsub not ready (reconnect in progress)")

// subKind selects which Redis verb a (un)subscribe issues and which refcount
// map tracks it: subExact = SUBSCRIBE/UNSUBSCRIBE in subscribedChannels;
// subPattern = PSUBSCRIBE/PUNSUBSCRIBE in subscribedPatterns. It is threaded
// through the one race-critical retry/check-lock-check loop so that subtle
// lock-release-across-the-network-call logic stays single-sourced rather than
// duplicated per verb. Exact callers pass subExact explicitly — the
// group-action subscribe path is byte-for-byte unchanged, just tagged.
type subKind int

const (
	subExact   subKind = iota // Redis SUBSCRIBE  / UNSUBSCRIBE  → subscribedChannels
	subPattern                // Redis PSUBSCRIBE / PUNSUBSCRIBE → subscribedPatterns
)

// refcounts returns the refcount map for kind. The caller MUST hold b.mu (the
// maps are mutated under it everywhere else).
func (b *RedisBroadcaster) refcounts(kind subKind) map[string]int {
	if kind == subPattern {
		return b.subscribedPatterns
	}
	return b.subscribedChannels
}

// redisSubscribe issues the kind's subscribe verb on ps. PSUBSCRIBE for a
// pattern, SUBSCRIBE for an exact channel — both multiplex onto ps's single
// .Channel() the one processMessages pump reads (the single-PubSub-instance
// invariant: never a second *redis.PubSub).
func redisSubscribe(ctx context.Context, ps *redis.PubSub, kind subKind, name string) error {
	if kind == subPattern {
		return ps.PSubscribe(ctx, name)
	}
	return ps.Subscribe(ctx, name)
}

// redisUnsubscribe issues the kind's unsubscribe verb on ps (PUNSUBSCRIBE vs
// UNSUBSCRIBE).
func redisUnsubscribe(ctx context.Context, ps *redis.PubSub, kind subKind, name string) error {
	if kind == subPattern {
		return ps.PUnsubscribe(ctx, name)
	}
	return ps.Unsubscribe(ctx, name)
}

// subscribeTo subscribes to a Redis channel (exact SUBSCRIBE or, for
// kind==subPattern, PSUBSCRIBE) with dedup and retry.
//
// The lock is released across the Redis (P)SUBSCRIBE network call so concurrent
// publishes/subscribes/closes are not blocked. Retries handle transient
// Redis failures and the rare race where reconnect() swaps b.pubsub
// between our snapshot and our state-update phase (we detect the swap and
// retry against the new connection).
//
// When trySubscribe reports errPubsubNotReady (pubsub is nil because
// reconnect() is in progress), the retry budget is extended to cover the
// reconnect window so subscriptions arriving mid-reconnect succeed against
// the new connection rather than failing fast.
func (b *RedisBroadcaster) subscribeTo(channel, label string, kind subKind) error {
	const maxAttempts = 3
	const retryDelay = 100 * time.Millisecond

	// When reconnect is in progress, extend the deadline to cover the
	// full reconnect-delay (plus slack for the SUBSCRIBE/Receive round-trip
	// after the sleep). Bounded so a permanently-broken pubsub doesn't hang.
	b.mu.RLock()
	reconnectBudget := b.reconnectDelay + 2*time.Second
	b.mu.RUnlock()
	notReadyDeadline := time.Now().Add(reconnectBudget)

	// Two separate counters:
	//   * iteration: gates the inter-attempt sleep so any retry (notReady or
	//     normal-error) waits retryDelay before re-issuing trySubscribe.
	//   * normalAttempts: gates the maxAttempts break and is only incremented
	//     for non-notReady errors. Without this split, notReady spins (up to
	//     ~reconnectDelay/retryDelay of them) would silently consume the
	//     3-attempt budget, so a single real error after reconnect completes
	//     would terminate the loop and produce a misleading "after 3 attempts"
	//     error message.
	var lastErr error
	iteration := 0
	normalAttempts := 0
	for {
		if iteration > 0 {
			select {
			case <-b.ctx.Done():
				return fmt.Errorf("context cancelled while retrying %s channel subscribe: %w", label, b.ctx.Err())
			case <-time.After(retryDelay):
			}
		}
		iteration++

		err := b.trySubscribe(channel, label, kind)
		if err == nil {
			return nil
		}
		lastErr = err

		// Transient: reconnect in progress. Keep retrying until the
		// reconnect window elapses, then fall through to normal budget.
		// Does NOT consume the normalAttempts budget — those retries are
		// reserved for actual subscribe failures.
		if errors.Is(err, errPubsubNotReady) && time.Now().Before(notReadyDeadline) {
			slog.Debug("Subscribe waiting for reconnect to complete",
				slog.String("component", "redis_broadcaster"),
				slog.String("channel", channel))
			continue
		}

		normalAttempts++
		if normalAttempts >= maxAttempts {
			break
		}

		slog.Warn("Subscribe attempt failed, retrying",
			slog.String("component", "redis_broadcaster"),
			slog.String("channel", channel),
			slog.Int("attempt", normalAttempts),
			slog.Int("max_attempts", maxAttempts),
			slog.Any("error", err))
	}

	return fmt.Errorf("failed to subscribe to %s channel after %d attempts: %w", label, maxAttempts, lastErr)
}

// trySubscribe performs one subscribe attempt using a check-lock-check pattern:
//  1. Read-lock to fast-path closed/pubsub-nil checks; if the channel already
//     has a non-zero refcount, increment it under the write lock and skip the
//     network call.
//  2. Release the lock and perform the Redis SUBSCRIBE outside any lock.
//  3. Re-acquire the write lock and verify state is still valid (not closed,
//     pubsub not swapped by a concurrent reconnect) before bumping the refcount.
//
// Reference counting: every successful trySubscribe increments the channel's
// refcount, regardless of whether a Redis SUBSCRIBE was actually issued.
// The Redis SUBSCRIBE only fires on the 0→1 transition; subsequent callers
// piggyback on the existing subscription. Symmetrically, UnsubscribeDynamic
// decrements the refcount and only issues UNSUBSCRIBE on the 1→0 transition.
//
// A duplicate Redis SUBSCRIBE issued by a racing call is harmless — the
// command is idempotent on the Redis side. A pubsub-pointer swap during
// the network call is treated as a transient failure; the retry loop will
// re-snapshot and try again against the new pubsub.
func (b *RedisBroadcaster) trySubscribe(channel, label string, kind subKind) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("broadcaster is closed")
	}
	if b.pubsub == nil {
		// Distinguish two cases for the caller:
		//   * reconnecting = true  → transient (reconnect in progress); the
		//     retry loop should wait for the new pubsub.
		//   * reconnecting = false → permanent (Subscribe never called); the
		//     normal 3-attempt budget should fail fast.
		reconnecting := b.reconnecting
		b.mu.Unlock()
		if reconnecting {
			return fmt.Errorf("%s: %w", label, errPubsubNotReady)
		}
		return fmt.Errorf("not subscribed")
	}
	// Already subscribed (refcount > 0): bump the count, skip the network call.
	// The Redis subscription is shared; the next caller's UnsubscribeDynamic
	// only tears it down when the count returns to zero.
	refcounts := b.refcounts(kind)
	if refcounts[channel] > 0 {
		refcounts[channel]++
		b.mu.Unlock()
		return nil
	}
	pubsub := b.pubsub
	b.mu.Unlock()

	// One hook for both verbs: it asserts b.mu is not held across the network
	// (P)SUBSCRIBE, so the pattern path gets the same lock-release race
	// coverage the exact path has, for free.
	if h := currentSubscribeHook(); h != nil {
		h()
	}

	if err := redisSubscribe(b.ctx, pubsub, kind, channel); err != nil {
		return fmt.Errorf("failed to subscribe to %s channel: %w", label, err)
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("broadcaster is closed")
	}
	if b.pubsub != pubsub {
		b.mu.Unlock()
		return fmt.Errorf("pubsub connection changed during subscribe to %s channel", label)
	}
	// A racing trySubscribe may have completed its (P)SUBSCRIBE between our
	// release-lock and re-acquire-lock; either way the count belongs to us.
	// refcounts still points at the same reference-type map (subscribedChannels
	// or subscribedPatterns); this write is lock-held via the b.mu.Lock() above.
	refcounts[channel]++
	count := refcounts[channel]
	b.mu.Unlock()

	slog.Info("Subscribed to "+label+" channel",
		slog.String("component", "redis_broadcaster"),
		slog.String("channel", channel),
		slog.Int("refcount", count))
	return nil
}

// UnsubscribeFromGroup decrements the refcount for a group channel.
// When the refcount reaches zero, issues a Redis UNSUBSCRIBE and removes
// the channel from the tracking map. Pairs with SubscribeToGroup.
func (b *RedisBroadcaster) UnsubscribeFromGroup(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}
	return b.unsubscribeFrom(channelGroup+groupID, "group", subExact)
}

// UnsubscribeFromUser decrements the refcount for a user channel.
// Pairs with SubscribeToUser.
func (b *RedisBroadcaster) UnsubscribeFromUser(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	return b.unsubscribeFrom(channelUser+userID, "user", subExact)
}

// UnsubscribeFromServerAction decrements the refcount for a server action channel.
// Pairs with SubscribeToServerAction.
func (b *RedisBroadcaster) UnsubscribeFromServerAction(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	return b.unsubscribeFrom(channelServerAction+userID, "server action", subExact)
}

// UnsubscribeFromGroupAction decrements the refcount for a group action channel.
// Pairs with SubscribeToGroupAction.
func (b *RedisBroadcaster) UnsubscribeFromGroupAction(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}
	return b.unsubscribeFrom(channelGroupAction+groupID, "group action", subExact)
}

// unsubscribeFrom decrements the refcount for a channel. On the 1→0
// transition, it issues a Redis UNSUBSCRIBE (outside any lock) and removes
// the entry from subscribedChannels.
//
// Calls on channels with no refcount are a no-op — the channel was never
// subscribed (or has already been torn down). This makes deferred cleanup
// in the WebSocket handler robust against partial-failure setup paths.
func (b *RedisBroadcaster) unsubscribeFrom(channel, label string, kind subKind) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	refcounts := b.refcounts(kind)
	count := refcounts[channel]
	if count <= 0 {
		// Either never subscribed, or already cleared. Idempotent no-op so
		// disconnect-time cleanup is robust against earlier setup failures.
		b.mu.Unlock()
		return nil
	}
	if count > 1 {
		refcounts[channel] = count - 1
		b.mu.Unlock()
		return nil
	}
	// count == 1: clear the entry under the lock so concurrent SubscribeTo
	// observes a zero count and re-issues (P)SUBSCRIBE rather than piggybacking.
	delete(refcounts, channel)
	pubsub := b.pubsub
	b.mu.Unlock()

	if pubsub == nil {
		// pubsub may be nil while reconnect() is mid-flight. The map entry is
		// already gone, so reconnect()'s replay won't include this channel.
		// Nothing to send to Redis — the broadcaster will simply not re-SUBSCRIBE
		// on reconnect, which is the desired terminal state.
		slog.Debug("Refcount reached 0 while pubsub unavailable; skipping Redis UNSUBSCRIBE",
			slog.String("component", "redis_broadcaster"),
			slog.String("channel", channel))
		return nil
	}

	if err := redisUnsubscribe(b.ctx, pubsub, kind, channel); err != nil {
		return fmt.Errorf("failed to unsubscribe from %s channel: %w", label, err)
	}

	slog.Info("Unsubscribed from "+label+" channel",
		slog.String("component", "redis_broadcaster"),
		slog.String("channel", channel))
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
	if err := jsonutil.API.Unmarshal([]byte(redisMsg.Payload), &typeCheck); err != nil {
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
	case "group_action":
		return b.handleGroupActionMessage(redisMsg)
	case "topic_action":
		return b.handleTopicActionMessage(redisMsg)
	default:
		return b.handleBroadcastMessage(redisMsg)
	}
}

// handleBroadcastMessage processes a broadcast message.
func (b *RedisBroadcaster) handleBroadcastMessage(redisMsg *redis.Message) error {
	var msg BroadcastMessage
	if err := jsonutil.API.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
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
	return dispatchTypedMessage(redisMsg, func() func(*ServerActionMessage) error {
		b.mu.RLock()
		defer b.mu.RUnlock()
		return b.serverActionHandler
	})
}

// handleGroupActionMessage processes a group action message.
//
// handleMessage already did a partial parse (Type + InstanceID) to route and
// dropped own-instance messages. This does a fresh FULL unmarshal of the same
// payload — the established route-then-decode pattern shared by the
// server/broadcast/topic handlers (a small, deliberate cost on the
// cross-instance path; not refactored to a single decode here because the
// group-action dispatch path is shared with the pub/sub-topic rollout and
// kept stable). The msg.InstanceID re-check is a cheap redundant guard against
// the already-decoded struct (no separate unmarshal for the guard itself).
func (b *RedisBroadcaster) handleGroupActionMessage(redisMsg *redis.Message) error {
	var msg GroupActionMessage
	if err := jsonutil.API.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal group action message: %w", err)
	}
	if msg.InstanceID == b.instanceID {
		return nil
	}

	b.mu.RLock()
	handler := b.groupActionHandler
	b.mu.RUnlock()

	if handler == nil {
		slog.Warn("No group action handler registered, ignoring message",
			slog.String("component", "redis_broadcaster"))
		return nil
	}

	return handler(&msg)
}

// handleTopicActionMessage processes a topic action message. Mirrors
// handleGroupActionMessage (same envelope, same handler signature, same
// route-then-decode pattern: handleMessage partial-parsed Type+InstanceID and
// dropped own-instance messages; this does a fresh full unmarshal, and the
// msg.InstanceID re-check is a cheap redundant guard against the decoded
// struct — no separate unmarshal for the guard). The receiving handler
// resolves the connection set by msg.Topic.
//
// Phase 3 (instanceID, seq) double-fire dedup runs HERE — on the single
// serialized processMessages goroutine, BEFORE the handler does registry
// resolution (proposal §"Cross-instance exactly-once": "inside the existing
// single processMessages pump … before registry resolution/enqueue"). With
// PSUBSCRIBE, a connection holding both an exact SUBSCRIBE and a matching
// pattern PSUBSCRIBE makes Redis deliver one PUBLISH twice to this instance;
// the ring drops the second copy. Group-action messages never reach here
// (routed to handleGroupActionMessage) and have no PSUBSCRIBE, so the shared
// seq counter is only ever compared on this topic path — the LOAD-BEARING
// INVARIANT on the GroupActionMessage Seq field.
func (b *RedisBroadcaster) handleTopicActionMessage(redisMsg *redis.Message) error {
	var msg GroupActionMessage
	if err := jsonutil.API.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal topic action message: %w", err)
	}
	if msg.InstanceID == b.instanceID {
		return nil
	}

	// (instanceID, seq) double-fire dedup. seq==0 ⇒ pre-Phase-2/pre-upgrade
	// sender that omits Seq (JSON→0): EVERY message from it has seq=0 and it
	// has no topic PSUBSCRIBE (no double-fire), so the ring is bypassed
	// ENTIRELY — neither dedup-checked (process unconditionally) NOR recorded
	// (recording (id,0) would collapse all-but-one of that instance's messages
	// once a second seq=0 arrives). Short-circuit && enforces both halves:
	// seenThenRecord is not even called when msg.Seq==0. Lock-free: the ring is
	// touched only on this one serialized pump goroutine.
	if msg.Seq != 0 && b.seenRing.seenThenRecord(seenID{instanceID: msg.InstanceID, seq: msg.Seq}) {
		slog.Debug("Dropped double-fire topic message (SUBSCRIBE+PSUBSCRIBE)",
			slog.String("component", "redis_broadcaster"),
			slog.String("topic", msg.Topic),
			slog.String("from_instance", msg.InstanceID),
			slog.Uint64("seq", msg.Seq))
		return nil
	}

	b.mu.RLock()
	handler := b.topicActionHandler
	b.mu.RUnlock()

	if handler == nil {
		slog.Warn("No topic action handler registered, ignoring message",
			slog.String("component", "redis_broadcaster"))
		return nil
	}

	return handler(&msg)
}

// dispatchTypedMessage unmarshals a Redis message into a typed struct, retrieves
// the handler via getHandler, and calls it. Used by handleServerActionMessage
// and handleGroupActionMessage to avoid repeating the unmarshal+dispatch pattern.
func dispatchTypedMessage[T any](redisMsg *redis.Message, getHandler func() func(*T) error) error {
	var msg T
	if err := jsonutil.API.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	handler := getHandler()
	if handler == nil {
		slog.Warn("No handler registered, ignoring message",
			slog.String("component", "redis_broadcaster"))
		return nil
	}

	return handler(&msg)
}

// reconnect attempts to re-establish the Redis subscription after a failure.
// It replays all dynamic channel subscriptions (exact SUBSCRIBE) AND all
// pattern subscriptions (PSUBSCRIBE) that were active before disconnection,
// both onto the one new *redis.PubSub the pump will read.
//
// The lock is released across the reconnect-delay sleep and across the
// Redis SUBSCRIBE/Receive network calls so concurrent publishes,
// subscribes, and Close() are not blocked for the duration. The flow is:
//  1. Lock to close the stale pubsub, set b.reconnecting = true, and
//     snapshot the channel + pattern lists.
//  2. Release the lock; sleep (interruptible via b.ctx.Done()) so Close()
//     can short-circuit a long reconnectDelay.
//  3. Issue Redis SUBSCRIBE + Receive (connectivity smoke-test) outside any
//     lock, then PSUBSCRIBE the snapshotted patterns onto the SAME new
//     PubSub. No second Receive: adding a pattern to a PubSub is the same
//     fire-then-.Channel()-drains-the-confirmation model as the live
//     dynamic-add path in trySubscribe (which also issues no Receive); the
//     single Receive exists only to fail fast if the connection is dead.
//  4. Re-acquire the lock; if Close() raced ahead, tear down the new
//     pubsub. Otherwise install it as the live one.
//  5. On every exit path, clear b.reconnecting via deferred unlock so
//     a stalled reconnect doesn't trap concurrent subscribeTo callers.
//
// While b.reconnecting is true, trySubscribe distinguishes "transient
// pubsub-nil" from "permanent pubsub-nil" via errPubsubNotReady so
// dynamic subscribes arriving mid-reconnect can wait for the new
// connection rather than failing fast inside their normal retry budget.
func (b *RedisBroadcaster) reconnect() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("broadcaster is closed")
	}

	// Close old subscription under lock and mark reconnect-in-progress so
	// concurrent trySubscribe calls see "transient unavailable" rather than
	// "permanent failure" while pubsub is nil.
	if b.pubsub != nil {
		_ = b.pubsub.Close()
		b.pubsub = nil
	}
	b.reconnecting = true

	// Snapshot channels (global + all dynamic exact channels) and patterns
	// (PSUBSCRIBE globs) to replay onto the one new PubSub.
	channels := make([]string, 0, 1+len(b.subscribedChannels))
	channels = append(channels, channelGlobal)
	for ch := range b.subscribedChannels {
		channels = append(channels, ch)
	}
	patterns := make([]string, 0, len(b.subscribedPatterns))
	for p := range b.subscribedPatterns {
		patterns = append(patterns, p)
	}
	delay := b.reconnectDelay
	b.mu.Unlock()

	// Clear the reconnecting flag on every exit path (success, sleep cancel,
	// Subscribe/Receive failure, closed-during-install) so subscribeTo doesn't
	// hang waiting for a reconnect that has already aborted.
	defer func() {
		b.mu.Lock()
		b.reconnecting = false
		b.mu.Unlock()
	}()

	if h := currentReconnectHook(); h != nil {
		h()
	}

	// Wait before reconnecting (interruptible so Close() doesn't block for delay)
	select {
	case <-b.ctx.Done():
		return fmt.Errorf("context cancelled before reconnect: %w", b.ctx.Err())
	case <-time.After(delay):
	}

	// Re-subscribe to all exact channels at once (outside any lock); the
	// single Receive is the connectivity smoke-test.
	newPubSub := b.client.Subscribe(b.ctx, channels...)
	if _, err := newPubSub.Receive(b.ctx); err != nil {
		_ = newPubSub.Close()
		return fmt.Errorf("failed to resubscribe: %w", err)
	}

	// Replay patterns onto the SAME new PubSub; no extra Receive — the
	// .Channel() pump drains the psubscribe confirmation (go-redis v9.17.2;
	// re-verify this if the go.mod go-redis version bumps). Failure → tear
	// down so the caller re-drives reconnect, as for Subscribe above.
	if len(patterns) > 0 {
		if err := newPubSub.PSubscribe(b.ctx, patterns...); err != nil {
			_ = newPubSub.Close()
			return fmt.Errorf("failed to re-psubscribe patterns: %w", err)
		}
	}

	// Install new pubsub under lock
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		_ = newPubSub.Close()
		return fmt.Errorf("broadcaster is closed")
	}
	b.pubsub = newPubSub
	dynamicCount := len(b.subscribedChannels)
	patternCount := len(b.subscribedPatterns)
	b.mu.Unlock()

	slog.Info("Reconnected successfully",
		slog.String("component", "redis_broadcaster"),
		slog.Int("dynamic_channels", dynamicCount),
		slog.Int("dynamic_patterns", patternCount))
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

	// Close pubsub with write lock.
	// Safe to nil subscribedChannels/subscribedPatterns: b.closed is already
	// true (set above), so subscribeTo() and reconnect() will return early
	// before accessing the maps.
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pubsub != nil {
		if err := b.pubsub.Close(); err != nil {
			return fmt.Errorf("failed to close pubsub: %w", err)
		}
		b.pubsub = nil
	}
	b.subscribedChannels = nil
	b.subscribedPatterns = nil

	slog.Info("Closed",
		slog.String("component", "redis_broadcaster"),
		slog.String("instance_id", b.instanceID))
	return nil
}
