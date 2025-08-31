# Page Rendering Lifecycle

This document explains the complete page rendering lifecycle in LiveTemplate, from application initialization through fragment updates, including the tree-based optimization system and multi-tenant security architecture.

## Simple Overview

LiveTemplate follows a straightforward 4-phase rendering lifecycle:

1. **Setup** → Create secure application + page with JWT tokens
2. **Render** → Generate initial HTML with fragment markers  
3. **Update** → Send optimized tree-based fragments (92%+ bandwidth savings)
4. **Cleanup** → Automatic memory management and resource cleanup

```mermaid
graph LR
    A[🏗️ Setup] --> B[🎨 Render] 
    B --> C[⚡ Update]
    C --> C
    C --> D[🧹 Cleanup]
    
    style A fill:#e3f2fd
    style B fill:#fff3e0  
    style C fill:#e8f5e8
    style D fill:#f3e5f5
```

**Key Benefits:**

- **92%+ bandwidth savings** through tree-based optimization
- **Enterprise security** with multi-tenant JWT isolation  
- **Sub-75ms latency** for real-time updates
- **Phoenix LiveView compatible** client structures

## Detailed Overview

LiveTemplate implements a secure two-layer architecture with tree-based optimization that provides ultra-efficient HTML template updates while maintaining strict multi-tenant isolation. The rendering lifecycle encompasses application management, page creation, template parsing, and fragment generation.

## Architecture Components

The page rendering lifecycle involves these key components:

- **Application**: Multi-tenant isolation container with JWT-based authentication
- **Page**: Isolated user session with stateless design for horizontal scaling
- **TemplateAwareGenerator**: Hierarchical template parsing and boundary detection
- **SimpleTreeGenerator**: Single unified strategy for all template patterns
- **TokenService**: Standard JWT tokens with replay protection
- **PageRegistry**: Thread-safe page storage with TTL cleanup

## Complete Rendering Lifecycle

```mermaid
graph TB
    A[Client Request] --> B[Application Creation/Retrieval]
    B --> C[JWT Token Validation]
    C --> D[Page Creation]
    D --> E[Template Parsing & Boundary Detection]
    E --> F[Initial HTML Rendering]
    F --> G[Fragment Annotation Injection]
    G --> H[Client Receives Initial HTML]
    H --> I[Data Update Trigger]
    I --> J[Fragment Generation]
    J --> K[Tree-Based Optimization]
    K --> L[Static/Dynamic Separation]
    L --> M[Client Fragment Update]
    M --> N{More Updates?}
    N -->|Yes| I
    N -->|No| O[Page Cleanup/Expiration]
```

## Phase 1: Application and Page Initialization

### Application Creation

```mermaid
sequenceDiagram
    participant C as Client
    participant A as Application
    participant T as TokenService
    participant PR as PageRegistry
    
    C->>A: NewApplication(options)
    A->>T: Initialize JWT service
    A->>PR: Initialize page registry
    A->>A: Setup multi-tenant isolation
    A-->>C: Application instance
```

**Process Steps:**

1. **Application Instantiation**: Creates isolated application container
2. **Token Service Setup**: Initializes JWT service with replay protection
3. **Page Registry Setup**: Creates thread-safe page storage with TTL cleanup
4. **Security Boundaries**: Establishes multi-tenant isolation controls

### Page Creation and Template Analysis

```mermaid
sequenceDiagram
    participant A as Application
    participant P as Page
    participant TAG as TemplateAwareGenerator
    participant TB as TemplateBoundary
    
    A->>P: NewPage(template, data, options)
    P->>TAG: Parse template structure
    TAG->>TB: Analyze template boundaries
    TB->>TB: Classify constructs (fields, conditionals, ranges)
    TB-->>TAG: Boundary structure
    TAG-->>P: Parsed template metadata
    P->>P: Generate JWT token
    P-->>A: Page instance with token
```

**Template Boundary Classification:**

- **Static Content**: HTML segments that never change
- **Simple Fields**: `{{.Name}}` - Direct value substitution
- **Conditionals**: `{{if .Active}}...{{else}}...{{end}}` - Branch selection
- **Ranges**: `{{range .Items}}...{{end}}` - List iteration
- **Nested Structures**: Complex combinations with hierarchical parsing

## Phase 2: Initial Rendering

### HTML Generation and Fragment Annotation

```mermaid
sequenceDiagram
    participant C as Client
    participant P as Page
    participant TAG as TemplateAwareGenerator
    participant HTML as HTML Output
    
    C->>P: page.Render()
    P->>TAG: Execute template with data
    TAG->>TAG: Generate HTML content
    TAG->>TAG: Inject fragment annotations
    TAG->>HTML: Annotated HTML
    HTML-->>P: Complete HTML document
    P-->>C: Initial HTML with fragment markers
```

**Fragment Annotation Process:**

1. **Template Execution**: Standard Go template rendering with provided data
2. **Boundary Detection**: Identify dynamic regions during rendering
3. **Fragment ID Generation**: Deterministic IDs based on template + data signature
4. **Annotation Injection**: Insert HTML comments or attributes for client identification
5. **Complete Document**: Return fully annotated HTML ready for client consumption

## Phase 3: Fragment Updates (Tree-Based Optimization)

### Update Trigger and Fragment Generation

```mermaid
sequenceDiagram
    participant C as Client
    participant P as Page
    participant STG as SimpleTreeGenerator
    participant Cache as Static Cache
    
    C->>P: RenderFragments(ctx, newData)
    P->>P: Compare data changes
    P->>STG: Generate tree-based fragments
    STG->>STG: Hierarchical template parsing
    STG->>STG: Static/dynamic separation
    STG->>Cache: Check static content cache
    Cache-->>STG: Cached static segments
    STG->>STG: Generate SimpleTreeData structures
    STG-->>P: Optimized fragment array
    P-->>C: Fragment updates (92%+ bandwidth savings)
```

### Tree-Based Optimization Deep Dive

```mermaid
flowchart TD
    A[New Data] --> B[Template Boundary Analysis]
    B --> C[Hierarchical Parsing]
    C --> D{Change Type}
    D -->|Simple Field| E[Direct Value Update]
    D -->|Conditional| F[Branch Selection]
    D -->|Range| G[List Iteration Analysis]
    D -->|Nested| H[Recursive Tree Processing]
    
    E --> I[Static/Dynamic Separation]
    F --> I
    G --> I
    H --> I
    
    I --> J[Static Content Caching]
    J --> K[Generate SimpleTreeData]
    K --> L[Client-Compatible Structure]
    L --> M[Fragment Output]
    
    style E fill:#e1f5fe
    style F fill:#fff3e0
    style G fill:#f3e5f5
    style H fill:#e8f5e8
```

## Tree-Based Data Structures

### Simple Field Template Example

**Template:**

```html
<p>Hello {{.Name}}!</p>
```

**Generated Tree Structure:**

```json
{
  "s": ["<p>Hello ", "!</p>"],
  "0": "Alice"
}
```

### Complex Nested Structure Example

**Template:**

```html
{{range .Users}}
  <div>
    {{if .Active}}✓{{else}}✗{{end}} 
    {{.Name}}
  </div>
{{end}}
```

**Generated Tree Structure:**

```json
{
  "s": ["", ""],
  "0": [
    {
      "s": ["<div>", " ", "</div>"],
      "0": {"s": ["✓"], "0": ""},
      "1": "Alice"
    },
    {
      "s": ["<div>", " ", "</div>"],
      "0": {"s": ["✗"], "0": ""},
      "1": "Bob"
    }
  ]
}
```

## Phase 4: Client-Side Processing

### Fragment Application and Caching

```mermaid
sequenceDiagram
    participant S as Server
    participant C as Client
    participant SC as Static Cache
    participant DOM as DOM
    
    S->>C: Fragment updates (SimpleTreeData)
    C->>SC: Check static content cache
    SC-->>C: Cached static segments
    C->>C: Reconstruct HTML from tree structure
    C->>DOM: Apply targeted DOM updates
    DOM-->>C: Updated page state
    C->>SC: Update cache with new static content
```

**Client Processing Steps:**

1. **Fragment Reception**: Receive SimpleTreeData structures from server
2. **Static Cache Check**: Verify cached static content availability
3. **Tree Reconstruction**: Rebuild HTML from static segments and dynamic values
4. **DOM Updates**: Apply minimal, targeted changes to existing DOM
5. **Cache Updates**: Store new static content for future updates

## Security and Multi-Tenant Isolation

### Cross-Application Access Prevention

```mermaid
flowchart LR
    A1[App Instance 1] --> P1[Pages 1-100]
    A2[App Instance 2] --> P2[Pages 101-200]
    A3[App Instance 3] --> P3[Pages 201-300]
    
    P1 -.->|❌ Blocked| P2
    P1 -.->|❌ Blocked| P3
    P2 -.->|❌ Blocked| P1
    P2 -.->|❌ Blocked| P3
    P3 -.->|❌ Blocked| P1
    P3 -.->|❌ Blocked| P2
    
    style A1 fill:#e3f2fd
    style A2 fill:#fff3e0
    style A3 fill:#f3e5f5
```

### JWT Token Lifecycle

```mermaid
sequenceDiagram
    participant P as Page
    participant T as TokenService
    participant C as Client
    participant A as Application
    
    P->>T: Generate JWT token
    T->>T: Sign with application-specific secret
    T-->>P: Signed JWT token
    P-->>C: Token for page access
    
    Note over C: Later request
    C->>A: GetPage(token)
    A->>T: Validate token signature
    T->>T: Check replay protection
    T->>T: Verify expiration
    T-->>A: Token validation result
    A-->>C: Page access granted/denied
```

## Performance Characteristics

### Bandwidth Optimization Results

```mermaid
graph LR
    A[Original HTML] -->|92%+ reduction| B[Tree-Based Fragment]
    C[Complex Template 590 bytes] -->|95.9% savings| D[Optimized 24 bytes]
    E[Simple Text Update] -->|75%+ savings| F[Static Cache + Dynamic]
    
    style A fill:#ffcdd2
    style B fill:#c8e6c9
    style C fill:#ffcdd2
    style D fill:#c8e6c9
    style E fill:#ffcdd2
    style F fill:#c8e6c9
```

### Performance Metrics (v1.0 Achieved)

- **Fragment Generation**: >16,000 fragments/sec
- **Page Creation**: >70,000 pages/sec  
- **P95 Latency**: <75ms for fragment generation
- **Template Parsing**: <5ms average, <25ms max
- **Concurrent Support**: 1000+ pages per instance (8GB RAM)
- **Memory Usage**: <8MB per page for typical applications

## Memory Management and Cleanup

### Page Lifecycle Management

```mermaid
stateDiagram-v2
    [*] --> Created: NewPage()
    Created --> Active: First render
    Active --> Active: Fragment updates
    Active --> Inactive: No activity timeout
    Inactive --> Active: New request
    Inactive --> Expired: TTL exceeded
    Active --> Expired: Manual close
    Expired --> [*]: Cleanup resources
    
    note right of Expired
        Memory cleanup
        Token invalidation
        Cache clearing
    end note
```

## Error Handling and Graceful Degradation

### Error Recovery Flow

```mermaid
flowchart TD
    A[Fragment Generation Request] --> B{Template Parse Error?}
    B -->|Yes| C[Log Error Context]
    B -->|No| D[Continue Processing]
    
    C --> E{Graceful Degradation Available?}
    E -->|Yes| F[Return Fallback Fragment]
    E -->|No| G[Return Error Response]
    
    D --> H{Data Validation Error?}
    H -->|Yes| I[Sanitize Data]
    H -->|No| J[Generate Fragments]
    
    I --> J
    J --> K{Generation Success?}
    K -->|Yes| L[Return Fragments]
    K -->|No| M[Return Previous State]
    
    F --> N[Client Continues]
    G --> O[Client Error Handler]
    L --> N
    M --> N
    
    style C fill:#ffcdd2
    style G fill:#ffcdd2
    style M fill:#fff3e0
```

## Integration Points and Extensibility

### Custom Function Integration

The page rendering lifecycle supports custom template functions and middleware:

```mermaid
graph LR
    A[Template Functions] --> B[Template Parsing]
    C[Middleware] --> D[Fragment Generation]
    E[Custom Validators] --> F[Data Processing]
    G[Cache Strategies] --> H[Static Content Optimization]
    
    B --> I[Enhanced Template Processing]
    D --> J[Enriched Fragment Data]
    F --> K[Validated Data Flow]
    H --> L[Optimized Bandwidth Usage]
```

## Monitoring and Observability

The lifecycle includes built-in metrics collection (no external dependencies):

- **Template parsing performance**: Parse time distribution and error rates
- **Fragment generation metrics**: Generation rate, size distribution, optimization ratios
- **Memory usage tracking**: Per-page memory consumption, cleanup effectiveness
- **Security metrics**: Failed authentication attempts, cross-app access blocks
- **Cache effectiveness**: Hit rates, bandwidth savings achieved

## Conclusion

The LiveTemplate page rendering lifecycle provides a comprehensive solution for ultra-efficient HTML template updates with enterprise-grade security. The tree-based optimization system achieves 92%+ bandwidth savings while maintaining strict multi-tenant isolation through JWT-based authentication and application boundaries.

Key benefits:

- **Single Unified Strategy**: Tree-based optimization handles all template complexity
- **Security First**: Multi-tenant isolation prevents cross-application data access
- **Performance Optimized**: Sub-75ms P95 latency with massive bandwidth savings
- **Production Ready**: Comprehensive error handling, monitoring, and resource management
- **Phoenix LiveView Compatible**: Generated structures mirror LiveView client format
