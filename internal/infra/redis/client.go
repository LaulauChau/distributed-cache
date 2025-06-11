package redis

import (
	"context"
	"errors"
	"fmt"

	"github.com/lachau/distributed-cache/internal/domain/cache"
	domainErrors "github.com/lachau/distributed-cache/internal/domain/errors"
	"github.com/lachau/distributed-cache/internal/infra/config"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	client *redis.Client
}

func NewClient(cfg config.RedisConfig) (*Client, error) {
	if len(cfg.URLs) == 0 {
		return nil, fmt.Errorf("redis URLs cannot be empty")
	}

	opts, err := redis.ParseURL(cfg.URLs[0])
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	opts.PoolSize = cfg.PoolSize
	opts.ReadTimeout = cfg.Timeout
	opts.WriteTimeout = cfg.Timeout
	opts.MaxRetries = cfg.MaxRetries

	client := redis.NewClient(opts)

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{client: client}, nil
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", domainErrors.ErrInvalidKey
	}

	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domainErrors.ErrKeyNotFound
		}
		return "", fmt.Errorf("redis get failed: %w", err)
	}

	return result, nil
}

func (c *Client) Set(ctx context.Context, key, value string) error {
	if key == "" {
		return domainErrors.ErrInvalidKey
	}
	if value == "" {
		return domainErrors.ErrInvalidValue
	}

	err := c.client.Set(ctx, key, value, 0).Err()
	if err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if key == "" {
		return domainErrors.ErrInvalidKey
	}

	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("redis delete failed: %w", err)
	}

	return nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) HealthCheck(ctx context.Context) error {
	err := c.client.Ping(ctx).Err()
	if err != nil {
		return domainErrors.ErrCacheUnavailable
	}
	return nil
}

var _ cache.CacheRepository = (*Client)(nil)
