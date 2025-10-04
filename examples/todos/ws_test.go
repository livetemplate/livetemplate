package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

func TestWebSocketBasic(t *testing.T) {
	// Get a free port
	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	portStr := fmt.Sprintf("%d", port)
	serverURL := fmt.Sprintf("http://localhost:%s", portStr)
	wsURL := fmt.Sprintf("ws://localhost:%s/", portStr)

	// Start server on dynamic port
	cmd := exec.Command("go", "run", "main.go", "db_manager.go")
	cmd.Env = append([]string{"PORT=" + portStr, "TEST_MODE=1"}, cmd.Environ()...)

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

	t.Logf("Received initial message, length: %d bytes", len(msg))
	t.Logf("Initial message: %s", string(msg))

	// Verify initial state
	if !strings.Contains(string(msg), "Todo App") {
		t.Error("Initial message should contain 'Todo App'")
	}

	// Send add action
	t.Log("Sending add todo action...")
	addAction := map[string]interface{}{
		"action": "add",
		"data": map[string]interface{}{
			"text": "Test Todo Item",
		},
	}
	addJSON, _ := json.Marshal(addAction)

	if err := conn.WriteMessage(websocket.TextMessage, addJSON); err != nil {
		t.Fatalf("Failed to send add action: %v", err)
	}

	// Read add response with timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		// Print server logs before failing
		time.Sleep(500 * time.Millisecond)
		t.Fatalf("Failed to read add response: %v\nServer logs:\n%s", err, serverLogs.String())
	}

	t.Logf("Received add response, length: %d bytes", len(msg))
	t.Logf("Response: %s", msg)

	// Verify the response contains the todo
	if !strings.Contains(string(msg), "Test Todo Item") {
		t.Error("Add response should contain the new todo item")
	}

	// Extract todo ID from response for toggle test
	// The response should contain data-key="todo-..."
	var todoID string
	msgStr := string(msg)
	if idx := strings.Index(msgStr, `data-key="`); idx != -1 {
		start := idx + len(`data-key="`)
		end := strings.Index(msgStr[start:], `"`)
		if end != -1 {
			todoID = msgStr[start : start+end]
			t.Logf("Extracted todo ID: %s", todoID)
		}
	}

	if todoID != "" {
		// Send toggle action
		t.Log("Sending toggle action...")
		toggleAction := map[string]interface{}{
			"action": "toggle",
			"data": map[string]interface{}{
				"id": todoID,
			},
		}
		toggleJSON, _ := json.Marshal(toggleAction)

		if err := conn.WriteMessage(websocket.TextMessage, toggleJSON); err != nil {
			t.Fatalf("Failed to send toggle action: %v", err)
		}

		// Read toggle response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read toggle response: %v", err)
		}

		t.Logf("Received toggle response: %s", msg)

		// Verify completion state changed
		if !strings.Contains(string(msg), "checked") {
			t.Error("Toggle response should mark todo as checked")
		}
	}

	// Test adding a second todo to verify multiple todos work
	t.Log("Sending second todo action...")
	secondTodoAction := map[string]interface{}{
		"action": "add",
		"data": map[string]interface{}{
			"text": "Second Todo Item",
		},
	}
	secondTodoJSON, _ := json.Marshal(secondTodoAction)

	if err := conn.WriteMessage(websocket.TextMessage, secondTodoJSON); err != nil {
		t.Fatalf("Failed to send second todo action: %v", err)
	}

	// Read second todo response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read second todo response: %v\nServer logs:\n%s", err, serverLogs.String())
	}

	t.Logf("Received second todo response: %s", msg)

	// Verify the response contains the second todo
	if !strings.Contains(string(msg), "Second Todo Item") {
		t.Errorf("Second todo response should contain 'Second Todo Item', got: %s", string(msg))
	}

	// Verify we don't have [object Object] in the response
	if strings.Contains(string(msg), "[object Object]") {
		t.Errorf("Response contains '[object Object]' which indicates a serialization error: %s", string(msg))
	}

	t.Log("✅ WebSocket test passed!")
}
