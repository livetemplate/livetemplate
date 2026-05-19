# BroadcastAction Redesign — Live Progress Tracker

**Canonical plan:** the §"Phased implementation plan (tracker)" section of `docs/proposals/broadcast-action-redesign-proposal.md` (the converged Publish/Subscribe spec, #415; read-only baseline).
**This file:** the writable companion. Each implementation session (one phase at a time — typically a Claude Code session, per the multi-session model this loop is built for) updates it as its **first and last action**. The proposal section never mutates; all execution drift lives here.

---

## Phase Status

| Phase | Status | Owner / session | Started | Completed | Learnings file | Notes |
|---|---|---|---|---|---|---|
| 0 — Foundations (registry + topics.go) | complete | claude-code / 2026-05-17 | 2026-05-17 | 2026-05-17 | `phase-0.md` ✔ | Additive only; gate (non-race + lint) green. ⚠ Phase 1 must resolve the flagged subscribe-after-Unregister race (see `phase-0.md`). |
| 1 — Context API + ACL (single instance) | complete | claude-code / 2026-05-18 | 2026-05-18 | 2026-05-18 | `phase-1.md` ✔ | Gate GREEN (V1–V7,V10–V12,V16,V19,V21; non-race suite + enforced lint 0 issues + scoped `-race`). 5 deviations (3 audit-predicted + `msg.GroupID`-absent + typed `*TopicForbiddenError`); nil-`r` ACL hardening shipped. `BroadcastAction` untouched. **Not committed — awaiting user signoff.** Phase 2 seam: `dispatchToTopic` has no remote leg yet. |
| 2 — Cross-instance (Redis) | complete | claude-code / 2026-05-18 | 2026-05-18 | 2026-05-18 | `phase-2.md` ✔ | Gate GREEN (V8, V9 via 2 RedisBroadcasters on 1 testcontainer; full `scripts/pre-commit.sh` incl. Redis + lint 0 issues + scoped `-race`; Phase-1 V-items regression-green). 6 deviations (registry returns transition-bool + `SubscribedTopics`; root-pkg LIFO disconnect-sweep; `LiveHandler.Publish` not `*Template`; 2 real broadcasters not in-memory; seen-ring → Phase 3; round-4 normalization landed). Benchmarks: fan-out N=1..100 = 0.9µs..99µs; cross-instance 0.96× the GroupActionMessage baseline (no overhead). Inherited #420/#422 blocker was already resolved upstream by #421/#423. `BroadcastAction` untouched. **Not committed — awaiting user signoff.** Phase 3 seam: `b.pubsub` single-PubSub for PSUBSCRIBE; `subscribedPatterns` parallel map; seen-ring folds into `handleTopicActionMessage`. |
| 3 — Wildcards (multi-segment) | not started | — | — | — | `phase-3.md` | Inherits from Phase 2 (see `phase-2.md` "Adjustments"): PSUBSCRIBE on the **same `b.pubsub`** the single `processMessages` pump reads; add `subscribedPatterns map[string]int` ∥ `subscribedChannels` (replay in `reconnect()`); relay branches on `isPatternTopic` (exact-SUBSCRIBE vs PSUBSCRIBE — never expand a pattern); **seen-ring `(instanceID,seq)` dedup lands here** (N=64 bounded, single pump ⇒ no lock) folded into `handleTopicActionMessage` before `GetByTopicExcept` — **two ring constraints (doc'd at the `seq` field, from #424 round-3 review): `seq` monotonic per-instance not per-Type; `seq==0` = pre-upgrade sender ⇒ ring MUST bypass dedup (process unconditionally, no double-fire from pre-Phase-2 instances)**; local strict `segmentMatch` re-match for PSUBSCRIBE over-delivery also folds there; `releaseRelayedTopics`/transition-bool pattern generalizes to patterns as-is. |
| 4 — Client error envelope (parallelizable 1–3) | not started | — | — | — | `phase-4.md` | — |
| 5 — Removal + in-repo/lvt migration | not started | — | — | — | `phase-5.md` | — |
| 6 — Docs + examples + ecosystem | not started | — | — | — | `phase-6.md` | — |

Status values: `not started` → `in progress` → `blocked` → `complete`.

---

## How to use this file

**At the start of your phase:**

1. Read the §"Phased implementation plan (tracker)" + §"Per-phase audit + learnings protocol" sections of `docs/proposals/broadcast-action-redesign-proposal.md`.
2. Read the previous phase's `phase-N-1.md` (Phase 0 has none; Phase 4 reads only `phase-0.md`; Phase 6 reads all). It carries the **adjustments recommended for the next phase** — apply them before doing the phase's own Audit.
3. Run this phase's **Audit (start)** as specified in the proposal. The Audit MAY reshape the task list — that is expected, not a deviation; record what changed.
4. Mark this phase's row `in progress` with your session id + date.
5. Create `phase-N.md` from the template at the bottom of this file.

**At the end of your phase:**

1. Fill `phase-N.md` completely — do not skip "New scope surfaced" even if the answer is "none."
2. Roll every surfaced scope item into the Ledger below.
3. Update the Phase Status row: status → `complete`, fill the completed date + learnings-file check.
4. If you discovered work that changes a later phase's scope, note it in that phase's row so the next session sees it before reading the plan.
5. Do not create mid-session WIP commits without an explicit request. `phase-N.md` is **not** ephemeral: it is committed as that phase's close-out record, in the phase's own implementation PR alongside its code, and stays in git history (it is the durable handoff the next phase's Audit reads). "Leave uncommitted" applies only to in-session drafts before the phase closes.

**If you block:** set status `blocked`, record the blocker in the row, write `phase-N-partial.md` with what was attempted and why it stalled. Once unblocked, set status back to `in progress`, clear/annotate the blocker note, and record the resumption date in the `Notes` column.

---

## Surfaced-Scope & Deferral Ledger

The analog of a running budget: every phase rolls (a) scope surfaced mid-phase and (b) anything that maps onto an Appendix B "Alternatives considered" entry into one visible place. There is **no `Deferred (post-v1)` section** in this spec, and the proposal (including Appendix B) is the read-only baseline — so an item is never "written into Appendix B." It either ships in v1 or is **logged here as reconciled-with-Appendix-B**: Appendix B's text stays frozen; the journal entry below IS the record. Phase 6's exit performs the final pass.

### Known at design time (pre-seeded from the converged spec)

- **Severe footgun — dispatch-symmetry naming hazard.** `Publish(topic, "Delete", …)` invokes the same `Delete` a client-wired `name="Delete"` triggers, on every subscribed peer. v1 hard requirement: `Publish` MUST `slog.Warn` on a collision with a **client-wired** action name. This needs the new small `internal/parse/` wired-name pass, landed in **Phase 1**, gated by **`V19`**. Verify the warning is not silently dropped.
- **Bounded (not severe) — `Publish` is send-side ungated; gate at the caller.** Neither `ctx.Publish` nor `handler.Publish` runs the ACL; the Subscribe-time ACL gates *who reads*. There is **no built-in all-users topic** (`GlobalTopic()` was removed — Appendix B), so no topic reaches the whole user base by construction. The residue is bounded to one identity (`UserTopic`) or exactly the connections the app's own ACL admitted. Design intent: an app-wide announcement is an ordinary developer topic the app must allow in `WithTopicACL` and publish from trusted code. Docs (Phase 6) must surface this as named guidance, not a buried paragraph.
- **Intentional, not new — fan-out backpressure is drop-on-overflow.** `Publish` local fan-out enqueues via `Connection.EnqueueDispatch` (non-blocking, drops on a full per-connection buffer). Identical to existing `BroadcastAction` behavior (not a regression); surfaced by `wsBufferFull` / `wsSlowClientCloses`; tuned via `WithWebSocketBufferSize` / `LVT_WS_BUFFER_SIZE`. Treat as the accepted pre-existing model, not a new problem to solve in this work.
- **Explicitly rejected in Appendix B — do not reintroduce without a logged maintainer decision:** a built-in `GlobalTopic()`/all-users primitive; a `SessionTopic(groupID)` constructor; a `Publish` debounce/coalesce helper; a trie/radix pattern index or glob engine; `GroupID`-field reuse for the topic envelope (a new `Topic` field is used instead).
- **Multi-segment wildcards are IN v1** (Phase 3; Appendix B "single trailing `*` only" was superseded). The matcher is a flat O(P) segment scan by design. The open contingency is whether multi-segment **holds** under implementation pressure — Phase 3's Learnings makes the explicit call; if it is reduced, log the decision + rationale in the journal below (Appendix B stays frozen — the journal entry is the record).

### Surfaced during Phase N (fill in as discovered)

- _Phase 0:_ (1) **⚠ Latent subscribe-after-`Unregister` leak** — `SubscribeConnectionToTopic` has no connection-liveness guard (none needed in Phase 0: no caller). Phase 1's `ctx.Subscribe` exposes it; Phase 1 **must** choose policy (a) registry `<-conn.done` short-circuit (matches `EnqueueDispatch`) or (b) `ctx.Subscribe` error return. See `phase-0.md`. (2) **`GOWORK=off` invariant** for manual go cmds in the nested worktree (pre-commit hook self-handles it — verified `scripts/pre-commit.sh:18,35,154`). (3) **`GetByTopicExcept` gained an injected `match` param** (forced by the `internal/session`→root import-cycle boundary; Phase 1 `dispatchToTopic` passes `segmentMatch`). (4) **Pre-existing `-race` flake `TestRangeBuildLatency_PostPhase7`** — hard wall-clock ceilings, no `-race`/`-short` guard; reproduced identically on clean `main`; unrelated to Phase 0; gate is non-race so unaffected; later phases must scope `-race` to relevant pkgs, not full `./...`. None map onto an Appendix-B alternative (multi-segment wildcards remain IN v1 as the spec intends).
- _Phase 1:_ (1) **nil-`r` ACL hardening (shipped).** Server-originated Contexts (dispatched/server-initiated/upload) inject `r=nil`; the spec's own canonical ACL pattern derefs `r`, so `checkTopicACL` now deny-by-defaults when `r==nil` before calling the hook (SelfTopic ACL-exempt, never reaches it). Phase 4/6 docs: hooks aren't called from server-originated contexts. (2) **WS error envelope is WS-connect-path only**, server-side, Phase 1; keep-open-vs-close (V14) finalization deferred to Phase 4 — envelope contract `{"type":"error","code":"topic_forbidden","topic":…}`. (3) **HTTP-GET ACL-denial → HTTP 500** (no 403 remap; spec doesn't require it) — Phase 6 troubleshooting-docs note. (4) **`ctx.Publish` permissive on `lvt:`** (no SelfTopic-equality; anti-spoof is Subscribe-side §2, send ungated §3) — pinned, record for Phase 4/6. (5) **`lvt-on:` client-side only** → wired-name pass is a statics regex scan, not `internal/parse/` nodes (Phase 6 docs). (6) **`ServerActionMessage` has no `GroupID`** → used per-conn `conn.GroupID` (bot-verifiable correction). (7) **subscribe-after-`Unregister` race = silent-drop** in `SubscribeConnectionToTopic` (`<-conn.done`, mirrors `EnqueueDispatch`; user-confirmed). (8) **PR #419 Claude-bot round (`28b8f6cb`):** fixed 2 functional bugs (upload-complete Publish drain; Mount/OnConnect Publish now fans out per spec §"Publish on GET-phase Mount is not a no-op"), test-race lock, app-global collision-warn dedup, 2 doc caveats; **declined** the pre-existing `processBroadcasts`-on-upload gap (out of scope, `BroadcastAction` untouched until Phase 5); **deferred** a 6-item test-coverage matrix to Phase 2 (bot-framed "not blockers"; see `phase-1.md`). (9) **Round 2/3 converged (no new functional issues):** doc-clarity line, conditional `wiredCollisionWarned` alloc, comment-density trims, 3 cosmetic doc-nits — all addressed; `wsAction`-dup declined (no dup exists). (10) **⚠ Pre-existing flaky `pubsub` `-race`** (`TestReconnect_DoesNotBlockConcurrentPublish`) surfaced on PR #419 cross-repo `-race ./...`; not Phase 1's (0 pubsub files changed; green 3× locally); re-ran the job; **Phase 2 owns the fix** (see Phase 2 row + `phase-1.md`). None map onto an Appendix-B alternative; multi-segment wildcards remain IN v1 (Phase 3).
- _Phase 2:_ (1) **Registry public-surface change (not an Appendix-B item; a correctness requirement).** `SubscribeConnectionToTopic`/`UnsubscribeConnectionFromTopic` now return the per-connection 0→1/1→0 transition `bool`; new `SubscribedTopics(conn) []string` snapshot accessor. Needed for exactly-once cross-instance relay (broadcaster channel refcount is instance-wide; registry subscribe is per-connection idempotent). Phase 3's pattern relay reuses the identical transition-bool + LIFO disconnect-sweep shape. (2) **Disconnect relay teardown is root-package-driven** (`releaseRelayedTopics` deferred after `Unregister` so LIFO runs it while `conn.subscribedTopics` is live) — `internal/session` can't import `pubsub` (cycle), so it can't live in `Unregister`. (3) **Seen-ring dedup deferred to Phase 3** (advisor-confirmed): Phase 2 shipped only the `Topic`+`Seq` wire plumbing + per-instance atomic counter (incremented on every `GroupActionMessage` emit, group-action AND topic, per spec "Seq scope"); the ring is the consumer of the SUBSCRIBE+PSUBSCRIBE double-fire that exists only in Phase 3 (V17). `Seq` round-trip cross-instance-proven by V8/V9. (4) **Phase 1 round-4 normalization fix landed** (was deferred to Phase 2): `canonicalActionKey = toSnakeCase` (the form `methodNameToActions` reduces every style to); closes the "save" vs "Save" false-negative; V19 green (logged value stays the raw action). (5) **Benign pre-existing testcontainer-cleanup log** (`redis.go:146 "client is closed"`) also fires in pre-existing `TestRedisBroadcaster_RefcountSurvivesUnsubscribeUntilZero`; `getTestRedisClient`'s own cleanup vs broadcaster Close ordering — harmless (container reaped regardless). (6) **`LiveHandler.Publish` ≠ `*Template.Publish`** — the spec's "handler" is the value `Template.Handle()` returns. None map onto an Appendix-B alternative; multi-segment wildcards remain IN v1 (Phase 3).
- _Phase 3:_ TBD
- _Phase 4:_ TBD
- _Phase 5:_ TBD
- _Phase 6:_ TBD — final reconciliation logged here (Appendix B stays frozen; this is the record).

---

## Decisions Reaffirmed / Reversed

The proposal body + Appendix B "Alternatives considered" is the immutable baseline. If a phase must reverse or add to a decision (e.g., reintroduce a rejected alternative, or reduce multi-segment wildcards), log it here with date and one-sentence reason; the proposal stays the original baseline, this is the journal.

| Date | Decision reversed / added | New choice | Reason |
|---|---|---|---|
| — | — | — | — |

---

## Per-Phase Learnings File Template

Create each `phase-N.md` from this skeleton (kept identical to the §"Per-phase audit + learnings protocol" copy in the proposal — if you change one, change both):

<!-- INVARIANT: keep this skeleton byte-identical to the copy in the proposal §"Per-phase audit + learnings protocol" — change both together -->
```markdown
# Phase N — Learnings

**Session:** <date / id>   **Status at exit:** complete | partial | blocked
**Plan reference:** Phase N in docs/proposals/broadcast-action-redesign-proposal.md

## What shipped
## Deviations from plan
## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)
## Adjustments recommended for the next phase
## Open questions for the user
## File / commit / V-item pointers
```
