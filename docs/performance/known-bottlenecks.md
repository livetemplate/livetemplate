# Known Performance Bottlenecks

**Last Profiled:** 2025-11-10
**Go Version (profiles):** 1.25.3
**Go Version (benchmarks):** 1.26.0
**Architecture:** arm64 (Apple M2)

> **Note:** The CPU and memory profiles (pprof output) below are from the original 2025-11-10 session
> on Go 1.25.3. Function names have been updated to reflect the current codebase.
> Benchmark numbers throughout have been updated to Go 1.26.0 results.

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
Time: 2025-11-10 09:35:50 CET
Duration: 66.83s, Total samples = 118.61s (177.48%)
Showing nodes accounting for 102s, 86.00% of 118.61s total
Dropped 923 nodes (cum <= 0.59s)
      flat  flat%   sum%        cum   cum%
     0.03s 0.025% 0.025%     73.09s 61.62%  runtime.systemstack
     0.04s 0.034% 0.059%     50.96s 42.96%  runtime.gcBgMarkWorker.func2
     2.14s  1.80%  1.86%     50.82s 42.85%  runtime.gcDrain
         0     0%  1.86%     37.49s 31.61%  runtime.gcDrainMarkWorkerIdle (inline)
     0.30s  0.25%  2.12%     26.84s 22.63%  runtime.gcBgMarkWorker
     0.02s 0.017%  2.13%     23.51s 19.82%  runtime.preemptM
    23.42s 19.75% 21.88%     23.42s 19.75%  runtime.pthread_kill
         0     0% 21.88%     23.42s 19.75%  runtime.signalM (inline)
     0.07s 0.059% 21.94%     22.53s 19.00%  runtime.schedule
     0.02s 0.017% 21.95%     22.12s 18.65%  runtime.mcall
     0.37s  0.31% 22.27%     21.94s 18.50%  runtime.park_m
     0.16s  0.13% 22.40%     21.64s 18.24%  runtime.(*gcWork).balance
     0.01s 0.0084% 22.41%     21.11s 17.80%  runtime.preemptone
```

### Key Findings

#### Runtime/GC Overhead
- **Impact:** ~62% of CPU time in garbage collection
- **Analysis:** The benchmark suite allocates heavily, triggering frequent GC cycles
- **Optimization Opportunity:** Reduce allocations (see Memory Bottlenecks section)

#### Fingerprinting (Phase 2: Build)
- **Location:** `internal/build/fingerprint.go:CalculateStructureFingerprint`
- **Impact:** 5.93% of CPU time (cumulative)
- **Optimization Opportunity:** Fingerprints are now lazy-cached on TreeNode via `GetStructureFingerprint()`. Consider faster hashing algorithms (xxhash) for further improvement.

#### Overall Distribution
Most CPU time is spent in:
1. Garbage collection (62%)
2. Runtime overhead (scheduling, memory management)
3. Application logic (fingerprinting, template rendering, tree diffing)

The high GC overhead indicates that memory allocation reduction would have the most significant performance impact.

## Memory Bottlenecks

### Analysis Summary

```
File: livetemplate.test
Type: alloc_space
Time: 2025-11-10 09:38:31 CET
Showing nodes accounting for 53296.42MB, 89.55% of 59518.77MB total
Dropped 593 nodes (cum <= 297.59MB)
      flat  flat%   sum%        cum   cum%
11250.34MB 18.90% 18.90% 11250.34MB 18.90%  golang.org/x/net/html.NewTokenizerFragment
 5365.74MB  9.02% 27.92%  5365.74MB  9.02%  text/template.addValueFuncs
 4353.49MB  7.31% 35.23%  4353.49MB  7.31%  text/template.addFuncs (inline)
 2891.70MB  4.86% 40.09%  5108.28MB  8.58%  encoding/json.mapEncoder.encode
 2397.04MB  4.03% 44.12%  2397.04MB  4.03%  reflect.copyVal
 1781.82MB  2.99% 47.11%  1781.82MB  2.99%  text/template.builtins (inline)
 1653.45MB  2.78% 49.89%  6767.23MB 11.37%  encoding/json.Marshal
 1594.57MB  2.68% 52.57%  1594.57MB  2.68%  html/template.makeEscaper (inline)
 1539.38MB  2.59% 55.15%  2165.66MB  3.64%  encoding/json.(*decodeState).objectInterface
 1522.66MB  2.56% 57.71%  1701.17MB  2.86%  golang.org/x/net/html.(*parser).addElement (inline)
 1471.99MB  2.47% 60.19%  4912.33MB  8.25%  html/template.(*escaper).escapeTemplateBody
 1459.98MB  2.45% 62.64%  1459.98MB  2.45%  maps.Copy (html/template context maps)
 1317.86MB  2.21% 64.85%  1317.86MB  2.21%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).SetDynamic
 1167.32MB  1.96% 66.81%  1167.32MB  1.96%  html/template.(*escaper).editActionNode
```

### LiveTemplate-Specific Allocations

```
 1317.86MB  2.21%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).SetDynamic
  664.64MB  1.12%  github.com/livetemplate/livetemplate/internal/context.ExecuteTemplateWithContext
  613.53MB  1.03%  github.com/livetemplate/livetemplate/internal/build.NewTreeNode
  473.03MB  0.79%  github.com/livetemplate/livetemplate/internal/build.NewTreeNodeWithStatics
  444.04MB  0.75%  github.com/livetemplate/livetemplate/internal/build.CalculateStructureFingerprint
  440.61MB  0.74%  github.com/livetemplate/livetemplate/internal/context.AddLvtToData
  233.57MB  0.39%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).UnmarshalJSON
  198.02MB  0.33%  github.com/livetemplate/livetemplate.(*Template).renderHTML
  162.51MB  0.27%  github.com/livetemplate/livetemplate/internal/diff.CompareTreesAndGetChangesWithPath
  149.52MB  0.25%  github.com/livetemplate/livetemplate/internal/diff.FindRangeConstructMatches
```

### Allocations per Operation

**Initial Render (includes template parsing, one-time cost):**
- Total allocations: ~3,979 allocs/op
- Bytes allocated: ~429 KB/op
- Example: BenchmarkTemplateExecute/initial-render-8 (1768635 ns/op, 429488 B/op, 3979 allocs/op)

**Subsequent Render (per-session, reuses parsed template):**
- Total allocations: ~245 allocs/op
- Bytes allocated: ~29 KB/op
- Example: BenchmarkTemplateExecute/subsequent-render-8 (41262 ns/op, 29304 B/op, 245 allocs/op)

**Small Update:**
- Total allocations: ~229 allocs/op
- Bytes allocated: ~28 KB/op
- Example: BenchmarkTemplateExecuteUpdates/small-update-8 (38554 ns/op, 28320 B/op, 229 allocs/op)

**Large Update:**
- Total allocations: ~752 allocs/op
- Bytes allocated: ~79 KB/op
- Example: BenchmarkTemplateExecuteUpdates/large-update-8 (233551 ns/op, 78625 B/op, 752 allocs/op)

**Range Operations:**
- Add items: 844 allocs/op, 85 KB/op
- Remove items: 456 allocs/op, 46 KB/op
- Reorder items: 588 allocs/op, 60 KB/op
- Update items: 588 allocs/op, 60 KB/op

**Complex User Journey (E2E):**
- Total allocations: ~23,652 allocs/op
- Bytes allocated: ~2.9 MB/op
- Example: BenchmarkE2EUserJourney-8 (4250095 ns/op, 2919203 B/op, 23652 allocs/op)

### Cache Memory Usage

**Parse Caches:**
- Template parsing involves significant allocations in text/template and html/template
- addValueFuncs: 5.4 GB (9.02% of total)
- addFuncs: 4.4 GB (7.31% of total)
- Template escaper operations: ~5 GB (8.25% cumulative)

**Fingerprint Cache:**
- Lazy-computed per TreeNode via `GetStructureFingerprint()`
- MD5 hash truncated to 64 bits (16 hex chars)
- No eviction needed — cached on the tree node itself, lifetime tied to node
- Replaced the earlier `ClientStructureRegistry` (LRU, max 1000 entries), removed in PR #86

## Optimization Priorities

Based on profiling data, prioritize:

1. **[High] Reduce Template Parsing Allocations**
   - Current: 18.9 GB in html.NewTokenizerFragment (18.90% of allocations)
   - Current: 12.7 GB in text/template functions (16.33% of allocations)
   - Potential improvement: Implement template caching, reduce parsing frequency
   - Impact: Would reduce GC pressure significantly (62% CPU time)

2. **[High] Optimize TreeNode Operations (Phase 2: Build)**
   - Current: 1.3 GB in SetDynamic (2.21% of allocations)
   - Current: 613 MB in NewTreeNode (1.03% of allocations)
   - Current: 473 MB in NewTreeNodeWithStatics (0.79% of allocations)
   - Potential improvement: Object pooling, reduce map allocations
   - Impact: ~4% reduction in total allocations

3. **[Medium] JSON Serialization (Phase 5: Send)**
   - Current: 6.8 GB in JSON marshaling (11.37% cumulative)
   - Potential improvement: Use faster JSON library or custom serialization
   - Impact: 11% of allocations, but necessary for wire format

4. **[Medium] Fingerprinting Optimization (Phase 2: Build)**
   - Current: 444 MB allocations + 5.93% CPU time
   - Potential improvement: Use faster hash algorithm (xxhash), cache results
   - Impact: 6% CPU reduction

5. **[Low] Diff Operations (Phase 3: Diff)**
   - Current: 162 MB in CompareTreesAndGetChangesWithPath (0.27%)
   - Current: 149 MB in FindRangeConstructMatches (0.25%)
   - Potential improvement: Algorithmic optimizations
   - Impact: <1% of total allocations

## Optimization Task List

### High Priority Tasks

- [ ] **Implement Template Parse Caching**
  - Location: `template.go`, Phase 1 (Parse)
  - Goal: Reduce 18.9 GB allocations in `html.NewTokenizerFragment` and 12.7 GB in text/template functions
  - Approach: Cache parsed `text/template.Template` and `html/template.Template` objects by template string hash
  - Expected Impact: 50% reduction in allocations, 30-40% reduction in CPU time (less GC)
  - Verification: Run `make bench-compare` and `make profile-mem` to confirm reduction

- [ ] **Reduce Template Re-parsing Frequency**
  - Location: `template.go`, Phase 1 (Parse)
  - Goal: Eliminate redundant template parsing in benchmarks and production
  - Approach: Add `isParsed` flag to Template struct, skip parsing if already done
  - Expected Impact: 15-20% reduction in Parse phase allocations
  - Verification: Check `BenchmarkParse` results show improved performance

- [ ] **Implement TreeNode Object Pooling**
  - Location: `internal/build/tree_ops.go`, Phase 2 (Build)
  - Goal: Reduce 2.4 GB allocations from TreeNode creation (SetDynamic, NewTreeNode, NewTreeNodeWithStatics)
  - Approach: Use `sync.Pool` for TreeNode objects, reset and reuse instead of allocating new
  - Expected Impact: 4% reduction in total allocations, 5-10% reduction in Build phase time
  - Verification: Run `BenchmarkTreeNodeCreation` and `BenchmarkBuildTree` to confirm

- [ ] **Optimize TreeNode Map Allocations**
  - Location: `internal/build/types.go`, Phase 2 (Build)
  - Goal: Reduce map allocations in TreeNode.Dynamics and TreeNode.Statics
  - Approach: Pre-allocate maps with estimated capacity based on template complexity
  - Expected Impact: 1-2% reduction in allocations
  - Verification: Profile memory and check `SetDynamic` allocations decrease

### Medium Priority Tasks

- [ ] **Replace MD5 Fingerprinting with xxhash**
  - Location: `internal/build/fingerprint.go`, Phase 2 (Build)
  - Goal: Reduce 5.93% CPU time in `CalculateStructureFingerprint`
  - Approach: Replace `crypto/md5` with `github.com/cespare/xxhash/v2` for non-cryptographic hashing
  - Expected Impact: 5-6% CPU reduction in fingerprinting operations
  - Verification: Run `BenchmarkCalculateStructureFingerprint_*` and verify performance improvement

- [x] **Implement Fingerprint Caching** *(Completed — lazy-cached on TreeNode)*
  - Location: `internal/build/types.go:GetStructureFingerprint()`, Phase 2 (Build)
  - Approach: Fingerprints are lazy-computed on first access and cached on the TreeNode itself
  - Impact: O(1) structure comparison after first access; no separate cache data structure needed

- [ ] **Evaluate Faster JSON Library**
  - Location: `internal/send/message.go`, Phase 5 (Send)
  - Goal: Reduce 6.8 GB allocations in JSON marshaling (11.37% of total)
  - Approach: Benchmark `encoding/json` vs `github.com/json-iterator/go` vs `github.com/goccy/go-json`
  - Expected Impact: 10-20% reduction in Send phase allocations and time
  - Verification: Run `BenchmarkSendMessage` and compare marshal performance

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

- [ ] **Reduce Range Match Allocations**
  - Location: `internal/diff/range_ops.go`, Phase 3 (Diff)
  - Goal: Optimize `FindRangeConstructMatches` allocations (149 MB)
  - Approach: Pre-allocate match slices, use object pooling for match results
  - Expected Impact: <1% reduction in total allocations
  - Verification: Run `BenchmarkRangeOperations` and check Diff phase performance

- [ ] **Reduce HTML Parsing Fallback Frequency**
  - Location: `internal/parse/parser.go`, Phase 1 (Parse)
  - Goal: Decrease 11.3 GB allocations in `html.NewTokenizerFragment` fallback
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

**Last Updated:** 2026-03-10
**Completed Tasks:** 1/14
**In Progress:** 0
**Blocked:** 0

When completing a task:
1. Mark checkbox with `[x]`
2. Add completion date and PR link
3. Update benchmarks and profiles
4. Document actual vs expected impact

## Phase-Specific Analysis

### Phase 1: Parse
- **Primary Bottleneck:** Template parsing via text/template and html/template
- **Allocations:** ~30 GB (50% of total)
- **Recommendation:** Cache parsed templates aggressively, reduce re-parsing

### Phase 2: Build
- **Primary Bottleneck:** TreeNode allocations and fingerprinting
- **Allocations:** ~2.8 GB (4.7% of total)
- **CPU Time:** 5.93% (fingerprinting)
- **Recommendation:** Object pooling for TreeNodes, optimize fingerprint caching

### Phase 3: Diff
- **Primary Bottleneck:** Tree comparison with large structures
- **Allocations:** ~312 MB (0.52% of total)
- **Recommendation:** Optimize comparison algorithms for deeply nested trees

### Phase 4: Render
- **Primary Bottleneck:** HTML parsing for fallback cases
- **Allocations:** 11.3 GB in html.NewTokenizerFragment (18.90%)
- **Recommendation:** Reduce fallback to HTML parsing, improve template construct coverage

### Phase 5: Send
- **Primary Bottleneck:** JSON marshaling
- **Allocations:** 6.8 GB (11.37% of total)
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
