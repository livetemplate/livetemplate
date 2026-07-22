package livetemplate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsFrameReader starts a goroutine that reads frames from ws and returns a
// channel of their bodies, closed when the connection errors or closes.
//
// The reader sets NO per-read deadline on purpose. gorilla/websocket poisons a
// connection after ANY read error — a read-deadline timeout included, since
// hideTempErr still records it in c.readErr — and there is no way to clear that
// state. A retry loop that calls SetReadDeadline + ReadMessage and continues on
// error therefore fast-spins on the poisoned conn (each ReadMessage returns the
// stored error immediately) until gorilla's readErrCount>=1000 guard panics
// "repeated read on failed websocket connection". That only bites when a frame
// arrives slower than the deadline, i.e. under CPU load, which made it a
// load-flaky panic. A single blocking read with no deadline never times out, so
// it never poisons the conn; callers re-drive delivery with their own ticker.
//
// It also clears any deadline connectWS left set for the initial-render read so
// the blocking read below does not inherit a stale one. t.Cleanup closes ws so
// the goroutine always unblocks by test end.
//
// Call at most once per connection: the goroutine owns ws.ReadMessage for the
// connection's lifetime, and gorilla/websocket forbids concurrent reads — a
// second reader on the same conn is a data race. Every current caller awaits a
// given conn once; a test needing two sequential frames should keep reading the
// single returned channel rather than starting another reader.
func wsFrameReader(t *testing.T, ws *websocket.Conn) <-chan string {
	t.Helper()
	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	frames := make(chan string, 16)
	go func() {
		defer close(frames)
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			select {
			case frames <- string(msg):
			default: // consumer done or momentarily behind — a re-trigger re-drives
			}
		}
	}()
	return frames
}

// awaitWSFrame drives trigger once, re-drives it every retrigger to absorb Redis
// SUBSCRIBE-propagation latency, and returns when a frame read from ws contains
// want. It fails the test if a frame contains reject first (pass "" to disable
// that guard), if ws closes, or if the overall 8s deadline elapses.
//
// It substring-matches the raw frame rather than asserting tree[slot]==want on
// purpose: a tree update carries only the CHANGED dynamics and the value may be
// nested, so a raw-body contains check is the robust signal that the value
// landed end-to-end (all these cross-instance tests need to assert).
func awaitWSFrame(t *testing.T, ws *websocket.Conn, want, reject string, retrigger time.Duration, trigger func()) {
	t.Helper()
	frames := wsFrameReader(t, ws)
	overall := time.NewTimer(8 * time.Second)
	defer overall.Stop()
	tick := time.NewTicker(retrigger)
	defer tick.Stop()

	trigger()
	for {
		select {
		case body, ok := <-frames:
			if !ok {
				t.Fatalf("WS closed before a frame containing %q arrived", want)
			}
			if reject != "" && strings.Contains(body, reject) {
				t.Fatalf("forbidden frame containing %q arrived: %s", reject, body)
			}
			if strings.Contains(body, want) {
				return
			}
		case <-tick.C:
			trigger()
		case <-overall.C:
			t.Fatalf("timed out waiting for WS frame containing %q", want)
		}
	}
}

// TestWSFrameReader_SurvivesSlowFrame is the regression guard for the load-flaky
// panic that wsFrameReader fixes. It drives awaitWSContains against a server
// that sends the awaited frame only after 600ms — far longer than the 150ms
// per-read deadline the previous SetReadDeadline+retry helper used. With that
// old helper the first read timed out, poisoned the gorilla connection, and the
// retry loop fast-spun to the "repeated read on failed websocket connection"
// panic (readErrCount>=1000). wsFrameReader sets no per-read deadline, so the
// slow frame is simply delivered when it arrives.
func TestWSFrameReader_SurvivesSlowFrame(t *testing.T) {
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		time.Sleep(600 * time.Millisecond)
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"tree":{"0":"late-hello"}}`))
		time.Sleep(2 * time.Second) // hold open so the reader goroutine can deliver
	}))
	defer srv.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// The server sends unprompted, so trigger is a no-op; the assertion is that
	// the 600ms-delayed frame is delivered without a panic.
	awaitWSContains(t, ws, "late-hello", func() {})
}

// sendWSAction sends an action message over the WebSocket, matching the
// wire format the client uses.
func sendWSAction(t *testing.T, ws *websocket.Conn, action string, data map[string]interface{}) {
	t.Helper()
	msg := map[string]interface{}{
		"action": action,
	}
	if data != nil {
		msg["data"] = data
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

// sendWSActionWithSubmitter sends an action message carrying an explicit
// submitter (SubmitEvent.submitter.name), the wire shape the client uses under
// lvt-on:submit routing where action and submitter differ.
func sendWSActionWithSubmitter(t *testing.T, ws *websocket.Conn, action, submitter string, data map[string]interface{}) {
	t.Helper()
	msg := map[string]interface{}{"action": action, "submitter": submitter}
	if data != nil {
		msg["data"] = data
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

// assertTreeSlot checks that the parsed WS update response has the given
// value at tree slot key. Navigate responses are tree UPDATES containing
// only changed dynamic slot values, so this is the correct way to verify
// specific field values without fragile substring matching.
func assertTreeSlot(t *testing.T, context string, resp map[string]any, slotKey, wantValue string) {
	t.Helper()
	tree, ok := resp["tree"].(map[string]any)
	if !ok {
		t.Fatalf("%s: response has no tree: %#v", context, resp)
	}
	got := fmt.Sprintf("%v", tree[slotKey])
	if got != wantValue {
		t.Errorf("%s: tree[%q] = %q, want %q", context, slotKey, got, wantValue)
	}
}
