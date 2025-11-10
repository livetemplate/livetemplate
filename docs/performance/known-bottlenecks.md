# Known Performance Bottlenecks

**Last Profiled:** 2025-11-10
**Go Version:** 1.25.3
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
- **Location:** `github.com/livetemplate/livetemplate.calculateFingerprintOld`
- **Impact:** 5.93% of CPU time (cumulative)
- **Optimization Opportunity:** Consider faster hashing algorithms or caching fingerprints

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
  444.04MB  0.75%  github.com/livetemplate/livetemplate.calculateFingerprintOld
  440.61MB  0.74%  github.com/livetemplate/livetemplate/internal/context.AddLvtToData
  233.57MB  0.39%  github.com/livetemplate/livetemplate/internal/build.(*TreeNode).UnmarshalJSON
  198.02MB  0.33%  github.com/livetemplate/livetemplate.(*Template).renderHTML
  162.51MB  0.27%  github.com/livetemplate/livetemplate/internal/diff.CompareTreesAndGetChangesWithPath
  149.52MB  0.25%  github.com/livetemplate/livetemplate/internal/diff.FindRangeConstructMatches
```

### Allocations per Operation

**Initial Render (simple template):**
- Total allocations: ~237 allocs/op
- Bytes allocated: ~29 KB/op
- Example: BenchmarkTemplateConcurrent/1-8 (22710 ns/op, 28999 B/op, 237 allocs/op)

**Small Update:**
- Total allocations: ~221 allocs/op
- Bytes allocated: ~28 KB/op
- Example: BenchmarkTemplateExecuteUpdates/small-update-8 (17150 ns/op, 28041 B/op, 221 allocs/op)

**Large Update:**
- Total allocations: ~728 allocs/op
- Bytes allocated: ~78 KB/op
- Example: BenchmarkTemplateExecuteUpdates/large-update-8 (59740 ns/op, 78055 B/op, 728 allocs/op)

**Range Operations:**
- Add items: 756 allocs/op, 82 KB/op
- Remove items: 405 allocs/op, 44 KB/op
- Reorder items: 525 allocs/op, 58 KB/op
- Update items: 525 allocs/op, 58 KB/op

**Complex User Journey (E2E):**
- Total allocations: ~22,880 allocs/op
- Bytes allocated: ~2.9 MB/op
- Example: BenchmarkE2EUserJourney-8 (1664558 ns/op, 2895522 B/op, 22874 allocs/op)

### Cache Memory Usage

**Parse Caches:**
- Template parsing involves significant allocations in text/template and html/template
- addValueFuncs: 5.4 GB (9.02% of total)
- addFuncs: 4.4 GB (7.31% of total)
- Template escaper operations: ~5 GB (8.25% cumulative)

**Structure Registry:**
- Max entries: 1000 (LRU eviction)
- Estimated overhead: Minimal compared to template parsing overhead

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
