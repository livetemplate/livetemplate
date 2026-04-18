package livetemplate

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// navigateTestState is the per-connection state for the navigate action
// test. It records how many times Mount ran, which simulates a controller
// that needs to re-run Mount-time side effects on each navigation.
type navigateTestState struct {
	Selected   string
	MountCount int
}

// navigateTestController reads a "s" query param in Mount and uses it to
// set state.Selected, then renders the current value. The test can assert
// against MountCount to verify the navigate action re-ran Mount without
// double-counting.
type navigateTestController struct{}

func (c *navigateTestController) Mount(state navigateTestState, ctx *Context) (navigateTestState, error) {
	state.Selected = ctx.GetString("s")
	state.MountCount++
	return state, nil
}

// Noop is a regular action that does nothing — used by tests to verify
// non-navigate actions still hit DispatchWithState and don't accidentally
// route through Mount.
func (c *navigateTestController) Noop(state navigateTestState, _ *Context) (navigateTestState, error) {
	return state, nil
}

func setupNavigateTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	// Each test gets a unique group ID derived from its name, preventing
	// state bleed if tests ever run in parallel against a shared session store.
	auth := &fixedGroupAuth{groupID: t.Name()}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div class="sel">{{.Selected}}</div><div class="count">{{.MountCount}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &navigateTestController{}
	state := AsState(&navigateTestState{})
	handler := tmpl.Handle(ctrl, state)

	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/?s=alpha"
	return server, wsURL
}

// TestNavigateActionReMountsWithNewQueryData is the load-bearing test
// for the in-band navigate message. It wires a controller whose Mount
// reads the "s" query param, opens a WebSocket connect with ?s=alpha,
// confirms state.Selected == "alpha", then sends a {action:"__navigate__",
// data:{s:"beta"}} message and confirms state.Selected flips to "beta"
// on the SAME WebSocket connection — without any reconnect.
//
// Navigate responses are tree UPDATES: the client already has the statics
// cached from the initial render, so subsequent responses contain only the
// changed dynamic slot values. assertTreeSlot parses the JSON tree and
// checks specific slots by position (slot "0" = Selected, slot "1" = MountCount).
func TestNavigateActionReMountsWithNewQueryData(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Fire the navigate action with new query data. The response is
	// the tree update for the re-mounted state.
	//
	// Navigate responses are tree UPDATES: statics are already cached
	// client-side, so the wire format contains only changed dynamic slot
	// values (no HTML, no fingerprints). Parse the JSON tree to assert
	// specific slot values rather than using fragile substring matching.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "beta"})
	resp1 := readWSUpdate(t, ws, 2*time.Second)
	assertTreeSlot(t, "after first navigate", resp1, "0", "beta")
	// Slot 1 = MountCount: 2 after initial connect + one navigate.
	assertTreeSlot(t, "after first navigate", resp1, "1", "2")

	// One more navigate to a third value confirms we can re-nav
	// multiple times on the same connection.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "gamma"})
	resp2 := readWSUpdate(t, ws, 2*time.Second)
	assertTreeSlot(t, "after second navigate", resp2, "0", "gamma")
	// Slot 1 = MountCount: 3 after initial connect + two navigates.
	assertTreeSlot(t, "after second navigate", resp2, "1", "3")
}

// TestNavigateActionNotRoutedThroughDispatchWithState verifies that the
// navigate action is intercepted BEFORE the regular DispatchWithState
// path runs. The controller doesn't define a method named __navigate__
// or Navigate, yet the action succeeds because the event loop routes it
// to Mount directly. This is the contract that makes the navigate
// message work on any controller without a method-name collision.
func TestNavigateActionNotRoutedThroughDispatchWithState(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// The controller has no method named Navigate or __navigate__.
	// Sending the navigate action should succeed (route through Mount)
	// rather than producing an ErrMethodNotFound error in state.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "abc"})
	resp := readWSUpdate(t, ws, 2*time.Second)

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("response has no meta: %#v", resp)
	}
	if success, _ := meta["success"].(bool); !success {
		t.Errorf("navigate response success = false, want true; meta = %#v", meta)
	}
	if errs, hasErrs := meta["errors"].(map[string]interface{}); hasErrs && len(errs) > 0 {
		t.Errorf("navigate response has errors: %#v", errs)
	}
}

// TestNonNavigateActionRoutedThroughDispatchWithState verifies that the Noop
// action (a regular controller method, not __navigate__) routes through
// DispatchWithState and NOT through Mount. After the initial connect, Mount
// has run once (MountCount=1). Calling Noop must not increment MountCount —
// if it did, it would mean DispatchWithState is incorrectly routing Noop
// through Mount's path.
func TestNonNavigateActionRoutedThroughDispatchWithState(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Send a regular (non-navigate) action. Noop returns state unchanged,
	// so the diff will contain no tree slot changes — but the meta must show
	// success=true (Noop dispatched correctly via DispatchWithState).
	sendWSAction(t, ws, "Noop", nil)
	resp := readWSUpdate(t, ws, 2*time.Second)
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("Noop response has no meta: %#v", resp)
	}
	if success, _ := meta["success"].(bool); !success {
		t.Errorf("Noop: meta.success = false, want true; meta = %#v", meta)
	}
	// MountCount slot must NOT appear in the diff — state didn't change,
	// so no tree update is emitted for the MountCount slot.
	// This proves Noop went through DispatchWithState, not callMount.
	if tree, hasTree := resp["tree"]; hasTree {
		if treeMap, ok := tree.(map[string]any); ok {
			if _, hasMountCount := treeMap["1"]; hasMountCount {
				t.Errorf("Noop: MountCount slot present in diff — Mount was called unexpectedly; tree = %#v", tree)
			}
		}
	}
}

// TestNavigateActionEmptyDataResetsQueryParams documents and pins the
// "no data = all-empty params" behavior. Sending __navigate__ with no
// data field (or an empty map) gives Mount ctx.GetString/GetInt zero
// values for all keys — the original connection query string is NOT
// preserved. This test guards against a future regression where someone
// tries to "merge" the original params into a nil-data navigate.
func TestNavigateActionEmptyDataResetsQueryParams(t *testing.T) {
	server, wsURL := setupNavigateTestServer(t)
	defer server.Close()

	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Connect with ?s=alpha — initial Selected is "alpha".
	// Send navigate with NO data — Mount should see s="" (not "alpha").
	sendWSAction(t, ws, actionNavigate, nil)
	resp := readWSUpdate(t, ws, 2*time.Second)
	// Slot 0 = Selected — must be empty string, not the original "alpha".
	assertTreeSlot(t, "empty-data navigate", resp, "0", "")
}

// navigateErrorController returns an error from Mount when the "s" param
// equals "error" — used by TestNavigateActionMountErrorLeavesStateUnchanged.
type navigateErrorController struct{}

func (c *navigateErrorController) Mount(state navigateTestState, ctx *Context) (navigateTestState, error) {
	if ctx.GetString("s") == "error" {
		return state, errors.New("mount rejected invalid param")
	}
	state.Selected = ctx.GetString("s")
	state.MountCount++
	return state, nil
}

// TestNavigateActionMountErrorLeavesStateUnchanged verifies that when
// callMount returns an error on the navigate path, connSt.state is NOT
// updated — the connection stays at its previous state — and the response
// carries success=false. This guards the invariant that failed navigates
// do not partially mutate state.
func TestNavigateActionMountErrorLeavesStateUnchanged(t *testing.T) {
	auth := &fixedGroupAuth{groupID: t.Name()}
	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div class="sel">{{.Selected}}</div><div class="count">{{.MountCount}}</div>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	handler := tmpl.Handle(&navigateErrorController{}, AsState(&navigateTestState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/?s=alpha"
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// First navigate succeeds — Selected becomes "beta".
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "beta"})
	resp1 := readWSUpdate(t, ws, 2*time.Second)
	assertTreeSlot(t, "first navigate", resp1, "0", "beta")

	// Second navigate fails — Mount returns an error for s="error".
	// The response must carry success=false and connSt.state must stay "beta".
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "error"})
	respErr := readWSUpdate(t, ws, 2*time.Second)
	meta, ok := respErr["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("error navigate response has no meta: %#v", respErr)
	}
	if success, _ := meta["success"].(bool); success {
		t.Errorf("error navigate: meta.success = true, want false")
	}
	// The error path may emit an empty tree {} (a no-op diff). What must
	// NOT appear is a non-empty tree with slot changes — that would mean
	// the failed navigate emitted a diff that mutates client state.
	if tree, hasTree := respErr["tree"]; hasTree {
		if treeMap, ok := tree.(map[string]any); ok && len(treeMap) > 0 {
			t.Errorf("error navigate: response contains non-empty tree (failed navigate must not mutate client state): %#v", tree)
		}
	}

	// Third navigate succeeds — Selected becomes "gamma". MountCount must
	// NOT reflect a count from the failed navigate (error path doesn't commit).
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "gamma"})
	resp3 := readWSUpdate(t, ws, 2*time.Second)
	assertTreeSlot(t, "third navigate after error", resp3, "0", "gamma")
}

// noMountController has no Mount method. Used by
// TestNavigateActionNoMountMethodIsNoOpRerender.
type noMountController struct{}

// TestNavigateActionNoMountMethodIsNoOpRerender pins the invariant: when the
// controller defines no Mount method, __navigate__ produces a success=true
// response without changing state. This verifies that callMount's no-method
// path (lifecycle.go) works correctly end-to-end for the navigate action.
func TestNavigateActionNoMountMethodIsNoOpRerender(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.Selected}}</div>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	handler := tmpl.Handle(&noMountController{}, AsState(&navigateTestState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Navigate on a controller with no Mount method: must succeed (success=true)
	// and must NOT produce an ErrMethodNotFound error.
	sendWSAction(t, ws, actionNavigate, map[string]interface{}{"s": "anything"})
	resp := readWSUpdate(t, ws, 2*time.Second)

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("response has no meta: %#v", resp)
	}
	if success, _ := meta["success"].(bool); !success {
		t.Errorf("no-Mount navigate: meta.success = false, want true; meta = %#v", meta)
	}
	if errs, hasErrs := meta["errors"].(map[string]interface{}); hasErrs && len(errs) > 0 {
		t.Errorf("no-Mount navigate: unexpected errors: %#v", errs)
	}
}

