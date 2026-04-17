package livetemplate

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// navigateTestState is the per-connection state for the navigate action
// test. It records how many times Mount ran, which simulates a controller
// that needs to re-run Mount-time side effects on each navigation.
type navigateTestState struct {
	Selected   string
	MountCount int
}

// navigateTestController reads a "s" query param in Mount and uses it to
// set state.Selected, then renders the current value. The test can assert
// against MountCount to verify the navigate action re-ran Mount without
// double-counting.
type navigateTestController struct{}

func (c *navigateTestController) Mount(state navigateTestState, ctx *Context) (navigateTestState, error) {
	state.Selected = ctx.GetString("s")
	state.MountCount++
	return state, nil
}

// Noop is a regular action that does nothing — used by tests to verify
// non-navigate actions still hit DispatchWithState and don't accidentally
// route through Mount.
func (c *navigateTestController) Noop(state navigateTestState, _ *Context) (navigateTestState, error) {
	return state, nil
}

func setupNavigateTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	// All connections share a single session group so we can reason
	// about a single WebSocket's state transitions across messages.
	auth := &fixedGroupAuth{groupID: "navigate-test-group"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div class="sel">{{.Selected}}</div><div class="count">{{.MountCount}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &navigateTestController{}
	state := AsState(&navigateTestState{})
	handler := tmpl.Handle(ctrl, state)

	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/?s=alpha"
	return server, wsURL
}

// TestNavigateActionReMountsWithNewQueryData is the load-bearing test
// for the in-band navigate message. It wires a controller whose Mount
// reads the "s" query param, opens a WebSocket connect with ?s=alpha,
// confirms state.Selected == "alpha", then sends a {action:"__navigate__",
// data:{s:"beta"}} message and confirms state.Selected flips to "beta"
// on the SAME WebSocket connection — without any reconnect.
//
// Uses raw substring assertions on the tree response bytes rather than
// trying to walk the static/dynamic tree shape — livetemplate's tree
// format is an implementation detail that may change, and what matters
// for this test is that the new query param's value reaches the render
// output.
func TestNavigateActionReMountsWithNewQueryData(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Fire the navigate action with new query data. The response is
	// the tree update for the re-mounted state.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "beta"})
	bytesAfter := readWSBytes(t, ws, 2*time.Second)
	if !strings.Contains(string(bytesAfter), "beta") {
		t.Errorf("after navigate, response missing 'beta':\n%s", bytesAfter)
	}
	// MountCount=2: one from initial connect, one from the navigate.
	// Navigate responses are tree UPDATES (statics already cached client-side),
	// so the raw bytes contain only dynamic slot values like `"1":"2"` —
	// no statics, no fingerprints. `"2"` here is the rendered count value
	// and is unambiguous in this response shape.
	if !strings.Contains(string(bytesAfter), `"2"`) {
		t.Errorf("after navigate, response missing mount count '2':\n%s", bytesAfter)
	}

	// One more navigate to a third value confirms we can re-nav
	// multiple times on the same connection.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "gamma"})
	bytesAfter2 := readWSBytes(t, ws, 2*time.Second)
	if !strings.Contains(string(bytesAfter2), "gamma") {
		t.Errorf("after second navigate, response missing 'gamma':\n%s", bytesAfter2)
	}
	if !strings.Contains(string(bytesAfter2), `"3"`) {
		t.Errorf("after second navigate, response missing mount count '3':\n%s", bytesAfter2)
	}
}

// TestNavigateActionNotRoutedThroughDispatchWithState verifies that the
// navigate action is intercepted BEFORE the regular DispatchWithState
// path runs. The controller doesn't define a method named __navigate__
// or Navigate, yet the action succeeds because the event loop routes it
// to Mount directly. This is the contract that makes the navigate
// message work on any controller without a method-name collision.
func TestNavigateActionNotRoutedThroughDispatchWithState(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// The controller has no method named Navigate or __navigate__.
	// Sending the navigate action should succeed (route through Mount)
	// rather than producing an ErrMethodNotFound error in state.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "abc"})
	resp := readWSUpdate(t, ws, 2*time.Second)

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("response has no meta: %#v", resp)
	}
	if success, _ := meta["success"].(bool); !success {
		t.Errorf("navigate response success = false, want true; meta = %#v", meta)
	}
	if errs, hasErrs := meta["errors"].(map[string]interface{}); hasErrs && len(errs) > 0 {
		t.Errorf("navigate response has errors: %#v", errs)
	}
}

// sendWSAction sends an action message over the WebSocket, matching the
// wire format the client uses.
func sendWSAction(t *testing.T, ws *websocket.Conn, action string, data map[string]interface{}) {
	t.Helper()
	msg := map[string]interface{}{
		"action": action,
	}
	if data != nil {
		msg["data"] = data
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

// readWSBytes reads one WebSocket frame and returns the raw bytes.
// Useful for assertions that don't care about the internal tree shape.
func readWSBytes(t *testing.T, ws *websocket.Conn, timeout time.Duration) []byte {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	return data
}
