# LiveTemplate: High Level Design (HLD) - First Public Release

A high-level design document defining the architecture and key technical decisions for LiveTemplate's first public release, focusing on secure session isolation with reliable fragment updates and a foundation for future optimization.

## Executive Summary

LiveTemplate is evolving from a singleton page model to a focused update generation library with secure session isolation. The library's core purpose is generating fast and efficient page updates (value patches and fragment replacements) while leaving network transport to the application layer.

### Key Problems Solved

1. **Security**: Eliminates data leakage between users through application-scoped isolation
2. **Multi-tenancy**: Enables multiple applications and organizations on shared infrastructure  
3. **Performance**: Fast and efficient update generation (value patches + fragment replacements)
4. **Focus**: Pure update generation - network transport handled by application layer

### Design Principles for v1.0

- **Security First**: Multi-tenant isolation and JWT-based authentication
- **Performance First**: Value patches (primary) with fragment replacement fallback
- **Zero Configuration**: Works out-of-the-box with sensible defaults
- **Focused Scope**: Update generation only - no network transport concerns
- **Operational Excellence**: Comprehensive metrics and error handling

---

## Problem Definition

### Current Limitations

LiveTemplate's singleton page model creates fundamental issues for production applications:

**Security Vulnerabilities**:
- All clients share the same page instance
- No isolation between users or applications
- Potential for cross-user data leakage

**Performance Issues**:
- Full page reloads for all updates
- No systematic update optimization
- Inefficient update generation for changed data

**Scalability Constraints**:
- Cannot support multi-tenant deployments
- No horizontal scaling capabilities
- Limited concurrent user support

---

## Solution Architecture for v1.0

### Two-Layer Security Model

```go
// Application-level isolation
app1 := livetemplate.NewApplication() // E-commerce site
app2 := livetemplate.NewApplication() // Analytics dashboard

// Session-level isolation within applications
userPage1 := app1.NewPage(template, userData1) // User A's cart
userPage2 := app1.NewPage(template, userData2) // User B's cart

// Cross-application access is impossible
app2.GetPage(userPage1.GetToken()) // Returns ErrInvalidApplication
```

### Fragment Update Strategy (v1.0)

LiveTemplate v1.0 implements a **Four-Tier Strategy Hierarchy** that attempts optimal strategies first and degrades only when technically/deterministically infeasible:

**1. Static/Dynamic Mode** (Highest Preference - 85-95% reduction):
- **HTML Diffing Approach**: Render template with both old and new data
- **Pattern Analysis**: Compare rendered HTML to identify change patterns
- **Text-Only Changes**: When only values change, structure stays identical → Static/Dynamic
- **Simple Operations**: When changes are append/prepend/remove → Granular Operations
- **Complex Changes**: When structure completely different → Markers or Replacement
- **Data-Driven Selection**: Strategy based on actual changes, not template prediction

**2. Marker Compilation Mode** (Second Preference - 70-85% reduction):
- Pre-render with marker data to discover exact value positions
- Enables precise value patching for complex template constructs
- Handles conditionals and bounded lists through position mapping
- Falls back when template structure is unpredictably dynamic

**3. Granular Fragment Operations** (Third Preference - 60-80% reduction):
- Append, prepend, insert, remove, and targeted replace operations
- Used when value patching impossible but structural operations viable
- Handles variable-length lists, dynamic insertions, conditional blocks

**4. Fragment Replacement** (Final Fallback - 40-60% reduction):
- Complete HTML fragment replacement when all other strategies fail
- Guaranteed compatibility with any template complexity
- Used for recursive templates, unpredictable custom functions

**HTML Diffing-Based Strategy Selection**:
- **Render Both Versions**: Template + oldData → oldHTML, Template + newData → newHTML
- **Analyze Diff Pattern**: Compare HTML structures to identify change complexity
- **Pattern Classification**: Text-only, element addition/removal, attribute changes, structural rewrites
- **Strategy Recommendation**: Select optimal approach based on actual change pattern
- **Deterministic Selection**: Same change pattern always selects same strategy

### Horizontal Scaling Design

Pages are designed for stateless update generation across application instances:

```go
// On Instance A
page := app.NewPage(template, data)
updates := page.RenderFragments(ctx, newData)
token := page.GetToken()

// On Instance B (different server) 
samePage, _ := app.GetPage(token) // Reconstructed from token
updates := samePage.RenderFragments(ctx, newData) // Same updates generated
```

---

## Core Technical Concepts

### 1. Application Isolation

Each application instance provides a security boundary:

```go
type Application struct {
    id           string        // Unique application identifier
    tokenService *TokenService // JWT-based authentication
    pageRegistry *PageRegistry // Isolated page storage
}
```

**Security Properties**:
- Pages cannot be accessed across applications
- Token validation enforces application scope
- Memory budgets prevent resource exhaustion

### 2. Page Session Management

Pages represent isolated user sessions with deterministic tokens:

```go
type Page struct {
    id           string
    applicationID string
    templateHash string
    data         interface{}
    // Token contains all necessary reconstruction data
}
```

**Key Features**:
- Stateless design enables horizontal scaling
- JWT tokens contain reconstruction metadata
- Pure update generation - no transport concerns
- Cleanup with configurable TTLs

### 3. Fragment Update Protocol (v1.0)

Three-tier update protocol optimizing for efficiency:

**Value Patch Updates** (Primary):
**Strategy 1: Static/Dynamic Updates** (85-95% reduction):
```json
{
  "fragment_id": "user-profile",
  "action": "static_dynamic",
  "statics": ["<div class=\"user-", \">Welcome, ", "! You have ", " messages.</div>"],
  "dynamics": {"0": "admin", "1": "Alice", "2": "3"}
}
```
*Works when: Template structure remains stable between updates*

**Strategy 2: Marker Compilation Updates** (70-85% reduction):
```json
{
  "fragment_id": "alert-section", 
  "action": "value_patches",
  "patches": [
    {"position": 17, "length": 3, "new_value": "warning", "old_value": "§1§"},
    {"position": 35, "length": 3, "new_value": "Server down", "old_value": "§2§"}
  ]
}
```
*Works when: Positions discoverable through marker pre-rendering*

**Strategy 3: Granular Operations** (60-80% reduction):
```json
{
  "fragment_id": "todo-list",
  "action": "append",
  "data": "<li data-id=\"4\">New task</li>"
}
```
*Works when: Structural operations viable (lists, conditional blocks)*

**Strategy 4: Fragment Replacement** (40-60% reduction):
```json
{
  "fragment_id": "complex-section",
  "action": "replace", 
  "data": "<div>Completely new HTML structure</div>"
}
```
*Works when: All other strategies technically infeasible*

**Four-Tier Benefits**:
- Maximizes bandwidth efficiency by trying optimal strategies first
- Automatic degradation only when technically necessary
- 100% template compatibility through guaranteed fallback
- Superior performance through intelligent strategy selection

---

## Four-Tier Strategy Analysis

### Deterministic Strategy Selection Logic

LiveTemplate v1.0 uses **rule-based deterministic strategy selection** based on HTML change patterns. The same template construct will **always** choose the same strategy, ensuring predictable library behavior:

**Core Principle**: Strategy selection is based on **change types** using deterministic rules.

**Deterministic Rules**:
1. **Text-only changes** → Strategy 1 (Static/Dynamic)
2. **Attribute changes** → Strategy 2 (Markers) 
3. **Structural changes** → Strategy 3 (Granular)
4. **Mixed change types** → Strategy 4 (Replacement)

**Why Deterministic?**
- Library users can predict which strategy will be used
- Same template constructs always behave the same way
- Performance is predictable and debuggable
- Deterministic rule-based decisions

#### 1. Simple Value Interpolation (`{{.Field}}`)
**Strategy 1 - Static/Dynamic**: ✅ **Perfect Fit**
```html
<!-- Template -->
<span>Hello {{.Name}}</span>

<!-- Analysis: Structure never changes -->
Statics: ["<span>Hello ", "</span>"]
Dynamics: {"0": "Alice"}
```
**No degradation needed**: Template structure is completely stable.

#### 2. Conditional Blocks (`{{if .Condition}}...{{end}}`)
**HTML Diffing Analysis**:

```html
<!-- Template -->
{{if .ShowAlert}}<div class="alert-{{.AlertType}}">{{.Message}}</div>{{end}}

<!-- Case A: Values change, condition stays true -->
OldHTML: <div class="alert-info">Server maintenance</div>
NewHTML: <div class="alert-warning">Database issue</div>
Diff Pattern: TEXT_CHANGES_ONLY
Strategy: Static/Dynamic ✅
Statics: ["<div class=\"alert-", "\">", "</div>"]
Dynamics: {"0": "warning", "1": "Database issue"}

<!-- Case B: Condition changes true→false -->
OldHTML: <div class="alert-info">Server issue</div>
NewHTML: (empty)
Diff Pattern: ELEMENT_REMOVAL
Strategy: Granular Operation ✅
Action: {"operation": "remove", "selector": "[data-fragment-id='alert']"}

<!-- Case C: Condition changes false→true -->
OldHTML: (empty)
NewHTML: <div class="alert-warning">New alert</div>
Diff Pattern: ELEMENT_ADDITION
Strategy: Granular Operation ✅
Action: {"operation": "append", "content": "<div class=\"alert-warning\">New alert</div>"}
```

#### 3. Range/Loop Constructs (`{{range .Items}}...{{end}}`)
**HTML Diffing Analysis**:

```html
<!-- Template -->
{{range .TodoItems}}<li data-id="{{.ID}}">{{.Text}}</li>{{end}}

<!-- Case A: Same count, values change -->
OldHTML: <li data-id="1">Buy milk</li><li data-id="2">Walk dog</li>
NewHTML: <li data-id="1">Buy bread</li><li data-id="2">Feed cat</li>
Diff Pattern: TEXT_CHANGES_ONLY
Strategy: Static/Dynamic ✅
Statics: ["<li data-id=\"", "\">", "</li><li data-id=\"", "\">", "</li>"]
Dynamics: {"0": "1", "1": "Buy bread", "2": "2", "3": "Feed cat"}

<!-- Case B: List grows by one -->
OldHTML: <li data-id="1">Task 1</li><li data-id="2">Task 2</li>
NewHTML: <li data-id="1">Task 1</li><li data-id="2">Task 2</li><li data-id="3">Task 3</li>
Diff Pattern: ELEMENT_APPEND
Strategy: Granular Operation ✅
Action: {"operation": "append", "content": "<li data-id=\"3\">Task 3</li>"}

<!-- Case C: List shrinks (item removed) -->
OldHTML: <li data-id="1">A</li><li data-id="2">B</li><li data-id="3">C</li>
NewHTML: <li data-id="1">A</li><li data-id="3">C</li>
Diff Pattern: ELEMENT_REMOVAL
Strategy: Granular Operation ✅
Action: {"operation": "remove", "selector": "[data-id='2']"}

<!-- Case D: Complex reorder/restructure -->
OldHTML: <li>A</li><li>B</li><li>C</li>
NewHTML: <li>C</li><li>A</li><li>B</li><li>D</li><li>E</li>
Diff Pattern: COMPLEX_STRUCTURAL_CHANGE
Strategy: Fragment Replacement ❌
Action: {"operation": "replace", "content": "<li>C</li><li>A</li><li>B</li><li>D</li><li>E</li>"}
```

#### 4. Nested Templates (`{{template "name" .}}`)
**Strategy 1 - Static/Dynamic**: ✅ **When Structure Predictable**
```html
<!-- Simple nested template with stable structure -->
<div>{{template "user" .User}}</div>
<!-- "user" template: <span class="user-{{.Role}}">{{.Name}}</span> -->

Statics: ["<div><span class=\"user-", "\">", "</span></div>"]
Dynamics: {"0": "admin", "1": "Alice"}
```

**Strategy 2 - Markers**: ⚠️ **For Static Template Selection**
```html
<!-- When template name is known at analysis time -->
Marker compilation through nested template resolution
```

**Strategy 4 - Replacement**: ❌ **For Dynamic/Recursive Templates**
```html
<!-- When template selection is dynamic: {{template .TemplateName .}} -->
<!-- When recursion depth unpredictable -->
Action: "replace" - too complex for analysis
```

#### 5. Custom Functions (`{{customFunc .Data}}`)
**Strategy 1 - Static/Dynamic**: ✅ **When Output Predictable**
```html
<!-- Template with deterministic function -->
<div>{{formatDate .CreatedAt}}</div>

<!-- Function executes, result analyzed -->
Statics: ["<div>", "</div>"]
Dynamics: {"0": "January 15, 2024"} // formatDate output
```

**Strategy 2 - Markers**: ⚠️ **When Function Side-Effect Free**
```html
<!-- Use marker compilation for position discovery -->
MarkerData: {CreatedAt: "2024-01-01T00:00:00Z"}
Marker result analyzed for positions
```

**Strategy 4 - Replacement**: ❌ **For Complex/Side-Effect Functions**
```html
<!-- When function has side effects, unpredictable output structure -->
<!-- When function output varies dramatically in size/structure -->
Action: "replace" - analysis too risky
```

### HTML Diffing-Based Strategy Engine

```go
type StrategyAnalyzer struct {
    htmlDiffer            *HTMLDiffer
    staticDynamicAnalyzer *StaticDynamicAnalyzer
    markerCompiler        *MarkerCompiler
    granularAnalyzer      *GranularAnalyzer
    config                *AnalyzerConfig
}

func (sa *StrategyAnalyzer) SelectOptimalStrategy(tmpl *template.Template, oldData, newData interface{}) (*UpdateStrategy, error) {
    // 1. Render both versions for HTML diffing
    oldHTML, err := sa.render(tmpl, oldData)
    if err != nil {
        return nil, err
    }
    newHTML, err := sa.render(tmpl, newData)
    if err != nil {
        return nil, err
    }
    
    // 2. Analyze HTML diff to determine change pattern
    diff := sa.htmlDiffer.Analyze(oldHTML, newHTML)
    
    // 3. Select strategy based on deterministic rules
    switch diff.RecommendStrategy() {
    case StaticDynamicStrategy:
        return sa.generateStaticDynamic(oldHTML, newHTML, diff)
    case GranularStrategy:
        return sa.generateGranularOperations(oldHTML, newHTML, diff)
    case MarkerStrategy:
        return sa.generateMarkerPatches(tmpl, oldData, newData, diff)
    default:
        return sa.generateReplacement(newHTML), nil
    }
}

type HTMLDiff struct {
    ChangeType    ChangeType
    Changes       []Change
    Confidence    float64  // Quality metric: How sure we are of change detection (0.0-1.0)
}

type Change struct {
    Type        string   // "text", "attribute", "element_add", "element_remove"
    Position    int      // Character position in HTML
    OldValue    string
    NewValue    string
    Element     string   // HTML element affected
    // Note: No confidence field - strategy selection is deterministic
}

// Deterministic strategy selection based on change types
func (d *HTMLDiff) RecommendStrategy() StrategyType {
    hasText := false
    hasAttribute := false
    hasStructural := false
    
    for _, change := range d.Changes {
        switch change.Type {
        case "text":
            hasText = true
        case "attribute":
            hasAttribute = true
        case "element_add", "element_remove", "element_change":
            hasStructural = true
        }
    }
    
    // Rule-based deterministic selection:
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

### Deterministic Strategy Selection Benefits

**Important**: Strategy selection is **completely deterministic** based on HTML change pattern analysis. The same template construct with the same change pattern will **always** choose the same strategy.

**Benefits of deterministic approach**:
- **Predictable performance**: Same patterns always have same performance characteristics
- **Library reliability**: Users can depend on consistent behavior  
- **Debugging simplicity**: No unpredictable confidence-based variations
- **Testing effectiveness**: Same inputs always produce same outputs

### Client-Side Reconstruction

```javascript
function reconstructFragment(fragment) {
    let html = fragment.s[0] || "";
    
    for (let i = 0; i < Object.keys(fragment.d).length; i++) {
        const dynamicValue = fragment.d[i.toString()];
        
        // Handle different dynamic types
        if (Array.isArray(dynamicValue)) {
            // Range construct - reconstruct each item
            html += dynamicValue.map(item => renderItem(item, fragment.s)).join("");
        } else if (typeof dynamicValue === 'boolean') {
            // Conditional - include/exclude content based on value
            if (dynamicValue) {
                html += fragment.s[i + 1] || "";
            }
        } else {
            // Simple value - direct substitution
            html += dynamicValue + (fragment.s[i + 1] || "");
        }
    }
    
    return html;
}
```

**HTML Diffing-Enhanced Four-Tier Benefits**:
- **Data-Driven Strategy Selection**: Based on actual HTML changes, not template guessing
- **Higher Static/Dynamic Success Rate**: 60-70% vs 30% with template analysis alone
- **Pattern Recognition**: Text-only changes, simple operations, complex rewrites
- **Optimal Bandwidth Efficiency**: Always selects most efficient viable strategy
- **Confidence Scoring**: High accuracy in strategy selection through diff analysis
- **Performance Monitoring**: Track strategy effectiveness across change patterns
- **Intelligent Caching**: Cache diff patterns by data signature for repeated scenarios

---

## First Public Release Strategy

### Release Scope and Priorities

**Primary Goals for v1.0**:
1. **Rock-solid Security**: Multi-tenant isolation with comprehensive testing
2. **Optimal Performance**: Value patches (primary) + fragment replace (fallback)
3. **Developer Experience**: Zero-configuration API with clear error messages
4. **Focused Scope**: Pure update generation - no transport layer complexity

**Core v1.0 Implementation**:
1. **Dual Update Strategy**: Value patches (primary) + fragment replacement (fallback)
2. **Template Analysis**: AST-based capability detection for strategy selection
3. **Position Tracking**: Efficient value patch position management
4. **Automatic Fallback**: Seamless degradation when value patching not possible

**Explicitly Out of Scope**:
1. **Network Transport**: WebSocket, SSE, HTTP - handled by application layer
2. **Client-side Application**: Update application logic - consumer responsibility
3. **Message Queuing/Delivery**: Transport reliability - not LiveTemplate's concern

**Deferred to v2.0**:
1. **Advanced Optimizations**: Complex memory management strategies
2. **Enhanced Analysis**: Advanced template pattern detection
3. **Performance Tuning**: Fine-tuning based on real-world usage patterns

### Conservative Performance Targets

**v1.0 Targets with HTML Diffing-Enhanced Strategy**:
- 85-95% size reduction for text-only changes (Strategy 1: Static/Dynamic) - 60-70% of cases
- 70-85% size reduction for position-discoverable changes (Strategy 2: Markers) - 15-20% of cases
- 60-80% size reduction for simple operations (Strategy 3: Granular) - 10-15% of cases
- 40-60% size reduction minimum guaranteed (Strategy 4: Replacement) - 5-10% of cases
- >90% strategy selection accuracy through HTML diff analysis
- 100% template compatibility (four-tier fallback ensures universal support)
- P95 update generation latency < 75ms (includes HTML diffing overhead)
- Support 1,000 concurrent pages per instance

**Measurement Strategy**:
- Comprehensive metrics collection from day one
- Real-world usage data to drive v2.0 optimization decisions
- A/B testing framework for future value patch validation

### Risk Mitigation

**Known Risks and Mitigations**:
1. **Complexity Underestimation**: 75% buffer on all estimates
2. **Performance Assumptions**: Conservative targets with monitoring
3. **Token Security**: Standard JWT with proven libraries
4. **Memory Leaks**: Aggressive TTL cleanup and monitoring

---

## Security Architecture

### Token-Based Authentication

```go
type TokenService struct {
    signingKey   []byte
    tokenTTL     time.Duration
    nonceStore   *NonceStore // Replay protection
}

// Standard JWT with security best practices
func (ts *TokenService) GenerateToken(appID, pageID string) (string, error)
func (ts *TokenService) VerifyToken(token string) (*PageToken, error)
```

**Security Features**:
- Standard JWT implementation preventing algorithm confusion
- Replay attack prevention with nonce tracking
- Key rotation support
- Cross-application isolation enforcement

### Memory Management

```go
type MemoryManager struct {
    maxPages        int           // Default: 1000 pages per application
    pageTTL         time.Duration // Default: 1 hour
    cleanupInterval time.Duration // Default: 5 minutes
}
```

**Resource Protection**:
- Per-application page limits
- TTL-based automatic cleanup
- Memory usage monitoring
- Graceful degradation under pressure

---

## Implementation Strategy (LLM-Assisted Development)

### Immediate Implementation Approach
With LLM-assisted development, implementation can proceed immediately in focused iterations:

### Phase 1: Security Foundation (Immediate)
**Tasks 1-30**: Complete in focused development sessions
- Application isolation with JWT tokens
- Page lifecycle management  
- Template analysis and fragment identification
- Comprehensive security testing

### Phase 2: HTML Diffing-Enhanced Four-Tier System (Following Phase 1)
**Tasks 31-50**: Build HTML diffing-based intelligent update system
- HTML diffing engine for change pattern analysis
- Static/dynamic generation for text-only changes (Strategy 1)
- Marker compilation system for position-discoverable changes (Strategy 2)
- Granular operations for simple structural changes (Strategy 3)
- Fragment replacement for complex structural changes (Strategy 4)
- Strategy selection based on HTML diff complexity scoring
- Performance validation with HTML diffing overhead analysis

### Phase 3: Production Readiness (Final phase)
**Tasks 51-60**: Complete production features
- Comprehensive metrics and monitoring
- Operational requirements (health checks, logging)
- Documentation and examples
- Performance benchmarking and validation

### Success Criteria for v1.0 Release

**Security**:
- Zero cross-application data leaks in security testing
- JWT implementation passes security audit
- Memory usage bounded under load

**Performance**:
- 85-95% size reduction for text-only changes (Strategy 1) - 60-70% of templates
- 70-85% size reduction for position-discoverable changes (Strategy 2) - 15-20% of templates
- 60-80% size reduction for simple operations (Strategy 3) - 10-15% of templates
- 40-60% size reduction for complex changes (Strategy 4) - 5-10% of templates
- >90% strategy selection accuracy through HTML diff analysis
- P95 update generation latency under 75ms (includes HTML diffing processing)
- Support 1,000 concurrent pages per instance
- 100% template compatibility with four-tier fallback

**Reliability**:
- 99.9% uptime in staging environment
- Comprehensive error handling and recovery
- Memory leaks eliminated through monitoring

---

## Appendix: Technical Specifications

### Fragment ID Generation

```go
func generateFragmentID(templatePath string, position int) string {
    h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", templatePath, position)))
    return fmt.Sprintf("f-%x", h[:8]) // 16-character deterministic ID
}
```

### Update Generation Strategy

```go
type UpdateConfig struct {
    PreferValuePatches   bool // Default: true (primary strategy)
    MaxValueUpdatesPerFragment int  // Default: 50
    FallbackToReplace    bool // Default: true
}
```

### Error Handling

```go
var (
    ErrPageNotFound       = errors.New("page not found")
    ErrInvalidApplication = errors.New("invalid application access")
    ErrTokenExpired       = errors.New("token expired")
    ErrMemoryBudgetExceeded = errors.New("memory budget exceeded")
)
```

### Metrics Collection

```go
// Simple built-in metrics - no external dependencies
type Metrics struct {
    PagesCreated         uint64
    UpdatesGenerated     uint64  
    UpdateLatencyP95     float64  // milliseconds
    BandwidthSavingsAvg  float64  // percentage
    MemoryUsageBytes     uint64
    TokenValidationFails uint64
}

// Optional: Export in Prometheus format if needed
func (m *Metrics) PrometheusText() string { /* ... */ }
```

---

**Note**: This v1.0-focused design prioritizes reliability and security over aggressive optimization. The architecture establishes a solid foundation for future enhancements while delivering immediate value through secure multi-tenant fragment updates.