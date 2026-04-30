# Streaming Range Rendering — Phase 0 Pre-Implementation Audit

**Status:** Phase 0 audit complete — all 6 gates closed
**Date:** 2026-04-29
**Proposal:** [`docs/proposals/streaming-range-rendering-proposal.md`](./streaming-range-rendering-proposal.md) (PR #360)
**Scope:** Sign off the proposal's §15 Phase 0 audit checklist before any implementation PR opens. The proposal commits to a hard cutover (§8) — no runtime safety hatch — so this audit is the load-bearing recovery contract: any unverified claim that ships broken can only be undone by reverting the implementation PR.

## Pinned references

| Repo | HEAD | Status |
|---|---|---|
| `livetemplate/livetemplate` | `89688276` | clean main |
| `livetemplate/client` | `2d1fa56a` | clean |

The `livetemplate/examples` repo is not pinned at this phase — Phase 6 (Patterns Example) is the gate where its readiness is audited.

## Gate summary

| # | Gate | Result |
|---|---|---|
| 1 | §6 client verification | **PASS** |
| 2 | OQ1 — heterogeneous-range fallback long-term | **DECISION: indefinite legacy fallback** |
| 3 | OQ2 — volatile-field op | **DEFERRED to Phase 5 measurement** |
| 4 | Empty-range condition | **LOCKED** as `len(newItems) == 0` |
| 5 | `Range.Items` reader sweep | **PASS for production**; 63 test-side hits enumerated |
| 6 | `oldByKey` semantics | **LOCKED** as `map[string]struct{}` (presence-only) |

**Sign-off:** 6/6 gates closed. Phase 1 unblocked.

---

## Gate 1 — §6 client verification (PASS)

**Pinned client commit:** `2d1fa56a4616bac66ca4b680c78a59af089690d2` (status: clean).

### Function locations

- `mergeRangeItem` — `state/tree-renderer.ts:899-907` (3-line wrapper at 905-907; lines 899-904 are the leading doc comment shown in the code block below).
- `deepMergeTreeNodes` — `state/tree-renderer.ts:189-270` (the actual merge implementation).
- `shouldFullReplace` — `state/tree-renderer.ts:95` (structure-transition detector).
- Call sites that dispatch to `mergeRangeItem`: lines 355 and 674 (the `case "u":` handlers in `applyDifferentialOpsToRange` and the nested helper).

### `mergeRangeItem` body (verbatim)

```typescript
// state/tree-renderer.ts:899-907
/**
 * Merges changes into a range item using deep merge to preserve statics.
 * When the server sends partial updates like {"5": {"0": "new text"}},
 * we need to merge this into the existing item's field 5, not replace it.
 * Shallow spread would lose the statics ({"s": [...]}) that the client has cached.
 */
private mergeRangeItem(item: any, changes: any, statePath: string): any {
  return this.deepMergeTreeNodes(item, changes, `${statePath}.item`);
}
```

### `deepMergeTreeNodes` core merge loop (lines 225-267)

```typescript
const merged: any = { ...existing };

for (const [key, value] of Object.entries(update)) {
  const fieldPath = currentPath ? `${currentPath}.${key}` : key;

  // Check if value is a differential operations array
  const isDifferentialOps =
    Array.isArray(value) && value.length > 0 &&
    Array.isArray(value[0]) && typeof value[0][0] === "string";

  // Check if existing value is a range structure
  const existingIsRange =
    merged[key] && typeof merged[key] === "object" &&
    !Array.isArray(merged[key]) &&
    Array.isArray(merged[key].d) && Array.isArray(merged[key].s);

  if (isDifferentialOps && existingIsRange) {
    merged[key] = deepClone(merged[key]);
    this.applyDifferentialOpsToRange(merged[key], value, fieldPath);
  } else if (
    typeof value === "object" && value !== null && !Array.isArray(value) &&
    typeof merged[key] === "object" && merged[key] !== null && !Array.isArray(merged[key])
  ) {
    merged[key] = this.deepMergeTreeNodes(merged[key], value, fieldPath);
  } else {
    merged[key] = value;
  }
}

return merged;
```

### Sub-gate findings

| Sub-gate | Result | Justification |
|---|---|---|
| 1. Key-wise merge of payload | **PASS** | Line 225 shallow-copies `existing`. Line 227 iterates `Object.entries(update)`. Line 265 assigns `merged[key] = value`. Existing keys not present in the payload are preserved by the shallow copy and never re-touched. |
| 2. `""` treated as overwrite (clear) | **PASS** | Line 265 unconditionally assigns `value` to `merged[key]` — no skip-on-empty branch. A payload of `{"0": ""}` overwrites position 0 with the empty string; the client's render then displays nothing at that slot, satisfying §5c's "remove that text" semantic. |
| 3. Missing keys preserved (het-range fallback compat) | **PASS** | The loop only processes keys present in `Object.entries(update)`. Keys absent from the payload remain in `merged` from the shallow copy at line 225. Stream mode emits every position (so this is moot under stream mode), but the het-range fallback path can still send partial deltas and they continue to work. |
| 4. No diffing/skipping short-circuits | **PASS** | The `case "u":` dispatches at lines 355 and 674 gate on `if (updateIndex >= 0 && changes)`. An empty `{}` is truthy and falls into the merge loop (which iterates zero times — net no-op, correct). No deep-equality check, no "detect no change and skip" branch. |

### Adjacent code review

- **`shouldFullReplace` (line 95, called at line 218):** detects structure transitions where existing is range-shaped but update is not — and replaces wholesale. Stream-mode `["u"]` payloads are plain dynamics maps, not range structures, so `shouldFullReplace` returns false and the key-wise merge proceeds. Confirmed safe under stream mode.
- **Differential-ops branch (lines 245-254):** triggers only when the *value* at a key is a list-of-ops (`Array.isArray(value)` AND `Array.isArray(value[0])`) AND the existing target is a range with `d`/`s` arrays. Stream-mode top-level `["u"]` payloads carry plain object dynamics, never a list-of-ops. This branch handles the **recursive nested-range** case — consistent with §5a's "stream mode applies only at top-level ranges; nested ranges always take the legacy serialization path."
- **No deep-equality short-circuit, no payload-diffing pre-check, no "discard missing keys" path** anywhere in the merge implementation.

### Consumer-side contract verdict

§8 calls out the consumer-side contract: "the receiver MUST replace, not merge." Under stream mode's "every position present" producer-side invariant (§5c), the existing key-wise merge produces the **same observable result as a full replace** of the item's dynamics — every position the new render declares is overwritten in one pass. The het-range fallback path (which emits partial deltas) continues to work because absent keys are left as-is.

**No client change required.** The §8 statement that needs adding to the spec ("the receiver MUST treat `["u"]` as a full snapshot — replace the item's dynamics") is a *spec clarification*, not a behavioural ask on the existing client. Phase 4's spec update covers it.

---

## Gate 2 — Open Question 1 (heterogeneous-range fallback long-term) — DECISION

**Question (proposal §13.1):** Keep indefinite legacy fallback for heterogeneous ranges, or extend stream mode with per-item statics in `["u"]` (e.g. `["u", key, dynamics, statics?]`)?

**Decision:** Keep the **indefinite legacy fallback**, matching the proposal's current stance.

**Rationale:**
- The legacy path remains correct on het-ranges, just non-optimal on retained memory.
- Extending stream mode to handle per-item statics would require a wire-spec change beyond §5c (a new optional payload field on `["u"]`) — increasing the surface area of the cutover for a workload that isn't proven to be a problem.
- Reconnect rebuilds `StreamState` from scratch, so the het-fallback "stickiness" lasts only the connection lifetime — bounded by typical web session length (minutes to hours).

**Revisit trigger (recorded for the future):** if observability ever surfaces long-lived (>1h) sessions stuck on the het-range fallback at a measurable rate. No specific threshold today — there is no telemetry pipeline for this signal yet, so committing to a number would be premature precision.

**Revisit cadence (no external tracking issue needed):** Each subsequent phase appends its audit checkpoint to this same audit doc (per the proposal's §15 audit-history thread). Gate 2 is reviewed at every phase checkpoint — Phases 1, 2, 3, 4, 5 — by the phase author, even if only as a one-line "Gate 2 trigger remains unmet" note. If the trigger remains unmet at the Phase 5 checkpoint, Gate 2 is closed at that checkpoint as "resolved by stable absence of triggering signal" — definitive resolution rather than indefinite-pending. This makes the OQ1 decision non-immortal without requiring an external issue tracker.

---

## Gate 3 — Open Question 2 (volatile-field op) — DEFERRED

**Question (proposal §13.2):** Ship a future targeted-field op (e.g., `["uf", key, fieldIdx, value]`) as an explicit opt-in for high-frequency single-field updates on multi-field items, or rely on `permessage-deflate`?

**Decision:** **Deferred to Phase 5 benchmark output.**

The proposal explicitly defers this to `LargeTableController.UpdateRandomRow` measurement at 10k rows (combined with `BenchmarkRangeDiff_Stream_Update` at N ∈ {10, 100, 10k}). The audit confirms the deferral with one constraint: the wire-cost ceiling that would *trigger* a `["uf"]` follow-up issue is to be set when Phase 5 numbers land — picking a number before the data exists would be theatre.

**Acceptance criterion for closing OQ2:** Phase 5's audit checkpoint must include either (a) a written "no `["uf"]` follow-up needed; observed wire cost is acceptable" note, or (b) a tracking issue filed for the `["uf"]` op with the measured ceiling and the workloads that exceeded it.

**Maximum acceptable wire-cost ratio for closing OQ2 without filing `["uf"]`:** _____ × the equivalent legacy `["u"]` op for the same single-field update on a multi-field item. (TBD — Phase 5 reviewer fills in the observed ratio at the Phase 5 audit checkpoint, then chooses option (a) if the ratio is at or below the placeholder, or option (b) if above.)

---

## Gate 4 — Empty-range condition — LOCKED

**Predicate:** `len(newItems) == 0`.

This handles both nil and empty-initialized slices uniformly (Go's `len(nil) == 0`). Per §5a's "Empty-range edge case" paragraph: a first render with zero items defers the phase 1 → phase 2 transition, leaves `StreamState=nil, Items=[]`, takes the legacy path (which correctly emits an empty range tree), and re-evaluates on the next non-empty render.

**Test coverage (Phase 1 deliverable, not yet written):** §11 test 12 ("Empty-range deferral golden") will exercise this directly once authored — render 1 produces zero items, asserts `Range.StreamState == nil && Range.Items != nil` post-render; render 2 produces ≥1 items, asserts the homogeneity check fires and `StreamState` is now populated, `Items` is now nil.

---

## Gate 5 — `Range.Items` reader sweep — PASS for production

### Production-code & §10-covered sweep — PASS

`grep -rn "Range\.Items\|rangeData\.Items\|tree\.Range\.Items" --include='*.go'` from the repo root returns **85 total hits**:

- **22 hits across 10 files** are covered by §10 implementation-table rows (production code + the `internal/diff/diff_bench_test.go` test-fixture file §10 explicitly lists for adaptation).
- **63 hits across 10 files** are in tests NOT named in §10 — enumerated below as the parallel-audit queue.

| File | Hits | §10 row |
|---|---|---|
| `internal/build/types.go` | 6 | `internal/build/types.go` row (`Clone`, `MarshalJSON` direct emission, `ToMap` deep-convert, `UnmarshalJSON` deserialization, `convertValueToMap`) |
| `internal/diff/range_ops.go` | 2 | `internal/diff/range_ops.go` row (`extractRangeData` assignments at lines 127, 147) |
| `internal/diff/tree_compare.go` | 2 | `internal/diff/tree_compare.go` row (`handleMatchedRanges` no-op branch at lines 125-126) |
| `internal/diff/prepare.go` | 1 | `internal/diff/prepare.go` row (line 53) |
| `internal/diff/helpers_compare.go` | 1 | `internal/diff/helpers_compare.go` row — grep `rangeItemsEqual(a.Range.Items` |
| `internal/diff/helpers_value.go` | 1 | `internal/diff/helpers_value.go` row — grep `len(node.Range.Items) > 0` |
| `internal/diff/diff_bench_test.go` | 4 | `internal/diff/diff_bench_test.go` row (test-fixture mutations at lines 110, 167, 180, 192) |
| `internal/keys/loader.go` | 1 | `internal/keys/loader.go` row (line 33) |
| `internal/fuzz/invariants/helpers.go` | 2 | `internal/fuzz/invariants/helpers.go` row (lines 28-29) |
| `internal/fuzz/invariants/verifier.go` | 2 | `internal/fuzz/invariants/verifier.go` row (lines 232, 403) |

**No unlisted production sites.** The §10 implementation table is exhaustive against the current `main`. (The 6 hits in `internal/build/types.go` include the `UnmarshalJSON` deserialization site at line 535 and the `Clone` deep-copy sites at lines 677/679, both already covered by the §10 row's `Clone` discussion; the `MarshalJSON` and `ToMap` sites are the two emission paths after the post-audit drift correction.)

### §10 labeling drift — corrected in same PR

While verifying the table, one labeling issue surfaced:

- **§10 `internal/build/types.go` row** characterizes `TreeNode.MarshalJSON` as having "two emission sites for `"d"`." Reality: ONE site in `TreeNode.MarshalJSON` (grep `result\["d"\] = tn\.Range\.Items` in `internal/build/types.go`); the SECOND site is in `TreeNode.ToMap` (grep `convertedItems := make` in the same file, via the `convertValueToMap` helper called per item). The fix policy (omit `"d"` when `Items==nil && StreamState!=nil`) applies to both, but they live in two distinct methods — not two sites in one method.

The proposal is being amended in the same PR as this audit doc to use precise method names. Affected lines in the proposal: 240 (§10 row body), 246 (cross-reference), 248 (cross-reference), 270 (§15 audit checklist), 281 (Phase 1 deliverable), 359 (§11 test-7).

### Test-file parallel-audit queue — 63 hits across 10 files

Per §15: "Test files don't need entries in the implementation table, but they do need a parallel audit." Excluding `internal/diff/diff_bench_test.go` (covered by §10 in the table above), 63 hits remain:

| File | Hits | Notes |
|---|---|---|
| `internal/parse/range_test.go` | 32 | Hot zone — most are `len(tree.Range.Items)` count assertions and direct `tree.Range.Items[N]` index access for type assertions. Parse-layer tests construct their own trees, so most should pass unchanged once the parse layer is left alone. Verify post-Phase 3. |
| `e2e_update_spec_test.go` | 7 | Includes empty-range and N-item assertions plus `ops = tn.Range.Items` / `ops = changes.Range.Items` accesses. **The `ops = ...` lines access the changes-tree `Range.Items` (the op list, NOT the retained tree's per-item dynamics)** — per §5a's "two roles" clarification, this usage is *unchanged* by stream mode. The retained-tree assertions may need updating. |
| `internal/parse/range_statics_test.go` | 6 | Logging statements accessing `Range.Items` for diagnostics. Low risk. |
| `internal/diff/prepare_test.go` | 5 | Includes a `"nil Range.Items"` test case at line 519 with `if resultNode.Range.Items != nil` assertion — **this test directly validates the proposal's Gate 5 fix to `PrepareTreeForClient`** and should pass unchanged once §10's `prepare.go` row is implemented. |
| `internal/parse/var_decl_test.go` | 4 | `items := rangeTree.Range.Items` assignments + `len()` assertions. |
| `template_test.go` | 3 | `len(rangeNode.Range.Items)` assertions and one iteration loop at line 3738. **Audit during Phase 3** — these tests run end-to-end against `Template`; if they exercise stream-mode trees, assertions may need to inspect `StreamState.Keys` instead. |
| `tree_test.go` | 3 | `len(rangeNode.Range.Items) != N` invariant checks. **Audit during Phase 3.** |
| `internal/parse/stdlib_parity_test.go` | 1 | `for _, item := range tree.Range.Items` iteration. |
| `internal/diff/tree_compare_test.go` | 1 | Line 175: `if changes.HasRange() && len(changes.Range.Items) > 0` — also reading the changes-tree direction; unchanged by stream mode. |
| `internal/diff/loadmore_test.go` | 1 | Line 141: asserts "full range sent (not differential)." **Likely needs re-targeting** — under stream mode, the "full range sent" behaviour fires *only* on the het-range fallback path. Concretely: reconstruct the fixture as a heterogeneous range (items with mixed structure types — e.g., a list where some items render `<span>...</span>` and others render `<del>...</del>`) so the "full range sent" assertion still validates real fallback behaviour rather than passing trivially against an unused stream-mode emission shape. Phase 3 owns this re-targeting. |

**Audit verdict:** the test-side queue is enumerated (63 hits across 10 files, sum verified); no PR opens to change them in this audit phase. Phase 3's "CI green across all suites" gate catches them in the implementation PR.

---

## Gate 6 — `oldByKey` semantics — LOCKED

**Decision:** Add a NEW field `ctx.oldKeySet map[string]struct{}` to `rangeContext` for the three shared presence-only helpers; keep the existing `ctx.oldByKey map[string]interface{}` unchanged for the three legacy-only callers that read item bodies. Populate `oldKeySet` on **both** paths; populate `oldByKey` **only on the legacy path**.

**Rationale.** Four functions actually consume `ctx.oldByKey` in current `main`; not all of them are presence-only:

| Caller | Access pattern | Path |
|---|---|---|
| `isComplexInsertionPatternCtx` (grep `func isComplexInsertionPatternCtx`) | presence (`_, inOld := ctx.oldByKey[keyStr]`) | shared (both paths) |
| `areAllItemsAtEndCtx` (grep `func areAllItemsAtEndCtx`) | presence (`_, exists := ctx.oldByKey[itemKey]`) at the inner loop | shared (both paths) |
| `isPureReorderingCtx` (grep `func isPureReorderingCtx`) | **value** — type-asserts to `*TreeNode`, reads `.Dynamics` to detect position-field-only changes; also calls `DetectPositionField(ctx.oldByKey)` to read keys | legacy only |
| `generateUpdateOps` (grep `func generateUpdateOps`) | **value** — passes `oldItem` to `compareRangeItemsWithKeyPos` for partial-delta computation | legacy only |

The two shared presence-only helpers (`isComplexInsertionPatternCtx`, `areAllItemsAtEndCtx`) switch to `ctx.oldKeySet`. The two legacy-only callers continue to read `ctx.oldByKey` (value-typed) — they are never reached on the stream path because `GenerateRangeStreamOperations` (the new top-level entry per §10) does its own update/reorder logic from `StreamState.Hashes` instead of dispatching to `generateUpdateOps` or `isPureReorderingCtx`.

**Note on `areAllItemsAtStartCtx` (called by `isComplexInsertionPatternCtx` as an early-return check).** This helper is *not* an `oldByKey` consumer — it iterates only `ctx.addedKeys` and `ctx.newItems` (its guards do not consult `oldByKey` or `oldItems`; it has a top-of-function `len(ctx.addedKeys) == 0` guard plus inline `return false` paths inside the iteration loop, none of which read old-side state). It still needs stream-mode adaptation, but the adaptation falls out *upstream* of the helper itself: when `ctx.addedKeys` is correctly derived from `StreamState.Keys` (rather than from `oldItems`/`newItems` differencing), `areAllItemsAtStartCtx` works unchanged on the stream path. No function-body edit is required.

**Why not just change `oldByKey` to `map[string]struct{}` everywhere?** That was the earlier (incorrect) sketch. It would silently break `isPureReorderingCtx`'s `oldItem.(*TreeNode)` type assertion and `generateUpdateOps`'s `compareRangeItemsWithKeyPos(oldItem, newItem, ...)` call — both compile errors at minimum, behavior changes in any reflection-style callsites that survived. Adding the second field is the safer split.

**Implementation note.** Populate `ctx.oldKeySet` on **both** paths (from `oldKeys` on the legacy path, from `StreamState.Keys` on the stream path). The "uniform population" rule prevents the regression where `areAllItemsAtEndCtx`'s `len(ctx.oldKeySet) == 0` early-return would fire spuriously on every stream render and silently break N≥4 tail-append workloads (the regression flagged in §10's `helpers_range.go` row).

---

## Phase 1 audit checkpoint

**Date:** 2026-04-29  
**Branch:** `phase1/streaming-range-types` (worktree `.worktrees/streaming-range-phase1/`)  
**Base commit:** `f075d7b5` (PR #361 audit-doc merge into `main`)

### Files touched

| File | Change |
|---|---|
| `internal/build/types.go` | Add `RangeStreamState` type + `RangeData.StreamState` field; preserve nil `Items` in `Clone`; add stream-mode deep-copy block in `Clone`; add omit-`"d"` guard in `MarshalJSON` and `ToMap`. |
| `internal/build/types_test.go` | 10 new tests: `TestRangeData_IsStreamMode` (table-driven across legacy/legacy-empty/stream/transitional cases for the new `IsStreamMode()` helper), `TestTreeNode_Clone_PreservesNilItems`, `TestRangeStreamState_DeepClone`, `TestTreeNode_MarshalJSON_OmitsD_StreamMode` + `_EmitsD_LegacyMode` + `_PreservesStatics_StreamMode` (regression guard against tying "s" emission to "d") + `_EmitsNullD_LegacyEmptyMode` (boundary: Items==nil && StreamState==nil preserves prior null shape), `TestTreeNode_ToMap_OmitsD_StreamMode` + `_EmitsD_LegacyMode` + `_EmitsEmptyD_LegacyEmptyMode` (ToMap parallel, preserves pre-existing "d": [] shape for the same boundary). |
| `internal/keys/hash.go` | Add `ItemHashUint64(dynamics []interface{}) uint64` after `GenerateItemHashFromSlice`. Godoc covers nil-skip + nil-vs-`""` divergence. |
| `internal/keys/hash_test.go` | 5 new tests: `TestItemHashUint64_Deterministic`, `_NilVsEmptyString`, `_PositionMatters`, `_NilEntriesSkipped`, `_CollisionStress_NoDupesIn10k` (10k synthetic dynamics, all hashes unique). |

### Test results

- **15 new tests** (10 in `types_test.go` + 5 in `hash_test.go`) — all pass.
- **Full test suite** (`GOWORK=off go test ./...`) — green; zero regressions across the 17 internal/* packages, the root `livetemplate` package, or `pubsub`.
- **Build sanity** — `go build` clean; pre-existing `interface{} → any` linter hints are unchanged from the base commit.

### Per Phase 1 audit checkpoint requirement

> Per proposal §11 Phase 1 audit checkpoint: "existing test suite green (no regression); new tests assert shape-only invariants. Reviewer confirms `RangeData.Items` is still the source of truth on every render path — no caller has been switched yet."

`grep -rn "StreamState" internal/ --include='*.go'` returns matches in exactly four locations, all expected:

1. **Type definition** — `internal/build/types.go` (the `RangeData.StreamState` field + `RangeStreamState` struct itself).
2. **`Clone` deep-copy block** — `internal/build/types.go` (handles the field IF present; never reached on production trees today).
3. **`MarshalJSON` / `ToMap` omit-`"d"` guards** — `internal/build/types.go` (the guard only fires when `Items==nil && StreamState!=nil`, a configuration no production caller produces today).
4. **Test files** + **godoc cross-reference** in `internal/keys/hash.go` (documentation only, no executable read).

**Confirmed: zero production callers construct or read `StreamState`.** All existing render paths continue to populate `RangeData.Items` and read from it; the new field is dormant until Phase 2 wires `GenerateRangeStreamOperations` to populate it.

### Phase 2 unblocked

Phase 2 deliverable per proposal §11: `GenerateRangeStreamOperations(streamState *RangeStreamState, newItems []interface{}, statics interface{}, metadata map[string]interface{}, stripStatics bool) []interface{}` plus the `rangeContext.oldKeySet` field and the helper-by-helper adaptations cataloged across §10's full implementation table — including `range_ops.go`, `tree_compare.go`, `helpers_range.go`, `prepare.go` (the wire-format path silently drops `StreamState` today; needs `StreamState` preservation), `helpers_compare.go` (`rangeItemsEqual` compares only `Items`; two stream-mode trees differing in `StreamState.Hashes` would compare equal), `helpers_value.go` (`HasRangeItems` returns false for stream-mode trees), `keys/loader.go` (`LoadExistingKeyMappings` reads only `Items`), and the fuzz-invariant sites — plus Gate 6. None of these depend on additional Phase 1 work; the foundational types are now in place.

**Phase 2 invariant note: `IsStreamMode()` transitional case.** The helper currently returns `false` (legacy mode wins) when `Items != nil && StreamState != nil` — a state Phase 2's StreamState producer must never create. Phase 2 should consider promoting this branch from a silent fallback to a `panic`/`log.Warn` so producer-side invariant violations surface during development rather than silently falling back to legacy emission.

---

## Audit sign-off

All six §15 gates close as recorded. Phase 1 (foundational types) is unblocked under the proposal's audit-checkpoint sequencing. The two open questions retain explicit owners (OQ1 closed by decision; OQ2 owned by Phase 5 measurement).

**No code changes are part of this audit.** The proposal-side drift correction (§10 / §11 method-name precision) ships in the same PR as this audit doc to keep the implementation reference self-consistent before any `*.go` lands.

## What this audit does NOT cover

- The implementation PR itself (Phase 1+ deliverable; this audit only signs off the precondition).
- Client-repo audits beyond §6 (no other client touchpoints exist for this proposal).
- Examples-repo readiness (Phase 6 deliverable — `LargeTableController` + `large-table.tmpl`).
- The Phase 5 wire-cost ceiling for OQ2 (deferred until measurement data exists).
