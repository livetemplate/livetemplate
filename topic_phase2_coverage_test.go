package livetemplate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/internal/session"
	"github.com/livetemplate/livetemplate/internal/upload"
	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// ============================================================================
// Phase 2 — deferred-coverage tests accepted from the PR #419 Claude-bot
// review (phase-1.md "Deferred Phase 2 test coverage") plus the Phase 1
// round-4 symmetry-collision normalization fix. Single-instance; no Redis.
//
//   1. ctx.Publish invalid (grammar-rejected) topic
//   2. ctx.Publish at the MaxBroadcastsPerAction cap
//   3. ctx.Unsubscribe stops delivery
//   4. server-originated Subscribe (nil r) → deny-by-default + ErrNoRequestContext
//   5. ctx.Publish from an upload-complete handler (the 28b8f6cb drain)
//   6. WS topic_forbidden envelope emitted on the ACL-denied WS-connect path
//   7. symmetry-collision warning now catches a style mismatch (Save vs save)
// ============================================================================

// p2ErrController stores the error returned by the ctx.Publish under test, the
// same singleton-holds-the-real-error strategy aclProbeController uses.
type p2ErrController struct {
	mu  sync.Mutex
	err error
}

func (c *p2ErrController) Mount(s probeState, _ *Context) (probeState, error) { return s, nil }
func (c *p2ErrController) setErr(e error) {
	c.mu.Lock()
	c.err = e
	c.mu.Unlock()
}
func (c *p2ErrController) lastErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *p2ErrController) PubBadTopic(s probeState, ctx *Context) (probeState, error) {
	// A space is not in the segment grammar [a-zA-Z0-9_-]+ → rejected by
	// ctx.Publish before queueing (send still validates topic grammar even
	// though it runs no ACL).
	c.setErr(ctx.Publish("bad topic", "X", nil))
	return s, nil
}

func (c *p2ErrController) PubSpam(s probeState, ctx *Context) (probeState, error) {
	// Cap is len(topicPubs) >= MaxBroadcastsPerAction: the first
	// MaxBroadcastsPerAction calls append, the next returns the cap error.
	var last error
	for i := 0; i <= MaxBroadcastsPerAction; i++ {
		last = ctx.Publish("cap/x", "A", nil)
	}
	c.setErr(last)
	return s, nil
}

// 1. Invalid (grammar) topic → error, nothing queued.
func TestTopic_Phase2_PublishInvalidTopic(t *testing.T) {
	ctrl := &p2ErrController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()
	sendWSAction(t, ws, "pub_bad_topic", nil)
	// The action still returns its own normal render response; drain it as a
	// barrier. The proof the invalid topic was rejected *before* queueing is
	// the returned error, not the absence of a dispatch.
	_ = ws.SetReadDeadline(time.Now().Add(time.Second))
	_, _, _ = ws.ReadMessage()

	err := ctrl.lastErr()
	if err == nil {
		t.Fatal("expected an error for an invalid (grammar) topic, got nil")
	}
	if !strings.Contains(err.Error(), "bad topic") && !strings.Contains(err.Error(), "segment") && !strings.Contains(err.Error(), "topic") {
		t.Errorf("error does not look like a grammar rejection: %v", err)
	}
}

// 2. MaxBroadcastsPerAction cap → the over-cap call errors.
func TestTopic_Phase2_PublishCap(t *testing.T) {
	ctrl := &p2ErrController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()
	sendWSAction(t, ws, "pub_spam", nil)
	// Drain barrier: the action ran (its render response, if any).
	_ = ws.SetReadDeadline(time.Now().Add(time.Second))
	_, _, _ = ws.ReadMessage()

	err := ctrl.lastErr()
	if err == nil {
		t.Fatalf("expected the over-cap Publish to error (cap=%d)", MaxBroadcastsPerAction)
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Errorf("error does not mention the cap: %v", err)
	}
}

// p2UnsubController: Mount subscribes the topic; Drop unsubscribes it (and
// flips a visible field so the WS render is a synchronization barrier).
type p2UnsubState struct {
	Msg     string
	Dropped bool
}
type p2UnsubController struct{}

func (c *p2UnsubController) Mount(s p2UnsubState, ctx *Context) (p2UnsubState, error) {
	return s, ctx.Subscribe("t/x")
}
func (c *p2UnsubController) Drop(s p2UnsubState, ctx *Context) (p2UnsubState, error) {
	ctx.Unsubscribe("t/x")
	s.Dropped = true
	return s, nil
}
func (c *p2UnsubController) Reload(s p2UnsubState, ctx *Context) (p2UnsubState, error) {
	s.Msg = ctx.GetString("msg")
	return s, nil
}

// 3. ctx.Unsubscribe removes delivery (the connection no longer receives a
// subsequent Publish to that topic).
func TestTopic_Phase2_Unsubscribe(t *testing.T) {
	server, wsURL, h := setupTopicServerH(t, &p2UnsubController{}, AsState(&p2UnsubState{}),
		`<div>{{.Msg}}{{if .Dropped}} dropped{{end}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	sendWSAction(t, ws, "drop", nil)
	if u := readWSUpdate(t, ws, 3*time.Second); !strings.Contains(toJSON(u), "dropped") {
		t.Fatalf("Drop barrier not observed (Unsubscribe may not have run): %v", u)
	}

	// Out-of-band Publish to the now-unsubscribed topic must not reach ws.
	if err := h.Publish("t/x", "Reload", map[string]interface{}{"msg": "after"}); err != nil {
		t.Fatalf("handler.Publish failed: %v", err)
	}
	expectNoMessage(t, ws, time.Second, "Unsubscribed connection must not receive the topic")
}

// p2NilRController: a connection's action Publishes to SelfTopic; the peer in
// the same anonymous group runs Probe via DISPATCH (server-originated, r=nil)
// and records the ctx.Subscribe error there.
type p2NilRState struct{ Probed bool }
type p2NilRController struct {
	mu  sync.Mutex
	err error
	hit bool
}

func (c *p2NilRController) Mount(s p2NilRState, ctx *Context) (p2NilRState, error) {
	_ = ctx.Subscribe(ctx.SelfTopic()) // ACL-exempt
	return s, nil
}
func (c *p2NilRController) Trigger(s p2NilRState, ctx *Context) (p2NilRState, error) {
	return s, ctx.Publish(ctx.SelfTopic(), "Probe", nil)
}
func (c *p2NilRController) Probe(s p2NilRState, ctx *Context) (p2NilRState, error) {
	err := ctx.Subscribe("dev/topic") // server-originated: r == nil
	c.mu.Lock()
	c.err, c.hit = err, true
	c.mu.Unlock()
	s.Probed = true
	return s, nil
}

// 4. A developer-topic Subscribe from a server-originated (r==nil) dispatched
// action is denied by default with Cause ErrNoRequestContext (the audit-driven
// nil-r hardening), even though WithTopicACL is configured.
func TestTopic_Phase2_ServerOriginatedSubscribeNilRequest(t *testing.T) {
	ctrl := &p2NilRController{}
	server, wsURL := setupTopicServer(t, ctrl, AsState(&p2NilRState{}),
		`<div>{{if .Probed}}probed{{end}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}),
		WithTopicACL(func(string, string, *http.Request) (bool, error) { return false, nil }))
	defer server.Close()

	conn1 := connectWS(t, wsURL)
	defer func() { _ = conn1.Close() }()
	conn2 := connectWS(t, wsURL) // same anonymous group → same SelfTopic
	defer func() { _ = conn2.Close() }()

	// conn1 publishes to the shared SelfTopic; sender-excluded, so conn2 (the
	// peer) runs Probe via dispatch with a nil request.
	sendWSAction(t, conn1, "trigger", nil)
	if u := readWSUpdate(t, conn2, 3*time.Second); !strings.Contains(toJSON(u), "probed") {
		t.Fatalf("Probe barrier not observed on the peer: %v", u)
	}

	ctrl.mu.Lock()
	hit, err := ctrl.hit, ctrl.err
	ctrl.mu.Unlock()
	if !hit {
		t.Fatal("Probe never ran on the peer connection")
	}
	if !errors.Is(err, ErrTopicForbidden) {
		t.Errorf("expected ErrTopicForbidden, got %v", err)
	}
	if !errors.Is(err, ErrNoRequestContext) {
		t.Errorf("expected Cause ErrNoRequestContext (nil-r hardening), got %v", err)
	}
}

// p2UploadState / controller: the upload-complete handler explicitly Publishes,
// which must drain (the 28b8f6cb fix) and dispatch to a subscribed peer.
type p2UploadState struct{ Avatar string }
type p2UploadController struct{}

func (c *p2UploadController) UploadAvatarComplete(s p2UploadState, ctx *Context) (p2UploadState, error) {
	return s, ctx.Publish("t/x", "Reload", map[string]interface{}{"msg": "uploaded"})
}

// 5. ctx.Publish from an upload-complete handler drains and dispatches to a
// peer subscribed to that topic (handleUploadComplete drives processTopicPublishes).
func TestTopic_Phase2_PublishFromUploadComplete(t *testing.T) {
	tmpl, err := New("test", WithUpload("avatar", UploadConfig{}))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if tmpl, err = tmpl.Parse(`<div>{{.Avatar}}</div>`); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	registry := session.NewConnectionRegistry()
	peer := &session.Connection{GroupID: "peer-grp", UserID: "bob"}
	registry.Register(peer, 1)
	defer registry.Unregister(peer)
	if !registry.SubscribeConnectionToTopic(peer, "t/x") {
		t.Fatal("expected peer subscribe to report the 0→1 transition")
	}

	handler := &liveHandler{
		config:   mountConfig{Template: tmpl, Controller: &p2UploadController{}},
		registry: registry,
	}

	uploadRegistry := upload.NewRegistry()
	if err := uploadRegistry.CreateUpload("avatar", UploadConfig{}); err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}
	uploadObj, ok := uploadRegistry.GetUpload("avatar").(*upload.Upload)
	if !ok {
		t.Fatal("expected *upload.Upload")
	}
	if err := uploadObj.AddEntry(&uploadtypes.UploadEntry{
		ID: "entry-1", ClientName: "a.txt", ClientType: "text/plain", ClientSize: 1, TempPath: "/tmp/a.txt",
	}); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	uploader := &session.Connection{GroupID: "uploader-grp", UserID: "alice", Template: tmpl}
	state := &connState{state: p2UploadState{}, messages: make(map[string]string), groupID: "uploader-grp"}
	raw := []byte(`{"action":"upload_complete","upload_name":"avatar","entry_ids":["entry-1"]}`)
	if err := handler.handleUploadComplete(context.Background(), raw, state, uploadRegistry, uploader); err != nil {
		t.Fatalf("handleUploadComplete failed: %v", err)
	}

	select {
	case req := <-peer.DispatchChan:
		if req.Action != "Reload" {
			t.Errorf("peer got action %q, want Reload", req.Action)
		}
		if got, _ := req.Data["msg"].(string); got != "uploaded" {
			t.Errorf("peer payload msg = %q, want uploaded", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upload-complete Publish did not dispatch to the subscribed peer")
	}
}

// p2DenyController: a required (gated) Subscribe in Mount under deny-all →
// Mount fails → the WS-connect path emits the topic_forbidden envelope.
type p2DenyController struct{}

func (c *p2DenyController) Mount(s probeState, ctx *Context) (probeState, error) {
	if err := ctx.Subscribe("locked/topic"); err != nil {
		return s, err
	}
	return s, nil
}

// 6. The server-emitted WS topic_forbidden envelope (Phase 1 emission; Phase 4
// is the TS consumer). Deny-all default denies the gated Subscribe in Mount.
func TestTopic_Phase2_TopicForbiddenEnvelope(t *testing.T) {
	server, wsURL := setupTopicServer(t, &p2DenyController{}, AsState(&probeState{}), `<div>{{.N}}</div>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"})) // neither ACL nor OpenTopics → deny-all
	defer server.Close()

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer func() { _ = ws.Close() }()
	if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected the topic_forbidden envelope frame, read failed: %v", err)
	}

	var env struct {
		Type  string `json:"type"`
		Code  string `json:"code"`
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal(msg, &env); err != nil {
		t.Fatalf("envelope is not JSON: %s", msg)
	}
	if env.Type != "error" || env.Code != "topic_forbidden" || env.Topic != "locked/topic" {
		t.Errorf("unexpected envelope: %+v (raw %s)", env, msg)
	}
}

// p2CollideController: action Publishes "Save" (PascalCase) while the template
// wires <button name="save"> (lowercase) — both dispatch to Save(); the
// Phase 2 canonicalization must now flag this style mismatch.
type p2CollideController struct{}

func (c *p2CollideController) Mount(s probeState, ctx *Context) (probeState, error) {
	_ = ctx.Subscribe(ctx.SelfTopic())
	return s, nil
}
func (c *p2CollideController) PubSave(s probeState, ctx *Context) (probeState, error) {
	return s, ctx.Publish("t/x", "Save", nil)
}
func (c *p2CollideController) Save(s probeState, _ *Context) (probeState, error) { return s, nil }
func (c *p2CollideController) PubOther(s probeState, ctx *Context) (probeState, error) {
	return s, ctx.Publish("t/x", "ZzzUnwired", nil)
}

// 7. Phase 1 round-4 fix: the symmetry-collision warning now catches a
// style-mismatched collision (Publish "Save" vs <button name="save">), and
// still produces no false positive for a genuinely unwired action name.
func TestTopic_Phase2_SymmetryCollisionStyleMismatch(t *testing.T) {
	sb := captureSlog(t)
	server, wsURL := setupTopicServer(t, &p2CollideController{}, AsState(&probeState{}),
		`<form><button name="save">x</button></form>`,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	sendWSAction(t, ws, "pub_save", nil)
	_ = ws.SetReadDeadline(time.Now().Add(time.Second))
	_, _, _ = ws.ReadMessage()
	if !strings.Contains(sb.String(), "Publish action name collides with a client-wired action") {
		t.Errorf("expected collision warning for Publish(\"Save\") vs name=\"save\", not in:\n%s", sb.String())
	}

	sb2 := captureSlog(t)
	sendWSAction(t, ws, "pub_other", nil)
	_ = ws.SetReadDeadline(time.Now().Add(time.Second))
	_, _, _ = ws.ReadMessage()
	if strings.Contains(sb2.String(), "collides with a client-wired action") {
		t.Errorf("false-positive collision warning for unwired \"ZzzUnwired\":\n%s", sb2.String())
	}
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
