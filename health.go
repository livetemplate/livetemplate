package livetemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
		slog.Error("Failed to encode liveness response",
			slog.String("error", err.Error()))
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
			slog.Error("Failed to encode readiness response (no checkers)",
				slog.String("error", err.Error()))
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
		slog.Error("Failed to encode readiness response",
			slog.String("error", err.Error()),
			slog.Int("status_code", statusCode))
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

// healthCheckStore is a minimal Store implementation for health checking.
type healthCheckStore struct {
	value string
}

func (h *healthCheckStore) Change(ctx *ActionContext) error {
	return nil
}

// Check verifies the session store is accessible by performing a Get/Set/Delete cycle.
// This properly validates that the store can read, write, and delete data.
func (c *SessionStoreHealthChecker) Check(ctx context.Context) error {
	const healthCheckKey = "__livetemplate_health_check__"

	// Create a simple test store
	testStores := make(Stores)
	testStores["_health"] = &healthCheckStore{value: "ok"}

	// Test write operation
	c.store.Set(ctx, healthCheckKey, testStores)

	// Test read operation - verify we can retrieve what we just set
	retrieved := c.store.Get(ctx, healthCheckKey)
	if retrieved == nil {
		return fmt.Errorf("health check failed: unable to retrieve test data from session store")
	}

	// Verify the data matches
	if healthStore, ok := retrieved["_health"]; ok {
		if hs, ok := healthStore.(*healthCheckStore); ok {
			if hs.value != "ok" {
				return fmt.Errorf("health check failed: retrieved data does not match expected value")
			}
		} else {
			return fmt.Errorf("health check failed: retrieved data has unexpected type")
		}
	} else {
		return fmt.Errorf("health check failed: retrieved data is missing expected key")
	}

	// Test delete operation - clean up after ourselves
	c.store.Delete(ctx, healthCheckKey)

	// Verify deletion worked
	if afterDelete := c.store.Get(ctx, healthCheckKey); afterDelete != nil {
		return fmt.Errorf("health check failed: unable to delete test data from session store")
	}

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
// Uses PingContext to ensure the operation respects context cancellation and timeouts.
func (r *RedisHealthChecker) Check(ctx context.Context) error {
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Use PingContext which properly respects context cancellation
	// No goroutine needed - PingContext handles timeout internally
	return r.store.PingContext(timeoutCtx)
}
