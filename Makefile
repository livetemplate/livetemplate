.PHONY: bench bench-10x bench-save bench-compare bench-quick profile-cpu profile-mem profile-all

# Run all benchmarks
bench:
	GOWORK=off go test -bench=. -benchmem ./...

# Run benchmarks 10 times for statistical confidence
bench-10x:
	GOWORK=off go test -bench=. -benchmem -count=10 ./... 2>&1 | grep "^Benchmark" | grep -v -E "(livetemplate\.New|WARN|INFO|DEBUG|ERROR)" | tee /tmp/bench-results.txt

# Save current results as baseline
bench-save:
	GOWORK=off go test -bench=. -benchmem ./... 2>&1 | grep "^Benchmark" | grep -v -E "(livetemplate\.New|WARN|INFO|DEBUG|ERROR)" > testdata/benchmarks/baseline.txt
	@echo "Baseline saved to testdata/benchmarks/baseline.txt"

# Compare current vs baseline using benchstat
bench-compare:
	@echo "Running current benchmarks..."
	@GOWORK=off go test -bench=. -benchmem ./... 2>&1 | grep "^Benchmark" | grep -v -E "(livetemplate\.New|WARN|INFO|DEBUG|ERROR)" > /tmp/current-bench.txt
	@echo "\nComparing against baseline..."
	@benchstat testdata/benchmarks/baseline.txt /tmp/current-bench.txt

# Quick smoke test (critical benchmarks only)
bench-quick:
	GOWORK=off go test -bench='Benchmark(E2E|Template)' -benchmem -timeout=5m ./...

# Profile CPU
profile-cpu:
	@mkdir -p profiles
	GOWORK=off go test -bench=. -benchmem -cpuprofile=profiles/cpu.prof ./...
	@echo "\nAnalyze with: go tool pprof profiles/cpu.prof"

# Profile memory
profile-mem:
	@mkdir -p profiles
	GOWORK=off go test -bench=. -benchmem -memprofile=profiles/mem.prof ./...
	@echo "\nAnalyze with: go tool pprof profiles/mem.prof"

# Profile everything
profile-all: profile-cpu profile-mem
	@echo "\nProfiles saved in profiles/ directory"
