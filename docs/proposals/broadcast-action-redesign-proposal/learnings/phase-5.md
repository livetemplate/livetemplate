# Phase 5 — Learnings

**Session:** 2026-05-20 / claude-code   **Status at exit:** in progress
**Plan reference:** Phase 5 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live code. Inputs read in protocol order: the
proposal §"Phase 5" bullet + §"Per-phase audit + learnings protocol"; then
`phase-1.md` through `phase-4.md` and `progress.md` Phase 5 row (which
already records the Phase 4 additions to Phase 5 scope — committed lvt
`replace` to resolve into a real pin + revert lvt e2e's local-bundle handler
→ `e2etest.ServeClientLibrary` now that `@livetemplate/client@0.9.2` is on
npm). Phase 5 is the removal-+-migration phase; the prior `phase-N.md`
adjustments-for-next-phase confirmed Phases 0–4 are purely additive and
`BroadcastAction` remained fully functional through v0.9.2.

Worktrees: `<livetemplate>/.worktrees/broadcast-redesign-phase-5` off `main`
@ `ab855777 chore(release): v0.9.2`; `<lvt>/.worktrees/broadcast-redesign-phase-5`
off `main` @ `d2ffa33 chore(scripts): fix pre-commit lint invocation (#328)`.
`.worktrees/` gitignored in both. client repo unchanged in Phase 5 (the
removal is server-side only; client's `lvt:error` consumer ships unchanged
at v0.9.2 on npm).

### User-decided up-front

- **Lint baseline**: separate PR first (chose option A) — landed as
  [lvt#329](https://github.com/livetemplate/lvt/pull/329); Phase 5 lvt
  commits land on a clean baseline.
- **Removal release version**: **minor v0.9.2 → v0.10.0** (pre-1.0 convention
  for breaking change; client + lvt will follow with their own v0.10.0
  alignment).
- **Removal cadence**: **hard removal in one release** (no deprecation
  cycle) — matches the proposal's atomic-removal constraint.

### Live-code Audit findings

**(1) Authoritative `.BroadcastAction(` invocation sweep (in livetemplate):**

```
grep -rn '\.BroadcastAction(' . --include='*.go' | grep -v '\.worktrees'
```

13 invocation lines across **5 files** (the proposal's predicted set held — 5
production-call files — but with a higher per-file call count than the
proposal's 1-each assumption):

| File | Calls |
|---|---|
| `broadcast_test.go` | 3 (lines 47, 62, 308) |
| `context_broadcast_test.go` | 7 (lines 11, 12, 37, 61, 67, 84, 93) |
| `e2e/docker/app/main.go` | 1 (line 98) |
| `lifecycle_integration_test.go` | 1 (line 171) |
| `navigate_test.go` | 1 (line 233) |

Plus **1 godoc-comment ref** in `context.go:650` (an example inside the
`ctx.BroadcastAction` doc comment itself; will be deleted with the method).

Authoritative count handed to Phase 6's V20 sweep: `grep -rn
'\.BroadcastAction(' . --include='*.go'` must return **zero** matches
post-migration. The full-symbol `grep -rn 'BroadcastAction'` sweep (the V20
gate's broader form) must also return zero across the impacted repos, excluding
proposal docs + changelog.

**(2) Full `BroadcastAction` grep surface — 14 files (vs proposal's
predicted ~11 — drift recorded):**

- 5 call-site files (above) — call sites get migrated to
  `Subscribe(SelfTopic()) + Publish + reconciler`
- 9 implementation / comment files:
  - `context.go` — defines `BroadcastAction` + `broadcastRequest` struct +
    `broadcasts` field + `pendingBroadcasts()` + `MaxBroadcastsPerAction`
    const + 1 godoc example
  - `handle_test.go` — holds `TestEphemeral_BroadcastActionStillWorks` (no
    direct call; **proposal asks for repurposing** — see Deviations)
  - `mount.go` — the group-dispatch path: `processBroadcasts` (line 1614),
    `dispatchBroadcastToGroup` (line 1622), 2 call sites that drain
    `actionCtx.pendingBroadcasts()` (lines 906, 1436), 2 recursion-guard
    blocks with BroadcastAction-named slog.Error messages (lines 1787–1790,
    2202–2209), 5 BroadcastAction-naming comment refs
  - `session_impl.go` — 2 comment refs (lines 27, 60)
  - `internal/session/registry.go:87` — 1 comment ref
  - `pubsub/types.go` — 3 comment refs (lines 85, 213, 238)
  - `pubsub/redis.go:995` — 1 comment ref
  - `topic_context.go:157` — 1 comment ref ("same shallow-copy footgun as
    `ctx.BroadcastAction`, per...")
  - `topic_test.go:226` — 1 comment ref ("where `BroadcastAction` cannot")

**(3) Group-dispatch path is the only-caller-of structurally:** confirmed.
`processBroadcasts` is called only at the 2 sites in `mount.go` that drain
`actionCtx.pendingBroadcasts()`. Once `pendingBroadcasts()` is gone (with
the `broadcasts []broadcastRequest` field), both call sites disappear, and
`processBroadcasts` → `dispatchBroadcastToGroup` cascade to dead code. The
`PublishGroupAction` pubsub interface method (called only by
`dispatchBroadcastToGroup`'s cross-instance leg) is **also** dead — Phase 5
removes it from `pubsub/types.go` + `pubsub/redis.go`. The
`GroupActionMessage` type **stays** (topic Publish still uses it for the
cross-instance topic channel; `pubsub/types.go:238`'s "group_action" vs
"topic_action" distinction is preserved with only the topic side remaining).

**(4) lvt 3-file surface — confirmed (live grep `../lvt internal/serve/`):**

- `internal/serve/websocket.go:30` — `type WebSocketManager` def +
  `Broadcast(data)` method at line 123
- `internal/serve/server.go:266` — the one call site:
  `s.wsManager.Broadcast(map[string]interface{}{"type":"reload","path":path})`
- `internal/serve/websocket_test.go` — 3 test functions:
  `TestWebSocketManager_CreateAndClose` (line 14), `TestWebSocketManager_Broadcast`
  (line 28), `TestWebSocketManager_MultipleClients` (line 75)

lvt's `WebSocketManager.Broadcast()` is **NOT** a `BroadcastAction` consumer
— it's lvt's internal dev-server reload-broadcast over its own WS, unrelated
to livetemplate's API. The rename to `ReloadClients()` is a clarity rename
(post-Phase-5 the word "broadcast" without context is ambiguous; lvt's
specific use case is "reload connected dev-mode clients on file change").

lvt's `go.mod` currently pins `require github.com/livetemplate/livetemplate
v0.9.1` — needs bump to **v0.10.0** (Phase 5's removal release). Because
v0.10.0 doesn't exist yet at the time the lvt Phase-5 branch is created,
the same committed-`replace`-then-resolve pattern from Phase 4 applies:
initially `replace github.com/livetemplate/livetemplate =>
../../../livetemplate/.worktrees/broadcast-redesign-phase-5`, converted to
the real pin once the livetemplate v0.10.0 release ships.

**(5) lvt#327 (the Phase-4 draft V14 e2e PR) integration:** Phase 5
deliverable from `phase-4.md` is to revert lvt#327's `serveLocalPhase4ClientBundle`
→ `e2etest.ServeClientLibrary` now that `@livetemplate/client@0.9.2` is on
npm. **Decision: lvt#327 is superseded by the Phase-5 lvt PR** rather than
updated in place. The Phase-5 lvt branch will cherry-pick / re-author the
V14 e2e file with `ServeClientLibrary` directly, plus the pin bump + rename.
Close lvt#327 with a "superseded by Phase-5 PR" comment at merge time.

## What shipped

**livetemplate (this PR, atomic single commit):**

- API surface removed from `context.go`:
  - `broadcastRequest` struct
  - `Context.broadcasts []broadcastRequest` field
  - `Context.BroadcastAction(action, data)` method (+ godoc)
  - `MaxBroadcastsPerAction` const (RENAMED, see Deviations — not deleted)
  - `Context.pendingBroadcasts()` internal method
- Dead group-dispatch removed from `mount.go`:
  - `processBroadcasts(groupID, excludeConn, broadcasts)` (line 1614)
  - `dispatchBroadcastToGroup(groupID, excludeConn, action, data)` (line 1622)
  - 2 drain call sites: WS action loop (line 906) and HTTP POST action path (line 1436)
  - 2 BroadcastAction-named pendingBroadcasts() drop blocks in handleDispatchedAction (lines 1787–1794) and the server-initiated action loop (lines 2202–2214)
  - Comment-ref rewording at 5 sites (875–876, 1656–1657, 1667–1672, 1683–1687, 1756–1766, 2153–2164, 2733–2738)
- Constant rename + relocation: `MaxBroadcastsPerAction` → `MaxPublishesPerAction`, moved from `context.go` to `topic_context.go` (it caps `Publish`, not the now-removed BroadcastAction). Updated 3 use sites in `topic_context.go` and 5 references in `topic_phase2_coverage_test.go`.
- Documentation rewording for removed-API refs (no behavior change): `session_impl.go` (2 sites), `internal/session/registry.go` (1), `pubsub/types.go` (3), `pubsub/redis.go` (1), `topic_context.go` (3), `topic_test.go` (1).
- Test migrations (every controller using BroadcastAction now uses Subscribe(SelfTopic())+Publish):
  - `broadcast_test.go`: `broadcastTestController` (Mount adds Subscribe; Increment/SetMessage use Publish), `syncController` (Mount adds Subscribe; Add uses Publish). Renames: `TestHTTPPost_BroadcastAction_*` → `TestHTTPPost_Publish_*`, `TestWSAction_BroadcastAction_*` → `TestWSAction_Publish_*`, `TestBroadcastAction_ExplicitRefreshDispatchesToPeers` → `TestPublish_ExplicitRefreshDispatchesToPeers`. Removed: `noSyncController` + `TestBroadcastAction_NoAutomaticPeerDispatch` (the new model has no "auto" path to disprove).
  - `lifecycle_integration_test.go`: `flashBroadcastController` (Mount adds Subscribe; Bump uses Publish for PeerSync). Renamed `TestBroadcastAction_PeerCanSetFlash` → `TestPublishToSelfTopic_PeerCanSetFlash`. Section header reworded.
  - `navigate_test.go`: `navigateBroadcastTestController` (Mount adds idempotent Subscribe; re-Mount path uses Publish for RefreshGreeting). Comment-block rewording on test + controller godoc.
  - `e2e/docker/app/main.go`: `ChatController.Mount` adds `Subscribe(SelfTopic())`; `Send` uses `Publish(SelfTopic(), "RefreshMessages", nil)`.
  - `handle_test.go`: renamed `TestPerConnectionState_NoAutoBroadcast` → `TestPerConnectionState_NoAutoFanOut`; reworded comment; deleted `TestEphemeral_BroadcastActionStillWorks` (no replacement — the equivalent assertion is now provided by the topic-Publish tests in broadcast_test.go).
- Test deletions (tested removed-API internals):
  - `context_broadcast_test.go` — entirely deleted (covered `broadcasts` field, `pendingBroadcasts()`, `MaxBroadcastsPerAction` cap behavior on the now-removed queue; the cap on the new Publish queue is independently covered by `TestTopic_Phase2_PublishCap` in `topic_phase2_coverage_test.go`).
- `CLAUDE.md`: the "BroadcastAction ordering" caveat replaced with a "Publish ordering" caveat + a new "Peer fan-out is opt-in" caveat that makes the Subscribe+Publish requirement explicit.

**Cross-repo (deferred to follow-up PRs after livetemplate v0.10.0 release):**

- `client` repo: no change in Phase 5; the `lvt:error` consumer shipped in v0.9.2 is forward-compatible.
- `lvt` repo: separate PR (#329 — already merged for the lint baseline). The Phase-5 lvt PR opens after this livetemplate PR merges and tags v0.10.0; it bumps the `go.mod` pin (initially via `replace` per the Phase-N-resolved pattern), renames `WebSocketManager.Broadcast() → ReloadClients()` (a clarity rename — lvt's internal dev-server reload broadcaster is not a livetemplate BroadcastAction consumer), and integrates the V14 chromedp e2e from lvt#327 with `serveLocalPhase4ClientBundle` reverted to `e2etest.ServeClientLibrary`. lvt#327 is closed as superseded.

## Deviations from plan

The Phase 5 audit (Audit findings section above) was wrong in **three** places. Each is recorded here so the proposal's "audit was wrong, here's what we found" contract holds.

1. **`PublishGroupAction` kept — NOT removed.** Audit §"(3) Group-dispatch path is the only-caller-of structurally" claimed `PublishGroupAction` was called only by `dispatchBroadcastToGroup`'s cross-instance leg, making it dead after Phase 5 removal. **The audit missed `session_impl.go:180`**, where `localSession.TriggerAction` calls `gab.PublishGroupAction(s.groupID, action, payload)` directly for the cross-instance fan-out of server-initiated actions (`Session.TriggerAction`). `PublishGroupAction` stays in the `pubsub.GroupActionBroadcaster` interface and in `pubsub/redis.go`. The interface and its docs were reworded to frame the method as backing `Session.TriggerAction`, not the removed `BroadcastAction`.

2. **`MaxBroadcastsPerAction` renamed, NOT deleted.** Audit §"(2) Full BroadcastAction grep surface" listed the constant as a BroadcastAction-only artifact slated for deletion. **The audit missed that Phase 4's Publish path reuses the same constant**: `topic_context.go:177` checks `len(c.topicPubs) >= MaxBroadcastsPerAction` to cap per-action Publish calls. Deleting the constant would silently uncap the Publish path. **Decision**: rename to `MaxPublishesPerAction` and relocate the declaration to `topic_context.go` (right next to `Publish` where it actually applies). The old name was a Phase-4 carryover that became misleading the moment Publish started reusing it; Phase 5 fixed the naming. Note: `MaxBroadcastsPerAction` is exported — the rename is itself a breaking API change for any external code referencing the constant. This is documented as part of the v0.10.0 breaking-change footprint.

3. **`topic_phase2_coverage_test.go` missed by the audit.** Audit §"(1) Authoritative `.BroadcastAction(` invocation sweep" identified 5 call-site files. **The audit's full-symbol sweep missed `topic_phase2_coverage_test.go`** (6 references to `MaxBroadcastsPerAction` in comments and test code, including the cap-overflow test loop at line 64 and the error-message assertion at line 112). All 6 references updated to `MaxPublishesPerAction` via the rename.

**Word-boundary regex blind spot (process learning, recorded for future phases):** the initial sweep used `grep -rn '\bBroadcastAction\b'` to find identifier-level refs. Underscore is a `\w` character in POSIX/PCRE regex, so `\bBroadcastAction\b` does NOT match `TestBroadcastAction_PeerCanSetFlash`. The substring grep `grep -rn 'BroadcastAction'` catches it; the word-boundary version doesn't. Used the substring form for the final zero-match verification.

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

- **`MaxBroadcastsPerAction` is an exported breaking-change rename.** Pre-1.0 we don't carry deprecation aliases, but the change should be called out in the v0.10.0 release notes alongside the `BroadcastAction` removal.
- **`pubsub.GroupActionBroadcaster` interface stays.** The proposal's removal-list implied this interface was dead; in fact it backs `Session.TriggerAction`'s cross-instance fan-out, which is independent of the removed `BroadcastAction`. Phase 6's V20 sweep should NOT flag `GroupActionBroadcaster` or `PublishGroupAction` as drift — they are live API surface.

## Adjustments recommended for the next phase

- **Phase 6 (V20 zero-hits sweep):** the authoritative grep is `grep -rn 'BroadcastAction\|broadcastRequest\|pendingBroadcasts\|MaxBroadcastsPerAction\|processBroadcasts\|dispatchBroadcastToGroup' --include='*.go' . | grep -v '\.worktrees'`. Excluding `docs/proposals/`, `CHANGELOG.md`, and v0.10.0 release notes (which mention the removal by name), this MUST return zero matches across livetemplate + client + lvt.
- **lvt Phase-5 PR (next):** uses the same committed-`replace`-then-resolve pattern as Phase 4. The lint baseline is already on a clean baseline (lvt#329 merged), so the Phase-5 lvt diff is purely the go.mod pin bump + the `WebSocketManager.Broadcast` → `ReloadClients` rename + the V14 e2e integration.
- **Release ordering:** livetemplate v0.10.0 ships first (this PR after maintainer signoff), then the lvt Phase-5 PR converts the `replace` to a real pin and ships lvt v0.10.0. Client stays at v0.9.2 (no changes needed in Phase 5).

## Open questions for the user

- None blocking. The pre-Phase-5 questions (lint baseline cadence, removal version, deprecation cadence) were resolved up-front in the "User-decided up-front" section above.

## File / commit / V-item pointers

- Worktree: `livetemplate/.worktrees/broadcast-redesign-phase-5` (off `main` @ `ab855777 chore(release): v0.9.2`)
- Atomic commit: `2703d1d0 feat(broadcast)!: Phase 5 — remove ctx.BroadcastAction; migrate call sites to Subscribe/Publish self-topic` (18 files, +370/-466).
- Pull request: livetemplate#429 (opened 2026-05-20, off `main` @ `ab855777` v0.9.2).
- V-item map (proposal §V-items):
  - V20 (BroadcastAction zero-hits sweep): **landed in this PR** — substring grep across the repo returns zero matches; Phase 6 inherits a clean canvas.
  - V21 (Subscribe(SelfTopic())+Publish migration parity): **landed** — every controller previously using BroadcastAction now uses the canonical opt-in pattern; tests cover WS + HTTP + e2e Docker.
- Phase 5 row in `progress.md` flipped from `in_progress` → `complete` in this PR.
