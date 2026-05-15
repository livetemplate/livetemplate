# Publish/Subscribe Topic Model — Research & Proposal

**Status:** Proposed (supersedes the earlier "implicit peer sync + topics" two-target design)

## Context

LiveTemplate today exposes a single broadcast primitive — `ctx.BroadcastAction(action, data)` — that fans out a server-side controller invocation to all WebSocket connections in the originator's session group, sender excluded. Two ergonomic problems and one capability gap:

1. **Boilerplate at every mutation site.** The dominant pattern (`e2e/docker/app/main.go:Send`, `broadcast_test.go:syncController.Add`, `Increment`, `SetMessage`): mutate state → `ctx.BroadcastAction("RefreshFoo", nil)` → write a `RefreshFoo` controller method whose only job is to re-read freshly-persisted state.
2. **No same-user, cross-device target.** Whether `BroadcastAction` reaches a user's other devices depends entirely on how `Authenticator.GetSessionGroup` maps `userID → groupID`. With `BasicAuthenticator` it works (groupID = userID); with custom per-device groupIDs it silently doesn't.
3. **No cross-user topic broadcast.** PubSub already implements `PublishGlobal`/`PublishToGroup`/`PublishToUser`/`PublishGroupAction` (`pubsub/types.go`, grep `^type Broadcaster interface`; Redis impls in `pubsub/redis.go`, grep `^func \(b \*RedisBroadcaster\) Publish`), but the public Context API exposes only group-scoped broadcasts. No way to fan out to "everyone in chat room 42" without coercing the Authenticator into a contrived groupID.

An earlier revision of this proposal addressed these with **two** fan-out concerns: implicit user/group peer-sync *and* explicit topics. Maintainer review (2026-05-15) judged two broadcast targets to be excess API surface and a footgun source (an empty-userID fan-out hazard, a userID-vs-groupID scope rule, a state-merge tag, a render-only mode with a silent-no-op failure). **The intended outcome is now a single primitive: a named topic, with classic publish/subscribe naming, where per-identity targeting is a topic derived from the identity** — exactly the `Phoenix.PubSub` model. Per-connection state is the default; nothing fans out unless you `Publish`.

### Prior art: Phoenix (verified)

`Phoenix.PubSub` (hexdocs v2.2.0) has **no** per-user or per-connection broadcast primitive — only `subscribe(pubsub, topic)` and `broadcast(pubsub, topic, message)`. Targeting one user is purely a topic-string convention (`"user:123"`): subscribe each of that user's processes to `"user:#{id}"`, broadcast to it. Phoenix LiveView has no implicit cross-tab sync — a LiveView process is per-connection; cross-tab/device/node convergence is explicit `subscribe` in `mount/3` plus a `handle_info/2` that updates only the relevant assigns and re-renders. This proposal is Phoenix-faithful: the reconciler method below is the `handle_info` analog.

### Related prior change

The reserved `Sync()` controller method (framework auto-dispatched to peers) was removed in [PR #406](https://github.com/livetemplate/livetemplate/pull/406) two days before this proposal — that removal produced today's explicit-`BroadcastAction` boilerplate. Stale `Sync` references remain in docs (`docs/content/recipes/sync-and-broadcast.md`, Pattern #26 in `docs/content/recipes/patterns/_app/handlers_realtime.go`, and `livetemplate/docs/guides/ephemeral-components.md` — bare `Sync` in its state-init lifecycle list). The implementation PR cleans these up; see §6. **Cleanup sweep:** `grep -rn '\bSync\b' docs/ --exclude-dir=proposals` (bare word, not just `Sync()` — `ephemeral-components.md` omits the parens).

## At a glance

One `Publish`, one `Subscribe`, identity-derived topic strings. Three shapes:

```go
// Self-sync (same user, multi-device/tab) — explicit, Phoenix-style.
func (c *Ctl) Mount(s S, ctx *livetemplate.Context) (S, error) {
    ctx.Subscribe(ctx.SelfTopic())                  // explicit; no auto-subscribe
    return s, nil
}
func (c *Ctl) AddItem(s S, ctx *livetemplate.Context) (S, error) {
    c.store.Add(ctx.GetString("title"))             // mutate the shared source
    ctx.Publish(ctx.SelfTopic(), "Reload", nil)     // notify this identity's connections
    return s, nil
}
func (c *Ctl) Reload(s S, ctx *livetemplate.Context) (S, error) {
    s.Items = c.store.List()                        // re-read ONLY shared fields
    return s, nil                                   // per-connection fields untouched → preserved
}
```

```go
// Cross-user room — same primitive, a developer-named topic.
func (c *ChatCtl) Mount(s S, ctx *livetemplate.Context) (S, error) {
    s.RoomID = ctx.Param("room")
    ctx.Subscribe("room/" + s.RoomID)               // ACL-gated (deny-all default)
    return s, nil
}
func (c *ChatCtl) Send(s S, ctx *livetemplate.Context) (S, error) {
    msg := c.persist(ctx.GetString("body"))
    ctx.Publish("room/"+s.RoomID, "NewMessage", map[string]any{
        "id": msg.ID, "author": ctx.UserID(), "body": msg.Body,
    })
    return s, nil
}
```

```go
// Out-of-band (webhook / cron) — handler-level, no Context.
handler.Publish(livetemplate.UserTopic("alice"), "DM", map[string]any{"from": "bob"})
handler.Publish("auction/42", "BidUpdate", bidData)
handler.Publish(livetemplate.GlobalTopic(), "Maintenance", map[string]any{"at": deadline})
```

One concern, one code shape. Identity targeting (`SelfTopic()`, `UserTopic`, `SessionTopic`, `GlobalTopic`) is just a topic-name helper.

## Use cases (exhaustive enumeration)

| # | Use case | Under the pub/sub topic model |
|---|---|---|
| A | Same user, multi-device sync (phone → desktop) | `ctx.Subscribe(ctx.SelfTopic())` in Mount + `ctx.Publish(ctx.SelfTopic(), "Reload", …)` after mutation + a `Reload` reconciler that re-reads shared fields. Works regardless of the Authenticator's groupID strategy — `SelfTopic()` keys on `UserID` when authenticated. |
| B | Same browser, multi-tab (anonymous) | Identical; `SelfTopic()` resolves to `lvt:session:<GroupID>`. |
| C | Cross-user collaborative (chat, shared doc) | `ctx.Subscribe("room/"+id)` + `ctx.Publish("room/"+id, "NewMessage", data)` + handler. |
| D | Cross-user, auth-gated room | Same + `WithTopicACL` gating `room/*`. |
| E | Anonymous-readable topic (auction bids, ticker) | Same + ACL allows / `WithOpenTopics()`. |
| F | Server push to one user (webhook/cron) | `handler.Publish(livetemplate.UserTopic("alice"), "DM", data)`. |
| G | Server push to one topic | `handler.Publish("auction/42", "BidUpdate", data)`. |
| H | Global announcement | Subscribers `ctx.Subscribe(livetemplate.GlobalTopic())`; `handler.Publish(livetemplate.GlobalTopic(), "Maintenance", data)`. |
| I | Sender exclusion (originating tab updates via its own action response) | The calling connection is excluded from its own `Publish` by default. Preserved. |
| J | HTTP POST mutating state, fan out to peer WS connections | After the action, `ctx.Publish(ctx.SelfTopic(), "Reload", …)` runs regardless of transport; WS subscribers reconcile, the POST gets its normal response. |
| K | Per-connection state that should *not* sync (collapsed-panel flag, draft text) | **Default, correct by construction.** The reconciler writes only shared fields; per-connection fields in each receiver's *own* state are never written, so they survive. No tag, no merge, no API. |

C/D/E/F/G/H are the capabilities missing today; A/B/I/J have working machinery (`handleDispatchedAction`, `GetByGroupExcept`) the model reuses; K is free.

## Current architecture (key references)

Grep anchors (CLAUDE.md) so they don't drift.

- **`ctx.BroadcastAction`** queues `broadcastRequest{action,data}`. Grep `^func \(c \*Context\) BroadcastAction` in `context.go`; cap `MaxBroadcastsPerAction = 100`. **Removed by this proposal.**
- **`dispatchBroadcastToGroup` / `processBroadcasts`** run after the action returns. Grep `^func \(h \*liveHandler\) dispatchBroadcastToGroup` / `^func \(h \*liveHandler\) processBroadcasts` in `mount.go`. `Publish`'s local fan-out reuses this shape (registry lookup + `EnqueueDispatch`); cross-instance reuses `PublishGroupAction`'s Redis pattern.
- **`Connection.DispatchChan`** is the per-connection serial mailbox. Grep `DispatchChan chan \*DispatchRequest` in `internal/session/registry.go`; the event loop selects on it (grep `case req := <-connection.DispatchChan` in `mount.go`) and invokes `handleDispatchedAction` (grep `^func \(h \*liveHandler\) handleDispatchedAction`), which **runs the controller method against the receiving connection's own state**. This is the load-bearing fact that makes the reconciler pattern correct with no merge machinery: each receiver's state is independent, so a selective reconciler leaves per-connection fields intact.
- **`Connection.UserID` / `GroupID`** are populated at `Register` (grep `^func \(r \*ConnectionRegistry\) Register`); `byUser`/`byGroup` are indexed there. `SelfTopic()` is computable from these fields with no new index.
- **`Authenticator.GetSessionGroup`** (grep `GetSessionGroup\(r \*http.Request` in `auth.go`): `BasicAuthenticator` returns `userID`; `AnonymousAuthenticator` returns a cookie-bound ID. Unchanged.
- **PubSub** (`pubsub/types.go`, `pubsub/redis.go`) — `PublishGroupAction`/`SubscribeGroupActions` are the template for the new single topic channel; `subscribedChannels` ref-count + `reconnect()` replay is the template for pattern subscriptions.

## Proposed design

### Principle: two concerns, not three

1. **State scope** — who shares the same persisted `State`. Decided by `Authenticator.GetSessionGroup`; **unchanged**.
2. **Topic fan-out** — the single, explicit publish/subscribe mechanism. Identity targeting is a derived topic name; per-connection is the default.

The earlier revision had a third "auto-sync scope" concern (implicit peer sync). It is **deleted** — collapsed into (2): self-sync is `Subscribe(SelfTopic())` + `Publish` + a reconciler.

### 1. Self-sync = Subscribe(SelfTopic()) + Publish + reconciler

`ctx.SelfTopic()` returns this connection's identity-derived topic: `lvt:user:<UserID>` when authenticated, else `lvt:session:<GroupID>`. The developer **explicitly** subscribes to it in `Mount` (Phoenix-style; no framework auto-subscribe — no hidden lifecycle). After a mutation, `ctx.Publish(ctx.SelfTopic(), "Reload", …)` dispatches the `Reload` action to every connection of that identity (sender excluded). Each receiver runs `Reload` against **its own** `connState.state` (verified behavior of `handleDispatchedAction`).

**Invariant — both identity fields empty is a programmer error.** Removing the old empty-`userID` footgun moved the symmetric risk to the anonymous path: if a custom `Authenticator.GetSessionGroup` returns `""` *and* the user is unauthenticated, `lvt:session:` would be a single degenerate topic fanning out across every such connection. `SelfTopic()` MUST guard this: when both `UserID` and `GroupID` are empty it logs `slog.Error("SelfTopic called with empty UserID and GroupID")` and returns `""`; `Subscribe("")`/`Publish("")` are rejected with an error (never a wildcard, never the reserved root). This mirrors the earlier revision's `GetByUserExcept`-empty-userID decision (log-and-degrade, don't panic production). `BasicAuthenticator`/`AnonymousAuthenticator` never hit this; it only catches a misimplemented custom Authenticator.

**Per-connection state is safe by construction (use case K).** The reconciler (`Reload`) writes only shared fields — from a payload or a shared source (the controller singleton's mutex-guarded data, or the group-keyed session store). Fields that are per-connection (a collapsed-panel bool, a draft string) live in the receiver's own state and are simply never written by the reconciler, so they survive the re-render unchanged. This is exactly Phoenix's `handle_info` semantics. There is **no** state-merge, **no** `lvt:"local"` tag, **no** opt-out flag — the earlier revision's §1a is gone, and the silent-no-op failure of a render-only mode is gone with it (there is no render-only mode; every `Publish` invokes a method the developer wrote).

**Ordering.** Mutate the shared source *before* `Publish` so the reconciler reads fresh data. For a synchronously-mutated controller singleton this is trivially true. For a reconciler that re-reads the group-keyed session store, the originator's `persistState` write must commit before the dispatch is published cross-instance — the existing `processBroadcasts`-after-`persistState` ordering already provides this; document it as a hard requirement so session-store implementors don't switch to async writes silently.

**The reconciler is not "boilerplate to eliminate."** It is the explicit, selective reconciliation point — the thing that makes per-connection state correct and makes "what converges" reviewable in code. This is a deliberate trade: one small method per sync pattern, in exchange for one mental model and zero implicit fan-out.

### 2. Topics (the one primitive)

A connection `Subscribe`s to named topics; `Publish` sends `(action, data)` to a topic; every subscriber (local + cross-instance) runs `action` against its own state via `handleDispatchedAction`. There is exactly one mode (always invoke a handler method) — no render-only mode.

**Subscription (Mount):**
```go
ctx.Subscribe("room/" + state.RoomID)   // or ctx.Subscribe(ctx.SelfTopic())
```

**Publish (action):**
```go
ctx.Publish("room/"+state.RoomID, "NewMessage", map[string]any{"id": id, "body": body})
```
The receiver-side controller defines a `NewMessage` method (same dispatch path as today's `BroadcastAction` — `handleDispatchedAction` → `DispatchWithState`). The calling connection is excluded by default (use case I).

**Dispatch model: named-action, not a `handle_info` inbox (resolved decision).** `Publish`'s `action` resolves to a controller method *by name*, through the **exact same** `DispatchWithState` resolver that routes user-initiated (button/form) actions — `handleDispatchedAction` at `mount.go` (grep `^func \(h \*liveHandler\) handleDispatchedAction`) builds a `Context` with the action and calls `DispatchWithState` unchanged. We deliberately do **not** introduce a Phoenix-style single inbox method (`handle_info`/`OnPublish`): Go has no pattern matching, so one inbox would force a manual `switch` on a message-type string in every controller — more boilerplate and less type safety than one well-named method per message — and a reserved magic method is exactly the shape PR #406 removed (`Sync()`). Named dispatch adds zero new machinery, which is this proposal's core low-risk property.

**Symmetry (the consequence, by design).** Because topic dispatch and user-action dispatch share one resolver, a topic action and a same-named user action invoke the **same method**: `ctx.Publish(topic, "Delete", …)` runs the same `Delete` a `<button name="Delete">` triggers. This is intentional and symmetric — server-pushed and client-initiated invocations of the same logical action are the same handler (the same property the removed `Sync()` had). It is **not** a separate namespace. Practical guidance for the docs/scaffolds: name a topic action for the *receiver's* reaction (`"NewMessage"`, `"Reload"`, `"PresenceChanged"`), and do not reuse a destructive user-action name (`"Delete"`, `"Submit"`) as a topic action unless you intend peers to run exactly that handler. **Why this matters concretely:** because `Publish(topic, "Delete", …)` and a `<button name="Delete">` resolve to the *same* `Delete` method, a careless `Publish(topic, "Delete", …)` would run `Delete` on **every subscribed peer connection** — i.e. one server-side broadcast could delete records for every viewer, not just the originator. The recursion guard (a dispatched action that calls `Publish` is logged-and-dropped) bounds the blast radius to one hop, but it does not stop the first hop — name topic actions for what the *receiver* should do, never after a mutation the *sender* performs.

**Out-of-band (webhook/cron):** `handler.Publish(topic, action, data)` — same primitive, no `Context`. Identity helpers are pure string constructors usable anywhere: `livetemplate.UserTopic(userID)`, `livetemplate.SessionTopic(groupID)`, `livetemplate.GlobalTopic()`.

**Reserved `lvt:` namespace + anti-spoof (security, baked in — not optional).** Identity-derived topics live under the reserved `lvt:` prefix. `Subscribe` **rejects** any `lvt:`-prefixed topic that is not the caller's own `SelfTopic()` or `GlobalTopic()` — a connection cannot subscribe to another user's `lvt:user:<x>`. Developer topic names must not start with `lvt:`. This structurally removes the earlier revision's empty-userID fan-out footgun: the anonymous self-topic is the specific non-empty `GroupID`, never an empty key matching all anonymous connections.

**Subscription is server-driven.** `Subscribe` is called from controller code (`Mount`/actions); the client sends no subscribe message. On WS reconnect the server re-runs `Mount`, which re-subscribes. `Subscribe` on an HTTP GET is a no-op for the subscription itself (it only materializes with a `Connection`) but the ACL still runs eagerly so an unauthorized subscribe is rejected before the WS upgrade.

**Wildcard / hierarchical topics** (in v1, resolved decision). A subscription may end in a single trailing `*` segment:

```go
ctx.Subscribe("room/*")                       // receives room/42, room/99, …
ctx.Publish("room/42", "NewMessage", data)    // reaches room/* subscribers + room/42 subscribers
```

- **Grammar.** Canonical separator `/` (HTTP-path convention). Topic chars `[a-zA-Z0-9/_-]+`. The only wildcard form is a single trailing `*` as the final segment (`room/*`, `user/alice/*`). Mid-pattern globs (`room/*/log`) are out of v1 (§"Deferred"). The Redis transport keeps its `livetemplate:topic:` channel prefix; the slash stays literal in the channel suffix.
- **Matcher.** No glob matcher exists in the codebase; v1 adds a tiny one — trailing-`*` reduces to `strings.HasPrefix(concreteTopic, pattern[:len(pattern)-1])`. No regex, no trie.
- **Registry.** Add `byTopicPattern map[string][]*Connection` alongside the exact `byTopic` map. A publish to concrete `room/42` resolves to `union(byTopic["room/42"], { conns in byTopicPattern[p] : p matches "room/42" })`, **deduplicated by connection identity** — a connection subscribed to *both* `room/42` and `room/*` receives **exactly one** frame. Pattern scan is O(P) over distinct patterns; exact lookup stays O(1).
- **ACL receives the pattern, not the concrete topic.** `WithTopicACL` is called once at `Subscribe("room/*")` with `topic == "room/*"` — a coarser question ("may this user subscribe to the whole `room/*` space?"). Apps needing per-room authorization subscribe to concrete topics.
- **New topics auto-match existing wildcard subscribers — by design.** A first-ever `Publish("room/99", …)` reaches existing `room/*` subscribers with **no** re-invocation of the ACL. Intentional (you don't re-authorize per never-before-seen room); documented so it is not mistaken for an authorization bug.
- **Cross-instance.** A publish (`ctx.Publish` or `handler.Publish`) to a *concrete* topic always `PUBLISH`es to the single exact channel `livetemplate:topic:{name}` (e.g. `livetemplate:topic:room/42`) — publishers never publish to patterns. Subscribers receive it two ways: an exact subscriber holds `SUBSCRIBE livetemplate:topic:room/42`; a wildcard subscriber holds `PSUBSCRIBE livetemplate:topic:room/*`, and Redis pattern-matching delivers the concrete publish to it (Redis `*` spans `/`). So `handler.Publish("room/42", …)` from instance A reaches a `room/*` subscriber on instance B with no extra publisher-side logic. Track wildcard subscriptions in a new `subscribedPatterns map[string]int` parallel to the existing `subscribedChannels` ref-count map in `pubsub/redis.go`, replayed in `reconnect()` exactly like the exact-channel set. Extends the existing subscription-tracking mechanism; not a new architecture.

### 3. Topic ACL (auth gating)

A single global hook configured at template construction. It is called once per `Subscribe`, with the literal subscribed name (the pattern `"room/*"`, not concrete matches):

```go
template := livetemplate.New("app",
    livetemplate.WithTopicACL(func(topic, userID string, r *http.Request) (allowed bool, err error) {
        switch {
        case strings.HasPrefix(topic, "public/"):
            return true, nil
        case strings.HasPrefix(topic, "room/"):   // pattern-aware: covers "room/*" and "room/42"
            return userID != "" && c.UserMayJoinRooms(userID), nil
        }
        return false, fmt.Errorf("unknown topic: %s", topic)
    }))
```

On `(false, _)` the subscription is rejected with `ErrTopicForbidden`.

**Default is `deny all`** (resolved decision). With neither `WithTopicACL` nor `WithOpenTopics()`, every `Subscribe` returns `ErrTopicForbidden`. Topics are identity-agnostic and cross-user — the ACL is their **only** boundary (a group broadcast is Authenticator-bounded; a topic is not). Defaulting that single boundary open is a footgun, and the docs-site pattern scaffolds get copy-pasted into user projects, so an allow-all default would teach an insecure idiom. The opt-in is one self-documenting line:

```go
template := livetemplate.New("app", livetemplate.WithOpenTopics()) // every topic public; explicit
```

`WithOpenTopics()` and `WithTopicACL(fn)` are mutually exclusive (both set = construction-time error). Discovery path: `Subscribe` with no config → `ErrTopicForbidden` → you learn you must configure one.

**Self/global topics are ACL-exempt — with different rationales.** `ctx.SelfTopic()` is exempt because you are inherently authorized for your own identity's topic, and the reserved-namespace rule already prevents subscribing to anyone else's. `livetemplate.GlobalTopic()` is exempt for a *different* reason: it is read-only-from-the-client (subscribing only means "I will receive global announcements" — benign) and exempting it keeps maintenance-banner recipes working under deny-all. The ACL only gates developer-named (non-`lvt:`) topics; deny-all never breaks self-sync. **Caution (must land in the docs):** `GlobalTopic()` is ACL-exempt **and** its fan-out is *every connected user* — only call `Subscribe(livetemplate.GlobalTopic())` in controllers genuinely designed to receive global announcements, and treat `handler.Publish(livetemplate.GlobalTopic(), …)` as the highest-blast-radius call in the API. It is not a convenient "broadcast to lots of people" shortcut.

**Wire-format note (client surface).** An ACL denial on the WS-connect path makes `Mount` return an error the current TS client cannot distinguish from a generic connection failure. The implementation should extend the WS response envelope with `{ "type": "error", "code": "topic_forbidden", "topic": "..." }`. The TS client's `handleWebSocketPayload` (grep `livetemplate-client.ts`: `handleWebSocketPayload`) already shape-tests for upload fields before falling back to `UpdateResponse`; the same pattern admits a `type === "error"` branch surfaced as an `lvt:error` `CustomEvent`. This is the **only** client-side change this proposal requires.

**ACL is evaluated at subscribe time, not per broadcast** — consistent with WS session-lifetime authorization (a revoked session keeps receiving until the connection drops or the controller calls `ctx.Unsubscribe`). For per-message authorization, check inside the receiver method. Immediate revocation requires dropping the connection or `ctx.Unsubscribe` from a server-side action.

**The `*http.Request` passed to the ACL is the WS upgrade request**, captured at connection establishment. Request-scoped values from HTTP middleware (a JWT in `r.Context()`) are available; DB-mutated permissions since the upgrade are not unless the hook re-queries the source of truth.

**ACL on HTTP GET is a hot path.** `Subscribe` is a no-op on GET but the ACL still runs eagerly. For a high-traffic page with a DB-backed ACL: prefer stateless checks (JWT claims); or return early on `r.Method == "GET"` and defer to a WS-only path (loses "rejected before WS upgrade" — a deliberate trade-off). Recommendation: stateless by default.

**Neither `ctx.Publish` nor `handler.Publish` runs the ACL.** Send and receive are independent: the Subscribe-time ACL gates *who reads* a topic. `ctx.Publish` callers are already gated by whether they can invoke the action at all (existing authorization); `handler.Publish` is trusted server code (webhook/cron) with no caller identity to check — it deliberately skips the ACL, exactly like the `Session.TriggerAction` it replaces for use case F. Send-side gating, when wanted, belongs in the action handler, not the topic layer.

### 4. Migration — `BroadcastAction` is removed

Pre-release: the library has no production users outside the ecosystem repos. `ctx.BroadcastAction` is **removed**, not deprecated — a clean break. Every call site migrates this wave to `Publish` + a reconciler/handler. The paired echo methods are not deleted but **repurposed** as reconcilers where peers must reflect a change.

**In `livetemplate/` itself:**
- `e2e/docker/app/main.go:Send` — `BroadcastAction("RefreshMessages", nil)` → `ctx.Publish(ctx.SelfTopic(), "Reload", nil)`; `RefreshMessages` → a `Reload` reconciler (re-read shared messages).
- `broadcast_test.go` — echo methods are **not** uniformly `Refresh*`-named: `Increment` → `RefreshCount`, `SetMessage` → `SyncMessage`, `syncController.Add` → `Refresh`. Migrate each to a `Publish(SelfTopic(), …)` + reconciler; a `Refresh*` grep alone misses `SyncMessage`.
- `context_broadcast_test.go` (14 sites), `lifecycle_integration_test.go` (5), `handle_test.go` (4), `navigate_test.go` (5) — authoritative in-repo migration checklist (cross-ref §6 `livetemplate` row). `TestBroadcastAction_NoAutomaticPeerDispatch` asserted "no peer dispatch without an explicit call" — still true under pub/sub (nothing fans out without `Publish`); repurpose it to assert that, not delete it.

**In `examples/` (verified `grep -rn "BroadcastAction\b" examples/`):**
- `landing-demo/main.go` (3: Increment/Decrement/Reset), `shared-notepad/main.go` (1: Save), `todos/controller.go` (4: Add/Toggle/Delete/Update) — each becomes `Subscribe(ctx.SelfTopic())` in Mount + `Publish(ctx.SelfTopic(), "Reload", nil)` + a `Reload` reconciler (use case A/B).
- `chat/main.go` (3: UserJoined/NewMessage/UserLeft) — `Subscribe("chat/"+room)` + `Publish("chat/"+room, "NewMessage", data)` + handlers (use case C).

**In `tinkerdown/`:** `examples/literate-counter-include/_app/counter.go` and `examples/literate-linked-include/_app/counter.go` (3 each). Migrate to `Subscribe(ctx.SelfTopic())`+`Publish`; the tutorial comments must clarify `sharedAuth` (constant groupID) is an artificial teaching setup.

**Empty echo methods** that carried no reconciliation logic (pure re-render triggers) become dead code; ones that re-read shared data become the reconciler. Grep both `Refresh*` and `Sync*` — naming is not uniform.

### 5. Final API surface

**On `*Context`:**
- `ctx.Subscribe(topic string) error` — wildcard trailing-`*` allowed; ACL deny-all default except self/global; no-op subscription on HTTP GET (ACL still runs).
- `ctx.Unsubscribe(topic string)`.
- `ctx.Publish(topic string, action string, data map[string]any) error` — invoke `action(data)` on each subscriber; calling connection excluded by default.
- `ctx.SelfTopic() string` — `lvt:user:<UserID>` if authenticated, else `lvt:session:<GroupID>`. ACL-exempt; reserved namespace.

**Package-level (usable from anywhere, incl. webhook/cron):**
- `livetemplate.UserTopic(userID) string`, `livetemplate.SessionTopic(groupID) string`, `livetemplate.GlobalTopic() string` — pure reserved-name constructors.

**On `LiveHandler`:**
- `handler.Publish(topic, action, data)` — out-of-band, no `Context`.

**On the template builder:**
- `WithTopicACL(fn)` — called once per `Subscribe` with the literal name (pattern, not concrete).
- `WithOpenTopics()` — opt into permissive topics; mutually exclusive with `WithTopicACL`; required because the default is deny-all.

**Removed (the simplification payoff):** `ctx.BroadcastAction`; `ctx.SkipPeerSync`; `WithImplicitSyncDisabled`; `handler.BroadcastToUser`; `handler.BroadcastGlobal` (now `Publish` to `UserTopic`/`GlobalTopic`); the `lvt:"local"` tag and its merge machinery; render-only dispatch mode; `GetByUserExcept`; the `RenderInvalidation` message type and the `livetemplate:render:{groupID}` channel; the userID-vs-groupID fan-out scope rule; the render-fan-out coalescing-bounds design.

**Unchanged:** `Session` interface and `Session.TriggerAction` (still valid for goroutines holding a captured Session; orthogonal to topics — it dispatches to one session, not a topic); `Authenticator`; all PubSub interfaces.

## Impacted repositories

| Repo (`../<name>/`) | pin | `BroadcastAction` sites | Migration |
|---|---|---|---|
| `livetemplate` (this repo) | — | `e2e/docker/app/main.go`, `broadcast_test.go`, `context_broadcast_test.go`, `lifecycle_integration_test.go`, `handle_test.go`, `navigate_test.go` | Core implementation; rewrite e2e Docker app; migrate/repurpose the test files (see §4). |
| `lvt` | `v0.8.23-0.2026...` | **0** | `go.mod` pin bump **plus an internal rename** (resolved): `WebSocketManager.Broadcast()` → `ReloadClients()` in `internal/serve/` — 3 files: `websocket.go` (def), `server.go` (the single production caller), `websocket_test.go` (`TestWebSocketManager_Broadcast` + 2 calls → `TestWebSocketManager_ReloadClients`). Internal-only, zero user-facing impact; the name is accurate (it only emits `{"type":"reload"}` on file change) and removes the cross-repo `Broadcast` collision. Scaffolds emit CRUD only — no migration. |
| `client` (TS) | `0.9.0` | N/A | One change: a `type === "error"` branch in `handleWebSocketPayload` for `topic_forbidden` (§3 wire-format note). Otherwise the stateless diff path is unaffected. |
| `examples` | `v0.9.0` | 11 sites / 4 apps (§4) | `landing-demo`/`shared-notepad`/`todos` → `Subscribe(SelfTopic())`+`Publish`+reconciler; `chat` → developer topic + handlers. |
| `tinkerdown` | `v0.8.16` (stale) | 6 sites / 2 examples | Bump to `v0.9.x`; migrate to `Subscribe(SelfTopic())`+`Publish`; clarify `sharedAuth` is teaching-only. |
| `devbox-dash` | (single-user) | 0 | Routine version bump. |
| `docs` (site) | `v0.8.23` | 22 content files; `handlers_realtime.go` Patterns #27/#28 use it; Pattern #26 still references removed `Sync()` | **Heaviest impact.** See "Docs migration scope". |

**Docs migration scope** — two surfaces:

- **In-repo contributor docs** (`livetemplate/docs/`): `references/controller-pattern.md`, `references/pubsub.md`, `design/ARCHITECTURE.md`, `guides/ephemeral-components.md` (stale bare `Sync` in its state-init list).
- **Site docs** (`../docs/content/`, verified `grep -rln "BroadcastAction" docs/content/`):
  - **Top-of-funnel** — `index.md`, `getting-started/your-first-app.md`.
  - **Reference** — `reference/api.md`, `reference/controller-pattern.md`, `reference/server-actions.md`, `reference/session.md`, `reference/pubsub.md`, `reference/navigate.md`, `reference/limitations.md`.
  - **Guides** — `guides/standard-html-reactivity.md`, `guides/progressive-complexity.md`.
  - **Recipes** — `recipes/broadcasting.md` (rewrite end-to-end as pub/sub), `recipes/sync-and-broadcast.md` (rewrite/retire — the `Sync()` half is stale post-#406), `recipes/counter/index.md`, `recipes/architecture-flow.md`, `recipes/progressive-enhancement/index.md`, `recipes/todos/index.md`.
  - **Pattern scaffolds** (critical path — copy-pasted into user projects) — `recipes/patterns/_app/templates/realtime/{broadcasting,multi-user-sync,server-push}.tmpl` + `recipes/patterns/_app/handlers_realtime.go` (Patterns #26–#28). Rewrite to `Subscribe`/`Publish`/reconciler.
  - **Deny-all ripple (must-fix):** any scaffold/recipe calling `Subscribe` **fails out of the box** under deny-all (`ErrTopicForbidden`). The docs-PR author must add `livetemplate.WithOpenTopics()` (with a one-line "topics are deny-by-default; this is the public-demo opt-in" comment — itself pedagogical) **or** a real `WithTopicACL` example. Self-sync recipes are exempt (`SelfTopic()` bypasses the ACL).
  - **Historical** — `changelog.md` gets an entry, not a rewrite.

**Release order (single coordinated wave):** 1. `livetemplate` core + tests → 2. `client` error-envelope (parallel) → 3. **`docs`** recipes/reference/scaffolds (gating — scaffolds are copy-pasted) → 4. `lvt` pin bump + `ReloadClients()` rename → 5. `examples` migration → 6. `tinkerdown`/`devbox-dash` lag bumps.

## Critical files to modify (for the implementation that would follow)

- `context.go` — add `Subscribe`, `Unsubscribe`, `Publish`, `SelfTopic`; reuse the existing `broadcasts []broadcastRequest` slice pattern for a `topicPublishes` queue (drained after the action like `processBroadcasts`). `Subscribe` validates the grammar (`[a-zA-Z0-9/_-]+`, optional single trailing `*`), enforces the reserved-`lvt:` anti-spoof rule, and runs the ACL (deny-by-default; self/global exempt) before recording.
- package file (e.g. `topics.go`) — `UserTopic`/`SessionTopic`/`GlobalTopic` constructors + a shared reserved-namespace validator used by both `Subscribe` and the constructors.
- `config.go` / builder — `WithTopicACL(fn)` and `WithOpenTopics()`; both set = hard error at `New(...)`; neither = deny-all.
- `mount.go` — add `dispatchToTopic` next to `dispatchBroadcastToGroup` (reuse the registry-lookup + `EnqueueDispatch` shape); wire the post-action drain of `topicPublishes`; reuse the existing recursion guard (grep `BroadcastAction calls inside a dispatched action are ignored`) so a `Publish` inside a dispatched action is logged-and-dropped. **No** `dispatchPeerSyncToUser`, **no** render-only handler, **no** `RenderInvalidation`.
- `internal/session/registry.go` —
  - (a) `byTopic map[string][]*Connection` (exact) **and** `byTopicPattern map[string][]*Connection` (trailing-`*`).
  - (b) `SubscribeConnectionToTopic` / `UnsubscribeConnectionFromTopic` / `GetByTopicExcept`. `GetByTopicExcept(concrete, excludeConn)` returns `union(byTopic[concrete], { conns in byTopicPattern[p] : p matches concrete })` **deduplicated by `*Connection` identity**; pattern match = the trailing-`*` `HasPrefix` helper, O(P).
  - (c) Wire **both** topic maps into the existing `Unregister()` cleanup path (grep `^func \(r \*ConnectionRegistry\) Unregister`), where `byUser`/`byGroup` cleanup already happens. **Not** `Connection.Close()` (it delegates to `Unregister()`).
  - (d) Optionally add a `Kind` field to `DispatchRequest` as a forward-compatible placeholder (single value `KindAction` in v1; zero-valued, backward-compatible). **Drop `GetByUserExcept` entirely** (not needed; the empty-userID footgun never exists).
- `pubsub/types.go`, `pubsub/redis.go` — one new channel scheme `livetemplate:topic:{name}` for `Publish` (exact `SUBSCRIBE`) and `PSUBSCRIBE livetemplate:topic:{prefix}*` for wildcard subscriptions; add `subscribedPatterns map[string]int` parallel to `subscribedChannels`, replayed in `reconnect()`. Envelope reuses the `GroupActionMessage` JSON shape (`{"type","action","data","timestamp","instanceID"}`). **No** `livetemplate:render:{groupID}` channel.
- `auth.go` — no change.
- `e2e/docker/app/main.go` — migration target (§4).
- In-repo `docs/references/{controller-pattern,pubsub}.md`, `docs/design/ARCHITECTURE.md` — document the two-concern pub/sub model. Site docs scoped in §6.

## Existing utilities to reuse

- `handleDispatchedAction` (grep `^func \(h \*liveHandler\) handleDispatchedAction` in `mount.go`) — runs the action against the receiver's own state; `Publish` reuses it unchanged. This is what makes the reconciler/per-connection-state guarantee free.
- `Connection.EnqueueDispatch` (grep `^func \(c \*Connection\) EnqueueDispatch`) — non-blocking, drop-on-overflow mailbox; `Publish`'s local fan-out enqueues one per subscriber.
- `dispatchBroadcastToGroup` (grep in `mount.go`) — structural template for `dispatchToTopic` (registry lookup → `EnqueueDispatch` loop → cross-instance publish).
- `pubsub.RedisBroadcaster.PublishGroupAction` / `SubscribeGroupActions` (grep `pubsub/redis.go`) — template for the single topic channel; `subscribedChannels` + `reconnect()` replay — template for `subscribedPatterns`.
- The existing recursion guard (grep `BroadcastAction calls inside a dispatched action are ignored`) — reused verbatim for `Publish`.
- `processBroadcasts` post-action hook site (grep `^func \(h \*liveHandler\) processBroadcasts`) — the drain point for the new `topicPublishes` queue, preserving persist-before-publish ordering.

## Verification plan (when the implementation lands)

E2E tests (mirroring `broadcast_test.go` structure):

1. **Self-sync, two devices, one user.** Custom Authenticator → per-device groupIDs. Alice connects two WS (different deviceID), both `Subscribe(ctx.SelfTopic())` in Mount. Tab 1 mutates + `Publish(SelfTopic(), "Reload", nil)`. Assert Tab 2's `Reload` runs and it receives a diff. Confirms A regardless of groupID strategy.
2. **Self-sync, anonymous.** Two tabs via `AnonymousAuthenticator` (`SelfTopic()` → `lvt:session:<gid>`). Confirms B.
3. **Per-connection field preserved (K).** Same as test 1; Tab 1 also mutates a per-connection field that the `Reload` reconciler does **not** write. Assert Tab 2's shared field updates while Tab 2's own per-connection field is unchanged. No tag involved — it works because `Reload` is selective.
4. **Cross-user topic, public.** Two anon tabs `Subscribe("public/feed")` (under `WithOpenTopics()`); one `Publish`es; the other's handler runs. Confirms E.
5. **Topic ACL, denied.** `WithTopicACL` returns `(false, nil)` for `"private/admin"`; `Subscribe("private/admin")` → `ErrTopicForbidden`. Confirms D.
6. **Self/global ACL-exempt under deny-all.** No `WithTopicACL`, no `WithOpenTopics()`: `Subscribe(ctx.SelfTopic())` and `Subscribe(livetemplate.GlobalTopic())` succeed; `Subscribe("room/1")` → `ErrTopicForbidden`.
7. **Reserved-namespace anti-spoof.** Connection for user `bob` calling `Subscribe(livetemplate.UserTopic("alice"))` (i.e. `lvt:user:alice`) is rejected. Any non-self `lvt:`-prefixed `Subscribe` is rejected.
8. **Out-of-band.** Goroutine without a Context: `handler.Publish(livetemplate.UserTopic("alice"), "DM", data)` runs alice's `DM`; `handler.Publish(livetemplate.GlobalTopic(), "Maint", data)` reaches global subscribers. Confirms F/H.
9. **Cross-instance.** Two `liveHandler`s behind a shared `RedisBroadcaster`. `Publish` on instance A reaches a subscriber on instance B over the single `livetemplate:topic:{name}` channel.
10. **Sender exclusion.** The connection that calls `Publish` does not receive its own dispatch (use case I).
11. **Recursion guard.** A handler invoked via `Publish` calls `Publish` again → logged-and-dropped (reuses the existing `BroadcastAction`-era guard).
12. **`Subscribe` on HTTP GET.** Plain GET to a Mount calling `Subscribe`: no error, normal render, no `byTopic` entry; ACL still ran. Upgrade to WS for the same identity → subscription materializes.
13. **`lvt` scaffolds compile.** `lvt new` → `go build ./...` succeeds against the redesigned `livetemplate` (sanity for the pin bump; scaffolds emit no broadcasts).
14. **Client error envelope.** `WithTopicACL` denies `"private/admin"`; TS client `Subscribe("private/admin")` → `lvt:error` `CustomEvent` `{ code:"topic_forbidden", topic:"private/admin" }`; WS stays open.
15. **Docs-e2e stale `Sync`.** `docs/e2e/patterns/patterns_test.go` exercises Patterns #26–#31; #26 calls dead `Sync()`. Identify which cases the docs rewrite must update. **Acceptance:** the bare-`Sync` lifecycle reference is gone from both `livetemplate/docs/` and `../docs/content/`. The naive `grep -rn '\bSync\b'` over-matches Go-stdlib `sync.Mutex`/`sync.Once`/`"sync"` inside doc code fences, so the acceptance check is `grep -rnE '\bSync\b' docs/ --include='*.md' --exclude-dir=proposals | grep -vE 'sync\.[A-Za-z]|"sync"|/sync\b'` returning **no** results in a lifecycle-hook context (the residual hits, if any, must all be Go `sync` package usages, not the removed `Sync()` controller hook). `--exclude-dir=proposals` still required (this proposal records `Sync()` as historical context).
16. **Deny-all default.** No ACL config: `Subscribe("anything")` → `ErrTopicForbidden`. `WithOpenTopics()` → succeeds. `WithTopicACL` → hook decides. Both set → hard error at `New(...)`.
17. **Wildcard fan-out + dedupe.** `Subscribe("room/*")`; `Publish("room/42", …)` reaches it. A connection subscribed to **both** `room/42` and `room/*` gets **exactly one** frame. First-ever `Publish("room/99", …)` reaches the `room/*` subscriber with the ACL **not** re-invoked. Cross-instance: a `room/*` subscriber on instance B receives instance A's `room/42` via `PSUBSCRIBE`.
18. **ACL receives the pattern.** On `Subscribe("room/*")` the hook's `topic` argument is the literal `"room/*"`.

Run `go test -v -race ./...`.

## Design constraints (must be satisfied by v1)

- **Topic GC on disconnect.** Topics with no subscribers must not leak in `byTopic`/`byTopicPattern`. Cleanup wires into the existing `Unregister()` index-cleanup path (where `byUser`/`byGroup` already clean up) — not a separate goroutine.
- **Cross-instance ordering.** When a reconciler re-reads the group-keyed session store, the originator's `persistState` write must commit before the dispatch is published cross-instance — the existing `processBroadcasts`-after-`persistState` ordering provides this; document it as a hard requirement.
- **No implicit fan-out.** `Publish` is always an explicit developer call. There is no high-frequency implicit render path, so the earlier revision's render-fan-out coalescer is not needed; if a single developer `Publish`-per-keystroke pattern emerges, debouncing is the developer's call at the call site (same as any action).

## Benchmarks required before implementation finalizes

1. **Topic fan-out latency**, varying subscriber count N ∈ {1,5,10,50,100} — wall-clock from `Publish` to "all subscribers enqueued".
2. **Wildcard pattern-scan cost**, varying distinct patterns P ∈ {1,10,100} against a concrete publish — confirms the O(P) linear scan is acceptable; informs whether the trie deferral ever matters.
3. **Cross-instance `Publish` round-trip**, single Redis, 1/5/10 instances — compares against the existing `GroupActionMessage` path so the single topic channel adds no measurable overhead.

Report numbers in the PR description.

## Phased implementation plan (tracker)

The implementation ships as one coordinated release wave but is built and verified in dependency order. Each phase is independently testable and gated by its own subset of the §"Verification plan" items (numbered `V1`–`V18` below = list items 1–18). All boxes unchecked — this is still a design doc; the implementation PR fills them in.

Cross-reference: §6's *release order* is the cross-repo publish sequence; the phases below are the *engineering* sequence. They agree — Phases 0–5 are the `livetemplate`/`lvt`/`client` core (release steps 1–4); Phase 6 is the docs→examples→lag-bump tail (release steps 3, 5, 6).

### Phase 0 — Foundations (pure additions, no behavior wired)
- [ ] `internal/session/registry.go`: add `byTopic` + `byTopicPattern map[string][]*Connection`; `SubscribeConnectionToTopic` / `UnsubscribeConnectionFromTopic` / `GetByTopicExcept` (deduped exact∪pattern union); wire both maps into the existing `Unregister()` cleanup path (grep `^func \(r \*ConnectionRegistry\) Unregister`).
- [ ] New `topics.go`: `UserTopic`/`SessionTopic`/`GlobalTopic` constructors; shared reserved-`lvt:` namespace validator; topic grammar validator (`[a-zA-Z0-9/_-]+`, optional single trailing `*`); trailing-`*` `HasPrefix` matcher.
- [ ] Optional forward-compat `DispatchRequest.Kind` placeholder (single `KindAction`, zero-valued).
- **Gate:** registry + helper **unit** tests green (subscribe/unsubscribe, dedup union, `Unregister` cleanup, grammar/namespace/matcher edge cases). No e2e yet.

### Phase 1 — Context API + ACL (single instance, no Redis)
- [ ] `context.go`: `Subscribe`/`Unsubscribe`/`Publish`/`SelfTopic`; `topicPublishes` queue (reuse the `broadcasts []broadcastRequest` slice pattern).
- [ ] `config.go`/builder: `WithTopicACL(fn)` + `WithOpenTopics()`; both-set = hard error at `New(...)`; neither = deny-all; self/global ACL-exempt.
- [ ] `mount.go`: `dispatchToTopic` (local fan-out via `EnqueueDispatch`, reusing `dispatchBroadcastToGroup`'s shape); drain `topicPublishes` at the `processBroadcasts` post-action site (preserves persist-before-publish ordering); reuse the existing recursion guard (grep `BroadcastAction calls inside a dispatched action are ignored`); `Subscribe`-on-HTTP-GET no-op with ACL still eager.
- **Gate:** `V1`–`V7`, `V10`–`V12`, `V16` green (self-sync 2-device + anon, K-by-construction, public topic, ACL denied, self/global exempt, anti-spoof, sender-exclusion, recursion guard, HTTP GET, deny-all default). Single-instance only.

### Phase 2 — Cross-instance (Redis)
- [ ] `pubsub/types.go` + `pubsub/redis.go`: one `livetemplate:topic:{name}` channel (exact `SUBSCRIBE`); envelope reuses the `GroupActionMessage` JSON shape; `PublishToTopic`/`SubscribeToTopic` modeled on `PublishGroupAction`/`SubscribeGroupActions`.
- [ ] `mount.go`: `dispatchToTopic` cross-instance leg; `handler.Publish` out-of-band entry point (no `Context`).
- **Gate:** `V8` (out-of-band `handler.Publish` to `UserTopic`/`GlobalTopic`), `V9` (cross-instance over the single channel) green — Redis-container e2e.

### Phase 3 — Wildcards
- [ ] `pubsub/redis.go`: `PSUBSCRIBE livetemplate:topic:{prefix}*`; `subscribedPatterns map[string]int` parallel to `subscribedChannels`; replay in `reconnect()` in the same loop.
- [ ] Confirm ACL receives the literal pattern; first-ever concrete publish auto-matches existing pattern subscribers with no re-ACL.
- **Gate:** `V17` (wildcard fan-out + dedupe + first-ever + cross-instance via `PSUBSCRIBE`), `V18` (ACL receives the pattern) green.

### Phase 4 — Client error envelope (`../client`, parallelizable with 1–3)
- [ ] `client` TS: `type === "error"` branch in `handleWebSocketPayload`; surface as an `lvt:error` `CustomEvent`. No change to the diff path.
- **Gate:** `V14` (denied `Subscribe` → `lvt:error` event, WS stays open) green; client jest suite green.

### Phase 5 — Removal + in-repo/lvt migration
- [ ] Remove `ctx.BroadcastAction` (and the now-unused group-dispatch path it was the only caller of, if any) from `context.go`/`mount.go`.
- [ ] Migrate in-repo call sites to `Subscribe(SelfTopic())`+`Publish`+reconciler: `e2e/docker/app/main.go`, `broadcast_test.go`, `context_broadcast_test.go`, `lifecycle_integration_test.go`, `handle_test.go`, `navigate_test.go`. Repurpose `TestBroadcastAction_NoAutomaticPeerDispatch` (still true: nothing fans out without `Publish`).
- [ ] `lvt`: `go.mod` pin bump + `WebSocketManager.Broadcast()` → `ReloadClients()` (3 files: `internal/serve/{websocket.go,server.go,websocket_test.go}`).
- **Gate:** `V13` (lvt scaffolds compile) green; full `go test -race ./...` green in `livetemplate` **and** `lvt`; pre-commit hook green.

### Phase 6 — Docs + examples + ecosystem (the §6 release tail)
- [ ] Site docs rewrite per §6 "Docs migration scope" (top-of-funnel, reference, guides, recipes); rewrite the 3 pattern scaffolds to `Subscribe`/`Publish`/reconciler; apply the **deny-all ripple** fix (`WithOpenTopics()` or real `WithTopicACL` in every scaffold/recipe that subscribes; self-sync recipes exempt).
- [ ] In-repo contributor docs: `references/{controller-pattern,pubsub}.md`, `design/ARCHITECTURE.md`, and the stale bare-`Sync` in `guides/ephemeral-components.md`.
- [ ] `examples`: migrate 4 apps (`landing-demo`/`shared-notepad`/`todos` → self-sync; `chat` → developer topic).
- [ ] `tinkerdown` (2 examples + `sharedAuth` comment clarification) and `devbox-dash` pin bumps.
- **Gate:** `V15` green — `docs/e2e/patterns/patterns_test.go` passes and the acceptance sweep `grep -rn '\bSync\b' docs/ --exclude-dir=proposals` (both `livetemplate/docs/` and `../docs/content/`) returns no stale lifecycle references; `examples` build + e2e green.

### Cross-cutting (every phase)
- [ ] Pre-commit hook (golangci-lint + full Go suite) green before each commit; never `--no-verify`.
- [ ] Benchmarks (§"Benchmarks required") run before Phase 2 sign-off; numbers in the PR description.
- [ ] No phase merges to the release wave until its gating `V`-items pass on CI.

## Resolved decisions

Decided by the maintainer (2026-05-15):

- **Single primitive — pub/sub topics.** Collapse the two-target (implicit sync + topics) design to one topic primitive; per-identity targeting is a derived topic string. Phoenix-faithful (verified: `Phoenix.PubSub` has no per-user primitive).
- **Self-sync = action-mode + reconciler.** No render-only self-sync, no `lvt:"local"`, no state-merge. The reconciler plays the *role* of Phoenix `handle_info` (the place pushed messages reconcile state) but is an ordinary named method, not an inbox; per-connection state is safe by construction.
- **Render-only mode dropped entirely.** One method, one mode (`Publish` always invokes a handler). Eliminates the silent-no-op footgun.
- **Named-action dispatch, not a `handle_info` inbox** (resolved 2026-05-15, from prereview). `Publish`'s `action` resolves to a method by name via the existing `DispatchWithState` resolver — the same one user/button actions use. Rejected a single reserved inbox method: Go has no pattern matching (one inbox ⇒ per-controller type-switch boilerplate + weaker type safety) and reserved magic methods are what #406 removed. Consequence is **intentional symmetry** — a topic action and a same-named user action are the same handler — documented in §2 with naming guidance.
- **`ctx.BroadcastAction` removed entirely** (pre-release; clean break; all call sites migrate this wave).
- **Explicit `Subscribe(ctx.SelfTopic())` in Mount** — no framework auto-subscribe (Phoenix-pure; no hidden lifecycle).
- **Publish/Subscribe naming** — the redundant `Topic` suffix dropped (topic is the only target).
- **Default topic ACL → `deny all` + `WithOpenTopics()`.** Self/global topics ACL-exempt; reserved `lvt:` namespace + anti-spoof.
- **Wildcard / hierarchical topics in v1** (promoted from "Deferred" by explicit maintainer decision). *Why it moved:* the topics-only collapse makes topics the single primitive, and the cross-user cases (C/D/E) need `Subscribe("room/*")` to avoid issuing N concrete subscriptions per connection (one per room a user can see) — without wildcards, the room-chat pattern degrades or forces the developer back toward Authenticator abuse, the exact thing this proposal removes. Scope: single trailing `*`, `/` separator, `HasPrefix` matcher, deduped exact∪pattern fan-out, ACL receives the literal pattern, Redis `PSUBSCRIBE` cross-instance. Implemented as **Phase 3** (gated by `V17`/`V18`); if implementation pressure forces it back out of v1, Phase 3 and those `V`-items rebase as a unit and this bullet returns to "Deferred."
- **`lvt` dev-server `Broadcast()` → `ReloadClients()`** — internal-only, 3 files.

## Deferred (post-v1)

- **Mid-pattern / multi-segment wildcards** (`room/*/log`, `*/alice`). v1 is a single trailing `*` only; richer matching needs a real matcher and a multi-match precedence rule.
- **Trie/radix pattern index** for high-`P` fan-out. v1's O(P) linear scan is fine for expected pattern counts; revisit only if a deployment has thousands of distinct live patterns.
- **A `Publish` debounce/coalesce helper.** Not needed in v1 (fan-out is explicit); a convenience wrapper could be added if a real high-frequency pattern emerges.
