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

// Counter represents the counter data model using the unified tree diff approach
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

// Increment increases the counter value and changes color (data model action method)
func (c *Counter) Increment(ctx *livetemplate.ActionContext) error {
	c.mu.Lock()
	c.Value++
	c.Color = c.getNextColor()

	// Get the data while holding the lock to avoid deadlock with ToMap()
	colorHex := availableColors[c.Color]
	if colorHex == "" {
		colorHex = availableColors["color-red"] // fallback
	}
	data := map[string]any{
		"Counter": c.Value,
		"Color":   colorHex,
	}
	c.mu.Unlock()

	// Set response data using the clean ActionContext API
	return ctx.Data(data)
}

// Decrement decreases the counter value and changes color (data model action method)
func (c *Counter) Decrement(ctx *livetemplate.ActionContext) error {
	c.mu.Lock()
	c.Value--
	c.Color = c.getNextColor()

	// Get the data while holding the lock to avoid deadlock with ToMap()
	colorHex := availableColors[c.Color]
	if colorHex == "" {
		colorHex = availableColors["color-red"] // fallback
	}
	data := map[string]any{
		"Counter": c.Value,
		"Color":   colorHex,
	}
	c.mu.Unlock()

	// Set response data using the clean ActionContext API
	return ctx.Data(data)
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
	templatePage *livetemplate.ApplicationPage // Template page using unified tree diff
}

func NewServer() *Server {
	app, err := livetemplate.NewApplication()
	if err != nil {
		log.Fatal(err)
	}

	// Parse and auto-register the template using standard ParseFiles pattern
	// Template will be registered as "index" (filename without extension)
	_, err = app.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("Failed to parse template:", err)
	}

	counter := NewCounter()

	// Create a template page with stable token for consistent rendering
	// Use "index" since that's what ParseFiles registered (filename without extension)
	templatePage, err := app.NewPage("index", counter.ToMap())
	if err != nil {
		log.Fatal("Failed to create template page:", err)
	}

	// Register counter as a data model
	// Actions will be automatically detected from methods with the clean signature:
	// func(ctx *livetemplate.ActionContext) error
	err = templatePage.RegisterDataModel(counter)
	if err != nil {
		log.Fatal("Failed to register counter data model:", err)
	}
	log.Printf("✅ Registered counter data model with unified tree diff support")

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
	// Add the token to the template data so client can connect to WebSocket
	data["Token"] = s.templatePage.GetToken()

	log.Printf("🌐 HTTP render with unified tree diff - Counter=%d, Color=%s", s.counter.GetValue(), s.counter.GetColor())
	log.Printf("📡 Request from: %s, User-Agent: %s", r.RemoteAddr, r.Header.Get("User-Agent"))

	// Clean and intuitive: render with data and serve in one call
	if err := s.templatePage.ServeHTTP(w, data); err != nil {
		log.Printf("❌ Error serving page: %v", err)
		http.Error(w, "Failed to serve page", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Served page with unified tree diff - Token: %s", s.templatePage.GetToken())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔌 WebSocket connection attempt from: %s", r.RemoteAddr)

	// Get page from request - handles all authentication complexity internally
	page, err := s.app.GetPage(r)
	if err != nil {
		log.Printf("❌ Failed to get page from WebSocket request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get page: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("✅ WebSocket page retrieved successfully")

	// Upgrade to WebSocket
	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("🚀 WebSocket connected - page-token-based optimization enabled")

	// Extremely simple message loop - no connection management needed!
	for {
		var message livetemplate.ActionMessage
		err := conn.ReadJSON(&message)
		if err != nil {
			log.Printf("📤 WebSocket disconnected: %v", err)
			break
		}

		log.Printf("⚡ Processing action: %s", message.Action)

		// Ultra-simple: just call HandleAction - page token tracks everything!
		fragmentMap, err := page.HandleAction(context.TODO(), &message)
		if err != nil {
			log.Printf("❌ Action handler error: %v", err)
			continue
		}

		log.Printf("📦 Generated %d fragment updates (page-token optimized)", len(fragmentMap))

		if err := conn.WriteJSON(fragmentMap); err != nil {
			log.Printf("❌ WebSocket send error: %v", err)
			break
		}

		log.Printf("✅ Sent optimized fragments (reconnection-friendly)")
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("🚀 LiveTemplate Unified Counter App")
	fmt.Println("===================================")

	server := NewServer()

	http.HandleFunc("/", server.handleHome)
	http.HandleFunc("/ws", server.handleWebSocket)

	// Serve the bundled LiveTemplate client library
	http.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("📦 Serving bundled client to: %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	fmt.Printf("🌟 Unified Counter app running on http://localhost:%s\n", port)
	fmt.Println("💡 Features:")
	fmt.Println("  • Unified tree diff optimization")
	fmt.Println("  • 88%+ bandwidth savings")
	fmt.Println("  • No HTML intrinsics knowledge required")
	fmt.Println("  • Phoenix LiveView compatible JSON structure")
	fmt.Println("  • Real-time WebSocket updates")
	fmt.Println()

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
