package serve

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Client struct {
	ID         string
	conn       *websocket.Conn
	send       chan []byte
	manager    *WebSocketManager
	mu         sync.Mutex
	closedOnce sync.Once
}

type WebSocketManager struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	running    bool
	stop       chan struct{}
}

func NewWebSocketManager() *WebSocketManager {
	wsm := &WebSocketManager{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
	}
	go wsm.run()
	return wsm
}

func (wsm *WebSocketManager) run() {
	wsm.mu.Lock()
	wsm.running = true
	wsm.mu.Unlock()

	for {
		select {
		case <-wsm.stop:
			wsm.mu.Lock()
			wsm.running = false
			for client := range wsm.clients {
				client.Close()
			}
			wsm.mu.Unlock()
			return

		case client := <-wsm.register:
			wsm.mu.Lock()
			wsm.clients[client] = true
			wsm.mu.Unlock()
			log.Printf("WebSocket client connected: %s (total: %d)", client.ID, len(wsm.clients))

		case client := <-wsm.unregister:
			wsm.mu.Lock()
			if _, ok := wsm.clients[client]; ok {
				delete(wsm.clients, client)
				close(client.send)
				log.Printf("WebSocket client disconnected: %s (total: %d)", client.ID, len(wsm.clients))
			}
			wsm.mu.Unlock()

		case message := <-wsm.broadcast:
			wsm.mu.RLock()
			clients := make([]*Client, 0, len(wsm.clients))
			for client := range wsm.clients {
				clients = append(clients, client)
			}
			wsm.mu.RUnlock()

			for _, client := range clients {
				select {
				case client.send <- message:
				default:
					wsm.unregister <- client
				}
			}
		}
	}
}

func (wsm *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := generateClientID()
	client := &Client{
		ID:      clientID,
		conn:    conn,
		send:    make(chan []byte, 256),
		manager: wsm,
	}

	wsm.register <- client

	go client.writePump()
	go client.readPump()
}

func (wsm *WebSocketManager) Broadcast(data interface{}) {
	message, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal broadcast message: %v", err)
		return
	}

	wsm.broadcast <- message
}

func (wsm *WebSocketManager) Close() {
	wsm.mu.Lock()
	if !wsm.running {
		wsm.mu.Unlock()
		return
	}
	wsm.mu.Unlock()

	close(wsm.stop)
}

func (wsm *WebSocketManager) ClientCount() int {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	return len(wsm.clients)
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

func (c *Client) readPump() {
	defer func() {
		c.manager.unregister <- c
		c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to unmarshal client message: %v", err)
			continue
		}

		log.Printf("Received from client %s: %v", c.ID, msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}

			n := len(c.send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return
				}
				if _, err := w.Write(<-c.send); err != nil {
					return
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Close() {
	c.closedOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.conn.Close()
	})
}

func (c *Client) Send(data interface{}) error {
	message, err := json.Marshal(data)
	if err != nil {
		return err
	}

	select {
	case c.send <- message:
		return nil
	default:
		return nil
	}
}

var clientCounter uint64
var clientCounterMu sync.Mutex

func generateClientID() string {
	clientCounterMu.Lock()
	defer clientCounterMu.Unlock()
	clientCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune(clientCounter))
}
