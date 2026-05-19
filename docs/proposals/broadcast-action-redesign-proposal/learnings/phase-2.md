# Phase 2 — Learnings

**Session:** 2026-05-18 / claude-code   **Status at exit:** complete
**Plan reference:** Phase 2 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live `main` @ `cec0d3ea` (Phase 0 #417 `fcaa92b2`,
Phase 1 #419 `9836d116`, pubsub race fixes #421 `1bca9aef` + #423 `cec0d3ea` all
merged). Worktree `.worktrees/broadcast-redesign-phase-2` off `main`. Read
`phase-1.md` first (per protocol); applied its "Adjustments for Phase 2".

**Confirmed reuse targets (grep anchors held):**
- `pubsub/types.go`: `Broadcaster` (base), `GroupActionBroadcaster`
  (`PublishGroupAction`/`SubscribeGroupActions`), `GroupActionSubscriber`
  (`SubscribeToGroupAction`/`UnsubscribeFromGroupAction`, ref-counted),
  `DynamicSubscriber`, `GroupActionMessage`/`GroupActionHandler`.
- `pubsub/redis.go`: `PublishGroupAction` (`channelGroupAction+groupID`,
  `publishJSON`), `SubscribeGroupActions` (stores `groupActionHandler`),
  `SubscribeToGroupAction`→`subscribeTo`, `UnsubscribeFromGroupAction`→
  `unsubscribeFrom`, `subscribedChannels` ref-count map, `subscribeTo`/
  `trySubscribe`/`unsubscribeFrom`, `processMessages` single pump,
  `handleMessage` (own-instance filter `typeCheck.InstanceID == b.instanceID`
  at redis.go:734, then type switch), `reconnect()` replays `subscribedChannels`.
- `mount.go`: `dispatchBroadcastToGroup` (local `GetByGroupExcept`+`EnqueueDispatch`
  loop, then `GroupActionBroadcaster` cross-instance block), `dispatchToTopic`
  (Phase 1 local-only, explicit "Phase 2 adds remote leg here" seam, no stub),
  `processTopicPublishes`, `handleGroupActionMessage`, `GetByTopicExcept`.
- `template.go` `Handle()`: pub/sub subscriber wiring site (1647–1665) — where
  `SubscribeGroupActions(handler.handleGroupActionMessage)` is registered;
  `LiveHandler` is the `*liveHandler` returned here (the spec's `handler.`).
- `topic_runtime.go`: `liveTopicSubscriber.registerTopic`/`unregisterTopic`
  (the Redis-relay seam; already holds `s.h`+`s.conn`).
- `topic_context.go`: `ctx.Publish` validation/queue (`topicPublish`,
  `MaxBroadcastsPerAction` cap, permissive-`lvt:`).

**Envelope check:** `GroupActionMessage` has only `Type/GroupID/Action/Data/
Timestamp/InstanceID` — **no field carries a per-instance monotonic counter**;
`Topic string`+`Seq uint64` additions are clean (confirmed per plan).

**Deviations from the plan's predictions (audit reshaped the task list — its purpose):**
1. **No in-memory broadcaster exists.** Phase 1 single-instance tests used a
   `nil` PubSubBroadcaster (registry-local only). V8/V9 cross-instance tests
   therefore use the **real `RedisBroadcaster` via testcontainers**, two
   `liveHandler`s sharing one Redis — exactly the proposal's Tier-1 harness note
   ("Redis items via testcontainers"), not a deviation from intent.
2. **`handler.Publish` is `LiveHandler.Publish`, not `*Template.Publish`.** The
   spec's "`handler.Publish`" maps to the `LiveHandler` value returned by
   `Template.Handle()` (the `*liveHandler` built at `template.go:1622`), which
   already holds `config.PubSubBroadcaster`+`registry`. Out-of-band Publish is a
   new method on that interface; no Context, `excludeConn=nil`, no ACL (send
   ungated §3), no recursion guard (not in a dispatched action).
3. **Inherited `-race` blocker is RESOLVED upstream.** `phase-1.md` / the
   progress.md Phase 2 row flagged `TestReconnect_DoesNotBlockConcurrentPublish`
   as a Phase-2-owned `pubsub` race to fix/skip-guard. It (#420) **and** the
   sibling `subscribeHook` test-seam race (#422) were already fixed by #421
   (`1bca9aef`) + #423 (`cec0d3ea`), both on `main`. Phase 2 inherits a clean
   `pubsub` `-race`; no fix/skip-guard work needed — only scope `-race` to the
   touched packages (`pubsub`/`internal/session`/topic tests) per phase-0 policy.

**Scope decision (advisor-confirmed): seen-ring dedup is Phase 3, not Phase 2.**
The `(instanceID, seq)` seen-ring is the *consumer* of the double-fire problem,
which only exists once a `PSUBSCRIBE` wildcard subscription co-exists with an
exact `SUBSCRIBE` (Phase 3, gated by V17). Phase 2 has exact `SUBSCRIBE` only.
Phase 2 therefore ships the **wire plumbing only**: the `Topic`+`Seq` envelope
fields and the per-instance atomic counter incremented on every
`GroupActionMessage` emit (group-action *and* topic, per the spec's "Seq scope"
note). The ring lands in Phase 3 alongside `PSUBSCRIBE`, exercised by V17. The
Phase-2 learnings-exit "confirmation that the `(instanceID, seq)` dedup key (not
`timestamp`) held cross-instance" is satisfied by a stable, correctly-keyed wire
format Phase 3 inherits — V17 (Phase 3) asserts ring correctness.

**Benchmark-harness readiness (cross-cutting precondition, confirmed):** the
existing Redis testcontainers harness in `pubsub/redis_test.go` + `session_test.go`
is runnable here (pre-commit gate already runs the full suite incl. Redis
testcontainers). Baseline `GroupActionMessage` path (`PublishGroupAction`) is
measurable for the round-trip comparison. (Readiness only; numbers below.)

## What shipped

Cross-instance (Redis) Publish/Subscribe wired onto Phase 1's single-instance
seam. `BroadcastAction` + its group-dispatch path **untouched** (still fully
functional, as required through Phases 0–4).

- **`pubsub/types.go`** — `GroupActionMessage` gains `Topic string`
  (`json:"topic,omitempty"`) + `Seq uint64` (`json:"seq"`); the existing
  `groupID` json tag is **unchanged** (BroadcastAction wire bytes preserved).
  The shared-counter LOAD-BEARING INVARIANT is documented on the struct (safe
  only while group-action stays exact-`SUBSCRIBE`-only). Two new optional
  interfaces: `TopicActionBroadcaster{PublishToTopic; SubscribeToTopicActions}`
  and `TopicChannelSubscriber{SubscribeToTopicChannel;
  UnsubscribeFromTopicChannel}` (ref-counted, DynamicSubscriber contract).
  Topic messages reuse `GroupActionMessage`/`GroupActionHandler`.
- **`pubsub/redis.go`** — `channelTopic = "livetemplate:topic:"`; per-instance
  `seq atomic.Uint64` + `b.seq.Add(1)` set on **both** `PublishGroupAction` and
  `PublishToTopic` (the spec's "Seq scope"); `topicActionHandler` field;
  `PublishToTopic`/`SubscribeToTopicActions`/`SubscribeToTopicChannel`/
  `UnsubscribeFromTopicChannel` mirroring the group-action methods (the last
  two go through the existing `subscribeTo`/`unsubscribeFrom`, so they slot
  into `subscribedChannels` and get **free reconnect-replay**); `"topic_action"`
  case in the single-pump `handleMessage` → `handleTopicActionMessage`
  (own-instance already filtered by the InstanceID guard).
- **`mount.go`** — `dispatchToTopic` cross-instance leg (the
  `TopicActionBroadcaster` block mirroring `dispatchBroadcastToGroup`'s
  `GroupActionBroadcaster` block); `handleTopicActionMessage` resolving
  `GetByTopicExcept(msg.Topic, nil, segmentMatch)` → `EnqueueDispatch`;
  `LiveHandler.Publish(topic, action, data)` out-of-band entry (validates
  topic/action like ctx.Publish, NO ACL/recursion-guard/symmetry-warn,
  `excludeConn=nil`, dispatches immediately); `defer h.releaseRelayedTopics`
  registered right after `defer h.registry.Unregister` (LIFO ⇒ teardown sweep
  runs while `conn.subscribedTopics` is still live).
- **`topic_runtime.go`** — `registerTopic`/`unregisterTopic` relay
  `SubscribeToTopicChannel`/`UnsubscribeFromTopicChannel` on the per-connection
  transition bool; `releaseRelayedTopics` disconnect sweep (root-package driven
  — `internal/session` can't import `pubsub`).
- **`internal/session/registry.go`** — `SubscribeConnectionToTopic`/
  `UnsubscribeConnectionFromTopic` now return the per-connection 0→1 / 1→0
  transition `bool`; new `SubscribedTopics(conn) []string` snapshot accessor.
- **`topic_wired_actions.go`** — Phase 1 round-4 fix: `canonicalActionKey =
  toSnakeCase` (the form `methodNameToActions` reduces every style to); wired
  set + Publish arg both canonicalized; dedup keyed on the canonical form. A
  style-mismatched collision (`Publish "Save"` vs `name="save"`) is now caught;
  V19 stays green (logged value is the raw action; only the detection/dedup
  key is canonical).
- **`template.go`** — `SubscribeToTopicActions(handler.handleTopicActionMessage)`
  wired in `Handle()` next to `SubscribeGroupActions`.

**Gate (Phase 2 = V8, V9): GREEN.** `TestTopic_V8_OutOfBandHandlerPublish_*`
(announcements single-instance + UserTopic cross-instance A→B) and
`TestTopic_V9_CrossInstancePublish` pass against two real `RedisBroadcaster`s
on one Redis testcontainer. Full `scripts/pre-commit.sh` green (go fmt +
golangci-lint enable-only set **0 issues** + full `go test ./...` incl. Redis
testcontainers, root `ok 115.4s`). `-race` scoped green: `pubsub` 25.2s (the
#420/#422 fixes from #421/#423 proven under contention), `internal/session`
32.0s, root `TestTopic` 7.2s. Phase-1 V1–V7/V10–V12/V16/V19/V21
regression-green.

**Benchmarks (§"Benchmarks required" items 1 & 3; item 2 = Phase 3):**

| Topic fan-out (local enqueue) | ns/op | B/op | allocs/op |
|---|---|---|---|
| N=1 | 922 | 232 | 6 |
| N=5 | 2 392 | 400 | 10 |
| N=10 | 4 999 | 920 | 18 |
| N=50 | 21 917 | 4 328 | 62 |
| N=100 | 99 037 | 8 744 | 114 |

Flat registry scan + `EnqueueDispatch` loop; ≤100µs to enqueue 100 subscribers
— acceptable (the slight super-linearity N=50→100 is benchmark drain-goroutine
scheduling, not the registry scan, which is a plain slice iterate).

**Cross-instance round-trip (single Redis, 50 iters):** topic **250.7µs** vs
`PublishGroupAction` baseline **259.9µs**, ratio **0.96×** — no measurable
overhead (expected: the topic path is the same envelope/pump/`publishJSON`
machinery with a different `Type` + routing key).

**Spec-exit confirmation (bot-verifiable, for the Phase 3 audit):** the
`(instanceID, seq)` dedup key (NOT `timestamp`) held cross-instance — every
cross-instance `GroupActionMessage` carried a **non-zero, strictly
monotonically-increasing per-instance `Seq`** (`atomic.Uint64.Add(1)` on every
emit, group-action AND topic), preserved byte-for-byte end-to-end through Redis
(verified by V8/V9 round-tripping the envelope); **`Timestamp` was used for no
dedup decision anywhere** (it stays observability-only, exactly as the spec
requires). The ring *consumer* is Phase 3 (V17 — no SUBSCRIBE+PSUBSCRIBE
double-fire exists until then), but the *key* it will consume is now wired,
correct, and proven on the wire. Persist-before-publish ordering holds (the
drain is unchanged from Phase 1, at the post-`persistState` `processBroadcasts`
site; cross-instance adds no new ordering surface — the publish happens after
the same drain).

**`BroadcastAction` untouched — `Seq` addition is spec-directed, not a
deviation.** `PublishGroupAction` now sets `Seq` on its envelope. This is
explicitly mandated by §"Critical files" ("`Seq` is one per-instance counter
incremented for every `GroupActionMessage` the instance emits — topic *and*
group-action"). It is an **additive, backward-compatible JSON field** (Go's
unmarshal ignores unknown fields; `groupID`'s tag is unchanged so existing
group-action wire bytes are byte-identical except the new `"seq":N`). The
spec's "BroadcastAction untouched through Phases 0–4" governs the API,
semantics, and dispatch path — all unchanged; extending the shared envelope is
the spec's own instruction, recorded here so the Phase 3 audit reads it as
spec-directed, not as drift.

## Deviations from plan

1. **Registry signatures changed (audit reshaped the task list).**
   `SubscribeConnectionToTopic`/`UnsubscribeConnectionFromTopic` were `void`;
   now return the per-connection 0→1 / 1→0 transition `bool`, and a new
   `SubscribedTopics(conn) []string` accessor was added. Forced by
   correctness: the broadcaster channel refcount is **instance-wide** but the
   registry subscribe is **per-connection idempotent**, so the relay must fire
   exactly once per connection-topic — the registry's existing transition
   signal had to be exposed. Mirrors the `DynamicSubscriber` "issue transport
   SUBSCRIBE only on the 0→1 transition" contract. Phase 1 callers ignored the
   (then-void) return; updated.
2. **Disconnect relay teardown is root-package-driven, not in `Unregister`.**
   `internal/session` cannot import `pubsub` (import cycle — same boundary
   that forced Phase 0's injected `match` param). `releaseRelayedTopics` is a
   `*liveHandler` method deferred immediately after `defer
   h.registry.Unregister` so Go `defer` LIFO runs it **before** Unregister
   nils `conn.subscribedTopics`; it reads the live set via `SubscribedTopics`.
3. **`handler.Publish` is `LiveHandler.Publish`, not `*Template.Publish`.** The
   spec's "`handler.Publish`" maps to the `LiveHandler` value `Template.Handle()`
   returns (the `*liveHandler`), which already holds the broadcaster + registry.
   Added to the `LiveHandler` interface (all `handler.(*liveHandler)` test
   assertions compiled clean once implemented).
4. **No in-memory broadcaster — V8/V9 use two real `RedisBroadcaster`s on one
   Redis testcontainer.** Two distinct InstanceIDs (a *shared* broadcaster
   would self-filter via the own-instance drop and `SubscribeToTopicActions`
   "already subscribed" — genuinely two processes need two broadcasters). This
   *is* the proposal's Tier-1 harness intent ("Redis items via testcontainers,
   two liveHandler instances"); "sharing one RedisBroadcaster" in the prose was
   imprecise — corrected here.
5. **Seen-ring dedup deferred to Phase 3 (advisor-confirmed).** Phase 2 ships
   only the `Topic`+`Seq` wire plumbing + per-instance atomic counter. The
   `(instanceID, seq)` seen-ring is the *consumer* of the SUBSCRIBE+PSUBSCRIBE
   double-fire, which exists only once Phase 3 adds `PSUBSCRIBE` (gated by
   V17). The Phase-2 learnings-exit "confirmation that the dedup key held
   cross-instance" is satisfied by a stable, correctly-keyed wire format Phase
   3 inherits (the `Seq` round-trips through V8/V9); V17 asserts ring
   correctness.
6. **Phase 1 round-4 normalization fix landed here** (it was deferred to Phase
   2). `canonicalActionKey = toSnakeCase` — the invariant form
   `methodNameToActions` collapses camelCase/snake_case/PascalCase to. Both
   sides canonicalized; the false-negative class ("save" vs "Save") is closed;
   no spurious warnings (canonicalization only widens true collisions).

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

- **Registry public-surface change (bool returns + `SubscribedTopics`).** Not
  an Appendix-B item; a mechanical correctness requirement for exactly-once
  cross-instance relay. Phase 3's pattern (`byTopicPattern`/`PSUBSCRIBE`) relay
  reuses the identical transition-bool + disconnect-sweep shape.
- **Benign pre-existing testcontainer-cleanup log** (`redis.go:146 "Failed to
  cleanup Redis container: redis: client is closed"`) also fires in the
  pre-existing `TestRedisBroadcaster_RefcountSurvivesUnsubscribeUntilZero`;
  it is `getTestRedisClient`'s own t.Cleanup racing the broadcaster Close —
  harmless (the container is reaped regardless). Not worth special-casing.
- None map onto an Appendix-B alternative. Multi-segment wildcards remain IN
  v1 (Phase 3 owns them; the general `segmentMatch` resolver is already used by
  `handleTopicActionMessage`/`dispatchToTopic`).

## Adjustments recommended for the next phase

Phase 3 (wildcards / multi-segment PSUBSCRIBE) inherits a clean seam:

- **Single-PubSub-instance invariant target confirmed:** the single
  `processMessages` pump reads `b.pubsub` (`*redis.PubSub`, established by
  `subscribeTo`, refreshed in `reconnect()`). Phase 3's `PSUBSCRIBE` MUST be
  issued on that **same `b.pubsub`** — verify against `subscribeTo`/
  `processMessages` (do not create a second PubSub).
- **Pattern channel scaffolding:** add `subscribedPatterns map[string]int`
  parallel to `subscribedChannels`, replayed in the same `reconnect()` loop.
  `SubscribeToTopicChannel`/`UnsubscribeFromTopicChannel` (exact) are the
  template; add `SubscribeToTopicPattern`/`UnsubscribeFromTopicPattern`
  (PSUBSCRIBE). The relay in `registerTopic`/`releaseRelayedTopics` must branch
  on `isPatternTopic(topic)` to pick exact-SUBSCRIBE vs PSUBSCRIBE — the relay
  MUST NOT expand a pattern into per-concrete SUBSCRIBEs (relay invariant).
- **Seen-ring dedup lands in Phase 3** in the single pump *before* registry
  resolution. The `Seq` counter + envelope field are already wired and
  cross-instance-proven (V8/V9). `handleTopicActionMessage` is the resolution
  site to fold the `(instanceID, seq)` bounded ring (N=64, no map, single
  serialized pump ⇒ no lock) in front of `GetByTopicExcept`. **Two ring
  constraints (documented at the `seq` field, surfaced by the #424 round-3
  review):** (a) `seq` is monotonic per-instance only, never per-Type (group +
  topic interleave one counter — the ring keys on `(instanceID, seq)`, never
  assumes contiguity); (b) **`seq==0` ⇒ pre-upgrade sender** — a rolling-upgrade
  instance running pre-`Seq` code omits the field (JSON→0), so *every* message
  from it has seq=0; the ring MUST bypass dedup when `seq==0` (process
  unconditionally — a pre-Phase-2 instance has no topic `PSUBSCRIBE`, hence no
  double-fire, so this is correct). A naive `(instanceID,0)` key would collapse
  all-but-one of an old instance's messages.
- **Local strict re-match:** `handleTopicActionMessage` already resolves via
  `GetByTopicExcept(msg.Topic, nil, segmentMatch)` (the general exact∪pattern
  resolver, matcher already injected) — Phase 3's PSUBSCRIBE over-delivery
  rejection folds in here (Redis `*` spans `/`; drop a non-`segmentMatch`).
- **Transition-bool + disconnect-sweep pattern generalizes:**
  `releaseRelayedTopics` iterates `registry.SubscribedTopics(conn)` (exact ∪
  pattern once `byTopicPattern` relay exists) — Phase 3 reuses it as-is.

## Open questions for the user

- **None blocking.** The seen-ring scope (Phase 3, not Phase 2) was an
  advisor-confirmed engineering call, not a user decision — recorded in the
  Audit + Deviation 5 for the Phase 3 audit. The out-of-band `handler.Publish`
  deliberately emits **no** symmetry-collision warning (trusted server code; no
  per-Context template binding to resolve a wired-name set against) — recorded
  for the Phase 6 docs.
- Per the signoff gate: commit/PR is left to the user (no push/PR without
  explicit signoff after manual testing). `phase-2.md` commits in Phase 2's
  own PR alongside its code.

## File / commit / V-item pointers

- **New files:** `topic_cross_instance_test.go` (V8/V9), 
  `topic_phase2_coverage_test.go` (6 deferred-coverage + the round-4
  normalization test), `topic_bench_test.go` (fan-out-by-N benchmark +
  cross-instance-vs-group-action latency test).
- **Modified:** `pubsub/types.go`, `pubsub/redis.go`, `mount.go`,
  `topic_runtime.go`, `topic_wired_actions.go`, `template.go`,
  `internal/session/registry.go`.
- **V-items:** V8 — `TestTopic_V8_OutOfBandHandlerPublish_Announcements`
  (single-instance F/H) + `TestTopic_V8_OutOfBandHandlerPublish_UserTopicCrossInstance`
  (cross-instance F, A→B); V9 — `TestTopic_V9_CrossInstancePublish`. All green
  (Redis testcontainers; skip if Docker absent). Deferred coverage:
  `TestTopic_Phase2_{PublishInvalidTopic,PublishCap,Unsubscribe,
  ServerOriginatedSubscribeNilRequest,PublishFromUploadComplete,
  TopicForbiddenEnvelope,SymmetryCollisionStyleMismatch}` — all green.
- **Gate command:** `bash scripts/pre-commit.sh` (green: go fmt +
  `golangci-lint --enable-only=errcheck,govet,ineffassign,staticcheck,unparam,unused`
  0 issues + full `go test ./...` incl. Redis testcontainers); `-race` scoped
  to `./pubsub/ ./internal/session/` + root `-run TestTopic` (all green).
- **Commit:** local branch `broadcast-redesign-phase-2` (worktree
  `.worktrees/broadcast-redesign-phase-2`); **not committed** — left for user
  signoff per the gate. `phase-2.md` commits in Phase 2's own PR.
