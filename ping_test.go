package livetemplate

import (
	"io"
	"net/http"
	"net/url"
	"strings"
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

// TestPingRejectedOverHTTP: __ping__ is WebSocket-only. Posting it over the
// HTTP-fetch/no-JS tier must return a clear 400 "wrong transport" error — the
// same treatment __navigate__ gets — not a confusing ErrMethodNotFound
// fall-through (no controller method named __ping__ exists).
func TestPingRejectedOverHTTP(t *testing.T) {
	server, _ := setupNavigateTestServer(t)
	defer server.Close()

	form := url.Values{}
	form.Set("lvt-action", actionPing)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("__ping__ over HTTP status = %d, want 400; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "only supported over WebSocket") {
		t.Errorf("__ping__ over HTTP body = %q, want a wrong-transport message", body)
	}
}
