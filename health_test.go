package livetemplate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockHealthChecker is a mock HealthChecker for testing
type mockHealthChecker struct {
	err   error
	delay time.Duration
}

func (m *mockHealthChecker) Check(ctx context.Context) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.err
}

func TestHealthHandler_Live(t *testing.T) {
	handler := NewHealthHandler(0)

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	handler.Live(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", response.Status)
	}

	if response.Timestamp == "" {
		t.Error("Expected timestamp to be set")
	}
}

func TestHealthHandler_Ready_NoCheckers(t *testing.T) {
	handler := NewHealthHandler(0)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", response.Status)
	}
}

func TestHealthHandler_Ready_AllHealthy(t *testing.T) {
	handler := NewHealthHandler(0)

	// Register multiple healthy checkers
	handler.RegisterChecker("database", &mockHealthChecker{err: nil})
	handler.RegisterChecker("redis", &mockHealthChecker{err: nil})
	handler.RegisterChecker("session-store", &mockHealthChecker{err: nil})

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", response.Status)
	}

	if len(response.Checks) != 3 {
		t.Errorf("Expected 3 checks, got %d", len(response.Checks))
	}

	// Verify all checks are healthy
	for _, check := range response.Checks {
		if check.Status != "healthy" {
			t.Errorf("Expected check %q to be healthy, got %q", check.Name, check.Status)
		}
		if check.Error != "" {
			t.Errorf("Expected no error for check %q, got %q", check.Name, check.Error)
		}
	}
}

func TestHealthHandler_Ready_OneUnhealthy(t *testing.T) {
	handler := NewHealthHandler(0)

	// Register mixed healthy/unhealthy checkers
	handler.RegisterChecker("database", &mockHealthChecker{err: nil})
	handler.RegisterChecker("redis", &mockHealthChecker{err: errors.New("connection refused")})
	handler.RegisterChecker("session-store", &mockHealthChecker{err: nil})

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got %q", response.Status)
	}

	// Find the redis check
	var redisCheck *HealthStatus
	for i, check := range response.Checks {
		if check.Name == "redis" {
			redisCheck = &response.Checks[i]
			break
		}
	}

	if redisCheck == nil {
		t.Fatal("Redis check not found in response")
	}

	if redisCheck.Status != "unhealthy" {
		t.Errorf("Expected redis check to be unhealthy, got %q", redisCheck.Status)
	}

	if redisCheck.Error != "connection refused" {
		t.Errorf("Expected error 'connection refused', got %q", redisCheck.Error)
	}
}

func TestHealthHandler_Ready_Timeout(t *testing.T) {
	handler := NewHealthHandler(100 * time.Millisecond)

	// Register a slow checker that should timeout
	handler.RegisterChecker("slow-service", &mockHealthChecker{
		delay: 200 * time.Millisecond,
	})

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got %q", response.Status)
	}

	if len(response.Checks) != 1 {
		t.Fatalf("Expected 1 check, got %d", len(response.Checks))
	}

	check := response.Checks[0]
	if check.Status != "unhealthy" {
		t.Errorf("Expected check to be unhealthy due to timeout, got %q", check.Status)
	}

	if check.Error == "" {
		t.Error("Expected error message for timeout")
	}
}

func TestHealthHandler_RegisterChecker_Concurrent(t *testing.T) {
	handler := NewHealthHandler(0)

	// Register checkers concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			handler.RegisterChecker("checker", &mockHealthChecker{})
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no race conditions (would be caught by go test -race)
	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	handler.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHealthHandler_ConcurrentChecks(t *testing.T) {
	handler := NewHealthHandler(1 * time.Second)

	// Register multiple checkers with slight delays
	for i := 0; i < 5; i++ {
		handler.RegisterChecker("checker", &mockHealthChecker{
			delay: 50 * time.Millisecond,
		})
	}

	start := time.Now()

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	handler.Ready(w, req)

	elapsed := time.Since(start)

	// Checks should run concurrently, so total time should be ~50ms, not 250ms
	if elapsed > 150*time.Millisecond {
		t.Errorf("Checks took too long (%v), should run concurrently", elapsed)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSessionStoreHealthChecker(t *testing.T) {
	store := NewMemorySessionStore()
	checker := NewSessionStoreHealthChecker(store)

	ctx := context.Background()
	if err := checker.Check(ctx); err != nil {
		t.Errorf("SessionStore health check failed: %v", err)
	}
}

func TestSessionStoreHealthChecker_Integration(t *testing.T) {
	handler := NewHealthHandler(0)
	store := NewMemorySessionStore()

	// Register session store health check
	handler.RegisterChecker("session-store", NewSessionStoreHealthChecker(store))

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", response.Status)
	}

	// Verify session-store check
	var sessionStoreCheck *HealthStatus
	for i, check := range response.Checks {
		if check.Name == "session-store" {
			sessionStoreCheck = &response.Checks[i]
			break
		}
	}

	if sessionStoreCheck == nil {
		t.Fatal("session-store check not found")
	}

	if sessionStoreCheck.Status != "healthy" {
		t.Errorf("Expected session-store to be healthy, got %q", sessionStoreCheck.Status)
	}
}

func TestHealthResponse_JSONFormat(t *testing.T) {
	handler := NewHealthHandler(0)
	handler.RegisterChecker("test", &mockHealthChecker{err: errors.New("test error")})

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.Ready(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Verify required fields
	if response.Status == "" {
		t.Error("Status field is empty")
	}
	if response.Timestamp == "" {
		t.Error("Timestamp field is empty")
	}
	if response.TotalTime == "" {
		t.Error("TotalTime field is empty")
	}
	if len(response.Checks) == 0 {
		t.Error("Checks array is empty")
	}
}
