# Performance Benchmarking System Design

**Date:** 2025-11-10
**Status:** Approved
**Author:** Design brainstorming session

## Overview

This document describes the comprehensive performance benchmarking system for LiveTemplate. The system provides phase-specific and end-to-end benchmarks, baseline tracking with benchstat for regression detection, CPU/memory profiling for bottleneck discovery, and CI integration for automated performance monitoring.

## Goals

1. **Comprehensive Coverage**: Benchmark all 5 phases (Parse → Build → Diff → Render → Send) plus end-to-end scenarios
2. **Regression Detection**: Catch performance regressions before merging PRs using statistical comparison
3. **Bottleneck Discovery**: Use profiling to identify and document optimization opportunities
4. **Performance Documentation**: Provide clear performance characteristics for users and contributors

## Architecture

### Benchmark Organization

```
livetemplate/
├── internal/
│   ├── parse/
│   │   ├── parse_bench_test.go          # Phase 1 benchmarks
│   │   └── (existing test files)
│   ├── build/
│   │   ├── build_bench_test.go          # Phase 2 benchmarks
│   │   └── (existing test files)
│   ├── diff/
│   │   ├── diff_bench_test.go           # Phase 3 benchmarks
│   │   └── (existing test files)
│   ├── render/
│   │   ├── render_bench_test.go         # Phase 4 benchmarks
│   │   └── (existing test files)
│   └── send/
│       ├── send_bench_test.go           # Phase 5 benchmarks
│       └── (existing test files)
├── template_bench_test.go               # End-to-end template operations
├── e2e_bench_test.go                    # User journey scenarios
├── tree_bench_test.go                   # Existing (keep fingerprint benchmarks)
├── testdata/
│   ├── benchmarks/
│   │   ├── baseline.txt                 # Git-tracked baseline results
│   │   └── README.md                    # Baseline update guide
│   └── fixtures/                        # Existing test templates
├── profiles/                            # Gitignored profiling output
│   └── .gitignore
├── docs/
│   └── performance/
│       ├── benchmarking-guide.md        # How-to guide
│       ├── performance-characteristics.md
│       └── known-bottlenecks.md
├── Makefile                             # Benchmark targets
└── .github/
    └── workflows/
        └── benchmark.yml                # CI integration
```

**Design Principles:**
- **Co-location**: Phase-specific benchmarks live with their phase code
- **Centralization**: End-to-end benchmarks in root for visibility
- **Reuse**: Leverage existing `tree_bench_test.go` helpers and fixtures
- **Separation**: Profiling output gitignored, findings documented in markdown

### Naming Conventions

**Phase-specific benchmarks:**
```go
BenchmarkParseTemplate
BenchmarkBuildTree
BenchmarkCompareTreesNoChanges
BenchmarkNodeRender
BenchmarkSerializeUpdate
```

**End-to-end benchmarks:**
```go
BenchmarkE2EInitialRender
BenchmarkE2ESmallUpdate
BenchmarkE2ETodoApp
```

**Sub-benchmarks for scale variations:**
```go
BenchmarkBuildTree/small-10-nodes
BenchmarkBuildTree/medium-100-nodes
BenchmarkBuildTree/large-1000-nodes
```

## Phase-Specific Benchmark Coverage

### Phase 1: Parse (`internal/parse/parse_bench_test.go`)

**Entry Points:**
- `BenchmarkParse` - Full parse operation
- `BenchmarkBuildTree` - AST to tree conversion

**Construct-Specific:**
- `BenchmarkParseField` - Simple `{{.Field}}`
- `BenchmarkParseConditional` - `{{if}}...{{else}}...{{end}}`
- `BenchmarkParseRange` - `{{range}}...{{end}}`
- `BenchmarkParseWith` - `{{with}}...{{end}}`
- `BenchmarkParseTemplateInvoke` - `{{template "name" .}}`
- `BenchmarkParseNested` - Deeply nested constructs

**Cache Effectiveness:**
- `BenchmarkParseWithCache/cold` - First parse
- `BenchmarkParseWithCache/warm` - Cached parse
- `BenchmarkEvaluatePipeWithCache` - Expression caching

**Scale Variations:**
- Small: 10 nodes
- Medium: 100 nodes
- Large: 1000 nodes

### Phase 2: Build (`internal/build/build_bench_test.go`)

**Tree Operations:**
- `BenchmarkTreeNodeCreation` - TreeNode allocation
- `BenchmarkTreeNodeMarshalJSON` - Custom JSON marshaling
- `BenchmarkFingerprintCalculation` - Already in `tree_bench_test.go`

**Wrapper Operations:**
- `BenchmarkWrapperInjection/full-html` - Full HTML document
- `BenchmarkWrapperInjection/fragment` - HTML fragment
- `BenchmarkExtractWrapperContent` - Content extraction

**Context Operations:**
- `BenchmarkContextWithStatics` - First render context
- `BenchmarkContextWithoutStatics` - Update context

### Phase 3: Diff (`internal/diff/diff_bench_test.go`)

**Comparison Operations:**
- `BenchmarkCompareTreesNoChanges` - Identical trees
- `BenchmarkCompareTreesSmallChange` - 1-2 fields changed
- `BenchmarkCompareTreesLargeChange` - Many fields changed
- `BenchmarkCompareTreesStructuralChange` - Structure changed

**Range Differential Operations:**
- `BenchmarkRangeDiffUpdate` - Update existing items
- `BenchmarkRangeDiffInsert` - Insert new items
- `BenchmarkRangeDiffRemove` - Remove items
- `BenchmarkRangeDiffReorder` - Reorder items
- `BenchmarkRangeDiffMixed` - Combined operations

**Client Preparation:**
- `BenchmarkPrepareTreeForClient/with-statics`
- `BenchmarkPrepareTreeForClient/without-statics`

**Scale Variations:**
- 10 changes
- 100 changes
- 1000 changes

### Phase 4: Render (`internal/render/render_bench_test.go`)

**HTML Rendering:**
- `BenchmarkNodeRender` - Single node
- `BenchmarkTreeToHTML/simple`
- `BenchmarkTreeToHTML/nested`
- `BenchmarkTreeToHTML/with-ranges`

**Minification:**
- `BenchmarkMinifyHTML` - HTML compression

### Phase 5: Send (`internal/send/send_bench_test.go`)

**Message Parsing:**
- `BenchmarkParseActionFromHTTP` - HTTP form data
- `BenchmarkParseActionFromWebSocket` - WebSocket JSON

**Response Preparation:**
- `BenchmarkPrepareUpdate` - Update wrapping
- `BenchmarkSerializeUpdate` - JSON serialization
- `BenchmarkMarshalOrderedJSON` - Ordered JSON output

## End-to-End Benchmark Coverage

### Template Operations (`template_bench_test.go`)

**Core Operations:**
- `BenchmarkTemplateExecute/initial-render` - First `Execute()` call
- `BenchmarkTemplateExecute/subsequent-render` - Second `Execute()` call
- `BenchmarkTemplateExecuteUpdates/no-changes` - No data changes
- `BenchmarkTemplateExecuteUpdates/small-update` - 1-2 fields changed
- `BenchmarkTemplateExecuteUpdates/large-update` - Many fields changed

**Template Complexity Variations:**
- `BenchmarkTemplate/simple-fields` - Just `{{.Field}}` replacements
- `BenchmarkTemplate/with-conditionals` - If/else branches
- `BenchmarkTemplate/with-ranges` - List iteration
- `BenchmarkTemplate/deeply-nested` - Complex nesting
- `BenchmarkTemplate/mixed-constructs` - All construct types

**Real-World Templates:**
- `BenchmarkTemplate/counter` - Counter example (from testdata)
- `BenchmarkTemplate/todos` - Todo list example (from testdata)

**Concurrent Operations:**
- `BenchmarkTemplateConcurrent/1-goroutine`
- `BenchmarkTemplateConcurrent/10-goroutines`
- `BenchmarkTemplateConcurrent/100-goroutines`

### User Journey Scenarios (`e2e_bench_test.go`)

**Complete Journeys:**
- `BenchmarkE2EUserJourney` - Refactored from `tree_bench_test.go`
- `BenchmarkE2ETodoApp` - Complete todo CRUD flow
- `BenchmarkE2ECounterApp` - Counter increment flow

**Interaction Patterns:**
- `BenchmarkE2EFormSubmission` - Action handling
- `BenchmarkE2ERangeOperations/add-items`
- `BenchmarkE2ERangeOperations/remove-items`
- `BenchmarkE2ERangeOperations/reorder-items`
- `BenchmarkE2ERangeOperations/update-items`

**Session Scenarios:**
- `BenchmarkE2EMultipleSessions/1-session`
- `BenchmarkE2EMultipleSessions/10-sessions`
- `BenchmarkE2EMultipleSessions/100-sessions`

**Memory Scenarios:**
- `BenchmarkE2EMemoryUsage/1000-renders` - Long-running session
- `BenchmarkE2EMemoryUsage/cache-effectiveness` - Cache hit rates

### Benchmark Helpers

**Reuse from `tree_bench_test.go`:**
- `createFlatTree(size int)`
- `createNestedTree(depth, breadth int)`
- `createRangeTree(itemCount int)`

**New Helpers:**
- `createTemplateWithData(templateStr string, data interface{})`
- `createTestTemplate(complexity string)` - "simple", "medium", "complex"
- `simulateUserActivity(template *Template, activities []Activity)`
- `generateRealisticUpdates(template *Template, updateCount int)`

### Data Sizes

- **Small**: 10 items, 2-3 nesting levels, ~500 bytes rendered
- **Medium**: 100 items, 4-5 nesting levels, ~5KB rendered
- **Large**: 1000 items, 6+ nesting levels, ~50KB rendered
- **XLarge**: 10,000 items (stress test scenarios only)

## Baseline System & Regression Tracking

### Baseline Management

**File: `testdata/benchmarks/baseline.txt`**
- Standard Go benchmark output format (from `go test -bench=. -benchmem`)
- Git-tracked, updated manually after verified performance improvements
- Serves as the comparison reference for all regression checks

**When to Update Baselines:**
1. Performance improvements are intentionally made
2. Benchmarks show consistent improvement (run 10x to verify)
3. Changes are reviewed and approved

**DO NOT** update baselines to "fix" regressions.

### Makefile Targets

```makefile
# Run all benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run benchmarks 10 times for statistical confidence
bench-10x:
	go test -bench=. -benchmem -count=10 ./... | tee /tmp/bench-results.txt

# Save current results as baseline
bench-save:
	go test -bench=. -benchmem ./... > testdata/benchmarks/baseline.txt

# Compare current vs baseline using benchstat
bench-compare:
	@echo "Running current benchmarks..."
	@go test -bench=. -benchmem ./... > /tmp/current-bench.txt
	@echo "\nComparing against baseline..."
	@benchstat testdata/benchmarks/baseline.txt /tmp/current-bench.txt

# Quick smoke test (critical benchmarks only)
bench-quick:
	go test -bench='Benchmark(E2E|Template)' -benchmem -timeout=5m ./...

# Profile CPU
profile-cpu:
	@mkdir -p profiles
	go test -bench=. -benchmem -cpuprofile=profiles/cpu.prof ./...
	@echo "\nAnalyze with: go tool pprof profiles/cpu.prof"

# Profile memory
profile-mem:
	@mkdir -p profiles
	go test -bench=. -benchmem -memprofile=profiles/mem.prof ./...
	@echo "\nAnalyze with: go tool pprof profiles/mem.prof"

# Profile everything
profile-all: profile-cpu profile-mem
	@echo "\nProfiles saved in profiles/ directory"
```

### Benchstat Integration

Uses `golang.org/x/perf/cmd/benchstat` for statistical comparison:
- Shows delta percentages and p-values for significance
- Filters noise through multiple iterations
- Provides confidence intervals

**Example Output:**
```
name                    old time/op    new time/op    delta
TemplateExecute-8         1.23µs ± 2%    1.15µs ± 1%   -6.50%  (p=0.000 n=10+10)
E2ESmallUpdate-8          45.2µs ± 3%    43.1µs ± 2%   -4.65%  (p=0.001 n=10+10)

name                    old alloc/op   new alloc/op   delta
TemplateExecute-8          856B ± 0%      792B ± 0%   -7.48%  (p=0.000 n=10+10)
```

## CI Integration

### GitHub Actions Workflow

**File: `.github/workflows/benchmark.yml`**

**Triggers:**
- Every pull request to main branch
- Manual workflow dispatch

**Workflow Steps:**
1. Checkout code with full history
2. Install Go 1.21+
3. Install benchstat tool
4. Run benchmarks (5 iterations for statistical confidence)
5. Compare against committed baseline using benchstat
6. Check for regressions with thresholds
7. Post comparison comment on PR
8. Upload results as artifacts (30-day retention)

### Regression Thresholds

**Critical Benchmarks** (block merge):
- Pattern: `Benchmark(E2E|Template).*`
- Threshold: >20% regression → Fail CI
- Threshold: >10% regression → Warning comment

**Phase-Specific Benchmarks** (informational):
- All other benchmarks
- No CI failure, warnings only
- Used for optimization work

### CI Behavior

**On Regression Detection:**
- >20% on critical: Fail CI, block merge, require explanation
- >10% on critical: Warning comment, require justification
- Phase-specific: Informational comment only

**Manual Override:**
- If regression is justified (e.g., correctness fix), document in PR description
- Maintainer can merge despite warning
- Update baseline in follow-up optimization work

**PR Comment Format:**
```
## Performance Benchmark Results

[benchstat comparison table]

⚠️ Regressions >10% require explanation
❌ Regressions >20% block merge

See benchmarking guide for details.
```

## Profiling Strategy

### Profiling Workflow

**Step 1: Generate Profiles**
```bash
make profile-cpu    # CPU profiling
make profile-mem    # Memory profiling
make profile-all    # Both
```

**Step 2: Analyze Profiles**
```bash
# Interactive web interface
go tool pprof -http=:8080 profiles/cpu.prof
go tool pprof -http=:8080 profiles/mem.prof

# Command-line top functions
go tool pprof -top profiles/cpu.prof
go tool pprof -top profiles/mem.prof

# Generate flame graphs
go tool pprof -flamegraph profiles/cpu.prof > profiles/cpu-flame.svg
```

**Step 3: Targeted Profiling**
```bash
# Profile specific benchmarks
go test -bench=BenchmarkE2E -cpuprofile=profiles/e2e-cpu.prof
go test -bench=BenchmarkParse -memprofile=profiles/parse-mem.prof

# Execution trace for concurrency analysis
go test -bench=BenchmarkTemplateConcurrent -trace=profiles/trace.out
go tool trace profiles/trace.out
```

### Profiling Targets

**CPU Profiling Focus:**
- Parse phase: AST walking, expression evaluation, cache lookups
- Build phase: Tree construction, fingerprint calculation
- Diff phase: Tree comparison, deep equality checks
- Template operations: `Execute()` and `ExecuteUpdates()` hot paths
- Concurrent scenarios: Lock contention, goroutine scheduling

**Memory Profiling Focus:**
- Allocation patterns per phase
- TreeNode creation and growth
- String allocations during HTML rendering
- Cache memory usage (parse caches, structure registry)
- Range operation allocations (update/insert/remove ops)

### Bottleneck Documentation

After profiling, document findings in `docs/performance/known-bottlenecks.md`:

**Structure:**
```markdown
# Known Performance Bottlenecks

Last profiled: YYYY-MM-DD
Go version: 1.21

## CPU Bottlenecks

### Phase 1: Parse
- **Finding**: [Description from profile data]
- **Location**: file.go:line
- **Impact**: X% of total CPU time
- **Optimization opportunity**: [Potential fix/improvement]

### Phase 2: Build
[...]

## Memory Bottlenecks

### Allocations per Operation
- Initial render: X MB
- Small update: Y MB
- Large update: Z MB

### Cache Memory Usage
[...]

## Optimization Priorities

1. [High] Most impactful bottleneck
2. [Medium] Secondary bottleneck
3. [Low] Minor optimization opportunity
```

**Profile Storage:**
- `profiles/` directory is gitignored (profiles are machine-specific)
- Document findings in markdown (portable, reviewable)
- Include profile generation commands for reproducibility

## Documentation Structure

### User-Facing: README.md Section

```markdown
## Performance

LiveTemplate is designed for high-performance reactive updates with minimal bandwidth.

### Key Metrics

| Operation | Latency | Bandwidth Savings |
|-----------|---------|-------------------|
| Initial Render | ~1.2ms | - |
| Small Update | ~120µs | 85% vs full render |
| Large Update (100 items) | ~2.5ms | 65% vs full render |
| Range Add/Remove | ~150µs | 80% vs full render |

*Benchmarked on Go 1.21, Apple M1, typical web app templates*

### Running Benchmarks

```bash
# Run all benchmarks
make bench

# Compare against baseline
make bench-compare

# Generate profiles
make profile-cpu
make profile-mem
```

See [Performance Guide](docs/performance/benchmarking-guide.md) for details.
```

### Developer-Facing: `docs/performance/benchmarking-guide.md`

Comprehensive guide covering:
- How to run benchmarks locally
- How to interpret benchstat output
- When and how to update baselines
- How to use profiling tools
- How to write new benchmarks
- CI integration details

### Technical Deep-Dive: `docs/performance/performance-characteristics.md`

Detailed analysis of each phase:
- Architectural overview with performance implications
- Per-phase analysis: operations, complexity, optimizations, benchmark results
- End-to-end performance characteristics
- Scalability characteristics
- Memory usage patterns
- Links to optimization opportunities

### Bottleneck Analysis: `docs/performance/known-bottlenecks.md`

Generated after initial profiling:
- CPU bottlenecks by phase
- Memory bottlenecks and allocation patterns
- Optimization priorities
- Updated regularly as profiling reveals insights

## Implementation Plan

### Phase 1: Setup Infrastructure
1. Create `testdata/benchmarks/` directory with README
2. Create `profiles/` directory with `.gitignore`
3. Create `docs/performance/` directory
4. Add Makefile targets for benchmarks and profiling
5. Set up `.github/workflows/benchmark.yml`

### Phase 2: Phase-Specific Benchmarks
1. Create `internal/parse/parse_bench_test.go`
2. Create `internal/build/build_bench_test.go`
3. Create `internal/diff/diff_bench_test.go`
4. Create `internal/render/render_bench_test.go`
5. Create `internal/send/send_bench_test.go`

### Phase 3: End-to-End Benchmarks
1. Create `template_bench_test.go`
2. Create `e2e_bench_test.go`
3. Refactor `tree_bench_test.go` (move user journey to e2e)

### Phase 4: Profiling & Baseline
1. Run initial benchmarks, save as baseline
2. Generate CPU and memory profiles
3. Analyze profiles, document bottlenecks
4. Create `docs/performance/known-bottlenecks.md`

### Phase 5: Documentation
1. Create `docs/performance/benchmarking-guide.md`
2. Create `docs/performance/performance-characteristics.md`
3. Update README with performance section
4. Document baseline update workflow

### Phase 6: CI Validation
1. Test CI workflow on a test branch
2. Verify benchstat comparison works correctly
3. Verify regression thresholds trigger appropriately
4. Validate PR comment posting

## Success Criteria

1. **Comprehensive Coverage**: All 5 phases + end-to-end scenarios benchmarked
2. **Regression Detection**: CI catches performance regressions >10% on critical paths
3. **Bottleneck Discovery**: CPU and memory profiles identify optimization opportunities
4. **Documentation**: Users understand performance characteristics, contributors know how to benchmark
5. **Maintainability**: Baseline system is easy to use, benchmarks are stable and meaningful

## Future Enhancements

- **Trend Analysis**: Track performance over time with historical data
- **Continuous Benchmarking**: Automated baseline updates on main branch
- **Visualization**: Performance dashboards with graphs
- **Microbenchmarks**: More granular benchmarks for specific algorithms
- **Comparative Benchmarks**: Compare against other template engines

## References

- Go Benchmarking: https://golang.org/pkg/testing/#hdr-Benchmarks
- Benchstat: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
- pprof: https://github.com/google/pprof
- LiveTemplate 5-Phase Architecture: `CLAUDE.md`
