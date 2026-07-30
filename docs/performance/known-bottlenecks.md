# Known Performance Bottlenecks

**Last CPU Profiled:** 2026-07-30 (composite-pipeline suite)
**Last Memory Profiled:** 2026-07-30 (composite-pipeline suite) · 2026-07-29 (full root suite)
**Last Range-Diff Benchmarked:** 2026-05-01 (Phase 5 streaming-range)
**Go Version:** 1.26.1
**Architecture:** linux/arm64 (Neoverse-N1, 8 cores, shared machine at load ~10)

> **Reading the numbers.** The 2026-07 profiles were captured on a loaded shared
> machine. pprof percentages are shares of this process's own samples and remain
> valid for *ranking*; allocs/op and wireB/op are exact and machine-independent;
> **ns/op values are indicative only** (±20-30% between runs). Clean absolute
> timings belong to the CI benchmark gate (amd64 ubuntu runner) whose baseline is
> maintained in `testdata/benchmarks/baseline.txt`. Numbers dated 2026-03 were
> captured on Apple M2 (darwin/arm64) and are not comparable across machines.

## Bottleneck Report — 2026-07-30, composite workloads, ranked

Profiled **through the composite-pipeline benchmarks** (the real cycle: action
dispatch → controller → render → diff → serialize → real `writePump`), which the
pre-2026-07 suite never exercised. Sources: `profiles/composite-{cpu,mem}.prof`
captured over `BenchmarkCompositeUpdate`, `BenchmarkTopicFanout_FullPipeline/N=100`,
`BenchmarkWideTableAction`, `BenchmarkLargeDocDiff/files=10,lines=100/autokey`,
`BenchmarkChatAppendFanout/hist=100,peers=10`, `BenchmarkUpload_Proxied/1MB`.

### 1. The per-item pipeline constant — server cost scales with rendered size, not change size

**The headline the isolated micro-benches hid.** Across three unrelated workload
shapes — wide grid (seat-picker), chat history, large document (prereview) — one
action costs **~90-100 allocs and ~25-30µs (loaded) per rendered range item**,
regardless of how small the actual state change is:

| Workload | One action costs | Wire ships |
|---|---|---|
| 50×20 grid, 1-cell toggle | ~25ms · 108k allocs | 1.5 KB |
| 1k-line doc, 1-line comment | ~28ms · 93k allocs | 6.5 KB (auto-key) / 305 B (data-key) |
| 10k-line doc, 1-line comment | ~242ms · 919k allocs | 13 KB (auto-key) |
| 50k-line doc, 1-line comment | ~1.2s · 4.6M allocs | 33 KB (auto-key) |
| 10k-msg chat, 1 append × 10 peers | ~357ms · 4.3M allocs | 1.3 KB |

Every action re-renders and re-diffs the entire template against fresh state.
The diff engine keeps the *wire* minimal, but the server pays O(total items) in
render+diff work for an O(1) mutation. This is the dominant scalability ceiling:
fan-out multiplies it per subscriber (per-subscriber marginal cost ≈ one full
single-connection interaction, ~92 allocs — measured linear through N=10000).

**Candidate optimization:** memoized/partial re-render — skip rebuilding range
item subtrees whose input data is unchanged (e.g. reuse the previous render's
`TreeNode` subtree keyed by the existing stream-mode content hash before
re-rendering the item). Expected impact: turns O(total items) per action into
O(changed items + hash checks) for the build phase — for the 50k-line document
that is the difference between ~1.2s and low tens of ms. High effort: touches
the build path's correctness-critical statics/dynamics contract.
**Identification only — not undertaken in this plan.**

### 2. TreeNode construction — 24.8% of composite allocations

`build.NewTreeNode` 14.8% flat + `build.NewTreeNodeWithStatics` 10.0% flat
(composite mem profile). Unchanged ranking vs 2026-03 (22.7% then, full-suite),
now confirmed under real workloads. The 2026-03 `sync.Pool` investigation stands:
pooling gains <3% because GC clears pools; an arena or a flattened tree layout
remains the only structural fix. Largely subsumed by #1 — memoizing unchanged
subtrees would eliminate most TreeNode churn as a side effect.

### 3. Serialization — ~16.5% cumulative of composite allocations

`json-iterator frozenConfig.Marshal` 16.45% cum; inside it
`TreeNode.MarshalJSON` 13.1% cum, `sortKeysMapEncoder.Encode` 13.8% cum,
`hex.EncodeToString` 2.6% flat, plus reflect2 map iteration 4.6% cum. Grew from
2.11% (2026-03, pre-json-iterator, full-suite) — the wire-format marshal now
runs once per receiver per action, so fan-out multiplies it.

**Candidate optimizations:** (a) audit whether the sorted-map encoder config is
needed on the hot path — the tree wire format already emits ordered keys via the
custom `MarshalJSON` (expected: several % of total allocs); (b) marshal the
response envelope once per publish and share the byte slice across subscribers
whose diff is identical (the chat/topic push case — today each of N identical
updates is re-marshaled). Expected impact at N=100 fan-out: removes up to N-1
redundant marshals per publish.

### 4. Streaming-range per-item hashing — ~12.3% cumulative

`keys.buildHashParts` 12.3% cum / `keys.formatHashPart` 10.9% cum /
`hex.EncodeToString` + `fnv.New128a` beneath — the stream-mode content hashing
introduced by the 2026-05 streaming-range rewrite, now a top-5 allocator under
composite load (invisible to the old suite; first flagged by the 2026-07-29
full-suite snapshot at 7.7% cum). Each range item's dynamics are stringified and
hex-encoded on every render.

**Candidate optimization:** hash without materializing intermediate strings
(feed the FNV hasher incrementally; keep the 64-bit hash as `uint64` instead of
a hex `string` key). Expected impact: most of the ~12% cum, plus downstream
savings in `newStreamRangeContext` (3.9% flat) and `sameKeySet` (1.3% flat).
Medium effort, self-contained in `internal/keys` + `internal/diff`.

### 5. Per-render template AST walking — ~40% cumulative

`parse.walkAST`/`walkList` 40.4% cum (composite mem), `newOrderedVars` 3.9%
flat, `buildRangeItemVarCtx` 6.2% cum; CPU-side `walkAST/walkList` is 19.3% cum
of composite CPU. The parsed AST is cached (2026-03, PR #219), but the *walk*
still happens per render per receiver, allocating per-item variable contexts.
This is the build-phase engine behind finding #1 — same fix (skip unchanged
subtrees), listed for attribution rather than as a separate work item.

### 6. Auto-key churn on in-place edits — wire, not CPU

Editing an item in place changes its content-hash auto-key, so the diff ships
the containing range's op list instead of a minimal update: wire grows with the
containing range's size (6.5→33 KB for a 1-line edit as the file grows —
`BenchmarkLargeDocDiff/*/autokey`). With explicit `data-key` the same edit ships
**~305 B steady-state, independent of document size** (+~9% allocs for key
extraction). The first render after a structure change still ships that
subtree's statics once (amortizes across the session).

**Candidate optimization:** none needed in the engine — this is the documented
key-stability design; the fix is usage-level: `data-key` is strongly recommended
for large edit-in-place ranges (see CLAUDE.md "data-key in Range Templates").
Optional follow-up: a debug-mode warning when a large range re-keys an existing
item on consecutive renders.

### 7. Cross-instance Redis — constant hop, amortized by N≈100 (healthy)

Relay round trip ~110-130µs / 91 allocs over in-process miniredis
(`pubsub/BenchmarkRedisTopicRelay`); full-pipeline cross-instance fan-out
converges with in-process fan-out by N=100 (`BenchmarkRedisCrossInstanceFanout`:
~238µs at N=1 vs ~57µs in-process; indistinguishable at N≥100). Redis traffic is
O(1) per publish. Relay allocations split between deserialize (`handleMessage`,
29% cum of the pubsub profile) and go-redis/miniredis internals. No action
needed — documented as scaling-claim protection; no surveyed consumer app uses
Redis fan-out in production.

### 8. Uploads — healthy, deprioritized

Proxied streams 700-1100 MB/s at a **constant 114 allocs/op across
64KB→16MB** (genuinely zero-copy; the CPU is `mime/multipart` boundary scanning,
6.1% cum of composite CPU, proportional to bytes — stdlib floor). Volume is
disk-write-bound (190-435 MB/s, 121 allocs/op). Direct/Preview are 12µs/7µs
protocol handling. No framework bottleneck.

## Profiling Methodology

```bash
make profile-cpu            # root package, all benches
make profile-mem
make profile-pkg PKG=./internal/session BENCH='AsyncSendThroughput$'
make profile-pkg PKG=./pubsub BENCH='RedisTopicRelay$'

# Composite-only profile (what this report used):
GOWORK=off go test -run '^$' \
  -bench 'CompositeUpdate$|TopicFanout_FullPipeline/N=100$|WideTableAction|LargeDocDiff/files=10,lines=100/autokey|ChatAppendFanout/hist=100,peers=10$|Upload_Proxied/1MB' \
  -benchtime=2s -cpuprofile=profiles/composite-cpu.prof -memprofile=profiles/composite-mem.prof .

go tool pprof -top -cum profiles/composite-cpu.prof
go tool pprof -top -alloc_space profiles/composite-mem.prof
```

## CPU Profile — 2026-07-30, composite suite

Loaded-machine caveat applies to absolute times; ranking is valid.

```
Duration: 24.65s, samples 30.56s
      cum%   (cumulative, selected)
     51.6%  Template.ExecuteUpdates            ← the per-action pipeline
     48.1%  Template.buildTree
     26.4%  runtime.mallocgc                   ← allocation machinery
     19.3%  parse.walkAST / walkList           ← per-render AST walk (finding #5)
     15.4%  Template.renderHTMLWithData
     15.1%  html/template.(*Template).Execute  ← stdlib render floor
     14.3%  Template.compareTreesAndGetChanges ← diff phase
      8.8%  reflect.Value.call                 ← controller dispatch + data map
      6.1%  mime/multipart boundary scan       ← upload byte path (healthy)

      flat% (selected)
      8.6%  runtime.asyncPreempt               ← loaded-box scheduling noise
      5.7%  bytealg.LastIndexByte              ← multipart boundary scan
     ~14%   runtime lock2/unlock2 + GC mark    ← allocation pressure (findings #2-#4)
```

## Memory Profile — 2026-07-30, composite suite

```
6.77 GB total allocated across the profile run
      flat  flat%        cum%
 1002.1MB  14.8%         —     build.NewTreeNode                    (#2)
  677.1MB  10.0%         —     build.NewTreeNodeWithStatics         (#2)
  343.1MB   5.1%       13.1%   build.(*TreeNode).MarshalJSON        (#3)
  264.0MB   3.9%         —     parse.newOrderedVars                 (#5)
  261.2MB   3.9%         —     diff.newStreamRangeContext           (#4)
  259.0MB   3.8%       13.8%   json-iterator sortKeysMapEncoder     (#3)
  175.5MB   2.6%         —     encoding/hex.EncodeToString          (#4)
  166.5MB   2.5%        4.6%   reflect2 UnsafeMapType.UnsafeIterate (#3)
  157.0MB   2.3%        6.2%   parse.buildRangeItemVarCtx           (#5)
  153.0MB   2.3%         —     reflect.unsafe_New
  143.2MB   2.1%       16.5%   json-iterator frozenConfig.Marshal   (#3)
  139.8MB   2.1%       10.9%   diff.GenerateRangeStreamOperations   (#4)
  136.0MB   2.0%       40.4%   parse.walkAST                        (#5)
  134.1MB   2.0%       10.9%   keys.formatHashPart                  (#4)
```

### Drift vs 2026-03 (full-suite snapshot, 2026-07-29, v0.22.0)

The 2026-03 ranking held in shape — `NewTreeNode` still #1 — but three
subsystems that did not exist in March are now top-10 allocators:

1. **Serialization** (~11% cum full-suite, 16.5% composite) — the
   encoding/json → json-iterator migration moved the cost, and composite
   workloads multiplied it per receiver. March: 2.1%.
2. **Streaming-range hashing** (`keys.formatHashPart`, 7.7% cum full-suite,
   10.9% composite) — from the 2026-05 rewrite.
3. **Topics dispatch** (`dispatchToTopic`, 4.5% cum full-suite) — the pub/sub
   build-out.

Reflection is materially DOWN (`buildDataMapWithContext` 8.7% → 2.5%) — the
2026-03 dedup work held. A full-suite `os.readdir*` cluster (~3.7%) traces to
template-file loading in error-path benches, not a hot path.

## Allocations per Operation (2026-07-30 unless noted)

**Composite pipeline (real cycle, per interaction):**
- Single interaction (`BenchmarkCompositeUpdate`): **97 allocs, 6.3 KB, 78.7 wireB**
- Per-subscriber marginal in fan-out: **~92 allocs** (linear N=1→10000)
- 100-action journey: `BenchmarkE2EUserJourney` (full pipeline, 9705 allocs)
  vs `BenchmarkUserJourney_ExecuteUpdatesOnly` (the render+diff primitive the
  pre-2026-07 bench of that E2E name actually measured, 5683 allocs) — the old
  bench missed ~40% of allocations and ~60% of wall time per interaction.

**Primitives (2026-03-22, Apple M2 — for continuity):**
- Subsequent render: 61 allocs / ~3 KB; small update: 46 allocs / 2.2 KB;
  large update: 123 allocs / 5.2 KB.
- Range-diff per-call numbers (2026-05-01) are unchanged; see git history of
  this file for the full tables.

### Streaming Range Retention (Phase 5, 2026-05-01)

Homogeneous ranges retain a per-range snapshot (`RangeStreamState{Keys, Hashes,
Fingerprint}`) instead of per-item `*TreeNode`s. Measured by
`internal/diff/range_memory_test.go` (HeapAlloc delta, 8 retained trees,
double-GC + warm-up):

| N | Legacy retained (B/item) | Stream retained (B/item) | Drop |
|---|---|---|---|
| 10 | ~270 | ~80 | ~3.4× |
| 100 | ~245 | ~47 | ~5.2× |
| 1000 | ~256 | ~41 | ~6.2× |
| 10000 | ~256 | ~41 | ~6.3× |

For a 10k-row table held by 100 connections, retained memory drops ~250 MB →
~40 MB. CI gate: `TestRangeRetainedMemory_LegacyVsStream` (ratio floors
1.8×/4×/5×, skipped under `-short`). Wire trade-off: worst-case single-field
update ~1.4× (well under the projected 3× ceiling). The per-CALL cost of the
stream path is finding #4 above — the retention win stands; the hashing
allocations are the follow-up.

## Optimization Priorities

1. **[High — new 2026-07-30] Partial re-render / unchanged-subtree memoization**
   (finding #1). The only fix that changes the O(total items)-per-action
   ceiling. High effort, build-path invasive. Prerequisite: priority 2 makes
   the memoization check nearly free.
2. **[Medium — new 2026-07-30] Streaming-range hash allocation removal**
   (finding #4). Self-contained in `internal/keys`/`internal/diff`; ~12% of
   composite allocations.
3. **[Medium — new 2026-07-30] Serialization: sorted-map encoder audit +
   shared fan-out marshal** (finding #3). Up to N-1 redundant marshals per
   publish removed.
4. **[Investigated 2026-03 — Not Viable] TreeNode `sync.Pool`** — pool is cold
   under GC; <3% gain. Superseded by priority 1.
5. **[Low — Diminishing Returns] Reflection** — already deduplicated (PR #224);
   code generation not worth the complexity (unchanged from 2026-03).
6. **[Low — stdlib Bound] `html/template` execution floor** (~15% CPU cum) —
   cannot improve without replacing the engine.

## Optimization Task List

### Completed
- [x] Eliminate redundant HTML parsing + cache template AST *(2026-03-21, PR #219)*
- [x] Dynamics map → slice *(2026-03-21, PR #220)*
- [x] Shared statics, buffer pool, reflection dedup *(2026-03-21, PR #224)*
- [x] FNV-1a fingerprinting *(2026-03-18, commit 1e351ca)*
- [x] rangeContext range-diff optimization *(2026-03-19, commit b9faf28)*
- [x] Streaming-range retention drop *(2026-05-01, PRs #361-365)*
- [x] Faster JSON library *(shipped: encoding/json → json-iterator, commit
      d1a8ba8d)* — the migration moved the floor; serialization is now 16.5%
      cum of composite allocations (finding #3) and the follow-ups are the
      sorted-map audit and shared fan-out marshal.
- [~] TreeNode struct pooling *(investigated 2026-03-23 — not viable, see
      Priorities)*

### Open
- [ ] **Partial re-render / unchanged-subtree memoization** (Priority 1 — finding #1)
- [ ] **Streaming-range hash without string materialization** (Priority 2 — finding #4)
- [ ] **Sorted-map encoder audit + shared fan-out marshal** (Priority 3 — finding #3)
- [ ] Debug-mode warning on per-render re-keying of large ranges (finding #6, optional)
- [ ] Allocation budget tests (`testing.AllocsPerRun` pinned to
      `BenchmarkCompositeUpdate`'s 97 allocs/interaction)
- [ ] Profile production workloads (pprof endpoints on a real consumer app)

**Last Updated:** 2026-07-30

## Per-Subsystem Notes

- **`internal/session` (write path):** `Connection.Send` is the package's only
  measurable allocator (one `wsMessage` per send; 74% of its own micro-profile);
  the real-pump `BenchmarkAsyncSendThroughput` shows the pump adds no
  allocations over enqueue. Not a bottleneck.
- **`pubsub` (Redis relay):** per-relay allocations split between message
  deserialize (`handleMessage`, 29% cum) and go-redis proto reads; ~91 allocs
  per relay. miniredis itself contributes ~12% of the profile (in-process fake —
  absent in production).
- **Uploads:** see finding #8 — healthy.

## Regenerating Profiles

```bash
make profile-all                          # root package, everything
make profile-pkg PKG=./pubsub BENCH=...   # any non-root package
go tool pprof -http=:8080 profiles/composite-cpu.prof
```

Composite-only capture: see Profiling Methodology above. When updating this
document, re-run the composite capture and refresh the two profile tables, the
report rankings, and the header dates — and record Go version, CPU model, and
machine load alongside any ns/op you quote.
