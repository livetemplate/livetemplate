package livetemplate

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// triggerActionTestState carries state for the TriggerAction lifecycle test.
// The Pushed field is populated by the DataLoaded action, which is invoked
// indirectly via session.TriggerAction from a background goroutine.
type triggerActionTestState struct {
	Pushed string
}

// triggerActionTestController exercises the OnConnect -> goroutine ->
// TriggerAction -> action handler pipeline end-to-end. It captures the
// TriggerAction error return so tests can assert disconnect semantics.
//
// The goroutine spawned in OnConnect honours a cancel context so that
// test teardown (via t.Fatal or normal exit) cancels the sleep and
// skips the TriggerAction call. Without this, a test failure before
// the sleep elapses would leave the goroutine asleep while defers
// tore down the server, producing a race under `go test -race`.
type triggerActionTestController struct {
	mu           sync.Mutex
	triggered    int
	triggerErr   error
	gotSession   bool
	done         chan struct{}
	connectDelay time.Duration
	cancelCtx    context.Context
}

func (c *triggerActionTestController) OnConnect(state triggerActionTestState, ctx *Context) (triggerActionTestState, error) {
	sess := ctx.Session()
	c.mu.Lock()
	c.gotSession = sess != nil
	c.mu.Unlock()
	if sess == nil {
		if c.done != nil {
			close(c.done)
		}
		return state, nil
	}
	go func() {
		select {
		case <-time.After(c.connectDelay):
		case <-c.cancelCtx.Done():
			// Test is tearing down — skip TriggerAction on the stale
			// server and signal completion so the test drain can exit.
			if c.done != nil {
				close(c.done)
			}
			return
		}
		err := sess.TriggerAction("dataLoaded", map[string]interface{}{
			"data": "hello from goroutine",
		})
		c.mu.Lock()
		c.triggered++
		c.triggerErr = err
		c.mu.Unlock()
		if c.done != nil {
			close(c.done)
		}
	}()
	return state, nil
}

func (c *triggerActionTestController) DataLoaded(state triggerActionTestState, ctx *Context) (triggerActionTestState, error) {
	state.Pushed = ctx.GetString("data")
	return state, nil
}

// TestLocalSession_TriggerActionFromOnConnect verifies that when a controller
// spawns a goroutine from OnConnect and calls session.TriggerAction, the
// targeted action handler runs and the resulting state diff is pushed to
// the connected WebSocket client.
//
// Before the localSession fix, ctx.Session() returned nil inside OnConnect
// and the goroutine silently did nothing — so the client would only ever
// see the initial render. This test would hang on readWSUpdate without the
// fix.
func TestLocalSession_TriggerActionFromOnConnect(t *testing.T) {
	auth := &fixedUserAuth{groupID: "local-session-group", userID: "test-user"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Pushed: {{.Pushed}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Cancel context that the spawned OnConnect goroutine honours so
	// test teardown aborts the TriggerAction call instead of racing
	// against server.Close().
	cancelCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ctrl := &triggerActionTestController{
		done:         make(chan struct{}),
		connectDelay: 100 * time.Millisecond,
		cancelCtx:    cancelCtx,
	}
	handler := tmpl.Handle(ctrl, AsState(&triggerActionTestState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Initial render.
	ws, initial := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()
	if v := treeDynamic(t, initial, "0"); v != "" {
		t.Fatalf("Expected empty initial Pushed, got dynamic 0=%q", v)
	}

	// Server push arrives. readWSUpdate has a 5s timeout and reads the next
	// frame off the socket. A failure to receive it means OnConnect didn't
	// get a session or the goroutine's TriggerAction was a no-op.
	update := readWSUpdate(t, ws, 5*time.Second)
	tree, ok := update["tree"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tree in server-push update, got: %v", update)
	}
	if v := fmt.Sprintf("%v", tree["0"]); v != "hello from goroutine" {
		t.Fatalf("Expected Pushed='hello from goroutine', got dynamic 0=%q", v)
	}

	// Wait for the goroutine to have recorded its result.
	select {
	case <-ctrl.done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutine did not signal completion")
	}

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	if !ctrl.gotSession {
		t.Error("OnConnect received a nil session — WithSession wiring is broken")
	}
	if ctrl.triggered != 1 {
		t.Errorf("Expected TriggerAction to be called once, got %d", ctrl.triggered)
	}
	if ctrl.triggerErr != nil {
		t.Errorf("TriggerAction returned unexpected error: %v", ctrl.triggerErr)
	}
}

// TestLocalSession_TriggerActionDisconnectedReturnsError verifies that a
// TriggerAction call with no live connections and no PubSub broadcaster
// returns an error — the signal that background goroutines use to exit
// cleanly when their session ends.
func TestLocalSession_TriggerActionDisconnectedReturnsError(t *testing.T) {
	auth := &fixedUserAuth{groupID: "disconnect-group", userID: "disconnect-user"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Pushed}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// cancelCtx is unused in this test (no OnConnect goroutine is
	// spawned because no WebSocket is ever connected), but must be
	// non-nil to avoid panicking if OnConnect is accidentally called.
	handler := tmpl.Handle(
		&triggerActionTestController{
			done:      make(chan struct{}),
			cancelCtx: context.Background(),
		},
		AsState(&triggerActionTestState{}),
	)

	// Cast to *liveHandler so we can construct a localSession directly —
	// the same code path that runs inside OnConnect.
	h, ok := handler.(*liveHandler)
	if !ok {
		t.Fatalf("Handle() returned unexpected concrete type %T", handler)
	}
	sess := newLocalSession(h, "never-connected-group")

	// No WebSocket has ever connected for this group, and there's no
	// PubSubBroadcaster, so local+remote delivery both have nothing to do.
	err = sess.TriggerAction("dataLoaded", map[string]interface{}{"data": "test"})
	if err == nil {
		t.Fatal("Expected TriggerAction to return an error when no connections exist, got nil")
	}
	if !strings.Contains(err.Error(), "no connected sessions") {
		t.Errorf("Expected 'no connected sessions' in error, got: %v", err)
	}
}

// chainedTriggerState exercises the dispatched->TriggerAction->dispatched
// re-entry path covered by #337. It records both that the dispatched
// action ran and that the chained call returned without error.
type chainedTriggerState struct {
	First  string
	Second string
}

// chainedTriggerController spawns an OnConnect goroutine that calls
// TriggerAction("first", ...). The First handler — invoked through the
// dispatch queue — chains TriggerAction("second", ...). The flag on the
// localSession built for the dispatched context should make the second
// call emit a debug log line; the first call (from OnConnect) should not.
type chainedTriggerController struct {
	mu        sync.Mutex
	firstRan  bool
	secondRan bool
	chainErr  error
	done      chan struct{}
	doneOnce  sync.Once
	cancelCtx context.Context
}

func (c *chainedTriggerController) OnConnect(state chainedTriggerState, ctx *Context) (chainedTriggerState, error) {
	sess := ctx.Session()
	if sess == nil {
		return state, nil
	}
	go func() {
		select {
		case <-time.After(50 * time.Millisecond):
		case <-c.cancelCtx.Done():
			return
		}
		_ = sess.TriggerAction("first", map[string]interface{}{"data": "from-onconnect"})
	}()
	return state, nil
}

func (c *chainedTriggerController) First(state chainedTriggerState, ctx *Context) (chainedTriggerState, error) {
	state.First = ctx.GetString("data")
	c.mu.Lock()
	c.firstRan = true
	c.mu.Unlock()
	// Chained TriggerAction from inside a dispatched handler — this is the
	// invocation that should emit the #337 observability log line.
	if sess := ctx.Session(); sess != nil {
		c.mu.Lock()
		c.chainErr = sess.TriggerAction("second", map[string]interface{}{"data": "chained"})
		c.mu.Unlock()
	}
	return state, nil
}

func (c *chainedTriggerController) Second(state chainedTriggerState, ctx *Context) (chainedTriggerState, error) {
	state.Second = ctx.GetString("data")
	c.mu.Lock()
	c.secondRan = true
	c.mu.Unlock()
	c.doneOnce.Do(func() { close(c.done) })
	return state, nil
}

// syncBuf is a thread-safe bytes.Buffer wrapper for capturing slog output
// from a goroutine the test does not control. bytes.Buffer is not safe
// for concurrent Write/String, so the slog handler's writes race with the
// test's assertions under `go test -race`.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestLocalSession_TriggerActionFromDispatchedLogsDebug verifies the
// #337 Option A fix: when a dispatched-action handler calls
// ctx.Session().TriggerAction, a slog.Debug line is emitted so that
// runaway recursive chains are detectable in logs. Calls originating
// from OnConnect (not dispatched) must NOT log.
//
// Note: this test mutates the global slog.Default() via SetDefault. Do
// not add t.Parallel() here — concurrent tests would race on the global
// logger. Cleanup restores the previous default.
func TestLocalSession_TriggerActionFromDispatchedLogsDebug(t *testing.T) {
	// Capture slog output via a thread-safe wrapper — the slog handler
	// writes from the dispatch goroutine while assertions read from the
	// test goroutine, so a plain bytes.Buffer would race under -race.
	// Debug level is required because slog.Debug is filtered out by the
	// default handler level.
	var buf syncBuf
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	auth := &fixedUserAuth{groupID: "chained-trigger-group", userID: "chained-user"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.First}}|{{.Second}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ctrl := &chainedTriggerController{
		done:      make(chan struct{}),
		cancelCtx: cancelCtx,
	}
	handler := tmpl.Handle(ctrl, AsState(&chainedTriggerState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Drain server-push frames in the background. We don't care about
	// their content; we only need the WebSocket reader to keep up with
	// the dispatcher so EnqueueDispatch isn't backpressured. The reader
	// exits on socket close (deferred above) or read timeout.
	go func() {
		for {
			if err := ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				return
			}
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	select {
	case <-ctrl.done:
	case <-time.After(5 * time.Second):
		t.Fatal("chained TriggerAction did not run within timeout")
	}

	ctrl.mu.Lock()
	firstRan := ctrl.firstRan
	secondRan := ctrl.secondRan
	chainErr := ctrl.chainErr
	ctrl.mu.Unlock()

	if !firstRan {
		t.Fatal("first action handler did not run")
	}
	if !secondRan {
		t.Fatal("second (chained) action handler did not run")
	}
	if chainErr != nil {
		t.Fatalf("chained TriggerAction returned error: %v", chainErr)
	}

	// Assert: the chained TriggerAction inside First emitted the debug log.
	logged := buf.String()
	if !strings.Contains(logged, "Session.TriggerAction called from within a dispatched action") {
		t.Errorf("expected #337 Option A debug log, not found. Captured:\n%s", logged)
	}
	// Confirm the action name in the log is "second" (the chained call).
	// The OnConnect-originated "first" call must NOT log, so a "action=first"
	// debug line should be absent.
	if !strings.Contains(logged, "action=second") {
		t.Errorf("expected log to reference action=second, got:\n%s", logged)
	}
	if strings.Contains(logged, "action=first") &&
		strings.Contains(logged, "Session.TriggerAction called from within a dispatched action") {
		// Both could appear if the message text matched independently; check
		// the more specific case: same line containing both.
		for _, line := range strings.Split(logged, "\n") {
			if strings.Contains(line, "Session.TriggerAction called from within a dispatched action") &&
				strings.Contains(line, "action=first") {
				t.Errorf("OnConnect-originated TriggerAction(first) must NOT log; got line: %s", line)
			}
		}
	}
}

// TestLocalSession_FromDispatchedFlag is a structural fallback: it
// verifies the constructor variant sets the fromDispatched flag,
// independent of slog capture. If the integration test above is brittle,
// this still validates the wiring change.
func TestLocalSession_FromDispatchedFlag(t *testing.T) {
	auth := &fixedUserAuth{groupID: "g", userID: "u"}
	tmpl, err := New("t", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div></div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(
		&triggerActionTestController{done: make(chan struct{}), cancelCtx: context.Background()},
		AsState(&triggerActionTestState{}),
	)
	h, ok := handler.(*liveHandler)
	if !ok {
		t.Fatalf("Handle() returned unexpected concrete type %T", handler)
	}

	plain := newLocalSession(h, "g")
	if plain.fromDispatched {
		t.Error("newLocalSession should produce fromDispatched=false")
	}

	dispatched := newLocalSessionFromDispatched(h, "g")
	if !dispatched.fromDispatched {
		t.Error("newLocalSessionFromDispatched should produce fromDispatched=true")
	}
}
