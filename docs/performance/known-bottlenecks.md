# Known Performance Bottlenecks

**Last Profiled:** 2026-03-19
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

> **Note:** Total allocation volume is 80.8 GB vs 59.5 GB in the previous session. This reflects
> more benchmark iterations (faster per-op code → higher `b.N`), not a regression. Per-operation
> allocation counts (B/op, allocs/op) decreased across the board — see Allocations per Operation below.

### Analysis Summary

```
File: livetemplate.test
Type: alloc_space
Time: 2026-03-19 20:54:41 CET
Showing nodes accounting for 71049.58MB, 87.91% of 80820.80MB total
Dropped 701 nodes (cum <= 404.10MB)
      flat  flat%   sum%        cum   cum%
23859.12MB 29.52% 29.52% 23859.12MB 29.52%  golang.org/x/net/html.NewTokenizerFragment
 4953.86MB  6.13% 35.65%  4953.86MB  6.13%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).SetDynamic
 4768.48MB  5.90% 41.55%  4768.48MB  5.90%  github.com/livetemplate/livetemplate/internal/parse.newEvaluator
 4056.65MB  5.02% 46.57%  4056.65MB  5.02%  text/template.builtins (inline)
 3304.85MB  4.09% 50.66%  3680.86MB  4.55%  golang.org/x/net/html.(*parser).addElement (inline)
 2512.16MB  3.11% 53.77%  2512.16MB  3.11%  github.com/livetemplate/livetemplate/internal/build.NewTreeNode (partial-inline)
 2189.64MB  2.71% 56.48%  2189.64MB  2.71%  github.com/livetemplate/livetemplate/internal/build.NewTreeNodeWithStatics
 1734.29MB  2.15% 58.62% 31051.92MB 38.42%  golang.org/x/net/html.ParseWithOptions
 1441.32MB  1.78% 60.41%  4188.02MB  5.18%  github.com/livetemplate/livetemplate/internal/context.ExecuteTemplateWithContext
 1340.52MB  1.66% 62.06%  1340.52MB  1.66%  reflect.unsafe_New
 1288.33MB  1.59% 63.66%  1700.85MB  2.10%  encoding/json.(*decodeState).objectInterface
 1273.17MB  1.58% 65.23%  3129.77MB  3.87%  html/template.New
 1021.68MB  1.26% 66.50%  1021.68MB  1.26%  bytes.growSlice
  958.60MB  1.19% 67.68%   958.60MB  1.19%  golang.org/x/net/html.(*parser).addText
  952.26MB  1.18% 68.86%   952.26MB  1.18%  text/template/parse.New (inline)
  947.72MB  1.17% 70.03%  1197.73MB  1.48%  github.com/livetemplate/livetemplate/internal/context.AddLvtToData
  912.04MB  1.13% 71.16%   912.04MB  1.13%  html/template.makeEscaper (inline)
  864.67MB  1.07% 72.23%   864.67MB  1.07%  text/template/parse.(*Tree).newText (inline)
  814.15MB  1.01% 73.24%   814.15MB  1.01%  github.com/livetemplate/livetemplate/internal/diff.findRangeConstructsRecursive
```

### LiveTemplate-Specific Allocations

```
 4953.86MB  6.13%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).SetDynamic
 4768.48MB  5.90%  github.com/livetemplate/livetemplate/internal/parse.newEvaluator
 2512.16MB  3.11%  github.com/livetemplate/livetemplate/internal/build.NewTreeNode
 2189.64MB  2.71%  github.com/livetemplate/livetemplate/internal/build.NewTreeNodeWithStatics
 1441.32MB  1.78%  github.com/livetemplate/livetemplate/internal/context.ExecuteTemplateWithContext
  947.72MB  1.17%  github.com/livetemplate/livetemplate/internal/context.AddLvtToData
  814.15MB  1.01%  github.com/livetemplate/livetemplate/internal/diff.findRangeConstructsRecursive
  663.52MB  0.82%  github.com/livetemplate/livetemplate/internal/parse.getSortedKeys
  557.51MB  0.69%  github.com/livetemplate/livetemplate/internal/build.CalculateStructureFingerprint
  525.51MB  0.65%  github.com/livetemplate/livetemplate/internal/parse.walkAST
  423.04MB  0.52%  github.com/livetemplate/livetemplate.(*Template).renderHTML
  402.53MB  0.50%  github.com/livetemplate/livetemplate/internal/diff.CompareTreesAndGetChangesWithPath
  377.57MB  0.47%  github.com/livetemplate/livetemplate/internal/diff.FindRangeConstructMatches
```

### Allocations per Operation

**Initial Render (includes template parsing, one-time cost):**
- Total allocations: ~3,954 allocs/op
- Bytes allocated: ~417 KB/op
- Example: BenchmarkTemplateExecute/initial-render-8 (1406553 ns/op, 426485 B/op, 3954 allocs/op)

**Subsequent Render (per-session, reuses parsed template):**
- Total allocations: ~170 allocs/op
- Bytes allocated: ~20 KB/op
- Example: BenchmarkTemplateExecute/subsequent-render-8 (10572 ns/op, 20818 B/op, 170 allocs/op)

**Small Update:**
- Total allocations: ~154 allocs/op
- Bytes allocated: ~19 KB/op
- Example: BenchmarkTemplateExecuteUpdates/small-update-8 (9579 ns/op, 19840 B/op, 154 allocs/op)

**Large Update:**
- Total allocations: ~357 allocs/op
- Bytes allocated: ~30 KB/op
- Example: BenchmarkTemplateExecuteUpdates/large-update-8 (22937 ns/op, 30390 B/op, 357 allocs/op)

**Range Operations:**
- Add items: 406 allocs/op, 32 KB/op
- Remove items: 268 allocs/op, 25 KB/op
- Reorder items: 315 allocs/op, 28 KB/op
- Update items: 315 allocs/op, 28 KB/op

**Complex User Journey (E2E):**
- Total allocations: ~16,174 allocs/op
- Bytes allocated: ~2.0 MB/op
- Example: BenchmarkE2EUserJourney-8 (1083172 ns/op, 2073551 B/op, 16174 allocs/op)

### Cache Memory Usage

**Parse Caches:**
- Template parsing involves significant allocations in text/template and html/template
- builtins: 4.1 GB (5.02% of total)
- Template escaper operations: ~3.1 GB (3.87% cumulative)
- newEvaluator (custom AST evaluator): 4.8 GB (5.90% of total)

**Fingerprint Cache:**
- Lazy-computed per TreeNode via `GetStructureFingerprint()`
- FNV-1a 128-bit hash truncated to 64 bits (16 hex chars)
- No eviction needed — cached on the tree node itself, lifetime tied to node
- Replaced the earlier `ClientStructureRegistry` (LRU, max 1000 entries), removed in PR #86

## Optimization Priorities

Based on profiling data, prioritize:

1. **[High] Reduce Template Parsing Allocations**
   - Current: 23.9 GB in html.NewTokenizerFragment (29.52% of allocations)
   - Current: 4.1 GB in text/template.builtins (5.02% of allocations)
   - Potential improvement: Implement template caching, reduce parsing frequency
   - Impact: Would reduce GC pressure significantly (59% CPU time)

2. **[High] Optimize TreeNode Operations (Phase 2: Build)**
   - Current: 5.0 GB in SetDynamic (6.13% of allocations)
   - Current: 2.5 GB in NewTreeNode (3.11% of allocations)
   - Current: 2.2 GB in NewTreeNodeWithStatics (2.71% of allocations)
   - Potential improvement: Object pooling, reduce map allocations
   - Impact: ~12% reduction in total allocations

3. **[Medium] Parse Evaluator Allocations (Phase 1: Parse)**
   - Current: 4.8 GB in newEvaluator (5.90% of total)
   - Current: 664 MB in getSortedKeys (0.82% of total)
   - Potential improvement: Pool evaluators, cache sorted key slices
   - Impact: ~7% reduction in total allocations

4. **[Medium] JSON Serialization (Phase 5: Send)**
   - Current: 1.7 GB in JSON decoding (2.10% cumulative)
   - Potential improvement: Use faster JSON library or custom serialization
   - Impact: Reduced from 11.37% in Nov 2025 — lower priority now

5. **[Low] Diff Operations (Phase 3: Diff)**
   - Current: 814 MB in findRangeConstructsRecursive (1.01%)
   - Current: 403 MB in CompareTreesAndGetChangesWithPath (0.50%)
   - Current: 378 MB in FindRangeConstructMatches (0.47%)
   - Potential improvement: Algorithmic optimizations
   - Impact: ~2% of total allocations

## Optimization Task List

### High Priority Tasks

- [x] **Eliminate Redundant HTML Parsing + Cache Template AST** *(Completed 2026-03-21, PR #219)*
  - Location: `template.go`, `internal/parse/api.go`, `internal/compat/tree.go`
  - Approach: (1) Eliminated redundant `ExtractTemplateContent` calls on main update path — html.Parse() now only runs on first render and rare fallback. (2) Cached `*parse.Template` AST after first parse, reused on subsequent renders. (3) Pre-computed evaluator builtins map once at parse time.
  - Actual Impact: 50-57% allocation reduction per render. E2E user journey 16174→7084 allocs. Small update 154→66 allocs. ~3x faster update latency.

- [ ] **Implement TreeNode Object Pooling**
  - Location: `internal/build/tree_ops.go`, Phase 2 (Build)
  - Goal: Reduce 9.7 GB allocations from TreeNode creation (SetDynamic, NewTreeNode, NewTreeNodeWithStatics)
  - Approach: Use `sync.Pool` for TreeNode objects, reset and reuse instead of allocating new
  - Expected Impact: 12% reduction in total allocations, 10-15% reduction in Build phase time
  - Verification: Run `BenchmarkTreeNodeCreation` and `BenchmarkBuildTree` to confirm

- [ ] **Optimize TreeNode Map Allocations**
  - Location: `internal/build/types.go`, Phase 2 (Build)
  - Goal: Reduce map allocations in TreeNode.Dynamics and TreeNode.Statics
  - Approach: Pre-allocate maps with estimated capacity based on template complexity
  - Expected Impact: 1-2% reduction in allocations
  - Verification: Profile memory and check `SetDynamic` allocations decrease

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
  - Goal: Decrease 23.9 GB allocations in `html.NewTokenizerFragment` fallback
  - Approach: Improve template construct coverage to avoid fallback to HTML parsing
  - Expected Impact: 10-15% reduction if fallback usage decreases significantly
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

**Last Updated:** 2026-03-21
**Completed Tasks:** 5/15
**In Progress:** 0
**Blocked:** 0

When completing a task:
1. Mark checkbox with `[x]`
2. Add completion date and PR link
3. Update benchmarks and profiles
4. Document actual vs expected impact

## Phase-Specific Analysis

### Phase 1: Parse
- **Primary Bottleneck:** Template parsing via text/template, html/template, and AST evaluator
- **Allocations:** ~33 GB (41% of total) — html.NewTokenizerFragment 29.52%, newEvaluator 5.90%, builtins 5.02%
- **Recommendation:** Cache parsed templates aggressively, pool evaluators, reduce re-parsing

### Phase 2: Build
- **Primary Bottleneck:** TreeNode allocations
- **Allocations:** ~9.7 GB (12.0% of total) — SetDynamic 6.13%, NewTreeNode 3.11%, NewTreeNodeWithStatics 2.71%
- **CPU Time:** Fingerprinting now <1% (FNV-1a, previously 5.93% with MD5)
- **Recommendation:** Object pooling for TreeNodes, pre-allocate maps

### Phase 3: Diff
- **Primary Bottleneck:** Range construct discovery and tree comparison
- **Allocations:** ~1.6 GB (2.0% of total) — findRangeConstructsRecursive 1.01%, CompareTreesAndGetChangesWithPath 0.50%, FindRangeConstructMatches 0.47%
- **Status:** Range diff operations 59% faster after `rangeContext` optimization (commit b9faf28)
- **Recommendation:** Optimize comparison algorithms for deeply nested trees

### Phase 4: Render
- **Primary Bottleneck:** HTML parsing for fallback cases
- **Allocations:** 23.9 GB in html.NewTokenizerFragment (29.52%)
- **Recommendation:** Reduce fallback to HTML parsing, improve template construct coverage

### Phase 5: Send
- **Primary Bottleneck:** JSON marshaling and decoding
- **Allocations:** ~1.7 GB in JSON decoding (2.10% cumulative)
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
