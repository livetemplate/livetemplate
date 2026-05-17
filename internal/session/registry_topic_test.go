package session

import (
	"sort"
	"strings"
	"testing"
)

// testSegmentMatch is a deliberately-independent, known-correct matcher used to
// drive GetByTopicExcept's union/dedup logic. The real matcher lives in
// package livetemplate (topics.go) and cannot be imported here (import cycle) —
// GetByTopicExcept takes the matcher as a parameter precisely so this layer is
// testable in isolation. This is NOT a copy of the contributed segmentMatch;
// it only needs to be correct enough to exercise the registry.
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
	sort.Strings(out)
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
	if diff := connIDs(got, id); !equalStrings(diff, want) {
		t.Errorf("GetByTopicExcept(room/42) = %v, want %v (A once, B via pattern, C excluded by topic, P excluded as sender)", diff, want)
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

func TestUnregister_TopicGC(t *testing.T) {
	r := NewConnectionRegistry()
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

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
