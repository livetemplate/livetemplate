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

			// Warmup: adding a channel to an already-active go-redis PubSub
			// does not synchronously confirm the SUBSCRIBE, so a single warm
			// publish can race it and be lost (pub/sub has no replay) —
			// benchmark rounds re-run this whole setup, so the race recurs
			// per round. Re-publish until every subscriber has delivered at
			// least one dispatched frame, then let the extra in-flight
			// dispatches settle and drain the stale wakeup ticks. Mirrors
			// TestTopic_Phase2_CrossInstanceRoundTripVsGroupAction's
			// warmup-retry.
			warmDeadline := time.Now().Add(15 * time.Second)
			initialMsgs := make([]int64, len(subscribers))
			for i, s := range subscribers {
				initialMsgs[i] = s.conn.MsgsWritten()
			}
			for {
				publisher.dispatch(b, topicFanoutFrames[1])
				time.Sleep(50 * time.Millisecond)
				warmed := 0
				for i, s := range subscribers {
					if s.conn.MsgsWritten() > initialMsgs[i] {
						warmed++
					}
				}
				if warmed == len(subscribers) {
					break
				}
				if time.Now().After(warmDeadline) {
					b.Fatalf("warmup: only %d/%d subscribers received the publish within 15s", warmed, len(subscribers))
				}
			}
			for prev := int64(-1); ; {
				cur := int64(0)
				for _, s := range all {
					cur += s.conn.MsgsWritten()
				}
				if cur == prev {
					break
				}
				if time.Now().After(warmDeadline) {
					b.Fatal("settle: writes never quiesced within the warmup deadline")
				}
				prev = cur
				time.Sleep(50 * time.Millisecond)
			}
			for _, s := range all {
				select {
				case <-s.conn.Writes():
				default:
				}
			}
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
