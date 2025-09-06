package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
)

var availableColors = []string{
	"color-red",
	"color-teal",
	"color-blue",
	"color-green",
	"color-yellow",
	"color-pink",
	"color-purple",
	"color-lightblue",
	"color-orange",
	"color-turquoise",
	"color-darkred",
	"color-emerald",
}

// Counter represents the counter data model
type Counter struct {
	mu    sync.RWMutex
	Value int    `json:"Counter"`
	Color string `json:"Color"`
}

// NewCounter creates a new counter with initial state
func NewCounter() *Counter {
	c := &Counter{
		Value: 0,
		Color: "",
	}
	c.Color = c.getNextColor()
	return c
}

// Increment increases the counter value and changes color
func (c *Counter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Value++
	c.Color = c.getNextColor()
}

// Decrement decreases the counter value and changes color
func (c *Counter) Decrement() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Value--
	c.Color = c.getNextColor()
}

// getNextColor ensures color always changes from current color
func (c *Counter) getNextColor() string {
	// Filter out current color to ensure it changes
	var filteredColors []string
	for _, color := range availableColors {
		if color != c.Color {
			filteredColors = append(filteredColors, color)
		}
	}
	
	if len(filteredColors) == 0 {
		// Fallback if somehow no colors available (shouldn't happen with initial empty color)
		return availableColors[0]
	}
	
	return filteredColors[rand.Intn(len(filteredColors))]
}

// ToMap converts the counter to a map for template rendering
func (c *Counter) ToMap() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"Counter": c.Value,
		"Color":   c.Color,
	}
}

// GetValue returns the current counter value (thread-safe)
func (c *Counter) GetValue() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Value
}

// GetColor returns the current color (thread-safe)
func (c *Counter) GetColor() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Color
}

type Server struct {
	app        *livetemplate.Application
	counter    *Counter
	upgrader   websocket.Upgrader
	sharedPage *livetemplate.ApplicationPage // Single shared page for all clients
}

type ActionMessage struct {
	Type   string         `json:"type"`
	Action string         `json:"action"`
	Data   map[string]any `json:"data"`
}

type CacheStatusMessage struct {
	Type            string   `json:"type"`
	Token           string   `json:"token"`
	HasCache        bool     `json:"has_cache"`
	CachedFragments []string `json:"cached_fragments"`
}

func NewServer() *Server {
	app, err := livetemplate.NewApplication()
	if err != nil {
		log.Fatal(err)
	}
	
	// Register the template once with the application
	err = app.RegisterTemplateFromFile("counter", "templates/index.html")
	if err != nil {
		log.Fatal("Failed to register template:", err)
	}
	
	counter := NewCounter()
	
	// Create a single shared page that persists across reloads
	sharedPage, err := app.NewPage("counter", counter.ToMap())
	if err != nil {
		log.Fatal("Failed to create shared page:", err)
	}
	
	server := &Server{
		app:        app,
		counter:    counter,
		sharedPage: sharedPage,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	
	log.Printf("Created shared page with stable token: %s", sharedPage.GetToken())
	
	return server
}


func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := s.counter.ToMap()
	log.Printf("HTTP render with data: Counter=%d, Color=%s", s.counter.GetValue(), s.counter.GetColor())

	// Update shared page with current data
	err := s.sharedPage.SetData(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update page data: %v", err), http.StatusInternalServerError)
		return
	}

	// Render using the shared page (same token every time)
	html, err := s.sharedPage.Render()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render page: %v", err), http.StatusInternalServerError)
		return
	}

	// Embed stable token in HTML for WebSocket to use
	html = strings.ReplaceAll(html, "PAGE_TOKEN_PLACEHOLDER", s.sharedPage.GetToken())

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Error writing response: %v", err)
	}

	log.Printf("Served page with stable token: %s", s.sharedPage.GetToken())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Get token from query param
	token := r.URL.Query().Get("token")
	if token == "" {
		log.Printf("No token provided in WebSocket connection")
		return
	}

	// Verify token matches our shared page
	if token != s.sharedPage.GetToken() {
		log.Printf("Token mismatch: expected %s, got %s", s.sharedPage.GetToken(), token)
		return
	}

	log.Printf("WebSocket connected to shared page with stable token: %s", token)
	
	// Track client cache status
	clientHasCache := false
	cachedFragments := make(map[string]bool)

	// Handle messages
	for {
		var rawMessage map[string]interface{}
		err := conn.ReadJSON(&rawMessage)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		messageType, ok := rawMessage["type"].(string)
		if !ok {
			log.Printf("Invalid message: missing or invalid type")
			continue
		}

		switch messageType {
		case "cache_status":
			// Handle cache status message
			var cacheMsg CacheStatusMessage
			msgBytes, _ := json.Marshal(rawMessage)
			json.Unmarshal(msgBytes, &cacheMsg)
			
			clientHasCache = cacheMsg.HasCache
			cachedFragments = make(map[string]bool)
			for _, fragID := range cacheMsg.CachedFragments {
				cachedFragments[fragID] = true
			}
			
			log.Printf("Client cache status: %t (%d cached fragments)", clientHasCache, len(cacheMsg.CachedFragments))
			continue

		case "action":
			// Handle action message
			var actionMsg ActionMessage
			msgBytes, _ := json.Marshal(rawMessage)
			json.Unmarshal(msgBytes, &actionMsg)
			
			log.Printf("Received action: %s", actionMsg.Action)

			// Update counter based on action
			oldValue := s.counter.GetValue()
			switch actionMsg.Action {
			case "increment":
				s.counter.Increment()
			case "decrement":
				s.counter.Decrement()
			default:
				log.Printf("Unknown action: %s", actionMsg.Action)
				continue
			}

			log.Printf("Counter updated from %d to %d with color %s", oldValue, s.counter.GetValue(), s.counter.GetColor())

			// Generate fragments using proper LiveTemplate API with both Counter and Color
			newData := s.counter.ToMap()
			log.Printf("Generating fragments with new data: Counter=%d, Color=%s", s.counter.GetValue(), s.counter.GetColor())
			log.Printf("New data map: %+v", newData)

			fragments, err := s.sharedPage.RenderFragments(context.Background(), newData)
			if err != nil {
				log.Printf("Error rendering fragments: %v", err)
				continue
			}

			// Modify fragments based on client cache status
			if clientHasCache {
				log.Printf("Client has cache, filtering statics from %d fragments", len(fragments))
				fragments = s.filterStaticsFromFragments(fragments, cachedFragments)
				log.Printf("Filtered to %d fragments without statics", len(fragments))
			}

			log.Printf("Generated %d fragments", len(fragments))
			for i, frag := range fragments {
				log.Printf("Fragment %d: ID=%s, Data=%+v", i, frag.ID, frag.Data)
				
				// Debug: Check JSON marshaling
				if jsonData, err := json.Marshal(frag.Data); err == nil {
					log.Printf("Fragment %d JSON: %s", i, string(jsonData))
				}
			}

			// Send fragments directly to client
			if err := conn.WriteJSON(fragments); err != nil {
				log.Printf("Error sending fragments: %v", err)
				break
			}

			log.Printf("Counter updated to: %d", s.counter.GetValue())
		
		default:
			log.Printf("Unknown message type: %s", messageType)
			continue
		}
	}
}

// filterStaticsFromFragments removes statics from fragments that client already has cached
func (s *Server) filterStaticsFromFragments(fragments []*livetemplate.Fragment, cachedFragments map[string]bool) []*livetemplate.Fragment {
	var filtered []*livetemplate.Fragment
	
	for _, frag := range fragments {
		// Create a copy of the fragment
		newFrag := &livetemplate.Fragment{
			ID:       frag.ID,
			Data:     frag.Data,
			Metadata: frag.Metadata,
		}
		
		// If client has this fragment cached, remove statics from data
		if cachedFragments[frag.ID] {
			if treeData, ok := frag.Data.(map[string]interface{}); ok {
				// Create new tree data without statics
				newTreeData := make(map[string]interface{})
				for k, v := range treeData {
					if k != "s" { // Remove statics ("s" field)
						newTreeData[k] = v
					}
				}
				newFrag.Data = newTreeData
				log.Printf("Removed statics from fragment %s", frag.ID)
			}
		}
		
		filtered = append(filtered, newFrag)
	}
	
	return filtered
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := NewServer()
	
	http.HandleFunc("/", server.handleHome)
	http.HandleFunc("/ws", server.handleWebSocket)
	
	// Serve the LiveTemplate client library
	http.HandleFunc("/client/livetemplate-client.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "../../client/livetemplate-client.js")
	})
	
	fmt.Printf("Counter app running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}