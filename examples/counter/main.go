package main

import (
	"context"
	"encoding/json"
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
	log.Printf("📋 WebSocket URL: %s", r.URL.String())
	log.Printf("🔍 WebSocket Query params: %v", r.URL.Query())

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

	log.Printf("🚀 WebSocket connected with unified tree diff enabled: %t", page.HasActions())

	// Handle messages using the page with registered actions
	for {
		var message livetemplate.ActionMessage
		err := conn.ReadJSON(&message)
		if err != nil {
			log.Printf("📤 WebSocket disconnected: %v", err)
			break
		}

		log.Printf("⚡ Processing action: %s", message.Action)
		log.Printf("🔍 DEBUG: Message cache: %+v", message.Cache)
		if len(message.Cache) > 0 {
			log.Printf("🔍 DEBUG: Cached fragments: %v", message.Cache)
		} else {
			log.Printf("🔍 DEBUG: No cache in message - sending full statics")
		}

		fragments, err := page.HandleAction(context.TODO(), &message)
		if err != nil {
			log.Printf("❌ Action handler error: %v", err)
			continue
		}

		log.Printf("📦 Generated %d unified tree diff fragments", len(fragments))

		// Log fragment details for debugging
		if len(fragments) > 0 {
			fragment := fragments[0]
			log.Printf("🔧 Fragment strategy: %d, size: ~%d bytes",
				func() int {
					if fragment.Metadata != nil {
						return fragment.Metadata.Strategy
					}
					return 0
				}(),
				len(fmt.Sprintf("%v", fragment.Data)))

			// DEBUG: Log the actual fragment structure
			fragmentJSON, _ := json.MarshalIndent(fragment, "", "  ")
			log.Printf("🔍 DEBUG Fragment structure:\n%s", string(fragmentJSON))
		}

		// Transform fragments to new efficient format where lvt-id is the outer key
		fragmentMap := transformFragmentsToMap(fragments)

		// Send fragments to client in new format
		if err := conn.WriteJSON(fragmentMap); err != nil {
			log.Printf("❌ WebSocket send error: %v", err)
			break
		}

		log.Printf("✅ Sent unified tree diff fragments to client")
	}
}

// transformFragmentsToMap converts fragment array to key-value map format for efficiency
func transformFragmentsToMap(fragments []*livetemplate.Fragment) map[string]interface{} {
	fragmentMap := make(map[string]interface{})

	for _, fragment := range fragments {
		// Use the fragment ID as the outer key and the data as the value
		fragmentMap[fragment.ID] = fragment.Data
	}

	return fragmentMap
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
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../dist/"))).ServeHTTP(w, r)
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
