# LiveTemplate - Ultra-Efficient HTML Template Updates

LiveTemplate is a high-performance Go library for ultra-efficient HTML template update generation using **tree-based optimization**. It provides secure multi-tenant isolation with JWT-based authentication and achieves exceptional bandwidth savings through intelligent fragment updates.

## 🚀 Quick Start

```bash
# Install the library
go get github.com/livefir/livetemplate

# Run tests
go test -v ./...

# Run with CI validation
./scripts/validate-ci.sh

# Run load tests (production validation)
go test -run "TestProduction" -v
```

## ✨ Key Features

### Tree-Based Optimization
- **Single unified strategy** that adapts to all template patterns
- **92%+ bandwidth savings** for typical real-world templates
- **Hierarchical template parsing** supporting nested conditionals, ranges, and complex structures
- **Static/dynamic separation** with client-side caching for maximum efficiency

### Security & Reliability
- **Multi-tenant isolation** with JWT-based authentication
- **Cross-application access prevention** with application boundaries
- **Memory management** with configurable limits and cleanup
- **Production-ready** with comprehensive security testing

### Performance
- **P95 latency <75ms** under production load
- **1000+ concurrent pages** support per instance
- **Tree-based parsing** for optimal update generation
- **Memory bounds** with automatic cleanup

## 📖 Core API

### Application Management

```go
// Create a new isolated application
app, err := livetemplate.NewApplication(
    livetemplate.WithMaxMemoryMB(200),
    livetemplate.WithApplicationMetricsEnabled(true),
)
if err != nil {
    panic(err)
}
defer app.Close()

// Create a page within the application
tmpl := template.Must(template.New("page").Parse(`
    <div class="app">
        <h1>{{.Title}}</h1>
        <p>User: {{.User}}</p>
        <div class="content">{{.Content}}</div>
    </div>
`))

data := map[string]interface{}{
    "Title":   "My Application",
    "User":    "John Doe", 
    "Content": "Welcome to LiveTemplate!",
}

page, err := app.NewApplicationPage(tmpl, data)
if err != nil {
    panic(err)
}
defer page.Close()
```

### JWT Token-Based Access

```go
// Get JWT token for the page (for WebSocket connections)
token := page.GetToken()

// Later, retrieve the page using the token
retrievedPage, err := app.GetApplicationPage(token)
if err != nil {
    panic(err) // Could be invalid token, expired, or cross-app access
}

// Generate initial HTML
html, err := retrievedPage.Render()
if err != nil {
    panic(err)
}
fmt.Println("Initial HTML:", html)
```

### Real-time Fragment Updates

```go
// Update data and get efficient fragment updates
newData := map[string]interface{}{
    "Title":   "Updated Application", 
    "User":    "Jane Doe",           
    "Content": "Real-time updates!",  
}

fragments, err := page.RenderFragments(context.Background(), newData)
if err != nil {
    panic(err)
}

// Process fragment updates - tree-based structure
for _, fragment := range fragments {
    fmt.Printf("Fragment %s (Strategy: %s): %v\n", 
        fragment.ID, fragment.Strategy, fragment.Data)
}
```

### WebSocket Integration Example

```go
func websocketHandler(w http.ResponseWriter, r *http.Request) {
    // Extract token from WebSocket request
    token := r.Header.Get("Authorization")
    
    // Get the page using JWT token (with application isolation)
    page, err := app.GetApplicationPage(token)
    if err != nil {
        http.Error(w, "Unauthorized", 401)
        return
    }
    defer page.Close()
    
    // Upgrade to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    
    // Handle real-time updates
    for {
        var newData map[string]interface{}
        if err := conn.ReadJSON(&newData); err != nil {
            break
        }
        
        // Generate efficient fragment updates
        fragments, err := page.RenderFragments(context.Background(), newData)
        if err != nil {
            log.Printf("Fragment generation error: %v", err)
            continue
        }
        
        // Send fragment updates to client
        for _, fragment := range fragments {
            if err := conn.WriteJSON(fragment); err != nil {
                return
            }
        }
    }
}
```

## 🎯 Tree-Based Optimization

LiveTemplate uses **tree-based optimization** to analyze templates and generate minimal update structures:

### How It Works

1. **Template Parsing**: Hierarchical parsing of Go templates into structured boundaries
2. **Static/Dynamic Separation**: Identifies static HTML content vs dynamic template values
3. **Tree Structure Generation**: Creates minimal client data structures similar to Phoenix LiveView
4. **Incremental Updates**: Sends only changed dynamic values, static content cached client-side

### Example: Nested Template Optimization

```go
templateSource := `{{range .Users}}<div>{{if .Active}}✓{{else}}✗{{end}} {{.Name}}</div>{{end}}`

data := map[string]interface{}{
    "Users": []interface{}{
        map[string]interface{}{"Name": "Alice", "Active": true},
        map[string]interface{}{"Name": "Bob", "Active": false},
    },
}

// First render includes full structure
result := {
    "s": ["",""],
    "0": [
        {"s": ["<div>"," ","</div>"], "0": {"s": ["✓"], "0": ""}, "1": "Alice"},
        {"s": ["<div>"," ","</div>"], "0": {"s": ["✗"], "0": ""}, "1": "Bob"}
    ]
}

// Subsequent updates send only changed values (static content cached)
update := {
    "0": [
        {"0": {"0": ""}, "1": "Alice"}, // Only changed values
        {"0": {"0": ""}, "1": "Robert"} // Name changed
    ]
}
```

### Supported Template Constructs

- **Simple Fields**: `{{.Name}}` - Direct value substitution
- **Conditionals**: `{{if .Active}}...{{else}}...{{end}}` - Branch selection
- **Ranges**: `{{range .Items}}...{{end}}` - List iteration with individual item tracking  
- **Nested Structures**: Complex combinations with proper hierarchical parsing
- **Static Content**: Preserved and cached client-side for maximum efficiency

## 🔒 Security & Multi-tenancy

### Application Isolation
```go
// Create separate applications (complete isolation)
tenantA, _ := livetemplate.NewApplication()
tenantB, _ := livetemplate.NewApplication()

// Pages are isolated within applications
pageA, _ := tenantA.NewApplicationPage(tmpl, dataA)
pageB, _ := tenantB.NewApplicationPage(tmpl, dataB)

// Cross-tenant access is blocked by JWT validation
tokenA := pageA.GetToken()
_, err := tenantB.GetApplicationPage(tokenA) // Returns error: cross-application access denied
```

### JWT Token Security
- **Tamper-proof**: Cryptographically signed JWT tokens
- **Application-scoped**: Tokens only work within the originating application
- **Expiration**: Configurable TTL with automatic cleanup
- **Replay protection**: Optional anti-replay mechanisms

## 📊 Performance Characteristics

Based on production load testing with 1000+ concurrent pages:

### Latency Performance
- **P95 latency**: <75ms for fragment generation
- **Page creation**: >70,000 pages/sec  
- **Fragment generation**: >16,000 fragments/sec
- **Tree parsing**: <5ms average, <25ms max

### Bandwidth Efficiency  
- **Tree-based optimization**: 92%+ size reduction for typical templates
- **Complex nested templates**: 95.9% savings (24 bytes vs 590 bytes)
- **Simple text updates**: 75%+ savings with static content caching
- **Real-world applications**: 85-95% bandwidth reduction

### Memory Management
- **Memory per page**: <8MB for typical applications
- **Automatic cleanup**: TTL-based expiration with background cleanup
- **Bounded memory**: Configurable limits with graceful degradation
- **Memory leak protection**: Comprehensive leak detection and prevention

## 🛡️ Production Readiness

### Security Testing
```go
// Comprehensive security test suite
go test -run "TestSecurity" -v  // Multi-tenant isolation
go test -run "TestPenetration" -v  // Security attacks  
go test -run "TestIntegration" -v  // End-to-end security
```

### Load Testing
```go
// Production load testing
go test -run "TestProduction_LoadTesting" -v     // 1000+ concurrent pages
go test -run "TestProduction_MemoryLeak" -v      // Memory leak detection  
go test -run "TestProduction_Benchmark" -v       // Performance benchmarks
```

### Monitoring & Metrics
```go
// Application-level metrics
metrics := app.GetApplicationMetrics()
fmt.Printf("Active pages: %d\n", metrics.ActivePages)
fmt.Printf("Memory usage: %d bytes (%.1f%%)\n", 
    metrics.MemoryUsage, metrics.MemoryUsagePercent)
fmt.Printf("Token failures: %d\n", metrics.TokenFailures)

// Page-level metrics  
pageMetrics := page.GetApplicationPageMetrics()
fmt.Printf("Fragment generations: %d\n", pageMetrics.TotalGenerations)
fmt.Printf("Success rate: %.1f%%\n", 
    float64(pageMetrics.SuccessfulGenerations)/float64(pageMetrics.TotalGenerations)*100)
```

## 🎛️ Configuration Options

### Application Configuration
```go
app, err := livetemplate.NewApplication(
    livetemplate.WithMaxMemoryMB(500),           // Memory limit
    livetemplate.WithApplicationMetricsEnabled(true), // Enable metrics
)
```

### Memory Management
```go
// Set memory limits and cleanup
app, err := livetemplate.NewApplication(
    livetemplate.WithMaxMemoryMB(200),  // 200MB limit per application
)

// Manual cleanup
expiredCount := app.CleanupExpiredPages()
fmt.Printf("Cleaned up %d expired pages\n", expiredCount)
```

## 📁 Project Structure

```
livetemplate/
├── application.go              # Public API - Application management
├── page.go                     # Public API - Fragment types
├── internal/                   # Internal implementation
│   ├── app/                   # Application isolation & lifecycle  
│   ├── page/                  # Page session management
│   ├── token/                 # JWT token service
│   └── strategy/              # Tree-based optimization
├── examples/                   # Usage examples and demos
├── load_test.go               # Production load testing
├── security_test.go           # Security validation
├── docs/                      # Design documentation
│   ├── HLD.md                # High-level design
│   └── LLD.md                # Implementation roadmap  
└── scripts/
    └── validate-ci.sh         # Full CI validation
```

## 🔧 Development & Testing

### Run All Tests
```bash
# Full test suite with CI validation
./scripts/validate-ci.sh

# Individual test categories
go test -run "TestApplication" -v    # Application API tests
go test -run "TestSecurity" -v       # Security isolation tests  
go test -run "TestProduction" -v     # Load & performance tests
go test -run "TestIntegration" -v    # End-to-end integration tests
```

### Performance Benchmarking
```bash
# Benchmark fragment generation performance
go test -bench="BenchmarkFragment" -benchmem

# Load test with 1000+ concurrent pages
go test -run "TestProduction_LoadTesting" -v

# Memory leak detection over time
go test -run "TestProduction_MemoryLeak" -v
```

## 📚 Documentation

- **[docs/HLD.md](docs/HLD.md)** - High-level design and architecture
- **[docs/LLD.md](docs/LLD.md)** - Low-level implementation details  
- **[CLAUDE.md](CLAUDE.md)** - Development guidelines and implementation notes
- **Load Testing** - Production validation in `load_test.go`
- **Security Testing** - Multi-tenant security in `security_test.go`

## 💻 JavaScript Client Integration

LiveTemplate includes a production-ready JavaScript client for seamless browser integration with tree-based optimization.

### Client Installation

```html
<!-- Include the client library -->
<script src="pkg/client/web/tree-fragment-client.js"></script>
```

### Basic Usage

```javascript
// Initialize the client
const client = new TreeFragmentClient({
    enableLogging: false,
    enableMetrics: true,
    maxCacheSize: 1000
});

// Process initial fragment from Go library
const initialFragment = {
    id: 'user-dashboard',
    data: {"0":"John","1":"42","s":["<div>Welcome "," (Level ",")!</div>"]}
};
const html = client.processFragment(initialFragment, true);
document.getElementById('content').innerHTML = html;

// Process incremental update (only dynamic values)
const updateFragment = {
    id: 'user-dashboard', 
    data: {"0":"Jane","1":"45"} // Static structure cached client-side
};
const updatedHtml = client.processFragment(updateFragment, false);
document.getElementById('content').innerHTML = updatedHtml;

// Calculate bandwidth savings
const savings = client.calculateSavings(initialFragment.data, updateFragment.data);
console.log(`Bandwidth saved: ${savings.savingsFormatted}`);
```

### WebSocket Integration

```javascript
// WebSocket connection with LiveTemplate backend
const ws = new WebSocket('ws://localhost:8080/ws');
const client = new TreeFragmentClient();

ws.onmessage = function(event) {
    const fragment = JSON.parse(event.data);
    
    // Apply tree-based optimization update to DOM
    const html = client.processFragment(fragment, false);
    const element = document.getElementById(fragment.id);
    if (element) {
        element.innerHTML = html;
    }
};

// Send updates to server
function updateData(newData) {
    ws.send(JSON.stringify({
        type: 'update',
        token: pageToken,
        data: newData
    }));
}
```

### Advanced Features

```javascript
// Configure client options
const client = new TreeFragmentClient({
    enableLogging: true,           // Debug logging
    enableMetrics: true,           // Performance tracking
    maxCacheSize: 500,             // Max cached fragments
    autoCleanupInterval: 300000,   // 5 minutes
    showErrors: false              // Error display in DOM
});

// Performance monitoring
const metrics = client.getMetrics();
console.log(`Cache hit rate: ${metrics.cacheHitRate}%`);
console.log(`Average processing time: ${metrics.averageProcessingTime}ms`);
console.log(`Bandwidth saved: ${metrics.bandwidthSaved} bytes`);

// Cache management
const stats = client.getCacheStats();
console.log(`Cached fragments: ${stats.cachedStructures}`);
console.log(`Cache usage: ${stats.cacheUsagePercent}%`);

// Clear specific fragment cache
client.clearCache('fragment-id');

// Clear all caches
client.clearAllCaches();
```

### Browser Testing

Use the built-in browser test suite to validate client functionality:

```bash
# Open browser test suite
open e2e/utils/client/browser-tester.html

# Or run Node.js integration tests
cd e2e/utils/client
node integration-tester.js
```

### Tree Structure Format

The JavaScript client works with tree structures from the Go library:

```javascript
// Simple field structure
{
    "s": ["<p>Hello ", "!</p>"],  // Static parts (cached)
    "0": "World"                   // Dynamic value
}
// Renders: <p>Hello World!</p>

// Complex nested structure  
{
    "s": ["<div>", "</div>"],
    "0": {
        "s": ["<span>", " (", ")</span>"], 
        "0": "John",
        "1": "Admin"
    }
}
// Renders: <div><span>John (Admin)</span></div>

// Range structure
{
    "s": ["<ul>", "</ul>"],
    "0": [
        {"s": ["<li>", "</li>"], "0": "Item 1"},
        {"s": ["<li>", "</li>"], "0": "Item 2"}
    ]
}
// Renders: <ul><li>Item 1</li><li>Item 2</li></ul>
```

### Performance Benefits

- **Client-side caching**: Static structures cached after first render
- **Minimal bandwidth**: Only dynamic values transmitted on updates
- **Tree optimization**: 92%+ bandwidth savings for typical updates
- **Fast processing**: <1ms average fragment processing time
- **Memory efficient**: Smart caching with automatic cleanup

## 🔄 Migration from v0.x

If migrating from older LiveTemplate versions:

### Old API (v0.x)
```go
// Old session-based API (deprecated)
st := livetemplate.New(tmpl)
initial, _ := st.NewSession(ctx, data)
session, _ := st.GetSession(initial.SessionID, initial.Token)  
```

### New API (v1.0)
```go
// New application-based API (recommended)
app, _ := livetemplate.NewApplication()
page, _ := app.NewApplicationPage(tmpl, data)
token := page.GetToken()
retrievedPage, _ := app.GetApplicationPage(token)
```

**Key differences:**
- **Tree-based optimization**: Single unified strategy replaces four-tier system
- **Security**: JWT tokens replace session IDs for better security
- **Isolation**: Application boundaries prevent cross-tenant access  
- **Performance**: Tree-based parsing achieves 92%+ bandwidth savings
- **Memory**: Bounded memory with automatic cleanup and limits

## 🏗️ Use Cases

- **Multi-tenant SaaS platforms** - Secure tenant isolation
- **Real-time dashboards** - Efficient data visualization updates
- **Live collaboration tools** - Minimal bandwidth for real-time sync
- **E-commerce applications** - Dynamic pricing and inventory updates
- **Chat applications** - Efficient message and presence updates
- **Content management systems** - Live preview with fragment updates

## 🤝 Contributing

1. Read the implementation guidelines in `CLAUDE.md`
2. Follow the test-driven development approach outlined in `docs/LLD.md`
3. Ensure security tests pass: `go test -run "TestSecurity" -v`
4. Validate performance: `go test -run "TestProduction" -v`
5. Run full CI validation: `./scripts/validate-ci.sh`

## 📄 License

[Add your license information here]

---

LiveTemplate v1.0 - Ultra-efficient HTML template updates with tree-based optimization and enterprise-grade security.