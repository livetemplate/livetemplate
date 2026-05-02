# Build-Latency Optimization Proposal

**Status:** Phase 7 (A)+(B) and Phase 8 (HTML-render skip) implemented in this PR. Phase 9 (pre-build hash short-circuit) outlined for follow-up.
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

Parallelism gives less than ideal 8× because GC pauses serialize across workers (per-iteration allocation of ~1.86M objects at N=10k drives frequent GC) and the per-item walk includes I/O-style reflection that doesn't scale with cores. Phase 8 below recovers the rest.

### Regression test

`range_build_latency_test.go` (new, top-level package). `TestRangeBuildLatency_PostPhase7` builds a 5-field row template (mirrors LargeTable shape), warms up Execute → ExecuteUpdates × 4, then measures the median of 5 single-row-mutation `ExecuteUpdates` calls. Ceilings:

- N=1,000: 40 ms (median observed: 16 ms — 2.5× margin)
- N=10,000: 220 ms (median observed: 137 ms — 1.6× margin)

Skipped under `-short`. The N=10,000 case allocates ~90 MB before GC.

## Phase 8 — HTML-render skip on the steady-state path (shipped)

`template.go:buildTree` previously called `renderHTMLWithData(dataWithLvt)` on every render to produce a full HTML string. Profiling at N=10k showed this took **~110 ms per call** — over half of the post-Phase-7 latency at that scale.

**Why it was safe to skip on the main path:**
- `generateDiffBasedTree`'s main branch (`hasInitialTree == true`) builds the new tree directly from the parsed AST + data via `parse.BuildTree` (`buildTreeWithCache(newData, ctx)` at `template.go:~1385`). The `currentHTML` argument is unused on this branch; the comment at the function head is explicit: "tree generation uses t.templateStr (template source), not extracted rendered HTML."
- HTML escaping for the diff is handled independently by `valueToString` in `internal/parse/eval.go:540`, which applies `html.EscapeString` to dynamic values before they enter the tree. The `html/template.Execute` render was NOT supplying escaping for the diff path — only for the first-render `lastHTML` cache and the AST-failure fallback path.
- Side-effecting funcs would today fire twice per render (once via `html/template.Execute`, once via `parse.BuildTree`'s evaluator). After skip, they fire once. This is more correct (a render shouldn't have observable side effects), and Go template authors don't rely on the double-call as a contract.

**Implementation:**
- `hasInitialTree` converted from `bool` to `atomic.Bool` so `buildTree` can read it without acquiring `t.mu`. The latch is monotonic (false→true, exactly once when the first AST tree lands), so a stale "false" read only causes one extra dead render at most — never wrong output.
- `buildTree` skips `renderHTMLWithData` when `hasInitialTree.Load()` returns true. First render and the AST-failure fallback path still call it.

**Files:** `template.go` (declaration, 4 read sites, 1 write site, the buildTree skip).

**Measured impact** (combined Phase 7 + Phase 8):

| N | Pre-Phase-7 | Post Phase 7 only | Post Phase 7+8 | Total change |
|---|---|---|---|---|
| 200 | 6.6 ms | 6.7 ms | **3.5 ms** | **−47%** |
| 1,000 | 36 ms | 24.8 ms | **12.2 ms** | **−66%** |
| 10,000 | **306 ms** | 201 ms | **75 ms** | **−75%** |

`TestStreamMode_ReconnectResyncEmitsFullTree` exercises the fallback path (`hasInitialTree.Store(false)`) and confirms it still emits a full tree on the next render. The latency regression test ceilings were tightened to 25 ms (N=1000) / 150 ms (N=10000) — gates that catch any future regression that re-introduces the dead render.

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
