package pubsub

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// Relay-level cross-instance benchmarks: PublishToTopic on instance A →
// topic-action handler firing on instance B, over an in-process miniredis
// (no Docker). This measures the broadcaster's serialize + relay + dispatch
// overhead in isolation; the full-pipeline cross-instance fan-out (render +
// diff + send per subscriber) is BenchmarkRedisCrossInstanceFanout in the
// root package.
//
// Honest caveat (plan): cross-instance Redis fan-out protects an ADVERTISED
// horizontal-scaling claim — no surveyed consumer app uses it in production
// (they all use in-process fan-out). miniredis models the protocol, not
// network latency; the real-Redis variant below (LVT_REDIS_INTEGRATION)
// covers wire RTT.

// newRelayPair returns two broadcaster instances ("two logical LiveTemplate
// instances") connected to the given Redis address, with B's pump running, a
// hits channel firing per delivered topic action, and B subscribed to topic.
func newRelayPair(tb testing.TB, addr, topic string) (brA *RedisBroadcaster, hits chan struct{}) {
	tb.Helper()
	// Silence the broadcaster's lifecycle INFO logs (they interleave with
	// benchmark output). Mirrors the root package's discardLogs helper,
	// which is not importable from here.
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	tb.Cleanup(func() { slog.SetDefault(prev) })
	newClient := func() redis.UniversalClient {
		return redis.NewClient(&redis.Options{Addr: addr, DialTimeout: 5 * time.Second})
	}
	brA = NewRedisBroadcaster(newClient())
	brB := NewRedisBroadcaster(newClient())
	tb.Cleanup(func() { _ = brA.Close(); _ = brB.Close() })

	hits = make(chan struct{}, 1024)
	if err := brB.RegisterTopicActionHandler(func(*GroupActionMessage) error {
		hits <- struct{}{}
		return nil
	}); err != nil {
		tb.Fatalf("RegisterTopicActionHandler: %v", err)
	}
	// The pump must be running for the topic handler to fire.
	if err := brB.Subscribe(func(*BroadcastMessage) error { return nil }); err != nil {
		tb.Fatalf("Subscribe: %v", err)
	}
	if err := brB.SubscribeToTopicChannel(topic); err != nil {
		tb.Fatalf("SubscribeToTopicChannel: %v", err)
	}
	return brA, hits
}

// runTopicRelayBench measures one publish → remote-handler-delivery round
// trip per op. A warmup publish-retry loop absorbs SUBSCRIBE propagation so
// the timed region only sees the steady state.
func runTopicRelayBench(b *testing.B, brA *RedisBroadcaster, hits chan struct{}, topic string) {
	b.Helper()
	payload := map[string]interface{}{"value": "tick"}

	warmDeadline := time.Now().Add(15 * time.Second)
warmup:
	for {
		if err := brA.PublishToTopic(topic, "SyncValue", payload); err != nil {
			b.Fatalf("warmup publish: %v", err)
		}
		select {
		case <-hits:
			break warmup
		case <-time.After(200 * time.Millisecond):
			if time.Now().After(warmDeadline) {
				b.Fatal("warmup: no round-trip within 15s")
			}
		}
	}
drain:
	for { // drain warmup stragglers
		select {
		case <-hits:
		default:
			break drain
		}
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := brA.PublishToTopic(topic, "SyncValue", payload); err != nil {
			b.Fatalf("publish: %v", err)
		}
		<-hits
	}
}

// BenchmarkRedisTopicRelay measures the per-publish cross-instance relay
// round trip over in-process miniredis. Compare with the in-process
// enqueue-only BenchmarkTopicFanoutByN_EnqueueOnly and the full-pipeline
// BenchmarkTopicFanout_FullPipeline (root package) to see what the Redis hop
// itself adds.
func BenchmarkRedisTopicRelay(b *testing.B) {
	mr := miniredis.RunT(b)
	brA, hits := newRelayPair(b, mr.Addr(), "bench/topic")
	runTopicRelayBench(b, brA, hits, "bench/topic")
}

// BenchmarkRedisTopicRelay_RealRedis is the same round trip over a real
// Redis, gated behind LVT_REDIS_INTEGRATION (set it to the Redis address,
// e.g. "localhost:6379"). It adds genuine network RTT that miniredis cannot
// model; run occasionally for the latency story, not in CI.
func BenchmarkRedisTopicRelay_RealRedis(b *testing.B) {
	addr := os.Getenv("LVT_REDIS_INTEGRATION")
	if addr == "" {
		b.Skip("set LVT_REDIS_INTEGRATION=<redis addr> to run the real-Redis relay benchmark")
	}
	brA, hits := newRelayPair(b, addr, "bench/topic")
	runTopicRelayBench(b, brA, hits, "bench/topic")
}
