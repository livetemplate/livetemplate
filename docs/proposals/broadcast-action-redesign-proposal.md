# BroadcastAction Redesign — Research & Proposal

**Status:** Proposed

## Context

LiveTemplate today exposes a single broadcast primitive — `ctx.BroadcastAction(action, data)` — that fans out a server-side controller invocation to all WebSocket connections in the originator's session group, sender excluded. The current API has two ergonomic problems and one capability gap:

1. **Boilerplate at every mutation site.** The dominant usage pattern (visible in `e2e/docker/app/main.go:Send`, `broadcast_test.go:syncController.Add`, `broadcast_test.go:Increment`, `broadcast_test.go:SetMessage`) is: mutate state → `ctx.BroadcastAction("RefreshFoo", nil)` → write a `RefreshFoo` controller method whose only job is to re-read the freshly-persisted state. The action call exists solely to fan-out a re-render. This is mechanical, easy to forget, and adds a second controller method per mutation pattern.
2. **No same-user, cross-device default.** Whether `BroadcastAction` reaches other devices of the same user depends entirely on how `Authenticator.GetSessionGroup` happens to map `userID → groupID`. With `BasicAuthenticator` it works because groupID = userID; with custom per-device groupIDs it silently doesn't. There's no "this is the same human" primitive.
3. **No cross-user topic broadcast.** PubSub already implements `PublishGlobal`, `PublishToGroup`, `PublishToUser`, `PublishGroupAction` (interface in `pubsub/types.go:54-95`; Redis implementations span roughly `pubsub/redis.go:104-323` — see `^func (b \*RedisBroadcaster) Publish` for current anchors), but the public Context API exposes only group-scoped broadcasts. There is no way to fan out to "everyone in chat room 42" or "everyone watching auction 17" without coercing the Authenticator into producing a contrived groupID — which breaks state-isolation semantics.

The intended outcome is a redesign where (a) state changes propagate automatically across the same user's devices with zero developer effort, (b) topic-scoped broadcasts become a first-class concept with authentication gating, and (c) the existing escape hatches remain for the rare cases where neither default fits.

**Sibling-repo caveat:** the research pass for this proposal did not have `../lvt` or `../client` reachable. External usage patterns from those repos may surface additional cases — this proposal should be re-validated against `lvt` and `client` before implementation.

## Use cases (exhaustive enumeration)

| # | Use case | Today | Pain |
|---|---|---|---|
| A | Same user, multi-device state sync (user adds an item on phone → desktop updates) | Works if `groupID = userID`; needs explicit `BroadcastAction` + matching re-render method | Boilerplate; silently broken if custom Authenticator splits userID across groupIDs |
| B | Same browser, multi-tab sync (anonymous, two tabs) | Works via `AnonymousAuthenticator` cookie + `BroadcastAction` | Same boilerplate as A |
| C | Cross-user collaborative editing (chat room, shared doc, live cursor) | **Not supported in public API.** Would require lying to the Authenticator | The big gap |
| D | Cross-user, auth-gated room (private chat, team channel) | Not supported | The big gap, plus needs auth |
| E | Anonymous-readable topic (public auction bids, live sports score, stock ticker) | Not supported | The big gap, with no auth required |
| F | Server push to one user from outside an action (webhook, cron, background job) | `Session.TriggerAction` if you cached a Session handle from OnConnect | Requires keeping the Session around; awkward outside connection lifecycle |
| G | Server push to one topic from outside an action | Not supported in public API; only `pubsub.PublishGlobal` exists | No handler-level entry point |
| H | Global announcement (maintenance banner, deploy notice) | Not supported in public API | `pubsub.PublishGlobal` is internal-only from the developer's perspective |
| I | Sender exclusion (originating tab updates via its own action response, not the broadcast) | `BroadcastAction` already excludes sender via `GetByGroupExcept` | None — preserve this semantic |
| J | HTTP POST mutating state, fan-out to peer WS connections | Works (`mount.go:processBroadcasts` runs after action regardless of transport) | None — preserve |
| K | Per-connection state that should *not* sync (transient UI like collapsed-panel flags) | N/A — state is group-scoped, so today this is a developer-discipline problem | Needs an opt-out story under implicit fan-out |

Use cases A, B, F, I, J have working machinery in the codebase already — the redesign must preserve them. C, D, E, G, H are the missing capabilities. K becomes important once fan-out is implicit.

## Current architecture (key references)

- **`ctx.BroadcastAction`** queues `broadcastRequest{action, data}` on the Context (`context.go:603-614`). Cap: `MaxBroadcastsPerAction = 100`.
- **`mount.go:processBroadcasts` / `dispatchBroadcastToGroup`** runs after the action returns successfully (`mount.go:1544-1582`). It calls `registry.GetByGroupExcept` for local fan-out and `pubsub.GroupActionBroadcaster.PublishGroupAction` for cross-instance fan-out.
- **`Connection.DispatchChan`** (`internal/session/registry.go:40-72`) is the per-connection mailbox. The event loop in `mount.go:889-891` selects on it and invokes `handleDispatchedAction`, which **runs the controller method again** against the receiving connection's state (`mount.go:1587-1639`).
- **`Authenticator.GetSessionGroup`** decides who shares state (`auth.go:23-37`). `BasicAuthenticator.GetSessionGroup` returns `userID` (`auth.go:193-198`); `AnonymousAuthenticator.GetSessionGroup` returns a cookie-bound ID (`auth.go:78-91`).
- **`Connection.UserID`** is already populated at register time (`internal/session/registry.go:40-59`), and `ConnectionRegistry.byUser` already indexes by it (`registry.go:307-580`, method `GetByUser` at line 501). The registry currently exposes `GetByGroupExcept` (line 468) but **not** a `GetByUserExcept` — that's a small new function to add, modeled byte-for-byte after `GetByGroupExcept`. The underlying index is already there; the missing piece is the exclusion accessor.
- **PubSub** has `PublishToUser`, `PublishServerAction`, `PublishGlobal`, `PublishGroupAction` (`pubsub/types.go:54-95`). All five channel patterns exist (`docs/references/pubsub.md:108-118`). The plumbing is there; only the public-API surface is missing.

## Proposed design

### Principle: separate three concerns that are currently fused

1. **State scope** — who shares the same `State` struct. Decided by `Authenticator.GetSessionGroup` today; no change.
2. **Auto-sync scope** — which other connections re-render after a mutation. **New:** defaults to the user's connections (or the session group's connections if anonymous). Opt-out per action.
3. **Topic fan-out** — explicit, cross-cutting, identity-agnostic. **New:** named topics with server-side subscription and a single ACL hook.

`BroadcastAction` today entangles (1) and (2). Splitting them lets the default cover use cases A/B with no API surface, and lets topics (3) cover C/D/E without abusing the Authenticator.

### 1. Implicit peer sync (default behavior)

After any successful action — HTTP POST, WebSocket action, or server-initiated dispatch — the framework re-renders every peer connection that shares state with the originator and sends them a diff. **No controller method invocation on the peer; no `ctx.BroadcastAction` call at the mutation site.**

**Mechanics:**
- Add a post-action hook in `mount.go` (next to the existing `processBroadcasts` call site) that:
  1. Resolves the fan-out target set: `registry.GetByGroupExcept(groupID, originator)` is the existing primitive — same set as today's BroadcastAction, but the rendering path is different.
  2. For each peer connection, enqueue a lightweight **`renderRequest`** (new type, sibling of `DispatchRequest`) on `Connection.DispatchChan` — or, cleaner, add a second channel `Connection.RenderChan` to avoid collapsing the two semantics.
  3. The connection's event loop dequeues, snapshots the latest persisted state, runs the **render phase only** (`Parse → Build → Diff → Send`, skipping the action dispatch step), and writes the diff.
- Cross-instance: publish a new `RenderInvalidation{groupID}` message via PubSub (analogous to `GroupActionMessage` but with no action name or payload — just "your group's state changed, re-read"). Each receiving instance walks its local connections in that group and enqueues a `renderRequest` per peer.

**Why not just call `BroadcastAction` internally?** Because BroadcastAction re-runs the controller method on the peer with the *original* action's data. That's wrong for an implicit re-sync — the peer's controller would re-execute the mutation (idempotency-dependent, error-prone, double-counted side effects). The new render-only path skips the controller entirely; peers only re-read state and re-render their existing template.

**Opt-out:** `ctx.SkipPeerSync()` on the Context. Sets a flag the post-action hook checks. For controllers where many actions are per-connection, also `livetemplate.WithImplicitSyncDisabled()` on the template handler.

**Default fan-out scope decision (matters for use case A):**
- If `ctx.UserID() != ""`: fan-out target is `registry.GetByUserExcept(userID, originator)` — every connection of this human, regardless of how their `groupID`s are arranged. (`GetByUserExcept` does not exist today; see "Critical files to modify".)
- Else (anonymous, `userID == ""`): fan-out target is `registry.GetByGroupExcept(groupID, originator)` — the existing group-scoped semantic. Explicitly **not** `GetByUser("")`, which would match every anonymous connection across every group and leak state between unrelated browsers.
- **This is the key DX change.** Today `BroadcastAction` only follows groupID. The new implicit sync follows userID when present, group when not. This makes "same user, all devices" work regardless of the Authenticator's groupID strategy.

**Cross-instance state-consistency constraint.** The render-only path on a peer reads the latest persisted state from the session store. For this to be safe across instances, the originator's `persistState` write **must commit before** the `RenderInvalidation` is published to PubSub — otherwise a remote instance can re-render with stale state. Two implementation options: (a) publish only after `persistState` returns successfully (preferred; matches existing `processBroadcasts` ordering at `mount.go:processBroadcasts`); (b) include a monotonic state version in the invalidation message and have receivers retry/skip if the store version they read is older. Option (a) is simpler and consistent with current behavior; document it as a hard requirement so session-store implementors don't switch to async writes silently.

**Per-connection state caveat (use case K):** because state is still group-scoped, fields that should diverge per-connection (collapsed panels, draft text) need to live in client-side state or in a per-connection sidecar — that's a pre-existing constraint and outside this proposal's scope. The opt-out flag is the band-aid until per-connection state is addressed.

### 2. Topics (new public API)

Topics are **named broadcast channels**, independent of session groups and users. Connections subscribe explicitly; broadcasts to a topic reach all subscribed connections regardless of identity.

**Server-side subscription** (Mount or OnConnect):
```go
func (c *ChatController) Mount(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.RoomID = ctx.Param("room")
    ctx.SubscribeTopic("room/" + state.RoomID)
    return state, nil
}
```

**Broadcast from an action:**
```go
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    // ... persist message ...
    ctx.BroadcastToTopic("room/" + state.RoomID, "NewMessage", map[string]interface{}{
        "id": msg.ID, "author": ctx.UserID(), "body": msg.Body,
    })
    return state, nil
}
```

**Receiver-side:** the controller defines a `NewMessage` method that runs on each subscribed connection (same dispatch path as today's BroadcastAction — `handleDispatchedAction`). This is the *one* place we keep the existing "fan-out a controller method" semantic, because topic broadcasts often carry payloads peers need to act on (e.g., append to a chat log), not just "re-render with new state".

**Server-initiated topic broadcast** (use cases G, H — from webhooks, cron, etc.):
```go
handler.BroadcastToTopic("auction/42", "BidUpdate", data)
handler.BroadcastToUser("alice", "DM", data)
handler.BroadcastGlobal("Maintenance", data)
```

These wrap the existing pubsub `Publish*` methods plus local-instance fan-out (same pattern as `dispatchBroadcastToGroup`).

**Interaction with topic broadcasts (deduplication semantics).** A peer connection that is both in the originator's user/group fan-out set *and* subscribed to a topic the originator broadcasts to in the same action could receive two messages from one action: one render diff (from implicit sync) and one action dispatch (from the topic broadcast). The proposal's stance: this is **not** an error condition and the framework does not dedupe. They serve different purposes — the implicit sync re-renders the peer's current view against the new state; the topic broadcast invokes a controller method (which itself may produce a render diff after the peer's state mutates). If a developer wants strictly one update, they can `ctx.SkipPeerSync()` before `BroadcastToTopic`. Document this trade-off; do not over-engineer dedup logic that would require pairing renders to topic identities.

### 3. Topic ACL (auth gating)

A single global hook configured at template construction:

```go
template := livetemplate.New("app",
    livetemplate.WithTopicACL(func(topic, userID string, r *http.Request) (allowed bool, err error) {
        switch {
        case strings.HasPrefix(topic, "public/"):
            return true, nil
        case strings.HasPrefix(topic, "user/"):
            return userID != "" && strings.HasSuffix(topic, "/"+userID), nil
        case strings.HasPrefix(topic, "room/"):
            return userID != "" && c.UserInRoom(userID, topic), nil
        }
        return false, fmt.Errorf("unknown topic: %s", topic)
    }))
```

`ctx.SubscribeTopic(name)` calls the ACL; on `(false, _)` it returns `ErrTopicForbidden` and the subscription is rejected. Default (no ACL configured) is `allow all` — same posture as today's group broadcasts.

**ACL is evaluated at subscribe time, not on every broadcast.** This is an explicit design decision, consistent with how WebSocket session-lifetime authorization already works (an authenticated session that has its permissions revoked continues to receive WS messages until the connection drops or the controller calls `ctx.UnsubscribeTopic`). For per-message authorization, the developer should perform the check inside the receiver controller method instead. The trade-off: cheap fan-out (one ACL call per Mount) versus revocation latency (bounded by connection lifetime). Applications needing immediate revocation must explicitly drop the connection or call `UnsubscribeTopic` from a server-side action when permissions change.

`BroadcastToTopic` does **not** ACL-check the sender, because:
- Sending and receiving are independent operations.
- The Mount-time ACL gates who can read; senders are gated by whether they're allowed to invoke the action handler at all (existing authorization).
- If a developer wants send-side gating, it goes in the action handler, not the topic layer.

### 4. Migration of `ctx.BroadcastAction`

Pre-release scope note: the library has no production users outside the ecosystem repos (`lvt`, `client`) at the time of writing, so this change does not require a migration guide or major-version ceremony. The behavior change ships as the new default. Two opt-out paths exist for the cases where per-tab sovereignty is intentional:
- Per-action: `ctx.SkipPeerSync()`.
- Per-handler: `livetemplate.WithImplicitSyncDisabled()` at template construction.

Most existing call sites become redundant:
- `e2e/docker/app/main.go:Send` — the `BroadcastAction("RefreshMessages", nil)` is purely a re-render trigger. Delete; implicit peer sync covers it. Also delete the empty `RefreshMessages` controller method.
- `broadcast_test.go:Increment`, `SetMessage`, `Add` — same pattern. The `Refresh*` methods become unused.

Call sites that **stay** as `BroadcastAction`:
- Any case where the peer needs to react to a payload the sender chose, not just re-render with new state. (None in the current repo, but conceptually valid.)
- Any case where the developer explicitly wants the peer's controller method to run with specific data (e.g., a chat client that does optimistic updates and needs the canonical server message on peers).

Keep `ctx.BroadcastAction` in the API; document it as **"dispatch this action to peer connections in my group"** — same semantics as today, but no longer the default fan-out mechanism.

### 5. Final API surface (summary)

**On `*Context`:**
- `ctx.BroadcastAction(action, data)` — *(unchanged, deprioritized in docs)* — peer-group dispatch with controller invocation.
- `ctx.SkipPeerSync()` — *(new)* — opt out of implicit fan-out for this action only.
- `ctx.SubscribeTopic(name)` / `ctx.UnsubscribeTopic(name)` — *(new)*.
- `ctx.BroadcastToTopic(topic, action, data)` — *(new)* — cross-cutting topic dispatch.

**On `LiveHandler`:**
- `handler.BroadcastToUser(userID, action, data)` — *(new)* — out-of-band push to a user.
- `handler.BroadcastToTopic(topic, action, data)` — *(new)*.
- `handler.BroadcastGlobal(action, data)` — *(new)*.

**On the template builder:**
- `WithTopicACL(fn)` — *(new)*.
- `WithImplicitSyncDisabled()` — *(new, opt-out at handler level)*.

**Unchanged:**
- `Session` interface and `Session.TriggerAction` — still the right tool for goroutines holding a captured Session handle.
- `Authenticator` interface — no change.
- All PubSub interfaces — they were already capable; this just exposes them.

## Critical files to modify (for the implementation that would follow)

- `context.go` — add `SkipPeerSync`, `SubscribeTopic`, `UnsubscribeTopic`, `BroadcastToTopic`; reuse the existing `broadcasts []broadcastRequest` slice pattern for a new `topicBroadcasts` queue.
- `mount.go:1544-1582` — split `dispatchBroadcastToGroup` into three siblings: `dispatchBroadcastToGroup` (existing), `dispatchPeerSyncToUser` (new, render-only), `dispatchToTopic` (new).
- `mount.go:889-891` — extend the connection event-loop select to also drain a new `RenderChan` (or accept a discriminated request on the existing `DispatchChan`).
- `internal/session/registry.go` — (a) add `GetByUserExcept(userID, excludeConn)` modeled after `GetByGroupExcept` at line 468; (b) add `byTopic map[string][]*Connection`; (c) add `SubscribeConnectionToTopic(conn, topic)` / `UnsubscribeConnectionFromTopic(conn, topic)` / `GetByTopicExcept(topic, excludeConn)`; (d) wire `byTopic` cleanup into the existing `Connection.Close()` / `Unregister` path so topic subscriptions don't outlive their connection. Reuse existing `byUser` index for `BroadcastToUser`.
- `pubsub/types.go`, `pubsub/redis.go` — add `RenderInvalidationMessage` type + `PublishRenderInvalidation` method + `SubscribeRenderInvalidations` handler. Add topic-channel pattern `livetemplate:topic:{name}`.
- `auth.go` — no change.
- `docs/references/pubsub.md`, `docs/references/controller-pattern.md`, `docs/design/ARCHITECTURE.md` — document the three-concern model (state scope, sync scope, topic fan-out).

## Existing utilities to reuse

- `ConnectionRegistry.GetByUser` (`internal/session/registry.go:501`) and `GetByGroupExcept` (line 468) — the building blocks for use case A's fan-out. `GetByUserExcept` is new but trivially derivable from these two.
- `Connection.EnqueueDispatch` (`internal/session/registry.go:79-102`) — the non-blocking, drop-on-overflow mailbox primitive. New `EnqueueRender` follows the same shape.
- `pubsub.RedisBroadcaster.PublishGroupAction` / `SubscribeGroupActions` (`pubsub/redis.go:305-323`) — template for the new `PublishRenderInvalidation` and topic equivalents.
- `pubsub.DynamicSubscriber.SubscribeToGroup` (`pubsub/types.go:54-95`) — pattern for dynamic topic subscription on connect.
- `mount.go:processBroadcasts` post-action hook site — the right place to add the implicit render-fan-out call.
- The existing "broadcasts inside a dispatched action are dropped" guard in `mount.go` — grep anchor: `BroadcastAction calls inside a dispatched action are ignored`. Replicate verbatim for render-fan-out and topic broadcasts, with the same recursion-storm rationale.

## Verification plan (when the implementation lands)

End-to-end tests (mirroring `broadcast_test.go` structure):
1. **Implicit sync, two devices, one user.** Custom Authenticator returns `groupID = "device-" + userID + "-" + deviceID` (per-device groups). User "alice" connects two WebSockets with different `deviceID`. Tab 1 invokes a mutation. Assert Tab 2 receives a render diff *without* either tab calling `BroadcastAction`. Confirms use case A works regardless of groupID strategy.
2. **Implicit sync, anonymous.** Two tabs in one browser via `AnonymousAuthenticator`. Tab 1 mutates. Tab 2 receives diff. Confirms use case B.
3. **Opt-out per action.** Action calls `ctx.SkipPeerSync()`. Peer tab does **not** receive a diff. Confirms K.
4. **Topic broadcast, public.** Two anonymous tabs both `SubscribeTopic("public/feed")` in Mount. One invokes `BroadcastToTopic`. The other receives the action dispatch. Confirms use case E.
5. **Topic ACL, denied.** `WithTopicACL` returns `(false, nil)` for `"private/admin"`. Anonymous tab's `SubscribeTopic("private/admin")` returns `ErrTopicForbidden`. Confirms use case D's auth gate.
6. **Out-of-band `handler.BroadcastToUser`.** Goroutine without a Context calls `handler.BroadcastToUser("alice", "DM", data)`. Alice's connection's controller `DM` method runs and a diff is sent. Confirms use case F without holding a Session.
7. **Cross-instance.** Two `liveHandler` instances behind a shared `RedisBroadcaster`. Tab on instance A mutates; tab on instance B receives the render diff. Then a `BroadcastToTopic` from instance A reaches a subscriber on instance B. Confirms PubSub plumbing for both the new paths.
8. **Sender exclusion preserved.** Tab 1 mutates; Tab 1 must not receive a *second* render diff after its own action response. Confirms I.
9. **Recursion guard.** Action handler invoked via topic dispatch calls `ctx.BroadcastToTopic` again — verify it's logged-and-dropped, matching `mount.go:1626-1631` behavior for `BroadcastAction`.

Run `go test -v -race ./...` and the existing broadcast suite (`go test -run TestWSAction_BroadcastAction -v`) to confirm no regressions in the legacy `BroadcastAction` path.

## Design constraints (must be satisfied by v1)

- **Render fan-out coalescing.** A user with 10 tabs typing in a high-frequency input could otherwise produce N × M renders per second under implicit sync. v1 must coalesce render requests per connection on a short timer (10ms is a reasonable starting point; tune after benchmarks). The design should land in the first implementation rather than be retrofitted under load — a single `time.Timer` per connection that resets on each `EnqueueRender` and fires the most recent invalidation is sufficient.
- **Topic GC on disconnect.** Topics with no subscribers leak in `byTopic` if not cleaned up. `Connection.Close()` (grep anchor: `func (c *Connection) Close`) currently drops `byUser` and `byGroup` entries — the new `byTopic` cleanup must be wired into the *same* code path, not a separate goroutine, to avoid the "no unsubscribe" trade-off described in `docs/references/pubsub.md` ("No Unsubscribe" section).
- **Cross-instance write ordering.** Persisted state writes must commit before the corresponding `RenderInvalidation` is published. See the constraint note in §1.

## Open questions

- **Per-connection state.** Use case K is a real gap that implicit sync makes more visible. Worth a separate proposal for `state.PerConnection` or similar.
- **Wildcard / hierarchical topics.** Out of scope for v1; `"room/*"` patterns can be added later if needed.
