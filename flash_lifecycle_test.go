package livetemplate

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	lvtcontext "github.com/livetemplate/livetemplate/internal/context"
)

// newFlashState is a test helper that returns a minimal connState with the
// messages map initialised, ready for flash lifecycle tests.
func newFlashState() *connState {
	return &connState{
		messages:    make(map[string]string),
		flashExpiry: make(map[string]time.Time),
	}
}

// flashPresent returns true if the given key is in cs.getMessages().
func flashPresent(cs *connState, key string) bool {
	_, ok := cs.getMessages()[lvtcontext.FlashPrefix+key]
	return ok
}

// TestFlashPersistsAcrossRenders verifies that non-expiry flash survives
// repeated pruneExpiredFlash calls. This guards against the regression where
// background Refresh ticks cleared flash by calling the old clearFlash()
// (which removed ALL flash). Under the new lifecycle, pruneExpiredFlash only
// removes messages whose expiry has elapsed; non-expiry flash stays until
// ClearFlash is explicitly called.
func TestFlashPersistsAcrossRenders(t *testing.T) {
	cs := newFlashState()
	cs.setFlash("success", "Saved!", 0)
	if !flashPresent(cs, "success") {
		t.Fatal("flash not set")
	}

	// Simulate several render cycles — pruneExpiredFlash runs after each render.
	for i := 0; i < 5; i++ {
		cs.pruneExpiredFlash()
		if !flashPresent(cs, "success") {
			t.Fatalf("flash cleared after render %d, want it to persist until ClearFlash", i+1)
		}
	}
}

// TestClearFlashRemovesMessage verifies that clearFlashKey removes the flash
// message immediately, regardless of any expiry setting.
func TestClearFlashRemovesMessage(t *testing.T) {
	cs := newFlashState()
	cs.setFlash("error", "Something went wrong", 0)
	if !flashPresent(cs, "error") {
		t.Fatal("flash not set")
	}

	cs.clearFlashKey("error")

	if flashPresent(cs, "error") {
		t.Error("flash still present after clearFlashKey, want it removed")
	}
}

// TestFlashExpiryPrunesAfterDeadline verifies that flash set with a non-zero
// expiry is removed by pruneExpiredFlash once the deadline has passed.
func TestFlashExpiryPrunesAfterDeadline(t *testing.T) {
	cs := newFlashState()
	// Set flash with a real expiry, then manually backdate the deadline so
	// it appears already elapsed — avoids real time.Sleep in tests.
	cs.setFlash("info", "Processing...", time.Hour)
	if !flashPresent(cs, "info") {
		t.Fatal("flash not set")
	}

	// Backdate the expiry to force it to be past-due.
	cs.flashExpiry["info"] = time.Now().Add(-1 * time.Second)

	cs.pruneExpiredFlash()

	if flashPresent(cs, "info") {
		t.Error("flash still present after expiry deadline, want it pruned")
	}
}

// TestFlashExpiryDoesNotAffectNonExpiredMessages verifies that pruneExpiredFlash
// only removes expired entries and leaves non-expired flash intact.
func TestFlashExpiryDoesNotAffectNonExpiredMessages(t *testing.T) {
	cs := newFlashState()
	cs.setFlash("success", "Done!", 0)               // no expiry — persists forever
	cs.setFlash("info", "Transient...", time.Minute) // expires in 1 minute (not yet)

	// Manually backdate only the "info" key to be past-due.
	cs.flashExpiry["info"] = time.Now().Add(-1 * time.Second)

	cs.pruneExpiredFlash()

	if !flashPresent(cs, "success") {
		t.Error("non-expiry flash 'success' was incorrectly pruned")
	}
	if flashPresent(cs, "info") {
		t.Error("expired flash 'info' still present after prune")
	}
}

// TestFlashSetThenClearThenSetAgain verifies that clearFlashKey followed by
// a new setFlash for the same key works correctly — the new message replaces
// the cleared one, and subsequent pruneExpiredFlash calls leave it intact.
func TestFlashSetThenClearThenSetAgain(t *testing.T) {
	cs := newFlashState()
	cs.setFlash("status", "first", 0)
	cs.clearFlashKey("status")
	if flashPresent(cs, "status") {
		t.Fatal("flash not cleared")
	}

	cs.setFlash("status", "second", 0)
	if !flashPresent(cs, "status") {
		t.Fatal("flash not re-set after clear")
	}

	// Verify value is the new one.
	msgs := cs.getMessages()
	if v := msgs[lvtcontext.FlashPrefix+"status"]; v != "second" {
		t.Errorf("flash value = %q, want %q", v, "second")
	}

	// Pruning should leave it in place (no expiry set).
	cs.pruneExpiredFlash()
	if !flashPresent(cs, "status") {
		t.Error("re-set flash was incorrectly pruned")
	}
}

// flashIntegState is the per-connection state for the flash event loop
// integration test. It carries a Counter so that Increment always produces a
// state change — forcing the diff engine to emit a tree update that would
// reveal an absent flash slot if flash were incorrectly cleared between renders.
type flashIntegState struct {
	Counter int
}

type flashIntegController struct{}

func (c *flashIntegController) ShowFlash(state flashIntegState, ctx *Context) (flashIntegState, error) {
	ctx.SetFlash("info", "persistent")
	return state, nil
}

func (c *flashIntegController) Increment(state flashIntegState, ctx *Context) (flashIntegState, error) {
	state.Counter++
	return state, nil
}

func (c *flashIntegController) SetTransientFlash(state flashIntegState, ctx *Context) (flashIntegState, error) {
	ctx.SetFlash("status", "transient", FlashExpiry(time.Millisecond))
	return state, nil
}

// TestFlashSurvivesSubsequentWSAction is a full-stack regression test for the
// old clearFlash() bug. The old event loop called clearFlash() after every
// successful render, including server-driven refreshes, so flash set by one
// action was silently cleared before the next render.
//
// This test exercises the event loop integration:
//  1. "ShowFlash" action sets flash "info" = "persistent" (slot 0 changes).
//  2. "Increment" action mutates state (Counter 0→1, slot 1 changes).
//
// With the old clearFlash() bug, the Increment render would see flash="" (a
// change from "persistent") and include slot 0 in the diff with value "".
// With the fix (pruneExpiredFlash), flash is unchanged — slot 0 does NOT
// appear in the Increment diff. The test asserts slot 0 is absent from the
// Increment response, which fails if flash was cleared between renders.
func TestFlashSurvivesSubsequentWSAction(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Template slot 0 = Flash("info"), slot 1 = Counter.
	tmpl, err = tmpl.Parse(`<span class="flash">{{.lvt.Flash "info"}}</span><span class="count">{{.Counter}}</span>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	handler := tmpl.Handle(&flashIntegController{}, AsState(&flashIntegState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Step 1: SetFlash — flash "info" = "persistent" stored in connSt.
	sendWSAction(t, ws, "ShowFlash", nil)
	resp1 := readWSUpdate(t, ws, 2*time.Second)
	// Slot 0 = flash value must be "persistent".
	tree1, ok := resp1["tree"].(map[string]any)
	if !ok {
		t.Fatalf("ShowFlash response has no tree: %#v", resp1)
	}
	if v := fmt.Sprintf("%v", tree1["0"]); v != "persistent" {
		t.Fatalf("ShowFlash: tree slot 0 = %q, want %q", v, "persistent")
	}

	// Step 2: Increment — Counter changes (0→1), flash unchanged.
	// The diff engine ONLY sends changed dynamics. With the fix, flash slot 0
	// is unchanged ("persistent") so it must NOT appear in the diff.
	// With the old clearFlash() bug, flash was cleared after Step 1's render,
	// so slot 0 would appear in this diff with value "" — detectable here.
	sendWSAction(t, ws, "Increment", nil)
	resp2 := readWSUpdate(t, ws, 2*time.Second)
	tree2, ok := resp2["tree"].(map[string]any)
	if !ok {
		t.Fatalf("Increment response has no tree: %#v", resp2)
	}
	// Counter slot must show the new value.
	if v := fmt.Sprintf("%v", tree2["1"]); v != "1" {
		t.Errorf("Increment: Counter slot = %q, want %q", v, "1")
	}
	// Flash slot must be absent (unchanged). If it appears, it means flash was
	// cleared between renders — the old clearFlash() regression.
	if flashVal, present := tree2["0"]; present {
		t.Errorf("Increment diff contains flash slot: got %q — flash was incorrectly cleared between renders (regression: old clearFlash bug)", flashVal)
	}
}

// flashNavTestState is per-connection state for the navigate + flash interaction
// test. Selected is set by Mount from the "s" query param.
type flashNavTestState struct {
	Selected string
}

type flashNavTestController struct{}

func (c *flashNavTestController) Mount(state flashNavTestState, ctx *Context) (flashNavTestState, error) {
	if s := ctx.GetString("s"); s != "" {
		state.Selected = s
	}
	return state, nil
}

func (c *flashNavTestController) ShowFlash(state flashNavTestState, ctx *Context) (flashNavTestState, error) {
	ctx.SetFlash("info", "persistent")
	return state, nil
}

// TestFlashSurvivesNavigateAction verifies that flash set before a
// __navigate__ action survives the navigate's callMount path. The navigate
// path is a distinct code branch from DispatchWithState (it calls callMount
// directly), so it needs its own coverage.
//
// Test flow:
//  1. "ShowFlash" action stores flash "info"="persistent" in connSt.
//  2. "__navigate__" fires → callMount runs, Selected changes "alpha"→"beta".
//  3. The navigate diff must include slot 1 (Selected changed) and must NOT
//     include slot 0 (flash unchanged). A slot-0 entry in the navigate diff
//     would mean flash was cleared by the navigate path — the regression.
func TestFlashSurvivesNavigateAction(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "flash-nav-group"}
	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Template slot 0 = Flash("info"), slot 1 = Selected.
	tmpl, err = tmpl.Parse(`<span class="flash">{{.lvt.Flash "info"}}</span><span class="sel">{{.Selected}}</span>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	handler := tmpl.Handle(&flashNavTestController{}, AsState(&flashNavTestState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/?s=alpha"
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Step 1: ShowFlash — flash "info" = "persistent" stored in connSt.
	sendWSAction(t, ws, "ShowFlash", nil)
	resp1 := readWSUpdate(t, ws, 2*time.Second)
	tree1, ok := resp1["tree"].(map[string]any)
	if !ok {
		t.Fatalf("ShowFlash response has no tree: %#v", resp1)
	}
	if v := fmt.Sprintf("%v", tree1["0"]); v != "persistent" {
		t.Fatalf("ShowFlash: flash slot = %q, want %q", v, "persistent")
	}

	// Step 2: Navigate with new param. callMount runs, Selected changes.
	// Flash must survive the navigate's callMount path unchanged.
	sendWSAction(t, ws, actionNavigate, map[string]any{"s": "beta"})
	resp2 := readWSUpdate(t, ws, 2*time.Second)
	tree2, ok := resp2["tree"].(map[string]any)
	if !ok {
		t.Fatalf("navigate response has no tree: %#v", resp2)
	}
	// Selected slot must show the navigated-to value.
	if v := fmt.Sprintf("%v", tree2["1"]); v != "beta" {
		t.Errorf("navigate: Selected slot = %q, want %q", v, "beta")
	}
	// Flash slot must be absent (unchanged). Its presence means flash was
	// cleared by the navigate's callMount path — the regression.
	if flashVal, present := tree2["0"]; present {
		t.Errorf("navigate diff contains flash slot: got %q — flash was incorrectly cleared by navigate (callMount path regression)", flashVal)
	}
}

// TestFlashExpiryThroughPublicAPI exercises the full path from
// ctx.SetFlash(key, msg, FlashExpiry(d)) through the event loop to the diff
// engine. Unit tests in this file call connState.setFlash() directly; this
// test verifies that FlashExpiry flows correctly through the public API and
// that pruneExpiredFlash removes the flash entry before the next render.
//
// Test flow:
//  1. "SetTransientFlash" action sets flash "status"="transient" with 1ms expiry.
//     The first render includes slot 0 = "transient".
//  2. After the 1ms expiry elapses, "Increment" changes state (Counter 0→1).
//     pruneExpiredFlash removes the lapsed flash; slot 0 is absent from the diff.
func TestFlashExpiryThroughPublicAPI(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Template slot 0 = Flash("status"), slot 1 = Counter.
	tmpl, err = tmpl.Parse(`<span class="flash">{{.lvt.Flash "status"}}</span><span class="count">{{.Counter}}</span>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	handler := tmpl.Handle(&flashIntegController{}, AsState(&flashIntegState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws := connectWS(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Step 1: SetTransientFlash via ctx.SetFlash(..., FlashExpiry(1ms)).
	// The first render includes the flash slot.
	sendWSAction(t, ws, "SetTransientFlash", nil)
	resp1 := readWSUpdate(t, ws, 2*time.Second)
	tree1, ok := resp1["tree"].(map[string]any)
	if !ok {
		t.Fatalf("SetTransientFlash response has no tree: %#v", resp1)
	}
	if v := fmt.Sprintf("%v", tree1["0"]); v != "transient" {
		t.Fatalf("SetTransientFlash: flash slot = %q, want %q", v, "transient")
	}

	// Step 2: Increment — Counter changes (0→1), expiry has elapsed.
	// Sleep 5ms to guarantee the 1ms expiry has elapsed even on a loaded CI
	// machine; the WS round-trip alone is usually sufficient, but this makes
	// the test reliable under -race and heavy load without a meaningful slowdown.
	time.Sleep(5 * time.Millisecond)
	// pruneExpiredFlash removes the lapsed flash before the render,
	// so slot 0 must be absent from the diff.
	sendWSAction(t, ws, "Increment", nil)
	resp2 := readWSUpdate(t, ws, 2*time.Second)
	tree2, ok := resp2["tree"].(map[string]any)
	if !ok {
		t.Fatalf("Increment response has no tree: %#v", resp2)
	}
	if flashVal, present := tree2["0"]; present {
		t.Errorf("flash slot present after 1ms expiry: got %q — FlashExpiry not enforced through public ctx.SetFlash API", flashVal)
	}
}
