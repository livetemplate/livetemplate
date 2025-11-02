# LiveTemplate Production Scalability & Availability Roadmap

**Version:** 1.0
**Last Updated:** 2025-11-01
**Status:** 🔴 In Progress

---

## Vision

Enable LiveTemplate applications to scale from **single-host hobby projects** to **production systems handling millions of concurrent WebSocket connections** while maintaining operational simplicity.

### Target Deployment Scenarios

| Scenario | Connections | Instances | Redis Required | Use Case |
|----------|------------|-----------|----------------|----------|
| **Hobby** | <1,000 | 1 | No | Personal projects, prototypes |
| **Small Business** | 1K-10K | 1-2 | Optional | Startups, internal tools |
| **Production** | 10K-100K | 2-10 | Yes | SaaS applications |
| **Enterprise** | 100K-1M+ | 10-100+ | Yes | Large-scale platforms |

---

## Guiding Principles

### 1. **Progressive Enhancement**
- Single-host deployments should be trivial (no external dependencies required)
- Each scaling tier adds complexity only when needed
- Feature flags enable/disable distributed features

### 2. **Operational Simplicity**
- **Redis as the ONLY external dependency** for distributed features
- No Kafka, no etcd, no Consul, no ZooKeeper
- Redis provides: sessions, pub/sub, and queuing

### 3. **Production-First Design**
- Health checks, metrics, graceful shutdown are TABLE STAKES
- Observability built-in, not bolted-on
- Fail-safe defaults (connection limits, timeouts, circuit breakers)

### 4. **Backward Compatibility**
- Existing single-host applications continue to work
- Distributed features are opt-in via configuration
- No breaking changes to core API

---

## Milestone Overview

| Milestone | Focus | Duration | Status | Target Completion |
|-----------|-------|----------|--------|-------------------|
| **M1: Production Foundation** | Critical fixes, health checks, limits | 4-6 weeks | 🔴 TODO | Week 6 |
| **M2: Horizontal Scaling** | Redis integration, multi-instance | 4-5 weeks | 🔴 TODO | Week 11 |
| **M3: Enterprise Scale** | Advanced resilience, optimization | 8-12 weeks | 🔴 TODO | Week 26 |

**Overall Progress:** `[████████░░░░░░░░░░░░] 42% (25/59 tasks)`

---

## Milestone 1: Production Foundation (CRITICAL)

**Goal:** Make LiveTemplate production-ready for **single-host and basic multi-host** deployments with proper observability, resource limits, and operational safety.

**Success Criteria:**
- ✅ Applications can be deployed with confidence
- ✅ Health checks enable Kubernetes readiness/liveness probes
- ✅ Graceful shutdown prevents connection loss during deploys
- ✅ Connection limits prevent OOM kills
- ✅ Prometheus metrics enable capacity planning

**Duration:** 4-6 weeks
**Progress:** `[█████████████░░░░░░░] 25/35 tasks (71%)`

---

### 1.1 Health Checks & Readiness ✅

**Files:** `health.go` (new), `health_test.go` (new)

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Design `HealthChecker` interface for dependency checks | 🔴 CRITICAL | ✅ DONE | 2 days | - |
| Implement `/health/live` endpoint (basic liveness) | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Implement `/health/ready` endpoint (checks dependencies) | 🔴 CRITICAL | ✅ DONE | 2 days | - |
| Add health check for SessionStore (ping test) | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Add health check for database connections | 🟡 HIGH | 🔴 TODO | 1 day | - |
| Document Kubernetes probe configuration | 🟡 HIGH | 🔴 TODO | 1 day | - |

**Acceptance Criteria:**
- `/health/live` returns 200 OK when process is running
- `/health/ready` returns 503 Service Unavailable when dependencies unhealthy
- Kubernetes examples show proper probe configuration
- Health checks complete in <100ms

**Example API:**
```go
// health.go
type HealthChecker interface {
    Check(ctx context.Context) error
}

type HealthHandler struct {
    checkers map[string]HealthChecker
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request)
```

---

### 1.2 Connection Limits & Load Shedding ✅

**Files:** `limits.go` (new), `mount.go`, `template.go`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Add `MaxConnections` config option | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Implement connection counting in `ConnectionLimits` | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Reject new connections when at capacity (503 response) | 🔴 CRITICAL | ✅ DONE | 2 days | - |
| Add per-group connection limits (prevent single user exhaustion) | 🟡 HIGH | ✅ DONE | 2 days | - |
| Add backpressure metrics (rejections counter) | 🟡 HIGH | ✅ DONE | 1 day | - |
| Document capacity planning (connections vs memory) | 🟡 MEDIUM | 🔴 TODO | 1 day | - |

**Acceptance Criteria:**
- Connection attempts beyond limit receive HTTP 503
- `active_connections` metric exposed
- Per-group limits prevent single-user DOS
- Documentation explains memory-per-connection estimation

**Example Usage:**
```go
handler := livetemplate.Mount(rootStore,
    livetemplate.WithMaxConnections(10000),
    livetemplate.WithMaxConnectionsPerGroup(100),
)
```

---

### 1.3 Graceful Shutdown ✅

**Files:** `mount.go`, `shutdown_test.go` (new)

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Add `Shutdown(ctx context.Context)` method to `LiveHandler` | 🔴 CRITICAL | ✅ DONE | 2 days | - |
| Stop accepting new connections during shutdown | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Send close frames to all active WebSocket connections | 🔴 CRITICAL | ✅ DONE | 2 days | - |
| Wait for in-flight actions to complete (with timeout) | 🟡 HIGH | ✅ DONE | 2 days | - |
| Add example with `http.Server.Shutdown()` integration | 🟡 HIGH | ✅ DONE | 1 day | - |
| Document shutdown timeout recommendations | 🟡 MEDIUM | ✅ DONE | 1 day | - |

**Acceptance Criteria:**
- Clients receive WebSocket close frame (code 1001 Going Away)
- In-flight actions complete before shutdown (with timeout)
- No goroutine leaks after shutdown
- Zero-downtime deploy possible with connection draining

**Example:**
```go
// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
handler.Shutdown(ctx)
```

---

### 1.4 Observability & Metrics Export

**Files:** `internal/observe/metrics.go`, `internal/observe/prometheus.go` (new), `mount.go`, `template.go`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Design Prometheus metric names (follow naming conventions) | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Implement `PrometheusExporter` (exposes `/metrics` endpoint) | 🔴 CRITICAL | ✅ DONE | 3 days | - |
| Export connection metrics (active, total, rejected) | 🔴 CRITICAL | ✅ DONE | 1 day | - |
| Export performance metrics (action duration histograms) | 🟡 HIGH | ✅ DONE | 2 days | - |
| Add request tracing with trace IDs (log correlation) | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Create Grafana dashboard JSON for monitoring | 🟡 MEDIUM | 🔴 TODO | 2 days | - |

**Acceptance Criteria:**
- `/metrics` endpoint returns Prometheus format
- All key metrics exposed (connections, actions, errors, durations)
- Trace IDs appear in all structured logs
- Grafana dashboard visualizes key metrics

**Prometheus Metrics:**
```
livetemplate_connections_active{instance="host1"} 1234
livetemplate_connections_total{instance="host1"} 5678
livetemplate_connections_rejected_total{instance="host1"} 12
livetemplate_action_duration_seconds{action="create",quantile="0.95"} 0.042
livetemplate_broadcasts_sent_total{instance="host1"} 9876
```

---

### 1.5 Configuration Management ✅

**Files:** `config.go` (new), `config_test.go` (new), `docs/CONFIGURATION.md` (new)

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Design environment variable schema (LVT_* prefix) | 🟡 HIGH | ✅ DONE | 1 day | - |
| Implement config loading from env vars | 🟡 HIGH | ✅ DONE | 2 days | - |
| Add validation for config values | 🟡 HIGH | ✅ DONE | 1 day | - |
| Document all configuration options | 🟡 MEDIUM | ✅ DONE | 2 days | - |
| Update examples to use env-based config | 🟡 MEDIUM | ✅ DONE | 1 day | - |

**Acceptance Criteria:**
- All config options available via environment variables
- Config validation prevents invalid values
- Examples show 12-factor app pattern

**Example:**
```bash
export LVT_MAX_CONNECTIONS=10000
export LVT_SHUTDOWN_TIMEOUT=30s
export LVT_LOG_LEVEL=info
export LVT_REDIS_URL=redis://localhost:6379/0  # Optional for M2
```

---

### 1.6 Production Examples

**Files:** `examples/production/` (new directory)

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Create `examples/production/single-host/` with full setup | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Add Dockerfile with multi-stage build | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Add docker-compose.yml with Postgres + Redis | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Add systemd service file for bare-metal deploys | 🟡 MEDIUM | 🔴 TODO | 1 day | - |
| Add Kubernetes manifests (Deployment, Service, Ingress) | 🟡 MEDIUM | 🔴 TODO | 3 days | - |
| Document deployment procedures | 🟡 MEDIUM | 🔴 TODO | 2 days | - |

**Acceptance Criteria:**
- Production example runs with `docker-compose up`
- Kubernetes deployment passes health checks
- Documentation covers common deployment scenarios

---

### Milestone 1 Summary

**Total Tasks:** 35
**Completed:** 22
**In Progress:** 0
**TODO:** 13

**Estimated Effort:** 4-6 weeks with 1 engineer

**Critical Path:**
1. Health checks + Connection limits (Week 1-2)
2. Graceful shutdown (Week 2-3)
3. Prometheus metrics (Week 3-4)
4. Configuration + Examples (Week 5-6)

**Dependencies:** None (all can start immediately)

**Risks:**
- WebSocket close frame handling may require client updates
- Prometheus integration may reveal need for better histogram implementation

---

## Milestone 2: Horizontal Scaling (HIGH PRIORITY)

**Goal:** Enable **true horizontal scaling** across multiple instances using Redis for distributed state and pub/sub.

**Success Criteria:**
- ✅ Applications can run 2-10 instances behind load balancer
- ✅ Sessions persist across instance restarts
- ✅ Broadcasts reach all users across all instances
- ✅ Zero-downtime deploys via rolling updates
- ✅ Auto-scaling based on connection count

**Duration:** 6-8 weeks
**Progress:** `[░░░░░░░░░░] 0/19 tasks (0%)`

---

### 2.1 Redis Session Store

**Files:** `session_redis.go` (new), `session.go`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Design Redis key schema (sessions, groups, TTLs) | 🔴 CRITICAL | 🔴 TODO | 2 days | - |
| Implement `RedisSessionStore` struct with connection pool | 🔴 CRITICAL | 🔴 TODO | 3 days | - |
| Implement `Get/Set/Delete/List` methods | 🔴 CRITICAL | 🔴 TODO | 3 days | - |
| Add automatic TTL refresh on session access | 🔴 CRITICAL | 🔴 TODO | 2 days | - |
| Add Redis health check integration | 🟡 HIGH | 🔴 TODO | 1 day | - |
| Implement connection retry with exponential backoff | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Add serialization tests (ensure Store types are serializable) | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Document migration from MemorySessionStore | 🟡 MEDIUM | 🔴 TODO | 1 day | - |

**Acceptance Criteria:**
- Sessions survive instance restarts
- Users can reconnect to different instance
- Redis failures don't crash application
- TTL automatically extends on activity

**Redis Key Schema:**
```
livetemplate:session:{groupID}        -> JSON serialized Stores
livetemplate:session:{groupID}:access -> Last access timestamp
TTL: 24 hours (configurable)
```

**Example Usage:**
```go
import "github.com/redis/go-redis/v9"

redisClient := redis.NewClient(&redis.Options{
    Addr: os.Getenv("LVT_REDIS_URL"),
})

sessionStore := livetemplate.NewRedisSessionStore(redisClient,
    livetemplate.WithTTL(24*time.Hour),
)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
)
```

---

### 2.2 Distributed Pub/Sub for Broadcasting

**Files:** `pubsub/` (new package), `mount.go`, `registry.go`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Design pub/sub message format (action, groupID, userID, payload) | 🔴 CRITICAL | 🔴 TODO | 2 days | - |
| Implement `PubSubBroadcaster` using Redis Pub/Sub | 🔴 CRITICAL | 🔴 TODO | 4 days | - |
| Modify `Broadcast*` methods to publish messages | 🔴 CRITICAL | 🔴 TODO | 3 days | - |
| Implement subscriber goroutine to receive and fan-out | 🔴 CRITICAL | 🔴 TODO | 3 days | - |
| Add local-first optimization (skip Redis for same-instance) | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Handle Redis disconnection/reconnection gracefully | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Add broadcast latency metrics (p50, p95, p99) | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Document broadcast guarantees (at-most-once delivery) | 🟡 MEDIUM | 🔴 TODO | 1 day | - |

**Acceptance Criteria:**
- User with tabs on Instance A and B sees consistent state
- Broadcasts reach all instances within 50ms (p95)
- Local broadcasts don't hit Redis (performance optimization)
- Failed broadcasts logged, don't crash application

**Redis Channels:**
```
livetemplate:broadcast:group:{groupID}  -> Group-specific broadcasts
livetemplate:broadcast:user:{userID}    -> User-specific broadcasts
livetemplate:broadcast:global           -> Global broadcasts
```

**Message Format:**
```json
{
  "type": "broadcast",
  "groupID": "session-abc123",
  "userID": "user-xyz789",
  "excludeConnID": "conn-456",
  "payload": {
    "action": "updateCounter",
    "data": {...}
  },
  "timestamp": "2025-11-01T12:00:00Z",
  "instanceID": "host1"
}
```

---

### 2.3 Client Reconnection Improvements

**Files:** `client/transport/websocket.ts`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Implement exponential backoff (1s → 2s → 4s → 8s → 16s max) | 🔴 CRITICAL | 🔴 TODO | 1 day | - |
| Add jitter to prevent thundering herd | 🔴 CRITICAL | 🔴 TODO | 1 day | - |
| Add max retry limit (e.g., 10 attempts) | 🟡 HIGH | 🔴 TODO | 1 day | - |
| Emit reconnection events for UI feedback | 🟡 HIGH | 🔴 TODO | 1 day | - |
| Add session resume protocol (send last known state) | 🟡 MEDIUM | 🔴 TODO | 3 days | - |

**Acceptance Criteria:**
- Reconnection delay increases exponentially
- 1000 clients don't all reconnect at same instant
- UI shows "Reconnecting..." indicator
- Session state restored after reconnect

**Example:**
```typescript
// Exponential backoff with jitter
const delay = Math.min(
  1000 * Math.pow(2, attempt) + Math.random() * 1000,
  16000 // max 16s
);
```

---

### Milestone 2 Summary

**Total Tasks:** 10
**Completed:** 0
**In Progress:** 0
**TODO:** 10

**Estimated Effort:** 4-5 weeks with 1 engineer

**Critical Path:**
1. Redis Session Store (Week 1-2)
2. Distributed Pub/Sub (Week 3-4)
3. Client reconnection (Week 4-5)

**Dependencies:**
- Requires Milestone 1 (health checks, graceful shutdown)
- Requires Redis server (6.x or 7.x)

**Risks:**
- Redis Pub/Sub has no delivery guarantees (at-most-once)
- Large broadcast fan-out may increase latency
- Serialization of custom Store types may require user code changes

---

## Milestone 3: Enterprise Scale (MEDIUM PRIORITY)

**Goal:** Optimize LiveTemplate for **millions of concurrent connections** with advanced resilience patterns and performance optimizations.

**Success Criteria:**
- ✅ Single instance handles 50K+ connections
- ✅ Cluster handles 1M+ connections across 20+ instances
- ✅ Circuit breakers prevent cascading failures
- ✅ Multi-region deployments supported
- ✅ Sub-second recovery from instance failures

**Duration:** 8-12 weeks
**Progress:** `[░░░░░░░░░░] 0/14 tasks (0%)`

---

### 3.1 Advanced Connection Management

**Files:** `limits.go`, `mount.go`, `client/`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Implement WebSocket compression (permessage-deflate) | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Add connection pooling for database queries | 🟡 HIGH | 🔴 TODO | 2 days | - |
| Implement rate limiting per action type | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Add DDOS protection (per-IP rate limits) | 🟡 MEDIUM | 🔴 TODO | 3 days | - |
| Optimize goroutine usage (connection pooling) | 🟡 MEDIUM | 🔴 TODO | 5 days | - |

**Acceptance Criteria:**
- WebSocket bandwidth reduced 40-60% with compression
- Connection spikes don't exhaust database pool
- Per-action rate limits prevent abuse
- IP-based rate limiting blocks DDOS

---

### 3.2 Circuit Breakers & Resilience

**Files:** `resilience/` (new package)

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Implement circuit breaker for Redis operations | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Implement circuit breaker for database operations | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Add fallback to MemorySessionStore on Redis failure | 🟡 HIGH | 🔴 TODO | 3 days | - |
| Add retry with exponential backoff utility | 🟡 MEDIUM | 🔴 TODO | 2 days | - |
| Add bulkhead pattern for resource isolation | 🟡 MEDIUM | 🔴 TODO | 3 days | - |

**Acceptance Criteria:**
- Redis outage degrades gracefully (fallback to memory)
- Database timeout doesn't cascade to other connections
- Circuit breaker metrics exposed
- Automatic recovery when dependency healthy

---

### 3.3 Performance Optimization

**Files:** `template.go`, `tree.go`, `internal/build/`, `internal/diff/`

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Implement tree cache eviction (LRU policy) | 🟡 MEDIUM | 🔴 TODO | 3 days | - |
| Add tree depth/size validation (prevent memory bombs) | 🟡 MEDIUM | 🔴 TODO | 2 days | - |
| Optimize fingerprint calculation (faster hashing) | 🟡 MEDIUM | 🔴 TODO | 3 days | - |
| Add memory limits per connection (configurable) | 🟡 MEDIUM | 🔴 TODO | 2 days | - |
| Profile and optimize hot paths (CPU + memory) | 🟡 LOW | 🔴 TODO | 5 days | - |

**Acceptance Criteria:**
- Memory per connection reduced 30%
- Tree diffing 20% faster
- Connections bounded by memory limits
- Load tests show 50K+ connections per instance

---

### 3.4 Multi-Region Support

**Files:** `docs/MULTI_REGION.md` (new)

| Task | Priority | Status | Est. Effort | Assignee |
|------|----------|--------|-------------|----------|
| Document multi-region architecture patterns | 🟡 MEDIUM | 🔴 TODO | 3 days | - |
| Add region-aware load balancing guidance | 🟡 MEDIUM | 🔴 TODO | 2 days | - |
| Test Redis replication across regions | 🟡 LOW | 🔴 TODO | 3 days | - |
| Document CDN integration for static assets | 🟡 LOW | 🔴 TODO | 2 days | - |

**Acceptance Criteria:**
- Documentation covers active-active multi-region
- Redis replication tested and documented
- CDN integration reduces initial page load

---

### Milestone 3 Summary

**Total Tasks:** 14
**Completed:** 0
**In Progress:** 0
**TODO:** 14

**Estimated Effort:** 8-12 weeks with 1 engineer

**Critical Path:**
1. Connection optimization (Week 1-3)
2. Circuit breakers (Week 4-6)
3. Performance tuning (Week 7-10)
4. Multi-region (Week 11-12)

**Dependencies:**
- Requires Milestone 2 (distributed state)
- Requires load testing infrastructure

**Risks:**
- Performance optimizations may require breaking API changes
- Multi-region adds significant operational complexity

---

## Progress Tracking

### Overall Roadmap Progress

| Milestone | Tasks | Completed | Progress | Status |
|-----------|-------|-----------|----------|--------|
| **M1: Production Foundation** | 35 | 22 | `[████████████░░░░░░░░] 63%` | 🟡 IN PROGRESS |
| **M2: Horizontal Scaling** | 10 | 0 | `[░░░░░░░░░░] 0%` | 🔴 TODO |
| **M3: Enterprise Scale** | 14 | 0 | `[░░░░░░░░░░] 0%` | 🔴 TODO |
| **TOTAL** | **59** | **22** | `[███████░░░░░░░░░░░░░] 37%` | 🟡 IN PROGRESS |

### Key Metrics

| Metric | Current | M1 Target | M2 Target | M3 Target |
|--------|---------|-----------|-----------|-----------|
| Max Connections (single host) | ~4K | 10K | 20K | 50K+ |
| Instances Supported | 1 | 1-2 | 2-10 | 10-100+ |
| Deployment Downtime | Minutes | <30s | 0s | 0s |
| Redis Required | No | No | Yes | Yes |
| Health Check Endpoints | 0 | 2 | 2 | 2 |
| Prometheus Metrics | 0 | 15+ | 20+ | 30+ |
| Production Examples | 0 | 1 | 2 | 3 |

### Weekly Update Template

```markdown
## Week N Update (YYYY-MM-DD)

### Completed This Week
- [ ] Task 1
- [ ] Task 2

### In Progress
- [ ] Task 3 (50% complete)

### Blockers
- None / Description of blocker

### Next Week Goals
- [ ] Task 4
- [ ] Task 5

### Metrics
- Overall Progress: X/51 tasks (Y%)
- M1 Progress: X/18 tasks (Y%)
```

---

## Appendix A: Redis as Single External Dependency

### Why Redis?

Redis provides **three critical capabilities** needed for horizontal scaling:

1. **Distributed Session Store**
   - Fast key-value storage with TTL
   - Atomic operations for consistency
   - Persistence options (RDB, AOF)

2. **Pub/Sub for Broadcasting**
   - Low-latency message distribution
   - Topic-based routing (groups, users)
   - No need for dedicated message queue

3. **Future: Rate Limiting & Queues**
   - Token bucket rate limiting (INCR + EXPIRE)
   - Background job queues (Lists + BLPOP)
   - Distributed locks (SET NX EX)

### Why NOT Other Tools?

| Tool | Why Not? |
|------|----------|
| **Kafka** | Overkill for broadcast use case, complex ops |
| **RabbitMQ** | Adds second dependency, Redis Pub/Sub sufficient |
| **etcd/Consul** | Service discovery not needed (load balancer handles) |
| **Memcached** | No Pub/Sub, no persistence |
| **ZooKeeper** | Coordination not needed, complex ops |

### Redis Deployment Options

| Option | Pros | Cons | Use Case |
|--------|------|------|----------|
| **Standalone** | Simple, low cost | Single point of failure | Dev, small deployments |
| **Redis Sentinel** | Auto-failover, HA | More complex | Production (M2) |
| **Redis Cluster** | Sharding, massive scale | Most complex | Enterprise (M3) |
| **Managed (AWS ElastiCache, etc.)** | Zero ops, backups | Vendor lock-in, cost | Production preferred |

### Graceful Degradation Without Redis

When Redis is unavailable, LiveTemplate should:
- ✅ Fall back to `MemorySessionStore` (single-instance only)
- ✅ Disable distributed broadcasts (local-only)
- ✅ Log warnings about degraded mode
- ❌ Do NOT crash or refuse connections

**Configuration:**
```go
sessionStore := livetemplate.NewRedisSessionStore(redisClient,
    livetemplate.WithFallbackToMemory(true), // Enable graceful degradation
)
```

---

## Appendix B: Migration Guide (M1 → M2)

### Single-Host to Multi-Host Migration

**Before (M1):**
```go
// Single host, in-memory sessions
sessionStore := livetemplate.NewMemorySessionStore()
handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithMaxConnections(5000),
)
```

**After (M2):**
```go
// Multi-host, Redis sessions
redisClient := redis.NewClient(&redis.Options{
    Addr: os.Getenv("LVT_REDIS_URL"),
})

sessionStore := livetemplate.NewRedisSessionStore(redisClient)
broadcaster := livetemplate.NewRedisBroadcaster(redisClient)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithBroadcaster(broadcaster),
    livetemplate.WithMaxConnections(10000),
)
```

**Deployment Changes:**
1. Deploy Redis (Sentinel recommended)
2. Update environment variables (`LVT_REDIS_URL`)
3. Deploy application with new config
4. Configure load balancer with sticky sessions
5. Scale to 2+ instances

**Testing:**
1. Open browser tabs on different instances (check load balancer logs)
2. Trigger broadcast → verify both tabs update
3. Restart one instance → verify sessions persist

---

## Appendix C: Contributing to This Roadmap

### How to Update Progress

1. **Mark task as in-progress:**
   - Change status from 🔴 TODO to 🟡 IN PROGRESS
   - Add assignee name
   - Update milestone progress bar

2. **Mark task as complete:**
   - Change status to ✅ DONE
   - Update milestone progress: `(N+1)/Total tasks`
   - Recalculate progress bar

3. **Add new tasks:**
   - Add to appropriate milestone
   - Update "Total Tasks" count
   - Recalculate all progress percentages

### Progress Bar Format

```
[████████░░] X/Y tasks (Z%)
```

- Each █ represents 10% complete
- Update after every task completion

---

## References

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture
- [OBSERVABILITY.md](docs/OBSERVABILITY.md) - Logging and metrics
- [BROADCASTING.md](docs/BROADCASTING.md) - Broadcasting patterns
- [SCALING.md](docs/SCALING.md) - To be created in M1
- [REDIS_INTEGRATION.md](docs/REDIS_INTEGRATION.md) - To be created in M2

---

**Questions or feedback?** Open an issue or discussion on GitHub.
