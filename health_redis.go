package livetemplate

import (
	"context"
	"time"
)

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
