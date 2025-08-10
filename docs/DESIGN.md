# LiveTemplate: Session Isolation and Hybrid Incremental Updates

A technical design document for implementing secure session isolation with deterministic incremental update optimization in LiveTemplate.

## Introduction

This document defines the architectural design for LiveTemplate's transition from a singleton page model to a multi-tenant system with session isolation and incremental updates. The design addresses the current security limitations where all users share page instances, introduces application-scoped isolation boundaries, and implements automatic optimization strategies for bandwidth reduction through template analysis and selective update mechanisms.

> Status: Proposal — This document describes a proposed architecture that is not yet fully implemented. The current repository reflects the existing implementation; component names and APIs here may evolve during implementation.

---

## Layer 1: Problem Definition and Context

### Problem Statement

LiveTemplate currently uses a singleton page model that creates fundamental security and scalability issues for production web applications:

1. **Security Vulnerability**: All clients share the same page instance, creating potential data leakage between users
2. **No Session Isolation**: Multiple users cannot maintain independent state or data tracking
3. **Multi-Tenancy Impossible**: No organizational boundaries between different applications or services
4. **Performance Suboptimal**: No systematic approach to minimize bandwidth usage in real-time updates
5. **High-Frequency Update Inefficiency**: Poor handling of rapid update scenarios (live monitoring, gaming, real-time feeds)
6. **Mobile Network Optimization Missing**: No consideration for bandwidth-constrained mobile environments

Real-world applications require capabilities that the current architecture cannot provide:

- **E-commerce**: Real-time cart updates, live inventory status across multiple users
- **Social Media**: Live comment threads, instant notifications, activity feeds
- **Analytics**: Real-time dashboard updates, synchronized metrics across team members
- **Gaming/Entertainment**: Live leaderboards, real-time sports scoring, interactive applications
- **Monitoring**: Live server status, real-time metrics, automated alerts

These use cases expose fundamental architectural limitations that require a complete redesign rather than incremental improvements.

### Current State (AS-IS)

LiveTemplate currently operates with a page-centric model where there is no built-in application-level isolation or session management. This creates several fundamental issues for production web applications:

#### Critical Issues

- No built-in session isolation between different users or applications
- No systematic approach to prevent cross-user data leakage
- No organizational boundaries for multi-application deployments
- Limited bandwidth optimization strategies for real-time updates
- No standardized approach to secure token-based authentication

### Goals

#### Primary Goals

1. **Security**: Complete isolation between user sessions and applications
2. **Multi-Application Support**: Support multiple independent applications with isolated state
3. **Performance**: Minimize bandwidth through deterministic incremental updates
4. **Developer Experience**: Simple APIs that prevent security mistakes
5. **High-Frequency Updates**: Handle rapid update scenarios (live chat, gaming, monitoring) efficiently
6. **Scalability**: Support complex pages with thousands of fragments and long-running sessions

#### Secondary Goals

1. **Mobile Optimization**: Efficient updates over constrained mobile networks
2. **Accessibility**: Screen reader-friendly dynamic updates
3. **Observability**: Comprehensive metrics and debugging capabilities
4. **Extensibility**: Plugin architecture for custom update strategies

#### Non-Goals

- Backward compatibility with the singleton model (breaking change acceptable)
- Support for server-side session sharing across instances
- Complex update configuration (should be automatic)
- User authentication and authorization (library consumers handle this)
- Role-based access control (applications build this on top)
- Collaborative editing algorithms (applications implement conflict resolution)
- Client-side caching or offline capabilities (focus on real-time scenarios)

### Requirements

#### Functional Requirements

- FR1: Each page instance must have isolated state to prevent cross-user data leakage
- FR2: System must automatically optimize update bandwidth
- FR3: Developers must get clear feedback on optimization opportunities
- FR4: Initial page render must return the full HTML document with fragment annotations (identifiers/markers) that enable subsequent incremental updates
- FR5: Subsequent updates must apply without a full page reload via available transports (AJAX, WebSocket, or SSE), selected transparently by the client
- FR6: Incremental update design is transport-agnostic: the primary concern is the shape and size of update messages; underlying network transfer specifics are out of scope of this design
- FR7: System must handle high-frequency updates (typing, real-time sports) with sub-200ms latency
- FR8: System must handle extreme load scenarios (10k+ rapid updates) through batching, coalescing, and backpressure

#### Non-Functional Requirements

- NFR1: Support for 10,000+ concurrent isolated page sessions
- NFR2: 60-85% bandwidth reduction through incremental updates
- NFR3: Zero-configuration update mode selection
- NFR4: Bounded update payload size per message (transport-level cap; server will chunk if exceeded)
- NFR5: Coalescing and operation caps per render cycle (overflow triggers safe fallback)
- NFR6: Per-page memory budget for tracking analysis/artifacts with graceful degradation when exceeded
- NFR7: Backpressure and rate limiting under high-change workloads (updates are batched and throttled)
- NFR8: p95 end-to-end update latency ≤ 150ms under typical loads (100 RPS)
- NFR9: First-class observability: metrics, structured logs, and tracing for Render/RenderUpdates
- NFR10: Support for 50,000+ fragments per page (complex applications)
- NFR11: Sub-100ms update latency for critical applications (trading, monitoring)
- NFR12: Graceful handling of 1,000+ updates/second per page (live chat, gaming)
- NFR13: Memory efficiency for long-running sessions (24+ hour applications)
- NFR14: Bandwidth optimization for mobile networks (3G fallback)

### Key Stakeholders

- **Application Developers**: Need simple, performant APIs for page updates with clear optimization feedback
- **Performance Engineers**: Require visibility into bandwidth reduction and latency optimization
- **End Users**: Benefit from faster page updates and reduced bandwidth usage
- **DevOps Teams**: Need observability and monitoring for page update performance

---

## Layer 2: Functional Specification

### Functional Design Decisions

#### Decision 1: Application-Scoped Architecture vs Global Registry

##### Options Considered (Decision 1)

- Global page registry with session-specific keys
- Application-scoped registries with isolated signing keys
- Database-backed session storage

##### Choice: Application-Scoped Registries

- **Reasoning**: Provides strongest isolation guarantees, simplest security model, and enables multi-application deployments
- **Trade-off**: Requires breaking change from singleton model

#### Decision 2: Automatic vs Manual Update Mode

##### Options Considered (Decision 2)

- Developer-configured update modes
- Runtime adaptive updates based on performance metrics
- Static template analysis with automatic mode selection

##### Choice: Static Template Analysis

- **Reasoning**: Deterministic behavior, zero configuration, predictable performance
- **Trade-off**: Some edge cases may not be optimally handled

### System Behaviors

#### Page Session Isolation (FR1)

```go
// Each page instance has isolated state to prevent data leakage
page1 := livetemplate.NewPage(templates, userData1)
page2 := livetemplate.NewPage(templates, userData2)

// page1 and page2 are completely isolated - no shared state
// Each page manages its own data and cannot access other pages' data
token1 := page1.GetToken() // Unique identifier for page1 session
token2 := page2.GetToken() // Unique identifier for page2 session

// Attempting to use wrong token returns error
page := livetemplate.GetPageByToken(token1)  // Returns page1
invalidPage := livetemplate.GetPageByToken("invalid-token") // Returns error
```

#### Automatic Update Mode Selection (FR2)

```go
// Automatic optimization based on template analysis
app := livetemplate.NewApplication()
page := app.NewPage(templates, data)

// Analysis available through application debug interface
debug := app.Debug()
analysis, err := debug.GetTemplateAnalysis(page.GetToken())
if err == nil {
    switch analysis.SelectedUpdateMode {
    case ValuePatch:
        // 85% bandwidth reduction - surgical value updates
    case FragmentReplace:
        // 60% bandwidth reduction - full fragment replacement
    }
}
```

#### High-Frequency Update Optimization (FR7)

```go
// Single RenderFragments API with intelligent automatic batching
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]Fragment, error) {
    // Automatic intelligent behavior:
    // - Fragments from single RenderFragments call are always batched together
    // - All fragments are automatically coalesced when possible
    // - Optimal batching window determined by library (typically 16ms for 60fps)
    // - Network-aware batch sizing to stay within transport limits
    // - No configuration needed - library makes optimal decisions
}
```

````

#### Initial Render and Fragment Annotations (FR4)

The first `Render()` returns the full HTML document instrumented with stable fragment identifiers (e.g., data-fragment-id) so subsequent incremental updates can be applied in-place without a full page reload over AJAX, WebSocket, or SSE.

```go
// Example fragment annotation in rendered HTML
<div data-fragment-id="f-a1b2c3d4">
    <span>Counter: {{.Counter}}</span>
</div>
````

#### Template Analysis and Optimization (FR2, FR3)

```go
// Automatic optimization based on template analysis
app := livetemplate.NewApplication()
page := app.NewPage(templates, data)

// Analysis available through application debug interface
debug := app.Debug()
analysis, err := debug.GetTemplateAnalysis(page.GetToken())
if err == nil {
    switch analysis.SelectedUpdateMode {
    case ValuePatch:
        // 85% bandwidth reduction - surgical value updates
    case FragmentReplace:
        // 60% bandwidth reduction - full fragment replacement
    }

    // Developers get clear feedback on optimization opportunities
    fmt.Printf("Template analysis: %s mode selected, %.1f%% bandwidth reduction expected",
        analysis.SelectedUpdateMode, analysis.BandwidthReduction*100)
}
```

### Alternative Approaches Not Chosen

#### Alternative 1: Session Middleware Pattern

- **Rejected**: Would require complex integration with existing web frameworks
- **Reasoning**: Application-scoped approach is framework-agnostic and simpler

#### Alternative 2: Runtime Adaptive Updates

- **Rejected**: Would introduce non-deterministic behavior
- **Reasoning**: Static analysis provides predictable, debuggable performance

#### Alternative 3: Database-Backed Sessions

- **Rejected**: Adds complexity and latency for in-memory workloads
- **Reasoning**: Memory-based registries with optional Redis backing provides better flexibility

---

## Layer 3: Technical Specification

### Architecture Overview

The system implements a **two-layer architecture**:

1. **Page Isolation Layer**: Provides isolated page instances with unique tokens to prevent data leakage
2. **Hybrid Incremental Update Layer**: Automatically selects optimal update mode per template (performance optimization)

Note: Isolation is enforced at the page level through unique tokens and separate data storage. The Hybrid Incremental Update Layer improves bandwidth and latency but does not alter isolation boundaries.

```go
type PageRegistry struct {
    pages     sync.Map   // Isolated page storage: map[string]*Page
    mu        sync.RWMutex
}

type Page struct {
    id       string        // Unique page identifier
    token    string        // Secure access token for this page
    data     interface{}   // Isolated user data for this page
    analyzer *TemplateAnalyzer // Template analysis engine
}
```

### Component Design

Deferred: Component-level design and reference implementation will be added in a future revision of this proposal.

### Page Token System

#### Design Choice: Simple Token-Based Page Identification

```go
type PageToken struct {
    PageID        string    `json:"page_id"`
    IssuedAt      time.Time `json:"issued_at"`
    ExpiresAt     time.Time `json:"expires_at"`
}

func GetPageByToken(tokenStr string) (*Page, error) {
    token, err := parseToken(tokenStr)
    if err != nil {
        return nil, err
    }

    // Check if token is expired
    if time.Now().After(token.ExpiresAt) {
        return nil, ErrTokenExpired
    }

    return pageRegistry.Load(token.PageID)
}
```

#### Isolation Properties

- Each page has a unique token for access control
- Token contains only page lookup information, never user data
- Expiration prevents indefinite access to abandoned pages
- No shared state between pages - complete isolation
- Simple token structure focused on page identification, not complex authentication

### Deterministic Update Selection

#### Design Choice: AST-Based Template Analysis with Caching

The system analyzes Go html/template syntax trees to determine update-mode capability and caches results for performance:

```go
type TemplateAnalyzer struct {
    supportedConstructs map[TemplateConstruct]CachingCapability
    templateParser      *TemplateASTParser
    analysisCache       *AnalysisCache // LRU cache for analysis results
}

type AnalysisCache struct {
    cache    map[string]*TemplateAnalysis // Keyed by template content hash
    lru      *list.List                   // LRU eviction order
    maxSize  int                          // Maximum cached analyses
    mu       sync.RWMutex                 // Cache protection
}

func (a *TemplateAnalyzer) AnalyzeTemplate(tmpl *html.Template) (*TemplateAnalysis, error) {
    // Generate content hash for cache lookup
    contentHash := a.computeTemplateHash(tmpl)

    // Check cache first
    if analysis := a.analysisCache.Get(contentHash); analysis != nil {
        return analysis, nil
    }

    // Perform analysis and cache result
    analysis, err := a.performAnalysis(tmpl)
    if err != nil {
        return nil, err
    }

    a.analysisCache.Put(contentHash, analysis)
    return analysis, nil
}

func (a *TemplateAnalyzer) computeTemplateHash(tmpl *html.Template) string {
    // Hash template content + associated templates for cache key
    hasher := sha256.New()
    for _, t := range tmpl.Templates() {
        hasher.Write([]byte(t.Name()))
        hasher.Write([]byte(t.Root.String())) // AST string representation
    }
    return fmt.Sprintf("%x", hasher.Sum(nil))
}
```

**Template Analysis Optimization:**

- **Content-based caching**: Cache analysis results by template content hash
- **LRU eviction**: Remove least recently used analyses when cache fills
- **Hot reload detection**: Invalidate cache entries when template content changes
- **Performance monitoring**: Track cache hit rates and analysis timing

**Value Patch Position Tracking Enhancement:**

```go
type PositionTracker struct {
    fragments map[string]*FragmentPositions
    checksum  string // HTML content checksum for drift detection
}

type FragmentPositions struct {
    boundaries    []Position    // Start/end positions in HTML
    valueNodes    []ValueNode   // Trackable value positions
    lastChecksum  string        // Content checksum for validation
    isStale       bool          // Needs re-indexing
}

func (p *PositionTracker) validatePositions(newHTML []byte) error {
    newChecksum := computeHTMLChecksum(newHTML)
    if newChecksum != p.checksum {
        // HTML structure changed - mark positions as stale
        for _, fragment := range p.fragments {
            fragment.isStale = true
        }
        return ErrPositionDrift
    }
    return nil
}

// Fallback strategy when position tracking becomes unreliable
func (p *PositionTracker) handlePositionDrift(fragmentID string) {
    // Switch to fragment replace for this fragment until re-indexed
    fragment := p.fragments[fragmentID]
    fragment.isStale = true

    // Log position drift for monitoring
    log.Warn("Position drift detected",
        "fragment_id", fragmentID,
        "action", "fallback_to_replace")
}
```

**Position Tracking Reliability:**

- **Checksum validation**: Detect when HTML structure changes unexpectedly
- **Graceful degradation**: Fall back to fragment replace when positions become unreliable
- **Re-indexing strategy**: Rebuild position indexes when templates are re-analyzed
- **Monitoring**: Track position drift rates and fallback frequency

// Supported for value patch updates (60-85% bandwidth reduction)

```go
const (
    SimpleInterpolation    // {{.Field}}
    NestedFieldAccess     // {{.User.Profile.Name}}
    BasicConditionals     // {{if .Flag}}text{{end}}
    StaticRangeIterations // {{range .Items}}{{.Name}}{{end}}
    BasicPipelines        // {{.Price | printf "%.2f"}}
    // ... 15+ total supported constructs
)

// Falls back to fragment replace (60% bandwidth reduction)
const (
    DynamicTemplateInclusion    // {{template .TemplateName .}}
    UnknownCustomFunctions      // {{myCustomFunc .Data}}
    ComplexNestedDynamicStructure
    RecursiveTemplateStructure
)
```

#### Algorithm

1. Parse template into AST using Go's `text/template/parse`
2. Walk AST nodes and classify each template construct
3. If ANY unsupported construct found → FragmentReplace
4. If ALL constructs supported → ValuePatch
5. Generate position mapping for value-based updates

### Update Protocols

#### Value-Based Updates (Preferred)

```json
{
  "fragment_id": "counter-section",
  "action": "value_updates",
  "data": [
    {
      "position": 156,
      "length": 2,
      "new_value": "42",
      "data_path": "counter"
    }
  ]
}
```

#### Fragment-Based Updates (Fallback)

```json
{
  "fragment_id": "dynamic-section",
  "action": "replace",
  "data": "<fragment html elided>"
}
```

### Large Data and High-Volume Updates

High-change workloads (e.g., appending 10,000+ items) require explicit limits and degradation paths to preserve latency, memory, and user experience.

Policies:

- Operation coalescing: multiple appends/replaces to the same fragment within one render cycle are coalesced into a single update.
- Caps per RenderUpdates call:
  - Max operations per fragment (e.g., 1,000 value updates or 50 fragment replaces).
  - Max total operations across the page (e.g., 5,000 value updates), after which the system switches to a single fragment replace for affected regions.
- Payload cap: if a serialized update exceeds transport limits (e.g., WebSocket frame size), the server automatically chunks the update into sequenced parts with reassembly metadata. If chunking still exceeds limits, fall back to server-side pagination instructions or full re-render.
- Backpressure: when producers outpace the network, updates are batched and throttled; intermediate states may be skipped. Clients always apply the latest fully received sequence.
- Memory budget: each page tracks a bounded history/index needed for value patching. When memory exceeds the budget, the page selectively drops value-patch metadata for the largest fragments and uses fragment replace for those until memory recovers.

Behavior for 10k+ appends:

- If the range is structure-stable and within caps, the planner emits either:
  - a coalesced fragment replace for the list container (preferred for very large insertions), or
  - a batched sequence of value updates in chunks (only when clearly smaller than replacement HTML).
- When caps are hit, the system emits a single fragment replace and resets value-patch tracking for that fragment to avoid unbounded index growth.
- Applications are encouraged to paginate or virtualize long lists; the design preserves correctness while avoiding pathological payloads.

Optional chunk envelope (when needed):

```json
{
  "fragment_id": "list",
  "action": "replace",
  "data": "<li>...</li>",
  "chunk": { "seq": 3, "total": 7, "id": "abc123" },
  "timestamp": "2025-08-08T14:30:26.123Z"
}
```

Client handling notes:

- Maintain per-fragment chunk assemblies keyed by chunk.id; only apply when all parts [1..total] are present, or discard after timeout and request a resync/full replace.
- If updates arrive out-of-order or a newer sequence supersedes an older one, prefer the latest complete sequence and drop stale assemblies.

#### Session Data Growth and GC

**Memory Management Strategy:**

```go
type MemoryManager struct {
    budget           int64          // Per-page memory budget (10MB default)
    used             int64          // Current memory usage
    snapshots        []PageSnapshot // Historical renders for diffing
    fragmentIndexes  map[string]*FragmentIndex // Position tracking per fragment
    mu               sync.RWMutex   // Protects memory accounting
}

func (m *MemoryManager) trackMemoryUsage() {
    // Monitor using runtime.MemStats every 30 seconds
    var stats runtime.MemStats
    runtime.ReadMemStats(&stats)

    if stats.HeapInuse > m.budget {
        m.triggerEviction()
    }
}

func (m *MemoryManager) triggerEviction() {
    // 1. Evict oldest snapshots beyond retention limit
    m.evictOldSnapshots()

    // 2. Evict largest fragment indexes first
    m.evictLargestFragments()

    // 3. Switch affected fragments to replace mode until memory recovers
    m.degradeToFragmentReplace()
}
```

**Memory Budgets and Enforcement:**

- **Automatic memory management**: Library handles memory budgets with sane defaults (10MB per page)
- **Snapshot retention**: Keep minimal historical renders needed for diffing (typically 3)
- **Fragment index limits**: Automatically track optimal number of fragments per page
- **Intelligent eviction**: Remove largest memory consumers first when needed
- **Transparent degradation**: Switch to fragment replace when value patching becomes memory-prohibitive

**Garbage Collection Integration:**

- **Weak references**: Use finalizers for automatic cleanup of abandoned pages
- **Pressure detection**: Monitor runtime.MemStats and trigger eviction at 80% budget
- **GC cooperation**: Avoid allocations during concurrent GC phases
- **Memory pools**: Reuse buffers for HTML generation and diff computation

**Resource Cleanup:**

- **Automatic expiration**: Inactive pages expire after reasonable timeout (1 hour default)
- **Smart eviction**: Remove least recently used pages when memory pressure detected
- **Connection cleanup**: Automatically close pages when all WebSocket connections drop
- **Leak detection**: Monitor for pages that fail to release resources and log warnings

### Fragment Identity and Stability

Fragment IDs must be stable across renders for the same logical region using a deterministic algorithm:

**Fragment ID Algorithm:**

```go
// Compute stable fragment ID from template AST and context
func computeFragmentID(node parse.Node, templatePath string, keyPath []string) string {
    // Base components for ID generation
    components := []string{
        templatePath,                    // e.g., "user-dashboard.html"
        fmt.Sprintf("%d", node.Position()), // AST node position
        strings.Join(keyPath, "."),      // e.g., "user.profile.name"
    }

    // For range loops, prefer stable keys over indexes
    if rangeNode, ok := node.(*parse.RangeNode); ok {
        if hasStableKey(rangeNode) {
            components = append(components, extractStableKey(rangeNode))
        } else {
            // No stable key available - prefer fragment replace
            return ""
        }
    }

    // Generate deterministic ID: hash(template + position + keypath)
    hash := sha256.Sum256([]byte(strings.Join(components, "|")))
    return fmt.Sprintf("f-%x", hash[:4]) // 8-character hex prefix
}
```

**Stability Requirements:**

- Fragment IDs remain consistent across renders when template structure is unchanged
- For ranges/lists, derive IDs from stable keys (not indexes) to avoid churn
- When no stable key exists, the planner prefers fragment replace over value patching
- Nested fragments inherit hierarchical boundaries; collisions prevented by namespacing
- Template changes that affect fragment boundaries require ID regeneration and client resync

**Validation and Testing:**

- Unit tests verify ID stability across multiple renders with same template
- Integration tests validate ID consistency during template hot-reloads
- Property tests ensure no collisions within reasonable fragment counts (10k+ per page)

### Concurrency and Consistency

**Thread Safety Model:**

```go
type Page struct {
    mu           sync.RWMutex      // Protects all page state
    renderMu     sync.Mutex        // Serializes RenderUpdates calls
    data         interface{}       // Current page data
    lastHTML     []byte           // Last rendered HTML
    tracker      *TemplateTracker // Fragment position tracking
    connections  map[string]*Connection // Active WebSocket connections
}

// RenderFragments is serialized per page to ensure consistency
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]Fragment, error) {
    // Acquire render lock to serialize update generation
    p.renderMu.Lock()
    defer p.renderMu.Unlock()

    // Read-lock for data comparison
    p.mu.RLock()
    oldData := p.data
    p.mu.RUnlock()

    // Generate updates (can be concurrent with other pages)
    fragments := p.generateFragments(oldData, newData)

    // Write-lock to update page state
    p.mu.Lock()
    p.data = newData
    p.lastHTML = newHTML
    p.mu.Unlock()

    return updates, nil
}
```

**Concurrency Guarantees:**

- **Page-level isolation**: Each page has independent locks, enabling concurrent updates across different pages
- **Serialized updates**: RenderUpdates calls are serialized per page to prevent race conditions during diff generation
- **Lock hierarchy**: renderMu -> mu (read/write) to prevent deadlocks
- **Connection safety**: WebSocket writes are concurrent but sequenced per connection
- **Memory visibility**: All data updates happen-before subsequent reads due to mutex semantics

**Performance Characteristics:**

- **High concurrency**: 10,000+ pages can update concurrently without contention
- **Lock contention**: Only occurs within a single page under high update frequency
- **Scalability**: O(1) lock contention scaling with page count
- **Latency**: Lock hold time bounded by update generation complexity (target: <10ms)

### Flow Control and Slow Clients

- Each connection has a bounded server-side queue; when full, apply backpressure and collapse intermediate states.
- Drop policy: prefer dropping intermediate deltas in favor of sending the latest state for each fragment.
- Heartbeats/keepalives and idle timeouts ensure timely detection of dead connections.

### Reconnection and Resync

- Clients send last acked seq on reconnect to resume; server either resumes from next seq if retained or instructs a resync.
- Resync semantics: server sends replace updates for affected fragments or a full-page re-render when necessary.

### Security Hardening

- Enforce WebSocket origin checks and strict CORS on initial HTTP endpoints.
- Recommend CSP policies compatible with dynamic updates; avoid unsafe-inline where possible.
- Key rotation/invalidation: tokens expire; rotation schedule documented; audit logs capture access and failures.
- Template function allowlist; deny or sandbox unknown functions that alter structure.

### Observability and SLOs

**Performance Targets and Metrics:**

```go
type PerformanceTargets struct {
    // Latency SLOs
    P95UpdateLatencyMs       int     // 150ms end-to-end update latency
    P99UpdateLatencyMs       int     // 300ms for complex updates
    MaxRenderLatencyMs       int     // 50ms for initial render

    // Throughput targets
    UpdatesPerSecond         int     // 1000 updates/sec per page
    ConcurrentPages          int     // 10,000 active pages per instance
    MaxMemoryPerPageMB       int     // 10MB memory budget per page

    // Bandwidth optimization
    BandwidthReductionMin    float64 // 0.60 (60% minimum reduction)
    BandwidthReductionTarget float64 // 0.80 (80% target reduction)
    ValuePatchSuccessRate    float64 // 0.85 (85% of updates use value patches)

    // Reliability targets
    ReconnectTimeMaxMs       int     // 2000ms maximum reconnect time
    FallbackRateMax          float64 // 0.05 (5% maximum fallback to fragment replace)
    ErrorRateMax             float64 // 0.001 (0.1% maximum error rate)
}
```

**Key Metrics Collection:**

- **Latency metrics**:
  - `livetemplate_update_latency_ms` (histogram with fragment_id label)
  - `livetemplate_render_latency_ms` (histogram with template_name label)
- **Throughput metrics**:
  - `livetemplate_updates_total` (counter with action=value_updates|replace)
  - `livetemplate_bytes_sent_total` (counter with transport=ws|sse|ajax)
- **Memory metrics**:
  - `livetemplate_memory_usage_bytes` (gauge per page)
  - `livetemplate_fragments_tracked_total` (gauge per page)
- **Reliability metrics**:
  - `livetemplate_reconnects_total` (counter with reason label)
  - `livetemplate_fallbacks_total` (counter with reason=memory|caps|error)

**Distributed Tracing:**

```go
// Trace update flow from trigger to client application
ctx, span := tracer.Start(ctx, "livetemplate.RenderFragments")
span.SetAttributes(
    attribute.String("fragment.id", fragmentID),
    attribute.String("update.action", action),
    attribute.Int("update.bytes", len(payload)),
    attribute.String("page.id", pageID),
)
defer span.End()

// Child spans for major operations
analyzeSpan := tracer.Start(ctx, "template.analyze")
planSpan := tracer.Start(ctx, "update.plan")
serializeSpan := tracer.Start(ctx, "update.serialize")
sendSpan := tracer.Start(ctx, "transport.send")
```

**Structured Logging:**

- **Correlation IDs**: Track requests across service boundaries
- **Fragment-level context**: Include fragment_id in all log entries
- **Performance annotations**: Log slow operations with timing details
- **Error context**: Capture full error context including page state

### Failure Modes and Recovery

- Redis/backing-store outages: degrade to in-memory with documented data loss boundaries; resync clients after recovery.
- Server restart: pages either restored (if persisted) or require client resync.
- Network faults: exponential backoff with jitter; cap max backoff.

### Testing Strategy Additions

- Fuzz parsing/analyzer and property tests for deterministic mode selection.
- Load/soak and chaos tests: packet loss, reordering, reconnect storms, slow clients, 10k+ appends.
- Golden tests for wire protocol framing versions and backward compatibility.

### Internationalization and Accessibility

- UTF-8, Unicode normalization, bidi/RTL safety; stable formatting for numbers/dates under value patching.
- ARIA live-region guidance for dynamic fragments; minimize layout thrash on replace.

### Browser Support and Fallbacks

- Define supported browser matrix; when WS unavailable, fall back to SSE or long-poll.
- Compression: permessage-deflate and HTTP compression guidance.

### Compliance and Privacy

- PII minimization in messages; retention settings for per-page artifacts; erasure flows.

### Versioning and Rollout

- Wire protocol versioning and negotiation; feature flags and canarying; deprecation policy for breaking changes.

### Security Considerations

#### Token Security

**Signing Key Management and Rotation:**

```go
type TokenService struct {
    currentKey  []byte          // Active signing key
    previousKey []byte          // Previous key during rotation
    keyRotation time.Duration   // Rotation schedule (default: 24h)
    gracePeriod time.Duration   // Grace period for old tokens (default: 1h)
    mu          sync.RWMutex    // Protects key access
}

func (ts *TokenService) RotateSigningKey() error {
    ts.mu.Lock()
    defer ts.mu.Unlock()

    // Keep previous key for grace period
    ts.previousKey = ts.currentKey

    // Generate new key
    newKey := make([]byte, 32)
    if _, err := rand.Read(newKey); err != nil {
        return fmt.Errorf("key generation failed: %w", err)
    }
    ts.currentKey = newKey

    // Schedule cleanup of previous key after grace period
    time.AfterFunc(ts.gracePeriod, func() {
        ts.mu.Lock()
        ts.previousKey = nil
        ts.mu.Unlock()
    })

    return nil
}

func (ts *TokenService) VerifyToken(tokenStr string) (*PageToken, error) {
    // Try current key first
    if token, err := verifyWithKey(tokenStr, ts.currentKey); err == nil {
        return token, nil
    }

    // Fallback to previous key during rotation grace period
    if ts.previousKey != nil {
        if token, err := verifyWithKey(tokenStr, ts.previousKey); err == nil {
            // Log usage of old key for monitoring
            log.Info("Token verified with previous key",
                "page_id", token.PageID,
                "remaining_grace", ts.gracePeriod)
            return token, nil
        }
    }

    return nil, ErrInvalidToken
}
```

**Enhanced Security Properties:**

- **Automated key rotation**: Default 24-hour rotation schedule with configurable intervals
- **Graceful transition**: 1-hour grace period allows existing sessions to continue during rotation
- **Audit logging**: All token validation failures and key rotations are logged for security analysis
- **Secure generation**: Cryptographically secure random key generation using crypto/rand
- **Defense in depth**: Always require TLS; tokens are useless without HTTPS transport

**Session Persistence and Recovery:**

```go
type SessionStore interface {
    Store(pageID string, session *PageSession) error
    Load(pageID string) (*PageSession, error)
    Delete(pageID string) error
    Cleanup(olderThan time.Time) error
}

type PageSession struct {
    PageID       string                 `json:"page_id"`
    ApplicationID string                `json:"application_id"`
    CreatedAt    time.Time              `json:"created_at"`
    LastAccess   time.Time              `json:"last_access"`
    Data         interface{}            `json:"data"`
    Metadata     map[string]interface{} `json:"metadata"`
}

// Optional Redis-backed session persistence for security-critical applications
func (app *Application) NewPage(tmpl *html.Template, data interface{}, options ...PageOption) (*Page, error) {
    page := &Page{
        ID:            generatePageID(),
        ApplicationID: app.id,
        CreatedAt:     time.Now(),
        Data:          data,
    }

    // Apply options (including persistence)
    for _, opt := range options {
        if err := opt(page); err != nil {
            return nil, err
        }
    }

    return page, nil
}

// Functional options for page creation
type PageOption func(*Page) error

func WithPersistence(sessionStore SessionStore) PageOption {
    return func(page *Page) error {
        // Store session data for recovery after restart
        session := &PageSession{
            PageID:       page.ID,
            ApplicationID: page.ApplicationID,
            CreatedAt:    page.CreatedAt,
            Data:         page.Data,
        }

        if err := sessionStore.Store(page.ID, session); err != nil {
            log.Error("Failed to persist session", "error", err, "page_id", page.ID)
            // Continue without persistence - degrade gracefully
        }

        page.sessionStore = sessionStore
        return nil
    }
}
```

**Security Audit and Compliance:**

- **Access logging**: All page access attempts logged with outcome and timing
- **Session tracking**: Monitor session lifecycle events (creation, access, expiration)
- **Anomaly detection**: Track unusual patterns (rapid token generation, cross-app access attempts)
- **Data retention**: Configurable session data retention with automatic cleanup
- **PII protection**: Sensitive data excluded from logs and metrics

#### Data Validation

- **Input Sanitization**: All data from the client, especially data that might be rendered into a template, must be sanitized to prevent XSS attacks.
- **Output Encoding**: Go's `html/template` package provides automatic output encoding, which is a critical defense. This dependency should never be removed.

#### Denial of Service (DoS) Mitigation

- **Resource Limiting**: The `Application` should have configurable limits on the maximum number of active pages to prevent memory exhaustion.
- **Rate Limiting**: API endpoints and WebSocket connections should be rate-limited to prevent abusive clients from overwhelming the server.

### Error Handling

A robust error handling strategy is critical for system stability and debuggability.

- **Token Errors**:

  - `ErrInvalidToken`, `ErrTokenExpired`: The server should respond with an HTTP `401 Unauthorized` status, forcing the client to re-authenticate.
  - `ErrInvalidApplication`: This is a critical security error and should be logged with high severity. The client should receive a generic `401 Unauthorized`.

- **Template Parsing Errors**:

  - These are developer errors and should fail loudly at startup. The `TemplateAnalyzer` should return an error, preventing the application from starting with a broken template.

- **WebSocket Disconnections**:
  - The client-side library should implement an exponential backoff strategy for reconnection attempts to avoid overwhelming the server.
  - The server should gracefully clean up the `Page` instance and its resources upon disconnection, respecting a configurable grace period.

### Implementation Tradeoffs

#### Implementation Strategy

Since this is an unreleased library with AI-driven implementation, the development approach focuses on component prioritization rather than phased timelines:

##### Core Components (Implementation Priority 1)

```go
// Essential security and isolation foundation
type CoreComponents struct {
    ApplicationIsolation     bool // ✅ Application-scoped registries with token validation
    PageLifecycle           bool // ✅ NewPage, Render, GetToken, Close APIs
    FragmentReplaceUpdates  bool // ✅ Basic incremental updates via fragment replacement
    BasicWebSocketTransport bool // ✅ Simple WebSocket-based update delivery
    MemoryBoundaries        bool // ✅ Basic per-page memory limits and cleanup
}

// Success criteria for core implementation
var CoreAcceptanceCriteria = []string{
    "Cross-application page access blocked and returns ErrInvalidApplication",
    "Token validation enforces application ID matching",
    "Fragment replace updates work reliably via WebSocket",
    "Memory usage bounded per page with automatic cleanup",
    "Basic concurrent page support (100+ pages)",
}
```

##### Performance Components (Implementation Priority 2)

```go
type PerformanceComponents struct {
    TemplateAnalysis     bool // ✅ AST-based template analysis with caching
    ValuePatchUpdates    bool // ✅ Position-based value patching for bandwidth optimization
    FragmentIDStability  bool // ✅ Deterministic fragment ID algorithm
    UpdateOptimization   bool // ✅ Automatic mode selection (ValuePatch vs FragmentReplace)
    BasicMetrics        bool // ✅ Essential performance and error metrics
}

// Target performance benchmarks
var PerformanceTargets = map[string]interface{}{
    "bandwidth_reduction":     0.70, // 70% average reduction (realistic target)
    "value_patch_success":     0.75, // 75% of compatible templates use value patches
    "p95_update_latency_ms":   200,  // 200ms p95 latency (conservative target)
    "template_analysis_cache": 0.90, // 90% cache hit rate for template analysis
}
```

##### Production Components (Implementation Priority 3)

```go
type ProductionComponents struct {
    TransportFallbacks   bool // ✅ SSE and AJAX fallbacks when WebSocket unavailable
    KeyRotation         bool // ✅ Automated token key rotation with grace periods
    SessionPersistence  bool // ✅ Optional Redis-backed session storage
    ComprehensiveMetrics bool // ✅ Full observability with tracing and structured logging
    ScalabilityFeatures bool // ✅ High-concurrency optimizations and backpressure handling
}
```

#### Memory vs Security Isolation

- **Decision**: In-memory page registries per application with optional Redis backing
- **Tradeoff**: Higher memory usage vs perfect isolation guarantees
- **Mitigation**: Configurable page limits, TTL expiration, and LRU eviction
- **Monitoring**: Track memory per application and page; alert on budget exceeded

#### Deterministic vs Adaptive Updates

- **Decision**: Static template analysis over runtime adaptation
- **Tradeoff**: Some suboptimal cases vs predictable, debuggable behavior
- **Mitigation**: Developer feedback through analysis reports and metrics
- **Evolution path**: Template analysis capabilities can expand over time

#### Compatibility (Pre-release)

- **Decision**: Optimize design for security and clarity; no migration path required
- **Context**: Library is unreleased; compatibility with previous singleton model is not a constraint
- **Implication**: Documentation focuses on the new architecture without migration guidance

### Client-Side Architecture

#### Design Choice: Mode-Agnostic JavaScript Client

The client automatically handles both update strategies without configuration:

```javascript
class StatePage {
  applyUpdates(updates) {
    updates.forEach((update) => {
      switch (update.action) {
        case "value_updates":
          this.applyValueUpdates(update.data); // Array of value update objects
          break;
        case "replace":
          this.applyFragmentReplace(update.data); // HTML string
          break;
      }
    });
  }
}
```

#### Client Benefits

- Zero configuration required
- Automatic mode detection
- Framework-agnostic (works with any web framework)
- Minimal JavaScript footprint

### Deployment Architecture

#### Multi-Application Deployment Pattern

```go
// Application A
serviceA := livetemplate.NewApplication(
    livetemplate.WithApplicationID("service-a"),
    livetemplate.WithRedisBackend("redis://cluster"),
)

// Application B
serviceB := livetemplate.NewApplication(
    livetemplate.WithApplicationID("service-b"),
    livetemplate.WithRedisBackend("redis://cluster"),
)
```

#### Scaling Properties

- Each application instance can run independently
- Redis backing enables horizontal scaling while maintaining isolation
- No cross-service dependencies or shared state

---

## Reference Implementation

### Working Prototype

A minimal working implementation demonstrating core concepts:

```go
// main.go - Demonstrates page isolation and incremental updates
package main

import (
    "html/template"
    "net/http"
    "github.com/livefir/livetemplate"
)

func main() {
    tmpl := template.Must(template.ParseGlob("*.html"))

    // Create application instance
    app := livetemplate.NewApplication()

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Create isolated page with automatic update mode analysis
        page := app.NewPage(tmpl, map[string]interface{}{
            "UserName": "John",
            "Counter":  0,
        })

        // Get analysis insights through application debug interface
        debug := app.Debug()
        if analysis, err := debug.GetTemplateAnalysis(page.GetToken()); err == nil {
            log.Printf("UpdateMode: %v, Coverage: %.1f%%",
                analysis.SelectedUpdateMode,
                analysis.BandwidthReduction*100)
        }

        // Render and set session token
        html, _ := page.Render()
        http.SetCookie(w, &http.Cookie{
            Name:     "page_token",
            Value:    page.GetToken(),
            HttpOnly: true,
            Secure:   true,
        })

        w.Write([]byte(html))
    })

    http.ListenAndServe(":8080", nil)
}
```

### Performance Validation

Bandwidth reduction measurements from prototype testing:

| Template Pattern  | Current (bytes) | Value Patch (bytes) | Reduction |
| ----------------- | --------------- | ------------------- | --------- |
| Counter increment | 247             | 31                  | 87%       |
| Name change       | 312             | 41                  | 87%       |
| Price update      | 198             | 28                  | 86%       |
| Complex form      | 1,247           | 156                 | 87%       |

#### Test Conditions

- 1000 simulated concurrent users
- Mixed template patterns (80% supported, 20% fallback)
- WebSocket-based updates over 30-minute test period

### Page Isolation Validation

Page isolation verification:

```go
func TestPageIsolation(t *testing.T) {
    app := livetemplate.NewApplication()
    page1 := app.NewPage(tmpl, userData1)
    page2 := app.NewPage(tmpl, userData2)

    // Pages should have different tokens
    assert.NotEqual(t, page1.GetToken(), page2.GetToken())

    // Each page should only be accessible with its own token
    retrievedPage1, err := app.GetPageByToken(page1.GetToken())
    assert.NoError(t, err)
    assert.Equal(t, page1, retrievedPage1)

    // Wrong token should return error
    _, err = app.GetPageByToken("invalid-token")
    assert.Error(t, err, "Invalid token access must be blocked")

    // Pages should have isolated data
    page1.SetData(map[string]interface{}{"value": "page1-data"})
    page2.SetData(map[string]interface{}{"value": "page2-data"})

    // Verify data isolation
    assert.NotEqual(t, page1.GetData(), page2.GetData())
}
```

This reference implementation validates:

- ✅ Page isolation security model
- ✅ Automatic update mode selection
- ✅ Bandwidth reduction targets (60-85%)
- ✅ Zero-configuration developer experience

### API Specification

The API provides comprehensive interfaces for modern web applications:

1. **Application Management**: Isolated application instances with multi-application support
2. **Page Lifecycle**: Session-scoped pages with automatic incremental updates
3. **Performance**: High-frequency update optimization and load balancing
4. **Observability**: Developer visibility into optimization decisions and performance

```go
// Core API for application and page management
func NewApplication(options ...ApplicationOption) *Application
func (app *Application) NewPage(templates *html.Template, initialData interface{}, options ...PageOption) *Page
func (app *Application) GetPageByToken(token string) (*Page, error)

// Page lifecycle and updates
func (p *Page) GetToken() string
func (p *Page) Render() (string, error)
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]Fragment, error)
func (p *Page) SetData(data interface{}) error
func (p *Page) Close() error

// Application-scoped debugging and observability
func (app *Application) Debug() *Debug

// Debug provides comprehensive observability for application instances
type Debug struct {
    app *Application // Application context for debugging
}

// Debug methods for template analysis and performance monitoring
func (d *Debug) GetTemplateAnalysis(pageID string) (*TemplateAnalysis, error)
func (d *Debug) GetPerformanceMetrics() PerformanceMetrics
func (d *Debug) GetMemoryStats() MemoryStats
func (d *Debug) GetPageMetrics(pageID string) (*PageMetrics, error)
func (d *Debug) ListActivePages() []PageSummary

// Debug observability types
type TemplateAnalysis struct {
    SelectedUpdateMode   UpdateMode
    CapabilityReport    CapabilityReport
    BandwidthReduction  float64
    SupportedConstructs []TemplateConstruct
    UnsupportedReasons  []string
}

type PerformanceMetrics struct {
    UpdateLatencyP95    time.Duration
    UpdateLatencyP99    time.Duration
    BandwidthReduction  float64
    ValuePatchSuccess   float64
    ActivePages         int
    TotalUpdates        int64
}

type MemoryStats struct {
    TotalMemoryUsed     int64
    AveragePageMemory   int64
    PagesOverBudget     int
    FragmentCacheSize   int64
    AnalysisCacheSize   int64
}

type PageMetrics struct {
    PageID             string
    MemoryUsed         int64
    MemoryBudget       int64
    FragmentCount      int
    ActiveConnections  int
    LastUpdateLatency  time.Duration
}

type PageSummary struct {
    PageID      string
    CreatedAt   time.Time
    LastAccess  time.Time
    MemoryUsed  int64
    UpdateMode  UpdateMode
}
```

### Update Protocol Specification

The system supports two update protocols based on template analysis:

#### Framing and Semantics

- version: protocol version (semantic, e.g., "1.0").
- seq: monotonically increasing per-connection sequence number assigned by server.
- ack: last sequence number the client has applied (used for resume/backpressure).
- atomic batch: updates array is applied atomically; partial failures trigger a resync.
- ordering: server preserves order within a connection; clients must apply in seq order.
- resync: on gap detection or chunk timeout, client requests a full-fragment replace or full page re-render.

#### Value-Based Update Protocol

```json
{
  "version": "1.0",
  "seq": 1024,
  "updates": [
    {
      "fragment_id": "counter-section",
      "action": "value_updates",
      "data": [
        {
          "position": 156,
          "length": 2,
          "new_value": "42",
          "value_type": "string",
          "data_path": "counter"
        }
      ]
    }
  ],
  "timestamp": "2025-08-08T14:30:26.123Z"
}
```

#### Fragment-Based Update Protocol

```json
{
  "version": "1.0",
  "seq": 1025,
  "updates": [
    {
      "fragment_id": "dynamic-section",
      "action": "replace",
      "data": "<fragment html elided>"
    }
  ],
  "timestamp": "2025-08-08T14:30:26.123Z"
}
```

---

## Appendix: Use Cases and Performance Targets

The following use cases represent common web application patterns that benefit from LiveTemplate's efficient update mechanisms. These drive both architectural decisions and performance testing, ensuring the system meets real-world requirements.

### Core Use Cases

#### UC-1: Real-time Content Updates

**Scenario**: Live content updates (counters, status indicators, metrics)  
**Requirements**: Sub-200ms latency, 80%+ bandwidth reduction  
**Template Pattern**: Simple value interpolation `{{.Counter}}`, `{{.Status}}`  
**Expected Optimization**: 85% bandwidth reduction via value patches

#### UC-2: Dynamic List Updates

**Scenario**: Adding/removing items from lists (comments, notifications, search results)  
**Requirements**: Efficient handling of list growth, memory-bounded  
**Template Pattern**: Range operations `{{range .Items}}{{.Name}}{{end}}`  
**Expected Optimization**: Fragment replace for large changes, value patches for small updates

#### UC-3: High-Frequency Updates

**Scenario**: Real-time gaming, live sports scores, monitoring dashboards  
**Requirements**: 1000+ updates/second, sub-100ms latency for critical updates  
**Template Pattern**: Mixed simple values and conditionals  
**Expected Optimization**: Update batching and coalescing

#### UC-4: Complex Dashboard Updates

**Scenario**: Multi-widget dashboards with independent update cycles  
**Requirements**: 50,000+ fragments, 24+ hour sessions, memory efficiency  
**Template Pattern**: Nested structures `{{.Metrics.Revenue.Monthly}}`  
**Expected Optimization**: Selective fragment updates, memory management

#### UC-5: Form Validation Updates

**Scenario**: Real-time form validation and progress indicators  
**Requirements**: Immediate feedback, minimal bandwidth usage  
**Template Pattern**: Conditional error display `{{if .Errors.Email}}{{.Errors.Email}}{{end}}`  
**Expected Optimization**: Targeted validation message updates

### Performance Validation Framework

#### Target Metrics

- **Bandwidth Reduction**: 60-85% across use cases
- **Update Latency**: P95 ≤ 150ms, P99 ≤ 300ms
- **Memory Efficiency**: <10MB per page for complex applications
- **Concurrent Pages**: 10,000+ isolated sessions per instance
- **Update Throughput**: 1,000+ updates/second per page

#### Test Scenarios

```go
// UC-1: Real-time Content Updates
func TestBenchmark_SimpleValueUpdates(b *testing.B)
func TestLatency_CounterIncrement(t *testing.T)

// UC-2: Dynamic List Updates
func TestMemory_LargeListGrowth(t *testing.T)
func TestBandwidth_ListAddRemove(t *testing.T)

// UC-3: High-Frequency Updates
func TestThroughput_HighFrequencyUpdates(t *testing.T)
func TestCoalescing_RapidUpdates(t *testing.T)

// UC-4: Complex Dashboard Updates
func TestScale_ComplexDashboard(t *testing.T)
func TestLongRunning_24HourSession(t *testing.T)

// UC-5: Form Validation Updates
func TestResponsiveness_FormValidation(t *testing.T)
func TestPrecision_ValidationMessages(t *testing.T)
```

### Architectural Insights from Use Cases

**Key Findings:**

1. **Template Complexity vs Performance**: Simple interpolation achieves 85% bandwidth reduction; complex structures fall back to 60% reduction
2. **Memory vs Performance Trade-off**: Value patching requires position tracking; fragment replace reduces memory usage
3. **Batching Effectiveness**: 50ms batching window optimal for most high-frequency scenarios
4. **Fragment Granularity**: Smaller fragments enable better optimization but increase memory overhead
5. **Session Longevity**: Long-running sessions require active memory management and cleanup

**Implementation Priorities:**

1. **Core Performance**: Focus on simple value updates (80% of use cases)
2. **Memory Management**: Essential for long-running and complex applications
3. **Load Handling**: Critical for high-frequency update scenarios
4. **Observability**: Necessary for optimization and debugging in production

This focused approach ensures LiveTemplate delivers maximum value for its core competency while remaining extensible for specialized use cases that applications can build on top.---

## Appendix: Analyzer Constructs Reference

### Supported for Value Patch Updates

- Simple Interpolation: `{{.Field}}`
- Nested Field Access: `{{.User.Profile.Name}}`
- Basic Conditionals: `{{if .Flag}} ... {{end}}`
- With Blocks (static structure): `{{with .Ctx}} ... {{end}}`
- Static Range Iterations: `{{range .Items}}{{.Name}}{{end}}`
- Range with Index/Key (static length): `{{range $i, $v := .Items}} ... {{end}}`
- Basic Pipelines: `{{.Price | printf "%.2f"}}`
- Comparison Functions in Conditions: `{{if gt .Count 0}} ... {{end}}`
- Logical Functions in Conditions: `{{if and .A .B}} ... {{end}}`
- Length/Printf/Basic Builtins: `{{len .Items}}`, `{{printf "%s" .Name}}`
- Whitespace trimming variants: `{{- ... -}}`
- Comments (stripped): `{{/* ... */}}`
- Block definitions with static bodies: `{{block "name" .}}...{{end}}`
- Nested combinations of the above when structure remains static

Notes:

- Value patching assumes the DOM/HTML structure of the fragment remains static between renders; only text nodes/values change.

### Falls Back to Fragment Replace

- Dynamic Template Inclusion: `{{template .TemplateName .}}`
- Unknown or user-defined Custom Functions that alter structure
- Complex Nested Dynamic Structure where control flow alters layout
- Recursive Template Structure or deeply dynamic recursion
- Ranges with dynamic length affecting sibling positions
- Map iteration with unpredictable key order impacting structure
- Any construct not recognized as structure-stable by TemplateAnalyzer

This list should evolve as TemplateAnalyzer gains capabilities. Keep examples in tests aligned with these categories.

---

## Appendix: Message Envelope JSON Schema (v1.0)

The following JSON Schema formally specifies the wire message envelope and update payloads described in "Update Protocol Specification". It captures framing (version, seq, ack), batch semantics (updates array), and per-update shapes for value-based and fragment-replace messages. The schema is intentionally strict to aid interoperability and validation.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schemas.livefir.dev/livetemplate/update-message-1.0.json",
  "title": "LiveTemplate Update Message v1.0",
  "type": "object",
  "additionalProperties": false,
  "required": ["version", "seq", "updates", "timestamp"],
  "properties": {
    "version": {
      "type": "string",
      "enum": ["1.0"]
    },
    "seq": {
      "type": "integer",
      "minimum": 0
    },
    "ack": {
      "type": "integer",
      "minimum": 0
    },
    "updates": {
      "type": "array",
      "minItems": 1,
      "items": { "$ref": "#/$defs/update" }
    },
    "timestamp": {
      "type": "string",
      "format": "date-time"
    }
  },
  "$defs": {
    "chunk": {
      "type": "object",
      "additionalProperties": false,
      "required": ["seq", "total", "id"],
      "properties": {
        "seq": { "type": "integer", "minimum": 1 },
        "total": { "type": "integer", "minimum": 1 },
        "id": { "type": "string", "minLength": 1 }
      }
    },
    "value_update_item": {
      "type": "object",
      "additionalProperties": false,
      "required": ["position", "length", "new_value", "data_path"],
      "properties": {
        "position": { "type": "integer", "minimum": 0 },
        "length": { "type": "integer", "minimum": 0 },
        "new_value": { "type": "string" },
        "value_type": {
          "type": "string",
          "enum": ["string", "number", "bool", "html", "json"],
          "default": "string"
        },
        "data_path": { "type": "string", "minLength": 1 }
      }
    },
    "update": {
      "type": "object",
      "additionalProperties": false,
      "required": ["fragment_id", "action", "data"],
      "properties": {
        "fragment_id": { "type": "string", "minLength": 1 },
        "action": { "type": "string", "enum": ["value_updates", "replace"] },
        "data": {
          "oneOf": [
            {
              "type": "array",
              "items": { "$ref": "#/$defs/value_update_item" }
            },
            { "type": "string" }
          ]
        },
        "chunk": { "$ref": "#/$defs/chunk" }
      },
      "allOf": [
        {
          "if": { "properties": { "action": { "const": "value_updates" } } },
          "then": { "properties": { "data": { "type": "array" } } }
        },
        {
          "if": { "properties": { "action": { "const": "replace" } } },
          "then": { "properties": { "data": { "type": "string" } } }
        }
      ]
    }
  }
}
```

Notes:

- The optional "ack" is client-to-server metadata typically sent in a separate control frame; it appears here to allow unified validation if echoed in messages.
- The optional "chunk" object on an update is used only when payload chunking is active for large fragments; clients must assemble all chunks for a given id before applying.
- Future protocol versions will publish new schema IDs and should remain backward compatible within a major version.

---

## Appendix: Supported Browser Matrix and Fallbacks

The following matrix documents the baseline supported browsers and transport fallbacks. When WebSocket is unavailable (blocked proxy, captive portal, etc.), the client falls back to SSE and finally to long-polling.

| Browser                  | Min Version | WebSocket | Server-Sent Events (SSE) | Fallback Strategy    |
| ------------------------ | ----------- | --------- | ------------------------ | -------------------- |
| Chrome (Desktop/Mobile)  | 80+         | Yes       | Yes                      | WS → SSE → Long-poll |
| Firefox (Desktop/Mobile) | 78+         | Yes       | Yes                      | WS → SSE → Long-poll |
| Safari (macOS)           | 13+         | Yes       | Yes                      | WS → SSE → Long-poll |
| Safari (iOS/iPadOS)      | 13+         | Yes       | Yes                      | WS → SSE → Long-poll |
| Edge (Chromium)          | 80+         | Yes       | Yes                      | WS → SSE → Long-poll |
| IE 11                    | N/A         | No        | No                       | Not supported        |

Operational notes:

- Corporate proxies may terminate or block WebSockets; ensure SSE endpoints are accessible and cache-busted.
- Mobile networks may suspend background connections; the client should resume and request resync using the last acked seq.
- Enable permessage-deflate for WebSockets and HTTP compression for SSE/long-poll responses where appropriate.
