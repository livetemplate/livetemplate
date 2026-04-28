# Streaming Range Rendering

**Status:** Proposed
**Date:** 2026-04-28
**Issue:** TBD

## TL;DR

**Problem:** Rendering `{{range .Items}}` retains every item's *dynamics-only* TreeNode in `lastTree.Range.Items` per connection so the diff engine can compute incremental ops. Statics are already deduplicated to a single `RangeData.Statics` array, but the per-item slice still grows linearly with list size — typically tens to a few hundred bytes per item per connection — so a 10k-row table across many viewers retains tens of MB of redundant per-item content the server doesn't actually need between renders.

**Insight:** The wire protocol already streams. `["a"]`, `["p"]`, `["i"]`, `["r"]`, `["u"]`, `["o"]` are emitted today by `GenerateRangeDifferentialOperations` and applied incrementally by the LiveTemplate client. What's missing is a server-side memory model that doesn't require holding the items between renders.

**Solution:** Replace the retained dynamics-only items with a per-range cache of `{ keys, content-hashes, fingerprint }` (~24–40 bytes/item) and emit *whole-item* `["u"]` updates when a hash changes. **One path, no toggles, no controller-facing API change.** Existing range examples (`DeleteRowController`, `ClickToLoadController`, `InfiniteScrollController`, `SortableController` in the [livetemplate/examples](https://github.com/livetemplate/examples) repo) work unchanged. Op codes unchanged. The only observable wire change is the *shape* of the `["u"]` payload (full dynamics map instead of partial-delta), which requires a corresponding update to `docs/specifications/tree-update-specification.md`.

This is closer to the spirit of [Phoenix LiveView Streams](https://fly.io/phoenix-files/phoenix-dev-blog-streams/) — *don't keep the list on the server* — but the implementation diverges: livetemplate's wire is already stream-shaped, so no new public surface (typed handles, `stream_insert`/`stream_delete`, opt-in markers) is needed. The change is purely internal to the diff/render pipeline plus a one-line tightening of the wire spec for `["u"]`.

## The Problem

`Template.lastTree` is a `*treeNode` retained on the per-connection template (`template.go`). For every range construct, that retained tree contains a `RangeData` with one shared `Statics` array and an `Items` slice of *dynamics-only* TreeNodes — statics are stripped at build time via `extractItemDynamics` (`internal/parse/range.go`). Items are individually small. The cost is in the slice: it holds every item across every active connection, and on the next render `GenerateRangeDifferentialOperations` (`internal/diff/range_ops.go`) builds a `rangeContext` from `oldItems` + `newItems`, walks both sides by key, and emits ops via `generateRemovalOps` / `generateUpdateOps` / `generateInsertionOps`.

The retention has three observable consequences:

1. **Memory scales with list size × concurrent viewer count.** A 10k-row table with ~100 B per dynamics-only item across 1k viewers retains ~1 GB across `lastTree.Range.Items` slices, plus the auxiliary `oldByKey` maps allocated per render in `newRangeContext`.
2. **Operators paginate to dodge the ceiling.** The existing patterns (`ClickToLoadController`, `InfiniteScrollController` in the examples repo) cap visible items at a page size and grow the slice on demand. Lists that genuinely want to be large (audit logs, analytics tables, chat history) hit a soft ceiling around the 10k mark before per-connection memory becomes a concern.
3. **Real-time fan-out pays the full per-render diff cost.** Even when the wire-side ops are tiny, the server still allocates and walks `oldByKey`/`newByKey` over the full slice on every broadcast, because the diff engine must reconstruct the op list by comparing the new full slice against the retained items.

The diff engine itself isn't the problem — it already emits the right ops. The problem is the *memory budget of retaining items the server doesn't need to remember*.

## First Principles

What does the server actually need between renders to emit correct ops for `{{range .Items}}`?

| Need | Why | Storage |
|---|---|---|
| Ordered list of keys | To map index → key for `["i", afterKey, item]` insertions and to detect reorder | `[]string` |
| A change-detection signal per item | To distinguish "key still present, content changed" from "key still present, unchanged" | One hash per item is sufficient |
| Static-structure fingerprint of the item template | To detect het-range transitions on subsequent renders, replacing per-item `ClientNeedsStatics` calls (which would otherwise need retained `*TreeNode`s) | One `string` |

**How `Fingerprint` interacts with `ClientNeedsStatics`.** The existing `ClientNeedsStatics(*TreeNode, *TreeNode)` flow in `internal/diff/tree_compare.go` requires both old and new `*TreeNode`s in hand to call `GetStructureFingerprint()` on each. In stream mode, `Range.Items` is `nil` after the first render — there are no per-item nodes to compare. Instead, `RangeStreamState.Fingerprint` caches the fingerprint computed on the first render's items, and on each subsequent render the diff path compares it against the new render's item-template fingerprint. If they match → stay in stream mode. If they diverge → fall back to full-tree replacement for this render (no transition back to stream mode for this range; per §5a, phase 2 → phase 3 is a one-way edge for that connection). Statics omission on insertion ops continues to use `PrepareTreeForClient(item, clientHasRangeStatics)` exactly as today, gated by the same fingerprint comparison — just sourced from `StreamState.Fingerprint` rather than the retained tree.

What does the server *not* need?

- **The dynamics of items.** When a hash mismatches, the proposal emits the whole new item's dynamics — there is no per-field diff that requires "old vs new" content comparison.
- **A history of past renders.** Reconnect goes through Mount; the cache rebuilds from the controller's first render after reconnect (no replay log).
- **Per-item statics.** Range items in homogeneous ranges share a single statics array (already true). Heterogeneous ranges fall back to full-tree replacement (see "Heterogeneous-range fallback").

This is the entire derivation, independent of any specific framework's API. Phoenix Streams arrives at the same memory contract via a typed handle (`stream/4`, `stream_insert/3`, `stream_delete/2`); livetemplate doesn't need that handle because the diff engine already maps slice mutations to the right wire ops.

## Proposed Design

### 5a. Internal `RangeStreamState`

Add a new internal type next to `RangeData` in `internal/build/types.go`, and a new field on `RangeData`:

```go
// RangeStreamState is the per-range cache retained between renders.
// It replaces RangeData.Items as the source of truth for the next diff.
type RangeStreamState struct {
    Keys        []string  // ordered keys, indexed parallel to Hashes
    Hashes      []uint64  // content hash per item (FNV-1a 64, full bits)
    Fingerprint string    // structure fingerprint of the item statics
}

type RangeData struct {
    Items       []interface{}     // populated on first render and on the het-range fallback path; nil on subsequent stream renders
    Statics     []string          // shared item-template statics (unchanged)
    StreamState *RangeStreamState // populated by stream-mode renders
}
```

After the first stream-mode render, the diff path sets `RangeData.StreamState` from the rendered items, then nils `RangeData.Items` so the retention bill is paid only by `StreamState`. The het-range fallback continues to populate `Items` and leaves `StreamState` nil.

`nil StreamState` is **not** an unambiguous "het-range" signal — it has three legitimate states the diff path must distinguish:

| Phase | `StreamState` | `Items` | Diff path behaviour |
|---|---|---|---|
| First render of a stream-capable range | `nil` (not yet built) | populated | Build full tree (initial render); after emit, populate `StreamState` from rendered items and nil `Items` |
| Subsequent renders of a stream-capable range | non-`nil` | `nil` | Stream-mode diff against `StreamState` |
| Heterogeneous range, every render | `nil` | populated | Legacy full-tree fallback via existing `ClientNeedsStatics` check |

Phase 1 and Phase 3 look identical at render *entry* (`StreamState=nil`, `Items=populated`); the disambiguation fires *after* the new tree is built. After rendering, the implementation inspects the rendered items' statics: if all items share a homogeneous statics fingerprint, populate `StreamState`, nil `Items` (phase 1 → phase 2 transition). If items have heterogeneous statics, leave both fields as-is and stay in phase 3 for this range's lifetime. The transition is a one-way edge that fires once per stream-capable range per connection. If the implementation skips the `StreamState` population step, every subsequent render silently falls back to legacy full-tree behaviour without surfacing an error — the regression tests in §11 catch this.

**Empty-range edge case.** A first render that produces zero items (`[]interface{}{}`) has no statics fingerprint to evaluate. The implementation **defers the phase 1 → phase 2 transition** in this case: leave `StreamState=nil, Items=[]`, take the legacy path (which correctly emits an empty range tree), and re-evaluate on the next render that produces items. This avoids creating a `StreamState{Keys:[], Hashes:[], Fingerprint:""}` whose empty fingerprint would mis-classify a later non-empty render as het-range. The first non-empty render runs the homogeneity check and transitions normally.

**Known constraint:** the phase 3 → phase 2 transition does not exist. A range that is heterogeneous on its first render stays on the legacy full-tree path for that connection's lifetime, even if a later render makes it homogeneous (e.g., an error-state row clears). This is acceptable because (a) the legacy path remains correct, just non-optimal; (b) most ranges are structurally stable; (c) on reconnect the classification re-runs from scratch. If future workloads make this constraint costly, a refresh path can be added without changing the wire contract.

### 5b. Stream diff procedure

On each render, given the new slice and the retained `RangeStreamState`:

```
For each item i in newItems (in order):
    k_new, h_new ← key(item), hashItemContent(item)
    if k_new not in oldKeys:
        emit ["a", [item.dynamics]] / ["p", [item.dynamics]] / ["i", afterKey, item.dynamics]
        per the existing insertion-classifier logic (and including statics/metadata
        only on first emission of the range, per the existing wire spec)
    elif h_new ≠ oldHashes[indexOf(k_new)]:
        emit ["u", k_new, item.dynamics]   # whole-item update; see §5c
    else:
        # no op; key + hash unchanged

For each k_old in oldKeys not in newKeys:
    emit ["r", k_old]

If keysets are equal but order differs:
    emit ["o", newKeys]
```

`hashItemContent(item)` resolves to `keys.ItemHashUint64(item.Dynamics)` (see §10): the input is the rendered item's `TreeNode.Dynamics` slice (`[]interface{}` — not a map; positional). The existing `keys.GenerateItemHashFromSlice([]interface{}) string` already does this with FNV-1a 64-bit, but truncates to 12 hex chars (48 bits) for `HashPrefixLength`. The new `keys.ItemHashUint64` is mechanically the same primitive but returns a full `uint64` (no truncation), avoiding the addition of a fourth hasher to the codebase. Serialisation goes through `jsonutil.API.Marshal` (`internal/jsonutil`, the project's `json-iterator` wrapper configured for `encoding/json` compatibility), which sorts map keys deterministically — but the input here is a slice, so key ordering is moot; positional ordering is enough for stability.

**Concrete `generateUpdateOps` emission loop (replacing the current `compareRangeItemsWithKeyPos`-based delta logic).** For each new item where `keys.ItemHashUint64(newItem.Dynamics) != streamState.Hashes[indexOfKey(k_new)]`:

```
fullDynamics := make(map[string]interface{}, len(newItem.Dynamics))
for i := range newItem.Dynamics {
    if newItem.Dynamics[i] == nil {
        fullDynamics[strconv.Itoa(i)] = ""   // explicit clear
    } else {
        fullDynamics[strconv.Itoa(i)] = newItem.Dynamics[i]
    }
}
emit ["u", k_new, fullDynamics]
```

No per-field comparison. The map carries every position the new item declares, with `""` for nil slots — the §5c invariant.

Insertion classification (`["a"]` for tail-only appends, `["p"]` for head-only prepends, `["i", afterKey, item]` for arbitrary mid-list inserts, with fallback to full-tree replacement when ≥4 unique insertion points are detected) is delegated to the existing `handleIncrementalInsertionsCtx` and `handleIndividualInsertionsCtx` helpers in `internal/diff/range_ops.go`. They take a `*rangeContext` today; the implementation will adapt them to source insertion classification from `RangeStreamState.Keys` rather than `oldItems`.

**Op ordering when a render produces both updates and a reorder.** Same contract as today: `["r"]` removals first, then `["u"]` updates, then `["a"]/["p"]/["i"]` insertions, finally `["o"]` reorder. The client applies them in receive order and morph-patches once. Stream mode preserves this ordering; no client-side change.

**Worked example.** A list with five items, identified by `data-key="t1".."t5"`. Between renders, item `t3` changes its body; a new item `t6` is appended. The new slice is `[t1, t2, t3', t4, t5, t6]`. The diff procedure:

- `t1, t2`: same key, same hash → no op
- `t3'`: same key, different hash → `["u", "t3", {"0": "...", "1": "...", "2": "..."}]` (full dynamics map; see §5c)
- `t4, t5`: same key, same hash → no op
- `t6`: new key, contiguous tail → `["a", [t6_dynamics]]` (statics omitted on the wire because the client already has them cached, per the existing `PrepareTreeForClient` flow)
- No reorder; no removal.

Total: **two ops** on the wire. The retained `RangeStreamState` updates to `Keys: [t1..t6], Hashes: [h1, h2, h3', h4, h5, h6]`.

### 5c. Whole-item updates: payload shape, removals, and the bounded tradeoff

Today's `["u", key, changes]` carries only the changed dynamic positions, e.g. `{"2": "new title"}`. The wire spec at `docs/specifications/tree-update-specification.md` line 377 explicitly says *"Only changed fields are included."* Stream mode breaks that contract: it emits a full dynamics map, e.g. `{"0": "unchanged author", "1": "unchanged body", "2": "new title"}`.

**How removals are encoded.** Today's delta path emits the empty string `""` for any dynamic that was cleared between renders, so the client knows to remove that text. Stream mode mirrors this: the full dynamics map always includes every position, and any position whose value is logically absent on the new item is emitted as `""`. The client's existing merge logic — which performs a key-wise merge of the payload into the current item — sees the same `""` clear it sees today, just for every position rather than only the changed ones. **No client-side merge logic change is required**, provided this "always emit every position" invariant holds.

**Spec update required.** `docs/specifications/tree-update-specification.md` (`grep "Only changed fields"`) must be updated as part of this implementation: `["u", itemId, changes]` becomes "*MAY include unchanged fields, MUST include every dynamic position present on either the old or new item.*" The op code does not change; the payload contract relaxes from "minimal-delta" to "complete-snapshot." This is the only change to the wire spec.

**Why one path, not two.** An earlier sketch hedged with a `WithStreamThreshold(n)` toggle and a `lvt:"stream"`/`lvt:"diff"` struct tag, switching strategies based on cardinality. Pricing the costs of "always stream" honestly:

| Concern | Cost of always-stream | Verdict |
|---|---|---|
| Server memory | Uniformly cheaper at every size | **Win** |
| Wire size on no-change items | Both emit nothing | Wash |
| Wire size on whole-item changes (typical) | Both emit ~the same | Wash |
| Wire size on per-field-only changes (worst case) | ~2–5× larger; `permessage-deflate` (RFC 7692, default-on for most WebSocket servers including livetemplate's) absorbs most of it; absolute bytes are small | **Bounded loss** |
| Server CPU per render | FNV-1a hashing ≈ today's reflection-based field compare | Wash |
| Client CPU | Merge a slightly larger object; negligible | Wash |
| Hash collisions (FNV-1a 64) | ~2.7×10⁻³ expected collisions at 1M items (= the soak-test scale in §11); ~0.27 at 10M items (estimate; the test plan's collision soak result becomes the documented budget) | Negligible |

**What happens when a collision actually hits.** Two items with different content hashing to the same `uint64` causes the diff engine to treat the new item as unchanged: no `["u"]` op is emitted, and the user sees stale data on that item until either (a) the item changes again to a content that hashes differently, or (b) the connection reconnects (which rebuilds `StreamState` from a fresh render). Not data loss, not a security issue, but a visible correctness divergence — and ranges often render *actionable* content (delete buttons with row IDs, edit forms with hidden inputs, checkbox values) where stale display can drive a wrong action: a user clicking "delete" on what they believe is row B may submit row A's ID. The ≤1-collision-per-1M-items soak budget is calibrated against this, not just display-only content.
| Focus / cursor preservation on inputs | Client-side morph patches at the DOM level, not at wire granularity | No change |

The threshold/tag mechanism was hedging against "small lists with frequent single-field updates" — a real workload, but the regression is bounded in absolute bytes, fully recovered by compression on most transports. Carrying two code paths for that case costs more than it saves: a tuning knob users have to learn, a struct tag that sometimes appears and sometimes doesn't, and a behavioural cliff when a list crosses the threshold mid-session. **One path is simpler and cheaper to reason about.** The proposal commits to the single path and owns the bounded wire-cost regression rather than burying it.

### 5d. Heterogeneous-range internal fallback

Ranges where item structure varies — `{{range .Items}}{{if .HasError}}<del>...</del>{{else}}<span>...</span>{{end}}{{end}}` — produce items with divergent statics. The existing diff engine handles this by emitting a full tree, gated by `ClientNeedsStatics` (`internal/diff/tree_compare.go`).

Stream mode preserves this fallback. When the per-render structural check detects heterogeneous statics, the diff path leaves `RangeData.StreamState` nil, populates `RangeData.Items` with full per-item TreeNodes, and emits the full Range tree as today. This is an internal branch, not a user-facing toggle.

## Wire Protocol

**No new op codes, but the `["u"]` payload contract changes** from partial-delta to full-item-snapshot — see §5c for the new shape and the `""`-for-absent invariant, and §8 for the corresponding update to `docs/specifications/tree-update-specification.md`. The six range ops (`a`, `p`, `i`, `r`, `u`, `o`) exist today with the semantics the proposal needs. Both switch statements in the LiveTemplate client (`state/tree-renderer.ts` in the [livetemplate/client](https://github.com/livetemplate/client) repo) handle every op the new strategy emits.

**One observable payload change.** Stream-mode `["u"]` carries a full dynamics map rather than a partial delta. The client's existing `mergeRangeItem` logic *appears* to perform a key-wise merge that accepts both partial and full payloads, so no client code change is required *provided* stream mode obeys the "always emit every dynamic position, with `""` for absent" invariant in §5c. (This is a contract on the producer; if a future implementer relaxes it without also updating the client, removals will silently fail. The test plan's "wire-format compatibility" coverage locks the invariant in.)

### Pre-implementation verification gate

The "no client code change required" claim is load-bearing under the hard-cutover policy (§8: no safety hatch, recovery-by-revert). Before the implementation PR opens, complete this checklist:

- [ ] Grep `state/tree-renderer.ts` in the [livetemplate/client](https://github.com/livetemplate/client) repo for `mergeRangeItem`. Read the function body. Confirm it performs key-wise merge of the payload object into the current item.
- [ ] Confirm `mergeRangeItem` treats an empty-string value (`""`) as a clear/replace, not a skip.
- [ ] Confirm no client-side path discards "missing" keys — the wire contract is that *every* dynamic position is present on stream-mode `["u"]`, but the client must still tolerate today's partial-delta form for the het-range fallback path.

If any of these checks fail, the proposal needs a corresponding client-side change documented before the server implementation lands.

**This is a PR prerequisite, not a soft expectation.** The implementation PR description should link to the grep output from the [livetemplate/client](https://github.com/livetemplate/client) repo (commit hash + line numbers + the `mergeRangeItem` body) so reviewers don't have to re-perform the verification. Under the hard-cutover policy (§8), a missed client-side incompatibility ships broken to all users on the first release after merge.

`docs/specifications/tree-update-specification.md` is updated to reflect the new `["u"]` payload contract (see §5c). This is the only spec change.

## Memory & Performance

Estimates, to be replaced with measured numbers from the implementation's benchmarks. Order-of-magnitude only.

| Cardinality | Legacy retained memory | Stream retained memory | Wire size on append-1 | Wire size on update-1-field |
|---|---|---|---|---|
| 10 items × 3 fields | ~1.5 KB | ~0.3 KB | identical | legacy ~50 B / stream ~150 B |
| 100 items × 3 fields | ~15 KB | ~3 KB | identical | legacy ~50 B / stream ~150 B |
| 10k items × 3 fields | ~1.5 MB | ~300 KB | identical | legacy ~50 B / stream ~150 B |

Per-connection retained memory drops by roughly 5×. Wire size on the worst-case workload (single-field updates on multi-field items) grows by roughly 3× in absolute bytes; gzip on the WebSocket recovers most of it. Wire size on inserts, deletes, reorders, and unchanged renders is identical.

## Reconnect & Resync

Client reconnect re-runs `Mount` via the existing `mount.go` callMount path (`callMount(h.config.Controller, connSt.state, lifecycleCtx)` in the WS connect and HTTP GET paths). The server discards `lastTree` on a fresh handler and rebuilds `RangeStreamState` from the controller's first post-reconnect render. This matches today's behaviour; no replay log, no snapshot, no versioning. Mention is included only because the proposal explicitly does *not* introduce reconnect semantics.

## Migration

**Zero migration for user code.** The change is internal. Existing controllers in the examples repo (`DeleteRowController`, `ClickToLoadController`, `InfiniteScrollController`, `SortableController` in `examples/patterns/handlers_lists.go` of the [livetemplate/examples](https://github.com/livetemplate/examples) repo) require no edits. Templates require no edits. There is no public livetemplate-server API to deprecate.

**Hard cutover, no safety hatch.** No `WithLegacyRangeDiff(true)` or equivalent shims. The legacy delta path is deleted in the same PR that lands the stream path; recovery from a defect is by reverting the implementation PR rather than by runtime opt-out. Justification: the change has no controller-facing surface, the test coverage in §11 is comprehensive, and a temporary public-API knob would itself need to be deprecated and removed later.

**Spec doc update.** `docs/specifications/tree-update-specification.md` (`grep "Only changed fields"` — currently asserts "Only changed fields are included in the changes object") is updated to reflect the relaxed `["u"]` payload contract (see §5c). Any external test or tooling that asserts the old "minimal-delta" form must be updated.

## Implementation Notes

| File | Change |
|---|---|
| `internal/build/types.go` | Add `RangeStreamState`. Add `StreamState *RangeStreamState` field on `RangeData`. The diff path nils `RangeData.Items` once `StreamState` is populated; the het-range fallback leaves `StreamState` nil and uses `Items`. `NewRangeData` (the existing `RangeData` constructor) requires no change — `StreamState` starts `nil` on every construction path and is set later by the diff path's first stream-mode render. **`TreeNode.Clone()`** (currently deep-copies `RangeData.Items`/`Statics`) is extended to deep-copy `StreamState.Keys` and `StreamState.Hashes` slices — a shallow pointer copy would alias the backing arrays across clones and break per-connection independence. `StreamState.Fingerprint` is a `string` (immutable in Go), so a value copy is safe; no special handling. **`TreeNode.MarshalJSON`** has two emission sites for `"d"` and they fail differently when `Items == nil`: the direct path (`result["d"] = tn.Range.Items`) emits `"d": null`, while the deep-convert path in `convertValueToMap` (`make([]interface{}, len(tn.Range.Items))` then `range tn.Range.Items`) emits `"d": []` (because `len(nil) == 0`). Both are wrong per the proposal's contract. Both must be updated: if `Range.Items == nil && Range.StreamState != nil`, omit the `"d"` field entirely. **Intent: defensive, not load-bearing.** Retained stream-mode trees are not expected to reach the wire (the diff path emits ops, not full trees). However, marshaling can happen for debug logging, test fixtures, or future code paths the proposal doesn't anticipate; emitting `"d": null` or `"d": []` instead of the omitted-field shape would corrupt those uses. The fix is belt-and-suspenders. (An implementer who prefers fail-fast can emit a `panic("BUG: stream-mode RangeData marshaled")` here in dev/test builds — both shapes are acceptable.) The §11 test for this case must cover **both** code paths, not just one. (`Statics` copy in `Clone()` is unchanged — it's already deep-copied today and remains so; called out only to prevent a refactor from accidentally dropping it.) |
| `internal/keys/hash.go` | Add `ItemHashUint64(dynamics []interface{}) uint64` (full 64-bit FNV-1a, no truncation) co-located with the existing `GenerateItemHashFromSlice` to prevent cross-package duplication divergence. The new function shares the underlying FNV-1a 64-bit primitive with `GenerateItemHashFromSlice`; only the truncation step and return type differ. **Nil-skipping behaviour MUST match the existing primitive** (`GenerateItemHashFromSlice` skips entries where `val == nil` via `if val == nil { continue }` — verified in `hash.go`). This is consistent with stream-mode wire encoding, where nil positions are emitted as `""` in the `["u"]` payload (§5c): a slice transitioning from `nil` to `""` at a position changes the rendered HTML to be the same empty content but the hash *will* differ (because `""` is hashed while `nil` is skipped) — that's a phantom no-op-render but a real wire emission, which is acceptable. Document this explicitly in `ItemHashUint64`'s godoc to prevent a future implementer from "simplifying" the nil-skip away. `internal/build` calls `keys.ItemHashUint64` to populate `RangeStreamState.Hashes`. Document the collision budget in code; the soak-test result becomes the documented number. |
| `internal/diff/range_ops.go` | Replace `newRangeContext`'s reliance on `oldItems` (`[]interface{}` → `oldByKey map[string]interface{}` lookup) with `RangeStreamState`-driven lookup (`StreamState.Keys[i] → StreamState.Hashes[i]`). The whole "old side" of the comparison switches from interface-typed items to keyed hashes. **`extractRangeData` currently takes `(oldValue, newValue interface{})` and reads `oldNode.Range.Items` directly.** The implementation **commits to a new top-level entry** alongside `GenerateRangeDifferentialOperations` (proposed name `GenerateRangeStreamOperations(streamState *RangeStreamState, newItems []interface{}, statics interface{}, metadata map[string]interface{}, stripStatics bool) []interface{}`) that takes `*RangeStreamState` directly. The current `extractRangeData` is left untouched as the legacy/het-range path; the caller (in `tree_compare.go`) chooses between the two entries based on `oldNode.Range.StreamState != nil`. This keeps the stream/legacy split structurally obvious rather than threading both paths through one extractor. Adjust `generateRemovalOps`/`generateUpdateOps`/`generateInsertionOps` to source comparisons from `Hashes`. `generateUpdateOps` now emits the full dynamics map for changed items, with `""` for absent positions per §5c — note that the existing implementation sorts keys with `sort.Strings(sortedNewKeys)` before emitting, while the stream loop iterates in *insertion order* (the new natural order); `["u"]` ops are order-agnostic on the client, but golden files in `e2e_update_spec_test.go` will need regeneration to reflect the new emission order. Adapt `handleIncrementalInsertionsCtx` and `handleIndividualInsertionsCtx` to consume `StreamState.Keys` rather than `oldItems` for insertion-point classification. The exported `CompareRangeItemsForChanges` (~line 327) — which today wraps `compareRangeItemsWithKeyPos` for partial-delta comparison — is no longer called by `generateUpdateOps` after this change. The het-range fallback emits full Range trees, not partial deltas, so `CompareRangeItemsForChanges` becomes dead code; delete it (and its private helper `compareRangeItemsWithKeyPos` if no other callers remain) **alongside its dedicated test suite** (`TestCompareRangeItemsForChanges_NoDiff`, `_NilInputs`, `_Changed`, `_NonTreeNodeToTreeNode`, `_NonExistentToTreeNode` in `range_ops_test.go`) as part of the same PR rather than leaving stale exports or orphaned tests. The heterogeneous-range branch (returning `nil` for full-tree fallback) is preserved. |
| `internal/diff/helpers_range.go` | **`isComplexInsertionPatternCtx`** (with `maxInsertionPoints = 3`, so triggers at ≥4 unique insertion points) is called from `GenerateRangeDifferentialOperations` *before* the per-op helpers run, and currently builds `insertionPoints` from `ctx.newItems` (which is seeded from `RangeData.Items`). In stream mode `Items` is nil, so this check must source insertion points from `StreamState.Keys` instead. Without this adaptation, scattered insertions on stream-mode ranges silently emit individual `"i"` ops past the threshold instead of falling back to full-tree replacement — a real correctness divergence from today's behaviour. The same adaptation applies to `areAllItemsAtStartCtx` and `areAllItemsAtEndCtx` (both iterate `ctx.newItems` directly to classify all-prepend / all-append patterns; both feed the same fallback decision). All three helpers need to be adapted together — adapting only `isComplexInsertionPatternCtx` while leaving the other two reading from `ctx.newItems` would compile and pass insertion-classification unit tests but silently misbehave on the start/end fast paths. **Adaptation strategy committed:** extend `rangeContext` so the "old side" can be sourced from either `oldItems` (legacy path) or `StreamState` (stream path). Specifically: populate `ctx.oldKeys []string` and `ctx.oldByKey map[string]struct{}` (presence-only — no item bodies needed) from `StreamState.Keys` on stream renders. Inside the three helpers, replace early-return guards of the form `len(ctx.oldItems) == 0` with `len(ctx.oldByKey) == 0` (the presence-only map is populated on both paths, so this guard is uniform). The membership check `_, exists := ctx.oldByKey[itemKey]` (used by `areAllItemsAtEndCtx` / `areAllItemsAtStartCtx` at the inner loop) keeps its current semantics — presence is all that's needed; old item bodies are never read. **Do not leave `ctx.oldByKey` empty on the stream path** — that was an earlier incorrect adaptation sketch; it would cause `areAllItemsAtEndCtx`'s early-return to fire on every stream render and silently regress N≥4 tail-append workloads (e.g. `InfiniteScrollController` once it grows past 4 items per page) to full-tree replacement. Test plan §11 item 1 includes an explicit ≥4-item tail-append golden for stream mode to catch this. |
| `internal/diff/tree_compare.go` | `ClientNeedsStatics` flow is unchanged. |
| `template.go` | `Template.compareTreesAndGetChangesWithContext` (`grep "func.*compareTreesAndGetChangesWithContext"`) is the integration point — the wrapper `compareTreesAndGetChanges` is a one-line passthrough; the dispatch between `GenerateRangeStreamOperations` and `GenerateRangeDifferentialOperations` belongs in the `WithContext` variant, not the wrapper. `t.lastTree` continues to retain the post-render tree; for ranges, `Range.StreamState` carries the cache between renders and `Range.Items` is nilled on stream renders. The phase-1→phase-2 transition (`StreamState` populated, then `Items` nilled) must happen atomically under the existing `t.mu` lock — a concurrent `Clone()` interleaving between the two steps would deep-copy both `StreamState` and a non-nil `Items`, producing an inconsistent clone. |
| `internal/keys/loader.go` | **`LoadExistingKeyMappings`** (`grep "func LoadExistingKeyMappings"`) calls `gen.LoadExistingKeys(node.Range.Items)` directly. In stream mode `Range.Items` is `nil`, so `LoadExistingKeys(nil)` is a Go no-op (range over nil) — the auto-key generator counter stays at 0 and the next render restarts numeric keys at "1", silently re-assigning keys that already exist in `StreamState.Keys`. This breaks key stability for ranges using the default sequential auto-keys (anything without `data-key`/`id` attributes). The fix walks `StreamState.Keys` and bumps the counter past any numeric key that originated as an auto-key. **Numeric `data-key` values are NOT counted** (e.g. `data-key="{{.ID}}"` with a database integer like `5000` would otherwise push the counter to 5001 and produce surprising auto-keys for ranges that mix explicit numeric IDs with auto-keyed items). To distinguish them, the existing key generator already tracks which keys it issued; load only those keys back into the counter. For ranges using auto-keys exclusively, this is the same behaviour as today's items-based path. For ranges using `data-key` exclusively, both paths are no-ops. For mixed ranges, the implementation must consult the generator's own issued-keys set — not blindly trust `strconv.Atoi` on every entry. |
| `internal/fuzz/invariants/verifier.go` | **`buildIDKeyMap`** (`grep "func buildIDKeyMap"`) reads `rangeData.Items` to construct ID→key maps used in invariant checks. In stream mode `Items` is `nil`, so the function returns an empty map and range invariants are silently vacuous for stream-mode ranges — the fuzz harness loses coverage. The fix: detect `rangeData.StreamState != nil` and build the ID→key map from `StreamState.Keys` instead. Also update the related `range tree.Range.Items` iteration site (`grep "for i, item := range tree.Range.Items"`) to take the stream path when applicable. The fuzz corpus should also gain entries that exercise stream-mode ranges (`Range.Items==nil && Range.StreamState!=nil`); without them the harness has no coverage of the new branch. |
| `docs/specifications/tree-update-specification.md` | Update the `["u", itemId, changes]` description (`grep "Only changed fields"` — currently asserts "Only changed fields are included in the changes object") to reflect the relaxed payload contract. |
| `e2e_update_spec_test.go`, `testdata/golden/`, `fuzz_diff_test.go` | Regenerate any golden files that assert the old "minimal-delta" `["u"]` payload form; the implementation PR must land golden updates atomically with the diff-engine change, otherwise the test suite will fail on the same commit that flips the payload shape. `fuzz_diff_test.go` exercises the range diff path via `tmpl.lastTree` + `compareTreesAndGetChanges` (10+ test functions, not golden-file-driven) — it must remain green, and the existing tests are part of the "all existing tests pass" gate. |

No public-API surface changes. No new options on `livetemplate.New(...)`. No new tags on state structs. No new methods on `*Context`. No safety-hatch flag.

## Test Plan

The implementation must add the following coverage. This list is the acceptance bar — a reviewer can use it as a checklist.

1. **Unit tests, `internal/diff/range_ops_test.go` (extend).** Diff-output goldens for each op (`a`, `p`, `i`, `r`, `u`, `o`) under the new key+hash strategy. Cover insert-at-head, insert-at-tail, mid-list insert, pure reorder, mixed reorder + update, mass delete, mass replace. Add explicit goldens for the §5c invariant: `["u"]` payloads include every dynamic position, with `""` encoding clears. Add a first-render golden (no prior `StreamState`) to lock in the "first render emits full tree, not stream ops" invariant from §5a's three-state lifecycle table. **Add a stream-mode scattered-insertion fallback golden:** a render with ≥4 unique insertion points must trigger `isComplexInsertionPatternCtx` and fall back to full-tree replacement, NOT emit a sequence of individual `["i"]` ops. (This is distinct from the het-range-statics fallback below — same statics, many distinct insertion positions.) Also add a het-range structural-fingerprint fallback (different item statics) as a separate case. **Add an N≥4 tail-append golden** that asserts the result is a single `["a", items]` op (not a full-tree fallback) — guards against the `areAllItemsAtEndCtx` early-return regression flagged in §10's helpers_range row. **Add a mixed-op golden:** a single render that produces both an item-content change AND a reorder must emit `["u", k, ...]` *before* `["o", newKeys]` per the §5b ordering contract; the test pins down the emission order so a future implementation refactor can't silently flip it.
2. **Memory regression, `internal/diff/range_memory_test.go` (new).** Construct ranges of N ∈ {10, 100, 1k, 10k} items, render twice, measure heap delta retained in `lastTree` after the second render via `runtime.ReadMemStats`. Assert per-item retained bytes drop by ≥4× vs the legacy baseline (captured as a golden number). Skip in `-short`.
3. **Hash collision soak, `internal/keys/hash_test.go` (extend, co-located with `ItemHashUint64`).** Generate N = 1M synthetic items with deliberately similar dynamic payloads using a **fixed PRNG seed** (e.g., `rand.New(rand.NewSource(0xLVT_SEED))`) so the test is deterministic — no flake-on-bad-seed risk. Assert collision count ≤ **1** (provisional ceiling — the implementation PR may revise downward based on observed soak results, but not upward). For context: the birthday-bound *expected number of collisions* for FNV-1a 64-bit at N=1M is N²/2⁶⁵ ≈ 0.027, so the empirical ceiling is intentionally conservative (≤1 is far above the expected ~0). The soak-test result becomes the documented FNV-1a 64-bit collision budget for `keys.ItemHashUint64` (replacing the order-of-magnitude estimate in §5c).
4. **Reconnect resync, `template_test.go` (extend).** Simulate "render → drop `lastTree` → render again with same state"; assert the second render emits a full initial tree, not stream ops. Locks in the "client re-runs Mount on reconnect" contract.
5. **Heterogeneous-range fallback, `internal/diff/range_ops_test.go`.** Range over items where some include an extra `{{if}}` branch; assert the diff falls back to full-tree replacement (`StreamState` is nil, `Items` is populated) and the resulting tree is structurally correct.
6. **Clone independence, `internal/build/types_test.go` (extend).** `TreeNode.Clone()` of a stream-mode tree produces independent `StreamState.Keys` and `StreamState.Hashes` slices — mutating one clone's slices must not affect the other. (Mirrors the existing `make + copy` pattern Clone already uses for `Items`; this just extends it to two more slices.)
7. **Wire-format with nil Items, `internal/build/types_test.go` (extend).** A `TreeNode` with `Range != nil`, `Range.Items == nil`, `Range.StreamState != nil` marshals to valid wire JSON (no `"d": null`, no `"d": []`, no panic, omitted-`"d"` shape per §10). Cover **both** code paths — the direct `MarshalJSON` site that today emits `result["d"] = tn.Range.Items` *and* the deep-convert `convertValueToMap` site that today emits `result["d"] = convertedItems`. Each fails differently when `Items == nil` (`null` vs `[]`), so a test that exercises only one site will silently leave the other broken.
8. **`extractRangeData` regression gate, `internal/diff/range_ops_test.go` (extend).** Phase 2 render (where retained `Range.Items == nil` and `Range.StreamState != nil`) MUST NOT take the `handleEmptyToItemsTransition` path. The routing mechanism this test pins down is the caller-side guard in `tree_compare.go`: `if oldNode.Range.StreamState != nil { call GenerateRangeStreamOperations } else { call GenerateRangeDifferentialOperations }`. Assert by inspecting the emitted ops or by counting calls to `extractRangeData`/`handleEmptyToItemsTransition` (both should be zero on Phase 2 renders). Catches the silent regression flagged in §10.
9. **Wire-format compat, `e2e_update_spec_test.go` (extend).** Existing spec tests *other than* `["u"]`-shape assertions must pass unchanged (this proves op-code compat). Tests asserting a partial `["u", key, {0: "..."}]` payload are updated to assert a full dynamics map (with `""` for absent positions), *or* repurposed as regression tests for the heterogeneous-range fallback. Add a new test enforcing the §5c invariant: every position present.
10. **Browser E2E, `lvt` repo (`livetemplate_core_test.go`, extend).** Existing range-op tests must pass without modification (proves wire compat for the op codes). Add one new browser test that scripts append/delete/reorder against a 5k-row table and asserts patch latency under a documented ceiling. Confirm that whole-item `["u"]` payloads with `""` clears correctly remove text in the rendered DOM.
11. **Benchmarks, `internal/diff/range_ops_bench_test.go` (new).** `BenchmarkRangeDiff_Stream_{Append,Update,Reorder}_{Small,Medium,Large}` at N ∈ {10, 100, 10k}. The numbers in the "Memory & Performance" table above are replaced with real benchmark output **as part of the implementation PR, not a follow-up**. Estimates do not survive the implementation PR — that is the gate before this proposal is marked Implemented.

## Patterns Example

The implementation must ship a new pattern under the [livetemplate/examples](https://github.com/livetemplate/examples) repo's `patterns/` subtree that exercises the large-list path end-to-end. Without it, the memory and scaling wins are invisible.

- **Files.** `examples/patterns/handlers_lists.go` (extend with `LargeTableController`); `examples/patterns/templates/lists/large-table.tmpl` (new); `examples/patterns/data.go` (extend with a deterministic 10k-row seed dataset); `examples/patterns/main.go` (register route); patterns index updated under "Lists & Data".
- **Scope.** A 10,000-row table with stable keys. The controller supports: filter (text input across rows, Tier 1 `Change()` method), sort (header click, button-name routing), append-N (button), update-random-row (button), delete-by-id (button), reset (button). Between filter, sort, append, update, delete, and reset, every range op the new path emits is reachable via UI.
- **Why this scope.** `infinite-scroll` already covers append-only growth; `sortable` already covers small-list reorder. Neither exercises a list large enough to demonstrate the memory win, and neither exercises mass updates. The new example is the visible payoff for the proposal and the integration-test workhorse.
- **E2E test.** `examples/patterns/patterns_test.go` extended with `TestLargeTable_*` subtests covering filter, sort, append, update, delete, reset. All waits use `e2etest.WaitFor` (no `chromedp.Sleep`) per `examples/CLAUDE.md` E2E rules. Each subtest asserts the resulting DOM state. Asserting bounded WebSocket message size requires a small in-test logger that wraps `WebSocket.prototype.send`/`onmessage` via `chromedp.Evaluate` and exposes the captured frame sizes back to Go — this proposal commits to adding that helper to `e2etest` (e.g. `e2etest.RecordWSFrames(ctx)`) so the bounded-WS-size assertion has a single canonical implementation rather than ad-hoc copies in each test file.
- **Tier discipline.** Standard HTML wherever possible (`<form method="POST">`, button `name` attributes, `Change()` for filter input). Tier 2 `lvt-*` only where standard HTML genuinely cannot express the interaction.
- **Manual UX testing.** Pico CSS only, CSP-clean, mobile-friendly. The author tests on iPhone over Tailscale per the project's manual-testing convention; the example's acceptance criteria require it.

## Open Questions

1. **Heterogeneous-range fallback long-term.** The proposal keeps the legacy full-tree path as the het-range branch indefinitely. Is that acceptable, or should stream mode be extended to handle structure changes via per-item statics included in the op (`["u", key, dynamics, statics?]`)?
2. **Volatile-field workloads.** A live counter on every row of a 10k-row dashboard is the worst case for whole-item updates: many updates per second, single field changing per update, large number of fields per item. Do we ship a future targeted-field op (e.g., `["uf", key, fieldIdx, value]`) as an explicit opt-in for that workload, or is `permessage-deflate` sufficient in practice? **The `LargeTableController.UpdateRandomRow` benchmark in §12 (combined with item 11's update benchmarks at N ∈ {10, 100, 10k}) produces the empirical data to answer this question** — if its numbers stay within an acceptable wire-cost ceiling, the targeted-field op is unnecessary. Defer the decision to that data; flag now so it isn't forgotten.

### Resolved during review

- **Hash function placement (closed).** Add `keys.ItemHashUint64(dynamics []interface{}) uint64` to `internal/keys/hash.go` — co-located with `GenerateItemHashFromSlice` (both share the FNV-1a 64-bit primitive; `ItemHashUint64` skips the 48-bit truncation and returns the full `uint64`). `internal/build` calls into `keys.ItemHashUint64` to populate `RangeStreamState.Hashes`. Keep the existing 128-bit `CalculateStructureFingerprint` in `internal/build/fingerprint.go` as-is — its width is irrelevant to correctness (it's stored as a string and compared with `==`), and changing it would force a golden-file rewrite for no functional gain.
- **Migration safety hatch (closed).** No `WithLegacyRangeDiff(true)` shim. Hard cutover, with recovery-by-revert. See §8 Migration.
- **`StreamState` nil ambiguity (closed).** §5a now spells out the three-state lifecycle (first render / subsequent stream renders / het-range) explicitly.

## References

- [Phoenix Dev Blog: Streams](https://fly.io/phoenix-files/phoenix-dev-blog-streams/) — the "don't keep the list on the server" insight.
- `internal/diff/range_ops.go` — `GenerateRangeDifferentialOperations`, `rangeContext`, `generateRemovalOps`, `generateUpdateOps`, `generateInsertionOps`.
- `internal/diff/tree_compare.go` — `ClientNeedsStatics`, the structure-fingerprint flow.
- `internal/parse/range.go` — `extractItemDynamics` (statics stripping at build time, the reason `Items` is dynamics-only today).
- `internal/build/fingerprint.go` — existing FNV-1a 128-bit structure fingerprint primitive.
- `internal/keys/hash.go` — existing FNV-1a 64-bit auto-key hash (truncated to 48 bits).
- `template.go` — `Template.compareTreesAndGetChangesWithContext` (the integration point) and `Template.lastTree` retention.
- `docs/specifications/tree-update-specification.md` — wire spec, line 377 (the `["u"]` op contract that this proposal updates).
- `state/tree-renderer.ts` in the [livetemplate/client](https://github.com/livetemplate/client) repo — case `"a"`/`"p"`/`"i"`/`"r"`/`"u"`/`"o"` handlers and `mergeRangeItem`.
- `examples/patterns/handlers_lists.go` in the [livetemplate/examples](https://github.com/livetemplate/examples) repo — `DeleteRowController`, `ClickToLoadController`, `InfiniteScrollController`, `SortableController`.
- `docs/proposals/tier1-file-uploads-proposal.md`, `docs/proposals/lifecycle-hooks-proposal.md` — house style.
