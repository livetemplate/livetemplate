package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestServe_Defaults tests starting dev server with default settings
func TestServe_Defaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Start server using goroutine-based approach
	t.Log("Testing serve command with default settings...")

	handle, err := startServeInBackground(t, appDir, "--no-browser", "--port", "9870")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9870")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Log("✅ Server started with default settings")
}

// TestServe_CustomPort tests starting dev server on custom port
func TestServe_CustomPort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Test that server accepts --port flag
	port := 9876
	t.Logf("Testing serve command with custom port %d...", port)

	handle, err := startServeInBackground(t, appDir, "--port", fmt.Sprintf("%d", port), "--no-browser")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running on custom port
	url := fmt.Sprintf("http://localhost:%d", port)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Server not responding on port %d: %v", port, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Logf("✅ Server running on custom port %d", port)
}

// TestServe_ModeComponent tests component development mode
func TestServe_ModeComponent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Test that server accepts --mode component flag
	t.Log("Testing serve command with mode=component...")

	handle, err := startServeInBackground(t, appDir, "--mode", "component", "--no-browser", "--port", "9877")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9877")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	defer resp.Body.Close()

	t.Log("✅ Server started in component mode")
}

// TestServe_ModeKit tests kit development mode
func TestServe_ModeKit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app first
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Create a test kit
	t.Log("Creating test kit structure...")
	if err := runLvtCommand(t, appDir, "kits", "create", "testkit"); err != nil {
		t.Fatalf("Failed to create test kit: %v", err)
	}

	// Test that server accepts --mode kit flag
	t.Log("Testing serve command with mode=kit...")

	handle, err := startServeInBackground(t, appDir, "--mode", "kit", "--no-browser", "--port", "9882")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9882")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	defer resp.Body.Close()

	t.Log("✅ Server started in kit mode")
}

// TestServe_ModeApp tests app development mode
func TestServe_ModeApp(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Test that server accepts --mode app flag
	t.Log("Testing serve command with mode=app...")

	handle, err := startServeInBackground(t, appDir, "--mode", "app", "--no-browser", "--port", "9878")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9878")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	defer resp.Body.Close()

	t.Log("✅ Server started in app mode")
}

// TestServe_NoBrowser tests --no-browser flag
func TestServe_NoBrowser(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Test that server accepts --no-browser flag
	t.Log("Testing serve command with --no-browser...")

	handle, err := startServeInBackground(t, appDir, "--no-browser", "--port", "9879")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9879")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	defer resp.Body.Close()

	t.Log("✅ Server started with --no-browser")
}

// TestServe_NoReload tests --no-reload flag
func TestServe_NoReload(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Test that server accepts --no-reload flag
	t.Log("Testing serve command with --no-reload...")

	handle, err := startServeInBackground(t, appDir, "--no-reload", "--no-browser", "--port", "9880")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9880")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	defer resp.Body.Close()

	t.Log("✅ Server started with --no-reload")
}

// TestServe_VerifyServerResponds tests that server actually responds to HTTP requests
func TestServe_VerifyServerResponds(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Start server in background
	port := 9881
	t.Logf("Starting server on port %d...", port)

	handle, err := startServeInBackground(t, appDir, "--port", fmt.Sprintf("%d", port), "--no-browser")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = handle.Shutdown() }()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Try to connect to server
	url := fmt.Sprintf("http://localhost:%d", port)
	t.Logf("Testing connection to %s...", url)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)

	if err != nil {
		t.Fatalf("Could not connect to server: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("✅ Server responded with status: %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Log("✅ Server response test passed")
}

// TestServe_ContextCancellation tests that server shuts down properly on context cancellation
func TestServe_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Start server
	t.Log("Testing context-based shutdown...")

	handle, err := startServeInBackground(t, appDir, "--port", "9883", "--no-browser")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://localhost:9883")
	if err != nil {
		t.Fatalf("Server not responding: %v", err)
	}
	resp.Body.Close()
	t.Log("✅ Server started successfully")

	// Now shut down via context cancellation
	if err := handle.Shutdown(); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	// Wait a bit for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- handle.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Server stopped with error: %v", err)
		}
		t.Log("✅ Server shut down cleanly via context cancellation")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for server shutdown")
	}
}
