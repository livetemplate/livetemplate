# LiveTemplate Roadmap

**Current version:** v0.8.17

This roadmap shows what's actively being worked on, what's coming next, and what's on the horizon. For known limitations and workarounds, see [Current Limitations](docs/references/current-limitations.md). For detailed feature proposals, see `docs/proposals/`. For completed historical plans, see `docs/archive/implementation-plans/`.

---

## Now: Finishing v0.9

### Production Polish

Remaining documentation and metrics work from the scalability milestones (M1/M2):

- [ ] Database connection health checker (`HealthChecker` for `*sql.DB`)
- [ ] Kubernetes probe configuration docs
- [ ] Capacity planning documentation (connections vs memory estimation)
- [ ] RedisSessionStore migration guide (from MemorySessionStore)
- [ ] Pub/sub latency metrics (p50/p95/p99)

### Client Simplification (Phase 6B/6C)

Deferred work from the architecture improvements — Phase 6A complete, remaining 6B/6C estimated 3-4 days:

- [ ] State management refactor — consolidate `rangeState`, `rangeStatics`, `rangeIdKeys` into unified state; simplify `deepMergeTreeNodes`
- [ ] Code quality pass — remove redundant type checks, extract helpers
- [ ] Fix E2E test isolation (chromedp tests currently excluded from CI)

See [docs/roadmap/ARCHITECTURE_IMPROVEMENTS.md](docs/roadmap/ARCHITECTURE_IMPROVEMENTS.md) for full context.

---

## Next: Feature Development

### Lifecycle Hooks (`lvt-hook`)

Declarative JS library integration for charts, maps, rich text editors. Connects DOM elements to JavaScript lifecycle callbacks (`mounted`, `updated`, `destroyed`). Client-side only — no server changes needed.

See [docs/proposals/lifecycle-hooks-proposal.md](docs/proposals/lifecycle-hooks-proposal.md)

### Session Resume Protocol

Enable clients to resume sessions after reconnection without full re-render. Requires coordination between client reconnection logic and server session state. Last remaining feature work from M2.

---

## Later: Scaling & Future Features

### Automatic Form Binding (`lvt-bind`)

Two-way binding between HTML form inputs and server state. Eliminates boilerplate for simple field updates.

See [docs/proposals/lvt-bind-proposal.md](docs/proposals/lvt-bind-proposal.md)

### Enterprise Scale (M3)

Advanced resilience and optimization for 50K+ connections per instance:

- WebSocket compression (`permessage-deflate`)
- Circuit breakers for Redis/DB with memory fallback
- Rate limiting per action type and per IP
- Tree cache eviction (LRU) and per-connection memory limits
- Performance profiling and hot path optimization
- Multi-region architecture documentation

See [docs/archive/implementation-plans/scalability-roadmap.md](docs/archive/implementation-plans/scalability-roadmap.md) for the full M3 task breakdown.

### Grafana Dashboard

Pre-built monitoring dashboard JSON for LiveTemplate Prometheus metrics.

---

## Blocked: Waiting on Upstream

### HTML Fallback Test Coverage (3 remaining scenarios)

Blocked on Go template features not yet available:

- Labelled `break`/`continue` — requires Go template upstream support
- `range` over `iter.Seq` / generator functions — requires Go template integration
- Spec documentation update — depends on the above

See [docs/roadmap/html-fallback-coverage.md](docs/roadmap/html-fallback-coverage.md)

---

## Completed

- **M1: Production Foundation** — Health checks, connection limits, graceful shutdown, Prometheus metrics, env-based configuration, production examples (28/35 tasks, remaining are docs listed above)
- **M2: Horizontal Scaling** — Redis session store, distributed pub/sub, client reconnection with exponential backoff and jitter (18/21 tasks, remaining listed above)
- **Architecture Improvements Phases 1-5** — Stateful structure bug fix (82% payload reduction), server-side ID metadata, unified range operations, type-safe TreeNode, optimized fingerprinting
- **v1.0 Internal Refactoring** — Zero breaking changes, internal package reorganization into 5-phase architecture
