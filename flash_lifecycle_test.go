package livetemplate

import (
	"testing"
	"time"

	lvtcontext "github.com/livetemplate/livetemplate/internal/context"
)

// newFlashState is a test helper that returns a minimal connState with the
// messages map initialised, ready for flash lifecycle tests.
func newFlashState() *connState {
	return &connState{
		messages: make(map[string]string),
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
