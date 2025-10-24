package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/livefir/livetemplate"
)

type ChatState struct {
	Messages      []Message
	Users         map[string]*User
	CurrentUser   string
	OnlineCount   int
	TotalMessages int
	mu            sync.RWMutex
}

type Message struct {
	ID        int
	Username  string
	Text      string
	Timestamp string
}

type User struct {
	Username string
	JoinedAt time.Time
	IsOnline bool
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch ctx.Action {
	case "send":
		var data struct {
			Message string `json:"message"`
		}

		if err := ctx.Bind(&data); err != nil {
			log.Printf("Failed to bind message data: %v", err)
			return nil
		}

		if data.Message == "" {
			return nil
		}

		s.TotalMessages++
		msg := Message{
			ID:        s.TotalMessages,
			Username:  s.CurrentUser,
			Text:      data.Message,
			Timestamp: time.Now().Format("15:04:05"),
		}

		s.Messages = append(s.Messages, msg)

		// Auto-broadcast handles syncing to other tabs automatically
		return nil

	case "join":
		var data struct {
			Username string `json:"username"`
		}

		if err := ctx.Bind(&data); err != nil {
			log.Printf("Failed to bind join data: %v", err)
			return nil
		}

		if data.Username == "" {
			return nil
		}

		s.CurrentUser = data.Username

		if _, exists := s.Users[data.Username]; !exists {
			s.Users[data.Username] = &User{
				Username: data.Username,
				JoinedAt: time.Now(),
				IsOnline: true,
			}
			s.updateOnlineCount()
		}

		return nil

	case "leave":
		if s.CurrentUser != "" {
			if user, exists := s.Users[s.CurrentUser]; exists {
				user.IsOnline = false
			}
			s.updateOnlineCount()
		}
		return nil
	}

	return nil
}

func (s *ChatState) updateOnlineCount() {
	count := 0
	for _, user := range s.Users {
		if user.IsOnline {
			count++
		}
	}
	s.OnlineCount = count
}

func (s *ChatState) Init() error {
	if s.Users == nil {
		s.Users = make(map[string]*User)
	}
	if s.Messages == nil {
		s.Messages = []Message{}
	}
	return nil
}

func main() {
	log.Println("chat starting...")

	// Create initial state
	state := &ChatState{
		Users:    make(map[string]*User),
		Messages: []Message{},
	}

	// Create template - uses default AnonymousAuthenticator
	// Each browser gets its own session (via cookie), tabs in same browser share state
	tmpl := livetemplate.New("chat", livetemplate.WithDevMode(true))

	// Mount handler
	http.Handle("/", tmpl.Handle(state))

	// Serve client library
	http.HandleFunc("/livetemplate-client.js", serveClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("🚀 Chat server starting on http://localhost:%s", port)
	log.Println("📝 Open multiple browser tabs to see automatic syncing")
	log.Println("💬 Messages appear instantly in all tabs of the same browser")
	log.Println("🌐 Each browser has its own isolated chat session")
	log.Println()

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func serveClientLibrary(w http.ResponseWriter, r *http.Request) {
	paths := []string{
		"livetemplate-client.js",
		"../client/dist/livetemplate-client.browser.js",
		"../../client/dist/livetemplate-client.browser.js",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(content)
			return
		}
	}

	http.Error(w, "Client library not found. For production, use CDN: https://cdn.jsdelivr.net/npm/@livefir/livetemplate-client/dist/livetemplate-client.browser.js", http.StatusNotFound)
}
