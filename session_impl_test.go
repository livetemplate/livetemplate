package livetemplate

import (
	"context"
	"fmt"
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
