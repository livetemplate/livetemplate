# LiveTemplate Performance Characteristics

## Summary

| Operation | Latency | Bandwidth Savings |
|-----------|---------|-------------------|
| Initial Render | ~20-65µs | — |
| Small Update (1-2 fields) | ~18-20µs | 85% vs full render |
| Large Update (5+ fields) | ~65µs | 65% vs full render |
| Range Operations | ~30-65µs | 80% vs full render |

Numbers from baseline (Go 1.26, Apple M1/M2). Full results in [`testdata/benchmarks/baseline.txt`](../../testdata/benchmarks/baseline.txt).

**How updates stay small:** The first render ships full HTML and the client caches the static parts. Subsequent renders ship only the changed dynamic values — the statics never travel again until the template structure itself changes (detected by an FNV-1a fingerprint).

### Running Benchmarks

```bash
# Run all benchmarks
make bench

# Compare against baseline
make bench-compare

# Generate performance profiles
make profile-cpu
make profile-mem
```

For statistically confident timing comparisons, use `make bench-10x` with `benchstat` — single-run timings can swing 2-4x due to thermal and GC noise.

---

> **Benchmark Environment:** Go 1.26.0, arm64 (Apple M2). Numbers updated 2026-03-22.
> These are single-run results (`make bench-save`); ns/op timings can vary significantly
> between runs (2-4x swings observed) due to system load, thermal throttling, and GC timing.
> Allocation counts (B/op, allocs/op) are deterministic and stable across runs — use these
> to assess actual code changes. For statistically confident timing comparisons, use
> `make bench-10x` with benchstat. The baseline is single-run to keep CI fast.
>
> **Baseline change notes:** This baseline captures PRs #219 (parse alloc reduction),
> #220 (Dynamics map→slice), and #224 (shared statics, buffer pool, reflection dedup).
> Key allocation improvements vs the previous (2026-03-19) baseline: subsequent render 170→61
> allocs, small update 154→46 allocs, large update 357→123 allocs, E2E range add 406→222
> allocs, E2E user journey 16174→5083 allocs. These are real improvements from eliminating
> map allocations in Dynamics, sharing sentinel statics, pooling bytes.Buffer, and deduplicating
> reflection lookups.

## Architectural Overview

LiveTemplate implements a 5-phase architecture optimized for minimal updates:

```
Parse → Build → Diff → Render → Send
```

**First Render:** Parse → Build → Render → Send (includes statics in tree)
**Updates:** Build (includes Diff) → Send (excludes statics from tree)

This architecture enables:
- **85%+ bandwidth savings** on updates vs full renders
- **Sub-millisecond update latency** for typical changes
- **O(n) complexity** for most operations

## Phase 1: Parse

### Operations

- Template parsing (Go `html/template` AST)
- AST walking and construct identification
- Expression evaluation with caching
- Tree structure compilation

### Complexity

- **Template parsing:** O(template size)
- **AST walking:** O(nodes in AST)
- **Expression evaluation:** O(expression complexity), direct reflection-based

### Optimizations

1. **Custom AST Evaluator**
   - Walks `PipeNode`/`CommandNode` directly via reflection
   - No template re-parsing or string serialization
   - No global caches needed — evaluation is fast enough without caching

2. **Lazy Structure Fingerprinting**
   - `GetStructureFingerprint()` uses O(1) cache lookup for subsequent calls
   - FNV-1a 128-bit truncated to 64 bits for compact representation

### Benchmark Results

```
BenchmarkParse/simple            2995 ns/op    5248 B/op     47 allocs/op
BenchmarkParse/conditional       4332 ns/op    6008 B/op     68 allocs/op
BenchmarkParse/range             4032 ns/op    5840 B/op     63 allocs/op
BenchmarkBuildTree/simple         398 ns/op     672 B/op     14 allocs/op
BenchmarkBuildTree/cond-true      678 ns/op    1328 B/op     27 allocs/op
BenchmarkBuildTree/range-small   2445 ns/op    3955 B/op     90 allocs/op
BenchmarkBuildTreeScale/small-10       6916 ns/op   11576 B/op    266 allocs/op
BenchmarkBuildTreeScale/medium-100    63388 ns/op  109862 B/op   2516 allocs/op
BenchmarkBuildTreeScale/large-1000   662906 ns/op 1096212 B/op  25763 allocs/op
```

### Key Findings

- Simple template parsing: ~3.0µs with 47 allocations
- BuildTree/simple down to ~398ns / 14 allocs (from 1216ns / 27 allocs) — Dynamics slice + shared statics
- BuildTree allocations reduced 48-29% across all variants
- BuildTreeScale/large-1000: 25763 allocs (was 34775) — 26% fewer allocations
- Tree building dominates for large datasets (100+ items)
- Memory usage scales predictably: ~1.1 MB per 1000 items (was ~1.7 MB)

## Phase 2: Build

### Operations

- TreeNode creation and manipulation
- Static/dynamic separation
- Fingerprint calculation (tree hashing)
- Wrapper div injection
- Context management (with/without statics)

### Complexity

- **Tree construction:** O(n) where n = number of nodes
- **Fingerprint calculation:** O(n), hashes entire tree
- **Wrapper injection:** O(html length)

### Optimizations

1. **Single Lock Strategy**
   - Acquire lock once for all state operations
   - Minimizes lock contention
   - Read-heavy operations outside lock

2. **Custom JSON Marshaling**
   - TreeNode.MarshalJSON() maintains key order
   - Optimized for wire format

3. **Fingerprint Caching**
   - FNV-1a hash of tree structure (replaced MD5 in commit 1e351ca)
   - Fast equality check without deep comparison
   - Cached until tree changes

4. **Dynamics `[]interface{}` Slice** *(PR #220)*
   - Replaced `map[string]interface{}` with `[]interface{}` for Dynamics
   - Eliminates map allocation, hash overhead, and string key storage
   - Index-based access via `PositionKey` cached string table

5. **Shared Sentinel Statics** *(PR #224)*
   - Empty `[]string{}` statics shared across TreeNodes instead of allocating per-node
   - `sync.Pool` for `bytes.Buffer` in JSON marshaling and HTML rendering
   - `PositionKey` cached string table avoids repeated `strconv.Itoa` calls

6. **Reflection Deduplication** *(PR #224)*
   - Deduplicated reflection lookups for controller/state method dispatch
   - Cached method resolution per type

### Benchmark Results

```
BenchmarkTreeNodeCreation/flat           411.5 ns/op    1264 B/op    17 allocs/op
BenchmarkTreeNodeMarshalJSON/nested-small  47939 ns/op   27942 B/op   358 allocs/op
BenchmarkWrapperInjection/full-html      3267 ns/op    7096 B/op    37 allocs/op
```

### Key Findings

- Flat tree creation: ~412ns / 17 allocs (was ~624ns / 19 allocs) — Dynamics slice eliminates map overhead
- JSON marshaling is the primary cost for nested structures
- Wrapper injection adds minimal overhead (~3.3µs)

## Phase 3: Diff

### Operations

- Tree comparison (deep equality)
- Range differential generation
- Update/Insert/Remove/Reorder operations
- Client preparation (static stripping)

### Complexity

- **Tree comparison:** O(n) where n = nodes in tree
- **Range diff:** O(m) where m = items in range
- **Static stripping:** O(n), single pass

### Optimizations

1. **Fingerprint-Based Static Stripping**
   - Structure fingerprints (FNV-1a, 64-bit) compared between old/new trees
   - When fingerprints match, client already has statics cached — updates omit static arrays
   - When fingerprints differ (structural change), full tree with statics is resent
   - Replaced the earlier `ClientStructureRegistry` (LRU, max 1000 entries), removed in PR #86
   - **Result: 65%+ size reduction** between updates

2. **Deep Equality Checks**
   - Only compare changed values
   - Skip comparison on identical fingerprints

3. **Differential Range Operations**
   - Minimal operations for list changes
   - Reuses existing DOM nodes on client
   - Operations: `["u", key, updates]`, `["i", after, pos, data]`, `["r", key]`, `["o", [keys]]`

4. **Pre-computed Range Context** *(Added in commit b9faf28)*
   - `rangeContext` pre-computes key maps and positions once per range diff
   - DeepEqual fast paths for string, int, float64, bool (avoids reflect.DeepEqual)
   - Package-level compiled regex for position field detection

5. **Pass-through Result Map** *(PR #224)*
   - `findRangeConstructsRecursive` passes result map through recursion instead of allocating intermediate maps
   - `PositionGetter` interface for zero-allocation dynamic position access

### Benchmark Results

```
BenchmarkCompareTreesNoChanges          134.6 ns/op   288 B/op     2 allocs/op
BenchmarkCompareTreesSmallChange         60.66 ns/op   144 B/op     2 allocs/op
BenchmarkCompareTreesLargeChange/10      200.4 ns/op   288 B/op     2 allocs/op
BenchmarkCompareTreesLargeChange/100    1322 ns/op    1920 B/op     2 allocs/op
BenchmarkRangeDiffUpdate                2231 ns/op   14104 B/op    16 allocs/op
BenchmarkRangeDiffInsert                2238 ns/op   14104 B/op    16 allocs/op
BenchmarkRangeDiffRemove                2258 ns/op   14104 B/op    16 allocs/op
BenchmarkRangeDiff_TreeNode_Update     26724 ns/op   33459 B/op   128 allocs/op
BenchmarkRangeDiff_TreeNode_Reorder    13438 ns/op   21116 B/op    22 allocs/op
BenchmarkRangeDiff_TreeNode_LargeList 317231 ns/op  449505 B/op  1038 allocs/op
```

### Key Findings

- No-change detection: ~135ns (was ~257ns) — 48% faster with Dynamics slice
- Small changes detected in ~61ns (was ~177ns) — 66% faster
- Large tree comparison/100: 1322ns / 2 allocs (was 6288ns / 12 allocs) — 79% faster, 83% fewer allocs
- Range operations average ~2.2µs regardless of operation type
- TreeNode range diff operations ~44% faster (e.g., Update 26.7µs vs 48.2µs)
- Memory usage scales linearly with changed nodes

## Phase 4: Render

### Operations

- HTML node rendering
- Tree to HTML conversion
- Void element handling

### Complexity

- **Node rendering:** O(n) where n = nodes in tree

### Optimizations

1. **Efficient String Building**
   - Uses `strings.Builder` for concatenation
   - Reduces allocations

2. **Void Element Handling**
   - Correct self-closing tags (`<br>`, `<img>`, etc.)
   - Single map lookup per element

### Benchmark Results

```
BenchmarkNodeRender                81.89 ns/op   56 B/op      3 allocs/op
BenchmarkTreeToHTML/simple         139.2 ns/op   64 B/op      4 allocs/op
BenchmarkTreeToHTML/nested         250.7 ns/op   144 B/op     7 allocs/op
BenchmarkTreeToHTMLScale/small-10  853.2 ns/op   544 B/op     16 allocs/op
BenchmarkTreeToHTMLScale/medium-100 7146 ns/op   3721 B/op    109 allocs/op
BenchmarkTreeToHTMLScale/large-1000 73113 ns/op  66995 B/op   1017 allocs/op
```

### Key Findings

- Single node rendering: ~82ns (minimal overhead)
- HTML conversion scales linearly: ~73µs per 1000 nodes
- Memory efficient: ~67 KB for 1000-node trees

## Phase 5: Send

### Operations

- Action message parsing (HTTP/WebSocket)
- Update response wrapping
- JSON serialization
- Ordered JSON marshaling

### Complexity

- **Message parsing:** O(message size)
- **JSON serialization:** O(tree size)

### Optimizations

1. **Efficient Parsing**
   - Reuses HTTP form parser
   - WebSocket JSON unmarshaling

2. **Ordered JSON**
   - Deterministic output for testing
   - Keys sorted: "s" first, then numeric

### Benchmark Results

```
BenchmarkParseActionFromHTTP          1968 ns/op    6962 B/op    30 allocs/op
BenchmarkParseActionFromWebSocket     827.0 ns/op   656 B/op     13 allocs/op
BenchmarkSerializeUpdate              998.0 ns/op   648 B/op     10 allocs/op
```

### Key Findings

- WebSocket parsing roughly 2.4x faster than HTTP (827ns vs 1968ns)
- Serialization: ~998ns per update (648 B/op, 10 allocs/op)
- Low allocation counts across all operations

### Async WebSocket Sends

The session registry (`internal/session/`) implements channel-based async message delivery:

**Architecture:**
- Each connection has a dedicated `writePump` goroutine that drains a buffered `sendChan`
- `Send()` queues messages to the channel (lock-free), the pump writes to the WebSocket
- Backpressure: slow clients are closed when buffer is full (fail-fast)
- Graceful shutdown: 5-second drain timeout with `sync.Once` protection

**Configuration:**
- Buffer size: `LVT_WS_BUFFER_SIZE` env var (default: 50)
- Config option: `WithWebSocketBufferSize(int)`
- Metrics: `wsBufferFull`, `wsSlowClientCloses`, `wsWriteErrors`, `wsSendBufferSize`

**Benchmark Results:**

```
BenchmarkRegisterUnregister        2698 ns/op     1210 B/op    13 allocs/op
BenchmarkGetByGroup                276.4 ns/op    896 B/op     1 allocs/op
BenchmarkCloseConnection           1224 ns/op     248 B/op     3 allocs/op
BenchmarkBroadcastToGroup          15726 ns/op    4096 B/op    101 allocs/op
BenchmarkMemoryUsage               40346 ns/op    98126 B/op   620 allocs/op
BenchmarkConcurrentRegistrations   2708 ns/op     1239 B/op    11 allocs/op
BenchmarkConcurrentConnections/1000  143886 ns/op  36001 B/op  2000 allocs/op
```

**Buffer Size Scaling:**

```
BenchmarkBufferSizes/buf_10    275.6 ns/op   113 B/op   3 allocs/op
BenchmarkBufferSizes/buf_50    172.5 ns/op    54 B/op   2 allocs/op
BenchmarkBufferSizes/buf_100   152.7 ns/op    47 B/op   2 allocs/op
BenchmarkBufferSizes/buf_500   175.3 ns/op    36 B/op   2 allocs/op
BenchmarkBufferSizes/buf_1000  175.7 ns/op    36 B/op   2 allocs/op
```

**Key Findings:**
- ~981 bytes per connection (`BenchmarkMemoryUsage`: 98126 B/op / 100 connections)
- Group lookup is fast (~276ns) with dual indexing by groupID and userID
- Register/unregister cycle: ~2.7µs
- Broadcast to 100-connection group: ~16µs (~157ns per connection)
- Buffer size sweet spot: 50-100 messages (diminishing returns above 100)

**Note:** `BenchmarkAsyncSendThroughput`, `BenchmarkConcurrentSend`, and `BenchmarkConcurrentConnections` at 10/100 connections still intermittently fail due to mock WebSocket drain timing. Tracked in [#186](https://github.com/livetemplate/livetemplate/issues/186). The `newDrainingBenchConn` helper (commit ead7c62) fixed the 1000-connection variant.

## End-to-End Performance

### Template Operations

**Initial Render Pipeline:**
1. Parse (if not cached)
2. Build tree WITH statics
3. Render HTML
4. Send (includes full tree)

**Update Pipeline:**
1. Build tree WITH statics (for comparison)
2. Diff against previous tree
3. Send (statics stripped)

### Benchmark Results

```
BenchmarkTemplateExecuteUpdates/no-changes    2263 ns/op   2201 B/op    46 allocs/op
BenchmarkTemplateExecuteUpdates/small         2305 ns/op   2201 B/op    46 allocs/op
BenchmarkTemplateExecuteUpdates/large         6739 ns/op   5187 B/op   123 allocs/op
```

### Real-World Examples

From benchmark data (Go 1.26.0):

| Operation | Latency | Bandwidth Savings |
|-----------|---------|-------------------|
| Initial Render | ~1.9ms | - |
| Small Update (1-2 fields) | ~2.3µs | 85% vs full render |
| Large Update (5+ fields) | ~6.7µs | 65% vs full render |
| Range Operations | ~5.5-9.5µs | 80% vs full render |

Latency numbers are from Go micro-benchmarks (no network). Update latencies dropped ~4x from the previous baseline due to Dynamics slice, shared statics, and buffer pooling. Wire size savings come from static stripping — updates omit cached `"s"` arrays.

### User Journey Performance

```
BenchmarkE2ERangeOperations/add-items      9516 ns/op   9404 B/op   222 allocs/op
BenchmarkE2ERangeOperations/remove-items   5453 ns/op   5495 B/op   124 allocs/op
BenchmarkE2ERangeOperations/reorder-items  6780 ns/op   6776 B/op   157 allocs/op
BenchmarkE2ERangeOperations/update-items   6745 ns/op   6776 B/op   157 allocs/op
```

### HTTP Mode Optimization

HTTP POST handlers now cache templates per session group using `sync.Map`, enabling diff-based updates for HTTP mode:

**How it works:**
- First POST for a session group clones the template and caches it in `httpTemplates` (`sync.Map`)
- Subsequent POSTs diff against the previous tree, matching WebSocket behavior
- Previously each POST was a full render with statics — now only changed dynamics are sent
- Per-entry `sync.Mutex` handles concurrent access from multiple tabs in the same session

**Cleanup:**
- Background `httpTemplateSweepLoop` goroutine runs every 10 minutes
- Evicts cache entries for sessions no longer in the `SessionStore`
- Prevents unbounded memory growth on long-running servers

**Impact:** HTTP mode now benefits from the same bandwidth savings as WebSocket mode (65%+ reduction on updates).

## Scalability Characteristics

### Template Complexity

Allocation counts scale with template complexity (ns/op varies between single runs):

- **Simple fields:** 115 allocs/op, ~4.7 KB/op
- **Conditionals:** 87 allocs/op, ~4.0 KB/op
- **Ranges:** 182 allocs/op, ~7.6 KB/op
- **Nested:** 176 allocs/op, ~7.7 KB/op

### Data Size Scaling

Tree operations scale with data size:

- **10 items:** 6.9µs
- **100 items:** 63µs (10x data, ~9x time)
- **1000 items:** 663µs (100x data, ~96x time)

Scaling is roughly linear with fixed overhead — allocation counts scale at O(n) (266 → 2516 → 25763 allocs/op for 10x data steps), confirming the algorithm is linear, while timing includes per-iteration fixed costs that inflate the apparent ratio at smaller sizes.

### Concurrent Session Scaling

```
BenchmarkTemplateConcurrent/goroutines-1      3704 ns/op   3033 B/op    61 allocs/op
BenchmarkTemplateConcurrent/goroutines-10     3588 ns/op   3033 B/op    61 allocs/op
BenchmarkTemplateConcurrent/goroutines-100    4191 ns/op   3034 B/op    61 allocs/op
```

Per-session allocations remain constant under concurrency (61 allocs/op regardless of goroutine count, down from 170 in previous baseline). The ~3x reduction in allocations comes from Dynamics slice, shared statics, and buffer pooling. Allocation stability confirms no lock contention was introduced.

## Memory Usage

### Per-Session Memory

From benchmark allocations:

- **Initial render (one-time):** ~419 KB allocated (`BenchmarkTemplateExecute/initial-render`)
- **Subsequent render:** ~3 KB allocated (`BenchmarkTemplateExecute/subsequent-render`)
- **Small update:** ~2.2 KB allocated
- **Large update:** ~5.2 KB allocated

### Cache Memory

**Parse Caches:**
- Unbounded growth (relies on GC)
- Typical size: Scales with unique template count

**Fingerprint Cache:**
- Lazy-computed per TreeNode via `GetStructureFingerprint()`
- O(1) comparison via cached fingerprint values
- No LRU or eviction — lifetime tied to tree nodes
- No separate data structure; fingerprint stored directly on `TreeNode`

### Allocation Patterns

From memory profiling:

- **Hot paths:** Tree building, reflection-based data map construction, JSON marshaling
- **Remaining allocations are dominated by Go runtime/stdlib costs** — TreeNode struct heap
  escapes (22.7%), reflection (12.7%), `text/template` internals (3.3%). These cannot be
  eliminated without replacing core Go mechanisms.

## Optimization Status

See [Known Bottlenecks](known-bottlenecks.md) for detailed profiling analysis.

### Diminishing Returns (as of 2026-03-23)

The library has reached a practical optimization floor for allocation-based improvements.
The remaining hotspots are:

1. **TreeNode struct allocation (22.7%)** — investigated `sync.Pool` recycling, rejected
   (only -2.7% gain; Go's GC may drop pool entries at any time). Would require arena allocation
   or replacing the tree-based architecture to improve further.
2. **Reflection overhead (12.7%)** — already deduplicated (PR #224). Further reduction
   requires code generation, which adds build complexity for modest gains.
3. **stdlib template execution (3.3%)** — Go's `text/template` internals; cannot optimize
   without replacing the template engine.

### Completed Optimizations

1. **PR #219: Parse alloc reduction** — eliminated redundant HTML parsing, cached AST, pre-computed builtins. 50-57% allocation reduction per render.
2. **PR #220: Dynamics map to slice** — replaced `map[string]interface{}` with `[]interface{}` for Dynamics. -20.6% geomean across all benchmarks.
3. **PR #224: Shared statics, buffer pool, reflection dedup** — shared sentinel statics, `sync.Pool` for `bytes.Buffer`, deduplicated reflection. -9.7% geomean.
4. **FNV-1a fingerprinting** (commit 1e351ca) — 43-47% faster fingerprinting, dropped from 5.93% CPU to <1%
5. **rangeContext optimization** (commit b9faf28) — 59% faster range diff, 54% fewer allocs
6. **Fingerprint caching** — lazy-computed on TreeNode, O(1) comparison

### Cumulative Impact (PRs #219 + #220 + #224 vs pre-optimization baseline)

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Small update allocs | 154 | 46 | **-70%** |
| Subsequent render allocs | 170 | 61 | **-64%** |
| E2E user journey allocs | 16,174 | 5,083 | **-69%** |
| Concurrent session allocs | 170 | 61 | **-64%** |
| Small update latency | ~10µs | ~2µs | **-80%** |

## Performance Testing

### Running Benchmarks

```bash
make bench
make bench-compare
```

### Regression Monitoring

CI runs benchmarks on every PR:
- Compares against baseline
- Warns on >10% regression (critical paths)
- Fails on >20% regression (critical paths)

### Profiling

```bash
make profile-cpu
make profile-mem
go tool pprof -http=:8080 profiles/cpu.prof
```

## References

- [Benchmarking Guide](benchmarking-guide.md) - How to run and interpret benchmarks
- [Known Bottlenecks](known-bottlenecks.md) - Current optimization opportunities
- [Tree Update Specification](../specifications/tree-update-specification.md) - Wire format details
