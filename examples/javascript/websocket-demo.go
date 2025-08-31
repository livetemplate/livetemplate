package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// LiveTemplate-style structures
type Fragment struct {
	ID       string      `json:"id"`
	Strategy string      `json:"strategy"`
	Action   string      `json:"action"`
	Data     interface{} `json:"data"`
	Size     int         `json:"size"`
	Type     string      `json:"type"` // "initial" or "update"
}

type NetworkMessage struct {
	Type      string      `json:"type"`      // "fragment"
	Timestamp time.Time   `json:"timestamp"`
	Fragment  *Fragment   `json:"fragment,omitempty"`
}


// Demo data structures
type UserDashboard struct {
	Name   string `json:"name"`
	Level  string `json:"level"`
	Score  int    `json:"score"`
	Status string `json:"status"`
	Avatar string `json:"avatar"`
}

type ProductCatalog struct {
	Products []Product `json:"products"`
}

type Product struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
	Stock int    `json:"stock"`
}

type LiveChat struct {
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	User      string    `json:"user"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

// Demo state
type DemoState struct {
	User         UserDashboard
	Products     ProductCatalog
	Chat         LiveChat
	UpdateCount  int
	TotalBytes   int
	SavedBytes   int
	Connections  map[*websocket.Conn]bool
}

var demoState = &DemoState{
	User: UserDashboard{
		Name:   "Alice Johnson",
		Level:  "Gold",
		Score:  1250,
		Status: "Online",
		Avatar: "👤",
	},
	Products: ProductCatalog{
		Products: []Product{
			{"Gaming Laptop", 1299, 15},
			{"Wireless Mouse", 79, 42},
			{"Mechanical Keyboard", 189, 28},
		},
	},
	Chat: LiveChat{
		Messages: []ChatMessage{
			{"System", "Welcome to LiveTemplate Demo!", time.Now()},
		},
	},
	Connections: make(map[*websocket.Conn]bool),
}

func main() {
	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("./")))
	
	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)
	
	// Start demo data updater
	go startDemoUpdates()
	
	fmt.Println("🚀 LiveTemplate WebSocket Demo Server starting...")
	fmt.Println("📡 WebSocket endpoint: ws://localhost:8080/ws")
	fmt.Println("🌐 Demo page: http://localhost:8080/websocket-demo.html")
	fmt.Println("Press Ctrl+C to stop")
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Add connection to demo state
	demoState.Connections[conn] = true
	defer delete(demoState.Connections, conn)

	log.Printf("New WebSocket connection from %s", r.RemoteAddr)
	
	// Send initial fragments
	sendInitialFragments(conn)
	
	// Handle incoming messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Read error: %v", err)
			break
		}
		
		handleClientMessage(conn, msg)
	}
}

func sendInitialFragments(conn *websocket.Conn) {
	log.Printf("🌐 Initial page load - sending complete template structures")
	
	// User Dashboard - Initial render with static structure
	userTreeData := generateUserDashboardFragment(true)
	userFragment := &Fragment{
		ID:       "user-dashboard",
		Strategy: "tree_based",
		Action:   "initial_render",
		Data:     userTreeData,
		Size:     len(fmt.Sprintf("%v", userTreeData)),
		Type:     "initial",
	}
	
	sendFragment(conn, userFragment)
	log.Printf("📦 User dashboard: %d bytes (includes static HTML structure)", userFragment.Size)
	
	// Product Catalog
	prodTreeData := generateProductCatalogFragment(true)
	prodFragment := &Fragment{
		ID:       "product-catalog", 
		Strategy: "tree_based",
		Action:   "initial_render",
		Data:     prodTreeData,
		Size:     len(fmt.Sprintf("%v", prodTreeData)),
		Type:     "initial",
	}
	
	sendFragment(conn, prodFragment)
	log.Printf("📦 Product catalog: %d bytes (includes static HTML structure)", prodFragment.Size)
	
	// Live Chat
	chatTreeData := generateLiveChatFragment(true)
	chatFragment := &Fragment{
		ID:       "live-chat",
		Strategy: "tree_based", 
		Action:   "initial_render",
		Data:     chatTreeData,
		Size:     len(fmt.Sprintf("%v", chatTreeData)),
		Type:     "initial",
	}
	
	sendFragment(conn, chatFragment)
	log.Printf("📦 Live chat: %d bytes (includes static HTML structure)", chatFragment.Size)
	
	totalInitial := userFragment.Size + prodFragment.Size + chatFragment.Size
	demoState.TotalBytes += totalInitial
	
	log.Printf("💾 Client cached static structures, total initial: %d bytes", totalInitial)
}

func handleClientMessage(conn *websocket.Conn, msg map[string]interface{}) {
	action, ok := msg["action"].(string)
	if !ok {
		return
	}
	
	switch action {
	case "update_user":
		handleUserUpdate(conn)
	case "update_products":
		handleProductUpdate(conn)
	case "add_chat_message":
		handleChatUpdate(conn)
	case "simulate_realtime":
		handleRealtimeSimulation(conn, msg)
	}
}

func handleUserUpdate(conn *websocket.Conn) {
	// Update user data
	names := []string{"Bob Wilson", "Carol Smith", "David Chen", "Emma Davis"}
	levels := []string{"Bronze", "Silver", "Gold", "Platinum", "Diamond"}
	statuses := []string{"Online", "Away", "Busy", "Playing"}
	
	demoState.User.Name = names[rand.Intn(len(names))]
	demoState.User.Level = levels[rand.Intn(len(levels))]
	demoState.User.Score = 500 + rand.Intn(2000)
	demoState.User.Status = statuses[rand.Intn(len(statuses))]
	demoState.UpdateCount++
	
	log.Printf("⚡ User update - sending dynamics only (statics cached client-side)")
	
	// Generate update fragment (dynamics only)
	treeData := generateUserDashboardFragment(false)
	fragment := &Fragment{
		ID:       "user-dashboard",
		Strategy: "tree_based",
		Action:   "update",
		Data:     treeData,
		Size:     len(fmt.Sprintf("%v", treeData)),
		Type:     "update",
	}
	
	sendFragment(conn, fragment)
	
	// Calculate savings
	initialSize := 250 // Approximate full template size
	saved := initialSize - fragment.Size
	demoState.TotalBytes += fragment.Size
	demoState.SavedBytes += saved
	
	savingsPercent := float64(saved) / float64(initialSize) * 100
	
	log.Printf("📈 Update sent: %d bytes vs %d bytes traditional (%.1f%% savings)", 
		fragment.Size, initialSize, savingsPercent)
}

func handleProductUpdate(conn *websocket.Conn) {
	// Update product prices and stock
	for i := range demoState.Products.Products {
		priceChange := rand.Intn(100) - 50
		demoState.Products.Products[i].Price += priceChange
		if demoState.Products.Products[i].Price < 10 {
			demoState.Products.Products[i].Price = 10
		}
		
		stockChange := rand.Intn(10) - 5
		demoState.Products.Products[i].Stock += stockChange
		if demoState.Products.Products[i].Stock < 0 {
			demoState.Products.Products[i].Stock = 0
		}
	}
	demoState.UpdateCount++
	
	log.Printf("⚡ Product update - sending dynamics only (statics cached)")
	
	treeData := generateProductCatalogFragment(false)
	fragment := &Fragment{
		ID:       "product-catalog",
		Strategy: "tree_based", 
		Action:   "update",
		Data:     treeData,
		Size:     len(fmt.Sprintf("%v", treeData)),
		Type:     "update",
	}
	
	sendFragment(conn, fragment)
	
	initialSize := 400 // Approximate full template size
	saved := initialSize - fragment.Size
	demoState.TotalBytes += fragment.Size
	demoState.SavedBytes += saved
	
	savingsPercent := float64(saved) / float64(initialSize) * 100
	log.Printf("📈 Update sent: %d bytes vs %d bytes traditional (%.1f%% savings)",
		fragment.Size, initialSize, savingsPercent)
}

func handleChatUpdate(conn *websocket.Conn) {
	messages := []string{
		"Check out the bandwidth savings!",
		"LiveTemplate is amazing!",
		"Look at those tiny update sizes!",
		"Real-time updates with minimal data!",
		"This beats traditional approaches!",
	}
	
	users := []string{"Developer", "User123", "TestUser", "Demo"}
	
	newMsg := ChatMessage{
		User:      users[rand.Intn(len(users))],
		Message:   messages[rand.Intn(len(messages))],
		Timestamp: time.Now(),
	}
	
	demoState.Chat.Messages = append(demoState.Chat.Messages, newMsg)
	// Keep only last 5 messages
	if len(demoState.Chat.Messages) > 5 {
		demoState.Chat.Messages = demoState.Chat.Messages[1:]
	}
	
	demoState.UpdateCount++
	
	log.Printf("⚡ Chat update - sending dynamics only (statics cached)")
	
	treeData := generateLiveChatFragment(false)
	fragment := &Fragment{
		ID:       "live-chat",
		Strategy: "tree_based",
		Action:   "update", 
		Data:     treeData,
		Size:     len(fmt.Sprintf("%v", treeData)),
		Type:     "update",
	}
	
	sendFragment(conn, fragment)
	
	initialSize := 300
	saved := initialSize - fragment.Size
	demoState.TotalBytes += fragment.Size
	demoState.SavedBytes += saved
	
	savingsPercent := float64(saved) / float64(initialSize) * 100
	log.Printf("📈 Update sent: %d bytes vs %d bytes traditional (%.1f%% savings)",
		fragment.Size, initialSize, savingsPercent)
}

func handleRealtimeSimulation(conn *websocket.Conn, msg map[string]interface{}) {
	duration, _ := msg["duration"].(float64)
	if duration == 0 {
		duration = 30 // Default 30 seconds
	}
	
	log.Printf("🎭 Starting realtime simulation for %.0f seconds", duration)
	
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	end := time.Now().Add(time.Duration(duration) * time.Second)
	
	go func() {
		for time.Now().Before(end) {
			select {
			case <-ticker.C:
				// Randomly choose which component to update
				switch rand.Intn(3) {
				case 0:
					handleUserUpdate(conn)
				case 1:
					handleProductUpdate(conn)
				case 2:
					handleChatUpdate(conn)
				}
			}
		}
		log.Printf("✅ Realtime simulation completed")
	}()
}

// Tree-based fragment generation (LiveTemplate-style)
func generateUserDashboardFragment(isInitial bool) map[string]interface{} {
	if isInitial {
		// Return complete structure with statics
		return map[string]interface{}{
			"s": []string{
				`<div class="user-dashboard"><div class="user-header"><span class="avatar">`,
				`</span><h3>Welcome `,
				`!</h3></div><div class="user-stats"><div class="stat"><label>Level:</label><span class="level">`,
				`</span></div><div class="stat"><label>Score:</label><span class="score">`,
				`</span></div><div class="stat"><label>Status:</label><span class="status `,
				`">`,
				`</span></div></div></div>`,
			},
			"0": demoState.User.Avatar,
			"1": demoState.User.Name,
			"2": demoState.User.Level,
			"3": strconv.Itoa(demoState.User.Score),
			"4": demoState.User.Status,
			"5": demoState.User.Status,
		}
	} else {
		// Return only dynamic values
		return map[string]interface{}{
			"0": demoState.User.Avatar,
			"1": demoState.User.Name,
			"2": demoState.User.Level,
			"3": strconv.Itoa(demoState.User.Score),
			"4": demoState.User.Status,
			"5": demoState.User.Status,
		}
	}
}

func generateProductCatalogFragment(isInitial bool) map[string]interface{} {
	if isInitial {
		products := make([]map[string]interface{}, len(demoState.Products.Products))
		for i, p := range demoState.Products.Products {
			products[i] = map[string]interface{}{
				"s": []string{
					`<div class="product"><span class="name">`,
					`</span><span class="price">$`,
					`</span><span class="stock">Stock: `,
					`</span></div>`,
				},
				"0": p.Name,
				"1": strconv.Itoa(p.Price),
				"2": strconv.Itoa(p.Stock),
			}
		}
		
		return map[string]interface{}{
			"s": []string{`<div class="product-catalog"><h4>Products</h4>`, `</div>`},
			"0": products,
		}
	} else {
		products := make([]map[string]interface{}, len(demoState.Products.Products))
		for i, p := range demoState.Products.Products {
			products[i] = map[string]interface{}{
				"0": p.Name,
				"1": strconv.Itoa(p.Price),
				"2": strconv.Itoa(p.Stock),
			}
		}
		
		return map[string]interface{}{
			"0": products,
		}
	}
}

func generateLiveChatFragment(isInitial bool) map[string]interface{} {
	if isInitial {
		messages := make([]map[string]interface{}, len(demoState.Chat.Messages))
		for i, m := range demoState.Chat.Messages {
			messages[i] = map[string]interface{}{
				"s": []string{
					`<div class="message"><span class="user">`,
					`:</span><span class="text">`,
					`</span><span class="time">`,
					`</span></div>`,
				},
				"0": m.User,
				"1": m.Message,
				"2": m.Timestamp.Format("15:04"),
			}
		}
		
		return map[string]interface{}{
			"s": []string{`<div class="live-chat"><h4>Live Chat</h4><div class="messages">`, `</div></div>`},
			"0": messages,
		}
	} else {
		messages := make([]map[string]interface{}, len(demoState.Chat.Messages))
		for i, m := range demoState.Chat.Messages {
			messages[i] = map[string]interface{}{
				"0": m.User,
				"1": m.Message,
				"2": m.Timestamp.Format("15:04"),
			}
		}
		
		return map[string]interface{}{
			"0": messages,
		}
	}
}

func sendFragment(conn *websocket.Conn, fragment *Fragment) {
	msg := NetworkMessage{
		Type:      "fragment",
		Timestamp: time.Now(),
		Fragment:  fragment,
	}
	
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Write error: %v", err)
	}
}

func startDemoUpdates() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if len(demoState.Connections) == 0 {
			continue
		}
		
		// Send periodic updates to all connections
		for conn := range demoState.Connections {
			go func(c *websocket.Conn) {
				switch rand.Intn(3) {
				case 0:
					handleUserUpdate(c)
				case 1:
					handleProductUpdate(c)
				case 2:
					handleChatUpdate(c)
				}
			}(conn)
		}
	}
}