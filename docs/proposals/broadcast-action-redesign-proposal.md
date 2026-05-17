# Publish/Subscribe Topic Model — Design & Implementation Specification

**Status:** Accepted — implementation pending.

This specification is self-contained: it describes the target design and the implementation
plan against the current codebase. It requires no knowledge of prior design iterations. The
record of earlier alternatives and the pre-implementation audit is in the [Appendix](#appendix).

## Context

LiveTemplate exposes a single broadcast primitive — `ctx.BroadcastAction(action, data)` — that
fans out a server-side controller invocation to every WebSocket connection in the originator's
session group, sender excluded. Three problems motivate this design:

1. **Echo methods are pure ceremony.** The dominant pattern (`e2e/docker/app/main.go:Send`, `broadcast_test.go:syncController.Add`, `Increment`, `SetMessage`): mutate state → `ctx.BroadcastAction("RefreshFoo", nil)` → write a `RefreshFoo` controller method whose only job is to re-read freshly-persisted state. That method carries no per-connection-state model (peers re-render wholesale) and its fan-out reach is implicit — whatever `Authenticator.GetSessionGroup` happens to map (overlaps problem 2).
2. **No same-user, cross-device target.** Whether `BroadcastAction` reaches a user's other devices depends entirely on how `Authenticator.GetSessionGroup` maps `userID → groupID`. With `BasicAuthenticator` it works (groupID = userID); with custom per-device groupIDs it silently doesn't.
3. **No cross-user topic broadcast.** The public API exposes only group-scoped broadcasts. There is no way to fan out to a named cross-user topic — "everyone in chat room 42" — except by coercing the `Authenticator` into a contrived groupID, which conflates *state scope* with *fan-out scope* (the two concerns this design separates). This is a missing **public primitive**, not missing plumbing: the cross-instance machinery this design reuses is catalogued in "Current architecture (key references)" and "Existing utilities to reuse", and it stays unchanged — the design adds a public surface over it, it does not replace it.

**The design:** a single primitive — a named topic, with classic publish/subscribe naming,
where per-identity targeting is a topic derived from the identity. This is the `Phoenix.PubSub`
model. Per-connection state is the default; nothing fans out unless you `Publish`.

This design does **not** eliminate the per-sync method: you still write a reconciler, and it
still re-reads and assigns shared state (`s.Items = c.store.List()`). Keeping it is a deliberate
trade — see [The reconciler method](#the-reconciler-method). What changes is that the one method
now does *selective, reviewable* work and gives per-connection state a correctness model,
instead of being a no-op re-render trigger whose fan-out is implicitly scoped. **Problems 2 and
3 are resolved outright; problem 1 is reframed, not removed** — the ceremony becomes a small,
purposeful reconciliation point.

`Phoenix.PubSub` (hexdocs v2.2.0) has **no** per-user or per-connection broadcast primitive —
only `subscribe(pubsub, topic)` and `broadcast(pubsub, topic, message)`. Targeting one user is
purely a topic-string convention (`"user:123"`): subscribe each of that user's processes to
`"user:#{id}"`, broadcast to it. Phoenix LiveView has no implicit cross-tab sync — a LiveView
process is per-connection; cross-tab/device/node convergence is explicit `subscribe` in
`mount/3` plus a `handle_info/2` that updates only the relevant assigns and re-renders. This
design is faithful to that model: the reconciler method (defined below) is the `handle_info`
analog.

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
// Reload is topic-only: invoked by Publish(SelfTopic(),"Reload",…), not
// wired to any client element. (Names share one resolver — see §2 Symmetry;
// don't reuse a destructive user-action name as a topic action.)
func (c *Ctl) Reload(s S, ctx *livetemplate.Context) (S, error) {
    s.Items = c.store.List()                        // re-read ONLY shared fields
    return s, nil                                   // per-connection fields untouched → preserved
}
```

```go
// Cross-user room — same primitive, a developer-named topic.
func (c *ChatCtl) Mount(s S, ctx *livetemplate.Context) (S, error) {
    s.RoomID = ctx.Param("room")
    if err := ctx.Subscribe("room/" + s.RoomID); err != nil {
        return s, err                               // gated topic: propagate (see §3)
    }
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

One concern, one code shape. Identity targeting (`SelfTopic()`, `UserTopic`, `SessionTopic`,
`GlobalTopic`) is just a topic-name helper.

## The reconciler method

A **reconciler** is an ordinary controller method that a `Publish` invokes on each subscribed
connection so that connection can re-read shared state and re-render. It is the place pushed
updates reconcile into local state — the role Phoenix's `handle_info/2` plays. It is not a
reserved or magic method; it is a method you write and name.

**The rule that makes it correct:** a reconciler writes **only shared fields** — from the
`Publish` payload, or from a shared source (the controller singleton's mutex-guarded data, or
the group-keyed session store). Fields that are per-connection (a collapsed-panel bool, a draft
string) live in each receiver's own state and are simply never written by the reconciler, so
they survive the re-render unchanged. This is correct *by construction* because each receiver
runs the reconciler against **its own** `connState.state` (the verified behavior of
`handleDispatchedAction`, see "Current architecture"); there is no state-merge, no `lvt:"local"`
tag, no opt-out flag.

```go
// A reconciler: re-reads the shared source, writes only shared fields.
func (c *Ctl) Reload(s S, ctx *livetemplate.Context) (S, error) {
    s.Items = c.store.List()   // shared
    // s.PanelCollapsed is per-connection — untouched → preserved on every peer
    return s, nil
}
```

The reconciler is not "boilerplate to eliminate." It is the explicit, selective reconciliation
point — the thing that makes per-connection state correct and makes "what converges" reviewable
in code. The deliberate trade: one small method per sync pattern, in exchange for one mental
model and zero implicit fan-out.

### Topic identities (authenticated vs anonymous)

The `Authenticator` supplies two identity values for every request (see
`Authenticator.GetSessionGroup`, `auth.go`):

- **`UserID`** — the authenticated principal. Non-empty when the user is logged in (or
  otherwise identified by the Authenticator). Empty for genuinely anonymous traffic.
- **`GroupID`** — the session-group key. Always set; it scopes which connections share the same
  persisted `State`. `BasicAuthenticator` returns `userID`; `AnonymousAuthenticator` returns a
  cookie-bound session id.

`ctx.SelfTopic()` returns this connection's identity-derived topic by picking the
**most-specific available identity scope**:

- **Authenticated** (`UserID != ""`) → `lvt:user:<UserID>`. Spans *all* of that user's
  connections — every device and tab — regardless of how the Authenticator maps users to
  groups. This is what makes use case A (phone → desktop) work where `BroadcastAction` cannot.
- **Anonymous** (`UserID == ""`) → `lvt:session:<GroupID>`. Spans the tabs of that one
  browser session (use case B).

These identity helpers (`SelfTopic()`, `UserTopic`, `SessionTopic`, `GlobalTopic`) are pure
string constructors in the reserved `lvt:` namespace (see §2), usable from a Context or
out-of-band.

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

C/D/E/F/G/H are the capabilities missing today; A/B/I/J have working machinery
(`handleDispatchedAction`, `GetByGroupExcept`) the model reuses; K is free.

## Current architecture (key references)

Grep anchors (CLAUDE.md) so they don't drift.

- **`ctx.BroadcastAction`** queues `broadcastRequest{action,data}`. Grep `^func \(c \*Context\) BroadcastAction` in `context.go`; cap `MaxBroadcastsPerAction = 100`. **Removed by this design.**
- **`dispatchBroadcastToGroup` / `processBroadcasts`** run after the action returns. Grep `^func \(h \*liveHandler\) dispatchBroadcastToGroup` / `^func \(h \*liveHandler\) processBroadcasts` in `mount.go`. `Publish`'s local fan-out reuses this shape (registry lookup + `EnqueueDispatch`); cross-instance reuses `PublishGroupAction`'s Redis pattern.
- **`Connection.DispatchChan`** is the per-connection serial mailbox. Grep `DispatchChan chan \*DispatchRequest` in `internal/session/registry.go`; the event loop selects on it (grep `case req := <-connection.DispatchChan` in `mount.go`) and invokes `handleDispatchedAction` (grep `^func \(h \*liveHandler\) handleDispatchedAction`), which **runs the controller method against the receiving connection's own state**. This is the load-bearing fact that makes the reconciler pattern correct with no merge machinery: each receiver's state is independent, so a selective reconciler leaves per-connection fields intact.
- **Identity (`UserID` / `GroupID`)** is resolved by the `Authenticator` (`Identify` + `GetSessionGroup`) and carried on the **`*Context`** (grep `WithUserID` in `mount.go`); the `Connection` mirrors it at `Register` (grep `^func \(r \*ConnectionRegistry\) Register`), where `byUser`/`byGroup` are indexed. `SelfTopic()` reads the **Context** identity, so it resolves on both the GET-time Mount (no `Connection` exists yet — Mount runs on GET per CLAUDE.md) and the WS path, with no new index.
- **`Authenticator.GetSessionGroup`** (grep `GetSessionGroup\(r \*http.Request` in `auth.go`): `BasicAuthenticator` returns `userID`; `AnonymousAuthenticator` returns a cookie-bound ID. Unchanged.
- **PubSub** (`pubsub/types.go`, `pubsub/redis.go`). The base interface `PublishGlobal`/`PublishToGroup`/`PublishToUser` (grep `^type Broadcaster interface`); the optional **`GroupActionBroadcaster` extension** `PublishGroupAction`/`SubscribeGroupActions` (grep `^type GroupActionBroadcaster interface`), reached via the `DynamicSubscriber` type-assertion (`mount.go`, grep `pubsub.DynamicSubscriber`); Redis impls in `pubsub/redis.go` (grep `^func \(b \*RedisBroadcaster\) Publish`). `PublishGroupAction`/`SubscribeGroupActions` are the template for the new topic channel, and the new topic methods extend that **same optional-extension pattern, not the base `Broadcaster`**. `subscribedChannels` ref-count + `reconnect()` replay is the template for pattern subscriptions.

## Design

### Principle: two concerns

1. **State scope** — who shares the same persisted `State`. Decided by `Authenticator.GetSessionGroup`; **unchanged** by this design.
2. **Topic fan-out** — the single, explicit publish/subscribe mechanism. Identity targeting is a derived topic name; per-connection is the default.

These are independent. Self-sync is concern (2) applied to an identity-derived topic:
`Subscribe(SelfTopic())` + `Publish` + a reconciler. There is no implicit fan-out anywhere.

### 1. Self-sync = Subscribe(SelfTopic()) + Publish + reconciler

The developer **explicitly** subscribes to `ctx.SelfTopic()` in `Mount` (Phoenix-style; no
framework auto-subscribe — no hidden lifecycle). After a mutation,
`ctx.Publish(ctx.SelfTopic(), "Reload", …)` dispatches the `Reload` action to every connection
of that identity (sender excluded). Each receiver runs `Reload` against **its own**
`connState.state` (verified behavior of `handleDispatchedAction`). See "The reconciler method"
for the correctness argument and "Topic identities" for how `SelfTopic()` resolves.

**Invariant — both identity fields empty is a programmer error.** If a custom
`Authenticator.GetSessionGroup` returns `""` *and* the user is unauthenticated, `lvt:session:`
would be a single degenerate topic fanning out across every such connection. `SelfTopic()`
**MUST** guard this: when both `UserID` and `GroupID` are empty it logs
`slog.Error("SelfTopic called with empty UserID and GroupID")` and returns `""`;
`Subscribe("")`/`Publish("")` are rejected with an error (never a wildcard, never the reserved
root). **This must not be silently swallowable.** The canonical self-sync idiom is
`_ = ctx.Subscribe(ctx.SelfTopic())` (§3 — `SelfTopic()` is ACL-exempt so the return is
normally ignored), so the empty-identity case MUST be loud *at the `SelfTopic()`/`Subscribe("")`
site itself* via the `slog.Error` above, **independent of whether the caller inspects the
returned error**. This is **fail-closed + loud**: no partial fan-out, not quietly tolerated —
but logged, not a production `panic` (a misimplemented custom Authenticator must not crash the
server). `BasicAuthenticator`/`AnonymousAuthenticator` never hit this; it only catches a
misimplemented custom Authenticator.

**Asymmetric case (`UserID == ""` but `GroupID != ""`):** `SelfTopic()` resolves to
`lvt:session:<GroupID>` (the anonymous path) regardless of authentication intent — a custom
Authenticator that authenticates a session but leaves `UserID` empty *silently degrades* "same
user, multi-device" to "same group." This is a valid anonymous-shaped topic, not an error, so
it is not `slog.Error`/`Warn`-logged; instead the invariant is a **contract on the
Authenticator**: implementations MUST populate `UserID` for authenticated sessions if they want
cross-device (not just cross-group) self-sync. For dev discoverability, `SelfTopic()` SHOULD
emit a single `slog.Debug("SelfTopic resolved to session scope; UserID empty", "groupID", gid)`
at the resolution site so a first-time custom-Authenticator author sees *why* multi-device sync
behaves like multi-tab sync. Debug level (not Warn) — it is legitimate for genuinely anonymous
traffic and must not be noisy in production.

**Ordering.** Mutate the shared source *before* `Publish` so the reconciler reads fresh data.
For a synchronously-mutated controller singleton this is trivially true. For a reconciler that
re-reads the group-keyed session store, the originator's `persistState` write must commit
before the dispatch is published cross-instance — the existing
`processBroadcasts`-after-`persistState` ordering provides this; it is a hard requirement so
session-store implementors don't switch to async writes silently.

### 2. Topics (the one primitive)

A connection `Subscribe`s to named topics; `Publish` sends `(action, data)` to a topic; every
subscriber (local + cross-instance) runs `action` against its own state via
`handleDispatchedAction`. There is exactly one mode (always invoke a handler method) — no
render-only mode, which eliminates a silent-no-op failure class entirely.

**Subscription (Mount):**
```go
ctx.Subscribe("room/" + state.RoomID)   // or ctx.Subscribe(ctx.SelfTopic())
```

**Publish (action):**
```go
ctx.Publish("room/"+state.RoomID, "NewMessage", map[string]any{"id": id, "body": body})
```
The receiver-side controller defines a `NewMessage` method (same dispatch path as
`BroadcastAction` today — `handleDispatchedAction` → `DispatchWithState`). The calling
connection is excluded by default (use case I).

**Dispatch model: named-action, not a single inbox.** `Publish`'s `action` resolves to a
controller method *by name*, through the **exact same** `DispatchWithState` resolver that
routes user-initiated (button/form) actions — `handleDispatchedAction` at `mount.go` (grep
`^func \(h \*liveHandler\) handleDispatchedAction`) builds a `Context` with the action and
calls `DispatchWithState` unchanged. The design deliberately does **not** introduce a
Phoenix-style single inbox method (`handle_info`/`OnPublish`): Go has no pattern matching, so
one inbox forces a manual `switch` on a message-type string in every controller — more
boilerplate and weaker type safety than one well-named method per message — and a reserved
magic method is a hidden-lifecycle hazard. Named dispatch adds zero new machinery, which is
this design's core low-risk property.

**Symmetry (the consequence, by design).** Because topic dispatch and user-action dispatch
share one resolver, a topic action and a same-named user action invoke the **same method**:
`ctx.Publish(topic, "Delete", …)` runs the same `Delete` a `<button name="Delete">` triggers.
This is intentional and symmetric — server-pushed and client-initiated invocations of the same
logical action are the same handler. It is **not** a separate namespace. Practical guidance for
the docs/scaffolds: name a topic action for the *receiver's* reaction (`"NewMessage"`,
`"Reload"`, `"PresenceChanged"`), and do not reuse a destructive user-action name (`"Delete"`,
`"Submit"`) as a topic action unless you intend peers to run exactly that handler. **Why this
matters concretely:** because `Publish(topic, "Delete", …)` and a `<button name="Delete">`
resolve to the *same* `Delete` method, a careless `Publish(topic, "Delete", …)` would run
`Delete` on **every subscribed peer connection** — one server-side broadcast could delete
records for every viewer, not just the originator. The recursion guard (a dispatched action
that calls `Publish` is logged-and-dropped) bounds the blast radius to one hop, but it does not
stop the first hop — name topic actions for what the *receiver* should do, never after a
mutation the *sender* performs. A runtime guard for this hazard is a v1 requirement (see
§"Design constraints" → "Dispatch symmetry").

**How a handler reads the payload (no form/payload split).** A topic-dispatched handler reads
the `data` map from `Publish(topic, action, data)` via the **same** `ctx.Get*` accessors a
user action uses — `ctx.GetString(k)`, `ctx.GetInt(k)`, `ctx.Get(k)`, etc.
`handleDispatchedAction` builds the Context with `NewContext(ctx, action, data)`, the
*identical* shape to the WS user-action path (`mount.go`, grep
`actionCtx := NewContext(.*msg\.Action, msg\.Data)`) and the HTTP path (grep
`NewContext(.*msg\.Action, mergedData)`). There is **one** data map and **one** accessor set —
no separate `FormData()` vs `Payload()` API. For a topic-dispatched call the data is exactly
what `Publish` passed (no form/query fields are present); for a user form/button action it is
the form/query fields. A handler that genuinely must branch on origin can read `ctx.Action()`,
but the symmetric design intends handlers to be origin-agnostic — react to the `data` they
receive, regardless of who sent it. `BroadcastAction`-dispatched receivers already read their
payload exactly this way; the implementation inherits the behavior.

**Out-of-band (webhook/cron):** `handler.Publish(topic, action, data)` — same primitive, no
`Context`. Identity helpers are pure string constructors usable anywhere:
`livetemplate.UserTopic(userID)`, `livetemplate.SessionTopic(groupID)`,
`livetemplate.GlobalTopic()`.

**Reserved `lvt:` namespace + anti-spoof (security, baked in — not optional).**
Identity-derived topics live under the reserved `lvt:` prefix. `Subscribe` **rejects** any
`lvt:`-prefixed topic that is not the caller's own `SelfTopic()` or `GlobalTopic()` — a
connection cannot subscribe to another user's `lvt:user:<x>`. Developer topic names must not
start with `lvt:`. **Exact-equality, not prefix-equality (security-critical):** the match
against `SelfTopic()`/`GlobalTopic()` is *string-exact*. A wildcard or pattern form in the
reserved namespace — e.g. `lvt:user:alice*` even when `alice` is the caller — is **rejected**:
it is not the exact `SelfTopic()` string, and prefix-equality would let `lvt:user:alice*`
capture `lvt:user:alice2`/`lvt:user:aliceXYZ` and expose other users whose IDs share a prefix.
Wildcards (below) apply **only** to developer (non-`lvt:`) topics; the `lvt:` namespace admits
exact self/global strings and nothing else. The anonymous self-topic is therefore the specific
non-empty `GroupID`, never an empty key matching all anonymous connections.

**Subscription is server-driven.** `Subscribe` is called from controller code (`Mount`/
actions); the client sends no subscribe message. On WS reconnect the server re-runs `Mount`,
which re-subscribes. `Subscribe` on an HTTP GET is a no-op for the subscription itself (it only
materializes with a `Connection`) but the ACL still runs eagerly so an unauthorized subscribe
is rejected before the WS upgrade.

**Handling `Subscribe`'s `error` return in Mount (canonical pattern).**
`ctx.Subscribe(topic) error` returns `ErrTopicForbidden` under deny-all when the ACL denies —
and because the ACL runs eagerly on HTTP GET, an unconfigured ACL surfaces this on the
*initial page render*, not the WS upgrade. The recommended patterns:

```go
func (c *Ctl) Mount(s S, ctx *livetemplate.Context) (S, error) {
    // Required subscription: propagate the error — unauthorized users
    // get a failed render, which is the intended access control.
    if err := ctx.Subscribe("room/" + s.RoomID); err != nil {
        return s, err
    }
    // ACL-exempt subscription: SelfTopic()/GlobalTopic() never hit the ACL,
    // so the return is ignored. The one non-ACL failure — an empty SelfTopic()
    // from a misimplemented Authenticator (§1 invariant) — is logged loudly
    // via slog.Error at the SelfTopic() site, so this ignored return is safe.
    _ = ctx.Subscribe(ctx.SelfTopic())
    _ = ctx.Subscribe(livetemplate.GlobalTopic())
    return s, nil
}
```

Ignoring a *gated* topic's error silently drops the subscription (the connection just never
receives that topic) — that is a bug, not a no-op; always propagate gated-topic errors. This
pattern must appear in the §6 docs rewrite and the pattern scaffolds, not only here, because
deny-all makes it the first thing a new user hits.

**Wildcard / hierarchical topics.** A subscription may use `*` as a whole path segment;
multiple `*` segments are allowed:

```go
ctx.Subscribe("room/*")                       // any single room: room/42, room/99
ctx.Subscribe("org/*/room/*")                 // any room in any org
ctx.Subscribe("*/alice")                      // alice in any namespace
ctx.Publish("room/42", "NewMessage", data)    // reaches room/* subscribers + room/42 subscribers
```

- **Grammar (developer topics only).** Canonical separator `/` (HTTP-path convention). A topic is a non-empty sequence of segments; each segment is either a literal in `[a-zA-Z0-9_-]+` or the single character `*`. Concrete (publishable) topics contain no `*`; subscription patterns may use `*` for any whole segment, any number of times (`room/*`, `room/*/log`, `*/alice`, `a/*/b/*`). `*` matches **exactly one non-empty segment** — never zero, never a partial segment (`ro*m` is invalid), never across `/`. This grammar **excludes `:`**, so it does **not** describe `lvt:`-namespace topics (`lvt:user:<id>` contains `:`). The two are validated separately and order matters: `Subscribe` runs the **reserved-namespace validator first** — an `lvt:`-prefixed argument is admitted only by *exact* string equality to the caller's `SelfTopic()`/`GlobalTopic()` (every other `lvt:` string rejected); the segment grammar applies **only to non-`lvt:` developer topics**. So `Subscribe(ctx.SelfTopic())` passes the reserved-namespace check and is never measured against this grammar. The Redis transport keeps its `livetemplate:topic:` channel prefix; the slash stays literal in the channel suffix. **Minimum-viable subset (implementation risk valve):** of these shapes only a single trailing `*` (`room/*`) is *strictly required* by the documented v1 use cases (C/D/E room-chat — one wildcard segment avoids N concrete subscriptions per connection). The deeper shapes (`room/*/log`, `*/alice`, `a/*/b/*`) fall out of the same segment matcher at no extra implementation cost and are the v1 target; but if Phase 3 pressure mounts they can be deferred by tightening *only* this grammar validator to reject non-trailing or multiple `*` — no other spec change, no use-case impact. Matcher, registry, and cross-instance machinery are written for the general case regardless.
- **Matcher.** A flat segment matcher: split pattern and concrete topic on `/`; require **equal segment count**; each pattern segment matches iff it is `*` or string-equal to the concrete segment. No regex, **no trie/radix index** — a linear O(P) scan over distinct patterns (segment compare each) is the matcher, by design; it is adequate for the expected pattern counts (validated by §"Benchmarks"). Exact lookup stays O(1).
- **No precedence rule needed.** Delivery is a **deduped union by connection identity**: a publish to concrete `room/42` reaches `union(byTopic["room/42"], { conns in byTopicPattern[p] : segmentMatch(p, "room/42") })`. A connection matching the publish via several of its *own* patterns (e.g. it holds both `room/*` and `*/42`) still receives **exactly one** dispatch. Because we union connections (not pick a pattern), there is no "most-specific pattern wins" rule to specify.
- **Registry.** Add `byTopicPattern map[string][]*Connection` alongside the exact `byTopic` map. `GetByTopicExcept(concrete, excludeConn)` returns the deduped union above. Pattern scan is O(P) over distinct patterns; exact lookup stays O(1).
- **ACL receives the literal pattern, not concrete matches.** `WithTopicACL` is called once at `Subscribe("room/*")` with `topic == "room/*"` — a coarser question ("may this user subscribe to the whole `room/*` space?"). Apps needing per-room authorization subscribe to concrete topics (see §3 caution).
- **New topics auto-match existing wildcard subscribers — by design.** A first-ever `Publish("room/99", …)` reaches existing `room/*` subscribers with **no** re-invocation of the ACL. Intentional (you don't re-authorize per never-before-seen room); documented so it is not mistaken for an authorization bug.
- **Cross-instance.** Publishers always `PUBLISH` to the single *exact* channel `livetemplate:topic:{name}` (e.g. `livetemplate:topic:room/42`) — publishers never publish to patterns. An exact subscriber holds `SUBSCRIBE livetemplate:topic:room/42`. A wildcard subscriber holds `PSUBSCRIBE livetemplate:topic:<glob>` where each `*` segment is translated to a Redis `*`. **Redis `*` is broader than our segment semantics** (Redis `*` spans `/`), so a `PSUBSCRIBE` may over-deliver — therefore the receiving instance **MUST re-apply the strict segment matcher locally** before resolving the connection set. This composes with the existing "resolve and dedup locally after Redis delivery, never trust Redis" model below (the single-pump local-resolution step now also strict-filters patterns); it is not a new mechanism. Track wildcard subscriptions in a new `subscribedPatterns map[string]int` parallel to the existing `subscribedChannels` ref-count map in `pubsub/redis.go`, replayed in `reconnect()` exactly like the exact-channel set.
- **Cross-instance double-fire dedup (correctness requirement).** An instance that holds *both* an exact subscriber (`SUBSCRIBE livetemplate:topic:room/42`) and a pattern subscriber (`PSUBSCRIBE livetemplate:topic:room/*`) receives the **same** Redis message **twice** — `SUBSCRIBE` and `PSUBSCRIBE` are independent Redis deliveries that both fire for one `PUBLISH`. The receiving instance MUST resolve the concrete topic to its connection set **once per Redis message** through the deduped `GetByTopicExcept` union and enqueue at most one `DispatchRequest` per connection — never fan out per-Redis-delivery, never trust Redis to deliver once. (Per-connection `DispatchChan` serialization does not save you — two enqueues = two dispatches.) `RedisBroadcaster` already runs a *single* message pump — `go b.processMessages()` (grep `pubsub/redis.go`: `^func \(b \*RedisBroadcaster\) processMessages`) reads one multiplexed `b.pubsub.Channel()` in one `for/select` loop; go-redis delivers **both** exact and pattern messages onto that one channel, so the two deliveries for one `PUBLISH` arrive **sequentially on one goroutine** (no concurrent-goroutine race in this codebase — we use the multiplexed channel + single pump, not per-subscription callbacks). The pump MUST dedup by the envelope's message identity (`instanceID` + `timestamp`) — process the first delivery, drop a same-id repeat — *before* resolving the connection set. A tiny last-seen-id check inside the single pump is sufficient because deliveries are serialized there; no extra locking, no seen-set TTL tuning. **Precondition (must hold):** go-redis (v9.x, verified `go.mod`) multiplexes pattern *and* exact messages onto one `.Channel()` **only per `*redis.PubSub` instance**. The `PSUBSCRIBE` therefore MUST be issued on the **same `*redis.PubSub` object** whose `.Channel()` the single `processMessages` pump reads (`b.pubsub`) — *not* a second PubSub. A separate PubSub for patterns creates a second channel + second pump, the deliveries are no longer serialized, and the no-locking argument collapses. `V17`'s cross-instance leg must assert exactly-once delivery to a dual-subscribed connection *and* that the dedup is keyed on message identity (so two genuinely distinct publishes are not collapsed).

### 3. Topic ACL (auth gating)

A single global hook configured at template construction. It is called once per `Subscribe`,
with the literal subscribed name (the pattern `"room/*"`, not concrete matches):

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

> **Per-room authorization caution.** The example's `room/` arm uses a coarse
> `c.UserMayJoinRooms(userID)` precisely because the ACL receives the *pattern* `"room/*"`, not
> a concrete room id — with a wildcard subscription the hook **cannot** gate individual rooms
> (it never sees `room/42` vs `room/99`). A naive single-topic ACL would instead write
> `c.UserInRoom(userID, topic)` (per-room). **Switching a controller from
> `Subscribe("room/"+id)` to `Subscribe("room/*")` silently downgrades that per-room check to
> "may join *any* room."** If you need per-room authorization, subscribe to the **concrete**
> topic (`"room/"+id`) so the ACL receives the specific id; reserve wildcards for spaces where
> "may access the whole prefix" is the actual authorization question (public feeds, a user's
> own `user/<id>/*`). This is the single most likely ACL mistake under the wildcard feature.

**Default is `deny all`.** With neither `WithTopicACL` nor `WithOpenTopics()`, every
`Subscribe` returns `ErrTopicForbidden`. Topics are identity-agnostic and cross-user — the ACL
is their **only** boundary (a group broadcast is Authenticator-bounded; a topic is not).
Defaulting that single boundary open is a footgun, and the docs-site pattern scaffolds get
copy-pasted into user projects, so an allow-all default would teach an insecure idiom. The
opt-in is one self-documenting line:

```go
template := livetemplate.New("app", livetemplate.WithOpenTopics()) // every topic public; explicit
```

`WithOpenTopics()` and `WithTopicACL(fn)` are mutually exclusive. **The conflict is detected at
`New(...)`, not inside the `With*()` calls** — each `With*()` only records intent on the
config; `New()` validates and fails with a **returned error** (not a panic: option values can
come from runtime config, and `New()` already returns `error`). This is deliberate so the
result is order-independent: `New("app", WithOpenTopics(), WithTopicACL(fn))` and
`New("app", WithTopicACL(fn), WithOpenTopics())` both fail identically. The check must not be
added at `With*()`-time (that would make it order-dependent and surprising). Discovery path:
`Subscribe` with no config → `ErrTopicForbidden` → you learn you must configure one.

**Self/global topics are ACL-exempt — with different rationales.** `ctx.SelfTopic()` is exempt
because you are inherently authorized for your own identity's topic, and the reserved-namespace
rule already prevents subscribing to anyone else's. `livetemplate.GlobalTopic()` is exempt for
a *different* reason: it is read-only-from-the-client (subscribing only means "I will receive
global announcements" — benign) and exempting it keeps maintenance-banner recipes working under
deny-all. The ACL only gates developer-named (non-`lvt:`) topics; deny-all never breaks
self-sync. **Caution (must land in the docs):** `GlobalTopic()` is ACL-exempt **and** its
fan-out is *every connected user* — only call `Subscribe(livetemplate.GlobalTopic())` in
controllers genuinely designed to receive global announcements, and treat
`handler.Publish(livetemplate.GlobalTopic(), …)` as the highest-blast-radius call in the API.
It is not a convenient "broadcast to lots of people" shortcut. **Corollary:** because
`handler.Publish` is ACL-exempt (trusted server code), *any* server-side code path holding a
`handler` reference can fan out to **every connected user** via `GlobalTopic()` with zero
gating. Treat `handler` access as equivalent to "may broadcast to all users" and scope who can
call it accordingly (see §"Design constraints" → "Global fan-out").

**Wire-format note (client surface).** An ACL denial on the WS-connect path makes `Mount`
return an error the current TS client cannot distinguish from a generic connection failure. The
implementation extends the WS response envelope with
`{ "type": "error", "code": "topic_forbidden", "topic": "..." }`. The TS client's
`handleWebSocketPayload` (grep `livetemplate-client.ts`: `handleWebSocketPayload`) already
shape-tests for upload fields before falling back to `UpdateResponse`; the same pattern admits
a `type === "error"` branch surfaced as an `lvt:error` `CustomEvent`. This is the **only**
client-side change this design requires.

**ACL is evaluated at subscribe time, not per broadcast** — consistent with WS session-lifetime
authorization (a revoked session keeps receiving until the connection drops or the controller
calls `ctx.Unsubscribe`). For per-message authorization, check inside the receiver method.
Immediate revocation requires dropping the connection or `ctx.Unsubscribe` from a server-side
action.

**The `*http.Request` passed to the ACL is the WS upgrade request**, captured at connection
establishment. Request-scoped values from HTTP middleware (a JWT in `r.Context()`) are
available; DB-mutated permissions since the upgrade are not unless the hook re-queries the
source of truth.

**ACL on HTTP GET is a hot path.** Eager-ACL-on-GET is the **fixed framework default** —
`Subscribe` is a no-op for the subscription on GET but the ACL hook still runs (so an
unauthorized subscribe is rejected before the WS upgrade). It is **not** a framework opt-out
toggle. The hot-path mitigation lives **inside the developer's own ACL hook**: prefer stateless
checks (JWT claims); or have the hook itself early-return on `r.Method == "GET"` and defer the
real check to the WS path (the developer thereby chooses to lose "rejected before WS upgrade" —
a deliberate, app-level trade-off, not a framework setting). Recommendation: stateless by
default.

**Neither `ctx.Publish` nor `handler.Publish` runs the ACL.** Send and receive are
independent: the Subscribe-time ACL gates *who reads* a topic. `ctx.Publish` callers are
already gated by whether they can invoke the action at all (existing authorization);
`handler.Publish` is trusted server code (webhook/cron) with no caller identity to check — it
deliberately skips the ACL, exactly like the `Session.TriggerAction` it replaces for use case
F. Send-side gating, when wanted, belongs in the action handler, not the topic layer.

### 4. Migration — `BroadcastAction` is removed

Pre-release: the library has no production users outside the ecosystem repos.
`ctx.BroadcastAction` is **removed**, not deprecated — a clean break. Every call site migrates
this wave to `Publish` + a reconciler/handler. The paired echo methods are not deleted but
**repurposed** as reconcilers where peers must reflect a change. (The non-uniform echo-method
names below — `RefreshCount`, `SyncMessage`, `Refresh` — exist because the reserved `Sync()`
controller method was removed in [PR #406](https://github.com/livetemplate/livetemplate/pull/406),
which is what produced today's explicit-`BroadcastAction` boilerplate; a `Refresh*`-only grep
misses `SyncMessage`.)

**In `livetemplate/` itself:**
- `e2e/docker/app/main.go:Send` — `BroadcastAction("RefreshMessages", nil)` → `ctx.Publish(ctx.SelfTopic(), "Reload", nil)`; `RefreshMessages` → a `Reload` reconciler (re-read shared messages).
- `broadcast_test.go` — echo methods are **not** uniformly `Refresh*`-named: `Increment` → `RefreshCount`, `SetMessage` → `SyncMessage`, `syncController.Add` → `Refresh`. Migrate each to a `Publish(SelfTopic(), …)` + reconciler; grep both `Refresh*` and `Sync*`.
- `context_broadcast_test.go` (14 sites), `lifecycle_integration_test.go` (5), `handle_test.go` (4), `navigate_test.go` (5) — authoritative in-repo migration checklist (cross-ref §6 `livetemplate` row). `TestBroadcastAction_NoAutomaticPeerDispatch` asserted "no peer dispatch without an explicit call" — still true under pub/sub (nothing fans out without `Publish`); repurpose it to assert that, not delete it.

**In `examples/` (verified `grep -rn "BroadcastAction\b" examples/`):**
- `landing-demo/main.go` (3: Increment/Decrement/Reset), `shared-notepad/main.go` (1: Save), `todos/controller.go` (4: Add/Toggle/Delete/Update) — each becomes `Subscribe(ctx.SelfTopic())` in Mount + `Publish(ctx.SelfTopic(), "Reload", nil)` + a `Reload` reconciler (use case A/B).
- `chat/main.go` (3: UserJoined/NewMessage/UserLeft) — `Subscribe("chat/"+room)` + `Publish("chat/"+room, "NewMessage", data)` + handlers (use case C).

**In `tinkerdown/`:** `examples/literate-counter-include/_app/counter.go` and
`examples/literate-linked-include/_app/counter.go` (3 each). Migrate to
`Subscribe(ctx.SelfTopic())`+`Publish`; the tutorial comments must clarify `sharedAuth`
(constant groupID) is an artificial teaching setup.

**Empty echo methods** that carried no reconciliation logic (pure re-render triggers) become
dead code; ones that re-read shared data become the reconciler. Grep both `Refresh*` and
`Sync*` — naming is not uniform.

### 5. Final API surface

**On `*Context`:**
- `ctx.Subscribe(topic string) error` — wildcard segment `*` allowed; ACL deny-all default except self/global; no-op subscription on HTTP GET (ACL still runs).
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

**Removed:** `ctx.BroadcastAction`; `ctx.SkipPeerSync`; `WithImplicitSyncDisabled`;
`handler.BroadcastToUser`; `handler.BroadcastGlobal` (now `Publish` to `UserTopic`/
`GlobalTopic`); the `lvt:"local"` tag and its merge machinery; render-only dispatch mode;
`GetByUserExcept`; the `RenderInvalidation` message type and the
`livetemplate:render:{groupID}` channel; the userID-vs-groupID fan-out scope rule; the
render-fan-out coalescing-bounds design.

**Unchanged:** `Session` interface and `Session.TriggerAction` (still valid for goroutines
holding a captured Session; orthogonal to topics — it dispatches to one session, not a topic);
`Authenticator`; all base PubSub interfaces.

## Impacted repositories

| Repo (`../<name>/`) | pin | `BroadcastAction` sites | Migration |
|---|---|---|---|
| `livetemplate` (this repo) | — | `e2e/docker/app/main.go`, `broadcast_test.go`, `context_broadcast_test.go`, `lifecycle_integration_test.go`, `handle_test.go`, `navigate_test.go` | Core implementation; rewrite e2e Docker app; migrate/repurpose the test files (see §4). |
| `lvt` | `v0.8.23-0.2026...` | **0** | `go.mod` pin bump **plus an internal rename**: `WebSocketManager.Broadcast()` → `ReloadClients()` in `internal/serve/` — 3 files: `websocket.go` (def), `server.go` (the single production caller), `websocket_test.go` (rename the test func `TestWebSocketManager_Broadcast` → `TestWebSocketManager_ReloadClients`, **and** update the `.Broadcast()` callers in **both** that func *and* `TestWebSocketManager_MultipleClients` — the 2 call sites span two test functions, only one of which is renamed). Internal-only, zero user-facing impact; the name is accurate (it only emits `{"type":"reload"}` on file change) and removes the cross-repo `Broadcast` collision. Scaffolds emit CRUD only — no migration. |
| `client` (TS) | `0.9.0` | N/A | One change: a `type === "error"` branch in `handleWebSocketPayload` for `topic_forbidden` (§3 wire-format note). Otherwise the stateless diff path is unaffected. |
| `examples` | `v0.9.0` | 11 sites / 4 apps (§4) | `landing-demo`/`shared-notepad`/`todos` → `Subscribe(SelfTopic())`+`Publish`+reconciler; `chat` → developer topic + handlers. |
| `tinkerdown` | `v0.8.16` (stale) | 6 sites / 2 examples | Bump to `v0.9.x`; migrate to `Subscribe(SelfTopic())`+`Publish`; clarify `sharedAuth` is teaching-only. |
| `devbox-dash` | (single-user) | 0 | Routine version bump. |
| `docs` (site) | `v0.8.23` | 22 content files; `handlers_realtime.go` Patterns #27/#28 use it; Pattern #26 uses `Sync()` | **Heaviest impact.** See "Docs migration scope". |

**Docs migration scope** — two surfaces:

- **In-repo contributor docs** (`livetemplate/docs/`): `references/controller-pattern.md`, `references/pubsub.md`, `design/ARCHITECTURE.md`, `guides/ephemeral-components.md` (a bare `Sync` reference in its state-init lifecycle list).
- **Site docs** (`../docs/content/`, verified `grep -rln "BroadcastAction" docs/content/`):
  - **Top-of-funnel** — `index.md`, `getting-started/your-first-app.md`.
  - **Reference** — `reference/api.md`, `reference/controller-pattern.md`, `reference/server-actions.md`, `reference/session.md`, `reference/pubsub.md`, `reference/navigate.md`, `reference/limitations.md`.
  - **Guides** — `guides/standard-html-reactivity.md`, `guides/progressive-complexity.md`.
  - **Recipes** — `recipes/broadcasting.md` (rewrite end-to-end as pub/sub), `recipes/sync-and-broadcast.md` (rewrite/retire), `recipes/counter/index.md`, `recipes/architecture-flow.md`, `recipes/progressive-enhancement/index.md`, `recipes/todos/index.md`.
  - **Pattern scaffolds** (critical path — copy-pasted into user projects) — `recipes/patterns/_app/templates/realtime/{broadcasting,multi-user-sync,server-push}.tmpl` + `recipes/patterns/_app/handlers_realtime.go` (Patterns #26–#28). Rewrite to `Subscribe`/`Publish`/reconciler.
  - **Deny-all ripple (must-fix):** any scaffold/recipe calling `Subscribe` **fails out of the box** under deny-all (`ErrTopicForbidden`). The docs-PR author must add `livetemplate.WithOpenTopics()` (with a one-line "topics are deny-by-default; this is the public-demo opt-in" comment — itself pedagogical) **or** a real `WithTopicACL` example. Self-sync recipes are exempt (`SelfTopic()` bypasses the ACL). **Troubleshooting note (in-repo docs):** because `Subscribe` runs the ACL eagerly even on HTTP GET, an unconfigured ACL surfaces `ErrTopicForbidden` on a *plain page load*, not on the WS upgrade. The in-repo "Troubleshooting" section must call this out explicitly ("`ErrTopicForbidden` on a GET → you called `Subscribe` without `WithTopicACL`/`WithOpenTopics`").
  - **Historical** — `changelog.md` gets an entry, not a rewrite.

**Release order (single coordinated wave):** 1. `livetemplate` core + tests → 2. `client`
error-envelope (parallel) → 3. **`docs`** recipes/reference/scaffolds (gating — scaffolds are
copy-pasted) → 4. `lvt` pin bump + `ReloadClients()` rename → 5. `examples` migration →
6. `tinkerdown`/`devbox-dash` lag bumps.

## Critical files to modify

- `context.go` — add `Subscribe`, `Unsubscribe`, `Publish`, `SelfTopic`; reuse the existing `broadcasts []broadcastRequest` slice pattern for a `topicPublishes` queue (drained after the action like `processBroadcasts`). `Subscribe`'s validation order (must be exactly this): **(1)** reserved-namespace validator — if the argument is `lvt:`-prefixed, accept only on *exact* equality to the caller's `SelfTopic()`/`GlobalTopic()`, reject every other `lvt:` string (anti-spoof); **(2)** for non-`lvt:` (developer) topics only, the segment grammar (segments of `[a-zA-Z0-9_-]+` or `*`, `/`-separated); **(3)** the ACL (deny-by-default; self/global exempt) before recording. The developer grammar is **never** applied to `lvt:` topics (it excludes `:` — see §2 "Grammar").
- package file (e.g. `topics.go`) — `UserTopic`/`SessionTopic`/`GlobalTopic` constructors + a shared reserved-namespace validator used by both `Subscribe` and the constructors; the segment-grammar validator; the segment matcher (`segmentMatch(pattern, concrete) bool` — split on `/`, equal count, per-segment `*`-or-equal).
- `config.go` / builder — `WithTopicACL(fn)` and `WithOpenTopics()`; both-set is a hard error raised **at `New(...)`** (order-independent), not at `With*()`-call time; neither set = deny-all.
- `mount.go` — add `dispatchToTopic` next to `dispatchBroadcastToGroup` (reuse the registry-lookup + `EnqueueDispatch` shape); wire the post-action drain of `topicPublishes`; reuse the existing recursion guard (grep `BroadcastAction calls inside a dispatched action are ignored`) so a `Publish` inside a dispatched action is logged-and-dropped. **No** `dispatchPeerSyncToUser`, **no** render-only handler, **no** `RenderInvalidation`.
- `internal/parse/` (+ a read-only accessor on `*Template`/handler) — **new (small):** during parse, collect the set of action names **wired to a client element**, defined precisely as: a `name=` attribute on a `<form>`/`<button>`/submit `<input>`; an `lvt-on:<event>="Action"` handler; and any other `lvt-*` attribute whose value the parser routes to `DispatchWithState`. The implementation MUST pin this to the exact `internal/parse/` node types it walks (so the `V19` test asserts a fixed set, not a moving target); at minimum it covers form/button `name=` and `lvt-on:`, and the implementation PR enumerates the full `lvt-*` action-bearing set there. Expose the set read-only for the `Publish`-time symmetry-collision lookup (§"Design constraints" → "Dispatch symmetry"). No public API; an internal accessor only. The parser does not track this today, so this is a small new parse-layer pass — it is the one place the symmetry guard's cost lands.
- `internal/session/registry.go` —
  - (a) `byTopic map[string][]*Connection` (exact) **and** `byTopicPattern map[string][]*Connection` (segment patterns).
  - (b) `SubscribeConnectionToTopic` / `UnsubscribeConnectionFromTopic` / `GetByTopicExcept`. `GetByTopicExcept(concrete, excludeConn)` returns `union(byTopic[concrete], { conns in byTopicPattern[p] : segmentMatch(p, concrete) })` **deduplicated by `*Connection` identity**; pattern match = the segment matcher, O(P).
  - (c) Wire **both** topic maps into the existing `Unregister()` cleanup path (grep `^func \(r \*ConnectionRegistry\) Unregister`), where `byUser`/`byGroup` cleanup already happens. **Not** `Connection.Close()` (it delegates to `Unregister()`).
  - (d) Optionally add a `Kind` field to `DispatchRequest` as a forward-compatible placeholder (single value `KindAction` in v1; zero-valued, backward-compatible). **Drop `GetByUserExcept` entirely** (not needed).
- `pubsub/types.go`, `pubsub/redis.go` — one new channel scheme `livetemplate:topic:{name}` for `Publish` (exact `SUBSCRIBE`) and `PSUBSCRIBE livetemplate:topic:<glob>` (each pattern `*` segment → Redis `*`) for wildcard subscriptions, with mandatory local strict re-match on receipt (§2 "Cross-instance"); add `subscribedPatterns map[string]int` parallel to `subscribedChannels`, replayed in `reconnect()`. Extend the `GroupActionMessage` envelope with a new `Topic string` json:"topic" field (the receiver resolves the connection set by topic; do **not** repurpose the existing `GroupID` field — a misnamed wire field misleads every reader). The `instanceID`+`timestamp` dedup key is unaffected. **No** `livetemplate:render:{groupID}` channel.
- `auth.go` — no change.
- `e2e/docker/app/main.go` — migration target (§4).
- In-repo `docs/references/{controller-pattern,pubsub}.md`, `docs/design/ARCHITECTURE.md` — document the two-concern pub/sub model. Site docs scoped in §6.

## Existing utilities to reuse

- `handleDispatchedAction` (grep `^func \(h \*liveHandler\) handleDispatchedAction` in `mount.go`) — runs the action against the receiver's own state; `Publish` reuses it unchanged. This is what makes the reconciler/per-connection-state guarantee free.
- `Connection.EnqueueDispatch` (grep `^func \(c \*Connection\) EnqueueDispatch`) — non-blocking, drop-on-overflow mailbox; `Publish`'s local fan-out enqueues one per subscriber.
- `dispatchBroadcastToGroup` (grep in `mount.go`) — structural template for `dispatchToTopic` (registry lookup → `EnqueueDispatch` loop → cross-instance publish).
- `pubsub.RedisBroadcaster.PublishGroupAction` / `SubscribeGroupActions` (grep `pubsub/redis.go`) — template for the topic channel; `subscribedChannels` + `reconnect()` replay — template for `subscribedPatterns`.
- The existing recursion guard (grep `BroadcastAction calls inside a dispatched action are ignored`) — reused verbatim for `Publish`.
- `processBroadcasts` post-action hook site (grep `^func \(h \*liveHandler\) processBroadcasts`) — the drain point for the new `topicPublishes` queue, preserving persist-before-publish ordering.

## Verification plan

### Test tiers & harness (how each `Vn` is implemented)

The `V`-items below are realized across three tiers; every item is assigned a tier so the
implementation PR knows where the test lives and how it runs.

- **Tier 1 — Go integration tests** (`livetemplate` repo, new `topic_test.go`, mirroring `broadcast_test.go`'s structure: an `httptest.Server` driving the real handler with in-process WebSocket clients). Fakes: a custom `Authenticator` returning per-device groupIDs (V1/V3), `AnonymousAuthenticator` (V2). Single-instance items use the in-memory broadcaster; cross-instance items (V9, the cross-instance leg of V17) stand up a **real Redis via testcontainers** with two `liveHandler` instances sharing one `RedisBroadcaster`. Covers the logic-level items **V1–V12, V16–V20**. Runner: `go test -v -race ./...` (Redis items gated by a `redis` build tag / testcontainers availability).
- **Tier 2 — chromedp browser e2e** (`lvt` repo `e2e/`, alongside `livetemplate_core_test.go`, per CLAUDE.md "Browser-based E2E Tests"). Required for the user-visible behavior — that a peer tab actually re-renders, that a per-connection DOM field survives a peer's reconcile, and the client error envelope. Two browser contexts (= two tabs/devices) against one or two real server instances (+ Redis for the cross-instance leg). **The harness MUST capture and surface on failure all four** (standing project rule, non-negotiable): (1) browser console logs (chromedp console events), (2) server logs (the test server's `slog` handler tee'd to a buffer), (3) WebSocket frames sent/received (CDP `Network.webSocketFrame*`), (4) rendered HTML (`chromedp.OuterHTML` of the wrapper). Covers the user-visible legs of **V1–V3** (peer re-render + per-connection field visible in the DOM), **V14** (`lvt:error` `CustomEvent`, WS stays open), **V17** (wildcard reconcile visible cross-tab).
- **Tier 3 — cross-repo CI steps** (scripted grep/build, not Go tests): **V13** (`lvt new` → `go build ./...`), **V15** (the `Sync` acceptance grep + `docs/e2e/patterns/patterns_test.go`), **V20** (the removed-API `grep -rn` sweep across all impacted repos). Run in the release-wave CI per the phase gates.

A `Vn` may have both a Tier-1 assertion (logic) and a Tier-2 assertion (user-visible) — V1 is
proven at the dispatch level in Tier 1 *and* at the DOM level in Tier 2. The Phased plan's
per-phase **Gate** lists which `Vn` must be green before that phase merges; the tier above maps
each to its runnable test.

### V-items

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
11. **Recursion guard.** A handler invoked via `Publish` calls `Publish` again → logged-and-dropped.
12. **`Subscribe` on HTTP GET.** Plain GET to a Mount calling `Subscribe`: no error, normal render, no `byTopic` entry; ACL still ran. Upgrade to WS for the same identity → subscription materializes.
13. **`lvt` scaffolds compile.** `lvt new` → `go build ./...` succeeds against the redesigned `livetemplate` (sanity for the pin bump; scaffolds emit no broadcasts).
14. **Client error envelope.** `WithTopicACL` denies `"private/admin"`; TS client `Subscribe("private/admin")` → `lvt:error` `CustomEvent` `{ code:"topic_forbidden", topic:"private/admin" }`; WS stays open.
15. **Docs-e2e `Sync` rewrite.** `docs/e2e/patterns/patterns_test.go` exercises Patterns #26–#31; #26 uses `Sync()`, which passes against the docs repo's pinned `v0.8.23` and breaks on the pin bump. Identify the cases the docs rewrite must update. **Acceptance:** the bare-`Sync` lifecycle reference is gone from both `livetemplate/docs/` and `../docs/content/`. Acceptance check: `grep -rnE '\bSync\b' docs/ --include='*.md' --exclude-dir=proposals` returns **no** controller-lifecycle `Sync` reference. The match is **case-sensitive** (`-E`, no `-i`), so Go-stdlib lowercase `sync.Mutex`/`sync.Once`/`"sync"` in code fences never match and need no filter. (An earlier `| grep -vE 'sync\.[A-Za-z]|"sync"|\bsync\b'` post-filter is removed: the `\bsync\b` clause was dead — lowercase `sync` cannot occur on a line selected by case-sensitive `\bSync\b` — and `sync\.[A-Za-z]` was unsafe, since a real stale-`Sync` line that also mentioned `sync.Something` would be wrongly masked.) Any residual hit is a literal capital-`Sync` token to eyeball-confirm is not the removed controller hook. `--exclude-dir=proposals` still required (this spec/appendix records `Sync()` as historical context).
16. **Deny-all default.** No ACL config: `Subscribe("anything")` → `ErrTopicForbidden`. `WithOpenTopics()` → succeeds. `WithTopicACL` → hook decides. Both set → hard error at `New(...)`.
17. **Wildcard fan-out + dedupe (multi-segment).** `Subscribe("room/*")`, `Subscribe("org/*/room/*")`, `Subscribe("*/alice")`; assert each receives the matching concrete publish and rejects non-matching segment counts (`room/*` does **not** match `room/42/log`). A connection subscribed to **both** `room/42` and `room/*` (and to two of its own matching patterns) gets **exactly one** frame. First-ever `Publish("room/99", …)` reaches the `room/*` subscriber with the ACL **not** re-invoked. Cross-instance: a `room/*/log` subscriber on instance B receives instance A's `room/42/log` via `PSUBSCRIBE`, and the over-broad Redis delivery for `room/42/other` is rejected by the local strict matcher.
18. **ACL receives the literal pattern.** On `Subscribe("org/*/room/*")` the hook's `topic` argument is the literal `"org/*/room/*"`.
19. **Symmetry collision warning.** A controller with a `Delete` method wired to `<button name="Delete">`: `ctx.Publish(topic, "Delete", …)` emits the `slog.Warn` collision log; `ctx.Publish(topic, "Reload", …)` where `Reload` is topic-only (no client element) emits **no** warning. Confirms the new parse-layer wired-name extraction feeds the `Publish`-time lookup with no false positive.
20. **Removed-API sweep.** Across all impacted repos (`livetemplate`, `lvt`, `client`, `examples`, `tinkerdown`, `devbox-dash`, `docs`), `grep -rn` finds **zero** references — in code *or* documentation — to any removed symbol: `BroadcastAction`, `SkipPeerSync`, `WithImplicitSyncDisabled`, `BroadcastToUser`, `BroadcastGlobal`, `GetByUserExcept`, `RenderInvalidation`, `livetemplate:render:`, `lvt:"local"`. Excluded: this spec's Appendix and `changelog.md` history entries (which legitimately record the removal). This is the migration-completeness gate.

**Runners.** Tier 1: `go test -v -race ./...` in `livetemplate` (Redis items via
testcontainers). Tier 2: the chromedp suite in `lvt` `e2e/` (console + server-log + WS-frame +
rendered-HTML capture wired in, surfaced on failure). Tier 3: the CI grep/build steps wired
into the phase gates (§"Phased implementation plan"). No phase merges until its gating `Vn`
pass on CI (§"Cross-cutting").

## Design constraints (must be satisfied by v1)

- **Topic GC on disconnect.** Topics with no subscribers must not leak in `byTopic`/`byTopicPattern`. Cleanup wires into the existing `Unregister()` index-cleanup path (where `byUser`/`byGroup` already clean up) — not a separate goroutine.
- **Cross-instance ordering.** When a reconciler re-reads the group-keyed session store, the originator's `persistState` write must commit before the dispatch is published cross-instance — the existing `processBroadcasts`-after-`persistState` ordering provides this; it is a hard requirement.
- **No implicit fan-out, no debounce helper.** `Publish` is always an explicit developer call; there is no high-frequency implicit render path. If a `Publish`-per-keystroke pattern emerges, debouncing is the **developer's** responsibility at the call site (same as any action) — the framework provides no `Publish` debounce/coalesce helper, by design.
- **Dispatch symmetry — naming hazard (severe).** Because topic dispatch and user-action dispatch share one resolver (§2), `Publish(topic, "Delete", …)` invokes the *same* `Delete` method a `<button name="Delete">` triggers — on **every subscribed peer**. A topic action accidentally named after a destructive user action means one server-side `Publish` mutates/deletes records for every connected viewer. v1 mitigations: name topic actions for the *receiver's* reaction (`Reload`, `NewMessage`, `PresenceChanged`), never after a sender-side mutation; the one-hop recursion guard bounds cascades but not the first hop. **Runtime countermeasure (v1 hard requirement — MUST, not advisory):** because naming conventions are violated eventually — especially in copy-pasted scaffolds — `Publish` **MUST** emit `slog.Warn("Publish action name collides with a client-wired action", "action", a, "topic", t)` when the `action` string matches a method the template parser also wired to a client element (precise definition — form/button `name=`, `lvt-on:`, other `lvt-*` action attributes — pinned to exact parse node types in §"Critical files"). A check against *any* controller method would be all-noise (every valid topic action — `Reload`, `NewMessage` — *is* a controller method); the warning is meaningful only against the set of names **wired to a client element**, which the parser does not track today, so it requires the small new parse-layer pass scoped in §"Critical files" (`internal/parse/`) and Phase 1, gated by `V19`. No public API change. Warn, not error — no false positives on intentional symmetric use; it stays a signal, not a block. This converts the highest-blast-radius footgun from "docs-only discipline" to "loud at runtime the first time it happens in dev"; the docs rewrite must surface it as a named warning, not a buried paragraph.
- **Cross-instance exactly-once.** A dual-subscribed connection (exact `room/42` + pattern `room/*`) must receive exactly one dispatch per `Publish` even though Redis fires the `SUBSCRIBE` and `PSUBSCRIBE` deliveries separately. The mechanism: dedup by the envelope's `instanceID`+`timestamp` message identity inside the existing single `processMessages` pump (deliveries are serialized there — no goroutine coordination needed in this codebase), *before* registry resolution/enqueue — never by trusting Redis or `DispatchChan` to dedupe. The `PSUBSCRIBE` MUST be on the same `*redis.PubSub` instance the pump reads (§2 "Cross-instance"). Gated by `V17`.
- **Global fan-out is ungated server-side (severe).** `livetemplate.GlobalTopic()` is ACL-exempt for `Subscribe` (benign — opt-in to announcements) *and* `handler.Publish` is ACL-exempt (trusted server code). The combination: **any** code path holding a `handler` reference can fan out to **every connected user** via `handler.Publish(livetemplate.GlobalTopic(), …)` with zero topic-layer gating. Concrete failure mode: an admin-only webhook whose `handler` leaks into a non-admin code path can trigger an app-wide broadcast. Treat `handler` access as equivalent to a "broadcast to all users" capability and scope who can reach it accordingly — like a "send to all" admin capability.

## Benchmarks required before implementation finalizes

1. **Topic fan-out latency**, varying subscriber count N ∈ {1,5,10,50,100} — wall-clock from `Publish` to "all subscribers enqueued".
2. **Wildcard pattern-scan cost**, varying distinct patterns P ∈ {1,10,100} (mixed segment counts) against a concrete publish — confirms the flat O(P) segment scan is acceptable at the expected pattern counts. (There is no trie/radix index by design; this benchmark validates that the linear scan is adequate, not whether to add one.)
3. **Cross-instance `Publish` round-trip**, single Redis, 1/5/10 instances — compares against the existing `GroupActionMessage` path so the topic channel adds no measurable overhead.

Report numbers in the PR description.

## Phased implementation plan (tracker)

The implementation ships as one coordinated release wave but is built and verified in
dependency order. Each phase is independently testable and gated by its own subset of the
§"Verification plan" items (`V1`–`V20` = list items 1–20). All boxes unchecked — the
implementation PR fills them in.

Cross-reference: §6's *release order* is the cross-repo publish sequence; the phases below are
the *engineering* sequence. They agree — Phases 0–5 are the `livetemplate`/`lvt`/`client` core
(release steps 1–4); Phase 6 is the docs→examples→lag-bump tail (release steps 3, 5, 6).

### Phase 0 — Foundations (pure additions, no behavior wired)
- [ ] `internal/session/registry.go`: add `byTopic` + `byTopicPattern map[string][]*Connection`; `SubscribeConnectionToTopic` / `UnsubscribeConnectionFromTopic` / `GetByTopicExcept` (deduped exact∪pattern union); wire both maps into the existing `Unregister()` cleanup path.
- [ ] New `topics.go`: `UserTopic`/`SessionTopic`/`GlobalTopic` constructors; shared reserved-`lvt:` namespace validator; segment-grammar validator (segments of `[a-zA-Z0-9_-]+` or `*`, `/`-separated; multi-`*` allowed); `segmentMatch(pattern, concrete)` matcher (split on `/`, equal count, per-segment `*`-or-equal — no regex, no trie).
- [ ] Optional forward-compat `DispatchRequest.Kind` placeholder (single `KindAction`, zero-valued).
- **Gate:** registry + helper **unit** tests green (subscribe/unsubscribe, dedup union, `Unregister` cleanup, grammar/namespace edge cases, segment matcher incl. multi-segment, segment-count mismatch, `*/x` and `a/*/b/*`). No e2e yet.

### Phase 1 — Context API + ACL (single instance, no Redis)
- [ ] `context.go`: `Subscribe`/`Unsubscribe`/`Publish`/`SelfTopic`; `topicPublishes` queue (reuse the `broadcasts []broadcastRequest` slice pattern).
- [ ] `config.go`/builder: `WithTopicACL(fn)` + `WithOpenTopics()`; both-set = hard error at `New(...)`; neither = deny-all; self/global ACL-exempt.
- [ ] `mount.go`: `dispatchToTopic` (local fan-out via `EnqueueDispatch`, reusing `dispatchBroadcastToGroup`'s shape); drain `topicPublishes` at the `processBroadcasts` post-action site (preserves persist-before-publish ordering); reuse the existing recursion guard; `Subscribe`-on-HTTP-GET no-op with ACL still eager.
- [ ] `internal/parse/` + accessor: collect client-wired action names; `Publish` emits the `slog.Warn` symmetry-collision log on a wired-name match (§"Design constraints"). Required in Phase 1 because the guard is part of `Publish`.
- **Gate:** `V1`–`V7`, `V10`–`V12`, `V16`, `V19` green. Single-instance only.

### Phase 2 — Cross-instance (Redis)
- [ ] `pubsub/types.go` + `pubsub/redis.go`: one `livetemplate:topic:{name}` channel (exact `SUBSCRIBE`); `GroupActionMessage` extended with the `Topic string` field per §"Critical files"; new methods extend the optional `GroupActionBroadcaster`/`DynamicSubscriber` pattern, **not** base `Broadcaster`; `PublishToTopic`/`SubscribeToTopic` modeled on `PublishGroupAction`/`SubscribeGroupActions`.
- [ ] `mount.go`: `dispatchToTopic` cross-instance leg; `handler.Publish` out-of-band entry point (no `Context`).
- **Gate:** `V8` (out-of-band `handler.Publish`), `V9` (cross-instance over the single channel) green — Redis-container e2e.

### Phase 3 — Wildcards (multi-segment)
- [ ] `pubsub/redis.go`: `PSUBSCRIBE livetemplate:topic:<glob>` (each pattern `*` segment → Redis `*`); `subscribedPatterns map[string]int` parallel to `subscribedChannels`; replay in `reconnect()` in the same loop.
- [ ] **Local strict re-match (must hold):** because Redis `*` spans `/` (broader than our whole-segment semantics), the receiving instance MUST re-apply `segmentMatch` to the concrete topic before resolving the connection set — fold into the existing single-pump local-resolution step. A `PSUBSCRIBE` over-delivery that fails `segmentMatch` is dropped.
- [ ] **Single-PubSub-instance invariant (must hold):** issue the `PSUBSCRIBE` on the **same `*redis.PubSub` object** the single `processMessages` pump reads (`b.pubsub`) — not a new PubSub. go-redis multiplexes pattern+exact onto one `.Channel()` only per instance; a separate PubSub breaks the serialized, no-locking dedup (§2 "Cross-instance double-fire dedup"). `V17`'s cross-instance leg asserts exactly-once delivery, which only holds if this invariant does.
- [ ] **Relay invariant (must hold):** a wildcard subscription is relayed with **`PSUBSCRIBE`** — the relay MUST NOT expand a pattern into per-concrete `SUBSCRIBE`s. Publishers always `PUBLISH` to the *exact* channel; cross-instance wildcard delivery works *only* because Redis pattern-matching connects the exact `PUBLISH` to the instance's `PSUBSCRIBE`. If the relay expanded `room/*` to `SUBSCRIBE livetemplate:topic:room/42`, a later `handler.Publish("room/43", …)` from another instance would silently miss that subscriber.
- [ ] Confirm ACL receives the literal pattern; first-ever concrete publish auto-matches existing pattern subscribers with no re-ACL.
- **Gate:** `V17` (multi-segment fan-out + dedupe + first-ever + cross-instance via `PSUBSCRIBE` + over-delivery rejection), `V18` (ACL receives the literal pattern) green.

### Phase 4 — Client error envelope (`../client`, parallelizable with 1–3)
- [ ] `client` TS: `type === "error"` branch in `handleWebSocketPayload`; surface as an `lvt:error` `CustomEvent`. No change to the diff path.
- **Gate:** `V14` (denied `Subscribe` → `lvt:error` event, WS stays open) green; client jest suite green.

### Phase 5 — Removal + in-repo/lvt migration
- **`BroadcastAction` status through Phases 0–4:** the new APIs (`Subscribe`/`Publish`/`SelfTopic`/ACL) are purely **additive** — `ctx.BroadcastAction` and its group-dispatch path remain **fully functional and untouched** until this phase. Phases 0–4 add code; nothing is removed or deprecated-with-warning before Phase 5. Consequence: the in-repo `BroadcastAction` tests compile and pass unchanged through Phases 0–4; they are migrated *in this phase*, atomically with the removal, so there is never an intermediate build where the API is half-removed.
- [ ] Remove `ctx.BroadcastAction` (and the now-unused group-dispatch path it was the only caller of, if any) from `context.go`/`mount.go`.
- [ ] Migrate in-repo call sites to `Subscribe(SelfTopic())`+`Publish`+reconciler: `e2e/docker/app/main.go`, `broadcast_test.go`, `context_broadcast_test.go`, `lifecycle_integration_test.go`, `handle_test.go`, `navigate_test.go`. Repurpose `TestBroadcastAction_NoAutomaticPeerDispatch`.
- [ ] `lvt`: `go.mod` pin bump + `WebSocketManager.Broadcast()` → `ReloadClients()` (3 files: `internal/serve/{websocket.go,server.go,websocket_test.go}` — both test funcs, see §6 `lvt` row).
- **Gate:** `V13` (lvt scaffolds compile) green; full `go test -race ./...` green in `livetemplate` **and** `lvt`; pre-commit hook green.

### Phase 6 — Docs + examples + ecosystem (the §6 release tail)
- [ ] Site docs rewrite per §6 "Docs migration scope" (top-of-funnel, reference, guides, recipes); rewrite the 3 pattern scaffolds to `Subscribe`/`Publish`/reconciler; apply the **deny-all ripple** fix (`WithOpenTopics()` or real `WithTopicACL` in every scaffold/recipe that subscribes; self-sync recipes exempt).
- [ ] In-repo contributor docs: `references/{controller-pattern,pubsub}.md`, `design/ARCHITECTURE.md`, and the stale bare-`Sync` in `guides/ephemeral-components.md`.
- [ ] `examples`: migrate 4 apps (`landing-demo`/`shared-notepad`/`todos` → self-sync; `chat` → developer topic).
- [ ] `tinkerdown` (2 examples + `sharedAuth` comment clarification) and `devbox-dash` pin bumps.
- **Gate:** `V15` green — `docs/e2e/patterns/patterns_test.go` passes and the `Sync` acceptance sweep returns no stale lifecycle references; `V20` green — removed-API sweep returns zero hits across all repos; `examples` build + e2e green.

### Cross-cutting (every phase)
- [ ] Pre-commit hook (golangci-lint + full Go suite) green before each commit; never `--no-verify`.
- [ ] Benchmarks (§"Benchmarks required") run before Phase 2 sign-off; numbers in the PR description.
- [ ] No phase merges to the release wave until its gating `V`-items pass on CI.

---

## Appendix

The body above is the complete, self-contained specification. This appendix is a non-normative
record of how the design was reached and the pre-implementation audit. Nothing here is required
to implement the spec.

### A. Revision history

- **Two-target → single-primitive (2026-05-15).** An earlier revision proposed *two* fan-out concerns: implicit user/group peer-sync *and* explicit topics. Maintainer review judged two broadcast targets to be excess API surface and a footgun source. Collapsed to the single topic primitive in the body (per-identity targeting is a derived topic string).
- **Post-merge audit (2026-05-15).** A code-grounded audit verified the load-bearing claims and fixed the defects it surfaced before implementation. See §C.
- **Prereview restructure.** Reorganized into a self-contained spec + this appendix; added the dedicated "reconciler method" section; resolved all previously-deferred items into the design; promoted multi-segment wildcards into v1.

### B. Alternatives considered (and why rejected)

- **Two broadcast targets (implicit peer-sync + explicit topics).** Rejected: two targets is excess API surface and a footgun source — an empty-userID fan-out hazard, a userID-vs-groupID scope rule, a state-merge tag, and a render-only mode with a silent-no-op failure. The single topic primitive subsumes self-sync as `Subscribe(SelfTopic())`.
- **A third "auto-sync scope" concern (implicit peer sync).** Rejected: collapsed into concern (2). Self-sync is `Subscribe(SelfTopic())` + `Publish` + a reconciler — explicit, no hidden lifecycle.
- **Render-only dispatch mode** and the **`lvt:"local"` state-merge tag**. Rejected: a render-only mode has a silent-no-op failure mode; per-connection state is instead correct *by construction* via the selective reconciler (use case K). No merge machinery.
- **A Phoenix-style single inbox method (`handle_info`/`OnPublish`).** Rejected: Go has no pattern matching, so one inbox forces a per-controller type-switch (more boilerplate, weaker type safety) and a reserved magic method is a hidden-lifecycle hazard. Named-action dispatch via the existing `DispatchWithState` resolver adds zero machinery.
- **`GetByUserExcept` / empty-userID fan-out.** Rejected/dropped: the reserved-`lvt:` namespace makes the anonymous self-topic a specific non-empty `GroupID`, so the empty-key footgun cannot exist; `GetByUserExcept` is unnecessary.
- **Single trailing `*` only.** Superseded: multi-segment wildcards (`room/*/log`, `*/alice`, `a/*/b/*`) are in v1 (body §2). The matcher remains a flat O(P) segment scan — no trie/radix index, no glob engine.
- **`Publish` debounce/coalesce helper.** Rejected: fan-out is always explicit; debouncing is the developer's call at the call site.
- **`GroupID` field reuse for the topic envelope.** Rejected in favor of a new `Topic` field: repurposing `GroupID` to carry a topic leaves a permanently misnamed wire field.

### C. Pre-implementation audit (2026-05-15)

A code-grounded audit verified the load-bearing claims against the codebase and found them
accurate: `handleDispatchedAction` runs the controller method against each receiver's own
`connState.state`; topic and user-action dispatch share the one `DispatchWithState` resolver;
the recursion guard exists; the single `processMessages` pump on one multiplexed
`pubsub.Channel()` is real (go-redis v9.x); `New()` returns `error` (so a hard error on the
mutually-exclusive options is idiomatic); the `Sync()` controller method was removed in PR #406
(commit `8fc9467b`); the cross-repo `BroadcastAction` site counts are exact.

Defects found and fixed in the body before implementation:

| Area | Issue | Resolution (in body) |
|---|---|---|
| Topic grammar | Developer grammar excludes `:` but `lvt:` topics contain it; `Subscribe` "validates the grammar" would reject `SelfTopic()` | §2 "Grammar" + §"Critical files": explicit two-validator order (reserved-namespace exact-match first; segment grammar for non-`lvt:` only) |
| Symmetry guard | "the parser already tracks wired action names" — false; actions resolve by reflective dispatch, no registry in `internal/parse/` | Guard kept (footgun is severe); the small new parse-layer wired-name pass is scoped in §"Critical files" + Phase 1 + `V19` |
| Self-sync | Canonical `_ = ctx.Subscribe(ctx.SelfTopic())` could silently swallow the empty-identity programmer error | §1: fail-closed + loud (`slog.Error` at the `SelfTopic()` site, independent of the ignored return) |
| PubSub interface | `PublishGroupAction`/`SubscribeGroupActions` are on `GroupActionBroadcaster`, not base `Broadcaster` | Anchors corrected; new methods extend the optional `DynamicSubscriber`/`GroupActionBroadcaster` pattern |
| Cross-instance dedup | No-lock dedup relies on an unstated precondition | §2: `PSUBSCRIBE` must be on the same `*redis.PubSub` instance the single pump reads |
| Envelope shape | `GroupActionMessage` also carries `groupID`; topic-travel unspecified | §"Critical files": add a `Topic` field (do not repurpose `GroupID`) |
| Misc precision | `SelfTopic()` identity source on GET; eager-ACL-on-GET as a fixed default; the `lvt` rename spans two test funcs; the docs repo pins pre-#406 `v0.8.23` so its `Sync()` e2e passes until the pin bump (stale comments there even cite now-wrong `mount.go` line numbers — a case for grep-anchors over line numbers) | Reworded in §"Current architecture", §3, §6, §"Verification" (`V15`) |
