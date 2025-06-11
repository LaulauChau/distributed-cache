package redis

import (
	"context"
	"testing"
	"time"

	"github.com/lachau/distributed-cache/internal/infra/config"
)

func TestRedisPool_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	redisURL, cleanup := setupRedisContainer(t)
	defer cleanup()

	cfg := config.RedisConfig{
		URLs:       []string{redisURL},
		PoolSize:   3,
		Timeout:    3 * time.Second,
		MaxRetries: 2,
	}

	pool, err := NewPool(cfg)
	if err != nil {
		t.Fatalf("failed to create redis pool: %v", err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			t.Logf("failed to close pool: %v", err)
		}
	}()

	t.Run("Get client from pool", func(t *testing.T) {
		client, err := pool.GetClient()
		if err != nil {
			t.Errorf("GetClient failed: %v", err)
		}

		if client == nil {
			t.Error("expected non-nil client")
		}
	})

	t.Run("Pool size", func(t *testing.T) {
		size := pool.Size()
		if size != 1 {
			t.Errorf("expected pool size 1, got %d", size)
		}
	})

	t.Run("Health check", func(t *testing.T) {
		ctx := context.Background()
		err := pool.HealthCheck(ctx)
		if err != nil {
			t.Errorf("health check failed: %v", err)
		}
	})

	t.Run("Operations through pool", func(t *testing.T) {
		client, err := pool.GetClient()
		if err != nil {
			t.Fatalf("GetClient failed: %v", err)
		}

		ctx := context.Background()
		key := "pool-test-key"
		value := "pool-test-value"

		err = client.Set(ctx, key, value)
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
}

func TestRedisPool_MultipleURLs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	redisURL1, cleanup1 := setupRedisContainer(t)
	defer cleanup1()

	redisURL2, cleanup2 := setupRedisContainer(t)
	defer cleanup2()

	cfg := config.RedisConfig{
		URLs:       []string{redisURL1, redisURL2},
		PoolSize:   2,
		Timeout:    3 * time.Second,
		MaxRetries: 2,
	}

	pool, err := NewPool(cfg)
	if err != nil {
		t.Fatalf("failed to create redis pool: %v", err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			t.Logf("failed to close pool: %v", err)
		}
	}()

	if pool.Size() != 2 {
		t.Errorf("expected pool size 2, got %d", pool.Size())
	}

	ctx := context.Background()
	err = pool.HealthCheck(ctx)
	if err != nil {
		t.Errorf("health check failed: %v", err)
	}
}

func TestRedisPool_FailureScenarios(t *testing.T) {
	t.Run("Empty URLs", func(t *testing.T) {
		cfg := config.RedisConfig{
			URLs: []string{},
		}

		_, err := NewPool(cfg)
		if err == nil {
			t.Error("expected error for empty URLs")
		}
	})

	t.Run("Invalid URL", func(t *testing.T) {
		cfg := config.RedisConfig{
			URLs:       []string{"invalid-url"},
			PoolSize:   1,
			Timeout:    1 * time.Second,
			MaxRetries: 1,
		}

		_, err := NewPool(cfg)
		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})

	t.Run("Closed pool operations", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		redisURL, cleanup := setupRedisContainer(t)
		defer cleanup()

		cfg := config.RedisConfig{
			URLs:       []string{redisURL},
			PoolSize:   1,
			Timeout:    3 * time.Second,
			MaxRetries: 2,
		}

		pool, err := NewPool(cfg)
		if err != nil {
			t.Fatalf("failed to create redis pool: %v", err)
		}

		err = pool.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		_, err = pool.GetClient()
		if err == nil {
			t.Error("expected error when getting client from closed pool")
		}

		ctx := context.Background()
		err = pool.HealthCheck(ctx)
		if err == nil {
			t.Error("expected error when health checking closed pool")
		}
	})
}
