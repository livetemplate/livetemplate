package livetemplate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/pubsub"
)

// ============================================================================
// Phase 2 gate — cross-instance (Redis) Tier-1 integration tests.
//
//   V8 — out-of-band handler.Publish (no Context): single-instance gated
//        "announcements" topic (F/H) AND the required cross-instance leg
//        (handler.Publish(UserTopic("alice"),…) on instance A reaches alice on
//        instance B — use case F's primary mode).
//   V9 — cross-instance ctx.Publish over the single livetemplate:topic:{name}
//        channel: a Publish on instance A reaches a subscriber on instance B.
//
// Two RedisBroadcaster instances (distinct InstanceIDs) share one Redis
// testcontainer — the genuine two-process model: a shared broadcaster would
// carry one InstanceID and handleMessage's own-instance filter would drop the
// cross-handler delivery. getTestRedisClient t.Skips when Docker is absent.
//
// Topic pub/sub has no replay, so a SUBSCRIBE that has not yet propagated to
// Redis silently loses the publish. Rather than guess a settle duration, the
// publish is retried until the subscriber observes it (or a hard deadline) —
// timing-robust by construction.
// ============================================================================

// setupTopicServerH is setupTopicServer plus the LiveHandler, so a test can
// call the out-of-band handler.Publish entry point (V8).
func setupTopicServerH(t *testing.T, ctrl interface{}, st State, tmplStr string, opts ...Option) (*httptest.Server, string, LiveHandler) {
	t.Helper()
	tmpl, err := New("test", opts...)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if tmpl, err = tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	h := tmpl.Handle(ctrl, st)
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	return server, wsURL, h
}

// awaitWSContains re-drives trigger every ~150ms until ws yields a frame whose
// raw body contains want, or the deadline elapses. The retry absorbs Redis
// SUBSCRIBE-propagation latency without a brittle fixed sleep (the publish is
// re-sent, so a delivery lost before the SUBSCRIBE landed is simply re-driven).
//
// Call at most once per connection — it dedicates a reader goroutine to ws (see
// wsFrameReader).
func awaitWSContains(t *testing.T, ws *websocket.Conn, want string, trigger func()) {
	t.Helper()
	awaitWSFrame(t, ws, want, "", 150*time.Millisecond, trigger)
}

// --- shared cross-instance fixtures ---

type xinstState struct{ Msg string }

const xinstTmpl = `<div>{{.Msg}}</div>`

// xinstSubController (instance B): subscribes a developer topic in Mount; the
// Reload reconciler re-reads the shared field from the Publish payload (a valid
// shared source per the reconciler rule — "from the Publish payload").
type xinstSubController struct{ topic string }

func (c *xinstSubController) Mount(s xinstState, ctx *Context) (xinstState, error) {
	return s, ctx.Subscribe(c.topic)
}
func (c *xinstSubController) Reload(s xinstState, ctx *Context) (xinstState, error) {
	s.Msg = ctx.GetString("msg")
	return s, nil
}

// xinstPubController (instance A): an action that Publishes cross-instance.
type xinstPubController struct{ topic string }

func (c *xinstPubController) Mount(s xinstState, _ *Context) (xinstState, error) { return s, nil }
func (c *xinstPubController) Send(s xinstState, ctx *Context) (xinstState, error) {
	return s, ctx.Publish(c.topic, "Reload", map[string]interface{}{"msg": ctx.GetString("msg")})
}

// xinstSelfController (instance B, authenticated): subscribes SelfTopic()
// (ACL-exempt); DM is the out-of-band reconciler for use case F.
type xinstSelfController struct{}

func (c *xinstSelfController) Mount(s xinstState, ctx *Context) (xinstState, error) {
	_ = ctx.Subscribe(ctx.SelfTopic())
	return s, nil
}
func (c *xinstSelfController) DM(s xinstState, ctx *Context) (xinstState, error) {
	s.Msg = ctx.GetString("from")
	return s, nil
}

func newSharedRedisBroadcasters(t *testing.T) (a, b *pubsub.RedisBroadcaster) {
	t.Helper()
	client := getTestRedisClient(t) // t.Skips if Docker unavailable; owns the client's t.Cleanup
	a = pubsub.NewRedisBroadcaster(client)
	b = pubsub.NewRedisBroadcaster(client)
	// Close only the broadcasters (their own PubSub subscriptions). The shared
	// client + container are torn down by getTestRedisClient's own t.Cleanup —
	// closing the client here too would double-close it.
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}

// ============================================================================
// V9 — cross-instance ctx.Publish over the single livetemplate:topic:{name}
// channel. Publish on A reaches a Subscriber on B; B's reconciler runs against
// B's own connState (the cross-instance reconciler guarantee).
// ============================================================================

func TestTopic_V9_CrossInstancePublish(t *testing.T) {
	bA, bB := newSharedRedisBroadcasters(t)

	const topic = "room/42"
	_, wsURLA, _ := setupTopicServerH(t, &xinstPubController{topic: topic}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "a"}), WithOpenTopics(), WithPubSubBroadcaster(bA))
	_, wsURLB, _ := setupTopicServerH(t, &xinstSubController{topic: topic}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "b"}), WithOpenTopics(), WithPubSubBroadcaster(bB))

	// B subscribes "room/42" in Mount → registry + relayed Redis SUBSCRIBE.
	wsB := connectWS(t, wsURLB)
	defer func() { _ = wsB.Close() }()

	// A drives the publish (its own action response is read+discarded on wsA).
	wsA := connectWS(t, wsURLA)
	defer func() { _ = wsA.Close() }()

	awaitWSContains(t, wsB, "ping", func() {
		sendWSAction(t, wsA, "send", map[string]interface{}{"msg": "ping"})
		_ = wsA.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _, _ = wsA.ReadMessage() // drain A's own action-response render
	})
}

// ============================================================================
// V8 — out-of-band handler.Publish (no Context).
//   (a) single-instance: a gated "announcements" developer topic (F/H — the
//       all-users channel built from ordinary primitives, not a built-in).
//   (b) cross-instance leg (required, use case F's primary mode):
//       handler.Publish(UserTopic("alice"),…) on A reaches alice on B.
// ============================================================================

func TestTopic_V8_OutOfBandHandlerPublish_Announcements(t *testing.T) {
	acl := func(topic, _ string, _ *http.Request) (bool, error) {
		if topic == "announcements" {
			return true, nil
		}
		return false, nil
	}
	_, wsURL, h := setupTopicServerH(t, &xinstSubController{topic: "announcements"}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&fixedGroupAuth{groupID: "g"}), WithTopicACL(acl))

	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	// No Context, no sender to exclude — the subscriber receives it.
	awaitWSContains(t, ws, "maintenance at 5pm", func() {
		if err := h.Publish("announcements", "Reload", map[string]interface{}{"msg": "maintenance at 5pm"}); err != nil {
			t.Fatalf("handler.Publish failed: %v", err)
		}
	})
}

func TestTopic_V8_OutOfBandHandlerPublish_UserTopicCrossInstance(t *testing.T) {
	bA, bB := newSharedRedisBroadcasters(t)

	// Instance A: no connections — purely the out-of-band publisher.
	_, _, hA := setupTopicServerH(t, &xinstSelfController{}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&perDeviceAuth{userID: "alice"}), WithOpenTopics(), WithPubSubBroadcaster(bA))

	// Instance B: alice connects; Mount subscribes SelfTopic() = lvt:user:alice.
	_, wsURLB, _ := setupTopicServerH(t, &xinstSelfController{}, AsState(&xinstState{}), xinstTmpl,
		WithAuthenticator(&perDeviceAuth{userID: "alice"}), WithOpenTopics(), WithPubSubBroadcaster(bB))

	wsB := connectWS(t, wsURLB+"?device=1")
	defer func() { _ = wsB.Close() }()

	// UserTopic("alice") == ctx.SelfTopic() for authenticated alice.
	if got := UserTopic("alice"); got != "lvt:user:alice" {
		t.Fatalf("UserTopic(\"alice\") = %q, want lvt:user:alice", got)
	}

	awaitWSContains(t, wsB, "bob", func() {
		if err := hA.Publish(UserTopic("alice"), "DM", map[string]interface{}{"from": "bob"}); err != nil {
			t.Fatalf("out-of-band handler.Publish failed: %v", err)
		}
	})
}
