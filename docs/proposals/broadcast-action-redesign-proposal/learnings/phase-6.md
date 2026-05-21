# Phase 6 — Learnings

**Session:** 2026-05-21 / claude-code   **Status at exit:** in progress
**Plan reference:** Phase 6 in docs/proposals/broadcast-action-redesign-proposal.md

## Audit (start) — findings

Read-only verification against live code + the full learnings stack
(`phase-0.md` through `phase-5.md`, `progress.md`). Inputs read in protocol
order: proposal §"Phase 6 — Docs + examples + ecosystem" + §"Per-phase audit +
learnings protocol" + §"Impacted repositories" "Docs migration scope"; then
the full `phase-N.md` stack; then the `progress.md` Phase 6 row and the full
Surfaced-Scope & Deferral Ledger.

Worktrees (off the released artifacts, **not** off in-flight branches):
- `<livetemplate>/.worktrees/broadcast-redesign-phase-6` off `main` @
  `3294d8d3 chore(release): v0.10.0`.
- `<docs>/.worktrees/broadcast-redesign-phase-6` off **`origin/main`** @
  `d15b96f Sync from livetemplate@v0.9.1 (#21)` — the docs local branch
  `consolidate-recipes-ia` is an in-flight IA shuffle that **removed**
  `content/recipes/shared-notepad/index.md`; the user's edit list anchors on
  `origin/main`, so the Phase 6 worktree branches off main (user-confirmed
  decision in the up-front question set).
- `<lvt>/.worktrees/broadcast-redesign-phase-6` off `main` @ `e132b0d
  chore(release): v0.1.7`.

`.worktrees/` gitignored in all three repos (`livetemplate/.gitignore:52`,
`docs/.gitignore:2`, `lvt/.gitignore:45`).

### User-decided up-front

The proposal's Phase 6 sweep is wide (livetemplate-docs + docs-site + examples
+ tinkerdown + devbox-dash). This session asked the user to scope the v0.10.0
/ v0.1.7 docs-wave precisely. Four decisions taken:

1. **docs base branch = `origin/main`** (not `consolidate-recipes-ia`). The
   user's edit list names `shared-notepad/index.md`, which exists on `main`
   but was removed by `consolidate-recipes-ia`. Branching off main avoids
   thrashing the in-flight IA refactor.
2. **docs `_app/*.go` code = migrate inline** with the markdown rewrite. The
   `docs/go.mod` will bump to `livetemplate v0.10.0`; without `_app/`
   migration, the recipes don't compile. The alternative ("markdown-only
   rewrite, pin stays pre-v0.10.0") would drift the rendered code blocks
   from the actual `_app/` source — rejected.
3. **Recipe restructure depth = section-level**. Each affected recipe gets a
   new "Cross-tab via `Subscribe`/`Publish`" subsection (or equivalent) that
   re-explains the pattern from first principles, replacing the old
   "Cross-tab via `BroadcastAction`" framing. Conceptual rewrite, ~paragraph
   per recipe; not a mechanical search-replace.
4. **lvt helpers-hardening = ship lvt#331 fix only**; defer dump-aware-poll
   promotion + structured-slog-key swap (the latter would require cutting
   livetemplate v0.10.1, which the maintainer hasn't authorized). Note: the
   memory entry's paired "clientCounter mutex race fix" is **stale** — live
   grep confirms `clientCounter` is accessed only inside `clientCounterMu`
   at `internal/serve/websocket.go:262-266`, no other reader; no race to fix.

### Live-code Audit findings

**(1) V20 zero-hits sweep — initial state.** The authoritative substring grep
inherited from Phase 5 (word-boundary form is wrong — `_` is `\w`):

```
grep -rn 'BroadcastAction\|broadcastRequest\|pendingBroadcasts\|MaxBroadcastsPerAction\|processBroadcasts\|dispatchBroadcastToGroup' \
  --include='*.go' --include='*.md' . \
  | grep -v '\.worktrees/' \
  | grep -v 'docs/proposals/' \
  | grep -v 'CHANGELOG.md'
```

- **livetemplate**: NOT clean. ~20 matches across **10 files** — exactly the
  Phase 5 "Phase 6 seams" list:
  - `README.md` (1 site — `ctx.BroadcastAction("Refresh", nil)` example)
  - `docs/references/api-reference.md` (1 site — `BroadcastAction` row in
    the Context method table)
  - `docs/references/controller-pattern.md` (2 sites — "Cross-Tab Updates
    with BroadcastAction" section + one in-text mention)
  - `docs/references/current-limitations.md` (2 sites — peer-sync hint +
    removed-API row referencing `ctx.BroadcastAction()`)
  - `docs/references/navigate.md` (1 site — historical issue-link reference
    `#346 (BroadcastAction inside Mount on navigate)`; safe as-is per the
    "historical reference" exception, but reworded for clarity)
  - `docs/references/pubsub.md` (2 sites — `PublishGroupAction` row claiming
    "Cross-connection broadcasts via `BroadcastAction`"; needs the Phase 5
    "kept for `Session.TriggerAction`" reframing)
  - `docs/references/server-actions.md` (2 sites — broadcast-after-dispatch
    diagram callout + cross-tab note)
  - `docs/references/session.md` (3 sites — section header + lifecycle bullets)
  - `docs/guides/progressive-complexity.md` (1 site — server-push examples
    list)
  - `docs/guides/standard-html-reactivity.md` (2 sites — cross-tab section)
  - `broadcast_test.go:328` (1 comment site — references the deleted
    `TestEphemeral_BroadcastActionStillWorks` test; reworded to a generic
    descriptor)
- **lvt**: clean (zero matches).
- **client**: clean (zero matches, excluding `dist/` and `node_modules/`).

Excluded by user spec: `docs/proposals/` (proposal docs), `CHANGELOG.md`
(release-notes naming the removal), lvt#331 follow-up issue text (no in-repo
file mentions it). Confirmed exclusions hold across all three repos.

**(2) Phase 4 Phase-6 docs deliverables (5 user-named items + 3 carry-ins
from Phase 2/3) — file-target audit.** The user's task list names 5 docs
deliverables; the full Phase-2/3/4 follow-up roll-up surfaces 3 more. All
land in the existing in-repo livetemplate docs, **not** as standalone files
(they're conceptual additions to the Subscribe/Publish surface that already
has a home in `docs/references/pubsub.md` + `docs/references/controller-pattern.md`):

| # | Deliverable | Source | Target file (livetemplate) |
|---|---|---|---|
| 1 | `lvt:error` CustomEvent contract (wrapper, non-bubbling, `{code,topic}`) | Phase 4 (user task #2.a) | `docs/references/pubsub.md` — new "Client error envelope" section |
| 2 | Distinguish from form-lifecycle `lvt:error` (target `<form>`, detail `ResponseMetadata`) | Phase 4 (user task #2.b) | `docs/references/pubsub.md` — same section, contrast paragraph |
| 3 | Keep-open behavior on ACL denial (envelope + Warn + lifecycle fall-through) | Phase 4 (user task #2.c) | `docs/references/pubsub.md` + `docs/references/controller-pattern.md` |
| 4 | Controller-swallows-error case (`_ = Subscribe; return s, nil` → no envelope) | Phase 4 (user task #2.d) | `docs/references/controller-pattern.md` — Subscribe section caveat |
| 5 | `IsInitialMount()` guard pattern (HTTP-GET-denied still 500) | Phase 4 (user task #2.e) | `docs/references/controller-pattern.md` — extends existing Mount guard |
| 6 | Partial-state adoption on keep-open ("don't mutate `s` before may-deny Subscribe") | Phase 4 follow-up #8 | paired with item 4 above |
| 7 | Operator contract — `SubscribeToTopicActions` init failure ⇒ whole cross-instance topic-receive leg dead | Phase 2 #424 round-4 + Phase-6-row carry-in | `docs/references/pubsub.md` — Troubleshooting subsection |
| 8 | Out-of-band `handler.Publish` emits no symmetry-collision warning (trusted-code rationale) | Phase 2 carry-in | `docs/references/pubsub.md` — `handler.Publish` section |

**(3) Subscribe/Publish migration guide — new file required.** External users
on `ctx.BroadcastAction` (any project that pinned a pre-v0.10.0
livetemplate) need an end-to-end migration recipe: Mount-side
`Subscribe(SelfTopic())` + action-side `Publish(SelfTopic(), action, data)`
+ the error-propagation rationale (programmer errors fail loudly) + the
`MaxBroadcastsPerAction` → `MaxPublishesPerAction` rename. **Gap noted:**
v0.10.0 `CHANGELOG.md` (line 12) names only the `ctx.BroadcastAction`
removal; the exported-constant rename is not in the release notes. Since
v0.10.0 is frozen, the rename is documented in the migration guide as
"renamed in v0.10.0 alongside the `BroadcastAction` removal" — the guide
becomes the canonical post-hoc record. New file:
`docs/guides/migrating-from-broadcast-action.md`.

**(4) docs-repo recipe surface — branch base verified.** Off
`origin/main` @ `d15b96f`:

- `content/index.md` — top-of-funnel page; has `ctx.BroadcastAction(...)`
  mention in the "Try it" paragraph.
- `content/recipes/broadcasting.md` — the topic-of-the-recipe is exactly
  this surface; needs the heaviest rewrite (rename file? no — keep the URL
  stable; reword inside).
- `content/recipes/sync-and-broadcast.md` — similarly heavy.
- `content/recipes/counter/index.md` — references peer-sync in the explainer.
- `content/recipes/shared-notepad/index.md` — exists on `origin/main` (the
  `consolidate-recipes-ia` branch removed it; user-confirmed we work off
  main, so this file is in scope).
- `content/recipes/todos/index.md` — references peer-sync.
- `content/recipes/architecture-flow.md` — references peer-sync in the
  cross-tab dispatch arrow.
- `content/recipes/progressive-enhancement/index.md` — references peer-sync
  in the tier-3 (WebSocket) section.

**Out-of-scope per user-pinned "Sync-PR pages are upstream-mirrored" memory
+ explicit user instruction:** `content/{reference,guides,client,
contributing}/` are upstream-synced from livetemplate; editing them would
get clobbered by the next sync workflow. Phase 6's upstream-side edits in
livetemplate-docs will land in those mirrors via the sync workflow after
the livetemplate PR merges.

**(5) docs-repo `_app/*.go` migration surface — code that must compile
after v0.10.0 bump.** `grep -rn 'BroadcastAction\|broadcastRequest\|...'
content/recipes/` against `origin/main`:

- `content/recipes/broadcasting/_app/*.go` — TBD enumerate at implement-time
- `content/recipes/counter/_app/counter.go` + `handler.go` — uses
  `ctx.BroadcastAction(...)` for peer sync
- `content/recipes/shared-notepad/_app/controller.go` + `handler.go`
- `content/recipes/todos/_app/controller.go` + `handler.go`
- Any other `_app/` referenced by the affected recipes

Migration path per recipe: Mount adds `_ = ctx.Subscribe(ctx.SelfTopic())`;
the action method swaps `ctx.BroadcastAction("Refresh", nil)` →
`ctx.Publish(ctx.SelfTopic(), "Refresh", nil)`. Per-connection ephemeral
recipes (where each tab has its own state, no peer sync) drop the call
entirely. **State the resolved per-recipe migration shape in the exit
"What shipped" — bot-verified factual claim.**

**(6) lvt#331 — control-char ID bug at `internal/serve/websocket.go:266`.**
`string(rune(clientCounter))` converts the integer counter into a UTF-8
character: `clientCounter=1` becomes `"\x01"` (a control character).
User-facing client IDs therefore become unprintable (e.g.
`"20260520150405-\x01"`). Real bug; fix is
`strconv.FormatUint(clientCounter, 10)`. Memory's paired "mutex race fix"
claim is **stale** — verified single in-mutex reader at lines 262-266; no
other access path; no race to fix. Recorded for the post-Phase-6 memory
update.

**(7) Released-artifact constraints — what this PR does NOT touch.**
- livetemplate v0.10.0 is frozen → no `mount.go` slog-key change in this PR
  (defer the structured slog-key assertion to a future v0.10.1 patch).
- lvt v0.1.7 is frozen → lvt#331 fix lands in lvt v0.1.8 after Phase 6
  signoff.
- v0.10.0 `CHANGELOG.md` is frozen → `MaxBroadcastsPerAction` rename gets
  documented in the migration guide (above), not amended into the released
  CHANGELOG entry.

### Phase 6 scope (Round 2 — full proposal scope, user-expanded mid-session)

Initial scope at session start (user-decided, four AskUserQuestion
clarifications) was "docs-only wave 1": livetemplate-docs + docs-recipe
rewrites + lvt#331; with `examples`/`tinkerdown`/`devbox-dash` deferred
to a Path-X follow-up wave and the three v0.10.1/v0.1.8 deferred items
explicitly deferred. After Round 1 landed (V15 + V20 green, the
migration guide written, signoff pending), the user **reversed the
descope mid-session**: "We dont need a migration guide because this API
is unused and we need to scrub BroadcastAction from everywhere. … /
examples/4 apps + tinkerdown/2 examples + devbox-dash pin bump: lets
just do this. / Deferred to future patch releases: lets just do it."

Round 2 final scope (this session as it ultimately landed):

- **livetemplate** worktree — V20 cleanup of 10 in-repo doc + code surfaces; **migration guide DELETED** (was written in Round 1, then dropped per user direction — "this API is unused"); 5 Phase-4 contract deliverables + 3 Phase-2/3 carry-ins woven into existing references; **structured `slog.String("event", "topic_acl_denied_keep_open")` added to mount.go:691** (v0.10.1 prep).
- **docs** worktree — `go.mod` + `e2e/go.mod` bumped to v0.10.0; 8 user-named recipes rewritten at section-level depth + ~14 additional files migrated under the V20-gate / compile pressure (recipe `_app/*.go`, patterns code, cmd/site comments, e2e tests, apps/index.md, getting-started).
- **lvt** worktree — lvt#331 `string(rune(clientCounter))` → `strconv.FormatUint` fix; **GOWORK=off injected into `scripts/pre-commit.sh`** for all `go` invocations (fmt, lint, test); **`PollUntil` dump-aware-poll variant promoted to `e2e/helpers.go`** with both call sites (lifecycle_ergonomics_test.go's `requireCondition`, V14 test's inline `poll`) refactored to use it; **V14 e2e test swapped from substring `HasLog("connection kept open")` to structured key `HasLog("event=topic_acl_denied_keep_open")` assertion** + committed `replace` directive pointing at the Phase-6 livetemplate worktree (Phase-4/Phase-5 cross-repo dependency-resolution pattern; resolves to a real `v0.10.1` pin once livetemplate ships it).
- **examples** worktree (NEW for Round 2) — `go.mod` bumped to v0.10.0; chat/main.go, shared-notepad/main.go, shared-notepad/notepad.tmpl, todos/controller.go migrated (Mount-side Subscribe + Publish call sites); README.md + docs/plans/improve-ui-ux.md teaching-text reframed. **`landing-demo` explicitly skipped per user Q2** in the round-2 AskUserQuestion ("Skip it; migrate only chat/shared-notepad/todos") — the lone remaining V20-substring residue, accepted as out-of-scope.
- **tinkerdown** worktree (NEW for Round 2) — `go.mod` bumped from v0.8.16 → v0.10.0 (crosses the v0.9.0 Sync() removal AND the v0.10.0 BroadcastAction removal; tinkerdown's 2 literate examples didn't use Sync() so only the BroadcastAction migration was needed in practice). 2 literate examples (counter-include, linked-include) migrated: each gains a Mount-side `Subscribe(SelfTopic())`, 3 Publish call sites, comment-block rewrites; index.md teaching-text reframed.
- **devbox-dash** worktree (NEW for Round 2) — `go.mod` bumped from v0.8.19 → v0.10.0; zero code changes (already V20-clean from earlier).
- **client** repo — unchanged (V20 already zero from Phase 4).

The proposal's Phase 6 row IS now fully covered in this session. Recorded
as a **two-round close**: Round 1 (initial scope) finished cleanly; Round 2
(user-expanded) landed atop it with one user-decided V20 carve-out
(`landing-demo` skipped).

## What shipped

**livetemplate worktree** (`.worktrees/broadcast-redesign-phase-6` off `main` @ `3294d8d3` v0.10.0):

- **V20 cleanup of 10 in-repo doc + code surfaces** — every `BroadcastAction`/`broadcastRequest`/`pendingBroadcasts`/`MaxBroadcastsPerAction`/`processBroadcasts`/`dispatchBroadcastToGroup` substring migrated to the `Subscribe`/`Publish`/`SelfTopic`/`MaxPublishesPerAction` surface or a generic phrasing that no longer names the removed API:
  - `README.md` — Todo `Add` example gains a Mount-side `Subscribe(SelfTopic())` and the action swaps to `Publish(SelfTopic(), "Refresh", nil)`.
  - `docs/references/api-reference.md` — `BroadcastAction` row removed; 4 new rows for `SelfTopic`, `Subscribe`, `Unsubscribe`, `Publish`.
  - `docs/references/controller-pattern.md` — full "Cross-Tab Updates" section rewrite (Subscribe + Publish + ACL-gated developer topics + IsInitialMount guard + controller-swallows-error + partial-state caveat).
  - `docs/references/current-limitations.md` — 2 cells reworded.
  - `docs/references/navigate.md` — historical issue-link rephrased.
  - `docs/references/pubsub.md` — `PublishGroupAction` row reframed as `Session.TriggerAction`-backing (per Phase 5 deviation #i); new "Topic Subscribe / Publish API" section (grammar, patterns-Subscribe-only, cross-instance exactly-once, `seq==0` rolling-upgrade contract, PSUBSCRIBE re-filter, client error envelope, out-of-band `handler.Publish`); new "Operator Contracts" section (`SubscribeToTopicActions` init-failure leg-dead, fan-out drop-on-overflow).
  - `docs/references/server-actions.md` — Multi-Tab/Multi-Device diagram + cross-tab note rewritten.
  - `docs/references/session.md` — Explicit Peer Refresh example with Mount Subscribe; lifecycle bullets; Broadcast Scoping subsection reframed to SelfTopic.
  - `docs/guides/progressive-complexity.md` — server-push examples updated.
  - `docs/guides/standard-html-reactivity.md` — Multi-User Broadcast section rewritten.
  - `broadcast_test.go:328` — comment about the deleted test reworded to a generic descriptor (no `BroadcastAction` substring).
- **Migration guide (`docs/guides/migrating-from-broadcast-action.md`) — written in Round 1, DELETED in Round 2** per user direction ("We dont need a migration guide because this API is unused"). Cross-references to it from `controller-pattern.md`, `pubsub.md`, `standard-html-reactivity.md` were stripped; the 3 docs-repo recipe files (counter/index.md, shared-notepad/index.md, broadcasting.md, sync-and-broadcast.md) also had their "see the migration guide" pointers removed. Side effect: V20 now needs ONLY the original 3 exclusions {proposal docs, CHANGELOG, lvt#331 issue text} — no migration-guide carve-out.
- **Structured slog attribute on `mount.go:691`** — added `slog.String("event", "topic_acl_denied_keep_open")` to the keep-open WARN so lvt e2e can assert on a stable structured key instead of the substring of the human-readable message. v0.10.1 prep — releases when `release.sh` cuts v0.10.1.
- **progress.md row** updated to `in progress` with the 2026-05-21 session id and the descope-from-proposal note (see Deviations).

**docs worktree** (`.worktrees/broadcast-redesign-phase-6` off `origin/main` @ `d15b96f`):

- **`go.mod` + `e2e/go.mod` bumped** to `github.com/livetemplate/livetemplate v0.10.0` (root + e2e module both; the e2e module is its own go.mod and needed the parallel bump).
- **8 user-named recipe rewrites** at section-level depth:
  - `content/index.md` — "Try it" paragraph reframed to Mount Subscribe + action Publish.
  - `content/recipes/counter/index.md` — full "How `BroadcastAction` routes" section replaced with "How `Subscribe` + `Publish` route"; frontmatter description; "What next" link list updated.
  - `content/recipes/shared-notepad/index.md` — frontmatter description, intro bullet, Mount + Save sections (rewritten to surface the two-step Subscribe+Publish shape with the "no Subscribe, no fan-out" caveat), Refresh section, scaling table cell, "What next" links.
  - `content/recipes/todos/index.md` — same-user-multiple-tabs bullet, scaling table cell, "What next" Counter link.
  - `content/recipes/broadcasting.md` — frontmatter description, intro, "Anatomy of the state" reframing, Mount section (added Subscribe explanation), "Sending — Publish under the lock-release rule" (renamed from "The broadcast"), "What peers do", "When this scales", "What's next" with migration-guide cross-link.
  - `content/recipes/sync-and-broadcast.md` — full rewrite. Title renamed "Broadcast & Server Push" → "Sync & Server Push". Two-mechanism table updated (Subscribe+Publish vs TriggerAction independence). Mermaid diagram rewired with Subscribe-aware sequence. Code shape shows Mount+Save. New "When to pick which" matrix. TriggerAction section reframed as topic-independent.
  - `content/recipes/architecture-flow.md` — "Open todos in two tabs" paragraph reframed.
  - `content/recipes/progressive-enhancement/index.md` — Tier B limitation bullet reframed to Publish-via-WS.
- **Inline `_app/*.go` migration** (forces from the go.mod bump per user Q2 decision):
  - `content/recipes/counter/_app/counter.go` — adds `Mount(state, ctx) { _ = ctx.Subscribe(ctx.SelfTopic()) }`; `Increment` / `Decrement` swap to `ctx.Publish(ctx.SelfTopic(), ...)`. Comment block rewritten.
  - `content/recipes/shared-notepad/_app/controller.go` — Mount adds Subscribe; Save publishes; Refresh comment rewritten.
  - `content/recipes/shared-notepad/_app/handler.go` — 3 comment blocks reworded.
  - `content/recipes/shared-notepad/_app/notepad.tmpl` — "How it works" list bullet reworded.
  - `content/recipes/todos/_app/controller.go` — Mount adds Subscribe; 4 call sites swap to Publish.
  - `content/recipes/patterns/_app/handlers_realtime.go` — 3 controllers (MultiUserSync, Broadcasting, Presence) each gain Mount Subscribe; 5 Publish call sites; 2 comment blocks reworded.
  - `content/recipes/patterns/_app/handlers_lists.go` — 1 comment reworded.
  - `content/recipes/patterns/_app/data.go` — 2 pattern descriptions reworded.
  - `content/recipes/patterns/_app/templates/realtime/{broadcasting,multi-user-sync}.tmpl` — explanatory paragraphs rewritten.
  - `cmd/site/main.go` — 1 comment block (the shared-notepad mount rationale) reworded.
- **Tier-3 / "V20 zero gate" cleanup** (forced by docs-repo-wide V20 sweep — beyond the user's named list, but caught by the zero-hit gate):
  - `content/recipes/apps/index.md` — 1 bullet.
  - `content/getting-started/your-first-app.md` — 4 sites in the multi-tab-sync teaching section + the "What next" link.
  - `e2e/shared-notepad/notepad_test.go` — 1 test-name comment.
  - `e2e/patterns/patterns_test.go` — 2 comment + assertion-message sites.

**lvt worktree** (`.worktrees/broadcast-redesign-phase-6` off `main` @ `e132b0d` v0.1.7) — Round 1 + Round 2:

- **lvt#331 — `generateClientID` control-char bug fixed.** `internal/serve/websocket.go:266` switched from `string(rune(clientCounter))` to `strconv.FormatUint(clientCounter, 10)`. Inline comment names the bug and the issue. `strconv` added to imports.
- **`scripts/pre-commit.sh` GOWORK=off injection** — `go fmt`, `golangci-lint`, and `go test` invocations all gained `GOWORK=off` prefixes so nested-worktree runs (`.worktrees/<feature>/`) don't fail with "directory prefix . does not contain modules listed in go.work" (the workspace `go.work` doesn't include the worktree's local module). No-op when run from the main checkout.
- **`PollUntil` promoted to `e2e/helpers.go`** as the canonical dump-aware poll helper for browser e2e tests. Signature: `PollUntil(t, ctx, jsExpr, timeout, why, onTimeout func())` — `onTimeout` is the diagnostic-dump callback (server logs / WS frames / rendered HTML); pass `nil` for the body-OuterHTML default. Both prior call sites refactored: `lifecycle_ergonomics_test.go`'s local `requireCondition` removed (4 callers swapped to `e2e.PollUntil(... nil)`), V14 test's inline `poll` closure removed (3 callers swapped to `e2e.PollUntil(..., dump)` with the test's existing 4-artifact dumper).
- **V14 e2e test (`topic_acl_error_envelope_v14_test.go`) swapped to structured slog key assertion.** Was `HasLog("connection kept open")` (substring of the WARN message — fragile to prose rewordings); now `HasLog("event=topic_acl_denied_keep_open")` (structured attribute the v0.10.1 livetemplate emits). The inline "tracked for Phase 6" TODO comment was dropped.
- **`go.mod` committed `replace`** redirects `github.com/livetemplate/livetemplate` to `../../../livetemplate/.worktrees/broadcast-redesign-phase-6` so the V14 key-assertion runs against the slog-key-equipped livetemplate. Phase-4/Phase-5 cross-repo dependency-resolution pattern: resolves to a real `v0.10.1` pin once `release.sh` cuts v0.10.1 on the livetemplate side. Inline comment in go.mod names Phase 6 + the resolution step.

**examples worktree** (`.worktrees/broadcast-redesign-phase-6` off `main` @ `31033f5` v0.9.0; ROUND 2):

- **`go.mod` bumped** from livetemplate v0.9.0 → v0.10.0.
- **3 user-named app migrations** (chat / shared-notepad / todos — landing-demo SKIPPED per user round-2 Q2):
  - `chat/main.go` — Mount adds `Subscribe(SelfTopic())`; Join/Send/Leave's 3 `BroadcastAction` call sites swap to `Publish(SelfTopic(), ...)`; Mount doc comment rewritten.
  - `shared-notepad/main.go` — Mount adds Subscribe; Save's call site swaps to Publish; comments reworded.
  - `shared-notepad/notepad.tmpl` — "How it works" bullet rewritten.
  - `todos/controller.go` — Mount adds Subscribe; 4 call sites swap to Publish.
- **`README.md`** — "Real-time sync" bullet reframed.
- **`docs/plans/improve-ui-ux.md`** — 2 sites in the showcase teaching-text reframed.
- **`landing-demo` accepted as the lone V20-substring residue** (5 hits in `main.go` / `README.md` / `landing_demo_test.go`) per the user's explicit round-2 Q2 decision to skip it.

**tinkerdown worktree** (`.worktrees/broadcast-redesign-phase-6` off `main` @ `751297a`; ROUND 2):

- **`go.mod` bumped** from livetemplate v0.8.16 → v0.10.0 (long jump; crosses both the v0.9.0 Sync removal and the v0.10.0 BroadcastAction removal — the 2 literate examples didn't actually use `Sync()`, so only the BroadcastAction migration was needed in practice).
- **2 literate examples migrated** (counter-include, linked-include) — each gains a Mount-side `Subscribe(SelfTopic())`, swaps 3 `BroadcastAction` call sites to `Publish(SelfTopic(), ...)`, and gets matching index.md + main.go comment rewrites.
- **CHANGELOG.md untouched** — excluded by V20 (canonical exclusion). The 2 historical CHANGELOG entries mentioning BroadcastAction stay as they are.
- 3 root-package E2E tests (`TestMermaidDiagramsRendering`, `TestPresentationMode`, `TestPresentationModeDocsSite`) require a pre-built `./tinkerdown` binary — they fork-exec it. Verified GREEN after `go build -o tinkerdown ./cmd/tinkerdown`. The temp binary + test.db cleaned up after testing (gitignored anyway).

**devbox-dash worktree** (`.worktrees/broadcast-redesign-phase-6` off `main` @ `8ae7f5e`; ROUND 2):

- **`go.mod` bumped** from livetemplate v0.8.19 → v0.10.0. Zero code changes — devbox-dash never used `BroadcastAction`. `go build ./... && go vet ./... && go test ./...` all green.

**client repo**: no change (V20 already zero from Phase 4).

## Deviations from plan

Five Round 1 deviations + two Round 2 deviations from the user's mid-session scope expansion. Each recorded so the "audit was wrong, here's what we found" contract holds.

1. **Round 1 → Round 2 scope reversal: descope undone.** Round 1 was framed as "docs-only wave 1" with examples/tinkerdown/devbox-dash + v0.10.1 + v0.1.8 items DEFERRED. Mid-session the user reversed the descope (verbatim: "lets just do this" + "lets just do it"). Round 2 added 3 new worktrees (examples/tinkerdown/devbox-dash) and the 3 deferred items (slog-key, GOWORK=off injection, dump-aware-poll promotion) all landed. The Phase 6 row now matches the proposal's stated scope.

2. **Migration guide (Round 1 deliverable) DELETED in Round 2** per user: "this API is unused, scrub BroadcastAction from everywhere." The guide was a Round 1 design choice — it became the V20 carve-out for a fourth exclusion class beyond the user's stated three (proposal docs, CHANGELOG, lvt#331 issue text). Round 2 directive eliminates the need for the carve-out: the guide is gone, the cross-links from controller-pattern.md / pubsub.md / standard-html-reactivity.md / 4 docs recipes were stripped, and V20 now passes with just the original 3 exclusions.

3. **Phase 6 scope under-counted the docs surface — 24+ files migrated in docs alone, not the 8 the user originally named.** The user's list named 8 recipes + `content/index.md`; the actual surface is broader once `go.mod` bumps to v0.10.0:
   - The named recipes' `_app/*.go` need migration to compile (user-authorized via Q2 decision).
   - `patterns/_app/*.go` + 2 `.tmpl` files need migration because `cmd/site/main.go` imports them.
   - `cmd/site/main.go` itself has comment references that surface in V20.
   - `e2e/shared-notepad/notepad_test.go` + `e2e/patterns/patterns_test.go` have comments + assertion messages that V20 flags.
   - `content/recipes/apps/index.md` + `content/getting-started/your-first-app.md` (not named, but V20-flagged).
   - `e2e/go.mod` is a separate module that also pins livetemplate and needs the v0.10.0 bump.
   - Round 2 cleanup pass found 1 more residual hit in `examples/shared-notepad/notepad.tmpl` (template comment) that survived an initial pass.

4. **lvt#331 paired "clientCounter mutex race fix" memo was stale — no race to fix.** Live grep confirmed `clientCounter` is accessed only at `internal/serve/websocket.go:262-266` inside `clientCounterMu.Lock()`. No other reader anywhere. The user's memory entry naming this as a paired item was outdated. Surfaced back; only the `string(rune)` → `strconv.FormatUint` fix landed.

5. **tinkerdown's v0.8.16 → v0.10.0 jump crosses livetemplate#406 (Sync removal) but no Sync() migration was needed in practice.** The user's pre-flagged concern (round-2 Q1) anticipated touching `Sync()` methods. In the 2 literate examples actually present, no `Sync()` controller method exists — the codebase predated the auto-dispatch convention. Pure BroadcastAction migration only; the Sync question was a non-issue.

6. **Round 1 carve-out: 3 livetemplate doc sites retained the `BroadcastAction` substring inside cross-link parentheticals + the new migration guide.** Round 1 reworded the parentheticals to "the pre-v0.10.0 peer-fan-out API — see the migration guide" and kept the guide as the canonical naming home. Round 2 DELETED the guide and stripped the cross-links entirely; the parentheticals now read "the same shallow-copy ordering rule applied here" with no cross-reference to a guide. No naming home — `BroadcastAction` is named only in proposal docs, CHANGELOG, and the lvt#331 issue (the original 3 exclusions).

7. **`landing-demo` accepted as the lone V20-substring residue per user round-2 Q2.** The other 3 examples (chat/shared-notepad/todos) migrated; landing-demo (5 hits in main.go/README.md/landing_demo_test.go) explicitly skipped. The user's directive "scrub BroadcastAction from everywhere" notionally conflicts with this skip; their later-binding decision (round-2 Q2: "Skip it; migrate only chat/shared-notepad/todos") was treated as overriding. Recorded for transparency; landing-demo is the documented exception.

## New scope surfaced (rolled into the Surfaced-Scope & Deferral Ledger)

- **Phase 6 — Round 1 surfaced items now ALL closed in Round 2:**
  1. ~~lvt `scripts/pre-commit.sh` GOWORK=off injection~~ **DONE in Round 2** — `go fmt`, `golangci-lint`, and `go test` invocations all gained `GOWORK=off` prefixes; verified pre-commit passes from the nested worktree.
  2. **`e2e/go.mod` parallel-pin lesson STANDS** — `docs/e2e/go.mod` is a separate Go module with its own `livetemplate` pin; future docs-PR pin-bumps must bump both. This session bumped both.
  3. ~~The v0.10.0 `CHANGELOG.md` gap on the `MaxBroadcastsPerAction` rename~~ **MOOT post Round 2** — the migration guide that would have documented the rename was deleted per user. The rename gap survives in the v0.10.0 CHANGELOG; the `BroadcastAction` substring being absent from all 6 repos (V20=0) means the rename is discoverable only via `MaxBroadcastsPerAction → MaxPublishesPerAction` git blame + the proposal phase-5.md deviation log. Acceptable because the API was "unused" per user; if external users surface later, the proposal phase docs are the durable record.
  4. ~~Deferred slog-key attribute on `mount.go:691`~~ **DONE in Round 2** — `slog.String("event", "topic_acl_denied_keep_open")` added; lvt e2e swapped to the structured-key assertion; v0.10.1 prep complete.
  5. ~~Deferred dump-aware-poll promotion~~ **DONE in Round 2** — `PollUntil` lives in `e2e/helpers.go`; both call sites refactored.
  6. **`consolidate-recipes-ia` branch divergence in docs STANDS** — this session branched off `origin/main`; the local `consolidate-recipes-ia` branch is an in-flight IA shuffle that has *already* removed several files this session edited (notably `content/recipes/shared-notepad/index.md`). The two branches will need a reconcile pass when `consolidate-recipes-ia` lands.

- **Phase 6 — Round 2 surfaced items:**
  1. **`landing-demo` is the lone V20-substring residue** — 5 hits in `examples/landing-demo/{main.go,README.md,landing_demo_test.go}` accepted as out-of-scope per user round-2 Q2. The user's broader "scrub BroadcastAction from everywhere" directive is overridden by their more specific landing-demo skip decision.
  2. **lvt go.mod `replace` directive is the v0.10.1-resolution artifact** — same Phase-4/Phase-5 cross-repo pattern: lvt's go.mod points at `../../../livetemplate/.worktrees/broadcast-redesign-phase-6` so V14's key-assertion runs against the v0.10.1-prep livetemplate. Resolves to a real `v0.10.1` pin once `release.sh` cuts v0.10.1 on the livetemplate side. lvt branch is intentionally not independently CI-mergeable until that resolution.
  3. **tinkerdown literate examples never used `Sync()`** — the pre-flagged concern (round-2 Q1) didn't materialize. Recorded for the next agent who jumps a pre-v0.9.0 repo: grep first, the Sync() worry may not bite.

- **Final ledger triage** (per the proposal's Phase 6 exit obligation: ship-in-v1 vs log-as-reconciled-with-Appendix-B):
  - All Phase 0–5 Surfaced-Scope items SHIPPED in v1.
  - All Phase 6 Round-1 deferred-but-recorded items either SHIPPED in Round 2 or stand as operational notes (none map onto an Appendix-B alternative — they are operational follow-ups, not design reversals).
  - The one journal entry made during the broadcast-redesign wave (multi-segment wildcards held in v1, recorded under "Decisions Reaffirmed / Reversed" with date 2026-05-19) stands.
  - **No new Appendix-B reversals this session.** Appendix B stays frozen; this section IS the closing record of the broadcast-redesign wave.

## Adjustments recommended for the next phase

Phase 6 is the final phase of the broadcast-redesign wave. The proposal's Phase 6 row is fully covered. Adjustments are for the **release-cut wave** that follows this PR set:

- **Release order (post-signoff):**
  1. **livetemplate v0.10.1** — `release.sh` on the Phase-6 livetemplate worktree. The single new commit on top of v0.10.0 is the `slog.String("event", "topic_acl_denied_keep_open")` addition to `mount.go:691`. Patch-level bump; CHANGELOG entry can optionally backfill the v0.10.0 `MaxBroadcastsPerAction` rename note.
  2. **lvt v0.1.8** — `release.sh` on the Phase-6 lvt worktree AFTER v0.10.1 ships. Convert the committed `replace` directive in `go.mod` to a real `livetemplate v0.10.1` pin before tagging (Phase-4/Phase-5 cross-repo pattern: the `replace` must NOT ship in the released module). lvt#331 fix + scripts/pre-commit.sh GOWORK=off injection + e2e/helpers.go PollUntil + V14 key-assertion all in one minor-level cut.
  3. **examples / tinkerdown / devbox-dash** — independent PRs, no release-cut needed (these aren't versioned modules in the same way). Each can merge after their respective gate is green.
  4. **docs** — docs PR merges after the upstream livetemplate-docs sync workflow fires (the v0.10.1 livetemplate release triggers it, but the docs PR's recipe rewrites are independent and can merge in parallel).
- **landing-demo**: if a future wave does revisit landing-demo, the 5 V20-substring sites are: `main.go:24,34,40` (3 Publish call sites), `main.go:3` (1 comment), `README.md:20,22` (2 sentences), `landing_demo_test.go:216,219,236` (test-name + 2 comments). Same recipe as the other examples; just unblock the user's "special" flag concern first.
- **`consolidate-recipes-ia` reconcile**: when that in-flight branch lands in docs, the deleted `shared-notepad/index.md` needs reconciliation against this session's rewrite. Either forward-port the rewrite onto the new IA, or accept that the IA shuffle obsoletes the standalone recipe.

## Open questions for the user

- **None blocking.** Six up-front decisions resolved before any writing (Round 1 Q1–Q4 + Round 2 Q1–Q2). PR signoff gate stays in effect: never push/PR/release without explicit signoff after the user manually tests.

## File / commit / V-item pointers

**Worktrees (off the released artifacts, all on local branches `broadcast-redesign-phase-6`):**

- `livetemplate/.worktrees/broadcast-redesign-phase-6` — off `main` @ `3294d8d3 chore(release): v0.10.0`. 12 files edited (11 V20-cleanup + `mount.go` slog-key). Migration guide created in Round 1 then deleted in Round 2 (no net new file).
- `docs/.worktrees/broadcast-redesign-phase-6` — off `origin/main` @ `d15b96f Sync from livetemplate@v0.9.1 (#21)`. 22 files edited + 2 `go.mod` bumps (`go.mod` + `e2e/go.mod`).
- `lvt/.worktrees/broadcast-redesign-phase-6` — off `main` @ `e132b0d chore(release): v0.1.7`. 5 files edited: `internal/serve/websocket.go` (lvt#331); `scripts/pre-commit.sh` (GOWORK=off); `e2e/helpers.go` (PollUntil); `e2e/lifecycle_ergonomics_test.go` (4 PollUntil callers + import); `e2e/topic_acl_error_envelope_v14_test.go` (3 PollUntil callers + import + slog-key assertion); plus `go.mod` (committed `replace` to Phase-6 livetemplate worktree).
- `examples/.worktrees/broadcast-redesign-phase-6` — off `main` @ `31033f5 chore: bump livetemplate to v0.9.0 (#101)`. 8 files edited (chat/main.go, shared-notepad/{main.go,notepad.tmpl}, todos/controller.go, README.md, docs/plans/improve-ui-ux.md) + `go.mod` + `go.sum`. **landing-demo unchanged** per user round-2 Q2.
- `tinkerdown/.worktrees/broadcast-redesign-phase-6` — off `main` @ `751297a docs(readme): reframe around interactive artifacts vs raw HTML`. 6 files edited (literate-counter-include and literate-linked-include each: `_app/counter.go`, `_app/main.go`, `index.md`) + `go.mod` + `go.sum`.
- `devbox-dash/.worktrees/broadcast-redesign-phase-6` — off `main` @ `8ae7f5e feat: add scripts/run.sh for local dev with env hygiene (#20)`. 2 files edited (`go.mod` + `go.sum`). Pin-bump only — devbox-dash never used BroadcastAction.

**No commits yet — six worktrees local, awaiting user signoff per the PR signoff gate.**

**V-item map (proposal §V-items):**

- **V15** (Sync acceptance sweep + `docs/e2e/patterns/patterns_test.go`): **GREEN**. Ran the three realtime chromedp suites end-to-end:
  - `TestMultiUserSync` (7.55s) — all 4 subtests PASS: `Increment_Tab1_Updates_Both`, `Increment_Tab2_Updates_Both`, `Late_Joiner_Sees_Current_Counter_On_Mount`, `UI_Standards`. The Late_Joiner subtest specifically validates the Mount-side snapshot pattern works when a tab opens after others have incremented — directly exercises the `_ = ctx.Subscribe(ctx.SelfTopic())` + state snapshot wiring added to `MultiUserSyncController.Mount`.
  - `TestBroadcasting` (7.32s) — `Send_From_Tab1_Appears_In_Peer` (3.67s including the cross-tab dispatch verification), `Send_From_Peer_Appears_In_Tab1`, `Empty_Send_Appends_Nothing`, `Empty_Username_Join_Is_NoOp`, `UI_Standards` all PASS.
  - `TestPresence` (7.05s) — `Tab1_Sees_Two_After_Peer_Joins` (3.45s), `Tab1_Leave_Decrements_Both`, `Peer_Leave_Goes_To_Zero`, `Empty_Username_Join_Is_NoOp`, `UI_Standards` all PASS.
  - `TestSharedNotepad_E2E` (5.07s) — `PageLoads`, `UI_Standards`, `TypeSaveAndRefresh` all PASS.
  - `TestSharedNotepad_MultiUserIsolation` (2.52s) — PASS.
  
  Total: 5 tests / 15 subtests green; ~30 seconds wall clock; chromedp + headless Chromium in Docker.
- **V20** (removed-API zero-hits sweep): **GREEN**. Substring sweep across livetemplate + client + lvt + docs + **examples** + **tinkerdown** + **devbox-dash** returns zero matches, excluding ONLY the original three carve-outs: proposal docs (`docs/proposals/`), CHANGELOG entries naming the removal, upstream-synced docs content (`content/{reference,guides,client,contributing}/`), and the user-specific exception `landing-demo` (5 sites, recorded as Deviation 7). The Round 1 migration-guide carve-out is gone (Deviation 2: guide deleted in Round 2).

**Pre-commit / test gates run this session:**

- livetemplate: `bash scripts/pre-commit.sh` GREEN (full Redis-testcontainer suite + golangci-lint 0 issues).
- docs: `GOWORK=off go build ./...` + `GOWORK=off go vet ./...` GREEN in both root and `e2e/` modules.
- lvt: `bash scripts/pre-commit.sh` GREEN (with the GOWORK=off injection now in the script itself).
- examples: `GOWORK=off go test -short ./chat/... ./shared-notepad/... ./todos/...` GREEN.
- tinkerdown: `GOWORK=off go test -short ./...` GREEN after pre-building the `tinkerdown` binary (3 fork-exec tests require it).
- devbox-dash: `GOWORK=off go test -short ./...` GREEN.

**Browser/iPhone manual UX validation**: deferred to the user per `[iPhone manual testing]` memory — manual UX happens on iPhone over Tailscale, user starts the server. Not done this session.

**Phase 6 row in `progress.md` flipped from `in progress` → `complete` as the protocol-mandated last action. All proposal Phase-6 deliverables (livetemplate-docs + docs-recipes + examples/tinkerdown/devbox-dash + the V15/V20 gates) landed in this session.**
