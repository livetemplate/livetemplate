# StateTemplate Session API V2 - Implementation Guide

## Quick Implementation Overview

This document provides a focused implementation guide for the improved StateTemplate session-based architecture with simplified APIs and clear usage patterns.

## Key Design Improvements

1. **Internal types use lowercase** - Better Go conventions
2. **Simplified Update model** - Removed update types burden from clients
3. **Clear API separation** - Different methods for different use cases
4. **Explicit usage patterns** - Clear guidance for server operations

## Core Components to Implement

### 1. Public API Types (Priority: HIGH)

```go
// statetemplate.go - Public API
type StateTemplate struct {
    manager   *sessionManager
    templates *template.Template
}

type Session struct {
    manager *sessionManager
    state   *sessionState
}

// Simple Update - no types to burden clients
type Update struct {
    FragmentID string    `json:"fragment_id"`
    HTML       string    `json:"html"`
    Action     string    `json:"action"` // "replace", "append", "remove"
    Timestamp  time.Time `json:"timestamp"`
}

// Initial render result - for server-side page generation
type InitialRender struct {
    HTML      string `json:"html"`       // Full annotated HTML
    SessionID string `json:"session_id"` // For subsequent updates
    Token     string `json:"token"`      // For session authentication
}
```

### 2. Internal Types (lowercase - unexported)

```go
// session_manager.go - Internal implementation
type sessionManager struct {
    templates *template.Template
    sessions  map[string]*sessionState
    store     sessionStore
    config    Config
    cleanup   *time.Ticker
    mutex     sync.RWMutex
}

type sessionState struct {
    id           string
    token        string
    lastSnapshot map[string]interface{}
    tracker      *TemplateTracker
    updateChan   chan Update
    created      time.Time
    lastAccess   time.Time
    closed       bool
    mutex        sync.RWMutex
}

type sessionStore interface {
    Create(session *sessionState) error
    Get(sessionID string) (*sessionState, error)
    Update(session *sessionState) error
    Delete(sessionID string) error
    GetExpired(before time.Time) ([]*sessionState, error)
}
```

### 3. Primary APIs (Priority: HIGH)

```go
// statetemplate.go - Main entry point
func New(templates *template.Template, options ...Option) *StateTemplate

// Use Case 1: Initial Page Load (Server-Side Rendering)
func (st *StateTemplate) NewSession(ctx context.Context, data interface{}) (*InitialRender, error)

// Use Case 2: Real-time Updates (WebSocket/SSE Connections)
func (st *StateTemplate) GetSession(sessionID, token string) (*Session, error)
func (s *Session) Render(ctx context.Context, data interface{}) error  // Triggers updates
func (s *Session) Updates() <-chan Update                               // Stream of changes
func (s *Session) Close() error                                         // Cleanup
```

### 4. Configuration (Priority: MEDIUM)

```go
// config.go
type Config struct {
    Expiration      time.Duration
    Store           sessionStore
    SigningKey      []byte
    MaxSessions     int
    CleanupInterval time.Duration
}

type Option func(*Config)

func WithExpiration(d time.Duration) Option
func WithStore(store sessionStore) Option
func WithSecurity(signingKey []byte) Option
func WithMaxSessions(max int) Option
```

## Implementation Priority

### Phase 1: Core Session Infrastructure (Week 1)

1. **sessionManager struct** - Internal session management
2. **sessionState struct** - Individual session data
3. **Basic NewSession()** - Returns InitialRender with full HTML
4. **Basic GetSession()** - Retrieves existing sessions

### Phase 2: Real-time Updates (Week 2)

1. **Session.Render()** - Processes data changes and triggers updates
2. **Session.Updates()** - Channel-based update streaming
3. **Fragment detection** - Incremental update calculation
4. **Update delivery** - Send to Updates() channel

### Phase 3: Session Management (Week 3)

1. **Session expiration** - Automatic cleanup
2. **Token security** - Encrypted session tokens
3. **Store interface** - Pluggable session storage
4. **Memory management** - Prevent leaks

## Usage Examples

### Example 1: HTTP Page Handler

```go
func pageHandler(w http.ResponseWriter, r *http.Request) {
    st := statetemplate.New(templates,
        statetemplate.WithExpiration(24*time.Hour),
    )

    data := &PageData{
        User: getCurrentUser(r),
        Posts: getPosts(),
    }

    // Get full HTML for initial page load
    initial, err := st.NewSession(ctx, data)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // Render page template with session info
    pageTemplate.Execute(w, struct {
        Content   string
        SessionID string
        Token     string
    }{
        Content:   initial.HTML,     // Full annotated HTML
        SessionID: initial.SessionID, // For WebSocket connection
        Token:     initial.Token,    // For authentication
    })
}
```

### Example 2: WebSocket Handler

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

    // Stream incremental updates to browser
    go func() {
        for update := range session.Updates() {
            conn.WriteJSON(update)
        }
    }()

    // Process incoming data changes
    for {
        var newData PageData
        if err := conn.ReadJSON(&newData); err != nil {
            break
        }

        // Trigger fragment updates - sends to Updates() channel
        if err := session.Render(ctx, &newData); err != nil {
            log.Printf("Render error: %v", err)
        }
    }
}
```

````

### 4. Security (Priority: HIGH)

```go
// session_security.go
type SessionToken struct {
    SessionID  string    `json:"session_id"`
    IssuedAt   time.Time `json:"issued_at"`
    ExpiresAt  time.Time `json:"expires_at"`
    ClientInfo ClientInfo `json:"client_info"`
    Nonce      string    `json:"nonce"`
}

func (sm *SessionManager) generateToken(sessionID string, clientInfo ClientInfo) (string, error)
func (sm *SessionManager) validateToken(token string) (*SessionToken, error)
func (sm *SessionManager) validateAccess(sessionID, token string) error
````

### 5. Storage Interface (Priority: MEDIUM)

```go
// session_store.go
type SessionStore interface {
    Create(session *Session) error
    Get(sessionID string) (*Session, error)
    Update(session *Session) error
    Delete(sessionID string) error
    GetExpired(before time.Time) ([]*Session, error)
}

// Start with memory implementation
type MemorySessionStore struct {
    sessions map[string]*Session
    mutex    sync.RWMutex
}
```

## Implementation Flow

### Phase 1: Core Session Management

1. **Create session.go** with Session struct
2. **Create session_manager.go** with SessionManager
3. **Create session_handle.go** with SessionHandle
4. **Implement basic lifecycle**: AcquireSession, GetSession, InvalidateSession
5. **Add memory-based storage**: MemorySessionStore

### Phase 2: Data Snapshot Processing

1. **Extend Update struct** with session context
2. **Implement SendDataSnapshot()** logic:
   - First snapshot → render full HTML (UpdateTypeInitial)
   - Subsequent snapshots → generate fragments (UpdateTypeFragment)
3. **Integrate with existing components**:
   - Use existing TemplateTracker for change detection
   - Use existing FragmentExtractor for fragment updates

### Phase 3: Security and Token Management

1. **Create session_security.go** with token handling
2. **Implement token generation/validation**
3. **Add access control** to all session operations
4. **Use crypto/aes** for token encryption

### Phase 4: Cleanup and Expiration

1. **Add background cleanup** goroutine
2. **Implement session expiration** logic
3. **Add memory management** for expired sessions

## Integration Points

### With Existing Renderer

```go
// renderer.go modifications
type Renderer struct {
    // Existing fields...
    sessionManager *SessionManager  // NEW: Optional session manager
}

// Add session mode enabler
func (r *Renderer) EnableSessionMode(config SessionConfig) *SessionManager {
    r.sessionManager = NewSessionManager(r, config)
    return r.sessionManager
}
```

### With Existing TemplateTracker

```go
// template_tracker.go - add session context
func (tt *TemplateTracker) DetectChangesForSession(sessionID string, oldData, newData interface{}) []string
```

### With Existing FragmentExtractor

```go
// fragment_extractor.go - add session-aware rendering
func (fe *FragmentExtractor) RenderFragmentsForSession(sessionID string, data interface{}) ([]Update, error)
```

## Key Implementation Rules

### Thread Safety

- **All session operations MUST be thread-safe**
- Use sync.RWMutex for session data access
- Use sync.Mutex for SessionManager operations

### Security

- **Never allow cross-session data access**
- Always validate tokens before session operations
- Generate cryptographically secure session IDs

### Memory Management

- **Implement aggressive cleanup** of expired sessions
- Set reasonable defaults for session expiration (24 hours)
- Monitor memory usage and implement limits

### Error Handling

- **Graceful degradation** when sessions expire
- Return appropriate errors for invalid tokens
- Handle concurrent session access safely

## Testing Strategy

### Unit Tests Required

```go
func TestSessionManager_AcquireSession(t *testing.T)
func TestSessionManager_GetSession(t *testing.T)
func TestSessionManager_InvalidateSession(t *testing.T)
func TestSessionHandle_SendDataSnapshot(t *testing.T)
func TestSession_Isolation(t *testing.T)        // Critical security test
func TestToken_Generation(t *testing.T)
func TestToken_Validation(t *testing.T)
func TestSession_Expiration(t *testing.T)
```

### Integration Tests Required

```go
func TestMultipleSessionsIsolation(t *testing.T)  // No cross-session data leakage
func TestSessionDataFlow(t *testing.T)            // End-to-end snapshot processing
func TestSessionCleanup(t *testing.T)             // Memory management
```

## File Structure

```text
statetemplate/
├── session.go              # Session and SessionManager types
├── session_handle.go        # SessionHandle implementation
├── session_store.go         # SessionStore interface and MemoryStore
├── session_security.go      # Token generation and validation
├── session_test.go          # Session unit tests
├── session_integration_test.go # Integration tests
├── realtime_renderer.go     # Modified with session support
└── docs/
    └── SESSION_DESIGN.md    # Complete design document
```

## Compatibility Strategy

### Backward Compatibility

```go
// Keep existing APIs working (deprecated but functional)
func (r *Renderer) SetInitialData(data interface{}) (string, error) {
    // Implementation using default session internally
}

func (r *Renderer) SendUpdate(newData interface{}) {
    // Implementation using default session internally
}

func (r *Renderer) GetUpdateChannel() <-chan Update {
    // Implementation using default session internally
}
```

### Migration Path

1. **v2.0**: Introduce session APIs alongside existing APIs
2. **v2.1**: Mark old APIs as deprecated
3. **v3.0**: Remove old APIs entirely

## Performance Targets

- **10,000+ concurrent sessions** per instance
- **<1ms session creation** time
- **<10ms fragment update** latency
- **<1MB memory** per active session

## Success Criteria

- [ ] Zero cross-session data leakage in security tests
- [ ] All existing tests continue to pass
- [ ] New session APIs work as designed
- [ ] Performance targets met under load
- [ ] Complete backward compatibility during transition

This guide provides the essential information for implementing the session-based architecture while maintaining focus on the critical components and their interactions.
