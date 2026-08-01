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
    Addr: os.Getenv("REDIS_URL"),
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
pubsubBroadcaster := livetemplate.NewRedisBroadcaster(redisClient)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithPubSubBroadcaster(pubsubBroadcaster),
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
pubsubBroadcaster := livetemplate.NewRedisBroadcaster(redisClient)

handler := livetemplate.Mount(rootStore,
    livetemplate.WithSessionStore(sessionStore),
    livetemplate.WithPubSubBroadcaster(pubsubBroadcaster),
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

## Migration Guide: Memory to Redis Session Store

This guide walks through migrating from in-memory session storage to Redis-backed storage for horizontal scaling.

### When to Migrate

**Stay with MemorySessionStore if:**
- Single instance deployment (no horizontal scaling needed)
- <1,000 concurrent connections
- Session loss on restart is acceptable
- Development/staging environments
- Cost is primary concern

**Migrate to RedisSessionStore when:**
- Scaling beyond single instance (horizontal scaling)
- Need session persistence across restarts
- Zero-downtime deployments required
- >1,000 concurrent connections expected
- Multi-region or multi-AZ deployments

### Prerequisites

1. **Redis Server**: Deploy Redis (Standalone, Sentinel, or Cluster)
2. **Go Redis Client**: Install `github.com/redis/go-redis/v9`
3. **State Serialization**: Ensure all State types are gob-serializable

### Step-by-Step Migration

#### Step 1: Set Up Redis

**Development (Docker):**
```bash
docker run -d \
  --name livetemplate-redis \
  -p 6379:6379 \
  redis:7-alpine \
  redis-server --appendonly yes
```

**Production (Managed Service):**
- AWS ElastiCache (Redis)
- Google Cloud Memorystore
- Azure Cache for Redis
- Redis Cloud

#### Step 2: Register State Types for Serialization

LiveTemplate uses Go's `encoding/gob` for serialization, which requires registering custom types.

**Before (works with MemorySessionStore):**
```go
// State holds data (cloned per session)
type TodoState struct {
    Items []Todo
}

// Controller holds dependencies (singleton)
type TodoController struct {
    DB *sql.DB
}

// Action method
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    todo := Todo{Title: ctx.GetString("title")}
    state.Items = append(state.Items, todo)
    return state, nil
}
```

**After (required for RedisSessionStore):**
```go
type TodoState struct {
    Items []Todo
}

type TodoController struct {
    DB *sql.DB
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    todo := Todo{Title: ctx.GetString("title")}
    state.Items = append(state.Items, todo)
    return state, nil
}

// Register all State types in init()
func init() {
    gob.Register(&TodoState{})
    gob.Register(&Todo{})  // Register nested types too
}
```

**Why?** Gob encoding preserves type information only for registered types. Without registration, deserialization fails.

#### Step 3: Update Application Code

**Before (MemorySessionStore):**
```go
package main

import "github.com/livetemplate/livetemplate"

func main() {
    // In-memory session store (default)
    sessionStore := livetemplate.NewMemorySessionStore()

    controller := &AppController{}
    state := &AppState{}
    handler := livetemplate.Mount(controller, livetemplate.AsState(state),
        livetemplate.WithSessionStore(sessionStore),
        livetemplate.WithMaxConnections(1000),
    )

    http.Handle("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

**After (RedisSessionStore):**
```go
package main

import (
    "github.com/livetemplate/livetemplate"
    "github.com/redis/go-redis/v9"
    "log"
    "os"
)

func main() {
    // Connect to Redis
    redisClient := redis.NewClient(&redis.Options{
        Addr:     os.Getenv("REDIS_URL"), // e.g., "localhost:6379"
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       0,
    })

    // Verify Redis connection
    if err := redisClient.Ping(context.Background()).Err(); err != nil {
        log.Fatalf("Failed to connect to Redis: %v", err)
    }

    // Create Redis session store with fallback
    sessionStore := livetemplate.NewRedisSessionStore(redisClient,
        livetemplate.WithSessionTTL(24*time.Hour),
        livetemplate.WithFallbackToMemory(true), // Graceful degradation
    )

    controller := &AppController{}
    state := &AppState{}
    handler := livetemplate.Mount(controller, livetemplate.AsState(state),
        livetemplate.WithSessionStore(sessionStore),
        livetemplate.WithMaxConnections(10000), // Can handle more now
    )

    http.Handle("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

#### Step 4: Configure Environment Variables

**Development (.env):**
```bash
REDIS_URL=localhost:6379
REDIS_PASSWORD=
```

**Production (Kubernetes Secret):**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: redis-credentials
type: Opaque
stringData:
  redis-url: "redis.production.svc.cluster.local:6379"
  redis-password: "your-secure-password"
```

**Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: redis-credentials
              key: redis-url
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-credentials
              key: redis-password
```

#### Step 5: Update Health Checks

Add Redis health check to ensure instance is ready before accepting traffic.

```go
import "github.com/livetemplate/livetemplate"

func main() {
    // ... Redis setup ...

    sessionStore := livetemplate.NewRedisSessionStore(redisClient)

    // Health check endpoints
    http.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
        // Check Redis connectivity
        if err := sessionStore.Ping(); err != nil {
            http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("READY"))
    })

    http.Handle("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

**Kubernetes Probe Configuration:**

Configure liveness and readiness probes to ensure Kubernetes can properly manage your application lifecycle.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: livetemplate-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: livetemplate
  template:
    metadata:
      labels:
        app: livetemplate
    spec:
      containers:
      - name: app
        image: your-registry/livetemplate-app:latest
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: redis-credentials
              key: redis-url

        # Liveness Probe: Is the application running?
        # Failure = Restart container
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 10   # Wait 10s after container starts
          periodSeconds: 30          # Check every 30s
          timeoutSeconds: 5          # Request timeout
          successThreshold: 1        # 1 success = healthy
          failureThreshold: 3        # 3 failures = restart (90s total)

        # Readiness Probe: Can the application accept traffic?
        # Failure = Remove from service endpoints
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 5     # Start checking after 5s
          periodSeconds: 10          # Check every 10s
          timeoutSeconds: 5          # Request timeout
          successThreshold: 1        # 1 success = ready
          failureThreshold: 2        # 2 failures = not ready (20s total)

        # Startup Probe: Has the application finished starting?
        # Use for slow-starting applications
        startupProbe:
          httpGet:
            path: /health/live
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 0     # Start immediately
          periodSeconds: 5           # Check every 5s
          timeoutSeconds: 3          # Request timeout
          successThreshold: 1        # 1 success = started
          failureThreshold: 30       # 30 failures = give up (150s total)

        resources:
          requests:
            memory: "4Gi"
            cpu: "2000m"
          limits:
            memory: "8Gi"
            cpu: "4000m"

        # Graceful shutdown: allow connections to drain
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 15"]
```

**Probe Configuration Guidelines:**

| Probe Type | Purpose | Failure Action | Recommended Settings |
|------------|---------|----------------|---------------------|
| **Liveness** | Detect deadlocks, hung processes | Restart container | `periodSeconds: 30`, `failureThreshold: 3` |
| **Readiness** | Detect temporary unavailability (Redis down, DB issues) | Remove from load balancer | `periodSeconds: 10`, `failureThreshold: 2` |
| **Startup** | Handle slow application startup | Delay liveness checks | `periodSeconds: 5`, `failureThreshold: 30` |

**When to Use Each Probe:**

1. **Liveness Probe** (`/health/live`):
   - **Always use** for all deployments
   - Should check only if application process is responsive
   - Do NOT check external dependencies (Redis, DB)
   - Fast check (<100ms response time)

2. **Readiness Probe** (`/health/ready`):
   - **Always use** for all deployments
   - Should check external dependencies (Redis, DB)
   - Allows application to temporarily become "not ready" without restart
   - Example: Redis connection lost → readiness fails → no new connections → Redis recovers → readiness passes → traffic resumes

3. **Startup Probe** (`/health/live`):
   - **Use if** application takes >30s to start (database migrations, cache warming)
   - **Skip if** application starts quickly (<10s)
   - Prevents liveness probe from restarting slow-starting apps

**Health Check Implementation Best Practices:**

```go
func setupHealthChecks(sessionStore *livetemplate.RedisSessionStore, db *sql.DB) {
    // Liveness: Just check if HTTP server is responding
    // Do NOT check external dependencies
    http.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    // Readiness: Check all critical dependencies
    http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
        defer cancel()

        // Check Redis
        if err := sessionStore.Ping(); err != nil {
            log.Printf("Readiness: Redis unhealthy: %v", err)
            http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
            return
        }

        // Check database (optional, if using database)
        if db != nil {
            if err := db.PingContext(ctx); err != nil {
                log.Printf("Readiness: Database unhealthy: %v", err)
                http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
                return
            }
        }

        // All checks passed
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("READY"))
    })

    // Optional: Detailed health check for monitoring (not for k8s probes)
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        health := struct {
            Status      string            `json:"status"`
            Checks      map[string]string `json:"checks"`
            Timestamp   time.Time         `json:"timestamp"`
            Connections int               `json:"active_connections"`
        }{
            Status:    "healthy",
            Checks:    make(map[string]string),
            Timestamp: time.Now(),
        }

        // Check Redis
        if err := sessionStore.Ping(); err != nil {
            health.Status = "unhealthy"
            health.Checks["redis"] = fmt.Sprintf("error: %v", err)
        } else {
            health.Checks["redis"] = "ok"
        }

        // Check database
        if db != nil {
            if err := db.Ping(); err != nil {
                health.Status = "unhealthy"
                health.Checks["database"] = fmt.Sprintf("error: %v", err)
            } else {
                health.Checks["database"] = "ok"
            }
        }

        // Return JSON response
        w.Header().Set("Content-Type", "application/json")
        if health.Status != "healthy" {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        json.NewEncoder(w).Encode(health)
    })
}
```

**Advanced Database Health Checks:**

For production deployments with databases, implement comprehensive health checks that verify not just connectivity, but also connection pool health and query performance.

```go
package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"

    _ "github.com/lib/pq" // PostgreSQL driver
)

type DatabaseHealthChecker struct {
    db      *sql.DB
    timeout time.Duration
}

func NewDatabaseHealthChecker(db *sql.DB) *DatabaseHealthChecker {
    return &DatabaseHealthChecker{
        db:      db,
        timeout: 3 * time.Second,
    }
}

// Check performs comprehensive database health check
func (d *DatabaseHealthChecker) Check(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, d.timeout)
    defer cancel()

    // 1. Ping: Verify basic connectivity
    if err := d.db.PingContext(ctx); err != nil {
        return fmt.Errorf("ping failed: %w", err)
    }

    // 2. Simple query: Verify database is responsive
    var result int
    if err := d.db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
        return fmt.Errorf("query failed: %w", err)
    }

    return nil
}

// Stats returns database connection pool statistics
func (d *DatabaseHealthChecker) Stats() sql.DBStats {
    return d.db.Stats()
}

// Detailed health check endpoint with database metrics
func setupDatabaseHealthCheck(db *sql.DB, sessionStore *livetemplate.RedisSessionStore) {
    dbChecker := NewDatabaseHealthChecker(db)

    // Simple readiness check for Kubernetes
    http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        // Check Redis
        if err := sessionStore.Ping(); err != nil {
            log.Printf("Readiness: Redis unhealthy: %v", err)
            http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
            return
        }

        // Check database
        if err := dbChecker.Check(ctx); err != nil {
            log.Printf("Readiness: Database unhealthy: %v", err)
            http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
            return
        }

        w.WriteHeader(http.StatusOK)
        w.Write([]byte("READY"))
    })

    // Detailed health check with metrics
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        health := struct {
            Status    string                 `json:"status"`
            Checks    map[string]interface{} `json:"checks"`
            Timestamp time.Time              `json:"timestamp"`
        }{
            Status:    "healthy",
            Checks:    make(map[string]interface{}),
            Timestamp: time.Now(),
        }

        // Redis health
        if err := sessionStore.Ping(); err != nil {
            health.Status = "unhealthy"
            health.Checks["redis"] = map[string]interface{}{
                "status": "error",
                "error":  err.Error(),
            }
        } else {
            health.Checks["redis"] = map[string]interface{}{
                "status": "ok",
            }
        }

        // Database health with detailed metrics
        dbHealth := map[string]interface{}{
            "status": "ok",
        }

        if err := dbChecker.Check(ctx); err != nil {
            health.Status = "unhealthy"
            dbHealth["status"] = "error"
            dbHealth["error"] = err.Error()
        } else {
            // Add connection pool statistics
            stats := dbChecker.Stats()
            dbHealth["connection_pool"] = map[string]interface{}{
                "open_connections":  stats.OpenConnections,
                "in_use":            stats.InUse,
                "idle":              stats.Idle,
                "max_open":          stats.MaxOpenConnections,
                "wait_count":        stats.WaitCount,
                "wait_duration_ms":  stats.WaitDuration.Milliseconds(),
                "max_idle_closed":   stats.MaxIdleClosed,
                "max_idle_time_closed": stats.MaxIdleTimeClosed,
                "max_lifetime_closed":  stats.MaxLifetimeClosed,
            }

            // Calculate pool utilization
            utilization := float64(0)
            if stats.MaxOpenConnections > 0 {
                utilization = float64(stats.OpenConnections) / float64(stats.MaxOpenConnections) * 100
            }
            dbHealth["pool_utilization_percent"] = utilization

            // Warn if pool is >80% utilized
            if utilization > 80 {
                dbHealth["warning"] = "connection pool utilization high"
            }
        }

        health.Checks["database"] = dbHealth

        // Return response
        w.Header().Set("Content-Type", "application/json")
        if health.Status != "healthy" {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        json.NewEncoder(w).Encode(health)
    })
}

// Configure database connection pool for production
func configureDatabasePool(db *sql.DB) {
    // Maximum number of open connections
    // Rule of thumb: (CPU cores × 2) + disk spindles
    // Example: 8 cores + 2 disks = 18 connections
    db.SetMaxOpenConns(25)

    // Maximum number of idle connections in pool
    // Should be same as MaxOpenConns for consistent performance
    db.SetMaxIdleConns(25)

    // Maximum lifetime of a connection
    // Helps with connection refresh and load balancer rotation
    db.SetConnMaxLifetime(5 * time.Minute)

    // Maximum idle time for a connection
    // Connections idle longer than this are closed
    db.SetConnMaxIdleTime(1 * time.Minute)
}

// Example main function with database health checks
func main() {
    // Setup database
    db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    // Configure connection pool
    configureDatabasePool(db)

    // Verify database is reachable on startup
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        log.Fatalf("Failed to ping database: %v", err)
    }
    log.Println("Database connection established")

    // Setup Redis session store
    redisClient := redis.NewClient(&redis.Options{
        Addr: os.Getenv("REDIS_URL"),
    })
    sessionStore := livetemplate.NewRedisSessionStore(redisClient)

    // Setup health checks
    setupDatabaseHealthCheck(db, sessionStore)

    // ... rest of application setup
}
```

**Database Connection Pool Best Practices:**

1. **Set Appropriate Connection Limits:**
   ```go
   // Too low: Queries queue, high latency
   db.SetMaxOpenConns(5)  // DON'T: Too few for production

   // Too high: Resource exhaustion, database overload
   db.SetMaxOpenConns(1000)  // DON'T: Way too many

   // Just right: Based on workload and database capacity
   db.SetMaxOpenConns(25)  // DO: Reasonable for most apps
   ```

   **Formula:**
   ```
   MaxOpenConns = (CPU cores × 2) + disk spindles

   Examples:
   - 8 core server + SSD: 8×2 + 1 = 17 ≈ 20 connections
   - 16 core server + RAID: 16×2 + 4 = 36 ≈ 40 connections
   ```

2. **Match Idle and Max Connections:**
   ```go
   // Inconsistent: Idle connections close/reopen frequently
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(5)   // DON'T: Creates connection churn

   // Consistent: Connections stay open and ready
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(25)  // DO: No connection churn
   ```

3. **Set Connection Lifetimes:**
   ```go
   // Infinite lifetime: Stale connections, load balancer issues
   // (default: no limit)

   // Reasonable lifetime: Fresh connections, LB-friendly
   db.SetConnMaxLifetime(5 * time.Minute)     // DO: Rotate connections
   db.SetConnMaxIdleTime(1 * time.Minute)     // DO: Close idle connections
   ```

4. **Monitor Connection Pool Metrics:**
   ```go
   // Log pool stats periodically
   go func() {
       ticker := time.NewTicker(30 * time.Second)
       for range ticker.C {
           stats := db.Stats()
           log.Printf("DB Pool: open=%d in_use=%d idle=%d wait_count=%d",
               stats.OpenConnections, stats.InUse, stats.Idle, stats.WaitCount)

           // Alert if pool is starved
           if stats.WaitCount > 100 {
               log.Printf("WARNING: High connection wait count: %d", stats.WaitCount)
           }
       }
   }()
   ```

**Prometheus Metrics for Database Health:**

```go
import (
    "database/sql"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dbConnectionsOpen = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "db_connections_open",
        Help: "Number of open database connections",
    })

    dbConnectionsInUse = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "db_connections_in_use",
        Help: "Number of database connections currently in use",
    })

    dbConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "db_connections_idle",
        Help: "Number of idle database connections",
    })

    dbConnectionWaitCount = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "db_connection_wait_count_total",
        Help: "Total number of times a connection was waited for",
    })

    dbConnectionWaitDuration = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "db_connection_wait_duration_seconds",
        Help: "Total time blocked waiting for connections",
    })
)

// Export database pool metrics to Prometheus
func exportDatabaseMetrics(db *sql.DB) {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for range ticker.C {
            stats := db.Stats()
            dbConnectionsOpen.Set(float64(stats.OpenConnections))
            dbConnectionsInUse.Set(float64(stats.InUse))
            dbConnectionsIdle.Set(float64(stats.Idle))
            dbConnectionWaitCount.Set(float64(stats.WaitCount))
            dbConnectionWaitDuration.Set(stats.WaitDuration.Seconds())
        }
    }()
}
```

**Alerting Rules:**

```yaml
# Prometheus alerting rules for database health
groups:
- name: database_health
  rules:
  # Database connectivity
  - alert: DatabaseDown
    expr: up{job="database"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database is down"

  # Connection pool exhaustion
  - alert: DatabasePoolExhausted
    expr: (db_connections_in_use / db_connections_open) > 0.9
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Database connection pool >90% utilized"

  # High wait count (connection starvation)
  - alert: DatabaseConnectionStarvation
    expr: rate(db_connection_wait_count_total[5m]) > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High database connection wait rate"

  # Slow queries
  - alert: DatabaseSlowQueries
    expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{handler="/health"}[5m])) > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Health check queries are slow (p95 > 1s)"
```

**Testing Database Health Checks:**

```bash
# Test health endpoint locally
curl -v http://localhost:8080/health | jq .

# Expected response:
{
  "status": "healthy",
  "checks": {
    "database": {
      "status": "ok",
      "connection_pool": {
        "open_connections": 10,
        "in_use": 2,
        "idle": 8,
        "max_open": 25,
        "wait_count": 0,
        "pool_utilization_percent": 40
      }
    },
    "redis": {
      "status": "ok"
    }
  },
  "timestamp": "2025-11-02T10:30:00Z"
}

# Simulate database failure (kill database container)
docker stop postgres-db

# Health check should fail
curl -v http://localhost:8080/health/ready
# Expected: HTTP 503 Service Unavailable

# Kubernetes should remove pod from service
kubectl get pods
# READY column shows 0/1

# Restore database
docker start postgres-db

# Health check should recover
curl -v http://localhost:8080/health/ready
# Expected: HTTP 200 OK
```

**Common Database Health Check Mistakes:**

❌ **DON'T: Use complex queries in health checks**
```go
// WRONG: Slow, locks tables
_, err := db.Query("SELECT * FROM users WHERE status = 'active' ORDER BY created_at DESC LIMIT 1000")
```

✅ **DO: Use simple, fast queries**
```go
// CORRECT: Fast, no locks
var result int
err := db.QueryRow("SELECT 1").Scan(&result)
```

❌ **DON'T: Ignore connection pool exhaustion**
```go
// WRONG: Health check passes but app is slow
if err := db.Ping(); err != nil {
    return err
}
// Missing: Check if pool is exhausted (high wait count)
```

✅ **DO: Check both connectivity and pool health**
```go
// CORRECT: Verify connectivity AND pool capacity
if err := db.Ping(); err != nil {
    return err
}
stats := db.Stats()
if stats.WaitCount > 100 {
    return fmt.Errorf("connection pool exhausted: wait_count=%d", stats.WaitCount)
}
```

❌ **DON'T: Set unlimited connection pool**
```go
// WRONG: Can exhaust database resources
db.SetMaxOpenConns(0)  // 0 = unlimited
```

✅ **DO: Set explicit, reasonable limits**
```go
// CORRECT: Explicit limit based on capacity
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(25)
```

**Common Probe Configuration Mistakes:**

❌ **DON'T: Check external dependencies in liveness probe**
```yaml
livenessProbe:
  httpGet:
    path: /health/ready  # WRONG: Checks Redis, DB
```
**Why:** If Redis is down temporarily, liveness fails → container restarts → Redis still down → restart loop

✅ **DO: Check only process health in liveness probe**
```yaml
livenessProbe:
  httpGet:
    path: /health/live  # CORRECT: Only checks if process responds
```

❌ **DON'T: Set aggressive failure thresholds**
```yaml
readinessProbe:
  periodSeconds: 5
  failureThreshold: 1  # WRONG: 1 failure = immediately removed
```
**Why:** Temporary network blip → immediately removed from load balancer → unnecessary disruption

✅ **DO: Allow for temporary failures**
```yaml
readinessProbe:
  periodSeconds: 10
  failureThreshold: 2  # CORRECT: 2 consecutive failures (20s) before removal
```

❌ **DON'T: Forget graceful shutdown**
```yaml
# No preStop hook = immediate termination
```
**Why:** WebSocket connections get abruptly closed → bad user experience

✅ **DO: Drain connections before shutdown**
```yaml
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 15"]  # Give connections time to close
```

**Testing Health Checks:**

```bash
# Test liveness probe locally
curl -v http://localhost:8080/health/live
# Expected: HTTP 200 OK

# Test readiness probe locally
curl -v http://localhost:8080/health/ready
# Expected: HTTP 200 OK (if Redis is up)
# Expected: HTTP 503 Service Unavailable (if Redis is down)

# Test in Kubernetes
kubectl get pods
# Check "READY" column: should show 1/1

kubectl describe pod livetemplate-app-xxx
# Check "Conditions" section for probe failures

# Simulate Redis failure
kubectl exec -it redis-0 -- redis-cli shutdown
# Watch readiness probe fail
kubectl get pods -w
# Should see READY change from 1/1 to 0/1

# Restore Redis
kubectl rollout restart statefulset/redis
# Watch readiness probe recover
# Should see READY change from 0/1 to 1/1
```

**Monitoring Probe Health:**

Query Kubernetes events to detect probe failures:
```bash
# Recent probe failures
kubectl get events --field-selector reason=Unhealthy

# Probe failures for specific pod
kubectl describe pod livetemplate-app-xxx | grep -A 5 "Liveness\|Readiness"
```

Prometheus metrics for probe failures:
```promql
# Liveness probe failures (container restarts)
rate(kube_pod_container_status_restarts_total{pod=~"livetemplate-app-.*"}[5m]) > 0

# Readiness probe failures (not ready)
kube_pod_status_ready{pod=~"livetemplate-app-.*", condition="false"} == 1
```

#### Step 6: Test the Migration

**Local Testing:**
```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Run application
REDIS_URL=localhost:6379 go run main.go

# Test session persistence
curl -c cookies.txt http://localhost:8080/
# Restart application
pkill -9 main && REDIS_URL=localhost:6379 go run main.go &
# Verify session persisted
curl -b cookies.txt http://localhost:8080/
```

**Integration Test:**
```go
func TestRedisSessionPersistence(t *testing.T) {
    // Setup Redis and handler
    redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    sessionStore := livetemplate.NewRedisSessionStore(redisClient)

    controller := &TestController{}
    state := &TestState{Value: 0}
    handler := livetemplate.Mount(controller, livetemplate.AsState(state),
        livetemplate.WithSessionStore(sessionStore),
    )

    // Create session
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/", nil)
    handler.ServeHTTP(w, r)

    // Extract session cookie
    cookies := w.Result().Cookies()
    sessionCookie := cookies[0]

    // Simulate restart by creating new handler
    handler2 := livetemplate.Mount(controller, livetemplate.AsState(&TestState{Value: 0}),
        livetemplate.WithSessionStore(sessionStore),
    )

    // Verify session persisted
    w2 := httptest.NewRecorder()
    r2 := httptest.NewRequest("GET", "/", nil)
    r2.AddCookie(sessionCookie)
    handler2.ServeHTTP(w2, r2)

    // Session should exist (no new session created)
    assert.Equal(t, sessionCookie.Value, w2.Result().Cookies()[0].Value)
}
```

#### Step 7: Deploy to Production

**Deployment Strategy:**

1. **Blue-Green Deployment** (Recommended for first migration):
   ```bash
   # Deploy new version with Redis to "green" environment
   kubectl apply -f deployment-green.yaml

   # Verify health checks pass
   kubectl get pods -l version=green

   # Switch traffic to green
   kubectl patch service app -p '{"spec":{"selector":{"version":"green"}}}'

   # Monitor for 24 hours (session TTL)

   # Decommission blue environment
   kubectl delete -f deployment-blue.yaml
   ```

2. **Rolling Update** (For subsequent deployments):
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   spec:
     replicas: 3
     strategy:
       type: RollingUpdate
       rollingUpdate:
         maxSurge: 1
         maxUnavailable: 0  # Zero downtime
   ```

### Migration Checklist

- [ ] Redis deployed and accessible from application
- [ ] All State types registered with `gob.Register()`
- [ ] Environment variables configured (REDIS_URL, REDIS_PASSWORD)
- [ ] Health checks updated to verify Redis connectivity
- [ ] RedisSessionStore configured with appropriate TTL
- [ ] Fallback to memory enabled for graceful degradation
- [ ] Local testing completed (session persistence verified)
- [ ] Integration tests passing
- [ ] Deployment strategy chosen (blue-green or rolling)
- [ ] Monitoring configured (Redis metrics, session counts)
- [ ] Rollback plan documented
- [ ] Session migration window communicated to users (if needed)

### Common Migration Issues

#### Issue: "gob: name not registered for interface type"

**Cause:** State type not registered with gob.

**Solution:**
```go
func init() {
    gob.Register(&YourStateType{})
}
```

#### Issue: "Sessions lost after migration"

**Cause:** MemorySessionStore sessions cannot be migrated to Redis.

**Solution:** Sessions will be recreated on next user visit. For critical sessions:
1. Set migration window during low-traffic period
2. Export sessions before migration: `sessionStore.List()`
3. Import to Redis after migration

#### Issue: "Redis connection timeout in production"

**Cause:** Network policy blocking Redis access.

**Solution:**
```yaml
# Kubernetes NetworkPolicy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-redis
spec:
  podSelector:
    matchLabels:
      app: livetemplate
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
```

#### Issue: "High Redis memory usage"

**Cause:** Sessions not expiring (TTL issue).

**Solution:**
```go
// Set appropriate TTL
sessionStore := livetemplate.NewRedisSessionStore(redisClient,
    livetemplate.WithSessionTTL(24*time.Hour),
)

// Monitor Redis memory
redis-cli INFO memory
```

### Performance Comparison

| Metric | MemorySessionStore | RedisSessionStore (Local) | RedisSessionStore (Remote) |
|--------|-------------------|--------------------------|---------------------------|
| Get Latency (p50) | <1μs | ~500μs | 1-5ms |
| Get Latency (p99) | <10μs | ~1ms | 5-20ms |
| Set Latency (p50) | <1μs | ~500μs | 1-5ms |
| Set Latency (p99) | <10μs | ~1ms | 5-20ms |
| Memory Overhead | None | Serialization | Serialization + Network |
| Persistence | No | Yes (RDB/AOF) | Yes (RDB/AOF) |
| Horizontal Scaling | No | Yes | Yes |

**Note:** Latencies are approximate and depend on network, Redis configuration, and data size.

### Rollback Plan

If issues arise after migration:

1. **Immediate rollback** (if within deployment window):
   ```bash
   kubectl rollout undo deployment/app
   ```

2. **Graceful rollback** (after deployment complete):
   - Deploy old version with MemorySessionStore
   - Users will lose sessions (expected behavior)
   - Communicate downtime if necessary

3. **Partial rollback** (keep Redis for some instances):
   ```go
   // Hybrid approach: Use Redis but fallback to memory on errors
   sessionStore := livetemplate.NewRedisSessionStore(redisClient,
       livetemplate.WithFallbackToMemory(true),
   )
   ```

### Next Steps After Migration

1. **Add distributed pub/sub** for multi-instance server-initiated actions:
   ```go
   pubsubBroadcaster := livetemplate.NewRedisBroadcaster(redisClient)
   handler := livetemplate.Mount(rootStore,
       livetemplate.WithSessionStore(sessionStore),
       livetemplate.WithPubSubBroadcaster(pubsubBroadcaster), // Enable cross-instance updates
   )
   ```

2. **Configure monitoring** for Redis metrics
3. **Set up alerting** for Redis connectivity issues
4. **Review capacity planning** for expected load

See [session.md](../references/session.md) for the Session API guide on server-initiated actions.

---

## Capacity Planning

This section provides formulas and guidelines for estimating resource requirements based on your expected load.

### Memory Estimation

#### Application Instance Memory

**Per WebSocket Connection:**
```
Conservative: 15-200 KB
Realistic (medium complexity): 50 KB
Optimized (M3): 30 KB
```

**Components of Connection Memory:**
- WebSocket connection buffer: 8-16 KB
- Go goroutine stack: 2-8 KB
- Template state (lastData, lastTree): 20-100 KB (depends on state size)
- Connection metadata: 1-5 KB

**Example Calculations:**

| Connections | Memory (Conservative) | Memory (Realistic) | Instances (16GB RAM) |
|-------------|----------------------|-------------------|---------------------|
| 1,000 | 200 MB | 50 MB | 1 |
| 10,000 | 2 GB | 500 MB | 1 |
| 50,000 | 10 GB | 2.5 GB | 1-2 |
| 100,000 | 20 GB | 5 GB | 2-4 |
| 1,000,000 | 200 GB | 50 GB | 10-20 |

**Application Instance Overhead:**
- Operating system: 1-2 GB
- Database connection pool: 500 MB - 2 GB
- Redis client: 100-500 MB
- Application code and runtime: 500 MB - 1 GB
- Buffer for spikes: 20-30%

**Formula for Instance Memory:**
```
Total Memory = (Connections × Memory per Connection) + Overhead + Spike Buffer
```

**Example:**
```
10,000 connections × 50 KB = 500 MB
Overhead (OS + DB + Redis + App) = 4 GB
Spike Buffer (30%) = 1.35 GB
Total Memory Required = 5.85 GB ≈ 6-8 GB instance
```

#### Redis Session Store Memory

**Per Session (Session Group):**
```
Base session metadata: 500 bytes - 1 KB
Serialized State: Varies by application (1-100 KB typical)
Redis overhead: 20% (data structure overhead, fragmentation)
```

**Example State Sizes:**
```go
// Small: ~2 KB
type TodoState struct {
    Items []Todo  // 10 items × 200 bytes
}

// Medium: ~20 KB
type DashboardState struct {
    Metrics   map[string]int     // 100 metrics × 50 bytes
    Alerts    []Alert            // 10 alerts × 500 bytes
    UserPrefs UserPreferences    // 1 KB
}

// Large: ~100 KB
type ChatState struct {
    Messages []Message  // 100 messages × 1 KB
    Users    []User     // 50 users × 100 bytes
}
```

**Redis Memory Formula:**
```
Redis Memory = (Active Sessions × Avg State Size × 1.2) + Redis Overhead
```

**Redis Overhead:**
- Redis process: 50-100 MB baseline
- Connection buffers: 10 MB per 1000 clients
- Replication buffer (if using Sentinel/Cluster): 100 MB - 1 GB

**Example Calculations:**

| Active Sessions | Avg Store Size | Redis Memory (No HA) | Redis Memory (Sentinel) |
|----------------|----------------|---------------------|------------------------|
| 1,000 | 5 KB | 6 MB + 50 MB = 56 MB | 156 MB |
| 10,000 | 10 KB | 120 MB + 50 MB = 170 MB | 270 MB |
| 100,000 | 20 KB | 2.4 GB + 100 MB = 2.5 GB | 3.5 GB |
| 1,000,000 | 30 KB | 36 GB + 500 MB = 36.5 GB | 37.5 GB |

**Redis Memory Recommendations:**
- **Development:** 256 MB - 1 GB (single instance)
- **Small Production:** 2-4 GB (single instance with persistence)
- **Production:** 8-16 GB (Sentinel, 3 nodes)
- **Enterprise:** 32-64 GB per node (Cluster, 6+ nodes)

**Session TTL Impact:**
```
# Shorter TTL = Lower memory usage
24 hour TTL:  100K sessions × 20 KB = 2.4 GB
6 hour TTL:   25K sessions × 20 KB = 600 MB  # 4x reduction
1 hour TTL:   4K sessions × 20 KB = 96 MB    # 25x reduction
```

**Monitoring Redis Memory:**
```bash
# Check current memory usage
redis-cli INFO memory

# Key metrics to monitor:
# - used_memory_human: Total memory used
# - used_memory_rss_human: OS-reported memory
# - mem_fragmentation_ratio: Should be 1.0-1.5
# - evicted_keys: Should be 0 (we use TTL, not eviction)

# Session count
redis-cli DBSIZE
```

### CPU Estimation

**Per Instance:**

| Load Type | CPU per 1K Connections | CPU per 10K Connections |
|-----------|----------------------|------------------------|
| Idle connections | 0.1-0.2 cores | 1-2 cores |
| Active browsing (1 action/min) | 0.5-1 cores | 5-10 cores |
| Heavy interaction (10 actions/min) | 2-4 cores | 20-40 cores |

**Redis CPU:**
- Baseline: 0.5-1 core
- Per 10K ops/sec: +0.5 cores
- Pub/Sub fan-out: +1-2 cores per 10K messages/sec

**Recommendation:**
- Application instances: 2-8 cores per instance (depending on interaction rate)
- Redis: 4-8 cores for production (single-threaded, but benefits from hypervisor scheduling)

### Network Bandwidth

**WebSocket Traffic:**
- Idle connection: ~1-5 KB/min (heartbeat)
- Active connection: 10-500 KB/min (depends on update frequency)
- Fan-out heavy: 1-10 MB/min (real-time dashboards, chat)

**Per Instance Bandwidth:**
```
10K connections × 100 KB/min avg = 1 GB/min = 16.7 MB/s
```

**Redis Pub/Sub Bandwidth:**
```
Message size × Publish rate × Instance count
Example: 5 KB message × 100 publishes/sec × 10 instances = 5 MB/s
```

**Recommendation:**
- Application instances: 1 Gbps minimum, 10 Gbps for large deployments
- Redis: 1 Gbps for Standalone/Sentinel, 10 Gbps for Cluster

### Connection Distribution

**Rule of Thumb:**
- Keep instances at 60-70% capacity for headroom
- Example: 16 GB instance → 10 GB for connections → ~200K connections (realistic)
- Target: 120-140K connections per instance in production

**Load Balancer Strategy:**
- Use **sticky sessions** (cookie-based affinity)
- Cookie name: `livetemplate-id` (LiveTemplate session ID)
- Cookie TTL: Match session TTL (24 hours default)
- Fallback: Least-connections algorithm for new sessions

**Session Distribution:**
- With sticky sessions, sessions stay on same instance (good for caching)
- Without sticky sessions, sessions distribute evenly across instances (requires Redis)
- Long-lived WebSocket connections can cause imbalance over time
- Solution: Periodic connection migration (Milestone 3 feature)

### Scaling Decision Matrix

Use this table to determine when to scale horizontally (add instances) vs vertically (larger instances):

| Scenario | Current State | Recommended Action |
|----------|--------------|-------------------|
| Memory at 80% | Single instance | Add more instances (horizontal scale) |
| CPU at 80% | Single instance | Add more instances or upgrade instance size |
| High publish/fan-out latency | Multiple instances | Add more Redis resources or optimize peer-fan-out paths |
| Uneven load | Multiple instances | Enable connection migration (M3) or adjust LB algorithm |
| Session store slow | Redis at capacity | Upgrade Redis instance or switch to Cluster |

### Capacity Planning Example

**Scenario:** E-commerce platform with 50,000 concurrent users

**Requirements:**
- 50,000 WebSocket connections
- Average state size: 30 KB per session
- Moderate interaction: 2 actions/min per user
- 10 fan-out publishes/min to all users (price updates)

**Calculations:**

**1. Application Instances:**
```
Connection memory: 50,000 × 50 KB = 2.5 GB
Overhead: 4 GB
Spike buffer (30%): 2 GB
Total per instance: 8.5 GB

Instance size: 16 GB (provides headroom)
Connections per instance: 10,000 (60% capacity)
Required instances: 50,000 / 10,000 = 5 instances

CPU per instance (moderate load): 4-6 cores
Recommended: 5× instances with 8 cores, 16 GB RAM
```

**2. Redis Session Store:**
```
Active sessions: 50,000
Avg store size: 30 KB
Redis memory: 50,000 × 30 KB × 1.2 = 1.8 GB
Add overhead: 100 MB
Total: 2 GB

Recommended: Redis Sentinel (3 nodes, 4 GB each)
```

**3. Load Balancer:**
```
Sticky sessions enabled
Algorithm: Least-connections fallback
Health checks: /health/ready (every 10s)
Connection draining: 30s timeout
```

**4. Total Infrastructure:**
```
Application: 5 instances × $50/month = $250
Redis Sentinel: 3 nodes × $30/month = $90
Load Balancer: $40/month
Database: $100/month
Total: ~$480/month (Tier 2-3 scale)
```

### Capacity Planning Tools

**Formula Spreadsheet:**
```
Target Connections: [input]
Memory per Connection: 50 KB (default)
Sessions per Connection: 1 (default)
State Size per Session: 20 KB (default)

→ Application Memory: [calculated]
→ Redis Memory: [calculated]
→ Instance Count: [calculated]
→ Estimated Cost: [calculated]
```

**Monitoring Capacity:**
```promql
# Connection capacity utilization
(livetemplate_connections_active / livetemplate_connections_max) > 0.7

# Memory capacity utilization
(process_resident_memory_bytes / node_memory_MemTotal_bytes) > 0.8

# Redis memory utilization
(redis_memory_used_bytes / redis_memory_max_bytes) > 0.8
```

### Right-Sizing Recommendations

**When to Scale Up (Vertical):**
- Single instance at capacity and traffic is bursty
- CPU-bound workloads (heavy computations in actions)
- Cost-effective for small deployments (<10K connections)

**When to Scale Out (Horizontal):**
- Need high availability and zero-downtime deployments
- Traffic is steady and predictable
- >10K concurrent connections
- Multi-region requirements

**When to Use Redis Cluster (vs Sentinel):**
- >100K active sessions
- Session store memory >16 GB
- Need horizontal sharding for session data
- Multi-region deployment with global session sharing

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
- [ ] Add `RedisBroadcaster` for cross-instance peer fan-out
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
| Publish/Fan-out Latency (p95) | <50ms | <100ms | <50ms |
| Memory per Connection | 100 KB | 70 KB | 30 KB |
| Goroutines per Connection | 1 | 1 | 0.5 |

### Multi-Instance (10 instances)

| Metric | M2 | M3 |
|--------|----|----|
| Total Connections | 200K | 500K+ |
| Publish Fan-out Time (10K users) | 200ms | 100ms |
| Session Lookup Latency (Redis) | <5ms | <2ms |
| Cross-Instance Publish Latency | <100ms | <50ms |

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

### Issue: High Publish/Fan-out Latency

**Symptoms:**
- Publishes take >500ms to reach all clients
- Users report stale data

**Solutions:**
1. **Redis latency:** Check `redis-cli --latency` and network latency
2. **Fan-out size:** Limit group sizes or shard groups
3. **Local optimization:** Ensure local publishes skip Redis (M2 feature)
4. **Compression:** Enable WebSocket compression (M3)

### Issue: Sessions Not Persisting

**Symptoms:**
- Users lose session on instance restart
- Sessions disappear after 24 hours

**Solutions:**
1. **Check Redis:** Verify Redis persistence (RDB/AOF) enabled
2. **Check TTL:** Ensure session TTL configured correctly
3. **Check serialization:** Verify custom State types are serializable
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
livetemplate_publishes_sent_total rate(5m) > 10000  # High publish/fan-out rate
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
- Publish/fan-out distribution
- Session count by group

---

## Next Steps

- **Roadmap:** See [ROADMAP.md](../../ROADMAP.md) for upcoming scaling features
- **Architecture:** See [ARCHITECTURE.md](../design/ARCHITECTURE.md) for system design

---

**Questions?** Open an issue on GitHub or join the discussion.
