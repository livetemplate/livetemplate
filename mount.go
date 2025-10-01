package livetemplate

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/websocket"
)

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
	config      MountConfig
	connections map[*websocket.Conn]*connState
	connMu      sync.RWMutex
}

type connState struct {
	stores Stores // Each connection gets cloned stores
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

	// Clone stores for this connection
	stores := h.cloneStores()

	// Send initial tree
	var buf bytes.Buffer
	err = connTmpl.ExecuteUpdates(&buf, h.getTemplateData(stores))
	if err != nil {
		log.Printf("Failed to generate initial tree: %v", err)
		return
	}

	err = WriteUpdateWebSocket(conn, buf.Bytes())
	if err != nil {
		log.Printf("Failed to send initial tree: %v", err)
		return
	}

	// Message loop
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse message
		msg, err := ParseActionFromWebSocket(data)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Handle action
		if err := h.handleAction(msg, stores); err != nil {
			log.Printf("Action error: %v", err)
			continue
		}

		// Generate and send update
		buf.Reset()
		err = connTmpl.ExecuteUpdates(&buf, h.getTemplateData(stores))
		if err != nil {
			log.Printf("Template update execution failed: %v", err)
			continue
		}

		err = WriteUpdateWebSocket(conn, buf.Bytes())
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

	// Get or create session stores
	sessionID := getSessionID(r)
	isNewSession := false
	stores := h.config.SessionStore.Get(sessionID)
	if stores == nil {
		stores = h.cloneStores()
		h.config.SessionStore.Set(sessionID, stores)
		isNewSession = true
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
		err := h.config.Template.Execute(w, h.getTemplateData(stores.(Stores)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// Handle POST request for actions
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse message
	msg, err := ParseActionFromHTTP(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle action
	if err := h.handleAction(msg, stores.(Stores)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save session
	h.config.SessionStore.Set(sessionID, stores)

	// Generate and send update
	var buf bytes.Buffer
	err = h.config.Template.ExecuteUpdates(&buf, h.getTemplateData(stores.(Stores)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteUpdateHTTP(w, buf.Bytes())
}

// handleAction routes the action to the correct store
func (h *liveHandler) handleAction(msg Message, stores Stores) error {
	// Parse action to extract store name
	storeName, action := ParseAction(msg.Action)

	if h.config.IsSingleStore {
		// Single store mode
		if storeName != "" {
			return fmt.Errorf(
				"unexpected store prefix '%s' in single-store mode\n"+
					"Use action '%s' instead of '%s'",
				storeName, action, msg.Action)
		}

		// Get the single store
		store := stores[""]
		store.Change(action, msg.Data)

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
		store := h.findStore(stores, storeName)
		if store == nil {
			return fmt.Errorf(
				"unknown store: '%s' in action '%s'\n"+
					"Available stores: %v",
				storeName, msg.Action, h.getStoreNames())
		}

		// Call Change with action ONLY (no store prefix)
		store.Change(action, msg.Data)
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
