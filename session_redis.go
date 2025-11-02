package livetemplate

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key schema:
// livetemplate:session:{groupID}        -> Gob-encoded Stores
// livetemplate:session:{groupID}:access -> Last access timestamp (Unix seconds)
// TTL: Configurable (default: 24 hours)

const (
	sessionKeyPrefix       = "livetemplate:session:"
	sessionAccessKeySuffix = ":access"
	defaultSessionTTL      = 24 * time.Hour
	defaultMaxRetries      = 3
	defaultRetryDelay      = 100 * time.Millisecond
)

// RedisSessionStore implements SessionStore using Redis for distributed session management.
//
// Features:
// - Thread-safe for concurrent access
// - Automatic TTL refresh on access
// - Connection retry with exponential backoff
// - Serialization of Store state using gob encoding
// - Suitable for multi-instance deployments
//
// Redis Key Schema:
//   - livetemplate:session:{groupID}        -> Gob-encoded Stores
//   - livetemplate:session:{groupID}:access -> Last access timestamp
//   - TTL: 24 hours (configurable)
type RedisSessionStore struct {
	client     redis.UniversalClient
	ttl        time.Duration
	maxRetries int
	retryDelay time.Duration
	ctx        context.Context
}

// RedisSessionStoreOption configures RedisSessionStore
type RedisSessionStoreOption func(*RedisSessionStore)

// WithSessionTTL sets the time-to-live for session groups.
// Sessions not accessed within this duration will be automatically expired by Redis.
// Default: 24 hours
func WithSessionTTL(ttl time.Duration) RedisSessionStoreOption {
	return func(s *RedisSessionStore) {
		s.ttl = ttl
	}
}

// WithMaxRetries sets the maximum number of retry attempts for Redis operations.
// Default: 3
func WithMaxRetries(maxRetries int) RedisSessionStoreOption {
	return func(s *RedisSessionStore) {
		s.maxRetries = maxRetries
	}
}

// WithRetryDelay sets the base delay between retry attempts.
// Actual delay uses exponential backoff: delay * 2^attempt
// Default: 100ms
func WithRetryDelay(delay time.Duration) RedisSessionStoreOption {
	return func(s *RedisSessionStore) {
		s.retryDelay = delay
	}
}

// NewRedisSessionStore creates a new Redis-backed session store.
//
// The client parameter can be:
//   - redis.Client for single-node Redis
//   - redis.ClusterClient for Redis Cluster
//   - redis.Ring for Redis Ring (sharding)
//   - redis.FailoverClient for Redis Sentinel
//
// Example:
//
//	client := redis.NewClient(&redis.Options{
//	    Addr: "localhost:6379",
//	})
//	store := livetemplate.NewRedisSessionStore(client,
//	    livetemplate.WithSessionTTL(24*time.Hour),
//	)
func NewRedisSessionStore(client redis.UniversalClient, opts ...RedisSessionStoreOption) *RedisSessionStore {
	s := &RedisSessionStore{
		client:     client,
		ttl:        defaultSessionTTL,
		maxRetries: defaultMaxRetries,
		retryDelay: defaultRetryDelay,
		ctx:        context.Background(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Get retrieves the Stores for a session group.
// Returns nil if the group doesn't exist or if deserialization fails.
// Automatically refreshes the TTL on successful access.
func (s *RedisSessionStore) Get(groupID string) Stores {
	key := sessionKeyPrefix + groupID

	// Get the serialized stores with retry
	data, err := s.getWithRetry(key)
	if err != nil {
		// Session doesn't exist or Redis error
		return nil
	}

	if len(data) == 0 {
		return nil
	}

	// Deserialize stores
	stores, err := s.deserializeStores(data)
	if err != nil {
		// Deserialization failed - session is corrupted
		// Delete it to prevent further issues
		s.Delete(groupID)
		return nil
	}

	// Refresh TTL on successful access (fire and forget)
	go s.refreshTTL(groupID)

	return stores
}

// Set stores Stores for a session group.
// Creates a new group if it doesn't exist, updates if it does.
// Sets the TTL and updates the last access timestamp.
func (s *RedisSessionStore) Set(groupID string, stores Stores) {
	key := sessionKeyPrefix + groupID
	accessKey := key + sessionAccessKeySuffix

	// Serialize stores
	data, err := s.serializeStores(stores)
	if err != nil {
		// Serialization failed - log error but don't crash
		// This is a critical error that should be monitored
		return
	}

	// Use pipeline for atomic operations
	pipe := s.client.Pipeline()
	ctx := context.Background()

	// Set the serialized stores
	pipe.Set(ctx, key, data, s.ttl)

	// Set the last access timestamp
	pipe.Set(ctx, accessKey, time.Now().Unix(), s.ttl)

	// Execute pipeline with retry
	_ = s.execPipelineWithRetry(pipe)
}

// Delete removes a session group and all its state.
func (s *RedisSessionStore) Delete(groupID string) {
	key := sessionKeyPrefix + groupID
	accessKey := key + sessionAccessKeySuffix

	// Delete both keys
	ctx := context.Background()
	pipe := s.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, accessKey)

	// Execute pipeline (ignore errors on delete)
	_, _ = pipe.Exec(ctx)
}

// List returns all active session group IDs.
// Used for broadcasting and cleanup operations.
func (s *RedisSessionStore) List() []string {
	ctx := context.Background()
	pattern := sessionKeyPrefix + "*"

	// Scan for all session keys
	keys := make([]string, 0)
	iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Skip access keys
		if len(key) > len(sessionAccessKeySuffix) &&
			key[len(key)-len(sessionAccessKeySuffix):] == sessionAccessKeySuffix {
			continue
		}
		// Extract groupID from key
		groupID := key[len(sessionKeyPrefix):]
		keys = append(keys, groupID)
	}

	if err := iter.Err(); err != nil {
		// Return empty list on error
		return []string{}
	}

	return keys
}

// serializeStores converts Stores to gob-encoded bytes for storage in Redis.
//
// IMPORTANT: Store types must be registered with gob.Register() before use.
// For example:
//
//	type MyStore struct { Value int }
//	func (m *MyStore) Change(ctx *ActionContext) error { return nil }
//
//	func init() {
//	    gob.Register(&MyStore{})
//	}
func (s *RedisSessionStore) serializeStores(stores Stores) ([]byte, error) {
	if stores == nil {
		return []byte{}, nil
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Encode the stores map
	if err := enc.Encode(stores); err != nil {
		return nil, fmt.Errorf("failed to gob-encode stores: %w", err)
	}

	return buf.Bytes(), nil
}

// deserializeStores converts gob-encoded bytes from Redis back to Stores.
//
// IMPORTANT: Store types must be registered with gob.Register() before use.
// See serializeStores() for details.
func (s *RedisSessionStore) deserializeStores(data []byte) (Stores, error) {
	if len(data) == 0 {
		return nil, nil
	}

	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var stores Stores
	if err := dec.Decode(&stores); err != nil {
		return nil, fmt.Errorf("failed to gob-decode stores: %w", err)
	}

	return stores, nil
}

// refreshTTL extends the TTL for a session group on access.
// This prevents active sessions from expiring.
func (s *RedisSessionStore) refreshTTL(groupID string) {
	key := sessionKeyPrefix + groupID
	accessKey := key + sessionAccessKeySuffix

	ctx := context.Background()
	pipe := s.client.Pipeline()

	// Extend TTL for both keys
	pipe.Expire(ctx, key, s.ttl)
	pipe.Expire(ctx, accessKey, s.ttl)

	// Update access timestamp
	pipe.Set(ctx, accessKey, time.Now().Unix(), s.ttl)

	// Execute pipeline (ignore errors - this is best-effort)
	_, _ = pipe.Exec(ctx)
}

// getWithRetry performs a GET operation with exponential backoff retry.
func (s *RedisSessionStore) getWithRetry(key string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		ctx := context.Background()
		data, err := s.client.Get(ctx, key).Bytes()

		if err == nil {
			return data, nil
		}

		// Don't retry on key not found
		if err == redis.Nil {
			return nil, nil
		}

		lastErr = err

		// Don't sleep after last attempt
		if attempt < s.maxRetries {
			// Exponential backoff: delay * 2^attempt
			backoff := time.Duration(float64(s.retryDelay) * math.Pow(2, float64(attempt)))
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("redis get failed after %d retries: %w", s.maxRetries, lastErr)
}

// execPipelineWithRetry executes a pipeline with exponential backoff retry.
func (s *RedisSessionStore) execPipelineWithRetry(pipe redis.Pipeliner) error {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		ctx := context.Background()
		_, err := pipe.Exec(ctx)

		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after last attempt
		if attempt < s.maxRetries {
			// Exponential backoff: delay * 2^attempt
			backoff := time.Duration(float64(s.retryDelay) * math.Pow(2, float64(attempt)))
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("redis pipeline failed after %d retries: %w", s.maxRetries, lastErr)
}

// Ping checks if the Redis connection is healthy.
// Used for health check integration.
func (s *RedisSessionStore) Ping() error {
	ctx, cancel := context.WithTimeout(s.ctx, 1*time.Second)
	defer cancel()

	return s.client.Ping(ctx).Err()
}

// Close closes the Redis client connection.
// Should be called when shutting down the application.
func (s *RedisSessionStore) Close() error {
	return s.client.Close()
}
