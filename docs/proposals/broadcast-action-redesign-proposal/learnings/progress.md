# BroadcastAction Redesign — Live Progress Tracker

**Canonical plan:** the §"Phased implementation plan (tracker)" section of `docs/proposals/broadcast-action-redesign-proposal.md` (the converged Publish/Subscribe spec, #415; read-only baseline).
**This file:** the writable companion. Each implementation session (one phase at a time — typically a Claude Code session, per the multi-session model this loop is built for) updates it as its **first and last action**. The proposal section never mutates; all execution drift lives here.

---

## Phase Status

| Phase | Status | Owner / session | Started | Completed | Learnings file |
|---|---|---|---|---|---|
| 0 — Foundations (registry + topics.go) | not started | — | — | — | `phase-0.md` |
| 1 — Context API + ACL (single instance) | not started | — | — | — | `phase-1.md` |
| 2 — Cross-instance (Redis) | not started | — | — | — | `phase-2.md` |
| 3 — Wildcards (multi-segment) | not started | — | — | — | `phase-3.md` |
| 4 — Client error envelope (parallelizable 1–3) | not started | — | — | — | `phase-4.md` |
| 5 — Removal + in-repo/lvt migration | not started | — | — | — | `phase-5.md` |
| 6 — Docs + examples + ecosystem | not started | — | — | — | `phase-6.md` |

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

**If you block:** set status `blocked`, record the blocker in the row, write `phase-N-partial.md` with what was attempted and why it stalled.

---

## Surfaced-Scope & Deferral Ledger

The analog of a running budget: every phase rolls (a) scope surfaced mid-phase and (b) anything proposed for reconciliation into Appendix B "Alternatives considered" into one visible place. There is **no `Deferred (post-v1)` section** in this spec — items either ship in v1 or are reconciled into Appendix B with a maintainer decision logged in the journal. Phase 6's exit performs the final reconciliation.

### Known at design time (pre-seeded from the converged spec)

- **Severe footgun — dispatch-symmetry naming hazard.** `Publish(topic, "Delete", …)` invokes the same `Delete` a client-wired `name="Delete"` triggers, on every subscribed peer. v1 hard requirement: `Publish` MUST `slog.Warn` on a collision with a **client-wired** action name. This needs the new small `internal/parse/` wired-name pass, landed in **Phase 1**, gated by **`V19`**. Verify the warning is not silently dropped.
- **Bounded (not severe) — `Publish` is send-side ungated; gate at the caller.** Neither `ctx.Publish` nor `handler.Publish` runs the ACL; the Subscribe-time ACL gates *who reads*. There is **no built-in all-users topic** (`GlobalTopic()` was removed — Appendix B), so no topic reaches the whole user base by construction. The residue is bounded to one identity (`UserTopic`) or exactly the connections the app's own ACL admitted. Design intent: an app-wide announcement is an ordinary developer topic the app must allow in `WithTopicACL` and publish from trusted code. Docs (Phase 6) must surface this as named guidance, not a buried paragraph.
- **Intentional, not new — fan-out backpressure is drop-on-overflow.** `Publish` local fan-out enqueues via `Connection.EnqueueDispatch` (non-blocking, drops on a full per-connection buffer). Identical to existing `BroadcastAction` behavior (not a regression); surfaced by `wsBufferFull` / `wsSlowClientCloses`; tuned via `WithWebSocketBufferSize` / `LVT_WS_BUFFER_SIZE`. Treat as the accepted pre-existing model, not a new problem to solve in this work.
- **Explicitly rejected in Appendix B — do not reintroduce without a logged maintainer decision:** a built-in `GlobalTopic()`/all-users primitive; a `SessionTopic(groupID)` constructor; a `Publish` debounce/coalesce helper; a trie/radix pattern index or glob engine; `GroupID`-field reuse for the topic envelope (a new `Topic` field is used instead).
- **Multi-segment wildcards are IN v1** (Phase 3; Appendix B "single trailing `*` only" was superseded). The matcher is a flat O(P) segment scan by design. The open contingency is whether multi-segment **holds** under implementation pressure — Phase 3's Learnings makes the explicit call; if it is reduced, log the decision + rationale here and reconcile the body into Appendix B.

### Surfaced during Phase N (fill in as discovered)

- _Phase 0:_ TBD
- _Phase 1:_ TBD
- _Phase 2:_ TBD
- _Phase 3:_ TBD
- _Phase 4:_ TBD
- _Phase 5:_ TBD
- _Phase 6:_ TBD — final reconciliation into Appendix B goes here.

---

## Decisions Reaffirmed / Reversed

The proposal body + Appendix B "Alternatives considered" is the immutable baseline. If a phase must reverse or add to a decision (e.g., reintroduce a rejected alternative, or reduce multi-segment wildcards), log it here with date and one-sentence reason; the proposal stays the original baseline, this is the journal.

| Date | Decision reversed / added | New choice | Reason |
|---|---|---|---|
| — | — | — | — |

---

## Per-Phase Learnings File Template

Create each `phase-N.md` from this skeleton (kept identical to the §"Per-phase audit + learnings protocol" copy in the proposal — if you change one, change both):

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
