# LiveTemplate Performance Characteristics

> **Benchmark Environment:** Go 1.26.0, arm64 (Apple M2). Numbers updated 2026-03-19.
> These are single-run results (`make bench-save`); ns/op timings can vary significantly
> between runs (2-4x swings observed) due to system load, thermal throttling, and GC timing.
> Allocation counts (B/op, allocs/op) are deterministic and stable across runs — use these
> to assess actual code changes. For statistically confident timing comparisons, use
> `make bench-10x` with benchstat. The baseline is single-run to keep CI fast.
>
> **Baseline change notes:** This baseline captures the FNV-1a fingerprinting optimization
> (commit 1e351ca, replacing MD5) and `rangeContext` range diff optimization (commit b9faf28).
> Key allocation improvements vs the previous (2026-03-16) baseline: subsequent render 245→170
> allocs, small update 229→154 allocs, large update 752→357 allocs, E2E range add 844→406
> allocs, E2E user journey 23652→16174 allocs. These are real improvements from the FNV-1a
> and rangeContext optimizations compounding across the update pipeline.

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
BenchmarkParse/simple            1764 ns/op    3744 B/op     43 allocs/op
BenchmarkParse/conditional       2861 ns/op    4504 B/op     64 allocs/op
BenchmarkParse/range             2527 ns/op    4336 B/op     59 allocs/op
BenchmarkBuildTree/simple        1216 ns/op    2825 B/op     27 allocs/op
BenchmarkBuildTree/cond-true     1900 ns/op    4129 B/op     48 allocs/op
BenchmarkBuildTree/range-small   4470 ns/op    7582 B/op    126 allocs/op
BenchmarkBuildTreeScale/small-10      11844 ns/op   19519 B/op    365 allocs/op
BenchmarkBuildTreeScale/medium-100   107243 ns/op  173291 B/op   3425 allocs/op
BenchmarkBuildTreeScale/large-1000  1095077 ns/op 1714725 B/op  34775 allocs/op
```

### Key Findings

- Simple template parsing: ~1.8µs with minimal allocations
- BuildTree is 5-9x faster than the previous re-parse approach
- 67-75% fewer allocations across all benchmarks
- Performance gains scale with template complexity
- Tree building dominates for large datasets (100+ items)
- Memory usage scales predictably: ~11.9 MB per 1000 items

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

### Benchmark Results

```
BenchmarkTreeNodeCreation/flat           624.4 ns/op    1312 B/op    19 allocs/op
BenchmarkTreeNodeMarshalJSON/nested-small  46096 ns/op   27943 B/op   358 allocs/op
BenchmarkWrapperInjection/full-html      3075 ns/op    7096 B/op    37 allocs/op
```

### Key Findings

- Flat tree creation is fast (~624ns)
- JSON marshaling is the primary cost for nested structures
- Wrapper injection adds minimal overhead (~3.1µs)

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

### Benchmark Results

```
BenchmarkCompareTreesNoChanges          257.1 ns/op   128 B/op     2 allocs/op
BenchmarkCompareTreesSmallChange        176.6 ns/op   416 B/op     3 allocs/op
BenchmarkCompareTreesLargeChange/10      660.6 ns/op   1032 B/op    6 allocs/op
BenchmarkCompareTreesLargeChange/100    6288 ns/op    9448 B/op    12 allocs/op
BenchmarkRangeDiffUpdate                2205 ns/op   14122 B/op   17 allocs/op
BenchmarkRangeDiffInsert                2192 ns/op   14122 B/op   17 allocs/op
BenchmarkRangeDiffRemove                2193 ns/op   14122 B/op   17 allocs/op
BenchmarkRangeDiff_TreeNode_Update     48169 ns/op   33622 B/op  128 allocs/op
BenchmarkRangeDiff_TreeNode_Reorder    28405 ns/op   21187 B/op   22 allocs/op
BenchmarkRangeDiff_TreeNode_LargeList 532559 ns/op  448500 B/op 1038 allocs/op
```

### Key Findings

- No-change detection is extremely fast (~257ns)
- Small changes detected in ~177ns
- Range operations average ~2.2µs regardless of operation type (down from ~5.4µs)
- Memory usage scales linearly with changed nodes

## Phase 4: Render

### Operations

- HTML node rendering
- Tree to HTML conversion
- HTML minification (optional)
- Void element handling

### Complexity

- **Node rendering:** O(n) where n = nodes in tree
- **Minification:** O(html length)

### Optimizations

1. **Efficient String Building**
   - Uses `strings.Builder` for concatenation
   - Reduces allocations

2. **Void Element Handling**
   - Correct self-closing tags (`<br>`, `<img>`, etc.)
   - Single map lookup per element

3. **Minification Optional**
   - Only enabled in production
   - Removes whitespace, comments

### Benchmark Results

```
BenchmarkNodeRender                70.95 ns/op   56 B/op      3 allocs/op
BenchmarkTreeToHTML/simple         116.8 ns/op   64 B/op      4 allocs/op
BenchmarkTreeToHTML/nested         209.1 ns/op   144 B/op     7 allocs/op
BenchmarkTreeToHTMLScale/small-10  731.7 ns/op   544 B/op     16 allocs/op
BenchmarkTreeToHTMLScale/medium-100 6076 ns/op   3721 B/op    109 allocs/op
BenchmarkTreeToHTMLScale/large-1000 63109 ns/op  66996 B/op   1017 allocs/op
```

### Key Findings

- Single node rendering: ~71ns (minimal overhead)
- HTML conversion scales linearly: ~63µs per 1000 nodes
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
BenchmarkParseActionFromHTTP          1746 ns/op    6962 B/op    30 allocs/op
BenchmarkParseActionFromWebSocket     722.5 ns/op   656 B/op     13 allocs/op
BenchmarkSerializeUpdate              877.5 ns/op   648 B/op     10 allocs/op
```

### Key Findings

- WebSocket parsing roughly 2.4x faster than HTTP (723ns vs 1746ns)
- Serialization: ~878ns per update (648 B/op, 10 allocs/op)
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
BenchmarkRegisterUnregister        2508 ns/op     1210 B/op    13 allocs/op
BenchmarkGetByGroup                246.4 ns/op    896 B/op     1 allocs/op
BenchmarkCloseConnection           1344 ns/op     248 B/op     3 allocs/op
BenchmarkBroadcastToGroup          21370 ns/op    4096 B/op    101 allocs/op
BenchmarkMemoryUsage               55731 ns/op    98101 B/op   620 allocs/op
BenchmarkConcurrentRegistrations   2738 ns/op     1241 B/op    11 allocs/op
BenchmarkConcurrentConnections/1000  145226 ns/op  36001 B/op  2000 allocs/op
```

**Buffer Size Scaling:**

```
BenchmarkBufferSizes/buf_10    267.4 ns/op   114 B/op   3 allocs/op
BenchmarkBufferSizes/buf_50    160.7 ns/op    56 B/op   2 allocs/op
BenchmarkBufferSizes/buf_100   139.1 ns/op    48 B/op   2 allocs/op
BenchmarkBufferSizes/buf_500   155.5 ns/op    37 B/op   2 allocs/op
BenchmarkBufferSizes/buf_1000  156.6 ns/op    36 B/op   2 allocs/op
```

**Key Findings:**
- ~981 bytes per connection (`BenchmarkMemoryUsage`: 98101 B/op ÷ 100 connections)
- Group lookup is fast (~246ns) with dual indexing by groupID and userID
- Register/unregister cycle: ~2.5µs
- Broadcast to 100-connection group: ~21µs (~214ns per connection)
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
BenchmarkTemplateExecuteUpdates/no-changes    9666 ns/op   19840 B/op   154 allocs/op
BenchmarkTemplateExecuteUpdates/small         9579 ns/op   19840 B/op   154 allocs/op
BenchmarkTemplateExecuteUpdates/large        22937 ns/op   30390 B/op   357 allocs/op
```

### Real-World Examples

From benchmark data (Go 1.26.0):

| Operation | Latency | Bandwidth Savings |
|-----------|---------|-------------------|
| Initial Render | ~16-21µs | - |
| Small Update (1-2 fields) | ~10µs | 85% vs full render |
| Large Update (5+ fields) | ~21-23µs | 65% vs full render |
| Range Operations | ~16-24µs | 80% vs full render |

Latency numbers are from Go micro-benchmarks (no network) and include per-benchmark template setup overhead. Wire size savings come from static stripping — updates omit cached `"s"` arrays.

### User Journey Performance

```
BenchmarkE2ERangeOperations/add-items      22950 ns/op   32967 B/op   406 allocs/op
BenchmarkE2ERangeOperations/remove-items   16015 ns/op   25839 B/op   268 allocs/op
BenchmarkE2ERangeOperations/reorder-items  18402 ns/op   28241 B/op   315 allocs/op
BenchmarkE2ERangeOperations/update-items   23705 ns/op   28240 B/op   315 allocs/op
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

Performance scales linearly with template complexity:

- **Simple fields:** ~16µs per render
- **Conditionals:** ~14µs per render
- **Ranges:** ~21µs per render (large update)
- **Nested:** ~21µs per render

### Data Size Scaling

Tree operations scale with data size:

- **10 items:** 12µs
- **100 items:** 107µs (10x data, ~9x time)
- **1000 items:** 1.1ms (100x data, ~92x time)

Scaling is roughly linear with fixed overhead — allocation counts scale at O(n) (365 → 3425 → 34775 allocs/op for 10x data steps), confirming the algorithm is linear, while timing includes per-iteration fixed costs that inflate the apparent ratio at smaller sizes.

### Concurrent Session Scaling

```
BenchmarkTemplateConcurrent/goroutines-1      14058 ns/op   20819 B/op   170 allocs/op
BenchmarkTemplateConcurrent/goroutines-10     12477 ns/op   20824 B/op   170 allocs/op
BenchmarkTemplateConcurrent/goroutines-100    14549 ns/op   20827 B/op   170 allocs/op
```

Per-session allocations remain constant under concurrency (170 allocs/op regardless of goroutine count). The degree of speedup varies between single-run baselines due to system load and thermal state — allocation stability confirms no lock contention was introduced.

## Memory Usage

### Per-Session Memory

From benchmark allocations:

- **Initial render:** ~20 KB allocated
- **Small update:** ~19 KB allocated
- **Large update:** ~30 KB allocated

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

- **Hot paths:** Tree building, JSON marshaling, AST evaluation
- **Optimization opportunities:** Reuse tree nodes, pool allocations, pool evaluators

## Optimization Opportunities

See [Known Bottlenecks](known-bottlenecks.md) for detailed profiling analysis.

### Current Priorities

1. Template parsing allocations (29.52% of total — html.NewTokenizerFragment)
2. TreeNode operations (12% of total — SetDynamic, NewTreeNode, NewTreeNodeWithStatics)
3. Parse evaluator allocations (5.90% — newEvaluator)

### Recently Completed

1. **FNV-1a fingerprinting** (commit 1e351ca) — 43-47% faster fingerprinting, dropped from 5.93% CPU to <1%
2. **rangeContext optimization** (commit b9faf28) — 59% faster range diff, 54% fewer allocs
3. **Fingerprint caching** — lazy-computed on TreeNode, O(1) comparison

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
