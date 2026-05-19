package pubsub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// (instanceID, seq) double-fire dedup ring: pins ring mechanics + the seq==0 two-halves discipline (process all, record none) that are awkward to force over real Redis. End-to-end coverage is V17.

func TestSeenRing(t *testing.T) {
	var r seenRing

	a7 := seenID{instanceID: "A", seq: 7}
	if r.seenThenRecord(a7) {
		t.Fatal("first sighting of (A,7) must be unseen")
	}
	if !r.seenThenRecord(a7) {
		t.Fatal("second sighting of (A,7) must be seen")
	}

	// (instanceID, seq) is the whole key: same seq, different instance is a
	// distinct id (group+topic interleave one per-instance counter, so seq is
	// only unique scoped by instanceID).
	if r.seenThenRecord(seenID{instanceID: "B", seq: 7}) {
		t.Fatal("(B,7) is a distinct id from (A,7)")
	}

	// Fill past capacity so the ring wraps; the oldest distinct id is evicted
	// and a re-sighting reads as unseen (bounded window, by design — a
	// double-fire's two copies arrive back-to-back so the small window is
	// sufficient; this asserts the bound is real, not unbounded growth).
	for i := 0; i < seenIDRingSize; i++ {
		r.seenThenRecord(seenID{instanceID: "fill", seq: uint64(1000 + i)})
	}
	if r.seenThenRecord(a7) {
		t.Fatal("(A,7) must have been evicted after the ring wrapped past it")
	}
}

// fakeTopicMsg builds the raw Redis payload handleTopicActionMessage decodes.
func fakeTopicMsg(t *testing.T, instanceID string, seq uint64, topic string) *redis.Message {
	t.Helper()
	payload, err := json.Marshal(&GroupActionMessage{
		Type:       "topic_action",
		Topic:      topic,
		Action:     "Reload",
		Seq:        seq,
		Timestamp:  time.Now(),
		InstanceID: instanceID,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return &redis.Message{Channel: channelTopic + topic, Payload: string(payload)}
}

func TestHandleTopicActionMessage_DedupAndSeqZeroBypass(t *testing.T) {
	calls := 0 // handle() is called synchronously below — no concurrency, plain int
	// Struct literal: handleTopicActionMessage needs only instanceID + the
	// handler + the (zero-value-valid) seenRing/mutex — no Redis client.
	b := &RedisBroadcaster{
		instanceID: "self",
		topicActionHandler: func(*GroupActionMessage) error {
			calls++
			return nil
		},
	}
	handle := func(m *redis.Message) {
		if err := b.handleTopicActionMessage(m); err != nil {
			t.Fatalf("handleTopicActionMessage: %v", err)
		}
	}

	// Own-instance message is dropped (sanity — the InstanceID guard, unchanged).
	handle(fakeTopicMsg(t, "self", 1, "room/1"))
	if calls != 0 {
		t.Fatalf("own-instance message must not dispatch, got %d calls", calls)
	}

	// SUBSCRIBE+PSUBSCRIBE double-fire: the SAME (instanceID, seq) twice ⇒ one
	// dispatch (the ring drops the second copy).
	dup := fakeTopicMsg(t, "A", 42, "room/42")
	handle(dup)
	handle(dup)
	if calls != 1 {
		t.Fatalf("double-fire (A,42) must dispatch exactly once, got %d", calls)
	}

	// seq==0 ⇒ pre-upgrade sender: EVERY message is seq=0. Both halves of the
	// discipline: NOT dedup-checked (both process) AND never recorded (so a
	// later seq==0 from the same instance is not collapsed against a stored
	// (id,0)). Two seq=0 from "old" ⇒ TWO more dispatches.
	handle(fakeTopicMsg(t, "old", 0, "room/9"))
	handle(fakeTopicMsg(t, "old", 0, "room/9"))
	if calls != 3 {
		t.Fatalf("two seq==0 messages must BOTH dispatch (bypass+no-record), want 3 total, got %d", calls)
	}

	// A real seq from "old" interleaved with its seq==0 stream still dedups on
	// repeat (the ring keys on the pair, unaffected by the bypassed zeros).
	real := fakeTopicMsg(t, "old", 5, "room/9")
	handle(real)
	handle(real)
	handle(fakeTopicMsg(t, "old", 0, "room/9")) // still bypassed → dispatches
	if calls != 5 {
		t.Fatalf("want 5 (3 + one (old,5) deduped-once + one bypassed seq0), got %d", calls)
	}
}
