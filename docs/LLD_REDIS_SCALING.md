# Low-Level Design: Redis-Based Horizontal Scaling

## 1. Overview

This document describes the low-level design for implementing Redis-based shared page storage in LiveTemplate, enabling true horizontal scaling with stateless server instances.

### 1.1 Current State
- **Single Host**: In-memory page storage with global unique lvt-ids
- **Limitations**: Server affinity required, no failover, memory constraints

### 1.2 Target State  
- **Multi-Host**: Redis-backed page storage with stateless servers
- **Benefits**: Auto-failover, load balancer flexibility, cross-server access

## 2. Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Server A      │    │   Server B      │    │   Server C      │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ Page Cache  │ │    │ │ Page Cache  │ │    │ │ Page Cache  │ │
│ │ (Template)  │ │    │ │ (Template)  │ │    │ │ (Template)  │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │Redis Client │ │    │ │Redis Client │ │    │ │Redis Client │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │   Redis Cluster │
                    │                 │
                    │ ┌─────────────┐ │
                    │ │ Page State  │ │
                    │ │ (Strong)    │ │
                    │ └─────────────┘ │
                    │                 │
                    │ ┌─────────────┐ │
                    │ │ Templates   │ │
                    │ │ (Eventual)  │ │
                    │ └─────────────┘ │
                    └─────────────────┘
```

## 3. Component Design

### 3.1 Redis Page Registry

```go
// RedisPageRegistry implements distributed page storage
type RedisPageRegistry struct {
    // Redis connection
    client    *redis.Client
    clusterClient *redis.ClusterClient  // For Redis Cluster
    
    // Configuration
    config    *RedisRegistryConfig
    
    // Local caching for templates (eventual consistency)
    templateCache sync.Map  // pageID -> *CachedTemplate
    
    // Metrics
    metrics   *RedisMetrics
    
    // Lifecycle
    stopCleanup chan struct{}
    wg         sync.WaitGroup
}

type RedisRegistryConfig struct {
    // Redis connection
    Addresses    []string      // Redis cluster addresses
    Password     string        // Redis password
    DB          int           // Database number (single instance)
    
    // Scaling configuration  
    KeyPrefix   string        // "livetemplate:" prefix for all keys
    MaxPages    int           // Per-application limit
    
    // TTL settings
    PageStateTTL    time.Duration  // 1 hour - strong consistency
    TemplateTTL     time.Duration  // 5 minutes - eventual consistency
    
    // Local caching
    LocalTemplateCacheTTL time.Duration  // 30 seconds
    LocalCacheSize        int            // Max templates in local cache
    
    // Performance tuning
    Pipeline          bool           // Use Redis pipelining
    CompressionLevel  int            // 0=none, 1-9=gzip levels
    
    // Cleanup
    CleanupInterval   time.Duration  // How often to clean expired pages
}

type CachedTemplate struct {
    Template  *template.Template
    Source    string
    ExpiresAt time.Time
    Hash      string  // For invalidation
}
```

### 3.2 Serializable Page Data

```go
// SerializablePage represents page state that can be stored in Redis
type SerializablePage struct {
    // Identity
    ID            string    `json:"id" redis:"id"`
    ApplicationID string    `json:"application_id" redis:"application_id"`
    
    // Template information
    TemplateSource string   `json:"template_source" redis:"template_source"`
    TemplateHash   string   `json:"template_hash" redis:"template_hash"`
    
    // Dynamic state (frequently updated)
    Data         json.RawMessage `json:"data" redis:"data"`
    
    // Metadata
    CreatedAt    time.Time `json:"created_at" redis:"created_at"`
    LastAccessed time.Time `json:"last_accessed" redis:"last_accessed"`
    AccessCount  int64     `json:"access_count" redis:"access_count"`
    
    // Fragment cache
    FragmentCache map[string]string `json:"fragment_cache" redis:"fragment_cache"`
}

// RedisPageState stores frequently changing data with strong consistency
type RedisPageState struct {
    Data         json.RawMessage `redis:"data"`
    LastAccessed time.Time       `redis:"last_accessed"`
    AccessCount  int64           `redis:"access_count"`
    FragmentCache map[string]string `redis:"fragment_cache"`
}

// RedisPageTemplate stores template info with eventual consistency
type RedisPageTemplate struct {
    TemplateSource string    `redis:"template_source"`
    TemplateHash   string    `redis:"template_hash"`
    CreatedAt      time.Time `redis:"created_at"`
}
```

### 3.3 Redis Key Structure

```
# Page State (Strong Consistency - Updated Frequently)
pages:state:{applicationID}:{pageID}
├── data: JSON              # Current page data
├── last_accessed: timestamp # Access tracking  
├── access_count: integer   # Usage metrics
└── fragment_cache: JSON    # Cached fragments

# Page Templates (Eventual Consistency - Updated Rarely)  
pages:template:{applicationID}:{pageID}
├── template_source: string # Go template source
├── template_hash: string   # For cache invalidation
└── created_at: timestamp   # Template creation time

# Application Indexes
pages:index:{applicationID}
└── SET of page IDs         # All pages for application

# Global Metrics
pages:metrics:global
├── total_pages: counter    # Total active pages
├── total_apps: counter     # Total applications  
└── last_cleanup: timestamp # Last cleanup run

# Cleanup tracking
pages:cleanup:{date}
└── SET of cleaned page IDs # Pages cleaned on date
```

## 4. API Design

### 4.1 Core Operations

```go
// RedisPageRegistry interface
type PageRegistry interface {
    // Page lifecycle
    Store(page *Page) error
    Get(pageID, applicationID string) (*Page, error)
    Update(pageID, applicationID string, data interface{}) error
    Delete(pageID, applicationID string) error
    
    // Application management
    GetByApplication(applicationID string) map[string]*Page
    DeleteApplication(applicationID string) error
    
    // Scaling operations
    Migrate(fromNodeID, toNodeID string) error
    HealthCheck() error
    
    // Metrics
    GetMetrics() *RegistryMetrics
    
    // Lifecycle
    Start() error
    Stop() error
}

// Implementation details
type RedisPageRegistry struct {
    // ... fields from above
}

func (r *RedisPageRegistry) Store(page *Page) error {
    // 1. Serialize page data
    state := &RedisPageState{
        Data:         marshalData(page.data),
        LastAccessed: time.Now(),
        AccessCount:  1,
        FragmentCache: page.fragmentCache,
    }
    
    template := &RedisPageTemplate{
        TemplateSource: page.templateSource,
        TemplateHash:   hashTemplate(page.templateSource),
        CreatedAt:      time.Now(),
    }
    
    // 2. Store with different TTLs using pipeline for atomicity
    pipe := r.client.Pipeline()
    
    // Store state (strong consistency)
    stateKey := r.pageStateKey(page.ApplicationID, page.ID)
    pipe.HMSet(ctx, stateKey, structToMap(state))
    pipe.Expire(ctx, stateKey, r.config.PageStateTTL)
    
    // Store template (eventual consistency)  
    templateKey := r.pageTemplateKey(page.ApplicationID, page.ID)
    pipe.HMSet(ctx, templateKey, structToMap(template))
    pipe.Expire(ctx, templateKey, r.config.TemplateTTL)
    
    // Update application index
    indexKey := r.applicationIndexKey(page.ApplicationID)
    pipe.SAdd(ctx, indexKey, page.ID)
    pipe.Expire(ctx, indexKey, r.config.PageStateTTL)
    
    // Execute pipeline
    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("failed to store page in Redis: %w", err)
    }
    
    // 3. Update local template cache
    r.cacheTemplate(page.ID, page.template, page.templateSource)
    
    return nil
}

func (r *RedisPageRegistry) Get(pageID, applicationID string) (*Page, error) {
    // 1. Check local template cache first
    cachedTemplate := r.getCachedTemplate(pageID)
    
    // 2. Load page state (always from Redis for strong consistency)
    stateKey := r.pageStateKey(applicationID, pageID)
    stateFields, err := r.client.HMGet(ctx, stateKey, 
        "data", "last_accessed", "access_count", "fragment_cache").Result()
    if err != nil {
        return nil, fmt.Errorf("failed to load page state: %w", err)
    }
    
    if stateFields[0] == nil {
        return nil, ErrPageNotFound
    }
    
    // 3. Load template (from cache or Redis)
    var tmpl *template.Template
    var templateSource string
    
    if cachedTemplate != nil && !cachedTemplate.IsExpired() {
        tmpl = cachedTemplate.Template
        templateSource = cachedTemplate.Source
    } else {
        // Load from Redis
        templateKey := r.pageTemplateKey(applicationID, pageID)
        templateFields, err := r.client.HMGet(ctx, templateKey,
            "template_source", "template_hash").Result()
        if err != nil || templateFields[0] == nil {
            return nil, fmt.Errorf("failed to load page template: %w", err)
        }
        
        templateSource = templateFields[0].(string)
        
        // Parse template
        tmpl, err = template.New("page").Parse(templateSource)
        if err != nil {
            return nil, fmt.Errorf("failed to parse template: %w", err)
        }
        
        // Cache locally
        r.cacheTemplate(pageID, tmpl, templateSource)
    }
    
    // 4. Reconstruct page object
    var data interface{}
    if err := json.Unmarshal([]byte(stateFields[0].(string)), &data); err != nil {
        return nil, fmt.Errorf("failed to unmarshal page data: %w", err)
    }
    
    page := &Page{
        ID:             pageID,
        ApplicationID:  applicationID,
        template:       tmpl,
        templateSource: templateSource,
        data:           data,
        lastAccessed:   parseTime(stateFields[1]),
        fragmentCache:  parseFragmentCache(stateFields[3]),
        treeGenerator:  strategy.NewSimpleTreeGenerator(),
        config:         DefaultConfig(),
    }
    
    // 5. Update access tracking
    go r.updateAccessTracking(applicationID, pageID)
    
    return page, nil
}

func (r *RedisPageRegistry) Update(pageID, applicationID string, newData interface{}) error {
    // Strong consistency update for page state
    stateKey := r.pageStateKey(applicationID, pageID)
    
    dataJSON, err := json.Marshal(newData)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %w", err)
    }
    
    // Use WATCH for optimistic locking
    err = r.client.Watch(ctx, func(tx *redis.Tx) error {
        // Check page still exists
        exists, err := tx.Exists(ctx, stateKey).Result()
        if err != nil || exists == 0 {
            return ErrPageNotFound
        }
        
        // Update with pipeline
        pipe := tx.Pipeline()
        pipe.HMSet(ctx, stateKey, map[string]interface{}{
            "data":          string(dataJSON),
            "last_accessed": time.Now().Format(time.RFC3339),
        })
        pipe.HIncrBy(ctx, stateKey, "access_count", 1)
        
        _, err = pipe.Exec(ctx)
        return err
    }, stateKey)
    
    return err
}
```

### 4.2 Consistency Models

```go
// ConsistencyPolicy defines consistency requirements
type ConsistencyPolicy struct {
    PageState    ConsistencyLevel  // STRONG - immediate user updates
    Templates    ConsistencyLevel  // EVENTUAL - deployment propagation  
    Fragments    ConsistencyLevel  // STRONG - real-time updates
    Cleanup      ConsistencyLevel  // EVENTUAL - background cleanup
}

type ConsistencyLevel int

const (
    ConsistencyEventual ConsistencyLevel = iota // Eventually consistent
    ConsistencyStrong                           // Strongly consistent
    ConsistencySession                          // Session consistency
)

// Implementation
func (r *RedisPageRegistry) applyConsistencyPolicy(operation string, data interface{}) error {
    switch operation {
    case "page_state_update":
        // Strong consistency - use Redis transactions
        return r.strongConsistencyUpdate(data)
        
    case "template_update":
        // Eventual consistency - simple write + local cache invalidation
        return r.eventualConsistencyUpdate(data)
        
    case "fragment_update":
        // Strong consistency - immediate visibility required
        return r.strongConsistencyUpdate(data)
        
    default:
        return r.eventualConsistencyUpdate(data)
    }
}

func (r *RedisPageRegistry) strongConsistencyUpdate(data interface{}) error {
    // Use Redis MULTI/EXEC for atomic updates
    pipe := r.client.TxPipeline()
    // ... add operations
    _, err := pipe.Exec(ctx)
    return err
}

func (r *RedisPageRegistry) eventualConsistencyUpdate(data interface{}) error {
    // Simple write + async cache invalidation
    err := r.client.Set(ctx, key, data, ttl).Err()
    if err == nil {
        go r.invalidateLocalCache(key)  // Async
    }
    return err
}
```

## 5. Performance Optimizations

### 5.1 Local Template Caching

```go
// Template caching to reduce Redis calls
type TemplateCache struct {
    cache    sync.Map           // pageID -> *CachedTemplate
    maxSize  int               // Maximum cached templates
    ttl      time.Duration     // Cache TTL
    metrics  *CacheMetrics
}

func (t *TemplateCache) Get(pageID string) *CachedTemplate {
    if val, ok := t.cache.Load(pageID); ok {
        cached := val.(*CachedTemplate)
        if !cached.IsExpired() {
            t.metrics.Hits++
            return cached
        }
        t.cache.Delete(pageID)  // Remove expired
    }
    t.metrics.Misses++
    return nil
}

func (t *TemplateCache) Set(pageID string, template *template.Template, source string) {
    // Implement LRU eviction if needed
    if t.size() >= t.maxSize {
        t.evictLRU()
    }
    
    cached := &CachedTemplate{
        Template:  template,
        Source:    source,
        ExpiresAt: time.Now().Add(t.ttl),
        Hash:      hashTemplate(source),
    }
    
    t.cache.Store(pageID, cached)
    t.metrics.Sets++
}
```

### 5.2 Redis Pipelining

```go
// Batch operations for better performance
type BatchOperation struct {
    Type    string      // "store", "update", "delete"
    PageID  string
    AppID   string
    Data    interface{}
}

func (r *RedisPageRegistry) ExecuteBatch(operations []BatchOperation) error {
    pipe := r.client.Pipeline()
    
    for _, op := range operations {
        switch op.Type {
        case "store":
            r.addStoreToPipeline(pipe, op)
        case "update":
            r.addUpdateToPipeline(pipe, op)
        case "delete":
            r.addDeleteToPipeline(pipe, op)
        }
    }
    
    results, err := pipe.Exec(ctx)
    if err != nil {
        return err
    }
    
    // Process results
    return r.processBatchResults(results, operations)
}
```

### 5.3 Compression

```go
// Compress large page data to reduce Redis memory
func (r *RedisPageRegistry) compressData(data []byte) ([]byte, error) {
    if len(data) < r.config.CompressionThreshold {
        return data, nil  // Don't compress small data
    }
    
    var buf bytes.Buffer
    writer := gzip.NewWriter(&buf)
    writer.Write(data)
    writer.Close()
    
    compressed := buf.Bytes()
    if len(compressed) >= len(data) {
        return data, nil  // Compression not beneficial
    }
    
    return compressed, nil
}
```

## 6. Error Handling & Recovery

### 6.1 Redis Connection Handling

```go
// Robust Redis connection with failover
type RedisConnectionManager struct {
    config    *RedisConfig
    client    redis.UniversalClient  // Supports both single and cluster
    retryBackoff time.Duration
    maxRetries   int
}

func (r *RedisConnectionManager) Execute(fn func(redis.UniversalClient) error) error {
    var lastErr error
    
    for attempt := 0; attempt <= r.maxRetries; attempt++ {
        if attempt > 0 {
            time.Sleep(r.retryBackoff * time.Duration(attempt))
        }
        
        err := fn(r.client)
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Check if error is retryable
        if !r.isRetryableError(err) {
            break
        }
        
        // Try to reconnect
        if r.isConnectionError(err) {
            r.reconnect()
        }
    }
    
    return fmt.Errorf("redis operation failed after %d attempts: %w", r.maxRetries+1, lastErr)
}

func (r *RedisConnectionManager) isRetryableError(err error) bool {
    if err == nil {
        return false
    }
    
    // Network errors, timeouts, etc.
    return redis.IsRetryableError(err) || 
           strings.Contains(err.Error(), "connection") ||
           strings.Contains(err.Error(), "timeout")
}
```

### 6.2 Graceful Degradation

```go
// Fallback to local storage when Redis is unavailable
type HybridPageRegistry struct {
    redis       *RedisPageRegistry
    local       *LocalPageRegistry
    mode        RegistryMode
    healthCheck *HealthChecker
}

type RegistryMode int

const (
    ModeRedis RegistryMode = iota  // Primary: Redis
    ModeLocal                      // Fallback: Local memory
    ModeHybrid                     // Mixed: Redis + Local cache
)

func (h *HybridPageRegistry) Store(page *Page) error {
    // Always try Redis first
    err := h.redis.Store(page)
    if err == nil {
        h.mode = ModeRedis
        return nil
    }
    
    // Log Redis failure
    log.Warnf("Redis store failed, falling back to local: %v", err)
    
    // Fallback to local storage
    h.mode = ModeLocal
    return h.local.Store(page)
}

func (h *HybridPageRegistry) Get(pageID, applicationID string) (*Page, error) {
    switch h.mode {
    case ModeRedis, ModeHybrid:
        page, err := h.redis.Get(pageID, applicationID)
        if err == nil {
            return page, nil
        }
        
        // Try local fallback
        if h.mode == ModeHybrid {
            return h.local.Get(pageID, applicationID)
        }
        return nil, err
        
    case ModeLocal:
        return h.local.Get(pageID, applicationID)
        
    default:
        return nil, ErrInvalidMode
    }
}
```

## 7. Monitoring & Metrics

### 7.1 Redis Metrics

```go
type RedisMetrics struct {
    // Operation counters
    StoreOps      prometheus.Counter
    GetOps        prometheus.Counter  
    UpdateOps     prometheus.Counter
    DeleteOps     prometheus.Counter
    
    // Performance metrics
    OperationDuration prometheus.HistogramVec  // By operation type
    RedisConnections  prometheus.Gauge
    
    // Cache metrics
    LocalCacheHits   prometheus.Counter
    LocalCacheMisses prometheus.Counter
    LocalCacheSize   prometheus.Gauge
    
    // Error metrics
    RedisErrors      prometheus.CounterVec  // By error type
    FallbackCount    prometheus.Counter     // Local fallback usage
    
    // Resource usage
    RedisMemoryUsage prometheus.Gauge
    PageCount        prometheus.GaugeVec   // By application
}

// Metrics collection
func (r *RedisPageRegistry) collectMetrics() {
    // Redis INFO command for memory usage
    info, err := r.client.Info(ctx, "memory").Result()
    if err == nil {
        memUsage := parseRedisMemoryUsage(info)
        r.metrics.RedisMemoryUsage.Set(float64(memUsage))
    }
    
    // Count pages per application
    apps := r.getAllApplications()
    for _, appID := range apps {
        count := r.getPageCount(appID)
        r.metrics.PageCount.WithLabelValues(appID).Set(float64(count))
    }
}
```

### 7.2 Health Checks

```go
type HealthChecker struct {
    registry    *RedisPageRegistry
    interval    time.Duration
    timeout     time.Duration
    
    // Health status
    lastCheck   time.Time
    isHealthy   bool
    lastError   error
}

func (h *HealthChecker) CheckHealth() error {
    ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
    defer cancel()
    
    // Test basic Redis operations
    testKey := "health:check:" + generateRandomID()
    testValue := time.Now().Format(time.RFC3339)
    
    // Test SET
    if err := h.registry.client.Set(ctx, testKey, testValue, time.Minute).Err(); err != nil {
        return fmt.Errorf("redis SET failed: %w", err)
    }
    
    // Test GET
    result, err := h.registry.client.Get(ctx, testKey).Result()
    if err != nil {
        return fmt.Errorf("redis GET failed: %w", err)
    }
    
    if result != testValue {
        return fmt.Errorf("redis data integrity check failed")
    }
    
    // Test DELETE
    if err := h.registry.client.Del(ctx, testKey).Err(); err != nil {
        return fmt.Errorf("redis DEL failed: %w", err)
    }
    
    // Test pipeline operations
    pipe := h.registry.client.Pipeline()
    pipe.Set(ctx, testKey+"_pipe", testValue, time.Minute)
    pipe.Get(ctx, testKey+"_pipe")
    pipe.Del(ctx, testKey+"_pipe")
    
    _, err = pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("redis pipeline failed: %w", err)
    }
    
    return nil
}
```

## 8. Migration Strategy

### 8.1 Gradual Migration

```go
// Migration phases for existing deployments
type MigrationPhase int

const (
    PhasePreparation MigrationPhase = iota  // Setup Redis, no data migration
    PhaseDualWrite                          // Write to both local and Redis
    PhaseRedisRead                          // Read from Redis, fallback to local
    PhaseRedisOnly                          // Redis-only operations
    PhaseCleanup                            // Remove local storage
)

type MigrationManager struct {
    currentPhase MigrationPhase
    registry     *HybridPageRegistry
    config       *MigrationConfig
}

func (m *MigrationManager) ExecuteMigration() error {
    phases := []MigrationPhase{
        PhasePreparation,
        PhaseDualWrite,
        PhaseRedisRead,
        PhaseRedisOnly,
        PhaseCleanup,
    }
    
    for _, phase := range phases {
        if err := m.executePhase(phase); err != nil {
            return fmt.Errorf("migration phase %d failed: %w", phase, err)
        }
        m.currentPhase = phase
        
        // Wait for phase completion confirmation
        if err := m.waitForPhaseCompletion(phase); err != nil {
            return err
        }
    }
    
    return nil
}
```

### 8.2 Data Migration

```go
func (m *MigrationManager) migrateExistingPages() error {
    // Get all pages from local registry
    localPages := m.registry.local.GetAll()
    
    batchSize := 100
    for i := 0; i < len(localPages); i += batchSize {
        batch := localPages[i:min(i+batchSize, len(localPages))]
        
        if err := m.migrateBatch(batch); err != nil {
            return fmt.Errorf("batch migration failed at index %d: %w", i, err)
        }
        
        // Rate limiting to avoid overwhelming Redis
        time.Sleep(10 * time.Millisecond)
    }
    
    return nil
}
```

## 9. Configuration

### 9.1 Redis Configuration

```yaml
# redis-config.yaml
redis:
  # Connection
  mode: "cluster"  # "single", "cluster", "sentinel"
  addresses:
    - "redis-1:6379"
    - "redis-2:6379"
    - "redis-3:6379"
  password: "${REDIS_PASSWORD}"
  database: 0
  
  # Performance
  pool_size: 100
  idle_timeout: "5m"
  read_timeout: "3s"
  write_timeout: "3s"
  
  # Consistency
  page_state_ttl: "1h"
  template_ttl: "5m"
  
  # Local caching
  local_template_cache_ttl: "30s"
  local_cache_size: 1000
  
  # Optimization
  pipeline_enabled: true
  compression_threshold: 1024  # bytes
  compression_level: 6         # gzip level
  
  # Reliability
  max_retries: 3
  retry_backoff: "100ms"
  health_check_interval: "30s"
```

### 9.2 Environment-Specific Configs

```go
// Development
func DevelopmentConfig() *RedisRegistryConfig {
    return &RedisRegistryConfig{
        Addresses:         []string{"localhost:6379"},
        MaxPages:          100,
        PageStateTTL:      30 * time.Minute,
        TemplateTTL:       5 * time.Minute,
        LocalCacheSize:    50,
        Pipeline:          false,  // Easier debugging
        CompressionLevel:  0,      // No compression in dev
    }
}

// Production  
func ProductionConfig() *RedisRegistryConfig {
    return &RedisRegistryConfig{
        Addresses:         getRedisClusterAddresses(),
        MaxPages:          10000,
        PageStateTTL:      2 * time.Hour,
        TemplateTTL:       15 * time.Minute,
        LocalCacheSize:    2000,
        Pipeline:          true,
        CompressionLevel:  6,
        CleanupInterval:   10 * time.Minute,
    }
}
```

## 10. Testing Strategy

### 10.1 Unit Tests

```go
// Test Redis operations with testcontainers
func TestRedisPageRegistry(t *testing.T) {
    // Setup Redis container
    ctx := context.Background()
    redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "redis:7-alpine",
            ExposedPorts: []string{"6379/tcp"},
            WaitingFor:   wait.ForLog("Ready to accept connections"),
        },
        Started: true,
    })
    require.NoError(t, err)
    defer redisContainer.Terminate(ctx)
    
    // Get Redis endpoint
    endpoint, err := redisContainer.Endpoint(ctx, "")
    require.NoError(t, err)
    
    // Test registry operations
    config := &RedisRegistryConfig{
        Addresses:    []string{endpoint},
        PageStateTTL: time.Minute,
        TemplateTTL:  time.Minute,
    }
    
    registry, err := NewRedisPageRegistry(config)
    require.NoError(t, err)
    
    // Test page lifecycle
    page := createTestPage(t)
    
    // Store
    err = registry.Store(page)
    require.NoError(t, err)
    
    // Get
    retrieved, err := registry.Get(page.ID, page.ApplicationID)
    require.NoError(t, err)
    require.Equal(t, page.ID, retrieved.ID)
    
    // Update
    newData := map[string]interface{}{"counter": 42}
    err = registry.Update(page.ID, page.ApplicationID, newData)
    require.NoError(t, err)
    
    // Delete
    err = registry.Delete(page.ID, page.ApplicationID)
    require.NoError(t, err)
}
```

### 10.2 Integration Tests

```go
// Test horizontal scaling scenarios
func TestHorizontalScaling(t *testing.T) {
    // Setup multiple server instances
    servers := make([]*Server, 3)
    for i := 0; i < 3; i++ {
        servers[i] = createServerInstance(t, i)
    }
    
    // Test cross-server page access
    page := createTestPage(t)
    
    // Store on server 0
    err := servers[0].registry.Store(page)
    require.NoError(t, err)
    
    // Access from server 1
    retrieved, err := servers[1].registry.Get(page.ID, page.ApplicationID)
    require.NoError(t, err)
    require.Equal(t, page.Data, retrieved.Data)
    
    // Update from server 2
    newData := map[string]interface{}{"updated": true}
    err = servers[2].registry.Update(page.ID, page.ApplicationID, newData)
    require.NoError(t, err)
    
    // Verify update visible on server 0
    updated, err := servers[0].registry.Get(page.ID, page.ApplicationID)
    require.NoError(t, err)
    require.Equal(t, newData, updated.Data)
}
```

## 11. Deployment Considerations

### 11.1 Redis Cluster Setup

```yaml
# redis-cluster.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-cluster-config
data:
  redis.conf: |
    cluster-enabled yes
    cluster-config-file nodes.conf
    cluster-node-timeout 5000
    appendonly yes
    
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis-cluster
spec:
  serviceName: redis-cluster
  replicas: 6
  selector:
    matchLabels:
      app: redis-cluster
  template:
    metadata:
      labels:
        app: redis-cluster
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command: ["redis-server"]
        args: ["/etc/redis/redis.conf"]
        ports:
        - containerPort: 6379
          name: client
        - containerPort: 16379
          name: gossip
        volumeMounts:
        - name: conf
          mountPath: /etc/redis/
        - name: data
          mountPath: /data
      volumes:
      - name: conf
        configMap:
          name: redis-cluster-config
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi
```

### 11.2 Application Configuration

```go
// Load configuration from environment
func LoadRedisConfig() *RedisRegistryConfig {
    return &RedisRegistryConfig{
        Addresses:         strings.Split(os.Getenv("REDIS_ADDRESSES"), ","),
        Password:          os.Getenv("REDIS_PASSWORD"),
        DB:               getIntEnv("REDIS_DB", 0),
        MaxPages:         getIntEnv("REDIS_MAX_PAGES", 10000),
        PageStateTTL:     getDurationEnv("REDIS_PAGE_STATE_TTL", 2*time.Hour),
        TemplateTTL:      getDurationEnv("REDIS_TEMPLATE_TTL", 15*time.Minute),
        LocalCacheSize:   getIntEnv("REDIS_LOCAL_CACHE_SIZE", 2000),
        Pipeline:         getBoolEnv("REDIS_PIPELINE", true),
        CompressionLevel: getIntEnv("REDIS_COMPRESSION", 6),
    }
}
```

## 12. Performance Benchmarks

Expected performance characteristics:

| Metric | Single Host | Redis (Single) | Redis (Cluster) |
|--------|-------------|----------------|-----------------|
| Page Creation | 70k/sec | 25k/sec | 45k/sec |
| Page Retrieval | 100k/sec | 35k/sec | 60k/sec |
| Page Update | 80k/sec | 30k/sec | 50k/sec |
| Memory per Page | 8KB | 12KB | 12KB |
| Latency P95 | <1ms | <5ms | <3ms |
| Failover Time | N/A | <2sec | <1sec |

Trade-offs:
- ✅ **Horizontal scaling**: Unlimited capacity
- ✅ **High availability**: Redis cluster redundancy  
- ✅ **Cross-server consistency**: Shared state
- ⚠️ **Latency increase**: Network overhead
- ⚠️ **Complexity**: Redis dependency and configuration

This completes the comprehensive Low-Level Design for Redis-based horizontal scaling in LiveTemplate.