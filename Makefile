.PHONY: bench bench-ci bench-10x bench-save bench-compare bench-quick system-card profile-cpu profile-mem profile-all profile-pkg coverage coverage-html

# Capacity-planning sub-benchmarks (high N / history / doc sizes) are excluded
# from CI, baseline, and comparison runs: they take hundreds of ms to seconds
# per op and are informational sweeps, not gate material. Run them directly
# (e.g. `go test -bench 'TopicFanout_FullPipeline/N=10000' .`) for capacity
# numbers. The CI workflow uses `make bench-ci`, so this is the single source
# of truth for the excluded set.
# NOTE: -skip matches each token as an UNANCHORED substring of the sub-bench
# path element, so exclusions prefix-match longer values (N=1000 also skips
# N=10000 — intended). The flip side: before adding a new sweep value, check
# no exclusion token is a substring of it (e.g. a hypothetical hist=10000x
# would be silently skipped by hist=1000).
BENCH_SKIP_CAPACITY := Benchmark(TopicFanout_FullPipeline|TriggerActionFanout|RedisCrossInstanceFanout|ChatAppendFanout|LargeDocDiff)/(N=1000|N=10000|hist=1000|hist=10000|peers=1000|files=50|files=100)
BENCH_FILTER := grep "^Benchmark" | grep -v -E "(livetemplate\.New|WARN|INFO|DEBUG|ERROR)"

# Run all benchmarks (including capacity sweeps — expect several minutes)
bench:
	GOWORK=off go test -bench=. -benchmem ./...

# GOMAXPROCS is pinned for every run that feeds benchstat: the -N suffix is
# part of the benchmark name benchstat pairs rows by, so a baseline from an
# 8-core machine would silently pair with NOTHING from a 4-vCPU CI runner and
# the gate would fail closed on every healthy PR. Any fixed value works; it
# must simply be the same everywhere.
BENCH_CPU := -cpu=8

# The gate/baseline benchmark set: everything except capacity sweeps.
# COUNT defaults to 1 (allocs/op — the gated metric — is deterministic).
bench-ci:
	@GOWORK=off go test -run='^$$' -bench=. -skip='$(BENCH_SKIP_CAPACITY)' $(BENCH_CPU) -benchmem -count=$(or $(COUNT),1) ./... 2>&1 | $(BENCH_FILTER)

# Run gate-set benchmarks 10 times for statistical confidence
bench-10x:
	GOWORK=off go test -run='^$$' -bench=. -skip='$(BENCH_SKIP_CAPACITY)' $(BENCH_CPU) -benchmem -count=10 ./... 2>&1 | $(BENCH_FILTER) | tee /tmp/bench-results.txt

# Save current gate-set results as baseline (with provenance header)
bench-save:
	@printf '# Single-run baseline over the gate set (make bench-ci; capacity sweeps excluded)\n# %s | %s | %s\n# The CI gate compares B/op and allocs/op (machine-independent); sec/op deltas vs this file are cross-machine and informational only.\n' "$$(go version)" "$$(uname -sm)" "$$(date +%F)" > testdata/benchmarks/baseline.txt
	@$(MAKE) --no-print-directory bench-ci >> testdata/benchmarks/baseline.txt
	@echo "Baseline saved to testdata/benchmarks/baseline.txt"

# Compare current vs baseline using benchstat
bench-compare:
	@echo "Running current benchmarks (gate set)..."
	@$(MAKE) --no-print-directory bench-ci > /tmp/current-bench.txt
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
