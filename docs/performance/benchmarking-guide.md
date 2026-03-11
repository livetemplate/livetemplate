# Performance Benchmarking Guide

## Overview

This guide explains how to run, interpret, and contribute to LiveTemplate's performance benchmarks.

## Benchmark Organization

### Phase-Specific Benchmarks

Benchmarks co-located with their phase code:

- **Phase 1 (Parse):** `internal/parse/parse_bench_test.go`
- **Phase 2 (Build):** `internal/build/build_bench_test.go`
- **Phase 3 (Diff):** `internal/diff/diff_bench_test.go`
- **Phase 4 (Render):** `internal/render/render_bench_test.go`
- **Phase 5 (Send):** `internal/send/send_bench_test.go`

### End-to-End Benchmarks

Comprehensive scenarios in root directory:

- **Template Operations:** `template_bench_test.go`
- **User Journeys:** `e2e_bench_test.go`
- **Tree Operations:** `tree_bench_test.go` (fingerprinting)
- **Error Handling:** `error_bench_test.go`
- **Session Operations:** `internal/session/registry_bench_test.go` (async sends, concurrent connections, buffer sizes)

## Running Benchmarks

### All Benchmarks

```bash
make bench
```

Runs all benchmarks once with memory statistics.

### Statistical Confidence (10 Iterations)

```bash
make bench-10x
```

Runs 10 iterations to account for variance. Use this before updating baselines.

### Specific Benchmarks

```bash
# Run phase-specific benchmarks
GOWORK=off go test -bench=. -benchmem ./internal/parse/ -run=^$

# Run end-to-end benchmarks only
GOWORK=off go test -bench=BenchmarkE2E -benchmem -run=^$

# Run specific benchmark
GOWORK=off go test -bench=BenchmarkTemplateExecute -benchmem -run=^$
```

### Quick Smoke Test

```bash
make bench-quick
```

Runs only critical benchmarks (E2E and Template operations) for faster feedback.

## Comparing Against Baseline

### Standard Comparison

```bash
make bench-compare
```

Runs current benchmarks and compares against committed baseline using benchstat.

### Manual Comparison

```bash
# Save current results
go test -bench=. -benchmem ./... > /tmp/new.txt

# Compare with benchstat
benchstat testdata/benchmarks/baseline.txt /tmp/new.txt
```

## Interpreting Results

### Benchmark Output Format

```
BenchmarkTemplateExecute/initial-render-8    50000    23456 ns/op    4567 B/op    123 allocs/op
```

- `50000`: Number of iterations
- `23456 ns/op`: Nanoseconds per operation
- `4567 B/op`: Bytes allocated per operation
- `123 allocs/op`: Number of allocations per operation
- `-8`: Number of CPUs (GOMAXPROCS)

### Benchstat Comparison Output

```
name                    old time/op    new time/op    delta
TemplateExecute-8         1.23µs ± 2%    1.15µs ± 1%   -6.50%  (p=0.000 n=10+10)
```

- **old time/op:** Baseline performance
- **new time/op:** Current performance
- **delta:** Percentage change (negative = improvement)
- **±:** Variance across iterations
- **p-value:** Statistical significance (p < 0.05 = significant)
- **n=10+10:** Number of samples in each dataset

### What to Look For

**Good:**
- Negative delta (performance improvement)
- Low variance (± small percentage)
- p < 0.05 (statistically significant)

**Concerning:**
- Positive delta >10% on critical benchmarks
- High variance (indicates unstable benchmarks)
- p > 0.05 (change might be noise)

## Updating Baselines

### When to Update

Update baselines only when:

1. **Performance improvements are intentional** - You made changes specifically to improve performance
2. **Improvements are consistent** - Verified across 10 iterations
3. **Changes are reviewed** - Another maintainer has reviewed the changes

### DO NOT update baselines to:
- "Fix" unexpected regressions
- Make CI pass without understanding why performance degraded
- Hide performance issues

### How to Update

```bash
# 1. Run benchmarks 10 times
make bench-10x

# 2. Review results for consistency
cat /tmp/bench-results.txt

# 3. If consistent improvements, save baseline
make bench-save

# 4. Commit with clear description
git add testdata/benchmarks/baseline.txt
git commit -m "perf: update baseline after [description of change]

Improvements:
- TemplateExecute: 15% faster
- TreeComparison: 20% fewer allocations

Verified across 10 iterations."
```

## Profiling

### Generate Profiles

```bash
# CPU profiling
make profile-cpu

# Memory profiling
make profile-mem

# Both
make profile-all
```

### Analyze Profiles

#### Interactive Web Interface

```bash
go tool pprof -http=:8080 profiles/cpu.prof
```

Opens browser with:
- Flame graph visualization
- Top functions
- Source code view
- Call graph

#### Command-Line Analysis

```bash
# Top 20 CPU consumers (cumulative)
go tool pprof -top -cum profiles/cpu.prof | head -20

# Top 20 memory allocators
go tool pprof -top -alloc_space profiles/mem.prof | head -20

# Function-specific details
go tool pprof -list=FunctionName profiles/cpu.prof
```

#### Flame Graph

```bash
go tool pprof -flamegraph profiles/cpu.prof > cpu-flame.svg
open cpu-flame.svg
```

### Profile Specific Benchmarks

```bash
# Profile just E2E benchmarks
GOWORK=off go test -bench=BenchmarkE2E -cpuprofile=profiles/e2e-cpu.prof -run=^$

# Profile with execution trace (for concurrency issues)
GOWORK=off go test -bench=BenchmarkTemplateConcurrent -trace=profiles/trace.out -run=^$
go tool trace profiles/trace.out
```

### What to Look For

**CPU Profile:**
- Functions consuming >10% of time
- Unexpected hot paths
- Lock contention (sync.Mutex.Lock)
- Excessive allocation (runtime.mallocgc)

**Memory Profile:**
- High allocation counts
- Large allocations
- Allocations in hot paths

## Writing New Benchmarks

### Benchmark Structure

```go
func BenchmarkFeature(b *testing.B) {
    // Setup (outside timing)
    data := setupTestData()

    // Reset timer after setup
    b.ResetTimer()

    // Report allocations
    b.ReportAllocs()

    // Benchmark loop
    for i := 0; i < b.N; i++ {
        result := functionToTest(data)
        // Use result to prevent optimization
        _ = result
    }
}
```

### Sub-Benchmarks

```go
func BenchmarkFeature(b *testing.B) {
    tests := []struct{
        name string
        data interface{}
    }{
        {"small", smallData},
        {"large", largeData},
    }

    for _, tt := range tests {
        b.Run(tt.name, func(b *testing.B) {
            b.ReportAllocs()
            for i := 0; i < b.N; i++ {
                _ = functionToTest(tt.data)
            }
        })
    }
}
```

### Guidelines

1. **Realistic workloads** - Use data representative of real usage
2. **Stable results** - Avoid randomness, use fixed seeds if needed
3. **Appropriate scale** - Test small/medium/large inputs
4. **Memory tracking** - Always use `b.ReportAllocs()`
5. **Setup isolation** - Use `b.ResetTimer()` after setup
6. **Prevent optimization** - Use results to prevent dead code elimination

## CI Integration

### How Benchmarks Run in CI

1. GitHub Actions runs benchmarks on every PR
2. 5 iterations for statistical confidence
3. Compares against committed baseline using benchstat
4. Posts comparison as PR comment

### Regression Thresholds

**Critical Benchmarks** (E2E, Template operations):
- >10% regression: Warning comment
- >20% regression: CI failure, blocks merge

**Phase-Specific Benchmarks:**
- Informational only, no CI failure

### Overriding CI

If regression is justified (e.g., correctness fix):

1. Document in PR description why regression is acceptable
2. Explain what was traded for correctness/features
3. Plan for future optimization (if applicable)
4. Maintainer can merge despite warning

## Troubleshooting

### Flaky Benchmarks

If benchmarks show high variance:

```bash
# Run with count to see variance
go test -bench=BenchmarkName -benchmem -count=10 | tee results.txt
benchstat results.txt
```

High variance (>5%) indicates:
- Non-deterministic code (random, timing-dependent)
- System noise (other processes)
- GC interference

Fix by:
- Using fixed seeds for randomness
- Increasing work per iteration
- Running on idle system

### Unexpectedly Slow Benchmarks

Check:
- Are you running in dev mode? (adds overhead)
- Is GOWORK causing issues? (use `GOWORK=off`)
- Are you timing setup code? (use `b.ResetTimer()`)
- Is profiling enabled? (adds overhead)

### Allocation Surprises

Profile to find unexpected allocations:

```bash
go test -bench=BenchmarkName -memprofile=mem.prof
go tool pprof -top -alloc_objects mem.prof
```

## Reference

### Make Targets

| Target | Description |
|--------|-------------|
| `make bench` | Run all benchmarks once |
| `make bench-10x` | Run 10 iterations for confidence |
| `make bench-save` | Save current as baseline |
| `make bench-compare` | Compare against baseline |
| `make bench-quick` | Critical benchmarks only |
| `make profile-cpu` | Generate CPU profile |
| `make profile-mem` | Generate memory profile |
| `make profile-all` | Generate all profiles |

### Tools

- **benchstat:** `go install golang.org/x/perf/cmd/benchstat@latest`
- **pprof:** Built into Go toolchain

### Related Documents

- [Performance Characteristics](performance-characteristics.md) - Detailed phase analysis
- [Known Bottlenecks](known-bottlenecks.md) - Current optimization opportunities
