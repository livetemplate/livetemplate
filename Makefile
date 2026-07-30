.PHONY: bench bench-10x bench-save bench-compare bench-quick system-card profile-cpu profile-mem profile-all profile-pkg coverage coverage-html

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

# Generate system card with capacity planning estimates
system-card:
	GOWORK=off go test -run TestSystemCard -v -timeout=120s .

# Profile CPU (root package only, as -cpuprofile doesn't work with ./...)
profile-cpu:
	@mkdir -p profiles
	GOWORK=off go test -bench=. -benchmem -cpuprofile=profiles/cpu.prof .
	@echo "\nAnalyze with: go tool pprof profiles/cpu.prof"

# Profile memory (root package only, as -memprofile doesn't work with ./...)
profile-mem:
	@mkdir -p profiles
	GOWORK=off go test -bench=. -benchmem -memprofile=profiles/mem.prof .
	@echo "\nAnalyze with: go tool pprof profiles/mem.prof"

# Profile everything
profile-all: profile-cpu profile-mem
	@echo "\nProfiles saved in profiles/ directory"

# Profile a single package (PKG required; -cpuprofile/-memprofile don't work
# with ./..., which is why profile-cpu/profile-mem are root-only). BENCH
# narrows the benchmark set (default: all). Example:
#   make profile-pkg PKG=./internal/session BENCH=AsyncSendThroughput
profile-pkg:
	@test -n "$(PKG)" || (echo "usage: make profile-pkg PKG=./internal/session [BENCH=regex]" && exit 1)
	@mkdir -p profiles
	GOWORK=off go test -run '^$$' -bench='$(or $(BENCH),.)' -benchmem \
		-cpuprofile=profiles/$(notdir $(PKG))-cpu.prof \
		-memprofile=profiles/$(notdir $(PKG))-mem.prof $(PKG)
	@echo "\nAnalyze with: go tool pprof profiles/$(notdir $(PKG))-cpu.prof"

# Show test coverage summary
coverage:
	@GOWORK=off go test -cover ./... 2>&1 | grep -E "^ok\s" | awk '{print $$2 "\t" $$5}'

# Generate detailed HTML coverage report
coverage-html:
	@mkdir -p coverage
	GOWORK=off go test -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@go tool cover -func=coverage/coverage.out | tail -1
	@echo "\nHTML report: coverage/coverage.html"
