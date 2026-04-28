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
| Static-structure fingerprint | To decide whether the client needs fresh statics on this render (existing `ClientNeedsStatics` flow at `internal/diff/tree_compare.go`) | One `string` |

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

After the first stream-mode render, the diff path sets `RangeData.StreamState` from the rendered items, then nils `RangeData.Items` so the retention bill is paid only by `StreamState`. The het-range fallback continues to populate `Items` and leaves `StreamState` nil — `nil` here is the unambiguous signal "this range is on the legacy full-tree path."

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

`hashItemContent(item)` is computed over the JSON-serialised dynamics map of the item's rendered TreeNode (not the raw Go struct), aligning with the existing fingerprint primitive in `internal/build/fingerprint.go`. See Open Questions for hash placement.

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

**Spec update required.** `docs/specifications/tree-update-specification.md` line 377 must be updated as part of this implementation: `["u", itemId, changes]` becomes "*MAY include unchanged fields, MUST include every dynamic position present on either the old or new item.*" The op code does not change; the payload contract relaxes from "minimal-delta" to "complete-snapshot." This is the only change to the wire spec.

**Why one path, not two.** An earlier sketch hedged with a `WithStreamThreshold(n)` toggle and a `lvt:"stream"`/`lvt:"diff"` struct tag, switching strategies based on cardinality. Pricing the costs of "always stream" honestly:

| Concern | Cost of always-stream | Verdict |
|---|---|---|
| Server memory | Uniformly cheaper at every size | **Win** |
| Wire size on no-change items | Both emit nothing | Wash |
| Wire size on whole-item changes (typical) | Both emit ~the same | Wash |
| Wire size on per-field-only changes (worst case) | ~2–5× larger; gzip absorbs most of it; absolute bytes are small | **Bounded loss** |
| Server CPU per render | FNV-1a hashing ≈ today's reflection-based field compare | Wash |
| Client CPU | Merge a slightly larger object; negligible | Wash |
| Hash collisions (FNV-1a 64) | ~10⁻⁶ at 10M items (estimate; the test plan's collision soak result becomes the documented budget) | Negligible |
| Focus / cursor preservation on inputs | Client-side morph patches at the DOM level, not at wire granularity | No change |

The threshold/tag mechanism was hedging against "small lists with frequent single-field updates" — a real workload, but the regression is bounded in absolute bytes, fully recovered by compression on most transports. Carrying two code paths for that case costs more than it saves: a tuning knob users have to learn, a struct tag that sometimes appears and sometimes doesn't, and a behavioural cliff when a list crosses the threshold mid-session. **One path is simpler and cheaper to reason about.** The proposal commits to the single path and owns the bounded wire-cost regression rather than burying it.

### 5d. Heterogeneous-range internal fallback

Ranges where item structure varies — `{{range .Items}}{{if .HasError}}<del>...</del>{{else}}<span>...</span>{{end}}{{end}}` — produce items with divergent statics. The existing diff engine handles this by emitting a full tree, gated by `ClientNeedsStatics` (`internal/diff/tree_compare.go`).

Stream mode preserves this fallback. When the per-render structural check detects heterogeneous statics, the diff path leaves `RangeData.StreamState` nil, populates `RangeData.Items` with full per-item TreeNodes, and emits the full Range tree as today. This is an internal branch, not a user-facing toggle.

## Wire Protocol

**No new op codes.** The six range ops (`a`, `p`, `i`, `r`, `u`, `o`) exist today with the semantics the proposal needs. Both switch statements in the LiveTemplate client (`state/tree-renderer.ts` in the [livetemplate/client](https://github.com/livetemplate/client) repo) handle every op the new strategy emits.

**One observable payload change.** Stream-mode `["u"]` carries a full dynamics map rather than a partial delta. The client's existing `mergeRangeItem` logic performs a key-wise merge that already accepts both partial and full payloads, so no client code change is required *provided* stream mode obeys the "always emit every dynamic position, with `""` for absent" invariant in §5c. (This is a contract on the producer; if a future implementer relaxes it without also updating the client, removals will silently fail. The test plan's "wire-format compatibility" coverage locks the invariant in.)

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

**Spec doc update.** `docs/specifications/tree-update-specification.md` line 377 ("Only changed fields are included in the changes object") is updated to reflect the relaxed `["u"]` payload contract (see §5c). Any external test or tooling that asserts the old "minimal-delta" form must be updated.

## Implementation Notes

| File | Change |
|---|---|
| `internal/build/types.go` | Add `RangeStreamState`. Add `StreamState *RangeStreamState` field on `RangeData`. The diff path nils `RangeData.Items` once `StreamState` is populated; the het-range fallback leaves `StreamState` nil and uses `Items`. |
| `internal/build/fingerprint.go` | Add `HashItemContent(*TreeNode) uint64` using the full 64-bit FNV-1a (`fnv.New64a`). Document the collision budget in code; the soak-test result becomes the documented number. |
| `internal/diff/range_ops.go` | Replace `newRangeContext`'s reliance on `oldItems` with `RangeStreamState`-driven lookup. Adjust `generateRemovalOps`/`generateUpdateOps`/`generateInsertionOps` to source comparisons from `Hashes`. `generateUpdateOps` now emits the full dynamics map for changed items, with `""` for absent positions per §5c. The heterogeneous-range branch (returning `nil` for full-tree fallback) is preserved. |
| `internal/diff/tree_compare.go` | `ClientNeedsStatics` flow is unchanged. |
| `template.go` | `Template.compareTreesAndGetChangesWithContext` is the integration point that learns about `["u"]` shape changes. `t.lastTree` continues to retain the post-render tree; for ranges, `Range.StreamState` carries the cache between renders and `Range.Items` is nilled on stream renders. |
| `docs/specifications/tree-update-specification.md` | Update the `["u", itemId, changes]` description (line 377) to reflect the relaxed payload contract. |

No public-API surface changes. No new options on `livetemplate.New(...)`. No new tags on state structs. No new methods on `*Context`. No safety-hatch flag.

## Test Plan

The implementation must add the following coverage. This list is the acceptance bar — a reviewer can use it as a checklist.

1. **Unit tests, `internal/diff/range_ops_test.go` (extend).** Diff-output goldens for each op (`a`, `p`, `i`, `r`, `u`, `o`) under the new key+hash strategy. Cover insert-at-head, insert-at-tail, mid-list insert, scattered inserts (heterogeneous fallback), pure reorder, mixed reorder + update, mass delete, mass replace. Add explicit goldens for the §5c invariant: `["u"]` payloads include every dynamic position, with `""` encoding clears.
2. **Memory regression, `internal/diff/range_memory_test.go` (new).** Construct ranges of N ∈ {10, 100, 1k, 10k} items, render twice, measure heap delta retained in `lastTree` after the second render via `runtime.ReadMemStats`. Assert per-item retained bytes drop by ≥4× vs the legacy baseline (captured as a golden number). Skip in `-short`.
3. **Hash collision soak, `internal/build/fingerprint_test.go` (extend).** Generate N = 1M synthetic items with deliberately similar dynamic payloads; assert collision count ≤ a documented ceiling. The result becomes the documented FNV-1a 64-bit collision budget for `HashItemContent` (replacing the order-of-magnitude estimate in §5c).
4. **Reconnect resync, `template_test.go` (extend).** Simulate "render → drop `lastTree` → render again with same state"; assert the second render emits a full initial tree, not stream ops. Locks in the "client re-runs Mount on reconnect" contract.
5. **Heterogeneous-range fallback, `internal/diff/range_ops_test.go`.** Range over items where some include an extra `{{if}}` branch; assert the diff falls back to full-tree replacement (`StreamState` is nil, `Items` is populated) and the resulting tree is structurally correct.
6. **Wire-format compat, `e2e_update_spec_test.go` (extend).** Existing spec tests must pass. Tests asserting a partial `["u", key, {0: "..."}]` payload are updated to assert a full dynamics map (with `""` for absent positions), *or* repurposed as regression tests for the heterogeneous-range fallback. Add a new test enforcing the §5c invariant: every position present.
7. **Browser E2E, `lvt` repo (`livetemplate_core_test.go`, extend).** Existing range-op tests must pass without modification (proves wire compat for the op codes). Add one new browser test that scripts append/delete/reorder against a 5k-row table and asserts patch latency under a documented ceiling. Confirm that whole-item `["u"]` payloads with `""` clears correctly remove text in the rendered DOM.
8. **Benchmarks, `internal/diff/range_ops_bench_test.go` (new).** `BenchmarkRangeDiff_Stream_{Append,Update,Reorder}_{Small,Medium,Large}` at N ∈ {10, 100, 10k}. The numbers in the "Memory & Performance" table above must be replaced with real benchmark output before this proposal is marked Implemented.

## Patterns Example

The implementation must ship a new pattern under the [livetemplate/examples](https://github.com/livetemplate/examples) repo's `patterns/` subtree that exercises the large-list path end-to-end. Without it, the memory and scaling wins are invisible.

- **Files.** `examples/patterns/handlers_lists.go` (extend with `LargeTableController`); `examples/patterns/templates/lists/large-table.tmpl` (new); `examples/patterns/data.go` (extend with a deterministic 10k-row seed dataset); `examples/patterns/main.go` (register route); patterns index updated under "Lists & Data".
- **Scope.** A 10,000-row table with stable keys. The controller supports: filter (text input across rows, Tier 1 `Change()` method), sort (header click, button-name routing), append-N (button), update-random-row (button), delete-by-id (button), reset (button). Between filter, sort, append, update, delete, and reset, every range op the new path emits is reachable via UI.
- **Why this scope.** `infinite-scroll` already covers append-only growth; `sortable` already covers small-list reorder. Neither exercises a list large enough to demonstrate the memory win, and neither exercises mass updates. The new example is the visible payoff for the proposal and the integration-test workhorse.
- **E2E test.** `examples/patterns/patterns_test.go` extended with `TestLargeTable_*` subtests covering filter, sort, append, update, delete, reset. All waits use `e2etest.WaitFor` (no `chromedp.Sleep`) per `examples/CLAUDE.md` E2E rules. Each subtest asserts the resulting DOM state. Asserting bounded WebSocket message size requires a small in-test logger that wraps `WebSocket.prototype.send`/`onmessage` via `chromedp.Evaluate` and exposes the captured frame sizes back to Go — this proposal commits to adding that helper to `e2etest` (e.g. `e2etest.RecordWSFrames(ctx)`) so the bounded-WS-size assertion has a single canonical implementation rather than ad-hoc copies in each test file.
- **Tier discipline.** Standard HTML wherever possible (`<form method="POST">`, button `name` attributes, `Change()` for filter input). Tier 2 `lvt-*` only where standard HTML genuinely cannot express the interaction.
- **Manual UX testing.** Pico CSS only, CSP-clean, mobile-friendly. The author tests on iPhone over Tailscale per the project's manual-testing convention; the example's acceptance criteria require it.

## Open Questions

1. **Hash function placement.** The existing primitives are `fnv.New128a()` in `internal/build/fingerprint.go` (used for structure fingerprinting) and `fnv.New64a()` *truncated to 12 hex chars / 48 bits* in `internal/keys/hash.go` (used for auto-keys, `HashPrefixLength = 12`). Neither directly fits the proposal's need for a *full* 64-bit per-item content hash. Recommendation: add a new `HashItemContent(*TreeNode) uint64` in `internal/build/fingerprint.go` using `fnv.New64a()` at full width — matching the test plan's collision budget and avoiding the 48-bit truncation. Should the structure fingerprint also be standardised on 64-bit, or kept at 128-bit-truncated-to-string for backward compat with golden files?
2. **Heterogeneous-range fallback long-term.** The proposal keeps the legacy full-tree path as the het-range branch indefinitely. Is that acceptable, or should stream mode be extended to handle structure changes via per-item statics included in the op (`["u", key, dynamics, statics?]`)?
3. **Volatile-field workloads.** A live counter on every row of a 10k-row dashboard is the worst case for whole-item updates: many updates per second, single field changing per update, large number of fields per item. Do we ship a future targeted-field op (e.g., `["uf", key, fieldIdx, value]`) as an explicit opt-in for that workload, or is `permessage-deflate` sufficient in practice? Punt to a follow-up; flag now so it isn't forgotten.

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
