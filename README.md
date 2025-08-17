# LiveTemplate - Ultra-Efficient HTML Template Update Generation

LiveTemplate is a high-performance Go library for ultra-efficient HTML template update generation using HTML diffing-enhanced four-tier strategy selection. It analyzes actual HTML changes to select optimal strategies and provides secure multi-tenant isolation with JWT-based authentication.

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

### HTML Diffing-Enhanced Strategy Selection
- **Strategy 1: Static/Dynamic** (85-95% reduction) - Text-only changes
- **Strategy 2: Marker Compilation** (70-85% reduction) - Position-discoverable changes  
- **Strategy 3: Granular Operations** (60-80% reduction) - Simple structural changes
- **Strategy 4: Fragment Replacement** (40-60% reduction) - Complex structural changes

### Security & Reliability
- **Multi-tenant isolation** with JWT-based authentication
- **Cross-application access prevention** with application boundaries
- **Memory management** with configurable limits and cleanup
- **Production-ready** with comprehensive security testing

### Performance
- **P95 latency <75ms** under production load
- **1000+ concurrent pages** support per instance
- **HTML diffing** for accurate strategy selection
- **Memory bounds** with automatic cleanup

## 📖 Core API

<<<<<<< HEAD
The library consists of four main components:

- **Renderer**: Main orchestrator managing template parsing and real-time updates
- **TemplateTracker**: Monitors data dependencies and detects changes using reflection
- **FragmentExtractor**: Extracts and categorizes template fragments for granular updates
- **TemplateAnalyzer**: Provides advanced template analysis and optimization

For detailed architecture documentation, see [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Usage

### Basic Real-time Rendering
=======
### Application Management
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d

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
    "Title":   "Updated Application", // Text change -> Static/Dynamic strategy
    "User":    "Jane Doe",           // Text change -> Static/Dynamic strategy  
    "Content": "Real-time updates!",  // Text change -> Static/Dynamic strategy
}

fragments, err := page.RenderFragments(context.Background(), newData)
if err != nil {
    panic(err)
}

// Process fragment updates
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

## 🎯 HTML Diffing-Enhanced Strategy Selection

LiveTemplate uses **HTML diffing** to analyze actual changes and select the optimal strategy:

### Strategy 1: Static/Dynamic (85-95% reduction)
**When**: Pure text content changes, HTML structure identical
```html
<!-- Old -->
<div class="user">John Doe</div>
<!-- New --> 
<div class="user">Jane Smith</div>
<!-- Result: StaticDynamicData with text-only updates -->
```

### Strategy 2: Marker Compilation (70-85% reduction)  
**When**: Attribute changes (with or without text changes)
```html
<!-- Old -->
<div class="status-active">Online</div>
<!-- New -->
<div class="status-inactive">Offline</div> 
<!-- Result: Marker-based attribute + text updates -->
```

### Strategy 3: Granular Operations (60-80% reduction)
**When**: Pure structural changes (no text/attribute changes)
```html
<!-- Old -->
<ul><li>Item 1</li></ul>
<!-- New -->
<ul><li>Item 1</li><li>Item 2</li></ul>
<!-- Result: Granular append operations -->
```

### Strategy 4: Fragment Replacement (40-60% reduction)
**When**: Complex mixed changes (structural + text/attribute)
```html
<!-- Old -->
<div class="card"><span>User: John</span></div>
<!-- New -->
<table class="profile"><tr><td>John</td><td>Admin</td></tr></table>
<!-- Result: Complete fragment replacement -->
```

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
- **P95 latency**: <75ms (including HTML diffing overhead)
- **Page creation**: >70,000 pages/sec  
- **Fragment generation**: >16,000 fragments/sec
- **HTML diffing**: <10ms average, <50ms max

### Bandwidth Efficiency  
- **Strategy 1 (Text-only)**: 85-95% size reduction (60-70% of cases)
- **Strategy 2 (Attributes)**: 70-85% size reduction (15-20% of cases)  
- **Strategy 3 (Structural)**: 60-80% size reduction (10-15% of cases)
- **Strategy 4 (Complex)**: 40-60% size reduction (5-10% of cases)

### Memory Management
- **Memory per page**: <12MB with HTML diffing overhead
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

<<<<<<< HEAD
func main() {
    // Create renderer
    tmpl := template.Must(template.New("example").Parse(`
        <div>
            <h1>{{.Title}}</h1>
            <p>Welcome, {{.User.Name}}!</p>
            {{range .Items}}
                <span>{{.}}</span>
            {{end}}
        </div>
    `))

    config := statetemplate.Config{
        WrapperTagName: "div",
        IDPrefix:       "fragment-",
    }

    renderer := statetemplate.NewRenderer("main", tmpl, config)

    // Initial render
    data := struct {
        Title string
        User  struct { Name string }
        Items []string
    }{
        Title: "My App",
        User:  struct{ Name string }{Name: "John"},
        Items: []string{"item1", "item2"},
    }

    // Create StateTemplate instance
    st := statetemplate.New(tmpl,
        statetemplate.WithExpiration(24*time.Hour),
    )

    // For initial page load - get full HTML with session info
    initial, err := st.NewSession(context.Background(), data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Initial HTML: %s\n", initial.HTML)
    fmt.Printf("Session ID: %s\n", initial.SessionID)

    // For real-time updates - get existing session and stream updates
    session, err := st.GetSession(initial.SessionID, initial.Token)
    if err != nil {
        panic(err)
    }
    defer session.Close()

    // Listen for real-time fragment updates
    go func() {
        for update := range session.Updates() {
            // Send to WebSocket clients
            fmt.Printf("Fragment %s: %s\n", update.FragmentID, update.HTML)
        }
    }()

    // Trigger updates with new data
    newData := struct {
        Title string
        User  struct { Name string }
        Items []string
    }{
        Title: "Updated App",
        User:  struct{ Name string }{Name: "Jane"},
        Items: []string{"item1", "item2", "item3"},
    }

    // This sends fragment updates to session.Updates() channel
    if err := session.Render(context.Background(), newData); err != nil {
        panic(err)
    }
}
=======
// Manual cleanup
expiredCount := app.CleanupExpiredPages()
fmt.Printf("Cleaned up %d expired pages\n", expiredCount)
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d
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
│   ├── diff/                  # HTML diffing engine
│   └── strategy/              # Strategy selection & optimization
├── load_test.go               # Production load testing
├── security_test.go           # Security validation
├── docs/                      # Design documentation
│   ├── HLD.md                # High-level design
│   └── LLD.md                # Implementation roadmap  
└── scripts/
    └── validate-ci.sh         # Full CI validation
```

<<<<<<< HEAD
## API Reference

### Core Types

#### `StateTemplate`

The main entry point that manages sessions and template rendering.

#### `Session`

Represents an isolated rendering session for a single client.

#### `Config`

```go
type Config struct {
    Expiration      time.Duration // Session expiration (default: 24h)
    Store           sessionStore  // Session storage backend
    SigningKey      []byte        // For secure session tokens
    MaxSessions     int           // Maximum concurrent sessions
    CleanupInterval time.Duration // Cleanup frequency (default: 5m)
}
```

#### `Update`

```go
type Update struct {
    FragmentID string    `json:"fragment_id"`
    HTML       string    `json:"html"`
    Action     string    `json:"action"` // "replace", "append", "remove"
    Timestamp  time.Time `json:"timestamp"`
}
```

#### `InitialRender`

```go
type InitialRender struct {
    HTML      string `json:"html"`       // Full annotated HTML
    SessionID string `json:"session_id"` // For subsequent updates
    Token     string `json:"token"`      // For session authentication
}
```

### Key Methods

#### `New(templates *template.Template, options ...Option) *StateTemplate`

Creates a new StateTemplate instance with configuration options.

#### `NewSession(ctx context.Context, data interface{}) (*InitialRender, error)`

Creates a new session and renders the complete template with initial data. Use this for initial page loads.

#### `GetSession(sessionID, token string) (*Session, error)`

Retrieves an existing session for real-time updates. Use this for WebSocket connections.

#### `Render(ctx context.Context, data interface{}) error`

Processes new data and triggers fragment updates. Updates are sent to the `Updates()` channel.

#### `Updates() <-chan Update`

Returns a channel that streams incremental fragment updates. Use this for real-time connections.

#### `Close() error`

Closes the session and releases resources.

## Usage Patterns

### Pattern 1: Initial Page Load (HTTP Handler)

Use `NewSession()` for server-side rendering of the initial page:

```go
func pageHandler(w http.ResponseWriter, r *http.Request) {
    st := statetemplate.New(templates,
        statetemplate.WithExpiration(24*time.Hour),
    )

    data := getPageData(r)

    // Get full HTML for initial page load
    initial, err := st.NewSession(ctx, data)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // Render page template with session info for JavaScript
    pageTemplate.Execute(w, struct {
        Content   string
        SessionID string
        Token     string
    }{
        Content:   initial.HTML,
        SessionID: initial.SessionID,
        Token:     initial.Token,
    })
}
```

### Pattern 2: Real-time Updates (WebSocket Handler)

Use `GetSession()` + `Updates()` for incremental updates:

```go
func websocketHandler(w http.ResponseWriter, r *http.Request) {
    sessionID := r.URL.Query().Get("session_id")
    token := r.URL.Query().Get("token")

    // Get existing session from initial page load
    session, err := st.GetSession(sessionID, token)
    if err != nil {
        http.Error(w, "Invalid session", 401)
        return
    }
    defer session.Close()

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    // Stream incremental updates
    go func() {
        for update := range session.Updates() {
            conn.WriteJSON(update)
        }
    }()

    // Process data changes
    for {
        var newData PageData
        if err := conn.ReadJSON(&newData); err != nil {
            break
        }

        // Trigger fragment updates
        if err := session.Render(ctx, &newData); err != nil {
            log.Printf("Render error: %v", err)
        }
    }
}
```

### Fragment Types

StateTemplate supports four types of template fragments:

#### Simple Fragments

- Direct field access: `{{.Field}}`
- Single data dependency with straightforward updates

#### Conditional Fragments

- If/with blocks: `{{if .Condition}}...{{end}}`
- May appear or disappear based on data conditions

#### Range Fragments

- Loop constructs: `{{range .Items}}...{{end}}`
- Granular item-level tracking for additions, removals, reordering

#### Block Fragments

- Named template sections: `{{block "name" .}}...{{end}}`
- Template composition and inheritance support

#### `UpdateTemplateTracker(tt *TemplateTracker, name string, tmpl *template.Template)`

Registers a template using advanced AST analysis.

## WebSocket Integration

For real-time web applications, StateTemplate generates WebSocket-compatible updates:

```go
// Set up WebSocket handler
func handleWebSocket(conn *websocket.Conn) {
    // Create renderer and channels
    renderer := statetemplate.NewRenderer("main", template, config)
    updateChan := make(chan statetemplate.DataUpdate)
    fragmentChan := make(chan statetemplate.Update)

    go renderer.StartUpdates(updateChan, fragmentChan)

    // Forward real-time updates to WebSocket
    go func() {
        for update := range fragmentChan {
            conn.WriteJSON(map[string]interface{}{
                "type":       "fragment_update",
                "fragmentId": update.FragmentID,
                "html":       update.HTML,
                "action":     update.Action,
            })
        }
    }()
}
```

For comprehensive documentation and examples, see [`docs/EXAMPLES.md`](docs/EXAMPLES.md).

## Example Data Structures

```go
type User struct {
    ID    int
    Name  string
    Email string
}

type AppData struct {
    Title       string
    CurrentUser *User
    UserCount   int
    Articles    []*Article
}
```

## Template Examples

### Header Template

```html
<header>
  <h1>{{.Title}}</h1>
  {{if .CurrentUser}}
  <p>Welcome, {{.CurrentUser.Name}}!</p>
  {{end}}
</header>
```

**Dependencies**: `Title`, `CurrentUser`, `CurrentUser.Name`

### Sidebar Template

```html
<aside>
  <p>Users: {{.UserCount}}</p>
  {{if .CurrentUser}}
  <p>Your ID: {{.CurrentUser.ID}}</p>
  {{end}}
</aside>
```

**Dependencies**: `UserCount`, `CurrentUser`, `CurrentUser.ID`

## Change Detection

The system performs deep comparison of data structures:

- **Primitive fields**: Direct value comparison
- **Struct fields**: Recursive field-by-field comparison
- **Pointer fields**: Handles nil pointers correctly
- **Nested structures**: Tracks changes at any depth

## Performance Considerations

- **Efficient Comparison**: Only compares fields that templates actually use
- **Minimal Re-rendering**: Only notifies about templates that need updates
- **Memory Efficient**: Doesn't store large data structures, just tracks changes
- **Concurrent Safe**: Thread-safe operations with proper synchronization

## Testing

Run the comprehensive test suite:
=======
## 🔧 Development & Testing
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d

### Run All Tests
```bash
# Full test suite with CI validation
./scripts/validate-ci.sh

<<<<<<< HEAD
# Run specific test suites
go test -v -run "TestRenderer"
go test -v -run "TestTemplateTracker"
go test -v -run "TestFragmentExtractor"

# Run examples with timeout
timeout 3s go run examples/simple/main.go
timeout 3s go run examples/e2e/main.go
=======
# Individual test categories
go test -run "TestApplication" -v    # Application API tests
go test -run "TestSecurity" -v       # Security isolation tests  
go test -run "TestProduction" -v     # Load & performance tests
go test -run "TestIntegration" -v    # End-to-end integration tests
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d
```

### Performance Benchmarking
```bash
# Benchmark fragment generation performance
go test -bench="BenchmarkFragment" -benchmem

# Load test with 1000+ concurrent pages
go test -run "TestProduction_LoadTesting" -v

<<<<<<< HEAD
- **Real-time Web Applications**: Live UI updates with WebSocket integration
- **Progressive Web Apps**: Efficient fragment-based updates for smooth UX
- **Live Dashboards**: Real-time data visualization with minimal bandwidth
- **Chat Applications**: Message and user state updates with granular control
- **E-commerce Sites**: Dynamic cart, inventory, and pricing updates
- **Content Management**: Live preview and collaborative editing features

## Project Structure

```text
statetemplate/
├── renderer.go                # Main renderer orchestrator
├── template_tracker.go        # Data change tracking
├── fragment_extractor.go      # Fragment extraction and categorization
├── template_analyzer.go       # Advanced template analysis
├── examples/                  # Usage examples and demos
├── docs/                      # Comprehensive documentation
├── scripts/                   # Development and validation scripts
└── testdata/                  # Test templates and data
=======
# Memory leak detection over time
go test -run "TestProduction_MemoryLeak" -v
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d
```

## 📚 Documentation

- **[docs/HLD.md](docs/HLD.md)** - High-level design and architecture
- **[docs/LLD.md](docs/LLD.md)** - Low-level implementation details  
- **[CLAUDE.md](CLAUDE.md)** - Development guidelines and implementation notes
- **Load Testing** - Production validation in `load_test.go`
- **Security Testing** - Multi-tenant security in `security_test.go`

## 🔄 Migration from v0.x

If migrating from older LiveTemplate versions:

### Old API (v0.x)
```go
// Old session-based API (deprecated)
st := livetemplate.New(tmpl)
initial, _ := st.NewSession(ctx, data)
session, _ := st.GetSession(initial.SessionID, initial.Token)  
```

<<<<<<< HEAD
- [`docs/SESSION_DESIGN.md`](docs/SESSION_DESIGN.md) - Session-based architecture design and implementation
- [`docs/SESSION_IMPLEMENTATION.md`](docs/SESSION_IMPLEMENTATION.md) - Session API implementation guide
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) - Detailed architectural overview
- [`docs/EXAMPLES.md`](docs/EXAMPLES.md) - WebSocket integration guide
- [`examples/README.md`](examples/README.md) - Usage examples and patterns
=======
### New API (v1.0)
```go
// New application-based API (recommended)
app, _ := livetemplate.NewApplication()
page, _ := app.NewApplicationPage(tmpl, data)
token := page.GetToken()
retrievedPage, _ := app.GetApplicationPage(token)
```

**Key differences:**
- **Security**: JWT tokens replace session IDs for better security
- **Isolation**: Application boundaries prevent cross-tenant access  
- **Performance**: HTML diffing enhances strategy selection accuracy
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

LiveTemplate v1.0 - Ultra-efficient HTML template updates with HTML diffing-enhanced strategy selection and enterprise-grade security.
>>>>>>> abfb306309a06c9ffc279d7e7cda8acfc64b604d
