package livetemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/websocket"
)

// Broadcaster allows stores to push updates to connected clients without user interaction
type Broadcaster interface {
	Send() error // Re-renders template and sends update to this connection
}

// BroadcastAware is implemented by stores that need server-initiated updates
// Examples: live notifications, stock tickers, background job status, real-time sync
type BroadcastAware interface {
	OnConnect(ctx context.Context, b Broadcaster) error
	OnDisconnect()
}

// broadcaster implements the Broadcaster interface for a single WebSocket connection
type broadcaster struct {
	conn     *websocket.Conn
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
	var tree treeNode
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

// MountConfig configures the mount handler
type MountConfig struct {
	Template          *Template
	Stores            Stores
	IsSingleStore     bool
	Upgrader          *websocket.Upgrader
	SessionStore      SessionStore
	WebSocketDisabled bool
}

// Mount creates an http.Handler that auto-generates updates when state changes
// For single store: actions like "increment", "decrement"
func Mount(tmpl *Template, store Store, opts ...MountOption) http.Handler {
	config := MountConfig{
		Template:      tmpl,
		Stores:        Stores{"": store}, // Empty key for single store
		IsSingleStore: true,
		Upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		SessionStore: NewMemorySessionStore(),
	}

	for _, opt := range opts {
		opt(&config)
	}

	return &liveHandler{config: config}
}

// MountStores creates an http.Handler for multiple named stores
// For multiple stores: actions like "counter.increment", "user.setName"
func MountStores(tmpl *Template, stores Stores, opts ...MountOption) http.Handler {
	if len(stores) == 0 {
		panic("MountStores requires at least one store")
	}

	config := MountConfig{
		Template:      tmpl,
		Stores:        stores,
		IsSingleStore: false,
		Upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		SessionStore: NewMemorySessionStore(),
	}

	for _, opt := range opts {
		opt(&config)
	}

	return &liveHandler{config: config}
}

// MountOption is a functional option for configuring Mount/MountStores
// Deprecated: Use Option with Template.Handle() instead
type MountOption func(*MountConfig)

// liveHandler handles both WebSocket and HTTP requests
type liveHandler struct {
	config MountConfig
}

type connState struct {
	stores   Stores            // Each connection gets cloned stores
	errors   map[string]string // Field errors from last action
	errorsMu sync.RWMutex      // Mutex for thread-safe error access
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
	conn, err := h.config.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected from %s", conn.RemoteAddr())

	// Clone template for this connection to avoid state conflicts
	// Each WebSocket connection needs its own template instance because
	// ExecuteUpdates() tracks state (lastTree, lastData, etc.)
	connTmpl, err := h.config.Template.Clone()
	if err != nil {
		log.Printf("Failed to clone template: %v", err)
		return
	}

	// Create connection state
	state := &connState{
		stores: h.cloneStores(),
		errors: make(map[string]string),
	}

	// Create context for broadcaster lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create broadcaster for server-initiated updates
	bc := &broadcaster{
		conn:     conn,
		template: connTmpl,
		state:    state,
		handler:  h,
	}

	// Call OnConnect for stores that implement BroadcastAware
	for _, store := range state.stores {
		if aware, ok := store.(BroadcastAware); ok {
			if err := aware.OnConnect(ctx, bc); err != nil {
				log.Printf("OnConnect failed for store: %v", err)
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
	var tree treeNode
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

	err = writeUpdateWebSocket(conn, responseBytes)
	if err != nil {
		log.Printf("Failed to send initial tree: %v", err)
		return
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

		// Parse message
		msg, err := parseActionFromWebSocket(data)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Handle action
		if err := h.handleAction(msg, state); err != nil {
			log.Printf("Action error: %v", err)
			continue
		}

		// Generate tree update
		buf.Reset()
		err = connTmpl.ExecuteUpdates(&buf, h.getTemplateData(state.stores), state.getErrors())
		if err != nil {
			log.Printf("Template update execution failed: %v", err)
			continue
		}

		// Parse tree from buffer
		var tree treeNode
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

		err = writeUpdateWebSocket(conn, responseBytes)
		if err != nil {
			log.Printf("WebSocket write failed: %v", err)
			break
		}
	}

	log.Printf("Client disconnected")
}

func (h *liveHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle HEAD request for capability check
	if r.Method == http.MethodHead {
		// Just return headers, no body
		return
	}

	// Get or create session state
	sessionID := getSessionID(r)
	isNewSession := false
	sessionData := h.config.SessionStore.Get(sessionID)

	var state *connState
	if sessionData == nil {
		state = &connState{
			stores: h.cloneStores(),
			errors: make(map[string]string),
		}
		h.config.SessionStore.Set(sessionID, state)
		isNewSession = true
	} else {
		state = sessionData.(*connState)
	}

	// Set session cookie if this is a new session
	if isNewSession {
		http.SetCookie(w, &http.Cookie{
			Name:     "livetemplate-session",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// Handle GET request for initial HTML page
	if r.Method == http.MethodGet {
		// Always reload data from database for GET requests to ensure fresh data
		// This prevents stale session state when WebSocket actions modify data
		for _, store := range state.stores {
			if initializer, ok := store.(StoreInitializer); ok {
				if err := initializer.Init(); err != nil {
					log.Printf("Warning: Store initialization failed for GET request: %v", err)
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

	// Parse message
	msg, err := parseActionFromHTTP(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle action
	if err := h.handleAction(msg, state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save session
	h.config.SessionStore.Set(sessionID, state)

	// Generate tree update
	var buf bytes.Buffer
	err = h.config.Template.ExecuteUpdates(&buf, h.getTemplateData(state.stores), state.getErrors())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse tree from buffer
	var tree treeNode
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

// handleAction routes the action to the correct store and captures errors
func (h *liveHandler) handleAction(msg message, state *connState) error {
	// Clear previous errors
	state.clearErrors()

	// Parse action to extract store name
	storeName, action := parseAction(msg.Action)

	var store Store
	if h.config.IsSingleStore {
		// Single store mode
		if storeName != "" {
			return fmt.Errorf(
				"unexpected store prefix '%s' in single-store mode\n"+
					"Use action '%s' instead of '%s'",
				storeName, action, msg.Action)
		}

		// Get the single store
		store = state.stores[""]

	} else {
		// Multi-store mode
		if storeName == "" {
			return fmt.Errorf(
				"action '%s' missing store prefix in multi-store mode\n"+
					"Available stores: %v\n"+
					"Use format: 'storeName.action' (e.g., 'counter.increment')",
				msg.Action, h.getStoreNames())
		}

		// Find store using case-insensitive matching
		store = h.findStore(state.stores, storeName)
		if store == nil {
			return fmt.Errorf(
				"unknown store: '%s' in action '%s'\n"+
					"Available stores: %v",
				storeName, msg.Action, h.getStoreNames())
		}
	}

	// Create action context
	ctx := &ActionContext{
		Action: action,
		Data:   newActionData(msg.Data),
	}

	// Call Change and capture error
	err := store.Change(ctx)

	if err != nil {
		// Process the error
		switch e := err.(type) {
		case FieldError:
			state.setError(e.Field, e.Message)
		case MultiError:
			for _, fieldErr := range e {
				state.setError(fieldErr.Field, fieldErr.Message)
			}
		default:
			state.setError("_general", err.Error())
		}
	}

	return nil
}

// findStore finds a store by name using case-insensitive matching
func (h *liveHandler) findStore(stores Stores, name string) Store {
	normalized := normalizeStoreName(name)

	for storeName, store := range stores {
		if normalizeStoreName(storeName) == normalized {
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

// cloneStores creates new instances of all stores
func (h *liveHandler) cloneStores() Stores {
	cloned := make(Stores)
	for name, store := range h.config.Stores {
		cloned[name] = cloneStore(store)
	}
	return cloned
}

// cloneStore creates a new instance of a store
func cloneStore(store Store) Store {
	storeType := reflect.TypeOf(store)
	if storeType.Kind() == reflect.Ptr {
		storeType = storeType.Elem()
	}

	// Create new instance
	newStore := reflect.New(storeType).Interface().(Store)

	// Copy field values
	copyStruct(newStore, store)

	// Call Init() if the store implements StoreInitializer
	if initializer, ok := newStore.(StoreInitializer); ok {
		if err := initializer.Init(); err != nil {
			// Log the error but don't fail - store is in a partially initialized state
			// The error will be handled when the store is actually used
			log.Printf("Warning: Store initialization failed: %v", err)
		}
	}

	return newStore
}

// copyStruct copies field values from src to dst
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

// getSessionID extracts session ID from cookie or header
func getSessionID(r *http.Request) string {
	// Try cookie first
	if cookie, err := r.Cookie("livetemplate-session"); err == nil {
		return cookie.Value
	}

	// Try header
	if sessionID := r.Header.Get("X-LiveTemplate-Session"); sessionID != "" {
		return sessionID
	}

	// Generate new session ID
	return generateRandomID()
}
