# PubSub Reference

Cross-instance messaging for horizontally scaled deployments.

For server-initiated actions, see [Server Actions](server-actions.md). For scaling tiers and Redis configuration, see [Scaling Guide](../guides/SCALING.md).

## Overview

In a single-instance deployment, all WebSocket connections live in the same process. State changes and broadcasts are delivered directly via the in-memory connection registry.

In multi-instance deployments, a user's connections may be spread across different servers. The `pubsub` package provides cross-instance messaging via Redis Pub/Sub so that broadcasts, group updates, and server actions reach all relevant connections regardless of which instance they're on.

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
| `PublishGroupAction(groupID, action, data)` | Group's action handler | Cross-connection broadcasts via `BroadcastAction` |

`GroupActionMessage` is used by `ctx.BroadcastAction()` for cross-instance delivery. Unlike `ServerActionMessage` (user-scoped), it targets all connections in a session group. Each receiving instance dispatches the action on local connections via their event loop.

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
