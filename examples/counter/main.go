package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
)

type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	Status      string `json:"status"`
	LastUpdated string `json:"last_updated"`
	SessionID   string `json:"session_id"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin for development
	},
}

var tmpl *livetemplate.Template

func getStatus(counter int) string {
	if counter > 0 {
		return "positive"
	} else if counter < 0 {
		return "negative"
	}
	return "zero"
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected from %s", conn.RemoteAddr())

	// Initial state
	state := &CounterState{
		Title:       "Live Counter",
		Counter:     0,
		Status:      "zero",
		LastUpdated: formatTime(),
		SessionID:   fmt.Sprintf("session-%d", time.Now().Unix()),
	}

	// Send initial full tree with statics on connection
	// Note: For production with multiple concurrent users, each WebSocket connection
	// should have its own template instance to avoid state conflicts
	var initialBuf bytes.Buffer
	err = tmpl.ExecuteUpdates(&initialBuf, state)
	if err != nil {
		log.Printf("Failed to generate initial tree: %v", err)
		return
	}
	initialJSON := initialBuf.Bytes()
	log.Printf("Sending initial tree: %s", string(initialJSON))

	err = conn.WriteMessage(websocket.TextMessage, initialJSON)
	if err != nil {
		log.Printf("Failed to send initial tree: %v", err)
		return
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse JSON message
		var msg struct {
			Action string                 `json:"action"`
			Data   map[string]interface{} `json:"data,omitempty"`
			Value  interface{}            `json:"value,omitempty"`
		}

		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		action := msg.Action
		log.Printf("Received action: %s", action)

		// Update state based on action
		switch action {
		case "increment":
			state.Counter++
		case "decrement":
			state.Counter--
		case "reset":
			state.Counter = 0
		default:
			log.Printf("Unknown action: %s", action)
			continue
		}

		// Update status and timestamp
		state.Status = getStatus(state.Counter)
		state.LastUpdated = formatTime()

		// Generate update using the shared template
		var updateBuf bytes.Buffer
		err = tmpl.ExecuteUpdates(&updateBuf, state)
		if err != nil {
			log.Printf("Template update execution failed: %v", err)
			continue
		}

		updateJSON := updateBuf.Bytes()
		log.Printf("Sending update: %s", string(updateJSON))

		// Send update to client
		err = conn.WriteMessage(websocket.TextMessage, updateJSON)
		if err != nil {
			log.Printf("WebSocket write failed: %v", err)
			break
		}
	}

	log.Printf("Client disconnected")
}

func serveFile(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		// Serve the initial HTML
		state := &CounterState{
			Title:       "Live Counter",
			Counter:     0,
			Status:      "zero",
			LastUpdated: formatTime(),
			SessionID:   fmt.Sprintf("session-%d", time.Now().Unix()),
		}

		err := tmpl.Execute(w, state)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case "/livetemplate-client.js":
		// Serve the LiveTemplate client browser bundle - try both paths
		clientPath := "../../client/dist/livetemplate-client.browser.js"
		if _, err := os.Stat(clientPath); err != nil {
			clientPath = "client/dist/livetemplate-client.browser.js"
			if _, err := os.Stat(clientPath); err != nil {
				log.Printf("Error: Could not find livetemplate-client.browser.js in any expected location")
				http.Error(w, "Client library not found", http.StatusNotFound)
				return
			}
		}
		log.Printf("Serving client library from: %s", clientPath)
		http.ServeFile(w, r, clientPath)

	default:
		http.NotFound(w, r)
	}
}

func main() {
	// Initialize template
	var err error
	tmpl = livetemplate.New("counter")

	// Try to load template from current directory first, then from project root
	templatePath := "counter.tmpl"
	_, err = tmpl.ParseFiles(templatePath)
	if err != nil {
		// Try from project root
		templatePath = "examples/counter/counter.tmpl"
		_, err = tmpl.ParseFiles(templatePath)
		if err != nil {
			log.Fatalf("Failed to parse template: %v", err)
		}
	}

	log.Println("LiveTemplate Counter Server starting...")

	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	// HTTP endpoints
	http.HandleFunc("/", serveFile)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	log.Printf("Server starting on http://localhost%s", port)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
