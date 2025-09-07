# LiveTemplate: Low Level Design (LLD) - First Public Release

## Overview

This document provides the low-level design specification for implementing LiveTemplate's first public release (v1.0). The focus is on fast and efficient update generation with secure session isolation, leaving network transport to the application layer.

**What this document covers:**

- Concrete component specifications for v1.0 implementation
- Security architecture with JWT-based multi-tenant isolation  
- Tree-based optimization system with single unified strategy
- Realistic 60-task implementation roadmap (transport removed)
- Test-driven development strategy
- Production-ready operational requirements

**Target audience:** Software engineers implementing LiveTemplate v1.0, with practical implementation details focused on reliability over aggressive optimization.

**Key design principles:**

- **Security First**: Multi-tenant isolation with comprehensive testing
- **Performance First**: Tree-based optimization achieving 92%+ bandwidth savings
- **Zero-configuration API**: Works out-of-the-box with sensible defaults
- **Focused Scope**: Update generation only - no transport layer complexity
- **Operational Excellence**: Comprehensive metrics and error handling

---

## Table of Contents

1. [API Boundary and Package Structure](#api-boundary-and-package-structure)
2. [Architecture Overview](#architecture-overview)  
3. [Core Components](#core-components)
4. [Test-Driven Development Strategy](#test-driven-development-strategy)
5. [Implementation Roadmap](#implementation-roadmap)
6. [Operational Requirements](#operational-requirements)
7. [Appendix](#appendix)

---

## API Boundary and Package Structure

### Public API Surface (v1.0)

LiveTemplate v1.0 exposes a minimal, zero-configuration public API:

```go
// Core public types
type Application struct { /* private fields */ }
type Page struct { /* private fields */ }
type Fragment struct {
    ID       string      `json:"id"`
    Strategy string      `json:"strategy"`   // "tree_based"
    Action   string      `json:"action"`     // "update_tree"
    Data     interface{} `json:"data"`       // SimpleTreeData structure
}

// Tree-based optimization data structure
type SimpleTreeData struct {
    S        []string                 `json:"s"`        // Static HTML segments
    Dynamics map[string]interface{}   `json:"dynamics"` // Dynamic values by key
}

// Configuration options
type ApplicationOption func(*Application) error
type PageOption func(*Page) error

// Core API functions
func NewApplication(options ...ApplicationOption) *Application
func (app *Application) NewPage(tmpl *html.Template, data interface{}, options ...PageOption) (*Page, error)
func (app *Application) GetPage(token string) (*Page, error)
func (app *Application) Close() error

func (p *Page) Render() (string, error)
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]Fragment, error)
func (p *Page) GetToken() string
func (p *Page) Close() error
```

### Internal Package Organization

```
internal/
├── app/           # Application isolation and lifecycle
├── page/          # Page session management  
├── token/         # JWT token service
├── strategy/      # Tree-based optimization and template parsing
├── metrics/       # Simple built-in metrics (no external dependencies)
└── memory/        # Memory management and cleanup
```

**All implementation complexity is hidden in internal packages** to maintain API stability.

---

## Architecture Overview

### Component Responsibilities

**Security Foundation (Priority 1)**:
1. **Application**: Multi-tenant isolation with JWT tokens
2. **PageRegistry**: Thread-safe page storage with TTL cleanup
3. **Page**: Isolated session state with template rendering
4. **TokenService**: Standard JWT with replay protection

**Update System (Priority 1)**:
1. **TemplateAwareGenerator**: Hierarchical template parsing and boundary detection
2. **SimpleTreeGenerator**: Single unified tree-based strategy
3. **TreeDataGenerator**: Static/dynamic content separation with client-side caching

**Production Features (Priority 2)**:
1. **MemoryManager**: Resource limits and cleanup
2. **MetricsCollector**: Simple built-in metrics (no external dependencies)

---

## Core Components

### 1. Application Component

**Purpose**: Multi-tenant application isolation with zero-configuration setup.

```go
type Application struct {
    id           string
    tokenService *TokenService
    pageRegistry *PageRegistry
    memoryManager *MemoryManager
    metrics      *MetricsCollector
    config       *ApplicationConfig
    mu           sync.RWMutex
}

type ApplicationConfig struct {
    MaxPages        int           // Default: 1000
    PageTTL         time.Duration // Default: 1 hour
    CleanupInterval time.Duration // Default: 5 minutes
    TokenTTL        time.Duration // Default: 24 hours
}

// Core operations
func NewApplication(opts ...ApplicationOption) *Application
func (app *Application) NewPage(tmpl *html.Template, data interface{}, opts ...PageOption) (*Page, error)
func (app *Application) GetPage(token string) (*Page, error)
func (app *Application) Close() error

// Operational methods
func (app *Application) GetMetrics() ApplicationMetrics
func (app *Application) GetPageCount() int
func (app *Application) CleanupExpiredPages() int
```

**Key Features**:
- Application ID generation using crypto/rand
- Cross-application access prevention through token validation
- Memory budget enforcement with graceful degradation
- Comprehensive metrics collection

### 2. Page Component

**Purpose**: Isolated user session with stateless design for horizontal scaling.

```go
type Page struct {
    id            string
    applicationID string
    templateHash  string
    template      *html.Template
    data          interface{}
    createdAt     time.Time
    lastAccessed  time.Time
    fragmentCache map[string]string
    config        *PageConfig
    mu            sync.RWMutex
}

type PageConfig struct {
    MaxFragments    int // Default: 100
    MaxMemoryMB     int // Default: 10MB
    UpdateBatchSize int // Default: 20
}

// Core operations
func (p *Page) Render() (string, error)
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]Fragment, error)
func (p *Page) GetToken() string
func (p *Page) SetData(data interface{}) error
func (p *Page) Close() error

// Operational methods
func (p *Page) GetMetrics() PageMetrics
func (p *Page) GetMemoryUsage() int64
func (p *Page) IsExpired(ttl time.Duration) bool
```

**Tree-Based Optimization Strategy (v1.0)**:
- **Template Boundary Analysis**: Hierarchical parsing of all Go template constructs
- **Static/Dynamic Separation**: Identifies static HTML content vs dynamic template values
- **Single Unified Strategy**: Tree-based optimization adapts to all template patterns
- **Client-Side Caching**: Static content cached client-side, only dynamic values transmitted
- **Phoenix LiveView Compatible**: Generated structures mirror LiveView client format
- **92%+ Bandwidth Savings**: Achieved through intelligent static/dynamic separation
- **Predictable Performance**: Consistent behavior across all template complexity levels

### 3. TokenService Component

**Purpose**: Standard JWT-based authentication with replay protection.

```go
type TokenService struct {
    signingKey   []byte
    algorithm    jwt.SigningMethod // Always HS256 in v1.0
    nonceStore   *NonceStore
    config       *TokenConfig
    mu           sync.RWMutex
}

type TokenConfig struct {
    TTL               time.Duration // Default: 24 hours
    NonceWindow       time.Duration // Default: 5 minutes
    MaxNoncePerWindow int           // Default: 1000
}

type PageToken struct {
    PageID        string    `json:"page_id"`
    ApplicationID string    `json:"app_id"`
    IssuedAt      time.Time `json:"iat"`
    ExpiresAt     time.Time `json:"exp"`
    Nonce         string    `json:"nonce"`
}

// Core operations using standard JWT
func (ts *TokenService) GenerateToken(appID, pageID string) (string, error)
func (ts *TokenService) VerifyToken(tokenStr string) (*PageToken, error)
func (ts *TokenService) RotateSigningKey() error
```

**Security Features**:
- Standard JWT implementation (github.com/golang-jwt/jwt/v5)
- HS256 algorithm only (prevents algorithm confusion attacks)
- Nonce-based replay protection
- Automatic key rotation support

### 4. TreeGenerator Component

**Purpose**: Tree-based optimization with template boundary parsing and static/dynamic separation.

```go
type SimpleTreeGenerator struct {
    cache map[string]*SimpleTreeData // Fragment structure cache
    mu    sync.RWMutex
}

type TemplateAwareGenerator struct {
    // Hierarchical template parsing and boundary detection
}

type TemplateBoundary struct {
    Type       TemplateBoundaryType // StaticContent, SimpleField, ConditionalIf, RangeLoop
    Content    string               // Original template content
    Start      int                  // Position in template
    End        int                  // Position in template
    FieldPath  string               // For SimpleField: ".User.Name"
    Condition  string               // For conditionals/ranges: ".Active"
    TrueBlock  []TemplateBoundary   // Nested content for conditionals/ranges
    FalseBlock []TemplateBoundary   // Else content
}

type TemplateBoundaryType int

const (
    StaticContent TemplateBoundaryType = iota
    SimpleField
    ConditionalIf
    RangeLoop
    ContextWith
    Complex
    Comment
)

type SimpleTreeData struct {
    S        []string                 `json:"s"`        // Static HTML segments
    Dynamics map[string]interface{}   `json:"dynamics"` // Dynamic content by key
}

type TreeGenerationResult struct {
    TreeData        *SimpleTreeData
    FragmentID      string
    TemplateHash    string
    GeneratedAt     time.Time
    BandwidthSaving float64  // Estimated bandwidth saving percentage
}

// Tree-based optimization generation
func (stg *SimpleTreeGenerator) GenerateFromTemplateSource(templateSource string, oldData, newData interface{}, fragmentID string) (*SimpleTreeData, error)
func (tag *TemplateAwareGenerator) ParseTemplateBoundaries(templateSource string) ([]TemplateBoundary, error)
func (tag *TemplateAwareGenerator) EvaluateFieldPath(fieldPath string, data interface{}) (interface{}, error)
func (stg *SimpleTreeGenerator) GenerateTreeStructure(boundaries []TemplateBoundary, data interface{}) (*SimpleTreeData, error)

// Template parsing support
type TemplateParser struct {
    cache map[string][]TemplateBoundary
    mu    sync.RWMutex
}

type DataEvaluator struct {
    // Field path evaluation with reflection
}

type TreeStructureBuilder struct {
    // Hierarchical tree structure generation
}

func (stg *SimpleTreeGenerator) GenerateTreeFromTemplate(templateSource string, data interface{}) (*SimpleTreeData, error) {
    // 1. Parse template into boundaries
    boundaries, err := stg.parseTemplateBoundaries(templateSource)
    if err != nil {
        return nil, err
    }
    
    // 2. Generate static segments and dynamic values
    statics := []string{}
    dynamics := make(map[string]interface{})
    
    // 3. Build tree structure with static/dynamic separation
    treeData := &SimpleTreeData{
        S:        statics,
        Dynamics: dynamics,
    }
    
    return treeData, nil
}

// Tree-based optimization - single strategy for all template patterns
func (stg *SimpleTreeGenerator) OptimizeForClientCaching(treeData *SimpleTreeData, isFirstRender bool) {
    if !isFirstRender {
        // Clear static arrays on subsequent renders - cached client-side
        treeData.S = nil
    }
    // Always send dynamic values for updates
}
```

### 5. TreeDataProcessor Component

**Purpose**: Tree structure processing and fragment generation for client consumption.

```go
type TreeDataProcessor struct {
    treeCache map[string]*SimpleTreeData
    config    *ProcessorConfig
    mu        sync.RWMutex
}

type TemplateTree struct {
    Boundaries []TemplateBoundary
    Hash       string
}

type TreeNode struct {
    Type        TemplateBoundaryType // Type of boundary
    Content     string               // Static content or field path
    Children    []*TreeNode          // Child nodes for nested structures
    Position    int                  // Position in template
    IsStatic    bool                 // Whether this node represents static content
}

type PhoenixLiveViewCompat struct {
    StaticSegments []string                 `json:"s"`        // Static HTML segments
    DynamicValues  map[string]interface{}   `json:"dynamics"` // Dynamic content by key
}

type ProcessorConfig struct {
    MaxCacheSize     int // Default: 1000 templates
    EnableCaching    bool // Default: true
    ClientCompatMode bool // Default: true (Phoenix LiveView format)
}

// Core operations with tree-based optimization
func (tdp *TreeDataProcessor) ProcessTreeData(treeData *SimpleTreeData, fragmentID string) (*Fragment, error)
func (tdp *TreeDataProcessor) GenerateFragmentID(templateHash string, dataSignature string) string
func (tdp *TreeDataProcessor) OptimizeForBandwidth(treeData *SimpleTreeData, isFirstRender bool) (*SimpleTreeData, error)
func (tdp *TreeDataProcessor) ConvertToPhoenixLiveViewFormat(treeData *SimpleTreeData) (*PhoenixLiveViewCompat, error)
```

**Fragment ID Algorithm (v1.0)**:
```go
func generateFragmentID(templateHash, dataSignature string) string {
    components := fmt.Sprintf("%s|%s", templateHash, dataSignature)
    hash := sha256.Sum256([]byte(components))
    return fmt.Sprintf("tree-%x", hash[:8]) // 16-character deterministic ID
}
```

### 6. TreeUpdateGenerator Component

**Purpose**: Generate tree-based updates using single unified optimization strategy.

```go
type TreeUpdateGenerator struct {
    treeGenerator *SimpleTreeGenerator
    processor     *TreeDataProcessor
    config        *UpdateConfig
}

type UpdateConfig struct {
    EnableTreeCaching   bool         // Default: true
    TreeCacheSize       int          // Default: 1000 templates
    EnableStaticCaching bool         // Default: true
    EnableCompression   bool         // Default: true
    LogTreeGeneration   bool         // Default: true for analysis
}

type DataChange struct {
    Path        string
    OldValue    interface{}
    NewValue    interface{}
    TreeData    *SimpleTreeData // Generated tree structure
    FragmentID  string         // Tree-based fragment identifier
    IsFirstRender bool         // Whether static content should be included
}

// Tree-based update generation
func (tug *TreeUpdateGenerator) GenerateTreeBasedUpdate(tmpl *html.Template, oldData, newData interface{}) ([]Fragment, error) {
    // 1. Generate tree structure from template and new data
    templateSource := extractTemplateSource(tmpl)
    fragmentID := tug.generateFragmentID(templateSource, newData)
    
    // 2. Create tree data using single unified strategy
    treeData, err := tug.treeGenerator.GenerateFromTemplateSource(templateSource, oldData, newData, fragmentID)
    if err != nil {
        return nil, err
    }
    
    // 3. Process tree data into client-compatible fragment
    fragment, err := tug.processor.ProcessTreeData(treeData, fragmentID)
    if err != nil {
        return nil, err
    }
    
    return []Fragment{*fragment}, nil
}

// Tree-based optimization functions
func (tug *TreeUpdateGenerator) GenerateTreeStructure(templateSource string, data interface{}) (*SimpleTreeData, error)
func (tug *TreeUpdateGenerator) OptimizeForStaticCaching(treeData *SimpleTreeData, isFirstRender bool) (*SimpleTreeData, error)
func (tug *TreeUpdateGenerator) ConvertToClientFormat(treeData *SimpleTreeData) (*Fragment, error)

// Performance optimization with tree-based insights
func (tug *TreeUpdateGenerator) OptimizeTreeForBandwidth(treeData *SimpleTreeData) error
func (tug *TreeUpdateGenerator) CacheTreeStructure(templateHash string, treeData *SimpleTreeData) error
func (tug *TreeUpdateGenerator) ValidateTreeStructure(treeData *SimpleTreeData) error
```

---

## Test-Driven Development Strategy

### Testing Philosophy for v1.0

LiveTemplate v1.0 follows comprehensive TDD with focus on reliability and security:

1. **Unit Tests** (60%): Individual component behavior
2. **Integration Tests** (30%): Component interaction and workflows  
3. **Security Tests** (10%): Multi-tenant isolation and authentication

### TDD Implementation Process

All features are implemented using Red-Green-Refactor:

```go
// 1. RED: Write failing test first
func TestApplicationIsolation(t *testing.T) {
    appA := livetemplate.NewApplication()
    appB := livetemplate.NewApplication()
    defer appA.Close()
    defer appB.Close()

    tmpl := template.Must(template.New("test").Parse("{{.Name}}"))
    pageA, err := appA.NewPage(tmpl, map[string]string{"Name": "Alice"})
    require.NoError(t, err)

    // Cross-application access should fail
    _, err = appB.GetPage(pageA.GetToken())
    assert.Equal(t, ErrInvalidApplication, err) // FAILS - not implemented
}

// 2. GREEN: Minimal implementation to pass
func (app *Application) GetPage(token string) (*Page, error) {
    pageToken, err := app.tokenService.VerifyToken(token)
    if err != nil {
        return nil, err
    }
    
    if pageToken.ApplicationID != app.id {
        return nil, ErrInvalidApplication
    }
    
    return app.pageRegistry.Get(pageToken.PageID)
}

// 3. REFACTOR: Improve while keeping tests green
// Add comprehensive error handling, metrics, logging, etc.
```

### Critical Test Categories

#### Security Tests (Must Pass for v1.0)

```go
func TestCrossApplicationIsolation(t *testing.T) {
    // Verify no cross-application page access
    // Verify token validation enforces application boundaries
    // Verify no memory sharing between applications
}

func TestTokenSecurity(t *testing.T) {
    // Verify JWT signature validation
    // Verify replay attack prevention
    // Verify token expiration handling
    // Verify key rotation doesn't break active tokens
}

func TestMemoryIsolation(t *testing.T) {
    // Verify page data isolation
    // Verify fragment cache isolation
    // Verify memory cleanup on page deletion
}
```

#### Performance Tests (Must Meet Targets)

```go
func TestBandwidthOptimization(t *testing.T) {
    // Measure tree-based optimization: 92%+ bandwidth savings for typical templates
    // Test complex nested templates: 95.9% savings (24 bytes vs 590 bytes)
    // Test simple text updates: 75%+ savings with static content caching
    // Validate Phoenix LiveView client compatibility
    // Measure template boundary parsing accuracy >95%
    // Test static/dynamic separation effectiveness
}

func TestLatencyRequirements(t *testing.T) {
    // Verify P95 update generation < 75ms for tree-based fragments
    // Test template boundary parsing latency <5ms average, <25ms max
    // Test under concurrent load (100+ pages) with tree structure caching
    // Measure memory usage for tree data caching and template parsing
    // Validate tree generation accuracy under various template complexity scenarios
}

func TestScalability(t *testing.T) {
    // Support 1000 concurrent pages
    // Memory usage remains bounded
    // Performance degrades gracefully under pressure
}
```

#### Integration Tests (End-to-End Workflows)

```go
func TestCompleteUpdateWorkflow(t *testing.T) {
    // Create application and page
    // Generate initial render with fragments
    // Update data and generate fragment updates
    // Verify fragment updates are correct
    // Verify metrics are collected
}

func TestMemoryManagement(t *testing.T) {
    // Create many pages
    // Verify TTL cleanup works
    // Verify memory limits are enforced
    // Verify graceful degradation
}
```

---

## Implementation Roadmap

### Realistic 60-Task Breakdown

**Phase 1: Security Foundation (Tasks 1-30, Weeks 1-8)**

*Application Layer (Tasks 1-10)*:
- Application struct and basic lifecycle
- Application ID generation and validation
- Configuration management with defaults
- Application registry for multi-tenant support
- Application metrics collection
- Application cleanup and shutdown
- Error handling and logging
- Unit tests for application layer
- Integration tests for application isolation
- Security tests for cross-application access

*Token Service (Tasks 11-20)*:
- JWT token generation with HS256
- Token verification and validation
- Nonce store for replay protection
- Token expiration handling
- Key rotation support
- Token metrics and monitoring
- Security audit of token implementation
- Token service unit tests
- Token integration tests
- Token security penetration tests

*Page Management (Tasks 21-30)*:
- Page struct and lifecycle management
- Page data isolation and thread safety
- Page registry with concurrent access
- Page TTL and expiration handling
- Page metrics collection
- Page cleanup and garbage collection
- Page unit tests
- Page integration tests
- Page security tests
- Page performance tests

**Phase 2: Tree-Based Optimization System (Tasks 31-50)**

*Tree-Based Strategy Implementation (Tasks 31-40)*:
- Template boundary parsing engine for hierarchical analysis
- Static/dynamic content separation algorithms
- Tree structure generation with Phoenix LiveView compatibility
- Single unified strategy for all template patterns
- Client-side caching optimization for static content
- Template-aware optimization for all Go template constructs
- Tree data caching system for performance
- Tree generator unit tests with boundary parsing validation
- Static content caching validation
- Template compatibility testing across all Go constructs

*Update Generation (Tasks 41-50)*:
- Tree-based update generation pipeline
- Single strategy generator for all template patterns
- Tree structure optimization for client consumption
- Static content caching integration
- Performance monitoring for tree generation
- Bandwidth optimization through static/dynamic separation
- Update generator unit tests with tree structure scenarios
- Integration tests for complete template parsing → tree generation workflows
- Performance benchmarking for tree-based optimization
- Tree structure effectiveness validation across template complexity levels

**Phase 3: Production Features (Tasks 51-60)**

*Memory Management (Tasks 51-55)*:
- Memory usage tracking and monitoring
- Memory limit enforcement  
- Graceful degradation under memory pressure
- Memory management unit tests
- Memory stress tests

*Operational Features (Tasks 56-60)*:
- Simple built-in metrics collection (no external dependencies)
- Structured logging with context  
- Health check endpoints
- Performance benchmarking and validation
- Optional Prometheus export format

### Implementation Schedule (LLM-Assisted Development)

**Immediate Implementation**: Tasks can be completed in focused development sessions

- **Phase 1 (Tasks 1-30)**: Security foundation with comprehensive testing
- **Phase 2 (Tasks 31-50)**: Dual-strategy update system with performance validation  
- **Phase 3 (Tasks 51-60)**: Production features and operational readiness

**LLM-Assisted Benefits**:
- Rapid prototyping and iteration
- Comprehensive test generation
- Consistent code patterns and architecture
- Immediate implementation of complex algorithms
- Parallel development of multiple components

### Definition of Done for Each Phase

**Phase 1 Complete When**:
- All security tests pass (0 cross-application access)
- JWT implementation audited by security expert
- 1000 concurrent pages supported in testing
- Memory usage bounded and monitored
- All unit tests >90% coverage

**Phase 2 Complete When**:
- Template boundary parsing achieves >95% accuracy across all Go template constructs
- Tree-based optimization achieves 92%+ bandwidth savings for typical templates
- Complex nested templates achieve 95.9% savings (24 bytes vs 590 bytes)
- Simple text updates achieve 75%+ savings with static content caching
- Phoenix LiveView client compatibility fully validated
- P95 update generation latency <75ms under load for tree-based fragments
- Integration tests cover complete template parsing → tree generation workflows
- Performance benchmarks validate tree optimization effectiveness across template complexity levels

**Phase 3 Complete When**:
- Comprehensive metrics and monitoring implemented
- Memory management working under load
- Performance benchmarks meet all targets (value patch + fragment replace)
- Production operational requirements met
- Security audit passed for production deployment

---

## Operational Requirements

### Metrics and Monitoring

```go
// Simple built-in metrics with no external dependencies
type ApplicationMetrics struct {
    PagesCreated         uint64    `json:"pages_created_total"`
    PagesActive          uint64    `json:"pages_active"`
    UpdatesGenerated     uint64    `json:"updates_generated_total"`
    UpdateLatencyP95     float64   `json:"update_latency_p95_ms"`
    
    // HTML diffing-enhanced strategy metrics
    HTMLDiffsGenerated   uint64    `json:"html_diffs_generated_total"`
    HTMLDiffCacheHits    uint64    `json:"html_diff_cache_hits_total"`
    HTMLDiffLatencyP95   float64   `json:"html_diff_latency_p95_ms"`
    
    // Strategy usage with HTML diff insights
    StaticDynamicUsed    uint64    `json:"strategy_static_dynamic_used_total"`
    MarkerCompilationUsed uint64   `json:"strategy_marker_used_total"`
    GranularOpsUsed      uint64    `json:"strategy_granular_used_total"`
    FragmentReplacementUsed uint64 `json:"strategy_replacement_used_total"`
    
    // Performance by strategy with diff overhead
    HTMLDiffAnalysisTime  float64  `json:"html_diff_analysis_avg_ms"`
    StrategySelectionTime float64  `json:"strategy_selection_avg_ms"`
    BandwidthSavingsStrategy1 float64 `json:"bandwidth_savings_strategy1_avg_percent"`
    BandwidthSavingsStrategy2 float64 `json:"bandwidth_savings_strategy2_avg_percent"`
    BandwidthSavingsStrategy3 float64 `json:"bandwidth_savings_strategy3_avg_percent"`
    BandwidthSavingsStrategy4 float64 `json:"bandwidth_savings_strategy4_avg_percent"`
    
    // HTML diff pattern tracking
    DiffPatternDistribution map[string]uint64 `json:"diff_pattern_distribution"`
    StrategyAccuracy       float64           `json:"strategy_selection_accuracy_percent"`
    
    MemoryUsageBytes     uint64    `json:"memory_usage_bytes"`
    TokenValidationFails uint64    `json:"token_validation_fails_total"`
    ErrorsTotal          uint64    `json:"errors_total"`
    LastUpdated          time.Time `json:"last_updated"`
}

// Optional: Export metrics in Prometheus format if needed
func (m *ApplicationMetrics) PrometheusExport() string {
    // Simple text format compatible with Prometheus
    // Only implemented if user needs it
}
```

### Error Handling Strategy

```go
// Sentinel errors for common cases
var (
    ErrPageNotFound         = errors.New("page not found")
    ErrInvalidApplication   = errors.New("invalid application access")
    ErrTokenExpired         = errors.New("token expired")
    ErrTokenInvalid         = errors.New("token invalid")
    ErrMemoryBudgetExceeded = errors.New("memory budget exceeded")
    ErrPageLimitExceeded    = errors.New("page limit exceeded")
    ErrTemplateInvalid      = errors.New("template invalid")
    ErrDataInvalid          = errors.New("data invalid")
)

// Error wrapping with context
type LiveTemplateError struct {
    Op  string
    Err error
    Context map[string]interface{}
}

func (e *LiveTemplateError) Error() string {
    return fmt.Sprintf("livetemplate: %s: %v", e.Op, e.Err)
}

func (e *LiveTemplateError) Unwrap() error {
    return e.Err
}
```

### Configuration Management

```go
type Config struct {
    Application ApplicationConfig `json:"application"`
    Page        PageConfig        `json:"page"`  
    Token       TokenConfig       `json:"token"`
    Fragment    FragmentConfig    `json:"fragment"`
    Transport   TransportConfig   `json:"transport"`
    Memory      MemoryConfig      `json:"memory"`
    Metrics     MetricsConfig     `json:"metrics"`
}

// Load configuration with defaults
func LoadConfig(path string) (*Config, error) {
    config := DefaultConfig()
    
    if path != "" {
        if err := loadFromFile(config, path); err != nil {
            return nil, err
        }
    }
    
    return config, config.Validate()
}
```

### Health Checks

```go
type HealthChecker struct {
    app *Application
}

func (hc *HealthChecker) Check(ctx context.Context) error {
    // Check application health
    if hc.app.GetPageCount() > hc.app.config.MaxPages {
        return errors.New("page limit exceeded")
    }
    
    // Check memory usage
    if hc.app.GetMemoryUsage() > hc.app.config.MaxMemoryMB*1024*1024 {
        return errors.New("memory limit exceeded")
    }
    
    // Check token service
    if err := hc.app.tokenService.HealthCheck(); err != nil {
        return fmt.Errorf("token service unhealthy: %w", err)
    }
    
    return nil
}
```

---

## Appendix

### A. Failure Mode Analysis

**Critical Failure Scenarios**:

1. **Memory Exhaustion**: TTL cleanup + page limits + monitoring
2. **Token Corruption**: Validation + regeneration + graceful degradation  
3. **Template Parse Errors**: Validation + clear error messages
4. **Network Partitions**: Reconnection + state recovery
5. **High Load**: Backpressure + circuit breakers + metrics

### B. Migration Strategy

**From Current Singleton Model**:
1. Run both systems in parallel during transition
2. Feature flags for gradual rollout
3. Data migration scripts for existing sessions
4. Rollback procedures if issues detected

### C. Security Review Checklist

- [ ] JWT implementation audited
- [ ] Cross-application access blocked  
- [ ] Replay attack prevention validated
- [ ] Memory isolation verified
- [ ] Input validation comprehensive
- [ ] Error messages don't leak sensitive data
- [ ] Transport security (HTTPS/WSS only)

### D. Performance Benchmarks

**Target Metrics for v1.0**:
- Tree-based optimization: 92%+ bandwidth savings for typical real-world templates
- Complex nested templates: 95.9% savings (24 bytes vs 590 bytes)
- Simple text updates: 75%+ savings with static content caching  
- Template boundary parsing accuracy: >95% across all Go template constructs
- Static/dynamic separation effectiveness: Consistent across all template patterns
- P95 update latency <75ms (under 100 concurrent pages, tree-based fragments)
- Template parsing: <5ms average, <25ms max
- 1000 concurrent pages supported (with 8GB RAM)
- Memory usage <8MB per page (typical applications)
- 99.9% uptime in staging environment
- Universal template compatibility (100% through single tree-based strategy)

---

**Note**: This v1.0 LLD prioritizes reliability, security, and operational excellence over aggressive optimization. The 75-task breakdown includes sufficient buffer for complexity while establishing a solid foundation for future enhancements.