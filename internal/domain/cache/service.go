package cache

import "context"

type CacheService interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	HealthCheck(ctx context.Context) error
	GetNodes() []string
}
