package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
)

type Server struct {
	app      *livetemplate.Application
	counter  int
	upgrader websocket.Upgrader
	tmpl     *template.Template
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
	
	tmpl := template.Must(template.New("counter").Parse(`{{.Counter}}`))
	
	return &Server{
		app:     app,
		counter: 0,
		tmpl:    tmpl,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Counter": s.counter,
	}

	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
	}
	
	page, err := s.app.NewApplicationPage(s.tmpl, initialData)
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
		case "decrement":
			s.counter--
		default:
			log.Printf("Unknown action: %s", msg.Action)
			continue
		}

		log.Printf("Counter updated from %d to %d", oldValue, s.counter)

		// Generate fragments using proper LiveTemplate API
		newData := map[string]any{"Counter": s.counter}
		log.Printf("Generating fragments with new data: %+v", newData)

		fragments, err := page.RenderFragments(context.Background(), newData)
		if err != nil {
			log.Printf("Error rendering fragments: %v", err)
			continue
		}
		

		log.Printf("Generated %d fragments", len(fragments))
		for i, frag := range fragments {
			log.Printf("Fragment %d: ID=%s, Strategy=%s, Data=%+v", i, frag.ID, frag.Strategy, frag.Data)
			
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