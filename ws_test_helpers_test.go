package livetemplate

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gorilla/websocket"
)

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
