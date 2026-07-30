package livetemplate

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/livetemplate/livetemplate/pubsub"
)

// BenchmarkRedisCrossInstanceFanout measures the full cross-instance
// pipeline over in-process miniredis (no Docker): a publisher action on
// instance A → dispatchToTopic → PublishToTopic → Redis → instance B's
// subscriber pump → dispatch to each of B's N local subscribers → per-
// subscriber action + render + diff + serialize through the real writePump.
//
// Honest caveat (plan): this protects the ADVERTISED horizontal-scaling
// claim — no surveyed consumer app uses Redis fan-out in production (all use
// in-process fan-out). miniredis models the pub/sub protocol, not network
// latency; see BenchmarkRedisTopicRelay_RealRedis (pubsub package) for the
// wire-RTT story.
func BenchmarkRedisCrossInstanceFanout(b *testing.B) {
	discardLogs(b)
	for _, n := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			mr := miniredis.RunT(b)
			newBroadcaster := func() *pubsub.RedisBroadcaster {
				client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DialTimeout: 5 * time.Second})
				br := pubsub.NewRedisBroadcaster(client)
				b.Cleanup(func() { _ = br.Close() })
				return br
			}

			appA := newCompositeApp(b, topicFanoutTemplate, &topicFanoutController{},
				AsState(&topicFanoutState{}), WithOpenTopics(), WithPubSubBroadcaster(newBroadcaster()))
			appB := newCompositeApp(b, topicFanoutTemplate, &topicFanoutController{},
				AsState(&topicFanoutState{}), WithOpenTopics(), WithPubSubBroadcaster(newBroadcaster()))

			publisher := appA.connect(b, "")
			subscribers := make([]*compositeSession, n)
			for i := range subscribers {
				subscribers[i] = appB.connect(b, "")
			}
			all := append([]*compositeSession{publisher}, subscribers...)

			// One warm op: connect-time SubscribeToTopicChannel blocks until
			// Redis confirms, so no retry loop is needed — this just primes
			// the path before timing.
			publisher.dispatch(b, topicFanoutFrames[1])
			awaitAll(b, subscribers)
			startBytes := wireBytesTotal(all...)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				publisher.dispatch(b, topicFanoutFrames[i%2])
				awaitAll(b, subscribers)
			}
			b.StopTimer()
			b.ReportMetric(float64(wireBytesTotal(all...)-startBytes)/float64(b.N), "wireB/op")
		})
	}
}
