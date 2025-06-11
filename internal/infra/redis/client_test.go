package redis

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/lachau/distributed-cache/internal/domain/errors"
	"github.com/lachau/distributed-cache/internal/infra/config"
)

type testingT interface {
	Fatalf(format string, args ...interface{})
	Logf(format string, args ...interface{})
}

func setupRedisContainer(t testingT) (string, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	redisURL := "redis://" + host + ":" + port.Port()

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return redisURL, cleanup
}

func TestRedisClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	redisURL, cleanup := setupRedisContainer(t)
	defer cleanup()

	cfg := config.RedisConfig{
		URLs:       []string{redisURL},
		PoolSize:   5,
		Timeout:    3 * time.Second,
		MaxRetries: 2,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Logf("failed to close client: %v", err)
		}
	}()

	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := client.Set(ctx, key, value)
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}

		result, err := client.Get(ctx, key)
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}

		if result != value {
			t.Errorf("expected %s, got %s", value, result)
		}
	})

	t.Run("Get nonexistent key", func(t *testing.T) {
		_, err := client.Get(ctx, "nonexistent-key")
		if err != errors.ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("Delete key", func(t *testing.T) {
		key := "delete-test"
		value := "delete-value"

		err := client.Set(ctx, key, value)
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}

		err = client.Delete(ctx, key)
		if err != nil {
			t.Errorf("Delete failed: %v", err)
		}

		_, err = client.Get(ctx, key)
		if err != errors.ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound after delete, got %v", err)
		}
	})

	t.Run("Invalid key validation", func(t *testing.T) {
		err := client.Set(ctx, "", "value")
		if err != errors.ErrInvalidKey {
			t.Errorf("expected ErrInvalidKey for empty key, got %v", err)
		}

		_, err = client.Get(ctx, "")
		if err != errors.ErrInvalidKey {
			t.Errorf("expected ErrInvalidKey for empty key, got %v", err)
		}

		err = client.Delete(ctx, "")
		if err != errors.ErrInvalidKey {
			t.Errorf("expected ErrInvalidKey for empty key, got %v", err)
		}
	})

	t.Run("Invalid value validation", func(t *testing.T) {
		err := client.Set(ctx, "key", "")
		if err != errors.ErrInvalidValue {
			t.Errorf("expected ErrInvalidValue for empty value, got %v", err)
		}
	})

	t.Run("Health check", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		if err != nil {
			t.Errorf("health check failed: %v", err)
		}
	})
}

func TestRedisClient_ConnectionFailure(t *testing.T) {
	cfg := config.RedisConfig{
		URLs:       []string{"redis://localhost:9999"},
		PoolSize:   1,
		Timeout:    1 * time.Second,
		MaxRetries: 1,
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("expected connection failure, got nil")
	}
}

func BenchmarkRedisClient_Operations(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	redisURL, cleanup := setupRedisContainer(b)
	defer cleanup()

	cfg := config.RedisConfig{
		URLs:       []string{redisURL},
		PoolSize:   10,
		Timeout:    3 * time.Second,
		MaxRetries: 2,
	}

	client, err := NewClient(cfg)
	if err != nil {
		b.Fatalf("failed to create redis client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			b.Logf("failed to close client: %v", err)
		}
	}()

	ctx := context.Background()

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := client.Set(ctx, "bench-key", "bench-value")
			if err != nil {
				b.Fatalf("Set failed: %v", err)
			}
		}
	})

	if err := client.Set(ctx, "bench-key", "bench-value"); err != nil {
		b.Fatalf("failed to set up benchmark data: %v", err)
	}

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.Get(ctx, "bench-key")
			if err != nil {
				b.Fatalf("Get failed: %v", err)
			}
		}
	})
}
