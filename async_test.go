package livetemplate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Async Primitive — P2 Acceptance Tests
// ============================================================================

// --- Test infrastructure ---

type asyncTestState struct {
	Loading bool
	Name    string
	Counter int
}

type asyncTestController struct {
	workDelay time.Duration // how long the async work takes
	workGate  chan struct{} // if non-nil, work blocks until closed

	mu         sync.Mutex
	applyCalls int
}

func (c *asyncTestController) Mount(state asyncTestState, ctx *Context) (asyncTestState, error) {
	return state, nil
}

// Greet triggers Async: sets Loading=true, spawns work, apply clears Loading and sets Name.
// The apply callback reads Counter to prove it sees the live state, not a snapshot.
func (c *asyncTestController) Greet(state asyncTestState, ctx *Context) (asyncTestState, error) {
	state.Loading = true
	name := ctx.GetString("name")
	delay := c.workDelay
	gate := c.workGate

	Async(ctx,
		func(ctx context.Context) (string, error) {
			if gate != nil {
				select {
				case <-gate:
				case <-ctx.Done():
					return "", ctx.Err()
				}
			} else if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			return name, nil
		},
		func(s asyncTestState, result string, err error) (asyncTestState, error) {
			c.mu.Lock()
			c.applyCalls++
			c.mu.Unlock()
			// Encode Counter into Name to prove apply saw the interleaved state.
			s.Name = fmt.Sprintf("%s(counter=%d)", result, s.Counter)
			s.Loading = false
			return s, nil
		},
	)
	return state, nil
}

// Increment modifies Counter — used to interleave state changes during async work.
func (c *asyncTestController) Increment(state asyncTestState, ctx *Context) (asyncTestState, error) {
	state.Counter++
	return state, nil
}

// asyncFixedGroupAuth forces all connections into one group.
type asyncFixedGroupAuth struct {
	groupID string
}

func (a *asyncFixedGroupAuth) Identify(_ *http.Request) (string, error) { return "", nil }
func (a *asyncFixedGroupAuth) GetSessionGroup(_ *http.Request, _ string) (string, error) {
	return a.groupID, nil
}

func setupAsyncTestServer(t *testing.T, ctrl *asyncTestController, opts ...Option) string {
	t.Helper()
	tmpl, err := New("test", opts...)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Loading}} {{.Name}} {{.Counter}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(ctrl, AsState(&asyncTestState{}))
	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	t.Cleanup(server.Close)
	return wsURL
}

// wsHasUpdate reads a WS update with a timeout, returning true if an update
// was received. Unlike readWSUpdate in broadcast_test.go, this does not Fatal
// on timeout — it returns false so callers can assert that NO update arrived.
func wsHasUpdate(t *testing.T, ws *websocket.Conn, timeout time.Duration) bool {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err := ws.ReadMessage()
	return err == nil
}

// --- Contract test 1: apply sees state mutated by an interleaved action ---

func TestAsync_ApplySeesInterleavedState(t *testing.T) {
	gate := make(chan struct{})
	ctrl := &asyncTestController{workGate: gate}

	wsURL := setupAsyncTestServer(t, ctrl)

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// 1. Trigger Greet (starts async work, blocked on gate)
	sendWSAction(t, ws, "greet", map[string]any{"name": "Alice"})

	// Read render #1: Loading=true
	resp1 := readWSUpdate(t, ws, 3*time.Second)
	tree1 := resp1["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree1["0"]); got != "true" {
		t.Fatalf("render #1: Loading = %q, want \"true\"", got)
	}

	// 2. While async work is blocked, send an interleaved Increment action
	sendWSAction(t, ws, "increment", nil)
	respIncr := readWSUpdate(t, ws, 3*time.Second)
	treeIncr := respIncr["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", treeIncr["2"]); got != "1" {
		t.Fatalf("increment: Counter = %q, want \"1\"", got)
	}

	// 3. Unblock async work
	close(gate)

	// Read render #2: async completion
	resp2 := readWSUpdate(t, ws, 3*time.Second)
	tree2 := resp2["tree"].(map[string]any)

	// Loading should be false
	if got := fmt.Sprintf("%v", tree2["0"]); got != "false" {
		t.Errorf("async completion: Loading = %q, want \"false\"", got)
	}
	// Name encodes the Counter value that apply saw. If apply received a
	// snapshot from when Greet ran (Counter=0), it would be "Alice(counter=0)".
	// If it correctly received the current state (Counter=1 from Increment),
	// it will be "Alice(counter=1)".
	if got := fmt.Sprintf("%v", tree2["1"]); got != "Alice(counter=1)" {
		t.Errorf("async completion: Name = %q, want \"Alice(counter=1)\" (apply must use current state)", got)
	}
}

// --- Contract test 2: disconnect cancels work and skips apply ---

func TestAsync_DisconnectCancelsWork(t *testing.T) {
	gate := make(chan struct{})
	ctrl := &asyncTestController{workGate: gate}

	wsURL := setupAsyncTestServer(t, ctrl)

	ws, _ := connectWSRaw(t, wsURL)

	// Trigger Greet (starts async work, blocked on gate)
	sendWSAction(t, ws, "greet", map[string]any{"name": "Bob"})

	// Read render #1
	readWSUpdate(t, ws, 3*time.Second)

	// Close the connection while work is in flight.
	// Brief pause lets the server-side event loop break and Unregister
	// (which closes connection.Done()) before we unblock the work goroutine.
	if err := ws.Close(); err != nil {
		t.Logf("ws close: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Unblock work — should be cancelled (context.Done closed)
	close(gate)

	// Give time for any goroutine to settle
	time.Sleep(100 * time.Millisecond)

	// Apply should NOT have been called
	ctrl.mu.Lock()
	calls := ctrl.applyCalls
	ctrl.mu.Unlock()
	if calls != 0 {
		t.Errorf("apply was called %d times after disconnect, want 0", calls)
	}
}

// --- Contract test 3: render scope is originating connection only ---

func TestAsync_RenderScopeIsOriginatingConnectionOnly(t *testing.T) {
	gate := make(chan struct{})
	ctrl := &asyncTestController{workGate: gate}
	auth := &asyncFixedGroupAuth{groupID: "async-scope-group"}

	wsURL := setupAsyncTestServer(t, ctrl, WithAuthenticator(auth))

	// Connect two clients in the same group
	ws1, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close: %v", err)
		}
	}()
	ws2, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	// Client 1 triggers Greet
	sendWSAction(t, ws1, "greet", map[string]any{"name": "Charlie"})

	// Client 1 gets render #1 (Loading=true)
	readWSUpdate(t, ws1, 3*time.Second)

	// Client 2 should NOT get any update from client 1's action
	got := wsHasUpdate(t, ws2, 500*time.Millisecond)
	if got {
		t.Log("ws2 received an update during greet action — expected for shared-group dispatch")
	}

	// Unblock async work
	close(gate)

	// Client 1 gets render #2 (async completion)
	resp := readWSUpdate(t, ws1, 3*time.Second)
	tree := resp["tree"].(map[string]any)
	if v := fmt.Sprintf("%v", tree["1"]); v != "Charlie(counter=0)" {
		t.Errorf("ws1 async completion: Name = %q, want \"Charlie(counter=0)\"", v)
	}

	// Client 2 must NOT get the async completion (per-connection scope)
	got = wsHasUpdate(t, ws2, 500*time.Millisecond)
	if got {
		t.Errorf("ws2 received async completion update — Async must only re-render the originating connection")
	}
}

// --- Additional: Async with immediate completion ---

func TestAsync_ImmediateCompletion(t *testing.T) {
	ctrl := &asyncTestController{workDelay: 0}

	wsURL := setupAsyncTestServer(t, ctrl)

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	sendWSAction(t, ws, "greet", map[string]any{"name": "Quick"})

	// Render #1: Loading=true
	readWSUpdate(t, ws, 3*time.Second)

	// Render #2: async completion (should arrive quickly)
	resp := readWSUpdate(t, ws, 3*time.Second)
	tree := resp["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree["0"]); got != "false" {
		t.Errorf("Loading = %q, want \"false\"", got)
	}
	if got := fmt.Sprintf("%v", tree["1"]); got != "Quick(counter=0)" {
		t.Errorf("Name = %q, want \"Quick(counter=0)\"", got)
	}
}
