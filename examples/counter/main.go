package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livefir/livetemplate"
)

type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	Status      string `json:"status"`
	LastUpdated string `json:"last_updated"`
	SessionID   string `json:"session_id"`
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

	// Update derived state
	s.Status = getStatus(s.Counter)
	s.LastUpdated = formatTime()
}

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

func serveInitialHTML(tmpl *livetemplate.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
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
		Status:      "zero",
		LastUpdated: formatTime(),
		SessionID:   "counter-example",
	}

	// Mount the live handler (handles both WebSocket and HTTP)
	http.Handle("/live", livetemplate.Mount(tmpl, state))

	// Serve initial HTML
	http.HandleFunc("/", serveInitialHTML(tmpl))

	// Serve client library
	http.HandleFunc("/livetemplate-client.js", livetemplate.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	log.Printf("Server starting on http://localhost%s", port)
	log.Printf("Live endpoint (WebSocket/HTTP): http://localhost%s/live", port)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
