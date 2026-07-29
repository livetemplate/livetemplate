package session

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/livetemplate/livetemplate/internal/benchharness"
)

const benchWSTextMessage = 1 // RFC 6455 text message type (mirrors WSTextMessage in root package)

// newDrainingBenchConn creates a connection with a dedicated drain goroutine
// that consumes messages from sendChan, preventing "client too slow" errors.
// The drain goroutine runs until cleanup is called.
//
// Hollow by design: the nil Conn means the real writePump and WriteMessage
// never run, so this measures ENQUEUE cost only. Benchmarks that claim to
// measure sending must use newRealPumpBenchConn instead.
func newDrainingBenchConn(bufferSize int) (conn *Connection, cleanup func()) {
	conn = &Connection{
		GroupID: "bench-group",
		UserID:  "bench-user",
	}
	conn.sendChan = make(chan *wsMessage, bufferSize)
	conn.done = make(chan struct{})
	conn.pumpExited = make(chan struct{})

	go func() {
		defer close(conn.pumpExited)
		for {
			select {
			case <-conn.sendChan:
			case <-conn.done:
				return
			}
		}
	}()

	return conn, func() {
		_ = conn.Close()
	}
}

// newRealPumpBenchConn registers a connection through the production
// Register path, so the REAL writePump dequeues each message and hands it
// to WriteMessage on the attached benchharness.Conn, which counts and
// discards it at the syscall boundary.
func newRealPumpBenchConn(bufferSize int) (*Connection, *benchharness.Conn, func()) {
	registry := NewConnectionRegistry()
	bc := benchharness.NewConn()
	conn := &Connection{
		Conn:    bc,
		GroupID: "bench-group",
		UserID:  "bench-user",
	}
	registry.Register(conn, bufferSize)
	return conn, bc, func() { registry.Unregister(conn) }
}

// BenchmarkAsyncSendThroughput measures send throughput through the real
// write pump: Send enqueues, the pump dequeues and writes each frame at the
// discard boundary. The timed region ends only after every queued message
// has actually been written, so the number covers the full send path, not
// just enqueue. Contrast with BenchmarkAsyncSendThroughput_EnqueueOnly,
// which measures what the pre-Phase-1 bench of this name measured.
func BenchmarkAsyncSendThroughput(b *testing.B) {
	conn, bc, cleanup := newRealPumpBenchConn(1000)
	defer func() { cleanup() }()
	payload := []byte("benchmark message")
	slowCloses := 0
	sentOnConn := 0

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := conn.Send(benchWSTextMessage, payload)
		if err == nil {
			sentOnConn++
			continue
		}
		if err != ErrClientTooSlow {
			b.Fatalf("Send failed: %v", err)
		}
		// The producer outran the pump and production closed the connection —
		// real backpressure behavior, not a bench failure. Recreate (which
		// drains the old conn) and keep going, reporting how often it happened.
		slowCloses++
		cleanup()
		conn, bc, cleanup = newRealPumpBenchConn(1000)
		sentOnConn = 0
	}
	for bc.MsgsWritten() < int64(sentOnConn) {
		runtime.Gosched()
	}
	b.StopTimer()
	if slowCloses > 0 {
		b.ReportMetric(float64(slowCloses), "slow-closes")
	}
}

// BenchmarkAsyncSendThroughput_EnqueueOnly is the old hollow benchmark,
// relabeled: its drain goroutine consumes sendChan into the void with a nil
// Conn, so the real writePump and WriteMessage never run. Kept so the
// honest-vs-enqueue-only contrast stays visible in benchmark output.
func BenchmarkAsyncSendThroughput_EnqueueOnly(b *testing.B) {
	conn, cleanup := newDrainingBenchConn(1000)
	defer func() { cleanup() }()
	payload := []byte("benchmark message")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := conn.Send(benchWSTextMessage, payload)
		if err == ErrClientTooSlow {
			// Even the drain goroutine can fall behind on a loaded machine;
			// recreate and continue, as production would after a slow-close.
			cleanup()
			conn, cleanup = newDrainingBenchConn(1000)
			continue
		}
		if err != nil {
			b.Fatalf("Send failed: %v", err)
		}
	}
}

// BenchmarkConcurrentConnections measures performance with many concurrent connections.
func BenchmarkConcurrentConnections(b *testing.B) {
	connectionCounts := []int{10, 100, 1000}

	for _, count := range connectionCounts {
		b.Run(fmt.Sprintf("%d_connections", count), func(b *testing.B) {
			connections := make([]*Connection, count)
			cleanups := make([]func(), count)

			for i := 0; i < count; i++ {
				connections[i], cleanups[i] = newDrainingBenchConn(50)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, conn := range connections {
					if err := conn.Send(benchWSTextMessage, []byte("test")); err != nil {
						b.Fatalf("Send failed: %v", err)
					}
				}
			}

			b.StopTimer()

			for _, fn := range cleanups {
				fn()
			}
		})
	}
}

// BenchmarkRegisterUnregister measures the cost of connection lifecycle.
func BenchmarkRegisterUnregister(b *testing.B) {
	registry := NewConnectionRegistry()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		conn := &Connection{
			Conn:    nil,
			GroupID: "bench-group",
			UserID:  "bench-user",
		}
		registry.Register(conn, 50)
		registry.Unregister(conn)
	}
}

// BenchmarkConcurrentSend measures concurrent sending from multiple goroutines.
func BenchmarkConcurrentSend(b *testing.B) {
	conn, cleanup := newDrainingBenchConn(10000)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := conn.Send(benchWSTextMessage, []byte("concurrent message")); err != nil {
				b.Fatalf("Send failed: %v", err)
			}
		}
	})
}

// BenchmarkGetByGroup measures connection lookup performance.
func BenchmarkGetByGroup(b *testing.B) {
	registry := NewConnectionRegistry()

	for i := 0; i < 100; i++ {
		conn := &Connection{
			Conn:    nil,
			GroupID: "bench-group",
			UserID:  "bench-user",
		}
		registry.Register(conn, 50)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		registry.GetByGroup("bench-group")
	}
}

// BenchmarkCloseConnection measures the cost of graceful connection shutdown.
func BenchmarkCloseConnection(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		conn, _ := newDrainingBenchConn(50)
		b.StartTimer()

		if err := conn.Close(); err != nil {
			b.Fatalf("Close failed: %v", err)
		}
	}
}

// BenchmarkMemoryUsage measures memory overhead per connection.
func BenchmarkMemoryUsage(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		registry := NewConnectionRegistry()
		connections := make([]*Connection, 100)
		b.StartTimer()

		for j := 0; j < 100; j++ {
			conn := &Connection{
				Conn:    nil,
				GroupID: "bench-group",
				UserID:  "bench-user",
			}
			registry.Register(conn, 50)
			connections[j] = conn
		}

		b.StopTimer()
		for _, conn := range connections {
			registry.Unregister(conn)
		}
	}
}

// BenchmarkBroadcastToGroup measures broadcast performance.
func BenchmarkBroadcastToGroup(b *testing.B) {
	registry := NewConnectionRegistry()

	connections := make([]*Connection, 100)
	for i := 0; i < 100; i++ {
		conn := &Connection{
			Conn:    nil,
			GroupID: "bench-group",
			UserID:  "bench-user",
		}
		// Large buffer prevents backpressure during broadcast
		registry.Register(conn, 10000)
		connections[i] = conn
	}

	message := []byte("broadcast message")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		groupConns := registry.GetByGroup("bench-group")
		for _, conn := range groupConns {
			if err := conn.Send(benchWSTextMessage, message); err != nil {
				b.Fatalf("Send failed: %v", err)
			}
		}
	}

	b.StopTimer()

	for _, conn := range connections {
		registry.Unregister(conn)
	}
}

// BenchmarkBufferSizes compares performance across different buffer sizes.
// Small buffers will experience backpressure (ErrClientTooSlow) under tight
// send loops, which is expected production behavior. This benchmark measures
// the Send() cost including occasional backpressure handling.
func BenchmarkBufferSizes(b *testing.B) {
	bufferSizes := []int{10, 50, 100, 500, 1000}

	for _, bufferSize := range bufferSizes {
		b.Run(fmt.Sprintf("buf_%d", bufferSize), func(b *testing.B) {
			conn, cleanup := newDrainingBenchConn(bufferSize)
			defer func() { cleanup() }()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Ignore ErrClientTooSlow — small buffers trigger backpressure
				// under tight loops, which is expected behavior
				if err := conn.Send(benchWSTextMessage, []byte("test")); err != nil {
					if err == ErrClientTooSlow {
						// Recreate connection to continue benchmarking
						cleanup()
						conn, cleanup = newDrainingBenchConn(bufferSize)
						continue
					}
					b.Fatalf("Send failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkConcurrentRegistrations measures concurrent registration performance.
func BenchmarkConcurrentRegistrations(b *testing.B) {
	registry := NewConnectionRegistry()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn := &Connection{
				Conn:    nil,
				GroupID: "bench-group",
				UserID:  "bench-user",
			}
			registry.Register(conn, 50)
			registry.Unregister(conn)
		}
	})
}

// BenchmarkGetByGroupExcept measures filtered group lookup performance.
func BenchmarkGetByGroupExcept(b *testing.B) {
	registry := NewConnectionRegistry()

	var excludeConn *Connection
	for i := 0; i < 100; i++ {
		conn := &Connection{
			Conn:    nil,
			GroupID: "bench-group",
			UserID:  "bench-user",
		}
		registry.Register(conn, 50)
		if i == 50 {
			excludeConn = conn
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		registry.GetByGroupExcept("bench-group", excludeConn)
	}
}
