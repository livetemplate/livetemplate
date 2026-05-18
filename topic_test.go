package livetemplate

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Tier-1 integration tests for the Publish/Subscribe topic model (Phase 1).
// Gate: V1–V7, V10–V12, V16, V19, V21 (single-instance; no Redis).
//
// Assertion strategy:
//   - Error V-items (V5/V6/V7/V16): the controller is a singleton the test
//     holds; its action stores the *real* error from ctx.Subscribe, so
//     errors.Is(…, ErrTopicForbidden) is asserted on the actual value.
//   - Dispatch V-items (V1/V3/V10/V11/V21): observe WS messages.
//   - Log V-items (V11/V19): tee slog to a buffer.
// ============================================================================

// --- Authenticator fakes ---

// perDeviceAuth: one fixed authenticated user, a per-connection groupID keyed
// off ?device=. Proves SelfTopic() keys on UserID (spans devices/groups) — V1.
type perDeviceAuth struct{ userID string }

func (a *perDeviceAuth) Identify(_ *http.Request) (string, error) { return a.userID, nil }
func (a *perDeviceAuth) GetSessionGroup(r *http.Request, _ string) (string, error) {
	return "grp-" + r.URL.Query().Get("device"), nil
}

// --- slog capture ---

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// captureSlog redirects the default slog logger to a buffer for the duration
// of the test (debug level so the recursion-guard slog.Error and the
// symmetry-collision slog.Warn are both captured). Not parallel-safe — these
// tests do not call t.Parallel().
func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	prev := slog.Default()
	sb := &syncBuffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(sb, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return sb
}

// --- Controllers ---

// aclProbeController stores the real error returned by ctx.Subscribe so error
// V-items can assert errors.Is on the actual value. `sub` subscribes
// data["topic"]; `subself` subscribes ctx.SelfTopic().
type aclProbeController struct {
	mu       sync.Mutex
	lastErr  error
	selfSeen string
}

func (c *aclProbeController) Mount(s probeState, _ *Context) (probeState, error) { return s, nil }

func (c *aclProbeController) Sub(s probeState, ctx *Context) (probeState, error) {
	err := ctx.Subscribe(ctx.GetString("topic"))
	c.mu.Lock()
	c.lastErr = err
	c.mu.Unlock()
	return s, nil
}

func (c *aclProbeController) Subself(s probeState, ctx *Context) (probeState, error) {
	c.mu.Lock()
	c.selfSeen = ctx.SelfTopic()
	c.lastErr = ctx.Subscribe(ctx.SelfTopic())
	c.mu.Unlock()
	return s, nil
}

func (c *aclProbeController) err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

type probeState struct{ N int }

// selfSyncController: Mount subscribes SelfTopic(); Add mutates the shared
// store and Publishes a Reload; Reload is a reconciler that re-reads ONLY the
// shared field; SetPanel writes a per-connection field the reconciler never
// touches; Get is a no-op render barrier.
type selfSyncController struct {
	mu    sync.Mutex
	items []string
}

type syncState struct {
	Items []string
	Panel string
}

func (c *selfSyncController) Mount(s syncState, ctx *Context) (syncState, error) {
	_ = ctx.Subscribe(ctx.SelfTopic())
	c.mu.Lock()
	s.Items = append([]string(nil), c.items...)
	c.mu.Unlock()
	return s, nil
}

func (c *selfSyncController) Add(s syncState, ctx *Context) (syncState, error) {
	c.mu.Lock()
	c.items = append(c.items, ctx.GetString("text"))
	c.mu.Unlock()
	if err := ctx.Publish(ctx.SelfTopic(), "Reload", nil); err != nil {
		return s, err
	}
	return s, nil
}

func (c *selfSyncController) Reload(s syncState, _ *Context) (syncState, error) {
	c.mu.Lock()
	s.Items = append([]string(nil), c.items...) // shared only
	c.mu.Unlock()
	return s, nil // s.Panel (per-connection) deliberately untouched → preserved
}

func (c *selfSyncController) SetPanel(s syncState, ctx *Context) (syncState, error) {
	s.Panel = ctx.GetString("v")
	return s, nil
}

// Bump appends "!" to the per-connection Panel and renders it. Reading Panel
// back through a value-changing action is the robust Tier-1 way to observe
// each receiver's own connState post-reconcile (the wire format only carries
// CHANGED dynamics, so an unchanged field never reappears in a later diff).
func (c *selfSyncController) Bump(s syncState, _ *Context) (syncState, error) {
	s.Panel += "!"
	return s, nil
}

func setupTopicServer(t *testing.T, ctrl interface{}, st State, tmplStr string, opts ...Option) (*httptest.Server, string) {
	t.Helper()
	tmpl, err := New("test", opts...)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if tmpl, err = tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	server := httptest.NewServer(tmpl.Handle(ctrl, st))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	return server, wsURL
}

func wsAction(t *testing.T, ws *websocket.Conn, action string, data map[string]interface{}) {
	t.Helper()
	b, _ := json.Marshal(map[string]interface{}{"action": action, "data": data})
	if err := ws.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("WS write %q failed: %v", action, err)
	}
}

// expectNoMessage asserts ws receives nothing within d (sender exclusion / not
// reconnect-durable / recursion-guard drop).
func expectNoMessage(t *testing.T, ws *websocket.Conn, d time.Duration, msg string) {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(d)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	if _, _, err := ws.ReadMessage(); err == nil {
		t.Error(msg)
	}
}

// ============================================================================
// V1 — self-sync, two devices, one user (per-device groupIDs).
// V3 — per-connection field preserved by the selective reconciler (K).
// V10 — sender exclusion.
// ============================================================================

func TestTopic_V1_V3_V10_SelfSyncTwoDevices(t *testing.T) {
	ctrl := &selfSyncController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&syncState{}),
		`<div>{{range .Items}}{{.}}|{{end}}#{{.Panel}}</div>`,
		WithAuthenticator(&perDeviceAuth{userID: "alice"}))
	defer server.Close()

	dev1 := connectWS(t, wsURL+"?device=1")
	defer func() { _ = dev1.Close() }()
	dev2 := connectWS(t, wsURL+"?device=2")
	defer func() { _ = dev2.Close() }()

	// dev2 sets a per-connection field (V3): the reconciler must never write it.
	wsAction(t, dev2, "set_panel", map[string]interface{}{"v": "PANEL2"})
	_ = readWSUpdate(t, dev2, 3*time.Second) // dev2 action response

	// dev1 mutates the shared store and Publishes Reload to SelfTopic().
	wsAction(t, dev1, "add", map[string]interface{}{"text": "hello"})
	_ = readWSUpdate(t, dev1, 3*time.Second) // dev1's own action response

	// V1: dev2 (different groupID, SAME user) runs the Reload reconciler and
	// gets a diff carrying the reconciled shared Items — proving SelfTopic()
	// keys on UserID and spans groups, where BroadcastAction cannot.
	reload := rawWSUpdate(t, dev2, 3*time.Second)
	if !strings.Contains(reload, "hello") {
		t.Fatalf("V1: dev2 did not receive the reconciled shared Items: %s", reload)
	}

	// V10: dev1 (sender) must NOT receive its own Publish.
	expectNoMessage(t, dev1, 300*time.Millisecond, "V10: sender received its own Publish")

	// V3 (K): dev2's per-connection Panel must have survived the Reload (the
	// reconciler writes only Items). Read it back through a value-changing
	// action — if Reload had wrongly written Panel it would be "!" not
	// "PANEL2!". This observes dev2's OWN connState post-reconcile.
	wsAction(t, dev2, "bump", nil)
	bumped := rawWSUpdate(t, dev2, 3*time.Second)
	if !strings.Contains(bumped, "PANEL2!") {
		t.Errorf("V3: dev2 per-connection Panel not preserved through reconcile (want \"PANEL2!\"): %s", bumped)
	}
}

func rawWSUpdate(t *testing.T, ws *websocket.Conn, timeout time.Duration) string {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("WS read failed: %v", err)
	}
	return string(msg)
}

// ============================================================================
// V2 — self-sync, anonymous (SelfTopic() → lvt:session:<groupID>).
// ============================================================================

func TestTopic_V2_SelfSyncAnonymous(t *testing.T) {
	ctrl := &selfSyncController{}
	// fixedGroupAuth: Identify→"" (anonymous), constant groupID → both tabs
	// share lvt:session:test-group.
	server, wsURL := setupTopicServer(t, ctrl, AsState(&syncState{}),
		`<div>{{range .Items}}{{.}}|{{end}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "test-group"}))
	defer server.Close()

	tab1 := connectWS(t, wsURL)
	defer func() { _ = tab1.Close() }()
	tab2 := connectWS(t, wsURL)
	defer func() { _ = tab2.Close() }()

	wsAction(t, tab1, "add", map[string]interface{}{"text": "anon-item"})
	_ = readWSUpdate(t, tab1, 3*time.Second)

	if u := readWSUpdate(t, tab2, 3*time.Second); u["meta"] == nil {
		t.Fatalf("V2: tab2 expected anonymous self-sync dispatch, got %v", u)
	}
}

// ============================================================================
// V5 — ACL denied → ErrTopicForbidden.
// V6 — only SelfTopic() is ACL-exempt under deny-all.
// V7 — reserved-namespace anti-spoof.
// V16 — deny-all default / WithOpenTopics / both-set hard error at New().
// ============================================================================

func subProbe(t *testing.T, wsURL, topic string) {
	t.Helper()
	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()
	wsAction(t, ws, "sub", map[string]interface{}{"topic": topic})
	_ = readWSUpdate(t, ws, 3*time.Second) // barrier: action ran
}

func TestTopic_V5_ACLDenied(t *testing.T) {
	ctrl := &aclProbeController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}),
		WithTopicACL(func(topic, _ string, _ *http.Request) (bool, error) {
			return topic != "private/admin", nil
		}))
	defer server.Close()

	subProbe(t, wsURL, "private/admin")
	if !errors.Is(ctrl.err(), ErrTopicForbidden) {
		t.Fatalf("V5: expected ErrTopicForbidden for denied topic, got %v", ctrl.err())
	}
	var tfe *TopicForbiddenError
	if !errors.As(ctrl.err(), &tfe) || tfe.Topic != "private/admin" {
		t.Errorf("V5: expected *TopicForbiddenError carrying the topic, got %v", ctrl.err())
	}

	ctrl.lastErr = errors.New("sentinel")
	subProbe(t, wsURL, "public/feed")
	if ctrl.err() != nil {
		t.Errorf("V5: allowed topic should subscribe cleanly, got %v", ctrl.err())
	}
}

func TestTopic_V6_OnlySelfACLExemptUnderDenyAll(t *testing.T) {
	ctrl := &aclProbeController{}
	// Deny-all: neither WithTopicACL nor WithOpenTopics.
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}))
	defer server.Close()

	// SelfTopic() is the only ACL-exempt topic → succeeds under deny-all.
	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()
	wsAction(t, ws, "subself", nil)
	_ = readWSUpdate(t, ws, 3*time.Second)
	if ctrl.err() != nil {
		t.Errorf("V6: Subscribe(SelfTopic()) must be ACL-exempt under deny-all, got %v", ctrl.err())
	}

	// A developer / all-users channel needs an explicit allow — no carve-out.
	for _, topic := range []string{"announcements", "room/1"} {
		ctrl.lastErr = nil
		wsAction(t, ws, "sub", map[string]interface{}{"topic": topic})
		_ = readWSUpdate(t, ws, 3*time.Second)
		if !errors.Is(ctrl.err(), ErrTopicForbidden) {
			t.Errorf("V6: %q must be denied under deny-all (no global carve-out), got %v", topic, ctrl.err())
		}
	}
}

func TestTopic_V7_ReservedNamespaceAntiSpoof(t *testing.T) {
	ctrl := &aclProbeController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&perDeviceAuth{userID: "bob"}), WithOpenTopics())
	defer server.Close()

	// bob subscribing alice's identity topic must be rejected (not the caller's
	// own SelfTopic()), even under WithOpenTopics — anti-spoof precedes the ACL.
	subProbe(t, wsURL+"?device=1", UserTopic("alice"))
	if ctrl.err() == nil {
		t.Fatalf("V7: subscribing another user's lvt:user: topic must be rejected")
	}
	if errors.Is(ctrl.err(), ErrTopicForbidden) {
		t.Errorf("V7: anti-spoof rejection is a reserved-namespace error, not the ACL ErrTopicForbidden: %v", ctrl.err())
	}

	// A non-self lvt: string in general is rejected.
	ctrl.lastErr = nil
	subProbe(t, wsURL+"?device=1", "lvt:user:bob-but-not-self")
	if ctrl.err() == nil {
		t.Errorf("V7: any non-self lvt: subscribe must be rejected")
	}
}

func TestTopic_V16_DenyAllDefaultAndOpenTopicsAndBothSet(t *testing.T) {
	// Deny-all default: any developer topic denied.
	ctrl := &aclProbeController{}
	srv, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}))
	subProbe(t, wsURL, "anything")
	if !errors.Is(ctrl.err(), ErrTopicForbidden) {
		t.Errorf("V16: deny-all default must reject %q, got %v", "anything", ctrl.err())
	}
	srv.Close()

	// WithOpenTopics: everything permitted.
	ctrl2 := &aclProbeController{}
	srv2, wsURL2 := setupTopicServer(t, ctrl2, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	subProbe(t, wsURL2, "anything")
	if ctrl2.err() != nil {
		t.Errorf("V16: WithOpenTopics must permit %q, got %v", "anything", ctrl2.err())
	}
	srv2.Close()

	// Both set → hard error at New(), order-independent.
	if _, err := New("x", WithTopicACL(func(string, string, *http.Request) (bool, error) { return true, nil }), WithOpenTopics()); err == nil {
		t.Error("V16: New() must reject WithTopicACL+WithOpenTopics (ACL-first order)")
	}
	if _, err := New("x", WithOpenTopics(), WithTopicACL(func(string, string, *http.Request) (bool, error) { return true, nil })); err == nil {
		t.Error("V16: New() must reject WithOpenTopics+WithTopicACL (open-first order)")
	}
}

// ============================================================================
// V4 — cross-user public topic under WithOpenTopics.
// ============================================================================

type roomController struct{}

func (c *roomController) Mount(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Subscribe("public/feed")
	return s, nil
}
func (c *roomController) Ping(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Publish("public/feed", "Pong", map[string]interface{}{"n": 1})
	return s, nil
}
func (c *roomController) Pong(s probeState, ctx *Context) (probeState, error) {
	s.N = ctx.GetInt("n")
	return s, nil
}

func TestTopic_V4_CrossUserPublicTopic(t *testing.T) {
	server, wsURL := setupTopicServer(t, &roomController{}, AsState(&probeState{}),
		`<div>{{.N}}</div>`, WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	a := connectWS(t, wsURL)
	defer func() { _ = a.Close() }()
	b := connectWS(t, wsURL)
	defer func() { _ = b.Close() }()

	wsAction(t, a, "ping", nil)
	_ = readWSUpdate(t, a, 3*time.Second) // a's own response
	if u := readWSUpdate(t, b, 3*time.Second); u["meta"] == nil {
		t.Fatalf("V4: peer b expected Pong dispatch on public/feed, got %v", u)
	}
	expectNoMessage(t, a, 300*time.Millisecond, "V4: publisher received its own Publish")
}

// ============================================================================
// V11 — recursion guard (Publish inside a dispatched action is dropped+logged).
// V19 — symmetry-collision warning (wired action) + no false positive.
// ============================================================================

type recursionController struct {
	mu sync.Mutex
	n  []string
}

func (c *recursionController) Mount(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Subscribe(ctx.SelfTopic())
	return s, nil
}
func (c *recursionController) Kick(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Publish(ctx.SelfTopic(), "Reload", nil)
	return s, nil
}

// Reload is topic-only (NOT wired to any client element) and itself re-Publishes
// — the recursion guard must drop+log the nested Publish.
func (c *recursionController) Reload(s probeState, ctx *Context) (probeState, error) {
	c.mu.Lock()
	c.n = append(c.n, "reload")
	c.mu.Unlock()
	_ = ctx.Publish(ctx.SelfTopic(), "Reload", nil) // must be dropped
	return s, nil
}

func TestTopic_V11_RecursionGuard(t *testing.T) {
	sb := captureSlog(t)
	ctrl := &recursionController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}),
		`<div>{{.N}}</div>`, WithAuthenticator(&fixedGroupAuth{groupID: "g"}))
	defer server.Close()

	sender := connectWS(t, wsURL)
	defer func() { _ = sender.Close() }()
	peer := connectWS(t, wsURL)
	defer func() { _ = peer.Close() }()

	wsAction(t, sender, "kick", nil)
	_ = readWSUpdate(t, sender, 3*time.Second) // sender's own response
	_ = readWSUpdate(t, peer, 3*time.Second)   // peer runs Reload once

	// The nested Publish from within the dispatched Reload is dropped+logged,
	// so the peer must NOT receive a second dispatch (one-hop bound).
	expectNoMessage(t, peer, 400*time.Millisecond, "V11: nested Publish was not dropped (peer got a 2nd dispatch)")

	if !strings.Contains(sb.String(), "Publish calls inside a") {
		t.Errorf("V11: expected the recursion-guard log, not found in:\n%s", sb.String())
	}
}

// symmetryController has a Delete method wired to <button name="Delete"> and a
// topic-only Reload (no client element).
type symmetryController struct{}

func (c *symmetryController) Mount(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Subscribe(ctx.SelfTopic())
	return s, nil
}
func (c *symmetryController) Delete(s probeState, _ *Context) (probeState, error) { return s, nil }
func (c *symmetryController) Reload(s probeState, _ *Context) (probeState, error) { return s, nil }
func (c *symmetryController) PubDelete(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Publish(ctx.SelfTopic(), "Delete", nil)
	return s, nil
}
func (c *symmetryController) PubReload(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Publish(ctx.SelfTopic(), "Reload", nil)
	return s, nil
}

func TestTopic_V19_SymmetryCollisionWarning(t *testing.T) {
	sb := captureSlog(t)
	server, wsURL := setupTopicServer(t, &symmetryController{}, AsState(&probeState{}),
		`<form><button name="Delete">x</button></form>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}))
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	// "Delete" IS wired (<button name="Delete">) → collision warning.
	wsAction(t, ws, "pub_delete", nil)
	_ = readWSUpdate(t, ws, 3*time.Second)
	if !strings.Contains(sb.String(), "Publish action name collides with a client-wired action") {
		t.Errorf("V19: expected collision warning for wired \"Delete\", not in:\n%s", sb.String())
	}

	// "Reload" is topic-only (not wired) → NO warning (no false positive).
	sb2 := captureSlog(t)
	wsAction(t, ws, "pub_reload", nil)
	_ = readWSUpdate(t, ws, 3*time.Second)
	if strings.Contains(sb2.String(), "collides with a client-wired action") {
		t.Errorf("V19: false-positive collision warning for topic-only \"Reload\":\n%s", sb2.String())
	}
}

// ============================================================================
// V12 — Subscribe on a plain HTTP GET: no error, normal render, ACL still ran.
// ============================================================================

func TestTopic_V12_SubscribeOnHTTPGet(t *testing.T) {
	var mu sync.Mutex
	aclCalls := 0
	// roomController.Mount subscribes the developer topic "public/feed", so a
	// plain GET exercises the eager-ACL-on-GET path (SelfTopic() would be
	// ACL-exempt and not prove the eager run).
	server, wsURL := setupTopicServer(t, &roomController{}, AsState(&probeState{}),
		`<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}),
		WithTopicACL(func(topic, _ string, _ *http.Request) (bool, error) {
			mu.Lock()
			aclCalls++
			mu.Unlock()
			return true, nil
		}))
	defer server.Close()

	// Plain HTTP GET: normal render, no error; the subscription itself is a
	// no-op (no Connection) but the ACL ran eagerly.
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("V12: GET must render normally (Subscribe-on-GET is a subscription no-op, not an error), got %d", resp.StatusCode)
	}
	mu.Lock()
	ranOnGet := aclCalls
	mu.Unlock()
	if ranOnGet == 0 {
		t.Errorf("V12: ACL must run eagerly even on a plain HTTP GET")
	}

	// Upgrade to WS for the same identity → the subscription now materializes:
	// a peer's Publish to public/feed reaches this connection.
	a := connectWS(t, wsURL)
	defer func() { _ = a.Close() }()
	b := connectWS(t, wsURL)
	defer func() { _ = b.Close() }()
	wsAction(t, b, "ping", nil)
	_ = readWSUpdate(t, b, 3*time.Second)
	if u := readWSUpdate(t, a, 3*time.Second); u["meta"] == nil {
		t.Fatalf("V12: WS subscription did not materialize (peer Publish not delivered): %v", u)
	}
}

// ============================================================================
// V21 — Subscribe-in-action is not reconnect-durable; Subscribe-in-Mount is.
//
// fixedGroupAuth makes a re-dial the SAME identity by construction (constant
// groupID, no cookie needed) — so the V21 "same-session reconnect helper" the
// spec allows scoping reduces to a plain re-dial. Recorded in phase-1.md.
// ============================================================================

type inActionSubController struct{}

func (c *inActionSubController) Mount(s probeState, _ *Context) (probeState, error) {
	return s, nil // deliberately does NOT subscribe "t"
}
func (c *inActionSubController) Arm(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Subscribe("t") // connection-lifetime only — never re-run on reconnect
	return s, nil
}
func (c *inActionSubController) Fire(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Publish("t", "Hit", nil)
	return s, nil
}
func (c *inActionSubController) Hit(s probeState, _ *Context) (probeState, error) { return s, nil }

type mountSubController struct{ inActionSubController }

func (c *mountSubController) Mount(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Subscribe("t") // reconnect-durable: Mount re-runs on reconnect
	return s, nil
}

func TestTopic_V21_InActionSubscribeNotReconnectDurable(t *testing.T) {
	ctrl := &inActionSubController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	sub := connectWS(t, wsURL)
	wsAction(t, sub, "arm", nil) // subscribe "t" inside an action
	_ = readWSUpdate(t, sub, 3*time.Second)

	pub := connectWS(t, wsURL)
	defer func() { _ = pub.Close() }()
	wsAction(t, pub, "fire", nil)
	_ = readWSUpdate(t, pub, 3*time.Second)
	if u := readWSUpdate(t, sub, 3*time.Second); u["meta"] == nil {
		t.Fatalf("V21 precondition: in-action subscriber should receive the pre-drop Publish")
	}

	// Drop + reconnect (same identity via fixedGroupAuth). Mount does NOT
	// re-subscribe "t", so the reconnected connection must NOT be delivered.
	_ = sub.Close()
	sub2 := connectWS(t, wsURL)
	defer func() { _ = sub2.Close() }()
	wsAction(t, pub, "fire", nil)
	_ = readWSUpdate(t, pub, 3*time.Second)
	expectNoMessage(t, sub2, 500*time.Millisecond,
		"V21: in-action subscription wrongly survived reconnect (must be Mount-durable only)")

	// Control: a controller that subscribes "t" in Mount IS reconnect-durable.
	ctrl2 := &mountSubController{}
	srvM, wsURLM := setupTopicServer(t, ctrl2, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "gm"}), WithOpenTopics())
	defer srvM.Close()
	m := connectWS(t, wsURLM)
	_ = m.Close()
	m2 := connectWS(t, wsURLM) // reconnect → Mount re-subscribes "t"
	defer func() { _ = m2.Close() }()
	pubM := connectWS(t, wsURLM)
	defer func() { _ = pubM.Close() }()
	wsAction(t, pubM, "fire", nil)
	_ = readWSUpdate(t, pubM, 3*time.Second)
	if u := readWSUpdate(t, m2, 3*time.Second); u["meta"] == nil {
		t.Errorf("V21: Mount-established subscription must survive reconnect")
	}
}
