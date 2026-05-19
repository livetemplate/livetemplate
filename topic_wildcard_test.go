package livetemplate

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Phase 3 gate — wildcards (multi-segment). V17 + V18.
//
//   V17 — multi-segment fan-out + dedupe + first-ever + cross-instance via
//         PSUBSCRIBE + over-delivery rejection.
//   V18 — the ACL hook receives the LITERAL pattern (never an expansion).
//
// Single-instance legs (no Redis): deterministic — they prove the
// segmentMatch-based resolution + the registry exact∪pattern union dedup +
// the ACL-literal-pattern + send-side-ungated (ACL not re-invoked at Publish).
// Cross-instance legs (Redis testcontainers, mirroring topic_cross_instance_
// test.go): the PSUBSCRIBE relay, the Redis-"*"-spans-"/" over-delivery
// rejection, and the SUBSCRIBE+PSUBSCRIBE double-fire seen-ring exactly-once.
// ============================================================================

// --- single-instance fixtures ---

type wcState struct{ Msg string }

const wcTmpl = `<div>{{.Msg}}</div>`

// wildcardController drives the single-instance V17/V18 legs. Sub subscribes
// the topic in action data (the connection stays alive for the whole test, so
// connection-lifetime durability is sufficient — V17 is a fan-out/dedup test,
// reconnect-durability is V21's concern, not re-proven here). Sub sets Msg so
// every Subscribe yields exactly one drainable frame. Pub publishes a CONCRETE
// topic; Reload is the receiver reconciler.
type wildcardController struct{}

func (c *wildcardController) Mount(s wcState, _ *Context) (wcState, error) { return s, nil }

func (c *wildcardController) Sub(s wcState, ctx *Context) (wcState, error) {
	topic := ctx.GetString("topic")
	if err := ctx.Subscribe(topic); err != nil {
		return s, err
	}
	// Distinct per topic so a SECOND Subscribe on the same connection still
	// produces a non-empty diff to drain (Msg "ack:room/42" → "ack:room/*"),
	// and so an undrained ack can never be mistaken for a Reload payload.
	s.Msg = "ack:" + topic
	return s, nil
}

func (c *wildcardController) Pub(s wcState, ctx *Context) (wcState, error) {
	return s, ctx.Publish(ctx.GetString("topic"), "Reload", map[string]interface{}{"msg": ctx.GetString("msg")})
}

func (c *wildcardController) Reload(s wcState, ctx *Context) (wcState, error) {
	s.Msg = ctx.GetString("msg")
	return s, nil
}

// subscribeWS sends Sub(topic) on ws and drains the "ack" frame so a later
// matching Pub→Reload frame is unambiguous.
func subscribeWS(t *testing.T, ws *websocket.Conn, topic string) {
	t.Helper()
	sendWSAction(t, ws, "sub", map[string]interface{}{"topic": topic})
	if got := rawWSUpdate(t, ws, 3*time.Second); !strings.Contains(got, "ack:"+topic) {
		t.Fatalf("Subscribe(%q) did not ack: %s", topic, got)
	}
}

// TestTopic_V17_SingleInstance_MultiSegmentFanoutAndDedup proves: trailing-,
// multi-, and leading-segment patterns each receive their matching concrete
// publish; a segment-count mismatch (deep/* vs deep/1/x) does NOT; and a
// connection dual-subscribed to dup/7 AND dup/* gets EXACTLY ONE frame (the
// registry exact∪pattern union deduped by *Connection identity).
//
// Connection-role discipline (gorilla read-poisoning): NextReader sets a
// permanent c.readErr on ANY read error, so a connection that has taken a read
// DEADLINE TIMEOUT (expectNoMessage) is dead for all later reads. Therefore
// every connection here has exactly ONE role — positive (only ever
// rawWSUpdate) or negative (only ever expectNoMessage, once, as its last op) —
// and each publish targets a DISJOINT namespace so no cross-talk forces a
// connection out of its role.
func TestTopic_V17_SingleInstance_MultiSegmentFanoutAndDedup(t *testing.T) {
	server, wsURL := setupTopicServer(t, &wildcardController{}, AsState(&wcState{}), wcTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithOpenTopics())
	defer server.Close()

	posRoom := connectWS(t, wsURL) // room/*       — trailing-* positive
	defer func() { _ = posRoom.Close() }()
	posOrg := connectWS(t, wsURL) // org/*/room/* — multi-segment positive
	defer func() { _ = posOrg.Close() }()
	posAlice := connectWS(t, wsURL) // */alice      — leading-* positive
	defer func() { _ = posAlice.Close() }()
	dual := connectWS(t, wsURL) // dup/7 + dup/* — exactly-one
	defer func() { _ = dual.Close() }()
	negDeep := connectWS(t, wsURL) // deep/*       — segment-count valve (negative)
	defer func() { _ = negDeep.Close() }()
	pub := connectWS(t, wsURL)
	defer func() { _ = pub.Close() }()

	subscribeWS(t, posRoom, "room/*")
	subscribeWS(t, posOrg, "org/*/room/*")
	subscribeWS(t, posAlice, "*/alice")
	subscribeWS(t, dual, "dup/7")
	subscribeWS(t, dual, "dup/*")
	subscribeWS(t, negDeep, "deep/*")

	publish := func(topic, msg string) {
		t.Helper()
		sendWSAction(t, pub, "pub", map[string]interface{}{"topic": topic, "msg": msg})
		_ = rawWSUpdate(t, pub, 2*time.Second) // drain pub's own (empty) render
	}

	// Trailing-* match (disjoint namespace ⇒ only posRoom matches room/9).
	publish("room/9", "R9")
	if got := rawWSUpdate(t, posRoom, 3*time.Second); !strings.Contains(got, "R9") {
		t.Fatalf("room/* must match room/9: %s", got)
	}

	// Multi-segment match.
	publish("org/9/room/7", "ORG")
	if got := rawWSUpdate(t, posOrg, 3*time.Second); !strings.Contains(got, "ORG") {
		t.Fatalf("org/*/room/* must match org/9/room/7: %s", got)
	}

	// Leading-* match.
	publish("wing/alice", "ALICE")
	if got := rawWSUpdate(t, posAlice, 3*time.Second); !strings.Contains(got, "ALICE") {
		t.Fatalf("*/alice must match wing/alice: %s", got)
	}

	// Dual exact∪pattern: dup/7 reaches dup/7 AND dup/* on ONE connection —
	// exactly one frame (registry dedups by *Connection identity).
	publish("dup/7", "D7")
	if got := rawWSUpdate(t, dual, 3*time.Second); !strings.Contains(got, "D7") {
		t.Fatalf("dual (dup/7 + dup/*) must receive dup/7: %s", got)
	}
	expectNoMessage(t, dual, 500*time.Millisecond,
		"V17: dual (dup/7 + dup/*) received a SECOND frame — exact∪pattern union not deduped") // dual's last op

	// Segment-count valve: deep/* (2-seg) must NOT match deep/1/x (3-seg).
	// negDeep's only op; nothing else is published into deep/.
	publish("deep/1/x", "DEEP")
	expectNoMessage(t, negDeep, 600*time.Millisecond,
		"V17: deep/* must NOT match deep/1/x (segment-count) — the wildcard valve")
}

// aclRecorder records every topic the ACL hook is invoked with, and counts
// invocations, so a test can assert the LITERAL pattern reached the hook (V18)
// and that Publish does NOT re-invoke it (V17 first-ever / send-side ungated).
type aclRecorder struct {
	mu     sync.Mutex
	topics []string
}

func (a *aclRecorder) allow(topic, _ string, _ *http.Request) (bool, error) {
	a.mu.Lock()
	a.topics = append(a.topics, topic)
	a.mu.Unlock()
	return true, nil
}

func (a *aclRecorder) snapshot() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.topics...)
}

// TestTopic_V18_ACLLiteralPattern_AndV17FirstEverNoReACL: the ACL hook receives
// the literal "org/*/room/*" (never expanded); a first-ever concrete publish
// matching that pattern is delivered with the ACL NOT re-invoked at Publish
// time (send-side ungated, proposal §3).
func TestTopic_V18_ACLLiteralPattern_AndV17FirstEverNoReACL(t *testing.T) {
	rec := &aclRecorder{}
	server, wsURL := setupTopicServer(t, &wildcardController{}, AsState(&wcState{}), wcTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithTopicACL(rec.allow))
	defer server.Close()

	sub := connectWS(t, wsURL)
	defer func() { _ = sub.Close() }()
	pub := connectWS(t, wsURL)
	defer func() { _ = pub.Close() }()

	subscribeWS(t, sub, "org/*/room/*")

	// V18: the hook saw the literal pattern, not a concrete expansion.
	got := rec.snapshot()
	foundLiteral := false
	for _, tp := range got {
		if tp == "org/*/room/*" {
			foundLiteral = true
		}
		if tp != "org/*/room/*" {
			t.Fatalf("V18: ACL invoked with non-literal topic %q (pattern must reach the hook verbatim, never expanded); all=%v", tp, got)
		}
	}
	if !foundLiteral {
		t.Fatalf("V18: ACL never invoked with the literal pattern; recorded=%v", got)
	}
	aclCallsAfterSub := len(got)

	// V17 first-ever: a never-before-published concrete that matches the
	// pattern is delivered, and the ACL is NOT consulted at Publish time.
	sendWSAction(t, pub, "pub", map[string]interface{}{"topic": "org/1/room/9", "msg": "FIRST"})
	_ = rawWSUpdate(t, pub, 2*time.Second)
	if frame := rawWSUpdate(t, sub, 3*time.Second); !strings.Contains(frame, "FIRST") {
		t.Fatalf("V17 first-ever: org/*/room/* subscriber must receive the first-ever org/1/room/9 publish: %s", frame)
	}
	if after := len(rec.snapshot()); after != aclCallsAfterSub {
		t.Fatalf("V17/§3: Publish must NOT re-invoke the ACL — calls went %d→%d", aclCallsAfterSub, after)
	}
}

// ============================================================================
// Cross-instance V17 (Redis testcontainers — skips if Docker absent).
// ============================================================================

// dualSubController (instance B) Mount-subscribes BOTH an exact topic and a
// matching pattern, so a single cross-instance publish double-fires
// (SUBSCRIBE livetemplate:topic:room/42 + PSUBSCRIBE livetemplate:topic:room/*);
// the seen-ring must collapse it to one dispatch.
type dualSubController struct{}

func (c *dualSubController) Mount(s xinstState, ctx *Context) (xinstState, error) {
	_ = ctx.Subscribe("room/42")
	return s, ctx.Subscribe("room/*")
}
func (c *dualSubController) Reload(s xinstState, ctx *Context) (xinstState, error) {
	s.Msg = ctx.GetString("msg")
	return s, nil
}

// TestTopic_V17_CrossInstance_PSubscribeDelivery: a room/* subscriber on
// instance B receives instance A's concrete room/42 publish — proving the
// relay PSUBSCRIBEs the pattern (one PSUBSCRIBE, not per-concrete) and Redis
// connects the exact PUBLISH to it cross-instance.
func TestTopic_V17_CrossInstance_PSubscribeDelivery(t *testing.T) {
	bA, bB := newSharedRedisBroadcasters(t)

	_, wsURLA, _ := setupTopicServerH(t, &xinstPubController{topic: "room/42"}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "a"}), WithOpenTopics(), WithPubSubBroadcaster(bA))
	_, wsURLB, _ := setupTopicServerH(t, &xinstSubController{topic: "room/*"}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "b"}), WithOpenTopics(), WithPubSubBroadcaster(bB))

	wsB := connectWS(t, wsURLB) // B Mount → Subscribe("room/*") → relayed PSUBSCRIBE
	defer func() { _ = wsB.Close() }()
	wsA := connectWS(t, wsURLA)
	defer func() { _ = wsA.Close() }()

	awaitWSContains(t, wsB, "ppp", func() {
		sendWSAction(t, wsA, "send", map[string]interface{}{"msg": "ppp"})
		_ = wsA.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _, _ = wsA.ReadMessage()
	})
}

// awaitWSContainsRejecting is awaitWSContains with a forbidden-substring guard:
// it retries trigger until a frame contains want (success) but FAILS the test
// if a frame contains reject first. Same reliability profile as awaitWSContains
// (the existing V8/V9 convention): want is guaranteed deliverable, so a read
// succeeds quickly on a local Redis testcontainer; the gorilla read-poisoning
// after a timeout is the same accepted risk the existing helper carries. The
// retry budget is a SUBSCRIBE/PSUBSCRIBE-propagation guard, not a load driver:
// the publish is expected to land on the first iteration and the loop normally
// runs once — it is not a deliberate repeated-publish stress.
func awaitWSContainsRejecting(t *testing.T, ws *websocket.Conn, want, reject string, trigger func()) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		trigger()
		if err := ws.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline failed: %v", err)
		}
		_, msg, err := ws.ReadMessage()
		if err != nil {
			continue
		}
		body := string(msg)
		if strings.Contains(body, reject) {
			t.Fatalf("forbidden frame containing %q arrived: %s", reject, body)
		}
		if strings.Contains(body, want) {
			return
		}
	}
	t.Fatalf("timed out waiting for WS frame containing %q", want)
}

// TestTopic_V17_CrossInstance_OverDeliveryRejected: B subscribes room/* →
// PSUBSCRIBE livetemplate:topic:room/*. Redis "*" spans "/", so a PUBLISH to
// livetemplate:topic:room/42/other is ALSO delivered to B's pattern PubSub
// (over-delivery). B's local strict segmentMatch re-resolution (room/* is
// 2-seg, room/42/other is 3-seg) drops it. Each trigger publishes the
// over-delivery "BAD" (room/42/other) BEFORE the positive control "GOOD"
// (room/42, which matches room/*), so if the valve were removed BAD would
// arrive first and fail the test; GOOD proves the pipe is live (not just slow).
func TestTopic_V17_CrossInstance_OverDeliveryRejected(t *testing.T) {
	bA, bB := newSharedRedisBroadcasters(t)

	_, wsURLA, _ := setupTopicServerH(t, &xinstMultiPubController{}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "a"}), WithOpenTopics(), WithPubSubBroadcaster(bA))
	_, wsURLB, _ := setupTopicServerH(t, &xinstSubController{topic: "room/*"}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "b"}), WithOpenTopics(), WithPubSubBroadcaster(bB))

	wsB := connectWS(t, wsURLB)
	defer func() { _ = wsB.Close() }()
	wsA := connectWS(t, wsURLA)
	defer func() { _ = wsA.Close() }()

	// This assertion's premise: Redis glob "*" spans "/", so a PUBLISH to
	// livetemplate:topic:room/42/other IS delivered to PSUBSCRIBE
	// livetemplate:topic:room/* (settled Redis semantics, go-redis v9). The
	// test proves our local segmentMatch drops that over-delivery. If a future
	// Redis ever tightened glob to not span "/", BAD would never be delivered
	// and this test would pass vacuously — audit the premise if Redis changes.
	t.Log("premise: Redis PSUBSCRIBE room/* over-delivers room/42/other (glob * spans /); test asserts local segmentMatch drops it")

	awaitWSContainsRejecting(t, wsB, "GOOD", "BAD", func() {
		sendWSAction(t, wsA, "pubboth", nil)
		_ = wsA.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _, _ = wsA.ReadMessage()
	})
}

// xinstMultiPubController (instance A): one action publishes the over-delivery
// concrete FIRST, then the positive control — both concrete (publishers never
// publish a pattern; that is rejected by ctx.Publish and is its own test).
type xinstMultiPubController struct{}

func (c *xinstMultiPubController) Mount(s xinstState, _ *Context) (xinstState, error) {
	return s, nil
}
func (c *xinstMultiPubController) Pubboth(s xinstState, ctx *Context) (xinstState, error) {
	if err := ctx.Publish("room/42/other", "Reload", map[string]interface{}{"msg": "BAD"}); err != nil {
		return s, err
	}
	return s, ctx.Publish("room/42", "Reload", map[string]interface{}{"msg": "GOOD"})
}

// TestTopic_V17_CrossInstance_DoubleFireExactlyOnce: a connection subscribed to
// BOTH room/42 (exact SUBSCRIBE) and room/* (PSUBSCRIBE) makes Redis deliver
// ONE cross-instance PUBLISH room/42 to B's PubSub TWICE (both copies carry A's
// identical (InstanceID, Seq)); the seen-ring drops the second so the
// connection's reconciler runs exactly once → exactly one WS frame.
//
// A warmup connection (wsWarm, same dualSubController) establishes that bB's
// SUBSCRIBE+PSUBSCRIBE for room/42 & room/* are registered in Redis BEFORE the
// measured connection exists. Because the broadcaster channel/pattern refcounts
// are instance-wide and shared, wsDual's Mount-time Subscribes are then a
// registry-only refcount bump (no new Redis command) — wsDual is hot the
// instant connectWS returns, so the SINGLE "solo" publish is delivered without
// a propagation race, and "exactly one frame" can be asserted with a single
// positive read + one trailing expectNoMessage (no read-after-timeout, which
// gorilla would poison).
func TestTopic_V17_CrossInstance_DoubleFireExactlyOnce(t *testing.T) {
	bA, bB := newSharedRedisBroadcasters(t)

	_, wsURLA, _ := setupTopicServerH(t, &xinstPubController{topic: "room/42"}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "a"}), WithOpenTopics(), WithPubSubBroadcaster(bA))
	_, wsURLB, _ := setupTopicServerH(t, &dualSubController{}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "b"}), WithOpenTopics(), WithPubSubBroadcaster(bB))

	wsA := connectWS(t, wsURLA)
	defer func() { _ = wsA.Close() }()

	// Warmup: prove bB's room/42 SUBSCRIBE + room/* PSUBSCRIBE are live in Redis.
	wsWarm := connectWS(t, wsURLB) // Mount → Subscribe("room/42")+Subscribe("room/*")
	defer func() { _ = wsWarm.Close() }()
	awaitWSContains(t, wsWarm, "warm", func() {
		sendWSAction(t, wsA, "send", map[string]interface{}{"msg": "warm"})
		_ = wsA.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _, _ = wsA.ReadMessage()
	})

	// The measured connection: its Mount Subscribes refcount-bump bB's already-
	// live channel + pattern (no new Redis command) — hot the moment connectWS
	// (which waits for the post-Mount initial render) returns.
	wsDual := connectWS(t, wsURLB)
	defer func() { _ = wsDual.Close() }()

	// Exactly ONE publish, AFTER wsDual is registered. Delivered to bB twice
	// (SUBSCRIBE room/42 + PSUBSCRIBE room/*); the seen-ring collapses it to one
	// dispatch per connection.
	sendWSAction(t, wsA, "send", map[string]interface{}{"msg": "solo"})
	_ = wsA.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, _ = wsA.ReadMessage()

	if got := rawWSUpdate(t, wsDual, 4*time.Second); !strings.Contains(got, "solo") {
		t.Fatalf("V17: dual-subscribed connection did not receive the cross-instance publish: %s", got)
	}
	expectNoMessage(t, wsDual, 600*time.Millisecond,
		"V17: dual (room/42 + room/*) received a SECOND cross-instance frame — the (instanceID,seq) seen-ring did not dedupe the SUBSCRIBE+PSUBSCRIBE double-fire") // wsDual's last op
}
