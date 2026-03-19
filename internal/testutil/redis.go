package testutil

import (
	"context"
	"log"
	"os/exec"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

var (
	dockerAvailable     bool
	dockerAvailableOnce sync.Once
)

// IsDockerAvailable checks if Docker is available for running containers.
// The result is cached after the first call.
func IsDockerAvailable() bool {
	dockerAvailableOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", "info")
		if err := cmd.Run(); err != nil {
			dockerAvailable = false
			return
		}
		dockerAvailable = true
	})
	return dockerAvailable
}

// SkipIfNoDocker skips the test if Docker is not available.
func SkipIfNoDocker(t *testing.T) {
	t.Helper()
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}
}

// RedisContainer holds the Redis testcontainer and client
type RedisContainer struct {
	Container *redis.RedisContainer
	Client    goredis.UniversalClient
}

// StartRedisContainer starts a Redis container for testing.
// If Docker is not available, the test is skipped.
func StartRedisContainer(ctx context.Context, t *testing.T) (*RedisContainer, error) {
	t.Helper()

	SkipIfNoDocker(t)

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
		if termErr := container.Terminate(ctx); termErr != nil {
			log.Printf("failed to terminate container after connection string error: %v", termErr)
		}
		return nil, err
	}

	// Parse options and create client
	opt, err := goredis.ParseURL(connStr)
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			log.Printf("failed to terminate container after URL parse error: %v", termErr)
		}
		return nil, err
	}

	client := goredis.NewClient(opt)

	// Wait for Redis to be ready with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			if err := client.Close(); err != nil {
				log.Printf("failed to close redis client on timeout: %v", err)
			}
			if err := container.Terminate(context.Background()); err != nil {
				log.Printf("failed to terminate container on timeout: %v", err)
			}
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
	var closeErr error
	if rc.Client != nil {
		if err := rc.Client.Close(); err != nil {
			closeErr = err
		}
	}
	if rc.Container != nil {
		if err := rc.Container.Terminate(ctx); err != nil {
			return err
		}
	}
	return closeErr
}

// GetTestRedisClient is a convenience function that starts a Redis container
// and returns a client. It registers cleanup with t.Cleanup.
// If Docker is not available, the test is skipped.
func GetTestRedisClient(t *testing.T) goredis.UniversalClient {
	t.Helper()

	SkipIfNoDocker(t)

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
