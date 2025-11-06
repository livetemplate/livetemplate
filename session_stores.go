package livetemplate

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionStore manages session groups, where each group contains Stores shared across connections.
//
// A session group is the fundamental isolation boundary: all connections with the same groupID
// share the same Stores instance. Different groupIDs have completely isolated state.
//
// For anonymous users: groupID is typically a browser-based identifier (all tabs share state).
// For authenticated users: groupID is typically the userID (each user has isolated state).
//
// Thread-safety: All implementations must be safe for concurrent access from multiple goroutines.
type SessionStore interface {
	// Get retrieves the Stores for a session group.
	// Returns nil if the group doesn't exist.
	// The context can be used for cancellation, timeouts, and tracing.
	Get(ctx context.Context, groupID string) Stores

	// Set stores Stores for a session group.
	// Creates a new group if it doesn't exist, updates if it does.
	// The context can be used for cancellation, timeouts, and tracing.
	Set(ctx context.Context, groupID string, stores Stores)

	// Delete removes a session group and all its state.
	// The context can be used for cancellation, timeouts, and tracing.
	Delete(ctx context.Context, groupID string)

	// List returns all active session group IDs.
	// Used for broadcasting and cleanup operations.
	// The context can be used for cancellation, timeouts, and tracing.
	List(ctx context.Context) []string
}

// ========================================
// In-Memory Session Store
// ========================================

// MemorySessionStore is an in-memory session store with automatic cleanup.
//
// Features:
// - Thread-safe for concurrent access
// - Tracks last access time for each group
// - Automatic cleanup of inactive groups (configurable TTL)
// - Suitable for single-instance deployments
//
// For multi-instance deployments, use a persistent SessionStore (e.g., Redis).
type MemorySessionStore struct {
	groups          map[string]Stores    // groupID → Stores
	lastAccess      map[string]time.Time // groupID → last access timestamp
	mu              sync.RWMutex         // Protects groups and lastAccess
	cleanupTTL      time.Duration        // Time to live for inactive groups
	cleanupInterval time.Duration        // How often to run cleanup (default: 1 hour)
	stopCh          chan struct{}        // Signal to stop cleanup goroutine
	ctx             context.Context      // Context for cleanup goroutine
	cancel          context.CancelFunc   // Cancel function for cleanup
}

// SessionStoreOption configures MemorySessionStore
type SessionStoreOption func(*MemorySessionStore)

// WithCleanupTTL sets the time-to-live for inactive session groups.
// Groups not accessed within this duration will be automatically cleaned up.
// Default: 24 hours
func WithCleanupTTL(ttl time.Duration) SessionStoreOption {
	return func(s *MemorySessionStore) {
		s.cleanupTTL = ttl
	}
}

// WithCleanupInterval sets how often the cleanup process runs.
// Lower intervals = more frequent cleanup but more CPU usage.
// Higher intervals = less frequent cleanup but potentially more memory usage.
// Default: 1 hour
func WithCleanupInterval(interval time.Duration) SessionStoreOption {
	return func(s *MemorySessionStore) {
		s.cleanupInterval = interval
	}
}

// NewMemorySessionStore creates a new in-memory session store with automatic cleanup.
//
// Default configuration:
// - Cleanup TTL: 24 hours
// - Cleanup interval: 1 hour
//
// The cleanup goroutine runs in the background and removes session groups that
// haven't been accessed within the TTL period. This prevents memory leaks from
// abandoned sessions.
//
// Call Close() to stop the cleanup goroutine when shutting down.
func NewMemorySessionStore(opts ...SessionStoreOption) *MemorySessionStore {
	ctx, cancel := context.WithCancel(context.Background())

	s := &MemorySessionStore{
		groups:          make(map[string]Stores),
		lastAccess:      make(map[string]time.Time),
		cleanupTTL:      24 * time.Hour, // Default: 24 hours
		cleanupInterval: 1 * time.Hour,  // Default: 1 hour
		stopCh:          make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Start cleanup goroutine
	go s.cleanupLoop()

	return s
}

// Get retrieves the Stores for a session group.
// Updates the last access time for the group.
// The context parameter is accepted for interface compliance but not used for in-memory operations.
func (s *MemorySessionStore) Get(ctx context.Context, groupID string) Stores {
	s.mu.Lock()
	defer s.mu.Unlock()

	stores := s.groups[groupID]
	if stores != nil {
		s.lastAccess[groupID] = time.Now()
	}
	return stores
}

// Set stores Stores for a session group.
// Updates the last access time for the group.
// The context parameter is accepted for interface compliance but not used for in-memory operations.
func (s *MemorySessionStore) Set(ctx context.Context, groupID string, stores Stores) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.groups[groupID] = stores
	s.lastAccess[groupID] = time.Now()
}

// Delete removes a session group and all its state.
// The context parameter is accepted for interface compliance but not used for in-memory operations.
func (s *MemorySessionStore) Delete(ctx context.Context, groupID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.groups, groupID)
	delete(s.lastAccess, groupID)
}

// List returns all active session group IDs.
// The context parameter is accepted for interface compliance but not used for in-memory operations.
func (s *MemorySessionStore) List(ctx context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groupIDs := make([]string, 0, len(s.groups))
	for groupID := range s.groups {
		groupIDs = append(groupIDs, groupID)
	}
	return groupIDs
}

// Close stops the cleanup goroutine.
// Should be called when shutting down the application.
func (s *MemorySessionStore) Close() {
	s.cancel()
	<-s.stopCh
}

// cleanupLoop runs in the background and removes inactive session groups.
func (s *MemorySessionStore) cleanupLoop() {
	defer close(s.stopCh)

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes session groups that haven't been accessed within the TTL period.
func (s *MemorySessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for groupID, lastAccess := range s.lastAccess {
		if now.Sub(lastAccess) > s.cleanupTTL {
			delete(s.groups, groupID)
			delete(s.lastAccess, groupID)
		}
	}
}

// ========================================
// Redis Session Store
// ========================================

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
	client       redis.UniversalClient
	ttl          time.Duration
	maxRetries   int
	retryDelay   time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	refreshChan  chan string    // Channel for TTL refresh requests
	refreshCache sync.Map       // Debounce map to avoid duplicate refreshes
	wg           sync.WaitGroup // Wait group for worker goroutine
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
	ctx, cancel := context.WithCancel(context.Background())

	s := &RedisSessionStore{
		client:      client,
		ttl:         defaultSessionTTL,
		maxRetries:  defaultMaxRetries,
		retryDelay:  defaultRetryDelay,
		ctx:         ctx,
		cancel:      cancel,
		refreshChan: make(chan string, 1000), // Buffered channel for async refresh
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Start background worker for TTL refresh operations
	s.wg.Add(1)
	go s.refreshWorker()

	return s
}

// Get retrieves the Stores for a session group.
// Returns nil if the group doesn't exist or if deserialization fails.
// Automatically refreshes the TTL on successful access.
// The context is used for Redis operations and can timeout/cancel requests.
func (s *RedisSessionStore) Get(ctx context.Context, groupID string) Stores {
	key := sessionKeyPrefix + groupID

	// Get the serialized stores with retry
	data, err := s.getWithRetry(ctx, key)
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
		s.Delete(ctx, groupID)
		return nil
	}

	// Queue TTL refresh asynchronously via worker (non-blocking)
	select {
	case s.refreshChan <- groupID:
		// Queued successfully
	default:
		// Channel full - skip refresh (better than spawning goroutine)
	}

	return stores
}

// Set stores Stores for a session group.
// Creates a new group if it doesn't exist, updates if it does.
// Sets the TTL and updates the last access timestamp.
// The context is used for Redis operations and can timeout/cancel requests.
func (s *RedisSessionStore) Set(ctx context.Context, groupID string, stores Stores) {
	key := sessionKeyPrefix + groupID
	accessKey := key + sessionAccessKeySuffix

	// Serialize stores
	data, err := s.serializeStores(stores)
	if err != nil {
		// CRITICAL: Serialization failed - data will not be persisted!
		// This should be monitored in production as it indicates data loss.
		log.Printf("ERROR: RedisSessionStore.Set(%s): serialization failed: %v", groupID, err)
		// TODO: Consider changing Set() signature to return error in next major version
		return
	}

	// Use pipeline for atomic operations
	pipe := s.client.Pipeline()

	// Set the serialized stores
	pipe.Set(ctx, key, data, s.ttl)

	// Set the last access timestamp
	pipe.Set(ctx, accessKey, time.Now().Unix(), s.ttl)

	// Execute pipeline with retry
	if err := s.execPipelineWithRetry(ctx, pipe); err != nil {
		// CRITICAL: Redis persistence failed - data will not be persisted!
		// This should be monitored in production as it indicates data loss.
		log.Printf("ERROR: RedisSessionStore.Set(%s): redis persistence failed: %v", groupID, err)
		// TODO: Consider changing Set() signature to return error in next major version
	}
}

// Delete removes a session group and all its state.
// The context is used for Redis operations and can timeout/cancel requests.
func (s *RedisSessionStore) Delete(ctx context.Context, groupID string) {
	key := sessionKeyPrefix + groupID
	accessKey := key + sessionAccessKeySuffix

	// Delete both keys
	pipe := s.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, accessKey)

	// Execute pipeline (ignore errors on delete)
	_, _ = pipe.Exec(ctx)
}

// List returns all active session group IDs.
// Used for broadcasting and cleanup operations.
// The context is used for Redis operations and can timeout/cancel requests.
func (s *RedisSessionStore) List(ctx context.Context) []string {
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
		return nil, fmt.Errorf("failed to gob-encode stores: %w (hint: custom Store types must be registered with gob.Register() in init())", err)
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
		return nil, fmt.Errorf("failed to gob-decode stores: %w (hint: custom Store types must be registered with gob.Register() in init())", err)
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
// The context is used for Redis operations and respects cancellation/timeout.
func (s *RedisSessionStore) getWithRetry(ctx context.Context, key string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

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

			// Use context-aware sleep
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return nil, fmt.Errorf("redis get failed after %d retries: %w", s.maxRetries, lastErr)
}

// execPipelineWithRetry executes a pipeline with exponential backoff retry.
// The context is used for Redis operations and respects cancellation/timeout.
func (s *RedisSessionStore) execPipelineWithRetry(ctx context.Context, pipe redis.Pipeliner) error {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := pipe.Exec(ctx)

		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after last attempt
		if attempt < s.maxRetries {
			// Exponential backoff: delay * 2^attempt
			backoff := time.Duration(float64(s.retryDelay) * math.Pow(2, float64(attempt)))

			// Use context-aware sleep
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return fmt.Errorf("redis pipeline failed after %d retries: %w", s.maxRetries, lastErr)
}

// Ping checks if the Redis connection is healthy.
// Used for health check integration.
func (s *RedisSessionStore) Ping() error {
	return s.PingContext(s.ctx)
}

// PingContext pings the Redis server with the given context.
// The context can be used to set timeouts or cancel the operation early.
func (s *RedisSessionStore) PingContext(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	return s.client.Ping(pingCtx).Err()
}

// Close closes the Redis client connection and stops the refresh worker.
// Should be called when shutting down the application.
func (s *RedisSessionStore) Close() error {
	// Signal worker to stop
	s.cancel()

	// Wait for worker to finish
	s.wg.Wait()

	// Close Redis client
	return s.client.Close()
}

// refreshWorker processes TTL refresh requests from the channel.
// Uses debouncing to avoid refreshing the same key multiple times in quick succession.
// Runs in a single goroutine to prevent goroutine leaks.
func (s *RedisSessionStore) refreshWorker() {
	defer s.wg.Done()

	// Use a ticker for periodic batch processing
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	pending := make(map[string]struct{}) // Deduplicate refresh requests

	for {
		select {
		case <-s.ctx.Done():
			// Context cancelled - shutdown
			return

		case groupID := <-s.refreshChan:
			// Queue for batch processing
			pending[groupID] = struct{}{}

		case <-ticker.C:
			// Process pending refreshes in batch
			if len(pending) == 0 {
				continue
			}

			// Process all pending refreshes
			for groupID := range pending {
				// Check if recently refreshed (debounce)
				if _, exists := s.refreshCache.LoadOrStore(groupID, time.Now()); !exists {
					// Not in cache - perform refresh
					s.refreshTTL(groupID)

					// Schedule cache cleanup after 1 second (debounce window)
					go func(id string) {
						time.Sleep(1 * time.Second)
						s.refreshCache.Delete(id)
					}(groupID)
				}
			}

			// Clear pending map
			pending = make(map[string]struct{})
		}
	}
}
