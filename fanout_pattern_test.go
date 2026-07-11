package livetemplate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This file is the executable specification for the "fan out to many live
// sessions from a background goroutine WITHOUT a Session-handle registry"
// pattern documented in docs/references/server-actions.md
// ("Fanning out to many sessions"). It locks the two supported shapes so the
// docs cannot drift from behavior:
//
//   - Per-user (all of one user's tabs): ctx.Subscribe(ctx.SelfTopic()) in
//     Mount + handler.Publish(UserTopic(user), action, data) out-of-band.
//     ACL-exempt — no WithTopicACL / WithOpenTopics needed.
//   - Shared cross-connection group (every viewer of one dashboard): a
//     developer topic, which is DENY-ALL by default and needs WithTopicACL or
//     WithOpenTopics — a deliberate security boundary.

// perUserGroupAuth assigns each authenticated user their OWN session group, so
// two connections with different Authorization headers land in DIFFERENT groups.
// This lets the tests distinguish per-user fan-out (SelfTopic/UserTopic) from
// cross-group fan-out (a shared developer topic).
type perUserGroupAuth struct{}

func (a *perUserGroupAuth) Identify(r *http.Request) (string, error) {
	user, _, _ := r.BasicAuth()
	return user, nil
}

func (a *perUserGroupAuth) GetSessionGroup(_ *http.Request, identity string) (string, error) {
	return "group-" + identity, nil
}

// fanoutState is a minimal per-connection state. Tick mirrors a shared counter;
// a refresh that re-renders changes the tree's slot-0 value — the observable
// proof the out-of-band dispatch re-ran render.
type fanoutState struct {
	Tick int
}

// fanoutController models the real-app shape: connections JOIN a live group in
// Mount, and an out-of-band background process refreshes them all. joinTopic
// selects which topic they join (SelfTopic vs a shared developer topic) so one
// controller serves both scenarios.
type fanoutController struct {
	shared    *int
	joinTopic func(ctx *Context) string
}

func (c *fanoutController) Mount(state fanoutState, ctx *Context) (fanoutState, error) {
	if err := ctx.Subscribe(c.joinTopic(ctx)); err != nil {
		return state, err
	}
	state.Tick = *c.shared
	return state, nil
}

func (c *fanoutController) Refresh(state fanoutState, ctx *Context) (fanoutState, error) {
	state.Tick = *c.shared
	return state, nil
}

// TestFanout_PerUserViaSelfTopic_NoRegistry verifies the ACL-EXEMPT path a
// single-user tool (e.g. a local dashboard) needs: a user's connections (all
// their tabs) join via ctx.Subscribe(ctx.SelfTopic()) in Mount, and a
// background goroutine refreshes them all with
// handler.Publish(UserTopic(user), "Refresh", nil) — with NO WithTopicACL /
// WithOpenTopics configured, and no hand-rolled map of Session handles.
func TestFanout_PerUserViaSelfTopic_NoRegistry(t *testing.T) {
	shared := 0
	ctrl := &fanoutController{
		shared:    &shared,
		joinTopic: func(ctx *Context) string { return ctx.SelfTopic() },
	}
	handler, wsURL, closeServer := fanoutServer(t, ctrl) // no ACL options
	defer closeServer()

	// Two tabs of the SAME user (same group), both joined SelfTopic. connectWS
	// reads the connect render, which the server sends only AFTER Mount's
	// Subscribe registers — so the subscription is live once these return.
	wsA := connectWSWithAuth(t, wsURL, "alice")
	defer fanoutClose(t, wsA)
	wsB := connectWSWithAuth(t, wsURL, "alice")
	defer fanoutClose(t, wsB)

	shared = 42
	if err := handler.Publish(UserTopic("alice"), "Refresh", nil); err != nil {
		t.Fatalf("out-of-band Publish failed: %v", err)
	}
	assertRefreshedTo(t, "tab A", wsA, "42")
	assertRefreshedTo(t, "tab B", wsB, "42")
}

// TestFanout_SharedGroupRequiresTopicACL pins the DELIBERATE constraint: a
// shared cross-group live group (different users all watching one dashboard)
// uses a developer topic, which is DENY-ALL by default. Without WithOpenTopics
// the Subscribe is denied so nobody is on the topic; with it, one out-of-band
// handler.Publish reaches every group. This is the security boundary the
// documented pattern must not silently erase.
func TestFanout_SharedGroupRequiresTopicACL(t *testing.T) {
	t.Run("denied_without_acl", func(t *testing.T) {
		shared := 0
		ctrl := &fanoutController{
			shared:    &shared,
			joinTopic: func(ctx *Context) string { return "dashboard" },
		}
		handler, wsURL, closeServer := fanoutServer(t, ctrl) // NO ACL option → deny-all
		defer closeServer()

		ws := connectWSWithAuth(t, wsURL, "alice")
		defer fanoutClose(t, ws)

		// The Subscribe was ACL-denied, so nobody is registered on "dashboard".
		// (Asserted via the registry rather than a WS read-timeout, which would
		// corrupt the gorilla connection.)
		if n := fanoutSubscriberCount(handler, "dashboard"); n != 0 {
			t.Errorf("expected 0 subscribers on developer topic without ACL, got %d", n)
		}
	})

	t.Run("delivered_with_open_topics", func(t *testing.T) {
		shared := 0
		ctrl := &fanoutController{
			shared:    &shared,
			joinTopic: func(ctx *Context) string { return "dashboard" },
		}
		handler, wsURL, closeServer := fanoutServer(t, ctrl, WithOpenTopics())
		defer closeServer()

		// Two DIFFERENT users (different groups) both joined "dashboard".
		wsA := connectWSWithAuth(t, wsURL, "alice")
		defer fanoutClose(t, wsA)
		wsB := connectWSWithAuth(t, wsURL, "bob")
		defer fanoutClose(t, wsB)

		if n := fanoutSubscriberCount(handler, "dashboard"); n != 2 {
			t.Fatalf("expected 2 cross-group subscribers with WithOpenTopics, got %d", n)
		}

		shared = 7
		if err := handler.Publish("dashboard", "Refresh", nil); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
		assertRefreshedTo(t, "alice", wsA, "7")
		assertRefreshedTo(t, "bob", wsB, "7")
	})
}

// fanoutServer builds a handler + httptest server for the pattern controller.
func fanoutServer(t *testing.T, ctrl *fanoutController, opts ...Option) (LiveHandler, string, func()) {
	t.Helper()
	allOpts := append([]Option{WithAuthenticator(&perUserGroupAuth{})}, opts...)
	tmpl, err := New("dash", allOpts...)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Tick}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(ctrl, AsState(&fanoutState{}))
	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	return handler, wsURL, server.Close
}

// fanoutSubscriberCount reports how many live connections are subscribed to
// topic — the deterministic way to prove an ACL admitted or denied a Subscribe
// without relying on a WS read (a read-deadline timeout permanently corrupts
// the gorilla connection, so "assert nothing arrives" is not a safe WS check).
func fanoutSubscriberCount(handler LiveHandler, topic string) int {
	h := handler.(*liveHandler)
	return len(h.registry.GetByTopicExcept(topic, nil, segmentMatch))
}

// assertRefreshedTo reads the next WS message and fails unless it is a
// successful tree update whose slot-0 value equals want — the observable proof
// that the out-of-band dispatch re-ran Refresh and re-rendered with new data.
// (Out-of-band dispatch responses carry no meta.action, so tree state — not an
// action echo — is the correct signal.)
func assertRefreshedTo(t *testing.T, who string, ws *websocket.Conn, want string) {
	t.Helper()
	got := readWSMessage(ws, 3*time.Second)
	if got == nil {
		t.Errorf("%s: never received out-of-band Refresh dispatch", who)
		return
	}
	meta, _ := got["meta"].(map[string]interface{})
	if meta == nil || meta["success"] != true {
		t.Errorf("%s: expected a successful tree update, got %v", who, got)
		return
	}
	tree, _ := got["tree"].(map[string]interface{})
	if tree == nil {
		t.Errorf("%s: dispatch carried no tree: %v", who, got)
		return
	}
	if v := tree["0"]; v != want {
		t.Errorf("%s: refresh rendered slot0=%v, want %q", who, v, want)
	}
}

func fanoutClose(t *testing.T, ws *websocket.Conn) {
	t.Helper()
	if err := ws.Close(); err != nil {
		t.Logf("ws close: %v", err)
	}
}

// readWSMessage reads one message within timeout, returning nil on timeout/error.
// A returned timeout corrupts the connection for further reads, so callers must
// not read again after a nil (each test reads exactly once per socket).
func readWSMessage(ws *websocket.Conn, timeout time.Duration) map[string]interface{} {
	if err := ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		return nil
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return nil
	}
	return resp
}
