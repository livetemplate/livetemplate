.PHONY: bench bench-10x bench-save bench-compare bench-quick profile-cpu profile-mem profile-all

# Run all benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run benchmarks 10 times for statistical confidence
bench-10x:
	go test -bench=. -benchmem -count=10 ./... | tee /tmp/bench-results.txt

# Save current results as baseline
bench-save:
	go test -bench=. -benchmem ./... > testdata/benchmarks/baseline.txt
	@echo "Baseline saved to testdata/benchmarks/baseline.txt"

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
