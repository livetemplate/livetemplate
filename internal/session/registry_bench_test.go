package session

import (
	"testing"

	"github.com/gorilla/websocket"
)

// BenchmarkAsyncSendThroughput measures message sending throughput
// with async channel-based sending.
func BenchmarkAsyncSendThroughput(b *testing.B) {
	registry := NewConnectionRegistry()

	conn := &Connection{
		Conn:    nil, // Nil for benchmark
		GroupID: "bench-group",
		UserID:  "bench-user",
	}

	registry.Register(conn, 1000) // Large buffer for throughput test
	defer registry.Unregister(conn)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := conn.Send(websocket.TextMessage, []byte("benchmark message")); err != nil {
			b.Fatalf("Send failed: %v", err)
		}
	}
}

// BenchmarkConcurrentConnections measures performance with many concurrent connections.
func BenchmarkConcurrentConnections(b *testing.B) {
	connectionCounts := []int{10, 100, 1000}

	for _, count := range connectionCounts {
		b.Run(string(rune('0'+count/100))+"00_connections", func(b *testing.B) {
			registry := NewConnectionRegistry()
			connections := make([]*Connection, count)

			// Create connections
			for i := 0; i < count; i++ {
				conn := &Connection{
					Conn:    nil,
					GroupID: "bench-group",
					UserID:  "bench-user",
				}
				registry.Register(conn, 50)
				connections[i] = conn
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Send messages concurrently to all connections
			for i := 0; i < b.N; i++ {
				for _, conn := range connections {
					if err := conn.Send(websocket.TextMessage, []byte("test")); err != nil {
						b.Fatalf("Send failed: %v", err)
					}
				}
			}

			b.StopTimer()

			// Cleanup
			for _, conn := range connections {
				registry.Unregister(conn)
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
	registry := NewConnectionRegistry()

	conn := &Connection{
		Conn:    nil,
		GroupID: "bench-group",
		UserID:  "bench-user",
	}

	registry.Register(conn, 10000) // Large buffer
	defer registry.Unregister(conn)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := conn.Send(websocket.TextMessage, []byte("concurrent message")); err != nil {
				b.Fatalf("Send failed: %v", err)
			}
		}
	})
}

// BenchmarkGetByGroup measures connection lookup performance.
func BenchmarkGetByGroup(b *testing.B) {
	registry := NewConnectionRegistry()

	// Create 100 connections in the same group
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
		registry := NewConnectionRegistry()
		conn := &Connection{
			Conn:    nil,
			GroupID: "bench-group",
			UserID:  "bench-user",
		}
		registry.Register(conn, 50)
		b.StartTimer()

		if err := conn.Close(); err != nil {
			b.Fatalf("Close failed: %v", err)
		}
	}
}

// BenchmarkMemoryUsage measures memory overhead per connection.
// This benchmark measures the memory cost of creating connections.
func BenchmarkMemoryUsage(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		registry := NewConnectionRegistry()
		connections := make([]*Connection, 100)
		b.StartTimer()

		// Create 100 connections
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
		// Cleanup
		for _, conn := range connections {
			registry.Unregister(conn)
		}
	}
}

// BenchmarkBroadcastToGroup measures broadcast performance.
func BenchmarkBroadcastToGroup(b *testing.B) {
	registry := NewConnectionRegistry()

	// Create 100 connections in the same group
	connections := make([]*Connection, 100)
	for i := 0; i < 100; i++ {
		conn := &Connection{
			Conn:    nil,
			GroupID: "bench-group",
			UserID:  "bench-user",
		}
		registry.Register(conn, 1000)
		connections[i] = conn
	}

	message := []byte("broadcast message")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		groupConns := registry.GetByGroup("bench-group")
		for _, conn := range groupConns {
			if err := conn.Send(websocket.TextMessage, message); err != nil {
				b.Fatalf("Send failed: %v", err)
			}
		}
	}

	b.StopTimer()

	// Cleanup
	for _, conn := range connections {
		registry.Unregister(conn)
	}
}

// BenchmarkBufferSizes compares performance across different buffer sizes.
func BenchmarkBufferSizes(b *testing.B) {
	bufferSizes := []int{10, 50, 100, 500, 1000}

	for _, bufferSize := range bufferSizes {
		b.Run(string(rune('0'+bufferSize/100))+"_size", func(b *testing.B) {
			registry := NewConnectionRegistry()
			conn := &Connection{
				Conn:    nil,
				GroupID: "bench-group",
				UserID:  "bench-user",
			}
			registry.Register(conn, bufferSize)
			defer registry.Unregister(conn)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				if err := conn.Send(websocket.TextMessage, []byte("test")); err != nil {
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

	// Create 100 connections in the same group
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
