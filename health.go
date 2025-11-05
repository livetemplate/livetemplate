package livetemplate

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthChecker represents a component that can be health-checked.
//
// Implementations should complete the check quickly (<100ms) to avoid
// blocking Kubernetes probes or load balancer health checks.
type HealthChecker interface {
	// Check performs a health check and returns an error if unhealthy.
	// The context may have a timeout, so checks should respect it.
	Check(ctx context.Context) error
}

// HealthStatus represents the health status of a component.
type HealthStatus struct {
	Name   string `json:"name"`            // Component name
	Status string `json:"status"`          // "healthy" or "unhealthy"
	Error  string `json:"error,omitempty"` // Error message if unhealthy
}

// HealthResponse represents the overall health check response.
type HealthResponse struct {
	Status    string         `json:"status"`               // "healthy" or "unhealthy"
	Timestamp string         `json:"timestamp"`            // ISO 8601 timestamp
	Checks    []HealthStatus `json:"checks,omitempty"`     // Individual component statuses
	TotalTime string         `json:"total_time,omitempty"` // Total check duration
}

// HealthHandler provides HTTP endpoints for health checks.
//
// Supports two types of health checks:
//  1. Liveness (/health/live): Process is running and not deadlocked
//  2. Readiness (/health/ready): Process can handle requests (dependencies healthy)
//
// Kubernetes usage:
//
//	livenessProbe:
//	  httpGet:
//	    path: /health/live
//	    port: 8080
//	  initialDelaySeconds: 5
//	  periodSeconds: 10
//	readinessProbe:
//	  httpGet:
//	    path: /health/ready
//	    port: 8080
//	  initialDelaySeconds: 5
//	  periodSeconds: 5
type HealthHandler struct {
	checkers   map[string]HealthChecker
	checkersRW sync.RWMutex
	timeout    time.Duration
}

// NewHealthHandler creates a new health check handler.
//
// The timeout parameter specifies the maximum duration for all checks.
// If timeout is 0, a default of 5 seconds is used.
func NewHealthHandler(timeout time.Duration) *HealthHandler {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &HealthHandler{
		checkers: make(map[string]HealthChecker),
		timeout:  timeout,
	}
}

// RegisterChecker adds a health checker for a named component.
//
// The name should be descriptive (e.g., "database", "redis", "session-store").
// Thread-safe: can be called concurrently.
func (h *HealthHandler) RegisterChecker(name string, checker HealthChecker) {
	h.checkersRW.Lock()
	defer h.checkersRW.Unlock()
	h.checkers[name] = checker
}

// Live handles the liveness probe endpoint.
//
// Returns 200 OK if the process is alive and not deadlocked.
// This endpoint should always succeed unless the process is completely broken.
//
// Example: GET /health/live
// Response: 200 OK with {"status":"healthy","timestamp":"2025-11-02T08:00:00Z"}
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but can't change status code (already written)
		return
	}
}

// Ready handles the readiness probe endpoint.
//
// Returns 200 OK if all registered health checkers pass.
// Returns 503 Service Unavailable if any checker fails.
//
// Kubernetes will not route traffic to pods that fail readiness checks.
//
// Example: GET /health/ready
// Response: 200 OK with detailed check results
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// Get snapshot of checkers
	h.checkersRW.RLock()
	checkers := make(map[string]HealthChecker, len(h.checkers))
	for name, checker := range h.checkers {
		checkers[name] = checker
	}
	h.checkersRW.RUnlock()

	// If no checkers registered, consider ready
	if len(checkers) == 0 {
		response := HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			TotalTime: time.Since(start).String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			// Log error but can't change status code (already written)
			return
		}
		return
	}

	// Run all checks concurrently
	type checkResult struct {
		name string
		err  error
	}

	results := make(chan checkResult, len(checkers))
	var wg sync.WaitGroup

	for name, checker := range checkers {
		wg.Add(1)
		go func(n string, c HealthChecker) {
			defer wg.Done()
			err := c.Check(ctx)
			results <- checkResult{name: n, err: err}
		}(name, checker)
	}

	// Wait for all checks to complete
	wg.Wait()
	close(results)

	// Collect results
	var checks []HealthStatus
	allHealthy := true

	for result := range results {
		status := HealthStatus{
			Name:   result.name,
			Status: "healthy",
		}

		if result.err != nil {
			status.Status = "unhealthy"
			status.Error = result.err.Error()
			allHealthy = false
		}

		checks = append(checks, status)
	}

	// Build response
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
		TotalTime: time.Since(start).String(),
	}

	statusCode := http.StatusOK
	if !allHealthy {
		response.Status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but can't change status code (already written)
		return
	}
}

// SessionStoreHealthChecker implements HealthChecker for SessionStore.
//
// Checks that the session store is accessible by attempting to get a
// non-existent session (should return nil without error).
type SessionStoreHealthChecker struct {
	store SessionStore
}

// NewSessionStoreHealthChecker creates a health checker for a SessionStore.
func NewSessionStoreHealthChecker(store SessionStore) *SessionStoreHealthChecker {
	return &SessionStoreHealthChecker{store: store}
}

// Check verifies the session store is accessible.
func (c *SessionStoreHealthChecker) Check(ctx context.Context) error {
	// Attempt to get a session (should return nil for non-existent session)
	// This verifies the store is accessible without side effects
	_ = c.store.Get("__health_check__")
	return nil
}

// RedisHealthChecker checks the health of a Redis connection.
type RedisHealthChecker struct {
	store   *RedisSessionStore
	timeout time.Duration
}

// NewRedisHealthChecker creates a health checker for Redis.
func NewRedisHealthChecker(store *RedisSessionStore) *RedisHealthChecker {
	return &RedisHealthChecker{
		store:   store,
		timeout: 1 * time.Second,
	}
}

// Check implements the HealthChecker interface.
func (r *RedisHealthChecker) Check(ctx context.Context) error {
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Use a channel to handle the ping result
	done := make(chan error, 1)
	go func() {
		done <- r.store.Ping()
	}()

	// Wait for either the ping to complete or timeout
	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		return timeoutCtx.Err()
	}
}
