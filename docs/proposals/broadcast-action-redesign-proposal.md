# BroadcastAction Redesign — Research & Proposal

**Status:** Proposed

## Context

LiveTemplate today exposes a single broadcast primitive — `ctx.BroadcastAction(action, data)` — that fans out a server-side controller invocation to all WebSocket connections in the originator's session group, sender excluded. The current API has two ergonomic problems and one capability gap:

1. **Boilerplate at every mutation site.** The dominant usage pattern (visible in `e2e/docker/app/main.go:Send`, `broadcast_test.go:syncController.Add`, `broadcast_test.go:Increment`, `broadcast_test.go:SetMessage`) is: mutate state → `ctx.BroadcastAction("RefreshFoo", nil)` → write a `RefreshFoo` controller method whose only job is to re-read the freshly-persisted state. The action call exists solely to fan-out a re-render. This is mechanical, easy to forget, and adds a second controller method per mutation pattern.
2. **No same-user, cross-device default.** Whether `BroadcastAction` reaches other devices of the same user depends entirely on how `Authenticator.GetSessionGroup` happens to map `userID → groupID`. With `BasicAuthenticator` it works because groupID = userID; with custom per-device groupIDs it silently doesn't. There's no "this is the same human" primitive.
3. **No cross-user topic broadcast.** PubSub already implements `PublishGlobal`, `PublishToGroup`, `PublishToUser`, `PublishGroupAction` (interface in `pubsub/types.go:54-95`; Redis implementations span roughly `pubsub/redis.go:104-323` — see `^func (b \*RedisBroadcaster) Publish` for current anchors), but the public Context API exposes only group-scoped broadcasts. There is no way to fan out to "everyone in chat room 42" or "everyone watching auction 17" without coercing the Authenticator into producing a contrived groupID — which breaks state-isolation semantics.

The intended outcome is a redesign where (a) state changes propagate automatically across the same user's devices with zero developer effort, (b) topic-scoped broadcasts become a first-class concept with authentication gating, and (c) the existing escape hatches remain for the rare cases where neither default fits.

**Sibling-repo audit (complete).** The post-proposal audit pass found: `lvt` has **zero** `BroadcastAction` call sites in source or templates — its scaffolds (`internal/generator/templates/`) emit CRUD action methods with no fan-out, so impact is a `go.mod` pin bump only. The TypeScript `client` already handles `UpdateResponse` arriving without a paired outgoing action (its diff path is stateless), so the implicit-render-sync flow needs no changes there; the only client work is a small error-envelope guard for `ErrTopicForbidden` (see §3, "Wire-format note"). The cross-repo migration scope — `examples/` (11 call sites across 4 apps), `tinkerdown/` (6), the docs site (22 content files plus three pattern scaffolds), and the in-repo `e2e/docker/app/main.go` — is enumerated in §6 "Impacted repositories" below.

**Related prior change:** the reserved `Sync()` controller method, which the framework auto-dispatched to peers, was removed two days before this proposal in [PR #406](https://github.com/livetemplate/livetemplate/pull/406). That removal motivated the current state where every mutation needs an explicit `BroadcastAction("RefreshX", nil)` pair — the boilerplate this proposal targets. See "Relationship to the removed `Sync()` method" inside §"Proposed design" for why implicit peer sync is not a return to `Sync()`.

## At a glance

The shape of the change, in code. Same `Send` action, three transports:

```go
// Today — every mutation needs an explicit broadcast call + a re-render method.
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.Messages = c.loadMessages()
    ctx.BroadcastAction("RefreshMessages", nil)  // boilerplate
    return state, nil
}
func (c *ChatController) RefreshMessages(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.Messages = c.loadMessages()             // duplicate of Send's body
    return state, nil
}
```

```go
// Proposed (same user, multi-device) — implicit. Zero broadcast code.
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.Messages = c.loadMessages()
    return state, nil   // framework re-renders peers automatically
}
```

```go
// Proposed (cross-user chat room) — explicit topic.
func (c *ChatController) Mount(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    state.RoomID = ctx.Param("room")
    ctx.SubscribeTopic("room/" + state.RoomID)   // server-side, ACL-gated
    return state, nil
}
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
    msg := c.persist(ctx.GetString("body"))
    ctx.BroadcastToTopic("room/"+state.RoomID, "NewMessage", map[string]interface{}{
        "id": msg.ID, "author": ctx.UserID(), "body": msg.Body,
    })
    return state, nil
}
```

```go
// Proposed (server push from a webhook / cron) — handler-level entry point.
handler.BroadcastToUser("alice", "DM", map[string]interface{}{"from": "bob"})
handler.BroadcastToTopic("auction/42", "BidUpdate", bidData)
handler.BroadcastGlobal("Maintenance", map[string]interface{}{"at": deadline})
```

Three concerns, three code shapes. The redesign separates them so each lives in the smallest API surface that fits.

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

References use grep anchors (per CLAUDE.md guidance) so they don't drift as line numbers shift.

- **`ctx.BroadcastAction`** queues `broadcastRequest{action, data}` on the Context. Grep: `^func \(c \*Context\) BroadcastAction` in `context.go`. Cap: `MaxBroadcastsPerAction = 100`.
- **`processBroadcasts` / `dispatchBroadcastToGroup`** run after the action returns successfully. Grep: `^func \(h \*liveHandler\) processBroadcasts` and `^func \(h \*liveHandler\) dispatchBroadcastToGroup` in `mount.go`. They call `registry.GetByGroupExcept` for local fan-out and `pubsub.GroupActionBroadcaster.PublishGroupAction` for cross-instance fan-out.
- **`Connection.DispatchChan`** is the per-connection mailbox. Grep: `DispatchChan chan \*DispatchRequest` in `internal/session/registry.go`. The event loop selects on it (grep in `mount.go`: `case req := <-connection.DispatchChan`) and invokes `handleDispatchedAction` (grep: `^func \(h \*liveHandler\) handleDispatchedAction`), which **runs the controller method again** against the receiving connection's state.
- **`Authenticator.GetSessionGroup`** decides who shares state (`auth.go`, grep: `GetSessionGroup\(r \*http.Request`). `BasicAuthenticator` returns `userID`; `AnonymousAuthenticator` returns a cookie-bound ID.
- **`Connection.UserID`** is already populated at register time, and `ConnectionRegistry.byUser` already indexes by it (grep in `internal/session/registry.go`: `^func \(r \*ConnectionRegistry\) GetByUser`). The registry currently exposes `GetByGroupExcept` (grep: `^func \(r \*ConnectionRegistry\) GetByGroupExcept`) but **not** a `GetByUserExcept` — that's a small new function to add, following the same pattern as `GetByGroupExcept`. The underlying index is already there; the missing piece is the exclusion accessor.
- **PubSub** interfaces in `pubsub/types.go`: `Broadcaster` (grep: `^type Broadcaster interface`) declares `PublishGlobal` / `PublishToUser` / `PublishServerAction`; `GroupActionBroadcaster` (grep: `^type GroupActionBroadcaster interface`) declares `PublishGroupAction`; `DynamicSubscriber` (grep: `^type DynamicSubscriber interface`) declares per-scope subscribe methods. Redis implementations in `pubsub/redis.go` — grep: `^func \(b \*RedisBroadcaster\) Publish` and `^func \(b \*RedisBroadcaster\) Subscribe`. The plumbing is there; only the public-API surface is missing.

## Proposed design

### Principle: separate three concerns that are currently fused

1. **State scope** — who shares the same `State` struct. Decided by `Authenticator.GetSessionGroup` today; no change.
2. **Auto-sync scope** — which other connections re-render after a mutation. **New:** defaults to the user's connections (or the session group's connections if anonymous). Opt-out per action.
3. **Topic fan-out** — explicit, cross-cutting, identity-agnostic. **New:** named topics with server-side subscription and a single ACL hook.

`BroadcastAction` today entangles (1) and (2). Splitting them lets the default cover use cases A/B with no API surface, and lets topics (3) cover C/D/E without abusing the Authenticator.

### Relationship to the removed `Sync()` method

Before May 2026, livetemplate had a reserved `Sync()` controller method that the framework auto-dispatched to peers after every action. It was removed in [PR #406](https://github.com/livetemplate/livetemplate/pull/406) ("refactor: remove reserved Sync action") for two reasons: (a) magic method names with no compile-time signal, and (b) the dispatched method still re-ran a controller method on each peer — same idempotency hazards that motivate this proposal's stance on `BroadcastAction`. The replacement after #406 was explicit `BroadcastAction("RefreshX", nil)` calls at every mutation site — exactly the boilerplate this proposal targets.

This proposal restores the **capability** `Sync()` offered (automatic peer convergence after a mutation) without the **mechanism** that made it brittle. Implicit peer sync does not invoke a controller method on the peer; it runs the render phase only (`Parse → Build → Diff → Send`) against the peer's already-persisted state. There is no peer-side controller invocation, so no idempotency hazard. This is also why opt-out is `ctx.SkipPeerSync()` (a flag) rather than "omit a `Sync` method" (presence-based magic): the rendering is the framework's responsibility, not the developer's.

Both the docs site and in-repo docs still carry stale `Sync` references from before #406 (notably `docs/content/recipes/sync-and-broadcast.md`, the Pattern #26 controller in `docs/content/recipes/patterns/_app/handlers_realtime.go`, and `livetemplate/docs/guides/ephemeral-components.md` — grep `Sync` in the state-init lifecycle list). The implementation PR must clean those up as part of its docs-update scope — see §6. **Cleanup-sweep note:** grep for the bare word `Sync` (`grep -rn '\bSync\b' docs/`), not just `Sync()` — `ephemeral-components.md` references it without parentheses and a `Sync()` sweep silently misses it.

### 1. Implicit peer sync (default behavior)

After any successful action — HTTP POST, WebSocket action, or server-initiated dispatch — the framework re-renders every peer connection that shares state with the originator and sends them a diff. **No controller method invocation on the peer; no `ctx.BroadcastAction` call at the mutation site.**

**Mechanics:**
- Add a post-action hook in `mount.go` (next to the existing `processBroadcasts` call site) that:
  1. Resolves the fan-out target set: `registry.GetByUserExcept(userID, originator)` when `ctx.UserID() != ""`, else `registry.GetByGroupExcept(groupID, originator)`.
  2. For each peer connection, enqueue a discriminated **`DispatchRequest{Kind: KindRender}`** on the existing `Connection.DispatchChan`. **Decision:** use a discriminated request on the existing channel rather than a second `Connection.RenderChan`. Rationale: a second channel doubles the event-loop `select` cases, forces explicit priority rules between actions and renders, and complicates the coalescing logic (the coalescer needs to drop pending renders without affecting action delivery). A single mailbox with a `Kind` field keeps the event loop linear; coalescing is a per-connection `*time.Timer` field on `Connection` that's independent of channel semantics.
  3. The connection's event loop dequeues, runs the **render phase only** (`Parse → Build → Diff → Send`, skipping the action dispatch step), and writes the diff.

**State source for the render-only path.** Local peers (same instance as the originator) read from the in-memory state object the originator just persisted — the same `connSt.state` pointer the existing `dispatchBroadcastToGroup` reaches via the registry. No store round-trip. Remote peers (cross-instance) re-read from the session store after receiving the `RenderInvalidation` message. This split is intentional: local fan-out should be fast (no extra I/O); the store round-trip is the cost of being on a different process. The cross-instance write-ordering constraint below ensures the remote read sees the post-mutation state.

- Cross-instance: publish a new `RenderInvalidation{groupID}` message via PubSub (analogous to `GroupActionMessage` but with no action name or payload — just "your group's state changed, re-read"). Each receiving instance walks its local connections in that group and enqueues a render-kind dispatch per peer.

**Why not just call `BroadcastAction` internally?** Because BroadcastAction re-runs the controller method on the peer with the *original* action's data. That's wrong for an implicit re-sync — the peer's controller would re-execute the mutation (idempotency-dependent, error-prone, double-counted side effects). The new render-only path skips the controller entirely; peers only re-read state and re-render their existing template.

**Implicit sync is suppressed inside dispatched actions.** When `handleDispatchedAction` runs a controller method on a peer (e.g., a topic broadcast or `BroadcastAction`), any state mutation that method produces does **not** trigger another round of implicit peer sync. This is the analog of the existing broadcast-storm guard (grep `mount.go`: `BroadcastAction calls inside a dispatched action are ignored`). Without it, a single topic broadcast that mutates state on every receiver would cascade: each receiver's mutation re-fans-out a render diff to all *their* peers, multiplying the original fan-out by N for an N-connection user. The suppression is a flag the post-action hook checks, set on the dispatched action's Context by `handleDispatchedAction`.

**`SubscribeTopic` on HTTP GET is a no-op.** Mount runs on every HTTP request and on WebSocket connect (per CLAUDE.md's Mount guard guidance). `ctx.SubscribeTopic(name)` records the desired topic on the Context but only attaches a subscription to a `Connection` when one exists — i.e., during WebSocket connect/reconnect. On HTTP GET, the call returns `nil` without subscribing. This means the same `Mount` body works for both transports without requiring `if ctx.IsInitialMount()` guards around topic subscription. The ACL check still runs on HTTP GET (so an unauthorized subscribe is rejected eagerly, before the WS upgrade), but the subscription itself materializes only when a Connection is present.

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

These wrap the existing pubsub `Publish*` methods plus local-instance fan-out (same pattern as `dispatchBroadcastToGroup`). Specifically, `handler.BroadcastToUser` is a thin public wrapper around the existing `pubsub.Broadcaster.PublishServerAction(userID, action, data)` (already in `pubsub/types.go`) plus a local-instance loop over `registry.GetByUser(userID)`. No new PubSub protocol is needed for use case F — just the handler-level entry point so webhook/cron code doesn't have to hold a Session.

**Interaction with topic broadcasts (deduplication semantics).** A peer connection that is both in the originator's user/group fan-out set *and* subscribed to a topic the originator broadcasts to in the same action receives two messages from one action: one render diff (from implicit sync), then one action dispatch (from the topic broadcast). **User-visible behavior:** two consecutive WebSocket frames in rapid succession — the first re-renders the peer's view against the new state, the second invokes the topic action's controller method which may mutate state and produce a third render diff. For most UI this is a fast double-update and indistinguishable from a single update; for focus-sensitive or animation-heavy UI it can be disruptive. The proposal's stance: this is **not** an error condition and the framework does not dedupe — the two paths serve different purposes (state diff vs. payload-carrying controller call). Developers who want strictly one update call `ctx.SkipPeerSync()` before `BroadcastToTopic`. Dedup logic pairing renders to topic identities would be expensive and not worth the complexity for v1.

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

`ctx.SubscribeTopic(name)` calls the ACL; on `(false, _)` it returns `ErrTopicForbidden` and the subscription is rejected. Default (no ACL configured) is `allow all` — same posture as today's group broadcasts. (Whether `allow all` is the right default given the surface-area expansion is an open question — see §"Open questions".)

**Wire-format note (client surface).** When a denial happens on the WS-connect path (Mount calling `SubscribeTopic` during a fresh upgrade), the controller's `Mount` returns an error that today's TS client cannot distinguish from a generic connection failure. The implementation should extend the WS response envelope with an optional discriminator so the client can surface the denial without dropping the connection. Recommended shape: `{ "type": "error", "code": "topic_forbidden", "topic": "..." }`. The TS client's `handleWebSocketPayload` (grep `livetemplate-client.ts`: `handleWebSocketPayload`) currently shape-tests for upload-specific fields before falling back to `UpdateResponse`; that same pattern admits a `type === "error"` branch. Browser code observes the denial via an `lvt:error` `CustomEvent` with the error object as `detail`. This is the **only** client-side change this proposal requires; the diff-handling path is unaffected by implicit peer sync (the client already accepts `UpdateResponse` frames that arrive without a paired outgoing action).

**ACL is evaluated at subscribe time, not on every broadcast.** This is an explicit design decision, consistent with how WebSocket session-lifetime authorization already works (an authenticated session that has its permissions revoked continues to receive WS messages until the connection drops or the controller calls `ctx.UnsubscribeTopic`). For per-message authorization, the developer should perform the check inside the receiver controller method instead. The trade-off: cheap fan-out (one ACL call per Mount) versus revocation latency (bounded by connection lifetime). Applications needing immediate revocation must explicitly drop the connection or call `UnsubscribeTopic` from a server-side action when permissions change.

**The `*http.Request` passed to the ACL is the WebSocket upgrade request, not a per-action request.** It's the request captured when the Connection was established. Request-scoped values set by HTTP middleware (e.g., a JWT parsed into `r.Context()`) are available; session state that has mutated since the upgrade — for example, a permission change in a database — is **not** reflected unless the ACL hook re-queries the source of truth on each call. For stateless checks (JWT in cookie, Bearer in header, role embedded in claims) this is fine. For database-backed permission models, the ACL hook is responsible for the lookup.

**ACL execution on HTTP GET is a hot path.** Because `SubscribeTopic` is a no-op on HTTP GET (it doesn't materialize a subscription without a Connection) but the ACL still runs eagerly, every HTTP page render that calls `SubscribeTopic` in Mount will execute the ACL hook. For a high-traffic landing page with a database-backed ACL, this can become a hot path. Two mitigations: (a) prefer stateless ACL checks (JWT claims) when feasible; (b) for database-backed ACL, the hook can return early on `r.Method == "GET"` and defer the real check to a WS-only path, but this loses the "subscribe rejected before WS upgrade" property and should be a deliberate trade-off. The proposal recommends (a) as the default.

`BroadcastToTopic` does **not** ACL-check the sender, because:
- Sending and receiving are independent operations.
- The Mount-time ACL gates who can read; senders are gated by whether they're allowed to invoke the action handler at all (existing authorization).
- If a developer wants send-side gating, it goes in the action handler, not the topic layer.

### 4. Migration of `ctx.BroadcastAction`

Pre-release scope note: the library has no production users outside the ecosystem repos (`lvt`, `client`) at the time of writing, so this change does not require a migration guide or major-version ceremony. The behavior change ships as the new default. Two opt-out paths exist for the cases where per-tab sovereignty is intentional:
- Per-action: `ctx.SkipPeerSync()`.
- Per-handler: `livetemplate.WithImplicitSyncDisabled()` at template construction.

Most existing call sites — both in-repo and across sibling repos — become redundant.

**In `livetemplate/` itself:**
- `e2e/docker/app/main.go:Send` — the `BroadcastAction("RefreshMessages", nil)` is purely a re-render trigger. Delete; implicit peer sync covers it. Also delete the empty `RefreshMessages` controller method.
- `broadcast_test.go` — same pattern, but note the echo methods are **not** uniformly `Refresh*`-named: `Increment` → `RefreshCount`, `SetMessage` → `SyncMessage`, `syncController.Add` → `Refresh`. The implementation PR must delete all three echo methods; a `Refresh*` grep alone would miss `SyncMessage`. (`Increment`/`SetMessage` also pass a data payload — `{"newCount": …}`, `{"value": …}` — that the echo method only uses to mirror state; under implicit sync the payload and the echo both disappear.)
- `context_broadcast_test.go` (14 call sites), `lifecycle_integration_test.go` (5), `handle_test.go` (4), `navigate_test.go` (5) — these are the remaining in-repo `BroadcastAction` test files from the §6 table; this list is the authoritative in-repo migration checklist. Each asserts the legacy peer-dispatch contract and must be retired or re-pointed at `WithImplicitSyncDisabled()` alongside `TestBroadcastAction_NoAutomaticPeerDispatch` (see §"Verification plan" item 11). Cross-reference: §6 "Impacted repositories", `livetemplate` row.

**In `examples/` (verified via `grep -rn "BroadcastAction\b" examples/`):**
- `landing-demo/main.go` — three `BroadcastAction` calls for `Increment`, `Decrement`, `Reset`. **Delete all three** (pure re-render triggers, use case A/B).
- `shared-notepad/main.go` — one `BroadcastAction("Refresh", nil)` after Save. **Delete** (use case A — sync across same-user tabs/devices is now implicit).
- `todos/controller.go` — four `BroadcastAction("RefreshTodos", nil)` calls in Add/Toggle/Delete/Update. **Delete all four**; the paired `RefreshTodos` controller method becomes unused.
- `chat/main.go` — three calls for `UserJoined`, `NewMessage`, `UserLeft`. **Migrate to topic API** (`ctx.BroadcastToTopic("chat:room:"+state.RoomID, "NewMessage", data)`, etc.). This is the canonical use-case-C migration target — the data is meaningful to peer handlers, not just a re-render signal.

**In `tinkerdown/`:**
- `examples/literate-counter-include/_app/counter.go` and `examples/literate-linked-include/_app/counter.go` — three `BroadcastAction` calls each (Increment/Decrement/Reset). These rely on a tutorial-only `sharedAuth` (constant `groupID`) to simulate visitor-shared sync. Under implicit peer sync, the explicit calls can be deleted; the tutorial comments must clarify that `sharedAuth` is an artificial setup, not production-shaped.

**Empty echo methods** (the second controller method per pattern that exists solely as a re-render target — `RefreshTodos`, `RefreshMessages`, `Refresh`, `RefreshCount`, `SyncMessage`, etc.) become dead code in every migration above. The naming is **not uniform** — grep for both `Refresh*` and `Sync*` echo methods, not just `Refresh*`. Flag them for deletion in the same step.

Call sites that **stay** as `BroadcastAction`:
- Any case where the peer needs to react to a payload the sender chose, not just re-render with new state. The `chat/` migration above shows the right replacement (`BroadcastToTopic`); `BroadcastAction` itself stays in the API for cases where group-scoped (not topic-scoped) controller dispatch is still wanted.
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

## Impacted repositories

The audit pass (post-proposal-v1) enumerated every consumer of `BroadcastAction` across the workspace. The table below replaces "the sibling-repo caveat" and feeds the release sequencing below.

| Repo (`../<name>/`) | `go.mod` / `package.json` pin | `BroadcastAction` call sites | Migration type |
|---|---|---|---|
| `livetemplate` (this repo) | — | `e2e/docker/app/main.go`, `broadcast_test.go`, `context_broadcast_test.go`, `lifecycle_integration_test.go`, `handle_test.go`, `navigate_test.go` | Core implementation; rewrite e2e Docker app; retire `TestBroadcastAction_NoAutomaticPeerDispatch` (see §"Verification plan" item 11). |
| `lvt` | `v0.8.23-0.2026...` | **0** | Version pin bump after release. Scaffolds (`internal/generator/templates/`, `internal/kits/system/{single,multi}/templates/`) emit CRUD action methods that mutate state and return — no broadcasts. The dev-server's `internal/serve/websocket.go:Broadcast()` is a hot-reload notifier, semantically unrelated; see §"Open questions" for the naming-overlap discussion. |
| `client` (TypeScript) | `0.9.0` | N/A (browser) | Add `type === "error"` branch in `handleWebSocketPayload` (see §3 "Wire-format note"). Optional: dispatch a distinct `lvt:broadcast` `CustomEvent` for topic broadcasts so apps can hook in without sniffing `meta.action`. |
| `examples` | `v0.9.0` | 11 sites across 4 apps — see §4 Migration for the per-file list | Delete redundant `Refresh*` calls in `landing-demo`, `shared-notepad`, `todos`. Migrate `chat` to the topic API (use case C). |
| `tinkerdown` | `v0.8.16` (stale) | 6 sites in 2 tutorial examples | Bump pin to `v0.9.x`; delete explicit broadcasts; clarify in tutorial comments that `sharedAuth` is teaching-only, not production-shaped. |
| `devbox-dash` | (single-user app) | 0 | Routine version bump. |
| `docs` (site) | `v0.8.23` | 22 content files mention `BroadcastAction`; `content/recipes/patterns/_app/handlers_realtime.go` Patterns #27 (Broadcasting) and #28 (Presence) use it; **Pattern #26 (Multi-User Sync) still references the removed `Sync()` method** — pre-existing stale state from PR #406 that this proposal's docs scope cleans up. | **Heaviest impact.** See "Docs migration scope" below for the file list. |

**Docs migration scope** — two distinct doc surfaces, both need rewrites:

- **In-repo contributor docs** (under `livetemplate/docs/`): `references/controller-pattern.md`, `references/pubsub.md`, `design/ARCHITECTURE.md`, and `guides/ephemeral-components.md` — the last carries a stale lifecycle reference left by #406: its state-init hook list names `Sync` (bare, no parens) alongside `Mount`/`OnConnect`, but the framework no longer dispatches it. This must be rewritten when implicit peer sync replaces the `Sync()` model.
- **Site docs** (under `../docs/content/`): user-facing. The full list, verified via `grep -rln "BroadcastAction" docs/content/`:
  - **Top-of-funnel** (touched first by new users) — `index.md`, `getting-started/your-first-app.md`.
  - **Reference** — `reference/api.md`, `reference/controller-pattern.md` §"Cross-Tab Updates with BroadcastAction", `reference/server-actions.md`, `reference/session.md`, `reference/pubsub.md`, `reference/navigate.md`, `reference/limitations.md`.
  - **Guides** — `guides/standard-html-reactivity.md`, `guides/progressive-complexity.md`.
  - **Recipes** — `recipes/broadcasting.md` (rewrite end-to-end), `recipes/sync-and-broadcast.md` (rewrite or retire — the `Sync()` half is stale post-#406), `recipes/counter/index.md` §"How BroadcastAction routes", `recipes/architecture-flow.md`, `recipes/progressive-enhancement/index.md`, `recipes/todos/index.md`.
  - **Pattern scaffolds** (critical path — these get copy-pasted into user projects) — `recipes/patterns/_app/templates/realtime/{broadcasting,multi-user-sync,server-push}.tmpl` and the matching Go controllers in `recipes/patterns/_app/handlers_realtime.go` (Patterns #26, #27, #28).
  - **Historical** — `changelog.md` only needs an entry, not a rewrite.

**Release order (critical path).** The dependency arrows below assume the redesign ships in a single coordinated wave:

1. `livetemplate` core API + new tests (this proposal becomes implementation).
2. `client` error-envelope branch (independent; can ship in parallel with #1).
3. **`docs` recipes + reference rewrites** — gating step, because the pattern scaffolds get copy-pasted into user projects.
4. `lvt` `go.mod` pin bump (no template changes needed; emits no `BroadcastAction`).
5. `examples` migration (replaces user-facing demos with the new idiom).
6. `tinkerdown`, `devbox-dash` lag bumps.

## Critical files to modify (for the implementation that would follow)

- `context.go` — add `SkipPeerSync`, `SubscribeTopic`, `UnsubscribeTopic`, `BroadcastToTopic`; reuse the existing `broadcasts []broadcastRequest` slice pattern for a new `topicBroadcasts` queue.
- `mount.go` — at `dispatchBroadcastToGroup`, split into three siblings: `dispatchBroadcastToGroup` (existing), `dispatchPeerSyncToUser` (new, render-only, uses `GetByUserExcept` or `GetByGroupExcept` per the fan-out scope rule), `dispatchToTopic` (new). At the connection event-loop `select` (grep: `case req := <-connection.DispatchChan`), extend the handler to switch on `req.Kind` — see "discriminated dispatch" decision in §1.
- `internal/session/registry.go` —
  - (a) **Add `Kind` field to `DispatchRequest`** (grep: `^type DispatchRequest struct`). New type `RequestKind` with values `KindAction` (existing default, backward-compatible zero value) and `KindRender` (new). All current call sites of `EnqueueDispatch` continue to work because they leave `Kind` zero-valued.
  - (b) Add `GetByUserExcept(userID, excludeConn)` following the same pattern as `GetByGroupExcept`. **Godoc invariant: `userID == ""` is a programmer error and MUST be guarded by the caller via `ctx.UserID() != ""` — calling with empty userID would match every anonymous connection across every group and leak state between unrelated browsers.** **Decision: on empty userID, the function returns an empty slice and emits `slog.Error("GetByUserExcept called with empty userID", ...)`** — consistent with `EnqueueDispatch`'s drop-on-overflow + log pattern, and avoids hard-killing production from a guard misuse.
  - (c) Add `byTopic map[string][]*Connection`.
  - (d) Add `SubscribeConnectionToTopic(conn, topic)` / `UnsubscribeConnectionFromTopic(conn, topic)` / `GetByTopicExcept(topic, excludeConn)`.
  - (e) Wire `byTopic` cleanup into the existing `Unregister()` path (grep: `^func \(r \*ConnectionRegistry\) Unregister`), where `byUser` and `byGroup` cleanup already happens. **Not** `Connection.Close()` — `Unregister()` is the canonical index-cleanup site; `Close()` is the lifecycle-shutdown call that delegates index work to `Unregister()`.
  - Reuse existing `byUser` index for `BroadcastToUser`.
- `pubsub/types.go`, `pubsub/redis.go` — add the `RenderInvalidationMessage` type + `PublishRenderInvalidation` method + `SubscribeRenderInvalidations` handler. **Channel pattern decision: `livetemplate:render:{groupID}` as a new, dedicated channel pattern.** Rationale: existing channels encode (scope, action-vs-broadcast) in their prefix (`livetemplate:groupaction:group:{id}` vs `livetemplate:broadcast:group:{id}`); a dedicated render channel keeps the schema regular and lets receivers subscribe selectively without parsing message envelopes. Add topic-channel pattern `livetemplate:topic:{name}` for the new `BroadcastToTopic`. **Wire format for handler-level `Broadcast*` entry points:** the new entry points serialize their `(action, data)` payload using the same envelope shape as the existing `GroupActionMessage` (JSON: `{"type": "...", "action": "...", "data": {...}, "timestamp": ..., "instanceID": ...}`). `handler.BroadcastGlobal` adapts to `Broadcaster.PublishGlobal([]byte)` by marshalling the envelope to bytes before calling the existing primitive — no change to the byte-oriented `PublishGlobal` signature; the action-oriented entry point lives at the handler layer.
- `auth.go` — no change.
- `e2e/docker/app/main.go` — migration target; delete `Send`'s `BroadcastAction("RefreshMessages", nil)` and the empty `RefreshMessages` controller method (the in-repo concrete example of the broadcast-as-re-render-trigger pattern this proposal eliminates).
- `docs/references/pubsub.md`, `docs/references/controller-pattern.md`, `docs/design/ARCHITECTURE.md` — document the three-concern model (state scope, sync scope, topic fan-out). **In-repo contributor docs only**; the user-facing site docs at `../docs/content/` are scoped in §6 "Impacted repositories".

## Existing utilities to reuse

- `ConnectionRegistry.GetByUser` and `GetByGroupExcept` (grep `internal/session/registry.go`: `^func \(r \*ConnectionRegistry\) GetByUser` / `^func \(r \*ConnectionRegistry\) GetByGroupExcept`) — the building blocks for use case A's fan-out. `GetByUserExcept` is new but trivially derivable from these two; see Critical Files for the `userID == ""` invariant.
- `Connection.EnqueueDispatch` (grep `internal/session/registry.go`: `^func \(c \*Connection\) EnqueueDispatch`) — the non-blocking, drop-on-overflow mailbox primitive. The render path reuses this with a discriminated `DispatchRequest{Kind: KindRender}` rather than adding a parallel `EnqueueRender`.
- `pubsub.RedisBroadcaster.PublishGroupAction` / `SubscribeGroupActions` (grep `pubsub/redis.go`: `^func \(b \*RedisBroadcaster\) (PublishGroupAction|SubscribeGroupActions)`) — template for the new `PublishRenderInvalidation` and topic equivalents.
- `pubsub.DynamicSubscriber.SubscribeToGroup` (grep `pubsub/types.go`: `^type DynamicSubscriber interface`) — pattern for dynamic topic subscription on connect.
- `processBroadcasts` post-action hook site (grep `mount.go`: `^func \(h \*liveHandler\) processBroadcasts`) — the right place to add the implicit render-fan-out call.
- The existing "broadcasts inside a dispatched action are dropped" guard in `mount.go` — grep: `BroadcastAction calls inside a dispatched action are ignored`. Replicate verbatim for render-fan-out and topic broadcasts, with the same recursion-storm rationale (see also "implicit sync suppressed inside dispatched actions" in §1).

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
9. **Recursion guard.** Action handler invoked via topic dispatch calls `ctx.BroadcastToTopic` again — verify it's logged-and-dropped, matching the existing behavior for `BroadcastAction` (grep `mount.go`: `BroadcastAction calls inside a dispatched action are ignored`).
10. **Suppress implicit sync inside dispatched actions.** A topic broadcast handler that mutates state on a peer must **not** trigger another implicit-sync round to the peer's peers. Set up three connections in the same group, subscribe two to a topic, broadcast from outside, verify each subscribed connection receives exactly one update (the topic dispatch), not two (one from the topic, one from the cascaded implicit sync).
11. **Invert `TestBroadcastAction_NoAutomaticPeerDispatch`.** The existing test (grep `broadcast_test.go`: `TestBroadcastAction_NoAutomaticPeerDispatch`) explicitly asserts that peers do *not* receive updates without an explicit `BroadcastAction` call. Under the new default this is a regression test for *the old behavior* and must either be deleted or repurposed under `WithImplicitSyncDisabled()` to assert that the opt-out actually suppresses fan-out.
12. **`SubscribeTopic` on HTTP GET.** Issue a plain HTTP GET to a Mount that calls `SubscribeTopic`. Assert: no error, the response renders normally, and no `byTopic` entry is created (because there's no Connection). Then upgrade to WS for the same group and assert the subscription materializes on the WS connection.
13. **`lvt`-generated scaffolds compile against the new API.** Run `lvt new` (or the equivalent generator entry in `../lvt/internal/generator/`) against a temp dir and confirm `go build ./...` succeeds on the generated project. Guards against accidentally breaking the scaffold's compile surface when bumping `lvt`'s `go.mod` pin to the redesigned `livetemplate`. This is a sanity check, not a behavior assertion — `lvt`'s scaffolds emit zero `BroadcastAction` calls, so no semantic migration is required there.
14. **Client error envelope.** With `WithTopicACL` denying `"private/admin"` and a TS client calling `SubscribeTopic("private/admin")` in Mount: assert the client surfaces the denial as an `lvt:error` `CustomEvent` with `detail = { code: "topic_forbidden", topic: "private/admin" }` and that the WebSocket connection stays open. Covers the wire-format note in §3.
15. **Docs-e2e under stale `Sync` references.** `docs/e2e/patterns/patterns_test.go` (in the `../docs/` repo) exercises Patterns #26–#31. Pattern #26's controller calls `Sync()`, which is dead code post-PR #406; today the test passes only because the framework no longer dispatches it. Verify which pattern test cases continue to pass under the new implicit-sync behavior, and identify which ones the docs rewrite must update before the implementation lands. **Acceptance:** `grep -rn '\bSync\b' docs/ --exclude-dir=proposals` (over both `livetemplate/docs/` and `../docs/content/`) returns no stale lifecycle references (bare `Sync`, not just `Sync()`) after the docs pass. The `--exclude-dir=proposals` is required — this proposal itself records the `Sync()` removal as historical context and would otherwise be a permanent false positive; it still covers `references/`, `guides/`, `design/`, and the entire site-docs tree.

Run `go test -v -race ./...` and the existing broadcast suite (`go test -run TestWSAction_BroadcastAction -v`) to confirm no regressions in the legacy `BroadcastAction` path.

## Design constraints (must be satisfied by v1)

- **Render fan-out coalescing — hybrid bounds.** A user with 10 tabs typing in a high-frequency input could otherwise produce N × M renders per second under implicit sync. v1 must coalesce render requests per connection using **two** bounds, not just a debounce:
  - `IdleDelay` (default 10ms): time since the *last* enqueued invalidation. Resets on every enqueue. A pure-debounce design uses only this, and under sustained high-frequency mutations the timer never fires — the peer never gets an update. Not acceptable.
  - `MaxDelay` (default 50ms): time since the *first* pending invalidation. Hard ceiling — fires regardless of the idle timer. Bounds the worst-case update latency.
  - The coalescer fires when *either* bound expires. State source: the most recent state version; intermediate invalidations are dropped. Implement as two `*time.Timer` fields on `Connection` (`idleTimer`, `maxTimer`), reset/set on each `EnqueueDispatch` with `Kind == KindRender`. Both timers are cancelled when the dispatch fires. Default values are placeholders pending the benchmarks below.
- **Topic GC on disconnect.** Topics with no subscribers leak in `byTopic` if not cleaned up. `Connection.Close()` (grep: `^func \(c \*Connection\) Close`) currently drops `byUser` and `byGroup` entries — the new `byTopic` cleanup must be wired into the *same* code path, not a separate goroutine, to avoid the "no unsubscribe" trade-off described in `docs/references/pubsub.md` ("No Unsubscribe" section).
- **Cross-instance write ordering.** Persisted state writes must commit before the corresponding `RenderInvalidation` is published. See the constraint note in §1.

## Benchmarks required before implementation finalizes

Performance claims in this proposal (the "implicit sync is cheaper than BroadcastAction", the coalescing defaults of 10ms idle / 50ms max) are reasoned, not measured. The implementation PR must include the following benchmark suite in `internal/session/registry_bench_test.go` or a sibling file, and the proposal's defaults must be revisited against the data:

1. **Per-action render fan-out latency**, varying `N = peer connection count` ∈ {1, 5, 10, 50, 100}. Measures the wall-clock cost of the post-action hook from "action returns" to "all peers have enqueued a render dispatch". Establishes a baseline for the coalescer to improve on.
2. **Implicit sync vs. `BroadcastAction` cost**, same peer counts. Confirms the proposal's claim that the render-only path is meaningfully cheaper than running a controller method on each peer.
3. **Coalescer hit-rate and update latency** under three workloads: (a) 1 action/sec (no coalescing benefit), (b) 10 actions/sec (some), (c) 100 actions/sec (peer should see only ~20 updates/sec at 50ms `MaxDelay`). The `IdleDelay` and `MaxDelay` defaults must be tuned against (b) to balance perceived responsiveness against CPU cost.
4. **Cross-instance `RenderInvalidation` round-trip**, single Redis instance, 1/5/10 instances subscribed. Compares against the existing `GroupActionMessage` path so we know whether the render-invalidation path adds measurable overhead.
5. **`GetByUserExcept` vs. `GetByGroupExcept` lookup cost**, varying connections-per-user ∈ {1, 5, 50}. Both are O(N) over a slice, but `byUser` may have longer slices than `byGroup` for high-fan-out users.

Implementation PR must report these numbers in the PR description and adjust this proposal's default values if the measurements warrant. Acceptance criteria: at 100 connections/user under workload (b), the post-action hook adds < 1ms of latency over today's no-fan-out baseline.

## Open questions

- **Per-connection state.** Use case K is a real gap that implicit sync makes more visible. Worth a separate proposal for `state.PerConnection` or similar. (No `state.PerConnection` follow-up exists in `docs/proposals/` at the time of writing.)
- **Wildcard / hierarchical topics.** Out of scope for v1; `"room/*"` patterns can be added later if needed.
- **Default ACL when `WithTopicACL` is not configured.** §3 specifies `allow all`, matching today's group-broadcast posture. But topics widen the surface — under `BroadcastAction` a sender could only fan out within their own authenticated session group; with topics any caller of `SubscribeTopic("X")` from `Mount` joins fan-out for `"X"`. Question: should the default be `deny all` (forcing explicit `WithTopicACL` opt-in), with a single-line `WithOpenTopics()` builder option for developers who want the current permissive behavior? Trade-off: `allow all` keeps demos and tutorials terse; `deny all` is the safer production default.
- **Naming overlap with `lvt`'s dev-server `Broadcast()`.** `lvt`'s development server has a `Broadcast()` method (`../lvt/internal/serve/websocket.go`) that fans hot-reload notifications to all connected dev clients. Different mechanism, different concern, but readers working across both packages will see `Broadcast` and have to context-switch. Question: rename one (`Broadcast` → `NotifyClients` in `lvt`, perhaps) when this proposal ships? Cost is a small `lvt` change; benefit is conceptual clarity.
