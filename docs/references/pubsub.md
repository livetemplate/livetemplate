# PubSub Reference

Cross-instance messaging for horizontally scaled deployments.

For server-initiated actions, see [Server Actions](server-actions.md). For scaling tiers and Redis configuration, see [Scaling Guide](../guides/SCALING.md).

## Overview

In a single-instance deployment, all WebSocket connections live in the same process. State changes and `ctx.Publish` peer fan-out are delivered directly via the in-memory connection registry.

In multi-instance deployments, a user's connections may be spread across different servers. The `pubsub` package provides cross-instance messaging via Redis Pub/Sub so that `ctx.Publish` calls, group updates, and server actions reach all relevant connections regardless of which instance they're on.

**When you need it:** Any deployment with 2+ application instances behind a load balancer.

## Setup

```go
import (
    "time"

    "github.com/livetemplate/livetemplate"
    "github.com/livetemplate/livetemplate/pubsub"
    "github.com/redis/go-redis/v9"
)

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

broadcaster := pubsub.NewRedisBroadcaster(client,
    pubsub.WithReconnectDelay(5*time.Second),
)
defer broadcaster.Close()

tmpl := livetemplate.New("app",
    livetemplate.WithPubSubBroadcaster(broadcaster),
)
```

With PubSub configured, `TriggerAction()` automatically publishes to Redis so all instances can update their local connections for the user:

```go
// Instance 1: User connects here
session.TriggerAction("update", nil)

// Instance 2: If user has tabs here, they also receive the update
// (Happens transparently via Redis PubSub)
```

No code changes needed in your controllers.

## Broadcaster Interface

The core interface that all broadcaster implementations must satisfy:

```go
type Broadcaster interface {
    PublishGlobal(payload []byte) error
    PublishToGroup(groupID string, payload []byte) error
    PublishToUser(userID string, payload []byte) error
    PublishServerAction(userID string, action string, data map[string]interface{}) error
    Subscribe(handler MessageHandler) error
    SubscribeServerActions(handler ServerActionHandler) error
    Close() error
}
```

`RedisBroadcaster` is the built-in implementation using Redis Pub/Sub.

## DynamicSubscriber Interface

An optional interface for broadcasters that support per-scope channel subscriptions:

```go
type DynamicSubscriber interface {
    SubscribeToGroup(groupID string) error
    SubscribeToUser(userID string) error
    SubscribeToServerAction(userID string) error
}
```

`RedisBroadcaster` implements both `Broadcaster` and `DynamicSubscriber`.

**How it works:** During WebSocket connection setup, the handler type-asserts the broadcaster:

```go
if ds, ok := broadcaster.(pubsub.DynamicSubscriber); ok {
    ds.SubscribeToGroup(groupID)
    ds.SubscribeToUser(userID)
    ds.SubscribeToServerAction(userID)
}
```

This happens automatically — no application code needed. If the broadcaster doesn't implement `DynamicSubscriber` (e.g., a custom implementation that handles channel management differently), the assertion returns false and subscriptions are skipped.

## Broadcast Scopes

| Method | Scope | Use Case |
|--------|-------|----------|
| `PublishGlobal(payload)` | All connections | System announcements |
| `PublishToGroup(groupID, payload)` | Session group | Collaborative editing |
| `PublishToUser(userID, payload)` | All user's connections | Notifications |
| `PublishServerAction(userID, action, data)` | User's action handler | Server-initiated actions |
| `PublishGroupAction(groupID, action, data)` | Group's action handler | Cross-instance fan-out backing `Session.TriggerAction` — invoked by `localSession.TriggerAction` for server-initiated actions, independent of any user-level Publish |
| `PublishToTopic(topic, msg)` | Topic subscribers | Cross-instance leg of `ctx.Publish` / `handler.Publish` — relayed only for topics the local instance has subscribers for, exact-match `SUBSCRIBE` or wildcard `PSUBSCRIBE` |

`GroupActionMessage` is used by `Session.TriggerAction` for cross-instance delivery of server-initiated actions (the `(groupID, action, data)` triple is mirrored verbatim). Unlike `ServerActionMessage` (user-scoped), it targets all connections in a session group. Each receiving instance dispatches the action on local connections via their event loop. `ctx.Publish` and `handler.Publish` ride a parallel `TopicActionMessage` channel (`PublishToTopic`) — the wire carries a `Topic` field on the envelope, the `Seq` field provides per-instance monotonic ordering, and a bounded in-process `(instanceID, seq)` seen-ring deduplicates the SUBSCRIBE+PSUBSCRIBE double-fire for cross-instance wildcard topics.

## Redis Channel Schema

Each scope maps to a dedicated Redis Pub/Sub channel:

| Channel Pattern | Scope | Description |
|----------------|-------|-------------|
| `livetemplate:broadcast:global` | Global | All instances subscribe at startup |
| `livetemplate:broadcast:group:{groupID}` | Group | Subscribed when a connection joins a group |
| `livetemplate:broadcast:user:{userID}` | User | Subscribed when an authenticated user connects |
| `livetemplate:action:user:{userID}` | ServerAction | Subscribed when an authenticated user connects |
| `livetemplate:groupaction:group:{groupID}` | GroupAction | Subscribed when a connection joins a group |

Per-scope channels provide **transport-level data isolation**: an instance only receives messages for groups and users it has active connections for.

## Subscription Lifecycle

### On WebSocket Connect

When a WebSocket connection is established, the handler automatically subscribes to the relevant Redis channels:

1. **Group channel** — always (every connection has a groupID)
2. **User channel** — if authenticated (userID is non-empty)
3. **Server action channel** — if authenticated

### Deduplication

Multiple connections can share the same groupID (e.g., multiple tabs in the same browser session) or userID (e.g., multiple devices). The `subscribedChannels` map inside `RedisBroadcaster` tracks active subscriptions and prevents duplicate Redis `SUBSCRIBE` calls. Calling `SubscribeToGroup("g1")` ten times results in only one Redis subscription.

### Reconnect Replay

If the Redis connection drops, `RedisBroadcaster` automatically reconnects and replays all tracked subscriptions atomically. The `subscribedChannels` map serves double duty: dedup during normal operation, and replay source during reconnection.

### No Unsubscribe

When the last connection for a group or user disconnects, the Redis subscription is **not** removed. This is by design:

- **Harmless**: When a message arrives for a group with no local connections, `GetByGroup()` returns an empty slice and the fan-out loop is a no-op
- **Self-healing**: Instance restarts (deploys) clear all stale subscriptions
- **Simpler**: No reference counting or coordinated unsubscribe logic needed

This is a known trade-off that can be optimized with reference counting in the future if Redis subscription cardinality becomes a concern.

## Data Isolation

LiveTemplate uses two independent isolation models:

### Session Isolation (State Boundaries)

Handled by the session store and connection registry. All connections with the same `groupID` share the same state instance. Different groups have completely separate state. This is unaffected by pubsub. See [Multi-Session Isolation](../design/multi-session-isolation.md) for details.

### Message Routing Isolation (PubSub)

Handled by per-scope Redis channels and application-layer filtering. Two layers provide defense-in-depth:

| Layer | Mechanism | Protects Against |
|-------|-----------|-----------------|
| **Transport** | Per-scope Redis channels | Instance only receives messages for its active groups/users. Limits exposure in memory dumps, debug logs, and telemetry. |
| **Application** | `registry.GetByGroup()` / `GetByUser()` exact-match lookups | Only connections belonging to the target group/user receive the message. Prevents delivery to wrong connections. |

Neither layer alone is sufficient. Transport isolation limits what data *reaches* a process. Application filtering limits what data *leaves* a process to end users.

## Topic Subscribe / Publish API

`ctx.Subscribe(topic)` opts the calling connection into a topic; `ctx.Publish(topic, action, data)` fans out a named action to every connection subscribed to that topic. The complete primer is in the [Controller+State Pattern reference](controller-pattern.md#cross-tab-updates-with-subscribe--publish); what follows is the wire-level and operator contract.

### Topic Grammar

Two namespaces, each with its own validator:

- **Reserved namespace** (`lvt:` prefix) — the framework owns these. `Subscribe` accepts a reserved-namespace topic only on **exact equality** to the caller's `SelfTopic()`; any other `lvt:` string is rejected (anti-spoof). `SelfTopic()` itself resolves to `lvt:session:<groupID>` and is ACL-exempt.
- **Developer namespace** (everything else) — segments matching `[a-zA-Z0-9_-]+` or the literal `*`, separated by `/`. Examples: `room/lobby`, `room/*/log`, `*/alice`. Single-segment and multi-segment wildcards are both supported. Developer topics run through `WithTopicACL` and are deny-by-default.

**Patterns are Subscribe-only.** Wildcards (`*` segments) can be passed to `Subscribe` to receive any matching concrete topic, but `Publish("room/*", ...)` is a hard error: the matcher would have no concrete topic to fan out to. The runtime rejects pattern-Publishes with a clean error rather than silently swallowing them.

### Cross-Instance Exactly-Once

Cross-instance topic delivery rides Redis. Two ordering / dedup constraints to know about:

- **`Seq` is monotonic per-instance, not per-Type.** A single counter increments on every `GroupActionMessage` emit (both group-action and topic flows), so the seen-ring keys on `(instanceID, seq)` without assuming contiguity per message type.
- **`seq == 0` ⇒ pre-upgrade sender.** A rolling-upgrade instance running pre-Phase-2 code omits the `Seq` field (JSON unmarshal → 0); the seen-ring **bypasses dedup** when `seq == 0` (process unconditionally). A pre-Phase-2 instance has no topic `PSUBSCRIBE`, hence no double-fire, so unconditional processing is correct. A naive `(instanceID, 0)` key would collapse all-but-one of an old instance's messages — the bypass exists precisely to prevent that.
- **Redis `*` spans `/`.** PSUBSCRIBE over-delivers (e.g. `room/*` matches `room/alice/log` at the Redis level). The framework re-applies the strict whole-segment `segmentMatch` in `handleTopicActionMessage` before dispatching to local subscribers — transparent to controllers, but the reason cross-instance wildcard topics are exactly-once.

### Client Error Envelope (`lvt:error`)

A controller that propagates a `*TopicForbiddenError` from `Mount` on the WS-connect path causes the server to:

1. Send a wire-level error envelope: `{"type":"error","code":"topic_forbidden","topic":"<denied topic>"}`
2. Log a structured `slog.Warn` (`"Mount Subscribe denied by topic ACL; surfaced to client, connection kept open"`) with the topic + error attributes
3. Adopt the controller's returned `newState` and **fall through to the shared success-path lifecycle** — `persistState`, `OnConnect`, drain pending publishes, send initial tree. The WS stays open and functional; no auto-reconnect storm

The TypeScript client (`@livetemplate/client` v0.9.0+) sees the envelope as a discriminator-first `type === "error"` branch in `handleWebSocketPayload` and dispatches a `CustomEvent` on the `[data-lvt-id]` wrapper element:

```ts
new CustomEvent("lvt:error", {
  detail: { code: "topic_forbidden", topic: "<denied topic>" },
  bubbles: false,
});
```

**Listening from application code.** Because the event does not bubble, listeners must register on the wrapper directly — or use a capture-phase listener on `document`/`window` to observe wrapper-targeted events without attaching per-wrapper:

```js
// Capture phase observes the dispatch on the wrapper even though bubbles:false.
document.addEventListener("lvt:error", (event) => {
  const { code, topic } = event.detail;
  if (code === "topic_forbidden") {
    // Render a toast, route to a fallback page, etc.
  }
}, true); // <-- capture: true
```

**Two `lvt:error` events share the name by design — they never collide.** The form-lifecycle manager (`state/form-lifecycle-manager.ts`) also dispatches a `lvt:error` event, but with a *different target* (the `<form>` element, not the wrapper) and a *different detail shape* (`ResponseMetadata`, not `{code, topic}`). Both are non-bubbling, so each event is observable only by listeners attached to its specific target (or in capture phase, observable on the path to that target). A `grep lvt:error` across application code will surface both call sites; verify the listener's target before treating one as the other.

**Envelope is emitted only on propagated error.** A controller that swallows the denied Subscribe — `_ = ctx.Subscribe("denied"); return s, nil` — produces **no envelope**, no Warn, no `lvt:error`. The propagation is the signal.

**HTTP GET path does not get `lvt:error`.** The keep-open lifecycle is the WS-connect-path Mount only. A denied Subscribe on the HTTP GET path surfaces as HTTP 500 (pre-existing Phase-1 behavior). The [`IsInitialMount` guard pattern](controller-pattern.md#subscribing-to-acl-gated-developer-topics) is how controllers avoid the 500 — skip the gated Subscribe on the initial GET so the WS can exercise keep-open.

### Out-of-Band `handler.Publish`

`Template.Handle()` returns a `LiveHandler` with its own `Publish(topic, action, data)` method — the trusted-server-code analogue of `ctx.Publish`. Used to push topic-scoped fan-outs from outside an action handler (cron jobs, webhook handlers, in-process queues).

**No symmetry-collision warning.** `ctx.Publish` runs a `slog.Warn` when the published action name collides with a client-wired action (the dispatch-symmetry hazard — see the proposal §"Design constraints"). `handler.Publish` deliberately **does not** emit the same warning: there is no per-Context template binding to resolve the wired-name set against, the caller is trusted server code, and the parser-derived wired-action set is template-scoped while `handler.Publish` is global. Documented as a deliberate gap, not a missed case.

## Operator Contracts

Two failure modes operators need to know about, neither of which is a code-change item — both surface in logs only:

**`SubscribeToTopicActions` init failure at `Handle()` ⇒ cross-instance topic-receive leg dead for that instance.** The handler subscribes to the `livetemplate:topic_action:*` channel at startup (in `Template.Handle()`). If that subscribe fails (Redis unreachable, network issue, etc.) the failure is **logged-only and the handler continues** — identical to the pre-existing `SubscribeGroupActions` / `SubscribeServerActions` init pattern at the same site (deliberately consistent). The local instance keeps serving HTTP and WS, but **every cross-instance topic Publish from other instances is silently dropped** for that instance until restart. Per-topic channel-subscribe failures only break one topic; this *init* failure breaks all topics.

Grep for `event=topic_action_subscribe_failed` in production logs (the structured slog attribute emitted by `template.go` alongside the `Failed to subscribe to topic actions` ERROR); treat a hit as a deploy-blocking alarm on multi-instance setups.

**`Publish` local fan-out is non-blocking, drops on a full per-connection buffer.** Same backpressure model as `Session.TriggerAction` (and as the pre-v0.10.0 peer-fan-out API it replaced). Surfaced via the existing `wsBufferFull` / `wsSlowClientCloses` metrics; tuned via `WithWebSocketBufferSize` / `LVT_WS_BUFFER_SIZE`. Not a regression — accepted pre-existing model.

## Troubleshooting

**Messages not received cross-instance:**
- Verify `WithPubSubBroadcaster(broadcaster)` is configured
- Check Redis connectivity from all instances
- Confirm instances use the same Redis server/cluster

**Subscriptions lost after Redis reconnection:**
- Should auto-recover via reconnect replay. Check logs for `"Reconnected successfully"` with `dynamic_channels` count.
- If `dynamic_channels=0`, subscriptions were never established — verify WebSocket connections are being set up correctly.

**High Redis Pub/Sub memory:**
- Each instance subscribes only to channels for its active groups/users
- Stale subscriptions (groups/users that disconnected) are harmless but accumulate until restart
- Monitor with `redis-cli pubsub numsub livetemplate:broadcast:global`

## See Also

- [Server Actions Reference](server-actions.md) — `TriggerAction` API
- [Session Reference](session.md) — Session stores and connection management
- [Multi-Session Isolation](../design/multi-session-isolation.md) — State isolation model
- [Scaling Guide](../guides/SCALING.md) — Redis configuration and scaling tiers
- [Configuration Reference](CONFIGURATION.md) — Environment variables and WebSocket settings
