package livetemplate

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// =============================================================================
// OnDisconnect Integration Test
// =============================================================================
//
// The reflection-level lifecycle_test.go verifies that callOnDisconnect can
// invoke a controller's OnDisconnect method when called directly. This test
// verifies the end-to-end path: a real WebSocket connection closes, the
// framework detects it, and OnDisconnect fires on the controller.
//
// This matters for patterns like Presence Tracking (proposal #28), which
// rely on OnDisconnect to clean up per-connection state.

type onDisconnectTestState struct {
	Count int
}

type onDisconnectTestController struct {
	disconnectCalls atomic.Int32
	disconnectCh    chan struct{}
}

func (c *onDisconnectTestController) Mount(state onDisconnectTestState, ctx *Context) (onDisconnectTestState, error) {
	return state, nil
}

func (c *onDisconnectTestController) OnDisconnect() {
	c.disconnectCalls.Add(1)
	// Non-blocking notification — test may not be listening yet.
	select {
	case c.disconnectCh <- struct{}{}:
	default:
	}
}

func TestOnDisconnect_FiresOnWebSocketClose(t *testing.T) {
	ctrl := &onDisconnectTestController{disconnectCh: make(chan struct{}, 1)}

	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(ctrl, AsState(&onDisconnectTestState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Open the WebSocket. connectWSRaw blocks until the initial render
	// frame is delivered, which means the server has already completed
	// Mount + OnConnect and entered its event loop. Discarding the frame
	// bytes is safe — the test doesn't care about the initial render
	// contents, only about triggering OnDisconnect on the subsequent close.
	ws, _ := connectWSRaw(t, wsURL)

	// Close the connection cleanly and wait for OnDisconnect.
	if err := ws.Close(); err != nil {
		t.Fatalf("WebSocket close failed: %v", err)
	}

	select {
	case <-ctrl.disconnectCh:
		// Success — OnDisconnect fired.
	case <-time.After(3 * time.Second):
		t.Fatal("OnDisconnect was not called within 3s after WebSocket close")
	}

	if got := ctrl.disconnectCalls.Load(); got != 1 {
		t.Errorf("OnDisconnect call count = %d, want 1", got)
	}
}

// TestOnDisconnect_FiresOnAbruptClose verifies the hook fires even when the
// client goes away without a clean WebSocket close frame (network drop,
// browser tab kill). This is the scenario that presence tracking needs to
// handle — most real disconnects are not clean.
func TestOnDisconnect_FiresOnAbruptClose(t *testing.T) {
	ctrl := &onDisconnectTestController{disconnectCh: make(chan struct{}, 1)}

	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(ctrl, AsState(&onDisconnectTestState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	// connectWSRaw blocks on the initial render so the server has
	// finished OnConnect by the time it returns. The initial frame
	// bytes are discarded because the test only cares about the
	// disconnect hook firing on the subsequent abrupt close.
	ws, _ := connectWSRaw(t, wsURL)

	// Close the underlying TCP connection without sending a close frame.
	// This simulates a browser tab being killed or network failure.
	if err := ws.UnderlyingConn().Close(); err != nil {
		t.Fatalf("Underlying conn close failed: %v", err)
	}

	// Abrupt-close detection depends on the server's read pump noticing
	// the TCP EOF, which can take longer than a clean close under CI
	// load. The t.Log markers below disambiguate a slow machine from a
	// real regression in CI output.
	deadline := time.After(5 * time.Second)
	progress := time.NewTicker(1 * time.Second)
	defer progress.Stop()
	elapsed := 0
	for {
		select {
		case <-ctrl.disconnectCh:
			return
		case <-progress.C:
			elapsed++
			t.Logf("OnDisconnect not yet received after %ds (server hasn't noticed TCP EOF yet)...", elapsed)
		case <-deadline:
			t.Fatal("OnDisconnect was not called within 5s after abrupt close")
		}
	}
}

// =============================================================================
// SetFlash via BroadcastAction Integration Test
// =============================================================================
//
// Verifies that ctx.SetFlash messages survive the BroadcastAction dispatch
// path: the initiating connection sees its own flash, and each peer's
// dispatched action handler can also set its own flash that gets rendered
// on that peer's client.
//
// Before this test, broadcast_test.go covered BroadcastAction itself but
// nothing verified that FlashSetter is wired on peer connections too.

type flashBroadcastState struct {
	Counter int
}

type flashBroadcastController struct{}

func (c *flashBroadcastController) Mount(state flashBroadcastState, ctx *Context) (flashBroadcastState, error) {
	return state, nil
}

// Bump increments the counter, sets a "success" flash on the initiator,
// and broadcasts PeerSync to peer connections.
func (c *flashBroadcastController) Bump(state flashBroadcastState, ctx *Context) (flashBroadcastState, error) {
	state.Counter++
	ctx.SetFlash("success", "bump complete on sender")
	ctx.BroadcastAction("PeerSync", map[string]interface{}{"counter": state.Counter})
	return state, nil
}

// PeerSync runs on peer connections when Bump broadcasts. It sets its own
// flash to verify FlashSetter is available in the dispatched action context.
func (c *flashBroadcastController) PeerSync(state flashBroadcastState, ctx *Context) (flashBroadcastState, error) {
	state.Counter = ctx.GetInt("counter")
	ctx.SetFlash("info", "peer received broadcast")
	return state, nil
}

func TestBroadcastAction_PeerCanSetFlash(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "flash-broadcast-group"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	// Template renders both flash slots so we can verify propagation by
	// inspecting the wire-format tree dynamics instead of relying on
	// internal state.
	tmpl, err = tmpl.Parse(
		`<div>{{.Counter}}|s:{{.lvt.Flash "success"}}|i:{{.lvt.Flash "info"}}</div>`,
	)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&flashBroadcastController{}, AsState(&flashBroadcastState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Two WebSocket connections in the same session group. ws1 will send
	// the Bump action; ws2 should receive the PeerSync broadcast.
	ws1 := connectWS(t, wsURL)
	defer func() { _ = ws1.Close() }()
	ws2 := connectWS(t, wsURL)
	defer func() { _ = ws2.Close() }()

	// No explicit settle delay is required: connectWS blocks until the
	// initial render arrives on the socket, which only happens after the
	// server has completed the mount handshake and registered the
	// connection in the group. By the time the second connectWS returns,
	// both connections are guaranteed to be in the registry.

	// Trigger Bump on ws1. This should produce:
	//   - ws1 response with state.Counter=1 and flash success="bump complete on sender"
	//   - ws2 dispatched PeerSync with state.Counter=1 and flash info="peer received broadcast"
	actionMsg := []byte(`{"action":"bump","data":{}}`)
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("ws1 write failed: %v", err)
	}

	// ws1 should receive its own response with the sender-side flash rendered
	// into the tree.
	ws1Update := readWSUpdate(t, ws1, 3*time.Second)
	assertTreeContains(t, ws1Update, "bump complete on sender", "ws1 (sender)")

	// ws2 should receive the dispatched PeerSync action with its own flash
	// rendered into the tree.
	ws2Update := readWSUpdate(t, ws2, 3*time.Second)
	assertTreeContains(t, ws2Update, "peer received broadcast", "ws2 (peer)")
}

// assertTreeContains walks the wire-format tree response and fails the test
// if `want` does not appear in any of the string dynamics. Recurses into
// nested tree maps. This is more resilient than indexing by numeric key,
// which can shift if the template structure changes.
func assertTreeContains(t *testing.T, update map[string]interface{}, want, label string) {
	t.Helper()
	tree, ok := update["tree"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s: expected tree in update, got: %v", label, update)
	}
	if treeContainsString(tree, want) {
		return
	}
	t.Errorf("%s: expected %q somewhere in tree dynamics, tree=%v", label, want, tree)
}

func treeContainsString(node map[string]interface{}, want string) bool {
	for _, v := range node {
		switch vv := v.(type) {
		case string:
			if strings.Contains(vv, want) {
				return true
			}
		case map[string]interface{}:
			if treeContainsString(vv, want) {
				return true
			}
		}
	}
	return false
}

type wsCtxCancelState struct {
	Count int
}

type wsCtxCancelController struct {
	captured chan context.Context
}

func (c *wsCtxCancelController) Mount(state wsCtxCancelState, ctx *Context) (wsCtxCancelState, error) {
	// Non-blocking send — if multiple Mounts run (reconnect), keep the first.
	select {
	case c.captured <- ctx.Context:
	default:
	}
	return state, nil
}

func TestWebSocketLifecycleCtx_CancelsOnDisconnect(t *testing.T) {
	ctrl := &wsCtxCancelController{captured: make(chan context.Context, 1)}

	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(ctrl, AsState(&wsCtxCancelState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)

	var mountCtx context.Context
	select {
	case mountCtx = <-ctrl.captured:
	case <-time.After(2 * time.Second):
		t.Fatal("Mount was not called within 2s of WebSocket connect")
	}

	if mountCtx == nil {
		t.Fatal("Mount captured a nil context")
	}
	if err := mountCtx.Err(); err != nil {
		t.Fatalf("ctx already canceled before disconnect: %v", err)
	}

	// Close the WebSocket. r.Context() should be canceled when the HTTP
	// handler returns, which propagates to the captured Mount ctx.
	if err := ws.Close(); err != nil {
		t.Fatalf("WebSocket close failed: %v", err)
	}

	select {
	case <-mountCtx.Done():
		// Success — lifecycle ctx canceled with the request.
	case <-time.After(2 * time.Second):
		t.Fatal("Mount ctx was not canceled within 2s after WebSocket close — lifecycleCtx is detached from r.Context() (issue #303)")
	}

	if err := mountCtx.Err(); err == nil {
		t.Errorf("expected mountCtx.Err() to be non-nil after Done(), got nil")
	}
}
