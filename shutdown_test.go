package livetemplate

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ShutdownState is a test state for shutdown tests
type ShutdownState struct {
	Value int
}

// ShutdownController is a test controller for shutdown tests
type ShutdownController struct{}

func TestLiveHandler_Shutdown_RejectsNewConnections(t *testing.T) {
	tmpl := Must(New("shutdown-test"))
	handler := tmpl.Handle(&ShutdownController{}, AsState(&ShutdownState{}))

	// Start shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		_ = handler.Shutdown(ctx)
	}()

	// Give shutdown a moment to start
	time.Sleep(50 * time.Millisecond)

	// Try to get the concrete handler to check shutdown state
	lh, ok := handler.(*liveHandler)
	if !ok {
		t.Fatal("Handler is not *liveHandler")
	}

	// Verify shutdown flag is set
	if !lh.isShutdown.Load() {
		t.Error("Expected isShutdown to be true after Shutdown() called")
	}
}

func TestLiveHandler_Shutdown_Idempotent(t *testing.T) {
	tmpl := Must(New("shutdown-idempotent-test"))
	handler := tmpl.Handle(&ShutdownController{}, AsState(&ShutdownState{}))

	lh, ok := handler.(*liveHandler)
	if !ok {
		t.Fatal("Handler is not *liveHandler")
	}

	ctx := context.Background()

	// Call Shutdown multiple times
	var wg sync.WaitGroup
	errors := make(chan error, 3)

	for range 3 {
		wg.Go(func() {
			errors <- lh.Shutdown(ctx)
		})
	}

	wg.Wait()
	close(errors)

	// All calls should complete without error
	for err := range errors {
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
	}
}

func TestLiveHandler_Shutdown_WaitsForConnections(t *testing.T) {
	tmpl := Must(New("shutdown-wait-test"))
	handler := tmpl.Handle(&ShutdownController{}, AsState(&ShutdownState{}))

	lh, ok := handler.(*liveHandler)
	if !ok {
		t.Fatal("Handler is not *liveHandler")
	}

	// Simulate active connections by incrementing WaitGroup
	lh.shutdownWg.Add(2)

	// Start shutdown in background
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error)
	go func() {
		done <- lh.Shutdown(ctx)
	}()

	// Give shutdown a moment to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown should still be waiting
	select {
	case <-done:
		t.Error("Shutdown completed too early, should be waiting for connections")
	default:
		// Good, still waiting
	}

	// Release one connection
	lh.shutdownWg.Done()
	time.Sleep(50 * time.Millisecond)

	// Should still be waiting
	select {
	case <-done:
		t.Error("Shutdown completed too early, should be waiting for remaining connection")
	default:
		// Good, still waiting
	}

	// Release second connection
	lh.shutdownWg.Done()

	// Now shutdown should complete
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Shutdown did not complete after all connections released")
	}
}

func TestLiveHandler_Shutdown_TimeoutForce(t *testing.T) {
	tmpl := Must(New("shutdown-timeout-test"))
	handler := tmpl.Handle(&ShutdownController{}, AsState(&ShutdownState{}))

	lh, ok := handler.(*liveHandler)
	if !ok {
		t.Fatal("Handler is not *liveHandler")
	}

	// Simulate a connection that won't close
	lh.shutdownWg.Add(1)

	// Start shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := lh.Shutdown(ctx)
	elapsed := time.Since(start)

	// Should timeout
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got %v", err)
	}

	// Should timeout after approximately 100ms
	if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("Shutdown took %v, expected ~100ms", elapsed)
	}

	// Clean up - don't leave the WaitGroup hanging
	lh.shutdownWg.Done()
}

func TestLiveHandler_Shutdown_EmptyHandler(t *testing.T) {
	tmpl := Must(New("shutdown-empty-test"))
	handler := tmpl.Handle(&ShutdownController{}, AsState(&ShutdownState{}))

	// Shutdown with no connections should complete immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := handler.Shutdown(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	// Should complete very quickly (< 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Empty shutdown took too long: %v", elapsed)
	}
}
