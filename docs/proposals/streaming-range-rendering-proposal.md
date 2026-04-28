# Streaming Range Rendering

**Status:** Proposed
**Date:** 2026-04-28
**Issue:** TBD

## TL;DR

**Problem:** Rendering `{{range .Items}}` retains every item's rendered TreeNode in `lastTree.Range.Items` per connection so the diff engine can compute incremental ops. Memory grows linearly with list size — roughly 150–200 bytes per item per connection — capping practical lists at ~10k items per active session.

**Insight:** The wire protocol already streams. `["a"]`, `["p"]`, `["i"]`, `["r"]`, `["u"]`, `["o"]` are emitted today by `GenerateRangeDifferentialOperations` and applied incrementally by `client/state/tree-renderer.ts`. What's missing is a server-side memory model that doesn't require holding the items between renders.

**Solution:** Replace per-item retention with a per-range cache of `{ keys, content-hashes, fingerprint }` (~24–40 bytes/item) and emit whole-item updates when a hash changes. **One path, no toggles, no controller-facing API change.** Existing examples (`DeleteRow`, `ClickToLoad`, `InfiniteScroll`, `Sortable`) work unchanged. Wire format unchanged. Client unchanged.

This is closer to the spirit of [Phoenix LiveView Streams](https://fly.io/phoenix-files/phoenix-dev-blog-streams/) — *don't keep the list on the server* — but the implementation diverges: livetemplate's wire is already stream-shaped, so no new public surface (typed handles, `stream_insert`/`stream_delete`, opt-in markers) is needed. The change is purely internal.

## The Problem

`Template.lastTree` is a `*treeNode` retained on the per-connection template (`template.go:231`). For every range construct, that retained tree contains a `RangeData` whose `Items` slice holds the rendered TreeNode for each item, including statics references and per-item dynamics. On the next render, `GenerateRangeDifferentialOperations` (`internal/diff/range_ops.go:77`) builds a `rangeContext` from `oldItems` + `newItems`, walks both sides by key, and emits ops via `generateRemovalOps` / `generateUpdateOps` / `generateInsertionOps`.

The retention has three observable consequences:

1. **Memory scales with list size × connection count.** A 10k-row table with 1k concurrent viewers retains ~10k × 1k × ~150B ≈ 1.5 GB across `lastTree.Range.Items` slices, plus the auxiliary `oldByKey` maps allocated per render in `newRangeContext`.
2. **Operators paginate to dodge the ceiling.** `examples/patterns/handlers_lists.go` shows the workaround: `ClickToLoadController` and `InfiniteScrollController` cap visible items at a page size and grow the slice on demand. Lists that genuinely want to be large (audit logs, analytics tables, chat history) hit a soft ceiling around the 10k mark before per-connection memory becomes a concern.
3. **Real-time fan-out is bounded by the per-connection diff cost.** Even when the wire-side ops are tiny, the server pays the full per-render diff cost on each broadcast, because the diff engine must reconstruct the op list by comparing the new full slice against the retained items.

The diff engine itself isn't the problem — the diff is already efficient and emits the right ops. The problem is the *memory budget of retaining items the server doesn't need to remember*.

## First Principles

What does the server actually need between renders to emit correct ops for `{{range .Items}}`?

| Need | Why | Storage |
|---|---|---|
| Ordered list of keys | To map index→key for `["i", afterKey, item]` insertions and to detect reorder | `[]string` |
| A change-detection signal per item | To distinguish "key still present, content changed" from "key still present, unchanged" | One hash per item is sufficient |
| Static-structure fingerprint | To decide whether the client needs fresh statics on this render (existing `ClientNeedsStatics` flow at `internal/diff/tree_compare.go:458`) | One `string` |

What does the server *not* need?

- **The rendered content of items.** When a hash mismatches, the proposal emits the whole new item — there is no per-field diff that requires "old vs new" content comparison.
- **A history of past renders.** Reconnect goes through Mount; the cache rebuilds from the controller's first render after reconnect (no replay log).
- **Per-item statics.** Range items in homogeneous ranges share a single statics array. Heterogeneous ranges fall back to full-tree replacement (see "Heterogeneous-range fallback").

This is the entire derivation, independent of any specific framework's API. Phoenix Streams arrives at the same memory contract by way of a typed handle (`stream/4`, `stream_insert/3`, `stream_delete/2`); livetemplate doesn't need that handle because the diff engine already maps slice mutations to the right wire ops.

## Proposed Design

### 5a. Internal `RangeStreamState`

Add a new internal type next to `RangeData` in `internal/build/types.go`:

```go
// RangeStreamState is the per-range cache retained between renders.
// It replaces RangeData.Items as the source of truth for the next diff.
type RangeStreamState struct {
    Keys        []string  // ordered keys, indexed parallel to Hashes
    Hashes      []uint64  // content hash per item (FNV-1a 64)
    Fingerprint string    // structure fingerprint of the item statics
}
```

The retained `lastTree.Range` for each range construct holds a `*RangeStreamState` instead of (or alongside, during a transition) `Items`. Memory per item drops from ~150–200 bytes (rendered TreeNode + dynamics map + slice overhead) to ~24–40 bytes (one `string` for the key plus one `uint64` for the hash, in two parallel slices).

### 5b. Stream diff procedure

On each render, given the new slice and the retained `RangeStreamState`:

```
For each item i in newItems:
    k_new, h_new ← key(item), hash(item)
    if k_new not in oldKeys:
        emit ["a", item] / ["p", item] / ["i", afterKey, item] per existing insertion-classifier logic
    elif h_new ≠ oldHashes[indexOf(k_new)]:
        emit ["u", k_new, item]   # whole-item update
    else:
        # no op; key + hash unchanged

For each k_old in oldKeys not in newKeys:
    emit ["r", k_old]

If keysets are equal but order differs:
    emit ["o", newKeys]
```

**Worked example.** A list with five items, identified by `data-key="t1".."t5"`. Between renders, item `t3` changes its body; a new item `t6` is appended. The new slice is `[t1, t2, t3', t4, t5, t6]`. The diff procedure:

- `t1, t2`: same key, same hash → no op
- `t3'`: same key, different hash → `["u", "t3", t3']`
- `t4, t5`: same key, same hash → no op
- `t6`: new key, contiguous tail → `["a", [t6]]`
- No reorder; no removal.

Total: **two ops** on the wire. The retained `RangeStreamState` updates to `Keys: [t1..t6], Hashes: [h1, h2, h3', h4, h5, h6]`.

### 5c. Whole-item updates: the bounded tradeoff

Today's `["u", key, dynamicsDelta]` carries only the changed dynamic fields, e.g. `{"0": "new title"}`. Stream mode emits the whole item's dynamics, e.g. `{"0": "new title", "1": "unchanged body", "2": "unchanged author"}`. For an item with `f` dynamic fields, of which `c` actually changed, the wire delta grows by ~`(f − c)/c`× — bounded above by the field count and recovered substantially by the WebSocket `permessage-deflate` extension (typical 4–10× compression on repeated text).

**Why one path, not two.** An earlier sketch hedged with a `WithStreamThreshold(n)` toggle and a `lvt:"stream"`/`lvt:"diff"` struct tag, switching strategies based on cardinality. Pricing the costs of "always stream" honestly:

| Concern | Cost of always-stream | Verdict |
|---|---|---|
| Server memory | Uniformly cheaper at every size | **Win** |
| Wire size on no-change items | Both emit nothing | Wash |
| Wire size on whole-item changes (typical) | Both emit ~the same | Wash |
| Wire size on per-field-only changes (worst case) | ~2–5× larger; gzip absorbs most of it; absolute bytes are small | **Bounded loss** |
| Server CPU per render | FNV-1a hashing ≈ today's reflection-based field compare | Wash |
| Client CPU | Merge a slightly larger object; negligible | Wash |
| Hash collisions (FNV-1a 64) | ~10⁻⁶ at 10M items; unsafe only beyond that | Negligible |
| Focus / cursor preservation on inputs | morphdom patches at the DOM level, not at wire granularity | No change |

The threshold/tag mechanism was hedging against "small lists with frequent single-field updates" — a real workload, but the regression is bounded in absolute bytes, fully recovered by compression on most transports. Carrying two code paths for that case costs more than it saves: a tuning knob users have to learn, a struct tag that sometimes appears and sometimes doesn't, and a behavioural cliff when a list crosses the threshold mid-session. **One path is simpler and cheaper to reason about.** The proposal commits to the single path and owns the bounded wire-cost regression rather than burying it.

### 5d. Heterogeneous-range internal fallback

Ranges where item structure varies — `{{range .Items}}{{if .HasError}}<del>...</del>{{else}}<span>...</span>{{end}}{{end}}` — produce items with divergent statics. The existing diff engine handles this by emitting a full tree, gated by `ClientNeedsStatics` (`internal/diff/tree_compare.go:458`).

Stream mode preserves this fallback. When the per-render structural check detects heterogeneous statics (the existing `ClientNeedsStatics` / range-match logic in `internal/diff/tree_compare.go`), the diff path emits a full Range tree as today rather than stream ops. This is an internal branch, not a user-facing toggle.

## Wire Protocol

**No changes.** The six range ops (`a`, `p`, `i`, `r`, `u`, `o`) exist today with the exact semantics the proposal needs. Both switch statements in `client/state/tree-renderer.ts` (around the top-level patcher and the nested-range patcher) handle every op the new strategy emits.

The only client-observable difference is the *shape of `["u"]` payloads*: instead of a partial dynamics map, they carry the full item's dynamics map. The client's existing merge logic (`mergeRangeItem`) treats the payload as a dynamics map and merges keys; sending all keys is a strict superset of sending some keys. No client change is required.

## Memory & Performance

Estimates, to be replaced with measured numbers from the implementation's benchmarks. Order-of-magnitude only.

| Cardinality | Legacy retained memory | Stream retained memory | Wire size on append-1 | Wire size on update-1-field |
|---|---|---|---|---|
| 10 items × 3 fields | ~1.5 KB | ~0.3 KB | identical | legacy ~50 B / stream ~150 B |
| 100 items × 3 fields | ~15 KB | ~3 KB | identical | legacy ~50 B / stream ~150 B |
| 10k items × 3 fields | ~1.5 MB | ~300 KB | identical | legacy ~50 B / stream ~150 B |

Per-connection retained memory drops by roughly 5×. Wire size on the worst-case workload (single-field updates on multi-field items) grows by roughly 3× in absolute bytes; gzip on the WebSocket recovers most of it. Wire size on inserts, deletes, reorders, and unchanged renders is identical.

## Reconnect & Resync

Client reconnect re-runs `Mount` via the existing `mount.go` callMount path. The server discards `lastTree` on a fresh handler and rebuilds `RangeStreamState` from the controller's first post-reconnect render. This matches today's behaviour; no replay log, no snapshot, no versioning. Mention is included only because the proposal explicitly does *not* introduce reconnect semantics.

## Migration

**Zero migration.** The change is internal. `examples/patterns/handlers_lists.go` (`DeleteRowController`, `ClickToLoadController`, `InfiniteScrollController`, `SortableController`) requires no edits. Templates require no edits. Wire format is unchanged. The legacy per-field-diff path is being *removed*, not deprecated, because there is no user-visible API to deprecate.

The one user-visible behaviour change is the `["u"]` payload shape: any external test or tooling that asserts a partial dynamics map will need to assert a full dynamics map after the implementation lands. See "Test Plan" for the required updates inside the project.

## Implementation Notes

| File | Change |
|---|---|
| `internal/build/types.go` | Add `RangeStreamState`; either add it as a sibling of `RangeData.Items` or repurpose the slot. Either way, `RangeData.Items` should no longer hold rendered item TreeNodes between renders for homogeneous ranges. |
| `internal/build/fingerprint.go` | Add `HashItem(item interface{}) uint64` using the existing FNV-1a primitive. Document the collision budget in code. |
| `internal/diff/range_ops.go` | Replace `newRangeContext`'s reliance on `oldItems` with `RangeStreamState`-driven lookup. Adjust `generateRemovalOps`/`generateUpdateOps`/`generateInsertionOps` to source comparisons from `Hashes`. The heterogeneous-range branch (returning `nil` for full-tree fallback) is preserved. |
| `internal/diff/tree_compare.go` | `ClientNeedsStatics` flow is unchanged. The top-level callers (`compareTreesAndGetChangesWithPath`) need to know that stream-mode `["u"]` payloads are whole-item, not delta. |
| `template.go` | `t.lastTree` continues to retain the post-render tree, but for ranges the retained `Range` field now holds a `*RangeStreamState` rather than item TreeNodes. The retention site at line 1358/1388 is the integration point. |

No public-API surface changes. No new options on `livetemplate.New(...)`. No new tags on state structs. No new methods on `*Context`.

## Test Plan

The implementation must add the following coverage. This list is the acceptance bar — a reviewer can use it as a checklist.

1. **Unit tests, `internal/diff/range_ops_test.go` (extend).** Diff-output goldens for each op (`a`, `p`, `i`, `r`, `u`, `o`) under the new key+hash strategy. Cover insert-at-head, insert-at-tail, mid-list insert, scattered inserts (heterogeneous fallback), pure reorder, mixed reorder + update, mass delete, mass replace.
2. **Memory regression, `internal/diff/range_memory_test.go` (new).** Construct ranges of N ∈ {10, 100, 1k, 10k} items, render twice, measure heap delta retained in `lastTree` after the second render via `runtime.ReadMemStats`. Assert per-item retained bytes drop by ≥4× vs the legacy baseline (captured as a golden number). Skip in `-short`.
3. **Hash collision soak, `internal/build/fingerprint_test.go` (extend).** Generate N = 1M synthetic items with deliberately similar dynamic payloads; assert collision count ≤ a documented ceiling. Document the FNV-1a collision budget as a code comment.
4. **Reconnect resync, `template_test.go` (extend).** Simulate "render → drop `lastTree` → render again with same state"; assert the second render emits a full initial tree, not stream ops. Locks in the "client re-runs Mount on reconnect" contract.
5. **Heterogeneous-range fallback, `internal/diff/range_ops_test.go`.** Range over items where some include an extra `{{if}}` branch; assert the diff falls back to full-tree replacement and the resulting tree is structurally correct.
6. **Wire-format compat, `e2e_update_spec_test.go` (extend).** Existing spec tests must pass. Any test asserting a partial `["u", key, {0: "..."}]` payload is updated to assert a whole-item payload, *or* repurposed as a regression test for the heterogeneous-range fallback.
7. **Browser E2E, `lvt` repo (`livetemplate_core_test.go`, extend).** Existing range-op tests must pass without modification (proves wire compat). Add one new browser test that scripts append/delete/reorder against a 5k-row table and asserts patch latency under a documented ceiling.
8. **Benchmarks, `internal/diff/range_ops_bench_test.go` (new).** `BenchmarkRangeDiff_Stream_{Append,Update,Reorder}_{Small,Medium,Large}` at N ∈ {10, 100, 10k}. The numbers in the "Memory & Performance" table above must be replaced with real benchmark output before this proposal is marked Implemented.

## Patterns Example

The implementation must ship a new pattern under `examples/patterns/` that exercises the large-list path end-to-end. Without it, the memory and scaling wins are invisible.

- **Files.** `examples/patterns/handlers_lists.go` (extend with `LargeTableController`); `examples/patterns/templates/lists/large-table.tmpl` (new); `examples/patterns/data.go` (extend with a deterministic 10k-row seed dataset); `examples/patterns/main.go` (register route); patterns index updated under "Lists & Data".
- **Scope.** A 10,000-row table with stable keys. The controller supports: filter (text input across rows, Tier 1 `Change()` method), sort (header click, button-name routing), append-N (button), update-random-row (button), delete-by-id (button), reset (button). Between filter, sort, append, update, delete, and reset, every range op the new path emits is reachable via UI.
- **Why this scope.** `infinite-scroll` already covers append-only growth; `sortable` already covers small-list reorder. Neither exercises a list large enough to demonstrate the memory win, and neither exercises mass updates. The new example is the visible payoff for the proposal and the integration-test workhorse.
- **E2E test.** `examples/patterns/patterns_test.go` extended with `TestLargeTable_*` subtests covering filter, sort, append, update, delete, reset. All waits use `e2etest.WaitFor` (no `chromedp.Sleep`) per `examples/CLAUDE.md` E2E rules. Each subtest asserts the resulting DOM state. Asserting bounded WebSocket message size requires a small in-test logger that wraps `WebSocket.prototype.send`/`onmessage` via `chromedp.Evaluate` and exposes the captured frame sizes back to Go — this proposal commits to adding that helper to `e2etest` (e.g. `e2etest.RecordWSFrames(ctx)`) so the bounded-WS-size assertion has a single canonical implementation rather than ad-hoc copies in each test file.
- **Tier discipline.** Standard HTML wherever possible (`<form method="POST">`, button `name` attributes, `Change()` for filter input). Tier 2 `lvt-*` only where standard HTML genuinely cannot express the interaction.
- **Manual UX testing.** Pico CSS only, CSP-clean, mobile-friendly. The author tests on iPhone over Tailscale per the project's manual-testing convention; the example's acceptance criteria require it.

## Open Questions

1. **Hash function choice.** FNV-1a 64-bit (matches the existing helper in `internal/build/fingerprint.go`) is the proposal default. xxh3 has lower collision rates on adversarial inputs, but adds a dependency. Is the FNV-1a collision budget acceptable for the largest expected list, or should the hash be made configurable?
2. **Heterogeneous-range fallback long-term.** The proposal keeps the legacy full-tree path as the het-range branch indefinitely. Is that acceptable, or should stream mode be extended to handle structure changes via per-item statics included in the op (`["u", key, dynamics, statics?]`)?
3. **Migration cadence.** Ship behind a config flag for one release as a safety hatch (`WithLegacyRangeDiff(true)`) and remove in the next, or commit to a hard cutover at v1? The proposal leans toward a hard cutover because there is no user-facing API to be cautious about, but a one-release safety hatch is cheap insurance.
4. **Volatile-field workloads.** A live counter on every row of a 10k-row dashboard is the worst case for whole-item updates: many updates per second, single field changing per update, large number of fields per item. Do we ship a future targeted-field op (e.g., `["uf", key, fieldIdx, value]`) as an explicit opt-in for that workload, or is `permessage-deflate` sufficient in practice? Punt to a follow-up; flag now so it isn't forgotten.

## References

- [Phoenix Dev Blog: Streams](https://fly.io/phoenix-files/phoenix-dev-blog-streams/) — the "don't keep the list on the server" insight.
- `internal/diff/range_ops.go` — `GenerateRangeDifferentialOperations`, `rangeContext`, `generateRemovalOps`, `generateUpdateOps`, `generateInsertionOps`.
- `internal/diff/tree_compare.go` — `ClientNeedsStatics`, the structure-fingerprint flow.
- `client/state/tree-renderer.ts` — case `"a"`/`"p"`/`"i"`/`"r"`/`"u"`/`"o"` handlers.
- `examples/patterns/handlers_lists.go` — `DeleteRowController`, `ClickToLoadController`, `InfiniteScrollController`, `SortableController`.
- `docs/proposals/tier1-file-uploads-proposal.md`, `docs/proposals/lifecycle-hooks-proposal.md` — house style.
