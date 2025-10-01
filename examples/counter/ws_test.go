package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketBasic(t *testing.T) {
	// Get a free port
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	portStr := fmt.Sprintf("%d", port)
	serverURL := fmt.Sprintf("http://localhost:%s", portStr)
	wsURL := fmt.Sprintf("ws://localhost:%s/", portStr)

	// Start server on dynamic port
	cmd := exec.Command("go", "run", "main.go")
	cmd.Env = append([]string{"PORT=" + portStr}, cmd.Environ()...)

	serverLogs := &bytes.Buffer{}
	cmd.Stdout = serverLogs
	cmd.Stderr = serverLogs

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		t.Logf("=== SERVER LOGS ===\n%s", serverLogs.String())
	}()

	// Wait for server
	time.Sleep(2 * time.Second)
	for i := 0; i < 30; i++ {
		if resp, err := http.Get(serverURL); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("Server is up, trying to connect WebSocket...")

	// Try to connect
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v, response: %v", err, resp)
	}
	defer conn.Close()

	t.Log("WebSocket connected successfully!")

	// Read first message (initial tree)
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	t.Logf("Received message, length: %d bytes", len(msg))
	t.Logf("First 100 bytes: %s", msg[:min(100, len(msg))])

	// Send increment action
	t.Log("Sending increment action...")
	action := []byte(`{"action":"increment","data":{}}`)
	if err := conn.WriteMessage(websocket.TextMessage, action); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read response
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	t.Logf("Received response, length: %d bytes", len(msg))
	t.Logf("Response: %s", msg)

	t.Log("✅ WebSocket test passed!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
