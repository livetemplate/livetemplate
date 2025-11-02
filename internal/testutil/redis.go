package testutil

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

// RedisContainer holds the Redis testcontainer and client
type RedisContainer struct {
	Container *redis.RedisContainer
	Client    goredis.UniversalClient
}

// StartRedisContainer starts a Redis container for testing
func StartRedisContainer(ctx context.Context, t *testing.T) (*RedisContainer, error) {
	t.Helper()

	// Start Redis container
	container, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		return nil, err
	}

	// Get connection string
	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, err
	}

	// Parse options and create client
	opt, err := goredis.ParseURL(connStr)
	if err != nil {
		container.Terminate(ctx)
		return nil, err
	}

	client := goredis.NewClient(opt)

	// Wait for Redis to be ready with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			client.Close()
			container.Terminate(context.Background())
			return nil, ctx.Err()
		default:
			if err := client.Ping(ctx).Err(); err == nil {
				return &RedisContainer{
					Container: container,
					Client:    client,
				}, nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Close closes the Redis client and terminates the container
func (rc *RedisContainer) Close(ctx context.Context) error {
	if rc.Client != nil {
		rc.Client.Close()
	}
	if rc.Container != nil {
		return rc.Container.Terminate(ctx)
	}
	return nil
}

// GetTestRedisClient is a convenience function that starts a Redis container
// and returns a client. It registers cleanup with t.Cleanup.
func GetTestRedisClient(t *testing.T) goredis.UniversalClient {
	t.Helper()

	ctx := context.Background()
	rc, err := StartRedisContainer(ctx, t)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		if err := rc.Close(context.Background()); err != nil {
			t.Logf("Failed to cleanup Redis container: %v", err)
		}
	})

	// Flush the database to ensure clean state
	rc.Client.FlushDB(ctx)

	return rc.Client
}
