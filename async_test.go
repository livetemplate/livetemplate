package livetemplate

import (
	"context"
	"fmt"
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
	auth := &fixedGroupAuth{groupID: "async-scope-group"}

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

// --- Contract test: apply error clears pending and re-renders ---

type asyncErrorController struct{}
type asyncErrorState struct {
	Status string
}

func (c *asyncErrorController) Mount(state asyncErrorState, ctx *Context) (asyncErrorState, error) {
	return state, nil
}

func (c *asyncErrorController) Fail(state asyncErrorState, ctx *Context) (asyncErrorState, error) {
	state.Status = "loading"
	Async(ctx,
		func(ctx context.Context) (string, error) {
			return "done", nil
		},
		func(s asyncErrorState, result string, err error) (asyncErrorState, error) {
			return s, fmt.Errorf("apply failed on purpose")
		},
	)
	return state, nil
}

func TestAsync_ApplyErrorClearsPendingAndReRenders(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.lvt.Pending}} {{.Status}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(&asyncErrorController{}, AsState(&asyncErrorState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	sendWSAction(t, ws, "fail", nil)

	// Render #1: Pending=true, Status=loading
	resp1 := readWSUpdate(t, ws, 3*time.Second)
	tree1 := resp1["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree1["0"]); got != "true" {
		t.Fatalf("render #1: Pending = %q, want \"true\"", got)
	}

	// Render #2: even though apply returned an error, Pending must be
	// cleared and the client must receive an update (not hang forever).
	got := wsHasUpdate(t, ws, 3*time.Second)
	if !got {
		t.Fatal("no re-render after apply error — client would be stuck on Loading")
	}
}

// --- Contract test: work panic is recovered and surfaced as error ---

type asyncPanicController struct{}
type asyncPanicState struct {
	Result string
}

func (c *asyncPanicController) Mount(state asyncPanicState, ctx *Context) (asyncPanicState, error) {
	return state, nil
}

func (c *asyncPanicController) Boom(state asyncPanicState, ctx *Context) (asyncPanicState, error) {
	state.Result = "loading"
	Async(ctx,
		func(ctx context.Context) (string, error) {
			panic("kaboom")
		},
		func(s asyncPanicState, result string, err error) (asyncPanicState, error) {
			if err != nil {
				s.Result = "error"
			} else {
				s.Result = result
			}
			return s, nil
		},
	)
	return state, nil
}

func TestAsync_WorkPanicRecoveredAsError(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.Result}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(&asyncPanicController{}, AsState(&asyncPanicState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	sendWSAction(t, ws, "boom", nil)

	// Render #1: Result=loading
	readWSUpdate(t, ws, 3*time.Second)

	// Render #2: apply should receive the panic as an error and set Result="error"
	resp2 := readWSUpdate(t, ws, 3*time.Second)
	tree2 := resp2["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree2["0"]); got != "error" {
		t.Errorf("Result = %q, want \"error\" (panic should be surfaced as err to apply)", got)
	}
}

// --- Contract test: apply panic is recovered and client still gets an update ---

type asyncApplyPanicController struct{}
type asyncApplyPanicState struct {
	Status string
}

func (c *asyncApplyPanicController) Mount(state asyncApplyPanicState, ctx *Context) (asyncApplyPanicState, error) {
	return state, nil
}

func (c *asyncApplyPanicController) Crash(state asyncApplyPanicState, ctx *Context) (asyncApplyPanicState, error) {
	state.Status = "loading"
	Async(ctx,
		func(ctx context.Context) (string, error) {
			return "done", nil
		},
		func(s asyncApplyPanicState, result string, err error) (asyncApplyPanicState, error) {
			panic("apply kaboom")
		},
	)
	return state, nil
}

func TestAsync_ApplyPanicRecoveredAndReRenders(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.lvt.Pending}} {{.Status}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(&asyncApplyPanicController{}, AsState(&asyncApplyPanicState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	sendWSAction(t, ws, "crash", nil)

	// Render #1: Pending=true, Status=loading
	resp1 := readWSUpdate(t, ws, 3*time.Second)
	tree1 := resp1["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree1["0"]); got != "true" {
		t.Fatalf("render #1: Pending = %q, want \"true\"", got)
	}

	// Render #2: apply panicked, but the client must still get an update with
	// Pending=false (state unchanged since apply panic prevented mutation).
	got := wsHasUpdate(t, ws, 3*time.Second)
	if !got {
		t.Fatal("no re-render after apply panic — client would be stuck on Loading")
	}
}

// --- P5: {{.lvt.Pending}} template variable ---

// pendingTemplateController uses {{.lvt.Pending}} in its template for zero-Go-code loading UX.
type pendingTemplateController struct {
	workGate chan struct{}
}

type pendingTemplateState struct {
	Name string
}

func (c *pendingTemplateController) Mount(state pendingTemplateState, ctx *Context) (pendingTemplateState, error) {
	return state, nil
}

func (c *pendingTemplateController) Greet(state pendingTemplateState, ctx *Context) (pendingTemplateState, error) {
	name := ctx.GetString("name")
	gate := c.workGate

	Async(ctx,
		func(ctx context.Context) (string, error) {
			if gate != nil {
				select {
				case <-gate:
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			return name, nil
		},
		func(s pendingTemplateState, result string, err error) (pendingTemplateState, error) {
			s.Name = result
			return s, nil
		},
	)
	return state, nil
}

func TestLvtPending_TrueOnRender1_FalseOnRender2(t *testing.T) {
	gate := make(chan struct{})
	ctrl := &pendingTemplateController{workGate: gate}

	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	// Use {{.lvt.Pending}} directly (renders "true"/"false") to avoid
	// nested conditional sub-trees in the diff.
	tmpl, err = tmpl.Parse(`<div>{{.lvt.Pending}} {{.Name}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(ctrl, AsState(&pendingTemplateState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Trigger Greet (registers Async, gate blocks work)
	sendWSAction(t, ws, "greet", map[string]any{"name": "Alice"})

	// Render #1: .lvt.Pending should be true
	resp1 := readWSUpdate(t, ws, 3*time.Second)
	tree1 := resp1["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree1["0"]); got != "true" {
		t.Errorf("render #1: .lvt.Pending = %q, want \"true\"", got)
	}

	// Unblock async work
	close(gate)

	// Render #2: .lvt.Pending should be false, Name = "Alice"
	resp2 := readWSUpdate(t, ws, 3*time.Second)
	tree2 := resp2["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree2["0"]); got != "false" {
		t.Errorf("render #2: .lvt.Pending = %q, want \"false\"", got)
	}
	if got := fmt.Sprintf("%v", tree2["1"]); got != "Alice" {
		t.Errorf("render #2: Name = %q, want \"Alice\"", got)
	}
}

// Pin per-render semantics: an interleaved non-async action renders
// .lvt.Pending=false even though async work is still in flight.
func TestLvtPending_PerRenderSemantics(t *testing.T) {
	gate := make(chan struct{})
	ctrl := &asyncTestController{workGate: gate}

	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.lvt.Pending}} {{.Loading}} {{.Name}} {{.Counter}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(ctrl, AsState(&asyncTestState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Trigger Greet (registers Async, gate blocks work)
	sendWSAction(t, ws, "greet", map[string]any{"name": "Alice"})

	// Render #1: .lvt.Pending=true (this action registered async work)
	resp1 := readWSUpdate(t, ws, 3*time.Second)
	tree1 := resp1["tree"].(map[string]any)
	if got := fmt.Sprintf("%v", tree1["0"]); got != "true" {
		t.Fatalf("render #1: .lvt.Pending = %q, want \"true\"", got)
	}

	// Send a non-async Increment while async work is still blocked
	sendWSAction(t, ws, "increment", nil)
	resp2 := readWSUpdate(t, ws, 3*time.Second)
	tree2 := resp2["tree"].(map[string]any)

	// .lvt.Pending should be false on this render (Increment didn't register
	// async work), even though Greet's async work is still in flight.
	if got := fmt.Sprintf("%v", tree2["0"]); got != "false" {
		t.Errorf("interleaved render: .lvt.Pending = %q, want \"false\" (per-render semantics)", got)
	}

	// Unblock async work and verify completion still arrives
	close(gate)
	readWSUpdate(t, ws, 3*time.Second)
}

func TestLvtPending_FalseForNonAsyncActions(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.lvt.Pending}} {{.Count}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&testHandleController{}, AsState(&testHandleState{}))
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, initial := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// The initial render should have .lvt.Pending = false
	pendingVal := treeDynamic(t, initial, "0")
	if pendingVal != "false" {
		t.Errorf("initial render: .lvt.Pending = %q, want \"false\"", pendingVal)
	}
}
