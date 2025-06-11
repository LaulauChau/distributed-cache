package redis

import (
	"context"
	"fmt"
	"sync"

	"github.com/lachau/distributed-cache/internal/infra/config"
	"github.com/lachau/distributed-cache/pkg/logger"
)

type Pool struct {
	clients []*Client
	current int
	mu      sync.RWMutex
	closed  bool
}

func NewPool(cfg config.RedisConfig) (*Pool, error) {
	if len(cfg.URLs) == 0 {
		return nil, fmt.Errorf("no redis URLs provided")
	}

	clients := make([]*Client, len(cfg.URLs))

	for i, url := range cfg.URLs {
		clientCfg := cfg
		clientCfg.URLs = []string{url}

		client, err := NewClient(clientCfg)
		if err != nil {
			logger.Error("failed to create redis client during pool initialization",
				"url", url,
				"error", err,
			)
			for j := range i {
				if closeErr := clients[j].Close(); closeErr != nil {
					logger.Warn("failed to close redis client during cleanup",
						"client_index", j,
						"error", closeErr,
					)
				}
			}
			return nil, fmt.Errorf("failed to create client for %s: %w", url, err)
		}
		clients[i] = client
		logger.Info("redis client created successfully",
			"url", url,
			"pool_size", len(clients),
		)
	}

	logger.Info("redis pool created successfully",
		"total_clients", len(clients),
		"urls", cfg.URLs,
	)

	return &Pool{
		clients: clients,
		current: 0,
	}, nil
}

func (p *Pool) GetClient() (*Client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}

	if len(p.clients) == 0 {
		return nil, fmt.Errorf("no clients available")
	}

	client := p.clients[p.current]
	p.current = (p.current + 1) % len(p.clients)

	return client, nil
}

func (p *Pool) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	for i, client := range p.clients {
		if err := client.HealthCheck(ctx); err != nil {
			logger.Error("redis client health check failed",
				"client_index", i,
				"error", err,
			)
			return err
		}
	}

	return nil
}

func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	logger.Info("closing redis pool", "client_count", len(p.clients))

	var errs []error
	for i, client := range p.clients {
		if err := client.Close(); err != nil {
			logger.Error("failed to close redis client",
				"client_index", i,
				"error", err,
			)
			errs = append(errs, err)
		}
	}

	p.closed = true
	p.clients = nil

	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}

	logger.Info("redis pool closed successfully")
	return nil
}

func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}
