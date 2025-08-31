package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
)

// Tweet represents a tweet in our Twitter clone
type Tweet struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Handle    string    `json:"handle"`
	Content   string    `json:"content"`
	Likes     int       `json:"likes"`
	Retweets  int       `json:"retweets"`
	Timestamp time.Time `json:"timestamp"`
	Liked     bool      `json:"liked"`
	Retweeted bool      `json:"retweeted"`
}

// User represents a user in our Twitter clone
type User struct {
	Username  string `json:"username"`
	Handle    string `json:"handle"`
	Followers int    `json:"followers"`
	Following int    `json:"following"`
}

// AppData represents the complete application state including UI state
type AppData struct {
	CurrentUser User    `json:"current_user"`
	Tweets      []Tweet `json:"tweets"`
	Users       []User  `json:"users"`

	// UI State for fragment-driven interactions
	ComposerText      string `json:"composer_text"`
	CharCount         string `json:"char_count"`
	TweetDisabled     bool   `json:"tweet_disabled"`
	TweetPosting      bool   `json:"tweet_posting"`
	ConnectionStatus  string `json:"connection_status"`
	ConnectionMessage string `json:"connection_message"`
}

// TemplateData represents data passed to templates including LiveTemplate metadata
type TemplateData struct {
	AppData
	PageToken string `json:"page_token"`
}

// Server holds the application state and LiveTemplate integration
type Server struct {
	app       *livetemplate.Application
	pages     map[string]*livetemplate.ApplicationPage
	pagesMu   sync.RWMutex
	data      *AppData
	dataMu    sync.RWMutex
	upgrader  websocket.Upgrader
	tweetID   int64
	templates *template.Template
}

// WebSocketMessage represents messages sent over WebSocket
type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ActionMessage represents user actions sent from client
type ActionMessage struct {
	Action  string                 `json:"action"`
	Payload map[string]interface{} `json:"payload"`
	Token   string                 `json:"token"`
}

func NewServer() (*Server, error) {
	// Create LiveTemplate application
	app, err := livetemplate.NewApplication()
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	// Parse templates
	templates := template.Must(template.ParseGlob("templates/*.html"))

	// Initialize with sample data including UI state
	initialData := &AppData{
		CurrentUser: User{
			Username:  "John Doe",
			Handle:    "@johndoe",
			Followers: 1234,
			Following: 567,
		},
		Tweets: []Tweet{
			{
				ID:        1,
				Username:  "Alice Smith",
				Handle:    "@alice",
				Content:   "Just built an amazing web app with LiveTemplate! 🚀",
				Likes:     42,
				Retweets:  12,
				Timestamp: time.Now().Add(-2 * time.Hour),
				Liked:     false,
				Retweeted: false,
			},
			{
				ID:        2,
				Username:  "Bob Wilson",
				Handle:    "@bobw",
				Content:   "Real-time updates without the complexity. This is the future of web development!",
				Likes:     28,
				Retweets:  8,
				Timestamp: time.Now().Add(-4 * time.Hour),
				Liked:     true,
				Retweeted: false,
			},
			{
				ID:        3,
				Username:  "Carol Johnson",
				Handle:    "@carol_j",
				Content:   "LiveTemplate makes building interactive UIs so much easier. Great work team! 👏",
				Likes:     156,
				Retweets:  34,
				Timestamp: time.Now().Add(-6 * time.Hour),
				Liked:     false,
				Retweeted: true,
			},
		},
		Users: []User{
			{Username: "Alice Smith", Handle: "@alice", Followers: 892, Following: 234},
			{Username: "Bob Wilson", Handle: "@bobw", Followers: 445, Following: 189},
			{Username: "Carol Johnson", Handle: "@carol_j", Followers: 2341, Following: 456},
		},
		// Initialize UI state for fragment-driven interactions
		ComposerText:      "",
		CharCount:         "280",
		TweetDisabled:     true,
		TweetPosting:      false,
		ConnectionStatus:  "connecting",
		ConnectionMessage: "Connecting...",
	}

	server := &Server{
		app:       app,
		pages:     make(map[string]*livetemplate.ApplicationPage),
		data:      initialData,
		templates: templates,
		tweetID:   4,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for demo
			},
		},
	}

	return server, nil
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	log.Printf("[SERVER] Handling root request from %s", r.RemoteAddr)

	// Create a new page with current data
	s.dataMu.RLock()
	currentData := *s.data // Create a copy
	s.dataMu.RUnlock()

	page, err := s.app.NewApplicationPage(s.templates.Lookup("index.html"), currentData)
	if err != nil {
		log.Printf("[SERVER] Error creating page: %v", err)
		http.Error(w, "Failed to create page", http.StatusInternalServerError)
		return
	}

	// Store the page for later fragment generation
	token := page.GetToken()
	s.pagesMu.Lock()
	s.pages[token] = page
	s.pagesMu.Unlock()

	log.Printf("[SERVER] Created page with token: %s", token)

	// Render initial HTML with fragment annotations
	html, err := page.Render()
	if err != nil {
		log.Printf("[SERVER] Error rendering page: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}

	log.Printf("[SERVER] Rendered initial HTML (%d bytes) with fragment annotations", len(html))

	// Inject the page token into the HTML
	tokenScript := fmt.Sprintf(`<script>window.LIVETEMPLATE_TOKEN = '%s';</script>`, token)
	html = strings.Replace(html, "</head>", tokenScript+"</head>", 1)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[SERVER] WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[SERVER] WebSocket connection established with %s", r.RemoteAddr)

	// Handle WebSocket connection
	for {
		var msg ActionMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("[SERVER] WebSocket read error: %v", err)
			break
		}

		log.Printf("[SERVER] WebSocket received action: %s", msg.Action)

		switch msg.Action {
		case "connect":
			// Send initial fragments for caching and update connection status
			token := msg.Token
			s.updateConnectionStatus(token, "connected", "WebSocket Connected")
			s.handleInitialFragments(conn, token)

		case "input":
			// Handle real-time input for composer (character counting, button state)
			s.handleComposerInput(conn, msg)

		case "like":
			s.handleLikeTweet(conn, msg)

		case "retweet":
			s.handleRetweet(conn, msg)

		case "tweet":
			s.handleNewTweet(conn, msg)
		}
	}

	log.Printf("[SERVER] WebSocket connection closed with %s", r.RemoteAddr)
}

func (s *Server) updateConnectionStatus(token, status, message string) {
	s.pagesMu.RLock()
	page, exists := s.pages[token]
	s.pagesMu.RUnlock()

	if !exists {
		return
	}

	// Update UI state
	s.dataMu.Lock()
	s.data.ConnectionStatus = status
	s.data.ConnectionMessage = message
	currentData := *s.data
	s.dataMu.Unlock()

	// Generate fragment update for connection status
	_, err := page.RenderFragments(context.Background(), currentData)
	if err == nil {
		log.Printf("[SERVER] Updated connection status: %s", message)
	}
}

func (s *Server) handleComposerInput(conn *websocket.Conn, msg ActionMessage) {
	value, ok := msg.Payload["value"].(string)
	if !ok {
		return
	}

	log.Printf("[SERVER] Handling composer input: '%s' (%d chars)", value, len(value))

	// Update composer state (this is the UI logic that was in client!)
	s.dataMu.Lock()
	s.data.ComposerText = value
	remaining := 280 - len(value)
	if remaining < 0 {
		s.data.CharCount = fmt.Sprintf("%d", remaining)
	} else {
		s.data.CharCount = fmt.Sprintf("%d", remaining)
	}
	s.data.TweetDisabled = len(strings.TrimSpace(value)) == 0 || len(value) > 280
	currentData := *s.data
	s.dataMu.Unlock()

	// Generate fragment updates for character count and button state
	s.sendFragmentUpdate(conn, msg.Token, currentData, "composer_update")
}

func (s *Server) handleInitialFragments(conn *websocket.Conn, token string) {
	log.Printf("[SERVER] Sending initial fragments for token: %s", token)

	s.pagesMu.RLock()
	page, exists := s.pages[token]
	s.pagesMu.RUnlock()

	if !exists {
		log.Printf("[SERVER] Page not found for token: %s", token)
		conn.WriteJSON(WebSocketMessage{Type: "error", Data: "Page not found"})
		return
	}

	// Get current data and render fragments for caching
	s.dataMu.RLock()
	currentData := *s.data
	s.dataMu.RUnlock()

	fragments, err := page.RenderFragments(context.Background(), currentData)
	if err != nil {
		log.Printf("[SERVER] Error rendering initial fragments: %v", err)
		conn.WriteJSON(WebSocketMessage{Type: "error", Data: "Failed to render fragments"})
		return
	}

	log.Printf("[SERVER] Generated %d initial fragments with static/dynamic caching data", len(fragments))

	// Send fragments for client caching
	conn.WriteJSON(WebSocketMessage{
		Type: "initial_fragments",
		Data: fragments,
	})
}

func (s *Server) handleLikeTweet(conn *websocket.Conn, msg ActionMessage) {
	tweetIDStr, ok := msg.Payload["tweet_id"].(string)
	if !ok {
		log.Printf("[SERVER] Invalid tweet_id in like action")
		return
	}

	tweetID, err := strconv.Atoi(tweetIDStr)
	if err != nil {
		log.Printf("[SERVER] Invalid tweet_id format: %v", err)
		return
	}

	log.Printf("[SERVER] Handling like action for tweet %d (server-driven UI)", tweetID)

	// Update data
	s.dataMu.Lock()
	for i := range s.data.Tweets {
		if s.data.Tweets[i].ID == tweetID {
			if s.data.Tweets[i].Liked {
				s.data.Tweets[i].Likes--
				s.data.Tweets[i].Liked = false
				log.Printf("[SERVER] Tweet %d unliked (likes: %d)", tweetID, s.data.Tweets[i].Likes)
			} else {
				s.data.Tweets[i].Likes++
				s.data.Tweets[i].Liked = true
				log.Printf("[SERVER] Tweet %d liked (likes: %d)", tweetID, s.data.Tweets[i].Likes)
			}
			break
		}
	}
	currentData := *s.data
	s.dataMu.Unlock()

	// Generate and send dynamic-only fragments
	s.sendFragmentUpdate(conn, msg.Token, currentData, "like")
}

func (s *Server) handleRetweet(conn *websocket.Conn, msg ActionMessage) {
	tweetIDStr, ok := msg.Payload["tweet_id"].(string)
	if !ok {
		log.Printf("[SERVER] Invalid tweet_id in retweet action")
		return
	}

	tweetID, err := strconv.Atoi(tweetIDStr)
	if err != nil {
		log.Printf("[SERVER] Invalid tweet_id format: %v", err)
		return
	}

	log.Printf("[SERVER] Handling retweet action for tweet %d", tweetID)

	// Update data
	s.dataMu.Lock()
	for i := range s.data.Tweets {
		if s.data.Tweets[i].ID == tweetID {
			if s.data.Tweets[i].Retweeted {
				s.data.Tweets[i].Retweets--
				s.data.Tweets[i].Retweeted = false
				log.Printf("[SERVER] Tweet %d unretweeted (retweets: %d)", tweetID, s.data.Tweets[i].Retweets)
			} else {
				s.data.Tweets[i].Retweets++
				s.data.Tweets[i].Retweeted = true
				log.Printf("[SERVER] Tweet %d retweeted (retweets: %d)", tweetID, s.data.Tweets[i].Retweets)
			}
			break
		}
	}
	currentData := *s.data
	s.dataMu.Unlock()

	// Generate and send dynamic-only fragments
	s.sendFragmentUpdate(conn, msg.Token, currentData, "retweet")
}

func (s *Server) handleNewTweet(conn *websocket.Conn, msg ActionMessage) {
	// Get content from composer (server has the current state!)
	s.dataMu.RLock()
	content := strings.TrimSpace(s.data.ComposerText)
	s.dataMu.RUnlock()

	if content == "" || len(content) > 280 {
		log.Printf("[SERVER] Invalid tweet content length: %d", len(content))
		return
	}

	log.Printf("[SERVER] Handling new tweet: %s", content)

	// Set posting state (UI feedback via fragments!)
	s.dataMu.Lock()
	s.data.TweetPosting = true
	s.data.TweetDisabled = true
	postingData := *s.data
	s.dataMu.Unlock()

	// Send posting state to client via fragments
	s.sendFragmentUpdate(conn, msg.Token, postingData, "tweet_posting")

	// Simulate posting delay
	time.Sleep(500 * time.Millisecond)

	// Create new tweet
	newTweet := Tweet{
		ID:        int(atomic.AddInt64(&s.tweetID, 1)),
		Username:  s.data.CurrentUser.Username,
		Handle:    s.data.CurrentUser.Handle,
		Content:   content,
		Likes:     0,
		Retweets:  0,
		Timestamp: time.Now(),
		Liked:     false,
		Retweeted: false,
	}

	// Update data and reset composer (all UI state managed server-side!)
	s.dataMu.Lock()
	s.data.Tweets = append([]Tweet{newTweet}, s.data.Tweets...)
	s.data.ComposerText = ""
	s.data.CharCount = "280"
	s.data.TweetDisabled = true
	s.data.TweetPosting = false
	currentData := *s.data
	s.dataMu.Unlock()

	log.Printf("[SERVER] Added new tweet with ID %d", newTweet.ID)

	// Generate and send dynamic-only fragments (includes reset composer)
	s.sendFragmentUpdate(conn, msg.Token, currentData, "new_tweet")
}

func (s *Server) sendFragmentUpdate(conn *websocket.Conn, token string, data AppData, action string) {
	s.pagesMu.RLock()
	page, exists := s.pages[token]
	s.pagesMu.RUnlock()

	if !exists {
		log.Printf("[SERVER] Page not found for token: %s", token)
		conn.WriteJSON(WebSocketMessage{Type: "error", Data: "Page not found"})
		return
	}

	// Generate dynamic-only fragments
	fragments, err := page.RenderFragments(context.Background(), data)
	if err != nil {
		log.Printf("[SERVER] Error rendering fragments for %s: %v", action, err)
		conn.WriteJSON(WebSocketMessage{Type: "error", Data: "Failed to render fragments"})
		return
	}

	log.Printf("[SERVER] Generated %d dynamic-only fragments for %s action", len(fragments), action)

	// Send dynamic updates
	conn.WriteJSON(WebSocketMessage{
		Type: "fragments",
		Data: fragments,
	})
}

// Ajax fallback handlers
func (s *Server) handleAjaxFragments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.Header.Get("X-Page-Token")
	if token == "" {
		http.Error(w, "Missing page token", http.StatusBadRequest)
		return
	}

	log.Printf("[SERVER] Ajax fragments request for token: %s", token)

	s.pagesMu.RLock()
	page, exists := s.pages[token]
	s.pagesMu.RUnlock()

	if !exists {
		log.Printf("[SERVER] Page not found for token: %s", token)
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	// Check if this is initial cache request
	cacheEmpty := r.Header.Get("X-Cache-Empty") == "true"

	s.dataMu.RLock()
	currentData := *s.data
	s.dataMu.RUnlock()

	fragments, err := page.RenderFragments(context.Background(), currentData)
	if err != nil {
		log.Printf("[SERVER] Error rendering Ajax fragments: %v", err)
		http.Error(w, "Failed to render fragments", http.StatusInternalServerError)
		return
	}

	if cacheEmpty {
		log.Printf("[SERVER] Sent %d initial Ajax fragments for caching", len(fragments))
	} else {
		log.Printf("[SERVER] Sent %d dynamic-only Ajax fragments", len(fragments))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fragments)
}

func (s *Server) handleAjaxAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg ActionMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Printf("[SERVER] Error decoding Ajax action: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[SERVER] Ajax action received: %s", msg.Action)

	// Process action (similar to WebSocket but respond via HTTP)
	switch msg.Action {
	case "like_tweet":
		s.processLikeTweet(msg)
	case "retweet":
		s.processRetweet(msg)
	case "new_tweet":
		s.processNewTweet(msg)
	}

	// Return fragments via Ajax
	s.pagesMu.RLock()
	page, exists := s.pages[msg.Token]
	s.pagesMu.RUnlock()

	if !exists {
		log.Printf("[SERVER] Page not found for token: %s", msg.Token)
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	s.dataMu.RLock()
	currentData := *s.data
	s.dataMu.RUnlock()

	fragments, err := page.RenderFragments(context.Background(), currentData)
	if err != nil {
		log.Printf("[SERVER] Error rendering Ajax response fragments: %v", err)
		http.Error(w, "Failed to render fragments", http.StatusInternalServerError)
		return
	}

	log.Printf("[SERVER] Ajax action response: %d fragments", len(fragments))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fragments)
}

// Process methods for Ajax (reuse logic from WebSocket handlers)
func (s *Server) processLikeTweet(msg ActionMessage) {
	tweetIDStr, ok := msg.Payload["tweet_id"].(string)
	if !ok {
		return
	}

	tweetID, err := strconv.Atoi(tweetIDStr)
	if err != nil {
		return
	}

	s.dataMu.Lock()
	for i := range s.data.Tweets {
		if s.data.Tweets[i].ID == tweetID {
			if s.data.Tweets[i].Liked {
				s.data.Tweets[i].Likes--
				s.data.Tweets[i].Liked = false
			} else {
				s.data.Tweets[i].Likes++
				s.data.Tweets[i].Liked = true
			}
			break
		}
	}
	s.dataMu.Unlock()
}

func (s *Server) processRetweet(msg ActionMessage) {
	tweetIDStr, ok := msg.Payload["tweet_id"].(string)
	if !ok {
		return
	}

	tweetID, err := strconv.Atoi(tweetIDStr)
	if err != nil {
		return
	}

	s.dataMu.Lock()
	for i := range s.data.Tweets {
		if s.data.Tweets[i].ID == tweetID {
			if s.data.Tweets[i].Retweeted {
				s.data.Tweets[i].Retweets--
				s.data.Tweets[i].Retweeted = false
			} else {
				s.data.Tweets[i].Retweets++
				s.data.Tweets[i].Retweeted = true
			}
			break
		}
	}
	s.dataMu.Unlock()
}

func (s *Server) processNewTweet(msg ActionMessage) {
	content, ok := msg.Payload["content"].(string)
	if !ok || content == "" {
		return
	}

	newTweet := Tweet{
		ID:        int(atomic.AddInt64(&s.tweetID, 1)),
		Username:  s.data.CurrentUser.Username,
		Handle:    s.data.CurrentUser.Handle,
		Content:   content,
		Likes:     0,
		Retweets:  0,
		Timestamp: time.Now(),
		Liked:     false,
		Retweeted: false,
	}

	s.dataMu.Lock()
	s.data.Tweets = append([]Tweet{newTweet}, s.data.Tweets...)
	s.dataMu.Unlock()
}

func (s *Server) Start() error {
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	// Routes
	http.HandleFunc("/", s.handleRoot)
	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/api/fragments", s.handleAjaxFragments)
	http.HandleFunc("/api/action", s.handleAjaxAction)

	log.Println("[SERVER] Starting Twitter clone demo server on :8080")
	log.Println("[SERVER] Visit http://localhost:8080 to see the demo")
	log.Println("[SERVER] Server logs will show LiveTemplate fragment lifecycle")

	return http.ListenAndServe(":8080", nil)
}

func (s *Server) Close() {
	if s.app != nil {
		s.app.Close()
	}
}

func main() {
	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
