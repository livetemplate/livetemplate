# LiveTemplate Performance Characteristics

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
BenchmarkParse/simple            1674 ns/op    3744 B/op     43 allocs/op
BenchmarkParse/conditional       2843 ns/op    4504 B/op     64 allocs/op
BenchmarkParse/range             2561 ns/op    4336 B/op     59 allocs/op
BenchmarkBuildTree/small-10      65382 ns/op   116537 B/op   1101 allocs/op
BenchmarkBuildTree/medium-100    663478 ns/op  1154593 B/op  10740 allocs/op
BenchmarkBuildTree/large-1000    7677857 ns/op 11552228 B/op 107122 allocs/op
```

### Key Findings

- Simple template parsing: ~1.7µs with minimal allocations
- Complex templates scale linearly with template size
- Tree building dominates for large datasets (100+ items)
- Memory usage scales predictably: ~11.5 MB per 1000 items

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
BenchmarkTreeNodeCreation/flat          446.8 ns/op   1236 B/op    24 allocs/op
BenchmarkTreeNodeMarshalJSON/nested     43895 ns/op   27942 B/op   358 allocs/op
BenchmarkWrapperInjection/full-html     2810 ns/op    7096 B/op    37 allocs/op
```

### Key Findings

- Flat tree creation is extremely fast (<500ns)
- JSON marshaling is the primary cost for nested structures
- Wrapper injection adds minimal overhead (~2.8µs)

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

1. **Registry-Based Static Stripping**
   - Client caches statics after first render
   - Updates omit static arrays
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
BenchmarkCompareTreesNoChanges          430.7 ns/op   112 B/op     2 allocs/op
BenchmarkCompareTreesSmallChange        158.9 ns/op   400 B/op     3 allocs/op
BenchmarkCompareTreesLargeChange/10     827.1 ns/op   1016 B/op    6 allocs/op
BenchmarkCompareTreesLargeChange/100    8232 ns/op    9432 B/op    12 allocs/op
BenchmarkRangeDiffUpdate                4594 ns/op    19032 B/op   36 allocs/op
BenchmarkRangeDiffInsert                4752 ns/op    19032 B/op   36 allocs/op
```

### Key Findings

- No-change detection is extremely fast (~431ns)
- Small changes detected in <200ns
- Range operations average ~4.6µs regardless of operation type
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
BenchmarkNodeRender                68.05 ns/op   56 B/op      3 allocs/op
BenchmarkTreeToHTML/simple         113.9 ns/op   64 B/op      4 allocs/op
BenchmarkTreeToHTML/nested         205.2 ns/op   144 B/op     7 allocs/op
BenchmarkTreeToHTMLScale/small-10  706.3 ns/op   544 B/op     16 allocs/op
BenchmarkTreeToHTMLScale/medium-100 5912 ns/op   3721 B/op    109 allocs/op
BenchmarkTreeToHTMLScale/large-1000 60374 ns/op  66999 B/op   1017 allocs/op
```

### Key Findings

- Single node rendering: ~68ns (minimal overhead)
- HTML conversion scales linearly: ~60µs per 1000 nodes
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
BenchmarkParseActionFromHTTP          1719 ns/op    6962 B/op    30 allocs/op
BenchmarkParseActionFromWebSocket     642.4 ns/op   656 B/op     13 allocs/op
BenchmarkSerializeUpdate              834.3 ns/op   648 B/op     10 allocs/op
```

### Key Findings

- WebSocket parsing 2.7x faster than HTTP (642ns vs 1719ns)
- Serialization overhead minimal (<1µs)
- Low allocation counts across all operations

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
BenchmarkTemplateExecuteUpdates/no-changes   17262 ns/op   28041 B/op   221 allocs/op
BenchmarkTemplateExecuteUpdates/small        17150 ns/op   28041 B/op   221 allocs/op
BenchmarkTemplateExecuteUpdates/large        59740 ns/op   78055 B/op   728 allocs/op
```

### Real-World Examples

From README performance numbers:

| Operation | Latency | Bandwidth |
|-----------|---------|-----------|
| Initial Render | ~1.2ms | 1782 bytes |
| First Update | ~120µs | 695 bytes (61% savings) |
| Second Update | ~120µs | 244 bytes (86% savings) |

### User Journey Performance

```
BenchmarkE2ERangeOperations/add-items      63018 ns/op   82480 B/op   756 allocs/op
BenchmarkE2ERangeOperations/remove-items   32976 ns/op   44672 B/op   405 allocs/op
BenchmarkE2ERangeOperations/reorder-items  43085 ns/op   58279 B/op   525 allocs/op
BenchmarkE2ERangeOperations/update-items   43812 ns/op   58280 B/op   525 allocs/op
```

## Scalability Characteristics

### Template Complexity

Performance scales linearly with template complexity:

- **Simple fields:** ~17µs per render
- **Conditionals:** ~17µs per render (optimized early-exit)
- **Ranges:** ~60µs per render (large update)
- **Nested:** Similar to ranges when data is comparable

### Data Size Scaling

Tree operations scale with data size:

- **10 items:** 65µs
- **100 items:** 663µs (10x data, ~10x time)
- **1000 items:** 7.7ms (100x data, ~100x time)

Confirms O(n) complexity.

### Concurrent Session Scaling

```
BenchmarkTemplateConcurrent/1      22710 ns/op   28999 B/op   237 allocs/op
BenchmarkTemplateConcurrent/10     20752 ns/op   29007 B/op   237 allocs/op
BenchmarkTemplateConcurrent/100    23224 ns/op   29006 B/op   237 allocs/op
```

Performance per session remains constant under concurrency.

## Memory Usage

### Per-Session Memory

From benchmark allocations:

- **Initial render:** ~28 KB allocated
- **Small update:** ~28 KB allocated
- **Large update:** ~78 KB allocated

### Cache Memory

**Parse Caches:**
- Unbounded growth (relies on GC)
- Typical size: Scales with unique template count

**Structure Registry:**
- Max 1000 entries (LRU eviction)
- Estimated: ~1-2 MB for typical applications

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
- [Tree Update Specification](../../docs/tree-update-specification.md) - Wire format details
