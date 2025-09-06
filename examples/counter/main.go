package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
)

var availableColors = map[string]string{
	"color-red":       "#ff6b6b",
	"color-teal":      "#4ecdc4",
	"color-blue":      "#45b7d1",
	"color-green":     "#96ceb4",
	"color-yellow":    "#feca57",
	"color-pink":      "#ff6fa6",
	"color-purple":    "#9b59b6",
	"color-lightblue": "#3498db",
	"color-orange":    "#e67e22",
	"color-turquoise": "#1abc9c",
	"color-darkred":   "#e74c3c",
	"color-emerald":   "#2ecc71",
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
	for colorName := range availableColors {
		if colorName != c.Color {
			filteredColors = append(filteredColors, colorName)
		}
	}
	
	if len(filteredColors) == 0 {
		// Fallback if somehow no colors available (shouldn't happen with initial empty color)
		for colorName := range availableColors {
			return colorName
		}
	}
	
	return filteredColors[rand.Intn(len(filteredColors))]
}

// ToMap converts the counter to a map for template rendering
func (c *Counter) ToMap() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	colorHex := availableColors[c.Color]
	if colorHex == "" {
		colorHex = availableColors["color-red"] // fallback
	}
	
	return map[string]any{
		"Counter": c.Value,
		"Color":   colorHex, // Now Color contains the hex value directly
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
	templatePage *livetemplate.ApplicationPage // Template page for stable token and rendering
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
	
	// Register action handlers at the application level
	app.RegisterAction("increment", func(currentData interface{}, actionData map[string]interface{}) (interface{}, error) {
		counter.Increment()
		return counter.ToMap(), nil
	})
	
	app.RegisterAction("decrement", func(currentData interface{}, actionData map[string]interface{}) (interface{}, error) {
		counter.Decrement()
		return counter.ToMap(), nil
	})

	server := &Server{
		app:          app,
		counter:      counter,
		templatePage: templatePage,
	}
	
	log.Printf("Created template page with stable token: %s", templatePage.GetToken())
	
	return server
}


func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := s.counter.ToMap()
	log.Printf("HTTP render with data: Counter=%d, Color=%s", s.counter.GetValue(), s.counter.GetColor())
	log.Printf("Request from: %s, User-Agent: %s", r.RemoteAddr, r.Header.Get("User-Agent"))

	// Clean and intuitive: render with data and serve in one call
	if err := s.templatePage.ServeHTTP(w, data); err != nil {
		log.Printf("Error serving page: %v", err)
		http.Error(w, "Failed to serve page", http.StatusInternalServerError)
		return
	}

	log.Printf("Served page with stable token: %s", s.templatePage.GetToken())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket connection attempt from: %s", r.RemoteAddr)
	log.Printf("WebSocket URL: %s", r.URL.String())
	log.Printf("WebSocket Query params: %v", r.URL.Query())
	
	// Get page from request - handles all authentication complexity internally
	page, err := s.app.GetPage(r)
	if err != nil {
		log.Printf("Failed to get page from WebSocket request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get page: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("WebSocket page retrieved successfully")

	// Upgrade to WebSocket
	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket connected with actions registered: %t", page.HasActions())

	// Handle messages using the page with registered actions
	for {
		var message map[string]interface{}
		err := conn.ReadJSON(&message)
		if err != nil {
			break
		}

		// Check message type
		msgType, _ := message["type"].(string)
		if msgType != "action" {
			continue
		}

		// Extract action name
		actionName, ok := message["action"].(string)
		if !ok {
			continue
		}

		log.Printf("Processing action: %s", actionName)

		// Process action using the page with registered actions
		actionData, _ := message["data"].(map[string]interface{})
		if actionData == nil {
			actionData = make(map[string]interface{})
		}

		fragments, err := page.HandleAction(context.Background(), actionName, actionData)
		if err != nil {
			log.Printf("Action handler error: %v", err)
			continue
		}

		log.Printf("Generated %d fragments", len(fragments))

		// Send fragments to client
		if err := conn.WriteJSON(fragments); err != nil {
			log.Printf("WebSocket send error: %v", err)
			break
		}
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
		log.Printf("Serving client JS to: %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, r, "../../client/livetemplate-client.js")
	})
	
	fmt.Printf("Counter app running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}