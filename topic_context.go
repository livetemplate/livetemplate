package livetemplate

import (
	"fmt"
	"log/slog"
)

// topicSubscriber is the handler-injected hook that ctx.Subscribe / Unsubscribe
// / Publish use to reach the topic ACL, the connection registry, and the
// client-wired-action set. It is injected via WithTopicSubscriber by the mount
// handler at every Context-build site (mirrors how Session / FlashSetter are
// injected). A nil topicSubscriber means topic operations are unavailable for
// this Context (e.g. a hand-built Context in a unit test): Subscribe returns an
// error instead of panicking, and Publish still queues (it is transport- and
// handler-agnostic) but skips the best-effort symmetry-collision warning.
type topicSubscriber interface {
	// checkTopicACL runs the configured ACL for a developer topic. Returns nil
	// if allowed, ErrTopicForbidden (possibly wrapped with the hook's
	// explanatory error) if denied or under the deny-all default. Never called
	// for ctx.SelfTopic() (ACL-exempt) or for a reserved lvt: topic.
	checkTopicACL(topic, userID string) error
	// registerTopic adds this Context's connection to the topic index. No-op
	// when no Connection exists yet (HTTP GET / POST): the subscription only
	// materializes with a WebSocket Connection. The ACL has already run
	// eagerly by the time this is called.
	registerTopic(topic string)
	// unregisterTopic removes this Context's connection from the topic index
	// (no-op when no Connection exists).
	unregisterTopic(topic string)
	// shouldWarnWiredCollision reports whether ctx.Publish should emit the
	// symmetry-collision warning for action — true iff action is wired to a
	// client element (form/button name=, lvt-on:) AND this is its first
	// Publish (deduped app-global per template, so a repeated Publish does
	// not flood the log). Proposal §"Design constraints" — Dispatch symmetry.
	shouldWarnWiredCollision(action string) bool
}

// topicPublish is a deferred Publish, drained after the action like
// broadcastRequest (the queue it mirrors). Topic carries the publish target so
// the handler resolves the subscriber set by topic (never the GroupID field).
type topicPublish struct {
	Topic  string
	Action string
	Data   map[string]interface{}
}

// WithTopicSubscriber returns a new Context wired to the given topic
// subscriber. Injected by the mount handler at every Context-build site.
func (c *Context) WithTopicSubscriber(ts topicSubscriber) *Context {
	newCtx := *c
	newCtx.topicSub = ts
	return &newCtx
}

// Subscribe subscribes this connection to topic. Topic fan-out is the single
// explicit publish/subscribe mechanism; nothing fans out unless you Publish.
//
// Reconnect durability (read before choosing where to call this): Subscribe is
// server-driven (the client sends no subscribe message). On WS reconnect the
// server re-runs Mount, which re-subscribes — so a reconnect-durable
// subscription MUST be established in Mount (derived from persisted/route
// state). Actions do not re-run on reconnect; a Subscribe issued only inside an
// action is connection-lifetime-only and is silently lost after a WS drop.
//
// On a plain HTTP GET the subscription itself is a no-op (it only materializes
// with a WebSocket Connection) but the ACL still runs eagerly — an unauthorized
// subscribe is rejected before the WS upgrade, surfacing on the initial page
// render, not the upgrade.
//
// Validation order (proposal §2): (1) reserved lvt: namespace — admit ONLY the
// caller's own SelfTopic(), by exact string equality (anti-spoof: a connection
// cannot subscribe to another identity's lvt:user:<x>); SelfTopic() is then
// ACL-exempt. (2) developer-topic segment grammar (never applied to lvt:
// topics). (3) the ACL (deny-all default; only SelfTopic() is exempt).
//
// Canonical Mount patterns:
//
//	// Required gated topic: propagate — unauthorized users get a failed
//	// render, which is the intended access control.
//	if err := ctx.Subscribe("room/" + s.RoomID); err != nil { return s, err }
//	// Self-sync: SelfTopic() is ACL-exempt, so the return is safe to ignore;
//	// the one non-ACL failure (empty SelfTopic() from a misimplemented
//	// Authenticator) is logged loudly via slog.Error at the SelfTopic() site.
//	// That empty-topic error is NOT ErrTopicForbidden — an
//	// errors.Is(err, ErrTopicForbidden) check will not catch it.
//	_ = ctx.Subscribe(ctx.SelfTopic())
//
// Ignoring a *gated* topic's error silently drops the subscription (the
// connection just never receives that topic) — always propagate gated errors.
func (c *Context) Subscribe(topic string) error {
	if topic == "" {
		return fmt.Errorf("livetemplate: cannot Subscribe to an empty topic")
	}
	if c.topicSub == nil {
		return fmt.Errorf("livetemplate: Subscribe is unavailable on this Context (no topic subscriber wired)")
	}

	// (1) Reserved lvt: namespace — anti-spoof. Admit ONLY the caller's own
	// SelfTopic(), by *exact* string equality; reject every other lvt: string
	// (prefix-equality would let lvt:user:alice* capture lvt:user:alice2). A
	// successful match is ACL-exempt, so step (3) is skipped.
	if isReservedTopic(topic) {
		self := c.SelfTopic()
		if self == "" || topic != self {
			return fmt.Errorf("livetemplate: cannot Subscribe to reserved topic %q (only your own ctx.SelfTopic() is permitted in the %q namespace)", topic, topicReservedPrefix)
		}
		c.topicSub.registerTopic(topic)
		return nil
	}

	// (2) Developer-topic segment grammar (never applied to lvt: topics — the
	// grammar excludes ':').
	if err := validateDeveloperTopic(topic); err != nil {
		return err
	}

	// (3) ACL — deny-all default; only SelfTopic() exempt (handled above).
	if err := c.topicSub.checkTopicACL(topic, c.userID); err != nil {
		return err
	}

	c.topicSub.registerTopic(topic)
	return nil
}

// Unsubscribe removes this connection's subscription to topic. No-op if not
// subscribed or if no Connection exists. Unlike Subscribe it runs no ACL — you
// may always stop receiving. Useful for immediate revocation from a
// server-side action.
func (c *Context) Unsubscribe(topic string) {
	if topic == "" || c.topicSub == nil {
		return
	}
	c.topicSub.unregisterTopic(topic)
}

// Publish sends (action, data) to every connection subscribed to topic (local
// + cross-instance), each running action against its own state via the same
// resolver user actions use. The calling connection is excluded by default.
//
// Publish runs NO ACL — send and receive are independent (proposal §3): the
// Subscribe-time ACL gates who reads. Reserved lvt: topics are permitted on the
// send side WITHOUT a SelfTopic()-equality check (anti-spoof is a
// Subscribe-side rule; publishing to lvt:user:alice only delivers to alice's
// own subscribers, granting the publisher no read access). Developer (non-lvt:)
// topics still must satisfy the segment grammar.
//
// Mount asymmetry (footgun): unlike Subscribe, Publish is NOT a no-op on an
// HTTP GET — it targets a topic's existing subscribers and is
// transport-agnostic. Because Mount runs on every GET (and POST), an unguarded
// Publish in Mount fans out on every page load. Guard side-effecting publishes
// in Mount with ctx.IsInitialMount() / ctx.IsReconnect().
//
// A Publish issued from inside a dispatched/server-initiated action is
// logged-and-dropped (one-hop recursion guard) to prevent fan-out storms.
//
// Ordering caveat (same shallow-copy footgun as ctx.BroadcastAction, per
// CLAUDE.md): ctx.With*() returns a shallow copy and the topicPubs slice
// diverges once it reallocates, so call Publish AFTER all With*() calls or a
// publish queued before the copy won't be drained.
func (c *Context) Publish(topic, action string, data map[string]interface{}) error {
	if topic == "" {
		return fmt.Errorf("livetemplate: cannot Publish to an empty topic")
	}
	if action == "" {
		return fmt.Errorf("livetemplate: cannot Publish with an empty action")
	}
	// Publishers publish to a CONCRETE topic; "*" is Subscribe-only. The
	// developer grammar permits "*" segments (it is general-case so the matcher
	// stays general), so a mistaken Publish("room/*", …) would pass
	// validateDeveloperTopic and then panic GetByTopicExcept ("concrete topic
	// must not contain \"*\""). Reject it here with a clear, non-panic error —
	// the footgun is newly plausible in Phase 3 now that "room/*" is a
	// first-class Subscribe target.
	if isPatternTopic(topic) {
		return fmt.Errorf("livetemplate: cannot Publish to wildcard pattern %q — publish to a concrete topic; patterns are Subscribe-only", topic)
	}
	if !isReservedTopic(topic) {
		if err := validateDeveloperTopic(topic); err != nil {
			return err
		}
	}
	if len(c.topicPubs) >= MaxBroadcastsPerAction {
		slog.Error("Publish cap reached, dropping",
			slog.String("topic", topic),
			slog.String("action", action),
			slog.Int("limit", MaxBroadcastsPerAction))
		return fmt.Errorf("livetemplate: Publish cap (%d) reached for this action", MaxBroadcastsPerAction)
	}

	// Topic and user-action dispatch share one resolver, so Publishing an
	// action name a client element is wired to runs that handler on every
	// peer — warn, don't block (proposal §"Design constraints").
	if c.topicSub != nil && c.topicSub.shouldWarnWiredCollision(action) {
		slog.Warn("Publish action name collides with a client-wired action",
			slog.String("action", action),
			slog.String("topic", topic))
	}

	c.topicPubs = append(c.topicPubs, topicPublish{Topic: topic, Action: action, Data: data})
	return nil
}

// pendingTopicPublishes returns and clears pending Publish requests. Drained by
// the mount handler at the post-action processBroadcasts site, after
// persistState, so a reconciler that re-reads the group-keyed session store
// sees the originator's committed write (persist-before-publish ordering).
func (c *Context) pendingTopicPublishes() []topicPublish {
	p := c.topicPubs
	c.topicPubs = nil
	return p
}
