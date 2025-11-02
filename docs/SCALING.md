# LiveTemplate Scaling Guide

**Target Audience:** DevOps engineers, SREs, and developers deploying LiveTemplate applications at scale.

**Last Updated:** 2025-11-01

---

## Overview

This guide covers scaling LiveTemplate applications from **single-host prototypes** to **production systems handling millions of concurrent WebSocket connections**.

---

## Scaling Tiers

### Tier 1: Single Host (Hobby)

**Capacity:** <1,000 concurrent connections
**Infrastructure:** 1 instance, no Redis
**Cost:** $5-20/month (VPS)

**Recommended For:**
- Personal projects
- Prototypes and MVPs
- Internal tools with <100 users
- Development and staging environments

**Configuration:**
```go
sessionStore := livetemplate.NewMemorySessionStore()
handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithMaxConnections(1000),
)
```

**Infrastructure:**
- 1 vCPU, 1-2 GB RAM
- SQLite or small Postgres/MySQL instance
- No load balancer needed

**Limitations:**
- Deployments cause downtime
- Sessions lost on restart
- No high availability

---

### Tier 2: Small Production (Startup)

**Capacity:** 1K-10K concurrent connections
**Infrastructure:** 1-2 instances, Redis optional
**Cost:** $50-200/month

**Recommended For:**
- Early-stage SaaS applications
- Small business tools
- 100-1000 active users

**Configuration:**
```go
// Option A: Still single-host with Redis for persistence
redisClient := redis.NewClient(&redis.Options{
    Addr: os.Getenv("LVT_REDIS_URL"),
})
sessionStore := livetemplate.NewRedisSessionStore(redisClient,
    livetemplate.WithFallbackToMemory(true),
)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithMaxConnections(5000),
)
```

**Infrastructure:**
- 2 vCPUs, 4 GB RAM per instance
- Redis Standalone (persistent)
- Managed Postgres/MySQL
- Optional: Simple load balancer

**Benefits Over Tier 1:**
- Sessions persist across restarts
- Near-zero downtime deploys possible
- Can scale to 2 instances if needed

---

### Tier 3: Production Scale (SaaS)

**Capacity:** 10K-100K concurrent connections
**Infrastructure:** 2-10 instances, Redis Sentinel
**Cost:** $500-2000/month

**Recommended For:**
- Production SaaS applications
- 1K-10K active users
- Mission-critical applications requiring HA

**Configuration:**
```go
// Multi-instance with Redis Sentinel for HA
redisClient := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{"sentinel1:26379", "sentinel2:26379"},
})

sessionStore := livetemplate.NewRedisSessionStore(redisClient)
broadcaster := livetemplate.NewRedisBroadcaster(redisClient)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithBroadcaster(broadcaster),
    livetemplate.WithMaxConnections(10000),
    livetemplate.WithMaxConnectionsPerGroup(500),
)
```

**Infrastructure:**
- 4-8 vCPUs, 8-16 GB RAM per instance
- Redis Sentinel (3 nodes for quorum)
- Managed database with connection pooling
- Load balancer with sticky sessions (cookie-based)
- Monitoring and alerting (Prometheus + Grafana)

**Deployment Pattern:**
- Kubernetes Deployment with 2-10 replicas
- HorizontalPodAutoscaler based on connection count
- Rolling updates with connection draining

**Key Metrics to Monitor:**
- `livetemplate_connections_active` per instance
- `livetemplate_connections_rejected_total` (backpressure)
- `livetemplate_action_duration_seconds` (p95, p99)
- Redis memory usage and latency

---

### Tier 4: Enterprise Scale

**Capacity:** 100K-1M+ concurrent connections
**Infrastructure:** 10-100+ instances, Redis Cluster
**Cost:** $5K-50K+/month

**Recommended For:**
- Large-scale platforms
- 10K+ active users
- Multi-region deployments
- Millions of messages per second

**Configuration:**
```go
// Redis Cluster for horizontal sharding
redisClient := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{
        "redis-node1:6379",
        "redis-node2:6379",
        "redis-node3:6379",
    },
})

sessionStore := livetemplate.NewRedisSessionStore(redisClient)
broadcaster := livetemplate.NewRedisBroadcaster(redisClient)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithBroadcaster(broadcaster),
    livetemplate.WithMaxConnections(50000),
    livetemplate.WithMaxConnectionsPerGroup(1000),
    livetemplate.WithWebSocketCompression(true), // M3 feature
)
```

**Infrastructure:**
- 8-32 vCPUs, 32-128 GB RAM per instance
- Redis Cluster (6+ nodes, sharded)
- Highly available database with read replicas
- CDN for static assets
- Multi-region deployment
- Advanced monitoring (distributed tracing, APM)

**Architecture Patterns:**
- Kubernetes with 10-100+ replicas
- HPA scales based on CPU + custom metrics
- Circuit breakers for all external dependencies
- Rate limiting per IP and per user
- WebSocket compression (40-60% bandwidth reduction)

---

## Capacity Planning

### Memory Estimation

**Per Connection:**
```
Conservative: 15-200 KB
Realistic (medium complexity): 50 KB
Optimized (M3): 30 KB
```

**Example Calculations:**

| Connections | Memory (Conservative) | Memory (Realistic) | Instances (16GB RAM) |
|-------------|----------------------|-------------------|---------------------|
| 1,000 | 200 MB | 50 MB | 1 |
| 10,000 | 2 GB | 500 MB | 1 |
| 50,000 | 10 GB | 2.5 GB | 1-2 |
| 100,000 | 20 GB | 5 GB | 2-4 |
| 1,000,000 | 200 GB | 50 GB | 10-20 |

**Add overhead for:**
- Operating system: 1-2 GB
- Database connections: 500 MB - 2 GB
- Redis client: 100-500 MB
- Application code: 500 MB - 1 GB
- Buffer for spikes: 20-30%

### Connection Distribution

**Rule of Thumb:**
- Keep instances at 60-70% capacity for headroom
- Example: 16 GB instance → 10 GB for connections → ~200K connections (realistic)
- Target: 120-140K connections per instance in production

**Load Balancer Strategy:**
- Use **sticky sessions** (cookie-based affinity)
- Cookie name: `LVT_SESSION_ID` or `SERVERID`
- Cookie TTL: Match session TTL (24 hours default)
- Fallback: Least-connections algorithm

---

## Scaling Checklist

### Before Scaling to Tier 2 (Redis + 2 Instances)

- [ ] Set up Redis (Standalone with persistence enabled)
- [ ] Update application to use `RedisSessionStore`
- [ ] Add health check endpoints (`/health/live`, `/health/ready`)
- [ ] Configure load balancer with sticky sessions
- [ ] Set up Prometheus metrics and Grafana dashboards
- [ ] Test session persistence (restart instance, verify session survives)
- [ ] Document deployment procedure

### Before Scaling to Tier 3 (Production HA)

- [ ] Deploy Redis Sentinel (3 nodes minimum)
- [ ] Add `RedisBroadcaster` for cross-instance broadcasts
- [ ] Configure Kubernetes with 2+ replicas
- [ ] Set up HorizontalPodAutoscaler
- [ ] Configure graceful shutdown (connection draining)
- [ ] Test rolling updates (zero downtime)
- [ ] Set up alerting (connection limits, error rates)
- [ ] Load test with realistic traffic (3x peak expected)
- [ ] Document runbook for common incidents

### Before Scaling to Tier 4 (Enterprise)

- [ ] Deploy Redis Cluster (6+ nodes)
- [ ] Enable WebSocket compression
- [ ] Add circuit breakers for external dependencies
- [ ] Implement rate limiting (per IP, per user)
- [ ] Set up distributed tracing
- [ ] Deploy to multiple regions (if required)
- [ ] Chaos engineering tests (kill random instances)
- [ ] Capacity plan for 5x current traffic
- [ ] Disaster recovery tested and documented

---

## Performance Benchmarks

### Single Instance (16 GB RAM, 8 vCPUs)

| Metric | M1 | M2 | M3 |
|--------|----|----|-----|
| Max Connections | 10K | 20K | 50K+ |
| Action Latency (p95) | <100ms | <50ms | <20ms |
| Broadcast Latency (p95) | <50ms | <100ms | <50ms |
| Memory per Connection | 100 KB | 70 KB | 30 KB |
| Goroutines per Connection | 1 | 1 | 0.5 |

### Multi-Instance (10 instances)

| Metric | M2 | M3 |
|--------|----|----|
| Total Connections | 200K | 500K+ |
| Broadcast Fan-out Time (10K users) | 200ms | 100ms |
| Session Lookup Latency (Redis) | <5ms | <2ms |
| Cross-Instance Broadcast Latency | <100ms | <50ms |

**Note:** Benchmarks are approximate and depend on hardware, network, and workload characteristics.

---

## Common Scaling Issues

### Issue: Connection Limit Reached

**Symptoms:**
- New connections rejected with 503 errors
- Metric: `livetemplate_connections_rejected_total` increasing

**Solutions:**
1. **Horizontal scale:** Add more instances
2. **Vertical scale:** Increase instance RAM
3. **Optimize:** Review connection lifecycle, reduce memory per connection
4. **Limit:** Set `MaxConnectionsPerGroup` to prevent single-user exhaustion

### Issue: High Broadcast Latency

**Symptoms:**
- Broadcasts take >500ms to reach all clients
- Users report stale data

**Solutions:**
1. **Redis latency:** Check `redis-cli --latency` and network latency
2. **Fan-out size:** Limit group sizes or shard groups
3. **Local optimization:** Ensure local broadcasts skip Redis (M2 feature)
4. **Compression:** Enable WebSocket compression (M3)

### Issue: Sessions Not Persisting

**Symptoms:**
- Users lose session on instance restart
- Sessions disappear after 24 hours

**Solutions:**
1. **Check Redis:** Verify Redis persistence (RDB/AOF) enabled
2. **Check TTL:** Ensure session TTL configured correctly
3. **Check serialization:** Verify custom Store types are serializable
4. **Fallback:** Ensure `WithFallbackToMemory` not masking issues

### Issue: Uneven Load Distribution

**Symptoms:**
- One instance at 90% CPU, others at 20%
- Load balancer not distributing evenly

**Solutions:**
1. **Sticky sessions:** Verify cookie-based affinity working
2. **Long-lived connections:** WebSockets can cause imbalance over time
3. **Rebalancing:** Implement periodic connection migration (M3 feature)
4. **Algorithm:** Try least-connections instead of round-robin

---

## Monitoring and Alerting

### Critical Metrics to Monitor

**Connection Health:**
```
livetemplate_connections_active{instance="host1"} > 8000  # 80% of 10K limit
livetemplate_connections_rejected_total > 100
```

**Performance:**
```
livetemplate_action_duration_seconds{quantile="0.95"} > 0.200  # 200ms
livetemplate_broadcasts_sent_total rate(5m) > 10000  # High broadcast rate
```

**Resource Usage:**
```
process_resident_memory_bytes > 13e9  # 13 GB of 16 GB
redis_connected_clients{instance="redis1"} > 9000  # 90% of Redis max clients
```

### Recommended Alerts

**Critical (page on-call):**
- Instances failing health checks (>1 min)
- Redis unavailable (>30s)
- Connection reject rate >100/min
- Error rate >1% (>100 errors/min)

**Warning (Slack notification):**
- Connection count >80% of limit
- Action latency p95 >200ms
- Memory usage >80%
- Redis replication lag >5s

**Info (metrics only):**
- Connection count trends
- Broadcast distribution
- Session count by group

---

## Next Steps

- **Redis Setup:** See [REDIS_INTEGRATION.md](REDIS_INTEGRATION.md) for Redis configuration
- **Roadmap:** See [ROADMAP.md](ROADMAP.md) for upcoming scaling features
- **Architecture:** See [ARCHITECTURE.md](ARCHITECTURE.md) for system design

---

**Questions?** Open an issue on GitHub or join the discussion.
