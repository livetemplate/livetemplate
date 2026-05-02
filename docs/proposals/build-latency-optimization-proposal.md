# Build-Latency Optimization Proposal

**Status:** Phase 7 (A)+(B) implemented in this PR. Phase 8 (HTML-render skip) attempted and reverted — broke the modal-cancel flow in the todos example by skipping a side effect of `html/template.Execute` that LVT's action-dispatching pipeline depends on; needs deeper investigation in its own PR. Phase 9 (pre-build hash short-circuit) outlined for follow-up.
**Owner:** badr.adnaan@gmail.com
**Date:** 2026-05-02

## TL;DR

The streaming-range Phases 1-5 reduced retained memory and wire size for large homogeneous ranges. They did **not** reduce per-render CPU time on the server: `Template.ExecuteUpdates` still walked every range item to compute fresh dynamics + hashes, costing ~30 μs/item. At N=10,000 a single update took 306 ms server-side — the dominant source of "extremely slow" UX in the Phase 6 LargeTable demo.

This proposal lands two CPU optimizations in Phase 7 and outlines two larger follow-ups.

## The Problem

`Template.ExecuteUpdates(w, state)` for a homogeneous range over N items spends time as follows (verified on linux/arm64, Go 1.26.1, N=10,000):

| Step | Cost/item | Where |
|---|---|---|
| html/template.Execute (renders the full HTML string for diff fallback) | ~10-12 μs | `template.go:renderHTMLWithData` |
| Reflection + var binding | ~5 μs | `internal/parse/range.go:iterateSlice`, `internal/parse/walker.go:buildRangeItemVarCtx` |
| Item template walk | ~8 μs | `internal/parse/walker.go:walkAST` recursing into `node.List` |
| **Hash computation** | **~12 μs (40% of `iterateSlice` cost)** | `internal/keys/hash.go:formatHashPart` — `json.Marshal` + reflection per dynamic value |
| Dynamics extraction + alloc | ~3 μs | `internal/parse/range.go:extractItemDynamics` |
| Diff (subsequent renders) | ~2 μs | stream-mode hash lookup |

Two observations follow:

1. **The hash path uses reflection-heavy `json.Marshal` to format every dynamic value before FNV-1a hashing.** For the LargeTable demo (5 fields per row, all string/int), reflection dominates.
2. **The per-item walk is embarrassingly parallel.** Each item gets its own `varContext`, the parsed AST is read-only after parse, the evaluator is read-only after construction (verified — `evaluator.builtins` is the only field, written once at `newEvaluator`), and `keyGen` is plumbed through `walkAST` but never invoked from the parse layer (verified — `keyGen.Next()` is called only by `internal/compat/tree.go` for the wrapper key, before `parse.BuildTree` runs).

## Phase 7 — what ships

### (A) Type-direct hash

Replace `formatHashPart`'s reflective `json.Marshal` path with a type switch covering the hot types (`string`, `int`/`int64`/`int32`/`int16`/`int8`, `uint` family, `bool`); preserve `json.Marshal` as the fallback for everything else (slices, maps, structs, floats, custom types).

Strings take a fast path that emits `"…"` directly when the contents contain no chars `json.Marshal` would escape (`<`, `>`, `&`, `"`, `\`, control bytes, non-ASCII). Strings with escape-required chars defer to `json.Marshal`. The fast-path check is a single byte loop.

**Wire-stability invariant.** `RangeStreamState.Hashes` are persisted across renders and stream-mode transition fingerprints depend on them being byte-stable across versions. The change is gated by `TestFormatHashPart_ByteEquivalentToJSON` in `internal/keys/hash_test.go`, which asserts `formatHashPart(key, val)` continues to produce `key:json.Marshal(val)` for every supported type.

**Files:** `internal/keys/hash.go`, `internal/keys/hash_test.go`.

**Measured impact** (`BenchmarkLargeTable_UpdateRandomRow_WireBytes` from `examples/patterns/large_table_bench_test.go`):

| N | Pre-(A) | Post-(A) only | Change |
|---|---|---|---|
| 200 | 6.6 ms | 5.6 ms | -15% |
| 1,000 | 36 ms | 30 ms | -17% |
| 10,000 | 306 ms | 247 ms | -19% |

Wire bytes unchanged at 127.8 B/op across all N (byte-stability holds).

### (B) Parallel range item build

Above `parallelIterateThreshold = 256` items, `iterateSlice` dispatches to a new `iterateSliceParallel` that splits per-item walks across `runtime.NumCPU()` workers. Each worker handles a contiguous index range and writes directly to its assigned slice indices, so result order matches input order without merge work. `reflect.Value.Index(i).Interface()` calls are pre-extracted on the main goroutine into a flat `[]interface{}` to avoid concurrent reflect access on the same `reflect.Value`.

**Threshold rationale.** Goroutine setup (~5-10 μs per worker) and `sync.WaitGroup` overhead (~1 μs) dominate at small N. At N=256 with 8 workers, parallel matches sequential; above, parallel pulls ahead. The threshold is a `const`; tunable in a follow-up if profiling reveals a better break-even.

**Thread-safety audit:**
- `evaluator`: read-only after `newEvaluator` (only field is `builtins`, written once). Concurrent reads safe.
- Parsed AST (`*parse.Template`): read-only after `parse.Parse`.
- `Context`: read-only after construction (fields set, then read).
- `keyGen`: never invoked from `iterateSlice` or its descendants (verified by grep — only `compat/tree.go` calls it, before `parse.BuildTree`). Plumbed through but inert here.
- Per-item state (`varContext`, returned `*TreeNode`, `Dynamics`): each item allocates its own; no sharing across workers.

**Files:** `internal/parse/range.go`.

**Measured impact** (combined with A):

| N | Pre-Phase-7 | Post (A)+(B) | Change |
|---|---|---|---|
| 200 | 6.6 ms | 6.7 ms | (no change — below threshold) |
| 1,000 | 36 ms | 24.8 ms | **-31%** |
| 10,000 | 306 ms | **201 ms** | **-34%** |

Parallelism gives less than ideal 8× because GC pauses serialize across workers (per-iteration allocation of ~1.86M objects at N=10k drives frequent GC) and the per-item walk includes I/O-style reflection that doesn't scale with cores. Phase 8 was attempted to close more of the gap by skipping the dead `html/template.Execute` call that follows; it broke an integration test (see Phase 8 section below) and is not in this PR.

### Regression test

`range_build_latency_test.go` (new, top-level package). `TestRangeBuildLatency_PostPhase7` builds a 5-field row template (mirrors LargeTable shape), warms up Execute → ExecuteUpdates × 4, then measures the median of 5 single-row-mutation `ExecuteUpdates` calls. Ceilings:

- N=1,000: 40 ms (median observed: 16 ms — 2.5× margin)
- N=10,000: 220 ms (median observed: 137 ms — 1.6× margin)

Skipped under `-short`. The N=10,000 case allocates ~90 MB before GC.

## Phase 8 — HTML-render skip on the steady-state path (attempted, reverted)

`template.go:buildTree` calls `renderHTMLWithData(dataWithLvt)` on every render to produce a full HTML string. Profiling at N=10k showed this takes **~110 ms per call** — over half of the post-Phase-7 latency at that scale. On paper it's dead work: `generateDiffBasedTree`'s steady-state main path doesn't consume the resulting HTML, and HTML escaping for the diff is handled by `valueToString` in `internal/parse/eval.go`.

**Reverted because:** with the skip in place, `examples/todos` `TestTodosE2E/Modal_Positioning_And_Cancel` failed reproducibly — the modal Cancel handler dispatched, but the modal didn't disappear from the DOM. Some side effect of `html/template.Execute` (independent of the rendered HTML output) is consumed by LVT's action-dispatching or state-update pipeline. The bot review correctly flagged this risk; the empirical failure confirmed it.

Needs a separate PR with deeper investigation:
- Find the specific path through LVT's action handler that depends on the html/template execution
- Either move that side effect onto a path that runs unconditionally, or document the constraint that the render is part of the per-action contract
- Then either skip safely with the constraint enforced, or close out Phase 8 as a "false economy" entry in the proposal.

**Initial safety analysis (held up under code review, failed under integration test):**
- `generateDiffBasedTree`'s main branch (`hasInitialTree == true`) builds the new tree directly from the parsed AST + data via `parse.BuildTree` (`buildTreeWithCache(newData, ctx)` at `template.go:~1385`). The `currentHTML` argument is unused on this branch; the comment at the function head is explicit: "tree generation uses t.templateStr (template source), not extracted rendered HTML."
- HTML escaping for the diff is handled independently by `valueToString` in `internal/parse/eval.go:540`, which applies `html.EscapeString` to dynamic values before they enter the tree.
- Side-effecting funcs would today fire twice per render (once via `html/template.Execute`, once via `parse.BuildTree`'s evaluator). After skip, they fire once.

**What went wrong empirically:** with the skip in place and `examples/todos` `TestTodosE2E/Modal_Positioning_And_Cancel` exercised, the test reliably failed. The Cancel button's action dispatched on the server (verified by no error from the click) but the modal stayed in the DOM client-side past the 5 s wait. Reverting only the skip (keeping Phase 7 A+B intact) made the test pass again. **There is a side effect of `html/template.Execute` that the LVT action-dispatch / state-update pipeline depends on, independent of the rendered HTML output.**

Candidates for the unidentified side effect (not yet narrowed down):
- Template execution may register or refresh some state in the html/template runtime that the LVT WebSocket dispatcher consults
- One of the LVT-specific funcs registered via `FuncMap` may have a hidden side effect that html/template invokes but `parse.BuildTree`'s evaluator doesn't (or invokes differently)
- The render may be the trigger for some lazy initialization (template cache, action handler binding, etc.)

Phase 8 is parked until the dependency is identified. Possible resolutions:
- Find and explicitly relocate the side effect onto a path that runs unconditionally (then the skip becomes safe)
- Document that `html/template.Execute` is part of the per-action contract and close out Phase 8 as a "false economy"
- Add an action-aware skip (e.g., skip only on server-push-style updates that don't dispatch an action)

## Phase 9 — Pre-build hash short-circuit (sparse-update CPU floor)

The streaming-range Phases 1-3 dropped `lastTree.Range.Items` after `TransitionToStreamMode` to reclaim memory (~215 B/item). For sparse-update workloads (1 of 10k rows changes per render — the LargeTable `UpdateRandomRow` shape), CPU could drop further if we kept the cached `*TreeNode` per item, hashed each new item's source struct fields cheaply (without going through `walkAST`), and reused the cached `*TreeNode` whenever the new hash matches the old.

Plan agent's analysis (see prior Phase 7 design doc) projects this as ~5 ms total at N=10k for sparse updates — another order of magnitude below the (A)+(B)+Phase-8 floor.

**Memory trade-off:** keeping `Items` alive regresses the Phase 1-3 retained-memory win (~2.15 MB at N=10k per template per session). For long-lived sessions this is significant. Recommended shape: opt-in via `WithRangeBuildHints(true)` config flag, with the existing memory profile as the default.

**Architectural changes:**
- Snapshot `lastTree.Range.Items` into a `cachedNodeMap[structHash]*TreeNode` BEFORE `TransitionToStreamMode` at `template.go:1663` (or teach `TransitionToStreamMode` to retain a hint cache).
- New `BuildHints` interface in `internal/parse/`, satisfied by `template.go` from the cache.
- Parse-time AST walk (`collectFieldRefs` in `internal/parse/eval.go`) to enumerate `FieldNode` references in the range body and decide whether the template supports hints. Disable on `MethodNode`, custom `IdentifierNode` funcs (whitelist `cachedBuiltins` from `eval.go:23`), and `{{template "x" .}}` invocations.

**Memory regression test:** assert that with the flag OFF, `TestRangeRetainedMemory_LegacyVsStream` numbers are unchanged.

## What this proposal does NOT do

- Reduce retained memory (Phases 1-3 already did).
- Change wire format (hash byte-equivalence is enforced).
- Touch `lvt` or `examples` repos (livetemplate-only PR).
- Address mobile-Safari DOM-mutation cost on 10k `<tr>` elements (client-side; would need virtualization in the client repo).
- Address heterogeneous-range path (legacy diff, not on the streaming hot path).
