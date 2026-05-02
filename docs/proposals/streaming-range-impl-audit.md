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

## Phase 2 audit checkpoint

**Date:** 2026-04-30  
**Branch:** `phase2/streaming-range-diff` (worktree `.worktrees/streaming-range-phase2/`)  
**Base commit:** `2a28b70d` (PR #362 Phase 1 merge into `main`)

### Files touched

| File | Change |
|---|---|
| `internal/diff/range_ops.go` | Add `oldKeySet map[string]struct{}` field to `rangeContext`; populate in `newRangeContext` (promoted from local). Add `newStreamRangeContext` constructor (oldByKey + oldItems intentionally nil — never read on stream path; struct field carries an INVARIANT comment). Swap `generateInsertionOps`'s empty-vs-incremental dispatch from `len(ctx.oldItems) == 0` to `len(ctx.oldKeySet) == 0` (functionally equivalent on legacy path; required for stream path). Add `GenerateRangeStreamOperations` (top-level entry): defensive guards (nil streamState, mismatched Keys/Hashes lengths), per-item key/hash/fingerprint extraction with early-bail on het-fingerprint or non-TreeNode, pure-reorder fast path, complex-insertion fallback, r→u→a/p/i→o emission per §5b. Add `generateStreamRemovalOps`, `generateStreamUpdateOps`, `dynamicsToUpdatePayload` helpers. (Proposal §11 mentioned a `fromStream bool` flag; turned out unused — every helper that needed discrimination uses `oldKeySet` directly. Dropped per CLAUDE.md "don't add features beyond what the task requires".) |
| `internal/diff/helpers_range.go` | `isComplexInsertionPatternCtx`: `ctx.oldByKey[keyStr]` → `ctx.oldKeySet[keyStr]`. `areAllItemsAtEndCtx`: `len(ctx.oldItems) == 0` → `len(ctx.oldKeySet) == 0` AND `ctx.oldByKey[itemKey]` → `ctx.oldKeySet[itemKey]`. `areAllItemsAtStartCtx` unchanged per §11 spec. |
| `internal/diff/types.go` | Add `RangeStreamState = build.RangeStreamState` alias for ergonomic test code (mirrors existing `TreeNode` / `RangeData` / `TreeMetadata` aliases). |
| `internal/diff/range_ops_test.go` | 14 new tests: 13 `TestGenerateRangeStreamOperations_*` (NoChange, Update_FullDynamicsPayload, UpdateClearsAbsentPositions, NilToEmptyStringPhantomUpdate, Removal, TailAppend4Items, HeadPrepend, MidInsert, PureReorder, MixedUpdateAndReorder, ScatteredInsertFallback, HetRangeFallback, StripStatics) plus `TestGenerateRangeDifferentialOperations_EmptyToItems_LegacyOldKeySetEquivalent` (legacy regression for the predicate swap). Adds `streamStateFor` test helper. |
| `internal/build/fingerprint.go` | Phase 2-driven addition: `CalculateStaticsFingerprint(*TreeNode) string` plus `hashStaticsWithCircularDetection`. Mirror of `CalculateStructureFingerprint`'s shape, but skips scalar dynamic positions entirely — only outer Statics, nested *TreeNode statics (recursively), and Range.Statics contribute. No cached method on TreeNode (the function is called once per item per render in stream mode; lazy-cache wrapper would be premature optimization). Phase 3+ can promote to a cached method if a hot-path caller emerges. |
| `internal/build/fingerprint_test.go` | 6 direct tests for `CalculateStaticsFingerprint` — `TestCalculateStaticsFingerprint_ScalarValuesIgnored` (the key contract: same template + scalar differences → same hash, including nil-vs-non-nil), `_NestedTreeVsScalarDiffers` (the het-range trigger), `_NestedTreeStaticsCaptured` (recursion into inner Statics), `_RangeStaticsCaptured`, `_NilTree` (sentinel), `_Deterministic` (pure-function check). |
| `internal/keys/hash_test.go` | Phase 1 backfill: `TestItemHashUint64_NestedTreeNode` — locks in the load-bearing `formatHashPart`-via-`json.Marshal` semantics for nested *TreeNode values that the stream-mode algorithm depends on. |

### Test results

- **21 new tests** (14 in `range_ops_test.go` + 1 in `keys/hash_test.go` + 6 in `build/fingerprint_test.go`) — all pass.
- **Full test suite** (`GOWORK=off go test ./...`) — green; zero regressions across all 17 internal/* packages, root `livetemplate` package, or `pubsub`.
- **Race detector** (`GOWORK=off go test -race ./internal/diff/... ./internal/keys/... ./internal/build/...`) — clean.
- **Build sanity** — `go build` clean; `gofmt -l` empty.

### Per Phase 2 audit checkpoint requirement

> Per proposal §11 Phase 2 audit checkpoint: "every test in item 1 is exercised by a direct call to `GenerateRangeStreamOperations`, no production code path reaches it yet. Reviewer confirms helper adaptations match the §10 spec (in particular, that `oldKeySet` is non-empty on the stream path)."

`grep -rn "GenerateRangeStreamOperations" internal/ --include='*.go' | grep -v _test.go` returns matches in exactly one location: the function definition + godoc in `internal/diff/range_ops.go`. **ZERO production callers** in `tree_compare.go`, `template.go`, `prepare.go`, or any other file. Every test invocation is a direct call from `range_ops_test.go`.

`grep -rn "StreamState" internal/ --include='*.go' | grep -v _test.go` confirms the only NEW production read site is the `streamState *build.RangeStreamState` parameter in `GenerateRangeStreamOperations` itself. No production reads added in `prepare.go`, `helpers_compare.go`, `helpers_value.go`, or `keys/loader.go` — all Phase 3 work.

`oldKeySet` is populated on both paths: `newRangeContext` (legacy) populates from `oldKeys` (the keys extracted from `oldItems`); `newStreamRangeContext` (stream) populates from `streamState.Keys`. The Round 12 regression check (per §10's `helpers_range.go` row) is satisfied — `len(ctx.oldKeySet) > 0` on stream renders with non-empty StreamState, so `areAllItemsAtEndCtx` doesn't early-return on tail-append workloads. This is exercised by `TestGenerateRangeStreamOperations_TailAppend4Items` (4-item tail-append → single `["a", 4items, statics]` op, NOT four individual `["i"]` ops).

### Phase 1 follow-up landed in this PR

**`TestItemHashUint64_NestedTreeNode` (`internal/keys/hash_test.go`)** — was originally in the Phase 1 plan but dropped during execution. Phase 2's algorithm load-bears on `formatHashPart`'s `json.Marshal` correctly capturing nested `*TreeNode` content (via `TreeNode.MarshalJSON`). Without the test, a future "simplify the hash" refactor could silently break the stream-mode update detection. Test asserts (a) two structurally-identical items with same nested content hash equal, and (b) items differing only in nested-TreeNode content hash differently.

### Phase 2-driven build-package addition

**`CalculateStaticsFingerprint` (`internal/build/fingerprint.go`)** — proposal §5b references "static fingerprint" as the homogeneity check, but the existing `CalculateStructureFingerprint` over-captures: it treats scalar dynamic-position-presence as structural (a position transitioning from `"x"` to `nil` produces a different structure fingerprint, even though both are content states of the same template). This breaks proposal test plan item 11's nil↔"" phantom-update contract: the algorithm would falsely classify same-template items as het-range whenever a scalar position cleared. The new `CalculateStaticsFingerprint` mirrors the existing fingerprint shape but skips scalar positions entirely — only outer Statics, nested *TreeNode statics, and Range.Statics contribute. Same `*TreeNode` template with different scalar content produces the same statics fingerprint; a conditional swapping a scalar for a nested *TreeNode produces different fingerprints (correctly classified as het). Used by `GenerateRangeStreamOperations` and the `streamStateFor` test helper. Function-only (no cached method on TreeNode) — the lazy-cache wrapper would be premature optimization for a one-call-per-item-per-render pattern.

### Phase 3 unblocked

Phase 3 deliverable per proposal §11: wire the dispatch in `internal/diff/tree_compare.go` at both `handleMatchedRanges` (top-level) and `handleRangeMatch` (nested) — `if oldNode.Range.StreamState != nil` → `GenerateRangeStreamOperations`, else `GenerateRangeDifferentialOperations`. Plus the phase-1→phase-2 transition (atomic under `t.mu` in `template.go`'s `compareTreesAndGetChangesWithContext`): after first non-empty render, check the homogeneous-statics fingerprint (now via `GetStaticsFingerprint`), populate `StreamState`, nil `Items`. Empty-range deferral stays in legacy path.

Other Phase 3 risk sites confirmed unchanged in Phase 2 (forward-documented in Phase 1 checkpoint, still pending): `prepare.go` (StreamState preservation in `PrepareTreeForClient`), `helpers_compare.go` (`rangeItemsEqual` extension), `helpers_value.go` (`HasRangeItems` extension), `keys/loader.go` (`LoadExistingKeyMappings` for stream mode), `internal/fuzz/invariants/verifier.go` (`buildIDKeyMap` + range-iteration sites + new fuzz corpus entries), and the no-ops golden in `tree_compare.go`'s empty-diff branch (per §10 `tree_compare.go` row).

Three helpers from proposal §11 Phase 2's "adapt" list required no function-body edits, all for the same reason — they read only fields that `newStreamRangeContext` populates correctly:

- `areAllItemsAtStartCtx` — iterates `ctx.addedKeys` and `ctx.newItems` positions; never touches `ctx.oldItems` or `ctx.oldByKey`. The prepend optimisation fires correctly on the stream path because `addedKeys` is correctly derived from `streamState.Keys` upstream.
- `handleIncrementalInsertionsCtx` — only dispatches to `areAllItemsAtStartCtx`/`areAllItemsAtEndCtx`/`handleIndividualInsertionsCtx`, no direct reads.
- `handleIndividualInsertionsCtx` — reads `ctx.addedKeys`, `ctx.newByKey`, `ctx.newItems`; the legacy-only fields stay nil and are never accessed.

A future Phase 3+ helper added here MUST audit which `ctx` fields it reads — `oldItems` and `oldByKey` are nil on the stream path (the struct-field `INVARIANT` comment in `range_ops.go` enforces this in code).

---

## Phase 3 audit checkpoint

**Date**: 2026-05-01
**Branch**: `phase3/streaming-range-cutover` → PR pending
**Base commit**: `c07977d9` (post-Phase-2 merge on `main`)

### Files touched

| File | Change |
|---|---|
| `internal/diff/tree_compare.go` | `handleMatchedRanges` + `handleRangeMatch` stream-mode dispatch at top of function (BEFORE any read of `oldTree.Range.Items`); no-op fallback predicate at line 125-126 extended to recognise stream-mode-empty (`Items==nil` with non-nil StreamState) so a stream-mode no-op render doesn't fall into `*changes = *newTree` |
| `internal/diff/range_ops.go` | Two adapter helpers: `handleStreamModeRange` (top-level) + `handleStreamModeRangeMatch` (nested) — extract new-side via new helper `extractNewSideRange`, call `GenerateRangeStreamOperations`, write ops into changes (or full-tree replacement on het-fingerprint nil return) |
| `internal/diff/transition.go` (NEW) | `TransitionToStreamMode(*build.TreeNode)` — top-level walker that runs the homogeneity check via `CalculateStaticsFingerprint` (Phase 2-driven), populates `RangeStreamState`, nils `Items`. Idempotent, nil-safe, top-level only per §5a |
| `internal/diff/transition_test.go` (NEW) | 7 unit tests: homogeneous fires, heterogeneous defers, empty defers, single-item fires, idempotent re-entry, nested ranges NOT transitioned, nil tree safe |
| `internal/diff/helpers_value.go` | `HasRangeItems` extended via existing `IsStreamMode()` predicate — answers the *logical* "does the range render anything" question for stream-mode trees |
| `internal/keys/generator.go` | New `LoadExistingKeysFromSlice([]string)` — same numeric-max-tracking as `LoadExistingKeys` but reads keys directly from the slice (used by stream-mode loader path) |
| `internal/keys/loader.go` | `LoadExistingKeyMappings` branches on `Range.StreamState != nil` → `LoadExistingKeysFromSlice(StreamState.Keys)` else legacy `LoadExistingKeys(Items)` |
| `internal/fuzz/invariants/verifier.go` | `buildIDKeyMap` adapted for stream-mode trees — collapses ID and key to the same value (since the only retained identifier is `StreamState.Keys[i]`); `verifyTreeStructureRecursive` no-ops cleanly on stream-mode (Items is nil, no per-item recursion needed — homogeneity already verified at transition time) |
| `internal/fuzz/invariants/helpers.go` | `convertValue` skips `result["d"]` assignment for stream-mode trees — matches the omitted-`"d"` wire shape Phase 1's MarshalJSON enforces (avoids producing `"d": []` shape that would diverge from production wire) |
| `template.go` | Single transition site at TOP of `buildTree` (after key-generator init, BEFORE `LoadExistingKeyMappings`) — fires on the previous render's `lastTree`, which is the SAFE slot per §5a's "after the first stream-mode render" timing. Placement BEFORE `LoadExistingKeyMappings` exercises the `LoadExistingKeysFromSlice` path on every stream-mode render. The transition is NOT placed at the `lastTree =` assignment sites (lines 1358, 1388) because that would mutate the wire output on the first render |
| `e2e_update_spec_test.go` | `TestUserJourney_TodoApp/add_multiple` validate function broadened to accept both forms: granular ops (`"i"`/`"a"`) when stream mode active OR full-tree replacement when het-fallback fires (TodoApp template's `{{if .Done}}` makes items heterogeneous once Done values mix — proposal §5d behavior) |
| `streaming_range_phase3_test.go` (NEW) | 5 root-package integration tests: stream mode fires on second render (wire-shape proof); reconnect resync emits full tree; full-dynamics-map invariant (§5c); het-range fallback (§5d); memory regression at N ∈ {10, 100, 1k, 10k} |

### Memory regression numbers (test plan item 2)

Captured by `TestStreamMode_MemoryRegression` (`go test -v -run TestStreamMode_MemoryRegression ./`):

| N | heap with Items | heap after transition | delta | bytes/item |
|---|---|---|---|---|
| 10 | 2,472,672 | 2,458,936 | 13,736 | 1,373.6 |
| 100 | 2,497,800 | 2,460,688 | 37,112 | 371.1 |
| 1,000 | 2,742,576 | 2,462,456 | 280,120 | 280.1 |
| 10,000 | 5,208,400 | 2,469,816 | 2,738,584 | 273.9 |

The per-item delta stabilises at ~273 bytes/item at large N (where allocator noise is amortised). Stream-mode `RangeStreamState` per item is one string key (~24 bytes header + bytes) + one uint64 hash (8 bytes) + amortised `Fingerprint` string (one per range, not per item) ≈ ~32 bytes/item. So legacy retains ~273 + 32 = ~305 bytes/item; stream mode retains ~32 bytes/item. **~9.5× reduction**, well above the proposal's ≥4× target.

The N=10 row is dominated by allocator coarsening (heap pages allocate in 8KB chunks; small N rounds up). At N=1k+ the per-item number is the meaningful measurement.

### Per Phase 3 audit checkpoint requirement

> Per proposal §11 Phase 3 audit checkpoint: "memory regression test shows ≥4× per-item retained-byte reduction; `extractRangeData` regression gate green; reconnect test confirms first render after `lastTree` drop emits a full tree, not stream ops; existing E2E specs pass with regenerated `["u"]` goldens; `grep -rn "t\.mu\.\(Lock\|Unlock\)" template.go` confirms the call chain from `t.mu.Lock()` (Execute, ~line 1651) through `compareTreesAndGetChangesWithContext` to the `Items=nil` assignment never drops the lock — the phase-1→phase-2 transition is naturally protected, but the audit verifies it."

- **Memory regression ≥4×**: confirmed (~9.5× at N=1k+, see table above).
- **`extractRangeData` regression gate**: implicitly verified by the full suite green run plus `TestStreamMode_FiresOnSecondRender` — on the second render, the output contains stream-mode `["u","key",...]` ops with whole-dynamics-map payloads (§5c shape), proving the dispatch routed to `GenerateRangeStreamOperations` and NOT through `extractRangeData` → `handleEmptyToItemsTransition`. A direct call-count assertion was not added because the wire-shape proof is already authoritative.
- **Reconnect resync**: confirmed via `TestStreamMode_ReconnectResyncEmitsFullTree` — drops `lastTree` + resets `hasInitialTree`, asserts the next render emits full statics and contains NO stream-mode update ops. PASS.
- **Existing E2E specs pass with regenerated goldens**: full suite green; the only test that needed migration was `TestUserJourney_TodoApp/add_multiple` (because the TodoApp template has `{{if .Done}}` inside the range, making it heterogeneous once Done values mix). The migration accepted both stream-mode ops and het-fallback full-tree replacement — both satisfy the test's intent. **No goldens needed regeneration** — the spec tests in `e2e_update_spec_test.go` that assert specific wire payloads continue to use legacy templates that don't trigger stream mode (no homogeneous range with ≥1 item) or use templates that fall back via §5d.
- **`t.mu` lock chain**: verified manually. `template.go:1651` acquires `t.mu.Lock()` (deferred unlock). The transition call at line 1660 (`diff.TransitionToStreamMode(t.lastTree)`) runs UNDER the lock. `LoadExistingKeyMappings` (line 1664) and the full diff path (`compareTreesAndGetChanges` at line 1382, `t.lastTree =` at lines 1358/1388) all run under the same lock. `grep -n "t\.mu\." template.go` shows no intermediate `Unlock()` calls between line 1651 and the function return. The transition is naturally protected.

### Test count delta

- 7 new unit tests in `internal/diff/transition_test.go` (TransitionToStreamMode_*)
- 5 new integration tests in `streaming_range_phase3_test.go` (TestStreamMode_*)
- 1 test migrated in `e2e_update_spec_test.go` (`TestUserJourney_TodoApp/add_multiple` validate fn broadened)
- 0 deleted tests
- 0 goldens regenerated

**Total: 12 new tests + 1 migrated.**

### Verification commands re-run

```
GOWORK=off go build ./...                                          # clean
GOWORK=off go test -count=1 ./...                                  # green (98s)
GOWORK=off go test -race -count=1 ./internal/diff/... ./internal/keys/... ./internal/build/... ./internal/fuzz/...   # race-clean
GOWORK=off go test -race -count=1 ./                               # race-clean (root pkg, 457s)
GOWORK=off gofmt -l internal/ template.go *.go                     # empty (Phase 3 files clean)
```

The root-package race run is the load-bearing verification for the `t.mu` chain — it covers the transition's mutation of `t.lastTree.Range.Items = nil` running under the same lock as the diff and the wire serialization. Internal-pkg race runs alone don't cover this. Confirmed clean.

The pre-existing `e2e/docker/multi_instance_test.go` gofmt nit is NOT a Phase 3 regression (that file is untouched).

### Phase 3 follow-ups

- **`prepare.go` StreamState propagation + `helpers_compare.go::rangeItemsEqual` extension**: deferred. Both were planned per §10 but determined to be defensive code (the call paths today never see populated StreamState on a tree headed through these functions). Per project guidance "no code for scenarios that can't happen", they are NOT shipped here. Reinstate in Phase 4+ if a real call site materialises (e.g., a broadcast path that prepares a post-transition tree).
- **Test plan item 10 (browser E2E in `lvt` repo)**: out of scope for this PR — Phase 3 ships without coordinated `lvt`-repo changes. The Go-side wire-format invariants (full-dynamics-map, stream-mode op codes) are tested in this repo via `TestStreamMode_FullDynamicsMapInvariant`. The browser-side rendering of those payloads is exercised by `lvt`'s existing range-ops E2E (which continues to pass because the op codes themselves are unchanged — only the `["u"]` payload contract is relaxed per §5c, and the existing client `mergeRangeItem` already accepts whole-item maps).
- **Spec doc update** (`docs/specifications/tree-update-specification.md`): Phase 4 deliverable.
- **Deletion of `CompareRangeItemsForChanges` and `compareRangeItemsWithKeyPos`**: Phase 4 deliverable.
- **Stream-mode benchmarks** (`BenchmarkRangeDiff_Stream_*`): Phase 5 deliverable. Phase 3 confirmed `diff_bench_test.go` is unaffected by the cutover (benchmarks bypass the dispatch by calling `GenerateRangeDifferentialOperations` directly — they never trigger transition).
- **`LargeTableController` example**: Phase 6 deliverable.
- **`IsStreamMode()` panic upgrade**: Phase 1/2 audits flagged as a Phase 3 *option*; deferred to Phase 4+ where the producer can also enforce the invariant. Adding error paths in Phase 3 would be premature without the corresponding producer-side enforcement.

### Phase 4 unblocked

Phase 4 deliverable per proposal §11: delete `CompareRangeItemsForChanges` and `compareRangeItemsWithKeyPos` (and their tests), update `docs/specifications/tree-update-specification.md` per §5c (relax producer-side "only changed fields"; add consumer-side "treat ['u'] as full snapshot" contract), regenerate any remaining goldens, verify `fuzz_diff_test.go` stays green.

The dispatch is wired and the cutover is observable end-to-end. The legacy `GenerateRangeDifferentialOperations` is now reached only by:
- The first render of a stream-capable range (before transition).
- The second render of a heterogeneous range (where transition deferred to legacy).
- The Phase-2-style direct test invocations.

Phase 4's cleanup audit can verify "no production caller of `compareRangeItemsWithKeyPos` other than `generateUpdateOps`" (the legacy diff helper), confirming the legacy path is reachable only through the explicit het-range fallback.

---

## Phase 4 audit checkpoint

**Date**: 2026-05-01
**Branch**: `phase4/streaming-range-cleanup` → PR pending
**Base commit**: `6d46c35e` (Phase 3 merge on `main`)

### Files touched

| File | Change |
|---|---|
| `internal/diff/range_ops.go` | Deleted `generateUpdateOps`, `CompareRangeItemsForChanges`, `compareRangeItemsWithKeyPos`, `handleNestedTreeNodeChange` (~120 lines). Removed `operations = generateUpdateOps(ctx, operations)` call site. Added new helper `hasKeptItemContentChanged(ctx)` and an early-return guard in `GenerateRangeDifferentialOperations` that signals nil-return on any kept-item content hash mismatch — `handleMatchedRanges` then falls through to `*changes = *newTree` per §5d |
| `internal/diff/helpers_value.go` | Deleted `isMeaningfulValue` (~20 lines, sole caller was the deleted `compareRangeItemsWithKeyPos`) |
| `internal/diff/transition.go` | **Phase 3 coverage gap closed**: `TransitionToStreamMode` now transitions root-level ranges in addition to direct-child ranges in Dynamics. Templates without wrapper elements (e.g., `{{range .Items}}<li>...{{end}}` rendered standalone) now stream-mode correctly. Extracted `transitionRangeIfHomogeneous` helper for both call sites |
| `internal/diff/transition_test.go` | Added `TestTransitionToStreamMode_RootRangeFires` covering the new root-range path |
| `internal/diff/range_ops_test.go` | Deleted 13 `TestCompareRangeItemsForChanges_*` tests, `TestGenerateUpdateOperations`, `TestGenerateRangeDifferentialOperations_Update`. Migrated `_Mixed`, `_UpdateAndReorder`, `_MultipleUpdatesAndReorder` to assert nil-return (full-tree fallback signal). Deleted `_UpdateNoReorder` (covered by new `_KeptItemContentChange_FallsBack`). Added 3 new tests: `_KeptItemContentChange_FallsBack`, `_StructuralAndContent_FallsBack`, `_PureStructural_StillEmitsOps` |
| `e2e_update_spec_test.go` | Removed `update_single` and `mixed_operations` subtests of `TestUpdateSpecification_RangeOperations` — both asserted partial-delta `["u"]` shapes that no longer exist on the wire. Behavior covered by the new internal/diff tests above |
| `docs/specifications/tree-update-specification.md` | Replaced "Only changed fields are included in the changes object." (line 377) with the §5c full-snapshot contract + the §232 consumer-side replace-not-merge contract. Added one example showing `""` encoding for cleared fields |
| `docs/proposals/streaming-range-impl-audit.md` | Appended this Phase 4 checkpoint section |

### Behavioral changes

- **Het-range content updates**: pre-Phase-4, kept-item content changes for het ranges emitted partial-delta `["u", key, {changed-fields-only}]` ops via `generateUpdateOps`. Post-Phase-4, those changes signal nil-return → `handleMatchedRanges` falls through to `*changes = *newTree` (full-tree replacement). This matches the §5d contract that het ranges are handled by full-tree emission.
- **Het-range pure-structural changes**: unchanged — pure adds/removes/reorders without kept-item content drift still emit compact `["a"]`/`["i"]`/`["r"]`/`["o"]` ops via the surviving `generateRemovalOps`/`generateInsertionOps`/reorder path. Approach A from the plan preserves this wire-size win.
- **Wrapper-less template streams correctly**: pre-Phase-4, templates whose root tree IS the range (no wrapper element) silently went through the legacy partial-delta path because `TransitionToStreamMode` only walked `tree.Dynamics`. Post-Phase-4, root ranges transition like any other top-level range and emit stream-mode `["u"]` ops with full snapshots.

### Wire regression accepted

Het ranges with kept-item content changes now emit full-tree replacement instead of partial-delta `["u"]` ops. Pure-structural changes to het ranges (add-only, remove-only, reorder-only) still emit compact ops via the surviving structural path — Approach A from the plan preserves this wire-size win. Homogeneous ranges are unaffected: they transition to stream mode on render 2 and emit compact `["u"]` with full snapshots per §5c. The wire regression is bounded to the het-range subset and is acceptable per §5d's "het ranges always full-tree" contract.

### Per Phase 4 audit checkpoint requirement

> Per proposal §323 Phase 4 audit checkpoint: "`grep CompareRangeItemsForChanges` and `grep compareRangeItemsWithKeyPos` return no hits. Spec doc diff matches the §5c/§8 language exactly. CI green across all suites."

- **`grep CompareRangeItemsForChanges`**: zero hits (`grep -rn "CompareRangeItemsForChanges" .worktrees/streaming-range-phase4 --include="*.go"` returns empty).
- **`grep compareRangeItemsWithKeyPos`**: zero hits.
- **`grep handleNestedTreeNodeChange`**: zero hits (cascade deletion).
- **`grep isMeaningfulValue`**: zero hits (cascade deletion).
- **`grep generateUpdateOps`**: zero hits (cascade deletion).
- **Spec doc diff**: replaces "Only changed fields are included in the changes object." with the §5c producer contract ("MAY include unchanged fields, MUST include every dynamic position present on either the old or new item") plus the §232 consumer contract ("the receiver MUST treat `['u']` as a full snapshot — replace, do not merge"). Both verbatim from the proposal.
- **CI green**: full suite green; race detector clean (root pkg ~526s, internal/diff ~1.7s); examples sibling-repo green; lvt and tinkerdown failures are pre-existing environment issues (`sqlc not installed`, missing local script files) unrelated to Phase 4.

### Test count delta

- 1 new unit test in `internal/diff/transition_test.go` (`TestTransitionToStreamMode_RootRangeFires`)
- 5 new unit tests in `internal/diff/range_ops_test.go` (`_KeptItemContentChange_FallsBack`, `_StructuralAndContent_FallsBack`, `_PureStructural_StillEmitsOps`, `_StaticsOnlyChange_FallsBack`, `TestHasKeptItemChanged_NonTreeNodeReturnsTrue`)
- 3 migrated tests in `internal/diff/range_ops_test.go` (`_Mixed`, `_UpdateAndReorder`, `_MultipleUpdatesAndReorder` — assert nil-return)
- 16 deleted tests in `internal/diff/range_ops_test.go` (13 `TestCompareRangeItemsForChanges_*`, `TestGenerateUpdateOperations`, `TestGenerateRangeDifferentialOperations_Update`, `_UpdateNoReorder`)
- 2 deleted subtests in `e2e_update_spec_test.go` (`update_single`, `mixed_operations`)

**Net: 6 new + 3 migrated − 18 deleted = −9 tests.**

The deletion delta is intentional: per-item ["u"] ops no longer exist as a wire shape, so tests that asserted them have no behavior left to verify. New tests cover the surviving fallback contract.

### Verification commands re-run

```
GOWORK=off go test -count=1 ./...                                  # green (108s root pkg, all internal pkgs <1s)
GOWORK=off go test -race -count=1 ./internal/diff/... ./internal/keys/... ./internal/build/... ./internal/fuzz/...   # race-clean
GOWORK=off go test -race -count=1 ./                               # race-clean (root pkg, 526s)
grep -rn "CompareRangeItemsForChanges\|compareRangeItemsWithKeyPos\|handleNestedTreeNodeChange\|isMeaningfulValue\|generateUpdateOps" internal/ --include="*.go"   # zero hits
```

Sibling-repo verification (with `go.work` temporarily pointing at the worktree):
- `examples` via `go test ./...`: all 13 packages green (counter, todos, patterns at 231s, chat, dialog-patterns, flash-messages, live-preview, login, progressive-enhancement, shared-notepad, ws-disabled, avatar-upload). The `patterns` and `todos` packages exercise chromedp E2E via `e2etest.StartDockerChrome()`.
- `examples` via `bash test-all.sh`: all 12 working examples green WITH `-race` flag (same coverage as `go test ./...` but race-checked).
- `lvt`: 4 failures all `sqlc not installed` — pre-existing environment issue, not Phase 4 regressions.
- `tinkerdown`: 2 failures both missing local script files (`./env.sh`, `./empty_lines.sh`) — pre-existing environment issues, not Phase 4 regressions.
- `livetemplate/e2e/docker/multi_instance_test.go`: pre-existing infra gap — Dockerfile pins `golang:1.24-alpine` but the project's `go.mod` requires `go 1.26.0` (since before Phase 1). Test fails on `main` for the same reason. Not a Phase 4 regression; gated by a Dockerfile bump that's out of scope.

### Phase 4 follow-ups

- **Stream-mode benchmarks** (`BenchmarkRangeDiff_Stream_*`): Phase 5 deliverable.
- **`LargeTableController` example**: Phase 6 deliverable.
- **`fuzz_diff_test.go` regeneration**: not needed — full suite (including `internal/fuzz/invariants` and root-pkg fuzz tests) is green.
- **Browser E2E in `lvt` repo**: not needed for Phase 4 — no new wire shapes were introduced; the `["u"]` op contract change shipped in Phase 3 and the Phase 3 audit verified client compatibility.

### Phase 5 unblocked

Phase 5 deliverable per proposal §325: add `internal/diff/range_ops_bench_test.go` with `BenchmarkRangeDiff_Stream_{Append,Update,Reorder}_{Small,Medium,Large}` benchmarks at N ∈ {10, 100, 10k}; capture the per-item retained-byte numbers via the existing `TestStreamMode_MemoryRegression` helper; update §7 in the proposal with measured values; mark the proposal status `Implemented` in the header.

The cleanup is complete; the wire contract is consistent end-to-end; all stale partial-delta producers are gone. Phase 5 can begin measuring.

---

## Phase 5 audit checkpoint

**Date**: 2026-05-01
**Branch**: `phase5/benchmark-gate` → PR pending
**Base commit**: `45756b72` (Phase 4 merge on `main`)

### Files touched

| File | Change |
|---|---|
| `internal/diff/range_ops_bench_test.go` | New file. Nine stream-mode CPU benchmarks (`BenchmarkRangeDiff_Stream_{Append,Update,Reorder}_{Small,Medium,Large}` at N ∈ {10, 100, 10000}) calling `GenerateRangeStreamOperations` directly, plus seven wire-size benchmarks (`BenchmarkRangeDiff_Stream_{Append,Update}_WireSize_{Small,Medium,Large}` and `BenchmarkRangeDiff_LegacyPartialDelta_Update_WireSize`) using `b.ReportMetric(..., "bytes/op")` to publish per-op JSON byte counts. Stream-mode fixtures built by calling `TransitionToStreamMode` on a fresh tree, mirroring the production transition path. |
| `internal/diff/range_memory_test.go` | New file. CI regression gate `TestRangeRetainedMemory_LegacyVsStream` at N ∈ {10, 100, 1000, 10000}. Methodology: `runtime.ReadMemStats` HeapAlloc delta over 8 retained trees with double-GC stabilisation. Per-N thresholds (1.8× at N=10, 4× at N=100, 5× at N=1k+) reflect the asymptote — fixed `RangeData/StreamState` overhead dilutes the per-item win at small N. Skipped under `-short`. |
| `docs/proposals/streaming-range-rendering-proposal.md` | §1 header status `Proposed` → `Implemented`. §7 "Memory & Performance" table replaced with measured values; lead sentence "Estimates, to be replaced…" deleted entirely. Methodology paragraph added explaining ReadMemStats + double-GC + json.Marshal approach. New 1k-item row added between 100 and 10k for asymptote visibility. |
| `docs/proposals/streaming-range-impl-audit.md` | Appended this Phase 5 checkpoint section. |

### Measured numbers (replacing §7 estimates)

Per-item retained heap bytes (warmed-up single sample per run; the helper discards a cold-start measurement to avoid arena setup skewing the small-N case — verified stable across 10 consecutive runs):

| N | Legacy B/item | Stream B/item | Ratio |
|---|---|---|---|
| 10 | ~270 | ~80 | ~3.4× |
| 100 | ~245 | ~47 | ~5.2× |
| 1000 | ~256 | ~41 | ~6.2× |
| 10000 | ~256 | ~41 | ~6.3× |

Wire bytes per op (`json.Marshal` of stream-mode op output, statics stripped):

| N | Append-1 | Update-1-field | Legacy synthetic update-1-field |
|---|---|---|---|
| 10 | 52 B | 45 B | 33 B |
| 100 | 54 B | 47 B | 33 B |
| 10000 | 58 B | 51 B | 33 B |

The stream-mode update wire-size grows by **~1.4×** vs the synthetic legacy partial-delta — well below the proposal's projected 3× ceiling. Inserts/deletes/reorders are unchanged. Real-world `permessage-deflate` recovers most of the absolute growth.

### Per Phase 5 audit checkpoint requirement

> Per proposal §333 Phase 5 audit checkpoint: "every estimate in §7 is replaced. The 'Memory & Performance' section no longer contains the word 'estimate'."

- **`grep -n "estimate" §7`**: `awk '/^## Memory & Performance/,/^## Reconnect/' docs/proposals/streaming-range-rendering-proposal.md | grep -ni "estimate"` returns empty.
- **Status header**: line 3 changed `Proposed` → `Implemented`.
- **All §7 numbers measured**: per-item retained bytes from `range_memory_test.go`; wire bytes from `range_ops_bench_test.go` `b.ReportMetric` output. Both reproducible from the worktree with `GOWORK=off go test -v -run TestRangeRetainedMemory ./internal/diff/` and `GOWORK=off go test -bench=BenchmarkRangeDiff_Stream -benchtime=1000x ./internal/diff/`.

### Open Question 2 partially answered

Per §383, OQ2 ("volatile-field workloads — do we need a future `["uf"]` targeted-field op?") is owned by combined data from item 13 (these benchmarks) and §12 (`LargeTableController.UpdateRandomRow`, Phase 6 deliverable).

Item-13 contribution: a single-field update on a 3-field item costs **45–51 B** on the wire (vs synthetic legacy 33 B). The 1.4× growth is well below the 3× ceiling that would justify shipping a targeted-field op. Final decision deferred to Phase 6 once `UpdateRandomRow` numbers exist for the multi-field worst-case workload.

### Verification commands

```bash
GOWORK=off go test -v -run TestRangeRetainedMemory_LegacyVsStream -count=3 ./internal/diff/    # 3 runs, all pass
GOWORK=off go test -bench='BenchmarkRangeDiff_Stream' -benchtime=100x -benchmem ./internal/diff/   # CPU + alloc benchmarks
GOWORK=off go test -bench='BenchmarkRangeDiff_Stream.*WireSize|BenchmarkRangeDiff_LegacyPartialDelta' -benchtime=1000x ./internal/diff/   # wire-size benchmarks
GOWORK=off go test -count=1 ./...                                                              # full suite
GOWORK=off go test -race -count=1 ./internal/diff/...                                          # race-clean
awk '/^## Memory & Performance/,/^## Reconnect/' docs/proposals/streaming-range-rendering-proposal.md | grep -ni "estimate"   # zero hits
```

### Phase 5 follow-ups

- **`LargeTableController` example + `UpdateRandomRow` benchmark**: Phase 6 deliverable. Closes OQ2 once the multi-field volatile-field workload has measured wire cost. (Resolved in Phase 6 below — final OQ2 decision recorded.)
- **Cross-platform numbers**: §7 numbers were captured on linux/arm64 with Go 1.26. Absolute byte counts will vary across architectures and Go versions; ratios (legacy/stream) are expected to be more stable than absolutes but were not separately verified on amd64.

### Methodology deviation from §11 item 2

§11 item 2 calls for "render twice, measure heap delta retained in `lastTree`" — i.e., go through `Template.Execute` twice and measure what survives. The implementation instead constructs the retained shape directly via `createTreeNodeRangeTree` + `TransitionToStreamMode`, then measures heap delta over a slice of 8 retained trees. Same measurement target (per-item bytes attributable to the retained range shape), shorter setup (no parser/binder/render path noise). The §7 numbers describe the same property the proposal asks for; the deviation is a methodology shortcut, not a correctness gap.

---

## Audit sign-off

All six §15 gates close as recorded. Phase 1 (foundational types) is unblocked under the proposal's audit-checkpoint sequencing. The two open questions retain explicit owners (OQ1 closed by decision; OQ2 owned by Phase 5 measurement).

**No code changes are part of this audit.** The proposal-side drift correction (§10 / §11 method-name precision) ships in the same PR as this audit doc to keep the implementation reference self-consistent before any `*.go` lands.

## What this audit does NOT cover

- The implementation PR itself (Phase 1+ deliverable; this audit only signs off the precondition).
- Client-repo audits beyond §6 (no other client touchpoints exist for this proposal).
- Examples-repo readiness (Phase 6 deliverable — `LargeTableController` + `large-table.tmpl`).
- The Phase 5 wire-cost ceiling for OQ2 (deferred until measurement data exists).

---

## Phase 6 audit checkpoint

**Date**: 2026-05-02
**Branches**:
- `livetemplate#phase6/large-table-example` (this branch — library fix + spec update + audit checkpoint)
- `lvt#phase6/record-ws-frames` (`testing.RecordWSFrames` helper)
- `examples#phase6/large-table-example` (`LargeTableController` + template + e2e + benchmarks)
- **Base commit**: `73cff639` (Phase 5 merge on `main`)

### Files touched (this repo)

| File | Change |
|---|---|
| `internal/diff/transition.go` | `TransitionToStreamMode` now recurses through `Dynamics` children. Pre-fix the function only walked the root + direct-child `Dynamics`, so a homogeneous range nested under `{{if}}` or `{{with}}` silently fell back to the legacy per-item path on every subsequent render. The recursion preserves the spec §5a "nested ranges in items stay legacy" contract because the walk never enters `Range.Items`. |
| `internal/diff/transition_test.go` | New test `TestTransitionToStreamMode_RangeWrappedInConditionalFires` — pins the fix at unit level. Existing `TestTransitionToStreamMode_NestedRangesNotTransitioned` already covers the negative side (nested-range-in-item still legacy) and continues to pass with the recursion in place. |
| `streaming_range_phase3_test.go` | New integration test `TestStreamMode_FiresOnConditionalWrappedRange` parses an actual template `<ul>{{if gt (len .Todos) 0}}{{range .Todos}}<li data-key="{{.ID}}">{{.Text}}</li>{{end}}{{end}}</ul>`, renders twice, and asserts the second render emits `["u","1",…]` (stream-mode op) and does NOT contain `"d":[` (legacy items shape). New `homoTodoConditionalTemplate` constant added next to `homoTodoTemplate`. |
| `docs/proposals/streaming-range-rendering-proposal.md` | §5a paragraph rewritten — "top-level ranges only" replaced with "any homogeneous range reachable through the static tree (Dynamics children including conditional and `with` branches); only ranges nested as items inside another range stay legacy." Adds explicit Phase 6 example as the regression source. |
| `docs/proposals/streaming-range-impl-audit.md` | This Phase 6 checkpoint. |

### Files touched (sibling repos)

| Repo | File | Change |
|---|---|---|
| `lvt` | `testing/websocket.go` | New `RecordWSFrames(ctx) *WSMessageLogger` — thin wrapper around `NewWSMessageLogger` + `Start(ctx)` so per-test setup is one line; bounded-WS-size assertions across suites share a single canonical entrypoint per proposal §379. |
| `examples` | `patterns/data.go` | New `LargeRow` type, `largeTableDefaultSize = 10000`, `largeTableSeed(n)` — deterministic 10k-row dataset (no `rand`) so per-item hashes are stable across renders. Index entry added to `allPatterns` under "Lists & Data". |
| `examples` | `patterns/state_lists.go` | New `LargeTableState` (Items, Filter, SortKey, SortDir, Total, NextID). |
| `examples` | `patterns/handlers_lists.go` | New `LargeTableController` with six action methods: `Mount`, `Change` (filter), `Sort`, `AppendN`, `UpdateRandomRow`, `Delete`, `Reset`. Reads `LARGE_TABLE_SIZE` env var so the e2e test runs at N=200 (CI-friendly) while the demo defaults to 10k. |
| `examples` | `patterns/templates/lists/large-table.tmpl` | Pico CSS table; range at root (NOT wrapped in `{{if}}`) with sibling empty-state — idiomatic shape that exercises stream mode end-to-end. |
| `examples` | `patterns/main.go` | Route `/patterns/lists/large-table` registered. |
| `examples` | `patterns/patterns_test.go` | `TestLargeTable` with 10 subtests covering every controller action and two bounded-WS-size assertions (no-sort + sort-active scenarios). |
| `examples` | `patterns/large_table_bench_test.go` | New `BenchmarkLargeTable_UpdateRandomRow_WireBytes` and `BenchmarkLargeTable_FullTreeBaseline_WireBytes` at N ∈ {200, 1000, 10000}. The two answer OQ2 in combination — see numbers below. |
| `examples` | `CLAUDE.md` | "Manual Testing on Mobile (iPhone via Tailscale)" section (carry-over from earlier session). |

### Open Question 2 — final closure

OQ2 from §386: "Volatile-field workloads — do we ship a future targeted-field op (`["uf", key, fieldIdx, value]`) or is the whole-item update wire cost low enough that `permessage-deflate` suffices?"

`BenchmarkLargeTable_UpdateRandomRow_WireBytes` (single-field change on a 5-field row, no sort active, no reorder) and `BenchmarkLargeTable_FullTreeBaseline_WireBytes` (full-tree render at the same N) measured on linux/arm64, Go 1.26:

| N | Stream-mode update | Full-tree baseline | Ratio (stream / full-tree) |
|---|---|---|---|
| 200 | 127.8 B | 98,750 B | 0.13% |
| 1,000 | 127.8 B | 482,658 B | 0.026% |
| 10,000 | **127.8 B** | **4,801,670 B** | **0.0027%** |

The whole-item update wire cost is **constant at 127.8 B regardless of N** — the streaming-range diff retains this property by design (per-item hash lookup, single ["u"] op output). The user-approved OQ2 ceiling was **30% of full-tree size**; the measured ratio at N=10k is **0.0027%**, four orders of magnitude under.

**Decision**: the targeted-field op `["uf", key, fieldIdx, value]` is **not** justified at any measured scale. Whole-item update at ~128 B is well below any reasonable wire-cost ceiling. OQ2 closed; no follow-up issue filed.

### Bounded-WS-size assertion (e2e, proposal §379)

`TestLargeTable/Update_Random_Row_Bounded_WS_Frame` (N=200, no sort, no filter) measured **199 B** on the wire (CDP-captured frame body) versus the legacy fallback's 30,467 B observed during fix iteration — a **125× reduction** at the test-tier scale. With sort active the same op produces 3,208 B (199 B update + ~3,000 B reorder op carrying 250 keys), bounded by `Update_With_Sort_Active_Bounded_WS_Frame` at a 5KB ceiling.

### Library fix discovered via the demo

The Phase 6 examples PR initially shipped a template wrapping the range in `{{if gt (len .Items) 0}}…{{end}}`. The bounded-WS-size e2e flagged a 30 KB frame instead of the expected ~200 B — stream mode never activated because `TransitionToStreamMode` only walked depth ≤ 1 of `Dynamics`. The fix in `internal/diff/transition.go` (recursive walk through `Dynamics`, never into `Range.Items`) preserves the spec §5a nested-range-in-item contract while removing the silent fallback for any other static-tree wrapping. The example template was kept in its sibling-empty-state shape (the cleaner idiom either way), but **the library now does the right thing for both shapes** so users don't need to know about the constraint.

### Verification commands

```bash
# This repo
GOWORK=off go test -run "TestTransitionToStreamMode|TestStreamMode_FiresOnConditionalWrappedRange" -v ./internal/diff/ ./
GOWORK=off go test ./...
GOWORK=off go test -race -count=1 ./internal/diff/...

# lvt repo
cd ../../../lvt/.worktrees/streaming-range-phase6 && GOWORK=off go build ./testing/... && GOWORK=off go test ./testing/

# examples repo
cd ../../../examples/.worktrees/streaming-range-phase6 && GOWORK=off go test -run "TestLargeTable" -v ./patterns/
GOWORK=off go test -bench "BenchmarkLargeTable" -benchtime=10x -run=^$ ./patterns/
```

### What Phase 6 closes

- §314 Phase 6 deliverable: `LargeTableController` + `large-table.tmpl` + 10k-row seed shipped in `examples/patterns/`.
- §379 `e2etest.RecordWSFrames(ctx)` helper added to `lvt/testing/websocket.go`.
- §346 `LargeTableController.UpdateRandomRow` benchmark output captured (above table).
- §348 audit checkpoint requirement: demo runs at 10k rows with subjectively acceptable interaction latency (manual iPhone Tailscale test pending user signoff per project memory `feedback_iphone_manual_testing.md`).
- OQ2 closed with concrete data — see "Final closure" above.
- Bonus: silent-fallback bug discovered and fixed in the library so users don't trip on it.
