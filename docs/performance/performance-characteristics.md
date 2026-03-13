# LiveTemplate Performance Characteristics

> **Benchmark Environment:** Go 1.26.0, arm64 (Apple M2). Numbers updated March 2026.
> Absolute timings are higher than the November 2025 baseline due to expanded benchmark
> scope (per-benchmark template setup costs, Controller+State cloning overhead). Allocation
> counts and memory-per-operation remain comparable. Relative performance characteristics
> (O(n) scaling, phase ratios) are unchanged.

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
- **Expression evaluation:** O(expression complexity), cached

### Optimizations

1. **Template Caching**
   - `pipeTemplateCache`: Caches compiled pipelines
   - `astTemplateCache`: Caches AST structures
   - Eliminates re-parsing on identical templates

2. **Expression Result Caching**
   - Cache key: `(templateName, pipeStr, dataHash)`
   - Intra-render optimization
   - Significant for repeated evaluations

3. **Lazy Initialization**
   - Capture functions created on-demand
   - Reduces memory for unused constructs

### Benchmark Results

From baseline (`testdata/benchmarks/baseline.txt`):

```
BenchmarkParse/simple            2575 ns/op    3744 B/op     43 allocs/op
BenchmarkParse/conditional       3340 ns/op    4504 B/op     64 allocs/op
BenchmarkParse/range             3609 ns/op    4336 B/op     59 allocs/op
BenchmarkBuildTreeScale/small-10      77563 ns/op   120040 B/op   1197 allocs/op
BenchmarkBuildTreeScale/medium-100    1295268 ns/op 1190517 B/op  11736 allocs/op
BenchmarkBuildTreeScale/large-1000    8085147 ns/op 11911935 B/op 117113 allocs/op
```

### Key Findings

- Simple template parsing: ~2.6µs with minimal allocations
- Complex templates scale linearly with template size
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
   - MD5 hash of tree structure
   - Fast equality check without deep comparison
   - Cached until tree changes

### Benchmark Results

```
BenchmarkTreeNodeCreation/flat          1228 ns/op    1312 B/op    19 allocs/op
BenchmarkTreeNodeMarshalJSON/nested-small  46611 ns/op   27940 B/op   358 allocs/op
BenchmarkWrapperInjection/full-html     3397 ns/op    7096 B/op    37 allocs/op
```

### Key Findings

- Flat tree creation is fast (~1.2µs)
- JSON marshaling is the primary cost for nested structures
- Wrapper injection adds minimal overhead (~3.4µs)

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
   - Structure fingerprints (MD5, 64-bit) compared between old/new trees
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

### Benchmark Results

```
BenchmarkCompareTreesNoChanges          459.1 ns/op   128 B/op     2 allocs/op
BenchmarkCompareTreesSmallChange        215.1 ns/op   416 B/op     3 allocs/op
BenchmarkCompareTreesLargeChange/10     1044 ns/op    1032 B/op    6 allocs/op
BenchmarkCompareTreesLargeChange/100    10144 ns/op   9448 B/op    12 allocs/op
BenchmarkRangeDiffUpdate                5332 ns/op    18944 B/op   37 allocs/op
BenchmarkRangeDiffInsert                5411 ns/op    18944 B/op   37 allocs/op
```

### Key Findings

- No-change detection is extremely fast (~459ns)
- Small changes detected in ~215ns
- Range operations average ~5.4µs regardless of operation type
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
BenchmarkNodeRender                69.08 ns/op   56 B/op      3 allocs/op
BenchmarkTreeToHTML/simple         126.2 ns/op   64 B/op      4 allocs/op
BenchmarkTreeToHTML/nested         217.7 ns/op   144 B/op     7 allocs/op
BenchmarkTreeToHTMLScale/small-10  720.0 ns/op   544 B/op     16 allocs/op
BenchmarkTreeToHTMLScale/medium-100 6071 ns/op   3721 B/op    109 allocs/op
BenchmarkTreeToHTMLScale/large-1000 62296 ns/op  66996 B/op   1017 allocs/op
```

### Key Findings

- Single node rendering: ~69ns (minimal overhead)
- HTML conversion scales linearly: ~62µs per 1000 nodes
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
BenchmarkParseActionFromHTTP          3714 ns/op    6962 B/op    30 allocs/op
BenchmarkParseActionFromWebSocket     927.2 ns/op   656 B/op     13 allocs/op
BenchmarkSerializeUpdate              3729 ns/op    648 B/op     10 allocs/op
```

### Key Findings

- WebSocket parsing 4.0x faster than HTTP (927ns vs 3714ns)
- Serialization overhead minimal (~3.7µs)
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
BenchmarkRegisterUnregister        3402 ns/op     1213 B/op    13 allocs/op
BenchmarkGetByGroup                418.4 ns/op    896 B/op     1 allocs/op
BenchmarkCloseConnection           3719 ns/op     254 B/op     3 allocs/op
BenchmarkBroadcastToGroup          41241 ns/op    4096 B/op    101 allocs/op
BenchmarkMemoryUsage               68110 ns/op    98028 B/op   619 allocs/op
BenchmarkConcurrentRegistrations   5131 ns/op     1245 B/op    11 allocs/op
```

**Key Findings:**
- ~980 bytes per connection (measured via `BenchmarkMemoryUsage`)
- Group lookup is fast (~418ns) with dual indexing by groupID and userID
- Register/unregister cycle: ~3.4µs
- Broadcast to 100-connection group: ~41µs (~410ns per connection)

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
BenchmarkTemplateExecuteUpdates/no-changes   26143 ns/op   28320 B/op   229 allocs/op
BenchmarkTemplateExecuteUpdates/small        38554 ns/op   28320 B/op   229 allocs/op
BenchmarkTemplateExecuteUpdates/large        233551 ns/op  78625 B/op   752 allocs/op
```

### Real-World Examples

From benchmark data (Go 1.26, expanded benchmark scope including per-benchmark template setup):

| Operation | Latency | Bandwidth Savings |
|-----------|---------|-------------------|
| Initial Render | ~20-76µs | - |
| Small Update (1-2 fields) | ~26-39µs | 85% vs full render |
| Large Update (5+ fields) | ~85-234µs | 65% vs full render |
| Range Operations | ~79-121µs | 80% vs full render |

Latency numbers are from Go micro-benchmarks (no network) and include per-benchmark template setup overhead. Wire size savings come from static stripping — updates omit cached `"s"` arrays. The README shows lower latency ranges (~30-65µs for range ops) which reflect the original Go 1.21 baseline without per-benchmark setup costs.

### User Journey Performance

```
BenchmarkE2ERangeOperations/add-items      121190 ns/op   84713 B/op   844 allocs/op
BenchmarkE2ERangeOperations/remove-items   79139 ns/op    45888 B/op   456 allocs/op
BenchmarkE2ERangeOperations/reorder-items  88578 ns/op    59833 B/op   588 allocs/op
BenchmarkE2ERangeOperations/update-items   85062 ns/op    59833 B/op   588 allocs/op
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

- **Simple fields:** ~89µs per render
- **Conditionals:** ~117µs per render (optimized early-exit)
- **Ranges:** ~137µs per render (large update)
- **Nested:** ~197µs per render

### Data Size Scaling

Tree operations scale with data size:

- **10 items:** 78µs
- **100 items:** 1.3ms (10x data, ~17x time)
- **1000 items:** 8.1ms (100x data, ~104x time)

Confirms O(n) complexity.

### Concurrent Session Scaling

```
BenchmarkTemplateConcurrent/goroutines-1      74338 ns/op   29305 B/op   245 allocs/op
BenchmarkTemplateConcurrent/goroutines-10     67784 ns/op   29307 B/op   245 allocs/op
BenchmarkTemplateConcurrent/goroutines-100    58917 ns/op   29310 B/op   245 allocs/op
```

Performance per session remains constant under concurrency.

## Memory Usage

### Per-Session Memory

From benchmark allocations:

- **Initial render:** ~29 KB allocated
- **Small update:** ~28 KB allocated
- **Large update:** ~79 KB allocated

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

- **Hot paths:** Tree building, JSON marshaling
- **Optimization opportunities:** Reuse tree nodes, pool allocations

## Optimization Opportunities

See [Known Bottlenecks](known-bottlenecks.md) for detailed profiling analysis.

### Current Priorities

1. Tree building for large datasets (1000+ items)
2. JSON marshaling for deeply nested structures
3. Template parsing cache efficiency

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
