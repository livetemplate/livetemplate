package session

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

const wsTextMessage = 1 // RFC 6455 text message type (mirrors WSTextMessage in root package)

// TestWritePumpRegistrationAndCleanup verifies that writePump goroutines
// are started on Register() and cleaned up on Unregister().
func TestWritePumpRegistrationAndCleanup(t *testing.T) {
	// This test verifies goroutine lifecycle without needing real WebSocket connections
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	registry := NewConnectionRegistry()

	// Register 10 connections (should start 10 writePumps)
	var connections []*Connection
	for i := 0; i < 10; i++ {
		conn := &Connection{
			Conn:    nil, // Nil conn, won't actually write
			GroupID: "test-group",
			UserID:  "test-user",
		}
		registry.Register(conn, 10)
		connections = append(connections, conn)
	}

	// Give writePump goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Unregister all connections
	for _, conn := range connections {
		registry.Unregister(conn)
	}

	// Give time for cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	final := runtime.NumGoroutine()
	diff := final - baseline
	if diff < 0 {
		diff = -diff
	}

	// Allow small variance for test framework
	if diff > 5 {
		t.Errorf("Potential goroutine leak: baseline=%d final=%d (diff=%d)", baseline, final, diff)
	}
}

// TestSendWithNilConnection verifies that Send() handles nil connections gracefully.
func TestSendWithNilConnection(t *testing.T) {
	conn := &Connection{
		Conn:    nil,
		GroupID: "test-group",
		UserID:  "test-user",
	}

	// Send without initializing async infrastructure (no Register call)
	err := conn.Send(wsTextMessage, []byte("test"))
	if err != nil {
		t.Fatalf("Send with nil conn failed: %v", err)
	}
}

// TestCloseWithNilConnection verifies that Close() handles nil connections gracefully.
func TestCloseWithNilConnection(t *testing.T) {
	conn := &Connection{
		Conn:    nil,
		GroupID: "test-group",
		UserID:  "test-user",
	}

	err := conn.Close()
	if err != nil {
		t.Fatalf("Close with nil conn failed: %v", err)
	}
}

// TestCloseIsIdempotent verifies that multiple Close() calls don't panic
// due to sync.Once protection.
func TestCloseIsIdempotent(t *testing.T) {
	registry := NewConnectionRegistry()

	conn := &Connection{
		Conn:    nil,
		GroupID: "test-group",
		UserID:  "test-user",
	}

	registry.Register(conn, 10)

	// Call Close() multiple times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := conn.Close(); err != nil {
				// First Close() succeeds, subsequent ones may return nil (idempotent)
				// so we don't fail the test here
				t.Logf("Close() returned: %v", err)
			}
		}()
	}

	wg.Wait()

	// Give Close() time to fully complete (close done channel)
	time.Sleep(20 * time.Millisecond)

	// If we get here without panic, sync.Once worked
	// Verify connection is actually closed
	err := conn.Send(wsTextMessage, []byte("test"))
	if err != ErrConnectionClosed {
		t.Errorf("Expected ErrConnectionClosed after Close(), got: %v", err)
	}
}

// TestNoGoroutineLeaks is the CRITICAL test that verifies writePump goroutines
// properly exit and don't leak. Tests 10,000 connection cycles.
func TestNoGoroutineLeaks(t *testing.T) {
	// Force GC to get baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	registry := NewConnectionRegistry()

	for i := 0; i < 10000; i++ {
		conn := &Connection{
			Conn:    nil,
			GroupID: "test-group",
			UserID:  "test-user",
		}

		// Register (starts writePump)
		registry.Register(conn, 50)

		// Unregister (should clean up writePump)
		registry.Unregister(conn)
	}

	// Force GC to clean up
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	final := runtime.NumGoroutine()

	// Allow small variance (5 goroutines) for test framework overhead
	diff := final - baseline
	if diff < 0 {
		diff = -diff
	}
	if diff > 5 {
		t.Errorf("Goroutine leak detected: baseline=%d final=%d (diff=%d)", baseline, final, diff)
	}
}

// TestExtendedGoroutineLeaks runs 1,000,000 connection cycles.
// This is skipped in short mode (-short flag).
func TestExtendedGoroutineLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extended leak test in short mode")
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	registry := NewConnectionRegistry()

	// Progress reporting every 100k iterations
	for i := 0; i < 1000000; i++ {
		if i > 0 && i%100000 == 0 {
			t.Logf("Processed %d connections...", i)
		}

		conn := &Connection{
			Conn:    nil,
			GroupID: "test-group",
			UserID:  "test-user",
		}

		registry.Register(conn, 50)
		registry.Unregister(conn)
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	final := runtime.NumGoroutine()
	diff := final - baseline
	if diff < 0 {
		diff = -diff
	}
	if diff > 5 {
		t.Errorf("Goroutine leak in extended test: baseline=%d final=%d (diff=%d)", baseline, final, diff)
	}
}

// TestSendAfterClose verifies that Send() returns ErrConnectionClosed
// after the connection is closed.
func TestSendAfterClose(t *testing.T) {
	registry := NewConnectionRegistry()

	conn := &Connection{
		Conn:    nil,
		GroupID: "test-group",
		UserID:  "test-user",
	}

	registry.Register(conn, 10)

	// Close the connection
	if err := conn.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Send should fail with ErrConnectionClosed
	err := conn.Send(wsTextMessage, []byte("test"))
	if err != ErrConnectionClosed {
		t.Errorf("Expected ErrConnectionClosed, got: %v", err)
	}
}

// TestConcurrentRegisterUnregister verifies that concurrent Register()
// and Unregister() operations don't cause race conditions.
func TestConcurrentRegisterUnregister(t *testing.T) {
	registry := NewConnectionRegistry()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent registrations and unregistrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				conn := &Connection{
					Conn:    nil,
					GroupID: "test-group",
					UserID:  "test-user",
				}
				registry.Register(conn, 50)
				time.Sleep(1 * time.Millisecond)
				registry.Unregister(conn)
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is still functional
	testConn := &Connection{
		Conn:    nil,
		GroupID: "test",
		UserID:  "test",
	}
	registry.Register(testConn, 50)

	if registry.Count() == 0 {
		t.Error("Registry corrupted after concurrent access")
	}

	registry.Unregister(testConn)
}

// TestBufferSizeConfiguration verifies that different buffer sizes work correctly.
func TestBufferSizeConfiguration(t *testing.T) {
	registry := NewConnectionRegistry()

	testCases := []int{1, 10, 50, 100, 1000}

	for _, bufferSize := range testCases {
		conn := &Connection{
			Conn:    nil,
			GroupID: "test-group",
			UserID:  "test-user",
		}

		registry.Register(conn, bufferSize)

		// Verify send channel has correct capacity
		if cap(conn.sendChan) != bufferSize {
			t.Errorf("Buffer size %d: expected capacity %d, got %d",
				bufferSize, bufferSize, cap(conn.sendChan))
		}

		registry.Unregister(conn)
	}
}

// TestAsyncInfrastructureInitialization verifies that Register() properly
// initializes all async sending infrastructure.
func TestAsyncInfrastructureInitialization(t *testing.T) {
	registry := NewConnectionRegistry()

	conn := &Connection{
		Conn:    nil,
		GroupID: "test-group",
		UserID:  "test-user",
	}

	// Before registration, channels should be nil
	if conn.sendChan != nil {
		t.Error("sendChan should be nil before Register()")
	}
	if conn.done != nil {
		t.Error("done should be nil before Register()")
	}
	if conn.pumpExited != nil {
		t.Error("pumpExited should be nil before Register()")
	}

	registry.Register(conn, 50)

	// After registration, channels should be initialized
	if conn.sendChan == nil {
		t.Error("sendChan should be initialized after Register()")
	}
	if conn.done == nil {
		t.Error("done should be initialized after Register()")
	}
	if conn.pumpExited == nil {
		t.Error("pumpExited should be initialized after Register()")
	}

	registry.Unregister(conn)
}

// TestConcurrentSendAndClose verifies that concurrent Send() and Close()
// operations don't cause race conditions or panics.
func TestConcurrentSendAndClose(t *testing.T) {
	registry := NewConnectionRegistry()

	conn := &Connection{
		Conn:    nil,
		GroupID: "test-group",
		UserID:  "test-user",
	}

	registry.Register(conn, 100)

	var wg sync.WaitGroup

	// Start multiple senders
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = conn.Send(wsTextMessage, []byte("test"))
			}
		}()
	}

	// Close concurrently while sends are happening
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond) // Let some sends happen first
		if err := conn.Close(); err != nil {
			t.Logf("Close() returned: %v", err)
		}
	}()

	wg.Wait()

	// Verify connection is closed
	err := conn.Send(wsTextMessage, []byte("after close"))
	if err != ErrConnectionClosed {
		t.Errorf("Expected ErrConnectionClosed after concurrent close, got: %v", err)
	}
}
