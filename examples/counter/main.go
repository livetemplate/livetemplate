package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livefir/livetemplate"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

func (s *CounterState) Change(action string, data map[string]interface{}) {
	switch action {
	case "increment":
		s.Counter++
	case "decrement":
		s.Counter--
	case "reset":
		s.Counter = 0
	default:
		log.Printf("Unknown action: %s", action)
		return
	}

	s.LastUpdated = formatTime()
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	// Initialize template
	tmpl := livetemplate.New("counter")

	// Try to load template from current directory first, then from project root
	templatePath := "counter.tmpl"
	_, err := tmpl.ParseFiles(templatePath)
	if err != nil {
		// Try from project root
		templatePath = "examples/counter/counter.tmpl"
		_, err = tmpl.ParseFiles(templatePath)
		if err != nil {
			log.Fatalf("Failed to parse template: %v", err)
		}
	}

	log.Println("LiveTemplate Counter Server starting...")

	// Create initial state for the store
	state := &CounterState{
		Title:       "Live Counter",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Mount the live handler - handles initial page, WebSocket, and HTTP actions
	http.Handle("/", livetemplate.Mount(tmpl, state))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	log.Printf("Server starting on http://localhost%s", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
