# Known Performance Bottlenecks

**Last Profiled:** 2026-03-22
**Go Version (profiles):** 1.26.0
**Go Version (benchmarks):** 1.26.0
**Architecture:** arm64 (Apple M2)

## Profiling Methodology

Profiles generated using:
```bash
make profile-cpu
make profile-mem
```

Analyzed using:
```bash
go tool pprof -top -cum profiles/cpu.prof
go tool pprof -top -alloc_space profiles/mem.prof
```

## CPU Bottlenecks

### Analysis Summary

```
File: livetemplate.test
Type: cpu
Time: 2026-03-19 20:53:34 CET
Duration: 66.90s, Total samples = 100.03s (149.52%)
Showing nodes accounting for 84.02s, 83.99% of 100.03s total
Dropped 1039 nodes (cum <= 0.50s)
      flat  flat%   sum%        cum   cum%
     0.09s  0.09%  0.09%     59.39s 59.37%  runtime.systemstack
     0.02s  0.02%  0.11%     26.20s 26.19%  runtime.gcBgMarkWorker.func2
     0.23s  0.23%  0.34%     26.12s 26.11%  runtime.gcDrain
    23.54s 23.53% 23.87%     23.54s 23.53%  runtime.kevent
         0     0% 23.87%     23.54s 23.53%  runtime.netpoll
         0     0% 23.87%     23.46s 23.45%  runtime.startTheWorldWithSema
         0     0% 23.87%     23.29s 23.28%  runtime.gcStart.func4
     0.03s  0.03% 23.90%     18.37s 18.36%  runtime.schedule
     0.04s  0.04% 23.94%     18.35s 18.34%  runtime.gcBgMarkWorker
     0.01s  0.01% 23.95%     17.41s 17.40%  runtime.mcall
     0.04s  0.04% 23.99%     17.01s 17.00%  runtime.park_m
     0.16s  0.16% 24.15%     16.90s 16.89%  runtime.findRunnable
         0     0% 24.15%     16.87s 16.86%  runtime.gcDrainMarkWorkerIdle (inline)
```

### Key Findings

#### Runtime/GC Overhead
- **Impact:** ~59% of CPU time in garbage collection
- **Analysis:** The benchmark suite allocates heavily, triggering frequent GC cycles
- **Optimization Opportunity:** Reduce allocations (see Memory Bottlenecks section)

#### Fingerprinting (Phase 2: Build)
- **Location:** `internal/build/fingerprint.go:CalculateStructureFingerprint`
- **Impact:** No longer a measurable CPU hotspot (previously 5.93% of CPU samples in Nov 2025)
- **Status:** Replaced MD5 with FNV-1a 128-bit (commit 1e351ca). Lazy-cached on TreeNode via `GetStructureFingerprint()`. No longer a significant bottleneck.
- **Tradeoff:** `ClientNeedsStatics` fingerprint comparison slowed from ~4ns to ~6.6ns (zero-alloc, likely from `atomic.Value` type assertion overhead). Acceptable: the 2.6ns increase per comparison is negligible vs the ~3100ns saved per range diff operation.

#### Overall Distribution
Most CPU time is spent in:
1. Garbage collection (59%)
2. Runtime overhead (scheduling, memory management)
3. Application logic (template rendering, tree diffing)

The high GC overhead indicates that memory allocation reduction would have the most significant performance impact.

## Memory Bottlenecks

> **Note:** Profile shape changed significantly after PRs #219, #220, #224. Previous top allocator
> `SetDynamic` (6.13%) dropped to 1.56% due to Dynamics map→slice conversion. `newEvaluator` (5.90%)
> eliminated by pre-computed builtins. NewTreeNode struct allocation is now the dominant LiveTemplate hotspot.

### Analysis Summary

```
File: livetemplate.test
Type: alloc_space
Time: 2026-03-22
Showing top allocators (LiveTemplate-specific highlighted below)
      flat  flat%   sum%        cum   cum%
 5398.16MB 12.09%         github.com/livetemplate/livetemplate/internal/build.NewTreeNode
 4096.50MB  9.18%         github.com/livetemplate/livetemplate/internal/build.NewTreeNodeWithStatics
 3597.37MB  8.06%         github.com/livetemplate/livetemplate/internal/context.buildDataMapWithContext
 1784.03MB  4.00%         reflect.unsafe_New
 1456.09MB  3.26%         text/template.(*Template).execute
 1361.57MB  3.05%         golang.org/x/net/html.NewTokenizerFragment
 1245.82MB  2.79%         encoding/json.(*decodeState).objectInterface
 1174.12MB  2.63%         github.com/livetemplate/livetemplate/internal/context.executeWithBuffer
 1039.63MB  2.33%         github.com/livetemplate/livetemplate/internal/diff.CompareTreesAndGetChangesWithPath
  865.19MB  1.94%         github.com/livetemplate/livetemplate/internal/build.(*TreeNode).MarshalJSON
  849.02MB  1.90%         github.com/livetemplate/livetemplate/internal/parse.walkAST
  833.58MB  1.87%         github.com/livetemplate/livetemplate.(*Template).renderHTMLWithData
  790.13MB  1.77%         github.com/livetemplate/livetemplate/internal/diff.FindRangeConstructMatches
  697.52MB  1.56%         github.com/livetemplate/livetemplate/internal/build.(*TreeNode).SetDynamic
  664.11MB  1.49%         encoding/json.mapEncoder.encode
  622.53MB  1.39%         github.com/livetemplate/livetemplate/internal/parse.walkList
  603.14MB  1.35%         github.com/livetemplate/livetemplate/internal/build.(*TreeNode).GetDynamics
  565.01MB  1.27%         github.com/livetemplate/livetemplate/internal/build.hashStructureWithCircularDetection
  556.57MB  1.25%         text/template.builtins
  520.53MB  1.17%         github.com/livetemplate/livetemplate/internal/parse.newOrderedVars
  514.64MB  1.15%         github.com/livetemplate/livetemplate/internal/parse.precomputeBuiltins
```

### LiveTemplate-Specific Allocations

```
 5398.16MB 12.09%  github.com/livetemplate/livetemplate/internal/build.NewTreeNode
 4096.50MB  9.18%  github.com/livetemplate/livetemplate/internal/build.NewTreeNodeWithStatics
 3597.37MB  8.06%  github.com/livetemplate/livetemplate/internal/context.buildDataMapWithContext
 1174.12MB  2.63%  github.com/livetemplate/livetemplate/internal/context.executeWithBuffer
 1039.63MB  2.33%  github.com/livetemplate/livetemplate/internal/diff.CompareTreesAndGetChangesWithPath
  865.19MB  1.94%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).MarshalJSON
  849.02MB  1.90%  github.com/livetemplate/livetemplate/internal/parse.walkAST
  833.58MB  1.87%  github.com/livetemplate/livetemplate.(*Template).renderHTMLWithData
  790.13MB  1.77%  github.com/livetemplate/livetemplate/internal/diff.FindRangeConstructMatches
  697.52MB  1.56%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).SetDynamic
  622.53MB  1.39%  github.com/livetemplate/livetemplate/internal/parse.walkList
  603.14MB  1.35%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).GetDynamics
  565.01MB  1.27%  github.com/livetemplate/livetemplate/internal/build.hashStructureWithCircularDetection
  520.53MB  1.17%  github.com/livetemplate/livetemplate/internal/parse.newOrderedVars
  514.64MB  1.15%  github.com/livetemplate/livetemplate/internal/parse.precomputeBuiltins
```

### Allocations per Operation

**Initial Render (includes template parsing, one-time cost):**
- Total allocations: ~3,910 allocs/op
- Bytes allocated: ~419 KB/op
- Example: BenchmarkTemplateExecute/initial-render-8 (1898294 ns/op, 419101 B/op, 3910 allocs/op)

**Subsequent Render (per-session, reuses parsed template):**
- Total allocations: ~61 allocs/op
- Bytes allocated: ~3 KB/op
- Example: BenchmarkTemplateExecute/subsequent-render-8 (3025 ns/op, 3033 B/op, 61 allocs/op)

**Small Update:**
- Total allocations: ~46 allocs/op
- Bytes allocated: ~2.2 KB/op
- Example: BenchmarkTemplateExecuteUpdates/small-update-8 (2263 ns/op, 2201 B/op, 46 allocs/op)

**Large Update:**
- Total allocations: ~123 allocs/op
- Bytes allocated: ~5.2 KB/op
- Example: BenchmarkTemplateExecuteUpdates/large-update-8 (6789 ns/op, 5187 B/op, 123 allocs/op)

**Range Operations:**
- Add items: 222 allocs/op, 9.4 KB/op
- Remove items: 124 allocs/op, 5.5 KB/op
- Reorder items: 157 allocs/op, 6.8 KB/op
- Update items: 157 allocs/op, 6.8 KB/op

**Complex User Journey (E2E):**
- Total allocations: ~5,083 allocs/op
- Bytes allocated: ~252 KB/op
- Example: BenchmarkE2EUserJourney-8 (256978 ns/op, 251941 B/op, 5083 allocs/op)

### Cache Memory Usage

**Parse Caches:**
- Template parsing involves significant allocations in text/template and html/template
- builtins: 557 MB (1.25% of total) — down from 4.1 GB after pre-computation
- Template escaper operations: reduced after AST caching
- newEvaluator: eliminated as allocator (builtins pre-computed in PR #219)
- precomputeBuiltins: 515 MB (1.15%) — one-time cost per template parse

**Fingerprint Cache:**
- Lazy-computed per TreeNode via `GetStructureFingerprint()`
- FNV-1a 128-bit hash truncated to 64 bits (16 hex chars)
- No eviction needed — cached on the tree node itself, lifetime tied to node
- Replaced the earlier `ClientStructureRegistry` (LRU, max 1000 entries), removed in PR #86

## Optimization Priorities

Based on profiling data, prioritize:

1. **[High] TreeNode Struct Pooling (Phase 2: Build)**
   - Current: 5.4 GB in NewTreeNode (12.09% of allocations)
   - Current: 4.1 GB in NewTreeNodeWithStatics (9.18% of allocations)
   - Combined: 22.4% of total — now the dominant LiveTemplate hotspot
   - Potential improvement: `sync.Pool` for TreeNode structs, reset and reuse
   - Impact: ~20% reduction in total allocations and GC pressure

2. **[High] Reflection Overhead**
   - Current: 3.6 GB in buildDataMapWithContext (8.06%)
   - Current: 1.8 GB in reflect.unsafe_New (4.00%)
   - Potential improvement: Code generation for data map construction, reduce reflection in hot paths
   - Impact: ~12% reduction in total allocations

3. **[Medium] stdlib Template Execution**
   - Current: 1.5 GB in text/template.(*Template).execute (3.26%)
   - Current: 1.2 GB in executeWithBuffer (2.63%)
   - Potential improvement: Template caching optimizations, reduce per-execution overhead
   - Impact: ~6% reduction in total allocations

4. **[Medium] JSON Serialization (Phase 5: Send)**
   - Current: 1.2 GB in JSON decoding (2.79%)
   - Current: 865 MB in MarshalJSON (1.94%)
   - Current: 664 MB in mapEncoder.encode (1.49%)
   - Potential improvement: Use faster JSON library or custom serialization
   - Impact: ~6% reduction in total allocations

5. **[Low] Diff Operations (Phase 3: Diff)**
   - Current: 1.0 GB in CompareTreesAndGetChangesWithPath (2.33%)
   - Current: 790 MB in FindRangeConstructMatches (1.77%)
   - Potential improvement: Algorithmic optimizations
   - Impact: ~4% of total allocations

## Optimization Task List

### High Priority Tasks

- [x] **Eliminate Redundant HTML Parsing + Cache Template AST** *(Completed 2026-03-21, PR #219)*
  - Location: `template.go`, `internal/parse/api.go`, `internal/compat/tree.go`
  - Approach: (1) Eliminated redundant `ExtractTemplateContent` calls on main update path — html.Parse() now only runs on first render and rare fallback. (2) Cached `*parse.Template` AST after first parse, reused on subsequent renders. (3) Pre-computed evaluator builtins map once at parse time.
  - Actual Impact: 50-57% allocation reduction per render. E2E user journey 16174→7084 allocs. Small update 154→66 allocs. ~3x faster update latency.

- [x] **Replace Dynamics map with []interface{} slice** *(Completed 2026-03-21, PR #220)*
  - Location: `internal/build/types.go`, Phase 2 (Build)
  - Approach: Replaced `map[string]interface{}` with `[]interface{}` for Dynamics. Index-based access via `PositionKey` cached string table. Updated all consumers (diff, build, send).
  - Actual Impact: Eliminated map hash/bucket allocations. BuildTree/simple 27→14 allocs. CompareTreesLargeChange/100 12→2 allocs. Small update 66→46 allocs.

- [x] **Shared Statics, Buffer Pool, Reflection Dedup** *(Completed 2026-03-21, PR #224)*
  - Location: `internal/build/`, `internal/context/`, Phase 2 (Build)
  - Approach: (1) Shared sentinel empty `[]string{}` statics across TreeNodes. (2) `sync.Pool` for `bytes.Buffer` in JSON marshaling and HTML rendering. (3) `PositionKey` cached string table avoids repeated `strconv.Itoa`. (4) Deduplicated reflection lookups for controller/state dispatch.
  - Actual Impact: Subsequent render 66→61 allocs, 7.5KB→3KB. E2E user journey 7084→5083 allocs.

- [ ] **Implement TreeNode Struct Pooling**
  - Location: `internal/build/types.go`, Phase 2 (Build)
  - Goal: Reduce 9.5 GB allocations from TreeNode creation (NewTreeNode 12.09%, NewTreeNodeWithStatics 9.18%)
  - Approach: Use `sync.Pool` for TreeNode structs, reset and reuse instead of allocating new
  - Expected Impact: 20% reduction in total allocations, 15-20% reduction in Build phase time
  - Verification: Run `BenchmarkTreeNodeCreation` and `BenchmarkBuildTree` to confirm

### Medium Priority Tasks

- [x] **Replace MD5 Fingerprinting with FNV-1a** *(Completed 2026-03-18, commit 1e351ca)*
  - Location: `internal/build/fingerprint.go`, Phase 2 (Build)
  - Approach: Replaced `crypto/md5` with `hash/fnv` (FNV-1a 128-bit truncated to 64 bits). Thread-safe atomic.Value caching.
  - Actual Impact: 43% faster small trees, 44% medium, 47% large, 17% deep-nested. Fingerprinting dropped from 5.93% CPU to <1%.

- [x] **Implement Fingerprint Caching** *(Completed — lazy-cached on TreeNode)*
  - Location: `internal/build/types.go:GetStructureFingerprint()`, Phase 2 (Build)
  - Approach: Fingerprints are lazy-computed on first access and cached on the TreeNode itself
  - Impact: O(1) structure comparison after first access; no separate cache data structure needed

- [ ] **Evaluate Faster JSON Library**
  - Location: `internal/send/message.go`, Phase 5 (Send)
  - Goal: Reduce JSON marshaling allocations
  - Approach: Benchmark `encoding/json` vs `github.com/json-iterator/go` vs `github.com/goccy/go-json`
  - Expected Impact: 10-20% reduction in Send phase allocations and time
  - Verification: Run `BenchmarkSendMessage` and compare marshal performance

- [x] **Pre-compute Evaluator Builtins** *(Completed 2026-03-21, PR #219)*
  - Location: `internal/parse/eval.go`, `internal/parse/api.go`, Phase 1 (Parse)
  - Approach: `precomputeBuiltins()` merges cachedBuiltins + user FuncMap once at parse time, stored on `parse.Template`. Eliminates per-render map allocation in `newEvaluator`.
  - Actual Impact: Eliminated 4.8 GB (5.90%) of allocations. BuildTree/simple 27→22 allocs.

- [x] **Optimize TreeNode Map Allocations** *(Completed 2026-03-21, PR #220)*
  - Location: `internal/build/types.go`, Phase 2 (Build)
  - Approach: Replaced Dynamics map with `[]interface{}` slice — eliminates map allocation entirely
  - Actual Impact: map replaced by slice, SetDynamic dropped from 6.13% to 1.56% of allocations

- [ ] **Implement Custom Binary Format (Optional)**
  - Location: `internal/send/`, Phase 5 (Send)
  - Goal: Alternative to JSON for internal communication (non-wire format)
  - Approach: Design compact binary format using `encoding/binary` or Protocol Buffers
  - Expected Impact: 30-50% reduction in Send phase allocations (internal only)
  - Verification: Add `BenchmarkSendBinary` and compare with JSON

### Low Priority Tasks

- [ ] **Optimize Diff Algorithm for Deep Trees**
  - Location: `internal/diff/tree_compare.go`, Phase 3 (Diff)
  - Goal: Reduce allocations in deeply nested tree comparisons
  - Approach: Use iterative traversal instead of recursive, reuse comparison buffers
  - Expected Impact: <1% reduction in total allocations
  - Verification: Run `BenchmarkCompareTrees` with deeply nested structures

- [x] **Reduce Range Diff Allocations** *(Completed 2026-03-19, commit b9faf28)*
  - Location: `internal/diff/range_ops.go`, Phase 3 (Diff)
  - Approach: `rangeContext` pre-computes key maps and positions once per range diff. DeepEqual fast paths for string, int, float64, bool. Package-level compiled regex for position field detection.
  - Actual Impact: 59% faster (5332→2205 ns/op), 54% fewer allocs (37→17). E2E range operations improved 5-6x.

- [ ] **Reduce HTML Parsing Fallback Frequency**
  - Location: `internal/parse/parser.go`, Phase 1 (Parse)
  - Goal: Further reduce 1.4 GB allocations in `html.NewTokenizerFragment` fallback (3.05%, down from 29.52% after PR #219)
  - Approach: Improve template construct coverage to avoid fallback to HTML parsing
  - Expected Impact: 2-3% reduction if fallback usage decreases significantly
  - Verification: Add logging to track fallback frequency, aim for <5% of templates

### Monitoring & Validation Tasks

- [ ] **Establish Performance Regression Tests**
  - Location: `.github/workflows/benchmark.yml`
  - Goal: Automatically detect performance regressions in CI
  - Approach: Already implemented, but add stricter thresholds (>5% warning, >10% failure)
  - Expected Impact: Prevents performance degradation over time
  - Verification: PR builds show benchmark comparison results

- [ ] **Add Allocation Budget Tests**
  - Location: `*_test.go` files
  - Goal: Set hard limits on allocations per operation
  - Approach: Use `testing.AllocsPerRun()` to enforce allocation budgets
  - Expected Impact: Catches allocation regressions at test time
  - Verification: Tests fail if allocations exceed defined budgets

- [ ] **Profile Production Workloads**
  - Location: Production environment
  - Goal: Validate that benchmark bottlenecks match real-world usage
  - Approach: Enable pprof HTTP endpoints, collect profiles from live traffic
  - Expected Impact: Discover real-world bottlenecks not visible in benchmarks
  - Verification: Compare production profiles with benchmark profiles

## Task Tracking

Update this section as tasks are completed:

**Last Updated:** 2026-03-22
**Completed Tasks:** 8/15
**In Progress:** 0
**Blocked:** 0

When completing a task:
1. Mark checkbox with `[x]`
2. Add completion date and PR link
3. Update benchmarks and profiles
4. Document actual vs expected impact

## Phase-Specific Analysis

### Phase 1: Parse
- **Primary Bottleneck:** AST walking and ordered variable management
- **Allocations:** walkAST 1.90%, walkList 1.39%, newOrderedVars 1.17%, precomputeBuiltins 1.15%
- **Status:** `newEvaluator` eliminated as allocator (was 5.90%), builtins pre-computed once at parse time (PR #219)
- **Recommendation:** Pool ordered variable maps, reduce AST walk allocations

### Phase 2: Build
- **Primary Bottleneck:** TreeNode struct allocations (dominant hotspot)
- **Allocations:** ~9.5 GB (22.4% of total) — NewTreeNode 12.09%, NewTreeNodeWithStatics 9.18%, SetDynamic 1.56%
- **Status:** Dynamics map→slice (PR #220) eliminated map overhead. SetDynamic dropped from 6.13% to 1.56%.
- **CPU Time:** Fingerprinting <1% (FNV-1a). hashStructureWithCircularDetection 1.27%.
- **Recommendation:** `sync.Pool` for TreeNode structs — the single highest-impact remaining optimization

### Phase 3: Diff
- **Primary Bottleneck:** Tree comparison and range construct matching
- **Allocations:** ~1.8 GB (4.1% of total) — CompareTreesAndGetChangesWithPath 2.33%, FindRangeConstructMatches 1.77%
- **Status:** Range diff operations 59% faster after `rangeContext` (commit b9faf28). Pass-through result map in findRangeConstructsRecursive (PR #224).
- **Recommendation:** Optimize comparison algorithms for deeply nested trees

### Phase 4: Render
- **Primary Bottleneck:** HTML parsing for fallback cases (reduced)
- **Allocations:** html.NewTokenizerFragment 3.05% (was 29.52% — PR #219 eliminated redundant calls)
- **Recommendation:** Continue reducing fallback to HTML parsing

### Phase 5: Send
- **Primary Bottleneck:** JSON marshaling and decoding
- **Allocations:** MarshalJSON 1.94%, JSON decoding 2.79%, mapEncoder 1.49%
- **Recommendation:** Consider faster JSON library or custom binary format

## Regenerating Profiles

To update this analysis after code changes:

```bash
make profile-all
go tool pprof -http=:8080 profiles/cpu.prof   # Interactive analysis
go tool pprof -http=:8080 profiles/mem.prof
```

Look for:
- Hot paths in CPU profile (cumulative % column)
- High allocation counts in memory profile
- Lock contention in concurrent benchmarks

### Quick Profile Commands

```bash
# Top CPU consumers
go tool pprof -top -cum profiles/cpu.prof | head -20

# Top memory allocators
go tool pprof -top -alloc_space profiles/mem.prof | head -20

# Filter for LiveTemplate functions
go tool pprof -top -alloc_space profiles/mem.prof | grep livetemplate
```
