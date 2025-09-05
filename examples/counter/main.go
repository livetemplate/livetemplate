package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"

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

type Server struct {
	app      *livetemplate.Application
	counter  int
	color    string
	upgrader websocket.Upgrader
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
	
	server := &Server{
		app:     app,
		counter: 0,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	
	// Set initial color using the same logic as getNextColor
	server.color = server.getNextColor()
	
	return server
}

// getNextColor ensures color always changes from current color
func (s *Server) getNextColor() string {
	// Filter out current color to ensure it changes
	var filteredColors []string
	for _, color := range availableColors {
		if color != s.color {
			filteredColors = append(filteredColors, color)
		}
	}
	
	if len(filteredColors) == 0 {
		// Fallback if somehow no colors available (shouldn't happen with initial empty color)
		return availableColors[0]
	}
	
	return filteredColors[rand.Intn(len(filteredColors))]
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Counter": s.counter,
		"Color":   s.color,
	}
	log.Printf("HTTP render with data: Counter=%d, Color=%s", s.counter, s.color)

	// Use LiveTemplate to render with annotations instead of direct template execution
	page, err := s.app.NewPageFromTemplate("counter", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create page: %v", err), http.StatusInternalServerError)
		return
	}
	defer page.Close()

	html, err := page.Render()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render page: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Create a page for this WebSocket connection using a simple counter template
	initialData := map[string]any{
		"Counter": s.counter,
		"Color":   s.color,
	}
	log.Printf("WebSocket creating page with initial data: Counter=%d, Color=%s", s.counter, s.color)
	log.Printf("Initial data map: %+v", initialData)
	
	page, err := s.app.NewPageFromTemplate("counter", initialData)
	if err != nil {
		log.Printf("Error creating page: %v", err)
		return
	}
	defer page.Close()

	// Debug: Check initial render
	initialRender, err := page.Render()
	if err != nil {
		log.Printf("Error rendering initial page: %v", err)
	} else {
		log.Printf("Initial page render: %s", initialRender)
	}

	// Send page token to client
	tokenMessage := map[string]any{
		"type":  "page_token",
		"token": page.GetToken(),
	}
	
	if err := conn.WriteJSON(tokenMessage); err != nil {
		log.Printf("Error sending token: %v", err)
		return
	}

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
		oldValue := s.counter
		switch msg.Action {
		case "increment":
			s.counter++
			s.color = s.getNextColor() // Change color on increment (guaranteed different)
		case "decrement":
			s.counter--
			s.color = s.getNextColor() // Change color on decrement (guaranteed different)
		default:
			log.Printf("Unknown action: %s", msg.Action)
			continue
		}

		log.Printf("Counter updated from %d to %d with color %s", oldValue, s.counter, s.color)

		// Generate fragments using proper LiveTemplate API with both Counter and Color
		newData := map[string]any{
			"Counter": s.counter,
			"Color":   s.color,
		}
		log.Printf("Generating fragments with new data: Counter=%d, Color=%s", s.counter, s.color)
		log.Printf("New data map: %+v", newData)

		fragments, err := page.RenderFragments(context.Background(), newData)
		if err != nil {
			log.Printf("Error rendering fragments: %v", err)
			continue
		}
		

		log.Printf("Generated %d fragments (Expected: 2 for both Counter and Color changes)", len(fragments))
		for i, frag := range fragments {
			log.Printf("Fragment %d: ID=%s, Data=%+v", i, frag.ID, frag.Data)
			
			// Debug: Check JSON marshaling
			if jsonData, err := json.Marshal(frag.Data); err == nil {
				log.Printf("Fragment %d JSON: %s", i, string(jsonData))
			}
		}

		// Send fragments to client
		response := map[string]any{
			"type":      "fragments", 
			"fragments": fragments,
		}

		if err := conn.WriteJSON(response); err != nil {
			log.Printf("Error sending fragments: %v", err)
			break
		}

		log.Printf("Counter updated to: %d", s.counter)
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
	
	fmt.Printf("Counter app running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}