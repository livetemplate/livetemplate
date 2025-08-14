# LiveTemplate: Low Level Design (LLD) - First Public Release

## Overview

This document provides the low-level design specification for implementing LiveTemplate's first public release (v1.0). The focus is on fast and efficient update generation with secure session isolation, leaving network transport to the application layer.

**What this document covers:**

- Concrete component specifications for v1.0 implementation
- Security architecture with JWT-based multi-tenant isolation  
- HTML diffing-enhanced four-tier intelligent update system
- Realistic 60-task implementation roadmap (transport removed)
- Test-driven development strategy
- Production-ready operational requirements

**Target audience:** Software engineers implementing LiveTemplate v1.0, with practical implementation details focused on reliability over aggressive optimization.

**Key design principles:**

- **Security First**: Multi-tenant isolation with comprehensive testing
- **Performance First**: Four-tier strategy maximizing efficiency (85-95% → 70-85% → 60-80% → 40-60%)
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
    Strategy string      `json:"strategy"`   // "static_dynamic", "markers", "granular", "replacement"
    Action   string      `json:"action"`     // Specific action within strategy
    Data     interface{} `json:"data"`       // Strategy-specific payload
}

// Strategy-specific data structures
type StaticDynamicData struct {
    Statics  []string              `json:"statics"`  // Static HTML segments
    Dynamics map[string]interface{} `json:"dynamics"` // Dynamic values
}

type MarkerPatchData struct {
    Patches []ValuePatch `json:"patches"` // Position-based value updates
}

type ValuePatch struct {
    Position int    `json:"position"`
    Length   int    `json:"length"`
    NewValue string `json:"new_value"`
    OldValue string `json:"old_value,omitempty"`
}

type GranularOpData struct {
    Operation string      `json:"operation"` // "append", "prepend", "insert", "remove"
    Content   string      `json:"content,omitempty"`
    Position  int         `json:"position,omitempty"`
    Selector  string      `json:"selector,omitempty"`
}

type ReplacementData struct {
    Content string `json:"content"` // Complete HTML replacement
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
├── fragment/      # Template analysis and update generation
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
1. **TemplateAnalyzer**: Template analysis for strategy selection
2. **FragmentExtractor**: HTML fragment identification and extraction
3. **UpdateGenerator**: Dual-strategy update generation (value patches + fragment replacement)

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

**HTML Diffing-Enhanced Four-Tier Strategy (v1.0)**:
- **HTML Diff Analysis**: Render template with old + new data, analyze actual HTML changes
- **Pattern Recognition**: Classify changes as text-only, simple operations, or complex rewrites
- **Strategy 1 - Static/Dynamic**: 85-95% reduction for text-only changes (60-70% of cases)
- **Strategy 2 - Marker Compilation**: 70-85% reduction for position-discoverable changes (15-20% of cases)
- **Strategy 3 - Granular Operations**: 60-80% reduction for simple structural changes (10-15% of cases)
- **Strategy 4 - Fragment Replacement**: 40-60% reduction for complex structural changes (5-10% of cases)
- **Data-Driven Selection**: Strategy based on actual HTML diff patterns, not template guessing
- **High Accuracy**: >90% optimal strategy selection through diff analysis

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

### 4. StrategyAnalyzer Component

**Purpose**: HTML diffing-based intelligent strategy selection with pattern analysis.

```go
type StrategyAnalyzer struct {
    htmlDiffer            *HTMLDiffer
    staticDynamicAnalyzer *StaticDynamicAnalyzer
    markerCompiler        *MarkerCompiler
    granularAnalyzer      *GranularAnalyzer
    strategyCache         map[string]*StrategyAnalysis
    config                *AnalyzerConfig
    mu                    sync.RWMutex
}

type HTMLDiffer struct {
    parser     *HTMLParser
    comparator *DOMComparator
    classifier *PatternClassifier
    config     *DiffConfig
}

type HTMLDiff struct {
    ChangeType    ChangeType
    Changes       []Change
    // Note: No confidence field - strategy selection is deterministic
    OldHTML       string   // Original rendered HTML
    NewHTML       string   // New rendered HTML
}

type Change struct {
    Type        ChangeType // TEXT, ATTRIBUTE, ELEMENT_ADD, ELEMENT_REMOVE
    Position    int        // Character position in HTML
    OldValue    string
    NewValue    string
    Element     string     // HTML element affected
    // Note: No confidence field - change detection is deterministic
}

type ChangeType int

const (
    TextChange ChangeType = iota
    AttributeChange
    ElementAddition
    ElementRemoval
    StructuralRewrite
)

type StrategyAnalysis struct {
    SelectedStrategy StrategyType
    Fragment         Fragment
    HTMLDiff        *HTMLDiff        // HTML diff that led to strategy selection
    PerformanceScore float64         // Expected bandwidth reduction
    SelectionReason  string          // Why this strategy was selected
    TemplateHash     string
    DataSignature    string          // Hash of data changes for caching
    GeneratedAt      time.Time
}

// HTML diffing-based strategy selection
func (sa *StrategyAnalyzer) SelectOptimalStrategy(tmpl *html.Template, oldData, newData interface{}) (*StrategyAnalysis, error)
func (sa *StrategyAnalyzer) AnalyzeHTMLDiff(tmpl *html.Template, oldData, newData interface{}) (*HTMLDiff, error)
func (sa *StrategyAnalyzer) GenerateStaticDynamic(diff *HTMLDiff) (*Fragment, error)
func (sa *StrategyAnalyzer) GenerateGranularOperations(diff *HTMLDiff) (*Fragment, error)
func (sa *StrategyAnalyzer) GenerateMarkerPatches(tmpl *html.Template, oldData, newData interface{}, diff *HTMLDiff) (*Fragment, error)
func (sa *StrategyAnalyzer) GenerateFragmentReplacement(newHTML string) (*Fragment, error)

// Strategy-specific analyzers
type StaticDynamicAnalyzer struct {
    cache map[string]*StaticDynamicResult
    mu    sync.RWMutex
}

type MarkerCompiler struct {
    markerPrefix string // "§"
    markerSuffix string // "§"
    maxListSize  int    // 20 items default
    counter      uint32 // Atomic counter
}

type GranularAnalyzer struct {
    operationPatterns map[string]*OperationPattern
    diffAnalyzer     *HTMLDiffer
    mu               sync.RWMutex
}

type PatternClassifier struct {
    textOnlyPattern      *regexp.Regexp
    elementAddPattern    *regexp.Regexp
    elementRemovePattern *regexp.Regexp
    complexRewriteThreshold float64
}

func (hd *HTMLDiffer) Analyze(oldHTML, newHTML string) *HTMLDiff {
    // 1. Parse both HTML into DOM trees
    oldDOM := hd.parser.Parse(oldHTML)
    newDOM := hd.parser.Parse(newHTML)
    
    // 2. Compare DOM trees to identify changes
    changes := hd.comparator.Compare(oldDOM, newDOM)
    
    // 3. Classify change patterns and complexity
    pattern := hd.classifier.ClassifyChanges(changes)
    
    return &HTMLDiff{
        ChangeType:  pattern.Type,
        Changes:     changes,
        Complexity:  pattern.Complexity,
        Confidence:  pattern.Confidence,
        OldHTML:     oldHTML,
        NewHTML:     newHTML,
    }
}

// Deterministic strategy selection based on change types
func (hd *HTMLDiff) RecommendStrategy() StrategyType {
    hasText := false
    hasAttribute := false
    hasStructural := false
    
    for _, change := range hd.Changes {
        switch change.Type {
        case "text":
            hasText = true
        case "attribute":
            hasAttribute = true
        case "element_add", "element_remove", "element_change":
            hasStructural = true
        }
    }
    
    // Rule-based deterministic selection (always predictable):
    if hasStructural {
        if hasText || hasAttribute {
            return ReplacementStrategy    // Complex: multiple change types
        }
        return GranularStrategy           // Pure structural changes
    }
    
    if hasAttribute {
        return MarkerStrategy             // Attribute changes (with/without text)
    }
    
    if hasText {
        return StaticDynamicStrategy      // Pure text-only changes
    }
    
    return StaticDynamicStrategy          // No changes or empty state
}
```

### 5. FragmentExtractor Component

**Purpose**: Fragment identification and extraction from templates with position tracking.

```go
type FragmentExtractor struct {
    fragmentCache map[string][]FragmentInfo
    config        *FragmentConfig
    mu            sync.RWMutex
}

type TemplateAST struct {
    Root        *ASTNode
    Expressions []DynamicExpression
    Hash        string
}

type ASTNode struct {
    Type     string     // "text", "action", "if", "range", "template"
    Text     string     // For text nodes
    Pipe     string     // For action nodes (e.g., ".Name")
    Children []*ASTNode // Child nodes
    Position int        // Position in template
}

type ClientReconstructionMeta struct {
    FragmentID      string            `json:"fragment_id"`
    ReconstructionJS string           `json:"reconstruction_js"`
    StaticCount     int               `json:"static_count"`
    DynamicCount    int               `json:"dynamic_count"`
    TemplateHash    string            `json:"template_hash"`
}

type FragmentConfig struct {
    MaxFragments     int // Default: 100 per page
    CacheSize        int // Default: 1000 templates
    MinFragmentSize  int // Default: 50 bytes
}

// Core operations with HTML diffing-enhanced strategy support
func (fe *FragmentExtractor) ExtractFragmentFromDiff(diff *HTMLDiff, strategy StrategyType) (Fragment, error)
func (fe *FragmentExtractor) GenerateFragmentID(templatePath string, strategy StrategyType, diffSignature string) string
func (fe *FragmentExtractor) OptimizeFragmentForDiffPattern(fragment Fragment, diff *HTMLDiff) (*Fragment, error)
func (fe *FragmentExtractor) ValidateDiffPatternCompatibility(fragment Fragment, diff *HTMLDiff) (bool, string)
```

**Fragment ID Algorithm (v1.0)**:
```go
func generateFragmentID(templatePath, dataPath string, position int) string {
    components := fmt.Sprintf("%s|%s|%d", templatePath, dataPath, position)
    hash := sha256.Sum256([]byte(components))
    return fmt.Sprintf("f-%x", hash[:8]) // 16-character deterministic ID
}
```

### 6. UpdateGenerator Component

**Purpose**: Generate updates using HTML diffing-enhanced four-tier strategy selection.

```go
type UpdateGenerator struct {
    strategyAnalyzer *StrategyAnalyzer
    htmlDiffer      *HTMLDiffer
    extractor       *FragmentExtractor
    config          *UpdateConfig
}

type UpdateConfig struct {
    EnableHTMLDiffing    bool         // Default: true
    DiffCacheSize       int          // Default: 1000 diff patterns
    CacheStrategies     bool         // Default: true
    EnableCompression   bool         // Default: true
    LogStrategySelection bool        // Default: true for analysis
}

type DataChange struct {
    Path         string
    OldValue     interface{}
    NewValue     interface{}
    HTMLDiff     *HTMLDiff    // Associated HTML diff analysis
    Strategy     StrategyType // Deterministically selected strategy based on change types
    // Note: No confidence field - strategy selection is deterministic
    FragmentIDs  []string
}

// HTML diffing-enhanced update generation
func (ug *UpdateGenerator) GenerateUpdatesFromDiff(tmpl *html.Template, oldData, newData interface{}) ([]Fragment, error) {
    // 1. Analyze HTML diff to understand change patterns
    diff, err := ug.strategyAnalyzer.AnalyzeHTMLDiff(tmpl, oldData, newData)
    if err != nil {
        return nil, err
    }
    
    // 2. Generate update using recommended strategy
    switch diff.RecommendStrategy() {
    case StaticDynamicStrategy:
        return ug.generateStaticDynamicFromDiff(diff)
    case GranularStrategy:
        return ug.generateGranularFromDiff(diff)
    case MarkerStrategy:
        return ug.generateMarkerFromDiff(tmpl, oldData, newData, diff)
    default:
        return ug.generateReplacementFromDiff(diff)
    }
}

// Strategy-specific generators enhanced with HTML diff data
func (ug *UpdateGenerator) generateStaticDynamicFromDiff(diff *HTMLDiff) ([]Fragment, error)
func (ug *UpdateGenerator) generateGranularFromDiff(diff *HTMLDiff) ([]Fragment, error)
func (ug *UpdateGenerator) generateMarkerFromDiff(tmpl *html.Template, oldData, newData interface{}, diff *HTMLDiff) ([]Fragment, error)
func (ug *UpdateGenerator) generateReplacementFromDiff(diff *HTMLDiff) ([]Fragment, error)

// Performance optimization with HTML diff insights
func (ug *UpdateGenerator) OptimizeBasedOnDiffPattern(fragment *Fragment, diff *HTMLDiff) error
func (ug *UpdateGenerator) BatchCompatibleDiffUpdates(fragments []Fragment) ([]Fragment, error)
func (ug *UpdateGenerator) ValidateStrategyEffectiveness(fragment Fragment, diff *HTMLDiff) error
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
    // Measure Strategy 1 (Static/Dynamic): 85-95% reduction for text-only changes
    // Measure Strategy 2 (Markers): 70-85% reduction for position-discoverable changes
    // Measure Strategy 3 (Granular): 60-80% reduction for simple structural changes
    // Measure Strategy 4 (Replacement): 40-60% reduction for complex changes
    // Test HTML diff-based strategy selection accuracy (>90% target)
    // Validate strategy distribution matches expected percentages (60-70%, 15-20%, 10-15%, 5-10%)
    // Measure HTML diffing overhead vs strategy selection benefits
}

func TestLatencyRequirements(t *testing.T) {
    // Verify P95 update generation < 75ms (includes HTML diffing overhead)
    // Test HTML diffing latency across different change patterns
    // Test under concurrent load (100+ pages) with HTML diff caching
    // Measure memory usage for HTML diff caching and pattern recognition
    // Validate HTML diff accuracy under various template complexity scenarios
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

**Phase 2: Dual-Strategy Update System (Tasks 31-50)**

*HTML Diffing-Enhanced Strategy Analysis (Tasks 31-40)*:
- HTML diffing engine implementation for change pattern analysis
- DOM parsing and comparison algorithms for accurate diff generation
- Pattern classification system (text-only, simple ops, complex rewrites)
- Strategy 1: Static/dynamic generation for text-only changes
- Strategy 2: Marker compilation for position-discoverable changes
- Strategy 3: Granular operations for simple structural changes
- Strategy 4: Fragment replacement for complex structural changes
- Strategy recommendation engine based on diff complexity scoring
- HTML diff caching system with pattern recognition
- Strategy analyzer unit tests with HTML diff validation

*Update Generation (Tasks 41-50)*:
- HTML diff-based update generation pipeline
- Strategy-specific generators enhanced with diff data (static/dynamic, markers, granular, replacement)
- Diff pattern recognition for optimal strategy selection
- Update optimization based on HTML diff insights
- Performance monitoring including HTML diffing overhead
- Bandwidth compression optimized for diff-identified patterns
- Update generator unit tests with HTML diff scenarios
- Integration tests for complete HTML diff → strategy → update workflows
- Performance benchmarking including diff analysis overhead
- Strategy effectiveness validation against actual HTML changes

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
- HTML diffing engine accurately identifies change patterns
- Strategy 1 (Static/Dynamic) achieves 85-95% size reduction for text-only changes (60-70% of cases)
- Strategy 2 (Markers) achieves 70-85% size reduction for position-discoverable changes (15-20% of cases)
- Strategy 3 (Granular) achieves 60-80% size reduction for simple structural changes (10-15% of cases)
- Strategy 4 (Replacement) provides 40-60% size reduction for complex changes (5-10% of cases)
- HTML diff-based strategy selection accuracy >90% across all change patterns
- P95 update generation latency <75ms under load (includes HTML diffing overhead)
- Integration tests cover complete HTML diff → strategy selection → update generation workflows
- Performance benchmarks validate strategy effectiveness against actual HTML change patterns

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
- Strategy 1: 85-95% bandwidth reduction for text-only changes (60-70% of cases)
- Strategy 2: 70-85% bandwidth reduction for position-discoverable changes (15-20% of cases)
- Strategy 3: 60-80% bandwidth reduction for simple structural changes (10-15% of cases)
- Strategy 4: 40-60% bandwidth reduction for complex changes (5-10% of cases)
- HTML diff-based strategy selection accuracy: >90% optimal choice
- HTML diffing pattern recognition accuracy: >95% correct classification
- P95 update latency <75ms (under 100 concurrent pages, includes HTML diffing)
- 1000 concurrent pages supported (with 8GB RAM)
- Memory usage <12MB per page (including HTML diffing and strategy caching overhead)
- 99.9% uptime in staging environment
- Universal template compatibility (100% through HTML diff analysis + four-tier fallback)

---

**Note**: This v1.0 LLD prioritizes reliability, security, and operational excellence over aggressive optimization. The 75-task breakdown includes sufficient buffer for complexity while establishing a solid foundation for future enhancements.