# Performance Baselines

## When to Update Baselines

Update baselines when:
1. Performance improvements are intentionally made
2. Benchmarks show consistent improvement (run 10x to verify)
3. Changes are reviewed and approved

DO NOT update baselines to "fix" regressions.

## How to Update

1. Run benchmarks multiple times:
   ```bash
   make bench-10x
   ```

2. Verify improvements are real and consistent

3. Save new baseline:
   ```bash
   make bench-save
   ```

4. Commit with description:
   ```bash
   git add testdata/benchmarks/baseline.txt
   git commit -m "perf: update baseline after {description}"
   ```

## Comparing Against Baseline

```bash
make bench-compare
```

Uses benchstat to show statistical comparison.
