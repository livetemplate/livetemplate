package livetemplate

import (
	"testing"
	"time"
)

// TestPingActionRepliesPong pins the wire shape the client heartbeat depends on
// (livetemplate/client#142): a `__ping__` message gets exactly `{"pong":true}` —
// a liveness reply with no `tree`/`meta`, distinct from an UpdateResponse.
func TestPingActionRepliesPong(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()
	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	sendWSAction(t, ws, actionPing, nil)
	resp := readWSUpdate(t, ws, 2*time.Second)

	if pong, ok := resp["pong"].(bool); !ok || !pong {
		t.Errorf("ping reply = %#v, want {\"pong\":true}", resp)
	}
	if _, has := resp["tree"]; has {
		t.Errorf("pong must not carry a tree (it is not an UpdateResponse): %#v", resp)
	}
	if _, has := resp["meta"]; has {
		t.Errorf("pong must not carry meta: %#v", resp)
	}
	if len(resp) != 1 {
		t.Errorf("pong should be exactly {\"pong\":true}, got %d keys: %#v", len(resp), resp)
	}
}

// TestPingDoesNotPerturbConnection confirms a ping is inert on the connection:
// it neither consumes nor corrupts the message stream, and a real action after
// one (or several) pings still round-trips normally.
func TestPingDoesNotPerturbConnection(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()
	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	for i := 0; i < 2; i++ {
		sendWSAction(t, ws, actionPing, nil)
		resp := readWSUpdate(t, ws, 2*time.Second)
		if pong, _ := resp["pong"].(bool); !pong {
			t.Fatalf("ping %d: reply = %#v, want pong", i, resp)
		}
	}

	// A real navigate action still works after the pings — the pings did not
	// disturb Mount/state or leave a stray frame on the wire.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "beta"})
	resp := readWSUpdate(t, ws, 2*time.Second)
	assertTreeSlot(t, "navigate after pings", resp, "0", "beta")
}

// TestPingIsSubjectToRateLimit pins that pings pass through the per-connection
// message rate limiter — the `__ping__` short-circuit sits AFTER `limiter.Allow()`,
// so a flood of pings is throttled like any other message and can't be used to
// bypass the limit.
func TestPingIsSubjectToRateLimit(t *testing.T) {
	// Burst 1: the first message is allowed, the second is throttled.
	server, wsURL := setupNavigateTestServer(t, WithMessageRateLimit(1, 1))
	defer server.Close()
	ws := connectWS(t, wsURL)
	defer func() { _ = ws.Close() }()

	// Send both before reading so no token refills between them.
	sendWSAction(t, ws, actionPing, nil)
	sendWSAction(t, ws, actionPing, nil)

	first := readWSUpdate(t, ws, 2*time.Second)
	if pong, _ := first["pong"].(bool); !pong {
		t.Fatalf("first ping should get a pong, got %#v", first)
	}

	second := readWSUpdate(t, ws, 2*time.Second)
	meta, ok := second["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("throttled ping should be an UpdateResponse with meta, got %#v", second)
	}
	errs, _ := meta["errors"].(map[string]interface{})
	if _, limited := errs["_rate_limit"]; !limited {
		t.Errorf("second ping should be rate-limited (meta.errors._rate_limit), got %#v", meta)
	}
}
