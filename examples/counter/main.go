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
	app          *livetemplate.Application
	counter      *Counter
	upgrader     websocket.Upgrader
	templatePage *livetemplate.ApplicationPage // Template page for stable token and rendering
}

type ActionMessage struct {
	Type   string         `json:"type"`
	Action string         `json:"action"`
	Data   map[string]any `json:"data"`
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
	
	// Create a template page with stable token for consistent rendering
	templatePage, err := app.NewPage("counter", counter.ToMap())
	if err != nil {
		log.Fatal("Failed to create template page:", err)
	}
	
	server := &Server{
		app:          app,
		counter:      counter,
		templatePage: templatePage,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	
	log.Printf("Created template page with stable token: %s", templatePage.GetToken())
	
	return server
}


func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := s.counter.ToMap()
	log.Printf("HTTP render with data: Counter=%d, Color=%s", s.counter.GetValue(), s.counter.GetColor())

	// Update template page with current data
	err := s.templatePage.SetData(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update page data: %v", err), http.StatusInternalServerError)
		return
	}

	// Render using the template page (same token every time)
	html, err := s.templatePage.Render()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render page: %v", err), http.StatusInternalServerError)
		return
	}

	// Embed stable token in HTML for WebSocket to use
	html = strings.ReplaceAll(html, "PAGE_TOKEN_PLACEHOLDER", s.templatePage.GetToken())

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Error writing response: %v", err)
	}

	log.Printf("Served page with stable token: %s", s.templatePage.GetToken())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Get page with token and cache info in one simple call
	page, err := s.app.GetPageFromRequest(r)
	if err != nil {
		log.Printf("Failed to get page from request: %v", err)
		return
	}

	cacheInfo := page.GetCacheInfo()
	log.Printf("WebSocket connected with token: %s, cache: %t (%d fragments)", 
		page.GetToken(), cacheInfo.HasCache, len(cacheInfo.CachedFragments))

	// Handle messages
	for {
		var msg ActionMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		log.Printf("Received action: %s", msg.Action)

		// Update counter based on action
		oldValue := s.counter.GetValue()
		switch msg.Action {
		case "increment":
			s.counter.Increment()
		case "decrement":
			s.counter.Decrement()
		default:
			log.Printf("Unknown action: %s", msg.Action)
			continue
		}

		log.Printf("Counter updated from %d to %d with color %s", oldValue, s.counter.GetValue(), s.counter.GetColor())

		// Generate fragments using LiveTemplate's transparent cache-aware API
		newData := s.counter.ToMap()
		log.Printf("Generating fragments with new data: Counter=%d, Color=%s", s.counter.GetValue(), s.counter.GetColor())
		log.Printf("New data map: %+v", newData)

		// LiveTemplate automatically handles cache filtering based on page's cache info
		fragments, err := page.RenderFragments(context.Background(), newData)
		if err != nil {
			log.Printf("Error rendering fragments: %v", err)
			continue
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
	}
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