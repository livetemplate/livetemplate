# Phase 3 — Learnings

**Session:** 2026-05-19 / claude-code   **Status at exit:** complete
**Plan reference:** Phase 3 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live `main` @ `b7b1ab08` (Phase 2 #424 squash;
Phases 0/1/2 all merged). Worktree `.worktrees/broadcast-redesign-phase-3` off
`main` (branch `broadcast-redesign-phase-3`). Read `phase-2.md` first (per
protocol) + the seeded Phase 3 row + Ledger; applied phase-2.md's "Adjustments
recommended for the next phase".

**Inherited seams re-verified against the Phase-2 code as built (all held):**

1. **Single-PubSub-instance invariant — CONFIRMED.** The single
   `processMessages` pump reads `b.pubsub.Channel()` (`pubsub/redis.go`
   `processMessages`). `b.pubsub` (`*redis.PubSub`) is established in
   `Subscribe()` (`b.client.Subscribe(b.ctx, channelGlobal)`) and refreshed in
   `reconnect()` (`b.pubsub = newPubSub`). Phase 3's `PSUBSCRIBE` is issued on
   this same `b.pubsub` (go-redis v9.17.2 multiplexes exact + pattern
   deliveries onto the one `.Channel()` per PubSub instance — verified in the
   go-redis source: `PSubscribe` adds to the same `*PubSub`'s `c.patterns`).
   Matches phase-2.md's stated target (`b.pubsub`, set by `subscribeTo`,
   refreshed in `reconnect()`).
2. **Relay invariant — CONFIRMED feasible.** `relayTopicSubscribe`
   (`topic_runtime.go`) unconditionally called `SubscribeToTopicChannel`
   (exact SUBSCRIBE). Phase 3 branches on pattern-ness so a pattern relays as
   a single `PSUBSCRIBE livetemplate:topic:<glob>`, never expanded into
   per-concrete SUBSCRIBEs. `relayTopicUnsubscribe` + `releaseRelayedTopics`
   branch identically.
3. **Local strict re-match — STRUCTURALLY ALREADY PRESENT (audit reshapes the
   task).** The proposal lists "Local strict re-match (must hold)" as a Phase 3
   work item, but `handleTopicActionMessage` (mount.go) already resolves via
   `registry.GetByTopicExcept(msg.Topic, nil, segmentMatch)`, and
   `GetByTopicExcept` iterates `byTopicPattern` applying
   `segmentMatch(pattern, concrete)`. Because `segmentMatch` requires equal
   segment count and `*` never spans `/`, a Redis PSUBSCRIBE over-delivery
   (`room/42/log` delivered to a `room/*` PSUBSCRIBE) resolves to zero local
   connections → dropped. **No new re-match code needed; the Phase 3 task here
   reduces to confirm + V17 over-delivery-rejection test** — exactly what
   phase-2.md predicted ("folds in here").
4. **reconnect() pattern replay — GAP confirmed (as predicted).**
   `reconnect()` snapshots only `b.subscribedChannels` (exact) and calls
   `b.client.Subscribe(b.ctx, channels...)`. Phase 3 adds
   `b.subscribedPatterns map[string]int` and replays it via
   `newPubSub.PSubscribe(b.ctx, patterns...)` on the **same** newPubSub before
   it is installed.
5. **Seen-ring dedup site — CONFIRMED, placement decided.** The proposal
   §"Cross-instance exactly-once" mandates the ring "inside the existing single
   `processMessages` pump … *before* registry resolution/enqueue". The pump is
   `pubsub/redis.go`; registry resolution (`GetByTopicExcept`) is
   `mount.go`'s `handleTopicActionMessage`, reached via the `handler(&msg)`
   callback. Placed in the **pubsub-layer** `handleTopicActionMessage`
   (`redis.go`), after the InstanceID re-check and before `handler(&msg)` —
   this is simultaneously "inside the processMessages pump" (proposal
   normative text) and "before `GetByTopicExcept`" (phase-2.md). The ring keys
   on `(InstanceID, Seq)` only — no `segmentMatch`, so no import-cycle issue.
   Recorded as a deliberate deviation vs phase-2.md's literal "folded into
   handleTopicActionMessage" wording (which is ambiguous between the two
   same-named methods) — see Deviations.
6. **Registry side fully Phase-0-built — no Phase 3 registry change.**
   `byTopicPattern`, `isPatternTopic`, `SubscribeConnectionToTopic` (routes
   patterns to `byTopicPattern`), `UnsubscribeConnectionFromTopic`,
   `SubscribedTopics` (returns exact ∪ pattern), `GetByTopicExcept` (deduped
   union, injected matcher, panics on `*` concrete + nil-match-with-patterns),
   and `Unregister`'s topic-GC (walks `conn.subscribedTopics`, handles
   `byTopicPattern`) — all present and tested since Phase 0.
7. **Two ring constraints verified documented in code.** `seq` monotonic
   per-instance-not-per-Type and `seq==0 ⇒ pre-upgrade ⇒ bypass dedup` are both
   spelled out at the `seq atomic.Uint64` field (`redis.go`) and the
   `GroupActionMessage` doc (`pubsub/types.go`, incl. the `json:"seq"`
   intentionally-not-omitempty note). The ring honors both.

**Surfaced scope (audit-discovered, not in the proposal's Phase 3 list):**
- **`Publish` to a `*` pattern panics.** `validateDeveloperTopic` *permits*
  `*` segments (grammar is general-case by design), so
  `ctx.Publish("room/*", …)` / `handler.Publish("room/*", …)` passes
  validation, then `dispatchToTopic` → `GetByTopicExcept("room/*", …)` panics
  ("concrete topic must not contain \"*\""). Latent in Phases 0–2 (only
  concrete topics were ever published). Phase 3 makes patterns a first-class
  developer concept (`Subscribe("room/*")`), so a mistaken `Publish` to a
  pattern is now a plausible footgun that crashes the post-action drain /
  out-of-band caller. Phase 3 adds an `isPatternTopic` reject guard at both
  Publish boundaries (clean error; "publish to a concrete topic — patterns are
  Subscribe-only"). Rolled into the Ledger.

**§"Benchmarks required" item 2 readiness (cross-cutting precondition):** the
existing benchmark harness (`topic_bench_test.go`, Phase 2) is runnable here;
the flat O(P) `segmentMatch` scan in `GetByTopicExcept` is the measurable
surface for the pattern-scan-cost benchmark (P ∈ {1,10,100}). Readiness only;
numbers below.

## What shipped

Cross-instance multi-segment wildcards on Phase 2's exact-SUBSCRIBE seam.
`BroadcastAction` + its group-dispatch path **untouched** (still fully
functional, as required through Phases 0–4). Multi-segment wildcards **HOLD in
v1** — the explicit Phase-3 call (recorded in the progress.md journal): the
flat O(P) `segmentMatch` resolver needed no reduction; the general-case matcher
Phase 0 built carried straight through to PSUBSCRIBE with no narrowing.

- **`pubsub/redis.go`** — `subscribedPatterns map[string]int` parallel to
  `subscribedChannels`. The race-critical retry / check-lock-check loop is
  **single-sourced**: `subscribeTo`/`trySubscribe`/`unsubscribeFrom` gained a
  `subKind` param (`subExact`/`subPattern`) selecting the refcount map
  (`b.refcounts(kind)`) and the Redis verb (`redisSubscribe`/`redisUnsubscribe`
  → `PSubscribe`/`PUnsubscribe` vs `Subscribe`/`Unsubscribe`); the 10 exact
  callers pass `subExact` explicitly (group-action path byte-for-byte
  unchanged, just tagged). `SubscribeToTopicPattern`/`UnsubscribeFromTopicPattern`
  issue PSUBSCRIBE/PUNSUBSCRIBE on the **same `b.pubsub`** the one
  `processMessages` pump reads. `reconnect()` snapshots `subscribedPatterns`
  and replays them via `newPubSub.PSubscribe(b.ctx, patterns...)` on the SAME
  new PubSub after the connectivity `Receive` (no second Receive — the
  `.Channel()` pump drains the psubscribe confirmation, identical to the live
  dynamic-add path; documented in `reconnect()`). `Close()` nils
  `subscribedPatterns` for parity.
- **`pubsub/redis.go`** — `seenRing` (fixed `[64]seenID`, lock-free, O(N)
  linear scan, no map) + `seenThenRecord`; folded into `handleTopicActionMessage`
  after the InstanceID re-check, before `handler(&msg)`. `seq==0` bypasses the
  ring ENTIRELY via short-circuit `&&` (not dedup-checked AND not recorded — the
  two-halves discipline; `seenThenRecord` is not called when `msg.Seq==0`).
- **`pubsub/types.go`** — `TopicPatternSubscriber` as a **separate** optional
  interface (not a `TopicChannelSubscriber` extension — no backward-incompatible
  widening for external broadcasters).
- **`topics.go`** — root-package `isPatternTopic` (cross-boundary twin of
  `session.isPatternTopic`, same `contains "*"` semantics; documented).
- **`topic_runtime.go`** — `relayTopicSubscribeOne`/`relayTopicUnsubscribeOne`
  (`*liveHandler` methods) branch on `isPatternTopic`: pattern →
  `TopicPatternSubscriber` PSUBSCRIBE, exact → `TopicChannelSubscriber`
  SUBSCRIBE; **never expand a pattern** (the relay invariant lives here, one
  source of truth, reused by the per-action relay AND the disconnect sweep).
  `releaseRelayedTopics` dropped its single top-level interface assert and now
  branches per-topic (a pattern is PUNSUBSCRIBEd, not mis-released as an exact
  UNSUBSCRIBE that would leak the PSUBSCRIBE refcount).
- **`topic_context.go` + `mount.go`** — surfaced-scope guard: `ctx.Publish`
  and `handler.Publish` reject `isPatternTopic(topic)` with a clean error
  ("publish to a concrete topic; patterns are Subscribe-only") instead of
  panicking `GetByTopicExcept` on a `"*"` concrete.

**Local strict re-match — confirmed structural, no code (audit-predicted).**
`handleTopicActionMessage` (mount.go) already resolves via
`GetByTopicExcept(msg.Topic, nil, segmentMatch)`; `segmentMatch`'s equal-segment-
count rule means a Redis PSUBSCRIBE over-delivery (`room/42/other` to a `room/*`
PSUBSCRIBE) resolves to zero local connections. V17's cross-instance
over-delivery leg proves it end-to-end; no new rejection code was needed.

**Gate (Phase 3 = V17, V18): GREEN.** Single-instance (deterministic, no Redis):
`TestTopic_V17_SingleInstance_MultiSegmentFanoutAndDedup` (trailing/multi/leading
patterns + segment-count valve + exact∪pattern exactly-one dedup),
`TestTopic_V18_ACLLiteralPattern_AndV17FirstEverNoReACL` (ACL gets the literal
pattern; first-ever concrete delivered with ACL NOT re-invoked at Publish).
Cross-instance (Redis testcontainers): `TestTopic_V17_CrossInstance_PSubscribeDelivery`,
`_OverDeliveryRejected`, `_DoubleFireExactlyOnce`. Pubsub units:
`TestSeenRing`, `TestHandleTopicActionMessage_DedupAndSeqZeroBypass` (the
seq==0 two-halves discipline), `TestRedisBroadcaster_ReconnectPreservesPatternSubscriptions`
(the `subscribedPatterns` PSubscribe-replay seam). `TestIsPatternTopic`,
`TestSegmentMatch` regression-green.

Full `scripts/pre-commit.sh` GREEN (go fmt + `golangci-lint
--enable-only=errcheck,govet,ineffassign,staticcheck,unparam,unused` **0
issues** + full `go test ./...` incl. Redis testcontainers; pubsub `ok 25.2s`).
Cross-instance tests 3×-repeat green (no flakiness). `-race` scoped green:
`pubsub` 25.9s (the kind-aware `trySubscribe` race seam under contention via
the single shared `currentSubscribeHook`), `internal/session` 32.3s, root
`TestTopic|TestSegmentMatch|TestIsPatternTopic` 11.2s.

**Benchmarks (§"Benchmarks required" item 2 — pattern-scan O(P); items 1 & 3
were Phase 2):**

| Pattern-scan, P distinct patterns, concrete matches none (worst case) | ns/op | B/op | allocs/op |
|---|---|---|---|
| P=1   | 836    | 256  | 6   |
| P=10  | 3 407  | 1000 | 24  |
| P=100 | 22 224 | 8193 | 204 |

Clean linear **O(P)** (~220 ns/pattern, ~80 B/pattern) — the flat
`segmentMatch` scan is adequate at expected pattern counts; **no trie/radix
index is warranted** (proposal §2 "Matcher" / Appendix B — this benchmark
validates the linear scan is adequate, not whether to add an index).

## Deviations from plan

1. **Seen-ring placed in the pubsub-layer `handleTopicActionMessage`
   (`redis.go`), not mount.go's.** phase-2.md's "folded into
   `handleTopicActionMessage` before `GetByTopicExcept`" is ambiguous between
   the two same-named methods. The proposal §"Cross-instance exactly-once" is
   normative and explicit: "inside the existing single `processMessages`
   pump … before registry resolution/enqueue". The pump is `pubsub`; the
   pubsub-layer `handleTopicActionMessage` runs on that one serialized
   goroutine and calls `handler(&msg)` (= mount.go's registry resolution), so
   placing the ring there is simultaneously "inside the pump" AND "before
   `GetByTopicExcept`". It also keeps the ring lock-free (one serialized pump,
   no import-cycle — the ring keys only on `(InstanceID, Seq)`, never
   `segmentMatch`). Advisor-confirmed as the call the canonical spec text
   points at.
2. **Local strict re-match was already structural (audit reshaped the task).**
   The proposal lists it as a Phase 3 work item; the audit found
   `GetByTopicExcept`+`segmentMatch` already drop over-delivery by
   construction. Reduced to confirm + the V17 cross-instance over-delivery
   test. (Audit-predicted; phase-2.md said "folds in here".)
3. **`subscribeTo`/`trySubscribe`/`unsubscribeFrom` gained a `subKind`
   parameter** (10 exact call sites updated to pass `subExact`). Not in the
   plan's wording, but forced by the single-PubSub + single-source-the-race-
   loop requirement: duplicating the #420/#422 lock-release seam per verb was
   the documented divergence hazard. Group-action behavior is byte-for-byte
   unchanged (the wire/semantics the "BroadcastAction untouched" rule governs);
   `redis_test.go`'s 5 internal `subscribeTo` callers were updated.
4. **Relay branch centralized as `*liveHandler` methods.**
   `relayTopicSubscribe/Unsubscribe` (on `*liveTopicSubscriber`) now delegate
   to `relayTopicSubscribeOne/relayTopicUnsubscribeOne` (on `*liveHandler`) so
   the relay-invariant branch is one source of truth shared by the per-action
   relay and the disconnect sweep. `releaseRelayedTopics`'s old single
   top-level `TopicChannelSubscriber` assert was a latent bug for patterns
   (would mis-release a pattern as an exact UNSUBSCRIBE, leaking the PSUBSCRIBE
   refcount) — fixed by per-topic branching.

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

- **`Publish` to a `*` pattern panicked `GetByTopicExcept`.** Latent in
  Phases 0–2 (only concretes were ever published); Phase 3 makes `room/*` a
  first-class Subscribe target, so a mistaken `Publish("room/*", …)` became a
  plausible server-crash footgun. Fixed in this phase: `isPatternTopic` reject
  guard at both `ctx.Publish` and `handler.Publish` boundaries (clean error,
  no panic). Not an Appendix-B item — a correctness/ergonomics requirement.
- **`subKind` refactor is a `pubsub`-internal surface change** (the
  unexported `subscribeTo`/`trySubscribe`/`unsubscribeFrom` signatures + 5
  in-package test callers). No exported API change. Phase 5's removal-surface
  audit should note `redis_test.go` now references `subExact`.
- **Pattern-path race coverage decision (recorded, not deferred):** the
  single-sourced `trySubscribe(kind)` means the existing
  `TestSubscribeTo_DoesNotBlockConcurrentOperations` already exercises the
  kind-aware logic against the same `currentSubscribeHook` seam. A dedicated
  `SubscribeToTopicPattern` concurrency variant was judged redundant given the
  single source (advisor-concurred); `pubsub -race` (25.9s) covers it.
- No item maps onto an Appendix-B alternative. **Multi-segment wildcards
  remain IN v1 and HOLD** (the explicit Phase-3 call) — no reduction; the
  general `segmentMatch` resolver carried through to PSUBSCRIBE unchanged.

## Adjustments recommended for the next phase

Phase 4 (client `lvt:error` envelope, parallelizable) — no Phase-3 server
envelope change; the `topic_forbidden` envelope is unchanged from Phase 1.
Phase 4's audit needs only `phase-0.md` (+ optionally `phase-1.md`); nothing
in Phase 3 alters the client contract.

Phase 5 (removal + migration):
- The `BroadcastAction` group-action wire/semantics/dispatch path is
  **untouched**; the `subKind` refactor only *tags* its subscribe calls
  (`subExact`) and single-sources the shared race loop — the removal surface is
  unchanged. The authoritative `grep -rn '\.BroadcastAction(' . --include='*.go'`
  sweep is unaffected by Phase 3 (it added no `BroadcastAction` call sites).
- Phase 5's removal-surface audit should be aware `pubsub` now has
  `subscribedPatterns`, `seenRing`, `TopicPatternSubscriber`,
  `SubscribeToTopicPattern`/`UnsubscribeFromTopicPattern` — none touch
  `BroadcastAction`; listed so the audit's "what's topic vs what's
  group-action" classification is complete.

Phase 6 (docs): document (a) patterns are **Subscribe-only** — publishing to a
`*` topic is a hard error (the new guard); (b) Redis `*` spans `/` so PSUBSCRIBE
over-delivers and the framework re-applies the strict whole-segment
`segmentMatch` on receive (transparent to developers, worth a troubleshooting
note); (c) the `seq==0` rolling-upgrade contract (a pre-Phase-2 instance's
messages all bypass the dedup ring — correct because it has no PSUBSCRIBE).

**Future-work hook (not v1 scope, logged so it is not lost):** the pattern-scan
bench shows ~80 B / 2 allocs **per pattern per publish** — `segmentMatch`
re-`strings.Split`s every registered pattern on every concrete publish. A
zero-allocation optimization is available *without* a trie: pre-split each
pattern into `[]string` segments at Subscribe time, store alongside it in
`byTopicPattern`, and have the matcher compare pre-split slices. This preserves
the flat O(P) design (no index — Appendix B stays frozen) while removing the
per-publish allocation. 22µs at P=100 is well inside the accepted envelope, so
this is a deliberate non-blocker, parked here for a future perf pass.

## Open questions for the user

- **None blocking.** The seen-ring placement (pubsub-layer vs mount.go) was an
  advisor-confirmed reading of the canonical spec text, not a user decision —
  recorded as Deviation 1. The pattern-path race-coverage call (no dedicated
  concurrency test given the single-sourced seam) was an advisor-concurred
  engineering judgement — recorded under New scope.
- Per the signoff gate: the branch is left **local and uncommitted**
  (`.worktrees/broadcast-redesign-phase-3`, branch `broadcast-redesign-phase-3`);
  no push/PR/merge until explicit user signoff after manual testing.
  `phase-3.md` commits in Phase 3's own PR alongside its code.

## File / commit / V-item pointers

- **New files:** `topic_wildcard_test.go` (V17 single + cross-instance, V18),
  `pubsub/seen_ring_test.go` (ring mechanics + seq==0 two-halves discipline).
- **Modified:** `pubsub/types.go`, `pubsub/redis.go`, `pubsub/redis_test.go`
  (5 `subExact` call-site updates + new reconnect-pattern test), `topics.go`,
  `topics_test.go` (`TestIsPatternTopic`), `topic_runtime.go`,
  `topic_context.go`, `mount.go`, `topic_bench_test.go` (pattern-scan bench).
- **V-items:** V17 — `TestTopic_V17_SingleInstance_MultiSegmentFanoutAndDedup`
  + `TestTopic_V17_CrossInstance_{PSubscribeDelivery,OverDeliveryRejected,
  DoubleFireExactlyOnce}`; V18 — `TestTopic_V18_ACLLiteralPattern_AndV17First
  EverNoReACL`. Supporting: `TestSeenRing`,
  `TestHandleTopicActionMessage_DedupAndSeqZeroBypass`,
  `TestRedisBroadcaster_ReconnectPreservesPatternSubscriptions`,
  `TestIsPatternTopic`. All green.
- **Gate command:** `PATH=$HOME/go/bin:$PATH bash scripts/pre-commit.sh`
  (GREEN); `-race` scoped to `./pubsub/ ./internal/session/` + root
  `-run 'TestTopic|TestSegmentMatch|TestIsPatternTopic'` (all green).
- **Commit:** local branch `broadcast-redesign-phase-3` (worktree
  `.worktrees/broadcast-redesign-phase-3`); **not committed** — left for user
  signoff per the gate. `phase-3.md` commits in Phase 3's own PR.

## File / commit / V-item pointers
