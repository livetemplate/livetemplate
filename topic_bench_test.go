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
			conns := make([]*session.Connection, 0, n)
			for i := 0; i < n; i++ {
				conn := &session.Connection{GroupID: fmt.Sprintf("g%d", i), UserID: fmt.Sprintf("u%d", i)}
				reg.Register(conn, 8)
				reg.SubscribeConnectionToTopic(conn, "bench/topic")
				conns = append(conns, conn)
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
			// Explicit teardown, panic-safe (a failed iteration would otherwise
			// leak the drain goroutines + registry entries).
			b.Cleanup(func() {
				close(stop)
				for _, c := range conns {
					reg.Unregister(c)
				}
			})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.dispatchToTopic("bench/topic", nil, "Reload", nil)
			}
			b.StopTimer()
		})
	}
}

// BenchmarkTopicPatternScanByP is §"Benchmarks required" item 2 (Phase 3): the
// wildcard pattern-scan cost as the number of DISTINCT registered patterns
// P ∈ {1,10,100} grows, against a concrete publish that matches NONE of them
// (worst case — every pattern's segmentMatch is evaluated). Confirms the flat
// O(P) segment scan in GetByTopicExcept is acceptable at expected pattern
// counts; there is NO trie/radix index by design (proposal §2 "Matcher" /
// Appendix B) — this validates the linear scan is adequate, not whether to add
// one. Patterns are mixed segment-count so the scan exercises both the
// fast count-mismatch reject and the first-literal-mismatch reject.
func BenchmarkTopicPatternScanByP(b *testing.B) {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	b.Cleanup(func() { slog.SetDefault(prev) })

	for _, p := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("P=%d", p), func(b *testing.B) {
			reg := session.NewConnectionRegistry()
			conns := make([]*session.Connection, 0, p)
			for i := 0; i < p; i++ {
				conn := &session.Connection{GroupID: fmt.Sprintf("g%d", i), UserID: fmt.Sprintf("u%d", i)}
				reg.Register(conn, 8)
				// Mixed segment counts: half 2-seg, half 4-seg, each with a
				// distinct leading literal so the concrete below matches none.
				var pat string
				if i%2 == 0 {
					pat = fmt.Sprintf("scope%d/*", i)
				} else {
					pat = fmt.Sprintf("org%d/*/room/*", i)
				}
				reg.SubscribeConnectionToTopic(conn, pat)
				conns = append(conns, conn)
			}
			h := &liveHandler{registry: reg, config: mountConfig{}}
			b.Cleanup(func() {
				for _, c := range conns {
					reg.Unregister(c)
				}
			})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// "nomatch/x" matches no scope%d/* (literal differs) and no
				// 4-seg pattern (count differs) ⇒ all P patterns are scanned.
				h.dispatchToTopic("nomatch/x", nil, "Reload", nil)
			}
			b.StopTimer()
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
	// Informational measurement, not a hard perf gate: the 3x bound only
	// guards against an order-of-magnitude regression. CI timing is noisy, so
	// skip under -short; the numbers are reported via t.Logf for the PR/record.
	if testing.Short() {
		t.Skip("skipping cross-instance latency measurement under -short (timing-sensitive, informational)")
	}
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

	const iters = 50
	measure := func(publish func() error, hits <-chan struct{}) time.Duration {
		// Warmup makes the SUBSCRIBE-propagation wait non-load-bearing: retry
		// publish until the first round-trip actually completes (or a generous
		// deadline), instead of a fixed sleep that flakes on a slow CI Redis.
		// Topic pub/sub has no replay, so re-publishing is the correct probe.
		warmDeadline := time.Now().Add(15 * time.Second)
		for {
			if err := publish(); err != nil {
				t.Fatalf("warmup publish failed: %v", err)
			}
			select {
			case <-hits:
				goto timed // SUBSCRIBE confirmed live end-to-end
			case <-time.After(200 * time.Millisecond):
				if time.Now().After(warmDeadline) {
					t.Fatal("warmup: no round-trip within 15s (Redis SUBSCRIBE never propagated)")
				}
			}
		}
	timed:
		// Drain any warmup stragglers so they don't count toward a timed iter.
		for {
			select {
			case <-hits:
			default:
				goto run
			}
		}
	run:
		var total time.Duration
		for i := 0; i < iters; i++ {
			start := time.Now()
			if err := publish(); err != nil {
				t.Fatalf("publish failed: %v", err)
			}
			select {
			case <-hits:
				total += time.Since(start)
			case <-time.After(3 * time.Second):
				t.Fatalf("round-trip %d timed out", i)
			}
		}
		return total / time.Duration(iters)
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
