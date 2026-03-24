package session

import (
	"fmt"
	"testing"
)

const benchWSTextMessage = 1 // RFC 6455 text message type

// newDrainingBenchConn creates a connection with a dedicated drain goroutine
// that consumes messages from sendChan, preventing "client too slow" errors.
// The drain goroutine runs until cleanup is called.
func newDrainingBenchConn(groupID string, bufferSize int) (conn *Connection, cleanup func()) {
	conn = &Connection{
		GroupID: groupID,
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

// BenchmarkAsyncSendThroughput measures message sending throughput
// with async channel-based sending.
func BenchmarkAsyncSendThroughput(b *testing.B) {
	conn, cleanup := newDrainingBenchConn("bench-group", 1000)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := conn.Send(benchWSTextMessage, []byte("benchmark message")); err != nil {
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
				connections[i], cleanups[i] = newDrainingBenchConn("bench-group", 50)
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
	conn, cleanup := newDrainingBenchConn("bench-group", 10000)
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
		conn, _ := newDrainingBenchConn("bench-group", 50)
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
			conn, cleanup := newDrainingBenchConn("bench-group", bufferSize)
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
						conn, cleanup = newDrainingBenchConn("bench-group", bufferSize)
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
