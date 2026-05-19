# Phase 4 — Learnings

**Session:** 2026-05-19 / claude-code   **Status at exit:** complete
**Plan reference:** Phase 4 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live code. Inputs read in protocol order: the
proposal §"Verification plan"/§"Test tiers & harness"/§"Phased implementation
plan" Phase 4 bullet + §"Per-phase audit + learnings protocol"; then
`phase-0.md` (Phase 4's primary input — the agreed error-envelope shape; Phase 0
is pure substrate, the envelope shape itself is the proposal/V14 + Phase 1's
server emission); then `phase-1.md` (server-side WS-connect envelope + the
**explicitly-deferred keep-open-vs-close decision**) and `phase-3.md`
Adjustments + progress.md Phase 4 row (both confirm **Phase 3 made NO
server-emitted-envelope change**). `phase-2.md` skimmed — no client-contract
bearing.

Worktrees `<repo>/.worktrees/broadcast-redesign-phase-4` off `main` in all
three repos (`.worktrees/` gitignored in each — verified: livetemplate
`.gitignore:52`, client `.gitignore`, lvt `.gitignore:45`). **lvt was on
`docs/cross-link`; its worktree is branched off `main` explicitly**
(`e9f6628`). livetemplate `main` @ `6b8d0f2d` (Phase 3, #426). client `main`
@ `b00ef39`.

### (a) Client dispatch structure — `../client`

- **`handleWebSocketPayload`** (`client/livetemplate-client.ts:364`) is a
  sequence of `(response as any).type === "<x>"` / shape early-return checks —
  `upload_progress` (`:370`), `upload_start` (`:378`), `upload_complete`
  (`:390`) — **then** the diff/update path.
- **The diff/update path this change MUST NOT touch:** the
  `if (this.wrapperElement) { … this.changeAutoWirer.analyzeStatics(response.tree);
  this.updateDOM(this.wrapperElement, response.tree, response.meta); …
  dispatchEvent(new CustomEvent("lvt:updated", …)) }` block at
  `livetemplate-client.ts:415–436`. The new branch is an early `return`
  **before** line 415 (mirrors the `upload_progress` shape at `:370`).
- **Wire entry:** `transport/websocket.ts:227` `JSON.parse(event.data)` as
  `UpdateResponse` → `onMessage(payload,event)` → `handleWebSocketPayload`.
  `UpdateResponse` (`types.ts:33`) = `{ tree: TreeNode; meta?: ResponseMetadata }`
  — the error envelope `{type,code,topic}` has **no `tree`**, so reaching the
  diff path would call `analyzeStatics(undefined)` — the early return is
  load-bearing, not cosmetic.
- **Canonical CustomEvent idiom (reuse, do not invent):**
  `this.wrapperElement.dispatchEvent(new CustomEvent("lvt:<name>",
  { detail: {…} }))` — `lvt:upload:error` (`:223`), `lvt:updated` (`:427`).
  Phase 4 emits `lvt:error` with `detail:{code,topic}`.
- **No client-initiated `Subscribe` RPC exists** (grep `subscribe`/`Subscribe`
  in `livetemplate-client.ts` → none). V14's "TS client `Subscribe(...)`" is
  shorthand for "the Go controller's Mount calls `ctx.Subscribe(...)`, the ACL
  denies it, the server emits the envelope, the client surfaces it." No
  client→server subscribe message; the change is purely a new inbound-message
  branch.
- **jest suite gating V14's logic leg:** `client/tests/` (jest, `roots:
  ['<rootDir>/tests']`, `testMatch **/*.test.ts`, `jest-environment-jsdom`).
  `tests/websocket.test.ts` tests the *transport* layer (`MockWebSocket`), not
  `handleWebSocketPayload`. The V14 logic-leg test is **new**, exercising
  `LiveTemplateClient.handleWebSocketPayload` end-to-end (feed the error
  envelope, assert `lvt:error` CustomEvent on the wrapper, assert the diff path
  was not entered). Client conventions govern (no CLAUDE.md; `CONTRIBUTING.md`
  + `.git/hooks/pre-commit`: lint-if-present (none) → `npm test` (jest
  `--no-cache`) → `npm run build` (`tsc` + esbuild); Conventional Commits;
  client `CONTRIBUTING.md` §"Protocol Changes" mandates CHANGELOG + coordinate
  release with core lib).

### (b) Keep-open-vs-close — RESOLVED: a coordinated livetemplate server change IS required (in-scope for Phase 4)

This is the Audit's task-list-reshaping finding (the protocol's explicit
purpose). The chain, pinned to live code:

1. The `topic_forbidden` envelope is emitted **only** when `callMount` returns
   a `*TopicForbiddenError`: `mount.go:670` `newState, err := callMount(...)`;
   `:674` `var tfe *TopicForbiddenError; if errors.As(err,&tfe) {
   h.sendTopicForbiddenEnvelope(connection, tfe.Topic) }`.
2. That same error path then `:681` **`return`**s. The function has
   `defer h.registry.Unregister(connection)` at `mount.go:535`; `Unregister`
   calls `conn.Close()` (phase-0.md, registry.go:392–426). **So the envelope
   path closes the WebSocket.** This is the "Phase 1 keeps the existing
   close-on-Mount-error behavior" phase-1.md flagged for Phase 4
   (`topic_runtime.go:26–29` comment says verbatim: finalizing whether the
   socket should stay open is "V14 / Phase 4's call").
3. Client `transport/websocket.ts:49` — `if (!this.manuallyClosed &&
   this.options.autoReconnect) this.scheduleReconnect()`; default
   `autoReconnect` on, `maxReconnectAttempts: 10`, exponential backoff.
4. ⇒ envelope → server closes → client reconnects → re-dial same identity →
   re-Mount → same denied `ctx.Subscribe` → envelope → close → … **reconnect
   storm**, connection permanently dead after 10 attempts.

V14 requires the envelope **AND** "WS stays open" — **unsatisfiable on the
Mount path with current server behavior**. The Tier-1 V5 path (`topic_test.go`
`aclProbeController.Sub` — subscribe *in an action*) keeps the WS open but
**does not emit the envelope** (phase-1.md: envelope is WS-connect-path only).
So no existing path yields both. **Decision: Option B server change is
in-scope for Phase 4** (the close-on-`*TopicForbiddenError` is, in effect, a
latent reconnect-storm bug; keeping the socket open is the correct product
behavior and is exactly what V14 encodes). This is **not** the
`BroadcastAction` path (untouched until Phase 5) and **not** the
diff/update path.

**Option B (chosen) vs Option A (rejected), with the sharper rationale:**
Option A = emit the envelope at the denial site inside
`liveTopicSubscriber`/`checkTopicACL` and keep the Mount-error sink as-is.
Rejected because it would make V14 a *controller-pattern contract* (the
developer must both propagate the error *and* the framework emits at the
denial site). Option B keeps emission at the existing Mount-error sink
(`mount.go:674`) so V14 is a clean **framework guarantee**: "controller
returns `*TopicForbiddenError` from Mount on the WS-connect path → envelope +
socket stays open, period." That is the contract Phase 6 docs state.

**Pinned post-failure lifecycle (Option B — continue as if Mount returned
`(newState, nil)`):** on `errors.As(err,&tfe)` at `mount.go:674`, after
`sendTopicForbiddenEnvelope`: **(i)** adopt `newState` from `callMount`
(`callLifecycleMethod` returns the controller's first return value; for
`return s, err` that is `s`); **(ii)** `h.persistState(...)`; **(iii)** drain
`lifecycleCtx.pendingTopicPublishes()` (publishes the controller queued
*before* the denied Subscribe must not be silently dropped); **(iv)**
`callOnConnect`; **(v)** send the initial tree; keep the WS open. Only the
**non-`TopicForbidden`** Mount-error path retains the `return`/close. The
controller-swallows-error case (`_ = ctx.Subscribe("denied"); return s, nil`)
emits **no** envelope (Mount returns nil) — acceptable for V14 (spec scenario
is the propagated error) but **must be documented** so Phase 6 docs do not
overpromise.

### V14 dual-tier mapping (proposal §"Test tiers & harness")

- **Tier 1 (logic, client jest)** — gates V14's client logic leg in
  `../client`. New jest test: error envelope in → `lvt:error` CustomEvent
  `{code:"topic_forbidden",topic:"private/admin"}` out, diff path untouched.
- **Tier 2 (user-visible, lvt chromedp e2e)** — `lvt` `e2e/`. Real server
  with `WithTopicACL` denying `private/admin`; controller Mount subscribes it
  & propagates the error; assert the browser sees the `lvt:error` CustomEvent
  **and the WS stays usable** (advisor sharpening: "stays open" must mean
  "stays *functional*" — send an action over the same socket post-envelope and
  assert a normal diff returns, not merely "socket not closed in 50ms").
  Harness MUST capture+surface on failure all four: browser console logs,
  server logs, WS frames, rendered HTML (verify reusable helpers in lvt
  `e2e/`; `RecordWSFrames` exists per lvt#317 — confirm console/server-log/
  OuterHTML capture or scope adding them).

### Server-emitted envelope (pinned, byte-for-byte — Phase 4's contract input)

`topic_runtime.go:17` `type topicErrorEnvelope struct { Type string
\`json:"type"\`; Code string \`json:"code"\`; Topic string \`json:"topic"\` }`
marshaled at `:34` as `{Type:"error", Code:"topic_forbidden", Topic:<topic>}`.
**On the wire (exact):**

```json
{"type":"error","code":"topic_forbidden","topic":"<denied topic>"}
```

The shipped `lvt:error` CustomEvent MUST agree: `detail.code` ⇐ `code`,
`detail.topic` ⇐ `topic`. Pinned both sides at exit.

### Cross-repo commit/PR placement (resolved per the proposal's ambiguity)

The proposal says "`phase-N.md` … committed in the phase's own implementation
PR alongside its code." Phase 4's code is cross-repo. **Resolution:**
`phase-4.md` + `progress.md` are livetemplate-repo files **and** Phase 4's
server change lands in livetemplate — so they commit in the **livetemplate**
PR, alongside `mount.go` + the Tier-1 test. The client TS change + jest is a
separate **client** PR. The V14 chromedp e2e is a separate **lvt** PR. Three
coordinated PRs; **release order:** livetemplate (server) and client (TS) are
wire-independent (the envelope shipped in Phase 1); the **lvt** e2e is the
gating last step (consumes both). All three left local — **no push/PR/merge
without explicit user signoff after manual test** (PR signoff gate).

## What shipped

Three coordinated repo changes, each gated through that repo's own real gate.
All three branches are `broadcast-redesign-phase-4` in `<repo>/.worktrees/` —
**left local, awaiting user signoff (no push/PR/merge)**.

- **livetemplate (Option B keep-open server change + Tier-1 regression).**
  - `mount.go` (the WS-connect Mount block): inverted the `errors.As` check
    so the **non**-`*TopicForbiddenError` Mount-error path keeps the existing
    `slog.Error("Mount failed")`+`return` (byte-identical close behavior for
    genuine server faults), and the `*TopicForbiddenError` path sends the
    envelope, logs `slog.Warn("Mount Subscribe denied by topic ACL;
    surfaced to client, connection kept open", topic=…, error=…)`, then
    **falls through** to the shared success-path lifecycle (adopts the
    controller's returned `newState`, `persistState`, `OnConnect`,
    `processTopicPublishes` drain, initial-tree send, read loop). The
    connection stays open and functional.
  - `topic_runtime.go`: updated the `sendTopicForbiddenEnvelope` doc comment
    (lines 23–29 previously said Phase 1 "keeps the existing close-on-Mount-
    error behavior; finalizing whether the socket should instead stay open
    is V14 / Phase 4's call") to reflect the now-shipped keep-open behavior
    and the no-disconnect-storm consequence. The `topicErrorEnvelope` struct
    + JSON tags are unchanged.
  - `topic_test.go`: new `TestTopic_V14_MountDenyEmitsEnvelopeAndKeepsConnectionOpen`
    (raw `websocket.DefaultDialer.Dial` — not `connectWS`, which assumes
    first-frame == initial render — to read frame 1 = envelope, frame 2 =
    initial render, then send `bump` action and assert PONG in the diff).
    Added `encoding/json` to imports.
- **client (`type==="error"` branch + jest V14 logic leg).**
  - `livetemplate-client.ts`: new **first-discriminator** early-return in
    `handleWebSocketPayload` — typed local cast `errorEnvelope` to
    `{type?, code?, topic?}`; on `type==="error"` dispatches
    `new CustomEvent("lvt:error", { detail: { code, topic } })` on
    `this.wrapperElement` and `return`s before the diff path. Inline comment
    documents the name overlap with `state/form-lifecycle-manager.ts`'s
    form-level `lvt:error` (different target, disjoint detail shape,
    non-bubbling — no functional collision; pinned per spec per advisor).
  - `tests/topic-error-envelope.test.ts`: 4 jest tests gating V14's logic
    leg — (a) exact `{code, topic}` detail on the wrapper for a
    `topic_forbidden` envelope; (b) **target-specific** (capture-phase
    `document` listener also catches it via capture, document bubble
    listener doesn't — advisor sharpening on target-specificity);
    (c) diff path NOT entered (`updateDOM` spied: not called; no
    `lvt:updated`; DOM untouched); (d) over-match guard — a normal
    `UpdateResponse` still flows to `updateDOM` and does NOT fire
    `lvt:error`.
- **lvt (V14 Tier-2 chromedp e2e + committed go.mod `replace`).**
  - `e2e/topic_acl_error_envelope_v14_test.go` (`//go:build browser`,
    `package e2e_test`): full **4-artifact capture** harness — console via
    the shared `installConsoleLogger` (lifecycle_ergonomics_test.go, same
    package; streamed live); server logs via `e2etest.NewServerLogger()`
    teed into the global `slog` default with **save+restore in
    `t.Cleanup`** (advisor parallel-safety); WS frames via
    `e2etest.RecordWSFrames`; rendered HTML via `chromedp.OuterHTML("html")`
    in a `dump()` closure called **before** any `t.Fatalf` (chrome ctx is
    cancelled by `defer cleanup()` before `t.Cleanup` runs, so the HTML
    must be captured at failure-time, not in cleanup). Controller Mount
    uses an **`IsInitialMount()` guard** (the Phase-4-Audit deviation, see
    "Deviations from plan" item 3 below). Page template installs a
    **capture-phase** `document` listener for `lvt:error` in `<head>`
    BEFORE `client.js` loads, capturing into `window.__lvtErrors`
    (race-free even though the wrapper-dispatched event is non-bubbling —
    capture phase observes the dispatch regardless of `bubbles:false`).
    Local Phase-4 client bundle served (NOT
    `e2etest.ServeClientLibrary` — that fetches from the unpkg CDN and
    the 1h disk cache can shadow `LVT_CLIENT_CDN_URL`, so an unreleased
    client cannot be reliably served via the canonical path).
  - `go.mod`: committed
    `replace github.com/livetemplate/livetemplate => ../../../livetemplate/.worktrees/broadcast-redesign-phase-4`
    (with an inline comment naming Phase 5 as its resolution). The lvt
    Phase-4 branch is **intentionally not independently CI-runnable until
    Phase 5's pin bump** — this is exactly the proposal's documented
    "lvt e2e gates last (consumes both)" release order, NOT a skipped test.
- **Tracker:** this file (`phase-4.md`); `progress.md` Phase 4 row flipped
  → `complete` (companion edit in this session, as protocol's
  "session's first and last action"); Ledger updated with the surfaced
  scope below; Phase 5 + Phase 6 rows seeded.

### Server-emitted envelope vs shipped `lvt:error` agreement (byte-for-byte pinned)

| Side | Source | Wire / event detail |
|---|---|---|
| **Server** | `topic_runtime.go:17–21,34` `topicErrorEnvelope{Type:"error",Code:"topic_forbidden",Topic:topic}` marshaled to `{"type":"error","code":"topic_forbidden","topic":"<denied topic>"}` | `type=error`, `code=topic_forbidden`, `topic=<denied>` |
| **Client** | `livetemplate-client.ts` new branch dispatches `new CustomEvent("lvt:error", { detail: { code: errorEnvelope.code, topic: errorEnvelope.topic } })` on `wrapperElement` | `detail.code=<server.code>`, `detail.topic=<server.topic>` |

Both Tier-1 (livetemplate `topic_test.go`: `env.Type=="error"` &&
`env.Code=="topic_forbidden"` && `env.Topic=="private/admin"`) **and**
Tier-2 (lvt e2e: `window.__lvtErrors[0].code==='topic_forbidden'` &&
`window.__lvtErrors[0].topic==='private/admin'`) assert these values
byte-for-byte against the canonical V14 spec strings. The client jest
suite asserts the same on a synthetic envelope. Three-tier agreement
pinned.

### Keep-open-vs-close resolution + that it pulled in a livetemplate server change

**YES — Phase 4 pulled in a coordinated livetemplate server change.**
Documented in the Audit; verified by the V14 Tier-1 + Tier-2 gates. The
chain (live-code pinned): server emits envelope **only** on `*TopicForbiddenError`
(`mount.go` `errors.As`); the same path *previously* returned → deferred
`Unregister` → `Connection.Close()` → WS closed; client auto-reconnects
on close (`transport/websocket.ts:49`, default on, 10 attempts) ⇒ envelope
→ close → re-Mount → re-deny → reconnect storm → permanent dead
connection. Option B's fall-through preserves the envelope and keeps the
socket open; the V14 Tier-2 e2e proves the WS round-trips a subsequent
action (the "stays *functional*, not merely un-closed" advisor sharpening).

### V14 status

| Tier | Where | Result |
|---|---|---|
| Tier 1 — Go integration (logic, server) | livetemplate `topic_test.go` `TestTopic_V14_…` | **GREEN** (0.00s; full `scripts/pre-commit.sh` green incl. fmt + golangci-lint `errcheck,govet,ineffassign,staticcheck,unparam,unused` **0 issues** + full `go test ./... -timeout=300s` incl. Redis testcontainers + scoped `-race` on touched root pkg 11.99s) |
| Client logic leg (jest) | client `tests/topic-error-envelope.test.ts` | **GREEN** (4/4 V14 tests pass; full client jest suite green: 29 suites, 551 tests, 100% pass) |
| Tier 2 — chromedp browser e2e (user-visible) | lvt `e2e/topic_acl_error_envelope_v14_test.go` (`//go:build browser`) | **GREEN** (1.57s end-to-end against locally-built client bundle + livetemplate Phase-4 worktree via committed `replace`; 4-artifact capture wired + surfaced — the initial failed run dumped server logs + WS frames + HTML and caught the IsInitialMount missing-guard) |

### jest deviations

None functional. The CONTRIBUTING.md gate is fmt-if-present (no linter
configured) + `npm test` + `npm run build`; my changes added a `tsconfig`-
clean TS branch and a test file that mirrors the `navigation.test.ts`
private-method-via-`(client as any)` access pattern. The single name overlap
with `state/form-lifecycle-manager.ts`'s form-level `lvt:error` is
documented inline + here; no rename (the advisor's call: spec-pinned name,
non-functional collision, two distinct events by target and detail shape).

## Deviations from plan

1. **Phase 4 was scoped in the proposal as a client-only change** (the row:
   "Client error envelope (`../client`, parallelizable with 1–3)"; the body:
   "`client` TS: `type === "error"` branch in `handleWebSocketPayload`"
   only). The Audit reshaped this (its protocol-mandated purpose) into a
   **3-repo coordinated change**: livetemplate server change (Option B
   keep-open) + Tier-1; client TS branch + jest; lvt Tier-2 chromedp e2e +
   committed go.mod `replace`. The reshape is grounded in V14's own dual-
   tier mapping in §"Test tiers & harness" (the lvt Tier-2 "WS stays
   open" leg is unsatisfiable without the livetemplate change — proven in
   the Audit by the auto-reconnect-storm chain).
2. **The committed go.mod `replace` in the lvt worktree** is a Phase-4
   artifact that didn't exist in the proposal. It is the only path that
   compiles the V14 e2e against the unreleased Phase-0..4 livetemplate
   (`./livetemplate` in go.work is Phase-3, not Phase-4). Made the lvt
   Phase-4 branch intentionally not-independently-mergeable — which matches
   the proposal's "lvt e2e gates last (consumes both)" release ordering.
   **Phase 5 resolves it** (its `lvt go.mod pin bump` deliverable now
   literally means "convert this `replace` into a real version pin").
3. **`IsInitialMount()` guard on the V14 e2e controller's Mount.** The
   proposal's V14 wording is "denied `Subscribe` in WS-connect Mount"; in a
   real browser flow (HTTP GET → page render → WS connect), Mount runs on
   BOTH paths, and on HTTP GET a denied `ctx.Subscribe` surfaces as HTTP
   500 (pre-existing per phase-1.md "HTTP-GET ACL-denial → HTTP 500";
   `mount.go` HTTP-GET path unchanged by Phase 4). Without the guard, the
   page would 500 before `client.js` ever loaded and there'd be no WS for
   V14 to exercise. The guard is the canonical Mount() pattern from
   livetemplate `CLAUDE.md` and scopes the denial to exactly V14's spec
   path (the WS-connect Mount). The first failed e2e run surfaced this via
   the 4-artifact dump — a small validation that the harness works as
   intended.
4. **Local-bundle handler instead of `e2etest.ServeClientLibrary`** in the
   lvt V14 e2e. The canonical helper fetches the published client from the
   unpkg CDN and caches on disk for 1h; that cache can shadow
   `LVT_CLIENT_CDN_URL`, so an unreleased client cannot be reliably served
   via the canonical path. Phase 5/6 swaps it back after client publish.

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

1. **Keep-open requires a livetemplate server change** (the central Audit
   reshape) — recorded in the Ledger as Phase 4's defining scope expansion.
   Not an Appendix-B alternative; not a deferral. Shipped in Phase 4.
2. **`lvt:error` name overlap with `state/form-lifecycle-manager.ts`'s
   form-level event** — different target (wrapper vs `<form>`), disjoint
   detail shape (`{code, topic}` vs `ResponseMetadata`), both non-bubbling.
   No functional collision. Pinned per spec (advisor call). Phase 6 docs
   must distinguish.
3. **Cross-repo `replace` is the Phase-5-resolved artifact pattern.** Any
   future cross-repo dev work that introduces an API on the dependee before
   the dependency publishes follows this exact shape: committed `replace`
   in the dependent repo's worktree go.mod, runtime/build guard comment
   pointing to the resolution phase, e2e/build proven green locally, no
   independent CI/merge until the pin bump.
4. **Pre-existing lvt `scripts/pre-commit.sh` lint-flag bug**
   (`golangci-lint --disable-all` was removed in v2; the script's lint step
   silently fails at flag-parse, meaning the lvt lint gate has been
   effectively non-enforced for some time — 84+ pre-existing lint issues
   were waiting on the floor). **Confirmed pre-existing on lvt main
   (line 36)**; not introduced by Phase 4. Out of Phase 4 scope. Verified
   my Phase-4 e2e file is lint-clean (0 issues attributable to it) by
   running v2 with the correct `--default=none --enable=…` flags. Flagged
   for a separate one-line script fix in Phase 5/6.
5. **lvt CDN client + 1h disk-cache shadowing** of `LVT_CLIENT_CDN_URL`
   means the canonical `ServeClientLibrary` path is unsuitable for testing
   unreleased client changes. The local-bundle pattern (Deviation 4) is
   the right approach for this specific class — generalize the lesson if
   a similar future test needs it.
6. **Mount runs on every HTTP request AND WS connect** (livetemplate
   `CLAUDE.md`) — controllers that subscribe a topic the ACL may deny must
   guard the Subscribe with `IsInitialMount()` (or `IsReconnect()`/
   `IsConnected()`) or the HTTP-GET path 500s before the WS can exercise
   V14's keep-open. **Phase 6 docs deliverable.**
7. **Controller-swallows-error case: no envelope emitted** (Mount returns
   nil → server treats it as success → no envelope path → no `lvt:error`).
   V14's contract is on the **propagated-error** scenario. Acceptable per
   spec but **Phase 6 docs must surface** to avoid overpromising.
8. **HTTP-GET denial behavior remains HTTP 500** (Phase 1 recorded; Phase 4
   only changes the WS-connect path's close→keep-open). Pin in Phase 6
   troubleshooting docs to avoid confusion ("I get HTTP 500 on the page,
   not the lvt:error CustomEvent" → "the lvt:error CustomEvent fires on the
   WS-connect Mount denied-Subscribe path; HTTP GET denied-Subscribe is a
   pre-existing HTTP 500, see IsInitialMount guard").

None map onto an Appendix-B alternative; multi-segment wildcards remain
**IN v1** (Phase 3 sealed; Phase 4 unrelated to wildcard semantics).

## Adjustments recommended for the next phase

**Phase 5 (Removal + in-repo/lvt migration):**
- **Resolve the committed lvt go.mod `replace` into a real version pin.**
  The grep anchor is exactly `replace github.com/livetemplate/livetemplate
  => ../../../livetemplate/.worktrees/broadcast-redesign-phase-4` (with the
  inline comment naming Phase 5). Phase 5's `go.mod pin bump` deliverable
  is literally this conversion (plus the `WebSocketManager.Broadcast()` →
  `ReloadClients()` migration the proposal already lists).
- **Revert the lvt V14 e2e to `e2etest.ServeClientLibrary`** once
  `@livetemplate/client` is published with the Phase-4 `lvt:error` branch.
  The local-bundle handler `serveLocalPhase4ClientBundle` is the swap
  point.
- **Phase-4 livetemplate touched no `BroadcastAction` call sites**, so
  Phase 5's authoritative invocation sweep
  `grep -rn '\.BroadcastAction(' . --include='*.go'` is unaffected by
  Phase 4. The new `*TopicForbiddenError` keep-open path is purely on the
  topic Subscribe/ACL surface — orthogonal to the BroadcastAction
  removal.
- **The lvt non-browser regression** (`internal/...`, `commands/...`,
  `testing/...`) is **already green** against the Phase-4 livetemplate
  worktree (verified this session) — Phases 0–4 are backward-compatible
  with current lvt code. Phase 5's removals are the first time
  livetemplate-side changes will require lvt-side migration.
- **Optional (advisable): fix the lvt `scripts/pre-commit.sh`
  `--disable-all` v2-incompatibility** in or before Phase 5. One-line:
  `--disable-all` → `--default=none` (v2 syntax). Currently the lvt lint
  gate is effectively non-enforced; bringing it back will surface 80+
  pre-existing issues at once.

**Phase 6 (Docs + examples + ecosystem):** the `lvt:error` CustomEvent
contract documentation deliverables, in priority order:
1. **The `lvt:error` CustomEvent contract** on the `[data-lvt-id]` wrapper
   with `detail: { code, topic }`, non-bubbling. Pin the byte-for-byte
   agreement with the server envelope.
2. **Distinguish from the form-level `lvt:error`** in
   `state/form-lifecycle-manager.ts` (different target = `<form>`,
   different detail shape = `ResponseMetadata`). Two distinct events
   sharing the name by historical accident; neither bubbles so they never
   collide at a listener — but a reader hitting `grep lvt:error` will
   assume a bug otherwise.
3. **Keep-open behavior** on a denied Subscribe in the WS-connect Mount:
   the `lvt:error` fires, the WS stays open and functional, no
   auto-reconnect storm. Explicit contrast with the pre-Phase-4 close
   behavior so anyone migrating from a prior livetemplate release knows
   what changed.
4. **Controller-swallows-error case**: `_ = ctx.Subscribe("denied"); return
   s, nil` → **no envelope emitted** (Mount returned nil). The contract is
   on the propagated-error scenario. Document this so apps don't expect
   `lvt:error` when they swallow.
5. **`IsInitialMount` guard pattern** for controllers that subscribe topics
   the ACL may deny — to avoid the pre-existing HTTP-GET-denied-→-500
   behavior aborting the page render before the WS can exercise the
   keep-open path.
6. **HTTP-GET denied behavior remains HTTP 500** (unchanged from Phase 1).
   Surface in troubleshooting docs.
7. **Cross-repo release order** for any future similar wave: livetemplate
   + client are wire-independent; lvt gates last via the e2e against both.
   The committed-`replace`-resolved-by-pin-bump pattern is the canonical
   shape.
8. **Partial-state adoption on `*TopicForbiddenError` keep-open**
   (surfaced by livetemplate#427 round-1 bot review): when Option B falls
   through, it adopts `newState` from `callMount` (the controller's first
   return value, returned alongside the error). For the canonical
   `return s, err` shape — where the controller never touches `s` before
   the denied `Subscribe` call — this is the pre-Subscribe state, which is
   the intended behavior. **But** if a controller mutates `s` before the
   denied `Subscribe` and then `return s, err`, that *partially-modified*
   state is silently adopted (not rolled back). Consistent with Go
   error-handling conventions, but could surprise controller authors who
   expect a clean rollback on Mount error. Phase 6 docs must surface this
   alongside the controller-swallows-error case (item 4) — the two-part
   guidance reads: "to surface the envelope, propagate the error
   (`return s, err`); to keep state clean, don't mutate `s` before a
   Subscribe that may be denied."

## Open questions for the user

**None blocking.** The user signoff gate is the next step. Three
coordinated PRs (livetemplate / client / lvt) await manual test +
explicit signoff. Release order (advisor-confirmed): client npm publish +
livetemplate release are wire-independent and may ship in either order
(envelope shipped Phase 1; the client TS change just adds a new consumer
that's a no-op if the server doesn't emit; the server keep-open change
just relaxes a close, no protocol change); the lvt PR + pin-bump is the
gating last step. Branches are local in `<repo>/.worktrees/broadcast-redesign-phase-4`.

Non-blocking notes recorded for transparency:
- The pre-existing lvt `scripts/pre-commit.sh` `--disable-all` flag bug
  was caught here; not in Phase 4 scope to fix. Flagged for Phase 5/6.
- The dual-event `lvt:error` name (Phase 6 doc deliverable) was confirmed
  spec-pinned by advisor — no rename, document the dual-meaning.

## File / commit / V-item pointers

**Files (per repo worktree `.worktrees/broadcast-redesign-phase-4`):**

- **livetemplate:**
  - `mount.go` (Option B keep-open: `if !errors.As(err, &tfe) { Error+return }`
    branch + `*TopicForbiddenError` envelope-send + Warn + fall-through to
    `connSt.state = newState` and the shared success-path lifecycle).
  - `topic_runtime.go` (stale comment update on `sendTopicForbiddenEnvelope`
    — describes keep-open + no-disconnect-storm; envelope struct unchanged).
  - `topic_test.go` (new `TestTopic_V14_MountDenyEmitsEnvelopeAndKeepsConnectionOpen`
    + `v14State` + `v14MountDenyController`; added `encoding/json` import).
  - `docs/proposals/broadcast-action-redesign-proposal/learnings/phase-4.md`
    (this file).
  - `docs/proposals/broadcast-action-redesign-proposal/learnings/progress.md`
    (Phase 4 row → complete + Ledger entry + Phase 5/6 row seeds).
- **client:**
  - `livetemplate-client.ts` (`handleWebSocketPayload`: new first-discriminator
    `errorEnvelope.type === "error"` branch dispatching `lvt:error`
    CustomEvent on `wrapperElement`; inline dual-event-name comment).
  - `tests/topic-error-envelope.test.ts` (new V14 jest logic leg —
    4 targeted assertions).
- **lvt:**
  - `e2e/topic_acl_error_envelope_v14_test.go` (new V14 Tier-2 chromedp
    browser e2e with full 4-artifact capture, `IsInitialMount` guard,
    local-bundle handler).
  - `go.mod` (committed `replace github.com/livetemplate/livetemplate =>
    ../../../livetemplate/.worktrees/broadcast-redesign-phase-4` +
    documenting inline comment).

**V-items:**
- **V14** (Client error envelope; the only V-item in Phase 4's gate):
  GREEN on all three tiers (Tier 1 livetemplate Go, client jest logic leg,
  Tier 2 lvt chromedp).
- Phase 4 added **no Phase 1–3 V-item regression** — full
  `scripts/pre-commit.sh` (livetemplate) green; lvt non-browser regression
  green; client jest full suite green; scoped `-race` clean.

**Commits:** none. **Three branches left local for user signoff**
(`<repo>/.worktrees/broadcast-redesign-phase-4` in livetemplate, client,
lvt) per the PR signoff gate. `phase-4.md` + `progress.md` are committed
in the **livetemplate** PR (alongside the server change) — the proposal's
"phase-N.md committed in the phase's own implementation PR alongside its
code" rule, with livetemplate as the natural carrier (livetemplate-repo
files + livetemplate server change). Client + lvt commit their own
deliverables.

**Gate commands (the actual ones run this session, for reproducibility at
signoff):**
- livetemplate: `PATH=$HOME/go/bin:$PATH bash scripts/pre-commit.sh` →
  fmt + lint **0 issues** + full `go test ./... -timeout=300s` incl. Redis
  testcontainers; plus `GOWORK=off go test -race -run
  'TestTopic_|TestBroadcast|TestHTTPPost_BroadcastAction|TestEphemeral_BroadcastAction'
  -timeout=180s .` (11.99s, clean).
- client: `npm test` (29 suites, 551 tests, 100% pass) +
  (the client pre-commit also runs `npm run build` which was run
  separately to produce the bundle the lvt e2e consumes).
- lvt: `GOWORK=off go test -short -count=1 -timeout=120s
  ./internal/... ./commands/... ./testing/...` (non-browser, all green);
  `GOWORK=off go test -tags=browser -v -timeout=5m -run
  'TestE2E_V14_TopicACLDeniedEmitsLvtErrorAndKeepsWSOpen' ./e2e/`
  (PASS 1.57s end-to-end).
