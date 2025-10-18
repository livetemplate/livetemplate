package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketManager_CreateAndClose(t *testing.T) {
	wsm := NewWebSocketManager()

	if wsm.ClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", wsm.ClientCount())
	}

	wsm.Close()

	if wsm.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after close, got %d", wsm.ClientCount())
	}
}

func TestWebSocketManager_Broadcast(t *testing.T) {
	wsm := NewWebSocketManager()
	defer wsm.Close()

	server := httptest.NewServer(http.HandlerFunc(wsm.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	if wsm.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", wsm.ClientCount())
	}

	testData := map[string]interface{}{
		"type": "test",
		"data": "hello",
	}

	wsm.Broadcast(testData)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	var received map[string]interface{}
	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if received["type"] != "test" {
		t.Errorf("Expected type=test, got %v", received["type"])
	}
	if received["data"] != "hello" {
		t.Errorf("Expected data=hello, got %v", received["data"])
	}
}

func TestWebSocketManager_MultipleClients(t *testing.T) {
	wsm := NewWebSocketManager()
	defer wsm.Close()

	server := httptest.NewServer(http.HandlerFunc(wsm.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 2: %v", err)
	}
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	if wsm.ClientCount() != 2 {
		t.Errorf("Expected 2 clients, got %d", wsm.ClientCount())
	}

	wsm.Broadcast(map[string]string{"message": "broadcast"})

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Client %d failed to read: %v", i+1, err)
		}

		var received map[string]string
		if err := json.Unmarshal(message, &received); err != nil {
			t.Fatalf("Client %d failed to unmarshal: %v", i+1, err)
		}

		if received["message"] != "broadcast" {
			t.Errorf("Client %d got wrong message: %v", i+1, received)
		}
	}
}

func TestWebSocketManager_ClientDisconnect(t *testing.T) {
	wsm := NewWebSocketManager()
	defer wsm.Close()

	server := httptest.NewServer(http.HandlerFunc(wsm.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if wsm.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", wsm.ClientCount())
	}

	conn.Close()
	time.Sleep(200 * time.Millisecond)

	if wsm.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", wsm.ClientCount())
	}
}
