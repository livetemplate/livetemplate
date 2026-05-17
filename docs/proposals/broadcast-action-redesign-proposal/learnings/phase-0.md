# Phase 0 — Learnings

**Session:** 2026-05-17 / claude-code   **Status at exit:** complete
**Plan reference:** Phase 0 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live code (worktree `broadcast-redesign-phase-0`,
branched from `main` @ `19774be3`). No prior phase to read.

- **No symbol collision.** `grep -rn 'byTopic\|byTopicPattern\|segmentMatch' internal/session/`
  → none. `topics.go` does not exist at repo root. Safe to add.
- **`ConnectionRegistry`** (`internal/session/registry.go:319-325`): fields
  `byGroup`, `byUser`, `mu sync.RWMutex`, `metrics`, `dispatchBufferSize`.
  `NewConnectionRegistry()` (`:330-335`) initializes `byGroup`/`byUser`.
- **`Unregister()`** (`registry.go:392-426`) is the **sole** index-cleanup path.
  Removes from `byGroup`/`byUser` under `r.mu.Lock()` via `removeConnection`,
  deletes emptied map keys, then calls `conn.Close()`.
- **Risky assumption — confirmed.** `Connection.Close()` (`registry.go:182`) only
  signals writePump shutdown + closes the socket; it does **not** mutate registry
  indices and does **not** call `Unregister()`. The slow-client path
  (`Send()` → `go c.Close()`, ~`registry.go:146`) closes the socket but index
  removal still flows through `Unregister()` later. **No third path bypasses
  `Unregister()`** → topic-map cleanup wired there is necessary and sufficient.
- **`removeConnection(conns, target)`** (`registry.go:572`): reused verbatim for
  the topic-map slice removal.
- **`DispatchRequest`** (`registry.go:69-72`): `{ Action string; Data map[...] }`.
  No `Kind` field today (see Deviations — deferred).
- **Surface is `UserTopic` only.** No `SessionTopic`/`GlobalTopic` references in
  the tree to mirror (spec §5 / Appendix B forbid them).

The audit did **not** reshape the task list — the plan's predicted reuse targets
and grep anchors all held against live code.

## What shipped

Pure-additive infrastructure; **no existing dispatch/broadcast path touched**
(`BroadcastAction` fully functional, as required through Phases 0–4).

- **`internal/session/registry.go`**
  - `ConnectionRegistry.byTopic` + `byTopicPattern map[string][]*Connection`;
    initialized in `NewConnectionRegistry()`.
  - `Connection.subscribedTopics map[string]struct{}` — per-connection
    membership set; the GC root `Unregister()` walks. Guarded by the existing
    `ConnectionRegistry.mu` (no new lock).
  - `SubscribeConnectionToTopic` / `UnsubscribeConnectionFromTopic` /
    `GetByTopicExcept` + unexported `isPatternTopic` helper. Reuses
    `removeConnection`; mirrors the `GetByGroupExcept` copy contract and the
    `byGroup`/`byUser` empty-slice-delete idiom.
  - `Unregister()` topic-GC: walks `conn.subscribedTopics`, removes from
    `byTopic`/`byTopicPattern` by `*`-presence, deletes emptied keys, nils the
    set. O(t), inside the existing locked cleanup section.
- **`topics.go`** (new, package `livetemplate`) — `UserTopic` (only identity
  constructor), `isReservedTopic`, `validateDeveloperTopic`,
  `isValidSegmentChar`, `segmentMatch` (general-case, multi-`*`).
- **Tests** — `internal/session/registry_topic_test.go` (6 tests, injected
  test matcher, `-race`); `topics_test.go` (`TestUserTopic`,
  `TestIsReservedTopic`, `TestValidateDeveloperTopic`, and `TestSegmentMatch`
  with 27 edge-case rows = the executable matcher spec). All green with
  `-race`. Phase 0's gate is unit-tests-only ("No e2e yet" per the plan —
  nothing is wired to exercise; e2e arrives with behavior in Phases 1–3).

**`segmentMatch` authored by the assistant** at the user's explicit request
(the learning-mode contribution was offered; the user replied "Implement
segmentMatch"). Split-and-compare; the `len()`-equality check is the
topic-isolation boundary (bounds `*` to exactly one segment); `* vs ""` guarded
explicitly so the matcher is total, not dependent on the upstream validator.

## Deviations from plan

- **`DispatchRequest.Kind` deferred to Phase 1.** The spec lists it as *optional*
  ("Optionally add a `Kind` field … forward-compatible placeholder"). A
  zero-valued field with no consumer is dead code in Phase 0 (no dispatch path
  branches on it until Phase 1). **Decision: add it in Phase 1 when it has a real
  consumer.** Phase 1's audit should add `Kind` (single `KindAction`,
  zero-valued, backward-compatible) to `DispatchRequest` at that time.

- **`GetByTopicExcept` signature gained a `match` parameter (forced by the
  package boundary).** The proposal §"Critical files" describes
  `GetByTopicExcept(concrete, excludeConn)` resolving the union with
  `segmentMatch`. But `segmentMatch` lives in `topics.go` (package
  `livetemplate`) and `internal/session` cannot import the root package
  (import cycle — the `Connection` `interface{}` fields exist precisely to
  avoid it). Realized as
  `GetByTopicExcept(concrete string, excludeConn *Connection, match func(pattern, concrete string) bool)`
  — the matcher is **dependency-injected** by the caller. This is a faithful
  realization of the spec's intent (the union is still
  `byTopic ∪ {pattern : match}`), not a semantic change. `match` is **required**
  — passing nil with pattern subscribers present panics by design (loud
  programmer-error, never a silent exact-only degradation). **Phase 1
  consequence:** `dispatchToTopic` must call
  `registry.GetByTopicExcept(concrete, excl, segmentMatch)`.

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

- **`GOWORK=off` is mandatory for *manual* go commands in any phase's worktree
  (recurs every phase).** The user's global rule places worktrees at
  `<repo>/.worktrees/<feature>`, physically nested under the main module. A
  shared workspace `/home/adnaan/code/livetemplate/go.work` lists `./livetemplate`
  (the main checkout) — *not* the nested worktree — as its sole relevant `use`
  entry, so a hand-run `go build|vet|test` from the worktree resolves packages
  against the **outer** module and fails (`main module … does not contain
  package …/.worktrees/…`). **The pre-commit hook is NOT affected** —
  `scripts/pre-commit.sh` already prefixes every go invocation with `GOWORK=off`
  (verified: lines 18 `go fmt`, 35 `golangci-lint`, 154 `go test`), so the gate
  needs no change. **Adjustment for every later phase's Audit:** prefix manual
  go commands with `GOWORK=off`; the hook handles itself. Environment invariant
  of the nested-worktree workflow, not a code change.
- **Matcher dependency-injection across the `internal/session` ↔ root package
  boundary** (see Deviations → `GetByTopicExcept`). Surfaced as a design point
  Phase 1 inherits: the root package owns `segmentMatch`; the registry takes it
  as a parameter. No trie/radix index (spec §2) — flat O(P) scan retained.
- **Pre-existing `-race` flake: `TestRangeBuildLatency_PostPhase7`
  (NOT introduced here; reproduced on untouched `main`).** A full
  `go test -race ./...` fails this one root-package test:
  `range_build_latency_test.go:107` asserts hard wall-clock ceilings
  (N=1000 → 50ms, N=10000 → 250ms) with **no `-race`/`-short` guard**. Under
  the race detector's instrumentation those medians become ~109ms and ~1.04s
  (≈2–4× — the documented race-detector overhead profile). **Verified
  pre-existing:** the same test fails identically on a clean `main` checkout
  (no Phase 0 changes) — `median=109.148571ms ceiling=50ms`,
  `median=1.070363473s ceiling=250ms` — i.e. a structural race-vs-wall-clock
  artifact in unrelated streaming-range ("Phase 7") code, with **zero** code
  path from `topics.go` / the registry topic methods. `internal/session`
  itself passed `-race` (31s). The **pre-commit gate is non-race**
  (`go test -v ./... -timeout=300s`) and passes this test and the whole suite.
  Out of Phase 0 scope to fix (the plan's scope guard limits Phase 0 to
  registry/topics; modifying a prior project's latency test is scope creep).
  **Adjustment for later phases:** do not gate a phase on a *full*
  `-race ./...` run; scope `-race` to the concurrency-relevant package(s) the
  phase touches (Phase 0: `internal/session` — passed) and rely on the non-race
  300s suite for the gate. Phase 2 (Redis) especially: scope `-race` to
  `pubsub`/`session`. Candidate standalone follow-up (not this wave): guard the
  latency test with `if testing.Short() || raceEnabled { t.Skip(...) }`.

No Appendix-B alternative was reintroduced. Multi-segment wildcards remain
**IN v1** (the general-case `segmentMatch`/`byTopicPattern` shipped as planned —
Phase 3's risk valve can still narrow *only* `validateDeveloperTopic` if needed,
exactly as the spec's contingency intends).

## Adjustments recommended for the next phase

- **Per-connection membership vs. Redis ref-count are different layers (do not
  conflate).** Phase 0's `Connection.subscribedTopics` is an idempotent **set**
  (membership; double-subscribe = one entry, single unsubscribe clears). Phase 2's
  `subscribedChannels map[string]int` in `pubsub/redis.go` is a **ref-count**
  (many distinct connections on one instance → one Redis `SUBSCRIBE`; a
  multiplexing concern). Phase 1/2 audits must keep these distinct.
- **`Subscribe` (Phase 1) wires the validators Phase 0 delivered, in this order:**
  (1) `isReservedTopic(topic)` → if true, admit **only** on exact
  `topic == ctx.SelfTopic()` (a one-liner at the call site — no extra helper
  needed; `isReservedTopic` is the reusable classification half); reject every
  other `lvt:` string. (2) else `validateDeveloperTopic(topic)` (segment
  grammar). (3) else the ACL (deny-all default; only `SelfTopic()` exempt). The
  grammar is **never** applied to `lvt:` topics (it excludes `:`).
  **Layering (must hold):** validation runs **in `ctx.Subscribe`, before** the
  `registry.SubscribeConnectionToTopic` call — the registry methods deliberately
  do **not** validate (reserved identity topics like `UserTopic`/`SelfTopic`
  legitimately bypass `validateDeveloperTopic`; validating inside the registry
  would wrongly reject them and double-validate developer topics). The registry
  is the index; `ctx.Subscribe` is the gate.
- **Bare `*` is spec-permitted (confirmed against the proposal — not a gap).**
  A single-segment `*` pattern passes `validateDeveloperTopic` and matches any
  *single-segment* concrete topic. Proposal §2 "Grammar": segments may be `*`
  "any number of times" — one `*` segment is valid. It is **bounded** (does not
  match multi-segment topics — count mismatch) and **ACL-gated** (deny-all
  default; a bare-`*` developer subscribe still needs `WithTopicACL`/
  `WithOpenTopics`). Adding a validator rejection would *contradict the spec
  grammar* and pre-empt Phase 3's valve (which narrows the validator only for
  non-trailing/multiple `*`, never bare `*`). No change; recorded so Phase 1
  and reviewers don't re-litigate.
- **⚠ Phase 1: guard empty-`UserID` reserved topics.** `UserTopic("")` →
  `"lvt:user:"` (the constructor is intentionally pure — proposal §5; no Phase 0
  change). Phase 1's `ctx.Subscribe`/`SelfTopic()` MUST reject an empty-`UserID`
  reserved subscribe before it reaches the registry, or `"lvt:user:"` could
  match across anonymous connections. The spec already has the adjacent
  invariant (proposal §1: anonymous `SelfTopic()` is `lvt:session:<GroupID>`,
  never empty; an empty `SelfTopic()` from a misimplemented Authenticator is
  logged `slog.Error`) — Phase 1 must additionally cover the out-of-band
  `UserTopic("")` vector at the `Subscribe` gate.
- **`dispatchToTopic` (Phase 1) must inject the matcher:**
  `registry.GetByTopicExcept(concrete, excludeConn, segmentMatch)`. The registry
  cannot reach `segmentMatch` itself (import cycle) — passing it is the caller's
  responsibility; nil + pattern subscribers = panic by design. *Optional
  (Phase 1's call):* a named wrapper `func topicMatch(p, c string) bool { return
  segmentMatch(p, c) }` at the single call site gives audits a stable grep
  anchor; passing `segmentMatch` directly is also fine (it is itself a named,
  greppable symbol). Phase 1 decides — recorded so the option isn't lost.
- **⚠ Latent race Phase 1 MUST resolve: subscribe-after-Unregister leak.**
  `Unregister()` sets `conn.subscribedTopics = nil` and drops the conn from all
  indexes. `SubscribeConnectionToTopic` has **no** connection-liveness guard
  (Phase 0 needs none — no caller). Once Phase 1's `ctx.Subscribe` calls it from
  action-handler code, a subscribe that races a WS drop (handler still running
  when the socket closes) will lazily re-create `subscribedTopics` and re-insert
  a **dead** conn into `byTopic`/`byTopicPattern` — a permanent leak Topic-GC
  cannot reclaim (the conn will never `Unregister` again). `r.mu` serializes the
  maps but does **not** order this against connection lifecycle. The existing
  `EnqueueDispatch` defends the analogous race with a `select { case <-c.done: }`
  check. **Phase 1 must choose and implement one policy** (Phase 0 deliberately
  does not — adding it now would be behavior with no caller):
  **(a)** registry-level `<-conn.done` short-circuit in
  `SubscribeConnectionToTopic`, silent-drop, matching `EnqueueDispatch`; or
  **(b)** an error return surfaced from `ctx.Subscribe` so the controller learns
  the subscription was refused. (a) is the lower-surprise match to existing
  registry behavior; (b) is more honest if Mount-time subscribe failure should
  be observable. Record the choice in `phase-1.md`.
- **Add the deferred `DispatchRequest.Kind`** (single `KindAction`, zero-valued,
  backward-compatible) in Phase 1 when the dispatch path has a real consumer.
- **Prefix manual go commands with `GOWORK=off`** in the worktree (see New
  scope). The pre-commit hook already self-handles this — do **not** add
  `GOWORK` handling to the hook. The gate is `GOWORK=off go test -v ./...
  -timeout=300s` (**no `-race`**); use `-race` separately for the
  concurrency-sensitive registry code as Phase 0 did (it passed).

## Open questions for the user

- **None blocking.** Non-blocking note recorded for transparency: gopls suggests
  `strings.SplitSeq` over `strings.Split` (the `stringsseq` modernize hint).
  Kept `strings.Split` — it is clearer for a security-boundary function and is
  **not** in the enforced pre-commit linter set
  (`errcheck,govet,ineffassign,staticcheck,unparam,unused`); staticcheck has no
  such check, so the gate is unaffected. Revisit only if project style adopts
  the iterator forms repo-wide.
- Phase 0 closes with a single local commit on branch
  `broadcast-redesign-phase-0`; **PR creation is left to the user** (not
  auto-pushed), per the harness "commit/push only when asked" rule and the plan.

## File / commit / V-item pointers

- **Code:** `internal/session/registry.go` (topic indexes, 3 methods,
  `isPatternTopic`, `Unregister` GC, `Connection.subscribedTopics`),
  `topics.go` (new).
- **Tests:** `internal/session/registry_topic_test.go` (new, 6 tests),
  `topics_test.go` (new, incl. 27-row `TestSegmentMatch`).
- **Tracker:** this file + `progress.md` (Phase 0 row → complete; Ledger
  "Surfaced during Phase 0" + this file's New-scope items).
- **V-items:** Phase 0 has **no gating `V`-item** — its gate is the
  registry+helper unit tests (all green with `-race`). `V1`–`V21` begin at
  Phase 1. Phase 0 is the substrate they build on.
- **Commit:** single Phase 0 close-out commit on branch
  `broadcast-redesign-phase-0` (SHA recoverable via
  `git log broadcast-redesign-phase-0`; a file cannot embed its own commit's
  SHA). Pre-commit hook green; never `--no-verify`.
- **Gate (non-race, = pre-commit):** `GOWORK=off go test ./... -timeout=300s`
  all green; root package `ok 109.3s` (well within 300s — proves additive-only,
  no regression). `golangci-lint` (errcheck,govet,ineffassign,staticcheck,
  unparam,unused): **0 issues**.
- **Full `-race` suite (`-timeout=600s`):** completed in 524s (no timeout);
  all packages green **except** the pre-existing
  `TestRangeBuildLatency_PostPhase7` latency flake documented under "New scope
  surfaced" — verified identical on clean `main`, unrelated to Phase 0,
  out-of-scope, and green in the non-race gate. `internal/session` (the only
  concurrency-relevant Phase 0 code) passed `-race` (31s). No data races. The
  earlier 240s `-race` FAIL was this same suite hitting the *tighter* timeout;
  at 600s it ran to completion and isolated the single pre-existing flake.
