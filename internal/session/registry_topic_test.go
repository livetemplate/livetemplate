package session

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

// testSegmentMatch is a deliberately-independent, known-correct matcher used to
// drive GetByTopicExcept's union/dedup logic. The real matcher lives in
// package livetemplate (topics.go) and cannot be imported here (import cycle) —
// GetByTopicExcept takes the matcher as a parameter precisely so this layer is
// testable in isolation. This is NOT the root-package segmentMatch; it only
// needs to be correct enough to exercise the registry's union/dedup.
func testSegmentMatch(pattern, concrete string) bool {
	p := strings.Split(pattern, "/")
	c := strings.Split(concrete, "/")
	if len(p) != len(c) {
		return false
	}
	for i := range p {
		if p[i] == "*" {
			if c[i] == "" {
				return false
			}
			continue
		}
		if p[i] != c[i] {
			return false
		}
	}
	return true
}

func connIDs(conns []*Connection, id map[*Connection]string) []string {
	out := make([]string, 0, len(conns))
	for _, c := range conns {
		out = append(out, id[c])
	}
	slices.Sort(out)
	return out
}

func TestSubscribeConnectionToTopic_ExactAndPattern(t *testing.T) {
	r := NewConnectionRegistry()
	conn := &Connection{}

	r.SubscribeConnectionToTopic(conn, "room/42") // exact
	r.SubscribeConnectionToTopic(conn, "room/*")  // pattern

	if len(r.byTopic["room/42"]) != 1 {
		t.Errorf("byTopic[room/42] = %d conns, want 1", len(r.byTopic["room/42"]))
	}
	if len(r.byTopicPattern["room/*"]) != 1 {
		t.Errorf("byTopicPattern[room/*] = %d conns, want 1", len(r.byTopicPattern["room/*"]))
	}
	if _, ok := conn.subscribedTopics["room/42"]; !ok {
		t.Error("subscribedTopics missing exact topic")
	}
	if _, ok := conn.subscribedTopics["room/*"]; !ok {
		t.Error("subscribedTopics missing pattern topic")
	}
}

func TestGetByTopicExcept_DedupedUnion(t *testing.T) {
	r := NewConnectionRegistry()
	connA := &Connection{}
	connB := &Connection{}
	connC := &Connection{}
	publisher := &Connection{}
	id := map[*Connection]string{connA: "A", connB: "B", connC: "C", publisher: "P"}

	// connA matches "room/42" via the exact index AND via two of its own
	// patterns — it must still appear exactly once.
	r.SubscribeConnectionToTopic(connA, "room/42")
	r.SubscribeConnectionToTopic(connA, "room/*")
	r.SubscribeConnectionToTopic(connA, "*/42")
	// connB matches only via a pattern.
	r.SubscribeConnectionToTopic(connB, "room/*")
	// connC is subscribed to an unrelated topic.
	r.SubscribeConnectionToTopic(connC, "other/9")
	// publisher is subscribed but is the excluded sender.
	r.SubscribeConnectionToTopic(publisher, "room/42")

	got := r.GetByTopicExcept("room/42", publisher, testSegmentMatch)
	want := []string{"A", "B"}
	if ids := connIDs(got, id); !slices.Equal(ids, want) {
		t.Errorf("GetByTopicExcept(room/42) = %v, want %v (A once, B via pattern, C excluded by topic, P excluded as sender)", ids, want)
	}
}

func TestGetByTopicExcept_ExcludesSender(t *testing.T) {
	r := NewConnectionRegistry()
	sender := &Connection{}
	r.SubscribeConnectionToTopic(sender, "feed")

	if got := r.GetByTopicExcept("feed", sender, testSegmentMatch); len(got) != 0 {
		t.Errorf("sender must be excluded from its own publish, got %d conns", len(got))
	}
}

// TestGetByTopicExcept_ExcludesSenderViaPattern pins sender exclusion when the
// publisher matches only through a pattern (not the exact index) — the
// exclude-vs-pattern interaction the exact-only ExcludesSender test misses.
func TestGetByTopicExcept_ExcludesSenderViaPattern(t *testing.T) {
	r := NewConnectionRegistry()
	sender := &Connection{}
	other := &Connection{}
	r.SubscribeConnectionToTopic(sender, "room/*") // sender via pattern only
	r.SubscribeConnectionToTopic(other, "room/*")

	got := r.GetByTopicExcept("room/42", sender, testSegmentMatch)
	if len(got) != 1 || got[0] != other {
		t.Errorf("publisher matching via pattern must still be excluded; got %d conns, want [other]", len(got))
	}
}

func TestUnregister_TopicGC(t *testing.T) {
	r := NewConnectionRegistry()
	// Bare Connection (no Register): Unregister() calls conn.Close(), which
	// nil-guards its `done` channel, so this is safe without a write pump.
	conn := &Connection{}
	r.SubscribeConnectionToTopic(conn, "room/42")
	r.SubscribeConnectionToTopic(conn, "room/*")

	r.Unregister(conn)

	if len(r.byTopic) != 0 {
		t.Errorf("byTopic leaked %d keys after Unregister, want 0", len(r.byTopic))
	}
	if len(r.byTopicPattern) != 0 {
		t.Errorf("byTopicPattern leaked %d keys after Unregister, want 0", len(r.byTopicPattern))
	}
	if conn.subscribedTopics != nil {
		t.Errorf("conn.subscribedTopics = %v after Unregister, want nil", conn.subscribedTopics)
	}
}

// TestSubscribeConnectionToTopic_Idempotent pins set semantics (NOT a
// ref-count): a double subscribe is one membership, a single unsubscribe
// clears it. Guards against a future regression that makes the in-process
// membership accidentally ref-counted (the Redis layer ref-counts; this layer
// must not).
func TestSubscribeConnectionToTopic_Idempotent(t *testing.T) {
	r := NewConnectionRegistry()
	conn := &Connection{}

	r.SubscribeConnectionToTopic(conn, "room/1")
	r.SubscribeConnectionToTopic(conn, "room/1") // duplicate

	if n := len(r.byTopic["room/1"]); n != 1 {
		t.Fatalf("byTopic[room/1] = %d after double subscribe, want 1 (set, not ref-count)", n)
	}

	r.UnsubscribeConnectionFromTopic(conn, "room/1") // single unsubscribe

	if got := r.GetByTopicExcept("room/1", nil, testSegmentMatch); len(got) != 0 {
		t.Errorf("after one unsubscribe the conn must be gone, got %d (ref-count leak?)", len(got))
	}
	if _, ok := conn.subscribedTopics["room/1"]; ok {
		t.Error("subscribedTopics still has room/1 after unsubscribe")
	}
}

func TestUnsubscribeConnectionFromTopic_NoOpWhenNotSubscribed(t *testing.T) {
	r := NewConnectionRegistry()
	conn := &Connection{}

	// Never subscribed — must not panic and must not create entries.
	r.UnsubscribeConnectionFromTopic(conn, "never/subscribed")

	if len(r.byTopic) != 0 || len(r.byTopicPattern) != 0 {
		t.Error("unsubscribe of a never-subscribed topic created index entries")
	}
}

// TestGetByTopicExcept_EmptyRegistry locks the zero-subscriber path: a publish
// to a topic nobody is subscribed to (fresh registry, both indexes empty) must
// return an empty slice and never nil-panic — the first publish to an
// unsubscribed topic hits exactly this.
func TestGetByTopicExcept_EmptyRegistry(t *testing.T) {
	r := NewConnectionRegistry()

	if got := r.GetByTopicExcept("room/42", nil, testSegmentMatch); len(got) != 0 {
		t.Errorf("empty registry GetByTopicExcept = %d conns, want 0", len(got))
	}
	// Exact-only present, no pattern subscribers — and vice versa — still safe.
	r.SubscribeConnectionToTopic(&Connection{}, "other/topic")
	if got := r.GetByTopicExcept("room/42", nil, testSegmentMatch); len(got) != 0 {
		t.Errorf("no matching subscriber GetByTopicExcept = %d conns, want 0", len(got))
	}
}

// TestGetByTopicExcept_PanicsOnWildcardConcrete pins the other half of the
// contract: GetByTopicExcept publishes to a concrete topic, so a "*" in
// concrete is a programmer error and must panic loudly rather than
// silently mis-resolve.
func TestGetByTopicExcept_PanicsOnWildcardConcrete(t *testing.T) {
	r := NewConnectionRegistry()
	defer func() {
		if recover() == nil {
			t.Error("GetByTopicExcept with a wildcard concrete must panic")
		}
	}()
	r.GetByTopicExcept("room/*", nil, testSegmentMatch)
}

// TestGetByTopicExcept_NilMatchSafeWhenNoPatterns documents the contract: a nil
// match is safe iff there are no pattern subscribers (the pattern loop is
// skipped, so match is never invoked). With pattern subscribers present, a nil
// match panics by design (a loud programmer error — never a silent exact-only
// degradation); that path is intentionally not exercised here.
func TestGetByTopicExcept_NilMatchSafeWhenNoPatterns(t *testing.T) {
	r := NewConnectionRegistry()
	conn := &Connection{}
	r.SubscribeConnectionToTopic(conn, "exact/only") // exact, no patterns indexed

	got := r.GetByTopicExcept("exact/only", nil, nil) // nil match — must not panic
	if len(got) != 1 {
		t.Errorf("nil match with no pattern subscribers = %d conns, want 1", len(got))
	}
}

// TestUnsubscribeConnectionFromTopic_DifferentTopicKeepsOthers covers the
// non-nil-map path the no-op test does not: a connection subscribed to A,
// unsubscribing a different topic B it never held, must still receive A.
func TestUnsubscribeConnectionFromTopic_DifferentTopicKeepsOthers(t *testing.T) {
	r := NewConnectionRegistry()
	conn := &Connection{}
	r.SubscribeConnectionToTopic(conn, "room/A")

	r.UnsubscribeConnectionFromTopic(conn, "room/B") // never subscribed to B

	if got := r.GetByTopicExcept("room/A", nil, testSegmentMatch); len(got) != 1 {
		t.Errorf("unsubscribing a different topic dropped room/A: got %d, want 1", len(got))
	}
	if _, ok := conn.subscribedTopics["room/A"]; !ok {
		t.Error("subscribedTopics lost room/A after unsubscribing an unrelated topic")
	}
}

// TestConcurrentTopicSubscription gives the race detector something to
// actually catch: sequential tests exercise lock correctness but not
// contention. Each worker owns disjoint (conn, topic) pairs so the final state
// is deterministic (every subscribe is matched by an unsubscribe → empty
// registry), while a reader hammers GetByTopicExcept concurrently. Run with
// -race. This validates the registry's lock discipline; it does NOT exercise a
// subscribe-after-Unregister race (no Unregister caller here — that needs a
// connection-liveness guard this layer does not yet have).
func TestConcurrentTopicSubscription(t *testing.T) {
	r := NewConnectionRegistry()
	const workers, iters = 8, 200

	var workerWg sync.WaitGroup
	stop := make(chan struct{})
	readerDone := make(chan struct{})

	// Reader hammers the RLock path while writers contend on Lock.
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
				_ = r.GetByTopicExcept("room/42", nil, testSegmentMatch)
			}
		}
	}()

	for w := 0; w < workers; w++ {
		workerWg.Add(1)
		go func(w int) {
			defer workerWg.Done()
			conn := &Connection{}
			exact := fmt.Sprintf("room/w%d", w)    // disjoint per worker
			pattern := fmt.Sprintf("org/w%d/*", w) // → deterministic final state
			for i := 0; i < iters; i++ {
				r.SubscribeConnectionToTopic(conn, exact)
				r.SubscribeConnectionToTopic(conn, pattern)
				r.UnsubscribeConnectionFromTopic(conn, exact)
				r.UnsubscribeConnectionFromTopic(conn, pattern)
			}
		}(w)
	}

	workerWg.Wait() // every subscribe was matched 1:1 by an unsubscribe
	close(stop)
	<-readerDone

	if len(r.byTopic) != 0 || len(r.byTopicPattern) != 0 {
		t.Errorf("concurrent subscribe/unsubscribe left residue: byTopic=%d byTopicPattern=%d",
			len(r.byTopic), len(r.byTopicPattern))
	}
}
