package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/livefir/livetemplate"
)

// ChatState holds the chat application state
type ChatState struct {
	Messages      []Message
	Users         map[string]*User // userID -> User
	CurrentUser   string           // Current logged-in user
	OnlineCount   int
	TotalMessages int
	mu            sync.RWMutex
}

// Message represents a chat message
type Message struct {
	ID        int
	Username  string
	Text      string
	Timestamp string
	IsMine    bool // For rendering purposes
}

// User represents a connected user
type User struct {
	Username string
	JoinedAt time.Time
	IsOnline bool
}

// Change handles user actions
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

		// Create new message
		s.TotalMessages++
		msg := Message{
			ID:        s.TotalMessages,
			Username:  s.CurrentUser,
			Text:      data.Message,
			Timestamp: time.Now().Format("15:04:05"),
		}

		s.Messages = append(s.Messages, msg)

		// Broadcast to all users - this will send the update to everyone
		// The broadcasting happens automatically via the handler
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

		// Set current user
		s.CurrentUser = data.Username

		// Add user if not exists
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

// Init initializes the chat state
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
	// Create template
	tmpl := livetemplate.New("chat",
		livetemplate.WithDevMode(true),
	)

	// Parse template
	_, err := tmpl.ParseFiles("examples/chat/chat.html")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Initialize state
	initialState := &ChatState{
		Users:    make(map[string]*User),
		Messages: []Message{},
	}

	// Create handler - uses anonymous auth by default
	handler := tmpl.Handle(initialState)

	// Serve static files
	http.Handle("/", handler)

	// Start server
	port := 8090
	fmt.Printf("🚀 Chat server starting on http://localhost:%d\n", port)
	fmt.Println("📝 Open multiple browser tabs to test multi-user chat")
	fmt.Println("💬 Messages are broadcast to all connected users")
	fmt.Println()

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal(err)
	}
}
