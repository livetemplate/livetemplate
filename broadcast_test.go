package livetemplate

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fixedGroupAuth always returns the same groupID so all connections
// (both WS and HTTP) share a single session group.
type fixedGroupAuth struct {
	groupID string
}

func (a *fixedGroupAuth) Identify(_ *http.Request) (string, error) { return "", nil }
func (a *fixedGroupAuth) GetSessionGroup(_ *http.Request, _ string) (string, error) {
	return a.groupID, nil
}

// broadcastTestState is the per-connection state for broadcast tests.
type broadcastTestState struct {
	Count   int
	Message string
}

// broadcastTestController demonstrates BroadcastAction from both WS and HTTP paths.
type broadcastTestController struct{}

func (c *broadcastTestController) Mount(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	state.Message = "mounted"
	return state, nil
}

// Increment is called via HTTP POST. It increments the counter and
// broadcasts RefreshCount to all WebSocket connections.
func (c *broadcastTestController) Increment(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	state.Count++
	ctx.BroadcastAction("RefreshCount", map[string]interface{}{"newCount": state.Count})
	return state, nil
}

// RefreshCount is dispatched on WebSocket connections by BroadcastAction.
func (c *broadcastTestController) RefreshCount(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	if v := ctx.GetInt("newCount"); v > 0 {
		state.Count = v
	}
	return state, nil
}

// SetMessage is a WS-only action that broadcasts to other WS connections.
func (c *broadcastTestController) SetMessage(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	state.Message = ctx.GetString("value")
	ctx.BroadcastAction("SyncMessage", map[string]interface{}{"value": state.Message})
	return state, nil
}

// SyncMessage is dispatched on other WS connections by BroadcastAction.
func (c *broadcastTestController) SyncMessage(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	state.Message = ctx.GetString("value")
	return state, nil
}

func setupBroadcastTestServer(t *testing.T, opts ...Option) (*httptest.Server, string) {
	t.Helper()

	// Use fixedGroupAuth so all connections share one group
	auth := &fixedGroupAuth{groupID: "test-group"}
	allOpts := append([]Option{WithAuthenticator(auth)}, opts...)

	tmpl, err := New("test", allOpts...)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}} - {{.Message}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &broadcastTestController{}
	state := AsState(&broadcastTestState{})
	handler := tmpl.Handle(ctrl, state)

	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	return server, wsURL
}

func connectWS(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, initialMsg, err := ws.ReadMessage()
	if err != nil {
		if closeErr := ws.Close(); closeErr != nil {
			t.Logf("WebSocket close error: %v", closeErr)
		}
		t.Fatalf("Failed to read initial render: %v", err)
	}
	t.Logf("connectWS initial render (%d bytes)", len(initialMsg))
	return ws
}

func readWSUpdate(t *testing.T, ws *websocket.Conn, timeout time.Duration) map[string]interface{} {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return resp
}

// TestHTTPPost_BroadcastAction_DispatchesToWebSocket verifies that
// BroadcastAction called from an HTTP POST action dispatches the named
// action to all WebSocket connections in the group.
func TestHTTPPost_BroadcastAction_DispatchesToWebSocket(t *testing.T) {
	server, wsURL := setupBroadcastTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("WebSocket close error: %v", err)
		}
	}()

	// Retry POST until WS receives the broadcast (covers registration race)
	var update map[string]interface{}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		form := url.Values{}
		form.Set("action", "increment")
		resp, err := http.Post(server.URL+"/", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("HTTP POST failed: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("response body close error: %v", err)
		}

		if err := ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline failed: %v", err)
		}
		_, msg, err := ws.ReadMessage()
		if err != nil {
			continue // WS not registered yet or no update — retry
		}
		if err := json.Unmarshal(msg, &update); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		break
	}
	if update == nil {
		t.Fatal("WS never received broadcast from HTTP POST")
	}

	meta, ok := update["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta in update")
	}
	if meta["success"] != true {
		t.Errorf("Expected success=true in broadcast update, got %v", meta["success"])
	}
}

// TestWSAction_BroadcastAction_DispatchesToOtherWS verifies that
// BroadcastAction called from a WebSocket action dispatches to
// other WebSocket connections in the same group (but not the sender).
func TestWSAction_BroadcastAction_DispatchesToOtherWS(t *testing.T) {
	server, wsURL := setupBroadcastTestServer(t)
	defer server.Close()

	// Connect two WebSocket clients (simulates two browser tabs)
	ws1 := connectWS(t, wsURL)
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close error: %v", err)
		}
	}()
	ws2 := connectWS(t, wsURL)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close error: %v", err)
		}
	}()

	// Tab 1 sends SetMessage action
	actionMsg := map[string]interface{}{
		"action": "set_message",
		"data":   map[string]interface{}{"value": "hello from tab 1"},
	}
	msgBytes, _ := json.Marshal(actionMsg)
	if err := ws1.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}

	// Tab 1 receives its own action response
	update1 := readWSUpdate(t, ws1, 3*time.Second)
	meta1, _ := update1["meta"].(map[string]interface{})
	if meta1["action"] != "set_message" {
		t.Errorf("Tab 1 expected action=set_message, got %v", meta1["action"])
	}

	// Tab 2 should receive the SyncMessage broadcast
	update2 := readWSUpdate(t, ws2, 3*time.Second)
	meta2, _ := update2["meta"].(map[string]interface{})
	if meta2["success"] != true {
		t.Errorf("Tab 2 expected success=true, got %v", meta2["success"])
	}

	// Tab 1 (sender) should NOT receive the broadcast dispatch.
	// It already got its own action response above — no second message expected.
	if err := ws1.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err := ws1.ReadMessage()
	if err == nil {
		t.Error("Tab 1 (sender) should NOT receive its own broadcast dispatch")
	}
}

// TestSharedState_HTTPPost_AutoBroadcasts verifies that WithSharedState()
// restores auto-broadcast behavior: HTTP POST actions automatically push
// state to all WebSocket connections without BroadcastAction.
func TestSharedState_HTTPPost_AutoBroadcasts(t *testing.T) {
	server, wsURL := setupBroadcastTestServer(t, WithSharedState())
	defer server.Close()

	// Connect a WebSocket client
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("WebSocket close error: %v", err)
		}
	}()

	// Retry POST until WS receives the broadcast (covers registration race)
	var update map[string]interface{}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		form := url.Values{}
		form.Set("action", "increment")
		resp, err := http.Post(server.URL+"/", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("HTTP POST failed: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("response body close error: %v", err)
		}

		if err := ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline failed: %v", err)
		}
		_, msg, err := ws.ReadMessage()
		if err != nil {
			continue
		}
		if err := json.Unmarshal(msg, &update); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		break
	}
	if update == nil {
		t.Fatal("Expected WebSocket to receive update in SharedState mode")
	}
}

func connectWSWithAuth(t *testing.T, wsURL, username, password string) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
		[]byte(username+":"+password)))
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("WebSocket dial with auth failed: %v", err)
	}
	if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err = ws.ReadMessage()
	if err != nil {
		if closeErr := ws.Close(); closeErr != nil {
			t.Logf("WebSocket close error: %v", closeErr)
		}
		t.Fatalf("Failed to read initial render: %v", err)
	}
	return ws
}

// TestSharedState_AuthenticatedUser_AutoSyncsAllTabs verifies that
// BasicAuthenticator + WithSharedState() auto-broadcasts to all tabs
// for the same authenticated user with zero BroadcastAction calls.
func TestSharedState_AuthenticatedUser_AutoSyncsAllTabs(t *testing.T) {
	auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
		return username == "testuser" && password == "testpass", nil
	})

	tmpl, err := New("test", WithAuthenticator(auth), WithSharedState())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}} - {{.Message}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &broadcastTestController{}
	handler := tmpl.Handle(ctrl, AsState(&broadcastTestState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Two tabs, same authenticated user → same groupID
	ws1 := connectWSWithAuth(t, wsURL, "testuser", "testpass")
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close error: %v", err)
		}
	}()
	ws2 := connectWSWithAuth(t, wsURL, "testuser", "testpass")
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close error: %v", err)
		}
	}()

	// Tab 1 sends increment — controller does NOT call BroadcastAction
	actionMsg := map[string]interface{}{
		"action": "increment",
	}
	msgBytes, _ := json.Marshal(actionMsg)
	if err := ws1.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}

	// Tab 1 receives its own action response
	update1 := readWSUpdate(t, ws1, 3*time.Second)
	meta1, _ := update1["meta"].(map[string]interface{})
	if meta1["action"] != "increment" {
		t.Errorf("Tab 1 expected action=increment, got %v", meta1["action"])
	}

	// Tab 2 receives auto-broadcast — zero BroadcastAction calls needed
	update2 := readWSUpdate(t, ws2, 3*time.Second)
	meta2, _ := update2["meta"].(map[string]interface{})
	if meta2["success"] != true {
		t.Errorf("Tab 2 expected success=true from auto-broadcast, got %v", meta2["success"])
	}
}
