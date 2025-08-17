# LiveTemplate v1.0 API Reference

## Overview

LiveTemplate v1.0 provides a secure, multi-tenant API for ultra-efficient HTML template rendering with HTML diffing-enhanced strategy selection. The API is built around `Application` and `ApplicationPage` types with JWT-based authentication and comprehensive security isolation.

## Core Architecture

### Application/Page Model
- **Applications** provide isolated execution environments for multi-tenant scenarios
- **Pages** are JWT-secured template instances within an application
- **Cross-application access** is blocked by design for security
- **Memory management** with configurable limits and automatic cleanup

### Security Foundation
- **JWT tokens** for all page access with cryptographic signing
- **Application boundaries** prevent cross-tenant data access
- **Memory isolation** with per-application limits
- **Token validation** with tamper detection and expiration

## Public API

### Application Management

#### `NewApplication(options ...ApplicationOption) (*Application, error)`

Creates a new isolated application instance with optional configuration.

```go
app, err := livetemplate.NewApplication(
    livetemplate.WithMaxMemoryMB(200),
    livetemplate.WithApplicationMetricsEnabled(true),
)
```

**Application Options:**
- `WithMaxMemoryMB(int)` - Set memory limit in MB (default: 100)
- `WithApplicationMetricsEnabled(bool)` - Enable metrics collection (default: true)

#### `Application.NewApplicationPage(tmpl *template.Template, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error)`

Creates a new page within the application with the provided template and data.

```go
page, err := app.NewApplicationPage(tmpl, data)
```

#### `Application.GetApplicationPage(token string) (*ApplicationPage, error)`

Retrieves a page by JWT token with application boundary enforcement.

```go
page, err := app.GetApplicationPage(token) // Cross-app access blocked
```

#### `Application.GetPageCount() int`

Returns the total number of active pages in the application.

#### `Application.CleanupExpiredPages() int`

Removes expired pages and returns the count of cleaned pages.

#### `Application.GetApplicationMetrics() ApplicationMetrics`

Returns comprehensive metrics for the application.

#### `Application.Close() error`

Releases all application resources and closes the application.

### Page Operations

#### `ApplicationPage.Render() (string, error)`

Generates the complete HTML output for the current page state.

```go
html, err := page.Render()
```

#### `ApplicationPage.RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error)`

Generates efficient fragment updates using HTML diffing-enhanced strategy selection.

```go
fragments, err := page.RenderFragments(context.Background(), newData)
for _, fragment := range fragments {
    // Process fragment update
    fmt.Printf("Strategy: %s, Action: %s\n", fragment.Strategy, fragment.Action)
}
```

#### `ApplicationPage.GetToken() string`

Returns the JWT token for secure page access.

```go
token := page.GetToken() // Use for WebSocket authentication
```

#### `ApplicationPage.SetData(data interface{}) error`

Updates the page data state.

#### `ApplicationPage.GetData() interface{}`

Returns the current page data.

#### `ApplicationPage.GetTemplate() *template.Template`

Returns the page template.

#### `ApplicationPage.GetApplicationPageMetrics() ApplicationPageMetrics`

Returns page-specific performance metrics.

#### `ApplicationPage.Close() error`

Releases page resources and removes from application.

## Data Types

### Fragment

Represents an update fragment with strategy information.

```go
type Fragment struct {
    ID       string            `json:"id"`       // Unique fragment identifier
    Strategy string            `json:"strategy"` // "static_dynamic", "markers", "granular", "replacement"
    Action   string            `json:"action"`   // Strategy-specific action
    Data     interface{}       `json:"data"`     // Strategy-specific payload
    Metadata *FragmentMetadata `json:"metadata,omitempty"` // Performance information
}
```

### FragmentMetadata

Contains performance and optimization information.

```go
type FragmentMetadata struct {
    GenerationTime   time.Duration `json:"generation_time"`   // Time to generate fragment
    OriginalSize     int64         `json:"original_size"`     // Original HTML size
    CompressedSize   int64         `json:"compressed_size"`   // Fragment update size
    CompressionRatio float64       `json:"compression_ratio"` // Size reduction ratio
    Strategy         string        `json:"strategy"`          // Selected strategy
    Confidence       float64       `json:"confidence"`        // Strategy confidence score
    FallbackUsed     bool          `json:"fallback_used"`     // Whether fallback was used
}
```

### ApplicationMetrics

Application-level performance data.

```go
type ApplicationMetrics struct {
    ApplicationID      string        `json:"application_id"`       // Unique application ID
    PagesCreated       int64         `json:"pages_created"`        // Total pages created
    PagesDestroyed     int64         `json:"pages_destroyed"`      // Total pages destroyed
    ActivePages        int64         `json:"active_pages"`         // Current active pages
    MaxConcurrentPages int64         `json:"max_concurrent_pages"` // Peak concurrent pages
    TokensGenerated    int64         `json:"tokens_generated"`     // Total JWT tokens generated
    TokensVerified     int64         `json:"tokens_verified"`      // Total tokens verified
    TokenFailures      int64         `json:"token_failures"`       // Token validation failures
    FragmentsGenerated int64         `json:"fragments_generated"`  // Total fragments generated
    GenerationErrors   int64         `json:"generation_errors"`    // Fragment generation errors
    MemoryUsage        int64         `json:"memory_usage"`         // Current memory usage (bytes)
    MemoryUsagePercent float64       `json:"memory_usage_percent"` // Memory usage percentage
    MemoryStatus       string        `json:"memory_status"`        // Memory status
    RegistryCapacity   float64       `json:"registry_capacity"`    // Registry capacity usage
    Uptime             time.Duration `json:"uptime"`               // Application uptime
    StartTime          time.Time     `json:"start_time"`           // Application start time
}
```

### ApplicationPageMetrics

Page-specific performance data.

```go
type ApplicationPageMetrics struct {
    PageID                string  `json:"page_id"`                 // Unique page ID
    ApplicationID         string  `json:"application_id"`          // Parent application ID
    CreatedAt             string  `json:"created_at"`              // Creation timestamp
    LastAccessed          string  `json:"last_accessed"`           // Last access timestamp
    Age                   string  `json:"age"`                     // Page age
    IdleTime              string  `json:"idle_time"`               // Time since last access
    MemoryUsage           int64   `json:"memory_usage"`            // Page memory usage
    FragmentCacheSize     int     `json:"fragment_cache_size"`     // Fragment cache size
    TotalGenerations      int64   `json:"total_generations"`       // Total fragment generations
    SuccessfulGenerations int64   `json:"successful_generations"`  // Successful generations
    FailedGenerations     int64   `json:"failed_generations"`      // Failed generations
    AverageGenerationTime string  `json:"average_generation_time"` // Average generation time
    ErrorRate             float64 `json:"error_rate"`              // Error rate percentage
}
```

## HTML Diffing-Enhanced Strategy Selection

### Strategy 1: Static/Dynamic (85-95% reduction)
**When**: Pure text content changes, HTML structure identical

```go
// Fragment.Strategy = "static_dynamic"
// Fragment.Data contains StaticDynamicData with text-only updates
```

### Strategy 2: Marker Compilation (70-85% reduction)  
**When**: Attribute changes (with or without text changes)

```go
// Fragment.Strategy = "markers"
// Fragment.Data contains marker-based updates for attributes and values
```

### Strategy 3: Granular Operations (60-80% reduction)
**When**: Pure structural changes (no text/attribute changes)

```go
// Fragment.Strategy = "granular"
// Fragment.Data contains specific operations like append, prepend, remove
```

### Strategy 4: Fragment Replacement (40-60% reduction)
**When**: Complex mixed changes (structural + text/attribute)

```go
// Fragment.Strategy = "replacement"
// Fragment.Data contains complete HTML replacement content
```

## Security Features

### JWT Token-Based Access
- **Cryptographically signed** JWT tokens for all page access
- **Application-scoped** tokens that only work within the originating application
- **Automatic expiration** with configurable TTL
- **Tamper detection** with signature verification

### Multi-tenant Isolation
- **Complete application isolation** - no cross-application data access
- **Memory boundaries** - per-application memory limits
- **Token boundaries** - JWT tokens scoped to applications
- **Resource isolation** - separate cleanup and lifecycle management

### Error Handling
- **Security-focused errors** - no information leakage in error messages
- **Cross-application access** returns generic "access denied" errors
- **Token tampering** detected and blocked
- **Memory limits** enforced with graceful degradation

## Performance Characteristics

### Latency Targets
- **P95 latency**: <75ms (including HTML diffing overhead)
- **Page creation**: >70,000 pages/sec
- **Fragment generation**: >16,000 fragments/sec
- **HTML diffing**: <10ms average, <50ms max

### Memory Management
- **Bounded memory**: Configurable per-application limits
- **Automatic cleanup**: TTL-based expiration with background cleanup
- **Memory per page**: <12MB with HTML diffing overhead
- **Leak protection**: Comprehensive detection and prevention

### Strategy Distribution
- **Strategy 1**: 60-70% of fragment updates (highest efficiency)
- **Strategy 2**: 15-20% of fragment updates (attribute changes)
- **Strategy 3**: 10-15% of fragment updates (structural changes)
- **Strategy 4**: 5-10% of fragment updates (complex changes)

## Usage Patterns

### Initial Page Load (HTTP Handler)
```go
func pageHandler(w http.ResponseWriter, r *http.Request) {
    app, _ := livetemplate.NewApplication()
    defer app.Close()
    
    data := getPageData(r)
    page, err := app.NewApplicationPage(template, data)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    defer page.Close()
    
    html, _ := page.Render()
    token := page.GetToken()
    
    // Send HTML + token to client for WebSocket setup
    renderPageTemplate(w, html, token)
}
```

### WebSocket Real-time Updates
```go
func websocketHandler(w http.ResponseWriter, r *http.Request) {
    token := r.Header.Get("Authorization")
    
    page, err := app.GetApplicationPage(token)
    if err != nil {
        http.Error(w, "Unauthorized", 401)
        return
    }
    defer page.Close()
    
    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    for {
        var newData map[string]interface{}
        conn.ReadJSON(&newData)
        
        fragments, _ := page.RenderFragments(context.Background(), newData)
        for _, fragment := range fragments {
            conn.WriteJSON(fragment)
        }
    }
}
```

### Multi-tenant SaaS Application
```go
// Each tenant gets isolated application
tenantApps := make(map[string]*livetemplate.Application)

func getTenantApp(tenantID string) *livetemplate.Application {
    if app, exists := tenantApps[tenantID]; exists {
        return app
    }
    
    app, _ := livetemplate.NewApplication(
        livetemplate.WithMaxMemoryMB(100), // Limit per tenant
    )
    tenantApps[tenantID] = app
    return app
}

func handleTenantRequest(tenantID string, template *template.Template, data interface{}) {
    app := getTenantApp(tenantID)
    page, _ := app.NewApplicationPage(template, data)
    
    // Pages are automatically isolated by application boundary
    // Cross-tenant access blocked by JWT validation
    token := page.GetToken() // Scoped to this tenant's application
}
```

## Migration from v0.x

### Key Changes in v1.0
1. **Application-based architecture** replaces session-based API
2. **JWT tokens** replace session IDs for better security
3. **HTML diffing** enhances strategy selection accuracy
4. **Memory management** with bounded limits and automatic cleanup
5. **Multi-tenant isolation** with application boundaries

### Migration Path
```go
// v0.x (deprecated)
st := livetemplate.New(tmpl)
initial, _ := st.NewSession(ctx, data)
session, _ := st.GetSession(initial.SessionID, initial.Token)

// v1.0 (current)
app, _ := livetemplate.NewApplication()
page, _ := app.NewApplicationPage(tmpl, data)
token := page.GetToken()
retrievedPage, _ := app.GetApplicationPage(token)
```

<<<<<<< HEAD
## Implementation Notes

### Internal Types (Not Public API)

The following types are implementation details and are not exported:

- `templateFragment` - Internal fragment representation
- `rangeFragment` - Range-specific fragment handling
- `conditionalFragment` - Conditional block fragments
- `templateTracker` - Data change tracking
- `fragmentExtractor` - Fragment extraction logic
- `advancedTemplateAnalyzer` - Template analysis

### Fragment Actions

The `Update.Action` field can contain:

- `"replace"` - Replace element content
- `"append"` - Add element to end of container
- `"prepend"` - Add element to beginning of container
- `"remove"` - Remove element
- `"insertafter"` - Insert after reference element (requires RangeInfo.ReferenceID)
- `"insertbefore"` - Insert before reference element (requires RangeInfo.ReferenceID)

### Range Operations

For templates with `{{range}}` blocks, StateTemplate automatically generates granular list updates:

```html
{{range .Items}}
<div>{{.Name}}</div>
{{end}}
```

When items are added, removed, or reordered, individual `Update` messages are generated with appropriate actions and range information.

## Migration from Earlier Versions

If you have code referencing older type names:

- `Renderer` → `Renderer` (already clean)
- `Update` → `Update` (already clean)
- `Config` → (removed, use Options pattern)
- `AddTemplate()` → Use Parse methods instead

=======
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d
## Error Handling

### Common Error Scenarios
- `"cross-application access denied"` - Token used in wrong application
- `"page not found"` - Invalid or expired token
- `"signature is invalid"` - Token tampering detected
- `"insufficient memory"` - Application memory limit reached
- `"registry at capacity"` - Page limit reached

### Best Practices
- Always check errors from `GetApplicationPage()` for security violations
- Use `defer page.Close()` to ensure resource cleanup
- Monitor `ApplicationMetrics.TokenFailures` for security issues
- Set appropriate memory limits based on expected usage

## Concurrency

### Thread Safety
- All public API methods are **thread-safe**
- Multiple goroutines can safely access the same application/page
- JWT tokens can be safely shared across goroutines
- Metrics collection is thread-safe

### Performance Considerations
- Use application instances across multiple requests for efficiency
- Page instances are lightweight - create as needed
- JWT validation is optimized for high throughput
- Memory cleanup runs in background goroutines