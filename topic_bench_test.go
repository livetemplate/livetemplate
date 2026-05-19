package livetemplate

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/session"
	"github.com/livetemplate/livetemplate/pubsub"
)

// ============================================================================
// Phase 2 cross-cutting benchmark gate (§"Benchmarks required" items 1 & 3;
// item 2 — wildcard pattern-scan — is Phase 3). Numbers are recorded in
// learnings/phase-2.md and the PR description.
//
//   1. Topic fan-out latency by subscriber count N — wall-clock from
//      dispatchToTopic ("Publish") to all N subscribers enqueued. Validates
//      the flat O(N) registry scan + EnqueueDispatch loop is acceptable.
//   3. Cross-instance Publish round-trip vs the existing GroupActionMessage
//      path (single Redis) — confirms the new topic channel adds no
//      measurable overhead over the baseline it is modeled on.
// ============================================================================

// BenchmarkTopicFanoutByN measures dispatchToTopic local fan-out for
// N ∈ {1,5,10,50,100}. Each subscriber drains its DispatchChan (the real
// event-loop model), so the measurement reflects "enqueued", not "dropped".
func BenchmarkTopicFanoutByN(b *testing.B) {
	// Silence slog: the dispatchToTopic slog.Debug + a transient
	// dispatch-full WARN would both pollute the timing and the output.
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	b.Cleanup(func() { slog.SetDefault(prev) })

	for _, n := range []int{1, 5, 10, 50, 100} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			reg := session.NewConnectionRegistry()
			// Generous dispatch buffer so a drain goroutine that briefly lags
			// the tight producer loop still absorbs without dropping — the
			// measurement must reflect "enqueued", not the cheaper drop path.
			reg.SetDispatchBufferSize(1 << 16)
			h := &liveHandler{registry: reg, config: mountConfig{}}
			stop := make(chan struct{})
			for i := 0; i < n; i++ {
				conn := &session.Connection{GroupID: fmt.Sprintf("g%d", i), UserID: fmt.Sprintf("u%d", i)}
				reg.Register(conn, 8)
				reg.SubscribeConnectionToTopic(conn, "bench/topic")
				go func(c *session.Connection) {
					for {
						select {
						case <-c.DispatchChan:
						case <-stop:
							return
						}
					}
				}(conn)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.dispatchToTopic("bench/topic", nil, "Reload", nil)
			}
			b.StopTimer()
			close(stop)
		})
	}
}

// TestTopic_Phase2_CrossInstanceRoundTripVsGroupAction times the topic
// round-trip (PublishToTopic on A → topic handler on B) against the existing
// PublishGroupAction baseline over one shared Redis, and logs both means + the
// ratio. The assertion is intentionally loose (topic within 3× the baseline):
// the deliverable is the recorded number showing no order-of-magnitude
// overhead, not a tight perf gate (which would flake on shared CI).
func TestTopic_Phase2_CrossInstanceRoundTripVsGroupAction(t *testing.T) {
	client := getTestRedisClient(t) // t.Skips if Docker unavailable
	bA := pubsub.NewRedisBroadcaster(client)
	bB := pubsub.NewRedisBroadcaster(client)
	t.Cleanup(func() { _ = bA.Close(); _ = bB.Close() })

	topicHits := make(chan struct{}, 1024)
	groupHits := make(chan struct{}, 1024)
	if err := bB.SubscribeToTopicActions(func(*pubsub.GroupActionMessage) error {
		topicHits <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("SubscribeToTopicActions: %v", err)
	}
	if err := bB.SubscribeGroupActions(func(*pubsub.GroupActionMessage) error {
		groupHits <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("SubscribeGroupActions: %v", err)
	}
	// The single pump must be running for either handler to fire.
	if err := bB.Subscribe(func(*pubsub.BroadcastMessage) error { return nil }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := bB.SubscribeToTopicChannel("lat/topic"); err != nil {
		t.Fatalf("SubscribeToTopicChannel: %v", err)
	}
	if err := bB.SubscribeToGroupAction("lat-grp"); err != nil {
		t.Fatalf("SubscribeToGroupAction: %v", err)
	}
	time.Sleep(300 * time.Millisecond) // let both SUBSCRIBEs propagate

	const iters = 50
	measure := func(publish func() error, hits <-chan struct{}) time.Duration {
		// Warm up + drain any stragglers.
		for {
			select {
			case <-hits:
			default:
				goto warm
			}
		}
	warm:
		var total time.Duration
		got := 0
		for i := 0; i < iters; i++ {
			start := time.Now()
			if err := publish(); err != nil {
				t.Fatalf("publish failed: %v", err)
			}
			select {
			case <-hits:
				total += time.Since(start)
				got++
			case <-time.After(3 * time.Second):
				t.Fatalf("round-trip %d timed out", i)
			}
		}
		return total / time.Duration(got)
	}

	topicMean := measure(func() error {
		return bA.PublishToTopic("lat/topic", "Reload", map[string]interface{}{"k": "v"})
	}, topicHits)
	groupMean := measure(func() error {
		return bA.PublishGroupAction("lat-grp", "Reload", map[string]interface{}{"k": "v"})
	}, groupHits)

	ratio := float64(topicMean) / float64(groupMean)
	t.Logf("cross-instance round-trip (single Redis, %d iters): topic=%v  groupAction=%v  ratio=%.2fx",
		iters, topicMean, groupMean, ratio)
	if ratio > 3.0 {
		t.Errorf("topic round-trip %v is >3x the GroupActionMessage baseline %v (ratio %.2f) — unexpected overhead",
			topicMean, groupMean, ratio)
	}
}
