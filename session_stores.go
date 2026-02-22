package livetemplate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log/slog"
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
	// Get retrieves the state for a session group.
	// Returns nil if the group doesn't exist.
	// The context can be used for cancellation, timeouts, and tracing.
	Get(ctx context.Context, groupID string) interface{}

	// Set stores state for a session group.
	// Creates a new group if it doesn't exist, updates if it does.
	// The context can be used for cancellation, timeouts, and tracing.
	Set(ctx context.Context, groupID string, state interface{})

	// Delete removes a session group and all its state.
	// The context can be used for cancellation, timeouts, and tracing.
	Delete(ctx context.Context, groupID string)

	// List returns all active session group IDs.
	// Used for broadcasting and cleanup operations.
	// The context can be used for cancellation, timeouts, and tracing.
	List(ctx context.Context) []string
}

// SingleStoreSetter is an optional interface that SessionStore implementations
// can implement to support targeted persistence of individual stores.
//
// This is an optimization for multi-store setups: instead of persisting all stores
// after every action, only the modified store is persisted. This is especially
// beneficial for Redis-based stores where network roundtrips are expensive.
//
// Implementation guidelines:
//   - MemorySessionStore: no-op (references are already updated in-place)
//   - RedisSessionStore: serialize and write only the specified store
//
// The framework will check if the SessionStore implements this interface
// and use SetStore when available, falling back to Set otherwise.
type SingleStoreSetter interface {
	// SetStore persists a single store within a session group.
	// This is more efficient than Set() when only one store has changed.
	// The storeName is the key in the Stores map (empty string for single-store mode).
	SetStore(ctx context.Context, groupID string, storeName string, store interface{})
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
	groups          map[string]interface{} // groupID → state
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
		groups:          make(map[string]interface{}),
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

// Get retrieves the state for a session group.
// Updates the last access time for the group.
// The context parameter is accepted for interface compliance but not used for in-memory operations.
func (s *MemorySessionStore) Get(ctx context.Context, groupID string) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.groups[groupID]
	if state != nil {
		s.lastAccess[groupID] = time.Now()
	}
	return state
}

// Set stores state for a session group.
// Updates the last access time for the group.
// The context parameter is accepted for interface compliance but not used for in-memory operations.
func (s *MemorySessionStore) Set(ctx context.Context, groupID string, state interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.groups[groupID] = state
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

// SetStore is a no-op for MemorySessionStore.
// Memory stores use references, so modifications to store objects are already
// persisted in-place. This method is provided for interface compliance.
func (s *MemorySessionStore) SetStore(ctx context.Context, groupID string, storeName string, store interface{}) {
	// No-op: MemorySessionStore uses references, so the store is already updated in-place.
	// We just update the last access time to prevent premature cleanup.
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAccess[groupID] = time.Now()
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

// Redis key schema (v2 - Hash-based):
// livetemplate:session:{groupID} -> Redis HASH
//   - "_meta" field: JSON {"version":"2","updated_at":timestamp}
//   - "{storeName}" field: base64-encoded Gob data (preserves type information)
// TTL: Configurable (default: 24 hours)
//
// Why Gob instead of JSON for store data?
// Gob preserves Go type information, allowing stores to be unmarshaled back to their
// original types. JSON would lose type info and return map[string]interface{}.
//
// Migration: Legacy v1 keys (Gob-encoded blob of entire Stores map) are automatically
// detected and migrated to v2 format on read. The legacy key is deleted after migration.

const (
	sessionKeyPrefix       = "livetemplate:session:"
	sessionAccessKeySuffix = ":access" // Legacy v1, kept for cleanup
	defaultSessionTTL      = 24 * time.Hour
	defaultMaxRetries      = 3
	defaultRetryDelay      = 100 * time.Millisecond

	// Hash field names for v2 schema
	metaField      = "_meta"
	schemaVersion2 = "2"
)

// sessionMeta holds metadata for a session stored in the Redis hash.
type sessionMeta struct {
	Version   string `json:"version"`
	UpdatedAt int64  `json:"updated_at"`
}

// RedisSessionStore implements SessionStore using Redis for distributed session management.
//
// Features:
// - Thread-safe for concurrent access
// - Automatic TTL refresh on access
// - Connection retry with exponential backoff
// - Hash-based storage with JSON serialization (v2 schema)
// - Automatic migration from legacy Gob-encoded blob format (v1)
// - Suitable for multi-instance deployments
//
// Redis Key Schema (v2):
//   - livetemplate:session:{groupID} -> Redis HASH
//   - "_meta" field: JSON metadata (version, updated_at)
//   - "{storeName}" fields: JSON-encoded individual stores
//   - TTL: 24 hours (configurable)
//
// The v2 schema uses Redis HASH to enable granular updates via HSET,
// which is more efficient than re-writing the entire session blob.
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

// Get retrieves the state for a session group.
// Returns nil if the group doesn't exist or if deserialization fails.
// Automatically refreshes the TTL on successful access.
// The context is used for Redis operations and can timeout/cancel requests.
//
// Note: RedisSessionStore now stores state as JSON-encoded data.
func (s *RedisSessionStore) Get(ctx context.Context, groupID string) interface{} {
	key := sessionKeyPrefix + groupID

	// First, try to get as a hash (v2 schema)
	hashData, err := s.client.HGetAll(ctx, key).Result()
	if err != nil && err != redis.Nil {
		// Redis error
		return nil
	}

	// Check if this is a v2 hash (has _meta field)
	if metaJSON, ok := hashData[metaField]; ok {
		var meta sessionMeta
		if err := json.Unmarshal([]byte(metaJSON), &meta); err == nil && meta.Version == schemaVersion2 {
			// This is v2 format - deserialize from hash
			stores := s.deserializeFromHash(hashData)
			if stores != nil {
				s.queueTTLRefresh(groupID)
			}
			return stores
		}
	}

	// No v2 data found - check for legacy v1 blob
	// This happens when HGETALL returns empty (key doesn't exist as hash)
	// or when the key exists but isn't a hash (legacy string format)
	if len(hashData) == 0 {
		// Try legacy blob format
		data, err := s.getWithRetry(ctx, key)
		if err != nil || len(data) == 0 {
			return nil
		}

		// Legacy v1 format found - this is incompatible with Controller+State pattern.
		// Delete the old session so it will be recreated with Mount().
		// Note: In pre-1.0 library, breaking changes are expected.
		_, err = s.deserializeLegacyStores(data)
		if err != nil {
			slog.Warn("Found legacy v1 session but failed to decode",
				slog.String("group_id", groupID),
				slog.Any("error", err))
		} else {
			slog.Info("Found legacy v1 session, deleting (incompatible with Controller+State)",
				slog.String("group_id", groupID))
		}
		s.Delete(ctx, groupID)
		return nil
	}

	// Empty hash or corrupted data
	return nil
}

// queueTTLRefresh queues an asynchronous TTL refresh for the given group.
func (s *RedisSessionStore) queueTTLRefresh(groupID string) {
	select {
	case s.refreshChan <- groupID:
		// Queued successfully
	default:
		// Channel full - skip refresh (better than spawning goroutine)
	}
}

// deserializeFromHash deserializes state from a Redis hash (v2 format).
// In Controller+State pattern, state is stored as JSON under the "state" key.
func (s *RedisSessionStore) deserializeFromHash(hashData map[string]string) interface{} {
	// New Controller+State pattern: state is stored as plain JSON under "state" key
	if stateJSON, ok := hashData["state"]; ok {
		var state interface{}
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
			slog.Warn("Failed to unmarshal state",
				slog.Any("error", err))
			return nil
		}
		return state
	}

	// No state found
	return nil
}

// serializeSingleStore encodes a single store for Redis storage.
//
// If the store has `lvt:"state"` tagged fields, only those fields are serialized.
// This allows controllers to have non-serializable dependencies (DB, Logger, etc.)
// while still persisting their state.
//
// If the store has no state tags, the entire store is serialized using Gob
// (backward compatible behavior).
func (s *RedisSessionStore) serializeSingleStore(store interface{}) (string, error) {
	// Check if store has state-tagged fields
	stateFields := ExtractState(store)
	if stateFields != nil {
		// Serialize only state fields
		data, err := SerializeState(stateFields)
		if err != nil {
			return "", fmt.Errorf("state serialization failed: %w", err)
		}
		// Prefix with "s:" to indicate state-only format
		return "s:" + base64.StdEncoding.EncodeToString(data), nil
	}

	// No state tags - serialize entire store using Gob (backward compatible)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(&store); err != nil {
		return "", fmt.Errorf("gob encode failed: %w (hint: custom types must be registered with gob.Register() in init())", err)
	}

	// Prefix with "g:" to indicate Gob format
	return "g:" + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// deserializeSingleStore decodes a single store from Redis storage.
//
// Supported formats (detected by prefix):
//   - "s:..." - State-only format (JSON envelope containing only `lvt:"state"` fields)
//   - "g:..." - Gob format (entire store encoded with gob)
//   - No prefix - Legacy gob format (backward compatibility)
// StateData wraps raw state bytes from Redis for later injection.
// When mount.go encounters this type in Stores, it knows to:
// 1. Clone the template's original store (which has dependencies)
// 2. Deserialize state into the clone using DeserializeState()
// 3. Inject state into the clone using InjectState()
//
// This type is exported so mount.go can detect and handle it.
type StateData struct {
	Raw []byte // Raw state envelope bytes (JSON format from SerializeState)
}

// IsStateData checks if a value is wrapped state data that needs injection.
func IsStateData(v interface{}) bool {
	_, ok := v.(*StateData)
	return ok
}

// GetStateData extracts the StateData wrapper from a value, if present.
// Returns nil if the value is not StateData.
func GetStateData(v interface{}) *StateData {
	sd, _ := v.(*StateData)
	return sd
}

// Set stores state for a session group.
// For the Controller+State pattern, this stores the state as JSON.
// The context is used for Redis operations and can timeout/cancel requests.
func (s *RedisSessionStore) Set(ctx context.Context, groupID string, state interface{}) {
	key := sessionKeyPrefix + groupID

	// Handle nil state by deleting the session
	if state == nil {
		s.Delete(ctx, groupID)
		return
	}

	// Serialize state as JSON
	stateJSON, err := json.Marshal(state)
	if err != nil {
		slog.Error("RedisSessionStore.Set: failed to marshal state",
			slog.String("group_id", groupID),
			slog.Any("error", err))
		return
	}

	// Build hash fields
	fields := make(map[string]interface{})

	// Add metadata
	meta := sessionMeta{
		Version:   schemaVersion2,
		UpdatedAt: time.Now().Unix(),
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		slog.Error("RedisSessionStore.Set: failed to marshal metadata",
			slog.String("group_id", groupID),
			slog.Any("error", err))
		return
	}
	fields[metaField] = string(metaJSON)

	// Store state under "state" key
	fields["state"] = string(stateJSON)

	// Use pipeline for atomic operations
	pipe := s.client.Pipeline()

	// Delete the key first to clear any old fields that may no longer exist
	pipe.Del(ctx, key)

	// Set all hash fields
	pipe.HSet(ctx, key, fields)

	// Set TTL
	pipe.Expire(ctx, key, s.ttl)

	// Execute pipeline with retry
	if err := s.execPipelineWithRetry(ctx, pipe); err != nil {
		slog.Error("RedisSessionStore.Set: redis persistence failed",
			slog.String("group_id", groupID),
			slog.Any("error", err))
	}
}

// SetStore persists a single store within a session group.
// Uses Redis HSET to update only the specified store field, which is
// more efficient than Set() when only one store has changed.
//
// This is the primary optimization of v2 schema: instead of serializing
// and writing all stores, we only serialize and write the modified store.
func (s *RedisSessionStore) SetStore(ctx context.Context, groupID string, storeName string, store interface{}) {
	key := sessionKeyPrefix + groupID

	// Serialize the single store using Gob (preserves type information)
	encoded, err := s.serializeSingleStore(store)
	if err != nil {
		slog.Error("RedisSessionStore.SetStore: serialization failed",
			slog.String("group_id", groupID),
			slog.String("store_name", storeName),
			slog.Any("error", err))
		return
	}

	// Update metadata
	meta := sessionMeta{
		Version:   schemaVersion2,
		UpdatedAt: time.Now().Unix(),
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		slog.Error("RedisSessionStore.SetStore: failed to marshal metadata",
			slog.String("group_id", groupID),
			slog.Any("error", err))
		return
	}

	// Use pipeline to update both the store field and metadata atomically
	pipe := s.client.Pipeline()

	// Set the store field and metadata
	pipe.HSet(ctx, key, storeName, encoded)
	pipe.HSet(ctx, key, metaField, string(metaJSON))

	// Refresh TTL
	pipe.Expire(ctx, key, s.ttl)

	// Execute pipeline with retry
	if err := s.execPipelineWithRetry(ctx, pipe); err != nil {
		slog.Error("RedisSessionStore.SetStore: redis persistence failed",
			slog.String("group_id", groupID),
			slog.String("store_name", storeName),
			slog.Any("error", err))
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

// deserializeLegacyStores converts gob-encoded bytes from Redis back to a map.
// This is used for v1 → v2 migration only (legacy format support).
//
// Note: In Controller+State pattern, this legacy format is incompatible with the new
// state type. Legacy sessions will be deleted and recreated as new sessions.
func (s *RedisSessionStore) deserializeLegacyStores(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var stores map[string]interface{}
	if err := dec.Decode(&stores); err != nil {
		return nil, fmt.Errorf("failed to gob-decode legacy stores: %w", err)
	}

	return stores, nil
}

// refreshTTL extends the TTL for a session group on access.
// This prevents active sessions from expiring.
func (s *RedisSessionStore) refreshTTL(groupID string) {
	key := sessionKeyPrefix + groupID

	ctx := context.Background()

	// Just extend the TTL on the hash key (v2 schema doesn't use separate access key)
	s.client.Expire(ctx, key, s.ttl)
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

// refreshWorker processes TTL refresh requests in batches.
// Uses debouncing to avoid refreshing the same key multiple times in quick succession.
// Runs in a single goroutine to prevent goroutine leaks.
//
// Cleanup Behavior:
// On shutdown (ctx.Done()), pending refresh operations in the queue are dropped.
// This is acceptable because:
//   - Redis keys have their TTL set during Get/Set operations
//   - Refresh is a best-effort optimization to extend TTL for active sessions
//   - Dropped refreshes only affect edge cases during shutdown
//   - Sessions will naturally expire after TTL if not accessed again
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
			// Note: Pending refresh operations in the queue are dropped.
			// This is acceptable as explained in the function documentation.
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
