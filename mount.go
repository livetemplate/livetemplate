package livetemplate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/internal/discovery"
	"github.com/livetemplate/livetemplate/internal/observe"
	"github.com/livetemplate/livetemplate/internal/session"
	"github.com/livetemplate/livetemplate/internal/upload"
	"github.com/livetemplate/livetemplate/internal/uploadtypes"
	"github.com/livetemplate/livetemplate/pubsub"
	"golang.org/x/time/rate"
)

// Session allows stores to trigger server-initiated actions for connected clients.
// Actions triggered via Session affect ALL connections for the current user (all tabs/devices).
//
// This is the recommended way to implement:
//   - Timers and ticks
//   - Background job completion notifications
//   - Webhook-triggered updates
//   - Cross-tab synchronization
//
// Security: Session is scoped to the current user only. There is no way to
// target other users, preventing unauthorized cross-user actions.
type Session interface {
	// TriggerAction dispatches the action to the matching store method,
	// then sends the updated template to ALL connections for this user.
	//
	// This behaves identically to client-initiated actions - the action is
	// dispatched to the matching method, errors are captured, and updates
	// are broadcast to all of the user's connections (tabs/devices).
	//
	// Example:
	//   session.TriggerAction("tick", nil)
	//   session.TriggerAction("new_notification", map[string]interface{}{"id": 123})
	TriggerAction(action string, data map[string]interface{}) error
}

// SessionAware is implemented by stores that need server-initiated actions.
// When a WebSocket connection is established, OnConnect is called with a Session
// handle that can be used to trigger actions from background goroutines.
//
// Example usage:
//
//	type TimerStore struct {
//	    Seconds int
//	    session livetemplate.Session
//	}
//
//	func (s *TimerStore) OnConnect(ctx context.Context, session livetemplate.Session) error {
//	    s.session = session
//	    go s.runTimer(ctx)
//	    return nil
//	}
//
//	func (s *TimerStore) runTimer(ctx context.Context) {
//	    ticker := time.NewTicker(time.Second)
//	    defer ticker.Stop()
//	    for {
//	        select {
//	        case <-ctx.Done():
//	            return
//	        case <-ticker.C:
//	            s.session.TriggerAction("tick", nil)
//	        }
//	    }
//	}
//
//	func (s *TimerStore) OnDisconnect() {
//	    // Cleanup if needed
//	}
//
//	func (s *TimerStore) Change(ctx *livetemplate.ActionContext) error {
//	    if ctx.Action == "tick" {
//	        s.Seconds++
//	    }
//	    return nil
//	}
type SessionAware interface {
	OnConnect(ctx context.Context, session Session) error
	OnDisconnect()
}

// Deprecated: Broadcaster is deprecated. Use Session instead.
// Broadcaster allows stores to push updates to connected clients without user interaction.
type Broadcaster interface {
	Send() error // Re-renders template and sends update to this connection
}

// Deprecated: BroadcastAware is deprecated. Use SessionAware instead.
// BroadcastAware is implemented by stores that need server-initiated updates.
type BroadcastAware interface {
	OnConnect(ctx context.Context, b Broadcaster) error
	OnDisconnect()
}

// broadcaster implements the Broadcaster interface for a single WebSocket connection
type broadcaster struct {
	conn     *session.Connection
	template *Template
	state    *connState
	handler  *liveHandler
	mu       sync.Mutex
}

func (b *broadcaster) Send() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Generate tree update
	var buf bytes.Buffer
	err := b.template.ExecuteUpdates(&buf, b.handler.getTemplateData(b.state.stores), b.state.getErrors())
	if err != nil {
		return fmt.Errorf("template update failed: %w", err)
	}

	// Parse tree from buffer
	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		return fmt.Errorf("failed to parse tree: %w", err)
	}

	// Wrap with metadata
	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success: len(b.state.getErrors()) == 0,
			Errors:  b.state.getErrors(),
		},
	}

	// Encode and send
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	return writeUpdateWebSocket(b.conn, responseBytes)
}

// liveSession implements the Session interface for server-initiated actions.
// It is scoped to a single user and triggers actions on ALL their connections.
type liveSession struct {
	userID         string                      // User ID for targeting all user's connections
	handler        *liveHandler                // Handler for registry access and action handling
	uploadRegistry uploadRegistry              // Upload registry for action context
	mu             sync.Mutex                  // Mutex for thread-safe operations
}

// TriggerAction dispatches the action to the matching store method,
// then sends updates to all those connections.
//
// In distributed deployments with PubSub configured, this also publishes the
// action to Redis so that other server instances can trigger the action on
// their local connections for this user.
func (s *liveSession) TriggerAction(action string, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Publish to Redis for distributed instances (if configured)
	if s.handler.config.PubSubBroadcaster != nil {
		if err := s.handler.config.PubSubBroadcaster.PublishServerAction(s.userID, action, data); err != nil {
			slog.Error("Failed to publish server action to pubsub",
				slog.String("user_id", s.userID),
				slog.String("action", action),
				slog.String("error", err.Error()))
			// Don't return early - still try local execution
		}
	}

	// Process action locally
	return s.triggerActionLocal(action, data)
}

// triggerActionLocal executes the action on local connections only.
// This is called both from TriggerAction and from the PubSub handler.
func (s *liveSession) triggerActionLocal(action string, data map[string]interface{}) error {
	// Get all connections for this user
	connections := s.handler.registry.GetByUser(s.userID)
	if len(connections) == 0 {
		slog.Debug("No connections for user in TriggerAction",
			slog.String("user_id", s.userID),
			slog.String("action", action))
		return nil
	}

	slog.Debug("TriggerAction for user",
		slog.String("user_id", s.userID),
		slog.String("action", action),
		slog.Int("connection_count", len(connections)))

	// Create message for action routing
	msg := message{
		Action: action,
		Data:   data,
	}

	// Process action for each connection
	var errCount int
	for _, conn := range connections {
		// Get connection state from stores
		stores, ok := conn.Stores.(Stores)
		if !ok {
			slog.Warn("Invalid stores type in connection",
				slog.String("user_id", s.userID))
			errCount++
			continue
		}

		// Create connection state for this action
		state := &connState{
			stores:  stores,
			errors:  make(map[string]string),
			groupID: conn.GroupID,
		}

		// Get upload registry from connection if available
		var uploadReg uploadRegistry
		if conn.Uploads != nil {
			uploadReg, _ = conn.Uploads.(uploadRegistry)
		}

		// Create context with timeout for server-initiated actions
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Handle action (routes to store's Change method)
		// nil for w, r since this is a server-initiated action (not HTTP request)
		modifiedStore, err := s.handler.handleAction(ctx, msg, state, uploadReg, nil, nil)
		if err != nil {
			slog.Warn("TriggerAction handleAction failed",
				slog.String("user_id", s.userID),
				slog.String("action", action),
				slog.String("error", err.Error()))
			cancel()
			errCount++
			continue
		}

		// Persist state after successful action
		// This is critical for SessionStores that return copies (e.g., RedisSessionStore)
		// Uses targeted persistence if SessionStore supports SingleStoreSetter
		s.handler.persistStore(ctx, conn.GroupID, modifiedStore, state.stores)

		cancel()

		// Send update to this connection
		if err := s.handler.sendUpdate(conn, s.handler.getTemplateData(state.stores)); err != nil {
			slog.Warn("TriggerAction sendUpdate failed",
				slog.String("user_id", s.userID),
				slog.String("action", action),
				slog.String("error", err.Error()))
			errCount++
			continue
		}
	}

	if errCount > 0 {
		return fmt.Errorf("TriggerAction failed for %d/%d connections", errCount, len(connections))
	}

	return nil
}

// LiveHandler is the interface returned by Template.Handle()
// It provides HTTP handling and lifecycle management for live template connections.
//
// For server-initiated actions, use the Session interface provided to stores
// via SessionAware.OnConnect(). Session.TriggerAction() is the recommended way
// to push updates from the server.
type LiveHandler interface {
	http.Handler

	// Shutdown gracefully shuts down the handler, draining connections.
	//
	// It performs the following steps:
	//  1. Stops accepting new WebSocket connections
	//  2. Sends close frames to all active WebSocket connections
	//  3. Waits for in-flight requests to complete (respecting context timeout)
	//
	// The context timeout controls how long to wait for connections to close.
	// After the timeout, remaining connections are forcefully closed.
	//
	// Example usage with http.Server:
	//   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	//   defer cancel()
	//   handler.Shutdown(ctx)
	//   server.Shutdown(ctx)
	Shutdown(ctx context.Context) error

	// MetricsHandler returns an http.Handler that exports Prometheus metrics.
	//
	// The handler responds to GET requests with metrics in Prometheus text format.
	// Typically mounted at /metrics for scraping by Prometheus.
	//
	// Example with standard library http mux:
	//   mux := http.NewServeMux()
	//   handler := template.Handle(store)
	//   mux.Handle("/live", handler)
	//   mux.Handle("/metrics", handler.MetricsHandler())
	//   http.ListenAndServe(":8080", mux)
	//
	// Example with gorilla/mux:
	//   r := mux.NewRouter()
	//   handler := template.Handle(store)
	//   r.Handle("/live", handler)
	//   r.Handle("/metrics", handler.MetricsHandler())
	MetricsHandler() http.Handler
}

// mountConfig configures the mount handler (internal only)
type mountConfig struct {
	Template      *Template
	Stores        Stores
	IsSingleStore bool
	// Controller+State pattern (new API)
	Controller         interface{} // Singleton controller with dependencies
	State              State       // Initial state template (cloned per session)
	UseControllerState bool        // Flag: true = use new pattern, false = use old Stores
	Upgrader               *websocket.Upgrader
	SessionStore           SessionStore
	Authenticator          Authenticator
	PubSubBroadcaster      pubsub.Broadcaster // Optional: for distributed broadcasting across instances
	AllowedOrigins         []string
	WebSocketDisabled      bool
	MaxConnections         int64                       // Maximum total connections (0 = unlimited)
	MaxConnectionsPerGroup int64                       // Maximum connections per group (0 = unlimited)
	CookieMaxAge           time.Duration               // Session cookie max age (default: 1 year)
	UploadConfigs          map[string]uploadtypes.UploadConfig // Upload field configurations
	wsBufferSize           int                                 // WebSocket send buffer size per connection (default: 50)
}

// mountOption is a functional option for configuring handlers (internal only)
type mountOption func(*mountConfig)

// liveHandler handles both WebSocket and HTTP requests
type liveHandler struct {
	config          mountConfig
	registry        *session.ConnectionRegistry
	limits          *session.ConnectionLimits
	metricsExporter *observe.PrometheusExporter
	tempFileManager uploadTempFileManager

	// Graceful shutdown state
	shutdownOnce sync.Once
	shutdownChan chan struct{}
	shutdownWg   sync.WaitGroup
	isShutdown   atomic.Bool
}

type connState struct {
	stores   Stores            // Each connection gets cloned stores
	errors   map[string]string // Field errors from last action
	errorsMu sync.RWMutex      // Mutex for thread-safe error access
	groupID  string            // Session/group ID for this connection
}

func (c *connState) setError(field, message string) {
	c.errorsMu.Lock()
	defer c.errorsMu.Unlock()
	c.errors[field] = message
}

func (c *connState) clearErrors() {
	c.errorsMu.Lock()
	defer c.errorsMu.Unlock()
	c.errors = make(map[string]string)
}

func (c *connState) getErrors() map[string]string {
	c.errorsMu.RLock()
	defer c.errorsMu.RUnlock()

	// Return copy to avoid race conditions
	result := make(map[string]string, len(c.errors))
	for k, v := range c.errors {
		result[k] = v
	}
	return result
}

func (h *liveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add header to indicate WebSocket availability
	if h.config.WebSocketDisabled {
		w.Header().Set("X-LiveTemplate-WebSocket", "disabled")
	} else {
		w.Header().Set("X-LiveTemplate-WebSocket", "enabled")
	}

	if websocket.IsWebSocketUpgrade(r) {
		if h.config.WebSocketDisabled {
			http.Error(w, "WebSocket is disabled on this endpoint", http.StatusBadRequest)
			return
		}
		h.handleWebSocket(w, r)
	} else {
		h.handleHTTP(w, r)
	}
}

func (h *liveHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if shutting down - reject new connections
	if h.isShutdown.Load() {
		http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
		return
	}

	// Track this connection goroutine for graceful shutdown
	h.shutdownWg.Add(1)
	defer h.shutdownWg.Done()

	// Authenticate user and get session group
	userID, err := h.config.Authenticator.Identify(r)
	if err != nil {
		log.Printf("Authentication failed: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groupID, err := h.config.Authenticator.GetSessionGroup(r, userID)
	if err != nil {
		log.Printf("Failed to get session group: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check connection limits before upgrading
	if !h.limits.CanAccept(groupID) {
		stats := h.limits.Stats()
		log.Printf("Connection rejected (at capacity): active=%d, max=%d, group=%s, groupCount=%d, maxPerGroup=%d",
			stats.ActiveConnections, stats.MaxConnections, groupID, h.limits.GroupConnectionCount(groupID), stats.MaxPerGroup)
		http.Error(w, "Service at capacity, please try again later", http.StatusServiceUnavailable)
		return
	}

	// Set session cookie if this is a new session (cookie doesn't exist)
	setCookieIfNew(w, r, groupID, h.config.CookieMaxAge)

	// Upgrade to WebSocket after authentication and limit check succeeds
	conn, err := h.config.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Acquire connection slot (increment counters)
	if err := h.limits.Acquire(groupID); err != nil {
		log.Printf("Failed to acquire connection slot: %v", err)
		if writeErr := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseServiceRestart, "Service at capacity")); writeErr != nil {
			log.Printf("Failed to send close message: %v", writeErr)
		}
		return
	}
	defer h.limits.Release(groupID)

	log.Printf("Client connected: user=%q, group=%q, addr=%s, active=%d", userID, groupID, conn.RemoteAddr(), h.limits.ActiveConnections())

	// Clone template for this connection to avoid state conflicts
	// Each WebSocket connection needs its own template instance because
	// ExecuteUpdates() tracks state (lastTree, lastData, etc.)
	connTmpl, err := h.config.Template.Clone()
	if err != nil {
		log.Printf("Failed to clone template: %v", err)
		return
	}

	// Get or create stores for this session group
	ctx := r.Context()
	stores := h.config.SessionStore.Get(ctx, groupID)
	if stores == nil {
		stores = h.cloneStores()
		h.config.SessionStore.Set(ctx, groupID, stores)
		log.Printf("Created new session group: %s", groupID)
	} else {
		// Hydrate any StateData wrappers into proper controller instances
		stores = h.hydrateStores(stores)
	}

	// Initialize upload registry for this connection (created by template initialization)
	uploadRegistry := h.newUploadRegistry()

	// Configure uploads from handler config
	for name, config := range h.config.UploadConfigs {
		if err := uploadRegistry.CreateUpload(name, config); err != nil {
			log.Printf("Failed to create upload %q: %v", name, err)
		}
	}

	// Set upload registry on template for .lvt.Uploads() support
	connTmpl.SetUploadRegistry(uploadRegistry)

	// Create Connection and register in registry
	connection := &session.Connection{
		Conn:     conn,
		GroupID:  groupID,
		UserID:   userID,
		Template: connTmpl,
		Stores:   stores,
		Uploads:  uploadRegistry,
	}

	h.registry.Register(connection, h.config.wsBufferSize)
	defer h.registry.Unregister(connection)
	defer func() {
		// Clean up temp files for this session on disconnect
		if err := h.tempFileManager.RemoveSession(groupID); err != nil {
			log.Printf("Failed to clean up temp files for session %s: %v", groupID, err)
		}
	}()
	log.Printf("Registered connection (total: %d, groups: %d)", h.registry.Count(), h.registry.GroupCount())

	// Create connection state (errors are per-connection, not shared)
	state := &connState{
		stores:  stores,
		errors:  make(map[string]string),
		groupID: groupID,
	}

	// Create context for session/broadcaster lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create session for server-initiated actions (new API)
	sess := &liveSession{
		userID:         userID,
		handler:        h,
		uploadRegistry: uploadRegistry,
	}

	// Create broadcaster for server-initiated updates (deprecated API)
	bc := &broadcaster{
		conn:     connection,
		template: connTmpl,
		state:    state,
		handler:  h,
	}

	// Call OnConnect for stores that implement SessionAware (new) or BroadcastAware (deprecated)
	for _, store := range state.stores {
		// Try SessionAware first (preferred)
		if aware, ok := store.(SessionAware); ok {
			if err := aware.OnConnect(ctx, sess); err != nil {
				log.Printf("SessionAware.OnConnect failed for store: %v", err)
			}
			// Schedule OnDisconnect call when WebSocket closes
			defer aware.OnDisconnect()
		} else if aware, ok := store.(BroadcastAware); ok {
			// Fall back to deprecated BroadcastAware
			if err := aware.OnConnect(ctx, bc); err != nil {
				log.Printf("BroadcastAware.OnConnect failed for store: %v", err)
			}
			// Schedule OnDisconnect call when WebSocket closes
			defer aware.OnDisconnect()
		}
	}

	// Send initial tree
	var buf bytes.Buffer

	err = connTmpl.ExecuteUpdates(&buf, h.getTemplateData(state.stores), state.getErrors())
	if err != nil {
		log.Printf("Failed to generate initial tree: %v", err)
		return
	}

	// Parse tree from buffer
	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		log.Printf("Failed to parse initial tree: %v", err)
		return
	}

	// Wrap with metadata (initial load has no action)
	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success: len(state.getErrors()) == 0,
			Errors:  state.getErrors(),
		},
	}

	// Encode and send wrapped response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal initial response: %v", err)
		return
	}

	err = writeUpdateWebSocket(connection, responseBytes)
	if err != nil {
		log.Printf("Failed to send initial tree: %v", err)
		return
	}

	// Create rate limiter for this connection (prevents DoS attacks)
	var limiter *rate.Limiter
	if h.config.Template.config.MessageRateLimit > 0 {
		burst := h.config.Template.config.MessageRateBurst
		if burst < 1 {
			burst = 1 // Minimum burst size for rate limiter to function
		}
		limiter = rate.NewLimiter(rate.Limit(h.config.Template.config.MessageRateLimit), burst)
	}

	// message loop
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Rate limiting check (per connection)
		if limiter != nil && !limiter.Allow() {
			// Rate limit exceeded - send error to client
			errorResp := UpdateResponse{
				Tree: nil,
				Meta: &ResponseMetadata{
					Success: false,
					Errors: map[string]string{
						"_rate_limit": "Too many requests. Please slow down.",
					},
				},
			}
			if respBytes, err := json.Marshal(errorResp); err == nil {
				_ = writeUpdateWebSocket(connection, respBytes) // Best effort
			}
			continue // Skip processing this message
		}

		// Parse message
		msg, err := parseActionFromWebSocket(data)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Check if this is an upload-related action
		uploadHandled, err := h.handleUploadAction(r.Context(), conn, data, msg, state, uploadRegistry, connection)
		if err != nil {
			log.Printf("Upload action error: %v", err)
			continue
		}
		if uploadHandled {
			// Upload action was handled, skip normal action processing
			continue
		}

		// Handle action with request context for timeout/cancellation/values
		// WebSocket actions pass nil for w/r - HTTP methods will return ErrNoHTTPContext
		modifiedStore, err := h.handleAction(r.Context(), msg, state, uploadRegistry, nil, nil)
		if err != nil {
			log.Printf("Action error: %v", err)
			continue
		}

		// Persist state after successful action
		// This is critical for SessionStores that return copies (e.g., RedisSessionStore)
		// Uses targeted persistence if SessionStore supports SingleStoreSetter
		h.persistStore(r.Context(), groupID, modifiedStore, state.stores)

		// Auto-broadcast to other connections in same session group
		// This ensures all tabs in the same browser session stay in sync
		h.autoBroadcastToGroup(groupID, h.getTemplateData(state.stores), connection)

		// Generate tree update
		buf.Reset()
		err = connTmpl.ExecuteUpdates(&buf, h.getTemplateData(state.stores), state.getErrors())
		if err != nil {
			log.Printf("Template update execution failed: %v", err)
			continue
		}

		// Parse tree from buffer
		var tree map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
			log.Printf("Failed to parse tree: %v", err)
			continue
		}

		// Wrap with metadata
		response := UpdateResponse{
			Tree: tree,
			Meta: &ResponseMetadata{
				Success: len(state.getErrors()) == 0,
				Errors:  state.getErrors(),
				Action:  msg.Action,
			},
		}

		// Encode and send wrapped response
		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			continue
		}

		err = writeUpdateWebSocket(connection, responseBytes)
		if err != nil {
			log.Printf("WebSocket write failed: %v", err)
			break
		}
	}

	log.Printf("Client disconnected: user=%q, group=%q (remaining: %d)", userID, groupID, h.registry.Count())
}

// setCookieIfNew sets the livetemplate-id cookie if it doesn't already exist
func setCookieIfNew(w http.ResponseWriter, r *http.Request, groupID string, cookieMaxAge time.Duration) {
	// Check if cookie already exists
	if cookie, err := r.Cookie("livetemplate-id"); err == nil && cookie.Value == groupID {
		// Cookie exists and matches - no need to set again
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "livetemplate-id",
		Value:    groupID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(cookieMaxAge.Seconds()),
	})
}

func (h *liveHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle HEAD request for capability check
	if r.Method == http.MethodHead {
		// Just return headers, no body
		return
	}

	// Authenticate user and get session group
	userID, err := h.config.Authenticator.Identify(r)
	if err != nil {
		log.Printf("HTTP authentication failed: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groupID, err := h.config.Authenticator.GetSessionGroup(r, userID)
	if err != nil {
		log.Printf("Failed to get session group for HTTP: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie if this is a new session (cookie doesn't exist)
	setCookieIfNew(w, r, groupID, h.config.CookieMaxAge)

	// Get or create stores for this session group
	ctx := r.Context()
	stores := h.config.SessionStore.Get(ctx, groupID)
	if stores == nil {
		stores = h.cloneStores()
		h.config.SessionStore.Set(ctx, groupID, stores)
		log.Printf("HTTP: Created new session group: %s", groupID)
	} else {
		// Hydrate any StateData wrappers into proper controller instances
		stores = h.hydrateStores(stores)
	}

	// Create connection state (errors are per-request, not persisted)
	state := &connState{
		stores:  stores,
		errors:  make(map[string]string),
		groupID: groupID,
	}

	// Handle GET request for initial HTML page
	if r.Method == http.MethodGet {
		// Always reload data from database for GET requests to ensure fresh data
		// This prevents stale session state when WebSocket actions modify data
		for name, store := range state.stores {
			if initializer, ok := store.(StoreInitializer); ok {
				if err := initializer.Init(); err != nil {
					slog.Error("Store initialization failed for GET request",
						slog.String("store", name),
						slog.String("group_id", groupID),
						slog.String("user_id", userID),
						slog.String("error", err.Error()))
					http.Error(w, "Failed to initialize application state", http.StatusInternalServerError)
					return
				}
			}
		}

		err := h.config.Template.Execute(w, h.getTemplateData(state.stores), state.getErrors())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle POST request for actions
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Initialize upload registry for HTTP requests (needed for multipart uploads)
	uploadRegistry := h.newUploadRegistry()

	// Configure uploads from handler config
	for name, config := range h.config.UploadConfigs {
		if err := uploadRegistry.CreateUpload(name, config); err != nil {
			log.Printf("HTTP: Failed to create upload %q: %v", name, err)
		}
	}

	// Parse message
	msg, err := parseActionFromHTTP(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle action with request context for timeout/cancellation/values
	// HTTP POST actions pass w, r so Change() can set cookies and redirect
	modifiedStore, err := h.handleAction(r.Context(), msg, state, uploadRegistry, w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Persist state after successful action
	// This is critical for SessionStores that return copies (e.g., RedisSessionStore)
	// Uses targeted persistence if SessionStore supports SingleStoreSetter
	h.persistStore(r.Context(), groupID, modifiedStore, state.stores)

	// Check if request contains multipart form data (file uploads)
	// Process uploads after action execution to allow ConsumeUpload to work
	if err := h.handleMultipartUploads(r, groupID, uploadRegistry, state.stores); err != nil {
		log.Printf("HTTP: Upload processing failed: %v", err)
		// Don't fail the request - upload errors are shown in template
	}

	// Set upload registry on template for HTTP response (template helpers need it)
	// Clone template to avoid race conditions with concurrent requests
	httpTmpl, err := h.config.Template.Clone()
	if err != nil {
		http.Error(w, "Failed to clone template", http.StatusInternalServerError)
		return
	}
	httpTmpl.SetUploadRegistry(uploadRegistry)

	// Auto-broadcast to all WebSocket connections in same session group
	// This ensures all tabs in the same browser session stay in sync
	// (HTTP request doesn't have a WebSocket connection to exclude)
	h.autoBroadcastToGroup(groupID, h.getTemplateData(state.stores), nil)

	// Generate tree update
	var buf bytes.Buffer
	err = httpTmpl.ExecuteUpdates(&buf, h.getTemplateData(state.stores), state.getErrors())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse tree from buffer
	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wrap with metadata
	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success: len(state.getErrors()) == 0,
			Errors:  state.getErrors(),
			Action:  msg.Action,
		},
	}

	// Send wrapped response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAction routes the action to the correct store and captures errors.
// For HTTP POST actions, w and r are provided so that ActionContext can set cookies/redirect.
// For WebSocket actions, w and r should be nil - HTTP methods will return ErrNoHTTPContext.
//
// Returns the name of the store that was modified (for targeted persistence).
// In single-store mode, returns empty string (the store is at key "").
func (h *liveHandler) handleAction(ctx context.Context, msg message, state *connState, uploadRegistry uploadRegistry, w http.ResponseWriter, r *http.Request) (modifiedStore string, err error) {
	// Clear previous errors
	state.clearErrors()

	// Parse action to extract store name
	storeName, action := parseAction(msg.Action)

	var store interface{}
	if h.config.IsSingleStore {
		// Single store mode
		if storeName != "" {
			return "", fmt.Errorf(
				"unexpected store prefix '%s' in single-store mode\n"+
					"Use action '%s' instead of '%s'",
				storeName, action, msg.Action)
		}

		// Get the single store (key is "" in single-store mode)
		store = state.stores[""]
		storeName = "" // Explicit for clarity

	} else {
		// Multi-store mode
		if storeName == "" {
			return "", fmt.Errorf(
				"action '%s' missing store prefix in multi-store mode\n"+
					"Available stores: %v\n"+
					"Use format: 'storeName.action' (e.g., 'counter.increment')",
				msg.Action, h.getStoreNames())
		}

		// Find store using case-insensitive matching
		store = h.findStore(state.stores, storeName)
		if store == nil {
			return "", fmt.Errorf(
				"unknown store: '%s' in action '%s'\n"+
					"Available stores: %v",
				storeName, msg.Action, h.getStoreNames())
		}
	}

	// Create upload accessor if upload registry is available
	var uploadAccessor uploadAccessor
	if uploadRegistry != nil {
		if reg, ok := uploadRegistry.(*upload.Registry); ok {
			uploadAccessor = upload.NewAccessor(reg)
		}
	}

	// Create action context with request context for timeout/cancellation/values
	// w and r are nil for WebSocket actions, non-nil for HTTP POST actions
	actionCtx := &ActionContext{
		Action:  action,
		Data:    newActionData(msg.Data),
		Ctx:     ctx,
		uploads: uploadAccessor,
		w:       w,
		r:       r,
	}

	// Route action to store method matching the action name
	// e.g., action "increment" → method Increment(ctx *ActionContext) error
	actionErr := Dispatch(store, actionCtx)

	if actionErr != nil {
		// Process the error
		switch e := actionErr.(type) {
		case FieldError:
			state.setError(e.Field, e.Message)
		case MultiError:
			for _, fieldErr := range e {
				state.setError(fieldErr.Field, fieldErr.Message)
			}
		default:
			state.setError("_general", actionErr.Error())
		}
	}

	// Return the store name that was modified (for targeted persistence)
	return storeName, nil
}

// findStore finds a store by name using case-insensitive matching
func (h *liveHandler) findStore(stores Stores, name string) interface{} {
	normalized := discovery.NormalizeStoreName(name)

	for storeName, store := range stores {
		if discovery.NormalizeStoreName(storeName) == normalized {
			return store
		}
	}

	return nil
}

// getTemplateData returns the data structure for template rendering
func (h *liveHandler) getTemplateData(stores Stores) interface{} {
	if h.config.IsSingleStore {
		// Return store directly for single store
		return stores[""]
	}

	// Return map of stores for multi-store
	data := make(map[string]interface{})
	for name, store := range stores {
		data[name] = store
	}
	return data
}

// newUploadRegistry creates a new upload registry instance.
// This is called for each connection to create isolated upload state.
func (h *liveHandler) newUploadRegistry() uploadRegistry {
	return h.config.Template.newUploadRegistry()
}

// cloneStores creates new instances of all stores
func (h *liveHandler) cloneStores() Stores {
	cloned := make(Stores)
	for name, store := range h.config.Stores {
		cloned[name] = cloneStore(store)
	}
	return cloned
}

// hydrateStores processes stores loaded from SessionStore, replacing any StateData
// wrappers with properly hydrated controller instances.
//
// When a store uses `lvt:"state"` tags, RedisSessionStore serializes only those fields
// and returns a *StateData wrapper on Get(). This function:
// 1. Detects StateData wrappers in the returned stores
// 2. Clones the original template store (which has dependencies like DB, Logger)
// 3. Deserializes the state from StateData.Raw
// 4. Injects the deserialized state into the cloned store
//
// This allows controllers to have non-serializable dependencies while still
// persisting their state across sessions.
func (h *liveHandler) hydrateStores(stores Stores) Stores {
	if stores == nil {
		return nil
	}

	for name, store := range stores {
		// Check if this is a StateData wrapper
		if sd := GetStateData(store); sd != nil {
			// Get the original template store (which has dependencies)
			templateStore := h.config.Stores[name]
			if templateStore == nil {
				slog.Warn("Cannot hydrate state: template store not found, using fresh clone",
					slog.String("store", name))
				// Fallback: use a fresh clone from template stores (user sees initial state)
				if fallback := h.config.Stores[name]; fallback != nil {
					stores[name] = cloneStore(fallback)
				}
				continue
			}

			// Clone the template store to get fresh instance with dependencies
			clonedStore := cloneStore(templateStore)

			// Deserialize state from the StateData wrapper
			stateMap, err := DeserializeState(sd.Raw, clonedStore)
			if err != nil {
				slog.Warn("Failed to deserialize state, using fresh clone",
					slog.String("store", name),
					slog.String("error", err.Error()))
				// Fallback: use the cloned store with initial state (better than crash)
				stores[name] = clonedStore
				continue
			}

			// Inject deserialized state into the cloned store
			if err := InjectState(clonedStore, stateMap); err != nil {
				slog.Warn("Failed to inject state, using fresh clone",
					slog.String("store", name),
					slog.String("error", err.Error()))
				// Fallback: use the cloned store with initial state
				stores[name] = clonedStore
				continue
			}

			// Replace the StateData wrapper with the hydrated store
			stores[name] = clonedStore

			slog.Debug("Hydrated store from state",
				slog.String("store", name))
		}
	}

	return stores
}

// cloneStore creates a new instance of a store.
// Works with any type - stores no longer need to implement the Store interface.
func cloneStore(store interface{}) interface{} {
	storeType := reflect.TypeOf(store)
	if storeType.Kind() == reflect.Ptr {
		storeType = storeType.Elem()
	}

	// Create new instance
	newStore := reflect.New(storeType).Interface()

	// Copy field values
	copyStruct(newStore, store)

	// Call Init() if the store implements StoreInitializer
	if initializer, ok := newStore.(StoreInitializer); ok {
		if err := initializer.Init(); err != nil {
			// Log the error but don't fail - store is in a partially initialized state
			// This will surface as an error when the store is first used (e.g., in GET handler)
			slog.Error("Store initialization failed during cloning",
				slog.String("store_type", fmt.Sprintf("%T", store)),
				slog.String("error", err.Error()))
		}
	}

	return newStore
}

// copyStruct copies field values from src to dst.
//
// IMPORTANT: Only exported (public) fields are copied. Unexported fields
// are silently skipped because they cannot be accessed via reflection.
//
// Stores should not rely on unexported fields for critical state, or should
// implement custom cloning logic by implementing a Clone() method.
func copyStruct(dst, src interface{}) {
	srcVal := reflect.ValueOf(src)
	dstVal := reflect.ValueOf(dst)

	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}
	if dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		dstField := dstVal.Field(i)

		if dstField.CanSet() {
			dstField.Set(srcField)
		}
	}
}

// getStoreNames returns the names of all stores
func (h *liveHandler) getStoreNames() []string {
	names := make([]string, 0, len(h.config.Stores))
	for name := range h.config.Stores {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// persistStore persists stores after a successful action.
// If the SessionStore implements SingleStoreSetter and a specific store was modified,
// only that store is persisted. Otherwise, all stores are persisted.
//
// This optimization is especially beneficial for Redis where network roundtrips
// are expensive and serializing only the modified store reduces payload size.
func (h *liveHandler) persistStore(ctx context.Context, groupID string, storeName string, stores Stores) {
	// Try targeted persistence if SessionStore supports it
	if setter, ok := h.config.SessionStore.(SingleStoreSetter); ok {
		// Use SetStore for targeted persistence
		setter.SetStore(ctx, groupID, storeName, stores[storeName])
		return
	}

	// Fall back to full persistence
	h.config.SessionStore.Set(ctx, groupID, stores)
}

// autoBroadcastToGroup broadcasts template updates to all connections in a group.
// Optionally excludes a specific connection (for WebSocket sender).
// Runs asynchronously to avoid blocking the caller.
//
// Note: Under high load, this may launch many concurrent goroutines.
// Each goroutine is relatively short-lived and uses per-connection template clones,
// so this is safe but could cause temporary resource spikes.
func (h *liveHandler) autoBroadcastToGroup(groupID string, data interface{}, excludeConn *session.Connection) {
	go func() {
		var conns []*session.Connection
		if excludeConn != nil {
			// WebSocket: exclude the sender
			conns = h.registry.GetByGroupExcept(groupID, excludeConn)
		} else {
			// HTTP: broadcast to all connections (no sender to exclude)
			conns = h.registry.GetByGroup(groupID)
		}

		if len(conns) == 0 {
			slog.Debug("No connections for auto-broadcast",
				slog.String("group_id", groupID))
			return
		}

		slog.Debug("Auto-broadcasting to group",
			slog.String("group_id", groupID),
			slog.Int("connection_count", len(conns)))

		var errCount int
		for _, conn := range conns {
			if err := h.sendUpdate(conn, data); err != nil {
				slog.Warn("Auto-broadcast send failed",
					slog.String("group_id", groupID),
					slog.String("user_id", conn.UserID),
					slog.String("error", err.Error()))
				errCount++
			}
		}

		if errCount > 0 {
			slog.Warn("Auto-broadcast completed with errors",
				slog.String("group_id", groupID),
				slog.Int("error_count", errCount),
				slog.Int("total_connections", len(conns)))
		}
	}()
}

// sendUpdate generates and sends a template update to a single connection
func (h *liveHandler) sendUpdate(conn *session.Connection, data interface{}) error {
	// Use the connection's cloned template for independent tree diffing
	var buf bytes.Buffer

	// Type assert Template from interface{} to *Template
	tmpl, ok := conn.Template.(*Template)
	if !ok {
		return fmt.Errorf("invalid template type in connection")
	}

	// Generate update using the connection's template
	// We pass the data directly - no errors to report for broadcasts
	err := tmpl.ExecuteUpdates(&buf, data, nil)
	if err != nil {
		return fmt.Errorf("template update failed: %w", err)
	}

	// Parse tree from buffer
	var tree map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		return fmt.Errorf("failed to parse tree: %w", err)
	}

	// Wrap with metadata
	response := UpdateResponse{
		Tree: tree,
		Meta: &ResponseMetadata{
			Success: true,
			Errors:  nil,
		},
	}

	// Encode response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Send using the connection's Send method (thread-safe)
	return conn.Send(websocket.TextMessage, responseBytes)
}

// handlePubSubMessage handles incoming pub/sub broadcast messages from other instances.
//
// This is called by the RedisBroadcaster subscriber when a message is received.
// It deserializes the payload and fans it out to relevant local connections.
func (h *liveHandler) handlePubSubMessage(msg *pubsub.BroadcastMessage) error {
	// Deserialize the payload
	var data interface{}
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Fan out based on message scope
	switch msg.Scope {
	case pubsub.ScopeGlobal:
		// Broadcast to all local connections
		connections := h.registry.GetAll()
		for _, conn := range connections {
			if err := h.sendUpdate(conn, data); err != nil {
				log.Printf("PubSub: Failed to send global broadcast to connection: %v", err)
			}
		}
		log.Printf("PubSub: Fanned out global broadcast to %d local connection(s)", len(connections))

	case pubsub.ScopeGroup:
		// Broadcast to all connections in the group
		connections := h.registry.GetByGroup(msg.GroupID)
		for _, conn := range connections {
			if err := h.sendUpdate(conn, data); err != nil {
				log.Printf("PubSub: Failed to send group broadcast to connection: %v", err)
			}
		}
		log.Printf("PubSub: Fanned out group broadcast to %d local connection(s) for group %s", len(connections), msg.GroupID)

	case pubsub.ScopeUser:
		// Broadcast to all connections for the user
		connections := h.registry.GetByUser(msg.UserID)
		for _, conn := range connections {
			if err := h.sendUpdate(conn, data); err != nil {
				log.Printf("PubSub: Failed to send user broadcast to connection: %v", err)
			}
		}
		log.Printf("PubSub: Fanned out user broadcast to %d local connection(s) for user %s", len(connections), msg.UserID)

	default:
		return fmt.Errorf("unknown broadcast scope: %s", msg.Scope)
	}

	return nil
}

// handleServerActionMessage handles incoming server action messages from other instances.
//
// This is called by the RedisBroadcaster subscriber when a server action message is received.
// It dispatches to the matching store method and sends updates to local connections for the target user.
func (h *liveHandler) handleServerActionMessage(msg *pubsub.ServerActionMessage) error {
	// Get all connections for this user
	connections := h.registry.GetByUser(msg.UserID)
	if len(connections) == 0 {
		slog.Debug("PubSub: No local connections for server action",
			slog.String("user_id", msg.UserID),
			slog.String("action", msg.Action))
		return nil
	}

	slog.Debug("PubSub: Handling server action",
		slog.String("user_id", msg.UserID),
		slog.String("action", msg.Action),
		slog.Int("connection_count", len(connections)))

	// Create message for action routing
	actionMsg := message{
		Action: msg.Action,
		Data:   msg.Data,
	}

	// Process action for each connection
	var errCount int
	for _, conn := range connections {
		// Get connection state from stores
		stores, ok := conn.Stores.(Stores)
		if !ok {
			slog.Warn("PubSub: Invalid stores type in connection",
				slog.String("user_id", msg.UserID))
			errCount++
			continue
		}

		// Create connection state for this action
		state := &connState{
			stores:  stores,
			errors:  make(map[string]string),
			groupID: conn.GroupID,
		}

		// Get upload registry from connection if available
		var uploadReg uploadRegistry
		if conn.Uploads != nil {
			uploadReg, _ = conn.Uploads.(uploadRegistry)
		}

		// Create context with timeout for server-initiated actions
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Handle action (routes to store's Change method)
		// nil for w, r since this is a PubSub-initiated action (not HTTP request)
		if _, err := h.handleAction(ctx, actionMsg, state, uploadReg, nil, nil); err != nil {
			slog.Warn("PubSub: handleAction failed for server action",
				slog.String("user_id", msg.UserID),
				slog.String("action", msg.Action),
				slog.String("error", err.Error()))
			cancel()
			errCount++
			continue
		}
		cancel()

		// Send update to this connection
		if err := h.sendUpdate(conn, h.getTemplateData(state.stores)); err != nil {
			slog.Warn("PubSub: sendUpdate failed for server action",
				slog.String("user_id", msg.UserID),
				slog.String("action", msg.Action),
				slog.String("error", err.Error()))
			errCount++
			continue
		}
	}

	if errCount > 0 {
		slog.Warn("PubSub: Server action failed for some connections",
			slog.String("user_id", msg.UserID),
			slog.String("action", msg.Action),
			slog.Int("errors", errCount),
			slog.Int("total", len(connections)))
	} else {
		slog.Debug("PubSub: Server action completed successfully",
			slog.String("user_id", msg.UserID),
			slog.String("action", msg.Action),
			slog.Int("connection_count", len(connections)))
	}

	return nil
}

// Shutdown gracefully shuts down the LiveHandler.
//
// It stops accepting new WebSocket connections, sends close frames to all
// active connections, and waits for connections to finish (respecting ctx timeout).
//
// This method can be called multiple times safely (only first call has effect).
func (h *liveHandler) Shutdown(ctx context.Context) error {
	var shutdownErr error

	h.shutdownOnce.Do(func() {
		log.Printf("LiveHandler: Starting graceful shutdown...")

		// Mark as shutting down (stops accepting new connections)
		h.isShutdown.Store(true)
		close(h.shutdownChan)

		// Get all active connections
		connections := h.registry.GetAll()
		log.Printf("LiveHandler: Closing %d active connections...", len(connections))

		// Send close frames to all connections
		closeMessage := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server shutting down")
		for _, conn := range connections {
			// Send close frame (best effort, ignore errors)
			if conn.Conn != nil {
				_ = conn.Send(websocket.CloseMessage, closeMessage)
			}
		}

		// Wait for all connection goroutines to finish
		done := make(chan struct{})
		go func() {
			h.shutdownWg.Wait()
			close(done)
		}()

		// Wait for shutdown or timeout
		select {
		case <-done:
			log.Printf("LiveHandler: All connections closed gracefully")
		case <-ctx.Done():
			log.Printf("LiveHandler: Shutdown timeout reached, forcing close of remaining connections")
			shutdownErr = ctx.Err()

			// Force close remaining connections
			for _, conn := range h.registry.GetAll() {
				if conn.Conn != nil {
					conn.Conn.Close()
				}
			}
		}

		log.Printf("LiveHandler: Shutdown complete")
	})

	return shutdownErr
}

// MetricsHandler returns an HTTP handler that exports Prometheus metrics.
func (h *liveHandler) MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Set content type for Prometheus text format
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Write metrics
		if err := h.metricsExporter.WriteMetrics(w); err != nil {
			log.Printf("Error writing metrics: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	})
}

// handleMultipartUploads processes multipart form file uploads from HTTP requests.
// It detects configured uploads, parses the files, and calls ConsumeUpload on UploadAware stores.
func (h *liveHandler) handleMultipartUploads(r *http.Request, sessionID string, uploadRegistry uploadRegistry, stores Stores) error {
	// Check if this is multipart form data
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil // Not a multipart request, nothing to do
	}

	// Type assert to get the concrete Registry type for accessing uploads
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Get all configured uploads
	uploads := registry.GetAllUploads()
	if len(uploads) == 0 {
		return nil // No uploads configured
	}

	// Process each configured upload
	for uploadName, uploadObj := range uploads {
		// Parse multipart upload for this field
		entries, err := upload.ParseMultipartUpload(r, uploadName, uploadObj.Config, sessionID, h.tempFileManager.(*upload.TempFileManager))
		if err != nil {
			// Check if it's just "no files found" error (expected for optional uploads)
			if strings.Contains(err.Error(), "no files found") {
				continue // Skip this upload field
			}
			log.Printf("Failed to parse multipart upload %q: %v", uploadName, err)
			continue // Continue with other uploads
		}

		// Add entries to upload registry
		for _, entry := range entries {
			if err := uploadObj.AddEntry(entry); err != nil {
				log.Printf("Failed to add entry to upload %q: %v", uploadName, err)
			}
		}

		// Get valid completed entries
		validEntries := uploadObj.GetValidEntries()
		if len(validEntries) == 0 {
			continue // No valid entries to consume
		}

		// Trigger upload completion action for stores to process uploads
		// Create connState to pass to handleAction
		state := &connState{
			stores: stores,
			errors: make(map[string]string),
		}

		uploadAction := fmt.Sprintf("upload_%s_complete", uploadName)
		uploadMsg := message{
			Action: uploadAction,
			Data:   make(map[string]interface{}),
		}

		// Call handleAction to process the upload
		// Upload completion actions don't need HTTP response access
		if _, err := h.handleAction(r.Context(), uploadMsg, state, registry, nil, nil); err != nil {
			log.Printf("HTTP upload action %q failed: %v", uploadAction, err)
		}
	}

	return nil
}

// handleUploadAction routes upload-related WebSocket actions to appropriate handlers.
// Returns (handled=true, err) if this was an upload action, (handled=false, nil) otherwise.
func (h *liveHandler) handleUploadAction(ctx context.Context, conn *websocket.Conn, rawData []byte, msg message, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) (bool, error) {
	switch msg.Action {
	case "upload_start":
		return true, h.handleUploadStart(ctx, conn, rawData, state, uploadRegistry, connection)
	case "upload_chunk":
		return true, h.handleUploadChunk(ctx, conn, rawData, state, uploadRegistry, connection)
	case "upload_complete":
		return true, h.handleUploadComplete(ctx, conn, rawData, state, uploadRegistry, connection)
	case "cancel_upload":
		return true, h.handleCancelUpload(ctx, conn, rawData, state, uploadRegistry, connection)
	default:
		return false, nil // Not an upload action
	}
}

// handleUploadStart processes upload_start action from WebSocket client.
// Client sends file metadata, server creates upload entries and responds with entry IDs.
func (h *liveHandler) handleUploadStart(ctx context.Context, conn *websocket.Conn, rawData []byte, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse upload_start message from raw WebSocket data
	startMsg, err := upload.ParseUploadStartMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid upload_start message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Get upload configuration
	uploadObj := registry.GetUpload(startMsg.UploadName)
	if uploadObj == nil {
		return fmt.Errorf("upload %q not configured", startMsg.UploadName)
	}

	uploadInstance, ok := uploadObj.(*upload.Upload)
	if !ok {
		return fmt.Errorf("invalid upload object type")
	}

	// Validate file count
	if err := upload.ValidateCount(len(startMsg.Files), uploadInstance.Config); err != nil {
		return fmt.Errorf("file count validation failed: %w", err)
	}

	// Create upload entries for each file
	response := &upload.UploadStartResponse{
		UploadName: startMsg.UploadName,
		Entries:    make([]upload.UploadEntryInfo, 0, len(startMsg.Files)),
	}

	tempFileManager := h.tempFileManager.(*upload.TempFileManager)
	sessionID := state.groupID

	// Check if external presigner is configured
	isExternal := uploadInstance.Config.External != nil

	for _, fileMeta := range startMsg.Files {
		// Generate entry ID
		entryID := upload.GenerateEntryID()

		// Create upload entry with metadata
		entry := &uploadtypes.UploadEntry{
			ID:         entryID,
			ClientName: fileMeta.Name,
			ClientType: fileMeta.Type,
			ClientSize: fileMeta.Size,
			Progress:   0,
			Done:       false,
			Valid:      false, // Will be validated when entry is added
			BytesRecv:  0,
		}

		var entryInfo upload.UploadEntryInfo

		if isExternal {
			// External upload: generate presigned URL
			presignMeta, err := uploadInstance.Config.External.Presign(entry)
			if err != nil {
				entryInfo = upload.UploadEntryInfo{
					EntryID:    entryID,
					ClientName: fileMeta.Name,
					Valid:      false,
					Error:      fmt.Sprintf("failed to presign: %v", err),
					AutoUpload: uploadInstance.Config.AutoUpload,
				}
			} else {
				// Store external reference in entry
				entry.ExternalRef = presignMeta.URL

				// Validate and add entry to registry
				if err := uploadInstance.AddEntry(entry); err != nil {
					entryInfo = upload.UploadEntryInfo{
						EntryID:    entryID,
						ClientName: fileMeta.Name,
						Valid:      false,
						Error:      err.Error(),
						AutoUpload: uploadInstance.Config.AutoUpload,
					}
				} else {
					// Return presigned metadata to client
					entryInfo = upload.UploadEntryInfo{
						EntryID:    entryID,
						ClientName: fileMeta.Name,
						Valid:      true,
						Error:      "",
						AutoUpload: uploadInstance.Config.AutoUpload,
						External: &upload.ExternalUploadMeta{
							Uploader: presignMeta.Uploader,
							URL:      presignMeta.URL,
							Fields:   presignMeta.Fields,
							Headers:  presignMeta.Headers,
						},
					}
				}
			}
		} else {
			// Server-side upload: create temp file
			tempPath, err := tempFileManager.CreateTempFile(sessionID, startMsg.UploadName, entryID)
			if err != nil {
				entryInfo = upload.UploadEntryInfo{
					EntryID:    entryID,
					ClientName: fileMeta.Name,
					Valid:      false,
					Error:      fmt.Sprintf("failed to create temp file: %v", err),
					AutoUpload: uploadInstance.Config.AutoUpload,
				}
			} else {
				entry.TempPath = tempPath

				// Validate and add entry to registry
				if err := uploadInstance.AddEntry(entry); err != nil {
					entryInfo = upload.UploadEntryInfo{
						EntryID:    entryID,
						ClientName: fileMeta.Name,
						Valid:      false,
						Error:      err.Error(),
						AutoUpload: uploadInstance.Config.AutoUpload,
					}
					// Remove temp file since entry is invalid
					os.Remove(tempPath)
				} else {
					entryInfo = upload.UploadEntryInfo{
						EntryID:    entryID,
						ClientName: fileMeta.Name,
						Valid:      true,
						Error:      "",
						AutoUpload: uploadInstance.Config.AutoUpload,
					}
				}
			}
		}

		response.Entries = append(response.Entries, entryInfo)
	}

	// Send UploadStartResponse to client so it can create upload entries
	responseData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal upload_start response: %w", err)
	}

	if err := connection.Send(websocket.TextMessage, responseData); err != nil {
		return fmt.Errorf("failed to send upload_start response: %w", err)
	}

	// Note: We don't send a tree update here because upload entries are created
	// client-side based on the UploadStartResponse. Tree updates happen after
	// upload completion when store data actually changes.

	return nil
}

// handleUploadChunk processes upload_chunk action from WebSocket client.
// Client sends base64-encoded chunk data, server decodes and appends to temp file.
func (h *liveHandler) handleUploadChunk(ctx context.Context, conn *websocket.Conn, rawData []byte, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse upload_chunk message from raw WebSocket data
	chunkMsg, err := upload.ParseUploadChunkMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid upload_chunk message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Find which upload this entry belongs to
	var targetUpload *upload.Upload
	for _, uploadObj := range registry.GetAllUploads() {
		if uploadObj.GetEntry(chunkMsg.EntryID) != nil {
			targetUpload = uploadObj
			break
		}
	}

	if targetUpload == nil {
		return fmt.Errorf("entry %q not found in any upload", chunkMsg.EntryID)
	}

	// Get the entry
	entry := targetUpload.GetEntry(chunkMsg.EntryID)
	if entry == nil {
		return fmt.Errorf("entry %q not found", chunkMsg.EntryID)
	}

	// Check if entry is valid
	if !entry.Valid {
		return fmt.Errorf("entry %q is invalid: %s", chunkMsg.EntryID, entry.Error)
	}

	// Decode base64 chunk
	chunkData, err := base64.StdEncoding.DecodeString(chunkMsg.ChunkBase64)
	if err != nil {
		return fmt.Errorf("failed to decode chunk: %w", err)
	}

	// Open temp file and append chunk
	tempFile, err := os.OpenFile(entry.TempPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}

	// Write chunk
	written, err := tempFile.Write(chunkData)
	if err != nil {
		if closeErr := tempFile.Close(); closeErr != nil {
			return fmt.Errorf("failed to write chunk: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Close file and check for errors (buffered writes may fail here)
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Update entry progress
	err = targetUpload.UpdateEntry(chunkMsg.EntryID, func(e *uploadtypes.UploadEntry) {
		e.BytesRecv += int64(written)
		if e.ClientSize > 0 {
			e.Progress = int((e.BytesRecv * 100) / e.ClientSize)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Broadcast progress update to this connection only
	progressMsg := &upload.UploadProgressMessage{
		Type:       "upload_progress",
		UploadName: targetUpload.Name,
		EntryID:    entry.ID,
		ClientName: entry.ClientName,
		Progress:   entry.Progress,
		BytesRecv:  entry.BytesRecv,
		BytesTotal: entry.ClientSize,
	}

	progressBytes, err := upload.SerializeUploadProgressMessage(progressMsg)
	if err != nil {
		log.Printf("Failed to serialize progress message: %v", err)
		return nil // Don't fail chunk processing due to progress message error
	}

	if err := writeUpdateWebSocket(connection, progressBytes); err != nil {
		log.Printf("Failed to send progress update: %v", err)
		// Don't fail - progress updates are best-effort
	}

	return nil
}

// handleUploadComplete processes upload_complete action from WebSocket client.
// Client indicates all chunks sent, server marks entries as done and calls ConsumeUpload.
func (h *liveHandler) handleUploadComplete(ctx context.Context, conn *websocket.Conn, rawData []byte, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse upload_complete message from raw WebSocket data
	completeMsg, err := upload.ParseUploadCompleteMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid upload_complete message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Get upload object
	uploadObj := registry.GetUpload(completeMsg.UploadName)
	if uploadObj == nil {
		return fmt.Errorf("upload %q not found", completeMsg.UploadName)
	}

	uploadInstance, ok := uploadObj.(*upload.Upload)
	if !ok {
		return fmt.Errorf("invalid upload object type")
	}

	// Mark all entries as done
	for _, entryID := range completeMsg.EntryIDs {
		err := uploadInstance.UpdateEntry(entryID, func(e *uploadtypes.UploadEntry) {
			e.Done = true
			e.Progress = 100
		})
		if err != nil {
			log.Printf("Failed to mark entry %q as done: %v", entryID, err)
		}
	}

	// Get completed valid entries
	completedEntries := uploadInstance.GetCompletedEntries()

	// Prepare response
	response := &upload.UploadCompleteResponse{
		UploadName: completeMsg.UploadName,
		Success:    true,
		Error:      "",
	}

	// Hybrid approach: Trigger action with special upload action name
	// Stores can handle "upload_<field>_complete" via UploadFieldComplete method
	// or wait for explicit form submission where uploads will be accessible via ActionContext
	if len(completedEntries) > 0 {
		uploadAction := fmt.Sprintf("upload_%s_complete", completeMsg.UploadName)
		uploadMsg := message{
			Action: uploadAction,
			Data:   make(map[string]interface{}), // Empty data, uploads are in ActionContext
		}

		// Call handleAction which will provide upload accessor to ActionContext
		// WebSocket upload completion actions don't need HTTP response access
		registry, ok := uploadRegistry.(*upload.Registry)
		if ok {
			if _, err := h.handleAction(ctx, uploadMsg, state, registry, nil, nil); err != nil {
				log.Printf("Upload action %q failed: %v", uploadAction, err)
				response.Success = false
				response.Error = err.Error()
			}
		}
	}

	// Send tree update to current connection to show upload completion immediately
	// This replaces the old upload_complete response to avoid duplicate messages
	if err := h.sendUpdate(connection, h.getTemplateData(state.stores)); err != nil {
		log.Printf("Failed to send tree update after upload: %v", err)
		return nil // Don't fail the upload, just skip the update
	}

	// Broadcast to other connections in the same group to show upload completion in all tabs
	// Exclude the current connection since we just sent the update above
	h.autoBroadcastToGroup(state.groupID, h.getTemplateData(state.stores), connection)

	return nil
}

// handleCancelUpload processes cancel_upload action from WebSocket client.
// Client cancels an upload, server cleans up temp file and removes entry.
func (h *liveHandler) handleCancelUpload(ctx context.Context, conn *websocket.Conn, rawData []byte, state *connState, uploadRegistry uploadRegistry, connection *session.Connection) error {
	// Parse cancel_upload message from raw WebSocket data
	cancelMsg, err := upload.ParseCancelUploadMessage(rawData)
	if err != nil {
		return fmt.Errorf("invalid cancel_upload message: %w", err)
	}

	// Type assert to get concrete Registry type
	registry, ok := uploadRegistry.(*upload.Registry)
	if !ok {
		return fmt.Errorf("invalid upload registry type")
	}

	// Find which upload this entry belongs to
	var targetUpload *upload.Upload
	for _, uploadObj := range registry.GetAllUploads() {
		if uploadObj.GetEntry(cancelMsg.EntryID) != nil {
			targetUpload = uploadObj
			break
		}
	}

	response := &upload.CancelUploadResponse{
		EntryID: cancelMsg.EntryID,
		Success: true,
	}

	if targetUpload == nil {
		// Entry not found - might have already been removed
		log.Printf("Entry %q not found for cancellation", cancelMsg.EntryID)
	} else {
		// Get entry to find temp file path
		entry := targetUpload.GetEntry(cancelMsg.EntryID)
		if entry != nil && entry.TempPath != "" {
			// Remove temp file directly
			if err := os.Remove(entry.TempPath); err != nil {
				log.Printf("Failed to remove temp file for entry %q: %v", cancelMsg.EntryID, err)
			}
		}

		// Remove entry from registry
		targetUpload.RemoveEntry(cancelMsg.EntryID)
	}

	// Serialize and send response
	respBytes, err := upload.SerializeCancelUploadResponse(response)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	if err := writeUpdateWebSocket(connection, respBytes); err != nil {
		return fmt.Errorf("failed to send cancel_upload response: %w", err)
	}

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
