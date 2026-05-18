# Phase 1 — Learnings

**Session:** 2026-05-18 / claude-code   **Status at exit:** complete
**Plan reference:** Phase 1 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live `main` @ `61a44c32` (Phase 0 merged at `fcaa92b2`,
#417; `topics.go` + registry topic infra present). Worktree
`.worktrees/broadcast-redesign-phase-1` off `main`. Note: `main` now includes #418
(`TestRangeBuildLatency_PostPhase7` skipped under `-race`) — the phase-0-flagged latency
flake is resolved upstream; the `-race` gate is simpler than phase-0.md predicted.

**Confirmed reuse targets (grep anchors held):** `topics.go`
(`UserTopic`/`isReservedTopic`/`validateDeveloperTopic`/`segmentMatch`);
`registry.go` `GetByTopicExcept(concrete, excludeConn, match)` (injected matcher),
`SubscribeConnectionToTopic`/`UnsubscribeConnectionFromTopic`/`Unregister` topic-GC/
`EnqueueDispatch` (`<-c.done` template)/`GetByGroupExcept` (copy-contract); `context.go`
`BroadcastAction`/`broadcastRequest`/`broadcasts`/`MaxBroadcastsPerAction`/`WithUserID`;
`template.go` `New`/`Option`/option-apply loop; `mount.go` `processBroadcasts` (post-
`persistState`)/`dispatchBroadcastToGroup`/`handleDispatchedAction`; `groupID` resolved
pre-lifecycle-ctx at mount.go 384 (WS) / 989 (HTTP GET).

**3 spec-vs-code deviations (the audit reshaped the task list — its purpose):**
1. `*Context` has `userID` but **no** `groupID`; `SelfTopic()` anon path needs it →
   add `groupID` + `WithGroupID`, wire at all 7 `WithUserID` sites.
2. `lvt-on:`/`lvt-*` action attrs are **client-side only**, never in the Go parse AST;
   button names surface via `detectSubmitButtonName` + `inputAttrRegex`/`ExtractFormSchema`
   statics regex → the wired-name pass is a **statics HTML scan**, not an `internal/parse/`
   node walk. §"Critical files" "pin to parse node types" reduces to "pin to the exact
   regex/attribute set."
3. Recursion guards (mount.go ~1624 dispatched, ~1986 server-initiated) are **post-hoc
   drop+`slog.Error`**, not flag-based → `Publish`'s guard mirrors that shape (drop
   `topicPublishes` + log at both sites), not a new flag.

**Decisions recorded (Phase 1's call per spec):**
- Subscribe-after-`Unregister` race → **silent-drop in registry** (`<-conn.done` short-
  circuit in `SubscribeConnectionToTopic`, mirroring `EnqueueDispatch`). User-confirmed.
- Server-side WS `{"type":"error","code":"topic_forbidden","topic":…}` envelope is **IN
  Phase 1** (lives with ACL-denial logic; Phase 4 = pure-TS consumer; V14 stays Phase 4).
- `ctx.Publish` is **permissive** on `lvt:` topics (no `SelfTopic()`-equality check):
  anti-spoof is spec-scoped to `Subscribe`; §3 send-side is ungated; publishing to an
  identity topic only delivers to that identity's own subscribers (no read access).

## What shipped

Single-instance Publish/Subscribe topic model wired onto Phase 0's substrate.
`BroadcastAction` and its group-dispatch path are **untouched** (still fully
functional, as required through Phases 0–4).

- **`context.go`** — `groupID` field + `WithGroupID`/`GroupID()`; `SelfTopic()`
  (`lvt:user:<UserID>` / `lvt:session:<GroupID>`; both-empty → `slog.Error` +
  `""`; anon → one `slog.Debug`); `topicSub`/`topicPubs` fields.
- **`topic_context.go`** (new) — the `topicSubscriber` injected interface;
  `WithTopicSubscriber`; `Subscribe` (exact 3-step order: reserved-namespace
  exact-self → developer grammar → ACL); `Unsubscribe`; `Publish`
  (permissive-`lvt:`, no ACL; cap; symmetry-collision `slog.Warn`);
  `pendingTopicPublishes`.
- **`topics.go`** — `sessionTopic` (unexported; no public `SessionTopic()`);
  `ErrTopicForbidden` sentinel + `*TopicForbiddenError{Topic,Cause}` (carries
  the topic for the WS envelope; `Is`→`ErrTopicForbidden`, `Unwrap`→Cause).
- **`topic_runtime.go`** (new) — `liveTopicSubscriber` (handler-bound, mirrors
  `localSession`): `checkTopicACL` (OpenTopics / hook / deny-all; **nil-`r`
  deny-by-default hardening**), `registerTopic`/`unregisterTopic` (no-op when no
  Connection), `isClientWiredAction`; `topicSubscriberFor`; the
  `{"type":"error","code":"topic_forbidden","topic":…}` envelope +
  `sendTopicForbiddenEnvelope`.
- **`topic_wired_actions.go`** (new) — `extractWiredActionNames` (statics regex
  scan: submit-capable `<button>`/`<input>` `name=` + `lvt-on:<event>="X"`),
  `Template.isClientWiredAction`; cached on `*Template.wiredActions` beside
  `formSchema`, shared into clones under `t.mu`.
- **`template.go`** — `TopicACLFunc`, `WithTopicACL`/`WithOpenTopics`;
  `Config.TopicACL`/`OpenTopics`; **both-set hard error at `New()`** after the
  option loop (order-independent); `mountConfig` carries the two fields;
  `wiredActions` field + parse-time population + clone sharing.
- **`internal/session/registry.go`** — `<-conn.done` liveness short-circuit in
  `SubscribeConnectionToTopic` (silent-drop policy); `DispatchKind`/`KindAction`
  + `DispatchRequest.Kind` (zero-valued, backward-compatible).
- **`mount.go`** — `processTopicPublishes`/`dispatchToTopic` (twin of
  `dispatchBroadcastToGroup`; `GetByTopicExcept(…, segmentMatch)`; cross-instance
  leg deferred to Phase 2, no stub); drain at both post-`persistState`
  `processBroadcasts` sites; `Publish` recursion guard mirrored at **both**
  guard sites (drop+`slog.Error`, not a flag); `WithGroupID` +
  `WithTopicSubscriber` injected at all 7 `WithUserID` sites; WS-connect
  Mount-failure emits the envelope via `errors.As(*TopicForbiddenError)`.
- **`topic_test.go`** (new) — 11 Tier-1 integration tests; `captureSlog` tee;
  `perDeviceAuth`; reuses `broadcast_test.go` helpers (`connectWS`,
  `readWSUpdate`, `fixedGroupAuth`).

**Gate (Phase 1 = V1–V7, V10–V12, V16, V19, V21): GREEN.** All 11
`TestTopic_*` pass; full non-race suite green (root `ok 111.8s`); enforced
golangci-lint (errcheck,govet,ineffassign,staticcheck,unparam,unused) **0
issues**; `-race` clean on `internal/session` (32.4s) and the topic/broadcast
tests (3.7s); no `BroadcastAction` regression (existing suite unchanged-green).
Single-instance only; no Redis.

## Deviations from plan

1. **`*Context` had no `groupID`** (predicted "carried on the `*Context`").
   Added `groupID` + `WithGroupID`; wired at the 7 `WithUserID` sites.
2. **`lvt-on:` is client-side only — `internal/parse/` was the wrong location.**
   The wired-name pass is a **statics regex scan** parallel to
   `ExtractFormSchema` (`topic_wired_actions.go`), not a parse-AST walk. The
   §"Critical files" "pin to parse node types" requirement is realized as
   "pin to the exact regex/attribute set" (submit `<button>`/`<input>` `name=`
   + `lvt-on:`). Adjustment for Phase 6 docs.
3. **Recursion guards are post-hoc drop+`slog.Error`, not flags.** `Publish`'s
   guard mirrors that shape (drop `topicPublishes` + log at both sites). The
   guard was **adapted, not reused verbatim** (a parallel block, same shape).
4. **Server-action site `msg.GroupID` does not exist.** `ServerActionMessage`
   has no `GroupID`; the plan's prediction was wrong. Used per-connection
   `conn.GroupID` (matches the adjacent `connState{groupID: conn.GroupID}` and
   `WithSession(...conn.GroupID)`). Bot-verifiable factual correction.
5. **`ErrTopicForbidden` is a typed `*TopicForbiddenError`**, not a bare
   sentinel — needed to carry the topic for the WS envelope. `errors.Is(…,
   ErrTopicForbidden)` still holds (canonical Mount pattern unaffected).

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

- **nil-`r` ACL hardening (audit-driven, shipped).** Server-originated
  Contexts (dispatched / server-initiated / upload) inject `r=nil`. The spec's
  own canonical ACL pattern dereferences `r` (`r.Header.Get("Upgrade")`), so
  invoking the hook with nil `r` would panic. `checkTopicACL` now deny-by-
  defaults when `r==nil` before calling the hook (SelfTopic() is ACL-exempt and
  never reaches it). Phase 4/6 docs: hooks are not called from server-originated
  contexts; a developer-topic Subscribe there is denied by design.
- **HTTP-GET ACL-denial surfaces as HTTP 500** (`mount.go` maps any Mount error
  → `http.Error(…, 500)`). Spec does not require a 403 remap; recorded so
  Phase 6 troubleshooting docs do not promise 403.
- **WS error envelope: emitted on the WS-connect path only.** The WS-action
  re-Mount path and HTTP-GET path do not emit it (spec wire-format note scopes
  it to WS-connect). Phase 1 keeps the existing close-on-Mount-error behavior;
  whether the socket should instead stay open (V14) is **Phase 4's call** — the
  envelope shape `{"type":"error","code":"topic_forbidden","topic":…}` is the
  server-emitted contract Phase 4's audit must consume.
- **`ctx.Publish` is permissive on `lvt:` topics** (no `SelfTopic()`-equality
  check) — pinned decision; anti-spoof is Subscribe-side only (§2), send-side
  ungated (§3). Record for Phase 4/6.
- None map onto an Appendix-B alternative. Multi-segment wildcards remain IN v1
  (Phase 3 owns them; Phase 1's matcher use is `segmentMatch` general-case).

## Adjustments recommended for the next phase

Phase 2 (Cross-instance / Redis) inherits a clean single-instance seam:

- `dispatchToTopic` (`mount.go`) has **no remote leg** — Phase 2 adds it after
  the local `EnqueueDispatch` loop, modeled on `dispatchBroadcastToGroup`'s
  `PublishGroupAction` block (deliberately not stubbed with a no-op call).
- `DispatchRequest.Kind` exists (`KindAction`, zero); `dispatchToTopic` sets it
  explicitly. Forward-compat placeholder ready if Phase 2 needs to branch.
- `GroupActionMessage` (`pubsub/types.go`) is **not yet** extended — Phase 2
  adds `Topic string` + `Seq uint64` (`(instanceID,seq)` dedup, **not**
  `timestamp`); persist-before-publish ordering already holds (drain is at the
  post-`persistState` `processBroadcasts` site).
- `handler.Publish` (out-of-band, no Context) is **not** implemented (Phase 2,
  V8). Phase 1's `ctx.Publish` queues onto the Context; the out-of-band entry
  point is a separate handler method Phase 2 adds.
- The injected `topicSubscriber` interface is the extension seam; Phase 2's
  cross-instance relay is handler-side (`liveTopicSubscriber`/`dispatchToTopic`)
  and needs no Context-API change.

**Deferred Phase 2 test coverage (from the PR #419 Claude-bot review, accepted
as Phase-2-scoped — the bot itself framed these "not blockers for Phase 1"):**
Phase 1's gate is the V-item set (all green); the following are tracked here
rather than expanded in Phase 1:
- `ctx.Publish` with an invalid topic (grammar-rejection path)
- `ctx.Publish` at the `MaxBroadcastsPerAction` cap
- `ctx.Unsubscribe` (no direct test yet)
- the WS `{"type":"error","code":"topic_forbidden"}` envelope end-to-end
  (server emission shipped Phase 1; client consumption is Phase 4 / V14)
- server-originated Subscribe with nil `r` hitting the deny-by-default
  hardening
- `ctx.Publish` from an upload-complete handler (the drain fixed in
  `28b8f6cb`; an upload-machinery e2e is Phase 2 scope)

The post-review functional fixes (`28b8f6cb`): upload-complete Publish
drain; Mount/OnConnect Publish drains (spec §"Publish on the GET-phase Mount
is not a no-op"); test-race lock; app-global collision-warn dedup
(`Template.wiredCollisionWarned`); two doc caveats. The pre-existing
`processBroadcasts`-absent-on-upload-complete gap was **declined** (out of
scope — `BroadcastAction` is untouched until Phase 5).

**⚠ Pre-existing flaky `-race` data race in `pubsub` (Phase 2 MUST address).**
The cross-repo CI job "Test LVT against Core Changes" runs `go test -race
./...`, which on one PR #419 run flagged `WARNING: DATA RACE` →
`--- FAIL: TestReconnect_DoesNotBlockConcurrentPublish`
(`pubsub/redis_test.go:1156`). **Not introduced by this PR:** `git diff
main...HEAD` changes **0** `pubsub/` files; the test was last touched by
unrelated #389/#394/#398; reproduced **green 3× locally** under `-race`
(intermittent — a timing/interleaving-dependent catch). Directly analogous to
the `TestRangeBuildLatency_PostPhase7` pre-existing `-race` flake Phase 0
documented (handled upstream by #418's skip-under-race). Handled here by
**re-running the flaky job** (pass) — not by modifying `pubsub`, which is
Phase 2's package and out of Phase 1's registry/topics/context scope (matches
Phase 0's accepted "scope `-race` per phase-touched packages; don't gate on
full `-race ./...`" guidance). **Phase 2 adjustment:** Phase 2 owns `pubsub`
and its `-race` gate — fix or `t.Skip`-guard
`TestReconnect_DoesNotBlockConcurrentPublish` there (it is a real, if rare,
race in the reconnect-vs-concurrent-publish path), and scope the Redis-leg
`-race` to `pubsub`/`session` rather than relying on the cross-repo
`-race ./...`.

## Open questions for the user

- **None blocking.** One decision was surfaced and pinned without asking
  (advisor-endorsed): the WS error envelope ships in Phase 1 server-side; V14
  (client) stays Phase 4. The keep-socket-open-vs-close finalization is
  explicitly deferred to Phase 4 (V14 owner). Flagged here for the Phase 4
  audit, not a blocker.
- PR creation / commit is left to the user per the signoff gate (no push/PR
  without explicit signoff after manual testing).

## File / commit / V-item pointers

- **New files:** `topic_context.go`, `topic_runtime.go`,
  `topic_wired_actions.go`, `topic_test.go`.
- **Modified:** `context.go`, `topics.go`, `template.go`, `mount.go`,
  `internal/session/registry.go`.
- **V-items:** V1/V3/V10 (`TestTopic_V1_V3_V10_SelfSyncTwoDevices`), V2, V4, V5,
  V6, V7, V11, V12, V16, V19, V21 — all green (Tier-1). Tier-2 chromedp
  (user-visible legs of V1–V3) is the release-wave cross-repo step, not Phase
  1's merge gate.
- **V21 note:** `fixedGroupAuth` makes a re-dial the same identity by
  construction (constant groupID, no cookie) — the spec-allowed "same-session
  reconnect helper" reduced to a plain `connectWS` re-dial; no new helper
  needed.
- **Commit:** local branch `broadcast-redesign-phase-1` (worktree
  `.worktrees/broadcast-redesign-phase-1`); **not committed** — left for user
  signoff per the gate. `phase-1.md` is committed in Phase 1's own PR.
- **Gate command:** `GOWORK=off go test ./... -timeout=300s` +
  `GOWORK=off golangci-lint run --enable-only=errcheck,govet,ineffassign,staticcheck,unparam,unused`
  (both green); `-race` scoped to `internal/session` + topic tests (green;
  full `-race ./...` not gated, per phase-0 — `#418` already skips the
  latency flake on `main`).
