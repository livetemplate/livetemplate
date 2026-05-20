package livetemplate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

// broadcastTestController demonstrates the Subscribe/Publish self-topic
// pattern from both WS and HTTP paths — the canonical opt-in peer-fanout
// idiom (Subscribe(SelfTopic()) in Mount + Publish to SelfTopic() in the
// action).
type broadcastTestController struct{}

func (c *broadcastTestController) Mount(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	// Subscribe self-topic so peer WS connections receive the RefreshCount /
	// SyncMessage dispatches from the Publish calls in actions below.
	if err := ctx.Subscribe(ctx.SelfTopic()); err != nil {
		return state, err
	}
	state.Message = "mounted"
	return state, nil
}

// Increment is called via HTTP POST. It increments the counter and Publishes
// RefreshCount to all WS peers subscribed to SelfTopic.
func (c *broadcastTestController) Increment(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	state.Count++
	if err := ctx.Publish(ctx.SelfTopic(), "RefreshCount", map[string]interface{}{"newCount": state.Count}); err != nil {
		return state, err
	}
	return state, nil
}

// RefreshCount is dispatched on peer WS connections by Increment's Publish.
func (c *broadcastTestController) RefreshCount(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	if v := ctx.GetInt("newCount"); v > 0 {
		state.Count = v
	}
	return state, nil
}

// SetMessage is a WS-only action that Publishes SyncMessage to peer WS
// connections subscribed to SelfTopic.
func (c *broadcastTestController) SetMessage(state broadcastTestState, ctx *Context) (broadcastTestState, error) {
	state.Message = ctx.GetString("value")
	if err := ctx.Publish(ctx.SelfTopic(), "SyncMessage", map[string]interface{}{"value": state.Message}); err != nil {
		return state, err
	}
	return state, nil
}

// SyncMessage is dispatched on peer WS connections by SetMessage's Publish.
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

// TestHTTPPost_Publish_DispatchesToWebSocket verifies that Publish to
// SelfTopic from an HTTP POST action dispatches the named action to all
// peer WebSocket connections that subscribed in Mount.
func TestHTTPPost_Publish_DispatchesToWebSocket(t *testing.T) {
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
		form.Set("lvt-action", "increment")
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

// TestWSAction_Publish_DispatchesToOtherWS verifies that Publish to
// SelfTopic from a WebSocket action dispatches to peer WebSocket
// connections in the same group (but not the sender).
func TestWSAction_Publish_DispatchesToOtherWS(t *testing.T) {
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

// --- Explicit peer refresh tests ---

type syncDBItem struct {
	ID   string
	Text string
}

type syncDB struct {
	mu    sync.Mutex
	items map[string][]syncDBItem
}

func (db *syncDB) addItem(userID, id, text string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.items[userID] = append(db.items[userID], syncDBItem{ID: id, Text: text})
}

func (db *syncDB) getItems(userID string) []syncDBItem {
	db.mu.Lock()
	defer db.mu.Unlock()
	result := make([]syncDBItem, len(db.items[userID]))
	copy(result, db.items[userID])
	return result
}

type itemsState struct {
	Items []syncDBItem
}

type syncController struct {
	DB *syncDB
}

func (c *syncController) Mount(state itemsState, ctx *Context) (itemsState, error) {
	// Subscribe self-topic so peer tabs of the same user receive the Refresh
	// dispatch from Add's Publish below.
	if err := ctx.Subscribe(ctx.SelfTopic()); err != nil {
		return state, err
	}
	state.Items = c.DB.getItems(ctx.UserID())
	return state, nil
}

func (c *syncController) Add(state itemsState, ctx *Context) (itemsState, error) {
	text := ctx.GetString("text")
	id := fmt.Sprintf("item-%d", len(state.Items)+1)
	c.DB.addItem(ctx.UserID(), id, text)
	state.Items = c.DB.getItems(ctx.UserID())
	if err := ctx.Publish(ctx.SelfTopic(), "Refresh", nil); err != nil {
		return state, err
	}
	return state, nil
}

func (c *syncController) Refresh(state itemsState, ctx *Context) (itemsState, error) {
	state.Items = c.DB.getItems(ctx.UserID())
	return state, nil
}

func TestPublish_ExplicitRefreshDispatchesToPeers(t *testing.T) {
	db := &syncDB{items: make(map[string][]syncDBItem)}

	auth := NewBasicAuthenticator(func(username, password string) (bool, error) {
		return username == "alice" && password == "pass", nil
	})

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{len .Items}} items</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &syncController{DB: db}
	handler := tmpl.Handle(ctrl, AsState(&itemsState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws1 := connectWSWithAuth(t, wsURL, "alice", "pass")
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close: %v", err)
		}
	}()
	ws2 := connectWSWithAuth(t, wsURL, "alice", "pass")
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	addMsg := map[string]interface{}{
		"action": "add",
		"data":   map[string]interface{}{"text": "buy milk"},
	}
	msgBytes, _ := json.Marshal(addMsg)
	if err := ws1.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("ws1 write failed: %v", err)
	}

	update1 := readWSUpdate(t, ws1, 3*time.Second)
	meta1, _ := update1["meta"].(map[string]interface{})
	if meta1["action"] != "add" {
		t.Errorf("Tab 1 expected action=add, got %v", meta1["action"])
	}

	update2 := readWSUpdate(t, ws2, 3*time.Second)
	meta2, _ := update2["meta"].(map[string]interface{})
	if meta2["success"] != true {
		t.Errorf("Tab 2 expected success=true from Refresh dispatch, got %v", meta2["success"])
	}

	items := db.getItems("alice")
	if len(items) != 1 || items[0].Text != "buy milk" {
		t.Errorf("Expected 1 item 'buy milk', got %v", items)
	}
}
